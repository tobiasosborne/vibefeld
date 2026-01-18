// Package main contains the af schema command for displaying proof schema information.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// validSections defines all valid section names and their aliases.
var validSections = map[string]string{
	// inference types
	"inference-types": "inference-types",
	"inference_types": "inference-types",
	"inferences":      "inference-types",
	// node types
	"node-types": "node-types",
	"node_types": "node-types",
	"nodes":      "node-types",
	// workflow states
	"workflow-states": "workflow-states",
	"workflow_states": "workflow-states",
	"workflow":        "workflow-states",
	// epistemic states
	"epistemic-states": "epistemic-states",
	"epistemic_states": "epistemic-states",
	"epistemic":        "epistemic-states",
	// taint states
	"taint-states": "taint-states",
	"taint_states": "taint-states",
	"taint":        "taint-states",
	// challenge targets
	"challenge-targets": "challenge-targets",
	"challenge_targets": "challenge-targets",
	"targets":           "challenge-targets",
	// combined states section
	"states": "states",
}

// TaintStateInfo contains metadata about a taint state.
type TaintStateInfo struct {
	ID          node.TaintState `json:"id"`
	Description string          `json:"description"`
}

// allTaintStates returns information about all taint states.
func allTaintStates() []TaintStateInfo {
	return []TaintStateInfo{
		{ID: node.TaintClean, Description: "No taint - all dependencies are clean"},
		{ID: node.TaintSelfAdmitted, Description: "This node itself was admitted without proof"},
		{ID: node.TaintTainted, Description: "Depends on an admitted node"},
		{ID: node.TaintUnresolved, Description: "Taint status not yet computed"},
	}
}

// newSchemaCmd creates the schema command for displaying proof schema information.
func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schema",
		GroupID: GroupQuery,
		Short:   "Show proof schema information",
		Long: `Display schema information for the AF proof framework.

The schema command shows valid values for:
  - Inference types (modus_ponens, universal_instantiation, etc.)
  - Node types (claim, local_assume, local_discharge, case, qed)
  - Workflow states (available, claimed, blocked)
  - Epistemic states (pending, validated, admitted, refuted, archived)
  - Taint states (clean, self_admitted, tainted, unresolved)
  - Challenge targets (statement, inference, gap, etc.)

This command works without an initialized proof directory because
the schema is static configuration data.

Examples:
  af schema                           Show all schema information
  af schema --format json             Output in JSON format
  af schema --section inference-types Show only inference types
  af schema -s states                 Show only state information`,
		RunE: runSchema,
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().StringP("section", "s", "", "Filter to specific section")
	cmd.Flags().StringP("dir", "d", ".", "Proof directory path (ignored for schema)")

	return cmd
}

// runSchema executes the schema command.
func runSchema(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	section, _ := cmd.Flags().GetString("section")

	// Validate format (case-insensitive)
	format = strings.ToLower(format)
	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	// Validate and normalize section
	normalizedSection := ""
	if section != "" {
		normalized, ok := validSections[section]
		if !ok {
			return fmt.Errorf("unknown section %q: valid sections are inference-types, node-types, workflow-states, epistemic-states, taint-states, challenge-targets, states", section)
		}
		normalizedSection = normalized
	}

	// Output based on format
	if format == "json" {
		return outputSchemaJSON(cmd, normalizedSection)
	}

	return outputSchemaText(cmd, normalizedSection)
}

// schemaJSON represents the JSON output structure.
type schemaJSON struct {
	InferenceTypes   []inferenceTypeJSON   `json:"inference_types,omitempty"`
	NodeTypes        []nodeTypeJSON        `json:"node_types,omitempty"`
	WorkflowStates   []workflowStateJSON   `json:"workflow_states,omitempty"`
	EpistemicStates  []epistemicStateJSON  `json:"epistemic_states,omitempty"`
	TaintStates      []taintStateJSON      `json:"taint_states,omitempty"`
	ChallengeTargets []challengeTargetJSON `json:"challenge_targets,omitempty"`
}

type inferenceTypeJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Form string `json:"form,omitempty"`
}

type nodeTypeJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	OpensScope  bool   `json:"opens_scope,omitempty"`
	ClosesScope bool   `json:"closes_scope,omitempty"`
}

type workflowStateJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type epistemicStateJSON struct {
	ID              string `json:"id"`
	Description     string `json:"description"`
	IsFinal         bool   `json:"is_final,omitempty"`
	IntroducesTaint bool   `json:"introduces_taint,omitempty"`
}

type taintStateJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type challengeTargetJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// outputSchemaJSON outputs schema information in JSON format.
func outputSchemaJSON(cmd *cobra.Command, section string) error {
	output := schemaJSON{}

	// Determine which sections to include
	includeAll := section == ""
	includeInference := includeAll || section == "inference-types"
	includeNodes := includeAll || section == "node-types"
	includeWorkflow := includeAll || section == "workflow-states" || section == "states"
	includeEpistemic := includeAll || section == "epistemic-states" || section == "states"
	includeTaint := includeAll || section == "taint-states" || section == "states"
	includeTargets := includeAll || section == "challenge-targets"

	if includeInference {
		for _, info := range schema.AllInferences() {
			output.InferenceTypes = append(output.InferenceTypes, inferenceTypeJSON{
				ID:   string(info.ID),
				Name: info.Name,
				Form: info.Form,
			})
		}
	}

	if includeNodes {
		for _, info := range schema.AllNodeTypes() {
			output.NodeTypes = append(output.NodeTypes, nodeTypeJSON{
				ID:          string(info.ID),
				Description: info.Description,
				OpensScope:  info.OpensScope,
				ClosesScope: info.ClosesScope,
			})
		}
	}

	if includeWorkflow {
		for _, info := range schema.AllWorkflowStates() {
			output.WorkflowStates = append(output.WorkflowStates, workflowStateJSON{
				ID:          string(info.ID),
				Description: info.Description,
			})
		}
	}

	if includeEpistemic {
		for _, info := range schema.AllEpistemicStates() {
			output.EpistemicStates = append(output.EpistemicStates, epistemicStateJSON{
				ID:              string(info.ID),
				Description:     info.Description,
				IsFinal:         info.IsFinal,
				IntroducesTaint: info.IntroducesTaint,
			})
		}
	}

	if includeTaint {
		for _, info := range allTaintStates() {
			output.TaintStates = append(output.TaintStates, taintStateJSON{
				ID:          string(info.ID),
				Description: info.Description,
			})
		}
	}

	if includeTargets {
		for _, info := range schema.AllChallengeTargets() {
			output.ChallengeTargets = append(output.ChallengeTargets, challengeTargetJSON{
				ID:          string(info.ID),
				Description: info.Description,
			})
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputSchemaText outputs schema information in human-readable text format.
func outputSchemaText(cmd *cobra.Command, section string) error {
	out := cmd.OutOrStdout()

	// Determine which sections to include
	includeAll := section == ""
	includeInference := includeAll || section == "inference-types"
	includeNodes := includeAll || section == "node-types"
	includeWorkflow := includeAll || section == "workflow-states" || section == "states"
	includeEpistemic := includeAll || section == "epistemic-states" || section == "states"
	includeTaint := includeAll || section == "taint-states" || section == "states"
	includeTargets := includeAll || section == "challenge-targets"

	needSeparator := false

	if includeInference {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Inference Types ===")
		fmt.Fprintln(out)
		for _, info := range schema.AllInferences() {
			fmt.Fprintf(out, "  %-30s %s\n", info.ID, info.Name)
			if info.Form != "" {
				fmt.Fprintf(out, "    %s\n", info.Form)
			}
		}
		needSeparator = true
	}

	if includeNodes {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Node Types ===")
		fmt.Fprintln(out)
		for _, info := range schema.AllNodeTypes() {
			fmt.Fprintf(out, "  %-20s %s\n", info.ID, info.Description)
		}
		needSeparator = true
	}

	if includeWorkflow {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Workflow States ===")
		fmt.Fprintln(out)
		for _, info := range schema.AllWorkflowStates() {
			fmt.Fprintf(out, "  %-15s %s\n", info.ID, info.Description)
		}
		needSeparator = true
	}

	if includeEpistemic {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Epistemic States ===")
		fmt.Fprintln(out)
		for _, info := range schema.AllEpistemicStates() {
			fmt.Fprintf(out, "  %-15s %s\n", info.ID, info.Description)
		}
		needSeparator = true
	}

	if includeTaint {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Taint States ===")
		fmt.Fprintln(out)
		for _, info := range allTaintStates() {
			fmt.Fprintf(out, "  %-15s %s\n", info.ID, info.Description)
		}
		needSeparator = true
	}

	if includeTargets {
		if needSeparator {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "=== Challenge Targets ===")
		fmt.Fprintln(out)
		for _, info := range schema.AllChallengeTargets() {
			fmt.Fprintf(out, "  %-15s %s\n", info.ID, info.Description)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newSchemaCmd())
}
