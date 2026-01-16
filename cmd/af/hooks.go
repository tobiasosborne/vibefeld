// Package main contains the af hooks command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/hooks"
	"github.com/tobias/vibefeld/internal/service"
)

func newHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage hooks for external integrations",
		Long: `Manage webhook and command hooks for external integrations.

Hooks are triggered asynchronously when events occur in the proof system,
allowing external systems to react to proof state changes.

Hook types:
  webhook  - HTTP POST to URL with event JSON payload
  command  - Execute shell command with event data as env vars

Events:
  node_created       - Fired when a new node is added
  node_validated     - Fired when a verifier validates a node
  challenge_raised   - Fired when a verifier raises a challenge
  challenge_resolved - Fired when a challenge is resolved

Examples:
  af hooks list                                    # List all hooks
  af hooks add node_created webhook https://...    # Add webhook hook
  af hooks add challenge_raised command "echo $AF_NODE_ID"
  af hooks remove hook-abc123                      # Remove a hook
  af hooks test hook-abc123                        # Test a hook`,
	}

	cmd.AddCommand(newHooksListCmd())
	cmd.AddCommand(newHooksAddCmd())
	cmd.AddCommand(newHooksRemoveCmd())
	cmd.AddCommand(newHooksTestCmd())

	return cmd
}

func newHooksListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured hooks",
		Long: `List all configured hooks in the proof directory.

Displays hook ID, event type, hook type, target, and enabled status.

Examples:
  af hooks list                  # List in text format
  af hooks list --format json    # List in JSON format`,
		RunE: runHooksList,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

func runHooksList(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load hooks config
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("error loading hooks config: %w", err)
	}

	// Output based on format
	format = strings.ToLower(format)
	if format == "json" {
		data, err := json.MarshalIndent(config.Hooks, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text format
	if len(config.Hooks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No hooks configured.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Add a hook with:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af hooks add <event-type> <hook-type> <target>")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Event types: node_created, node_validated, challenge_raised, challenge_resolved")
		fmt.Fprintln(cmd.OutOrStdout(), "Hook types: webhook, command")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "=== Configured Hooks ===")
	fmt.Fprintln(cmd.OutOrStdout())

	for _, h := range config.Hooks {
		status := "enabled"
		if !h.Enabled {
			status = "disabled"
		}

		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", h.ID, status)
		fmt.Fprintf(cmd.OutOrStdout(), "  Event:  %s\n", h.EventType)
		fmt.Fprintf(cmd.OutOrStdout(), "  Type:   %s\n", h.HookType)
		fmt.Fprintf(cmd.OutOrStdout(), "  Target: %s\n", h.Target)
		if h.Description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Desc:   %s\n", h.Description)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Total: %d hook(s)\n", len(config.Hooks))
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next: Use 'af hooks test <id>' to test a hook")

	return nil
}

func newHooksAddCmd() *cobra.Command {
	var description string
	var disabled bool

	cmd := &cobra.Command{
		Use:   "add <event-type> <hook-type> <target>",
		Short: "Add a new hook",
		Long: `Add a new hook to the proof directory.

Arguments:
  event-type  - The event to trigger on (node_created, node_validated,
                challenge_raised, challenge_resolved)
  hook-type   - The hook type (webhook or command)
  target      - The webhook URL or shell command to execute

For webhooks, the target must be a valid URL. The event payload is sent
as JSON in the POST body.

For commands, the target is executed as a shell command. Event data is
available as environment variables:
  AF_EVENT_TYPE    - The event type
  AF_NODE_ID       - The affected node ID
  AF_CHALLENGE_ID  - The challenge ID (if applicable)
  AF_TIMESTAMP     - When the event occurred

Examples:
  af hooks add node_created webhook https://example.com/hook
  af hooks add challenge_raised command "notify-send 'Challenge on $AF_NODE_ID'"
  af hooks add node_validated webhook https://slack.com/... --description "Slack notification"`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			return runHooksAdd(cmd, dir, args[0], args[1], args[2], description, disabled)
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringVar(&description, "description", "", "Optional description for the hook")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "Create hook in disabled state")

	return cmd
}

func runHooksAdd(cmd *cobra.Command, dir, eventTypeStr, hookTypeStr, target, description string, disabled bool) error {
	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Validate event type
	eventType := hooks.EventType(eventTypeStr)
	if err := hooks.ValidateEventType(eventType); err != nil {
		return fmt.Errorf("invalid event type %q: must be one of: node_created, node_validated, challenge_raised, challenge_resolved", eventTypeStr)
	}

	// Validate hook type
	hookType := hooks.HookType(hookTypeStr)
	if err := hooks.ValidateHookType(hookType); err != nil {
		return fmt.Errorf("invalid hook type %q: must be 'webhook' or 'command'", hookTypeStr)
	}

	// Create hook
	hook := hooks.Hook{
		ID:          hooks.GenerateHookID(),
		EventType:   eventType,
		HookType:    hookType,
		Target:      target,
		Enabled:     !disabled,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	// Validate the complete hook
	if err := hook.Validate(); err != nil {
		return err
	}

	// Load existing config
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("error loading hooks config: %w", err)
	}

	// Add hook
	config.AddHook(hook)

	// Save config
	if err := config.Save(dir); err != nil {
		return fmt.Errorf("error saving hooks config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Hook added: %s\n", hook.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Event:  %s\n", hook.EventType)
	fmt.Fprintf(cmd.OutOrStdout(), "  Type:   %s\n", hook.HookType)
	fmt.Fprintf(cmd.OutOrStdout(), "  Target: %s\n", hook.Target)
	if hook.Description != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Desc:   %s\n", hook.Description)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "Test with: af hooks test %s\n", hook.ID)

	return nil
}

func newHooksRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <hook-id>",
		Short: "Remove a hook",
		Long: `Remove a hook by its ID.

Use 'af hooks list' to see all hook IDs.

Example:
  af hooks remove hook-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runHooksRemove,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")

	return cmd
}

func runHooksRemove(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	hookID := args[0]

	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load config
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("error loading hooks config: %w", err)
	}

	// Remove hook
	if !config.RemoveHook(hookID) {
		return fmt.Errorf("hook not found: %s", hookID)
	}

	// Save config
	if err := config.Save(dir); err != nil {
		return fmt.Errorf("error saving hooks config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Hook removed: %s\n", hookID)

	return nil
}

func newHooksTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <hook-id>",
		Short: "Test a hook with a sample event",
		Long: `Test a hook by executing it with a sample event payload.

This sends a test event to verify the hook is configured correctly.
The test payload includes sample data for all fields.

Use 'af hooks list' to see all hook IDs.

Example:
  af hooks test hook-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runHooksTest,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")

	return cmd
}

func runHooksTest(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	hookID := args[0]

	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load config
	config, err := hooks.LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("error loading hooks config: %w", err)
	}

	// Find hook
	hook, found := config.GetHook(hookID)
	if !found {
		return fmt.Errorf("hook not found: %s", hookID)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Testing hook: %s\n", hookID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Type:   %s\n", hook.HookType)
	fmt.Fprintf(cmd.OutOrStdout(), "  Target: %s\n", hook.Target)
	fmt.Fprintln(cmd.OutOrStdout())

	// Create manager and test
	manager, err := hooks.NewManager(dir)
	if err != nil {
		return fmt.Errorf("error creating hook manager: %w", err)
	}

	err = manager.TestHook(hookID)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Test FAILED: %v\n", err)
		return fmt.Errorf("hook test failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Test OK - hook executed successfully")

	return nil
}

func init() {
	rootCmd.AddCommand(newHooksCmd())
}
