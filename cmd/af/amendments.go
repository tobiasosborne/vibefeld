package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
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
func renderAmendmentsText(cmd *cobra.Command, nodeID service.NodeID, currentStatement string, amendments []service.Amendment) error {
	if len(amendments) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s: no amendments (original statement unchanged)\n", nodeID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nCurrent statement:\n  %s\n", currentStatement)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Node %s: %d amendment(s)\n\n", nodeID, len(amendments))

	// Show version 0 (original) — the PreviousStatement of the first amendment
	fmt.Fprintf(cmd.OutOrStdout(), "[v0] Original\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n\n", amendments[0].PreviousStatement)

	// Show each amendment
	for i, a := range amendments {
		fmt.Fprintf(cmd.OutOrStdout(), "[v%d] %s by %s\n", i+1, a.Timestamp.String(), a.Owner)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", a.NewStatement)
		if i < len(amendments)-1 {
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}

	return nil
}

// amendmentJSON is the JSON representation of a single amendment version.
type amendmentJSON struct {
	Version   int    `json:"version"`
	Timestamp string `json:"timestamp,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Statement string `json:"statement"`
}

// amendmentsOutputJSON is the JSON output for the amendments command.
type amendmentsOutputJSON struct {
	NodeID          string          `json:"node_id"`
	TotalAmendments int             `json:"total_amendments"`
	Versions        []amendmentJSON `json:"versions"`
}

// renderAmendmentsJSON renders the amendment history as JSON.
func renderAmendmentsJSON(cmd *cobra.Command, nodeID service.NodeID, currentStatement string, amendments []service.Amendment) error {
	output := amendmentsOutputJSON{
		NodeID:          nodeID.String(),
		TotalAmendments: len(amendments),
	}

	if len(amendments) == 0 {
		output.Versions = []amendmentJSON{
			{Version: 0, Statement: currentStatement},
		}
	} else {
		output.Versions = make([]amendmentJSON, 0, len(amendments)+1)
		// Version 0: original
		output.Versions = append(output.Versions, amendmentJSON{
			Version:   0,
			Statement: amendments[0].PreviousStatement,
		})
		// Subsequent versions
		for i, a := range amendments {
			output.Versions = append(output.Versions, amendmentJSON{
				Version:   i + 1,
				Timestamp: a.Timestamp.String(),
				Owner:     a.Owner,
				Statement: a.NewStatement,
			})
		}
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func init() {
	rootCmd.AddCommand(newAmendmentsCmd())
}
