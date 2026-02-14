// Package main contains the af evidence command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

func newEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "evidence <node-id>",
		GroupID: GroupQuery,
		Short:   "List computational evidence attached to a node",
		Long: `Show all computational evidence (scripts, datasets, verification results)
attached to a proof node.

Examples:
  af evidence 1.2
  af evidence 1.2 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runEvidence,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runEvidence(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af evidence")
		return render.InvalidNodeIDError("af evidence", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s not found", nodeID.String())
	}

	evidence := st.GetEvidence(nodeID)
	return outputEvidenceList(cmd, nodeID, evidence, format)
}

func outputEvidenceList(cmd *cobra.Command, nodeID service.NodeID, evidence []state.Evidence, format string) error {
	switch strings.ToLower(format) {
	case "json":
		type jsonEvidence struct {
			FilePath     string `json:"file_path"`
			ContentHash  string `json:"content_hash"`
			EvidenceType string `json:"type,omitempty"`
			Description  string `json:"description,omitempty"`
			AttachedBy   string `json:"attached_by,omitempty"`
			Timestamp    string `json:"timestamp"`
		}
		items := make([]jsonEvidence, len(evidence))
		for i, ev := range evidence {
			items[i] = jsonEvidence{
				FilePath:     ev.FilePath,
				ContentHash:  ev.ContentHash,
				EvidenceType: ev.EvidenceType,
				Description:  ev.Description,
				AttachedBy:   ev.AttachedBy,
				Timestamp:    ev.Timestamp.String(),
			}
		}
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"count":    len(evidence),
			"evidence": items,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		if len(evidence) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No evidence attached to node %s\n", nodeID.String())
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Evidence for node %s (%d total):\n\n", nodeID.String(), len(evidence))
		for i, ev := range evidence {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s [%s]\n", i+1, ev.FilePath, ev.EvidenceType)
			if ev.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Description: %s\n", ev.Description)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     Hash: %s\n", ev.ContentHash)
			if ev.AttachedBy != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Attached by: %s\n", ev.AttachedBy)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "     When: %s\n", ev.Timestamp.String())
			if i < len(evidence)-1 {
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newEvidenceCmd())
}
