package schema

import "errors"

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

// ValidateWorkflowState validates that a string represents a valid workflow state.
func ValidateWorkflowState(s string) error {
	return errors.New("not implemented")
}

// GetWorkflowStateInfo returns metadata about a workflow state.
// Returns false if the state does not exist.
func GetWorkflowStateInfo(s WorkflowState) (WorkflowStateInfo, bool) {
	return WorkflowStateInfo{}, false
}

// AllWorkflowStates returns information about all workflow states.
func AllWorkflowStates() []WorkflowStateInfo {
	return nil
}

// ValidateWorkflowTransition checks if a transition from one workflow state to another is allowed.
func ValidateWorkflowTransition(from, to WorkflowState) error {
	return errors.New("not implemented")
}

// CanClaim returns true if the given workflow state allows claiming.
func CanClaim(s WorkflowState) bool {
	return false
}
