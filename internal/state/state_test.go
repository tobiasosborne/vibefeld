// Package state provides derived state from replaying ledger events.
package state

import (
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewState verifies that NewState creates an empty state with all maps initialized.
func TestNewState(t *testing.T) {
	s := NewState()

	if s == nil {
		t.Fatal("NewState() returned nil")
	}

	// Verify all maps are initialized (not nil) but empty
	// We test this indirectly by attempting to get non-existent items
	// which should return nil without panicking

	// These calls should not panic and should return nil
	if got := s.GetNode(mustParseNodeID(t, "1")); got != nil {
		t.Errorf("GetNode on empty state returned non-nil: %v", got)
	}
	if got := s.GetDefinition("def-1"); got != nil {
		t.Errorf("GetDefinition on empty state returned non-nil: %v", got)
	}
	if got := s.GetAssumption("asm-1"); got != nil {
		t.Errorf("GetAssumption on empty state returned non-nil: %v", got)
	}
	if got := s.GetExternal("ext-1"); got != nil {
		t.Errorf("GetExternal on empty state returned non-nil: %v", got)
	}
	if got := s.GetLemma("lem-1"); got != nil {
		t.Errorf("GetLemma on empty state returned non-nil: %v", got)
	}
}

// TestAddAndGetNode verifies adding and retrieving nodes by ID.
func TestAddAndGetNode(t *testing.T) {
	s := NewState()

	// Create a test node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}

	// Add the node
	s.AddNode(n)

	// Retrieve the node
	got := s.GetNode(nodeID)
	if got == nil {
		t.Fatal("GetNode returned nil after AddNode")
	}

	// Verify it's the same node
	if got.ID.String() != n.ID.String() {
		t.Errorf("GetNode returned wrong node: got ID %s, want %s", got.ID.String(), n.ID.String())
	}
	if got.Statement != n.Statement {
		t.Errorf("GetNode returned wrong statement: got %q, want %q", got.Statement, n.Statement)
	}
}

// TestAddAndGetDefinition verifies adding and retrieving definitions by ID.
func TestAddAndGetDefinition(t *testing.T) {
	s := NewState()

	// Create a test definition
	def, err := node.NewDefinition("TestDef", "A test definition content")
	if err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	// Add the definition
	s.AddDefinition(def)

	// Retrieve the definition
	got := s.GetDefinition(def.ID)
	if got == nil {
		t.Fatal("GetDefinition returned nil after AddDefinition")
	}

	// Verify it's the same definition
	if got.ID != def.ID {
		t.Errorf("GetDefinition returned wrong definition: got ID %s, want %s", got.ID, def.ID)
	}
	if got.Name != def.Name {
		t.Errorf("GetDefinition returned wrong name: got %q, want %q", got.Name, def.Name)
	}
	if got.Content != def.Content {
		t.Errorf("GetDefinition returned wrong content: got %q, want %q", got.Content, def.Content)
	}
}

// TestAddAndGetAssumption verifies adding and retrieving assumptions by ID.
func TestAddAndGetAssumption(t *testing.T) {
	s := NewState()

	// Create a test assumption
	asm, _ := node.NewAssumption("Test assumption statement")

	// Add the assumption
	s.AddAssumption(asm)

	// Retrieve the assumption
	got := s.GetAssumption(asm.ID)
	if got == nil {
		t.Fatal("GetAssumption returned nil after AddAssumption")
	}

	// Verify it's the same assumption
	if got.ID != asm.ID {
		t.Errorf("GetAssumption returned wrong assumption: got ID %s, want %s", got.ID, asm.ID)
	}
	if got.Statement != asm.Statement {
		t.Errorf("GetAssumption returned wrong statement: got %q, want %q", got.Statement, asm.Statement)
	}
}

// TestAddAndGetExternal verifies adding and retrieving externals by ID.
func TestAddAndGetExternal(t *testing.T) {
	s := NewState()

	// Create a test external
	ext, _ := node.NewExternal("Fermat's Last Theorem", "Wiles, A. (1995)")

	// Add the external
	s.AddExternal(&ext)

	// Retrieve the external
	got := s.GetExternal(ext.ID)
	if got == nil {
		t.Fatal("GetExternal returned nil after AddExternal")
	}

	// Verify it's the same external
	if got.ID != ext.ID {
		t.Errorf("GetExternal returned wrong external: got ID %s, want %s", got.ID, ext.ID)
	}
	if got.Name != ext.Name {
		t.Errorf("GetExternal returned wrong name: got %q, want %q", got.Name, ext.Name)
	}
	if got.Source != ext.Source {
		t.Errorf("GetExternal returned wrong source: got %q, want %q", got.Source, ext.Source)
	}
}

// TestGetExternalByName verifies retrieving externals by name.
func TestGetExternalByName(t *testing.T) {
	s := NewState()

	// Create and add an external
	ext, _ := node.NewExternal("Fermat-last-theorem", "Wiles, A. (1995)")
	s.AddExternal(&ext)

	// Retrieve by name
	got := s.GetExternalByName("Fermat-last-theorem")
	if got == nil {
		t.Fatal("GetExternalByName returned nil for existing external")
	}
	if got.ID != ext.ID {
		t.Errorf("GetExternalByName returned wrong external: got ID %s, want %s", got.ID, ext.ID)
	}
	if got.Name != ext.Name {
		t.Errorf("GetExternalByName returned wrong name: got %q, want %q", got.Name, ext.Name)
	}

	// Verify non-existent name returns nil
	notFound := s.GetExternalByName("non-existent")
	if notFound != nil {
		t.Errorf("GetExternalByName for non-existent name returned non-nil: %v", notFound)
	}
}

// TestGetExternalByName_MultipleExternals verifies GetExternalByName with multiple externals.
func TestGetExternalByName_MultipleExternals(t *testing.T) {
	s := NewState()

	// Add multiple externals
	ext1, _ := node.NewExternal("ZFC", "Zermelo-Fraenkel set theory")
	ext2, _ := node.NewExternal("AC", "Axiom of Choice")
	ext3, _ := node.NewExternal("CH", "Continuum Hypothesis")
	s.AddExternal(&ext1)
	s.AddExternal(&ext2)
	s.AddExternal(&ext3)

	// Verify each can be found by name
	tests := []struct {
		name     string
		expected *node.External
	}{
		{"ZFC", &ext1},
		{"AC", &ext2},
		{"CH", &ext3},
	}

	for _, tt := range tests {
		got := s.GetExternalByName(tt.name)
		if got == nil {
			t.Errorf("GetExternalByName(%q) returned nil", tt.name)
			continue
		}
		if got.ID != tt.expected.ID {
			t.Errorf("GetExternalByName(%q) returned wrong external: got ID %s, want %s", tt.name, got.ID, tt.expected.ID)
		}
	}
}

// TestAddAndGetLemma verifies adding and retrieving lemmas by ID.
func TestAddAndGetLemma(t *testing.T) {
	s := NewState()

	// Create a test lemma
	sourceNodeID := mustParseNodeID(t, "1.1")
	lem, err := node.NewLemma("Test lemma statement", sourceNodeID)
	if err != nil {
		t.Fatalf("Failed to create test lemma: %v", err)
	}

	// Add the lemma
	s.AddLemma(lem)

	// Retrieve the lemma
	got := s.GetLemma(lem.ID)
	if got == nil {
		t.Fatal("GetLemma returned nil after AddLemma")
	}

	// Verify it's the same lemma
	if got.ID != lem.ID {
		t.Errorf("GetLemma returned wrong lemma: got ID %s, want %s", got.ID, lem.ID)
	}
	if got.Statement != lem.Statement {
		t.Errorf("GetLemma returned wrong statement: got %q, want %q", got.Statement, lem.Statement)
	}
	if got.SourceNodeID.String() != lem.SourceNodeID.String() {
		t.Errorf("GetLemma returned wrong source node ID: got %s, want %s", got.SourceNodeID.String(), lem.SourceNodeID.String())
	}
}

// TestGetNonExistentNode verifies that getting a non-existent node returns nil.
func TestGetNonExistentNode(t *testing.T) {
	s := NewState()

	// Add a node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	s.AddNode(n)

	// Try to get a different node
	nonExistentID := mustParseNodeID(t, "1.1")
	got := s.GetNode(nonExistentID)
	if got != nil {
		t.Errorf("GetNode for non-existent ID returned non-nil: %v", got)
	}
}

// TestGetNonExistentDefinition verifies that getting a non-existent definition returns nil.
func TestGetNonExistentDefinition(t *testing.T) {
	s := NewState()

	// Add a definition
	def, err := node.NewDefinition("ExistingDef", "Content")
	if err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}
	s.AddDefinition(def)

	// Try to get a different definition
	got := s.GetDefinition("non-existent-id")
	if got != nil {
		t.Errorf("GetDefinition for non-existent ID returned non-nil: %v", got)
	}
}

