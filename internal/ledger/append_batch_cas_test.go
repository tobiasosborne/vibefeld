package ledger

import (
	"errors"
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
