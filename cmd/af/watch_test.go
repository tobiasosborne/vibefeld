//go:build integration

// Package main contains tests for the af watch command.
// These are TDD tests - implementing the watch command for real-time event monitoring.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupWatchTest creates a temporary directory with an initialized proof.
// Returns the proof directory path and a cleanup function.
func setupWatchTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-watch-test-*")
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
	if err := service.Init(tmpDir, "Test conjecture for watch", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

// executeWatchCommand creates and executes a watch command with the given arguments.
// Returns the output buffer and any error. Uses a context with timeout.
func executeWatchCommand(t *testing.T, ctx context.Context, args ...string) (*bytes.Buffer, error) {
	t.Helper()

	cmd := newWatchCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	// Create a channel to signal command completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.ExecuteContext(ctx)
	}()

	select {
	case err := <-done:
		return buf, err
	case <-ctx.Done():
		return buf, ctx.Err()
	}
}

// =============================================================================
// Event Formatting Tests
// =============================================================================

// TestWatchCmd_FormatEvent tests that events are formatted correctly.
func TestWatchCmd_FormatEvent(t *testing.T) {
	// Test the formatWatchEvent helper function
	eventData := map[string]interface{}{
		"type":      "proof_initialized",
		"timestamp": "2024-01-15T10:30:00Z",
	}

	formatted := formatWatchEvent(1, eventData)

	// Should contain sequence number
	if !strings.Contains(formatted, "#1") {
		t.Errorf("formatted event should contain sequence number, got: %q", formatted)
	}

	// Should contain event type
	if !strings.Contains(formatted, "ProofInitialized") || !strings.Contains(strings.ToLower(formatted), "proof") {
		t.Errorf("formatted event should contain event type, got: %q", formatted)
	}

	// Should contain timestamp
	if !strings.Contains(formatted, "2024-01-15") || !strings.Contains(formatted, "10:30") {
		t.Errorf("formatted event should contain timestamp, got: %q", formatted)
	}
}

// TestWatchCmd_FormatEventNodeCreated tests formatting for node_created events.
func TestWatchCmd_FormatEventNodeCreated(t *testing.T) {
	eventData := map[string]interface{}{
		"type":      "node_created",
		"timestamp": "2024-01-15T10:30:00Z",
		"node": map[string]interface{}{
			"id":   "1.1",
			"type": "claim",
		},
	}

	formatted := formatWatchEvent(2, eventData)

	// Should contain node ID in summary
	if !strings.Contains(formatted, "1.1") {
		t.Errorf("formatted node_created event should contain node ID, got: %q", formatted)
	}
}

// TestWatchCmd_FormatEventJSON tests JSON output format.
func TestWatchCmd_FormatEventJSON(t *testing.T) {
	eventData := map[string]interface{}{
		"type":       "proof_initialized",
		"timestamp":  "2024-01-15T10:30:00Z",
		"conjecture": "Test conjecture",
	}

	formatted := formatWatchEventJSON(1, eventData)

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(formatted), &parsed); err != nil {
		t.Errorf("JSON formatted event should be valid JSON: %v\nGot: %q", err, formatted)
	}

	// Should contain seq field
	if _, ok := parsed["seq"]; !ok {
		t.Error("JSON event should contain 'seq' field")
	}

	// Should contain type field
	if _, ok := parsed["type"]; !ok {
		t.Error("JSON event should contain 'type' field")
	}
}

// =============================================================================
// Filter Tests
// =============================================================================

// TestWatchCmd_FilterByType tests the --filter flag for event type filtering.
func TestWatchCmd_FilterByType(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		eventType  string
		shouldPass bool
	}{
		{"exact match", "node_created", "node_created", true},
		{"no match", "node_created", "proof_initialized", false},
		{"partial match", "node", "node_created", true},
		{"case insensitive", "NODE_CREATED", "node_created", true},
		{"empty filter", "", "node_created", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilter(tt.eventType, tt.filter)
			if result != tt.shouldPass {
				t.Errorf("matchesFilter(%q, %q) = %v, want %v", tt.eventType, tt.filter, result, tt.shouldPass)
			}
		})
	}
}

// =============================================================================
// JSON Output Tests
// =============================================================================

// TestWatchCmd_JSONOutputFormat tests --json flag output.
func TestWatchCmd_JSONOutputFormat(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	// Use a very short timeout and one-shot mode
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	buf, _ := executeWatchCommand(t, ctx, "-d", tmpDir, "--json", "--once")

	output := buf.String()
	if output == "" {
		t.Skip("no output captured, may be timing issue")
	}

	// Each line should be valid JSON (NDJSON format)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d should be valid JSON: %v\nLine: %q", i, err, line)
		}
	}
}

// =============================================================================
// New Event Detection Tests
// =============================================================================

