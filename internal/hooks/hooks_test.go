// Package hooks provides webhook and command hook support for external integrations.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// TestHookType_Validation tests hook type validation.
func TestHookType_Validation(t *testing.T) {
	tests := []struct {
		name    string
		hookType HookType
		valid   bool
	}{
		{"webhook valid", HookTypeWebhook, true},
		{"command valid", HookTypeCommand, true},
		{"empty invalid", HookType(""), false},
		{"unknown invalid", HookType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHookType(tt.hookType)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for invalid hook type")
			}
		})
	}
}

// TestEventType_Validation tests event type validation.
func TestEventType_Validation(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		valid     bool
	}{
		{"node_created valid", EventNodeCreated, true},
		{"node_validated valid", EventNodeValidated, true},
		{"challenge_raised valid", EventChallengeRaised, true},
		{"challenge_resolved valid", EventChallengeResolved, true},
		{"empty invalid", EventType(""), false},
		{"unknown invalid", EventType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEventType(tt.eventType)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for invalid event type")
			}
		})
	}
}

// TestHookConfig_Validation tests hook configuration validation.
func TestHookConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		hook   Hook
		valid  bool
		errMsg string
	}{
		{
			name: "valid webhook",
			hook: Hook{
				ID:        "hook-1",
				EventType: EventNodeCreated,
				HookType:  HookTypeWebhook,
				Target:    "https://example.com/webhook",
				Enabled:   true,
			},
			valid: true,
		},
		{
			name: "valid command",
			hook: Hook{
				ID:        "hook-2",
				EventType: EventNodeValidated,
				HookType:  HookTypeCommand,
				Target:    "notify-send 'Node validated'",
				Enabled:   true,
			},
			valid: true,
		},
		{
			name: "missing ID",
			hook: Hook{
				EventType: EventNodeCreated,
				HookType:  HookTypeWebhook,
				Target:    "https://example.com/webhook",
			},
			valid:  false,
			errMsg: "ID",
		},
		{
			name: "invalid event type",
			hook: Hook{
				ID:        "hook-3",
				EventType: EventType("invalid"),
				HookType:  HookTypeWebhook,
				Target:    "https://example.com/webhook",
			},
			valid:  false,
			errMsg: "event type",
		},
		{
			name: "invalid hook type",
			hook: Hook{
				ID:        "hook-4",
				EventType: EventNodeCreated,
				HookType:  HookType("invalid"),
				Target:    "https://example.com/webhook",
			},
			valid:  false,
			errMsg: "hook type",
		},
		{
			name: "missing target",
			hook: Hook{
				ID:        "hook-5",
				EventType: EventNodeCreated,
				HookType:  HookTypeWebhook,
				Target:    "",
			},
			valid:  false,
			errMsg: "target",
		},
		{
			name: "webhook with invalid URL",
			hook: Hook{
				ID:        "hook-6",
				EventType: EventNodeCreated,
				HookType:  HookTypeWebhook,
				Target:    "not-a-url",
			},
			valid:  false,
			errMsg: "URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hook.Validate()
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid {
				if err == nil {
					t.Errorf("expected error for invalid hook config")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			}
		})
	}
}

