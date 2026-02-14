// Package main contains the af update-external command implementation.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
)

func newUpdateExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-external <name-or-id>",
		GroupID: GroupAdmin,
		Short:   "Update an external reference",
		Long: `Update fields of an existing external reference.

Allows correcting stale or wrong external references (e.g., wrong
paper citation, updated URL). Specify the external by name or ID,
then provide the fields to update.

At least one of --name, --source, or --notes must be provided.

Examples:
  af update-external "Fermat" --source "Wiles, A. (1995) corrected"
  af update-external "Old Name" --name "New Name"
  af update-external "Theorem 3.1" --source "arXiv:2403.10485" --notes "Corrected from 2310.09740"
  af update-external abc123 --name "Better Name"`,
		Args: cobra.ExactArgs(1),
		RunE: runUpdateExternal,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().String("name", "", "New name for the external reference")
	cmd.Flags().String("source", "", "New source citation")
	cmd.Flags().String("notes", "", "Notes about the update")

	return cmd
}

func runUpdateExternal(cmd *cobra.Command, args []string) error {
	nameOrID := args[0]
	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	newName, _ := cmd.Flags().GetString("name")
	newSource, _ := cmd.Flags().GetString("source")
	newNotes, _ := cmd.Flags().GetString("notes")

	// Require at least one field to update
	if strings.TrimSpace(newName) == "" && strings.TrimSpace(newSource) == "" && strings.TrimSpace(newNotes) == "" {
		return fmt.Errorf("at least one of --name, --source, or --notes must be provided\n\nUsage:\n  af update-external <name-or-id> --source \"new citation\"")
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Resolve name-or-id to an actual ID
	extID, err := resolveExternalID(svc, nameOrID)
	if err != nil {
		return err
	}

	// Perform the update
	updated, err := svc.UpdateExternal(extID, newName, newSource, newNotes)
	if err != nil {
		return fmt.Errorf("error updating external: %w", err)
	}

	return outputUpdateExternal(cmd, updated, format)
}

// resolveExternalID resolves a name or ID to an external ID.
func resolveExternalID(svc *service.ProofService, nameOrID string) (string, error) {
	// Try as ID first
	ext, err := svc.ReadExternal(nameOrID)
	if err == nil && ext != nil {
		return ext.ID, nil
	}

	// Try as name — search all externals
	ids, err := svc.ListExternals()
	if err != nil {
		return "", fmt.Errorf("error listing externals: %w", err)
	}

	for _, id := range ids {
		ext, err := svc.ReadExternal(id)
		if err != nil {
			continue
		}
		if ext.Name == nameOrID {
			return ext.ID, nil
		}
	}

	return "", fmt.Errorf("external %q not found (searched by ID and name)", nameOrID)
}

func outputUpdateExternal(cmd *cobra.Command, ext *node.External, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return writeJSONOutput(cmd, map[string]interface{}{
			"id":           ext.ID,
			"name":         ext.Name,
			"source":       ext.Source,
			"content_hash": ext.ContentHash,
			"notes":        ext.Notes,
			"updated":      true,
		})
	default:
		fmt.Fprintln(cmd.OutOrStdout(), "External reference updated.")
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", ext.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", ext.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", ext.Source)
		if ext.Notes != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Notes:  %s\n", ext.Notes)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newUpdateExternalCmd())
}
