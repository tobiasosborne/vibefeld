package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
)

// ReplayStats holds statistics gathered during ledger replay.
type ReplayStats struct {
	EventsProcessed int `json:"events_processed"`
	Nodes           int `json:"nodes"`
	Challenges      struct {
		Total    int `json:"total"`
		Resolved int `json:"resolved"`
		Open     int `json:"open"`
	} `json:"challenges"`
	Definitions      int  `json:"definitions"`
	Valid            bool `json:"valid"`
	HashVerification *struct {
		Verified int  `json:"verified"`
		Total    int  `json:"total"`
		Valid    bool `json:"valid"`
	} `json:"hash_verification,omitempty"`
}

// newReplayCmd creates the replay command.
func newReplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "replay",
		GroupID: GroupAdmin,
		Short:   "Replay ledger to rebuild and verify state",
		Long: `Replay all events from the ledger to rebuild and verify the proof state.

The replay command processes all events in sequence order and shows statistics
about the proof including nodes, challenges, and definitions.

With --verify, the command also validates content hashes for all nodes to
ensure data integrity.

Examples:
  af replay                         Replay ledger in current directory
  af replay --dir /path/to/proof    Replay for specific proof directory
  af replay --verify                Verify content hashes during replay
  af replay --format json           Output in JSON format
  af replay -v                      Show detailed replay progress`,
		RunE: runReplay,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("verify", false, "Verify content hashes during replay")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed replay progress")

	return cmd
}

// runReplay executes the replay command.
func runReplay(cmd *cobra.Command, args []string) error {
	// Get flags
	dir := service.MustString(cmd, "dir")
	format := service.MustString(cmd, "format")
	verify := service.MustBool(cmd, "verify")
	verbose := service.MustBool(cmd, "verbose")

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Open ledger
	ledgerDir := filepath.Join(dir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("error accessing ledger: %w", err)
	}

	// Count events first
	eventCount, err := ldg.Count()
	if err != nil {
		return fmt.Errorf("error counting events: %w", err)
	}

	// Perform replay
	var st *state.State
	if verify {
		st, err = state.ReplayWithVerify(ldg)
	} else {
		st, err = state.Replay(ldg)
	}

	// Build stats
	stats := ReplayStats{
		EventsProcessed: eventCount,
		Valid:           err == nil,
	}

	if st != nil {
		// Count nodes
		stats.Nodes = len(st.AllNodes())

		// Count challenges
		challenges := st.AllChallenges()
		stats.Challenges.Total = len(challenges)
		for _, c := range challenges {
			if c.Status == state.ChallengeStatusResolved {
				stats.Challenges.Resolved++
			} else if c.Status == state.ChallengeStatusOpen {
				stats.Challenges.Open++
			}
		}

		// Count definitions by scanning ledger events
		defCount, _ := countDefinitions(ldg)
		stats.Definitions = defCount

		// Add hash verification info if verify mode
		if verify {
			stats.HashVerification = &struct {
				Verified int  `json:"verified"`
				Total    int  `json:"total"`
				Valid    bool `json:"valid"`
			}{
				Verified: stats.Nodes,
				Total:    stats.Nodes,
				Valid:    err == nil,
			}
		}
	}

	// Handle replay error
	if err != nil {
		if format == "json" {
			stats.Valid = false
			output, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(output))
			return nil
		}
		return fmt.Errorf("replay failed: %w", err)
	}

	// Output based on format
	if format == "json" {
		output, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("error formatting JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	}

	// Text format
	output := formatReplayText(stats, verify, verbose)
	fmt.Fprint(cmd.OutOrStdout(), output)

	return nil
}

// formatReplayText formats replay statistics as human-readable text.
func formatReplayText(stats ReplayStats, verify bool, verbose bool) string {
	var sb strings.Builder

	if verify {
		sb.WriteString("Replaying ledger with hash verification...\n")
	} else {
		sb.WriteString("Replaying ledger...\n")
	}

	sb.WriteString(fmt.Sprintf("  Events processed: %d\n", stats.EventsProcessed))
	sb.WriteString(fmt.Sprintf("  Nodes: %d\n", stats.Nodes))

	if stats.Challenges.Total > 0 || verbose {
		sb.WriteString(fmt.Sprintf("  Challenges: %d (%d resolved, %d open)\n",
			stats.Challenges.Total, stats.Challenges.Resolved, stats.Challenges.Open))
	}

	if stats.Definitions > 0 || verbose {
		sb.WriteString(fmt.Sprintf("  Definitions: %d\n", stats.Definitions))
	}

	if verify && stats.HashVerification != nil {
		if stats.HashVerification.Valid {
			sb.WriteString(fmt.Sprintf("  Hashes verified: %d/%d\n",
				stats.HashVerification.Verified, stats.HashVerification.Total))
		} else {
			sb.WriteString(fmt.Sprintf("  Hash verification failed: %d/%d valid\n",
				stats.HashVerification.Verified, stats.HashVerification.Total))
		}
	}

	if stats.Valid {
		if verify {
			sb.WriteString("  Replay complete. All hashes valid.\n")
		} else {
			sb.WriteString("  Replay complete. State is valid.\n")
		}
	} else {
		sb.WriteString("  Replay failed. State may be corrupted.\n")
	}

	// Add verbose details
	if verbose {
		sb.WriteString("\nDetails:\n")
		sb.WriteString(fmt.Sprintf("  Total events in ledger: %d\n", stats.EventsProcessed))
		sb.WriteString(fmt.Sprintf("  Node count: %d\n", stats.Nodes))
		sb.WriteString(fmt.Sprintf("  Definition count: %d\n", stats.Definitions))
		sb.WriteString(fmt.Sprintf("  Challenge count: %d\n", stats.Challenges.Total))
		if stats.Challenges.Total > 0 {
			sb.WriteString(fmt.Sprintf("    - Resolved: %d\n", stats.Challenges.Resolved))
			sb.WriteString(fmt.Sprintf("    - Open: %d\n", stats.Challenges.Open))
		}
	}

	return sb.String()
}

// countDefinitions counts the number of DefAdded events in the ledger.
func countDefinitions(ldg *ledger.Ledger) (int, error) {
	count := 0

	err := ldg.Scan(func(seq int, data []byte) error {
		// Parse event type
		var base struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil // Skip invalid events
		}

		// Count DefAdded events
		if base.Type == ledger.EventDefAdded {
			count++
		}

		return nil
	})

	return count, err
}

func init() {
	rootCmd.AddCommand(newReplayCmd())
}