// TestGetNonExistentAssumption verifies that getting a non-existent assumption returns nil.
func TestGetNonExistentAssumption(t *testing.T) {
	s := NewState()

	// Add an assumption
	asm, _ := node.NewAssumption("Existing assumption")
	s.AddAssumption(asm)

	// Try to get a different assumption
	got := s.GetAssumption("non-existent-id")
	if got != nil {
		t.Errorf("GetAssumption for non-existent ID returned non-nil: %v", got)
	}
}

// TestGetNonExistentExternal verifies that getting a non-existent external returns nil.
func TestGetNonExistentExternal(t *testing.T) {
	s := NewState()

	// Add an external
	ext, _ := node.NewExternal("Existing", "Source")
	s.AddExternal(&ext)

	// Try to get a different external
	got := s.GetExternal("non-existent-id")
	if got != nil {
		t.Errorf("GetExternal for non-existent ID returned non-nil: %v", got)
	}
}

// TestGetNonExistentLemma verifies that getting a non-existent lemma returns nil.
func TestGetNonExistentLemma(t *testing.T) {
	s := NewState()

	// Add a lemma
	sourceNodeID := mustParseNodeID(t, "1")
	lem, err := node.NewLemma("Existing lemma", sourceNodeID)
	if err != nil {
		t.Fatalf("Failed to create test lemma: %v", err)
	}
	s.AddLemma(lem)

	// Try to get a different lemma
	got := s.GetLemma("non-existent-id")
	if got != nil {
		t.Errorf("GetLemma for non-existent ID returned non-nil: %v", got)
	}
}

// TestDuplicateNodeOverwrites verifies that adding a node with an existing ID overwrites the previous one.
func TestDuplicateNodeOverwrites(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// Add first node
	n1, err := node.NewNode(nodeID, schema.NodeTypeClaim, "First statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create first node: %v", err)
	}
	s.AddNode(n1)

	// Add second node with same ID but different statement
	n2, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Second statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create second node: %v", err)
	}
	s.AddNode(n2)

	// Retrieve and verify it's the second node
	got := s.GetNode(nodeID)
	if got == nil {
		t.Fatal("GetNode returned nil after duplicate AddNode")
	}
	if got.Statement != "Second statement" {
		t.Errorf("Duplicate node did not overwrite: got statement %q, want %q", got.Statement, "Second statement")
	}
}

// TestDuplicateDefinitionOverwrites verifies that adding a definition with an existing ID overwrites the previous one.
func TestDuplicateDefinitionOverwrites(t *testing.T) {
	s := NewState()

	// Create first definition and record its ID
	def1, err := node.NewDefinition("First", "First content")
	if err != nil {
		t.Fatalf("Failed to create first definition: %v", err)
	}
	originalID := def1.ID
	s.AddDefinition(def1)

	// Create second definition with same ID (simulating event replay)
	def2, err := node.NewDefinition("Second", "Second content")
	if err != nil {
		t.Fatalf("Failed to create second definition: %v", err)
	}
	def2.ID = originalID // Use the same ID
	s.AddDefinition(def2)

	// Retrieve and verify it's the second definition
	got := s.GetDefinition(originalID)
	if got == nil {
		t.Fatal("GetDefinition returned nil after duplicate AddDefinition")
	}
	if got.Name != "Second" {
		t.Errorf("Duplicate definition did not overwrite: got name %q, want %q", got.Name, "Second")
	}
}

