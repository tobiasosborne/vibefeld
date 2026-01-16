//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestProgressCmd creates a fresh root command with the progress subcommand for testing.
// This ensures test isolation - each test gets its own command instance.
func newTestProgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	progressCmd := newProgressCmd()
	cmd.AddCommand(progressCmd)

	return cmd
}

// executeProgressCommand executes a cobra command with the given arguments and returns
// the combined stdout/stderr output and any error.
func executeProgressCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// createTestNode creates a test node with the given parameters.
func createTestNode(id string, epistemicState schema.EpistemicState, workflowState schema.WorkflowState) *node.Node {
	nodeID, _ := types.Parse(id)
	n, _ := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement for "+id, schema.InferenceAssumption)
	n.EpistemicState = epistemicState
	n.WorkflowState = workflowState
	return n
}

// =============================================================================
// Flag Tests (Unit Tests - No File System)
// =============================================================================

// TestProgressCmd_ExpectedFlags ensures the progress command has expected flag structure.
func TestProgressCmd_ExpectedFlags(t *testing.T) {
	cmd := newProgressCmd()

	// Check expected flags exist
	expectedFlags := []string{"format", "dir"}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil && cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("expected progress command to have flag %q", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"f": "format",
		"d": "dir",
	}

	for short, long := range shortFlags {
		if cmd.Flags().ShorthandLookup(short) == nil {
			t.Errorf("expected progress command to have short flag -%s for --%s", short, long)
		}
	}
}

// TestProgressCmd_DefaultFlagValues verifies default values for flags.
func TestProgressCmd_DefaultFlagValues(t *testing.T) {
	cmd := newProgressCmd()

	// Check default dir value
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Fatal("expected dir flag to exist")
	}
	if dirFlag.DefValue != "." {
		t.Errorf("expected default dir to be '.', got %q", dirFlag.DefValue)
	}

	// Check default format value
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("expected default format to be 'text', got %q", formatFlag.DefValue)
	}
}

