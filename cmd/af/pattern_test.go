// Package main contains integration tests for pattern-add and pattern-list commands.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupPatternTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-pattern-test-*")
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

func executePatternAddCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newPatternAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func executePatternListCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newPatternListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestPatternAddCmd_Success(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	output, err := executePatternAddCmd(t, "-d", tmpDir,
		"--name", "continuum-to-finite",
		"--description", "Applying infinite-dimensional identities at finite n",
		"--indicators", "Claims using limits for finite objects",
		"--remediation", "Verify identity holds at finite n")
	if err != nil {
		t.Fatalf("Expected success, got error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Failure pattern registered") {
		t.Errorf("Expected confirmation message, got: %s", output)
	}
	if !strings.Contains(output, "continuum-to-finite") {
		t.Errorf("Expected pattern name in output, got: %s", output)
	}
}

func TestPatternAddCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	output, err := executePatternAddCmd(t, "-d", tmpDir,
		"-n", "known-problem-sub", "-D", "Reducing novel to solved", "-f", "json")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}

	if result["name"] != "known-problem-sub" {
		t.Errorf("Expected name, got: %v", result["name"])
	}
}

func TestPatternAddCmd_MissingName(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	_, err := executePatternAddCmd(t, "-d", tmpDir, "-D", "some desc")
	if err == nil {
		t.Fatal("Expected error when name is missing")
	}
}

func TestPatternAddCmd_MissingDescription(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	_, err := executePatternAddCmd(t, "-d", tmpDir, "-n", "test-pattern")
	if err == nil {
		t.Fatal("Expected error when description is missing")
	}
}

func TestPatternListCmd_Empty(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	output, err := executePatternListCmd(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "No failure patterns") {
		t.Errorf("Expected 'no failure patterns' message, got: %s", output)
	}
}

func TestPatternListCmd_WithPatterns(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	// Add two patterns
	_, err := executePatternAddCmd(t, "-d", tmpDir,
		"-n", "continuum-to-finite", "-D", "Applying infinite-dim identities at finite n",
		"-I", "Claims using limits", "-R", "Verify at finite n")
	if err != nil {
		t.Fatalf("Failed to add first pattern: %v", err)
	}

	_, err = executePatternAddCmd(t, "-d", tmpDir,
		"-n", "known-problem-sub", "-D", "Reducing novel problems to solved ones",
		"--agent", "expert-001")
	if err != nil {
		t.Fatalf("Failed to add second pattern: %v", err)
	}

	output, err := executePatternListCmd(t, "-d", tmpDir)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(output, "2 total") {
		t.Errorf("Expected '2 total' in output, got: %s", output)
	}
	if !strings.Contains(output, "continuum-to-finite") {
		t.Errorf("Expected first pattern in output, got: %s", output)
	}
	if !strings.Contains(output, "known-problem-sub") {
		t.Errorf("Expected second pattern in output, got: %s", output)
	}
	if !strings.Contains(output, "expert-001") {
		t.Errorf("Expected agent in output, got: %s", output)
	}
}

func TestPatternListCmd_JSONFormat(t *testing.T) {
	tmpDir, cleanup := setupPatternTest(t)
	defer cleanup()

	_, err := executePatternAddCmd(t, "-d", tmpDir,
		"-n", "divergence-misinterpret", "-D", "Treating divergence as obstacle")
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	output, err := executePatternListCmd(t, "-d", tmpDir, "-f", "json")
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

	patterns, ok := result["patterns"].([]interface{})
	if !ok || len(patterns) != 1 {
		t.Fatalf("Expected 1 pattern, got: %v", result["patterns"])
	}

	first := patterns[0].(map[string]interface{})
	if first["name"] != "divergence-misinterpret" {
		t.Errorf("Expected pattern name, got: %v", first["name"])
	}
}
