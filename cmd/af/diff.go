package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// newDiffCmd creates the diff command for showing amendment diffs.
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff <node-id>",
		GroupID: GroupQuery,
		Short:   "Show diff between node statement versions",
		Long: `Show the difference between versions of a node's statement.

By default, shows the diff between the previous version and the current
version. Use --version to compare specific versions, or --all to show
all diffs in sequence.

Version numbers start at 0 (the original statement). Each amendment
increments the version number.

Use --since-challenge <challenge-id> to show all changes made since a
specific challenge was raised (useful for verifiers checking if their
challenge was addressed).

Examples:
  af diff 1.1                           Diff: previous vs current
  af diff 1.1 --version 0               Diff: original vs current
  af diff 1.1 --version 1               Diff: version 1 vs current
  af diff 1.1 --all                     Show all diffs in sequence
  af diff 1.1 --since-challenge ch-42   Changes since challenge ch-42
  af diff 1.1 -f json                   Machine-readable output`,
		Args: cobra.ExactArgs(1),
		RunE: runDiff,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().IntP("version", "v", -1, "Compare this version against current (-1 = previous)")
	cmd.Flags().Bool("all", false, "Show all diffs in chronological order")
	cmd.Flags().String("since-challenge", "", "Show changes since a challenge was raised")

	return cmd
}

// runDiff executes the diff command.
func runDiff(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	version, _ := cmd.Flags().GetInt("version")
	showAll, _ := cmd.Flags().GetBool("all")
	sinceChallenge, _ := cmd.Flags().GetString("since-challenge")

	if format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be text or json", format)
	}

	nodeID, err := service.ParseNodeID(args[0])
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", args[0], err)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return err
	}

	st, err := svc.LoadState()
	if err != nil {
		return err
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}

	amendments := st.GetAmendmentHistory(nodeID)

	if len(amendments) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no amendments — statement is unchanged\n", nodeID)
		return nil
	}

	stmts := statementAmendments(amendments)
	deps := dependencyChanges(amendments)

	if len(stmts) == 0 && len(deps) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no amendments — statement is unchanged\n", nodeID)
		return nil
	}

	// Build version list: version 0 = original, version N = after statement amendment N.
	versions := buildVersionList(stmts)

	// Handle --since-challenge
	if sinceChallenge != "" {
		return diffSinceChallenge(cmd, st, nodeID, sinceChallenge, stmts, deps, versions, format)
	}

	// Handle --all
	if showAll {
		return diffAll(cmd, nodeID, stmts, deps, versions, format)
	}

	if len(stmts) == 0 {
		// Only dependency amendments: nothing to compare statement-wise.
		if format == "json" {
			return renderDiffJSON(cmd, nodeID, nil, deps)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no statement changes\n", nodeID)
		renderDependencyChanges(cmd, deps)
		return nil
	}

	// Handle --version or default (previous vs current)
	fromVersion := version
	if fromVersion == -1 {
		// Default: previous version vs current
		fromVersion = len(stmts) - 1
	}

	toVersion := len(stmts) // current = last version

	if fromVersion < 0 || fromVersion >= toVersion {
		return fmt.Errorf("invalid version %d: node has %d amendment(s), valid versions are 0-%d", fromVersion, len(stmts), len(stmts))
	}

	diff := computeDiff(versions[fromVersion], versions[toVersion], fromVersion, toVersion)

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, []diffResult{diff}, deps)
	}
	renderDiffText(cmd, nodeID, []diffResult{diff}, deps)
	return nil
}

// buildVersionList returns all statement versions: index 0 = original, index N = after statement amendment N.
func buildVersionList(amendments []service.Amendment) []string {
	if len(amendments) == 0 {
		return []string{""}
	}
	versions := make([]string, 0, len(amendments)+1)
	versions = append(versions, amendments[0].PreviousStatement)
	for _, a := range amendments {
		versions = append(versions, a.NewStatement)
	}
	return versions
}