// TestHookConfig_LoadSave tests loading and saving hook configuration.
func TestHookConfig_LoadSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	afDir := filepath.Join(tmpDir, ".af")
	if err := os.MkdirAll(afDir, 0755); err != nil {
		t.Fatalf("failed to create .af directory: %v", err)
	}

	// Create a config
	config := &Config{
		Version: "1.0",
		Hooks: []Hook{
			{
				ID:        "hook-1",
				EventType: EventNodeCreated,
				HookType:  HookTypeWebhook,
				Target:    "https://example.com/webhook",
				Enabled:   true,
			},
			{
				ID:        "hook-2",
				EventType: EventChallengeRaised,
				HookType:  HookTypeCommand,
				Target:    "echo 'Challenge raised'",
				Enabled:   true,
			},
		},
	}

	// Save config
	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	hooksFile := filepath.Join(afDir, "hooks.json")
	if _, err := os.Stat(hooksFile); os.IsNotExist(err) {
		t.Fatal("hooks.json file not created")
	}

	// Load config
	loaded, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded config matches
	if loaded.Version != config.Version {
		t.Errorf("version mismatch: got %q, want %q", loaded.Version, config.Version)
	}
	if len(loaded.Hooks) != len(config.Hooks) {
		t.Errorf("hooks count mismatch: got %d, want %d", len(loaded.Hooks), len(config.Hooks))
	}
	for i, hook := range loaded.Hooks {
		if hook.ID != config.Hooks[i].ID {
			t.Errorf("hook[%d] ID mismatch: got %q, want %q", i, hook.ID, config.Hooks[i].ID)
		}
		if hook.EventType != config.Hooks[i].EventType {
			t.Errorf("hook[%d] EventType mismatch: got %q, want %q", i, hook.EventType, config.Hooks[i].EventType)
		}
		if hook.HookType != config.Hooks[i].HookType {
			t.Errorf("hook[%d] HookType mismatch: got %q, want %q", i, hook.HookType, config.Hooks[i].HookType)
		}
		if hook.Target != config.Hooks[i].Target {
			t.Errorf("hook[%d] Target mismatch: got %q, want %q", i, hook.Target, config.Hooks[i].Target)
		}
	}
}

// TestHookConfig_LoadEmpty tests loading config when file doesn't exist.
func TestHookConfig_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	afDir := filepath.Join(tmpDir, ".af")
	if err := os.MkdirAll(afDir, 0755); err != nil {
		t.Fatalf("failed to create .af directory: %v", err)
	}

	// Load should return empty config (not error) when file doesn't exist
	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(config.Hooks) != 0 {
		t.Errorf("expected empty hooks slice, got %d hooks", len(config.Hooks))
	}
}

// TestConfig_AddHook tests adding hooks to config.
func TestConfig_AddHook(t *testing.T) {
	config := NewConfig()

	hook := Hook{
		ID:        "hook-1",
		EventType: EventNodeCreated,
		HookType:  HookTypeWebhook,
		Target:    "https://example.com/webhook",
		Enabled:   true,
	}

	// Add hook
	config.AddHook(hook)

	if len(config.Hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(config.Hooks))
	}
	if config.Hooks[0].ID != hook.ID {
		t.Errorf("hook ID mismatch")
	}
}

// TestConfig_RemoveHook tests removing hooks from config.
func TestConfig_RemoveHook(t *testing.T) {
	config := NewConfig()
	config.Hooks = []Hook{
		{ID: "hook-1", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: "https://example.com/1"},
		{ID: "hook-2", EventType: EventNodeValidated, HookType: HookTypeWebhook, Target: "https://example.com/2"},
		{ID: "hook-3", EventType: EventChallengeRaised, HookType: HookTypeCommand, Target: "echo test"},
	}

	// Remove middle hook
	removed := config.RemoveHook("hook-2")
	if !removed {
		t.Error("expected RemoveHook to return true")
	}
	if len(config.Hooks) != 2 {
		t.Errorf("expected 2 hooks after removal, got %d", len(config.Hooks))
	}

	// Verify remaining hooks
	if config.Hooks[0].ID != "hook-1" || config.Hooks[1].ID != "hook-3" {
		t.Error("wrong hooks remaining after removal")
	}

	// Try to remove non-existent hook
	removed = config.RemoveHook("hook-99")
	if removed {
		t.Error("expected RemoveHook to return false for non-existent hook")
	}
}

// TestConfig_GetHook tests getting hook by ID.
func TestConfig_GetHook(t *testing.T) {
	config := NewConfig()
	config.Hooks = []Hook{
		{ID: "hook-1", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: "https://example.com/1"},
		{ID: "hook-2", EventType: EventNodeValidated, HookType: HookTypeWebhook, Target: "https://example.com/2"},
	}

	// Get existing hook
	hook, found := config.GetHook("hook-2")
	if !found {
		t.Error("expected to find hook-2")
	}
	if hook.ID != "hook-2" {
		t.Errorf("expected hook-2, got %s", hook.ID)
	}

	// Get non-existent hook
	_, found = config.GetHook("hook-99")
	if found {
		t.Error("expected not to find hook-99")
	}
}

