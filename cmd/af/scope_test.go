//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupScopeTest creates a temporary directory with an initialized proof.
func setupScopeTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize the proof directory structure
	if err := service.InitProofDir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Initialize proof via service
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		t.Fatal(err)
	}

	return tmpDir, func() {}
}

// executeScopeCommand creates and executes a scope command with the given arguments.
func executeScopeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newScopeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// =============================================================================
// Basic Scope Tests
// =============================================================================

func TestScopeCmd_NoProof(t *testing.T) {
	dir := t.TempDir()

	_, err := executeScopeCommand(t, "1", "--dir", dir)

	if err == nil {
		t.Fatal("expected error for uninitialized proof")
	}
}

func TestScopeCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	_, err := executeScopeCommand(t, "1.99", "--dir", tmpDir) // Non-existent node

	if err == nil {
		t.Fatal("expected error for non-existent node")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention node doesn't exist, got: %v", err)
	}
}

func TestScopeCmd_NodeNotInScope(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	output, err := executeScopeCommand(t, "1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Root node should not be in any scope
	if !strings.Contains(output, "not inside any scope") && !strings.Contains(output, "Scope depth: 0") {
		t.Errorf("expected output to show node not in scope, got: %s", output)
	}
}

func TestScopeCmd_NodeInScope(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create local assume node
	assumeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(assumeID, service.NodeTypeLocalAssume, "Assume x > 0", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}

	// Open scope via ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	scopeEvent := ledger.NewScopeOpened(assumeID, "Assume x > 0")
	if _, err := ldg.Append(scopeEvent); err != nil {
		t.Fatal(err)
	}

	// Create child node inside scope
	childID, _ := service.ParseNodeID("1.1.1")
	err = svc.CreateNode(childID, service.NodeTypeClaim, "Therefore something", service.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeScopeCommand(t, "1.1.1", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Scope depth: 1") && !strings.Contains(output, "1.1") {
		t.Errorf("expected output to show node in scope 1.1, got: %s", output)
	}
}

func TestScopeCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create local assume node
	assumeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(assumeID, service.NodeTypeLocalAssume, "Assume x > 0", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}

	// Open scope via ledger
	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	scopeEvent := ledger.NewScopeOpened(assumeID, "Assume x > 0")
	if _, err := ldg.Append(scopeEvent); err != nil {
		t.Fatal(err)
	}

	// Create child node
	childID, _ := service.ParseNodeID("1.1.1")
	err = svc.CreateNode(childID, service.NodeTypeClaim, "Therefore", service.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeScopeCommand(t, "1.1.1", "--dir", tmpDir, "--format", "json")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON is valid
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Check depth is present
	if _, ok := result["depth"]; !ok {
		t.Error("expected 'depth' field in JSON output")
	}
}

func TestScopeCmd_ShowAllScopes(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	// First scope
	assume1ID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(assume1ID, service.NodeTypeLocalAssume, "Assume A", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ldg.Append(ledger.NewScopeOpened(assume1ID, "Assume A")); err != nil {
		t.Fatal(err)
	}

	// Second scope
	assume2ID, _ := service.ParseNodeID("1.2")
	err = svc.CreateNode(assume2ID, service.NodeTypeLocalAssume, "Assume B", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ldg.Append(ledger.NewScopeOpened(assume2ID, "Assume B")); err != nil {
		t.Fatal(err)
	}

	output, err := executeScopeCommand(t, "--all", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "1.1") || !strings.Contains(output, "1.2") {
		t.Errorf("expected output to list both scopes, got: %s", output)
	}
	if !strings.Contains(output, "Assume A") || !strings.Contains(output, "Assume B") {
		t.Errorf("expected output to show scope statements, got: %s", output)
	}
}

func TestScopeCmd_AllScopesJSON(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create scope
	assumeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(assumeID, service.NodeTypeLocalAssume, "Assume A", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ldg.Append(ledger.NewScopeOpened(assumeID, "Assume A")); err != nil {
		t.Fatal(err)
	}

	output, err := executeScopeCommand(t, "--all", "--dir", tmpDir, "--format", "json")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON is valid
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Check required fields
	if _, ok := result["total_count"]; !ok {
		t.Error("expected 'total_count' field in JSON output")
	}
	if _, ok := result["active_count"]; !ok {
		t.Error("expected 'active_count' field in JSON output")
	}
}

func TestScopeCmd_MissingNodeID(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	_, err := executeScopeCommand(t, "--dir", tmpDir)

	if err == nil {
		t.Fatal("expected error for missing node ID without --all")
	}

	if !strings.Contains(err.Error(), "node ID required") {
		t.Errorf("error should mention node ID required, got: %v", err)
	}
}

func TestScopeCmd_ClosedScope(t *testing.T) {
	tmpDir, cleanup := setupScopeTest(t)
	defer cleanup()

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ledgerDir := filepath.Join(svc.Path(), "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create and open scope
	assumeID, _ := service.ParseNodeID("1.1")
	err = svc.CreateNode(assumeID, service.NodeTypeLocalAssume, "Assume A", service.InferenceAssumption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ldg.Append(ledger.NewScopeOpened(assumeID, "Assume A")); err != nil {
		t.Fatal(err)
	}

	// Close the scope
	dischargeID, _ := service.ParseNodeID("1.1.1")
	if _, err := ldg.Append(ledger.NewScopeClosed(assumeID, dischargeID)); err != nil {
		t.Fatal(err)
	}

	output, err := executeScopeCommand(t, "--all", "--dir", tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show closed scope
	if !strings.Contains(output, "closed") && !strings.Contains(output, "Closed") {
		t.Errorf("expected output to show closed scope, got: %s", output)
	}
}