// depChange is one dependency amendment in the diff output.
type depChange struct {
	Timestamp              string   `json:"timestamp,omitempty"`
	Owner                  string   `json:"owner,omitempty"`
	Reason                 string   `json:"reason,omitempty"`
	Reopened               bool     `json:"reopened,omitempty"`
	PreviousDependencies   []string `json:"previous_dependencies,omitempty"`
	NewDependencies        []string `json:"new_dependencies,omitempty"`
	PreviousValidationDeps []string `json:"previous_validation_deps,omitempty"`
	NewValidationDeps      []string `json:"new_validation_deps,omitempty"`
}

// dependencyChanges maps dependency amendments onto the diff output type.
func dependencyChanges(amendments []service.Amendment) []depChange {
	var out []depChange
	for _, a := range amendments {
		if a.Kind != service.AmendmentKindDependencies {
			continue
		}
		out = append(out, depChange{
			Timestamp:              a.Timestamp.String(),
			Owner:                  a.Owner,
			Reason:                 a.Reason,
			Reopened:               a.Reopened,
			PreviousDependencies:   service.ToStringSlice(a.PreviousDependencies),
			NewDependencies:        service.ToStringSlice(a.NewDependencies),
			PreviousValidationDeps: service.ToStringSlice(a.PreviousValidationDeps),
			NewValidationDeps:      service.ToStringSlice(a.NewValidationDeps),
		})
	}
	return out
}

// filterDepsSince keeps dependency amendments at or after t.
func filterDepsSince(deps []depChange, t service.Timestamp) []depChange {
	var out []depChange
	for _, d := range deps {
		if ts, err := service.ParseTimestamp(d.Timestamp); err == nil && !ts.Before(t) {
			out = append(out, d)
		}
	}
	return out
}

// diffResult represents a single diff between two versions.
type diffResult struct {
	FromVersion int    `json:"from_version"`
	ToVersion   int    `json:"to_version"`
	From        string `json:"from"`
	To          string `json:"to"`
	Changed     bool   `json:"changed"`
}

// computeDiff creates a diff result between two versions.
func computeDiff(from, to string, fromVersion, toVersion int) diffResult {
	return diffResult{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		From:        from,
		To:          to,
		Changed:     from != to,
	}
}

// diffAll shows all diffs in chronological order.
func diffAll(cmd *cobra.Command, nodeID service.NodeID, stmts []service.Amendment, deps []depChange, versions []string, format string) error {
	diffs := make([]diffResult, 0, len(stmts))
	for i := range stmts {
		diffs = append(diffs, computeDiff(versions[i], versions[i+1], i, i+1))
	}

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, diffs, deps)
	}
	return renderDiffText(cmd, nodeID, diffs, deps)
}

// diffSinceChallenge shows the diff between the statement at challenge-raise time and current, plus any dependency changes since.
func diffSinceChallenge(cmd *cobra.Command, st *service.State, nodeID service.NodeID, challengeID string, stmts []service.Amendment, deps []depChange, versions []string, format string) error {
	challenge := st.GetChallenge(challengeID)
	if challenge == nil {
		return fmt.Errorf("challenge %q not found", challengeID)
	}

	// Find the version that was active when the challenge was raised.
	// The challenge was raised at challenge.Created; find the last statement amendment before that time.
	versionAtChallenge := 0 // default: original
	for i, a := range stmts {
		if !a.Timestamp.After(challenge.Created) {
			versionAtChallenge = i + 1
		}
	}

	depsSince := filterDepsSince(deps, challenge.Created)

	currentVersion := len(versions) - 1
	if versionAtChallenge == currentVersion && len(depsSince) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no changes since challenge %s was raised\n", nodeID, challengeID)
		return nil
	}

	var diffs []diffResult
	if versionAtChallenge != currentVersion {
		diffs = append(diffs, computeDiff(versions[versionAtChallenge], versions[currentVersion], versionAtChallenge, currentVersion))
	}

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, diffs, depsSince)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Changes to node %s since challenge %s:\n\n", nodeID, challengeID)
	for i, d := range diffs {
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "---")
		}
		renderSingleDiff(cmd, d)
	}
	renderDependencyChanges(cmd, depsSince)
	return nil
}

