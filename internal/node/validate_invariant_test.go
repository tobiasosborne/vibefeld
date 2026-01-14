// Package node_test contains external tests for the node package.
package node_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestCheckValidationInvariant_NoChildren tests that a node with no children can be validated.
func TestCheckValidationInvariant_NoChildren(t *testing.T) {
	// Create a validated node with no children
	id, _ := types.Parse("1")
	n, err := node.NewNode(id, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated

	// getChildren returns empty slice (no children)
	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	err = node.CheckValidationInvariant(n, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant() = %v, want nil for node with no children", err)
	}
}

// TestCheckValidationInvariant_AllChildrenValidated tests that a node with all validated children can be validated.
func TestCheckValidationInvariant_AllChildrenValidated(t *testing.T) {
	// Create parent node
	parentID, _ := types.Parse("1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	parent.EpistemicState = schema.EpistemicValidated

	// Create validated child nodes
	childID1, _ := types.Parse("1.1")
	child1, err := node.NewNode(childID1, schema.NodeTypeClaim, "Child 1", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child1.EpistemicState = schema.EpistemicValidated

	childID2, _ := types.Parse("1.2")
	child2, err := node.NewNode(childID2, schema.NodeTypeClaim, "Child 2", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child2.EpistemicState = schema.EpistemicValidated

	// getChildren returns the validated children
	getChildren := func(id types.NodeID) []*node.Node {
		if id.String() == "1" {
			return []*node.Node{child1, child2}
		}
		return nil
	}

	err = node.CheckValidationInvariant(parent, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant() = %v, want nil for node with all validated children", err)
	}
}

// TestCheckValidationInvariant_UnvalidatedChild tests that a node with an unvalidated child cannot be validated.
func TestCheckValidationInvariant_UnvalidatedChild(t *testing.T) {
	// Create parent node
	parentID, _ := types.Parse("1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	parent.EpistemicState = schema.EpistemicValidated

	// Create a pending (unvalidated) child
	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Unvalidated child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child.EpistemicState = schema.EpistemicPending

	// getChildren returns the pending child
	getChildren := func(id types.NodeID) []*node.Node {
		if id.String() == "1" {
			return []*node.Node{child}
		}
		return nil
	}

	err = node.CheckValidationInvariant(parent, getChildren, nil)
	if err == nil {
		t.Error("CheckValidationInvariant() = nil, want error for node with unvalidated child")
	}
}

// TestCheckValidationInvariant_MixedChildStates tests that a node with mixed child states cannot be validated.
func TestCheckValidationInvariant_MixedChildStates(t *testing.T) {
	tests := []struct {
		name         string
		childStates  []schema.EpistemicState
		expectError  bool
		description  string
	}{
		{
			name:         "one validated one pending",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicPending},
			expectError:  true,
			description:  "mixed validated and pending",
		},
		{
			name:         "one validated one admitted",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicAdmitted},
			expectError:  false,
			description:  "admitted is acceptable like validated",
		},
		{
			name:         "one validated one refuted",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicRefuted},
			expectError:  true,
			description:  "refuted is not validated",
		},
		{
			name:         "all pending",
			childStates:  []schema.EpistemicState{schema.EpistemicPending, schema.EpistemicPending},
			expectError:  true,
			description:  "all pending children",
		},
		{
			name:         "three children one not validated",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicValidated, schema.EpistemicPending},
			expectError:  true,
			description:  "one pending among three",
		},
		{
			name:         "all validated",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicValidated, schema.EpistemicValidated},
			expectError:  false,
			description:  "all three validated is OK",
		},
		{
			name:         "all admitted",
			childStates:  []schema.EpistemicState{schema.EpistemicAdmitted, schema.EpistemicAdmitted},
			expectError:  false,
			description:  "all admitted is OK (escape hatch)",
		},
		{
			name:         "mixed validated and admitted",
			childStates:  []schema.EpistemicState{schema.EpistemicValidated, schema.EpistemicAdmitted, schema.EpistemicValidated},
			expectError:  false,
			description:  "mixed validated and admitted is OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parent node
			parentID, _ := types.Parse("1")
			parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}
			parent.EpistemicState = schema.EpistemicValidated

			// Create children with specified states
			children := make([]*node.Node, len(tt.childStates))
			for i, state := range tt.childStates {
				childID, _ := types.Parse("1." + string(rune('1'+i)))
				child, err := node.NewNode(childID, schema.NodeTypeClaim, "Child "+string(rune('1'+i)), schema.InferenceModusPonens)
				if err != nil {
					t.Fatalf("NewNode() error: %v", err)
				}
				child.EpistemicState = state
				children[i] = child
			}

			// getChildren returns the children
			getChildren := func(id types.NodeID) []*node.Node {
				if id.String() == "1" {
					return children
				}
				return nil
			}

			err = node.CheckValidationInvariant(parent, getChildren, nil)
			if tt.expectError && err == nil {
				t.Errorf("CheckValidationInvariant() = nil, want error for %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CheckValidationInvariant() = %v, want nil for %s", err, tt.description)
			}
		})
	}
}

