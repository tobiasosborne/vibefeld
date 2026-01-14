// Package state provides derived state from replaying ledger events.
package state

import (
	"bytes"
	"encoding/json"
	"fmt"

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

	// Scan through all events and apply them, tracking sequence numbers
	err := ldg.Scan(func(seq int, data []byte) error {
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

// parseEvent parses raw JSON bytes into a typed Event.
// Returns an error if the JSON is invalid or the event type is unknown.
// Uses optimized byte scanning to extract the type field, avoiding double JSON parsing.
func parseEvent(data []byte) (ledger.Event, error) {
	// Extract event type using fast byte scanning (no JSON unmarshal)
	eventType, err := extractEventType(data)
	if err != nil {
		return nil, err
	}

	// Now parse the full event based on type (single unmarshal)
	switch eventType {
	case ledger.EventProofInitialized:
		var e ledger.ProofInitialized
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse ProofInitialized: %w", err)
		}
		return e, nil

	case ledger.EventNodeCreated:
		var e ledger.NodeCreated
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodeCreated: %w", err)
		}
		return e, nil

	case ledger.EventNodesClaimed:
		var e ledger.NodesClaimed
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodesClaimed: %w", err)
		}
		return e, nil

	case ledger.EventNodesReleased:
		var e ledger.NodesReleased
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodesReleased: %w", err)
		}
		return e, nil

	case ledger.EventChallengeRaised:
		var e ledger.ChallengeRaised
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse ChallengeRaised: %w", err)
		}
		return e, nil

	case ledger.EventChallengeResolved:
		var e ledger.ChallengeResolved
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse ChallengeResolved: %w", err)
		}
		return e, nil

	case ledger.EventChallengeWithdrawn:
		var e ledger.ChallengeWithdrawn
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse ChallengeWithdrawn: %w", err)
		}
		return e, nil

	case ledger.EventNodeValidated:
		var e ledger.NodeValidated
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodeValidated: %w", err)
		}
		return e, nil

	case ledger.EventNodeAdmitted:
		var e ledger.NodeAdmitted
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodeAdmitted: %w", err)
		}
		return e, nil

	case ledger.EventNodeRefuted:
		var e ledger.NodeRefuted
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodeRefuted: %w", err)
		}
		return e, nil

	case ledger.EventNodeArchived:
		var e ledger.NodeArchived
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse NodeArchived: %w", err)
		}
		return e, nil

	case ledger.EventTaintRecomputed:
		var e ledger.TaintRecomputed
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse TaintRecomputed: %w", err)
		}
		return e, nil

	case ledger.EventDefAdded:
		var e ledger.DefAdded
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse DefAdded: %w", err)
		}
		return e, nil

	case ledger.EventLemmaExtracted:
		var e ledger.LemmaExtracted
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to parse LemmaExtracted: %w", err)
		}
		return e, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}
