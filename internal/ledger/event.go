// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

import (
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/types"
)

// EventType identifies the type of ledger event.
type EventType string

const (
	EventProofInitialized    EventType = "proof_initialized"
	EventNodeCreated         EventType = "node_created"
	EventNodesClaimed        EventType = "nodes_claimed"
	EventNodesReleased       EventType = "nodes_released"
	EventChallengeRaised     EventType = "challenge_raised"
	EventChallengeResolved   EventType = "challenge_resolved"
	EventChallengeWithdrawn  EventType = "challenge_withdrawn"
	EventChallengeSuperseded EventType = "challenge_superseded"
	EventNodeValidated       EventType = "node_validated"
	EventNodeAdmitted        EventType = "node_admitted"
	EventNodeRefuted         EventType = "node_refuted"
	EventNodeArchived        EventType = "node_archived"
	EventNodeAmended         EventType = "node_amended"
	EventTaintRecomputed     EventType = "taint_recomputed"
	EventDefAdded            EventType = "def_added"
	EventLemmaExtracted      EventType = "lemma_extracted"
	EventLockReaped          EventType = "lock_reaped"
	EventScopeOpened         EventType = "scope_opened"
	EventScopeClosed         EventType = "scope_closed"
	EventClaimRefreshed      EventType = "claim_refreshed"
	EventRefinementRequested EventType = "refinement_requested"
	EventNodeSubmitted       EventType = "node_submitted"
	EventNodeUnvalidated     EventType = "node_unvalidated"
	EventNodeUnadmitted      EventType = "node_unadmitted"
	EventApproachTried       EventType = "approach_tried"
	EventEvidenceAttached    EventType = "evidence_attached"
	EventOutlineSet          EventType = "outline_set"
	EventOutlineStageLinked  EventType = "outline_stage_linked"
	EventHintAdded           EventType = "hint_added"
	EventNodeVetoed          EventType = "node_vetoed"
	EventStrategyProposed    EventType = "strategy_proposed"
	EventPatternAdded        EventType = "pattern_added"
	EventClaimTested         EventType = "claim_tested"
	EventDefChecked          EventType = "def_checked"
)

// Event is the base interface for all ledger events.
type Event interface {
	// Type returns the event type identifier.
	Type() EventType

	// Timestamp returns when the event occurred.
	Timestamp() types.Timestamp
}

// BaseEvent contains common fields for all events.
type BaseEvent struct {
	EventType EventType       `json:"type"`
	EventTime types.Timestamp `json:"timestamp"`
}

// Type returns the event type identifier.
func (e BaseEvent) Type() EventType {
	return e.EventType
}

// Timestamp returns when the event occurred.
func (e BaseEvent) Timestamp() types.Timestamp {
	return e.EventTime
}

// ProofInitialized is emitted when a new proof is created.
type ProofInitialized struct {
	BaseEvent
	Conjecture string `json:"conjecture"`
	Author     string `json:"author"`
}

// NodeCreated is emitted when a new node is added to the proof tree.
type NodeCreated struct {
	BaseEvent
	Node node.Node `json:"node"`
}

// NodesClaimed is emitted when one or more nodes are claimed by an agent.
type NodesClaimed struct {
	BaseEvent
	NodeIDs []types.NodeID  `json:"node_ids"`
	Owner   string          `json:"owner"`
	Timeout types.Timestamp `json:"timeout"`
}

// NodesReleased is emitted when one or more nodes are released from a claim.
type NodesReleased struct {
	BaseEvent
	NodeIDs []types.NodeID `json:"node_ids"`
}

