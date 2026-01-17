// Package state provides derived state from replaying ledger events.
package state

import (
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
)

// -----------------------------------------------------------------------------
// Unit Tests for replay.go functions (not requiring filesystem)
// -----------------------------------------------------------------------------

// TestExtractEventType_ProofInitialized verifies extractEventType parses ProofInitialized correctly.
func TestExtractEventType_ProofInitialized(t *testing.T) {
	data := []byte(`{"type":"proof_initialized","conjecture":"test","author":"author"}`)

	eventType, err := extractEventType(data)
	if err != nil {
		t.Fatalf("extractEventType failed: %v", err)
	}

	if eventType != ledger.EventProofInitialized {
		t.Errorf("eventType: got %q, want %q", eventType, ledger.EventProofInitialized)
	}
}

// TestExtractEventType_NodeCreated verifies extractEventType parses NodeCreated correctly.
func TestExtractEventType_NodeCreated(t *testing.T) {
	data := []byte(`{"type":"node_created","node":{"id":"1","statement":"test"}}`)

	eventType, err := extractEventType(data)
	if err != nil {
		t.Fatalf("extractEventType failed: %v", err)
	}

	if eventType != ledger.EventNodeCreated {
		t.Errorf("eventType: got %q, want %q", eventType, ledger.EventNodeCreated)
	}
}

// TestExtractEventType_AllKnownTypes verifies extractEventType works for all known event types.
func TestExtractEventType_AllKnownTypes(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantType  ledger.EventType
	}{
		{"proof_initialized", `{"type":"proof_initialized"}`, ledger.EventProofInitialized},
		{"node_created", `{"type":"node_created"}`, ledger.EventNodeCreated},
		{"nodes_claimed", `{"type":"nodes_claimed"}`, ledger.EventNodesClaimed},
		{"nodes_released", `{"type":"nodes_released"}`, ledger.EventNodesReleased},
		{"challenge_raised", `{"type":"challenge_raised"}`, ledger.EventChallengeRaised},
		{"challenge_resolved", `{"type":"challenge_resolved"}`, ledger.EventChallengeResolved},
		{"challenge_withdrawn", `{"type":"challenge_withdrawn"}`, ledger.EventChallengeWithdrawn},
		{"challenge_superseded", `{"type":"challenge_superseded"}`, ledger.EventChallengeSuperseded},
		{"node_validated", `{"type":"node_validated"}`, ledger.EventNodeValidated},
		{"node_admitted", `{"type":"node_admitted"}`, ledger.EventNodeAdmitted},
		{"node_refuted", `{"type":"node_refuted"}`, ledger.EventNodeRefuted},
		{"node_archived", `{"type":"node_archived"}`, ledger.EventNodeArchived},
		{"node_amended", `{"type":"node_amended"}`, ledger.EventNodeAmended},
		{"taint_recomputed", `{"type":"taint_recomputed"}`, ledger.EventTaintRecomputed},
		{"def_added", `{"type":"def_added"}`, ledger.EventDefAdded},
		{"lemma_extracted", `{"type":"lemma_extracted"}`, ledger.EventLemmaExtracted},
		{"lock_reaped", `{"type":"lock_reaped"}`, ledger.EventLockReaped},
		{"scope_opened", `{"type":"scope_opened"}`, ledger.EventScopeOpened},
		{"scope_closed", `{"type":"scope_closed"}`, ledger.EventScopeClosed},
		{"claim_refreshed", `{"type":"claim_refreshed"}`, ledger.EventClaimRefreshed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType, err := extractEventType([]byte(tt.json))
			if err != nil {
				t.Fatalf("extractEventType failed: %v", err)
			}
			if eventType != tt.wantType {
				t.Errorf("eventType: got %q, want %q", eventType, tt.wantType)
			}
		})
	}
}

// TestExtractEventType_MissingType verifies extractEventType returns error for missing type field.
func TestExtractEventType_MissingType(t *testing.T) {
	data := []byte(`{"conjecture":"test","author":"author"}`)

	_, err := extractEventType(data)
	if err == nil {
		t.Fatal("extractEventType should fail for missing type field")
	}
}

