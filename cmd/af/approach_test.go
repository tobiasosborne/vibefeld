// Package main contains integration tests for approach-tried and approach-list commands.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

// setupApproachTest creates a temp proof dir with an initialized proof.
func setupApproachTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-approach-test-*")
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

func executeApproachTriedCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newApproachTriedCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executeApproachListCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newApproachListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestApproachTriedCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	output, err := executeApproachTriedCmd(t, "1", "-d", tmpDir,
		"--approach", "Induction on n", "--outcome", "Base case fails for n=0")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Failed approach recorded") {
		t.Errorf("Expected confirmation message, got: %s", output)
	}
	if !strings.Contains(output, "Induction on n") {
		t.Errorf("Expected approach in output, got: %s", output)
	}
}

func TestApproachTriedCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	output, err := executeApproachTriedCmd(t, "1", "-d", tmpDir,
		"-a", "By contradiction", "-o", "Leads to paradox", "-f", "json")
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
	if result["approach"] != "By contradiction" {
		t.Errorf("Expected approach, got: %v", result["approach"])
	}
}

func TestApproachTriedCmd_MissingApproach(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	_, err := executeApproachTriedCmd(t, "1", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error when approach is missing")
	}
}

func TestApproachTriedCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	_, err := executeApproachTriedCmd(t, "invalid", "-d", tmpDir,
		"-a", "some approach")
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}

func TestApproachTriedCmd_NodeNotFound(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	_, err := executeApproachTriedCmd(t, "1.99", "-d", tmpDir,
		"-a", "some approach")
	if err == nil {
		t.Fatal("Expected error for nonexistent node")
	}
}

func TestApproachListCmd_Empty(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	output, err := executeApproachListCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "No failed approaches") {
		t.Errorf("Expected 'no failed approaches' message, got: %s", output)
	}
}

func TestApproachListCmd_WithApproaches(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	// Record two approaches
	_, err := executeApproachTriedCmd(t, "1", "-d", tmpDir,
		"-a", "Induction on n", "-o", "Base case fails")
	if err != nil {
		t.Fatalf("Failed to record first approach: %v", err)
	}

	_, err = executeApproachTriedCmd(t, "1", "-d", tmpDir,
		"-a", "By contradiction", "-o", "Circular reasoning", "--agent", "prover-001")
	if err != nil {
		t.Fatalf("Failed to record second approach: %v", err)
	}

	// List approaches
	output, err := executeApproachListCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "2 total") {
		t.Errorf("Expected '2 total' in output, got: %s", output)
	}
	if !strings.Contains(output, "Induction on n") {
		t.Errorf("Expected first approach in output, got: %s", output)
	}
	if !strings.Contains(output, "By contradiction") {
		t.Errorf("Expected second approach in output, got: %s", output)
	}
	if !strings.Contains(output, "prover-001") {
		t.Errorf("Expected prover ID in output, got: %s", output)
	}
}

func TestApproachListCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	_, err := executeApproachTriedCmd(t, "1", "-d", tmpDir,
		"-a", "Direct computation", "-o", "Overflow at step 7")
	if err != nil {
		t.Fatalf("Failed to record approach: %v", err)
	}

	output, err := executeApproachListCmd(t, "1", "-d", tmpDir, "-f", "json")
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

	approaches, ok := result["approaches"].([]interface{})
	if !ok || len(approaches) != 1 {
		t.Fatalf("Expected 1 approach in array, got: %v", result["approaches"])
	}

	first := approaches[0].(map[string]interface{})
	if first["approach"] != "Direct computation" {
		t.Errorf("Expected approach text, got: %v", first["approach"])
	}
}

func TestApproachListCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupApproachTest(t)
	defer cleanup()

	_, err := executeApproachListCmd(t, "invalid", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}
