// Package render provides human-readable and JSON output formatting.
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
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

// JSONJobParent represents parent node info in JSON job output.
type JSONJobParent struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

// JSONJobDefinition represents a referenced definition in JSON job output.
type JSONJobDefinition struct {
	ID         string `json:"id"`
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// JSONJobExternal represents a referenced external in JSON job output.
type JSONJobExternal struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
}

// JSONJobChallenge represents an open challenge in JSON job output.
type JSONJobChallenge struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

// JSONJobEntryFull represents a single job entry with full context in JSON format.
type JSONJobEntryFull struct {
	ID          string              `json:"id"`
	Statement   string              `json:"statement"`
	Type        string              `json:"type"`
	Depth       int                 `json:"depth"`
	Parent      *JSONJobParent      `json:"parent,omitempty"`
	Definitions []JSONJobDefinition `json:"definitions,omitempty"`
	Externals   []JSONJobExternal   `json:"externals,omitempty"`
	Challenges  []JSONJobChallenge  `json:"challenges,omitempty"`
}

// JSONJobListFull represents a list of available jobs with full context in JSON format.
type JSONJobListFull struct {
	ProverJobs   []JSONJobEntryFull `json:"prover_jobs"`
	VerifierJobs []JSONJobEntryFull `json:"verifier_jobs"`
}

// JobsContext provides the additional context needed for rendering jobs with full details.
type JobsContext struct {
	State        *state.State
	NodeMap      map[string]*node.Node
	ChallengeMap map[string][]*node.Challenge
}

// RenderJobsJSONWithContext renders available jobs as JSON with full context.
// This includes parent info, referenced definitions, externals, and open challenges.
// Returns JSON string representation of the jobs.
// Returns empty JSON structure for nil job result.
func RenderJobsJSONWithContext(jobList *jobs.JobResult, ctx *JobsContext) string {
	if jobList == nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	jl := JSONJobListFull{
		ProverJobs:   make([]JSONJobEntryFull, 0, len(jobList.ProverJobs)),
		VerifierJobs: make([]JSONJobEntryFull, 0, len(jobList.VerifierJobs)),
	}

	for _, job := range jobList.ProverJobs {
		entry := buildJobEntryFull(job, ctx)
		jl.ProverJobs = append(jl.ProverJobs, entry)
	}

	for _, job := range jobList.VerifierJobs {
		entry := buildJobEntryFull(job, ctx)
		jl.VerifierJobs = append(jl.VerifierJobs, entry)
	}

	data, err := json.Marshal(jl)
	if err != nil {
		return `{"prover_jobs":[],"verifier_jobs":[]}`
	}

	return string(data)
}

// buildJobEntryFull builds a full job entry with context from state.
func buildJobEntryFull(job *node.Node, ctx *JobsContext) JSONJobEntryFull {
	entry := JSONJobEntryFull{
		ID:        job.ID.String(),
		Statement: job.Statement,
		Type:      string(job.Type),
		Depth:     job.Depth(),
	}

	if ctx == nil {
		return entry
	}

	// Add parent info if exists
	if parentID, hasParent := job.ID.Parent(); hasParent {
		if ctx.State != nil {
			if parent := ctx.State.GetNode(parentID); parent != nil {
				entry.Parent = &JSONJobParent{
					ID:        parent.ID.String(),
					Statement: parent.Statement,
				}
			}
		} else if ctx.NodeMap != nil {
			if parent := ctx.NodeMap[parentID.String()]; parent != nil {
				entry.Parent = &JSONJobParent{
					ID:        parent.ID.String(),
					Statement: parent.Statement,
				}
			}
		}
	}

	// Add definitions referenced by this node's context
	if ctx.State != nil && len(job.Context) > 0 {
		defs := make([]JSONJobDefinition, 0)
		for _, ctxRef := range job.Context {
			// Try to find definition by ID or name
			if def := ctx.State.GetDefinition(ctxRef); def != nil {
				defs = append(defs, JSONJobDefinition{
					ID:         def.ID,
					Term:       def.Name,
					Definition: def.Content,
				})
			} else if def := ctx.State.GetDefinitionByName(ctxRef); def != nil {
				defs = append(defs, JSONJobDefinition{
					ID:         def.ID,
					Term:       def.Name,
					Definition: def.Content,
				})
			}
		}
		if len(defs) > 0 {
			entry.Definitions = defs
		}
	}

	// Add externals referenced by this node's context
	if ctx.State != nil && len(job.Context) > 0 {
		exts := make([]JSONJobExternal, 0)
		for _, ctxRef := range job.Context {
			if ext := ctx.State.GetExternal(ctxRef); ext != nil {
				exts = append(exts, JSONJobExternal{
					ID:        ext.ID,
					Reference: ext.Source,
				})
			}
		}
		if len(exts) > 0 {
			entry.Externals = exts
		}
	}

	// Add open challenges on this node
	if ctx.ChallengeMap != nil {
		nodeIDStr := job.ID.String()
		if challenges := ctx.ChallengeMap[nodeIDStr]; len(challenges) > 0 {
			chs := make([]JSONJobChallenge, 0)
			for _, ch := range challenges {
				// Only include open challenges
				if ch.Status == node.ChallengeStatusOpen {
					chs = append(chs, JSONJobChallenge{
						ID:       ch.ID,
						Target:   string(ch.Target),
						Reason:   ch.Reason,
						Severity: ch.Severity,
					})
				}
			}
			if len(chs) > 0 {
				entry.Challenges = chs
			}
		}
	}

	return entry
}
