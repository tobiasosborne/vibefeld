// Package main contains the af verify-external command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/service"
)

// newVerifyExternalCmd creates the verify-external command for marking an external
// reference as verified by a human reviewer.
func newVerifyExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-external <ext-id>",
		Short: "Verify an external reference",
		Long: `Mark an external reference as verified by a human reviewer.

This command confirms that an external reference (citation, axiom, theorem)
has been properly checked and verified. Verification indicates that a human
has confirmed the external reference is accurate and appropriate for use
in the proof.

Examples:
  af verify-external abc123def456     Verify the external with ID abc123def456
  af verify-external abc123 -f json   Verify and output result as JSON
  af verify-external abc123 -d /path  Verify external in specific proof directory`,
		Args: cobra.ExactArgs(1),
		RunE: runVerifyExternal,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")

	return cmd
}

// runVerifyExternal executes the verify-external command.
func runVerifyExternal(cmd *cobra.Command, args []string) error {
	extID := args[0]

	// Validate external ID is not empty or whitespace
	if strings.TrimSpace(extID) == "" {
		return fmt.Errorf("external ID cannot be empty or whitespace")
	}

	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")

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

	// Read the external from filesystem
	ext, err := fs.ReadExternal(svc.Path(), extID)
	if err != nil {
		if strings.Contains(err.Error(), "no such file") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("external %q not found", extID)
		}
		return fmt.Errorf("error reading external: %w", err)
	}

	// Check if already verified
	alreadyVerified := isExternalVerified(ext.Notes)

	// Mark as verified by updating the Notes field with verification metadata
	if !alreadyVerified {
		ext.Notes = addVerificationToNotes(ext.Notes)

		// Write the updated external back to filesystem
		if err := fs.WriteExternal(svc.Path(), ext); err != nil {
			return fmt.Errorf("error updating external: %w", err)
		}
	}

	// Output based on format
	if format == "json" {
		return outputVerifyExternalJSON(cmd, ext, alreadyVerified)
	}

	return outputVerifyExternalText(cmd, ext, alreadyVerified)
}

// verificationMarker is the prefix used to mark an external as verified.
const verificationMarker = "[VERIFIED:"

// isExternalVerified checks if an external's Notes field indicates it has been verified.
func isExternalVerified(notes string) bool {
	return strings.Contains(notes, verificationMarker)
}

// addVerificationToNotes adds verification metadata to the notes field.
func addVerificationToNotes(notes string) string {
	verificationNote := fmt.Sprintf("%s %s]", verificationMarker, time.Now().UTC().Format(time.RFC3339))
	if strings.TrimSpace(notes) == "" {
		return verificationNote
	}
	return notes + " " + verificationNote
}

// outputVerifyExternalJSON outputs the verification result in JSON format.
func outputVerifyExternalJSON(cmd *cobra.Command, ext interface{}, alreadyVerified bool) error {
	// Create output structure
	type verifyOutput struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Source           string `json:"source"`
		Verified         bool   `json:"verified"`
		AlreadyVerified  bool   `json:"already_verified,omitempty"`
		ContentHash      string `json:"content_hash,omitempty"`
	}

	// Type assert to access fields - using marshal/unmarshal for flexibility
	data, err := json.Marshal(ext)
	if err != nil {
		return fmt.Errorf("error marshaling external: %w", err)
	}

	var extFields struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Source      string `json:"source"`
		ContentHash string `json:"content_hash"`
	}
	if err := json.Unmarshal(data, &extFields); err != nil {
		return fmt.Errorf("error unmarshaling external: %w", err)
	}

	output := verifyOutput{
		ID:              extFields.ID,
		Name:            extFields.Name,
		Source:          extFields.Source,
		Verified:        true,
		AlreadyVerified: alreadyVerified,
		ContentHash:     extFields.ContentHash,
	}

	result, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(result))
	return nil
}

// outputVerifyExternalText outputs the verification result in human-readable text format.
func outputVerifyExternalText(cmd *cobra.Command, ext interface{}, alreadyVerified bool) error {
	// Type assert to access fields
	data, err := json.Marshal(ext)
	if err != nil {
		return fmt.Errorf("error marshaling external: %w", err)
	}

	var fields struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("error unmarshaling external: %w", err)
	}

	if alreadyVerified {
		fmt.Fprintln(cmd.OutOrStdout(), "External reference was already verified.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "External reference verified successfully.")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", fields.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", fields.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", fields.Source)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af externals  - List all external references")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status     - View proof status")

	return nil
}

func init() {
	rootCmd.AddCommand(newVerifyExternalCmd())
}
