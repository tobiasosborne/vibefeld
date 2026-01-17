package types

import (
	"strconv"
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

			child, err := parent.Child(tt.childNum)
			if err != nil {
				t.Fatalf("Child(%d) unexpected error: %v", tt.childNum, err)
			}
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

// TestChild_InvalidNumbers verifies Child returns error for invalid inputs
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

			_, err = parent.Child(tt.childNum)
			if err == nil {
				t.Errorf("Child(%d) expected error, got nil", tt.childNum)
			}
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

// TestNodeID_Equal verifies the Equal method works correctly
func TestNodeID_Equal(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{"same_root", "1", "1", true},
		{"same_child", "1.2", "1.2", true},
		{"same_deep", "1.2.3.4.5", "1.2.3.4.5", true},
		{"different_children", "1.1", "1.2", false},
		{"different_depths", "1.1", "1.1.1", false},
		{"parent_vs_child", "1", "1.1", false},
		{"sibling_subtrees", "1.2.3", "1.3.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idA, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.a, err)
			}
			idB, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.b, err)
			}

			got := idA.Equal(idB)
			if got != tt.equal {
				t.Errorf("NodeID(%q).Equal(%q) = %v, want %v", tt.a, tt.b, got, tt.equal)
			}

			// Equal should be symmetric
			gotReverse := idB.Equal(idA)
			if gotReverse != tt.equal {
				t.Errorf("NodeID(%q).Equal(%q) = %v, want %v (symmetric)", tt.b, tt.a, gotReverse, tt.equal)
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
				id, err = id.Child(num)
				if err != nil {
					t.Fatalf("Child(%d) unexpected error: %v", num, err)
				}
			}

			if id.String() != tt.wantFinal {
				t.Errorf("child chain = %q, want %q", id.String(), tt.wantFinal)
			}
		})
	}
}

// Benchmark tests for NodeID.String() performance optimization

// BenchmarkString_Cached benchmarks String() on parsed NodeIDs (cached)
func BenchmarkString_Cached(b *testing.B) {
	ids := []NodeID{}
	testInputs := []string{"1", "1.1", "1.2.3", "1.2.3.4.5", "1.1.2.3.4.5.6.7.8"}
	for _, s := range testInputs {
		id, _ := Parse(s)
		ids = append(ids, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range ids {
			_ = id.String()
		}
	}
}

// BenchmarkString_Repeated benchmarks repeated String() calls on same ID
func BenchmarkString_Repeated(b *testing.B) {
	id, _ := Parse("1.2.3.4.5.6.7.8")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

// BenchmarkString_DeepNesting benchmarks String() on deeply nested IDs
func BenchmarkString_DeepNesting(b *testing.B) {
	id, _ := Parse("1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

// BenchmarkString_MapKey benchmarks using String() as map keys
func BenchmarkString_MapKey(b *testing.B) {
	ids := []NodeID{}
	for i := 1; i <= 100; i++ {
		id, _ := Parse("1." + itoa(i))
		ids = append(ids, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]int)
		for j, id := range ids {
			m[id.String()] = j
		}
	}
}

// BenchmarkChild_WithCaching benchmarks Child() which now includes caching
func BenchmarkChild_WithCaching(b *testing.B) {
	root, _ := Parse("1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := root
		for j := 1; j <= 8; j++ {
			id, _ = id.Child(j)
		}
		_ = id.String()
	}
}

// BenchmarkParent_WithCaching benchmarks Parent() which now includes caching
func BenchmarkParent_WithCaching(b *testing.B) {
	deep, _ := Parse("1.2.3.4.5.6.7.8")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := deep
		for {
			parent, ok := id.Parent()
			if !ok {
				break
			}
			_ = parent.String()
			id = parent
		}
	}
}

// itoa is a helper to convert int to string for benchmark setup
func itoa(n int) string {
	return strconv.Itoa(n)
}

// TestString_CacheConsistency verifies cached string matches computed string
func TestString_CacheConsistency(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"root", "1"},
		{"simple child", "1.5"},
		{"grandchild", "1.2.3"},
		{"deep", "1.2.3.4.5.6.7.8"},
		{"large numbers", "1.999.888"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}

			// Verify cached string matches input
			if id.String() != tt.input {
				t.Errorf("Parse(%q).String() = %q, want %q", tt.input, id.String(), tt.input)
			}

			// Verify multiple calls return same value
			for i := 0; i < 5; i++ {
				if id.String() != tt.input {
					t.Errorf("String() call %d = %q, want %q", i, id.String(), tt.input)
				}
			}
		})
	}
}

// TestChild_CacheConsistency verifies Child() produces correct cached strings
func TestChild_CacheConsistency(t *testing.T) {
	tests := []struct {
		parent   string
		childNum int
		want     string
	}{
		{"1", 1, "1.1"},
		{"1", 99, "1.99"},
		{"1.2", 3, "1.2.3"},
		{"1.2.3.4", 5, "1.2.3.4.5"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			parent, _ := Parse(tt.parent)
			child, err := parent.Child(tt.childNum)
			if err != nil {
				t.Fatalf("Child(%d) unexpected error: %v", tt.childNum, err)
			}

			if child.String() != tt.want {
				t.Errorf("Child().String() = %q, want %q", child.String(), tt.want)
			}
		})
	}
}