// TestDuplicateAssumptionOverwrites verifies that adding an assumption with an existing ID overwrites the previous one.
func TestDuplicateAssumptionOverwrites(t *testing.T) {
	s := NewState()

	// Create first assumption and record its ID
	asm1, _ := node.NewAssumption("First assumption")
	originalID := asm1.ID
	s.AddAssumption(asm1)

	// Create second assumption with same ID
	asm2, _ := node.NewAssumption("Second assumption")
	asm2.ID = originalID
	s.AddAssumption(asm2)

	// Retrieve and verify it's the second assumption
	got := s.GetAssumption(originalID)
	if got == nil {
		t.Fatal("GetAssumption returned nil after duplicate AddAssumption")
	}
	if got.Statement != "Second assumption" {
		t.Errorf("Duplicate assumption did not overwrite: got statement %q, want %q", got.Statement, "Second assumption")
	}
}

// TestDuplicateExternalOverwrites verifies that adding an external with an existing ID overwrites the previous one.
func TestDuplicateExternalOverwrites(t *testing.T) {
	s := NewState()

	// Create first external and record its ID
	ext1, _ := node.NewExternal("First", "First source")
	originalID := ext1.ID
	s.AddExternal(&ext1)

	// Create second external with same ID
	ext2, _ := node.NewExternal("Second", "Second source")
	ext2.ID = originalID
	s.AddExternal(&ext2)

	// Retrieve and verify it's the second external
	got := s.GetExternal(originalID)
	if got == nil {
		t.Fatal("GetExternal returned nil after duplicate AddExternal")
	}
	if got.Name != "Second" {
		t.Errorf("Duplicate external did not overwrite: got name %q, want %q", got.Name, "Second")
	}
}

// TestDuplicateLemmaOverwrites verifies that adding a lemma with an existing ID overwrites the previous one.
func TestDuplicateLemmaOverwrites(t *testing.T) {
	s := NewState()

	sourceNodeID := mustParseNodeID(t, "1")

	// Create first lemma and record its ID
	lem1, err := node.NewLemma("First lemma", sourceNodeID)
	if err != nil {
		t.Fatalf("Failed to create first lemma: %v", err)
	}
	originalID := lem1.ID
	s.AddLemma(lem1)

	// Create second lemma with same ID
	lem2, err := node.NewLemma("Second lemma", sourceNodeID)
	if err != nil {
		t.Fatalf("Failed to create second lemma: %v", err)
	}
	lem2.ID = originalID
	s.AddLemma(lem2)

	// Retrieve and verify it's the second lemma
	got := s.GetLemma(originalID)
	if got == nil {
		t.Fatal("GetLemma returned nil after duplicate AddLemma")
	}
	if got.Statement != "Second lemma" {
		t.Errorf("Duplicate lemma did not overwrite: got statement %q, want %q", got.Statement, "Second lemma")
	}
}

// TestMultipleNodes verifies that multiple nodes can be added and retrieved.
func TestMultipleNodes(t *testing.T) {
	s := NewState()

	// Create and add multiple nodes
	nodeIDs := []string{"1", "1.1", "1.2", "1.1.1"}
	nodes := make(map[string]*node.Node)

	for _, idStr := range nodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Statement for "+idStr, schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", idStr, err)
		}
		nodes[idStr] = n
		s.AddNode(n)
	}

	// Verify all nodes can be retrieved
	for idStr, expectedNode := range nodes {
		nodeID := mustParseNodeID(t, idStr)
		got := s.GetNode(nodeID)
		if got == nil {
			t.Errorf("GetNode(%s) returned nil", idStr)
			continue
		}
		if got.Statement != expectedNode.Statement {
			t.Errorf("GetNode(%s) returned wrong statement: got %q, want %q", idStr, got.Statement, expectedNode.Statement)
		}
	}
}

// TestMultipleDefinitions verifies that multiple definitions can be added and retrieved.
func TestMultipleDefinitions(t *testing.T) {
	s := NewState()

	// Create and add multiple definitions
	defNames := []string{"Def1", "Def2", "Def3"}
	defs := make(map[string]*node.Definition)

	for _, name := range defNames {
		def, err := node.NewDefinition(name, "Content for "+name)
		if err != nil {
			t.Fatalf("Failed to create definition %s: %v", name, err)
		}
		defs[def.ID] = def
		s.AddDefinition(def)
	}

	// Verify all definitions can be retrieved
	for id, expectedDef := range defs {
		got := s.GetDefinition(id)
		if got == nil {
			t.Errorf("GetDefinition(%s) returned nil", id)
			continue
		}
		if got.Name != expectedDef.Name {
			t.Errorf("GetDefinition(%s) returned wrong name: got %q, want %q", id, got.Name, expectedDef.Name)
		}
	}
}

// TestMixedEntities verifies that different entity types can coexist in the state.
func TestMixedEntities(t *testing.T) {
	s := NewState()

	// Add a node
	nodeID := mustParseNodeID(t, "1")
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	s.AddNode(n)

	// Add a definition
	def, err := node.NewDefinition("TestDef", "Test content")
	if err != nil {
		t.Fatalf("Failed to create definition: %v", err)
	}
	s.AddDefinition(def)

	// Add an assumption
	asm, _ := node.NewAssumption("Test assumption")
	s.AddAssumption(asm)

	// Add an external
	ext, _ := node.NewExternal("TestExt", "Test source")
	s.AddExternal(&ext)

	// Add a lemma
	lem, err := node.NewLemma("Test lemma", nodeID)
	if err != nil {
		t.Fatalf("Failed to create lemma: %v", err)
	}
	s.AddLemma(lem)

	// Verify all entities can be retrieved
	if got := s.GetNode(nodeID); got == nil {
		t.Error("GetNode returned nil for added node")
	}
	if got := s.GetDefinition(def.ID); got == nil {
		t.Error("GetDefinition returned nil for added definition")
	}
	if got := s.GetAssumption(asm.ID); got == nil {
		t.Error("GetAssumption returned nil for added assumption")
	}
	if got := s.GetExternal(ext.ID); got == nil {
		t.Error("GetExternal returned nil for added external")
	}
	if got := s.GetLemma(lem.ID); got == nil {
		t.Error("GetLemma returned nil for added lemma")
	}
}

