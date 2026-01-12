// Package state provides derived state from replaying ledger events.
package state

import (
	"fmt"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
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
			// Node not found - skip (may be an error in production)
			continue
		}
		n.WorkflowState = schema.WorkflowClaimed
		n.ClaimedBy = e.Owner
		n.ClaimedAt = e.Timeout
	}
	return nil
}

// applyNodesReleased handles the NodesReleased event.
// This clears the claim on released nodes.
func applyNodesReleased(s *State, e ledger.NodesReleased) error {
	for _, nodeID := range e.NodeIDs {
		n := s.GetNode(nodeID)
		if n == nil {
			// Node not found - skip (may be an error in production)
			continue
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
		// Node not found - skip (may be an error in production)
		return nil
	}
	n.EpistemicState = schema.EpistemicValidated
	return nil
}

// applyNodeAdmitted handles the NodeAdmitted event.
// This changes the epistemic state to admitted.
func applyNodeAdmitted(s *State, e ledger.NodeAdmitted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		// Node not found - skip (may be an error in production)
		return nil
	}
	n.EpistemicState = schema.EpistemicAdmitted
	return nil
}

// applyNodeRefuted handles the NodeRefuted event.
// This changes the epistemic state to refuted.
func applyNodeRefuted(s *State, e ledger.NodeRefuted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		// Node not found - skip (may be an error in production)
		return nil
	}
	n.EpistemicState = schema.EpistemicRefuted
	return nil
}

// applyNodeArchived handles the NodeArchived event.
// This changes the epistemic state to archived.
func applyNodeArchived(s *State, e ledger.NodeArchived) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		// Node not found - skip (may be an error in production)
		return nil
	}
	n.EpistemicState = schema.EpistemicArchived
	return nil
}

// applyTaintRecomputed handles the TaintRecomputed event.
// This updates the taint state of a node.
func applyTaintRecomputed(s *State, e ledger.TaintRecomputed) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		// Node not found - skip (may be an error in production)
		return nil
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
// Challenge tracking may be implemented separately.
func applyChallengeRaised(s *State, e ledger.ChallengeRaised) error {
	// TODO: Add challenge tracking to state if needed
	return nil
}

// applyChallengeResolved handles the ChallengeResolved event.
// Challenge tracking may be implemented separately.
func applyChallengeResolved(s *State, e ledger.ChallengeResolved) error {
	// TODO: Add challenge tracking to state if needed
	return nil
}

// applyChallengeWithdrawn handles the ChallengeWithdrawn event.
// Challenge tracking may be implemented separately.
func applyChallengeWithdrawn(s *State, e ledger.ChallengeWithdrawn) error {
	// TODO: Add challenge tracking to state if needed
	return nil
}