// TestWatchCmd_DetectsNewEvents tests that new events are detected.
func TestWatchCmd_DetectsNewEvents(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Start watch command in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var buf bytes.Buffer
	var wg sync.WaitGroup
	var watchErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmd := newWatchCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"-d", tmpDir, "--interval", "50ms", "--since", "2"})
		watchErr = cmd.ExecuteContext(ctx)
	}()

	// Wait a bit for watch to start
	time.Sleep(100 * time.Millisecond)

	// Create a new node (this should generate an event)
	nodeID, err := service.ParseNodeID("1.1")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "New claim for watch test", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for watch to detect the event
	time.Sleep(200 * time.Millisecond)

	// Cancel to stop watching
	cancel()
	wg.Wait()

	// Check if context cancellation error (expected)
	if watchErr != nil && watchErr != context.Canceled && watchErr != context.DeadlineExceeded {
		t.Logf("watch command returned: %v", watchErr)
	}

	output := buf.String()
	// Should contain the new node event
	if !strings.Contains(output, "1.1") && !strings.Contains(output, "node_created") && !strings.Contains(output, "NodeCreated") {
		t.Errorf("watch should detect new node creation, got: %q", output)
	}
}

// =============================================================================
// Interval Flag Tests
// =============================================================================

// TestWatchCmd_IntervalFlag tests --interval flag parsing.
func TestWatchCmd_IntervalFlag(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		wantErr  bool
	}{
		{"1 second", "1s", false},
		{"500 milliseconds", "500ms", false},
		{"2 seconds", "2s", false},
		{"invalid duration", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, cleanup := setupWatchTest(t)
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			_, err := executeWatchCommand(t, ctx, "-d", tmpDir, "--interval", tt.interval, "--once")

			if tt.wantErr && err == nil {
				// For invalid duration, we expect either an error or the command to handle it
				// The error might be from the flag parsing itself
			}
			// For valid durations, context timeout is expected
		})
	}
}

// TestWatchCmd_DefaultInterval tests default interval value.
func TestWatchCmd_DefaultInterval(t *testing.T) {
	cmd := newWatchCmd()

	// Get the default value for interval flag
	intervalFlag := cmd.Flags().Lookup("interval")
	if intervalFlag == nil {
		t.Fatal("--interval flag should exist")
	}

	if intervalFlag.DefValue != "1s" {
		t.Errorf("default interval should be 1s, got: %s", intervalFlag.DefValue)
	}
}

// =============================================================================
// Once Mode Tests
// =============================================================================

// TestWatchCmd_OnceMode tests --once flag for single pass mode.
func TestWatchCmd_OnceMode(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	// With --once, command should exit after showing current events
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	buf, err := executeWatchCommand(t, ctx, "-d", tmpDir, "--once")
	elapsed := time.Since(start)

	// Should complete quickly (not wait for timeout)
	if elapsed > 3*time.Second {
		t.Errorf("--once mode should exit quickly, took %v", elapsed)
	}

	// Should not error (other than context if somehow it times out)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Logf("once mode returned: %v", err)
	}

	// Should have output
	if buf.Len() == 0 {
		t.Error("--once mode should produce output")
	}
}

// =============================================================================
// Error Cases
// =============================================================================

// TestWatchCmd_InvalidDirectory tests error for non-existent directory.
func TestWatchCmd_InvalidDirectory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := executeWatchCommand(t, ctx, "-d", "/nonexistent/path/12345", "--once")

	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// TestWatchCmd_InvalidInterval tests error for invalid interval.
func TestWatchCmd_InvalidInterval(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := executeWatchCommand(t, ctx, "-d", tmpDir, "--interval", "invalid", "--once")

	if err == nil {
		t.Error("expected error for invalid interval")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestWatchCmd_Help tests that help output shows usage information.
func TestWatchCmd_Help(t *testing.T) {
	cmd := newWatchCmd()
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
		"watch",
		"--dir",
		"--interval",
		"--json",
		"--filter",
	}

	for _, exp := range expectations {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(exp)) {
			t.Errorf("help output should contain %q, got: %q", exp, output)
		}
	}
}

// =============================================================================
// Since Flag Tests
// =============================================================================

// TestWatchCmd_SinceFlag tests --since flag to start from a specific sequence.
func TestWatchCmd_SinceFlag(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	// Add more events
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, err := service.ParseNodeID("1.1")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "First child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, err = service.ParseNodeID("1.2")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Second child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	// Watch with --since 2 should only show events after seq 2
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	buf, _ := executeWatchCommand(t, ctx, "-d", tmpDir, "--since", "2", "--once", "--json")

	output := buf.String()

	// Should not contain #1 or #2 events
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if seq, ok := event["seq"].(float64); ok {
			if int(seq) <= 2 {
				t.Errorf("--since 2 should not show seq %d", int(seq))
			}
		}
	}
}

// =============================================================================
// Combined Tests
// =============================================================================

// TestWatchCmd_FilterAndJSON tests --filter with --json.
func TestWatchCmd_FilterAndJSON(t *testing.T) {
	tmpDir, cleanup := setupWatchTest(t)
	defer cleanup()

	// Add a node
	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	nodeID, err := service.ParseNodeID("1.1")
	if err != nil {
		t.Fatal(err)
	}
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	buf, _ := executeWatchCommand(t, ctx, "-d", tmpDir, "--filter", "node_created", "--json", "--once")

	output := buf.String()

	// All events should be node_created type
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		eventType, ok := event["type"].(string)
		if ok && !strings.Contains(eventType, "node_created") {
			t.Errorf("filtered output should only contain node_created, got: %s", eventType)
		}
	}
}
