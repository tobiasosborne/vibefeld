package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// newClaimCmd creates the claim command for claiming a node for work.
func newClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim <node-id>",
		Short: "Claim a job for work",
		Long: `Claim a proof node to work on as a prover or verifier.

Before working on a node, agents must claim it to prevent concurrent
modifications. Claims have a timeout to automatically release abandoned work.

Examples:
  af claim 1 --owner prover-001 --role prover
  af claim 1.2 --owner verifier-alpha --timeout 30m --role verifier
  af claim 1 -o prover-001 -r prover -t 2h --format json`,
		Args: cobra.ExactArgs(1),
		RunE: runClaim,
	}

	// Add flags
	cmd.Flags().StringP("owner", "o", "", "Owner identity for the claim (required)")
	cmd.Flags().StringP("timeout", "t", "1h", "Claim timeout duration (e.g., 30m, 1h, 2h30m)")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory")
	cmd.Flags().StringP("format", "f", "text", "Output format: text or json")
	cmd.Flags().StringP("role", "r", "prover", "Agent role: prover or verifier")

	// Mark owner as required
	cmd.MarkFlagRequired("owner")

	return cmd
}

// runClaim executes the claim command.
func runClaim(cmd *cobra.Command, args []string) error {
	// Parse node ID from positional argument
	nodeID, err := types.Parse(args[0])
	if err != nil {
		return fmt.Errorf("invalid node ID %q: %w", args[0], err)
	}

	// Get flags
	owner, _ := cmd.Flags().GetString("owner")
	timeoutStr, _ := cmd.Flags().GetString("timeout")
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	role, _ := cmd.Flags().GetString("role")

	// Validate owner is not empty or whitespace
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("owner cannot be empty")
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", timeoutStr, err)
	}

	// Validate timeout is positive
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", timeout)
	}

	// Validate role
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "prover" && role != "verifier" {
		return fmt.Errorf("invalid role %q: must be 'prover' or 'verifier'", role)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof directory: %w", err)
	}

	// Claim the node
	err = svc.ClaimNode(nodeID, owner, timeout)
	if err != nil {
		return fmt.Errorf("failed to claim node %s: %w", nodeID.String(), err)
	}

	// Load state for context rendering
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state for context: %w", err)
	}

	// Output result based on format
	if format == "json" {
		return outputClaimJSON(cmd, nodeID, owner, role, timeout, st)
	}

	return outputClaimText(cmd, nodeID, owner, timeout, role, st)
}

// outputClaimJSON outputs the claim result in JSON format.
func outputClaimJSON(cmd *cobra.Command, nodeID types.NodeID, owner, role string, timeout time.Duration, st *state.State) error {
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

	result := map[string]interface{}{
		"node_id":    nodeID.String(),
		"owner":      owner,
		"role":       role,
		"status":     "claimed",
		"context":    context,
		"timeout":    timeout.String(),
		"expires_at": expiresAt.Format(time.RFC3339),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	cmd.Println(string(data))
	return nil
}

// outputClaimText outputs the claim result in human-readable text format.
func outputClaimText(cmd *cobra.Command, nodeID types.NodeID, owner string, timeout time.Duration, role string, st *state.State) error {
	// Calculate expiration time
	expiresAt := time.Now().Add(timeout)

	cmd.Printf("Claimed node %s\n", nodeID.String())
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
