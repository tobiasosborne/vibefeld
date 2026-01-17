// Package state provides derived state from replaying ledger events.
package state

import (
	"fmt"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// Apply takes an event and updates the state accordingly.
// This is the core function for event sourcing - it replays events
// to build the current state.
//
// Returns an error if:
// - The state or event is nil
// - The event type is unknown
// - The event cannot be applied (e.g., node not found for state change)
func Apply(s *State, event ledger.Event) error {
	if s == nil {
		return fmt.Errorf("cannot apply event to nil state")
	}
	if event == nil {
		return fmt.Errorf("cannot apply nil event")
	}

	switch e := event.(type) {
	case ledger.ProofInitialized:
		return applyProofInitialized(s, e)
	case ledger.NodeCreated:
		return applyNodeCreated(s, e)
	case ledger.NodesClaimed:
		return applyNodesClaimed(s, e)
	case ledger.ClaimRefreshed:
		return applyClaimRefreshed(s, e)
	case ledger.NodesReleased:
		return applyNodesReleased(s, e)
	case ledger.NodeValidated:
		return applyNodeValidated(s, e)
	case ledger.NodeAdmitted:
		return applyNodeAdmitted(s, e)
	case ledger.NodeRefuted:
		return applyNodeRefuted(s, e)
	case ledger.NodeArchived:
		return applyNodeArchived(s, e)
	case ledger.TaintRecomputed:
		return applyTaintRecomputed(s, e)
	case ledger.DefAdded:
		return applyDefAdded(s, e)
	case ledger.LemmaExtracted:
		return applyLemmaExtracted(s, e)
	case ledger.ChallengeRaised:
		return applyChallengeRaised(s, e)
	case ledger.ChallengeResolved:
		return applyChallengeResolved(s, e)
	case ledger.ChallengeWithdrawn:
		return applyChallengeWithdrawn(s, e)
	case ledger.ChallengeSuperseded:
		return applyChallengeSuperseded(s, e)
	case ledger.NodeAmended:
		return applyNodeAmended(s, e)
	case ledger.ScopeOpened:
		return applyScopeOpened(s, e)
	case ledger.ScopeClosed:
		return applyScopeClosed(s, e)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type())
	}
}

// applyProofInitialized handles the ProofInitialized event.
// This event sets up the initial proof state.
func applyProofInitialized(s *State, e ledger.ProofInitialized) error {
	// ProofInitialized is accepted - the conjecture and author are stored
	// in the event stream, not necessarily in state
	return nil
}

// applyNodeCreated handles the NodeCreated event.
// This adds a new node to the state.
func applyNodeCreated(s *State, e ledger.NodeCreated) error {
	n := e.Node
	s.AddNode(&n)
	return nil
}

// applyNodesClaimed handles the NodesClaimed event.
// This updates the workflow state of claimed nodes.
func applyNodesClaimed(s *State, e ledger.NodesClaimed) error {
	for _, nodeID := range e.NodeIDs {
		n := s.GetNode(nodeID)
		if n == nil {
			return fmt.Errorf("node %s not found in state", nodeID.String())
		}
		// Validate the workflow state transition
		if err := schema.ValidateWorkflowTransition(n.WorkflowState, schema.WorkflowClaimed); err != nil {
			return fmt.Errorf("invalid workflow transition for node %s: %w", nodeID.String(), err)
		}
		n.WorkflowState = schema.WorkflowClaimed
		n.ClaimedBy = e.Owner
		n.ClaimedAt = e.Timeout
	}
	return nil
}

// applyClaimRefreshed handles the ClaimRefreshed event.
// This updates the claim timeout without changing workflow state.
func applyClaimRefreshed(s *State, e ledger.ClaimRefreshed) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Verify the node is still claimed by the same owner
	if n.WorkflowState != schema.WorkflowClaimed {
		return fmt.Errorf("node %s is not claimed", e.NodeID.String())
	}
	if n.ClaimedBy != e.Owner {
		return fmt.Errorf("node %s is claimed by %s, not %s", e.NodeID.String(), n.ClaimedBy, e.Owner)
	}
	// Update the timeout
	n.ClaimedAt = e.NewTimeout
	return nil
}