// ChallengeRaised is emitted when a verifier raises a challenge against a node.
//
// BatchID mirrors NodeValidated.BatchID: it marks this challenge as raised
// as part of a batch verification run (`af verdicts apply`, rk item V2, not
// yet implemented here), so a batch's mixed accept/challenge verdict list
// can be traced back to one batch id regardless of which event kind each
// item produced. RaisedBy (verifier identity) already existed before this
// change; both fields are the same driver-supplied-provenance kind as
// NodeValidated's — recorded and checkable, not adversary-proof.
type ChallengeRaised struct {
	BaseEvent
	ChallengeID string       `json:"challenge_id"`
	NodeID      types.NodeID `json:"node_id"`
	Target      string       `json:"target"`
	Reason      string       `json:"reason"`
	Severity    string       `json:"severity"`           // "critical", "major", "minor", or "note"
	RaisedBy    string       `json:"raised_by"`          // Agent ID of the verifier who raised this challenge
	Category    string       `json:"category,omitempty"` // Optional typed classification (gap, missing, dependency, ...)
	BatchID     string       `json:"batch_id,omitempty"` // Batch identifier, if raised as part of a batch (af verdicts apply)
}

// ChallengeResolved is emitted when a challenge is resolved (answered).
type ChallengeResolved struct {
	BaseEvent
	ChallengeID string `json:"challenge_id"`
}

// ChallengeWithdrawn is emitted when a verifier withdraws a challenge.
type ChallengeWithdrawn struct {
	BaseEvent
	ChallengeID string `json:"challenge_id"`
}

// ChallengeSuperseded is emitted when a challenge becomes moot because its parent
// node was archived or refuted. Per PRD p.177, this marks the challenge as superseded.
type ChallengeSuperseded struct {
	BaseEvent
	ChallengeID string       `json:"challenge_id"`
	NodeID      types.NodeID `json:"node_id"`
}

// NodeValidated is emitted when a verifier validates a node as correct.
//
// VerifiedBy and BatchID are rk PRD C3's kernel-side groundwork for batched
// verification: VerifiedBy is the verifier's identity, and BatchID (if
// non-empty) marks this validation as applied as part of a batch by `af
// verdicts apply` (rk item V2, not yet implemented here). Both are
// DRIVER-SUPPLIED PROVENANCE — recorded and mechanically checkable (e.g. a
// reviewer≠author check against node.Author), never adversary-proof
// enforcement. Both are optional and omitted for events that don't supply
// them, so old ledgers (and any code that doesn't care about batching)
// replay identically.
type NodeValidated struct {
	BaseEvent
	NodeID     types.NodeID `json:"node_id"`
	Note       string       `json:"note,omitempty"`        // Optional acceptance note (partial acceptance)
	VerifiedBy string       `json:"verified_by,omitempty"` // Agent ID of the verifier who validated this node
	BatchID    string       `json:"batch_id,omitempty"`    // Batch identifier, if validated as part of a batch (af verdicts apply)
}

// NodeAdmitted is emitted when a verifier admits a node without full verification.
type NodeAdmitted struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
}

// NodeRefuted is emitted when a verifier refutes a node as incorrect.
type NodeRefuted struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
}

// NodeArchived is emitted when a node is archived (branch abandoned).
type NodeArchived struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
}

// TaintRecomputed is emitted when a node's taint state is recalculated.
type TaintRecomputed struct {
	BaseEvent
	NodeID   types.NodeID    `json:"node_id"`
	NewTaint node.TaintState `json:"new_taint"`
}

// Definition represents a definition added to the proof.
type Definition struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Definition string          `json:"definition"`
	Created    types.Timestamp `json:"created"`
}

// DefAdded is emitted when a definition is added.
type DefAdded struct {
	BaseEvent
	Definition Definition `json:"definition"`
}

// Lemma represents an extracted lemma.
type Lemma struct {
	ID        string          `json:"id"`
	Statement string          `json:"statement"`
	NodeID    types.NodeID    `json:"node_id"`
	Created   types.Timestamp `json:"created"`
}

// LemmaExtracted is emitted when a lemma is extracted from the proof.
type LemmaExtracted struct {
	BaseEvent
	Lemma Lemma `json:"lemma"`
}

