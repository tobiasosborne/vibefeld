// Package main contains integration tests for attach and evidence commands.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupEvidenceTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-evidence-test-*")
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

// createTestFile creates a file with content inside the proof dir.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return name
}

func executeAttachCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newAttachCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executeEvidenceCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newEvidenceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestAttachCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "verify.py", "import math\nprint('ok')\n")

	output, err := executeAttachCmd(t, "1", "verify.py", "-d", tmpDir,
		"--type", "verification", "--description", "Basic check")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Evidence attached") {
		t.Errorf("Expected confirmation, got: %s", output)
	}
}

func TestAttachCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "check.py", "assert True\n")

	output, err := executeAttachCmd(t, "1", "check.py", "-d", tmpDir,
		"-t", "test", "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}
	if result["node_id"] != "1" {
		t.Errorf("Expected node_id '1', got: %v", result["node_id"])
	}
}

func TestAttachCmd_InvalidNodeID(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "test.py", "pass\n")

	_, err := executeAttachCmd(t, "invalid", "test.py", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node ID")
	}
}

func TestAttachCmd_FileNotFound(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	_, err := executeAttachCmd(t, "1", "nonexistent.py", "-d", tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing file")
	}
}

func TestAttachCmd_InvalidType(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "test.py", "pass\n")

	_, err := executeAttachCmd(t, "1", "test.py", "-d", tmpDir, "--type", "bogus")
	if err == nil {
		t.Fatal("Expected error for invalid evidence type")
	}
}

func TestEvidenceCmd_Empty(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	output, err := executeEvidenceCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "No evidence") {
		t.Errorf("Expected 'No evidence' message, got: %s", output)
	}
}

func TestEvidenceCmd_WithEvidence(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "verify.py", "import math\nresult = math.sqrt(2)\n")
	createTestFile(t, tmpDir, "monte_carlo.py", "# 100K trials\n")

	_, err := executeAttachCmd(t, "1", "verify.py", "-d", tmpDir,
		"-t", "verification", "--description", "Sqrt2 check")
	if err != nil {
		t.Fatalf("Failed to attach first: %v", err)
	}

	_, err = executeAttachCmd(t, "1", "monte_carlo.py", "-d", tmpDir,
		"-t", "computation", "--agent", "prover-001")
	if err != nil {
		t.Fatalf("Failed to attach second: %v", err)
	}

	output, err := executeEvidenceCmd(t, "1", "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "2 total") {
		t.Errorf("Expected '2 total', got: %s", output)
	}
	if !strings.Contains(output, "verify.py") {
		t.Errorf("Expected verify.py in output, got: %s", output)
	}
	if !strings.Contains(output, "monte_carlo.py") {
		t.Errorf("Expected monte_carlo.py in output, got: %s", output)
	}
}

func TestEvidenceCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupEvidenceTest(t)
	defer cleanup()

	createTestFile(t, tmpDir, "check.jl", "println(\"ok\")\n")

	_, err := executeAttachCmd(t, "1", "check.jl", "-d", tmpDir, "-t", "test")
	if err != nil {
		t.Fatalf("Failed to attach: %v", err)
	}

	output, err := executeEvidenceCmd(t, "1", "-d", tmpDir, "-f", "json")
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

	items := result["evidence"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["file_path"] != "check.jl" {
		t.Errorf("Expected file_path 'check.jl', got: %v", first["file_path"])
	}
	if first["content_hash"] == nil || first["content_hash"] == "" {
		t.Error("Expected non-empty content_hash")
	}
}
