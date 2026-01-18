//go:build integration

// Package main contains tests for the af log command.
// These are TDD tests - implementing the log command to view ledger events.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupLogTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupLogTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-log-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for log", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// setupLogTestWithEvents creates a test environment with multiple events.
func setupLogTestWithEvents(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, cleanup := setupLogTest(t)

	// Create a proof service and add some events
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Create a few nodes to generate events
	nodeID, err := service.ParseNodeID("1.1")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, service.NodeTypeClaim, "First child claim", service.InferenceModusPonens)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	nodeID, err = service.ParseNodeID("1.2")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, service.NodeTypeClaim, "Second child claim", service.InferenceModusPonens)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	// Accept a node
	nodeID, err = service.ParseNodeID("1")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	err = svc.AcceptNode(nodeID)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// executeLogCommand creates and executes a log command with the given arguments.
func executeLogCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newLogCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Basic Tests
// =============================================================================

// TestLogCmd_BasicOutput tests that log command displays events.
func TestLogCmd_BasicOutput(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should show at least the initialization event
	if !strings.Contains(output, "proof_initialized") && !strings.Contains(output, "ProofInitialized") {
		t.Errorf("expected output to contain proof initialization event, got: %q", output)
	}

	// Should contain sequence number
	if !strings.Contains(output, "#1") {
		t.Errorf("expected output to contain sequence number #1, got: %q", output)
	}
}

// TestLogCmd_MultipleEvents tests output with multiple events.
func TestLogCmd_MultipleEvents(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain multiple sequence numbers
	// We have: init, root node created, node 1.1 created, node 1.2 created, node validated
	if !strings.Contains(output, "#1") {
		t.Errorf("expected output to contain #1, got: %q", output)
	}
	if !strings.Contains(output, "#2") {
		t.Errorf("expected output to contain #2, got: %q", output)
	}
	if !strings.Contains(output, "#3") {
		t.Errorf("expected output to contain #3, got: %q", output)
	}
}

// TestLogCmd_EmptyLedger tests behavior with empty ledger.
func TestLogCmd_EmptyLedger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "af-log-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize directory structure but don't initialize proof
	if err := service.InitProofDir(tmpDir); err != nil {
		t.Fatal(err)
	}

	output, err := executeLogCommand(t, "-d", tmpDir)
	// Should not error, just show empty or "no events" message
	if err != nil {
		t.Logf("Command returned error for empty ledger: %v", err)
	}

	// Output should be empty or contain a message about no events
	if output != "" && !strings.Contains(strings.ToLower(output), "no") && !strings.Contains(strings.ToLower(output), "empty") {
		t.Logf("Output for empty ledger: %q", output)
	}
}

// =============================================================================
// Format Tests
// =============================================================================

// TestLogCmd_TextFormat tests text format output.
func TestLogCmd_TextFormat(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "text")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Text format should be human readable with sequence numbers
	if !strings.Contains(output, "#") {
		t.Errorf("text format should contain sequence marker #, got: %q", output)
	}

	// Should not be JSON
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(output), &jsonTest); err == nil {
		// If it parses as JSON, it might be a JSON array which is wrong for text format
		if strings.HasPrefix(strings.TrimSpace(output), "[") {
			t.Errorf("text format should not be JSON array, got: %q", output)
		}
	}
}

// TestLogCmd_JSONFormat tests JSON format output.
func TestLogCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be valid JSON array
	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Errorf("output should be valid JSON array: %v\nOutput: %q", err, output)
		return
	}

	// Should have at least one event
	if len(events) < 1 {
		t.Error("expected at least one event in JSON output")
		return
	}

	// Each event should have seq and type
	for i, event := range events {
		if _, ok := event["seq"]; !ok {
			t.Errorf("event %d missing 'seq' field", i)
		}
		if _, ok := event["type"]; !ok {
			t.Errorf("event %d missing 'type' field", i)
		}
	}
}

// TestLogCmd_JSONFormatMultipleEvents tests JSON with multiple events.
func TestLogCmd_JSONFormatMultipleEvents(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Errorf("output should be valid JSON array: %v", err)
		return
	}

	// Should have multiple events (init, root node, 2 child nodes, validation)
	if len(events) < 4 {
		t.Errorf("expected at least 4 events, got %d", len(events))
	}
}