// TestConfig_GetHooksForEvent tests filtering hooks by event type.
func TestConfig_GetHooksForEvent(t *testing.T) {
	config := NewConfig()
	config.Hooks = []Hook{
		{ID: "hook-1", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: "https://example.com/1", Enabled: true},
		{ID: "hook-2", EventType: EventNodeCreated, HookType: HookTypeCommand, Target: "echo test", Enabled: true},
		{ID: "hook-3", EventType: EventNodeValidated, HookType: HookTypeWebhook, Target: "https://example.com/2", Enabled: true},
		{ID: "hook-4", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: "https://example.com/3", Enabled: false},
	}

	hooks := config.GetHooksForEvent(EventNodeCreated)
	if len(hooks) != 2 {
		t.Errorf("expected 2 enabled hooks for node_created, got %d", len(hooks))
	}

	// Verify the returned hooks are correct
	ids := make(map[string]bool)
	for _, h := range hooks {
		ids[h.ID] = true
	}
	if !ids["hook-1"] || !ids["hook-2"] {
		t.Error("expected hook-1 and hook-2")
	}
	if ids["hook-4"] {
		t.Error("disabled hook should not be returned")
	}
}

// TestEventPayload_Creation tests creating event payloads.
func TestEventPayload_Creation(t *testing.T) {
	nodeID, _ := types.Parse("1.2.3")

	payload := NewEventPayload(EventNodeCreated, nodeID)

	if payload.Event != EventNodeCreated {
		t.Errorf("expected event %s, got %s", EventNodeCreated, payload.Event)
	}
	if payload.NodeID != "1.2.3" {
		t.Errorf("expected node ID 1.2.3, got %s", payload.NodeID)
	}
	if payload.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Test JSON marshaling
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var decoded EventPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if decoded.Event != payload.Event {
		t.Errorf("event mismatch after round-trip")
	}
}

// TestExecutor_Webhook tests webhook execution (mocked).
func TestExecutor_Webhook(t *testing.T) {
	// Create a test server
	var receivedBody []byte
	var receivedContentType string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedContentType = r.Header.Get("Content-Type")
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		receivedBody = buf.Bytes()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hook := Hook{
		ID:        "test-hook",
		EventType: EventNodeCreated,
		HookType:  HookTypeWebhook,
		Target:    server.URL,
		Enabled:   true,
	}

	nodeID, _ := types.Parse("1.2")
	payload := NewEventPayload(EventNodeCreated, nodeID)

	executor := NewExecutor()
	err := executor.ExecuteWebhook(context.Background(), hook, payload)
	if err != nil {
		t.Fatalf("webhook execution failed: %v", err)
	}

	// Verify the request
	mu.Lock()
	defer mu.Unlock()

	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}

	var receivedPayload EventPayload
	if err := json.Unmarshal(receivedBody, &receivedPayload); err != nil {
		t.Fatalf("failed to unmarshal received body: %v", err)
	}
	if receivedPayload.Event != EventNodeCreated {
		t.Errorf("expected event %s, got %s", EventNodeCreated, receivedPayload.Event)
	}
	if receivedPayload.NodeID != "1.2" {
		t.Errorf("expected node ID 1.2, got %s", receivedPayload.NodeID)
	}
}

// TestExecutor_WebhookError tests webhook execution with errors.
func TestExecutor_WebhookError(t *testing.T) {
	// Create a server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hook := Hook{
		ID:        "test-hook",
		EventType: EventNodeCreated,
		HookType:  HookTypeWebhook,
		Target:    server.URL,
		Enabled:   true,
	}

	nodeID, _ := types.Parse("1")
	payload := NewEventPayload(EventNodeCreated, nodeID)

	executor := NewExecutor()
	err := executor.ExecuteWebhook(context.Background(), hook, payload)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// TestExecutor_Command tests command execution (mocked).
func TestExecutor_Command(t *testing.T) {
	// Create a temp file to verify command ran
	tmpFile := filepath.Join(t.TempDir(), "output.txt")

	hook := Hook{
		ID:        "test-hook",
		EventType: EventNodeCreated,
		HookType:  HookTypeCommand,
		Target:    fmt.Sprintf("echo $AF_EVENT_TYPE > %s", tmpFile),
		Enabled:   true,
	}

	nodeID, _ := types.Parse("1.2.3")
	payload := NewEventPayload(EventNodeCreated, nodeID)

	executor := NewExecutor()
	err := executor.ExecuteCommand(context.Background(), hook, payload)
	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	// Give some time for async execution
	time.Sleep(100 * time.Millisecond)

	// Read the output file
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	output := strings.TrimSpace(string(data))
	if output != string(EventNodeCreated) {
		t.Errorf("expected output %q, got %q", EventNodeCreated, output)
	}
}

