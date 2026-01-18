//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Challenge Severity Tests (TDD - vibefeld-cm6n)
// =============================================================================

// setupChallengeSeverityTest creates a test environment with an initialized proof.
func setupChallengeSeverityTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-challenge-severity-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// TestChallengeSeverity_DefaultsToMajor tests that the default severity is "major".
func TestChallengeSeverity_DefaultsToMajor(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-f", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	severity, ok := result["severity"].(string)
	if !ok {
		t.Fatal("severity field missing from output")
	}

	if severity != "major" {
		t.Errorf("expected default severity 'major', got %q", severity)
	}
}

// TestChallengeSeverity_ExplicitSeverity tests setting explicit severity levels.
func TestChallengeSeverity_ExplicitSeverity(t *testing.T) {
	severities := []string{"critical", "major", "minor", "note"}

	for _, sev := range severities {
		t.Run(sev, func(t *testing.T) {
			tmpDir, cleanup := setupChallengeSeverityTest(t)
			defer cleanup()

			cmd := newChallengeCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", sev, "-f", "json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v", err)
			}

			severity, ok := result["severity"].(string)
			if !ok {
				t.Fatal("severity field missing from output")
			}

			if severity != sev {
				t.Errorf("expected severity %q, got %q", sev, severity)
			}
		})
	}
}

// TestChallengeSeverity_InvalidSeverity tests error for invalid severity values.
func TestChallengeSeverity_InvalidSeverity(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	invalidSeverities := []string{"high", "low", "unknown", "CRITICAL", "Major"}

	for _, sev := range invalidSeverities {
		t.Run(sev, func(t *testing.T) {
			cmd := newChallengeCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", sev})

			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for invalid severity %q, got nil", sev)
			}
		})
	}
}

// TestChallengeSeverity_TextOutputShowsSeverity tests that text output shows severity.
func TestChallengeSeverity_TextOutputShowsSeverity(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", "critical"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "critical") {
		t.Errorf("expected output to contain severity 'critical', got: %s", output)
	}
	if !strings.Contains(output, "blocks acceptance") {
		t.Errorf("expected output to mention that challenge blocks acceptance, got: %s", output)
	}
}

// TestChallengeSeverity_MinorDoesNotBlock tests that minor challenges show non-blocking message.
func TestChallengeSeverity_MinorDoesNotBlock(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", "minor"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "minor") {
		t.Errorf("expected output to contain severity 'minor', got: %s", output)
	}
	if !strings.Contains(output, "does NOT block") {
		t.Errorf("expected output to mention that challenge does NOT block acceptance, got: %s", output)
	}
}

// TestChallengeSeverity_NoteDoesNotBlock tests that note challenges show non-blocking message.
func TestChallengeSeverity_NoteDoesNotBlock(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", "note"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "note") {
		t.Errorf("expected output to contain severity 'note', got: %s", output)
	}
	if !strings.Contains(output, "does NOT block") {
		t.Errorf("expected output to mention that challenge does NOT block acceptance, got: %s", output)
	}
}

// TestChallengeSeverity_StoredInState tests that severity is stored in state.
func TestChallengeSeverity_StoredInState(t *testing.T) {
	tmpDir, cleanup := setupChallengeSeverityTest(t)
	defer cleanup()

	// Raise a challenge with severity "minor"
	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-r", "Test reason", "-s", "minor"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify in state
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	challenges := st.AllChallenges()
	if len(challenges) == 0 {
		t.Fatal("expected at least one challenge")
	}

	found := false
	for _, c := range challenges {
		if c.Severity == "minor" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected challenge with severity 'minor' in state")
	}
}