// TestExtractEventType_MalformedJSON verifies extractEventType handles malformed JSON.
func TestExtractEventType_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"missing_colon", `{"type" "proof_initialized"}`},
		{"unterminated_string", `{"type":"proof_initialized`},
		{"type_not_string", `{"type":123}`},
		{"empty", ``},
		{"just_brace", `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractEventType([]byte(tt.json))
			if err == nil {
				t.Errorf("extractEventType should fail for %s", tt.name)
			}
		})
	}
}

// TestExtractEventType_WhitespaceVariations verifies extractEventType handles whitespace.
func TestExtractEventType_WhitespaceVariations(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"spaces_around_colon", `{"type" : "proof_initialized"}`},
		{"tabs", `{"type"	:	"proof_initialized"}`},
		{"newlines", `{"type"
:
"proof_initialized"}`},
		{"multiple_spaces", `{"type"     :     "proof_initialized"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType, err := extractEventType([]byte(tt.json))
			if err != nil {
				t.Fatalf("extractEventType failed: %v", err)
			}
			if eventType != ledger.EventProofInitialized {
				t.Errorf("eventType: got %q, want %q", eventType, ledger.EventProofInitialized)
			}
		})
	}
}

// TestExtractEventType_EscapedCharacters verifies extractEventType handles escaped characters.
// Note: extractEventType uses fast byte scanning and doesn't unescape - it returns the raw bytes.
func TestExtractEventType_EscapedCharacters(t *testing.T) {
	// Type value with escaped quote - extractEventType skips escaped chars but doesn't unescape
	data := []byte(`{"type":"test\\escaped"}`)

	eventType, err := extractEventType(data)
	if err != nil {
		t.Fatalf("extractEventType failed: %v", err)
	}

	// extractEventType returns raw bytes (without unescaping), so escaped backslash remains
	if eventType != ledger.EventType(`test\\escaped`) {
		t.Errorf("eventType: got %q, want %q", eventType, `test\\escaped`)
	}
}

// TestParseEvent_ProofInitialized verifies parseEvent correctly parses ProofInitialized.
func TestParseEvent_ProofInitialized(t *testing.T) {
	data := []byte(`{"type":"proof_initialized","conjecture":"Test conjecture","author":"test-author","time":"2024-01-01T00:00:00Z"}`)

	event, err := parseEvent(data)
	if err != nil {
		t.Fatalf("parseEvent failed: %v", err)
	}

	if event.Type() != ledger.EventProofInitialized {
		t.Errorf("event type: got %q, want %q", event.Type(), ledger.EventProofInitialized)
	}

	pi, ok := event.(ledger.ProofInitialized)
	if !ok {
		t.Fatalf("event is not ProofInitialized: %T", event)
	}

	if pi.Conjecture != "Test conjecture" {
		t.Errorf("Conjecture: got %q, want %q", pi.Conjecture, "Test conjecture")
	}
	if pi.Author != "test-author" {
		t.Errorf("Author: got %q, want %q", pi.Author, "test-author")
	}
}

// TestParseEvent_NodeValidated verifies parseEvent correctly parses NodeValidated.
func TestParseEvent_NodeValidated(t *testing.T) {
	data := []byte(`{"type":"node_validated","node_id":"1.2.3","time":"2024-01-01T00:00:00Z"}`)

	event, err := parseEvent(data)
	if err != nil {
		t.Fatalf("parseEvent failed: %v", err)
	}

	if event.Type() != ledger.EventNodeValidated {
		t.Errorf("event type: got %q, want %q", event.Type(), ledger.EventNodeValidated)
	}

	nv, ok := event.(ledger.NodeValidated)
	if !ok {
		t.Fatalf("event is not NodeValidated: %T", event)
	}

	if nv.NodeID.String() != "1.2.3" {
		t.Errorf("NodeID: got %q, want %q", nv.NodeID.String(), "1.2.3")
	}
}

// TestParseEvent_ChallengeRaised verifies parseEvent correctly parses ChallengeRaised.
func TestParseEvent_ChallengeRaised(t *testing.T) {
	data := []byte(`{"type":"challenge_raised","challenge_id":"ch-001","node_id":"1","target":"statement","reason":"unclear","severity":"major","raised_by":"verifier-1","time":"2024-01-01T00:00:00Z"}`)

	event, err := parseEvent(data)
	if err != nil {
		t.Fatalf("parseEvent failed: %v", err)
	}

	cr, ok := event.(ledger.ChallengeRaised)
	if !ok {
		t.Fatalf("event is not ChallengeRaised: %T", event)
	}

	if cr.ChallengeID != "ch-001" {
		t.Errorf("ChallengeID: got %q, want %q", cr.ChallengeID, "ch-001")
	}
	if cr.NodeID.String() != "1" {
		t.Errorf("NodeID: got %q, want %q", cr.NodeID.String(), "1")
	}
	if cr.Severity != "major" {
		t.Errorf("Severity: got %q, want %q", cr.Severity, "major")
	}
}