// LockReaped is emitted when a stale lock is cleaned up.
type LockReaped struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
	Owner  string       `json:"owner"`
}

// NodeAmended is emitted when a prover corrects the statement of a node they own.
// The original statement is preserved in the PreviousStatement field for history.
type NodeAmended struct {
	BaseEvent
	NodeID            types.NodeID `json:"node_id"`
	PreviousStatement string       `json:"previous_statement"`
	NewStatement      string       `json:"new_statement"`
	Owner             string       `json:"owner"`
}

// NewProofInitialized creates a ProofInitialized event.
func NewProofInitialized(conjecture, author string) ProofInitialized {
	return ProofInitialized{
		BaseEvent: BaseEvent{
			EventType: EventProofInitialized,
			EventTime: types.Now(),
		},
		Conjecture: conjecture,
		Author:     author,
	}
}

// NewNodeCreated creates a NodeCreated event.
func NewNodeCreated(n node.Node) NodeCreated {
	return NodeCreated{
		BaseEvent: BaseEvent{
			EventType: EventNodeCreated,
			EventTime: types.Now(),
		},
		Node: n,
	}
}

// NewNodesClaimed creates a NodesClaimed event.
func NewNodesClaimed(nodeIDs []types.NodeID, owner string, timeout types.Timestamp) NodesClaimed {
	return NodesClaimed{
		BaseEvent: BaseEvent{
			EventType: EventNodesClaimed,
			EventTime: types.Now(),
		},
		NodeIDs: nodeIDs,
		Owner:   owner,
		Timeout: timeout,
	}
}

// NewNodesReleased creates a NodesReleased event.
func NewNodesReleased(nodeIDs []types.NodeID) NodesReleased {
	return NodesReleased{
		BaseEvent: BaseEvent{
			EventType: EventNodesReleased,
			EventTime: types.Now(),
		},
		NodeIDs: nodeIDs,
	}
}

// NewChallengeRaised creates a ChallengeRaised event with default severity (major) and empty RaisedBy.
func NewChallengeRaised(challengeID string, nodeID types.NodeID, target, reason string) ChallengeRaised {
	return NewChallengeRaisedWithSeverity(challengeID, nodeID, target, reason, "major", "")
}

// NewChallengeRaisedWithSeverity creates a ChallengeRaised event with the specified severity and raisedBy agent ID.
func NewChallengeRaisedWithSeverity(challengeID string, nodeID types.NodeID, target, reason, severity, raisedBy string) ChallengeRaised {
	return NewChallengeRaisedFull(challengeID, nodeID, target, reason, severity, raisedBy, "")
}

// NewChallengeRaisedFull creates a ChallengeRaised event with severity, raisedBy
// agent ID, and an optional typed category. An empty category means uncategorised.
// BatchID is empty (not part of a batch); use NewChallengeRaisedWithBatch for
// challenges raised as part of a batch verification run.
func NewChallengeRaisedFull(challengeID string, nodeID types.NodeID, target, reason, severity, raisedBy, category string) ChallengeRaised {
	return NewChallengeRaisedWithBatch(challengeID, nodeID, target, reason, severity, raisedBy, category, "")
}

// NewChallengeRaisedWithBatch creates a ChallengeRaised event with severity,
// raisedBy agent ID, optional typed category, and an optional batch id. The
// batch id marks this challenge as raised as part of a batch verification
// run (`af verdicts apply`, rk item V2); pass "" for singly-raised
// challenges, which is exactly what NewChallengeRaisedFull does.
func NewChallengeRaisedWithBatch(challengeID string, nodeID types.NodeID, target, reason, severity, raisedBy, category, batchID string) ChallengeRaised {
	return ChallengeRaised{
		BaseEvent: BaseEvent{
			EventType: EventChallengeRaised,
			EventTime: types.Now(),
		},
		ChallengeID: challengeID,
		NodeID:      nodeID,
		Target:      target,
		Reason:      reason,
		Severity:    severity,
		RaisedBy:    raisedBy,
		Category:    category,
		BatchID:     batchID,
	}
}

