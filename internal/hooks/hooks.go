// Package hooks provides webhook and command hook support for external integrations.
// Hooks are triggered asynchronously when events occur in the proof system,
// allowing external systems to react to proof state changes.
package hooks

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tobias/vibefeld/internal/types"
)

// HookType represents the type of hook.
type HookType string

const (
	// HookTypeWebhook sends an HTTP POST request with event JSON payload.
	HookTypeWebhook HookType = "webhook"

	// HookTypeCommand executes a shell command with event data as env vars.
	HookTypeCommand HookType = "command"
)

// hookTypeRegistry maps valid hook types to their descriptions.
var hookTypeRegistry = map[HookType]string{
	HookTypeWebhook: "HTTP POST to URL with event JSON payload",
	HookTypeCommand: "Execute shell command with event data as env vars",
}

// ValidateHookType validates a hook type.
func ValidateHookType(ht HookType) error {
	if _, exists := hookTypeRegistry[ht]; !exists {
		return fmt.Errorf("invalid hook type: %q", ht)
	}
	return nil
}

// AllHookTypes returns all valid hook types sorted alphabetically.
func AllHookTypes() []HookType {
	result := make([]HookType, 0, len(hookTypeRegistry))
	for ht := range hookTypeRegistry {
		result = append(result, ht)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// EventType represents the type of event that can trigger hooks.
type EventType string

const (
	// EventNodeCreated is fired when a new node is added to the proof tree.
	EventNodeCreated EventType = "node_created"

	// EventNodeValidated is fired when a verifier validates a node.
	EventNodeValidated EventType = "node_validated"

	// EventChallengeRaised is fired when a verifier raises a challenge.
	EventChallengeRaised EventType = "challenge_raised"

	// EventChallengeResolved is fired when a challenge is resolved.
	EventChallengeResolved EventType = "challenge_resolved"
)

// eventTypeRegistry maps valid event types to their descriptions.
var eventTypeRegistry = map[EventType]string{
	EventNodeCreated:       "Fired when a new node is added to the proof tree",
	EventNodeValidated:     "Fired when a verifier validates a node",
	EventChallengeRaised:   "Fired when a verifier raises a challenge",
	EventChallengeResolved: "Fired when a challenge is resolved",
}

// ValidateEventType validates an event type.
func ValidateEventType(et EventType) error {
	if _, exists := eventTypeRegistry[et]; !exists {
		return fmt.Errorf("invalid event type: %q", et)
	}
	return nil
}

// AllEventTypes returns all valid event types sorted alphabetically.
func AllEventTypes() []EventType {
	result := make([]EventType, 0, len(eventTypeRegistry))
	for et := range eventTypeRegistry {
		result = append(result, et)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// Hook represents a configured hook.
type Hook struct {
	// ID is the unique identifier for this hook.
	ID string `json:"id"`

	// EventType is the event that triggers this hook.
	EventType EventType `json:"event_type"`

	// HookType is the type of hook (webhook or command).
	HookType HookType `json:"hook_type"`

	// Target is the webhook URL or command to execute.
	Target string `json:"target"`

	// Enabled indicates whether this hook is active.
	Enabled bool `json:"enabled"`

	// Description is an optional human-readable description.
	Description string `json:"description,omitempty"`

	// CreatedAt is when the hook was created.
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Validate validates the hook configuration.
func (h *Hook) Validate() error {
	if h.ID == "" {
		return errors.New("hook ID cannot be empty")
	}

	if err := ValidateEventType(h.EventType); err != nil {
		return err
	}

	if err := ValidateHookType(h.HookType); err != nil {
		return err
	}

	if h.Target == "" {
		return errors.New("hook target cannot be empty")
	}

	// Validate URL for webhooks
	if h.HookType == HookTypeWebhook {
		_, err := url.ParseRequestURI(h.Target)
		if err != nil {
			return fmt.Errorf("invalid webhook URL: %w", err)
		}
	}

	return nil
}

// Config holds the hooks configuration for a proof directory.
type Config struct {
	// Version is the schema version.
	Version string `json:"version"`

	// Hooks is the list of configured hooks.
	Hooks []Hook `json:"hooks"`
}

// hooksFilename is the name of the hooks config file.
const hooksFilename = "hooks.json"

// NewConfig creates a new empty config.
func NewConfig() *Config {
	return &Config{
		Version: "1.0",
		Hooks:   make([]Hook, 0),
	}
}

// AddHook adds a hook to the config.
func (c *Config) AddHook(hook Hook) {
	c.Hooks = append(c.Hooks, hook)
}

// RemoveHook removes a hook by ID. Returns true if found and removed.
func (c *Config) RemoveHook(id string) bool {
	for i, h := range c.Hooks {
		if h.ID == id {
			c.Hooks = append(c.Hooks[:i], c.Hooks[i+1:]...)
			return true
		}
	}
	return false
}

// GetHook returns a hook by ID.
func (c *Config) GetHook(id string) (Hook, bool) {
	for _, h := range c.Hooks {
		if h.ID == id {
			return h, true
		}
	}
	return Hook{}, false
}

// GetHooksForEvent returns all enabled hooks for the given event type.
func (c *Config) GetHooksForEvent(eventType EventType) []Hook {
	var result []Hook
	for _, h := range c.Hooks {
		if h.EventType == eventType && h.Enabled {
			result = append(result, h)
		}
	}
	return result
}

// Save saves the config to the proof directory.
func (c *Config) Save(proofDir string) error {
	afDir := filepath.Join(proofDir, ".af")

	// Ensure .af directory exists
	if err := os.MkdirAll(afDir, 0755); err != nil {
		return fmt.Errorf("failed to create .af directory: %w", err)
	}

	path := filepath.Join(afDir, hooksFilename)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// LoadConfig loads the hooks config from the proof directory.
// If the file doesn't exist, returns an empty config (not an error).
func LoadConfig(proofDir string) (*Config, error) {
	path := filepath.Join(proofDir, ".af", hooksFilename)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read hooks config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse hooks config: %w", err)
	}

	// Ensure Hooks is not nil
	if config.Hooks == nil {
		config.Hooks = make([]Hook, 0)
	}

	return &config, nil
}

// EventPayload is the payload sent to hooks.
type EventPayload struct {
	// Event is the event type.
	Event EventType `json:"event"`

	// NodeID is the affected node ID (if applicable).
	NodeID string `json:"node_id,omitempty"`

	// ChallengeID is the challenge ID (if applicable).
	ChallengeID string `json:"challenge_id,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Details contains additional event-specific data.
	Details map[string]string `json:"details,omitempty"`
}

// NewEventPayload creates a new event payload.
func NewEventPayload(event EventType, nodeID types.NodeID) EventPayload {
	return EventPayload{
		Event:     event,
		NodeID:    nodeID.String(),
		Timestamp: time.Now().UTC(),
		Details:   make(map[string]string),
	}
}

// ToEnvVars converts the payload to environment variables.
func (p EventPayload) ToEnvVars() []string {
	vars := []string{
		fmt.Sprintf("AF_EVENT_TYPE=%s", p.Event),
		fmt.Sprintf("AF_NODE_ID=%s", p.NodeID),
		fmt.Sprintf("AF_TIMESTAMP=%s", p.Timestamp.Format(time.RFC3339)),
	}

	if p.ChallengeID != "" {
		vars = append(vars, fmt.Sprintf("AF_CHALLENGE_ID=%s", p.ChallengeID))
	}

	for k, v := range p.Details {
		// Convert key to uppercase and prefix with AF_
		key := fmt.Sprintf("AF_%s=%s", strings.ToUpper(k), v)
		vars = append(vars, key)
	}

	return vars
}

// Executor executes hooks.
type Executor struct {
	client *http.Client
}

// NewExecutor creates a new hook executor.
func NewExecutor() *Executor {
	return &Executor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExecuteWebhook executes a webhook hook.
func (e *Executor) ExecuteWebhook(ctx context.Context, hook Hook, payload EventPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.Target, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AF-Hooks/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}

	return nil
}

// ExecuteCommand executes a command hook.
func (e *Executor) ExecuteCommand(ctx context.Context, hook Hook, payload EventPayload) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Target)

	// Set environment variables
	cmd.Env = append(os.Environ(), payload.ToEnvVars()...)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return nil
}

// Execute executes a hook with the given payload.
func (e *Executor) Execute(ctx context.Context, hook Hook, payload EventPayload) error {
	switch hook.HookType {
	case HookTypeWebhook:
		return e.ExecuteWebhook(ctx, hook, payload)
	case HookTypeCommand:
		return e.ExecuteCommand(ctx, hook, payload)
	default:
		return fmt.Errorf("unknown hook type: %s", hook.HookType)
	}
}

// Manager manages hook execution for a proof directory.
type Manager struct {
	config   *Config
	executor *Executor
	proofDir string
}

// NewManager creates a new hook manager.
func NewManager(proofDir string) (*Manager, error) {
	config, err := LoadConfig(proofDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:   config,
		executor: NewExecutor(),
		proofDir: proofDir,
	}, nil
}

// Fire fires hooks for the given event asynchronously.
// This method returns immediately; hooks are executed in the background.
func (m *Manager) Fire(eventType EventType, payload EventPayload) {
	hooks := m.config.GetHooksForEvent(eventType)
	if len(hooks) == 0 {
		return
	}

	// Execute hooks asynchronously
	for _, hook := range hooks {
		go func(h Hook) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Best effort execution - errors are silently ignored
			// In production, you might want to log these errors
			_ = m.executor.Execute(ctx, h, payload)
		}(hook)
	}
}

// TestHook executes a hook with a sample payload and returns the result.
// Unlike Fire, this is synchronous and returns any error.
func (m *Manager) TestHook(hookID string) error {
	hook, found := m.config.GetHook(hookID)
	if !found {
		return fmt.Errorf("hook not found: %s", hookID)
	}

	// Create a sample payload
	nodeID, _ := types.Parse("1")
	payload := EventPayload{
		Event:       hook.EventType,
		NodeID:      nodeID.String(),
		Timestamp:   time.Now().UTC(),
		ChallengeID: "test-challenge-123",
		Details: map[string]string{
			"test": "true",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return m.executor.Execute(ctx, hook, payload)
}

// Config returns the hook configuration.
func (m *Manager) Config() *Config {
	return m.config
}

// Reload reloads the configuration from disk.
func (m *Manager) Reload() error {
	config, err := LoadConfig(m.proofDir)
	if err != nil {
		return err
	}
	m.config = config
	return nil
}

// GenerateHookID generates a unique hook ID.
func GenerateHookID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "hook-" + hex.EncodeToString(b)
}
