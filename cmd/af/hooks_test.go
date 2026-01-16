package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/hooks"
	"github.com/tobias/vibefeld/internal/service"
)

// setupHooksTest creates a temp directory with an initialized proof.
func setupHooksTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize proof
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}

	return tmpDir, func() {}
}

// TestHooksListCmd_Empty tests listing hooks when none are configured.
func TestHooksListCmd_Empty(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No hooks configured") {
		t.Errorf("expected 'No hooks configured' message, got: %s", output)
	}
}

// TestHooksListCmd_WithHooks tests listing hooks when some are configured.
func TestHooksListCmd_WithHooks(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	// Create config with hooks
	config := hooks.NewConfig()
	config.Hooks = []hooks.Hook{
		{
			ID:          "hook-1",
			EventType:   hooks.EventNodeCreated,
			HookType:    hooks.HookTypeWebhook,
			Target:      "https://example.com/webhook",
			Enabled:     true,
			Description: "Test webhook",
		},
		{
			ID:          "hook-2",
			EventType:   hooks.EventChallengeRaised,
			HookType:    hooks.HookTypeCommand,
			Target:      "echo 'Challenge raised'",
			Enabled:     false,
			Description: "Test command",
		},
	}
	if err := config.Save(dir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hook-1") {
		t.Errorf("expected hook-1 in output, got: %s", output)
	}
	if !strings.Contains(output, "hook-2") {
		t.Errorf("expected hook-2 in output, got: %s", output)
	}
	if !strings.Contains(output, "webhook") {
		t.Errorf("expected 'webhook' in output, got: %s", output)
	}
	if !strings.Contains(output, "command") {
		t.Errorf("expected 'command' in output, got: %s", output)
	}
}

// TestHooksListCmd_JSON tests listing hooks in JSON format.
func TestHooksListCmd_JSON(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	// Create config with a hook
	config := hooks.NewConfig()
	config.Hooks = []hooks.Hook{
		{
			ID:        "hook-1",
			EventType: hooks.EventNodeCreated,
			HookType:  hooks.HookTypeWebhook,
			Target:    "https://example.com/webhook",
			Enabled:   true,
		},
	}
	if err := config.Save(dir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--dir", dir, "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify JSON is valid
	var result []hooks.Hook
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, buf.String())
	}

	if len(result) != 1 {
		t.Errorf("expected 1 hook, got %d", len(result))
	}
	if result[0].ID != "hook-1" {
		t.Errorf("expected hook-1, got %s", result[0].ID)
	}
}

// TestHooksAddCmd_Webhook tests adding a webhook hook.
func TestHooksAddCmd_Webhook(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "node_created", "webhook", "https://example.com/hook", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hook added") {
		t.Errorf("expected 'Hook added' message, got: %s", output)
	}

	// Verify hook was saved
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(config.Hooks))
	}

	hook := config.Hooks[0]
	if hook.EventType != hooks.EventNodeCreated {
		t.Errorf("expected event_type node_created, got %s", hook.EventType)
	}
	if hook.HookType != hooks.HookTypeWebhook {
		t.Errorf("expected hook_type webhook, got %s", hook.HookType)
	}
	if hook.Target != "https://example.com/hook" {
		t.Errorf("expected target https://example.com/hook, got %s", hook.Target)
	}
	if !hook.Enabled {
		t.Error("expected hook to be enabled by default")
	}
}

// TestHooksAddCmd_Command tests adding a command hook.
func TestHooksAddCmd_Command(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "challenge_raised", "command", "echo 'test'", "--dir", dir, "--description", "Test hook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify hook was saved with description
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(config.Hooks))
	}

	hook := config.Hooks[0]
	if hook.EventType != hooks.EventChallengeRaised {
		t.Errorf("expected event_type challenge_raised, got %s", hook.EventType)
	}
	if hook.HookType != hooks.HookTypeCommand {
		t.Errorf("expected hook_type command, got %s", hook.HookType)
	}
	if hook.Description != "Test hook" {
		t.Errorf("expected description 'Test hook', got %s", hook.Description)
	}
}

// TestHooksAddCmd_InvalidEventType tests adding a hook with invalid event type.
func TestHooksAddCmd_InvalidEventType(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "invalid_event", "webhook", "https://example.com/hook", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid event type")
	}
	if !strings.Contains(err.Error(), "invalid event type") {
		t.Errorf("expected 'invalid event type' error, got: %v", err)
	}
}

