package service

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// TestCrashHelper is not a real test: the parent test re-execs the test binary
// with -test.run=TestCrashHelper and AF_TEST_LEDGER_DIR / AF_TEST_CRASH_AFTER_RENAME
// set. The ledger append hook then os.Exit(3)s after the requested rename,
// reproducing a process crash mid-batch. It is a no-op in a normal test run.
func TestCrashHelper(t *testing.T) {
	dir := os.Getenv("AF_TEST_LEDGER_DIR")
	if dir == "" {
		t.Skip("helper process only")
	}

	rootID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse root id: %v", err)
	}
	n, err := node.NewNode(rootID, schema.NodeTypeClaim, "crash conjecture", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}

	events := []ledger.Event{
		ledger.NewProofInitialized("crash conjecture", "author"),
		ledger.NewNodeCreated(*n),
		ledger.NewNodesClaimed([]types.NodeID{rootID}, "owner", types.FromTime(time.Now().Add(time.Hour))),
	}

	// On success (no crash hook) this returns; with the hook set it exits 3.
	if _, err := ledger.AppendBatchIfSequence(dir, events, 0); err != nil {
		t.Fatalf("AppendBatchIfSequence: %v", err)
	}
}

// TestAppendBatchIfSequence_CrashAfterRenameDurablePrefix re-execs this test
// binary and makes ledger.AppendBatchIfSequence os.Exit(3) right after the
// requested rename. The surviving ledger must contain exactly that many
// events, be a contiguous prefix starting at sequence 1, and replay cleanly
// through state.Replay.
func TestAppendBatchIfSequence_CrashAfterRenameDurablePrefix(t *testing.T) {
	for n := 1; n <= 3; n++ {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			dir := t.TempDir()

			cmd := exec.Command(os.Args[0], "-test.run=TestCrashHelper", "-test.v")
			cmd.Env = append(os.Environ(),
				"AF_TEST_LEDGER_DIR="+dir,
				"AF_TEST_CRASH_AFTER_RENAME="+strconv.Itoa(n),
			)
			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
				t.Fatalf("helper exit = %v (output %q), want exit code 3", err, out)
			}

			seqs, err := ledgerEventSequences(dir)
			if err != nil {
				t.Fatalf("list event sequences: %v", err)
			}
			if len(seqs) != n {
				t.Fatalf("surviving events = %v, want exactly %d files", seqs, n)
			}
			for i, seq := range seqs {
				if seq != i+1 {
					t.Fatalf("surviving sequences = %v, want contiguous prefix [1..%d]", seqs, n)
				}
			}

			ldg, err := ledger.NewLedger(dir)
			if err != nil {
				t.Fatalf("NewLedger: %v", err)
			}
			st, err := state.Replay(ldg)
			if err != nil {
				t.Fatalf("state.Replay over the %d-event prefix: %v", n, err)
			}
			if got := st.LatestSeq(); got != n {
				t.Fatalf("LatestSeq = %d, want %d", got, n)
			}
		})
	}
}

// TestAppendBatchIfSequence_RenameFailureLeavesReplayablePrefix verifies that a
// mid-batch rename failure keeps the already-renamed prefix (rather than
// rolling it back) and that the surviving prefix replays cleanly through state.
// It lives in package service (not ledger) so it can import state; the ledger
// package cannot, as state imports ledger.
func TestAppendBatchIfSequence_RenameFailureLeavesReplayablePrefix(t *testing.T) {
	dir := t.TempDir()

	// Block the third event's final path with a non-empty directory: renaming a
	// file over a non-empty directory fails on Unix.
	blocker := ledger.EventFilePath(dir, 3)
	if err := os.MkdirAll(blocker, 0755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("x"), 0644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	rootID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse root id: %v", err)
	}
	n, err := node.NewNode(rootID, schema.NodeTypeClaim, "first", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	events := []ledger.Event{
		ledger.NewProofInitialized("first", "agent"),
		ledger.NewNodeCreated(*n),
		ledger.NewNodesClaimed([]types.NodeID{rootID}, "owner", types.FromTime(time.Now().Add(time.Hour))), // this rename fails
	}

	seqs, err := ledger.AppendBatchIfSequence(dir, events, 0)
	if err == nil {
		t.Fatal("expected a rename error")
	}
	if len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("returned seqs = %v, want [1 2]", seqs)
	}

	ldg, err := ledger.NewLedger(dir)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	st, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("state.Replay after partial batch: %v", err)
	}
	if got := st.LatestSeq(); got != 2 {
		t.Fatalf("LatestSeq = %d, want 2", got)
	}
	if st.GetNode(rootID) == nil {
		t.Fatal("root node should be present in the replayed prefix")
	}
}

// ledgerEventSequences returns the sequence numbers present in dir, sorted.
func ledgerEventSequences(dir string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var seqs []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		seq, err := ledger.ParseFilename(e.Name())
		if err != nil {
			continue
		}
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	return seqs, nil
}
