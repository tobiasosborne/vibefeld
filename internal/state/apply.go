// Package state provides derived state from replaying ledger events.
package state

import (
	"fmt"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Apply takes an event and updates the state accordingly.
// This is the core function for event sourcing - it replays events
// to build the current state.
//
// Taint is deliberately not recomputed here: Apply runs once per event during
// replay, so tree-wide derivation here would make replay quadratic. replayInternal
// performs one authoritative taint.RecomputeAll pass after all events are applied.
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
	case ledger.NodeAmendedReopened:
		return applyNodeAmendedReopened(s, e)
	case ledger.NodeDepsAmended:
		return applyNodeDepsAmended(s, e)
	case ledger.ScopeOpened:
		return applyScopeOpened(s, e)
	case ledger.ScopeClosed:
		return applyScopeClosed(s, e)
	case ledger.RefinementRequested:
		return applyRefinementRequested(s, e)
	case ledger.NodeSubmitted:
		return applyNodeSubmitted(s, e)
	case ledger.NodeUnvalidated:
		return applyNodeUnvalidated(s, e)
	case ledger.NodeUnadmitted:
		return applyNodeUnadmitted(s, e)
	case ledger.ApproachTried:
		return applyApproachTried(s, e)
	case ledger.EvidenceAttached:
		return applyEvidenceAttached(s, e)
	case ledger.HintAdded:
		return applyHintAdded(s, e)
	case ledger.NodeVetoed:
		return applyNodeVetoed(s, e)
	case ledger.StrategyProposed:
		return applyStrategyProposed(s, e)
	case ledger.PatternAdded:
		return applyPatternAdded(s, e)
	case ledger.OutlineSet:
		return applyOutlineSet(s, e)
	case ledger.OutlineStageLinked:
		return applyOutlineStageLinked(s, e)
	case ledger.ClaimTested:
		return applyClaimTested(s, e)
	case ledger.DefChecked:
		return applyDefChecked(s, e)
	case ledger.NodeProofAuthored:
		return applyNodeProofAuthored(s, e)
	case ledger.LockReaped:
		return nil // lock reaping is informational, no state change needed
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
	n.ValidatedBy = e.VerifiedBy
	n.ValidationBatchID = e.BatchID
	n.ValidatedContentHash = e.ContentHash
	n.ValidatedHashChecked = e.ExpectedHashChecked

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
// This adds a new challenge to the state with status ChallengeStatusOpen.
func applyChallengeRaised(s *State, e ledger.ChallengeRaised) error {
	// Default to "major" if severity not set (backward compatibility)
	severity := e.Severity
	if severity == "" {
		severity = string(schema.DefaultChallengeSeverity())
	}

	// Validate severity
	if err := schema.ValidateChallengeSeverity(severity); err != nil {
		return fmt.Errorf("invalid challenge severity: %w", err)
	}

	// Validate category (empty is allowed — category is optional)
	if err := schema.ValidateChallengeCategory(e.Category); err != nil {
		return fmt.Errorf("invalid challenge category: %w", err)
	}

	c := &Challenge{
		ID:       e.ChallengeID,
		NodeID:   e.NodeID,
		Target:   e.Target,
		Reason:   e.Reason,
		Status:   ChallengeStatusOpen,
		Severity: severity,
		Category: e.Category,
		RaisedBy: e.RaisedBy,
		BatchID:  e.BatchID,
		Created:  e.EventTime,
	}
	s.AddChallenge(c)
	return nil
}

// applyChallengeResolved handles the ChallengeResolved event.
// This updates the challenge status to ChallengeStatusResolved.
func applyChallengeResolved(s *State, e ledger.ChallengeResolved) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = ChallengeStatusResolved
	s.InvalidateChallengeCache() // status changed, cache is now stale
	return nil
}

// applyChallengeWithdrawn handles the ChallengeWithdrawn event.
// This updates the challenge status to ChallengeStatusWithdrawn.
func applyChallengeWithdrawn(s *State, e ledger.ChallengeWithdrawn) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = ChallengeStatusWithdrawn
	s.InvalidateChallengeCache() // status changed, cache is now stale
	return nil
}

// applyChallengeSuperseded handles the ChallengeSuperseded event.
// This updates the challenge status to ChallengeStatusSuperseded.
// Per PRD p.177, a challenge is superseded when its parent node is archived or refuted,
// making the challenge moot.
func applyChallengeSuperseded(s *State, e ledger.ChallengeSuperseded) error {
	c := s.GetChallenge(e.ChallengeID)
	if c == nil {
		return fmt.Errorf("challenge %s not found", e.ChallengeID)
	}
	c.Status = ChallengeStatusSuperseded
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
		if c.Status == ChallengeStatusOpen {
			c.Status = ChallengeStatusSuperseded
			modified = true
		}
	}
	if modified {
		s.InvalidateChallengeCache() // status changed, cache is now stale
	}
}