// TestCheckValidationInvariant_NilNode tests that nil node returns nil (no violation).
func TestCheckValidationInvariant_NilNode(t *testing.T) {
	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	err := node.CheckValidationInvariant(nil, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant(nil, ...) = %v, want nil", err)
	}
}

// TestCheckValidationInvariant_NodeNotValidated tests that check doesn't apply to non-validated nodes.
func TestCheckValidationInvariant_NodeNotValidated(t *testing.T) {
	states := []schema.EpistemicState{
		schema.EpistemicPending,
		schema.EpistemicAdmitted,
		schema.EpistemicRefuted,
		schema.EpistemicArchived,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			// Create a node in non-validated state
			id, _ := types.Parse("1")
			n, err := node.NewNode(id, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}
			n.EpistemicState = state

			// Create a pending child (which would be a violation if parent were validated)
			childID, _ := types.Parse("1.1")
			child, err := node.NewNode(childID, schema.NodeTypeClaim, "Pending child", schema.InferenceModusPonens)
			if err != nil {
				t.Fatalf("NewNode() error: %v", err)
			}
			child.EpistemicState = schema.EpistemicPending

			getChildren := func(id types.NodeID) []*node.Node {
				if id.String() == "1" {
					return []*node.Node{child}
				}
				return nil
			}

			// The check should not apply to non-validated nodes
			err = node.CheckValidationInvariant(n, getChildren, nil)
			if err != nil {
				t.Errorf("CheckValidationInvariant() = %v, want nil for non-validated node in state %q", err, state)
			}
		})
	}
}

// TestCheckValidationInvariant_ErrorMessage tests that the error message contains useful details.
func TestCheckValidationInvariant_ErrorMessage(t *testing.T) {
	// Create parent node
	parentID, _ := types.Parse("1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	parent.EpistemicState = schema.EpistemicValidated

	// Create a pending child
	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Pending child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child.EpistemicState = schema.EpistemicPending

	getChildren := func(id types.NodeID) []*node.Node {
		if id.String() == "1" {
			return []*node.Node{child}
		}
		return nil
	}

	err = node.CheckValidationInvariant(parent, getChildren, nil)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	// Error should contain useful information
	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("Error message should not be empty")
	}

	// Error should mention the parent node ID
	if !containsSubstring(errMsg, "1") {
		t.Errorf("Error message should contain parent node ID '1', got: %s", errMsg)
	}
}