// diffOutputJSON is the JSON output for the diff command.
type diffOutputJSON struct {
	NodeID            string       `json:"node_id"`
	Diffs             []diffResult `json:"diffs,omitempty"`
	DependencyChanges []depChange  `json:"dependency_changes,omitempty"`
}

// renderDiffJSON renders diffs as JSON.
func renderDiffJSON(cmd *cobra.Command, nodeID service.NodeID, diffs []diffResult, deps []depChange) error {
	output := diffOutputJSON{
		NodeID:            nodeID.String(),
		Diffs:             diffs,
		DependencyChanges: deps,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// renderDiffText renders diffs in human-readable text.
func renderDiffText(cmd *cobra.Command, nodeID service.NodeID, diffs []diffResult, deps []depChange) error {
	if len(diffs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: %d diff(s)\n\n", nodeID, len(diffs))
	}

	for i, d := range diffs {
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "---")
			fmt.Fprintln(cmd.OutOrStdout())
		}
		renderSingleDiff(cmd, d)
	}

	renderDependencyChanges(cmd, deps)
	return nil
}

// renderDependencyChanges prints the dependency-amendment section.
func renderDependencyChanges(cmd *cobra.Command, deps []depChange) {
	if len(deps) == 0 {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nDependency changes (%d):\n", len(deps))
	for _, d := range deps {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s by %s: %s\n", d.Timestamp, d.Owner, d.Reason)
		fmt.Fprintf(cmd.OutOrStdout(), "    dependencies: %s -> %s\n", joinStringList(d.PreviousDependencies), joinStringList(d.NewDependencies))
		fmt.Fprintf(cmd.OutOrStdout(), "    validation_deps: %s -> %s\n", joinStringList(d.PreviousValidationDeps), joinStringList(d.NewValidationDeps))
		if d.Reopened {
			fmt.Fprintf(cmd.OutOrStdout(), "    reopened: validated -> pending\n")
		}
	}
}

func joinStringList(vals []string) string {
	if len(vals) == 0 {
		return "(none)"
	}
	return strings.Join(vals, ", ")
}

// renderSingleDiff renders one diff between two versions.
func renderSingleDiff(cmd *cobra.Command, d diffResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "v%d → v%d", d.FromVersion, d.ToVersion)
	if !d.Changed {
		fmt.Fprintf(cmd.OutOrStdout(), "  (no change)\n")
		return
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Line-by-line diff
	fromLines := strings.Split(d.From, "\n")
	toLines := strings.Split(d.To, "\n")

	// Simple line diff: show removed lines with -, added lines with +
	removed, added := diffLines(fromLines, toLines)

	for _, line := range removed {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", line)
	}
	for _, line := range added {
		fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", line)
	}
}

// diffLines computes removed and added lines between two line slices.
// Uses a simple set-based approach suitable for natural-language statements.
func diffLines(from, to []string) (removed, added []string) {
	fromSet := make(map[string]int)
	toSet := make(map[string]int)

	for _, line := range from {
		fromSet[line]++
	}
	for _, line := range to {
		toSet[line]++
	}

	// Lines in from but not in to (or fewer occurrences)
	countUsed := make(map[string]int)
	for _, line := range from {
		countUsed[line]++
		if countUsed[line] > toSet[line] {
			removed = append(removed, line)
		}
	}

	// Lines in to but not in from (or more occurrences)
	countUsed2 := make(map[string]int)
	for _, line := range to {
		countUsed2[line]++
		if countUsed2[line] > fromSet[line] {
			added = append(added, line)
		}
	}

	return removed, added
}

func init() {
	rootCmd.AddCommand(newDiffCmd())
}