// =============================================================================
// Filter Tests
// =============================================================================

// TestLogCmd_SinceFilter tests --since flag to filter by sequence number.
func TestLogCmd_SinceFilter(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Get all events first
	allOutput, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var allEvents []map[string]interface{}
	if err := json.Unmarshal([]byte(allOutput), &allEvents); err != nil {
		t.Fatalf("failed to parse all events: %v", err)
	}

	// Now get events since seq 3
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--since", "3")
	if err != nil {
		t.Fatalf("expected no error with --since, got: %v", err)
	}

	var filteredEvents []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &filteredEvents); err != nil {
		t.Fatalf("failed to parse filtered events: %v", err)
	}

	// Filtered should have fewer events
	if len(filteredEvents) >= len(allEvents) {
		t.Errorf("--since filter should reduce events: got %d filtered vs %d all", len(filteredEvents), len(allEvents))
	}

	// All filtered events should have seq > 3
	for _, event := range filteredEvents {
		seq, ok := event["seq"].(float64) // JSON numbers are float64
		if !ok {
			continue
		}
		if int(seq) <= 3 {
			t.Errorf("filtered event has seq %d which should be > 3", int(seq))
		}
	}
}

// TestLogCmd_SinceFilterText tests --since flag with text format.
func TestLogCmd_SinceFilterText(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "--since", "3")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should not contain early sequence numbers
	if strings.Contains(output, "#1 ") || strings.Contains(output, "#1\t") {
		t.Errorf("output should not contain #1 when --since 3, got: %q", output)
	}
	if strings.Contains(output, "#2 ") || strings.Contains(output, "#2\t") {
		t.Errorf("output should not contain #2 when --since 3, got: %q", output)
	}
}

// TestLogCmd_LimitFlag tests --limit flag.
func TestLogCmd_LimitFlag(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Limit to 2 events
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "-n", "2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected exactly 2 events with --limit 2, got %d", len(events))
	}
}

// TestLogCmd_LimitFlagText tests --limit with text format.
func TestLogCmd_LimitFlagText(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-n", "2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Count event lines (should have exactly 2 lines with #)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	eventLines := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			eventLines++
		}
	}

	if eventLines != 2 {
		t.Errorf("expected 2 event lines with --limit 2, got %d\nOutput: %q", eventLines, output)
	}
}

// TestLogCmd_LimitZero tests that limit 0 means unlimited.
func TestLogCmd_LimitZero(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Limit 0 should show all events
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "-n", "0")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse events: %v", err)
	}

	// Should have all events (at least 4)
	if len(events) < 4 {
		t.Errorf("limit 0 should show all events, got %d", len(events))
	}
}

// =============================================================================
// Reverse Tests
// =============================================================================

// TestLogCmd_ReverseFlag tests --reverse flag for newest-first ordering.
func TestLogCmd_ReverseFlag(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--reverse")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse events: %v", err)
	}

	if len(events) < 2 {
		t.Skip("need at least 2 events to test ordering")
	}

	// First event should have higher seq than last
	firstSeq, _ := events[0]["seq"].(float64)
	lastSeq, _ := events[len(events)-1]["seq"].(float64)

	if firstSeq < lastSeq {
		t.Errorf("--reverse should show newest first: first seq=%d, last seq=%d", int(firstSeq), int(lastSeq))
	}
}

// TestLogCmd_ReverseFlagText tests --reverse with text format.
func TestLogCmd_ReverseFlagText(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "--reverse")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Skip("need at least 2 lines to test ordering")
	}

	// First line should have higher sequence number than last
	// Lines format: "#N  EventType  ..."
	firstLine := lines[0]
	lastLine := lines[len(lines)-1]

	// Extract sequence numbers
	var firstSeq, lastSeq int
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			if firstSeq == 0 {
				fmt.Sscanf(strings.TrimSpace(line), "#%d", &firstSeq)
			}
			fmt.Sscanf(strings.TrimSpace(line), "#%d", &lastSeq)
		}
	}

	if firstSeq < lastSeq {
		t.Errorf("--reverse should show newest first\nFirst line: %s\nLast line: %s", firstLine, lastLine)
	}
}