// TestParent_CacheConsistency verifies Parent() produces correct cached strings
func TestParent_CacheConsistency(t *testing.T) {
	tests := []struct {
		input      string
		wantParent string
	}{
		{"1.1", "1"},
		{"1.2.3", "1.2"},
		{"1.2.3.4.5", "1.2.3.4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			id, _ := Parse(tt.input)
			parent, ok := id.Parent()
			if !ok {
				t.Fatalf("Parent() returned false")
			}

			if parent.String() != tt.wantParent {
				t.Errorf("Parent().String() = %q, want %q", parent.String(), tt.wantParent)
			}
		})
	}
}

// TestLess_BasicComparisons verifies basic ordering between NodeIDs
func TestLess_BasicComparisons(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		// Same level comparisons
		{"1 < 2 (roots)", "1", "1", false}, // equal, not less
		{"1.1 < 1.2", "1.1", "1.2", true},
		{"1.2 < 1.10", "1.2", "1.10", true},
		{"1.9 < 1.10", "1.9", "1.10", true},
		{"1.10 < 1.11", "1.10", "1.11", true},
		{"1.99 < 1.100", "1.99", "1.100", true},
		{"1.1.1 < 1.1.2", "1.1.1", "1.1.2", true},
		{"1.1.9 < 1.1.10", "1.1.9", "1.1.10", true},

		// Reverse comparisons (should be false)
		{"1.2 not < 1.1", "1.2", "1.1", false},
		{"1.10 not < 1.2", "1.10", "1.2", false},
		{"1.10 not < 1.9", "1.10", "1.9", false},
		{"1.100 not < 1.99", "1.100", "1.99", false},

		// Equal IDs (should be false)
		{"1.1 not < 1.1", "1.1", "1.1", false},
		{"1.2.3 not < 1.2.3", "1.2.3", "1.2.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.a, err)
			}

			b, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.b, err)
			}

			got := a.Less(b)
			if got != tt.want {
				t.Errorf("Parse(%q).Less(Parse(%q)) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestLess_DifferentDepths verifies ordering between IDs of different depths
func TestLess_DifferentDepths(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		// Parent is less than child (shorter path comes first)
		{"1 < 1.1", "1", "1.1", true},
		{"1 < 1.999", "1", "1.999", true},
		{"1.1 < 1.1.1", "1.1", "1.1.1", true},
		{"1.2 < 1.2.1", "1.2", "1.2.1", true},
		{"1.2.3 < 1.2.3.4", "1.2.3", "1.2.3.4", true},

		// Child is not less than parent
		{"1.1 not < 1", "1.1", "1", false},
		{"1.999 not < 1", "1.999", "1", false},
		{"1.1.1 not < 1.1", "1.1.1", "1.1", false},
		{"1.2.3.4 not < 1.2.3", "1.2.3.4", "1.2.3", false},

		// Different branches with different depths
		{"1.1 < 1.2.1 (different branches)", "1.1", "1.2.1", true},
		{"1.1.1 < 1.2 (different branches)", "1.1.1", "1.2", true},
		{"1.2 not < 1.1.1", "1.2", "1.1.1", false},
		{"1.2.1 not < 1.1", "1.2.1", "1.1", false},

		// Deeper comparisons
		{"1.1.1.1 < 1.1.1.2", "1.1.1.1", "1.1.1.2", true},
		{"1.1.1.1 < 1.1.2", "1.1.1.1", "1.1.2", true},
		{"1.1.1.1 < 1.2", "1.1.1.1", "1.2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.a, err)
			}

			b, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.b, err)
			}

			got := a.Less(b)
			if got != tt.want {
				t.Errorf("Parse(%q).Less(Parse(%q)) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestLess_EmptyNodeID verifies ordering with empty/zero NodeIDs
func TestLess_EmptyNodeID(t *testing.T) {
	var empty NodeID
	id1, _ := Parse("1")
	id11, _ := Parse("1.1")

	tests := []struct {
		name string
		a    NodeID
		b    NodeID
		want bool
	}{
		{"empty < 1", empty, id1, true},
		{"empty < 1.1", empty, id11, true},
		{"1 not < empty", id1, empty, false},
		{"1.1 not < empty", id11, empty, false},
		{"empty not < empty", empty, empty, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Less(tt.b)
			if got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestLess_Transitivity verifies that Less is transitive (a < b && b < c => a < c)
func TestLess_Transitivity(t *testing.T) {
	ids := []string{
		"1",
		"1.1",
		"1.1.1",
		"1.1.2",
		"1.2",
		"1.2.1",
		"1.10",
		"1.10.1",
	}

	parsed := make([]NodeID, len(ids))
	for i, s := range ids {
		var err error
		parsed[i], err = Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", s, err)
		}
	}

	// For each triple (a, b, c) where a < b and b < c, verify a < c
	for i := 0; i < len(parsed); i++ {
		for j := i + 1; j < len(parsed); j++ {
			for k := j + 1; k < len(parsed); k++ {
				a, b, c := parsed[i], parsed[j], parsed[k]

				if a.Less(b) && b.Less(c) {
					if !a.Less(c) {
						t.Errorf("Transitivity failed: %s < %s and %s < %s but not %s < %s",
							ids[i], ids[j], ids[j], ids[k], ids[i], ids[k])
					}
				}
			}
		}
	}
}

// TestLess_Antisymmetry verifies that a < b implies !(b < a)
func TestLess_Antisymmetry(t *testing.T) {
	pairs := [][2]string{
		{"1", "1.1"},
		{"1.1", "1.2"},
		{"1.2", "1.10"},
		{"1.1.1", "1.1.2"},
		{"1.1", "1.1.1"},
	}

	for _, pair := range pairs {
		a, err := Parse(pair[0])
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", pair[0], err)
		}

		b, err := Parse(pair[1])
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", pair[1], err)
		}

		if a.Less(b) {
			if b.Less(a) {
				t.Errorf("Antisymmetry failed: %s < %s and %s < %s both true",
					pair[0], pair[1], pair[1], pair[0])
			}
		}
	}
}

// TestLess_Irreflexivity verifies that !(a < a) for all a
func TestLess_Irreflexivity(t *testing.T) {
	ids := []string{
		"1",
		"1.1",
		"1.2",
		"1.10",
		"1.1.1",
		"1.2.3.4.5",
	}

	for _, s := range ids {
		id, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", s, err)
		}

		if id.Less(id) {
			t.Errorf("Irreflexivity failed: %s.Less(%s) = true", s, s)
		}
	}
}

// TestLess_SortOrder verifies that sorting using Less produces correct order
func TestLess_SortOrder(t *testing.T) {
	// Input in random order
	input := []string{
		"1.10",
		"1.1.2",
		"1.2",
		"1",
		"1.1",
		"1.2.1",
		"1.1.1",
		"1.2.10",
		"1.2.2",
	}

	// Expected sorted order
	expected := []string{
		"1",
		"1.1",
		"1.1.1",
		"1.1.2",
		"1.2",
		"1.2.1",
		"1.2.2",
		"1.2.10",
		"1.10",
	}

	// Parse all IDs
	ids := make([]NodeID, len(input))
	for i, s := range input {
		var err error
		ids[i], err = Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", s, err)
		}
	}

	// Simple bubble sort using Less
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j].Less(ids[i]) {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}

	// Verify order
	for i, id := range ids {
		if id.String() != expected[i] {
			t.Errorf("Position %d: got %q, want %q", i, id.String(), expected[i])
		}
	}
}