// applyNodeProofAuthored handles the NodeProofAuthored event.
// This stamps the decomposed parent node with the prover-of-record identity
// (the prover that proved it by decomposition). It records ONLY ProofAuthor —
// the node's own Author (content authorship) is deliberately never touched, so
// an existing author can never be clobbered. It always overwrites ProofAuthor
// so a re-decomposition after a later challenge records the new decomposer.
func applyNodeProofAuthored(s *State, e ledger.NodeProofAuthored) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	n.ProofAuthor = e.Author
	return nil
}

// applyNodeAmended handles the NodeAmended event.
// This updates a node's statement and records the amendment in history. It
// never changes the epistemic state (a reopened amendment is a distinct
// format-1.1 event).
func applyNodeAmended(s *State, e ledger.NodeAmended) error {
	return applyNodeStatement(s, e.NodeID, e.PreviousStatement, e.NewStatement, e.Owner, e.EventTime, false)
}

// applyNodeAmendedReopened handles the format-1.1 NodeAmendedReopened event:
// the statement change and the validated -> pending transition are one semantic
// unit, with no window in which the node is pending with the old statement or
// validated with the new one.
func applyNodeAmendedReopened(s *State, e ledger.NodeAmendedReopened) error {
	return applyNodeStatement(s, e.NodeID, e.PreviousStatement, e.NewStatement, e.Owner, e.EventTime, true)
}

// applyNodeStatement is the shared body of both statement-amendment events.
// For a reopened amendment the validated -> pending transition is validated
// before any mutation, so a failed Apply leaves the node untouched.
func applyNodeStatement(s *State, nodeID types.NodeID, previous, newStatement, owner string, ts types.Timestamp, reopened bool) error {
	n := s.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", nodeID.String())
	}
	if reopened {
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
			return fmt.Errorf("invalid reopen transition for node %s: %w", nodeID.String(), err)
		}
	}

	// Record the amendment in history
	amendment := Amendment{
		Kind:              AmendmentKindStatement,
		Timestamp:         ts,
		PreviousStatement: previous,
		NewStatement:      newStatement,
		Owner:             owner,
		Reopened:          reopened,
	}
	s.AddAmendment(nodeID, amendment)

	// Update the node's statement
	n.Statement = newStatement

	// Recompute content hash since statement changed
	n.ContentHash = n.ComputeContentHash()

	if reopened {
		if err := reopenValidated(n); err != nil {
			return err
		}
	}

	return nil
}

// applyNodeDepsAmended handles the NodeDepsAmended event (D2). Replay verifies
// the event's previous edge lists against the state it holds — a mismatch is a
// replay error, never an overwrite — then replaces both lists, recomputes the
// content hash, records a dependency amendment, and (when Reopened) performs
// validated -> pending in the same step.
//
// Every check (previous lists, previous hash, and the state/reopen contract)
// runs before any mutation, so a failed Apply leaves the state untouched. In
// particular a content-changing event on a validated node with Reopened=false
// is a replay error: the only legal way to change a validated node's edges is
// the reopened form, so a reader can never observe a validated node whose
// content changed without its verdict being invalidated.
func applyNodeDepsAmended(s *State, e ledger.NodeDepsAmended) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	if err := validateDepsAmendmentState(n, e.Reopened); err != nil {
		return err
	}
	if !sameIDSet(n.Dependencies, e.PreviousDependencies) {
		return fmt.Errorf("node %s dependency amendment mismatch: event expects previous %v, state holds %v",
			e.NodeID.String(), e.PreviousDependencies, n.Dependencies)
	}
	if !sameIDSet(n.ValidationDeps, e.PreviousValidationDeps) {
		return fmt.Errorf("node %s validation-dependency amendment mismatch: event expects previous %v, state holds %v",
			e.NodeID.String(), e.PreviousValidationDeps, n.ValidationDeps)
	}
	if e.PreviousContentHash != "" && n.ContentHash != e.PreviousContentHash {
		return fmt.Errorf("node %s content hash amendment mismatch: event expects previous %s, state holds %s",
			e.NodeID.String(), e.PreviousContentHash, n.ContentHash)
	}

	// All checks passed: apply the edge replacement and the reopen transition
	// together. reopenValidated cannot fail here because the contract was
	// validated above.
	n.Dependencies = e.NewDependencies
	n.ValidationDeps = e.NewValidationDeps
	n.ContentHash = n.ComputeContentHash()
	if e.Reopened {
		if err := reopenValidated(n); err != nil {
			return err
		}
	}

	s.AddAmendment(e.NodeID, Amendment{
		Kind:                   AmendmentKindDependencies,
		Timestamp:              e.EventTime,
		Owner:                  e.Owner,
		Reason:                 e.Reason,
		PreviousDependencies:   e.PreviousDependencies,
		NewDependencies:        e.NewDependencies,
		PreviousValidationDeps: e.PreviousValidationDeps,
		NewValidationDeps:      e.NewValidationDeps,
		PreviousContentHash:    e.PreviousContentHash,
		Reopened:               e.Reopened,
	})

	return nil
}

