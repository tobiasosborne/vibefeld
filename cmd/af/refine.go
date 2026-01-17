package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/lemma"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// childSpec represents a child node specification in the --children JSON input.
type childSpec struct {
	Statement string `json:"statement"`
	Type      string `json:"type"`
	Inference string `json:"inference"`
}

// validateNodeTypeAndInference validates node type and inference strings,
// returning the parsed types or an error with proper formatting.
func validateNodeTypeAndInference(cmdName, nodeTypeStr, inferenceStr string, examples []string) (schema.NodeType, schema.InferenceType, error) {
	if err := schema.ValidateNodeType(nodeTypeStr); err != nil {
		return "", "", render.InvalidValueError(cmdName, "type", nodeTypeStr, render.ValidNodeTypes, examples)
	}
	if err := schema.ValidateInference(inferenceStr); err != nil {
		return "", "", render.InvalidValueError(cmdName, "justification", inferenceStr, render.ValidInferenceTypes, examples)
	}
	return schema.NodeType(nodeTypeStr), schema.InferenceType(inferenceStr), nil
}

// parseDependencies parses a comma-separated list of node IDs and validates they exist.
func parseDependencies(depends string, st *state.State) ([]types.NodeID, error) {
	if strings.TrimSpace(depends) == "" {
		return nil, nil
	}
	var dependencies []types.NodeID
	depStrings := strings.Split(depends, ",")
	for _, depStr := range depStrings {
		depStr = strings.TrimSpace(depStr)
		if depStr == "" {
			continue
		}
		depID, err := types.Parse(depStr)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency ID %q: %v", depStr, err)
		}
		if st.GetNode(depID) == nil {
			return nil, fmt.Errorf("invalid dependency: node %s does not exist", depStr)
		}
		dependencies = append(dependencies, depID)
	}
	return dependencies, nil
}

// parseValidationDependencies parses a comma-separated list of validation dependency node IDs.
func parseValidationDependencies(requiresValidated string, st *state.State) ([]types.NodeID, error) {
	if strings.TrimSpace(requiresValidated) == "" {
		return nil, nil
	}
	var validationDeps []types.NodeID
	valDepStrings := strings.Split(requiresValidated, ",")
	for _, valDepStr := range valDepStrings {
		valDepStr = strings.TrimSpace(valDepStr)
		if valDepStr == "" {
			continue
		}
		valDepID, err := types.Parse(valDepStr)
		if err != nil {
			return nil, fmt.Errorf("invalid validation dependency ID %q: %v", valDepStr, err)
		}
		if st.GetNode(valDepID) == nil {
			return nil, fmt.Errorf("invalid validation dependency: node %s does not exist", valDepStr)
		}
		validationDeps = append(validationDeps, valDepID)
	}
	return validationDeps, nil
}

// findNextChildIDResult holds the result of finding the next available child ID.
type findNextChildIDResult struct {
	ChildID   types.NodeID
	ChildNum  int
	WarnDepth bool // true if child depth exceeds WarnDepth
}

// findNextChildID finds the next available child ID for a parent and validates depth constraints.
func findNextChildID(parentID types.NodeID, st *state.State, svc *service.ProofService) (findNextChildIDResult, error) {
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return findNextChildIDResult{}, fmt.Errorf("failed to generate child ID: %v", err)
		}
		if st.GetNode(candidateID) == nil {
			break
		}
		childNum++
	}

	childID, err := parentID.Child(childNum)
	if err != nil {
		return findNextChildIDResult{}, fmt.Errorf("failed to generate child ID: %v", err)
	}

	cfg, err := svc.Config()
	if err != nil {
		return findNextChildIDResult{}, fmt.Errorf("loading config: %v", err)
	}
	childDepth := childID.Depth()
	if childDepth > cfg.MaxDepth {
		return findNextChildIDResult{}, fmt.Errorf("depth %d exceeds MaxDepth %d; add breadth instead", childDepth, cfg.MaxDepth)
	}

	return findNextChildIDResult{
		ChildID:   childID,
		ChildNum:  childNum,
		WarnDepth: childDepth > cfg.WarnDepth,
	}, nil
}

