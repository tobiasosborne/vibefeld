// Package render provides human-readable formatting for AF framework types.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
)

// NextStep represents a suggested next action.
type NextStep struct {
	Command     string // e.g., "af claim 1.2"
	Description string // e.g., "Claim the available node for work"
	Priority    int    // Lower is higher priority (0 = most important)
}

// Context describes the current situation for generating suggestions.
type Context struct {
	Role           string       // "prover" or "verifier"
	CurrentCommand string       // The command that was just run
	State          *state.State // Current proof state (can be nil)
}

// SuggestNextSteps generates a list of suggested next actions based on context.
// The returned steps are sorted by priority (lowest first = highest priority).
func SuggestNextSteps(ctx Context) []NextStep {
	var steps []NextStep

	// Handle nil or empty state - suggest init
	if ctx.State == nil || len(ctx.State.AllNodes()) == 0 {
		steps = append(steps, NextStep{
			Command:     "af init",
			Description: "Initialize a new proof",
			Priority:    0,
		})
		steps = append(steps, NextStep{
			Command:     "af help",
			Description: "Get help on available commands",
			Priority:    10,
		})
		return steps
	}

	// Get all nodes for analysis
	nodes := ctx.State.AllNodes()

	// Check for open challenges
	openChallenges := ctx.State.OpenChallenges()

	// Find available nodes (for claiming)
	var availableNodes []*node.Node
	// Find claimed nodes (for refining/releasing)
	var claimedNodes []*node.Node
	// Find nodes ready for verification (claimed + pending + all children validated)
	var verifierReadyNodes []*node.Node
	// Check if all nodes are validated
	allValidated := true

	for _, n := range nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			allValidated = false
		}

		if n.WorkflowState == schema.WorkflowAvailable && n.EpistemicState == schema.EpistemicPending {
			availableNodes = append(availableNodes, n)
		}

		if n.WorkflowState == schema.WorkflowClaimed && n.EpistemicState == schema.EpistemicPending {
			claimedNodes = append(claimedNodes, n)
			// Check if all children are validated
			if ctx.State.AllChildrenValidated(n.ID) {
				verifierReadyNodes = append(verifierReadyNodes, n)
			}
		}
	}

	// Sort nodes by ID for consistent ordering
	sortNodesByIDForNextSteps(availableNodes)
	sortNodesByIDForNextSteps(claimedNodes)
	sortNodesByIDForNextSteps(verifierReadyNodes)

	// Generate suggestions based on role
	switch strings.ToLower(ctx.Role) {
	case "prover":
		steps = suggestProverSteps(ctx, availableNodes, claimedNodes, openChallenges, allValidated)
	case "verifier":
		steps = suggestVerifierSteps(ctx, verifierReadyNodes, openChallenges, allValidated)
	default:
		// Unknown or empty role - provide general suggestions
		steps = suggestGeneralSteps(ctx, availableNodes, verifierReadyNodes, openChallenges, allValidated)
	}

	// Always add status as a lower-priority option
	hasStatus := false
	for _, step := range steps {
		if strings.Contains(step.Command, "status") {
			hasStatus = true
			break
		}
	}
	if !hasStatus {
		steps = append(steps, NextStep{
			Command:     "af status",
			Description: "Check proof status",
			Priority:    50,
		})
	}

	// Sort by priority
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Priority < steps[j].Priority
	})

	return steps
}

// suggestProverSteps generates suggestions for a prover.
func suggestProverSteps(ctx Context, availableNodes, claimedNodes []*node.Node, openChallenges []*state.Challenge, allValidated bool) []NextStep {
	var steps []NextStep

	// If there are open challenges, suggest viewing them
	if len(openChallenges) > 0 {
		ch := openChallenges[0]
		steps = append(steps, NextStep{
			Command:     fmt.Sprintf("af show %s", ch.ID),
			Description: "View the open challenge details",
			Priority:    0,
		})
	}

	// Based on current command, suggest next action
	switch strings.ToLower(ctx.CurrentCommand) {
	case "init":
		// After init, suggest claiming the root node
		if len(availableNodes) > 0 {
			n := availableNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af claim %s", n.ID.String()),
				Description: "Claim the node to start working on it",
				Priority:    1,
			})
		}

	case "claim":
		// After claim, suggest refining
		if len(claimedNodes) > 0 {
			n := claimedNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af refine %s", n.ID.String()),
				Description: "Refine the claimed node with proof steps",
				Priority:    1,
			})
		}

	case "refine":
		// After refine, suggest releasing
		if len(claimedNodes) > 0 {
			n := claimedNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af release %s", n.ID.String()),
				Description: "Release the node for verification",
				Priority:    1,
			})
		}

	default:
		// General prover suggestions
		// Suggest claiming available nodes
		if len(availableNodes) > 0 {
			n := availableNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af claim %s", n.ID.String()),
				Description: "Claim an available node to work on",
				Priority:    1,
			})
		}

		// Suggest refining claimed nodes
		if len(claimedNodes) > 0 {
			n := claimedNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af refine %s", n.ID.String()),
				Description: "Continue refining the claimed node",
				Priority:    2,
			})
		}
	}

	// If all validated, suggest status check
	if allValidated {
		steps = append(steps, NextStep{
			Command:     "af status",
			Description: "View the completed proof status",
			Priority:    0,
		})
	}

	return steps
}

