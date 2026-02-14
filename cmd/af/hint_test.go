package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupHintTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(dir, "Test conjecture", "prover1"); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestHintCmd_Success tests adding a hint to a node.
func TestHintCmd_Success(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir, "--text", "Consider Godement-Jacquet"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hint added") {
		t.Errorf("expected 'Hint added' in output, got: %s", output)
	}
}

// TestHintCmd_WithAgent tests adding a hint with an agent ID.
func TestHintCmd_WithAgent(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir, "--text", "Try induction", "--agent", "expert1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hint added") {
		t.Errorf("expected 'Hint added' in output, got: %s", output)
	}
}

// TestHintCmd_JSONOutput tests JSON format output.
func TestHintCmd_JSONOutput(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir, "--text", "Use indexing systems", "-f", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if result["status"] != "added" {
		t.Errorf("expected status 'added', got: %v", result["status"])
	}
	if result["node_id"] != "1" {
		t.Errorf("expected node_id '1', got: %v", result["node_id"])
	}
}

// TestHintCmd_NodeNotFound tests error for non-existent node.
func TestHintCmd_NodeNotFound(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.99", "--dir", dir, "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
}

// TestHintsCmd_Empty tests listing hints when none exist.
func TestHintsCmd_Empty(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No hints") {
		t.Errorf("expected 'No hints' in output, got: %s", output)
	}
}

// TestHintsCmd_AfterAdd tests listing hints after adding some.
func TestHintsCmd_AfterAdd(t *testing.T) {
	dir := setupHintTest(t)

	// Add two hints
	for _, text := range []string{"First hint", "Second hint"} {
		cmd := newHintCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"1", "--dir", dir, "--text", text, "--agent", "expert1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("failed to add hint: %v", err)
		}
	}

	// List hints
	cmd := newHintsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "2 total") {
		t.Errorf("expected '2 total' in output, got: %s", output)
	}
	if !strings.Contains(output, "First hint") {
		t.Errorf("expected 'First hint' in output, got: %s", output)
	}
	if !strings.Contains(output, "Second hint") {
		t.Errorf("expected 'Second hint' in output, got: %s", output)
	}
}

// TestHintsCmd_JSONOutput tests JSON format for hints listing.
func TestHintsCmd_JSONOutput(t *testing.T) {
	dir := setupHintTest(t)

	// Add a hint
	addCmd := newHintCmd()
	addCmd.SetOut(new(bytes.Buffer))
	addCmd.SetErr(new(bytes.Buffer))
	addCmd.SetArgs([]string{"1", "--dir", dir, "--text", "Test hint", "--agent", "expert1"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("failed to add hint: %v", err)
	}

	// List in JSON
	cmd := newHintsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1", "--dir", dir, "-f", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if result["count"].(float64) != 1 {
		t.Errorf("expected count 1, got: %v", result["count"])
	}
	hints := result["hints"].([]interface{})
	if len(hints) != 1 {
		t.Errorf("expected 1 hint, got: %d", len(hints))
	}
}

// TestHintsCmd_NodeNotFound tests error for non-existent node.
func TestHintsCmd_NodeNotFound(t *testing.T) {
	dir := setupHintTest(t)

	cmd := newHintsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"1.99", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
