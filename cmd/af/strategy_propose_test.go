//go:build integration

// Package main contains integration tests for strategy-propose and strategy-list commands.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupStrategyProposeTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-strategy-propose-test-*")
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

func executeStrategyProposeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newStrategyProposeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executeStrategyListCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newStrategyListProposedCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestStrategyProposeCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	output, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Induction on the degree of the polynomial",
		"--novelty", "medium")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Strategy proposed") {
		t.Errorf("Expected confirmation message, got: %s", output)
	}
	if !strings.Contains(output, "Induction on the degree") {
		t.Errorf("Expected strategy in output, got: %s", output)
	}
}

func TestStrategyProposeCmd_WithAllFlags(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	output, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Godement-Jacquet bridge",
		"--novelty", "high",
		"--rationale", "Avoids case splitting entirely",
		"--agent", "prover-002")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Godement-Jacquet") {
		t.Errorf("Expected strategy in output, got: %s", output)
	}
}

func TestStrategyProposeCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	output, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Direct computation via generating functions",
		"--novelty", "low",
		"--rationale", "Applies known theorem X",
		"-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if result["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", result["node_id"])
	}
	if result["strategy"] != "Direct computation via generating functions" {
		t.Errorf("Expected strategy, got: %v", result["strategy"])
	}
	if result["novelty"] != "low" {
		t.Errorf("Expected novelty 'low', got: %v", result["novelty"])
	}
}

func TestStrategyProposeCmd_MissingStrategy(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error when strategy is missing")
	}
}

func TestStrategyProposeCmd_InvalidNovelty(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Some approach", "--novelty", "extreme")
	if err == nil {
		t.Fatal("Expected error for invalid novelty level")
	}

	if !strings.Contains(err.Error(), "novelty") {
		t.Errorf("Error should mention novelty, got: %v", err)
	}
}

func TestStrategyProposeCmd_DefaultNovelty(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	// No --novelty flag should default to "medium"
	output, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Some approach", "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["novelty"] != "medium" {
		t.Errorf("Expected default novelty 'medium', got: %v", result["novelty"])
	}
}

func TestStrategyProposeCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "invalid", "-d", tmpDir,
		"--strategy", "Some approach")
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}

func TestStrategyProposeCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "1.99", "-d", tmpDir,
		"--strategy", "Some approach")
	if err == nil {
		t.Fatal("Expected error for nonexistent node")
	}
}

func TestStrategyListProposedCmd_Empty(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	output, err := executeStrategyListCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "No strategies") {
		t.Errorf("Expected 'no strategies' message, got: %s", output)
	}
}

func TestStrategyListProposedCmd_WithStrategies(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Induction on n", "--novelty", "low",
		"--rationale", "Standard approach")
	if err != nil {
		t.Fatalf("Failed to propose first strategy: %v", err)
	}

	_, err = executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Godement-Jacquet bridge", "--novelty", "high",
		"--rationale", "Novel approach avoiding case splits", "--agent", "prover-002")
	if err != nil {
		t.Fatalf("Failed to propose second strategy: %v", err)
	}

	output, err := executeStrategyListCommand(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "2 total") {
		t.Errorf("Expected '2 total' in output, got: %s", output)
	}
	if !strings.Contains(output, "Induction on n") {
		t.Errorf("Expected first strategy, got: %s", output)
	}
	if !strings.Contains(output, "Godement-Jacquet") {
		t.Errorf("Expected second strategy, got: %s", output)
	}
	if !strings.Contains(output, "high") {
		t.Errorf("Expected novelty in output, got: %s", output)
	}
}

func TestStrategyListProposedCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyProposeCommand(t, "1", "-d", tmpDir,
		"--strategy", "Direct computation", "--novelty", "medium")
	if err != nil {
		t.Fatalf("Failed to propose strategy: %v", err)
	}

	output, err := executeStrategyListCommand(t, "1", "-d", tmpDir, "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}

	if result["count"] != float64(1) {
		t.Errorf("Expected count 1, got: %v", result["count"])
	}

	strategies, ok := result["strategies"].([]interface{})
	if !ok || len(strategies) != 1 {
		t.Fatalf("Expected 1 strategy in array, got: %v", result["strategies"])
	}

	first := strategies[0].(map[string]interface{})
	if first["strategy"] != "Direct computation" {
		t.Errorf("Expected strategy text, got: %v", first["strategy"])
	}
	if first["novelty"] != "medium" {
		t.Errorf("Expected novelty 'medium', got: %v", first["novelty"])
	}
}

func TestStrategyListProposedCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupStrategyProposeTest(t)
	defer cleanup()

	_, err := executeStrategyListCommand(t, "invalid", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}