// mustParseNodeID is a test helper that parses a NodeID string or fails the test.
func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// TestAllChildrenValidated verifies the AllChildrenValidated method.
func TestAllChildrenValidated(t *testing.T) {
	tests := []struct {
		name           string
		setupNodes     func(s *State, t *testing.T) types.NodeID // returns parent ID to check
		expectedResult bool
	}{
		{
			name: "no children returns true",
			setupNodes: func(s *State, t *testing.T) types.NodeID {
				parentID := mustParseNodeID(t, "1")
				n, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create node: %v", err)
				}
				s.AddNode(n)
				return parentID
			},
			expectedResult: true,
		},
		{
			name: "all children validated returns true",
			setupNodes: func(s *State, t *testing.T) types.NodeID {
				// Add parent
				parentID := mustParseNodeID(t, "1")
				parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create parent: %v", err)
				}
				s.AddNode(parent)

				// Add validated child 1.1
				child1ID := mustParseNodeID(t, "1.1")
				child1, err := node.NewNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child1: %v", err)
				}
				child1.EpistemicState = schema.EpistemicValidated
				s.AddNode(child1)

				// Add validated child 1.2
				child2ID := mustParseNodeID(t, "1.2")
				child2, err := node.NewNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child2: %v", err)
				}
				child2.EpistemicState = schema.EpistemicValidated
				s.AddNode(child2)

				return parentID
			},
			expectedResult: true,
		},
		{
			name: "one pending child returns false",
			setupNodes: func(s *State, t *testing.T) types.NodeID {
				// Add parent
				parentID := mustParseNodeID(t, "1")
				parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create parent: %v", err)
				}
				s.AddNode(parent)

				// Add validated child 1.1
				child1ID := mustParseNodeID(t, "1.1")
				child1, err := node.NewNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child1: %v", err)
				}
				child1.EpistemicState = schema.EpistemicValidated
				s.AddNode(child1)

				// Add pending child 1.2 (default state)
				child2ID := mustParseNodeID(t, "1.2")
				child2, err := node.NewNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child2: %v", err)
				}
				// child2 is pending by default
				s.AddNode(child2)

				return parentID
			},
			expectedResult: false,
		},
		{
			name: "grandchildren do not affect parent check",
			setupNodes: func(s *State, t *testing.T) types.NodeID {
				// Add parent
				parentID := mustParseNodeID(t, "1")
				parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create parent: %v", err)
				}
				s.AddNode(parent)

				// Add validated child 1.1
				child1ID := mustParseNodeID(t, "1.1")
				child1, err := node.NewNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child1: %v", err)
				}
				child1.EpistemicState = schema.EpistemicValidated
				s.AddNode(child1)

				// Add pending grandchild 1.1.1 (should not affect check on parent 1)
				grandchildID := mustParseNodeID(t, "1.1.1")
				grandchild, err := node.NewNode(grandchildID, schema.NodeTypeClaim, "Grandchild", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create grandchild: %v", err)
				}
				// grandchild is pending by default
				s.AddNode(grandchild)

				return parentID
			},
			expectedResult: true,
		},
		{
			name: "admitted child returns false",
			setupNodes: func(s *State, t *testing.T) types.NodeID {
				// Add parent
				parentID := mustParseNodeID(t, "1")
				parent, err := node.NewNode(parentID, schema.NodeTypeClaim, "Parent", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create parent: %v", err)
				}
				s.AddNode(parent)

				// Add admitted child (not validated)
				child1ID := mustParseNodeID(t, "1.1")
				child1, err := node.NewNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
				if err != nil {
					t.Fatalf("Failed to create child1: %v", err)
				}
				child1.EpistemicState = schema.EpistemicAdmitted
				s.AddNode(child1)

				return parentID
			},
			expectedResult: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState()
			parentID := tc.setupNodes(s, t)
			got := s.AllChildrenValidated(parentID)
			if got != tc.expectedResult {
				t.Errorf("AllChildrenValidated(%s) = %v, want %v", parentID, got, tc.expectedResult)
			}
		})
	}
}

// TestGetBlockingChallengesForNode_ReturnsCritical verifies that critical challenges are returned.
func TestGetBlockingChallengesForNode_ReturnsCritical(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a critical challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Unclear assumption",
		Status:   "open",
		Severity: "critical",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 1 {
		t.Fatalf("GetBlockingChallengesForNode returned %d challenges, want 1", len(got))
	}
	if got[0].ID != "ch-1" {
		t.Errorf("GetBlockingChallengesForNode returned wrong challenge: got ID %s, want ch-1", got[0].ID)
	}
}

// TestGetBlockingChallengesForNode_ReturnsMajor verifies that major challenges are returned.
func TestGetBlockingChallengesForNode_ReturnsMajor(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a major challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Missing justification",
		Status:   "open",
		Severity: "major",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 1 {
		t.Fatalf("GetBlockingChallengesForNode returned %d challenges, want 1", len(got))
	}
	if got[0].ID != "ch-1" {
		t.Errorf("GetBlockingChallengesForNode returned wrong challenge: got ID %s, want ch-1", got[0].ID)
	}
}

// TestGetBlockingChallengesForNode_ExcludesMinor verifies that minor challenges are not returned.
func TestGetBlockingChallengesForNode_ExcludesMinor(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge (should be excluded)
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "style",
		Reason:   "Could be clearer",
		Status:   "open",
		Severity: "minor",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 0 {
		t.Errorf("GetBlockingChallengesForNode returned %d challenges, want 0 (minor should be excluded)", len(got))
	}
}

// TestGetBlockingChallengesForNode_ExcludesNote verifies that note challenges are not returned.
func TestGetBlockingChallengesForNode_ExcludesNote(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a note challenge (should be excluded)
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "clarification",
		Reason:   "Consider adding more detail",
		Status:   "open",
		Severity: "note",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 0 {
		t.Errorf("GetBlockingChallengesForNode returned %d challenges, want 0 (note should be excluded)", len(got))
	}
}