// TestCheckValidationInvariant_DeepHierarchy tests the invariant with a deep node hierarchy.
func TestCheckValidationInvariant_DeepHierarchy(t *testing.T) {
	// Create a deep hierarchy: 1 -> 1.1 -> 1.1.1
	// All validated except leaf

	rootID, _ := types.Parse("1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	root.EpistemicState = schema.EpistemicValidated

	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child.EpistemicState = schema.EpistemicValidated

	grandchildID, _ := types.Parse("1.1.1")
	grandchild, err := node.NewNode(grandchildID, schema.NodeTypeClaim, "Grandchild", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	grandchild.EpistemicState = schema.EpistemicPending

	// getChildren returns children for each node
	getChildren := func(id types.NodeID) []*node.Node {
		switch id.String() {
		case "1":
			return []*node.Node{child}
		case "1.1":
			return []*node.Node{grandchild}
		default:
			return nil
		}
	}

	// Root should pass - it only checks direct children which are validated
	err = node.CheckValidationInvariant(root, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant(root) = %v, want nil (direct child 1.1 is validated)", err)
	}

	// Child should fail - its direct child (grandchild) is pending
	err = node.CheckValidationInvariant(child, getChildren, nil)
	if err == nil {
		t.Error("CheckValidationInvariant(child) = nil, want error (grandchild 1.1.1 is pending)")
	}
}

// containsSubstring is a helper to check if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCheckValidationInvariant_AdmittedChild tests that a node with an admitted child can be validated.
func TestCheckValidationInvariant_AdmittedChild(t *testing.T) {
	// Create parent node
	parentID, _ := types.Parse("1")
	parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	parent.EpistemicState = schema.EpistemicValidated

	// Create an admitted child (escape hatch)
	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "Admitted child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	child.EpistemicState = schema.EpistemicAdmitted

	// getChildren returns the admitted child
	getChildren := func(id types.NodeID) []*node.Node {
		if id.String() == "1" {
			return []*node.Node{child}
		}
		return nil
	}

	// Admitted children should be allowed (escape hatch per PRD)
	err = node.CheckValidationInvariant(parent, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant() = %v, want nil for node with admitted child (escape hatch)", err)
	}
}

// TestCheckValidationInvariant_ChallengeStates tests that challenges must be in acceptable states.
func TestCheckValidationInvariant_ChallengeStates(t *testing.T) {
	// Create a validated node
	nodeID, _ := types.Parse("1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated

	// No children
	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	tests := []struct {
		name           string
		challengeState node.ChallengeStatus
		expectError    bool
	}{
		{
			name:           "open challenge should fail",
			challengeState: node.ChallengeStatusOpen,
			expectError:    true,
		},
		{
			name:           "resolved challenge should pass",
			challengeState: node.ChallengeStatusResolved,
			expectError:    false,
		},
		{
			name:           "withdrawn challenge should pass",
			challengeState: node.ChallengeStatusWithdrawn,
			expectError:    false,
		},
		{
			name:           "superseded challenge should pass",
			challengeState: node.ChallengeStatusSuperseded,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a challenge with the given status
			challenge := &node.Challenge{
				ID:       "ch-001",
				TargetID: nodeID,
				Status:   tt.challengeState,
				Reason:   "Test challenge",
			}

			getChallenges := func(id types.NodeID) []*node.Challenge {
				if id.String() == "1" {
					return []*node.Challenge{challenge}
				}
				return nil
			}

			err := node.CheckValidationInvariant(n, getChildren, getChallenges)
			if tt.expectError && err == nil {
				t.Errorf("CheckValidationInvariant() = nil, want error for %s", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CheckValidationInvariant() = %v, want nil for %s", err, tt.name)
			}
		})
	}
}

// TestCheckValidationInvariant_NoChallenges tests that a node with no challenges can be validated.
func TestCheckValidationInvariant_NoChallenges(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated

	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	getChallenges := func(id types.NodeID) []*node.Challenge {
		return nil
	}

	err = node.CheckValidationInvariant(n, getChildren, getChallenges)
	if err != nil {
		t.Errorf("CheckValidationInvariant() = %v, want nil for node with no challenges", err)
	}
}

// TestCheckValidationInvariant_MultipleChallenges tests validation with multiple challenges.
func TestCheckValidationInvariant_MultipleChallenges(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated

	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	tests := []struct {
		name        string
		statuses    []node.ChallengeStatus
		expectError bool
	}{
		{
			name:        "all resolved",
			statuses:    []node.ChallengeStatus{node.ChallengeStatusResolved, node.ChallengeStatusResolved},
			expectError: false,
		},
		{
			name:        "mixed resolved and withdrawn",
			statuses:    []node.ChallengeStatus{node.ChallengeStatusResolved, node.ChallengeStatusWithdrawn},
			expectError: false,
		},
		{
			name:        "one open among resolved",
			statuses:    []node.ChallengeStatus{node.ChallengeStatusResolved, node.ChallengeStatusOpen},
			expectError: true,
		},
		{
			name:        "all acceptable states",
			statuses:    []node.ChallengeStatus{node.ChallengeStatusResolved, node.ChallengeStatusWithdrawn, node.ChallengeStatusSuperseded},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenges := make([]*node.Challenge, len(tt.statuses))
			for i, status := range tt.statuses {
				challenges[i] = &node.Challenge{
					ID:       "ch-00" + string(rune('1'+i)),
					TargetID: nodeID,
					Status:   status,
					Reason:   "Test challenge",
				}
			}

			getChallenges := func(id types.NodeID) []*node.Challenge {
				if id.String() == "1" {
					return challenges
				}
				return nil
			}

			err := node.CheckValidationInvariant(n, getChildren, getChallenges)
			if tt.expectError && err == nil {
				t.Errorf("CheckValidationInvariant() = nil, want error for %s", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CheckValidationInvariant() = %v, want nil for %s", err, tt.name)
			}
		})
	}
}

// TestCheckValidationInvariant_NilChallengesFunc tests that nil getChallenges func is handled gracefully.
func TestCheckValidationInvariant_NilChallengesFunc(t *testing.T) {
	nodeID, _ := types.Parse("1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode() error: %v", err)
	}
	n.EpistemicState = schema.EpistemicValidated

	getChildren := func(id types.NodeID) []*node.Node {
		return nil
	}

	// Passing nil for getChallenges should be handled gracefully (skips challenge check)
	err = node.CheckValidationInvariant(n, getChildren, nil)
	if err != nil {
		t.Errorf("CheckValidationInvariant() = %v, want nil for nil getChallenges", err)
	}
}