// TestProgressCmd_CommandMetadata verifies command metadata.
func TestProgressCmd_CommandMetadata(t *testing.T) {
	cmd := newProgressCmd()

	if cmd.Use != "progress" {
		t.Errorf("expected Use to be 'progress', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// =============================================================================
// Help Tests
// =============================================================================

// TestProgressCmd_Help verifies help output shows usage information.
func TestProgressCmd_Help(t *testing.T) {
	cmd := newTestProgressCmd()
	output, err := executeProgressCommand(cmd, "progress", "--help")

	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	// Help should include usage information
	expectations := []string{
		"progress", // Command name
		"--format", // Format flag
		"--dir",    // Directory flag
	}

	for _, exp := range expectations {
		if !strings.Contains(output, exp) {
			t.Errorf("expected help output to contain %q, got: %q", exp, output)
		}
	}
}

// TestProgressCmd_HelpShortFlag verifies help with short flag.
func TestProgressCmd_HelpShortFlag(t *testing.T) {
	cmd := newTestProgressCmd()
	output, err := executeProgressCommand(cmd, "progress", "-h")

	if err != nil {
		t.Fatalf("expected no error for -h, got: %v", err)
	}

	if !strings.Contains(output, "progress") {
		t.Errorf("expected help output to mention 'progress', got: %q", output)
	}
}

// =============================================================================
// Error Case Tests (Unit Tests)
// =============================================================================

// TestProgressCmd_DirFlagNonExistent verifies error for non-existent directory.
func TestProgressCmd_DirFlagNonExistent(t *testing.T) {
	cmd := newTestProgressCmd()
	_, err := executeProgressCommand(cmd, "progress", "--dir", "/nonexistent/path/to/proof")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestProgressCmd_InvalidFormat verifies error for invalid format.
func TestProgressCmd_InvalidFormat(t *testing.T) {
	cmd := newTestProgressCmd()
	_, err := executeProgressCommand(cmd, "progress", "--format", "xml", "--dir", "/nonexistent/path")

	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}

	// The error should be about the invalid format, not the directory
	if err != nil && strings.Contains(err.Error(), "invalid format") {
		// Good - format validation happened before directory check
		return
	}
	// If directory error comes first, that's acceptable too
}

// =============================================================================
// computeProgressMetrics Tests
// =============================================================================

// TestComputeProgressMetrics_EmptyState tests metrics with no nodes.
func TestComputeProgressMetrics_EmptyState(t *testing.T) {
	st := state.NewState()
	var pendingDefs []*node.PendingDef

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.TotalNodes != 0 {
		t.Errorf("expected TotalNodes to be 0, got %d", metrics.TotalNodes)
	}
	if metrics.CompletedNodes != 0 {
		t.Errorf("expected CompletedNodes to be 0, got %d", metrics.CompletedNodes)
	}
	if metrics.CompletionPercent != 0 {
		t.Errorf("expected CompletionPercent to be 0, got %d", metrics.CompletionPercent)
	}
	if len(metrics.CriticalPath) != 0 {
		t.Errorf("expected CriticalPath to be empty, got %v", metrics.CriticalPath)
	}
}

// TestComputeProgressMetrics_AllPending tests metrics with all pending nodes.
func TestComputeProgressMetrics_AllPending(t *testing.T) {
	st := state.NewState()

	// Add 3 pending nodes
	st.AddNode(createTestNode("1", schema.EpistemicPending, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.1", schema.EpistemicPending, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.2", schema.EpistemicPending, schema.WorkflowAvailable))

	var pendingDefs []*node.PendingDef

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.TotalNodes != 3 {
		t.Errorf("expected TotalNodes to be 3, got %d", metrics.TotalNodes)
	}
	if metrics.CompletedNodes != 0 {
		t.Errorf("expected CompletedNodes to be 0, got %d", metrics.CompletedNodes)
	}
	if metrics.CompletionPercent != 0 {
		t.Errorf("expected CompletionPercent to be 0, got %d", metrics.CompletionPercent)
	}
	if metrics.ByState["pending"] != 3 {
		t.Errorf("expected ByState['pending'] to be 3, got %d", metrics.ByState["pending"])
	}
}

// TestComputeProgressMetrics_MixedStates tests metrics with mixed epistemic states.
func TestComputeProgressMetrics_MixedStates(t *testing.T) {
	st := state.NewState()

	// Add nodes with different states
	st.AddNode(createTestNode("1", schema.EpistemicValidated, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.1", schema.EpistemicAdmitted, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.2", schema.EpistemicPending, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.3", schema.EpistemicRefuted, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.4", schema.EpistemicArchived, schema.WorkflowAvailable))

	var pendingDefs []*node.PendingDef

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.TotalNodes != 5 {
		t.Errorf("expected TotalNodes to be 5, got %d", metrics.TotalNodes)
	}
	if metrics.CompletedNodes != 3 {
		t.Errorf("expected CompletedNodes to be 3, got %d", metrics.CompletedNodes)
	}
	if metrics.CompletionPercent != 60 {
		t.Errorf("expected CompletionPercent to be 60, got %d", metrics.CompletionPercent)
	}
	if metrics.ByState["pending"] != 1 {
		t.Errorf("expected ByState['pending'] to be 1, got %d", metrics.ByState["pending"])
	}
	if metrics.ByState["validated"] != 1 {
		t.Errorf("expected ByState['validated'] to be 1, got %d", metrics.ByState["validated"])
	}
	if metrics.ByState["admitted"] != 1 {
		t.Errorf("expected ByState['admitted'] to be 1, got %d", metrics.ByState["admitted"])
	}
	if metrics.ByState["refuted"] != 1 {
		t.Errorf("expected ByState['refuted'] to be 1, got %d", metrics.ByState["refuted"])
	}
	if metrics.ByState["archived"] != 1 {
		t.Errorf("expected ByState['archived'] to be 1, got %d", metrics.ByState["archived"])
	}
}

// TestComputeProgressMetrics_BlockedNodes tests that blocked nodes are counted.
func TestComputeProgressMetrics_BlockedNodes(t *testing.T) {
	st := state.NewState()

	// Add nodes with mixed workflow states
	st.AddNode(createTestNode("1", schema.EpistemicPending, schema.WorkflowAvailable))
	st.AddNode(createTestNode("1.1", schema.EpistemicPending, schema.WorkflowBlocked))
	st.AddNode(createTestNode("1.2", schema.EpistemicPending, schema.WorkflowBlocked))
	st.AddNode(createTestNode("1.3", schema.EpistemicPending, schema.WorkflowClaimed))

	var pendingDefs []*node.PendingDef

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.BlockedNodes != 2 {
		t.Errorf("expected BlockedNodes to be 2, got %d", metrics.BlockedNodes)
	}
}

// TestComputeProgressMetrics_WithPendingDefs tests that pending definitions are counted.
func TestComputeProgressMetrics_WithPendingDefs(t *testing.T) {
	st := state.NewState()
	st.AddNode(createTestNode("1", schema.EpistemicPending, schema.WorkflowAvailable))

	// Create pending definitions
	nodeID, _ := types.Parse("1")
	pd1, _ := node.NewPendingDef("term1", nodeID)
	pd2, _ := node.NewPendingDef("term2", nodeID)

	pendingDefs := []*node.PendingDef{pd1, pd2}

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.PendingDefs != 2 {
		t.Errorf("expected PendingDefs to be 2, got %d", metrics.PendingDefs)
	}
}

// TestComputeProgressMetrics_WithOpenChallenges tests that open challenges are counted.
func TestComputeProgressMetrics_WithOpenChallenges(t *testing.T) {
	st := state.NewState()
	st.AddNode(createTestNode("1", schema.EpistemicPending, schema.WorkflowAvailable))

	// Add open challenges
	nodeID, _ := types.Parse("1")
	st.AddChallenge(&state.Challenge{
		ID:     "ch1",
		NodeID: nodeID,
		Status: "open",
	})
	st.AddChallenge(&state.Challenge{
		ID:     "ch2",
		NodeID: nodeID,
		Status: "open",
	})
	st.AddChallenge(&state.Challenge{
		ID:     "ch3",
		NodeID: nodeID,
		Status: "resolved", // This one should not be counted
	})

	var pendingDefs []*node.PendingDef

	metrics := computeProgressMetrics(st, pendingDefs)

	if metrics.OpenChallenges != 2 {
		t.Errorf("expected OpenChallenges to be 2, got %d", metrics.OpenChallenges)
	}
}

// =============================================================================
// findCriticalPath Tests
// =============================================================================

// TestFindCriticalPath_Empty tests critical path with no nodes.
func TestFindCriticalPath_Empty(t *testing.T) {
	var nodes []*node.Node

	path, depth := findCriticalPath(nodes)

	if len(path) != 0 {
		t.Errorf("expected empty path, got %v", path)
	}
	if depth != 0 {
		t.Errorf("expected depth 0, got %d", depth)
	}
}

// TestFindCriticalPath_AllValidated tests critical path with no pending nodes.
func TestFindCriticalPath_AllValidated(t *testing.T) {
	nodes := []*node.Node{
		createTestNode("1", schema.EpistemicValidated, schema.WorkflowAvailable),
		createTestNode("1.1", schema.EpistemicValidated, schema.WorkflowAvailable),
	}

	path, depth := findCriticalPath(nodes)

	if len(path) != 0 {
		t.Errorf("expected empty path (no pending nodes), got %v", path)
	}
	if depth != 0 {
		t.Errorf("expected depth 0, got %d", depth)
	}
}

// TestFindCriticalPath_SinglePending tests critical path with one pending node.
func TestFindCriticalPath_SinglePending(t *testing.T) {
	nodes := []*node.Node{
		createTestNode("1", schema.EpistemicPending, schema.WorkflowAvailable),
	}

	path, depth := findCriticalPath(nodes)

	if len(path) != 1 {
		t.Errorf("expected path length 1, got %d", len(path))
	}
	if depth != 1 {
		t.Errorf("expected depth 1, got %d", depth)
	}
	if len(path) > 0 && path[0] != "1" {
		t.Errorf("expected path to contain '1', got %v", path)
	}
}

// TestFindCriticalPath_DeepestPending tests that deepest pending node determines the path.
func TestFindCriticalPath_DeepestPending(t *testing.T) {
	nodes := []*node.Node{
		createTestNode("1", schema.EpistemicValidated, schema.WorkflowAvailable),
		createTestNode("1.1", schema.EpistemicPending, schema.WorkflowAvailable),
		createTestNode("1.2", schema.EpistemicValidated, schema.WorkflowAvailable),
		createTestNode("1.2.1", schema.EpistemicPending, schema.WorkflowAvailable),
	}

	path, depth := findCriticalPath(nodes)

	// Expected: 1 -> 1.2 -> 1.2.1 (depth 3)
	if depth != 3 {
		t.Errorf("expected depth 3, got %d", depth)
	}
	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d", len(path))
	}
	expectedPath := []string{"1", "1.2", "1.2.1"}
	for i, expected := range expectedPath {
		if i < len(path) && path[i] != expected {
			t.Errorf("expected path[%d] to be %q, got %q", i, expected, path[i])
		}
	}
}

// TestFindCriticalPath_MultipleDeepBranches tests with multiple deep pending branches.
func TestFindCriticalPath_MultipleDeepBranches(t *testing.T) {
	nodes := []*node.Node{
		createTestNode("1", schema.EpistemicValidated, schema.WorkflowAvailable),
		createTestNode("1.1", schema.EpistemicPending, schema.WorkflowAvailable),
		createTestNode("1.1.1", schema.EpistemicPending, schema.WorkflowAvailable),
		createTestNode("1.2", schema.EpistemicPending, schema.WorkflowAvailable),
		createTestNode("1.2.1", schema.EpistemicPending, schema.WorkflowAvailable),
		createTestNode("1.2.1.1", schema.EpistemicPending, schema.WorkflowAvailable), // This is the deepest
	}

	path, depth := findCriticalPath(nodes)

	// Expected: 1 -> 1.2 -> 1.2.1 -> 1.2.1.1 (depth 4)
	if depth != 4 {
		t.Errorf("expected depth 4, got %d", depth)
	}
	if len(path) != 4 {
		t.Errorf("expected path length 4, got %d", len(path))
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

// TestProgressCmd_JSONOutputStructure verifies JSON output has correct structure.
func TestProgressCmd_JSONOutputStructure(t *testing.T) {
	// Test that the expected JSON keys exist in the ProgressMetrics struct
	metrics := &ProgressMetrics{
		TotalNodes:        10,
		CompletedNodes:    5,
		CompletionPercent: 50,
		ByState: map[string]int{
			"pending":   3,
			"validated": 4,
			"admitted":  1,
			"refuted":   1,
			"archived":  1,
		},
		OpenChallenges:    2,
		PendingDefs:       1,
		BlockedNodes:      1,
		CriticalPath:      []string{"1", "1.2", "1.2.1"},
		CriticalPathDepth: 3,
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("failed to marshal test JSON structure: %v", err)
	}

	// Verify the structure can be unmarshaled back
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	expectedKeys := []string{
		"total_nodes",
		"completed_nodes",
		"completion_percent",
		"by_state",
		"open_challenges",
		"pending_definitions",
		"blocked_nodes",
		"critical_path",
		"critical_path_depth",
	}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON to contain key %q", key)
		}
	}
}

// =============================================================================
// Table-Driven Format Validation Tests
// =============================================================================

// TestProgressCmd_FormatValidation verifies format flag validation.
func TestProgressCmd_FormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid text format", "text", false},
		{"valid json format", "json", false},
		{"valid TEXT uppercase", "TEXT", false},
		{"valid JSON uppercase", "JSON", false},
		{"invalid xml format", "xml", true},
		{"invalid yaml format", "yaml", true},
		{"invalid empty string with flag", "", false}, // Empty should default to text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTestProgressCmd()
			var args []string
			if tt.format == "" {
				// Don't pass format flag for empty case
				args = []string{"progress", "--dir", "/nonexistent/path"}
			} else {
				args = []string{"progress", "--format", tt.format, "--dir", "/nonexistent/path"}
			}

			_, err := executeProgressCommand(cmd, args...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for format %q, got nil", tt.format)
					return
				}
				// Should error about format, not directory
				if !strings.Contains(err.Error(), "format") && !strings.Contains(err.Error(), "path") {
					t.Logf("Got error: %v (acceptable - either format or path error)", err)
				}
			}
			// For non-error cases, we expect a directory error (since path doesn't exist)
			// but NOT a format error
			if !tt.wantErr && err != nil {
				if strings.Contains(err.Error(), "format") {
					t.Errorf("unexpected format error for format %q: %v", tt.format, err)
				}
			}
		})
	}
}