// TestGetBlockingChallengesForNode_ExcludesResolved verifies that resolved challenges are not returned.
func TestGetBlockingChallengesForNode_ExcludesResolved(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a resolved critical challenge (should be excluded because it's resolved)
	s.AddChallenge(&Challenge{
		ID:         "ch-1",
		NodeID:     nodeID,
		Target:     "statement",
		Reason:     "Was unclear",
		Status:     "resolved",
		Severity:   "critical",
		Resolution: "Fixed the statement",
	})

	// Add a withdrawn major challenge (should also be excluded)
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Thought it was missing",
		Status:   "withdrawn",
		Severity: "major",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 0 {
		t.Errorf("GetBlockingChallengesForNode returned %d challenges, want 0 (resolved/withdrawn should be excluded)", len(got))
	}
}

// TestGetBlockingChallengesForNode_EmptyForNoNode verifies empty result for node with no challenges.
func TestGetBlockingChallengesForNode_EmptyForNoNode(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")
	otherNodeID := mustParseNodeID(t, "1.1")

	// Add challenges on a different node
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   otherNodeID,
		Target:   "statement",
		Reason:   "Some issue",
		Status:   "open",
		Severity: "critical",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 0 {
		t.Errorf("GetBlockingChallengesForNode returned %d challenges, want 0 (no challenges on this node)", len(got))
	}
}

// TestGetBlockingChallengesForNode_MixedSeverities verifies correct filtering with mixed severities.
func TestGetBlockingChallengesForNode_MixedSeverities(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add challenges of all severity levels
	s.AddChallenge(&Challenge{
		ID:       "ch-critical",
		NodeID:   nodeID,
		Status:   "open",
		Severity: "critical",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-major",
		NodeID:   nodeID,
		Status:   "open",
		Severity: "major",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-minor",
		NodeID:   nodeID,
		Status:   "open",
		Severity: "minor",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-note",
		NodeID:   nodeID,
		Status:   "open",
		Severity: "note",
	})
	// Add a resolved critical to verify status check
	s.AddChallenge(&Challenge{
		ID:       "ch-resolved",
		NodeID:   nodeID,
		Status:   "resolved",
		Severity: "critical",
	})

	got := s.GetBlockingChallengesForNode(nodeID)
	if len(got) != 2 {
		t.Fatalf("GetBlockingChallengesForNode returned %d challenges, want 2", len(got))
	}

	// Verify we got critical and major
	foundCritical := false
	foundMajor := false
	for _, c := range got {
		switch c.ID {
		case "ch-critical":
			foundCritical = true
		case "ch-major":
			foundMajor = true
		default:
			t.Errorf("Unexpected challenge returned: %s", c.ID)
		}
	}
	if !foundCritical {
		t.Error("Critical challenge not found in result")
	}
	if !foundMajor {
		t.Error("Major challenge not found in result")
	}
}

// TestHasBlockingChallenges_TrueForCritical verifies HasBlockingChallenges returns true
// when there is a critical challenge on the node.
func TestHasBlockingChallenges_TrueForCritical(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a critical challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Unclear assumption",
		Status:   "open",
		Severity: "critical",
	})

	got := s.HasBlockingChallenges(nodeID)
	if !got {
		t.Errorf("HasBlockingChallenges(%s) = false, want true (critical challenge exists)", nodeID)
	}
}

// TestHasBlockingChallenges_TrueForMajor verifies HasBlockingChallenges returns true
// when there is a major challenge on the node.
func TestHasBlockingChallenges_TrueForMajor(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a major challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Missing justification",
		Status:   "open",
		Severity: "major",
	})

	got := s.HasBlockingChallenges(nodeID)
	if !got {
		t.Errorf("HasBlockingChallenges(%s) = false, want true (major challenge exists)", nodeID)
	}
}

// TestHasBlockingChallenges_FalseForMinor verifies HasBlockingChallenges returns false
// when there are only minor challenges on the node.
func TestHasBlockingChallenges_FalseForMinor(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "style",
		Reason:   "Could be clearer",
		Status:   "open",
		Severity: "minor",
	})

	// Add a note challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   nodeID,
		Target:   "clarification",
		Reason:   "Consider adding more detail",
		Status:   "open",
		Severity: "note",
	})

	got := s.HasBlockingChallenges(nodeID)
	if got {
		t.Errorf("HasBlockingChallenges(%s) = true, want false (only minor/note challenges exist)", nodeID)
	}
}

// TestHasBlockingChallenges_FalseForNoChallenges verifies HasBlockingChallenges returns false
// when there are no challenges on the node.
func TestHasBlockingChallenges_FalseForNoChallenges(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")
	otherNodeID := mustParseNodeID(t, "1.1")

	// Add challenges on a different node
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   otherNodeID,
		Target:   "statement",
		Reason:   "Some issue",
		Status:   "open",
		Severity: "critical",
	})

	got := s.HasBlockingChallenges(nodeID)
	if got {
		t.Errorf("HasBlockingChallenges(%s) = true, want false (no challenges on this node)", nodeID)
	}
}

// TestHasBlockingChallenges_FalseForResolved verifies HasBlockingChallenges returns false
// when all blocking challenges have been resolved.
func TestHasBlockingChallenges_FalseForResolved(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a resolved critical challenge
	s.AddChallenge(&Challenge{
		ID:         "ch-1",
		NodeID:     nodeID,
		Target:     "statement",
		Reason:     "Was unclear",
		Status:     "resolved",
		Severity:   "critical",
		Resolution: "Fixed the statement",
	})

	// Add a withdrawn major challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Thought it was missing",
		Status:   "withdrawn",
		Severity: "major",
	})

	got := s.HasBlockingChallenges(nodeID)
	if got {
		t.Errorf("HasBlockingChallenges(%s) = true, want false (all blocking challenges resolved/withdrawn)", nodeID)
	}
}

// TestVerifierRaisedChallengeForNode_TrueIfRaised verifies that the method returns true
// when the specified agent has raised a challenge for the node.
func TestVerifierRaisedChallengeForNode_TrueIfRaised(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge raised by agent "verifier-1"
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Unclear assumption",
		Status:   "open",
		Severity: "major",
		RaisedBy: "verifier-1",
	})

	got := s.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if !got {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = false, want true (agent raised a challenge)", nodeID, "verifier-1")
	}
}

