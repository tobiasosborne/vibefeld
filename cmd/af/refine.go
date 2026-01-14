package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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

	cmd := &cobra.Command{
		Use:   "refine <parent-id>",
		Short: "Add a child node to a claimed parent",
		Long: `Add a child node to a claimed parent node.

The parent node must be claimed by the owner specified with --owner.
Child IDs are auto-generated (e.g., 1.1, 1.2 for children of node 1).

Examples:
  af refine 1 --owner agent1 --statement "First subgoal"
  af refine 1 --owner agent1 -s "Case 1" --type case --inference local_assume
  af refine 1.1 -o agent1 -s "Deeper refinement" -T claim -i modus_ponens
  af refine 1 --owner agent1 --children '[{"statement":"Child 1"},{"statement":"Child 2","type":"case"}]'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefine(cmd, args[0], owner, statement, nodeType, inference, dir, format, childrenJSON)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Agent/owner name (required, must match claim owner)")
	cmd.Flags().StringVarP(&statement, "statement", "s", "", "Child node statement (required for single child)")
	cmd.Flags().StringVarP(&nodeType, "type", "T", "claim", "Child node type (claim/local_assume/local_discharge/case/qed)")
	cmd.Flags().StringVarP(&inference, "inference", "i", "assumption", "Inference type")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringVar(&childrenJSON, "children", "", "JSON array of child specifications (mutually exclusive with --statement)")

	return cmd
}

func runRefine(cmd *cobra.Command, parentIDStr, owner, statement, nodeTypeStr, inferenceStr, dir, format, childrenJSON string) error {
	// Validate owner is not empty
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("owner is required")
	}

	// Check for mutually exclusive flags
	hasStatement := strings.TrimSpace(statement) != ""
	hasChildren := strings.TrimSpace(childrenJSON) != ""

	if hasStatement && hasChildren {
		return fmt.Errorf("--statement and --children are mutually exclusive; use one or the other")
	}

	if !hasStatement && !hasChildren {
		return fmt.Errorf("statement is required and cannot be empty")
	}

	// Parse parent ID
	parentID, err := types.Parse(parentIDStr)
	if err != nil {
		return fmt.Errorf("invalid parent ID: %v", err)
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

	// Single-child mode (original behavior)
	// Validate node type
	if err := schema.ValidateNodeType(nodeTypeStr); err != nil {
		validTypes := []string{"claim", "local_assume", "local_discharge", "case", "qed"}
		return fmt.Errorf("invalid node type %q. Valid types: %s", nodeTypeStr, strings.Join(validTypes, ", "))
	}
	nodeType := schema.NodeType(nodeTypeStr)

	// Validate inference type
	if err := schema.ValidateInference(inferenceStr); err != nil {
		return fmt.Errorf("invalid inference type %q: %v", inferenceStr, err)
	}
	inferenceType := schema.InferenceType(inferenceStr)

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

	// Call RefineNode
	err = svc.RefineNode(parentID, owner, childID, nodeType, statement, inferenceType)
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
	// Parse children JSON
	var children []childSpec
	if err := json.Unmarshal([]byte(childrenJSON), &children); err != nil {
		return fmt.Errorf("invalid JSON for --children: %v", err)
	}

	// Validate that children array is not empty
	if len(children) == 0 {
		return fmt.Errorf("--children requires at least one child specification")
	}

	// Convert to service.ChildSpec and validate each child specification
	specs := make([]service.ChildSpec, len(children))
	for i, child := range children {
		if strings.TrimSpace(child.Statement) == "" {
			return fmt.Errorf("child %d: statement is required and cannot be empty", i+1)
		}

		// Validate and apply default type
		childType := child.Type
		if childType == "" {
			childType = "claim" // default
		}
		if err := schema.ValidateNodeType(childType); err != nil {
			validTypes := []string{"claim", "local_assume", "local_discharge", "case", "qed"}
			return fmt.Errorf("child %d: invalid node type %q. Valid types: %s", i+1, childType, strings.Join(validTypes, ", "))
		}

		// Validate and apply default inference
		childInference := child.Inference
		if childInference == "" {
			childInference = "assumption" // default
		}
		if err := schema.ValidateInference(childInference); err != nil {
			return fmt.Errorf("child %d: invalid inference type %q: %v", i+1, childInference, err)
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

func init() {
	rootCmd.AddCommand(newRefineCmd())
}