// handleRefineError converts service-layer errors into user-friendly error messages.
func handleRefineError(err error, parentIDStr, owner string) error {
	if errors.Is(err, service.ErrNotClaimed) {
		return fmt.Errorf("parent node is not claimed. Claim it first with 'af claim %s'\n\nHint: Run 'af claim %s -o %s && af refine %s -o %s -s ...' to claim and refine in one step", parentIDStr, parentIDStr, owner, parentIDStr, owner)
	}
	if errors.Is(err, service.ErrOwnerMismatch) {
		return fmt.Errorf("owner does not match the claim owner for node %s", parentIDStr)
	}
	return err
}

// formatMultiChildOutput formats the output for multi-child refine operations.
func formatMultiChildOutput(cmd *cobra.Command, format, parentIDStr string, specs []service.ChildSpec, childIDs []types.NodeID) error {
	type createdChild struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Statement string `json:"statement"`
		Inference string `json:"inference"`
	}
	createdChildren := make([]createdChild, len(childIDs))
	for i, childID := range childIDs {
		createdChildren[i] = createdChild{
			ID:        childID.String(),
			Type:      string(specs[i].NodeType),
			Statement: specs[i].Statement,
			Inference: string(specs[i].Inference),
		}
	}

	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": parentIDStr,
			"children":  createdChildren,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(jsonBytes))
		return nil
	}

	// Text format
	cmd.Printf("Created %d child nodes under %s:\n", len(createdChildren), parentIDStr)
	for _, child := range createdChildren {
		cmd.Printf("  %s [%s]: %s\n", child.ID, child.Type, child.Statement)
	}
	cmd.Println("\nNext steps:")
	if len(createdChildren) > 0 {
		firstChildIDStr := createdChildren[0].ID
		firstChildID, _ := types.Parse(firstChildIDStr)
		if _, hasSiblingParent := firstChildID.Parent(); hasSiblingParent {
			cmd.Printf("  af refine %s --sibling -s ... - Add sibling (recommended for breadth)\n", firstChildIDStr)
			cmd.Printf("  af refine %s -s ...           - Add child (depth-first)\n", firstChildIDStr)
		} else {
			cmd.Printf("  af refine %s -s ...           - Add child\n", firstChildIDStr)
		}
	}
	cmd.Printf("  af status                     - View proof status\n")
	return nil
}

// refineOutputParams holds parameters for formatting refine command output.
type refineOutputParams struct {
	ParentIDStr    string
	ChildID        types.NodeID
	NodeTypeStr    string
	Statement      string
	Dependencies   []types.NodeID
	ValidationDeps []types.NodeID
}

// formatRefineOutput formats the output for a single-child refine operation.
func formatRefineOutput(cmd *cobra.Command, format string, params refineOutputParams) error {
	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": params.ParentIDStr,
			"child_id":  params.ChildID.String(),
			"type":      params.NodeTypeStr,
			"statement": params.Statement,
		}
		if len(params.Dependencies) > 0 {
			depStrs := make([]string, len(params.Dependencies))
			for i, dep := range params.Dependencies {
				depStrs[i] = dep.String()
			}
			result["depends_on"] = depStrs
		}
		if len(params.ValidationDeps) > 0 {
			valDepStrs := make([]string, len(params.ValidationDeps))
			for i, dep := range params.ValidationDeps {
				valDepStrs[i] = dep.String()
			}
			result["requires_validated"] = valDepStrs
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(jsonBytes))
		return nil
	}

	// Text format
	cmd.Printf("Node refined successfully.\n")
	cmd.Printf("  Parent: %s\n", params.ParentIDStr)
	cmd.Printf("  Child:  %s\n", params.ChildID.String())
	cmd.Printf("  Type:   %s\n", params.NodeTypeStr)
	cmd.Printf("  Statement: %s\n", params.Statement)
	if len(params.Dependencies) > 0 {
		depStrs := make([]string, len(params.Dependencies))
		for i, dep := range params.Dependencies {
			depStrs[i] = dep.String()
		}
		cmd.Printf("  Depends on: %s\n", strings.Join(depStrs, ", "))
	}
	if len(params.ValidationDeps) > 0 {
		valDepStrs := make([]string, len(params.ValidationDeps))
		for i, dep := range params.ValidationDeps {
			valDepStrs[i] = dep.String()
		}
		cmd.Printf("  Requires validated: %s\n", strings.Join(valDepStrs, ", "))
	}
	cmd.Println("\nNext steps:")
	if siblingParentID, hasSiblingParent := params.ChildID.Parent(); hasSiblingParent {
		cmd.Printf("  af refine %s --sibling -s ... - Add sibling (recommended for breadth)\n", params.ChildID.String())
		cmd.Printf("  af refine %s -s ...           - Add child (depth-first)\n", params.ChildID.String())
		_ = siblingParentID // parent where siblings would be added
	} else {
		cmd.Printf("  af refine %s -s ...           - Add child\n", params.ChildID.String())
	}
	cmd.Printf("  af status                     - View proof status\n")
	return nil
}