// suggestVerifierSteps generates suggestions for a verifier.
func suggestVerifierSteps(ctx Context, verifierReadyNodes []*node.Node, openChallenges []*state.Challenge, allValidated bool) []NextStep {
	var steps []NextStep

	// Based on current command, suggest next action
	switch strings.ToLower(ctx.CurrentCommand) {
	case "challenge":
		// After challenge, suggest status check
		steps = append(steps, NextStep{
			Command:     "af status",
			Description: "Check the proof status after challenge",
			Priority:    0,
		})

	default:
		// General verifier suggestions
		// Suggest accepting nodes ready for verification
		if len(verifierReadyNodes) > 0 {
			n := verifierReadyNodes[0]
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af accept %s", n.ID.String()),
				Description: "Accept the node after verification",
				Priority:    0,
			})
			steps = append(steps, NextStep{
				Command:     fmt.Sprintf("af challenge %s", n.ID.String()),
				Description: "Raise a challenge if issues are found",
				Priority:    1,
			})
		}
	}

	// If there are open challenges to check
	if len(openChallenges) > 0 {
		ch := openChallenges[0]
		steps = append(steps, NextStep{
			Command:     fmt.Sprintf("af show %s", ch.ID),
			Description: "View the open challenge details",
			Priority:    5,
		})
	}

	// If all validated
	if allValidated {
		steps = append(steps, NextStep{
			Command:     "af status",
			Description: "View the completed proof",
			Priority:    0,
		})
	}

	return steps
}

// suggestGeneralSteps generates suggestions when role is unknown.
func suggestGeneralSteps(ctx Context, availableNodes []*node.Node, verifierReadyNodes []*node.Node, openChallenges []*state.Challenge, allValidated bool) []NextStep {
	var steps []NextStep

	// Suggest claiming available nodes
	if len(availableNodes) > 0 {
		n := availableNodes[0]
		steps = append(steps, NextStep{
			Command:     fmt.Sprintf("af claim %s", n.ID.String()),
			Description: "Claim an available node to work on",
			Priority:    1,
		})
	}

	// Suggest accepting/challenging ready nodes
	if len(verifierReadyNodes) > 0 {
		n := verifierReadyNodes[0]
		steps = append(steps, NextStep{
			Command:     fmt.Sprintf("af accept %s", n.ID.String()),
			Description: "Accept the node after verification",
			Priority:    2,
		})
	}

	// If there are open challenges
	if len(openChallenges) > 0 {
		ch := openChallenges[0]
		steps = append(steps, NextStep{
			Command:     fmt.Sprintf("af show %s", ch.ID),
			Description: "View the open challenge",
			Priority:    3,
		})
	}

	// Always suggest help
	steps = append(steps, NextStep{
		Command:     "af help",
		Description: "Get help on available commands",
		Priority:    100,
	})

	return steps
}

// RenderNextSteps formats suggestions for display.
// Returns an empty string if steps is nil or empty.
func RenderNextSteps(steps []NextStep) string {
	if len(steps) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("Next steps:\n")

	// Find the maximum command length for alignment
	maxCmdLen := 0
	for _, step := range steps {
		if len(step.Command) > maxCmdLen {
			maxCmdLen = len(step.Command)
		}
	}

	// Render each step with alignment
	for _, step := range steps {
		// Pad command for alignment
		paddedCmd := step.Command
		for len(paddedCmd) < maxCmdLen {
			paddedCmd += " "
		}

		sb.WriteString(fmt.Sprintf("  -> %s    %s\n", paddedCmd, step.Description))
	}

	return sb.String()
}

// sortNodesByIDForNextSteps sorts nodes by their ID string for consistent ordering.
func sortNodesByIDForNextSteps(nodes []*node.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID.String() < nodes[j].ID.String()
	})
}
