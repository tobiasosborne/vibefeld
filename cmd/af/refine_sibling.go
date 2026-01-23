package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newRefineSiblingCmd creates the refine-sibling command for adding sibling nodes.
func newRefineSiblingCmd() *cobra.Command {
	var owner string
	var statement string
	var nodeType string
	var inference string
	var dir string
	var format string
	var childrenJSON string
	var depends string
	var requiresValidated string

	cmd := &cobra.Command{
		Use:     "refine-sibling <node-id> [statement]...",
		GroupID: GroupProver,
		Short:   "Add a sibling node (at the parent level)",
		Long: `Add a sibling node at the same level as the specified node.

This is equivalent to adding a child to the parent of the specified node.
For example, 'af refine-sibling 1.2' adds a new child (1.3, 1.4, etc.) to node 1.

Use this command to add breadth to the proof tree rather than depth.

You can provide statements as positional arguments for quick multi-sibling creation:
  af refine-sibling 1.2 "Step A" "Step B" --owner agent1
This creates 1.3 and 1.4 (siblings of 1.2) atomically.

Examples:
  af refine-sibling 1.1 --owner agent1 -s "Alternative approach"
  af refine-sibling 1.2 "Step A" "Step B" --owner agent1
  af refine-sibling 1.1 -o agent1 -s "Case 2" --type case --justification local_assume
  af refine-sibling 1 --owner agent1 --children '[{"statement":"Sibling 1"},{"statement":"Sibling 2"}]'

Workflow:
  Use 'af refine' to add depth (children) to a node.
  Use 'af refine-sibling' to add breadth (siblings) at the same level.
  Use 'af status' to view the updated proof tree.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var positionalStatements []string
			if len(args) > 1 {
				positionalStatements = args[1:]
			}
			return runRefineSibling(cmd, args[0], owner, statement, nodeType, inference, dir, format, childrenJSON, depends, requiresValidated, positionalStatements)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required, must match claim owner)")
	cmd.Flags().StringVarP(&statement, "statement", "s", "", "Sibling node statement (required for single sibling)")
	cmd.Flags().StringVarP(&nodeType, "type", "t", "claim", "Node type (claim/local_assume/local_discharge/case/qed)")
	cmd.Flags().StringVarP(&inference, "justification", "j", "assumption",
		"Justification/inference type\n"+
			"Valid: modus_ponens, modus_tollens, by_definition,\n"+
			"assumption, local_assume, local_discharge, contradiction,\n"+
			"universal_instantiation, existential_instantiation,\n"+
			"universal_generalization, existential_generalization")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringVar(&childrenJSON, "children", "", "JSON array of sibling specifications")
	cmd.Flags().StringVar(&depends, "depends", "", "Comma-separated list of node IDs this node depends on")
	cmd.Flags().StringVar(&requiresValidated, "requires-validated", "", "Comma-separated list of node IDs that must be validated before this node can be accepted")

	return cmd
}

func runRefineSibling(cmd *cobra.Command, nodeIDStr, owner, statement, nodeTypeStr, inferenceStr, dir, format, childrenJSON, depends, requiresValidated string, positionalStatements []string) error {
	examples := render.GetExamples("af refine-sibling")

	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af refine-sibling", "owner", examples)
	}

	// Check for mutually exclusive input methods
	hasStatement := strings.TrimSpace(statement) != ""
	hasChildren := strings.TrimSpace(childrenJSON) != ""
	hasPositional := len(positionalStatements) > 0

	activeInputMethods := 0
	if hasStatement {
		activeInputMethods++
	}
	if hasChildren {
		activeInputMethods++
	}
	if hasPositional {
		activeInputMethods++
	}

	if activeInputMethods > 1 {
		return render.NewUsageError("af refine-sibling",
			"--statement, --children, and positional statements are mutually exclusive; use only one",
			examples)
	}

	if activeInputMethods == 0 {
		return render.MissingFlagError("af refine-sibling", "statement", examples)
	}

	// Parse node ID
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af refine-sibling", nodeIDStr, examples)
	}

	// Cannot add sibling to root node
	if nodeID.IsRoot() {
		return fmt.Errorf("cannot add sibling to root node (root has no parent)")
	}

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %w", err)
	}

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("proof not initialized. Run 'af init' first")
		}
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if specified node exists
	if st.GetNode(nodeID) == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}

	// Get parent of specified node - that's where we'll add the sibling
	parentID, hasParent := nodeID.Parent()
	if !hasParent {
		return fmt.Errorf("cannot add sibling: node %s has no parent", nodeIDStr)
	}
	parentIDStr := parentID.String()

	// Handle multi-child mode with --children flag
	if hasChildren {
		return runRefineMulti(cmd, parentID, parentIDStr, owner, childrenJSON, dir, format, svc, st)
	}

	// Handle multi-child mode with positional statements
	if hasPositional {
		return runRefinePositional(cmd, parentID, parentIDStr, owner, nodeTypeStr, inferenceStr, format, svc, st, positionalStatements)
	}

	// Single-sibling mode
	nodeType, inferenceType, err := validateNodeTypeAndInference("af refine-sibling", nodeTypeStr, inferenceStr, examples)
	if err != nil {
		return err
	}

	// Validate definition citations
	if err := service.ValidateDefCitations(statement, st); err != nil {
		return fmt.Errorf("invalid definition citation: %w", err)
	}

	// Parse dependencies
	dependencies, err := parseDependencies(depends, st)
	if err != nil {
		return err
	}

	validationDeps, err := parseValidationDependencies(requiresValidated, st)
	if err != nil {
		return err
	}

	// Find next available child ID (sibling to the specified node)
	childResult, err := findNextChildID(parentID, st, svc)
	if err != nil {
		return err
	}

	if childResult.WarnDepth {
		cmd.Printf("Warning: Creating node at depth %d.\n\n", childResult.ChildID.Depth())
	}

	// Call the appropriate refine method
	if len(dependencies) > 0 || len(validationDeps) > 0 {
		err = svc.RefineNodeWithAllDeps(parentID, owner, childResult.ChildID, nodeType, statement, inferenceType, dependencies, validationDeps)
	} else {
		err = svc.RefineNode(parentID, owner, childResult.ChildID, nodeType, statement, inferenceType)
	}
	if err != nil {
		return handleRefineError(err, parentIDStr, owner)
	}

	return formatRefineOutput(cmd, format, refineOutputParams{
		ParentIDStr:    parentIDStr,
		ChildID:        childResult.ChildID,
		NodeTypeStr:    nodeTypeStr,
		Statement:      statement,
		Dependencies:   dependencies,
		ValidationDeps: validationDeps,
	})
}

func init() {
	rootCmd.AddCommand(newRefineSiblingCmd())
}
