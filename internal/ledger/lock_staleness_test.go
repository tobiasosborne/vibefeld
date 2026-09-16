package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestInspect_And_StaleLock_LivePIDRefused verifies that a lock whose recorded
// pid is alive is never considered stale.
func TestInspect_And_StaleLock_LivePIDRefused(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, lockFileName)

	if err := os.WriteFile(lockPath, []byte(`{"agent_id":"live-agent","pid":`+strconv.Itoa(os.Getpid())+`,"acquired_at":"`+time.Now().UTC().Format(time.RFC3339Nano)+`"}`), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	agentID, pid, _, present, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !present || agentID != "live-agent" || pid != os.Getpid() {
		t.Fatalf("Inspect = (%q, %d, present=%v)", agentID, pid, present)
	}

	stale, reason, ok, err := StaleLock(dir, time.Nanosecond)
	if err != nil {
		t.Fatalf("StaleLock: %v", err)
	}
	if !ok {
		t.Fatal("StaleLock reported no lock")
	}
	if stale {
		t.Fatalf("live pid must not be stale, reason=%q", reason)
	}
}

// TestStaleLock_DeadPIDRemoved verifies that a lock whose recorded pid is not
// alive is stale and can be removed.
func TestStaleLock_DeadPIDRemoved(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, lockFileName)

	// Obtain a pid that is definitely no longer running: start and reap a short
	// child process.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait child: %v", err)
	}

	if err := os.WriteFile(lockPath, []byte(`{"agent_id":"dead-agent","pid":`+strconv.Itoa(deadPID)+`,"acquired_at":"`+time.Now().UTC().Format(time.RFC3339Nano)+`"}`), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	stale, _, ok, err := StaleLock(dir, time.Hour)
	if err != nil {
		t.Fatalf("StaleLock: %v", err)
	}
	if !ok || !stale {
		t.Fatalf("dead pid must be stale (ok=%v stale=%v)", ok, stale)
	}

	if err := RemoveLockFile(dir); err != nil {
		t.Fatalf("RemoveLockFile: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed")
	}
}

// TestStaleLock_LegacyPlainTextContent verifies legacy plain-text lock files
// are readable and their age is taken from the file modification time.
func TestStaleLock_LegacyPlainTextContent(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, lockFileName)

	if err := os.WriteFile(lockPath, []byte("legacy-agent\n"), 0600); err != nil {
		t.Fatalf("write legacy lock: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	agentID, pid, _, present, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect legacy: %v", err)
	}
	if !present || agentID != "legacy-agent" || pid != 0 {
		t.Fatalf("Inspect = (%q, %d, present=%v)", agentID, pid, present)
	}

	stale, reason, ok, err := StaleLock(dir, time.Second)
	if err != nil {
		t.Fatalf("StaleLock: %v", err)
	}
	if !ok || !stale {
		t.Fatalf("old legacy lock must be stale (ok=%v stale=%v reason=%q)", ok, stale, reason)
	}
}

// TestStaleLock_NoLock verifies ok=false when no lock file exists.
func TestStaleLock_NoLock(t *testing.T) {
	_, _, ok, err := StaleLock(t.TempDir(), time.Second)
	if err != nil {
		t.Fatalf("StaleLock: %v", err)
	}
	if ok {
		t.Fatal("ok should be false with no lock file")
	}
}

// TestRelease_LegacyPlainTextOwnership verifies Release understands a legacy
// plain-text lock file written by the same agent.
func TestRelease_LegacyPlainTextOwnership(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("agent-A", time.Second); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Simulate a legacy writer replacing the JSON metadata with a bare id.
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte("agent-A"), 0600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release with legacy content: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, lockFileName)); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after release")
	}
}
