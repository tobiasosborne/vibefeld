// Package main contains tests for the global --dry-run guard and the
// commands that implement genuine dry-run previews.
//
// Regression coverage for vibefeld-52ff: --dry-run was a registered global
// persistent flag advertised in `af --help`, but no command honoured it, so
// every mutating command silently ignored it and wrote to the workspace
// anyway. The guard now refuses --dry-run on any command that has not opted
// in, and def-add previews instead of writing.
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newDryRunGuardedRoot builds a root command wired exactly like the real CLI
// with respect to the --dry-run persistent flag and the guard.
func newDryRunGuardedRoot() *cobra.Command {
	root := newTestRootCmd()
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output for debugging")
	root.PersistentFlags().Bool("dry-run", false, "Preview changes without making them")
	root.PersistentPreRunE = dryRunGuard
	return root
}

func setupDryRunDefTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-dryrun-test-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := service.Init(tmpDir, "Test conjecture for dry-run", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	return tmpDir, cleanup
}

// TestDryRunGuard_BlocksUnsupportedCommand ensures --dry-run on a command that
// has not opted in errors out BEFORE the command body runs (no mutation).
func TestDryRunGuard_BlocksUnsupportedCommand(t *testing.T) {
	root := newDryRunGuardedRoot()

	ran := false
	stub := &cobra.Command{
		Use:  "mutate",
		RunE: func(cmd *cobra.Command, args []string) error { ran = true; return nil },
	}
	root.AddCommand(stub)

	out, err := executeCommand(root, "--dry-run", "mutate")
	if err == nil {
		t.Fatal("expected an error when --dry-run is used on an unsupported command")
	}
	if ran {
		t.Error("command body must NOT execute under --dry-run when unsupported (it would mutate)")
	}
	combined := strings.ToLower(out + err.Error())
	if !strings.Contains(combined, "dry-run") {
		t.Errorf("error should mention dry-run, got: %v / %q", err, out)
	}
}

// TestDryRunGuard_AllowsSupportedCommand ensures an opted-in command runs so it
// can implement the preview itself.
func TestDryRunGuard_AllowsSupportedCommand(t *testing.T) {
	root := newDryRunGuardedRoot()

	ran := false
	stub := &cobra.Command{
		Use:  "safe",
		RunE: func(cmd *cobra.Command, args []string) error { ran = true; return nil },
	}
	markDryRunSupported(stub)
	root.AddCommand(stub)

	if _, err := executeCommand(root, "--dry-run", "safe"); err != nil {
		t.Fatalf("expected no guard error for a supported command, got: %v", err)
	}
	if !ran {
		t.Error("supported command should execute so it can handle dry-run itself")
	}
}

// TestDryRunGuard_NoFlagRunsNormally ensures the guard is inert without the flag.
func TestDryRunGuard_NoFlagRunsNormally(t *testing.T) {
	root := newDryRunGuardedRoot()

	ran := false
	stub := &cobra.Command{
		Use:  "mutate",
		RunE: func(cmd *cobra.Command, args []string) error { ran = true; return nil },
	}
	root.AddCommand(stub)

	if _, err := executeCommand(root, "mutate"); err != nil {
		t.Fatalf("unexpected error without --dry-run: %v", err)
	}
	if !ran {
		t.Error("command should run normally when --dry-run is absent")
	}
}

// TestDefAddCmd_SupportsDryRun ensures def-add advertises dry-run support.
func TestDefAddCmd_SupportsDryRun(t *testing.T) {
	if !supportsDryRun(newDefAddCmd()) {
		t.Error("def-add must be marked as supporting --dry-run")
	}
}

func TestRecomputeTaintCmd_SupportsDryRun(t *testing.T) {
	if !supportsDryRun(newRecomputeTaintCmd()) {
		t.Error("recompute-taint must be marked as supporting --dry-run")
	}
}

// TestDefAddCmd_DryRunDoesNotWrite is the core regression for vibefeld-52ff:
// `af def-add --dry-run` must preview without writing a definition.
func TestDefAddCmd_DryRunDoesNotWrite(t *testing.T) {
	tmpDir, cleanup := setupDryRunDefTest(t)
	defer cleanup()

	root := newDryRunGuardedRoot()
	root.AddCommand(newDefAddCmd())

	out, err := executeCommand(root,
		"--dry-run", "def-add", "widget", "A widget is a thing.", "-d", tmpDir)
	if err != nil {
		t.Fatalf("dry-run def-add should not error: %v\nOutput: %s", err, out)
	}

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("failed to open service: %v", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if st.GetDefinitionByName("widget") != nil {
		t.Error("dry-run must NOT write the definition to state")
	}

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "dry-run") && !strings.Contains(lower, "would") {
		t.Errorf("dry-run output should indicate a preview, got: %q", out)
	}
}

// TestDefAddCmd_DryRunReportsExistingName ensures the preview warns when the
// name already exists (the exact duplicate-key symptom from the field report)
// and does not overwrite the existing definition.
func TestDefAddCmd_DryRunReportsExistingName(t *testing.T) {
	tmpDir, cleanup := setupDryRunDefTest(t)
	defer cleanup()

	// Seed a real definition.
	seed := newDefAddCmd()
	sbuf := new(bytes.Buffer)
	seed.SetOut(sbuf)
	seed.SetErr(sbuf)
	seed.SetArgs([]string{"widget", "First.", "-d", tmpDir})
	if err := seed.Execute(); err != nil {
		t.Fatalf("seed add failed: %v", err)
	}

	// Dry-run the same name.
	root := newDryRunGuardedRoot()
	root.AddCommand(newDefAddCmd())
	out, err := executeCommand(root,
		"--dry-run", "def-add", "widget", "Second.", "-d", tmpDir)
	if err != nil {
		t.Fatalf("dry-run should not error: %v", err)
	}

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "exist") && !strings.Contains(lower, "duplicate") {
		t.Errorf("dry-run should warn about the existing name, got: %q", out)
	}

	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	d := st.GetDefinitionByName("widget")
	if d == nil {
		t.Fatal("seed definition missing after dry-run")
	}
	if d.Content != "First." {
		t.Errorf("dry-run must not overwrite the existing definition; content=%q", d.Content)
	}
}