// newRefineCmd creates the refine command for adding child nodes to a proof.
func newRefineCmd() *cobra.Command {
	var owner string
	var statement string
	var nodeType string
	var inference string
	var dir string
	var format string
	var childrenJSON string
	var depends string
	var requiresValidated string
	var sibling bool

	cmd := &cobra.Command{
		Use:   "refine <parent-id> [statement]...",
		Short: "Add a child node to a claimed parent",
		Long: `Add a child node to a claimed parent node.

The parent node must be claimed by the owner specified with --owner.
Child IDs are auto-generated (e.g., 1.1, 1.2 for children of node 1).

You can provide statements as positional arguments for quick multi-child creation:
  af refine 1 "Step A" "Step B" "Step C" --owner agent1
This creates 1.1, 1.2, and 1.3 atomically.

Use --depends to declare logical dependencies on other nodes (cross-references).
This tracks which steps a proof relies on and validates they exist.

Use --requires-validated to declare validation dependencies. A node with validation
dependencies cannot be accepted until all its validation dependencies are validated.
This enables cross-branch dependency tracking in proofs.

Examples:
  af refine 1 --owner agent1 --statement "First subgoal"
  af refine 1 "Step A" "Step B" --owner agent1  (creates 1.1, 1.2)
  af refine 1 --owner agent1 -s "Case 1" --type case --justification local_assume
  af refine 1.1 -o agent1 -s "Deeper refinement" -t claim -j modus_ponens
  af refine 1 --owner agent1 --children '[{"statement":"Child 1"},{"statement":"Child 2","type":"case"}]'
  af refine 1 --owner agent1 -s "By step 1.1, we have..." --depends 1.1
  af refine 1 --owner agent1 -s "Combining steps 1.1 and 1.2..." --depends 1.1,1.2
  af refine 1.5 --owner agent1 -s "Step 1.5" --requires-validated 1.1,1.2,1.3,1.4`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// args[0] is the parent ID, args[1:] are optional positional statements
			var positionalStatements []string
			if len(args) > 1 {
				positionalStatements = args[1:]
			}
			return runRefine(cmd, args[0], owner, statement, nodeType, inference, dir, format, childrenJSON, depends, requiresValidated, positionalStatements, sibling)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required, must match claim owner)")
	cmd.Flags().StringVarP(&statement, "statement", "s", "", "Child node statement (required for single child)")
	cmd.Flags().StringVarP(&nodeType, "type", "t", "claim", "Child node type (claim/local_assume/local_discharge/case/qed)")
	cmd.Flags().StringVarP(&inference, "justification", "j", "assumption",
		"Justification/inference type\n"+
			"Valid: modus_ponens, modus_tollens, by_definition,\n"+
			"assumption, local_assume, local_discharge, contradiction,\n"+
			"universal_instantiation, existential_instantiation,\n"+
			"universal_generalization, existential_generalization")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringVar(&childrenJSON, "children", "", "JSON array of child specifications (mutually exclusive with --statement)")
	cmd.Flags().StringVar(&depends, "depends", "", "Comma-separated list of node IDs this node depends on (e.g., 1.1,1.2)")
	cmd.Flags().StringVar(&requiresValidated, "requires-validated", "", "Comma-separated list of node IDs that must be validated before this node can be accepted (e.g., 1.1,1.2,1.3,1.4)")
	cmd.Flags().BoolVarP(&sibling, "sibling", "b", false, "Add sibling to specified node instead of child (creates at parent level)")

	return cmd
}