// NewChallengeResolved creates a ChallengeResolved event.
func NewChallengeResolved(challengeID string) ChallengeResolved {
	return ChallengeResolved{
		BaseEvent: BaseEvent{
			EventType: EventChallengeResolved,
			EventTime: types.Now(),
		},
		ChallengeID: challengeID,
	}
}

// NewChallengeWithdrawn creates a ChallengeWithdrawn event.
func NewChallengeWithdrawn(challengeID string) ChallengeWithdrawn {
	return ChallengeWithdrawn{
		BaseEvent: BaseEvent{
			EventType: EventChallengeWithdrawn,
			EventTime: types.Now(),
		},
		ChallengeID: challengeID,
	}
}

// NewChallengeSuperseded creates a ChallengeSuperseded event.
// This is used when a challenge becomes moot because its parent node was archived or refuted.
func NewChallengeSuperseded(challengeID string, nodeID types.NodeID) ChallengeSuperseded {
	return ChallengeSuperseded{
		BaseEvent: BaseEvent{
			EventType: EventChallengeSuperseded,
			EventTime: types.Now(),
		},
		ChallengeID: challengeID,
		NodeID:      nodeID,
	}
}

// NewNodeValidated creates a NodeValidated event.
func NewNodeValidated(nodeID types.NodeID) NodeValidated {
	return NewNodeValidatedWithNote(nodeID, "")
}

// NewNodeValidatedWithNote creates a NodeValidated event with an optional acceptance note.
// The note is used for partial acceptance where the verifier accepts the node
// but wants to record a minor issue or clarification. VerifiedBy and BatchID
// are empty; use NewNodeValidatedFull to record verifier identity and/or a
// batch id.
func NewNodeValidatedWithNote(nodeID types.NodeID, note string) NodeValidated {
	return NewNodeValidatedFull(nodeID, note, "", "")
}

// NewNodeValidatedFull creates a NodeValidated event with an optional
// acceptance note, verifier identity, and batch id. verifiedBy and batchID
// are DRIVER-SUPPLIED PROVENANCE (see NodeValidated doc comment) — pass ""
// for either when not applicable (e.g. a manual, non-batched `af accept`
// with no --agent given).
func NewNodeValidatedFull(nodeID types.NodeID, note, verifiedBy, batchID string) NodeValidated {
	return NodeValidated{
		BaseEvent: BaseEvent{
			EventType: EventNodeValidated,
			EventTime: types.Now(),
		},
		NodeID:     nodeID,
		Note:       note,
		VerifiedBy: verifiedBy,
		BatchID:    batchID,
	}
}

