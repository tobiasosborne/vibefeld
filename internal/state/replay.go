// Package state provides derived state from replaying ledger events.
package state

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/taint"
)

// Replay reads all events from the ledger and applies them to build the current state.
// Returns an error if the ledger is nil, contains invalid JSON, or has unknown event types.
func Replay(ldg *ledger.Ledger) (*State, error) {
	return replayInternal(ldg, false)
}

// ReplayWithVerify reads all events from the ledger, applies them to build state,
// and verifies content hashes on all nodes. Returns an error if any node's
// content hash does not match its computed hash.
func ReplayWithVerify(ldg *ledger.Ledger) (*State, error) {
	return replayInternal(ldg, true)
}

// replayInternal is the shared implementation for Replay and ReplayWithVerify.
func replayInternal(ldg *ledger.Ledger, verifyHashes bool) (*State, error) {
	if ldg == nil {
		return nil, fmt.Errorf("cannot replay from nil ledger")
	}

	state := NewState()

	// Track expected sequence number for validation (starts at 1)
	expectedSeq := 1

	// Scan through all events and apply them, tracking sequence numbers
	err := ldg.Scan(func(seq int, data []byte) error {
		// Validate sequence numbers are consecutive starting from 1
		if seq != expectedSeq {
			if seq < expectedSeq {
				return fmt.Errorf("duplicate sequence number detected: got %d, expected %d", seq, expectedSeq)
			}
			return fmt.Errorf("sequence gap detected: got %d, expected %d", seq, expectedSeq)
		}
		expectedSeq++

		// Parse the event type first
		event, err := parseEvent(data)
		if err != nil {
			return fmt.Errorf("failed to parse event %d: %w", seq, err)
		}

		// Apply the event to state
		if err := Apply(state, event); err != nil {
			return fmt.Errorf("failed to apply event %d (%s): %w", seq, event.Type(), err)
		}

		// Track the latest sequence number for optimistic concurrency control
		state.SetLatestSeq(seq)

		// Stamp the ledger sequence onto the amendment record just appended, so
		// the export projection can report amendment sequences.
		switch ev := event.(type) {
		case ledger.NodeAmended:
			state.SetLastAmendmentSeq(ev.NodeID, seq)
		case ledger.NodeAmendedReopened:
			state.SetLastAmendmentSeq(ev.NodeID, seq)
		case ledger.NodeDepsAmended:
			state.SetLastAmendmentSeq(ev.NodeID, seq)
		case ledger.NodesClaimed:
			// The claim generation is the ledger sequence of the NodesClaimed
			// event that created the current claim (D5). Derived state, so it is
			// stamped here, never carried on the event; a release (explicit or
			// fenced auto-release) clears it in Apply.
			for _, id := range ev.NodeIDs {
				if n := state.GetNode(id); n != nil {
					n.ClaimSeq = seq
				}
			}
		case ledger.NodeValidated:
			// Derived verdict sequence for support_current (D4); no event field,
			// stamped here so a later amendment can be ordered against it.
			if n := state.GetNode(ev.NodeID); n != nil {
				n.VerdictSeq = seq
			}
		case ledger.NodeAdmitted:
			// Admitted is also a terminal verdict: stamp the sequence so
			// support_current's revision guards cover admitted nodes too.
			if n := state.GetNode(ev.NodeID); n != nil {
				n.VerdictSeq = seq
			}
		case ledger.NodeArchived:
			// Derived archival sequence (D9): support_current compares a
			// direct child's archival against its parent's verdict, and the
			// durable abandoned-obligation snapshot is copied onto the node
			// so the checklist can read it without re-scanning the ledger.
			if n := state.GetNode(ev.NodeID); n != nil {
				n.ArchivedSeq = seq
				n.AbandonedObligations = append([]string(nil), ev.AbandonedObligations...)
			}
		case ledger.ChallengeRaised:
			if c := state.GetChallenge(ev.ChallengeID); c != nil {
				c.Seq = seq
			}
		}

		// Index an optional operation id so a retried operation can find the
		// sequence of its already-committed result. For a dependency amendment
		// we also bind the id to the event type, node and request fingerprint,
		// so a reuse of the same id for a different request is detectable.
		if opIDCarrier, ok := event.(interface{ GetOperationID() string }); ok {
			if id := opIDCarrier.GetOperationID(); id != "" {
				rec := OperationRecord{Seq: seq, EventType: string(event.Type())}
				if deps, ok := event.(ledger.NodeDepsAmended); ok {
					rec.NodeID = deps.NodeID.String()
					rec.RequestFingerprint = deps.RequestFingerprint
					rec.PreviousHash = deps.PreviousContentHash
					rec.Reopened = deps.Reopened
					if n := state.GetNode(deps.NodeID); n != nil {
						rec.NewHash = n.ContentHash
					}
				}
				state.RecordOperation(id, rec)
			}
		}

		// If verifying hashes and this is a NodeCreated event, verify the hash
		if verifyHashes {
			if nodeCreated, ok := event.(ledger.NodeCreated); ok {
				// Get the node from state (it was just added)
				n := state.GetNode(nodeCreated.Node.ID)
				if n != nil && !n.VerifyContentHash() {
					return fmt.Errorf("content hash verification failed for node %s", n.ID.String())
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// TaintRecomputed events are retained and applied as audit records, but
	// taint itself is derived. Recompute it authoritatively after the complete
	// event stream so ledgers produced by older versions self-heal on load.
	taint.RecomputeAll(state.AllNodes())

	return state, nil
}

// extractEventType extracts the event type from JSON data using fast byte scanning.
// This avoids a full JSON unmarshal just to read the "type" field, eliminating
// the overhead of double JSON parsing.
// Returns the type string and an error if the type field cannot be found.
func extractEventType(data []byte) (ledger.EventType, error) {
	// Look for "type": or "type" : (with potential whitespace)
	typeKey := []byte(`"type"`)
	idx := bytes.Index(data, typeKey)
	if idx == -1 {
		return "", fmt.Errorf("invalid JSON: missing type field")
	}

	// Skip past "type" and find the colon
	pos := idx + len(typeKey)
	for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n' || data[pos] == '\r') {
		pos++
	}
	if pos >= len(data) || data[pos] != ':' {
		return "", fmt.Errorf("invalid JSON: malformed type field")
	}
	pos++ // skip colon

	// Skip whitespace after colon
	for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n' || data[pos] == '\r') {
		pos++
	}
	if pos >= len(data) || data[pos] != '"' {
		return "", fmt.Errorf("invalid JSON: type value must be a string")
	}
	pos++ // skip opening quote

	// Find the closing quote (handle escaped quotes)
	start := pos
	for pos < len(data) {
		if data[pos] == '\\' && pos+1 < len(data) {
			pos += 2 // skip escaped character
			continue
		}
		if data[pos] == '"' {
			return ledger.EventType(data[start:pos]), nil
		}
		pos++
	}

	return "", fmt.Errorf("invalid JSON: unterminated type string")
}

// eventFactory is a function that creates a new instance of a specific event type.
type eventFactory func() ledger.Event

// eventFactories maps event type strings to factory functions that create
// instances of the corresponding event types. This registry pattern eliminates
// the repetitive switch statement and makes adding new event types trivial.
var eventFactories = map[ledger.EventType]eventFactory{
	ledger.EventProofInitialized:    func() ledger.Event { return &ledger.ProofInitialized{} },
	ledger.EventNodeCreated:         func() ledger.Event { return &ledger.NodeCreated{} },
	ledger.EventNodesClaimed:        func() ledger.Event { return &ledger.NodesClaimed{} },
	ledger.EventNodesReleased:       func() ledger.Event { return &ledger.NodesReleased{} },
	ledger.EventChallengeRaised:     func() ledger.Event { return &ledger.ChallengeRaised{} },
	ledger.EventChallengeResolved:   func() ledger.Event { return &ledger.ChallengeResolved{} },
	ledger.EventChallengeWithdrawn:  func() ledger.Event { return &ledger.ChallengeWithdrawn{} },
	ledger.EventChallengeSuperseded: func() ledger.Event { return &ledger.ChallengeSuperseded{} },
	ledger.EventNodeValidated:       func() ledger.Event { return &ledger.NodeValidated{} },
	ledger.EventNodeAdmitted:        func() ledger.Event { return &ledger.NodeAdmitted{} },
	ledger.EventNodeRefuted:         func() ledger.Event { return &ledger.NodeRefuted{} },
	ledger.EventNodeArchived:        func() ledger.Event { return &ledger.NodeArchived{} },
	ledger.EventNodeAmended:         func() ledger.Event { return &ledger.NodeAmended{} },
	ledger.EventNodeAmendedReopened: func() ledger.Event { return &ledger.NodeAmendedReopened{} },
	ledger.EventTaintRecomputed:     func() ledger.Event { return &ledger.TaintRecomputed{} },
	ledger.EventDefAdded:            func() ledger.Event { return &ledger.DefAdded{} },
	ledger.EventLemmaExtracted:      func() ledger.Event { return &ledger.LemmaExtracted{} },
	ledger.EventLockReaped:          func() ledger.Event { return &ledger.LockReaped{} },
	ledger.EventClaimRefreshed:      func() ledger.Event { return &ledger.ClaimRefreshed{} },
	ledger.EventScopeOpened:         func() ledger.Event { return &ledger.ScopeOpened{} },
	ledger.EventScopeClosed:         func() ledger.Event { return &ledger.ScopeClosed{} },
	ledger.EventRefinementRequested: func() ledger.Event { return &ledger.RefinementRequested{} },
	ledger.EventNodeSubmitted:       func() ledger.Event { return &ledger.NodeSubmitted{} },
	ledger.EventNodeUnvalidated:     func() ledger.Event { return &ledger.NodeUnvalidated{} },
	ledger.EventNodeUnadmitted:      func() ledger.Event { return &ledger.NodeUnadmitted{} },
	ledger.EventApproachTried:       func() ledger.Event { return &ledger.ApproachTried{} },
	ledger.EventEvidenceAttached:    func() ledger.Event { return &ledger.EvidenceAttached{} },
	ledger.EventOutlineSet:          func() ledger.Event { return &ledger.OutlineSet{} },
	ledger.EventOutlineStageLinked:  func() ledger.Event { return &ledger.OutlineStageLinked{} },
	ledger.EventHintAdded:           func() ledger.Event { return &ledger.HintAdded{} },
	ledger.EventNodeVetoed:          func() ledger.Event { return &ledger.NodeVetoed{} },
	ledger.EventStrategyProposed:    func() ledger.Event { return &ledger.StrategyProposed{} },
	ledger.EventPatternAdded:        func() ledger.Event { return &ledger.PatternAdded{} },
	ledger.EventClaimTested:         func() ledger.Event { return &ledger.ClaimTested{} },
	ledger.EventDefChecked:          func() ledger.Event { return &ledger.DefChecked{} },
	ledger.EventNodeProofAuthored:   func() ledger.Event { return &ledger.NodeProofAuthored{} },
	ledger.EventNodeDepsAmended:     func() ledger.Event { return &ledger.NodeDepsAmended{} },
}

// parseEvent parses raw JSON bytes into a typed Event.
// Returns an error if the JSON is invalid or the event type is unknown.
// Uses optimized byte scanning to extract the type field, avoiding double JSON parsing.
func parseEvent(data []byte) (ledger.Event, error) {
	// Extract event type using fast byte scanning (no JSON unmarshal)
	eventType, err := extractEventType(data)
	if err != nil {
		return nil, err
	}

	// Look up the factory for this event type
	factory, ok := eventFactories[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	// Create a new instance (pointer) and unmarshal into it
	eventPtr := factory()
	if err := json.Unmarshal(data, eventPtr); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", eventType, err)
	}

	// Dereference pointer to get value type (required for Apply's type assertions).
	// This type switch avoids reflection overhead in the hot path.
	return derefEvent(eventPtr), nil
}

// derefEvent dereferences an event pointer to return the value type.
// This eliminates reflection overhead in the event parsing hot path.
func derefEvent(eventPtr ledger.Event) ledger.Event {
	switch e := eventPtr.(type) {
	case *ledger.ProofInitialized:
		return *e
	case *ledger.NodeCreated:
		return *e
	case *ledger.NodesClaimed:
		return *e
	case *ledger.NodesReleased:
		return *e
	case *ledger.ChallengeRaised:
		return *e
	case *ledger.ChallengeResolved:
		return *e
	case *ledger.ChallengeWithdrawn:
		return *e
	case *ledger.ChallengeSuperseded:
		return *e
	case *ledger.NodeValidated:
		return *e
	case *ledger.NodeAdmitted:
		return *e
	case *ledger.NodeRefuted:
		return *e
	case *ledger.NodeArchived:
		return *e
	case *ledger.NodeAmended:
		return *e
	case *ledger.NodeAmendedReopened:
		return *e
	case *ledger.TaintRecomputed:
		return *e
	case *ledger.DefAdded:
		return *e
	case *ledger.LemmaExtracted:
		return *e
	case *ledger.LockReaped:
		return *e
	case *ledger.ClaimRefreshed:
		return *e
	case *ledger.ScopeOpened:
		return *e
	case *ledger.ScopeClosed:
		return *e
	case *ledger.RefinementRequested:
		return *e
	case *ledger.NodeSubmitted:
		return *e
	case *ledger.NodeUnvalidated:
		return *e
	case *ledger.NodeUnadmitted:
		return *e
	case *ledger.ApproachTried:
		return *e
	case *ledger.EvidenceAttached:
		return *e
	case *ledger.OutlineSet:
		return *e
	case *ledger.OutlineStageLinked:
		return *e
	case *ledger.HintAdded:
		return *e
	case *ledger.NodeVetoed:
		return *e
	case *ledger.StrategyProposed:
		return *e
	case *ledger.PatternAdded:
		return *e
	case *ledger.ClaimTested:
		return *e
	case *ledger.DefChecked:
		return *e
	case *ledger.NodeProofAuthored:
		return *e
	case *ledger.NodeDepsAmended:
		return *e
	default:
		// Should never happen since factory already validated the type
		return eventPtr
	}
}
