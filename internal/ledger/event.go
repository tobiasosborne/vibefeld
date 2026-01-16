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
	EventProofInitialized     EventType = "proof_initialized"
	EventNodeCreated          EventType = "node_created"
	EventNodesClaimed         EventType = "nodes_claimed"
	EventNodesReleased        EventType = "nodes_released"
	EventChallengeRaised      EventType = "challenge_raised"
	EventChallengeResolved    EventType = "challenge_resolved"
	EventChallengeWithdrawn   EventType = "challenge_withdrawn"
	EventChallengeSuperseded  EventType = "challenge_superseded"
	EventNodeValidated        EventType = "node_validated"
	EventNodeAdmitted         EventType = "node_admitted"
	EventNodeRefuted          EventType = "node_refuted"
	EventNodeArchived         EventType = "node_archived"
	EventNodeAmended          EventType = "node_amended"
	EventTaintRecomputed      EventType = "taint_recomputed"
	EventDefAdded             EventType = "def_added"
	EventLemmaExtracted       EventType = "lemma_extracted"
	EventLockReaped           EventType = "lock_reaped"
	EventScopeOpened          EventType = "scope_opened"
	EventScopeClosed          EventType = "scope_closed"
	EventClaimRefreshed       EventType = "claim_refreshed"
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
type ChallengeRaised struct {
	BaseEvent
	ChallengeID string       `json:"challenge_id"`
	NodeID      types.NodeID `json:"node_id"`
	Target      string       `json:"target"`
	Reason      string       `json:"reason"`
	Severity    string       `json:"severity"` // "critical", "major", "minor", or "note"
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
type NodeValidated struct {
	BaseEvent
	NodeID types.NodeID `json:"node_id"`
	Note   string       `json:"note,omitempty"` // Optional acceptance note (partial acceptance)
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
	NodeID   types.NodeID     `json:"node_id"`
	NewTaint node.TaintState  `json:"new_taint"`
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

// NewChallengeRaised creates a ChallengeRaised event with default severity (major).
func NewChallengeRaised(challengeID string, nodeID types.NodeID, target, reason string) ChallengeRaised {
	return NewChallengeRaisedWithSeverity(challengeID, nodeID, target, reason, "major")
}

// NewChallengeRaisedWithSeverity creates a ChallengeRaised event with the specified severity.
func NewChallengeRaisedWithSeverity(challengeID string, nodeID types.NodeID, target, reason, severity string) ChallengeRaised {
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
// but wants to record a minor issue or clarification.
func NewNodeValidatedWithNote(nodeID types.NodeID, note string) NodeValidated {
	return NodeValidated{
		BaseEvent: BaseEvent{
			EventType: EventNodeValidated,
			EventTime: types.Now(),
		},
		NodeID: nodeID,
		Note:   note,
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
