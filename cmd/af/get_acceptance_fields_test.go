//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
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
		cmd.AddCommand(newClaimCmd(), newAcceptCmd(), newGetCmd())
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

	run("accept", "1", "--agent", "verifier-1", "--with-note", "checked", "--confirm", "-d", dir)

	var accepted map[string]interface{}
	if err := json.Unmarshal([]byte(run("get", "1", "-f", "json", "-d", dir)), &accepted); err != nil {
		t.Fatalf("get JSON after accept: %v", err)
	}
	if accepted["validated_by"] != "verifier-1" {
		t.Errorf("validated_by = %v, want verifier-1", accepted["validated_by"])
	}
	if _, ok := accepted["validation_batch_id"]; !ok {
		// Empty batch id is dropped by the field's omitempty; the important
		// thing is that validated_by is present.
		t.Logf("validation_batch_id omitted (empty for a single accept), which is expected")
	}
}
