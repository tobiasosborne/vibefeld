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

// TestAcquireLock_WritesToken verifies every acquisition records a unique token
// in the JSON lock file.
func TestAcquireLock_WritesToken(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	l1, l2 := NewLedgerLock(dir1), NewLedgerLock(dir2)
	if err := l1.Acquire("agent", time.Second); err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	defer deferRelease(t, l1)
	if err := l2.Acquire("agent", time.Second); err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	defer deferRelease(t, l2)

	m1, err := readLockFile(filepath.Join(dir1, lockFileName))
	if err != nil {
		t.Fatalf("read lock 1: %v", err)
	}
	m2, err := readLockFile(filepath.Join(dir2, lockFileName))
	if err != nil {
		t.Fatalf("read lock 2: %v", err)
	}
	if m1.Token == "" || m2.Token == "" {
		t.Fatal("token must be recorded in the lock file")
	}
	if m1.Token == m2.Token {
		t.Fatal("two acquisitions must not share a token")
	}
}

// TestRelease_TokenMismatchDoesNotRemove verifies Release refuses to remove a
// lock carrying a different token, even when the agent id matches.
func TestRelease_TokenMismatchDoesNotRemove(t *testing.T) {
	dir := t.TempDir()
	lock := NewLedgerLock(dir)
	if err := lock.Acquire("agent-A", time.Second); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Another holder re-acquires with the same agent id but a new token.
	replacement := `{"agent_id":"agent-A","pid":1,"acquired_at":"2025-01-01T00:00:00Z","token":"deadbeef"}`
	path := filepath.Join(dir, lockFileName)
	if err := os.WriteFile(path, []byte(replacement), 0600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	if err := lock.Release(); err == nil {
		t.Fatal("Release must fail when the on-disk token differs")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the replacement lock must not be removed")
	}
}

// TestRemoveIfStale_LivePIDNotRemoved verifies a live-pid lock is never removed.
func TestRemoveIfStale_LivePIDNotRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockFileName)
	if err := os.WriteFile(path, []byte(`{"agent_id":"live","pid":`+strconv.Itoa(os.Getpid())+`,"acquired_at":"2020-01-01T00:00:00Z","token":"t"}`), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	removed, err := RemoveIfStale(dir, time.Nanosecond)
	if err != nil {
		t.Fatalf("RemoveIfStale: %v", err)
	}
	if removed {
		t.Fatal("live-pid lock must not be removed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("live-pid lock file must remain")
	}
}

// TestRemoveIfStale_DeadPIDRemoved verifies a dead-pid lock is removed.
func TestRemoveIfStale_DeadPIDRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockFileName)

	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait child: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"agent_id":"dead","pid":`+strconv.Itoa(deadPID)+`,"acquired_at":"2020-01-01T00:00:00Z","token":"t"}`), 0600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	removed, err := RemoveIfStale(dir, time.Hour)
	if err != nil {
		t.Fatalf("RemoveIfStale: %v", err)
	}
	if !removed {
		t.Fatal("dead-pid lock should be removed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("dead-pid lock file should be gone")
	}
}

// TestRemoveIfStale_LegacyOldRemoved verifies an old legacy plain-text lock is
// removed and a fresh one is kept.
func TestRemoveIfStale_LegacyOldRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockFileName)
	if err := os.WriteFile(path, []byte("legacy-agent"), 0600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	removed, err := RemoveIfStale(dir, time.Second)
	if err != nil {
		t.Fatalf("RemoveIfStale: %v", err)
	}
	if !removed {
		t.Fatal("old legacy lock should be removed")
	}
}

// TestRemoveIfStale_NoLock verifies a missing lock is a clean no-op.
func TestRemoveIfStale_NoLock(t *testing.T) {
	removed, err := RemoveIfStale(t.TempDir(), time.Second)
	if err != nil {
		t.Fatalf("RemoveIfStale: %v", err)
	}
	if removed {
		t.Fatal("no lock present, removed should be false")
	}
}

// TestSameLockForRemoval_TokenChanged verifies the compare-and-remove identity
// check rejects a changed token and a changed legacy mtime.
func TestSameLockForRemoval_TokenChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockFileName)
	before := []byte(`{"agent_id":"a","acquired_at":"2020-01-01T00:00:00Z","token":"t1"}`)
	after := []byte(`{"agent_id":"a","acquired_at":"2020-01-01T00:00:00Z","token":"t2"}`)
	meta, err := parseLockData(path, before)
	if err != nil {
		t.Fatalf("parse before: %v", err)
	}
	if sameLockForRemoval(path, before, after, meta) {
		t.Fatal("changed token must not compare equal for removal")
	}
	if !sameLockForRemoval(path, before, before, meta) {
		t.Fatal("unchanged token must compare equal for removal")
	}

	legacy := []byte("legacy-agent")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	legacyMeta, err := parseLockData(path, legacy)
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	// Different mtime for otherwise identical legacy content.
	newer := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, newer, newer); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	laterMeta, err := parseLockData(path, legacy)
	if err != nil {
		t.Fatalf("parse legacy later: %v", err)
	}
	if laterMeta.AcquiredAt.Equal(legacyMeta.AcquiredAt) {
		t.Fatal("test precondition: legacy mtime should have changed")
	}
	if sameLockForRemoval(path, legacy, legacy, legacyMeta) {
		t.Fatal("changed legacy mtime must not compare equal for removal")
	}
}