// TestLogCmd_ReverseLimitCombined tests --reverse with --limit.
func TestLogCmd_ReverseLimitCombined(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Get 2 newest events
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--reverse", "-n", "2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// Both should be the latest events (highest seq numbers)
	if len(events) >= 2 {
		seq1, _ := events[0]["seq"].(float64)
		seq2, _ := events[1]["seq"].(float64)

		// In reverse order, first should have higher seq
		if seq1 < seq2 {
			t.Errorf("reverse order incorrect: first seq=%d should be > second seq=%d", int(seq1), int(seq2))
		}
	}
}

// =============================================================================
// Error Cases
// =============================================================================

// TestLogCmd_InvalidDirectory tests error for non-existent directory.
func TestLogCmd_InvalidDirectory(t *testing.T) {
	_, err := executeLogCommand(t, "-d", "/nonexistent/path/12345")

	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// TestLogCmd_NotADirectory tests error when path is not a directory.
func TestLogCmd_NotADirectory(t *testing.T) {
	// Create a temporary file (not directory)
	tmpFile, err := os.CreateTemp("", "af-log-test-file-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = executeLogCommand(t, "-d", tmpFile.Name())

	if err == nil {
		t.Error("expected error when path is a file not directory")
	}
}

// TestLogCmd_InvalidSinceValue tests error for invalid --since value.
func TestLogCmd_InvalidSinceValue(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	_, err := executeLogCommand(t, "-d", tmpDir, "--since", "abc")

	if err == nil {
		t.Error("expected error for invalid --since value")
	}
}

// TestLogCmd_NegativeSince tests behavior with negative --since value.
func TestLogCmd_NegativeSince(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	// Negative since should either error or be treated as 0/show all
	output, err := executeLogCommand(t, "-d", tmpDir, "--since", "-1", "-f", "json")

	// Either it errors, or it shows all events
	if err != nil {
		// Error is acceptable
		return
	}

	// If no error, should show events
	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err == nil {
		if len(events) < 1 {
			t.Error("negative --since with no error should show events")
		}
	}
}

// TestLogCmd_InvalidLimitValue tests error for invalid --limit value.
func TestLogCmd_InvalidLimitValue(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	_, err := executeLogCommand(t, "-d", tmpDir, "-n", "abc")

	if err == nil {
		t.Error("expected error for invalid --limit value")
	}
}

// TestLogCmd_InvalidFormat tests behavior with invalid format.
func TestLogCmd_InvalidFormat(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "invalid")

	// Should either error or fall back to text format
	if err != nil {
		// Error is acceptable for invalid format
		if !strings.Contains(strings.ToLower(err.Error()), "format") && !strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Logf("Error for invalid format: %v", err)
		}
		return
	}

	// If no error, output should be text format (not JSON)
	if strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Error("invalid format should not produce JSON output")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

// TestLogCmd_Help tests that help output shows usage information.
func TestLogCmd_Help(t *testing.T) {
	cmd := newLogCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()

	// Check for expected help content
	expectations := []string{
		"log",
		"--dir",
		"--format",
		"--since",
		"--limit",
		"--reverse",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// TestLogCmd_DirFlagShortForm tests -d short form.
func TestLogCmd_DirFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with -d flag, got: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output with -d flag")
	}
}

// TestLogCmd_DirFlagLongForm tests --dir long form.
func TestLogCmd_DirFlagLongForm(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "--dir", tmpDir)
	if err != nil {
		t.Fatalf("expected no error with --dir flag, got: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output with --dir flag")
	}
}

// TestLogCmd_FormatFlagShortForm tests -f short form.
func TestLogCmd_FormatFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -f flag, got: %v", err)
	}

	var events []interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Errorf("expected valid JSON with -f json, got: %v", err)
	}
}

// TestLogCmd_LimitFlagShortForm tests -n short form.
func TestLogCmd_LimitFlagShortForm(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-n", "1", "-f", "json")
	if err != nil {
		t.Fatalf("expected no error with -n flag, got: %v", err)
	}

	var events []interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 event with -n 1, got %d", len(events))
	}
}

