package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func TestHandoff_NoProof(t *testing.T) {
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dir", dir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No proof initialized") {
		t.Errorf("expected 'No proof initialized', got: %s", output)
	}
}

func TestHandoff_NoProofJSON(t *testing.T) {
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}
	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dir", dir, "--format", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != `{"error":"proof not initialized"}` {
		t.Errorf("expected JSON error, got: %s", output)
	}
}

func TestHandoff_BasicProof(t *testing.T) {
	dir := t.TempDir()

	err := service.Init(dir, "All primes > 2 are odd", "test-author")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dir", dir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check key sections
	if !strings.Contains(output, "Handoff Report") {
		t.Error("missing 'Handoff Report' header")
	}
	if !strings.Contains(output, "All primes > 2 are odd") {
		t.Error("missing conjecture")
	}
	if !strings.Contains(output, "Completion:") {
		t.Error("missing completion line")
	}
	if !strings.Contains(output, "Next Steps:") {
		t.Error("missing next steps section")
	}
	if !strings.Contains(output, "Open Challenges: none") {
		t.Error("missing challenges section")
	}
}

func TestHandoff_BasicProofJSON(t *testing.T) {
	dir := t.TempDir()

	err := service.Init(dir, "sqrt(2) is irrational", "test-author")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dir", dir, "--format", "json"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report HandoffReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if report.Conjecture != "sqrt(2) is irrational" {
		t.Errorf("conjecture = %q, want %q", report.Conjecture, "sqrt(2) is irrational")
	}
	if report.TotalNodes != 1 {
		t.Errorf("total_nodes = %d, want 1", report.TotalNodes)
	}
	if len(report.NextSteps) == 0 {
		t.Error("expected at least one next step")
	}
}

func TestHandoff_WithSince(t *testing.T) {
	dir := t.TempDir()

	err := service.Init(dir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--dir", dir, "--since", "1"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Init creates 2 events (proof_initialized + node_created).
	// --since 1 should show event #2 (node_created).
	if !strings.Contains(output, "Recent Activity") {
		t.Error("missing recent activity section")
	}
	if !strings.Contains(output, "Created node") {
		t.Error("expected node_created event in recent activity")
	}
}

func TestHandoff_InvalidFormat(t *testing.T) {
	dir := t.TempDir()

	err := service.Init(dir, "Test", "author")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	cmd := newHandoffCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--dir", dir, "--format", "xml"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected 'invalid format' error, got: %v", err)
	}
}
