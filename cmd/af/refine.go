package main

import (
	"encoding/json"
	"fmt"
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

Examples:
  af refine 1 --owner agent1 --statement "First subgoal"
  af refine 1 "Step A" "Step B" --owner agent1  (creates 1.1, 1.2)
  af refine 1 --owner agent1 -s "Case 1" --type case --justification local_assume
  af refine 1.1 -o agent1 -s "Deeper refinement" -t claim -j modus_ponens
  af refine 1 --owner agent1 --children '[{"statement":"Child 1"},{"statement":"Child 2","type":"case"}]'
  af refine 1 --owner agent1 -s "By step 1.1, we have..." --depends 1.1
  af refine 1 --owner agent1 -s "Combining steps 1.1 and 1.2..." --depends 1.1,1.2`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// args[0] is the parent ID, args[1:] are optional positional statements
			var positionalStatements []string
			if len(args) > 1 {
				positionalStatements = args[1:]
			}
			return runRefine(cmd, args[0], owner, statement, nodeType, inference, dir, format, childrenJSON, depends, positionalStatements)
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

	return cmd
}

func runRefine(cmd *cobra.Command, parentIDStr, owner, statement, nodeTypeStr, inferenceStr, dir, format, childrenJSON, depends string, positionalStatements []string) error {
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

	// Parse parent ID
	parentID, err := types.Parse(parentIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af refine", parentIDStr, examples)
	}

	// Create service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %v", err)
	}

	// Load state to determine next child ID
	st, err := svc.LoadState()
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no events") || strings.Contains(errStr, "empty") ||
			strings.Contains(errStr, "no such file or directory") || strings.Contains(errStr, "does not exist") {
			return fmt.Errorf("proof not initialized. Run 'af init' first")
		}
		return fmt.Errorf("failed to load state: %v", err)
	}

	// Check if parent exists
	if st.GetNode(parentID) == nil {
		return fmt.Errorf("parent node %q does not exist", parentIDStr)
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
	// Validate node type
	if err := schema.ValidateNodeType(nodeTypeStr); err != nil {
		return render.InvalidValueError("af refine", "type", nodeTypeStr, render.ValidNodeTypes, examples)
	}
	nodeType := schema.NodeType(nodeTypeStr)

	// Validate inference type
	if err := schema.ValidateInference(inferenceStr); err != nil {
		return render.InvalidValueError("af refine", "justification", inferenceStr, render.ValidInferenceTypes, examples)
	}
	inferenceType := schema.InferenceType(inferenceStr)

	// Validate definition citations in statement
	if err := lemma.ValidateDefCitations(statement, st); err != nil {
		return fmt.Errorf("invalid definition citation: %v", err)
	}

	// Parse and validate dependencies
	var dependencies []types.NodeID
	if strings.TrimSpace(depends) != "" {
		depStrings := strings.Split(depends, ",")
		for _, depStr := range depStrings {
			depStr = strings.TrimSpace(depStr)
			if depStr == "" {
				continue
			}
			depID, err := types.Parse(depStr)
			if err != nil {
				return fmt.Errorf("invalid dependency ID %q: %v", depStr, err)
			}
			// Validate that the dependency node exists
			if st.GetNode(depID) == nil {
				return fmt.Errorf("invalid dependency: node %s does not exist", depStr)
			}
			dependencies = append(dependencies, depID)
		}
	}

	// Find next available child ID
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return fmt.Errorf("failed to generate child ID: %v", err)
		}
		if st.GetNode(candidateID) == nil {
			break
		}
		childNum++
	}

	childID, err := parentID.Child(childNum)
	if err != nil {
		return fmt.Errorf("failed to generate child ID: %v", err)
	}

	// Call RefineNodeWithDeps if we have dependencies, otherwise use RefineNode
	if len(dependencies) > 0 {
		err = svc.RefineNodeWithDeps(parentID, owner, childID, nodeType, statement, inferenceType, dependencies)
	} else {
		err = svc.RefineNode(parentID, owner, childID, nodeType, statement, inferenceType)
	}
	if err != nil {
		// Provide helpful error messages
		if strings.Contains(err.Error(), "not claimed") {
			return fmt.Errorf("parent node is not claimed. Claim it first with 'af claim %s'", parentIDStr)
		}
		if strings.Contains(err.Error(), "owner does not match") {
			return fmt.Errorf("owner does not match the claim owner for node %s", parentIDStr)
		}
		return err
	}

	// Output result
	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": parentIDStr,
			"child_id":  childID.String(),
			"type":      nodeTypeStr,
			"statement": statement,
		}
		if len(dependencies) > 0 {
			depStrs := make([]string, len(dependencies))
			for i, dep := range dependencies {
				depStrs[i] = dep.String()
			}
			result["depends_on"] = depStrs
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		cmd.Printf("Node refined successfully.\n")
		cmd.Printf("  Parent: %s\n", parentIDStr)
		cmd.Printf("  Child:  %s\n", childID.String())
		cmd.Printf("  Type:   %s\n", nodeTypeStr)
		cmd.Printf("  Statement: %s\n", statement)
		if len(dependencies) > 0 {
			depStrs := make([]string, len(dependencies))
			for i, dep := range dependencies {
				depStrs[i] = dep.String()
			}
			cmd.Printf("  Depends on: %s\n", strings.Join(depStrs, ", "))
		}
		cmd.Println("\nNext steps:")
		cmd.Printf("  af refine %s    - Add more children to this node\n", childID.String())
		cmd.Printf("  af status       - View proof status\n")
	}

	return nil
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

		// Validate and apply default type
		childType := child.Type
		if childType == "" {
			childType = "claim" // default
		}
		if err := schema.ValidateNodeType(childType); err != nil {
			return render.InvalidValueError("af refine", "type",
				fmt.Sprintf("child %d: %s", i+1, childType), render.ValidNodeTypes, examples)
		}

		// Validate and apply default inference
		childInference := child.Inference
		if childInference == "" {
			childInference = "assumption" // default
		}
		if err := schema.ValidateInference(childInference); err != nil {
			return render.InvalidValueError("af refine", "justification",
				fmt.Sprintf("child %d: %s", i+1, childInference), render.ValidInferenceTypes, examples)
		}

		// Validate definition citations in statement
		if err := lemma.ValidateDefCitations(child.Statement, st); err != nil {
			return fmt.Errorf("child %d: invalid definition citation: %v", i+1, err)
		}

		specs[i] = service.ChildSpec{
			NodeType:  schema.NodeType(childType),
			Statement: child.Statement,
			Inference: schema.InferenceType(childInference),
		}
	}

	// Use RefineNodeBulk for atomic multi-child creation
	childIDs, err := svc.RefineNodeBulk(parentID, owner, specs)
	if err != nil {
		// Provide helpful error messages
		if strings.Contains(err.Error(), "not claimed") {
			return fmt.Errorf("parent node is not claimed. Claim it first with 'af claim %s'", parentIDStr)
		}
		if strings.Contains(err.Error(), "owner does not match") {
			return fmt.Errorf("owner does not match the claim owner for node %s", parentIDStr)
		}
		return err
	}

	// Build output structure
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

	// Output result
	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": parentIDStr,
			"children":  createdChildren,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		cmd.Printf("Created %d child nodes under %s:\n", len(createdChildren), parentIDStr)
		for _, child := range createdChildren {
			cmd.Printf("  %s [%s]: %s\n", child.ID, child.Type, child.Statement)
		}
		cmd.Println("\nNext steps:")
		if len(createdChildren) > 0 {
			cmd.Printf("  af refine %s    - Add children to the first new node\n", createdChildren[0].ID)
		}
		cmd.Printf("  af status       - View proof status\n")
	}

	return nil
}

// runRefinePositional handles positional arguments for creating multiple child nodes at once.
// Example: af refine 1 "Step A" "Step B" "Step C" --owner agent1
// This creates nodes 1.1, 1.2, 1.3 atomically using the RefineNodeBulk method.
func runRefinePositional(cmd *cobra.Command, parentID types.NodeID, parentIDStr, owner, nodeTypeStr, inferenceStr, format string, svc *service.ProofService, st *state.State, statements []string) error {
	examples := render.GetExamples("af refine")

	// Validate node type (will be used for all children)
	if err := schema.ValidateNodeType(nodeTypeStr); err != nil {
		return render.InvalidValueError("af refine", "type", nodeTypeStr, render.ValidNodeTypes, examples)
	}
	nodeType := schema.NodeType(nodeTypeStr)

	// Validate inference type (will be used for all children)
	if err := schema.ValidateInference(inferenceStr); err != nil {
		return render.InvalidValueError("af refine", "justification", inferenceStr, render.ValidInferenceTypes, examples)
	}
	inferenceType := schema.InferenceType(inferenceStr)

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
		// Provide helpful error messages
		if strings.Contains(err.Error(), "not claimed") {
			return fmt.Errorf("parent node is not claimed. Claim it first with 'af claim %s'", parentIDStr)
		}
		if strings.Contains(err.Error(), "owner does not match") {
			return fmt.Errorf("owner does not match the claim owner for node %s", parentIDStr)
		}
		return err
	}

	// Build output structure
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

	// Output result
	if format == "json" {
		result := map[string]interface{}{
			"success":   true,
			"parent_id": parentIDStr,
			"children":  createdChildren,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		cmd.Println(string(jsonBytes))
	} else {
		cmd.Printf("Created %d child nodes under %s:\n", len(createdChildren), parentIDStr)
		for _, child := range createdChildren {
			cmd.Printf("  %s [%s]: %s\n", child.ID, child.Type, child.Statement)
		}
		cmd.Println("\nNext steps:")
		if len(createdChildren) > 0 {
			cmd.Printf("  af refine %s    - Add children to the first new node\n", createdChildren[0].ID)
		}
		cmd.Printf("  af status       - View proof status\n")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRefineCmd())
}
