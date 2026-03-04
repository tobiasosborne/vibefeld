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

func newClaimTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claim-test <node-id>",
		GroupID: GroupVerifier,
		Short:   "Run computational falsification test on a node's claim",
		Long: `Run a computational falsification test against a proof node's claim.

Tests can use external scripts or inline sympy expressions to check whether
algebraic identities, inequalities, or other mathematical claims actually hold.

Crux nodes (created with --crux flag) cannot be validated without a passing test.

Examples:
  af claim-test 1.2 --script ./check_identity.py
  af claim-test 1.2 --engine sympy --expression "simplify(x**2 - x**2) == 0"
  af claim-test 1.2 --script ./verify.sh --agent verifier-1

Script convention:
  Exit code 0 = test passed (claim not falsified)
  Exit code non-zero = test failed (claim falsified or error)
  stdout/stderr captured for audit trail`,
		Args: cobra.ExactArgs(1),
		RunE: runClaimTest,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text|json)")
	cmd.Flags().String("script", "", "Path to test script")
	cmd.Flags().String("engine", "script", "Test engine (script|sympy)")
	cmd.Flags().String("expression", "", "Sympy expression to evaluate (requires --engine sympy)")
	cmd.Flags().String("agent", "", "Agent ID running the test")

	return cmd
}

func runClaimTest(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	script := cli.MustString(cmd, "script")
	engine := cli.MustString(cmd, "engine")
	expression := cli.MustString(cmd, "expression")
	agent := cli.MustString(cmd, "agent")

	examples := []string{
		"af claim-test 1.2 --script ./check.sh",
		"af claim-test 1.2 --engine sympy --expression \"simplify(x**2 - x**2) == 0\"",
	}

	nodeIDStr := args[0]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af claim-test", nodeIDStr, examples)
	}

	// Validate inputs
	if engine == "sympy" {
		if strings.TrimSpace(expression) == "" {
			return render.MissingFlagError("af claim-test", "expression (required with --engine sympy)", examples)
		}
	} else {
		if strings.TrimSpace(script) == "" {
			return render.MissingFlagError("af claim-test", "script", examples)
		}
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	passed, output, err := svc.RunClaimTest(nodeID, engine, script, expression, agent)
	if err != nil {
		return fmt.Errorf("claim-test failed: %w", err)
	}

	return outputClaimTestResult(cmd, nodeIDStr, engine, passed, output, format)
}

func outputClaimTestResult(cmd *cobra.Command, nodeID, engine string, passed bool, output, format string) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id": nodeID,
			"engine":  engine,
			"passed":  passed,
		}
		if output != "" {
			result["output"] = output
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	default:
		status := "PASSED"
		if !passed {
			status = "FAILED"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Claim test %s for node %s (engine: %s)\n", status, nodeID, engine)
		if output != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nOutput:\n%s\n", output)
		}
		if passed {
			fmt.Fprintln(cmd.OutOrStdout(), "\nClaim not falsified. Node may proceed to validation.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\nClaim falsified! This node should be revised or refuted.")
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newClaimTestCmd())
}
