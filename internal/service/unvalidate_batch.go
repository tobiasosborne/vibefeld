package service

import (
	"fmt"
	"sort"
	"strings"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// ErrUnvalidateBatchNotFound is returned by UnvalidateBatch when no node
// currently carries the given batch id. This is a clean no-op, not a
// failure: the batch may already have been unvalidated, may never have
// existed, or may have had every one of its nodes individually unvalidated
// since. Exit code 7 (see internal/errors), distinct from both success (0)
// and from a genuine partial-failure error.
var ErrUnvalidateBatchNotFound = aferrors.New(aferrors.UNVALIDATE_BATCH_NOT_FOUND, "no node is currently validated under this batch id")

// UnvalidateBatchItemResult is the per-node outcome of one UnvalidateBatch
// call. Err is empty on success.
type UnvalidateBatchItemResult struct {
	Node types.NodeID `json:"node"`
	Err  string       `json:"error,omitempty"`
}

// UnvalidateBatchReport is the full per-node report from UnvalidateBatch.
type UnvalidateBatchReport struct {
	BatchID string                      `json:"batch_id"`
	Items   []UnvalidateBatchItemResult `json:"items"`
	Count   int                         `json:"count"`
}

// UnvalidateBatch bulk-revokes validation on every node whose current
// ValidationBatchID equals batchID (rk PRD C3 / IMPLEMENTATION_PLAN.md item
// V3: the bulk-revocation lever for batched verification). Finding affected
// nodes is a state scan (Node.ValidationBatchID, set by V1), not a ledger
// rescan — the state is already derived by LoadState.
//
// Each revocation is applied as UnvalidateNode's normal, attributed
// NodeUnvalidated event — never a silent state rewrite. Nodes are processed
// in sorted NodeID order for determinism.
//
// If no node currently carries batchID, returns a report with Count==0 and
// ErrUnvalidateBatchNotFound — a clean no-op, not a crash. If revocation
// fails partway through (e.g. a genuine concurrent modification), the
// report still lists every node's individual outcome, and the returned
// error wraps ErrConcurrentModification describing how many of the target
// nodes failed.
func (s *ProofService) UnvalidateBatch(batchID, reason, revokedBy string) (*UnvalidateBatchReport, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, fmt.Errorf("%w: batch id cannot be empty", ErrEmptyInput)
	}

	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	var targets []types.NodeID
	for _, n := range st.AllNodes() {
		if n.EpistemicState == schema.EpistemicValidated && n.ValidationBatchID == batchID {
			targets = append(targets, n.ID)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].String() < targets[j].String()
	})

	report := &UnvalidateBatchReport{BatchID: batchID}
	if len(targets) == 0 {
		return report, ErrUnvalidateBatchNotFound
	}

	failed := 0
	for _, id := range targets {
		item := UnvalidateBatchItemResult{Node: id}
		if err := s.UnvalidateNode(id, reason, revokedBy); err != nil {
			item.Err = err.Error()
			failed++
		} else {
			report.Count++
		}
		report.Items = append(report.Items, item)
	}

	if failed > 0 {
		return report, fmt.Errorf("%w: %d of %d nodes in batch %s failed to unvalidate",
			ErrConcurrentModification, failed, len(targets), batchID)
	}
	return report, nil
}
