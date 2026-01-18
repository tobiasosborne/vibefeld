// Package main contains the af def-reject command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

// newDefRejectCmd creates the def-reject command for rejecting pending definitions.
func newDefRejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "def-reject <term|node-id|id>",
		GroupID: GroupAdmin,
		Short:   "Reject a pending definition request",
		Long: `Reject a pending definition request.

This command cancels a pending definition request, indicating that
the definition is no longer needed for the proof. The pending definition
must be in "pending" status to be rejected.

You can look up a pending definition by:
- Term name (e.g., "group")
- Node ID that requested it (e.g., "1.1")
- Pending definition ID (full or partial)

Examples:
  af def-reject group                    Reject the pending def for term "group"
  af def-reject 1.1                      Reject the pending def requested by node 1.1
  af def-reject abc123                   Reject the pending def with ID starting with "abc123"
  af def-reject group --reason "Not needed"  Reject with a reason
  af def-reject group -f json            Output in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runDefReject,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("reason", "r", "", "Reason for rejection")

	return cmd
}

// runDefReject executes the def-reject command.
func runDefReject(cmd *cobra.Command, args []string) error {
	// Get the lookup argument
	lookup := args[0]
	if strings.TrimSpace(lookup) == "" {
		return fmt.Errorf("pending definition identifier cannot be empty")
	}

	// Get flags
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	reason, err := cmd.Flags().GetString("reason")
	if err != nil {
		return err
	}

	// Validate format
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Create proof service to verify initialization
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Get all pending definitions
	pendingDefs, err := svc.LoadAllPendingDefs()
	if err != nil {
		return fmt.Errorf("error loading pending definitions: %w", err)
	}

	// Find pending definition by various methods
	pd := findPendingDef(pendingDefs, lookup)

	if pd == nil {
		return fmt.Errorf("pending definition %q not found", lookup)
	}

	// Check if the pending definition is still in pending status
	if pd.Status != node.PendingDefStatusPending {
		return fmt.Errorf("cannot reject: pending definition %q is not in pending status (current status: %s)", pd.Term, pd.Status)
	}

	// Cancel the pending definition
	if err := pd.Cancel(); err != nil {
		return fmt.Errorf("error cancelling pending definition: %w", err)
	}

	// Write the updated pending definition back to filesystem
	if err := svc.WritePendingDef(pd.RequestedBy, pd); err != nil {
		return fmt.Errorf("error saving pending definition: %w", err)
	}

	// Output result based on format
	switch format {
	case "json":
		return outputDefRejectJSON(cmd, pd, reason)
	default:
		return outputDefRejectText(cmd, pd, reason)
	}
}

// outputDefRejectJSON outputs the def-reject result in JSON format.
func outputDefRejectJSON(cmd *cobra.Command, pd *node.PendingDef, reason string) error {
	result := map[string]interface{}{
		"term":         pd.Term,
		"id":           pd.ID,
		"requested_by": pd.RequestedBy.String(),
		"status":       string(pd.Status),
		"rejected":     true,
	}

	if reason != "" {
		result["reason"] = reason
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputDefRejectText outputs the def-reject result in human-readable text format.
func outputDefRejectText(cmd *cobra.Command, pd *node.PendingDef, reason string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Rejected pending definition for term '%s'\n", pd.Term)

	if reason != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", reason)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af pending-defs  - List all pending definitions")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status        - Check proof status")

	return nil
}

func init() {
	rootCmd.AddCommand(newDefRejectCmd())
}
