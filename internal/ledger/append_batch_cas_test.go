package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestAppendBatchIfSequence_StaleExpectedSeqAppendsNothing verifies that a
// stale expected sequence refuses the whole batch (not just its first event)
// and leaves the ledger untouched.
func TestAppendBatchIfSequence_StaleExpectedSeqAppendsNothing(t *testing.T) {
	dir := t.TempDir()

	// Seed the ledger with one event so expectedSeq 0 is stale.
	if _, err := Append(dir, NewProofInitialized("seed", "agent")); err != nil {
		t.Fatalf("seed Append: %v", err)
	}

	events := []Event{
		NewChallengeResolved("ch1"),
		NewChallengeWithdrawn("ch2"),
	}

	_, err := AppendBatchIfSequence(dir, events, 0) // stale: ledger is at 1
	if !errors.Is(err, ErrSequenceMismatch) {
		t.Fatalf("err = %v, want ErrSequenceMismatch", err)
	}

	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (nothing appended)", count)
	}
}

// TestAppendBatchIfSequence_MatchingExpectedSeqAppendsAll verifies the happy
// path assigns consecutive sequence numbers.
func TestAppendBatchIfSequence_MatchingExpectedSeqAppendsAll(t *testing.T) {
	dir := t.TempDir()

	events := []Event{
		NewProofInitialized("batch", "agent"),
		NewChallengeResolved("ch1"),
		NewChallengeWithdrawn("ch2"),
	}

	seqs, err := AppendBatchIfSequence(dir, events, 0)
	if err != nil {
		t.Fatalf("AppendBatchIfSequence: %v", err)
	}
	want := []int{1, 2, 3}
	if len(seqs) != len(want) {
		t.Fatalf("seqs = %v, want %v", seqs, want)
	}
	for i := range want {
		if seqs[i] != want[i] {
			t.Fatalf("seqs = %v, want %v", seqs, want)
		}
	}
}

// TestAppendBatchIfSequence_PartialRenameLeavesValidPrefix verifies that a
// mid-batch rename failure keeps the already-renamed prefix (rather than
// rolling it back) and that the resulting ledger replays cleanly.
func TestAppendBatchIfSequence_PartialRenameLeavesValidPrefix(t *testing.T) {
	dir := t.TempDir()

	// Block the third event's final path with a non-empty directory: renaming a
	// file over a non-empty directory fails on Unix.
	blocker := EventFilePath(dir, 3)
	if err := os.MkdirAll(blocker, 0755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("x"), 0644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	events := []Event{
		NewProofInitialized("first", "agent"),
		NewChallengeResolved("ch1"),
		NewChallengeWithdrawn("ch2"), // this rename fails
	}

	seqs, err := AppendBatchIfSequence(dir, events, 0)
	if err == nil {
		t.Fatal("expected a rename error")
	}
	if len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("returned seqs = %v, want [1 2]", seqs)
	}

	// The first two events must still be present (valid prefix).
	count, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	// And the prefix must replay without error.
	eventsRead, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll after partial batch: %v", err)
	}
	if len(eventsRead) != 2 {
		t.Fatalf("read %d events, want 2", len(eventsRead))
	}
}

// TestAppendBatchIfSequence_EmptyIsNoOp verifies an empty batch does nothing.
func TestAppendBatchIfSequence_EmptyIsNoOp(t *testing.T) {
	dir := t.TempDir()
	seqs, err := AppendBatchIfSequence(dir, nil, 0)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if seqs != nil {
		t.Fatalf("seqs = %v, want nil", seqs)
	}
}
