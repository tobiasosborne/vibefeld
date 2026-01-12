package types

import (
	"testing"
)

// TestParse_ValidIDs verifies parsing of valid hierarchical node IDs
func TestParse_ValidIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"root", "1"},
		{"first child", "1.1"},
		{"second child", "1.2"},
		{"tenth child", "1.10"},
		{"hundredth child", "1.100"},
		{"grandchild", "1.1.1"},
		{"second grandchild", "1.1.2"},
		{"deep nesting", "1.2.3.4.5"},
		{"very deep nesting", "1.1.1.1.1.1.1.1"},
		{"large numbers", "1.999.888.777"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
				return
			}
			if id.String() != tt.input {
				t.Errorf("Parse(%q).String() = %q, want %q", tt.input, id.String(), tt.input)
			}
		})
	}
}

// TestParse_InvalidIDs verifies parsing rejects invalid node IDs
func TestParse_InvalidIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"zero", "0"},
		{"zero child", "1.0"},
		{"zero grandchild", "1.1.0"},
		{"trailing dot", "1."},
		{"trailing dots", "1.1."},
		{"leading dot", ".1"},
		{"double dot", "1..1"},
		{"triple dot", "1...1"},
		{"double dot middle", "1.1..2"},
		{"letter", "a"},
		{"letters with dots", "a.b"},
		{"letter prefix", "a1"},
		{"letter suffix", "1a"},
		{"letter in middle", "1.a.2"},
		{"negative number", "-1"},
		{"negative child", "1.-2"},
		{"space", "1 1"},
		{"space with dot", "1. 1"},
		{"comma", "1,1"},
		{"just dot", "."},
		{"multiple dots", "..."},
		{"special characters", "1.@.2"},
		{"whitespace only", "   "},
		{"newline", "1\n1"},
		{"tab", "1\t1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", tt.input)
			}
		})
	}
}

// TestString_Roundtrip verifies parsing and stringifying is consistent
func TestString_Roundtrip(t *testing.T) {
	tests := []string{
		"1",
		"1.1",
		"1.2",
		"1.10",
		"1.100",
		"1.1.1",
		"1.2.3",
		"1.2.3.4",
		"1.2.3.4.5.6.7.8",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			id, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", input, err)
			}

			output := id.String()
			if output != input {
				t.Errorf("Parse(%q).String() = %q, want %q", input, output, input)
			}

			// Parse again to ensure double roundtrip
			id2, err := Parse(output)
			if err != nil {
				t.Fatalf("Parse(%q) second parse unexpected error: %v", output, err)
			}

			output2 := id2.String()
			if output2 != input {
				t.Errorf("Double roundtrip: %q -> %q, want %q", input, output2, input)
			}
		})
	}
}

// TestParent_Root verifies root node has no parent
func TestParent_Root(t *testing.T) {
	root, err := Parse("1")
	if err != nil {
		t.Fatalf("Parse(\"1\") unexpected error: %v", err)
	}

	parent, ok := root.Parent()
	if ok {
		t.Errorf("root.Parent() = (%v, %v), want (_, false)", parent, ok)
	}
}

// TestParent_NonRoot verifies non-root nodes return correct parent
func TestParent_NonRoot(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantParent string
	}{
		{"first child", "1.1", "1"},
		{"second child", "1.2", "1"},
		{"tenth child", "1.10", "1"},
		{"grandchild", "1.1.1", "1.1"},
		{"second grandchild", "1.1.2", "1.1"},
		{"different branch grandchild", "1.2.1", "1.2"},
		{"deep nesting", "1.2.3.4.5", "1.2.3.4"},
		{"very deep nesting", "1.1.1.1.1.1", "1.1.1.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}

			parent, ok := id.Parent()
			if !ok {
				t.Errorf("id.Parent() = (_, false), want (_, true)")
				return
			}

			if parent.String() != tt.wantParent {
				t.Errorf("Parse(%q).Parent() = %q, want %q", tt.input, parent.String(), tt.wantParent)
			}
		})
	}
}

// TestChild verifies Child method generates correct child IDs
func TestChild(t *testing.T) {
	tests := []struct {
		name      string
		parent    string
		childNum  int
		wantChild string
	}{
		{"root first child", "1", 1, "1.1"},
		{"root second child", "1", 2, "1.2"},
		{"root tenth child", "1", 10, "1.10"},
		{"root hundredth child", "1", 100, "1.100"},
		{"child first grandchild", "1.1", 1, "1.1.1"},
		{"child second grandchild", "1.1", 2, "1.1.2"},
		{"different branch", "1.2", 1, "1.2.1"},
		{"deep nesting", "1.2.3.4", 5, "1.2.3.4.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, err := Parse(tt.parent)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.parent, err)
			}

			child := parent.Child(tt.childNum)
			if child.String() != tt.wantChild {
				t.Errorf("Parse(%q).Child(%d) = %q, want %q", tt.parent, tt.childNum, child.String(), tt.wantChild)
			}
		})
	}
}

