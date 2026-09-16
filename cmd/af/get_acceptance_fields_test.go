//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

// TestGetShowsAcceptanceAndClaimFields is the D11/0l3d guard: `af get -f json`
// surfaces the recorded acceptance provenance and the claim owner/times.
func TestGetShowsAcceptanceAndClaimFields(t *testing.T) {
	dir := t.TempDir()
	if err := service.Init(dir, "Conjecture", "author"); err != nil {
		t.Fatalf("init: %v", err)
	}

	run := func(args ...string) string {
		t.Helper()
		cmd := newTestRootCmd()
		cmd.AddCommand(newClaimCmd(), newAcceptCmd(), newGetCmd(), newVerdictsCmd())
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return buf.String()
	}

	run("claim", "1", "--owner", "verifier-1", "--role", "verifier", "-d", dir)

	var claimed map[string]interface{}
	if err := json.Unmarshal([]byte(run("get", "1", "-f", "json", "-d", dir)), &claimed); err != nil {
		t.Fatalf("get JSON: %v", err)
	}
	if claimed["claimed_by"] != "verifier-1" {
		t.Errorf("claimed_by = %v, want verifier-1", claimed["claimed_by"])
	}
	if claimed["claimed_at"] == nil || claimed["claimed_at"] == "" {
		t.Errorf("expected claimed_at in get JSON, got %v", claimed["claimed_at"])
	}
	if claimed["expires_at"] == nil || claimed["expires_at"] == "" {
		t.Errorf("expected expires_at in get JSON, got %v", claimed["expires_at"])
	}

	// Apply the acceptance as a BATCHED verdict (af verdicts apply), so the
	// recorded validation_batch_id is non-empty and must be surfaced exactly.
	const batchID = "batch-d11-0l3d"
	verdictsPath := filepath.Join(dir, "verdicts.json")
	verdictsJSON := `{
		"schema_version": "1",
		"batch_id": "` + batchID + `",
		"verified_by": "verifier-1",
		"items": [
			{"node": "1", "verdict": "accept", "reason": "checked in batch"}
		]
	}`
	if err := os.WriteFile(verdictsPath, []byte(verdictsJSON), 0o644); err != nil {
		t.Fatalf("write verdicts: %v", err)
	}
	run("verdicts", "apply", verdictsPath, "-d", dir)

	var accepted map[string]interface{}
	if err := json.Unmarshal([]byte(run("get", "1", "-f", "json", "-d", dir)), &accepted); err != nil {
		t.Fatalf("get JSON after accept: %v", err)
	}
	if accepted["validated_by"] != "verifier-1" {
		t.Errorf("validated_by = %v, want verifier-1", accepted["validated_by"])
	}
	if got := accepted["validation_batch_id"]; got != batchID {
		t.Errorf("validation_batch_id = %v, want %q", got, batchID)
	}
}
