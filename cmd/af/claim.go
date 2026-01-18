package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// newClaimCmd creates the claim command for claiming a node for work.
func newClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claim <node-id>",
		GroupID: GroupWorkflow,
		Short:   "Claim a job for work",
		Long: `Claim a proof node to work on as a prover or verifier.

Before working on a node, agents must claim it to prevent concurrent
modifications. Claims have a timeout to automatically release abandoned work.

Use --refresh to extend the claim timeout on a node you already own,
without releasing and reclaiming (which would risk another agent claiming it).

Examples:
  af claim 1 --owner prover-001 --role prover
  af claim 1.2 --owner verifier-alpha --timeout 30m --role verifier
  af claim 1 -o prover-001 -r prover -t 2h --format json
  af claim 1 --owner prover-001 --refresh --timeout 2h

Workflow:
  After claiming a node as a prover, use 'af refine' to develop the proof.
  As a verifier, use 'af challenge' to raise objections or 'af accept' to validate.
  Use 'af release' if you cannot complete the work.`,
		Args: cobra.ExactArgs(1),
		RunE: runClaim,
	}

	// Add flags
	cmd.Flags().StringP("owner", "o", "", "Owner identity for the claim (required)")
	cmd.Flags().StringP("timeout", "t", config.DefaultClaimTimeout, "Claim timeout duration (e.g., 30m, 1h, 2h30m)")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory")
	cmd.Flags().StringP("format", "f", "text", "Output format: text or json")
	cmd.Flags().StringP("role", "r", "prover", "Agent role: prover or verifier")
	cmd.Flags().Bool("refresh", false, "Refresh an existing claim (extend timeout without releasing)")

	// Mark owner as required
	cmd.MarkFlagRequired("owner")

	return cmd
}

// runClaim executes the claim command.
func runClaim(cmd *cobra.Command, args []string) error {
	examples := render.GetExamples("af claim")

	// Parse node ID from positional argument
	nodeID, err := types.Parse(args[0])
	if err != nil {
		return render.InvalidNodeIDError("af claim", args[0], examples)
	}

	// Get flags
	owner := cli.MustString(cmd, "owner")
	timeoutStr := cli.MustString(cmd, "timeout")
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")
	role := cli.MustString(cmd, "role")
	refresh := cli.MustBool(cmd, "refresh")

	// Validate owner is not empty or whitespace
	if strings.TrimSpace(owner) == "" {
		return render.EmptyValueError("af claim", "owner", examples)
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return render.InvalidDurationError("af claim", "timeout", timeoutStr, examples)
	}

	// Validate timeout is positive
	if timeout <= 0 {
		return render.NewUsageError("af claim", fmt.Sprintf("--timeout must be positive, got %v", timeout), examples)
	}

	// Validate role
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "prover" && role != "verifier" {
		return render.InvalidValueError("af claim", "role", role, render.ValidRoles, examples)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof directory: %w", err)
	}

	// Either refresh an existing claim or claim a new node
	if refresh {
		err = svc.RefreshClaim(nodeID, owner, timeout)
		if err != nil {
			return fmt.Errorf("failed to refresh claim on node %s: %w", nodeID.String(), err)
		}
	} else {
		err = svc.ClaimNode(nodeID, owner, timeout)
		if err != nil {
			return fmt.Errorf("failed to claim node %s: %w", nodeID.String(), err)
		}
	}

	// Load state for context rendering
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state for context: %w", err)
	}

	// Output result based on format
	if format == "json" {
		return outputClaimJSON(cmd, nodeID, owner, role, timeout, st, refresh)
	}

	return outputClaimText(cmd, nodeID, owner, timeout, role, st, refresh)
}

// outputClaimJSON outputs the claim result in JSON format.
func outputClaimJSON(cmd *cobra.Command, nodeID types.NodeID, owner, role string, timeout time.Duration, st *state.State, refresh bool) error {
	// Render context based on role
	var context string
	if role == "prover" {
		context = render.RenderProverContext(st, nodeID)
	}
	// Note: verifier context requires a Challenge, which we don't have here
	// For verifier role claiming a node, we still show prover-style context
	// since they're claiming to examine the node
	if role == "verifier" {
		context = render.RenderProverContext(st, nodeID)
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(timeout)

	// Set status based on whether this was a refresh or new claim
	status := "claimed"
	if refresh {
		status = "refreshed"
	}

	result := map[string]interface{}{
		"node_id":    nodeID.String(),
		"owner":      owner,
		"role":       role,
		"status":     status,
		"context":    context,
		"timeout":    timeout.String(),
		"expires_at": expiresAt.Format(time.RFC3339),
	}

	// Add verification checklist for verifier role
	if role == "verifier" {
		targetNode := st.GetNode(nodeID)
		if targetNode != nil {
			checklistJSON := render.RenderVerificationChecklistJSON(targetNode, st)
			// Parse the checklist JSON and add it to the result
			// RenderVerificationChecklistJSON always returns valid JSON, but we handle
			// parse errors defensively in case the contract changes
			var checklist interface{}
			if err := json.Unmarshal([]byte(checklistJSON), &checklist); err != nil {
				result["verification_checklist_error"] = fmt.Sprintf("failed to parse checklist: %v", err)
			} else {
				result["verification_checklist"] = checklist
			}
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}

// outputClaimText outputs the claim result in human-readable text format.
func outputClaimText(cmd *cobra.Command, nodeID types.NodeID, owner string, timeout time.Duration, role string, st *state.State, refresh bool) error {
	// Calculate expiration time
	expiresAt := time.Now().Add(timeout)

	// Print action based on whether this was a refresh or new claim
	if refresh {
		cmd.Printf("Refreshed claim on node %s\n", nodeID.String())
	} else {
		cmd.Printf("Claimed node %s\n", nodeID.String())
	}
	cmd.Printf("  Owner:      %s\n", owner)
	cmd.Printf("  Role:       %s\n", role)
	cmd.Printf("  Timeout:    %s\n", timeout)
	cmd.Printf("  Expires at: %s\n", expiresAt.Format("15:04:05"))
	cmd.Println()

	// Render and display context based on role
	// Both prover and verifier roles use prover context when claiming a node
	// (verifier context is specifically for examining challenges)
	context := render.RenderProverContext(st, nodeID)
	if context != "" {
		cmd.Println(context)
	}

	// Display verification checklist for verifier role
	if role == "verifier" {
		targetNode := st.GetNode(nodeID)
		if targetNode != nil {
			checklist := render.RenderVerificationChecklist(targetNode, st)
			if checklist != "" {
				cmd.Println(checklist)
			}
		}
	}

	cmd.Println("Next steps:")
	if role == "prover" {
		cmd.Println("  af refine  - Add or modify proof content for this node")
	} else {
		cmd.Println("  af challenge - Raise a challenge on a claim")
		cmd.Println("  af accept    - Accept a valid claim")
	}
	cmd.Println("  af release - Release the claim if you cannot complete the work")

	return nil
}

func init() {
	rootCmd.AddCommand(newClaimCmd())
}
