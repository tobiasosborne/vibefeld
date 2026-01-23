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
	var nodeType string
	var inference string
	var dir string
	var format string
	var childrenJSON string
	var depends string
	var requiresValidated string

	cmd := &cobra.Command{
		Use:     "refine-sibling <node-id> <statement>...",
		GroupID: GroupProver,
		Short:   "Add a sibling node (at the parent level)",
		Long: `Add a sibling node at the same level as the specified node.

This is equivalent to adding a child to the parent of the specified node.
For example, 'af refine-sibling 1.2' adds a new child (1.3, 1.4, etc.) to node 1.

Use this command to add breadth to the proof tree rather than depth.

Provide statements as positional arguments:
  af refine-sibling 1.1 "Alternative approach" -o agent1      (creates 1.2)
  af refine-sibling 1.2 "Step A" "Step B" -o agent1           (creates 1.3, 1.4)

For complex cases with different types per sibling, use --children with JSON:
  af refine-sibling 1.1 --children '[{"statement":"Case 2","type":"case"}]' -o agent1

Examples:
  af refine-sibling 1.1 "Alternative approach" -o agent1
  af refine-sibling 1.2 "Step A" "Step B" -o agent1
  af refine-sibling 1.1 "Case 2" -o agent1 --type case --justification local_assume
  af refine-sibling 1.1 --children '[{"statement":"Sibling 1"},{"statement":"Sibling 2"}]' -o agent1

Workflow:
  Use 'af refine' to add depth (children) to a node.
  Use 'af refine-sibling' to add breadth (siblings) at the same level.
  Use 'af status' to view the updated proof tree.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var statements []string
			if len(args) > 1 {
				statements = args[1:]
			}
			return runRefineSibling(cmd, args[0], owner, nodeType, inference, dir, format, childrenJSON, depends, requiresValidated, statements)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required, must match claim owner)")
	cmd.Flags().StringVarP(&nodeType, "type", "t", "claim", "Node type (claim/local_assume/local_discharge/case/qed)")
	cmd.Flags().StringVarP(&inference, "justification", "j", "assumption",
		"Justification/inference type\n"+
			"Valid: modus_ponens, modus_tollens, by_definition,\n"+
			"assumption, local_assume, local_discharge, contradiction,\n"+
			"universal_instantiation, existential_instantiation,\n"+
			"universal_generalization, existential_generalization")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringVar(&childrenJSON, "children", "", "JSON array of sibling specs for complex cases")
	cmd.Flags().StringVar(&depends, "depends", "", "Comma-separated list of node IDs this node depends on")
	cmd.Flags().StringVar(&requiresValidated, "requires-validated", "", "Comma-separated list of node IDs that must be validated before this node can be accepted")

	return cmd
}

func runRefineSibling(cmd *cobra.Command, nodeIDStr, owner, nodeTypeStr, inferenceStr, dir, format, childrenJSON, depends, requiresValidated string, statements []string) error {
	examples := render.GetExamples("af refine-sibling")

	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af refine-sibling", "owner", examples)
	}

	// Check input methods: positional statements vs --children JSON
	hasChildren := strings.TrimSpace(childrenJSON) != ""
	hasStatements := len(statements) > 0

	if hasChildren && hasStatements {
		return render.NewUsageError("af refine-sibling",
			"positional statements and --children are mutually exclusive; use one or the other",
			examples)
	}

	if !hasChildren && !hasStatements {
		return render.NewUsageError("af refine-sibling",
			"statement required: provide as positional argument or use --children for complex cases",
			examples)
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

	// Handle --children JSON mode (for complex cases)
	if hasChildren {
		return runRefineMulti(cmd, parentID, parentIDStr, owner, childrenJSON, dir, format, svc, st)
	}

	// Handle positional statements (primary method)
	return runRefinePositional(cmd, parentID, parentIDStr, owner, nodeTypeStr, inferenceStr, format, svc, st, statements, depends, requiresValidated)
}

func init() {
	rootCmd.AddCommand(newRefineSiblingCmd())
}