// NewNodeAdmitted creates a NodeAdmitted event.
func NewNodeAdmitted(nodeID types.NodeID) NodeAdmitted {
	return NodeAdmitted{
		BaseEvent: BaseEvent{
			EventType: EventNodeAdmitted,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
	}
}

// NewNodeRefuted creates a NodeRefuted event.
func NewNodeRefuted(nodeID types.NodeID) NodeRefuted {
	return NodeRefuted{
		BaseEvent: BaseEvent{
			EventType: EventNodeRefuted,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
	}
}

// NewNodeArchived creates a NodeArchived event.
func NewNodeArchived(nodeID types.NodeID) NodeArchived {
	return NodeArchived{
		BaseEvent: BaseEvent{
			EventType: EventNodeArchived,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
	}
}

// NewTaintRecomputed creates a TaintRecomputed event.
func NewTaintRecomputed(nodeID types.NodeID, newTaint node.TaintState) TaintRecomputed {
	return TaintRecomputed{
		BaseEvent: BaseEvent{
			EventType: EventTaintRecomputed,
			EventTime: types.Now(),
		},
		NodeID:   nodeID,
		NewTaint: newTaint,
	}
}

// NewDefAdded creates a DefAdded event.
func NewDefAdded(def Definition) DefAdded {
	return DefAdded{
		BaseEvent: BaseEvent{
			EventType: EventDefAdded,
			EventTime: types.Now(),
		},
		Definition: def,
	}
}

// NewLemmaExtracted creates a LemmaExtracted event.
func NewLemmaExtracted(lemma Lemma) LemmaExtracted {
	return LemmaExtracted{
		BaseEvent: BaseEvent{
			EventType: EventLemmaExtracted,
			EventTime: types.Now(),
		},
		Lemma: lemma,
	}
}

// NewLockReaped creates a LockReaped event.
// Note: Uses FromTime to preserve full timestamp precision for accurate
// comparison with caller's timing windows.
func NewLockReaped(nodeID types.NodeID, owner string) LockReaped {
	return LockReaped{
		BaseEvent: BaseEvent{
			EventType: EventLockReaped,
			EventTime: types.FromTime(time.Now().UTC()),
		},
		NodeID: nodeID,
		Owner:  owner,
	}
}

// NewNodeAmended creates a NodeAmended event.
func NewNodeAmended(nodeID types.NodeID, previousStatement, newStatement, owner string) NodeAmended {
	return NodeAmended{
		BaseEvent: BaseEvent{
			EventType: EventNodeAmended,
			EventTime: types.Now(),
		},
		NodeID:            nodeID,
		PreviousStatement: previousStatement,
		NewStatement:      newStatement,
		Owner:             owner,
	}
}

// ScopeOpened is emitted when a local_assume node opens a new assumption scope.
// All descendant nodes of the assumption node are considered "inside" the scope
// until the scope is closed.
type ScopeOpened struct {
	BaseEvent
	NodeID    types.NodeID `json:"node_id"`   // The local_assume node that opens the scope
	Statement string       `json:"statement"` // The assumption statement
}

// ScopeClosed is emitted when an assumption scope is discharged (closed).
// This occurs when a contradiction is derived or the assumption is otherwise discharged.
type ScopeClosed struct {
	BaseEvent
	NodeID          types.NodeID `json:"node_id"`           // The local_assume node whose scope is being closed
	DischargeNodeID types.NodeID `json:"discharge_node_id"` // The node that discharged the scope
}

// NewScopeOpened creates a ScopeOpened event.
func NewScopeOpened(nodeID types.NodeID, statement string) ScopeOpened {
	return ScopeOpened{
		BaseEvent: BaseEvent{
			EventType: EventScopeOpened,
			EventTime: types.Now(),
		},
		NodeID:    nodeID,
		Statement: statement,
	}
}

// NewScopeClosed creates a ScopeClosed event.
func NewScopeClosed(nodeID types.NodeID, dischargeNodeID types.NodeID) ScopeClosed {
	return ScopeClosed{
		BaseEvent: BaseEvent{
			EventType: EventScopeClosed,
			EventTime: types.Now(),
		},
		NodeID:          nodeID,
		DischargeNodeID: dischargeNodeID,
	}
}

// ClaimRefreshed is emitted when an agent refreshes their claim on a node,
// extending the claim timeout without releasing and reclaiming.
type ClaimRefreshed struct {
	BaseEvent
	NodeID     types.NodeID    `json:"node_id"`
	Owner      string          `json:"owner"`
	NewTimeout types.Timestamp `json:"new_timeout"`
}

// NewClaimRefreshed creates a ClaimRefreshed event.
func NewClaimRefreshed(nodeID types.NodeID, owner string, newTimeout types.Timestamp) ClaimRefreshed {
	return ClaimRefreshed{
		BaseEvent: BaseEvent{
			EventType: EventClaimRefreshed,
			EventTime: types.Now(),
		},
		NodeID:     nodeID,
		Owner:      owner,
		NewTimeout: newTimeout,
	}
}

// RefinementRequested is emitted when a verifier requests refinement on a validated node.
// This reopens the node for further proof development by provers.
type RefinementRequested struct {
	BaseEvent
	NodeID      types.NodeID `json:"node_id"`
	Reason      string       `json:"reason"`
	RequestedBy string       `json:"requested_by,omitempty"` // Agent ID of the requester
}

// NewRefinementRequested creates a RefinementRequested event.
func NewRefinementRequested(nodeID types.NodeID, reason, requestedBy string) RefinementRequested {
	return RefinementRequested{
		BaseEvent: BaseEvent{
			EventType: EventRefinementRequested,
			EventTime: types.Now(),
		},
		NodeID:      nodeID,
		Reason:      reason,
		RequestedBy: requestedBy,
	}
}

// NodeSubmitted is emitted when a prover submits a draft node for formal verification.
// This transitions the node from draft to pending state.
type NodeSubmitted struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
	Owner  string       `json:"owner"`
}

// NewNodeSubmitted creates a NodeSubmitted event.
func NewNodeSubmitted(nodeID types.NodeID, owner string) NodeSubmitted {
	return NodeSubmitted{
		BaseEvent: BaseEvent{
			EventType: EventNodeSubmitted,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
		Owner:  owner,
	}
}

// NodeUnvalidated is emitted when a verifier revokes validation on a node,
// reverting it from validated back to pending for re-examination.
type NodeUnvalidated struct {
	BaseEvent
	NodeID    types.NodeID `json:"node_id"`
	Reason    string       `json:"reason,omitempty"`
	RevokedBy string       `json:"revoked_by,omitempty"`
}

// NodeUnadmitted is emitted when a verifier revokes an admission on a node,
// reverting it from admitted back to pending. Admit is a temporary, taint-
// introducing escape hatch — once the underlying claim has been rigorously
// verified, unadmit clears the admission so the node can be properly accepted.
type NodeUnadmitted struct {
	BaseEvent
	NodeID    types.NodeID `json:"node_id"`
	Reason    string       `json:"reason,omitempty"`
	RevokedBy string       `json:"revoked_by,omitempty"`
}

// ApproachTried is emitted when a prover records a failed proof approach,
// preventing other agents from re-attempting the same dead end.
type ApproachTried struct {
	BaseEvent
	NodeID   types.NodeID `json:"node_id"`
	Approach string       `json:"approach"`
	Outcome  string       `json:"outcome,omitempty"`
	TriedBy  string       `json:"tried_by,omitempty"`
}

// NewApproachTried creates an ApproachTried event.
func NewApproachTried(nodeID types.NodeID, approach, outcome, triedBy string) ApproachTried {
	return ApproachTried{
		BaseEvent: BaseEvent{
			EventType: EventApproachTried,
			EventTime: types.Now(),
		},
		NodeID:   nodeID,
		Approach: approach,
		Outcome:  outcome,
		TriedBy:  triedBy,
	}
}

// EvidenceAttached is emitted when computational evidence (a script, dataset,
// or verification result) is linked to a proof node.
type EvidenceAttached struct {
	BaseEvent
	NodeID       types.NodeID `json:"node_id"`
	FilePath     string       `json:"file_path"`               // Path relative to proof dir
	ContentHash  string       `json:"content_hash"`            // SHA256 of file content
	EvidenceType string       `json:"evidence_type,omitempty"` // verification, computation, test, other
	Description  string       `json:"description,omitempty"`
	AttachedBy   string       `json:"attached_by,omitempty"`
}

// NewEvidenceAttached creates an EvidenceAttached event.
func NewEvidenceAttached(nodeID types.NodeID, filePath, contentHash, evidenceType, description, attachedBy string) EvidenceAttached {
	return EvidenceAttached{
		BaseEvent: BaseEvent{
			EventType: EventEvidenceAttached,
			EventTime: types.Now(),
		},
		NodeID:       nodeID,
		FilePath:     filePath,
		ContentHash:  contentHash,
		EvidenceType: evidenceType,
		Description:  description,
		AttachedBy:   attachedBy,
	}
}

// OutlineStage represents a single stage in a proof outline.
type OutlineStage struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Criticality string `json:"criticality"` // "critical", "important", "routine"
}

// OutlineSet is emitted when a proof outline is defined or replaced.
// This replaces any previous outline entirely.
type OutlineSet struct {
	BaseEvent
	Stages []OutlineStage `json:"stages"`
	SetBy  string         `json:"set_by,omitempty"`
}

// NewOutlineSet creates an OutlineSet event.
func NewOutlineSet(stages []OutlineStage, setBy string) OutlineSet {
	return OutlineSet{
		BaseEvent: BaseEvent{
			EventType: EventOutlineSet,
			EventTime: types.Now(),
		},
		Stages: stages,
		SetBy:  setBy,
	}
}

// OutlineStageLinked is emitted when an outline stage is mapped to a subtree root node.
type OutlineStageLinked struct {
	BaseEvent
	Label  string       `json:"label"`
	NodeID types.NodeID `json:"node_id"`
}

// NewOutlineStageLinked creates an OutlineStageLinked event.
func NewOutlineStageLinked(label string, nodeID types.NodeID) OutlineStageLinked {
	return OutlineStageLinked{
		BaseEvent: BaseEvent{
			EventType: EventOutlineStageLinked,
			EventTime: types.Now(),
		},
		Label:  label,
		NodeID: nodeID,
	}
}

// HintAdded is emitted when a domain expert adds a directional hint to a node.
type HintAdded struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
	Text   string       `json:"text"`
	HintBy string       `json:"hint_by,omitempty"`
}

// NewHintAdded creates a HintAdded event.
func NewHintAdded(nodeID types.NodeID, text, hintBy string) HintAdded {
	return HintAdded{
		BaseEvent: BaseEvent{
			EventType: EventHintAdded,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
		Text:   text,
		HintBy: hintBy,
	}
}

// NodeVetoed is emitted when a human expert force-refutes a node,
// bypassing normal adversarial workflow. Unlike NodeRefuted, this can
// override any non-terminal state including validated and admitted nodes.
type NodeVetoed struct {
	BaseEvent
	NodeID   types.NodeID `json:"node_id"`
	Reason   string       `json:"reason"`
	VetoedBy string       `json:"vetoed_by,omitempty"`
}

// NewNodeVetoed creates a NodeVetoed event.
func NewNodeVetoed(nodeID types.NodeID, reason, vetoedBy string) NodeVetoed {
	return NodeVetoed{
		BaseEvent: BaseEvent{
			EventType: EventNodeVetoed,
			EventTime: types.Now(),
		},
		NodeID:   nodeID,
		Reason:   reason,
		VetoedBy: vetoedBy,
	}
}

// StrategyProposed is emitted when a prover proposes a proof strategy for a node.
// This records the strategy, its novelty assessment, and rationale for comparison.
type StrategyProposed struct {
	BaseEvent
	NodeID     types.NodeID `json:"node_id"`
	Strategy   string       `json:"strategy"`
	Novelty    string       `json:"novelty"`
	Rationale  string       `json:"rationale,omitempty"`
	ProposedBy string       `json:"proposed_by,omitempty"`
}

// NewStrategyProposed creates a StrategyProposed event.
func NewStrategyProposed(nodeID types.NodeID, strategy, novelty, rationale, proposedBy string) StrategyProposed {
	return StrategyProposed{
		BaseEvent: BaseEvent{
			EventType: EventStrategyProposed,
			EventTime: types.Now(),
		},
		NodeID:     nodeID,
		Strategy:   strategy,
		Novelty:    novelty,
		Rationale:  rationale,
		ProposedBy: proposedBy,
	}
}

// NewNodeUnvalidated creates a NodeUnvalidated event.
func NewNodeUnvalidated(nodeID types.NodeID, reason, revokedBy string) NodeUnvalidated {
	return NodeUnvalidated{
		BaseEvent: BaseEvent{
			EventType: EventNodeUnvalidated,
			EventTime: types.Now(),
		},
		NodeID:    nodeID,
		Reason:    reason,
		RevokedBy: revokedBy,
	}
}

// NewNodeUnadmitted creates a NodeUnadmitted event.
func NewNodeUnadmitted(nodeID types.NodeID, reason, revokedBy string) NodeUnadmitted {
	return NodeUnadmitted{
		BaseEvent: BaseEvent{
			EventType: EventNodeUnadmitted,
			EventTime: types.Now(),
		},
		NodeID:    nodeID,
		Reason:    reason,
		RevokedBy: revokedBy,
	}
}

// PatternAdded is emitted when a failure pattern is registered in the workspace.
// Failure patterns capture recurring proof anti-patterns (e.g., continuum-to-finite
// fallacy) so agents can recognize and avoid them.
type PatternAdded struct {
	BaseEvent
	Name        string `json:"name"`
	Description string `json:"description"`
	Indicators  string `json:"indicators,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	AddedBy     string `json:"added_by,omitempty"`
}

// NewPatternAdded creates a PatternAdded event.
func NewPatternAdded(name, description, indicators, remediation, addedBy string) PatternAdded {
	return PatternAdded{
		BaseEvent: BaseEvent{
			EventType: EventPatternAdded,
			EventTime: types.Now(),
		},
		Name:        name,
		Description: description,
		Indicators:  indicators,
		Remediation: remediation,
		AddedBy:     addedBy,
	}
}

// ClaimTested is emitted when a computational falsification test is run against
// a proof node's claim. The test can use an external script or an inline sympy
// expression. Results are stored for audit and used to gate acceptance of crux nodes.
type ClaimTested struct {
	BaseEvent
	NodeID     types.NodeID `json:"node_id"`
	Engine     string       `json:"engine"` // "script", "sympy"
	ScriptPath string       `json:"script_path,omitempty"`
	Expression string       `json:"expression,omitempty"`
	Passed     bool         `json:"passed"`
	Output     string       `json:"output,omitempty"`
	Agent      string       `json:"agent,omitempty"`
}

// NewClaimTested creates a ClaimTested event.
func NewClaimTested(nodeID types.NodeID, engine, scriptPath, expression string, passed bool, output, agent string) ClaimTested {
	return ClaimTested{
		BaseEvent: BaseEvent{
			EventType: EventClaimTested,
			EventTime: types.Now(),
		},
		NodeID:     nodeID,
		Engine:     engine,
		ScriptPath: scriptPath,
		Expression: expression,
		Passed:     passed,
		Output:     output,
		Agent:      agent,
	}
}

// DefChecked is emitted when a definition stress test is run against a registered
// definition. Tests verify boundary cases, non-triviality, or run user-provided
// scripts. Failed tests warn provers before building on flawed definitions.
type DefChecked struct {
	BaseEvent
	DefName    string `json:"def_name"`
	CheckType  string `json:"check_type"` // "boundary", "non_triviality", "script"
	ScriptPath string `json:"script_path,omitempty"`
	Passed     bool   `json:"passed"`
	Output     string `json:"output,omitempty"`
	Agent      string `json:"agent,omitempty"`
}

// NewDefChecked creates a DefChecked event.
func NewDefChecked(defName, checkType, scriptPath string, passed bool, output, agent string) DefChecked {
	return DefChecked{
		BaseEvent: BaseEvent{
			EventType: EventDefChecked,
			EventTime: types.Now(),
		},
		DefName:    defName,
		CheckType:  checkType,
		ScriptPath: scriptPath,
		Passed:     passed,
		Output:     output,
		Agent:      agent,
	}
}