// TestLogCmd_DefaultDirectory tests using current directory by default.
func TestLogCmd_DefaultDirectory(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	// Change to proof directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Execute without -d flag
	output, err := executeLogCommand(t)
	if err != nil {
		t.Fatalf("expected no error using current directory, got: %v\nOutput: %s", err, output)
	}

	// Should show events
	if !strings.Contains(output, "#1") {
		t.Errorf("expected events in output, got: %q", output)
	}
}

// =============================================================================
// Event Content Tests
// =============================================================================

// TestLogCmd_EventTypesDisplayed tests that different event types are shown.
func TestLogCmd_EventTypesDisplayed(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Collect event types
	types := make(map[string]bool)
	for _, event := range events {
		if eventType, ok := event["type"].(string); ok {
			types[eventType] = true
		}
	}

	// Should have proof_initialized and node_created events
	if !types["proof_initialized"] {
		t.Error("expected proof_initialized event type")
	}
	if !types["node_created"] {
		t.Error("expected node_created event type")
	}
	if !types["node_validated"] {
		t.Error("expected node_validated event type")
	}
}

// TestLogCmd_TimestampDisplayed tests that timestamps are included.
func TestLogCmd_TimestampDisplayed(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	// Wait a moment to ensure timestamp is recent
	time.Sleep(10 * time.Millisecond)

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	for i, event := range events {
		if _, ok := event["timestamp"]; !ok {
			t.Errorf("event %d missing 'timestamp' field", i)
		}
	}
}

// TestLogCmd_TextFormatContainsTimestamp tests text format includes timestamp.
func TestLogCmd_TextFormatContainsTimestamp(t *testing.T) {
	tmpDir, cleanup := setupLogTest(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should contain date-like pattern (YYYY-MM-DD or similar)
	if !strings.Contains(output, "202") { // Year should contain 202x
		t.Errorf("text format should contain timestamp/date, got: %q", output)
	}
}

// =============================================================================
// Combined Filter Tests
// =============================================================================

// TestLogCmd_SinceLimitCombined tests --since and --limit together.
func TestLogCmd_SinceLimitCombined(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Get events since seq 2 but limit to 2
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--since", "2", "-n", "2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have at most 2 events
	if len(events) > 2 {
		t.Errorf("expected at most 2 events, got %d", len(events))
	}

	// All events should have seq > 2
	for _, event := range events {
		if seq, ok := event["seq"].(float64); ok {
			if int(seq) <= 2 {
				t.Errorf("event seq %d should be > 2", int(seq))
			}
		}
	}
}

// TestLogCmd_SinceReverseCombined tests --since and --reverse together.
func TestLogCmd_SinceReverseCombined(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--since", "2", "--reverse")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// All events should have seq > 2
	for _, event := range events {
		if seq, ok := event["seq"].(float64); ok {
			if int(seq) <= 2 {
				t.Errorf("event seq %d should be > 2", int(seq))
			}
		}
	}

	// Should be in reverse order
	if len(events) >= 2 {
		firstSeq, _ := events[0]["seq"].(float64)
		lastSeq, _ := events[len(events)-1]["seq"].(float64)
		if firstSeq < lastSeq {
			t.Errorf("reverse order incorrect: first seq=%d should be > last seq=%d", int(firstSeq), int(lastSeq))
		}
	}
}

// TestLogCmd_AllFiltersCombined tests all filters together.
func TestLogCmd_AllFiltersCombined(t *testing.T) {
	tmpDir, cleanup := setupLogTestWithEvents(t)
	defer cleanup()

	// Get 2 newest events since seq 1
	output, err := executeLogCommand(t, "-d", tmpDir, "-f", "json", "--since", "1", "-n", "2", "--reverse")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var events []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &events); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have at most 2 events
	if len(events) > 2 {
		t.Errorf("expected at most 2 events, got %d", len(events))
	}

	// All should have seq > 1 (since excludes the specified seq)
	for _, event := range events {
		if seq, ok := event["seq"].(float64); ok {
			if int(seq) <= 1 {
				t.Errorf("event seq %d should be > 1", int(seq))
			}
		}
	}

	// Should be in reverse order
	if len(events) >= 2 {
		firstSeq, _ := events[0]["seq"].(float64)
		lastSeq, _ := events[len(events)-1]["seq"].(float64)
		if firstSeq < lastSeq {
			t.Errorf("reverse order incorrect")
		}
	}
}
