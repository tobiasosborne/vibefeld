// Package render provides human-readable and JSON output formatting.
package render

import (
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
)

// RenderJobs renders the list of available jobs (prover and verifier).
// Shows job details including node ID, reason, and instructions.
// Jobs are sorted by ID for consistent output.
func RenderJobs(jobList *jobs.JobResult) string {
	// Handle nil job list
	if jobList == nil {
		return ""
	}

	// Handle empty job list
	if jobList.IsEmpty() {
		return "No jobs available.\n\nProver jobs: 0 nodes awaiting refinement\nVerifier jobs: 0 nodes ready for review"
	}

	var sb strings.Builder

	// Sort prover jobs by ID for consistent output
	proverJobs := make([]*node.Node, len(jobList.ProverJobs))
	copy(proverJobs, jobList.ProverJobs)
	sortNodesByID(proverJobs)

	// Sort verifier jobs by ID for consistent output
	verifierJobs := make([]*node.Node, len(jobList.VerifierJobs))
	copy(verifierJobs, jobList.VerifierJobs)
	sortNodesByID(verifierJobs)

	// Render prover jobs section
	if len(proverJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Prover Jobs (%d available) ===\n", len(proverJobs)))
		sb.WriteString("Nodes awaiting refinement. Claim one and refine the proof.\n\n")
		for _, n := range proverJobs {
			renderJobNode(&sb, n)
		}
		sb.WriteString("\nNext: Run 'af claim <id>' to claim a prover job, then 'af refine <id>' to work on it.\n")
	}

	// Add separator between sections if both have jobs
	if len(proverJobs) > 0 && len(verifierJobs) > 0 {
		sb.WriteString("\n")
	}

	// Render verifier jobs section
	if len(verifierJobs) > 0 {
		sb.WriteString(fmt.Sprintf("=== Verifier Jobs (%d available) ===\n", len(verifierJobs)))
		sb.WriteString("Nodes ready for review. Verify or challenge the proof.\n\n")
		for _, n := range verifierJobs {
			renderJobNode(&sb, n)
		}
		sb.WriteString("\nNext: Run 'af accept <id>' to validate or 'af challenge <id>' to raise objections.\n")
	}

	return sb.String()
}

// renderJobNode renders a single job node entry.
// Note: Statements are NOT truncated because mathematical proofs require precision.
// Agents need the full statement text to work with.
func renderJobNode(sb *strings.Builder, n *node.Node) {
	// Sanitize statement (remove control chars, normalize whitespace) but do NOT truncate
	stmt := sanitizeStatement(n.Statement)

	// Render node line with ID, type, and full statement
	sb.WriteString(fmt.Sprintf("  [%s] %s: %q\n", n.ID.String(), string(n.Type), stmt))

	// Show claimed-by info for verifier jobs
	if n.ClaimedBy != "" {
		sb.WriteString(fmt.Sprintf("         claimed by: %s\n", n.ClaimedBy))
	}
}