// TestExecutor_CommandEnvVars tests that command execution sets correct env vars.
func TestExecutor_CommandEnvVars(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "env.txt")

	// Command that outputs all AF_ env vars
	hook := Hook{
		ID:        "test-hook",
		EventType: EventChallengeRaised,
		HookType:  HookTypeCommand,
		Target:    fmt.Sprintf("env | grep AF_ > %s", tmpFile),
		Enabled:   true,
	}

	nodeID, _ := types.Parse("1.2")
	payload := NewEventPayload(EventChallengeRaised, nodeID)
	payload.ChallengeID = "ch-123"
	payload.Details = map[string]string{"target": "gap", "reason": "Missing step"}

	executor := NewExecutor()
	err := executor.ExecuteCommand(context.Background(), hook, payload)
	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "AF_EVENT_TYPE=challenge_raised") {
		t.Error("missing AF_EVENT_TYPE env var")
	}
	if !strings.Contains(output, "AF_NODE_ID=1.2") {
		t.Error("missing AF_NODE_ID env var")
	}
	if !strings.Contains(output, "AF_CHALLENGE_ID=ch-123") {
		t.Error("missing AF_CHALLENGE_ID env var")
	}
}

// TestHookManager_FireAsync tests async hook firing.
func TestHookManager_FireAsync(t *testing.T) {
	var received []EventPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload EventPayload
		json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	afDir := filepath.Join(tmpDir, ".af")
	os.MkdirAll(afDir, 0755)

	config := NewConfig()
	config.Hooks = []Hook{
		{ID: "hook-1", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: server.URL, Enabled: true},
	}
	config.Save(tmpDir)

	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	nodeID, _ := types.Parse("1.1")
	payload := NewEventPayload(EventNodeCreated, nodeID)

	// Fire async
	manager.Fire(EventNodeCreated, payload)

	// Wait for async execution
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 webhook call, got %d", count)
	}
}

// TestHookManager_NoBlockMain tests that hooks don't block main operation.
func TestHookManager_NoBlockMain(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	afDir := filepath.Join(tmpDir, ".af")
	os.MkdirAll(afDir, 0755)

	config := NewConfig()
	config.Hooks = []Hook{
		{ID: "hook-1", EventType: EventNodeCreated, HookType: HookTypeWebhook, Target: server.URL, Enabled: true},
	}
	config.Save(tmpDir)

	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	nodeID, _ := types.Parse("1")
	payload := NewEventPayload(EventNodeCreated, nodeID)

	// Fire should return immediately
	start := time.Now()
	manager.Fire(EventNodeCreated, payload)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("Fire() blocked for %v, expected non-blocking", elapsed)
	}
}

// TestGenerateHookID tests hook ID generation.
func TestGenerateHookID(t *testing.T) {
	id1 := GenerateHookID()
	id2 := GenerateHookID()

	if id1 == "" {
		t.Error("generated ID should not be empty")
	}
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
	if !strings.HasPrefix(id1, "hook-") {
		t.Errorf("expected ID to start with 'hook-', got %s", id1)
	}
}

// TestAllEventTypes returns all valid event types.
func TestAllEventTypes(t *testing.T) {
	types := AllEventTypes()
	if len(types) != 4 {
		t.Errorf("expected 4 event types, got %d", len(types))
	}

	// Verify all expected types are present
	expected := map[EventType]bool{
		EventNodeCreated:      false,
		EventNodeValidated:    false,
		EventChallengeRaised:  false,
		EventChallengeResolved: false,
	}
	for _, et := range types {
		expected[et] = true
	}
	for et, found := range expected {
		if !found {
			t.Errorf("missing event type: %s", et)
		}
	}
}

// TestAllHookTypes returns all valid hook types.
func TestAllHookTypes(t *testing.T) {
	types := AllHookTypes()
	if len(types) != 2 {
		t.Errorf("expected 2 hook types, got %d", len(types))
	}
}
