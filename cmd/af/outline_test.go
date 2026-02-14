package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupOutlineTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "af-outline-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.InitProofDir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		t.Fatal(err)
	}
	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

func executeOutlineSetCmd(t *testing.T, dir string, stages ...string) (string, error) {
	t.Helper()
	cmd := newOutlineSetCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	args := []string{"--dir", dir}
	for _, s := range stages {
		args = append(args, "--stage", s)
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executeOutlineShowCmd(t *testing.T, dir string, extraArgs ...string) (string, error) {
	t.Helper()
	cmd := newOutlineCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	args := append([]string{"--dir", dir}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executeOutlineMapCmd(t *testing.T, label, nodeID, dir string) (string, error) {
	t.Helper()
	cmd := newOutlineMapCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{label, nodeID, "--dir", dir})
	err := cmd.Execute()
	return buf.String(), err
}

func executeCoverageCmd(t *testing.T, dir string, extraArgs ...string) (string, error) {
	t.Helper()
	cmd := newCoverageCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	args := append([]string{"--dir", dir}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestOutlineSet_Success(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	output, err := executeOutlineSetCmd(t, tmpDir,
		"A:critical:Establish base case",
		"B:important:Prove inductive step",
	)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "Outline set with 2 stages") {
		t.Errorf("Expected confirmation, got: %s", output)
	}
}

func TestOutlineShow_NoOutline(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	output, err := executeOutlineShowCmd(t, tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if !strings.Contains(output, "No outline set") {
		t.Errorf("Expected 'No outline set' message, got: %s", output)
	}
}

func TestOutlineShow_WithStages(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, err := executeOutlineSetCmd(t, tmpDir,
		"A:critical:Base case",
		"B:routine:Cleanup",
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err := executeOutlineShowCmd(t, tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if !strings.Contains(output, "A (critical)") {
		t.Errorf("Expected stage A, got: %s", output)
	}
	if !strings.Contains(output, "B (routine)") {
		t.Errorf("Expected stage B, got: %s", output)
	}
}

func TestOutlineShow_JSON(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case")

	output, err := executeOutlineShowCmd(t, tmpDir, "--format", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}
	if result["stages_total"] != float64(1) {
		t.Errorf("Expected 1 stage, got: %v", result["stages_total"])
	}
}

func TestOutlineMap_Success(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case")

	output, err := executeOutlineMapCmd(t, "A", "1", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "Stage \"A\" mapped to node 1") {
		t.Errorf("Expected confirmation, got: %s", output)
	}
}

func TestOutlineMap_InvalidLabel(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case")

	_, err := executeOutlineMapCmd(t, "Z", "1", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid label")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestOutlineMap_InvalidNode(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case")

	_, err := executeOutlineMapCmd(t, "A", "99", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid node")
	}
}

func TestCoverage_WithMappedStage(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case", "B:critical:Conclusion")
	_, _ = executeOutlineMapCmd(t, "A", "1", tmpDir)

	output, err := executeCoverageCmd(t, tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "1 mapped") {
		t.Errorf("Expected '1 mapped', got: %s", output)
	}
	if !strings.Contains(output, "critical stage(s) untouched: B") {
		t.Errorf("Expected untouched warning for B, got: %s", output)
	}
}

func TestHealth_CriticalUntouched(t *testing.T) {
	tmpDir, cleanup := setupOutlineTest(t)
	defer cleanup()

	_, _ = executeOutlineSetCmd(t, tmpDir, "A:critical:Base case")

	cmd := newHealthCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "critical outline stage") {
		t.Errorf("Expected critical stage warning in health, got: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Errorf("Expected WARNING status, got: %s", output)
	}
}