// TestParseEvent_UnknownType verifies parseEvent returns error for unknown event types.
func TestParseEvent_UnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown_event_type","data":"test"}`)

	_, err := parseEvent(data)
	if err == nil {
		t.Fatal("parseEvent should fail for unknown event type")
	}
}

// TestParseEvent_InvalidJSON verifies parseEvent returns error for invalid JSON.
func TestParseEvent_InvalidJSON(t *testing.T) {
	data := []byte(`{"type":"proof_initialized"`) // missing closing brace

	_, err := parseEvent(data)
	if err == nil {
		t.Fatal("parseEvent should fail for invalid JSON")
	}
}

// TestParseEvent_MissingTypeField verifies parseEvent returns error when type is missing.
func TestParseEvent_MissingTypeField(t *testing.T) {
	data := []byte(`{"conjecture":"test"}`)

	_, err := parseEvent(data)
	if err == nil {
		t.Fatal("parseEvent should fail when type field is missing")
	}
}

// TestReplay_NilLedger verifies Replay returns error for nil ledger.
func TestReplay_NilLedger(t *testing.T) {
	_, err := Replay(nil)
	if err == nil {
		t.Fatal("Replay should return error for nil ledger")
	}
}

// TestReplayWithVerify_NilLedger verifies ReplayWithVerify returns error for nil ledger.
func TestReplayWithVerify_NilLedger(t *testing.T) {
	_, err := ReplayWithVerify(nil)
	if err == nil {
		t.Fatal("ReplayWithVerify should return error for nil ledger")
	}
}

// TestEventFactoriesCompleteness verifies all known event types have factories.
func TestEventFactoriesCompleteness(t *testing.T) {
	expectedTypes := []ledger.EventType{
		ledger.EventProofInitialized,
		ledger.EventNodeCreated,
		ledger.EventNodesClaimed,
		ledger.EventNodesReleased,
		ledger.EventChallengeRaised,
		ledger.EventChallengeResolved,
		ledger.EventChallengeWithdrawn,
		ledger.EventChallengeSuperseded,
		ledger.EventNodeValidated,
		ledger.EventNodeAdmitted,
		ledger.EventNodeRefuted,
		ledger.EventNodeArchived,
		ledger.EventNodeAmended,
		ledger.EventTaintRecomputed,
		ledger.EventDefAdded,
		ledger.EventLemmaExtracted,
		ledger.EventLockReaped,
		ledger.EventClaimRefreshed,
	}

	for _, et := range expectedTypes {
		factory, ok := eventFactories[et]
		if !ok {
			t.Errorf("Missing factory for event type: %s", et)
			continue
		}
		// Verify factory produces non-nil event
		event := factory()
		if event == nil {
			t.Errorf("Factory for %s produced nil event", et)
		}
	}
}

// TestParseEvent_AllEventTypes verifies parseEvent can parse minimal valid JSON for all types.
func TestParseEvent_AllEventTypes(t *testing.T) {
	// Minimal valid JSON for each event type
	tests := []struct {
		name     string
		json     string
		wantType ledger.EventType
	}{
		{
			"proof_initialized",
			`{"type":"proof_initialized","conjecture":"test","author":"auth","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventProofInitialized,
		},
		{
			"node_validated",
			`{"type":"node_validated","node_id":"1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventNodeValidated,
		},
		{
			"node_admitted",
			`{"type":"node_admitted","node_id":"1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventNodeAdmitted,
		},
		{
			"node_refuted",
			`{"type":"node_refuted","node_id":"1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventNodeRefuted,
		},
		{
			"node_archived",
			`{"type":"node_archived","node_id":"1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventNodeArchived,
		},
		{
			"challenge_resolved",
			`{"type":"challenge_resolved","challenge_id":"ch-1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventChallengeResolved,
		},
		{
			"challenge_withdrawn",
			`{"type":"challenge_withdrawn","challenge_id":"ch-1","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventChallengeWithdrawn,
		},
		{
			"taint_recomputed",
			`{"type":"taint_recomputed","node_id":"1","new_taint":"clean","time":"2024-01-01T00:00:00Z"}`,
			ledger.EventTaintRecomputed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parseEvent([]byte(tt.json))
			if err != nil {
				t.Fatalf("parseEvent failed: %v", err)
			}
			if event.Type() != tt.wantType {
				t.Errorf("event type: got %q, want %q", event.Type(), tt.wantType)
			}
		})
	}
}
