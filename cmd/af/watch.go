// Package main contains the af watch command implementation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream events in real-time",
		Long: `Watch the event ledger for new events in real-time.

Polls the ledger directory at a configurable interval and displays
new events as they appear. Useful for monitoring proof progress
or debugging agent interactions.

The command runs continuously until interrupted with Ctrl+C.

Examples:
  af watch                      Watch for new events (default 1s interval)
  af watch --interval 500ms     Poll every 500 milliseconds
  af watch --json               Output events as NDJSON (newline-delimited JSON)
  af watch --filter node_created  Only show node creation events
  af watch --since 10           Start watching from sequence 10
  af watch -d ./proof           Watch specific proof directory
  af watch --once               Show current events and exit (no polling)`,
		RunE: runWatch,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().Duration("interval", 1*time.Second, "Poll interval (e.g., 1s, 500ms)")
	cmd.Flags().Bool("json", false, "Output events as NDJSON (newline-delimited JSON)")
	cmd.Flags().String("filter", "", "Filter events by type (partial match, case-insensitive)")
	cmd.Flags().Int("since", 0, "Start watching from this sequence number")
	cmd.Flags().Bool("once", false, "Show current events and exit (no continuous watching)")

	return cmd
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	interval, err := cmd.Flags().GetDuration("interval")
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}
	jsonOutput, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	filter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return err
	}
	since, err := cmd.Flags().GetInt("since")
	if err != nil {
		return err
	}
	once, err := cmd.Flags().GetBool("once")
	if err != nil {
		return err
	}

	// Create ledger instance
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Track the last seen sequence number
	lastSeq := since

	// Set up signal handling for graceful shutdown
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Create a context that's canceled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// If --once mode, just show current events and exit
	if once {
		return showCurrentEvents(cmd, ldg, lastSeq, filter, jsonOutput)
	}

	// Create ticker for polling
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial scan
	newLastSeq, err := scanAndDisplayEvents(cmd, ldg, lastSeq, filter, jsonOutput)
	if err != nil {
		return err
	}
	lastSeq = newLastSeq

	// Watch loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			newLastSeq, err := scanAndDisplayEvents(cmd, ldg, lastSeq, filter, jsonOutput)
			if err != nil {
				// Log error but continue watching
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: error scanning ledger: %v\n", err)
				continue
			}
			lastSeq = newLastSeq
		}
	}
}

// showCurrentEvents displays all current events and exits.
func showCurrentEvents(cmd *cobra.Command, ldg *ledger.Ledger, since int, filter string, jsonOutput bool) error {
	_, err := scanAndDisplayEvents(cmd, ldg, since, filter, jsonOutput)
	return err
}

// scanAndDisplayEvents scans the ledger for new events and displays them.
// Returns the highest sequence number seen.
func scanAndDisplayEvents(cmd *cobra.Command, ldg *ledger.Ledger, since int, filter string, jsonOutput bool) (int, error) {
	lastSeq := since

	err := ldg.Scan(func(seq int, data []byte) error {
		// Skip already seen events
		if seq <= since {
			return nil
		}

		// Parse the event
		var eventData map[string]interface{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			return fmt.Errorf("failed to parse event %d: %w", seq, err)
		}

		// Get event type
		eventType := ""
		if t, ok := eventData["type"].(string); ok {
			eventType = t
		}

		// Apply filter
		if !matchesFilter(eventType, filter) {
			// Update lastSeq even if filtered out
			if seq > lastSeq {
				lastSeq = seq
			}
			return nil
		}

		// Display the event
		var output string
		if jsonOutput {
			output = formatWatchEventJSON(seq, eventData)
		} else {
			output = formatWatchEvent(seq, eventData)
		}

		fmt.Fprintln(cmd.OutOrStdout(), output)

		// Update lastSeq
		if seq > lastSeq {
			lastSeq = seq
		}

		return nil
	})

	if err != nil {
		return lastSeq, err
	}

	return lastSeq, nil
}

// matchesFilter checks if an event type matches the filter string.
// Filter is case-insensitive and supports partial matching.
// Empty filter matches all events.
func matchesFilter(eventType, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(eventType), strings.ToLower(filter))
}

// formatWatchEvent formats an event for text output.
func formatWatchEvent(seq int, data map[string]interface{}) string {
	// Extract type
	eventType := ""
	if t, ok := data["type"].(string); ok {
		eventType = t
	}

	// Extract timestamp
	timestamp := ""
	if ts, ok := data["timestamp"].(string); ok {
		timestamp = formatTimestamp(ts)
	}

	// Format type for display
	displayType := formatEventType(eventType)

	// Generate summary
	summary := generateSummary(eventType, data)

	return fmt.Sprintf("#%-3d  %-20s  %s  %s", seq, displayType, timestamp, summary)
}

// formatWatchEventJSON formats an event for JSON output (NDJSON format).
func formatWatchEventJSON(seq int, data map[string]interface{}) string {
	// Build output object with seq first
	output := map[string]interface{}{
		"seq": seq,
	}

	// Copy all event data
	for k, v := range data {
		output[k] = v
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		// Fallback to basic format
		return fmt.Sprintf(`{"seq":%d,"error":"failed to marshal event"}`, seq)
	}

	return string(jsonBytes)
}

func init() {
	rootCmd.AddCommand(newWatchCmd())
}