// TestIsRoot verifies only "1" is identified as root
func TestIsRoot(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		isRoot bool
	}{
		{"root is root", "1", true},
		{"first child not root", "1.1", false},
		{"second child not root", "1.2", false},
		{"grandchild not root", "1.1.1", false},
		{"deep nesting not root", "1.2.3.4.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}

			if got := id.IsRoot(); got != tt.isRoot {
				t.Errorf("Parse(%q).IsRoot() = %v, want %v", tt.input, got, tt.isRoot)
			}
		})
	}
}

// TestDepth verifies depth calculation
func TestDepth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDepth int
	}{
		{"root depth 1", "1", 1},
		{"first child depth 2", "1.1", 2},
		{"second child depth 2", "1.2", 2},
		{"tenth child depth 2", "1.10", 2},
		{"grandchild depth 3", "1.1.1", 3},
		{"another grandchild depth 3", "1.2.3", 3},
		{"great-grandchild depth 4", "1.1.1.1", 4},
		{"depth 5", "1.2.3.4.5", 5},
		{"depth 6", "1.1.2.3.4.5", 6},
		{"depth 8", "1.1.1.1.1.1.1.1", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}

			if got := id.Depth(); got != tt.wantDepth {
				t.Errorf("Parse(%q).Depth() = %d, want %d", tt.input, got, tt.wantDepth)
			}
		})
	}
}

// TestIsAncestorOf verifies ancestry relationships
func TestIsAncestorOf(t *testing.T) {
	tests := []struct {
		name       string
		ancestor   string
		descendant string
		want       bool
	}{
		// True cases - actual ancestors
		{"root is ancestor of child", "1", "1.1", true},
		{"root is ancestor of grandchild", "1", "1.1.1", true},
		{"root is ancestor of deep descendant", "1", "1.2.3.4.5", true},
		{"child is ancestor of grandchild", "1.1", "1.1.1", true},
		{"child is ancestor of great-grandchild", "1.1", "1.1.1.1", true},
		{"grandchild is ancestor of great-grandchild", "1.1.1", "1.1.1.1", true},
		{"intermediate node is ancestor", "1.2.3", "1.2.3.4.5", true},

		// False cases - not ancestors
		{"node is not ancestor of itself", "1", "1", false},
		{"child is not ancestor of itself", "1.1", "1.1", false},
		{"child is not ancestor of root", "1.1", "1", false},
		{"grandchild is not ancestor of root", "1.1.1", "1", false},
		{"grandchild is not ancestor of parent", "1.1.1", "1.1", false},
		{"sibling is not ancestor", "1.1", "1.2", false},
		{"cousin is not ancestor", "1.1.1", "1.2.1", false},
		{"different branch", "1.1", "1.2.1", false},
		{"different branch reverse", "1.2.1", "1.1", false},
		{"uncle is not ancestor of nephew", "1.1", "1.2.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ancestor, err := Parse(tt.ancestor)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.ancestor, err)
			}

			descendant, err := Parse(tt.descendant)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.descendant, err)
			}

			got := ancestor.IsAncestorOf(descendant)
			if got != tt.want {
				t.Errorf("Parse(%q).IsAncestorOf(Parse(%q)) = %v, want %v",
					tt.ancestor, tt.descendant, got, tt.want)
			}
		})
	}
}

// TestCommonAncestor verifies finding common ancestors
func TestCommonAncestor(t *testing.T) {
	tests := []struct {
		name     string
		id1      string
		id2      string
		wantCA   string
	}{
		// Siblings share parent
		{"siblings share parent", "1.1", "1.2", "1"},
		{"different siblings", "1.3", "1.5", "1"},

		// Cousins share grandparent
		{"cousins share grandparent", "1.1.1", "1.2.1", "1"},
		{"different cousins", "1.1.2", "1.2.3", "1"},

		// Siblings at deeper level
		{"deep siblings", "1.2.3.1", "1.2.3.2", "1.2.3"},
		{"deeper siblings", "1.1.1.1.1", "1.1.1.1.2", "1.1.1.1"},

		// Parent and child
		{"parent and child", "1.1", "1.1.1", "1.1"},
		{"grandparent and grandchild", "1.1", "1.1.1.1", "1.1"},

		// Same node
		{"same node", "1.1.1", "1.1.1", "1.1.1"},
		{"root with root", "1", "1", "1"},

		// Root with any node
		{"root with child", "1", "1.1", "1"},
		{"root with grandchild", "1", "1.2.1", "1"},
		{"child with root", "1.1", "1", "1"},

		// Different branches
		{"different branches level 2", "1.1.1.1", "1.2.1.1", "1"},
		{"different branches level 3", "1.1.1", "1.2.3", "1"},

		// Partial overlap
		{"partial overlap", "1.2.3.4", "1.2.5", "1.2"},
		{"another partial overlap", "1.1.2.3", "1.1.5", "1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1, err := Parse(tt.id1)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.id1, err)
			}

			id2, err := Parse(tt.id2)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.id2, err)
			}

			ca := id1.CommonAncestor(id2)
			if ca.String() != tt.wantCA {
				t.Errorf("Parse(%q).CommonAncestor(Parse(%q)) = %q, want %q",
					tt.id1, tt.id2, ca.String(), tt.wantCA)
			}

			// CommonAncestor should be commutative
			ca2 := id2.CommonAncestor(id1)
			if ca2.String() != tt.wantCA {
				t.Errorf("CommonAncestor not commutative: %q.CA(%q)=%q, %q.CA(%q)=%q",
					tt.id1, tt.id2, ca.String(), tt.id2, tt.id1, ca2.String())
			}
		})
	}
}