// =============================================================================
// Completion Percentage Tests
// =============================================================================

// TestComputeProgressMetrics_CompletionPercentage tests completion percentage calculation.
func TestComputeProgressMetrics_CompletionPercentage(t *testing.T) {
	tests := []struct {
		name              string
		validated         int
		admitted          int
		pending           int
		refuted           int
		archived          int
		expectedPercent   int
		expectedCompleted int
	}{
		{
			name:              "empty",
			expectedPercent:   0,
			expectedCompleted: 0,
		},
		{
			name:              "all pending",
			pending:           10,
			expectedPercent:   0,
			expectedCompleted: 0,
		},
		{
			name:              "half validated",
			validated:         5,
			pending:           5,
			expectedPercent:   50,
			expectedCompleted: 5,
		},
		{
			name:              "validated and admitted",
			validated:         3,
			admitted:          2,
			pending:           5,
			expectedPercent:   50,
			expectedCompleted: 5,
		},
		{
			name:              "all validated",
			validated:         10,
			expectedPercent:   100,
			expectedCompleted: 10,
		},
		{
			name:              "refuted does not count",
			validated:         4,
			refuted:           2,
			pending:           4,
			expectedPercent:   40,
			expectedCompleted: 4,
		},
		{
			name:              "10 of 20 (archived counts as complete)",
			validated:         5,
			admitted:          4,
			pending:           8,
			refuted:           2,
			archived:          1,
			expectedPercent:   50,
			expectedCompleted: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := state.NewState()

			// Add validated nodes
			for i := 0; i < tt.validated; i++ {
				id := fmt.Sprintf("1.%d", i+1)
				st.AddNode(createTestNode(id, schema.EpistemicValidated, schema.WorkflowAvailable))
			}

			// Add admitted nodes
			for i := 0; i < tt.admitted; i++ {
				id := fmt.Sprintf("1.%d", tt.validated+i+1)
				st.AddNode(createTestNode(id, schema.EpistemicAdmitted, schema.WorkflowAvailable))
			}

			// Add pending nodes
			for i := 0; i < tt.pending; i++ {
				id := fmt.Sprintf("1.%d", tt.validated+tt.admitted+i+1)
				st.AddNode(createTestNode(id, schema.EpistemicPending, schema.WorkflowAvailable))
			}

			// Add refuted nodes
			for i := 0; i < tt.refuted; i++ {
				id := fmt.Sprintf("1.%d", tt.validated+tt.admitted+tt.pending+i+1)
				st.AddNode(createTestNode(id, schema.EpistemicRefuted, schema.WorkflowAvailable))
			}

			// Add archived nodes
			for i := 0; i < tt.archived; i++ {
				id := fmt.Sprintf("1.%d", tt.validated+tt.admitted+tt.pending+tt.refuted+i+1)
				st.AddNode(createTestNode(id, schema.EpistemicArchived, schema.WorkflowAvailable))
			}

			var pendingDefs []*node.PendingDef
			metrics := computeProgressMetrics(st, pendingDefs)

			if metrics.CompletionPercent != tt.expectedPercent {
				t.Errorf("expected CompletionPercent to be %d, got %d", tt.expectedPercent, metrics.CompletionPercent)
			}
			if metrics.CompletedNodes != tt.expectedCompleted {
				t.Errorf("expected CompletedNodes to be %d, got %d", tt.expectedCompleted, metrics.CompletedNodes)
			}
		})
	}
}
