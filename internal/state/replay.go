// Package state provides derived state from replaying ledger events.
package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tobias/vibefeld/internal/ledger"
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
	ledger.EventTaintRecomputed:     func() ledger.Event { return &ledger.TaintRecomputed{} },
	ledger.EventDefAdded:            func() ledger.Event { return &ledger.DefAdded{} },
	ledger.EventLemmaExtracted:      func() ledger.Event { return &ledger.LemmaExtracted{} },
	ledger.EventLockReaped:          func() ledger.Event { return &ledger.LockReaped{} },
	ledger.EventClaimRefreshed:      func() ledger.Event { return &ledger.ClaimRefreshed{} },
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

	// Dereference pointer to get value type (required for Apply's type assertions)
	// Uses reflection to avoid a separate type switch for each event type
	return reflect.ValueOf(eventPtr).Elem().Interface().(ledger.Event), nil
}
