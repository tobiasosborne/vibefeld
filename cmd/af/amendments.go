package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// newAmendmentsCmd creates the amendments command for listing node version history.
func newAmendmentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "amendments <node-id>",
		GroupID: GroupQuery,
		Short:   "List amendment history for a node",
		Long: `List all amendments (version history) for a proof node.

Each amendment records the previous statement, new statement, timestamp,
and who made the change. Amendments are numbered starting from 1.

The original statement (version 0) is shown first, followed by each
amendment in chronological order.

Examples:
  af amendments 1.1              List all amendments for node 1.1
  af amendments 1.1 -f json      Machine-readable output`,
		Args: cobra.ExactArgs(1),
		RunE: runAmendments,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runAmendments executes the amendments command.
func runAmendments(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

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

	if format == "json" {
		return renderAmendmentsJSON(cmd, nodeID, n.Statement, amendments)
	}

	return renderAmendmentsText(cmd, nodeID, n.Statement, amendments)
}

// renderAmendmentsText renders the amendment history in human-readable text.
// Statement amendments are numbered v1..vN (unchanged); dependency amendments
// are listed in ledger order and do not consume a statement version.
func renderAmendmentsText(cmd *cobra.Command, nodeID service.NodeID, currentStatement string, amendments []service.Amendment) error {
	if len(amendments) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no amendments (original statement unchanged)\n", nodeID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nCurrent statement:\n  %s\n", currentStatement)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Node %s: %d amendment(s)\n\n", nodeID, len(amendments))

	stmts := statementAmendments(amendments)
	original := currentStatement
	if len(stmts) > 0 {
		original = stmts[0].PreviousStatement
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[v0] Original\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n\n", original)

	version := 0
	for _, a := range amendments {
		if a.Kind == service.AmendmentKindDependencies {
			fmt.Fprintf(cmd.OutOrStdout(), "[deps] %s by %s", a.Timestamp.String(), a.Owner)
			if a.Reopened {
				fmt.Fprint(cmd.OutOrStdout(), " (reopened)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  reason: %s\n", a.Reason)
			fmt.Fprintf(cmd.OutOrStdout(), "  dependencies: %s -> %s\n", joinIDs(a.PreviousDependencies), joinIDs(a.NewDependencies))
			fmt.Fprintf(cmd.OutOrStdout(), "  validation_deps: %s -> %s\n\n", joinIDs(a.PreviousValidationDeps), joinIDs(a.NewValidationDeps))
			continue
		}
		version++
		fmt.Fprintf(cmd.OutOrStdout(), "[v%d] %s by %s\n", version, a.Timestamp.String(), a.Owner)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n\n", a.NewStatement)
	}

	return nil
}

// statementAmendments filters out dependency amendments.
func statementAmendments(amendments []service.Amendment) []service.Amendment {
	var out []service.Amendment
	for _, a := range amendments {
		if a.Kind != service.AmendmentKindDependencies {
			out = append(out, a)
		}
	}
	return out
}

func joinIDs(ids []service.NodeID) string {
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(service.ToStringSlice(ids), ", ")
}

// amendmentJSON is the JSON representation of a single statement amendment version.
type amendmentJSON struct {
	Version   int    `json:"version"`
	Timestamp string `json:"timestamp,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Statement string `json:"statement"`
}

// dependencyAmendmentJSON is the JSON representation of a dependency amendment.
type dependencyAmendmentJSON struct {
	Timestamp              string   `json:"timestamp,omitempty"`
	Owner                  string   `json:"owner,omitempty"`
	Reason                 string   `json:"reason,omitempty"`
	PreviousDependencies   []string `json:"previous_dependencies,omitempty"`
	NewDependencies        []string `json:"new_dependencies,omitempty"`
	PreviousValidationDeps []string `json:"previous_validation_deps,omitempty"`
	NewValidationDeps      []string `json:"new_validation_deps,omitempty"`
	Reopened               bool     `json:"reopened,omitempty"`
}

// amendmentsOutputJSON is the JSON output for the amendments command.
type amendmentsOutputJSON struct {
	NodeID               string                    `json:"node_id"`
	TotalAmendments      int                       `json:"total_amendments"`
	Versions             []amendmentJSON           `json:"versions"`
	DependencyAmendments []dependencyAmendmentJSON `json:"dependency_amendments,omitempty"`
}

// renderAmendmentsJSON renders the amendment history as JSON. Statement
// version numbering is unchanged; dependency amendments are a separate array.
func renderAmendmentsJSON(cmd *cobra.Command, nodeID service.NodeID, currentStatement string, amendments []service.Amendment) error {
	stmts := statementAmendments(amendments)
	output := amendmentsOutputJSON{
		NodeID:          nodeID.String(),
		TotalAmendments: len(amendments),
	}

	if len(stmts) == 0 {
		output.Versions = []amendmentJSON{{Version: 0, Statement: currentStatement}}
	} else {
		output.Versions = make([]amendmentJSON, 0, len(stmts)+1)
		output.Versions = append(output.Versions, amendmentJSON{Version: 0, Statement: stmts[0].PreviousStatement})
		for i, a := range stmts {
			output.Versions = append(output.Versions, amendmentJSON{
				Version:   i + 1,
				Timestamp: a.Timestamp.String(),
				Owner:     a.Owner,
				Statement: a.NewStatement,
			})
		}
	}

	for _, a := range amendments {
		if a.Kind != service.AmendmentKindDependencies {
			continue
		}
		output.DependencyAmendments = append(output.DependencyAmendments, dependencyAmendmentJSON{
			Timestamp:              a.Timestamp.String(),
			Owner:                  a.Owner,
			Reason:                 a.Reason,
			PreviousDependencies:   service.ToStringSlice(a.PreviousDependencies),
			NewDependencies:        service.ToStringSlice(a.NewDependencies),
			PreviousValidationDeps: service.ToStringSlice(a.PreviousValidationDeps),
			NewValidationDeps:      service.ToStringSlice(a.NewValidationDeps),
			Reopened:               a.Reopened,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func init() {
	rootCmd.AddCommand(newAmendmentsCmd())
}