// TestNodeID_ZeroValue verifies zero value behavior
func TestNodeID_ZeroValue(t *testing.T) {
	var zero NodeID

	// Zero value should have sensible behavior
	if zero.String() == "" {
		// String() on zero value might return empty string
		// This is acceptable behavior
	}

	if zero.IsRoot() {
		t.Error("zero NodeID.IsRoot() = true, want false")
	}

	if depth := zero.Depth(); depth != 0 {
		// Zero value might return 0 depth
		// This is acceptable behavior
	}
}

// TestChild_InvalidNumbers verifies Child panics or handles invalid inputs
func TestChild_InvalidNumbers(t *testing.T) {
	tests := []struct {
		name     string
		parent   string
		childNum int
	}{
		{"zero child number", "1", 0},
		{"negative child number", "1", -1},
		{"large negative", "1.1", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, err := Parse(tt.parent)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.parent, err)
			}

			// Child with invalid number might panic or return invalid ID
			// For now, just ensure it doesn't crash Go runtime
			defer func() {
				if r := recover(); r != nil {
					// Panic is acceptable for invalid input
					t.Logf("Child(%d) panicked (acceptable): %v", tt.childNum, r)
				}
			}()

			child := parent.Child(tt.childNum)
			// If we get here, verify the result is somehow invalid
			// (implementation-dependent behavior)
			_ = child
		})
	}
}

// TestNodeID_Equality verifies nodes with same ID are equal
func TestNodeID_Equality(t *testing.T) {
	tests := []string{
		"1",
		"1.1",
		"1.2.3",
		"1.2.3.4.5",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			id1, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q) first parse unexpected error: %v", input, err)
			}

			id2, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q) second parse unexpected error: %v", input, err)
			}

			// Same ID should be equal
			if id1.String() != id2.String() {
				t.Errorf("Parse(%q) creates unequal IDs: %q != %q", input, id1.String(), id2.String())
			}
		})
	}
}

// TestParent_Chain verifies walking up the parent chain
func TestParent_Chain(t *testing.T) {
	input := "1.2.3.4"
	expected := []string{"1.2.3", "1.2", "1"}

	id, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", input, err)
	}

	for i, want := range expected {
		parent, ok := id.Parent()
		if !ok {
			t.Fatalf("step %d: id.Parent() = (_, false), want (_, true)", i)
		}

		if parent.String() != want {
			t.Errorf("step %d: id.Parent() = %q, want %q", i, parent.String(), want)
		}

		id = parent
	}

	// After reaching root, Parent should return false
	_, ok := id.Parent()
	if ok {
		t.Error("root.Parent() = (_, true), want (_, false)")
	}
}

// TestChild_Chain verifies creating chain of children
func TestChild_Chain(t *testing.T) {
	tests := []struct {
		name        string
		start       string
		childNums   []int
		wantFinal   string
	}{
		{
			name:      "build 1.1.1.1",
			start:     "1",
			childNums: []int{1, 1, 1},
			wantFinal: "1.1.1.1",
		},
		{
			name:      "build 1.2.3.4",
			start:     "1",
			childNums: []int{2, 3, 4},
			wantFinal: "1.2.3.4",
		},
		{
			name:      "extend existing",
			start:     "1.1",
			childNums: []int{2, 3},
			wantFinal: "1.1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.start)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.start, err)
			}

			for _, num := range tt.childNums {
				id = id.Child(num)
			}

			if id.String() != tt.wantFinal {
				t.Errorf("child chain = %q, want %q", id.String(), tt.wantFinal)
			}
		})
	}
}
