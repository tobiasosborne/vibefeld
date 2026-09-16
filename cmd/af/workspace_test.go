//go:build !integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// newWorkspaceTestRoot builds a root command with the workspace command and the
// global --dry-run flag that the real root registers.
func newWorkspaceTestRoot() *cobra.Command {
	root := newTestRootCmd()
	root.PersistentFlags().Bool("dry-run", false, "Preview changes without making them")
	root.AddCommand(newWorkspaceCmd())
	return root
}

func executeWorkspace(t *testing.T, root *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupFormatWorkspace creates a real workspace (via service.Init) and rewrites
// its meta.json to the requested format to simulate an older workspace.
func setupFormatWorkspace(t *testing.T, version string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proof")
	if err := service.Init(dir, "Test conjecture", "test-author"); err != nil {
		t.Fatalf("service.Init: %v", err)
	}
	metaPath := filepath.Join(dir, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Version = version
	if err := config.Save(cfg, metaPath); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	return dir
}

func readMetaVersion(t *testing.T, dir string) string {
	t.Helper()
	cfg, err := config.Load(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg.Version
}

func TestWorkspaceUpgrade_DryRunWritesNothing(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")
	root := newWorkspaceTestRoot()

	out, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1", "--dry-run")
	if err != nil {
		t.Fatalf("upgrade --dry-run error = %v, output = %s", err, out)
	}
	if got := readMetaVersion(t, dir); got != "1.0" {
		t.Errorf("meta version after dry-run = %q, want 1.0", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup")); !os.IsNotExist(err) {
		t.Errorf("backup directory must not exist after dry-run (stat err=%v)", err)
	}
}

func TestWorkspaceUpgrade_BacksUpAndStamps(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")
	root := newWorkspaceTestRoot()

	out, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1", "-f", "json")
	if err != nil {
		t.Fatalf("upgrade error = %v, output = %s", err, out)
	}
	if got := readMetaVersion(t, dir); got != "1.1" {
		t.Fatalf("meta version after upgrade = %q, want 1.1", got)
	}

	backupRoot := filepath.Join(dir, "backup")
	entries, err := os.ReadDir(backupRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one backup dir, entries=%v err=%v", entries, err)
	}
	backup := filepath.Join(backupRoot, entries[0].Name())
	if _, err := os.Stat(filepath.Join(backup, "meta.json")); err != nil {
		t.Errorf("backup meta.json missing: %v", err)
	}
	ledgerBackup := filepath.Join(backup, "ledger")
	if _, err := os.Stat(ledgerBackup); err != nil {
		t.Fatalf("backup ledger/ missing: %v", err)
	}
	files, err := os.ReadDir(ledgerBackup)
	if err != nil || len(files) == 0 {
		t.Fatalf("backup ledger has no event files: files=%v err=%v", files, err)
	}
}

func TestWorkspaceUpgrade_Idempotent(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.1")
	root := newWorkspaceTestRoot()

	out, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1")
	if err != nil {
		t.Fatalf("idempotent upgrade error = %v, output = %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup")); !os.IsNotExist(err) {
		t.Errorf("idempotent upgrade must not create a backup (stat err=%v)", err)
	}
}

func TestWorkspaceUpgrade_RefusesDowngrade(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.1")
	root := newWorkspaceTestRoot()

	_, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.0")
	if err == nil {
		t.Fatal("downgrade must be refused")
	}
	if aferrors.Code(err) != aferrors.INVALID_STATE {
		t.Errorf("downgrade code = %v, want INVALID_STATE", aferrors.Code(err))
	}
}

func TestWorkspaceUpgrade_RejectsUnknownTarget(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")
	root := newWorkspaceTestRoot()

	_, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "9.9")
	if err == nil {
		t.Fatal("unknown target must be rejected")
	}
	if aferrors.Code(err) != aferrors.INVALID_TARGET {
		t.Errorf("unknown target code = %v, want INVALID_TARGET", aferrors.Code(err))
	}
}