// TestVerifierRaisedChallengeForNode_TrueEvenIfResolved verifies that the method returns true
// even when the challenge has been resolved.
func TestVerifierRaisedChallengeForNode_TrueEvenIfResolved(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a resolved challenge raised by agent "verifier-1"
	s.AddChallenge(&Challenge{
		ID:         "ch-1",
		NodeID:     nodeID,
		Target:     "statement",
		Reason:     "Was unclear",
		Status:     "resolved",
		Severity:   "major",
		Resolution: "Statement clarified",
		RaisedBy:   "verifier-1",
	})

	got := s.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if !got {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = false, want true (agent raised a challenge, now resolved)", nodeID, "verifier-1")
	}

	// Also test with withdrawn status
	s2 := NewState()
	s2.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Thought it was wrong",
		Status:   "withdrawn",
		Severity: "minor",
		RaisedBy: "verifier-1",
	})

	got2 := s2.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if !got2 {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = false, want true (agent raised a challenge, now withdrawn)", nodeID, "verifier-1")
	}
}

// TestVerifierRaisedChallengeForNode_FalseForDifferentAgent verifies that the method returns false
// when a different agent raised the challenge.
func TestVerifierRaisedChallengeForNode_FalseForDifferentAgent(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge raised by agent "verifier-1"
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Unclear assumption",
		Status:   "open",
		Severity: "major",
		RaisedBy: "verifier-1",
	})

	// Check for a different agent
	got := s.VerifierRaisedChallengeForNode(nodeID, "verifier-2")
	if got {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = true, want false (different agent raised the challenge)", nodeID, "verifier-2")
	}
}

// TestVerifierRaisedChallengeForNode_FalseForDifferentNode verifies that the method returns false
// when the challenge is for a different node.
func TestVerifierRaisedChallengeForNode_FalseForDifferentNode(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")
	otherNodeID := mustParseNodeID(t, "1.1")

	// Add a challenge on a different node
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   otherNodeID,
		Target:   "statement",
		Reason:   "Some issue",
		Status:   "open",
		Severity: "major",
		RaisedBy: "verifier-1",
	})

	got := s.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if got {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = true, want false (challenge is for different node)", nodeID, "verifier-1")
	}
}

// TestVerifierRaisedChallengeForNode_FalseWhenNoRaisedBy verifies that the method returns false
// when the challenge has no RaisedBy field set.
func TestVerifierRaisedChallengeForNode_FalseWhenNoRaisedBy(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add a challenge without RaisedBy (legacy challenge)
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Some issue",
		Status:   "open",
		Severity: "major",
		RaisedBy: "", // No agent specified
	})

	got := s.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if got {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = true, want false (challenge has no RaisedBy)", nodeID, "verifier-1")
	}
}

// TestVerifierRaisedChallengeForNode_MultipleChallenges verifies correct behavior
// with multiple challenges from different agents.
func TestVerifierRaisedChallengeForNode_MultipleChallenges(t *testing.T) {
	s := NewState()
	nodeID := mustParseNodeID(t, "1")

	// Add challenges from multiple agents
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   nodeID,
		Target:   "statement",
		Reason:   "Issue 1",
		Status:   "resolved",
		Severity: "major",
		RaisedBy: "verifier-1",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   nodeID,
		Target:   "inference",
		Reason:   "Issue 2",
		Status:   "open",
		Severity: "minor",
		RaisedBy: "verifier-2",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-3",
		NodeID:   nodeID,
		Target:   "context",
		Reason:   "Issue 3",
		Status:   "withdrawn",
		Severity: "note",
		RaisedBy: "verifier-1",
	})

	// Verifier-1 raised challenges (ch-1 and ch-3)
	got1 := s.VerifierRaisedChallengeForNode(nodeID, "verifier-1")
	if !got1 {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = false, want true", nodeID, "verifier-1")
	}

	// Verifier-2 raised a challenge (ch-2)
	got2 := s.VerifierRaisedChallengeForNode(nodeID, "verifier-2")
	if !got2 {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = false, want true", nodeID, "verifier-2")
	}

	// Verifier-3 never raised any challenges
	got3 := s.VerifierRaisedChallengeForNode(nodeID, "verifier-3")
	if got3 {
		t.Errorf("VerifierRaisedChallengeForNode(%s, %q) = true, want false", nodeID, "verifier-3")
	}
}

// TestChallengesByNodeID verifies the cached challenge lookup by node ID.
func TestChallengesByNodeID(t *testing.T) {
	s := NewState()

	// Create test nodes using hierarchical IDs
	node1ID := mustParseNodeID(t, "1")
	node2ID := mustParseNodeID(t, "1.1")

	// Add challenges to different nodes
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   node1ID,
		Reason:   "Issue 1 on node 1",
		Status:   "open",
		Severity: "major",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   node1ID,
		Reason:   "Issue 2 on node 1",
		Status:   "resolved",
		Severity: "minor",
	})
	s.AddChallenge(&Challenge{
		ID:       "ch-3",
		NodeID:   node2ID,
		Reason:   "Issue 1 on node 1.1",
		Status:   "open",
		Severity: "critical",
	})

	// Test ChallengesByNodeID
	byNode := s.ChallengesByNodeID()
	if byNode == nil {
		t.Fatal("ChallengesByNodeID returned nil")
	}

	// Node 1 should have 2 challenges
	if len(byNode[node1ID.String()]) != 2 {
		t.Errorf("Expected 2 challenges for node 1, got %d", len(byNode[node1ID.String()]))
	}

	// Node 1.1 should have 1 challenge
	if len(byNode[node2ID.String()]) != 1 {
		t.Errorf("Expected 1 challenge for node 1.1, got %d", len(byNode[node2ID.String()]))
	}

	// Test GetChallengesForNode
	node1Challenges := s.GetChallengesForNode(node1ID)
	if len(node1Challenges) != 2 {
		t.Errorf("Expected 2 challenges for node 1, got %d", len(node1Challenges))
	}

	// Non-existent node should return nil (not panic)
	node3ID := mustParseNodeID(t, "1.2")
	node3Challenges := s.GetChallengesForNode(node3ID)
	if node3Challenges != nil {
		t.Errorf("Expected nil for non-existent node, got %v", node3Challenges)
	}
}

