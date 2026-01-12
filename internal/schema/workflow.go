package schema

import "fmt"

// WorkflowState represents the workflow state of a node.
type WorkflowState string

const (
	// WorkflowAvailable indicates the node is free for any agent to claim.
	WorkflowAvailable WorkflowState = "available"
	// WorkflowClaimed indicates the node is currently owned by an agent.
	WorkflowClaimed WorkflowState = "claimed"
	// WorkflowBlocked indicates the node cannot be worked on (e.g., awaiting dependency).
	WorkflowBlocked WorkflowState = "blocked"
)

// WorkflowStateInfo contains metadata about a workflow state.
type WorkflowStateInfo struct {
	ID          WorkflowState
	Description string
}

var workflowStateRegistry = map[WorkflowState]WorkflowStateInfo{
	WorkflowAvailable: {
		ID:          WorkflowAvailable,
		Description: "Node is free for any agent to claim",
	},
	WorkflowClaimed: {
		ID:          WorkflowClaimed,
		Description: "Node is currently owned by an agent",
	},
	WorkflowBlocked: {
		ID:          WorkflowBlocked,
		Description: "Node cannot be worked on (e.g., awaiting dependency)",
	},
}

// workflowTransitions defines allowed state transitions.
// Key: from state, Value: set of allowed to states.
var workflowTransitions = map[WorkflowState]map[WorkflowState]bool{
	WorkflowAvailable: {
		WorkflowClaimed: true,
	},
	WorkflowClaimed: {
		WorkflowAvailable: true,
		WorkflowBlocked:   true,
	},
	WorkflowBlocked: {
		WorkflowAvailable: true,
	},
}

// ValidateWorkflowState validates that a string represents a valid workflow state.
func ValidateWorkflowState(s string) error {
	state := WorkflowState(s)
	if _, ok := workflowStateRegistry[state]; !ok {
		return fmt.Errorf("invalid workflow state: %q, must be one of: %s, %s, %s",
			s, WorkflowAvailable, WorkflowClaimed, WorkflowBlocked)
	}
	return nil
}

// GetWorkflowStateInfo returns metadata about a workflow state.
// Returns false if the state does not exist.
func GetWorkflowStateInfo(s WorkflowState) (WorkflowStateInfo, bool) {
	info, ok := workflowStateRegistry[s]
	return info, ok
}

// AllWorkflowStates returns information about all workflow states.
func AllWorkflowStates() []WorkflowStateInfo {
	return []WorkflowStateInfo{
		workflowStateRegistry[WorkflowAvailable],
		workflowStateRegistry[WorkflowClaimed],
		workflowStateRegistry[WorkflowBlocked],
	}
}

// ValidateWorkflowTransition checks if a transition from one workflow state to another is allowed.
//
// Valid transitions:
// - available → claimed (agent claims the node)
// - claimed → available (agent releases the node)
// - claimed → blocked (node blocked due to dependency)
// - blocked → available (blocker resolved)
//
// Invalid transitions:
// - available → blocked (must be claimed first)
// - blocked → claimed (must become available first)
// - same state → same state (no-op not allowed)
func ValidateWorkflowTransition(from, to WorkflowState) error {
	// Validate both states exist
	if err := ValidateWorkflowState(string(from)); err != nil {
		return err
	}
	if err := ValidateWorkflowState(string(to)); err != nil {
		return err
	}

	// Same state transition not allowed
	if from == to {
		return fmt.Errorf("invalid transition from %q to %q: cannot transition to same state", from, to)
	}

	// Check if transition is allowed
	allowedTargets, ok := workflowTransitions[from]
	if !ok || !allowedTargets[to] {
		return fmt.Errorf("invalid transition from %q to %q: transition not allowed", from, to)
	}

	return nil
}

// CanClaim returns true if the given workflow state allows claiming.
// Only available nodes can be claimed.
func CanClaim(s WorkflowState) bool {
	_, ok := workflowStateRegistry[s]
	if !ok {
		return false
	}
	return s == WorkflowAvailable
}