// applyNodesReleased handles the NodesReleased event.
// This clears the claim on released nodes.
func applyNodesReleased(s *State, e ledger.NodesReleased) error {
	for _, nodeID := range e.NodeIDs {
		n := s.GetNode(nodeID)
		if n == nil {
			return fmt.Errorf("node %s not found in state", nodeID.String())
		}
		// Validate the workflow state transition
		if err := schema.ValidateWorkflowTransition(n.WorkflowState, schema.WorkflowAvailable); err != nil {
			return fmt.Errorf("invalid workflow transition for node %s: %w", nodeID.String(), err)
		}
		n.WorkflowState = schema.WorkflowAvailable
		n.ClaimedBy = ""
		n.ClaimedAt = types.Timestamp{}
	}
	return nil
}

// applyNodeValidated handles the NodeValidated event.
// This changes the epistemic state to validated.
func applyNodeValidated(s *State, e ledger.NodeValidated) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicValidated

	// Auto-trigger taint recomputation after epistemic state change
	recomputeTaintForNode(s, n)

	return nil
}

// applyNodeAdmitted handles the NodeAdmitted event.
// This changes the epistemic state to admitted.
func applyNodeAdmitted(s *State, e ledger.NodeAdmitted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicAdmitted); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicAdmitted

	// Auto-trigger taint recomputation after epistemic state change
	recomputeTaintForNode(s, n)

	return nil
}

// applyNodeRefuted handles the NodeRefuted event.
// This changes the epistemic state to refuted.
// Per PRD p.177, refuting a node auto-supersedes any open challenges on it.
func applyNodeRefuted(s *State, e ledger.NodeRefuted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicRefuted); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicRefuted

	// Auto-supersede any open challenges on this node
	supersedeOpenChallengesForNode(s, e.NodeID)

	// Auto-trigger taint recomputation after epistemic state change
	recomputeTaintForNode(s, n)

	return nil
}

// applyNodeArchived handles the NodeArchived event.
// This changes the epistemic state to archived.
// Per PRD p.177, archiving a node auto-supersedes any open challenges on it.
func applyNodeArchived(s *State, e ledger.NodeArchived) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicArchived); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicArchived

	// Auto-supersede any open challenges on this node
	supersedeOpenChallengesForNode(s, e.NodeID)

	// Auto-trigger taint recomputation after epistemic state change
	recomputeTaintForNode(s, n)

	return nil
}

// applyTaintRecomputed handles the TaintRecomputed event.
// This updates the taint state of a node.
func applyTaintRecomputed(s *State, e ledger.TaintRecomputed) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	n.TaintState = e.NewTaint
	return nil
}

// applyDefAdded handles the DefAdded event.
// This adds a new definition to the state.
func applyDefAdded(s *State, e ledger.DefAdded) error {
	def := &node.Definition{
		ID:      e.Definition.ID,
		Name:    e.Definition.Name,
		Content: e.Definition.Definition,
		Created: e.Definition.Created,
	}
	s.AddDefinition(def)
	return nil
}

// applyLemmaExtracted handles the LemmaExtracted event.
// This adds a new lemma to the state.
func applyLemmaExtracted(s *State, e ledger.LemmaExtracted) error {
	lemma := &node.Lemma{
		ID:           e.Lemma.ID,
		Statement:    e.Lemma.Statement,
		SourceNodeID: e.Lemma.NodeID,
		Created:      e.Lemma.Created,
	}
	s.AddLemma(lemma)
	return nil
}

// applyChallengeRaised handles the ChallengeRaised event.
// This adds a new challenge to the state with status "open".
func applyChallengeRaised(s *State, e ledger.ChallengeRaised) error {
	// Default to "major" if severity not set (backward compatibility)
	severity := e.Severity
	if severity == "" {
		severity = "major"
	}
	c := &Challenge{
		ID:       e.ChallengeID,
		NodeID:   e.NodeID,
		Target:   e.Target,
		Reason:   e.Reason,
		Status:   "open",
		Severity: severity,
		RaisedBy: e.RaisedBy,
		Created:  e.EventTime,
	}
	s.AddChallenge(c)
	return nil
}