// TestChallengesByNodeIDCacheInvalidation verifies that adding a challenge invalidates the cache.
func TestChallengesByNodeIDCacheInvalidation(t *testing.T) {
	s := NewState()

	node1ID := mustParseNodeID(t, "1")

	// Add first challenge
	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   node1ID,
		Reason:   "Issue 1",
		Status:   "open",
		Severity: "major",
	})

	// Get the cache
	byNode1 := s.ChallengesByNodeID()
	if len(byNode1[node1ID.String()]) != 1 {
		t.Errorf("Expected 1 challenge, got %d", len(byNode1[node1ID.String()]))
	}

	// Add second challenge (should invalidate cache)
	s.AddChallenge(&Challenge{
		ID:       "ch-2",
		NodeID:   node1ID,
		Reason:   "Issue 2",
		Status:   "open",
		Severity: "minor",
	})

	// Get the cache again (should be rebuilt)
	byNode2 := s.ChallengesByNodeID()
	if len(byNode2[node1ID.String()]) != 2 {
		t.Errorf("Expected 2 challenges after cache invalidation, got %d", len(byNode2[node1ID.String()]))
	}
}

// TestChallengeMapForJobs verifies the conversion to node.Challenge format.
func TestChallengeMapForJobs(t *testing.T) {
	s := NewState()

	node1ID := mustParseNodeID(t, "1")

	s.AddChallenge(&Challenge{
		ID:       "ch-1",
		NodeID:   node1ID,
		Reason:   "Issue 1",
		Status:   "open",
		Severity: "major",
	})

	// Get the jobs-format map
	jobsMap := s.ChallengeMapForJobs()

	// Check that the conversion was correct
	challenges := jobsMap[node1ID.String()]
	if len(challenges) != 1 {
		t.Fatalf("Expected 1 challenge, got %d", len(challenges))
	}

	c := challenges[0]
	if c.ID != "ch-1" {
		t.Errorf("Expected ID 'ch-1', got %q", c.ID)
	}
	if c.TargetID.String() != node1ID.String() {
		t.Errorf("Expected TargetID %q, got %q", node1ID.String(), c.TargetID.String())
	}
	if c.Status != node.ChallengeStatusOpen {
		t.Errorf("Expected Status %q, got %q", node.ChallengeStatusOpen, c.Status)
	}
}