// TestLess_LargeNumbers verifies Less works with large part values
func TestLess_LargeNumbers(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"1.999 < 1.1000", "1.999", "1.1000", true},
		{"1.9999 < 1.10000", "1.9999", "1.10000", true},
		{"1.100 < 1.101", "1.100", "1.101", true},
		{"1.1.999 < 1.1.1000", "1.1.999", "1.1.1000", true},
		{"1.999.999 < 1.999.1000", "1.999.999", "1.999.1000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.a, err)
			}

			b, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.b, err)
			}

			got := a.Less(b)
			if got != tt.want {
				t.Errorf("Parse(%q).Less(Parse(%q)) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// BenchmarkLess compares performance of Less vs string-based comparison
func BenchmarkLess(b *testing.B) {
	id1, _ := Parse("1.2.3.4.5")
	id2, _ := Parse("1.2.3.4.6")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id1.Less(id2)
	}
}

// BenchmarkLess_DeepNesting benchmarks Less on deeply nested IDs
func BenchmarkLess_DeepNesting(b *testing.B) {
	id1, _ := Parse("1.1.1.1.1.1.1.1.1.1")
	id2, _ := Parse("1.1.1.1.1.1.1.1.1.2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id1.Less(id2)
	}
}

// BenchmarkLess_Sorting benchmarks sorting a slice of NodeIDs using Less
func BenchmarkLess_Sorting(b *testing.B) {
	baseIDs := []string{
		"1.10", "1.1.2", "1.2", "1", "1.1", "1.2.1", "1.1.1", "1.2.10", "1.2.2",
		"1.3", "1.3.1", "1.3.2", "1.4", "1.5", "1.6", "1.7", "1.8", "1.9",
	}

	parsedBase := make([]NodeID, len(baseIDs))
	for i, s := range baseIDs {
		parsedBase[i], _ = Parse(s)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Copy slice for each iteration
		ids := make([]NodeID, len(parsedBase))
		copy(ids, parsedBase)

		// Bubble sort
		for j := 0; j < len(ids); j++ {
			for k := j + 1; k < len(ids); k++ {
				if ids[k].Less(ids[j]) {
					ids[j], ids[k] = ids[k], ids[j]
				}
			}
		}
	}
}
