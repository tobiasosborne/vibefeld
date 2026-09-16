package service

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ledgerDir returns the ledger directory for this proof.
func (s *ProofService) ledgerDir() string {
	return filepath.Join(s.path, "ledger")
}

// commit is the single mutating primitive for this package. It implements the
// optimistic commit protocol:
//
//  1. Load state once, outside the ledger lock.
//  2. Run build against that one read (all validation and event construction
//     happen here, so preconditions and the appended events share one view).
//  3. Take the ledger lock and append the whole batch only if the ledger tail
//     is still st.LatestSeq(); otherwise fail with ErrConcurrentModification.
//
// Replay never runs under the lock. A conflict leaves the ledger untouched and
// the caller may retry with a fresh read (see commitRetry). A crash mid-batch
// leaves a valid prefix; callers must therefore keep each semantic unit to one
// event where possible.
//
// An empty event slice is a no-op (some callers conditionally append nothing);
// it performs no state read and takes no lock.
func (s *ProofService) commit(build func(st *state.State) ([]ledger.Event, error)) ([]int, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	events, err := build(st)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	// Test-only hook: runs in the exact window between the state read used
	// for build/validation and the CAS append, so tests can inject a
	// concurrent writer.
	if s.beforeAppend != nil {
		s.beforeAppend()
	}

	seqs, err := ledger.AppendBatchIfSequence(s.ledgerDir(), events, st.LatestSeq())
	if err != nil {
		return nil, wrapSequenceMismatch(err, "commit")
	}
	return seqs, nil
}

// commitRetry runs commit, retrying on ErrConcurrentModification up to n extra
// times (n=0 means a single attempt). Each attempt reloads state, so a mutation
// that arrives between attempts is observed. Non-conflict errors are returned
// immediately.
func (s *ProofService) commitRetry(n int, build func(st *state.State) ([]ledger.Event, error)) ([]int, error) {
	var lastErr error
	for attempt := 0; attempt <= n; attempt++ {
		seqs, err := s.commit(build)
		if err == nil {
			return seqs, nil
		}
		if !errors.Is(err, ErrConcurrentModification) {
			return nil, err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: commit retries exhausted", ErrConcurrentModification)
	}
	return nil, lastErr
}

// commitThenTaint commits the events built against one state read and then
// appends taint-audit events for nodeID, computed from the pre-transition
// snapshot taken inside the same build. Taint emission is deliberately a
// separate commit (see emitTaintRecomputedEvents): the epistemic transition is
// the authoritative record and replay re-derives taint if emission fails.
func (s *ProofService) commitThenTaint(nodeID types.NodeID, build func(st *state.State) ([]ledger.Event, error)) error {
	var oldTaints map[string]node.TaintState
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		events, err := build(st)
		if err != nil {
			return nil, err
		}
		oldTaints = snapshotTaintStates(st)
		return events, nil
	})
	if err != nil {
		return err
	}
	return s.emitTaintRecomputedEvents(nodeID, oldTaints)
}