// TestState_NonExistentDependencyResolution verifies how state handles nodes
// with dependencies that reference non-existent nodes (forward references).
// This is an edge case that can occur when:
// 1. Events are replayed in incorrect order
// 2. A node references another node that was never created
// 3. A dependency chain contains dangling references
//
// The state package stores nodes as-is; validation of dependency existence
// is performed by the node.ValidateDepExistence function. This test verifies
// that state correctly stores and retrieves nodes even when their dependencies
// don't exist, and that proper validation detects these issues.
func TestState_NonExistentDependencyResolution(t *testing.T) {
	s := NewState()

	// Create node 1 (no dependencies)
	node1ID := mustParseNodeID(t, "1")
	node1, err := node.NewNode(node1ID, schema.NodeTypeClaim, "Root claim", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	s.AddNode(node1)

	// Create node 1.1 with a dependency on non-existent node 1.99
	node2ID := mustParseNodeID(t, "1.1")
	nonExistentDepID := mustParseNodeID(t, "1.99")
	node2, err := node.NewNodeWithOptions(
		node2ID,
		schema.NodeTypeClaim,
		"Claim with non-existent dependency",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{nonExistentDepID},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	s.AddNode(node2)

	// Verify the node was added to state despite having non-existent dependency
	gotNode2 := s.GetNode(node2ID)
	if gotNode2 == nil {
		t.Fatal("Node with non-existent dependency should still be added to state")
	}

	// Verify the dependency is recorded even though it doesn't exist
	if len(gotNode2.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(gotNode2.Dependencies))
	}
	if gotNode2.Dependencies[0].String() != nonExistentDepID.String() {
		t.Errorf("Dependency ID mismatch: got %q, want %q", gotNode2.Dependencies[0].String(), nonExistentDepID.String())
	}

	// Verify the dependency node doesn't exist in state
	gotNonExistent := s.GetNode(nonExistentDepID)
	if gotNonExistent != nil {
		t.Error("Non-existent dependency node should not exist in state")
	}

	// Verify ValidateDepExistence correctly detects the missing dependency
	err = node.ValidateDepExistence(gotNode2, s)
	if err == nil {
		t.Error("ValidateDepExistence should return error for non-existent dependency")
	}
}

// TestState_NonExistentDependencyResolution_MultipleNonExistent verifies behavior
// when a node has multiple dependencies, some existing and some non-existent.
func TestState_NonExistentDependencyResolution_MultipleNonExistent(t *testing.T) {
	s := NewState()

	// Create existing dependency nodes
	existingDep1ID := mustParseNodeID(t, "1")
	existingDep1, _ := node.NewNode(existingDep1ID, schema.NodeTypeClaim, "Existing dep 1", schema.InferenceAssumption)
	s.AddNode(existingDep1)

	existingDep2ID := mustParseNodeID(t, "1.1")
	existingDep2, _ := node.NewNode(existingDep2ID, schema.NodeTypeClaim, "Existing dep 2", schema.InferenceAssumption)
	s.AddNode(existingDep2)

	// Create node with mix of existing and non-existent dependencies
	nodeID := mustParseNodeID(t, "1.2")
	nonExistent1 := mustParseNodeID(t, "1.98")
	nonExistent2 := mustParseNodeID(t, "1.99")

	nodeWithMixedDeps, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Node with mixed dependencies",
		schema.InferenceModusPonens,
		node.NodeOptions{
			Dependencies: []types.NodeID{existingDep1ID, nonExistent1, existingDep2ID, nonExistent2},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	s.AddNode(nodeWithMixedDeps)

	// Verify all 4 dependencies are recorded
	got := s.GetNode(nodeID)
	if got == nil {
		t.Fatal("Node should be in state")
	}
	if len(got.Dependencies) != 4 {
		t.Errorf("Expected 4 dependencies, got %d", len(got.Dependencies))
	}

	// Verify validation fails due to non-existent dependencies
	err = node.ValidateDepExistence(got, s)
	if err == nil {
		t.Error("ValidateDepExistence should fail for partially missing dependencies")
	}

	// Verify existing dependencies can be resolved
	if s.GetNode(existingDep1ID) == nil {
		t.Error("Existing dep 1 should be resolvable")
	}
	if s.GetNode(existingDep2ID) == nil {
		t.Error("Existing dep 2 should be resolvable")
	}

	// Verify non-existent dependencies cannot be resolved
	if s.GetNode(nonExistent1) != nil {
		t.Error("Non-existent dep 1 should not be resolvable")
	}
	if s.GetNode(nonExistent2) != nil {
		t.Error("Non-existent dep 2 should not be resolvable")
	}
}

// TestState_NonExistentDependencyResolution_CircularToNonExistent verifies
// behavior when a node's dependency chain leads to a non-existent node.
func TestState_NonExistentDependencyResolution_CircularToNonExistent(t *testing.T) {
	s := NewState()

	// Create chain: 1 -> 1.1 -> 1.2 -> (non-existent 1.3)
	node1ID := mustParseNodeID(t, "1")
	node1, _ := node.NewNode(node1ID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	s.AddNode(node1)

	node2ID := mustParseNodeID(t, "1.1")
	node2, _ := node.NewNodeWithOptions(
		node2ID,
		schema.NodeTypeClaim,
		"Second level",
		schema.InferenceModusPonens,
		node.NodeOptions{Dependencies: []types.NodeID{node1ID}},
	)
	s.AddNode(node2)

	// Node 1.2 depends on non-existent 1.3
	node3ID := mustParseNodeID(t, "1.2")
	nonExistentID := mustParseNodeID(t, "1.3")
	node3, _ := node.NewNodeWithOptions(
		node3ID,
		schema.NodeTypeClaim,
		"Third level with dangling ref",
		schema.InferenceModusPonens,
		node.NodeOptions{Dependencies: []types.NodeID{node2ID, nonExistentID}},
	)
	s.AddNode(node3)

	// All three nodes should exist in state
	if s.GetNode(node1ID) == nil {
		t.Error("Node 1 should exist")
	}
	if s.GetNode(node2ID) == nil {
		t.Error("Node 1.1 should exist")
	}
	if s.GetNode(node3ID) == nil {
		t.Error("Node 1.2 should exist")
	}

	// Validate each node
	// Node 1: no dependencies, should pass
	if err := node.ValidateDepExistence(s.GetNode(node1ID), s); err != nil {
		t.Errorf("Node 1 validation should pass: %v", err)
	}

	// Node 1.1: depends on existing node 1, should pass
	if err := node.ValidateDepExistence(s.GetNode(node2ID), s); err != nil {
		t.Errorf("Node 1.1 validation should pass: %v", err)
	}

	// Node 1.2: has non-existent dependency, should fail
	if err := node.ValidateDepExistence(s.GetNode(node3ID), s); err == nil {
		t.Error("Node 1.2 validation should fail due to non-existent dependency")
	}
}

// TestState_NonExistentDependencyResolution_LaterResolution verifies that
// dependencies that don't exist initially can be resolved after the
// dependency node is added to state (forward reference resolution).
func TestState_NonExistentDependencyResolution_LaterResolution(t *testing.T) {
	s := NewState()

	// Create node 1.1 that depends on node 1 (which doesn't exist yet)
	nodeID := mustParseNodeID(t, "1.1")
	depID := mustParseNodeID(t, "1")

	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Forward reference to node 1",
		schema.InferenceModusPonens,
		node.NodeOptions{Dependencies: []types.NodeID{depID}},
	)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	s.AddNode(n)

	// Initially, validation should fail (dependency doesn't exist)
	err = node.ValidateDepExistence(s.GetNode(nodeID), s)
	if err == nil {
		t.Error("Validation should fail before dependency is added")
	}

	// Now add the dependency node
	depNode, _ := node.NewNode(depID, schema.NodeTypeClaim, "Root", schema.InferenceAssumption)
	s.AddNode(depNode)

	// Now validation should pass (dependency exists)
	err = node.ValidateDepExistence(s.GetNode(nodeID), s)
	if err != nil {
		t.Errorf("Validation should pass after dependency is added: %v", err)
	}

	// Verify the dependency can now be resolved
	resolvedDep := s.GetNode(depID)
	if resolvedDep == nil {
		t.Error("Dependency should be resolvable after being added")
	}
	if resolvedDep.Statement != "Root" {
		t.Errorf("Resolved dependency statement mismatch: got %q", resolvedDep.Statement)
	}
}

// TestState_NonExistentDependencyResolution_SelfReference verifies behavior
// when a node references itself as a dependency.
func TestState_NonExistentDependencyResolution_SelfReference(t *testing.T) {
	s := NewState()

	nodeID := mustParseNodeID(t, "1")

	// Create a node that depends on itself (but isn't in state yet)
	n, err := node.NewNodeWithOptions(
		nodeID,
		schema.NodeTypeClaim,
		"Self-referencing node",
		schema.InferenceAssumption,
		node.NodeOptions{Dependencies: []types.NodeID{nodeID}},
	)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	// Before adding to state, validation should fail (node doesn't exist)
	err = node.ValidateDepExistence(n, s)
	if err == nil {
		t.Error("Validation should fail when self-reference doesn't exist in state")
	}

	// Add to state
	s.AddNode(n)

	// After adding, the self-reference can be resolved (node exists)
	// Note: ValidateDepExistence only checks existence, not semantic validity of self-reference
	err = node.ValidateDepExistence(s.GetNode(nodeID), s)
	if err != nil {
		t.Errorf("Validation should pass for self-reference when node is in state: %v", err)
	}

	// Verify the self-reference is recorded
	got := s.GetNode(nodeID)
	if len(got.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(got.Dependencies))
	}
	if got.Dependencies[0].String() != nodeID.String() {
		t.Errorf("Self-reference dependency mismatch: got %q", got.Dependencies[0].String())
	}
}
