package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newJobsCmd creates the jobs command.
func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List available jobs",
		Long: `List available prover and verifier jobs in the proof.

Verifier jobs are nodes ready for review (breadth-first model):
  - Has a statement
  - EpistemicState = "pending" (not yet verified)
  - WorkflowState = "available" (not claimed or blocked)
  - Has NO open/unresolved challenges

Prover jobs are nodes with challenges that need addressing:
  - EpistemicState = "pending"
  - Has one or more open challenges

Every new node is immediately a verifier job. When a verifier raises a
challenge, the node becomes a prover job. When challenges are resolved,
the node returns to verifier territory for final acceptance.

Examples:
  af jobs                     List all available jobs
  af jobs --role prover       List only prover jobs
  af jobs --role verifier     List only verifier jobs
  af jobs --format json       Output in JSON format`,
		RunE: runJobs,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("role", "r", "", "Filter by role (prover or verifier)")

	return cmd
}

// runJobs executes the jobs command.
func runJobs(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	role, _ := cmd.Flags().GetString("role")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Validate role if provided (check if flag was explicitly set)
	roleSet := cmd.Flags().Changed("role")
	role = strings.ToLower(role)
	if roleSet && role != "prover" && role != "verifier" {
		return fmt.Errorf("invalid role %q: must be 'prover' or 'verifier'", role)
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load current state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all nodes and build node map
	nodes := st.AllNodes()
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}

	// Build challenge map from state challenges
	// Maps node ID string -> slice of challenges on that node
	challengeMap := make(map[string][]*node.Challenge)
	for _, c := range st.AllChallenges() {
		nodeIDStr := c.NodeID.String()
		// Convert state.Challenge to node.Challenge (only Status field is needed by jobs package)
		nc := &node.Challenge{
			ID:       c.ID,
			TargetID: c.NodeID,
			Status:   node.ChallengeStatus(c.Status),
		}
		challengeMap[nodeIDStr] = append(challengeMap[nodeIDStr], nc)
	}

	// Find jobs
	jobResult := jobs.FindJobs(nodes, nodeMap, challengeMap)

	// Apply role filter if specified
	if roleSet && role == "prover" {
		jobResult = &jobs.JobResult{
			ProverJobs:   jobResult.ProverJobs,
			VerifierJobs: nil,
		}
	} else if roleSet && role == "verifier" {
		jobResult = &jobs.JobResult{
			ProverJobs:   nil,
			VerifierJobs: jobResult.VerifierJobs,
		}
	}

	// Output based on format
	if format == "json" {
		output := render.RenderJobsJSON(jobResult)
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	// Text format
	output := render.RenderJobs(jobResult)
	fmt.Fprint(cmd.OutOrStdout(), output)

	// Add summary line showing both job type counts
	proverCount := len(jobResult.ProverJobs)
	verifierCount := len(jobResult.VerifierJobs)
	fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %d prover job(s), %d verifier job(s)\n", proverCount, verifierCount)

	return nil
}

func init() {
	rootCmd.AddCommand(newJobsCmd())
}
