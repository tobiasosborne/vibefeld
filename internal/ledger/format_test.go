package ledger

import "testing"

func TestEventType_MinFormat(t *testing.T) {
	// Every existing event type is part of format 1.0.
	for _, et := range []EventType{
		EventProofInitialized,
		EventNodeCreated,
		EventNodeValidated,
		EventNodeProofAuthored,
		EventDefChecked,
	} {
		if got := et.MinFormat(); got != "1.0" {
			t.Errorf("%s.MinFormat() = %q, want 1.0", et, got)
		}
	}

	// Unknown types default to 1.0.
	if got := EventType("future_unknown").MinFormat(); got != "1.0" {
		t.Errorf("unknown MinFormat() = %q, want 1.0", got)
	}
}

func TestRegisterEventMinFormat(t *testing.T) {
	const future EventType = "future_format_1_1_event"
	RegisterEventMinFormat(future, "1.1")
	defer RegisterEventMinFormat(future, "1.0")

	if got := future.MinFormat(); got != "1.1" {
		t.Errorf("registered MinFormat() = %q, want 1.1", got)
	}
}