// validateDepsAmendmentState enforces the replay-side state contract for a
// dependency amendment. A reopened event must target a validated node; a
// non-reopened event must target a node that is still freely editable
// (pending, draft or needs_refinement). Admitted, refuted and archived nodes
// are never amendable.
func validateDepsAmendmentState(n *node.Node, reopened bool) error {
	if reopened {
		if n.EpistemicState != schema.EpistemicValidated {
			return fmt.Errorf("node %s dependency amendment reopens a node in state %s, want validated",
				n.ID.String(), n.EpistemicState)
		}
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
			return fmt.Errorf("invalid reopen transition for node %s: %w", n.ID.String(), err)
		}
		return nil
	}
	switch n.EpistemicState {
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return nil
	default:
		return fmt.Errorf("node %s dependency amendment is not reopened but state is %s; a content change on a validated node must reopen",
			n.ID.String(), n.EpistemicState)
	}
}

// reopenValidated performs the validated -> pending transition and clears the
// validation provenance, exactly as NodeUnvalidated does. Shared by the
// NodeAmended/NodesDepsAmended reopen paths so there is one definition of what
// "reopened" clears.
func reopenValidated(n *node.Node) error {
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
		return fmt.Errorf("invalid reopen transition for node %s: %w", n.ID.String(), err)
	}
	n.EpistemicState = schema.EpistemicPending
	clearValidationFields(n)
	return nil
}

// clearValidationFields clears the recorded validation provenance on a node.
// It clears the derived VerdictSeq along with the recorded verdict identity, so
// a reopened/unadmitted node cannot be mistaken for one whose verdict still
// covers its current content.
func clearValidationFields(n *node.Node) {
	n.ValidatedBy = ""
	n.ValidationBatchID = ""
	n.ValidatedContentHash = ""
	n.ValidatedHashChecked = false
	n.VerdictSeq = 0
}

// sameIDSet reports whether two ID slices contain the same IDs. Order is
// ignored because dependency order is not meaningful; the content hash already
// sorts. This makes replay tolerant of a harmless reordering while still
// rejecting a genuinely different edge set.
func sameIDSet(a, b []types.NodeID) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, id := range a {
		seen[id.String()]++
	}
	for _, id := range b {
		seen[id.String()]--
	}
	for _, c := range seen {
		if c != 0 {
			return false
		}
	}
	return true
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

// applyRefinementRequested handles the RefinementRequested event.
// This transitions a validated node to needs_refinement state,
// reopening it for further proof development by provers.
func applyRefinementRequested(s *State, e ledger.RefinementRequested) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal (only validated nodes can be refined)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicNeedsRefinement); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicNeedsRefinement
	clearValidationFields(n)
	return nil
}

// applyNodeSubmitted handles the NodeSubmitted event.
// This transitions a draft node to pending state, making it ready for
// formal verification.
func applyNodeSubmitted(s *State, e ledger.NodeSubmitted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal (only draft nodes can be submitted)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicPending

	return nil
}

// applyNodeUnvalidated handles the NodeUnvalidated event.
// This reverts a validated node back to pending for re-examination.
func applyNodeUnvalidated(s *State, e ledger.NodeUnvalidated) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal (only validated nodes can be unvalidated)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicPending
	clearValidationFields(n)

	return nil
}

// applyNodeUnadmitted handles the NodeUnadmitted event.
// This reverts an admitted node back to pending so it can be properly accepted
// (e.g., once the underlying claim has been rigorously verified).
func applyNodeUnadmitted(s *State, e ledger.NodeUnadmitted) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Validate the state transition is legal (only admitted nodes can be unadmitted)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
		return fmt.Errorf("invalid transition for node %s: %w", e.NodeID.String(), err)
	}
	n.EpistemicState = schema.EpistemicPending
	clearValidationFields(n)

	return nil
}