func runRefine(cmd *cobra.Command, nodeIDStr, owner, statement, nodeTypeStr, inferenceStr, dir, format, childrenJSON, depends, requiresValidated string, positionalStatements []string, sibling bool) error {
	examples := render.GetExamples("af refine")

	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af refine", "owner", examples)
	}

	// Check for mutually exclusive input methods
	hasStatement := strings.TrimSpace(statement) != ""
	hasChildren := strings.TrimSpace(childrenJSON) != ""
	hasPositional := len(positionalStatements) > 0

	// Count how many input methods are being used
	inputMethodCount := 0
	if hasStatement {
		inputMethodCount++
	}
	if hasChildren {
		inputMethodCount++
	}
	if hasPositional {
		inputMethodCount++
	}

	if inputMethodCount > 1 {
		return render.NewUsageError("af refine",
			"--statement, --children, and positional statements are mutually exclusive; use only one",
			examples)
	}

	if inputMethodCount == 0 {
		return render.MissingFlagError("af refine", "statement", examples)
	}

	// Parse node ID
	nodeID, err := types.Parse(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af refine", nodeIDStr, examples)
	}

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %w", err)
	}

	// Load state to determine next child ID
	st, err := svc.LoadState()
	if err != nil {
		// Check for uninitialized proof (missing ledger directory or no events)
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("proof not initialized. Run 'af init' first")
		}
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if node exists
	if st.GetNode(nodeID) == nil {
		return fmt.Errorf("node %q does not exist", nodeIDStr)
	}

	// Handle --sibling flag: convert node ID to parent ID
	var parentID types.NodeID
	var parentIDStr string
	if sibling {
		// Cannot add sibling to root node
		if nodeID.IsRoot() {
			return fmt.Errorf("cannot add sibling to root node (root has no parent)")
		}
		// Get parent of specified node - that's where we'll add the sibling
		var hasParent bool
		parentID, hasParent = nodeID.Parent()
		if !hasParent {
			return fmt.Errorf("cannot add sibling: node %s has no parent", nodeIDStr)
		}
		parentIDStr = parentID.String()
	} else {
		parentID = nodeID
		parentIDStr = nodeIDStr
	}

	// Handle multi-child mode with --children flag
	if hasChildren {
		return runRefineMulti(cmd, parentID, parentIDStr, owner, childrenJSON, dir, format, svc, st)
	}

	// Handle multi-child mode with positional statements
	if hasPositional {
		return runRefinePositional(cmd, parentID, parentIDStr, owner, nodeTypeStr, inferenceStr, format, svc, st, positionalStatements)
	}

	// Single-child mode (original behavior)
	// Validate node type and inference type
	nodeType, inferenceType, err := validateNodeTypeAndInference("af refine", nodeTypeStr, inferenceStr, examples)
	if err != nil {
		return err
	}

	// Validate definition citations in statement
	if err := lemma.ValidateDefCitations(statement, st); err != nil {
		return fmt.Errorf("invalid definition citation: %w", err)
	}

	// Parse and validate dependencies
	dependencies, err := parseDependencies(depends, st)
	if err != nil {
		return err
	}

	validationDeps, err := parseValidationDependencies(requiresValidated, st)
	if err != nil {
		return err
	}

	// Find next available child ID and check depth constraints
	childResult, err := findNextChildID(parentID, st, svc)
	if err != nil {
		return err
	}

	if childResult.WarnDepth {
		cmd.Printf("Warning: Creating node at depth %d. Consider adding siblings to parent instead.\n\n", childResult.ChildID.Depth())
	}

	// Call the appropriate refine method based on dependencies
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

