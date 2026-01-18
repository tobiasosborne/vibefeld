//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Accept With Note Tests (TDD - vibefeld-cm6n)
// =============================================================================

// setupAcceptNoteTest creates a test environment with an initialized proof.
func setupAcceptNoteTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-accept-note-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Initialize the proof directory structure
	if err := fs.InitProofDir(tmpDir); err != nil {
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

// TestAcceptWithNote_TextOutput tests that text output shows the note.
func TestAcceptWithNote_TextOutput(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "--with-note", "Minor issue but acceptable"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Minor issue but acceptable") {
		t.Errorf("expected output to contain the note, got: %s", output)
	}
	if !strings.Contains(output, "validated") {
		t.Errorf("expected output to mention validation, got: %s", output)
	}
}

// TestAcceptWithNote_JSONOutput tests that JSON output includes the note.
func TestAcceptWithNote_JSONOutput(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "--with-note", "Minor issue but acceptable", "-f", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	note, ok := result["note"].(string)
	if !ok {
		t.Fatal("note field missing from output")
	}

	if note != "Minor issue but acceptable" {
		t.Errorf("expected note 'Minor issue but acceptable', got %q", note)
	}
}

// TestAcceptWithNote_NodeStillValidated tests that node is validated even with note.
func TestAcceptWithNote_NodeStillValidated(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "--with-note", "Minor issue"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify node is validated
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	nodeID, _ := service.ParseNodeID("1")
	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("node not found")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("expected EpistemicState = validated, got %q", n.EpistemicState)
	}
}

// TestAcceptWithNote_EmptyNote tests that empty note doesn't appear in output.
func TestAcceptWithNote_EmptyNote(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir, "-f", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Note should not be present when not provided
	if _, ok := result["note"]; ok {
		t.Error("note field should not be present when not provided")
	}
}

// TestAcceptWithNote_CannotUseWithAll tests that --with-note cannot be used with --all.
func TestAcceptWithNote_CannotUseWithAll(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-d", tmpDir, "--all", "--with-note", "Some note"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when using --with-note with --all")
	}

	if !strings.Contains(err.Error(), "single node") {
		t.Errorf("expected error about single node, got: %v", err)
	}
}

// TestAcceptWithNote_CannotUseWithMultipleNodes tests that --with-note cannot be used with multiple nodes.
func TestAcceptWithNote_CannotUseWithMultipleNodes(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	// Create additional nodes
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	node11, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(node11, schema.NodeTypeClaim, "Child node", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "1.1", "-d", tmpDir, "--with-note", "Some note"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when using --with-note with multiple nodes")
	}

	if !strings.Contains(err.Error(), "single node") {
		t.Errorf("expected error about single node, got: %v", err)
	}
}

// TestAcceptWithNote_WithoutNote tests that accept works without note.
func TestAcceptWithNote_WithoutNote(t *testing.T) {
	tmpDir, cleanup := setupAcceptNoteTest(t)
	defer cleanup()

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "-d", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "validated") {
		t.Errorf("expected output to mention validation, got: %s", output)
	}
}
