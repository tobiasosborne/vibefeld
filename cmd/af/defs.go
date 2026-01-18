// Package main contains the af defs and af def commands for viewing definitions.
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// newDefsCmd creates the defs command for listing all definitions.
func newDefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "defs",
		GroupID: GroupQuery,
		Short:   "List all definitions in the proof",
		Long: `List all definitions that have been added to the proof.

Definitions provide formal terms that can be referenced in proof steps.
This command displays all definitions with their names.

Examples:
  af defs                     List all definitions
  af defs --format json       Output in JSON format
  af defs -d /path/to/proof   List definitions from specific directory`,
		RunE: runDefs,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("verbose", false, "Show verbose output")

	return cmd
}

// newDefCmd creates the def command for showing a specific definition.
func newDefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "def <name>",
		GroupID: GroupQuery,
		Short: "Show a specific definition by name",
		Long: `Show details of a specific definition.

Retrieves and displays the definition with the given name, including
its content and metadata.

Examples:
  af def group                Show the definition named "group"
  af def homomorphism -F      Show full details
  af def kernel -f json       Output in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: runDef,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().BoolP("full", "F", false, "Show full definition details")

	return cmd
}

// runDefs executes the defs command.
func runDefs(cmd *cobra.Command, args []string) error {
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

	// Get all definitions from ledger
	definitions, err := getAllDefinitions(dir)
	if err != nil {
		return fmt.Errorf("error loading definitions: %w", err)
	}

	// Sort by name
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})

	// Output based on format
	if format == "json" {
		return outputDefsJSON(cmd, definitions)
	}

	return outputDefsText(cmd, definitions)
}

// runDef executes the def command.
func runDef(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("definition name cannot be empty")
	}

	dir, _ := cmd.Flags().GetString("dir")
	format, _ := cmd.Flags().GetString("format")
	full, _ := cmd.Flags().GetBool("full")

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

	// Get all definitions and find by name
	definitions, err := getAllDefinitions(dir)
	if err != nil {
		return fmt.Errorf("error loading definitions: %w", err)
	}

	// Find definition by name (case-sensitive)
	var def *node.Definition
	for _, d := range definitions {
		if d.Name == name {
			def = d
			break
		}
	}

	if def == nil {
		return fmt.Errorf("definition %q not found", name)
	}

	// Output based on format
	if format == "json" {
		return outputDefJSON(cmd, def, full)
	}

	return outputDefText(cmd, def, full)
}

// getAllDefinitions reads all definitions from the ledger.
func getAllDefinitions(proofDir string) ([]*node.Definition, error) {
	ledgerDir := filepath.Join(proofDir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return nil, err
	}

	var definitions []*node.Definition

	err = ldg.Scan(func(seq int, data []byte) error {
		// Parse event type
		var base struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return nil // Skip invalid events
		}

		// Only process DefAdded events
		if base.Type != ledger.EventDefAdded {
			return nil
		}

		// Parse full DefAdded event
		var event ledger.DefAdded
		if err := json.Unmarshal(data, &event); err != nil {
			return nil // Skip malformed events
		}

		// Convert to node.Definition
		def := &node.Definition{
			ID:      event.Definition.ID,
			Name:    event.Definition.Name,
			Content: event.Definition.Definition,
			Created: event.Definition.Created,
		}
		definitions = append(definitions, def)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

// outputDefsJSON outputs definitions in JSON format.
func outputDefsJSON(cmd *cobra.Command, definitions []*node.Definition) error {
	type defOutput struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Content string          `json:"content"`
		Created types.Timestamp `json:"created"`
	}

	output := make([]defOutput, 0, len(definitions))
	for _, d := range definitions {
		output = append(output, defOutput{
			ID:      d.ID,
			Name:    d.Name,
			Content: d.Content,
			Created: d.Created,
		})
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputDefsText outputs definitions in text format.
func outputDefsText(cmd *cobra.Command, definitions []*node.Definition) error {
	if len(definitions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No definitions found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Definitions (%d):\n", len(definitions))
	fmt.Fprintln(cmd.OutOrStdout())

	for _, d := range definitions {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", d.Name)
	}

	return nil
}

// outputDefJSON outputs a single definition in JSON format.
func outputDefJSON(cmd *cobra.Command, def *node.Definition, full bool) error {
	type defOutput struct {
		ID          string          `json:"id,omitempty"`
		Name        string          `json:"name"`
		Content     string          `json:"content"`
		ContentHash string          `json:"content_hash,omitempty"`
		Created     types.Timestamp `json:"created,omitempty"`
	}

	output := defOutput{
		Name:    def.Name,
		Content: def.Content,
	}

	if full {
		output.ID = def.ID
		output.ContentHash = def.ContentHash
		output.Created = def.Created
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputDefText outputs a single definition in text format.
func outputDefText(cmd *cobra.Command, def *node.Definition, full bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Definition: %s\n", def.Name)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", def.Content)

	if full {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\n", def.ID)
		if def.ContentHash != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Content Hash: %s\n", def.ContentHash)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", def.Created.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newDefsCmd())
	rootCmd.AddCommand(newDefCmd())
}