// applyNodeVetoed handles the NodeVetoed event.
// This is a human expert force-refute that bypasses normal state transition
// validation. Unlike applyNodeRefuted, this can override any non-terminal state
// including validated and admitted nodes.
func applyNodeVetoed(s *State, e ledger.NodeVetoed) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	// Veto can override any state except already-refuted and archived
	if n.EpistemicState == schema.EpistemicRefuted {
		return fmt.Errorf("node %s is already refuted", e.NodeID.String())
	}
	if n.EpistemicState == schema.EpistemicArchived {
		return fmt.Errorf("node %s is archived and cannot be vetoed", e.NodeID.String())
	}
	n.EpistemicState = schema.EpistemicRefuted

	// Auto-supersede any open challenges on this node
	supersedeOpenChallengesForNode(s, e.NodeID)

	return nil
}

// applyApproachTried handles the ApproachTried event.
// This records a failed proof approach for a node.
func applyApproachTried(s *State, e ledger.ApproachTried) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}

	approach := FailedApproach{
		Timestamp: e.EventTime,
		Approach:  e.Approach,
		Outcome:   e.Outcome,
		TriedBy:   e.TriedBy,
	}
	s.AddFailedApproach(e.NodeID, approach)
	return nil
}

// applyStrategyProposed handles the StrategyProposed event.
// This records a proposed proof strategy for a node.
func applyStrategyProposed(s *State, e ledger.StrategyProposed) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}

	ps := ProposedStrategy{
		Timestamp:  e.EventTime,
		Strategy:   e.Strategy,
		Novelty:    e.Novelty,
		Rationale:  e.Rationale,
		ProposedBy: e.ProposedBy,
	}
	s.AddProposedStrategy(e.NodeID, ps)
	return nil
}

// applyPatternAdded handles the PatternAdded event.
// This registers a failure pattern in the workspace.
func applyPatternAdded(s *State, e ledger.PatternAdded) error {
	p := FailurePattern{
		Timestamp:   e.EventTime,
		Name:        e.Name,
		Description: e.Description,
		Indicators:  e.Indicators,
		Remediation: e.Remediation,
		AddedBy:     e.AddedBy,
	}
	s.AddPattern(p)
	return nil
}

// applyEvidenceAttached handles the EvidenceAttached event.
// This records computational evidence linked to a node.
func applyEvidenceAttached(s *State, e ledger.EvidenceAttached) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}

	ev := Evidence{
		Timestamp:    e.EventTime,
		FilePath:     e.FilePath,
		ContentHash:  e.ContentHash,
		EvidenceType: e.EvidenceType,
		Description:  e.Description,
		AttachedBy:   e.AttachedBy,
	}
	s.AddEvidence(e.NodeID, ev)
	return nil
}

// applyHintAdded handles the HintAdded event.
// This records a domain expert hint for a node.
func applyHintAdded(s *State, e ledger.HintAdded) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}

	hint := Hint{
		Timestamp: e.EventTime,
		Text:      e.Text,
		HintBy:    e.HintBy,
	}
	s.AddHint(e.NodeID, hint)
	return nil
}

// applyOutlineSet handles the OutlineSet event.
// This replaces the entire outline with the new set of stages.
func applyOutlineSet(s *State, e ledger.OutlineSet) error {
	stages := make([]OutlineStage, len(e.Stages))
	for i, stage := range e.Stages {
		stages[i] = OutlineStage{
			Label:       stage.Label,
			Description: stage.Description,
			Criticality: stage.Criticality,
		}
	}
	s.SetOutline(stages)
	return nil
}

// applyOutlineStageLinked handles the OutlineStageLinked event.
// This links a stage label to a subtree root node.
func applyOutlineStageLinked(s *State, e ledger.OutlineStageLinked) error {
	s.LinkOutlineStage(e.Label, e.NodeID)
	return nil
}

// applyClaimTested handles the ClaimTested event.
// Records the result of a computational falsification test against a node's claim.
func applyClaimTested(s *State, e ledger.ClaimTested) error {
	n := s.GetNode(e.NodeID)
	if n == nil {
		return fmt.Errorf("node %s not found in state", e.NodeID.String())
	}
	s.AddClaimTest(e.NodeID, ClaimTestResult{
		Timestamp:   e.EventTime,
		Engine:      e.Engine,
		ScriptPath:  e.ScriptPath,
		Expression:  e.Expression,
		Passed:      e.Passed,
		Output:      e.Output,
		Agent:       e.Agent,
		ContentHash: e.ContentHash,
	})
	return nil
}

// applyDefChecked handles the DefChecked event.
// Records the result of a definition stress test.
func applyDefChecked(s *State, e ledger.DefChecked) error {
	s.AddDefCheck(e.DefName, DefCheckResult{
		Timestamp:  e.EventTime,
		DefName:    e.DefName,
		CheckType:  e.CheckType,
		ScriptPath: e.ScriptPath,
		Passed:     e.Passed,
		Output:     e.Output,
		Agent:      e.Agent,
	})
	return nil
}
