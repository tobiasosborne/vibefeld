package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchCmd_NoFilters(t *testing.T) {
	// Create temp directory with initialized proof
	dir := t.TempDir()

	// Initialize a proof first
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Try search without any filters
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no filters provided")
	}
	if !strings.Contains(err.Error(), "at least one search criterion required") {
		t.Errorf("Expected 'at least one search criterion required' error, got: %v", err)
	}
}

func TestSearchCmd_InvalidState(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Try search with invalid state
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", dir, "--state", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid state filter")
	}
	errStr := err.Error()
	// Check for new format with valid values and examples
	if !strings.Contains(errStr, `invalid value "invalid" for --state`) {
		t.Errorf("Expected 'invalid value for --state' error, got: %v", err)
	}
	if !strings.Contains(errStr, "Valid values for --state:") {
		t.Errorf("Expected valid values section, got: %v", err)
	}
	if !strings.Contains(errStr, "pending") {
		t.Errorf("Expected 'pending' in valid values, got: %v", err)
	}
	if !strings.Contains(errStr, "Examples:") {
		t.Errorf("Expected examples section, got: %v", err)
	}
}

func TestSearchCmd_InvalidWorkflow(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Try search with invalid workflow state
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", dir, "--workflow", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid workflow filter")
	}
	errStr := err.Error()
	// Check for new format with valid values and examples
	if !strings.Contains(errStr, `invalid value "invalid" for --workflow`) {
		t.Errorf("Expected 'invalid value for --workflow' error, got: %v", err)
	}
	if !strings.Contains(errStr, "Valid values for --workflow:") {
		t.Errorf("Expected valid values section, got: %v", err)
	}
	if !strings.Contains(errStr, "available") {
		t.Errorf("Expected 'available' in valid values, got: %v", err)
	}
	if !strings.Contains(errStr, "Examples:") {
		t.Errorf("Expected examples section, got: %v", err)
	}
}

func TestSearchCmd_TextSearch(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof with a specific claim
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "The limit of convergent sequences is unique", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search for text in claim
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--text", "convergent"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected to find node [1] in search results")
	}
	if !strings.Contains(output, "text match") {
		t.Error("Expected 'text match' in match reason")
	}
}

func TestSearchCmd_TextSearchCaseInsensitive(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "The THEOREM states something", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search with lowercase
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--text", "theorem"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected case-insensitive match to find node [1]")
	}
}

func TestSearchCmd_TextSearchNoMatch(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim about mathematics", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search for non-existent text
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--text", "nonexistent"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No matching nodes found") {
		t.Error("Expected 'No matching nodes found' message")
	}
}

func TestSearchCmd_StateFilter(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof (root node is pending by default)
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search for pending nodes
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--state", "pending"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected to find pending node [1]")
	}
	if !strings.Contains(output, "state: pending") {
		t.Error("Expected 'state: pending' in match reason")
	}
}

func TestSearchCmd_WorkflowFilter(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof (root node is available by default)
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search for available nodes
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--workflow", "available"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected to find available node [1]")
	}
	if !strings.Contains(output, "workflow: available") {
		t.Error("Expected 'workflow: available' in match reason")
	}
}

func TestSearchCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test claim", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search with JSON output
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--state", "pending", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"id":"1"`) {
		t.Error("Expected JSON output with node ID")
	}
	if !strings.Contains(output, `"total":1`) {
		t.Error("Expected JSON output with total count")
	}
	if !strings.Contains(output, `"results":[`) {
		t.Error("Expected JSON output with results array")
	}
}

func TestSearchCmd_CombinedFilters(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Unique limit theorem", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search with combined filters
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dir", dir, "--text", "limit", "--state", "pending"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected to find node [1] with combined filters")
	}
	if !strings.Contains(output, "text match") {
		t.Error("Expected 'text match' in match reason")
	}
	if !strings.Contains(output, "state: pending") {
		t.Error("Expected 'state: pending' in match reason")
	}
}

func TestSearchCmd_PositionalArgument(t *testing.T) {
	dir := t.TempDir()

	// Initialize a proof
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--dir", dir, "--conjecture", "Test convergence", "--author", "test"})
	initCmd.SetOut(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to initialize proof: %v", err)
	}

	// Search using positional argument instead of --text flag
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"convergence", "--dir", dir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[1]") {
		t.Error("Expected positional argument to work as text search")
	}
}

func TestSearchCmd_NotInitialized(t *testing.T) {
	dir := t.TempDir()

	// Try search without initializing proof
	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", dir, "--state", "pending"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for uninitialized proof")
	}
	// Could be "not initialized" or a directory access error
	errStr := err.Error()
	if !strings.Contains(errStr, "not initialized") && !strings.Contains(errStr, "error") {
		t.Errorf("Expected error for uninitialized proof, got: %v", err)
	}
}

func TestSearchCmd_InvalidDirectory(t *testing.T) {
	nonexistent := filepath.Join(os.TempDir(), "nonexistent-dir-12345")

	cmd := newSearchCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", nonexistent, "--state", "pending"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid directory")
	}
}
