package verdicts

import (
	"strings"
	"testing"
)

func validFileJSON() string {
	return `{
		"schema_version": "1",
		"batch_id": "batch-1",
		"verified_by": "verifier-1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "Follows from def 3.2"},
			{"node": "1.2", "verdict": "challenge", "target": "inference", "severity": "critical", "reason": "Modus ponens misapplied"}
		]
	}`
}

func TestParseFile_ValidFileRoundTrips(t *testing.T) {
	f, err := ParseFile([]byte(validFileJSON()))
	if err != nil {
		t.Fatalf("expected valid file to parse, got error: %v", err)
	}
	if f.BatchID != "batch-1" {
		t.Errorf("BatchID = %q, want %q", f.BatchID, "batch-1")
	}
	if f.VerifiedBy != "verifier-1" {
		t.Errorf("VerifiedBy = %q, want %q", f.VerifiedBy, "verifier-1")
	}
	if len(f.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(f.Items))
	}
	if f.Items[0].Verdict != VerdictAccept {
		t.Errorf("Items[0].Verdict = %q, want %q", f.Items[0].Verdict, VerdictAccept)
	}
	if f.Items[1].Verdict != VerdictChallenge {
		t.Errorf("Items[1].Verdict = %q, want %q", f.Items[1].Verdict, VerdictChallenge)
	}
}

func TestParseFile_DefaultsChallengeTargetAndSeverity(t *testing.T) {
	data := `{
		"schema_version": "1",
		"batch_id": "b1",
		"verified_by": "v1",
		"items": [
			{"node": "1.1", "verdict": "challenge", "reason": "unclear"}
		]
	}`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Items[0].Target != "statement" {
		t.Errorf("Target = %q, want default %q", f.Items[0].Target, "statement")
	}
	if f.Items[0].Severity != "major" {
		t.Errorf("Severity = %q, want default %q", f.Items[0].Severity, "major")
	}
}

func TestParseFile_RejectsInvalidJSON(t *testing.T) {
	_, err := ParseFile([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseFile_RejectsTrailingContent(t *testing.T) {
	data := validFileJSON() + `{"extra":true}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for trailing content, got nil")
	}
}

func TestParseFile_RejectsUnknownFields(t *testing.T) {
	data := `{
		"schema_version": "1",
		"batch_id": "b1",
		"verified_by": "v1",
		"items": [{"node": "1.1", "verdict": "accept", "reason": "ok", "typo_field": true}]
	}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParseFile_RejectsWrongSchemaVersion(t *testing.T) {
	data := `{"schema_version": "2", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"accept","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected schema_version error, got: %v", err)
	}
}

// rk-qxp: a MULTI-item verdict file still REQUIRES batch_id — a genuine batch
// needs a shared id so `af unvalidate --batch <id>` can revoke it as a unit
// (the multi-item batch contract is preserved).
func TestParseFile_RejectsMissingBatchIDForMultiItemFile(t *testing.T) {
	data := `{"schema_version": "1", "verified_by": "v1", "items": [
		{"node":"1.1","verdict":"accept","reason":"x"},
		{"node":"1.2","verdict":"accept","reason":"y"}
	]}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "batch_id") {
		t.Fatalf("expected batch_id error for a multi-item file, got: %v", err)
	}
}

// rk-qxp: a SINGLE-item, non-batch verdict file may OMIT batch_id — this is
// the per-node apply rk's driver emits. An absent batch_id parses cleanly with
// an empty BatchID (which records NO batch provenance on the node at apply
// time), unblocking every per-node apply the driver sends.
func TestParseFile_AllowsMissingBatchIDForSingleItemFile(t *testing.T) {
	data := `{"schema_version": "1", "verified_by": "v1", "items": [{"node":"1","verdict":"accept","reason":"x"}]}`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("expected a single-item file with no batch_id to parse, got: %v", err)
	}
	if f.BatchID != "" {
		t.Errorf("BatchID = %q, want empty for a non-batch apply", f.BatchID)
	}
}

// rk-qxp: an explicitly-empty batch_id on a single-item file is likewise
// accepted (rk's driver may serialize the key as "" rather than omit it).
func TestParseFile_AllowsEmptyBatchIDForSingleItemFile(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "", "verified_by": "v1", "items": [{"node":"1","verdict":"accept","reason":"x"}]}`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("expected a single-item file with empty batch_id to parse, got: %v", err)
	}
	if f.BatchID != "" {
		t.Errorf("BatchID = %q, want empty", f.BatchID)
	}
}

func TestParseFile_RejectsMissingVerifiedBy(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "items": [{"node":"1","verdict":"accept","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "verified_by") {
		t.Fatalf("expected verified_by error, got: %v", err)
	}
}

func TestParseFile_RejectsEmptyItems(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": []}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "items") {
		t.Fatalf("expected items error, got: %v", err)
	}
}

// TestParseFile_RejectsBlanketAccept is the direct test of PRD C3's "no
// blanket accepts" mandate: an accept item with an empty reason must be
// rejected at parse time, not silently accepted.
func TestParseFile_RejectsBlanketAccept(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"accept","reason":""}]}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("expected mandatory-justification error for blanket accept, got: %v", err)
	}
}

func TestParseFile_RejectsChallengeWithEmptyReason(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"challenge","reason":""}]}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for challenge with empty reason, got nil")
	}
}

func TestParseFile_RejectsInvalidVerdict(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"maybe","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Fatalf("expected verdict error, got: %v", err)
	}
}

func TestParseFile_RejectsInvalidNodeID(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"not-a-node-id","verdict":"accept","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for invalid node id, got nil")
	}
}

func TestParseFile_RejectsInvalidChallengeTarget(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"challenge","target":"nonsense","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for invalid challenge target, got nil")
	}
}

func TestParseFile_RejectsInvalidChallengeSeverity(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"challenge","severity":"disastrous","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for invalid challenge severity, got nil")
	}
}

func TestParseFile_RejectsAcceptWithChallengeOnlyFields(t *testing.T) {
	data := `{"schema_version": "1", "batch_id": "b1", "verified_by": "v1", "items": [{"node":"1","verdict":"accept","severity":"major","reason":"x"}]}`
	_, err := ParseFile([]byte(data))
	if err == nil {
		t.Fatal("expected error for accept item carrying a challenge-only field, got nil")
	}
}

func TestParseFile_RejectsDuplicateNodeInSameFile(t *testing.T) {
	data := `{
		"schema_version": "1",
		"batch_id": "b1",
		"verified_by": "v1",
		"items": [
			{"node": "1.1", "verdict": "accept", "reason": "first"},
			{"node": "1.1", "verdict": "challenge", "reason": "second, contradictory"}
		]
	}`
	_, err := ParseFile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "1.1") {
		t.Fatalf("expected duplicate-node error mentioning 1.1, got: %v", err)
	}
}

// TestParseFile_PreservesItemOrder verifies that Items are returned in file
// order — order-dependence for children-before-parent accepts (PRD C3)
// depends on the engine never reordering the file.
func TestParseFile_PreservesItemOrder(t *testing.T) {
	data := `{
		"schema_version": "1",
		"batch_id": "b1",
		"verified_by": "v1",
		"items": [
			{"node": "1.2", "verdict": "accept", "reason": "child first"},
			{"node": "1", "verdict": "accept", "reason": "parent second"}
		]
	}`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Items[0].Node != "1.2" || f.Items[1].Node != "1" {
		t.Fatalf("item order not preserved: got %q, %q", f.Items[0].Node, f.Items[1].Node)
	}
}
