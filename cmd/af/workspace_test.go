//go:build !integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
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

// TestWorkspaceUpgrade_InvalidInputExitCodes pins the exit-3 (logic) taxonomy
// for bad command-line input: retriable plain errors must not leak out.
func TestWorkspaceUpgrade_InvalidInputExitCodes(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")
	root := newWorkspaceTestRoot()

	cases := []struct {
		name string
		args []string
		code aferrors.ErrorCode
	}{
		{"missing-to", []string{"workspace", "upgrade", "--dir", dir}, aferrors.EMPTY_INPUT},
		{"bad-target", []string{"workspace", "upgrade", "--dir", dir, "--to", "9.9"}, aferrors.INVALID_TARGET},
		{"bad-format", []string{"workspace", "upgrade", "--dir", dir, "--to", "1.1", "-f", "xml"}, aferrors.INVALID_TYPE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeWorkspace(t, root, tc.args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if aferrors.Code(err) != tc.code {
				t.Errorf("code = %v, want %v", aferrors.Code(err), tc.code)
			}
			if aferrors.ExitCode(err) != 3 {
				t.Errorf("exit code = %d, want 3", aferrors.ExitCode(err))
			}
		})
	}
}

// TestWorkspaceUpgrade_BackupNameCollisionUsesDistinctDir proves two upgrades
// cannot share or overwrite a backup directory: if the timestamped name exists,
// a suffixed, distinct directory is used and the existing one is untouched.
func TestWorkspaceUpgrade_BackupNameCollisionUsesDistinctDir(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")

	fixed := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	restoreNow := workspaceUpgradeNow
	workspaceUpgradeNow = func() time.Time { return fixed }
	defer func() { workspaceUpgradeNow = restoreNow }()

	collision := filepath.Join(dir, "backup", fixed.Format("20060102T150405.000000000Z"))
	if err := os.MkdirAll(collision, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(collision, "keep")
	if err := os.WriteFile(marker, []byte("untouched"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newWorkspaceTestRoot()
	out, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1", "-f", "json")
	if err != nil {
		t.Fatalf("upgrade error = %v, output = %s", err, out)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "backup"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected collision dir plus fresh backup, got %v", entries)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "untouched" {
		t.Fatalf("pre-existing backup dir was modified: err=%v data=%q", err, data)
	}
	if got := readMetaVersion(t, dir); got != "1.1" {
		t.Errorf("meta version = %q, want 1.1", got)
	}
}

// TestWorkspaceUpgrade_LockHeldBeforeMetaRead proves meta.json is not read
// until the ledger lock is held: with the lock held and meta.json deliberately
// corrupt, the upgrade fails on the lock, never on a JSON parse error.
func TestWorkspaceUpgrade_LockHeldBeforeMetaRead(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	lock := ledger.NewLedgerLock(filepath.Join(dir, "ledger"))
	if err := lock.Acquire("test-holder", time.Second); err != nil {
		t.Fatalf("test holder could not acquire lock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	restoreTimeout := workspaceUpgradeLockTimeout
	workspaceUpgradeLockTimeout = 50 * time.Millisecond
	defer func() { workspaceUpgradeLockTimeout = restoreTimeout }()

	root := newWorkspaceTestRoot()
	_, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1")
	if err == nil {
		t.Fatal("upgrade while the lock is held must fail")
	}
	if !strings.Contains(err.Error(), "lock") {
		t.Fatalf("upgrade must fail on the lock before reading meta.json; got %v", err)
	}
}

// TestWorkspaceUpgrade_DurabilityOrder asserts the backup is complete and its
// root directory fsynced before the new stamp is written, so a crash cannot
// leave a durable stamp whose backup is not durable.
func TestWorkspaceUpgrade_DurabilityOrder(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.0")

	restoreSteps := upgradeSteps
	upgradeSteps = []string{}
	defer func() { upgradeSteps = restoreSteps }()

	root := newWorkspaceTestRoot()
	if _, err := executeWorkspace(t, root, "workspace", "upgrade", "--dir", dir, "--to", "1.1"); err != nil {
		t.Fatalf("upgrade error = %v", err)
	}

	idx := func(step string) int {
		for i, s := range upgradeSteps {
			if s == step {
				return i
			}
		}
		return -1
	}
	for _, step := range []string{"lock", "load-config", "backup-dir", "backup", "fsync-root", "write-stamp"} {
		if idx(step) < 0 {
			t.Fatalf("step %q was not recorded; steps=%v", step, upgradeSteps)
		}
	}
	if idx("lock") > idx("load-config") {
		t.Errorf("lock must precede load-config: %v", upgradeSteps)
	}
	if !(idx("backup") < idx("fsync-root") && idx("fsync-root") < idx("write-stamp")) {
		t.Errorf("durability order must be backup < fsync-root < write-stamp: %v", upgradeSteps)
	}
}