// TestHooksAddCmd_InvalidHookType tests adding a hook with invalid hook type.
func TestHooksAddCmd_InvalidHookType(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "node_created", "invalid_type", "https://example.com/hook", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid hook type")
	}
	if !strings.Contains(err.Error(), "invalid hook type") {
		t.Errorf("expected 'invalid hook type' error, got: %v", err)
	}
}

// TestHooksAddCmd_InvalidWebhookURL tests adding a webhook with invalid URL.
func TestHooksAddCmd_InvalidWebhookURL(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "node_created", "webhook", "not-a-url", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "URL") {
		t.Errorf("expected URL-related error, got: %v", err)
	}
}

// TestHooksRemoveCmd tests removing a hook.
func TestHooksRemoveCmd(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	// Create config with hooks
	config := hooks.NewConfig()
	config.Hooks = []hooks.Hook{
		{
			ID:        "hook-1",
			EventType: hooks.EventNodeCreated,
			HookType:  hooks.HookTypeWebhook,
			Target:    "https://example.com/webhook",
			Enabled:   true,
		},
		{
			ID:        "hook-2",
			EventType: hooks.EventChallengeRaised,
			HookType:  hooks.HookTypeCommand,
			Target:    "echo test",
			Enabled:   true,
		},
	}
	if err := config.Save(dir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove", "hook-1", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hook removed") {
		t.Errorf("expected 'Hook removed' message, got: %s", output)
	}

	// Verify hook was removed
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Hooks) != 1 {
		t.Fatalf("expected 1 hook after removal, got %d", len(config.Hooks))
	}
	if config.Hooks[0].ID != "hook-2" {
		t.Errorf("expected hook-2 to remain, got %s", config.Hooks[0].ID)
	}
}

// TestHooksRemoveCmd_NotFound tests removing a non-existent hook.
func TestHooksRemoveCmd_NotFound(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove", "non-existent", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent hook")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// TestHooksTestCmd tests the test command with a webhook.
func TestHooksTestCmd(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	// Create a test server
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create config with hook pointing to test server
	config := hooks.NewConfig()
	config.Hooks = []hooks.Hook{
		{
			ID:        "test-hook",
			EventType: hooks.EventNodeCreated,
			HookType:  hooks.HookTypeWebhook,
			Target:    server.URL,
			Enabled:   true,
		},
	}
	if err := config.Save(dir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"test", "test-hook", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "success") && !strings.Contains(output, "OK") {
		t.Errorf("expected success message, got: %s", output)
	}

	if !received {
		t.Error("test server did not receive request")
	}
}

// TestHooksTestCmd_Command tests the test command with a shell command.
func TestHooksTestCmd_Command(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	// Create a temp file to verify command ran
	tmpFile := filepath.Join(dir, "test-output.txt")

	// Create config with command hook
	config := hooks.NewConfig()
	config.Hooks = []hooks.Hook{
		{
			ID:        "test-cmd",
			EventType: hooks.EventNodeValidated,
			HookType:  hooks.HookTypeCommand,
			Target:    "echo success > " + tmpFile,
			Enabled:   true,
		},
	}
	if err := config.Save(dir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"test", "test-cmd", "--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify command was executed
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("command did not create output file: %v", err)
	}

	if !strings.Contains(string(data), "success") {
		t.Errorf("expected 'success' in output file, got: %s", string(data))
	}
}

// TestHooksTestCmd_NotFound tests the test command with non-existent hook.
func TestHooksTestCmd_NotFound(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"test", "non-existent", "--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent hook")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// TestHooksCmd_NoProof tests hooks commands when proof is not initialized.
func TestHooksCmd_NoProof(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for uninitialized proof")
	}
}

// TestHooksAddCmd_Disabled tests adding a disabled hook.
func TestHooksAddCmd_Disabled(t *testing.T) {
	dir, cleanup := setupHooksTest(t)
	defer cleanup()

	cmd := newHooksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"add", "node_created", "webhook", "https://example.com/hook", "--dir", dir, "--disabled"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify hook was saved as disabled
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(config.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(config.Hooks))
	}

	if config.Hooks[0].Enabled {
		t.Error("expected hook to be disabled")
	}
}
