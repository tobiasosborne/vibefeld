package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

func setupRepairStatsTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "af-repair-stats-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	return tmpDir, cleanup
}

func executeRepairStatsCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRepairStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// addRepairChallengeAndResolve adds a challenge-resolve cycle via ledger.
func addRepairChallengeAndResolve(t *testing.T, proofDir string, nodeID types.NodeID, idx int) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	chalID := fmt.Sprintf("chal-repair-%d", idx)
	event := ledger.NewChallengeRaisedWithSeverity(chalID, nodeID, "correctness", "Issue found", "major", "verifier-1")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to raise challenge %d: %v", idx, err)
	}
	resolve := ledger.NewChallengeResolved(chalID)
	if _, err := ldg.Append(resolve); err != nil {
		t.Fatalf("failed to resolve challenge %d: %v", idx, err)
	}
}

func TestRepairStatsCmd_ScanNoFatigue(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	output, err := executeRepairStatsCmd(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "No subtrees show repair fatigue") {
		t.Errorf("Expected no fatigue message, got: %s", output)
	}
}

func TestRepairStatsCmd_NodeMetrics(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	output, err := executeRepairStatsCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "Repair Stats for Node 1") {
		t.Errorf("Expected node header, got: %s", output)
	}
	if !strings.Contains(output, "Challenges:") {
		t.Errorf("Expected challenge count, got: %s", output)
	}
}

func TestRepairStatsCmd_InvalidNode(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	_, err := executeRepairStatsCmd(t, "invalid", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}

func TestRepairStatsCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	_, err := executeRepairStatsCmd(t, "1.99", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for nonexistent node")
	}
}

func TestRepairStatsCmd_JSONScan(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	output, err := executeRepairStatsCmd(t, "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}
	if result["count"] != float64(0) {
		t.Errorf("Expected count 0, got: %v", result["count"])
	}
	subtrees, ok := result["fatigued_subtrees"].([]interface{})
	if !ok {
		t.Fatalf("Expected fatigued_subtrees array, got: %T", result["fatigued_subtrees"])
	}
	if len(subtrees) != 0 {
		t.Errorf("Expected empty array, got %d items", len(subtrees))
	}
}

func TestRepairStatsCmd_JSONNode(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	output, err := executeRepairStatsCmd(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}

	nodeMetrics, ok := result["node"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected node metrics object, got: %T", result["node"])
	}
	if nodeMetrics["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", nodeMetrics["node_id"])
	}
}

func TestRepairStatsCmd_WithChallenges(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	// Raise a challenge against node 1 via ledger
	nodeID, _ := types.Parse("1")
	ldg, err := ledger.NewLedger(filepath.Join(tmpDir, "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	event := ledger.NewChallengeRaisedWithSeverity("chal-test-1", nodeID, "correctness", "Is this really true?", "critical", "verifier-1")
	if _, err := ldg.Append(event); err != nil {
		t.Fatal(err)
	}

	output, err := executeRepairStatsCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "1 total") {
		t.Errorf("Expected '1 total' challenge count, got: %s", output)
	}
	if !strings.Contains(output, "1 open") {
		t.Errorf("Expected '1 open' in output, got: %s", output)
	}
}

func TestRepairStatsCmd_FatigueDetection(t *testing.T) {
	tmpDir, cleanup := setupRepairStatsTest(t)
	defer cleanup()

	nodeID, _ := types.Parse("1")

	// Create 5 challenge-resolve cycles to trigger alarm threshold
	for i := 0; i < 5; i++ {
		addRepairChallengeAndResolve(t, tmpDir, nodeID, i)
	}

	// Scan should detect fatigue at default threshold (alarm=5)
	output, err := executeRepairStatsCmd(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "fatigued subtree") {
		t.Errorf("Expected fatigue detection, got: %s", output)
	}
	if !strings.Contains(output, "repair cycles") {
		t.Errorf("Expected repair cycle count, got: %s", output)
	}
}