// applyChallengeResolved handles the ChallengeResolved event.
// This updates the challenge status to "resolved".
func applyChallengeResolved(s *State, e ledger.ChallengeResolved) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = "resolved"
	s.InvalidateChallengeCache() // status changed, cache is now stale
	return nil
}

// applyChallengeWithdrawn handles the ChallengeWithdrawn event.
// This updates the challenge status to "withdrawn".
func applyChallengeWithdrawn(s *State, e ledger.ChallengeWithdrawn) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = "withdrawn"
	s.InvalidateChallengeCache() // status changed, cache is now stale
	return nil
}

// applyChallengeSuperseded handles the ChallengeSuperseded event.
// This updates the challenge status to "superseded".
// Per PRD p.177, a challenge is superseded when its parent node is archived or refuted,
// making the challenge moot.
func applyChallengeSuperseded(s *State, e ledger.ChallengeSuperseded) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = "superseded"
	s.InvalidateChallengeCache() // status changed, cache is now stale
	return nil
}

// supersedeOpenChallengesForNode marks all open challenges for a specific node
// as superseded. This is called when a node is archived or refuted, making
// any challenges on it moot.
func supersedeOpenChallengesForNode(s *State, nodeID types.NodeID) {
	// Use the cached challengesByNode map for O(1) lookup
	challenges := s.GetChallengesForNode(nodeID)
	modified := false
	for _, c := range challenges {
		if c.Status == "open" {
			c.Status = "superseded"
			modified = true
		}
	}
	if modified {
		s.InvalidateChallengeCache() // status changed, cache is now stale
	}
}

// recomputeTaintForNode recomputes the taint state for a node and propagates
// taint changes to its descendants.
//
// This function is called automatically after epistemic state changes to ensure
// the taint system stays in sync with the proof state.
func recomputeTaintForNode(s *State, n *node.Node) {
	if n == nil || s == nil {
		return
	}

	// Get all nodes in the state for taint computation
	allNodes := s.AllNodes()

	// Build ancestor list for this node
	nodeMap := make(map[string]*node.Node)
	for _, nd := range allNodes {
		if nd != nil {
			nodeMap[nd.ID.String()] = nd
		}
	}

	var ancestors []*node.Node
	parentID, hasParent := n.ID.Parent()
	for hasParent {
		if parent, ok := nodeMap[parentID.String()]; ok {
			ancestors = append(ancestors, parent)
		}
		parentID, hasParent = parentID.Parent()
	}

	// Compute the taint for the node itself
	newTaint := taint.ComputeTaint(n, ancestors)
	n.TaintState = newTaint

	// Propagate taint to descendants
	taint.PropagateTaint(n, allNodes)
}

// applyNodeAmended handles the NodeAmended event.
// This updates a node's statement and records the amendment in history.
func applyNodeAmended(s *State, e ledger.NodeAmended) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}

	// Record the amendment in history
	amendment := Amendment{
		Timestamp:         e.EventTime,
		PreviousStatement: e.PreviousStatement,
		NewStatement:      e.NewStatement,
		Owner:             e.Owner,
	}
	s.AddAmendment(e.NodeID, amendment)

	// Update the node's statement
	n.Statement = e.NewStatement

	// Recompute content hash since statement changed
	n.ContentHash = n.ComputeContentHash()

	return nil
}

// applyScopeOpened handles the ScopeOpened event.
// This opens a new assumption scope at the given node.
func applyScopeOpened(s *State, e ledger.ScopeOpened) error {
	return s.OpenScope(e.NodeID, e.Statement)
}

// applyScopeClosed handles the ScopeClosed event.
// This closes the assumption scope at the given node.
func applyScopeClosed(s *State, e ledger.ScopeClosed) error {
	return s.CloseScope(e.NodeID)
}
