package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

func executeLedgerLockReap(t *testing.T, dir string) (string, error) {
	t.Helper()
	cmd := newReapCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--ledger-lock", "-d", dir, "-f", "json"})
	err := cmd.Execute()
	return buf.String(), err
}

func setupLedgerLockProof(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := service.Init(dir, "Test conjecture", "author"); err != nil {
		t.Fatalf("service.Init: %v", err)
	}
	return dir, filepath.Join(dir, "ledger", "ledger.lock")
}

func TestReapLedgerLock_LivePIDRefused(t *testing.T) {
	dir, lockPath := setupLedgerLockProof(t)

	meta := `{"agent_id":"live","pid":` + strconv.Itoa(os.Getpid()) + `,"acquired_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(lockPath, []byte(meta), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	if _, err := executeLedgerLockReap(t, dir); err == nil {
		t.Fatal("expected a non-nil error (exit 1) for a live lock")
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatal("a live lock must not be removed")
	}
}

func TestReapLedgerLock_DeadPIDRemoved(t *testing.T) {
	dir, lockPath := setupLedgerLockProof(t)

	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait child: %v", err)
	}

	meta := `{"agent_id":"dead","pid":` + strconv.Itoa(deadPID) + `,"acquired_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(lockPath, []byte(meta), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	if _, err := executeLedgerLockReap(t, dir); err != nil {
		t.Fatalf("reap dead-pid lock: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("dead-pid lock should have been removed")
	}
}

func TestReapLedgerLock_LegacyContentHandled(t *testing.T) {
	dir, lockPath := setupLedgerLockProof(t)

	if err := os.WriteFile(lockPath, []byte("legacy-agent"), 0600); err != nil {
		t.Fatalf("write legacy lock: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if _, err := executeLedgerLockReap(t, dir); err != nil {
		t.Fatalf("reap legacy lock: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("old legacy lock should have been removed")
	}
}

func TestReapLedgerLock_NoLockIsNoOp(t *testing.T) {
	dir, _ := setupLedgerLockProof(t)
	if _, err := executeLedgerLockReap(t, dir); err != nil {
		t.Fatalf("reap with no lock: %v", err)
	}
}