// runRefineMulti handles the --children flag for creating multiple child nodes at once.
// This uses the atomic RefineNodeBulk method to create all children in a single operation,
// preventing race conditions where other agents could grab the node between individual refines.
func runRefineMulti(cmd *cobra.Command, parentID types.NodeID, parentIDStr, owner, childrenJSON, dir, format string, svc *service.ProofService, st *state.State) error {
	examples := render.GetExamples("af refine")

	// Parse children JSON
	var children []childSpec
	if err := json.Unmarshal([]byte(childrenJSON), &children); err != nil {
		return render.NewUsageError("af refine",
			fmt.Sprintf("invalid JSON for --children: %v", err),
			[]string{"af refine 1 --owner agent1 --children '[{\"statement\":\"Step 1\"},{\"statement\":\"Step 2\",\"type\":\"case\"}]'"})
	}

	// Validate that children array is not empty
	if len(children) == 0 {
		return render.NewUsageError("af refine",
			"--children requires at least one child specification",
			examples)
	}

	// Convert to service.ChildSpec and validate each child specification
	specs := make([]service.ChildSpec, len(children))
	for i, child := range children {
		if strings.TrimSpace(child.Statement) == "" {
			return render.NewUsageError("af refine",
				fmt.Sprintf("child %d: statement is required and cannot be empty", i+1),
				examples)
		}

		// Apply defaults for type and inference
		childType := child.Type
		if childType == "" {
			childType = "claim" // default
		}
		childInference := child.Inference
		if childInference == "" {
			childInference = "assumption" // default
		}

		// Validate type and inference
		nodeType, inferenceType, err := validateNodeTypeAndInference("af refine", childType, childInference, examples)
		if err != nil {
			return fmt.Errorf("child %d: %w", i+1, err)
		}

		// Validate definition citations in statement
		if err := lemma.ValidateDefCitations(child.Statement, st); err != nil {
			return fmt.Errorf("child %d: invalid definition citation: %v", i+1, err)
		}

		specs[i] = service.ChildSpec{
			NodeType:  nodeType,
			Statement: child.Statement,
			Inference: inferenceType,
		}
	}

	// Use RefineNodeBulk for atomic multi-child creation
	childIDs, err := svc.RefineNodeBulk(parentID, owner, specs)
	if err != nil {
		return handleRefineError(err, parentIDStr, owner)
	}

	return formatMultiChildOutput(cmd, format, parentIDStr, specs, childIDs)
}

// runRefinePositional handles positional arguments for creating multiple child nodes at once.
// Example: af refine 1 "Step A" "Step B" "Step C" --owner agent1
// This creates nodes 1.1, 1.2, 1.3 atomically using the RefineNodeBulk method.
func runRefinePositional(cmd *cobra.Command, parentID types.NodeID, parentIDStr, owner, nodeTypeStr, inferenceStr, format string, svc *service.ProofService, st *state.State, statements []string) error {
	examples := render.GetExamples("af refine")

	// Validate node type and inference type (will be used for all children)
	nodeType, inferenceType, err := validateNodeTypeAndInference("af refine", nodeTypeStr, inferenceStr, examples)
	if err != nil {
		return err
	}

	// Convert positional statements to ChildSpec and validate each
	specs := make([]service.ChildSpec, len(statements))
	for i, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			return render.NewUsageError("af refine",
				fmt.Sprintf("statement %d is empty; all statements must be non-empty", i+1),
				examples)
		}

		// Validate definition citations in statement
		if err := lemma.ValidateDefCitations(stmt, st); err != nil {
			return fmt.Errorf("statement %d: invalid definition citation: %v", i+1, err)
		}

		specs[i] = service.ChildSpec{
			NodeType:  nodeType,
			Statement: stmt,
			Inference: inferenceType,
		}
	}

	// Use RefineNodeBulk for atomic multi-child creation
	childIDs, err := svc.RefineNodeBulk(parentID, owner, specs)
	if err != nil {
		return handleRefineError(err, parentIDStr, owner)
	}

	return formatMultiChildOutput(cmd, format, parentIDStr, specs, childIDs)
}

func init() {
	rootCmd.AddCommand(newRefineCmd())
}
