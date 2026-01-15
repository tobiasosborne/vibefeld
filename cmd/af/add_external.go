// Package main contains the af add-external command implementation.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newAddExternalCmd creates the add-external command.
func newAddExternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-external [NAME SOURCE]",
		Short: "Add an external reference to the proof",
		Long: `Add an external reference (axiom, theorem, paper citation) to the proof.

External references allow citing established results from papers,
textbooks, or other authoritative sources that can be used as
foundations for proof steps without requiring re-derivation.

Examples:
  af add-external "Fermat's Last Theorem" "Wiles, A. (1995)"
  af add-external "Prime Number Theorem" "de la Vallee Poussin (1896)"
  af add-external --name "Theorem 3.1" --source "Paper citation" --format json`,
		RunE: runAddExternal,
	}

	cmd.Flags().StringP("name", "n", "", "Name of the external reference (required)")
	cmd.Flags().StringP("source", "s", "", "Source citation for the reference (required)")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// runAddExternal executes the add-external command.
func runAddExternal(cmd *cobra.Command, args []string) error {
	// Get flags
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	source, err := cmd.Flags().GetString("source")
	if err != nil {
		return err
	}
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	// Support positional arguments: af add-external NAME SOURCE
	// Positional args take precedence if flags are not set
	if len(args) >= 2 {
		// If name flag wasn't explicitly set, use positional arg
		if strings.TrimSpace(name) == "" {
			name = args[0]
		}
		// If source flag wasn't explicitly set, use positional arg
		if strings.TrimSpace(source) == "" {
			source = args[1]
		}
	} else if len(args) == 1 {
		// Only one positional arg provided - give helpful error
		return fmt.Errorf("add-external requires both NAME and SOURCE\n\nUsage:\n  af add-external NAME SOURCE\n  af add-external --name NAME --source SOURCE")
	}

	// Validate name is provided and not empty/whitespace
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required and cannot be empty\n\nUsage:\n  af add-external NAME SOURCE\n  af add-external --name NAME --source SOURCE")
	}

	// Validate source is provided and not empty/whitespace
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("source is required and cannot be empty\n\nUsage:\n  af add-external NAME SOURCE\n  af add-external --name NAME --source SOURCE")
	}

	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Add the external reference via service
	extID, err := svc.AddExternal(name, source)
	if err != nil {
		return fmt.Errorf("error adding external reference: %w", err)
	}

	// Load state to get the full external object for output
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	ext := st.GetExternal(extID)
	if ext == nil {
		// Shouldn't happen, but handle gracefully
		return fmt.Errorf("external reference was created but could not be retrieved")
	}

	// Output result based on format
	switch strings.ToLower(format) {
	case "json":
		return outputAddExternalJSON(cmd, ext)
	default:
		return outputAddExternalText(cmd, ext)
	}
}

// outputAddExternalJSON outputs the add-external result in JSON format.
func outputAddExternalJSON(cmd *cobra.Command, ext interface{}) error {
	// Type assert to access fields
	type externalOutput struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Source      string `json:"source"`
		ContentHash string `json:"content_hash"`
	}

	// Use reflection-free approach by type asserting to the External struct
	extStruct, ok := ext.(interface {
		GetID() string
		GetName() string
		GetSource() string
		GetContentHash() string
	})

	var output externalOutput
	if ok {
		output = externalOutput{
			ID:          extStruct.GetID(),
			Name:        extStruct.GetName(),
			Source:      extStruct.GetSource(),
			ContentHash: extStruct.GetContentHash(),
		}
	} else {
		// Fallback: marshal and unmarshal to get fields
		data, err := json.Marshal(ext)
		if err != nil {
			return fmt.Errorf("error marshaling external: %w", err)
		}
		if err := json.Unmarshal(data, &output); err != nil {
			return fmt.Errorf("error unmarshaling external: %w", err)
		}
	}

	result, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(result))
	return nil
}

// outputAddExternalText outputs the add-external result in human-readable text format.
func outputAddExternalText(cmd *cobra.Command, ext interface{}) error {
	// Marshal and unmarshal to get fields
	type externalFields struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Source      string `json:"source"`
		ContentHash string `json:"content_hash"`
	}

	data, err := json.Marshal(ext)
	if err != nil {
		return fmt.Errorf("error marshaling external: %w", err)
	}

	var fields externalFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("error unmarshaling external: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "External reference added successfully.")
	fmt.Fprintf(cmd.OutOrStdout(), "  ID:     %s\n", fields.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", fields.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", fields.Source)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af status   - View proof status")
	fmt.Fprintln(cmd.OutOrStdout(), "  af refine   - Use this external in a proof step")

	return nil
}

func init() {
	rootCmd.AddCommand(newAddExternalCmd())
}
