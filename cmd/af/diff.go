package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
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
		cmd.Printf("Node %s: no amendments — statement is unchanged\n", nodeID)
		return nil
	}

	// Build version list: version 0 = original, version N = after amendment N
	versions := buildVersionList(amendments)

	// Handle --since-challenge
	if sinceChallenge != "" {
		return diffSinceChallenge(cmd, st, nodeID, sinceChallenge, amendments, versions, format)
	}

	// Handle --all
	if showAll {
		return diffAll(cmd, nodeID, amendments, versions, format)
	}

	// Handle --version or default (previous vs current)
	fromVersion := version
	if fromVersion == -1 {
		// Default: previous version vs current
		fromVersion = len(amendments) - 1
	}

	toVersion := len(amendments) // current = last version

	if fromVersion < 0 || fromVersion >= toVersion {
		return fmt.Errorf("invalid version %d: node has %d amendment(s), valid versions are 0-%d", fromVersion, len(amendments), len(amendments))
	}

	diff := computeDiff(versions[fromVersion], versions[toVersion], fromVersion, toVersion)

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, []diffResult{diff})
	}
	return renderDiffText(cmd, nodeID, []diffResult{diff})
}

// buildVersionList returns all statement versions: index 0 = original, index N = after amendment N.
func buildVersionList(amendments []service.Amendment) []string {
	versions := make([]string, 0, len(amendments)+1)
	versions = append(versions, amendments[0].PreviousStatement)
	for _, a := range amendments {
		versions = append(versions, a.NewStatement)
	}
	return versions
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
func diffAll(cmd *cobra.Command, nodeID service.NodeID, amendments []service.Amendment, versions []string, format string) error {
	diffs := make([]diffResult, 0, len(amendments))
	for i := range amendments {
		diffs = append(diffs, computeDiff(versions[i], versions[i+1], i, i+1))
	}

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, diffs)
	}
	return renderDiffText(cmd, nodeID, diffs)
}

// diffSinceChallenge shows the diff between the statement at challenge-raise time and current.
func diffSinceChallenge(cmd *cobra.Command, st *service.State, nodeID service.NodeID, challengeID string, amendments []service.Amendment, versions []string, format string) error {
	challenge := st.GetChallenge(challengeID)
	if challenge == nil {
		return fmt.Errorf("challenge %q not found", challengeID)
	}

	// Find the version that was active when the challenge was raised.
	// The challenge was raised at challenge.Created; find the last amendment before that time.
	versionAtChallenge := 0 // default: original
	for i, a := range amendments {
		if !a.Timestamp.After(challenge.Created) {
			versionAtChallenge = i + 1
		}
	}

	currentVersion := len(versions) - 1
	if versionAtChallenge == currentVersion {
		cmd.Printf("Node %s: no changes since challenge %s was raised\n", nodeID, challengeID)
		return nil
	}

	diff := computeDiff(versions[versionAtChallenge], versions[currentVersion], versionAtChallenge, currentVersion)

	if format == "json" {
		return renderDiffJSON(cmd, nodeID, []diffResult{diff})
	}

	cmd.Printf("Changes to node %s since challenge %s:\n\n", nodeID, challengeID)
	renderSingleDiff(cmd, diff)
	return nil
}

// diffOutputJSON is the JSON output for the diff command.
type diffOutputJSON struct {
	NodeID string       `json:"node_id"`
	Diffs  []diffResult `json:"diffs"`
}

// renderDiffJSON renders diffs as JSON.
func renderDiffJSON(cmd *cobra.Command, nodeID service.NodeID, diffs []diffResult) error {
	output := diffOutputJSON{
		NodeID: nodeID.String(),
		Diffs:  diffs,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// renderDiffText renders diffs in human-readable text.
func renderDiffText(cmd *cobra.Command, nodeID service.NodeID, diffs []diffResult) error {
	cmd.Printf("Node %s: %d diff(s)\n\n", nodeID, len(diffs))

	for i, d := range diffs {
		if i > 0 {
			cmd.Println("---")
			cmd.Println()
		}
		renderSingleDiff(cmd, d)
	}

	return nil
}

// renderSingleDiff renders one diff between two versions.
func renderSingleDiff(cmd *cobra.Command, d diffResult) {
	cmd.Printf("v%d → v%d", d.FromVersion, d.ToVersion)
	if !d.Changed {
		cmd.Printf("  (no change)\n")
		return
	}
	cmd.Println()

	// Line-by-line diff
	fromLines := strings.Split(d.From, "\n")
	toLines := strings.Split(d.To, "\n")

	// Simple line diff: show removed lines with -, added lines with +
	removed, added := diffLines(fromLines, toLines)

	for _, line := range removed {
		cmd.Printf("  - %s\n", line)
	}
	for _, line := range added {
		cmd.Printf("  + %s\n", line)
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
