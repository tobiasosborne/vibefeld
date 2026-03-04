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

func newClaimTestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claim-tests <node-id>",
		GroupID: GroupQuery,
		Short:   "List claim test history for a node",
		Long: `Show all computational falsification tests run against a node's claim.

Displays test results chronologically with engine, pass/fail status, and output.

Examples:
  af claim-tests 1.2
  af claim-tests 1.2 -f json`,
		Args: cobra.ExactArgs(1),
		RunE: runClaimTests,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")

	return cmd
}

func runClaimTests(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af claim-tests", nodeIDStr, nil)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	if st.GetNode(nodeID) == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}

	results := st.GetClaimTests(nodeID)

	switch strings.ToLower(format) {
	case "json":
		type jsonResult struct {
			Timestamp  string `json:"timestamp"`
			Engine     string `json:"engine"`
			ScriptPath string `json:"script_path,omitempty"`
			Expression string `json:"expression,omitempty"`
			Passed     bool   `json:"passed"`
			Output     string `json:"output,omitempty"`
			Agent      string `json:"agent,omitempty"`
		}
		jsonResults := make([]jsonResult, len(results))
		for i, r := range results {
			jsonResults[i] = jsonResult{
				Timestamp:  r.Timestamp.String(),
				Engine:     r.Engine,
				ScriptPath: r.ScriptPath,
				Expression: r.Expression,
				Passed:     r.Passed,
				Output:     r.Output,
				Agent:      r.Agent,
			}
		}
		out := map[string]interface{}{
			"node_id": nodeIDStr,
			"count":   len(results),
			"tests":   jsonResults,
		}
		jsonBytes, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	default:
		if len(results) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No claim tests for node %s.\n", nodeIDStr)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Claim tests for node %s (%d total):\n\n", nodeIDStr, len(results))
		for i, r := range results {
			status := "PASS"
			if !r.Passed {
				status = "FAIL"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] engine=%s", i+1, status, r.Engine)
			if r.Agent != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " agent=%s", r.Agent)
			}
			fmt.Fprintf(cmd.OutOrStdout(), " at %s\n", r.Timestamp.String())
			if r.Output != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     Output: %s\n", strings.TrimSpace(r.Output))
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newClaimTestsCmd())
}
