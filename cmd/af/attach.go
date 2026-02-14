// Package main contains the af attach command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attach <node-id> <file-path>",
		GroupID: GroupProver,
		Short:   "Attach computational evidence to a node",
		Long: `Link a script, dataset, or verification result to a proof node.

The file's SHA256 content hash is recorded in the ledger for reproducibility.
The file path is stored relative to the proof directory.

Examples:
  af attach 1.2 scripts/verify.py --type verification
  af attach 1.2 results/output.txt --type computation --description "Monte Carlo 100K trials"
  af attach 1.2 tests/check.jl --type test --agent prover-001 -f json`,
		Args: cobra.ExactArgs(2),
		RunE: runAttach,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("type", "t", "other", "Evidence type (verification|computation|test|other)")
	cmd.Flags().String("description", "", "Description of the evidence")
	cmd.Flags().String("agent", "", "Agent ID (who attached it)")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runAttach(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	evidenceType := cli.MustString(cmd, "type")
	description := cli.MustString(cmd, "description")
	agent := cli.MustString(cmd, "agent")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	filePath := args[1]

	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		examples := render.GetExamples("af attach")
		return render.InvalidNodeIDError("af attach", nodeIDStr, examples)
	}

	// Validate evidence type
	validTypes := map[string]bool{"verification": true, "computation": true, "test": true, "other": true}
	if !validTypes[strings.ToLower(evidenceType)] {
		return fmt.Errorf("invalid evidence type %q: must be verification, computation, test, or other", evidenceType)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	if err := svc.AttachEvidence(nodeID, filePath, evidenceType, description, agent); err != nil {
		return fmt.Errorf("error attaching evidence: %w", err)
	}

	return outputAttachResult(cmd, nodeID, filePath, evidenceType, description, format)
}

func outputAttachResult(cmd *cobra.Command, nodeID service.NodeID, filePath, evidenceType, description, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":   nodeID.String(),
			"file_path": filePath,
			"type":      evidenceType,
		}
		if description != "" {
			result["description"] = description
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Evidence attached to node %s\n", nodeID.String())
		fmt.Fprintf(cmd.OutOrStdout(), "  File: %s\n", filePath)
		fmt.Fprintf(cmd.OutOrStdout(), "  Type: %s\n", evidenceType)
		if description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", description)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newAttachCmd())
}
