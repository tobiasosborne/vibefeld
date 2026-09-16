package service

import (
	"fmt"
	"sort"
	"strings"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
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
// Discovery, the per-node ValidationBatchID / validated-transition re-checks,
// and the appended NodeUnvalidated events all run inside one commit closure
// against one state read, and the whole set is appended as one CAS batch. A
// node whose batch id changed (or which was already unvalidated) between an
// earlier reader's observation and this commit is therefore not revoked by a
// stale decision: the state this closure sees is the state the CAS protects.
// Each revocation is the normal, attributed NodeUnvalidated event — never a
// silent state rewrite. Nodes are processed in sorted NodeID order.
//
// If no node currently carries batchID, returns a report with Count==0 and
// ErrUnvalidateBatchNotFound — a clean no-op, not a crash. If a per-node
// re-check fails (e.g. the node was revalidated under a different batch),
// the report still lists that node's individual outcome, the successful
// revocations land as one batch, and the returned error wraps
// ErrConcurrentModification describing how many target nodes failed.
func (s *ProofService) UnvalidateBatch(batchID, reason, revokedBy string) (*UnvalidateBatchReport, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, fmt.Errorf("%w: batch id cannot be empty", ErrEmptyInput)
	}

	report := &UnvalidateBatchReport{BatchID: batchID}
	found := false

	_, commitErr := s.commit(func(st *state.State) ([]ledger.Event, error) {
		report.Items = nil
		report.Count = 0
		found = false

		var targets []*node.Node
		for _, n := range st.AllNodes() {
			if n.EpistemicState == schema.EpistemicValidated && n.ValidationBatchID == batchID {
				targets = append(targets, n)
			}
		}
		sort.Slice(targets, func(i, j int) bool {
			return targets[i].ID.String() < targets[j].ID.String()
		})

		if len(targets) == 0 {
			return nil, nil
		}
		found = true

		events := make([]ledger.Event, 0, len(targets))
		for _, n := range targets {
			item := UnvalidateBatchItemResult{Node: n.ID}
			// Re-check this node's preconditions against the state read
			// that this commit will CAS against. The batch id and the
			// revision under which it was validated are the authority,
			// not whatever a prior reader observed.
			if n.ValidationBatchID != batchID {
				item.Err = fmt.Sprintf("node %s no longer carries batch %s", n.ID.String(), batchID)
			} else if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
				item.Err = fmt.Sprintf("%v: node %s is in %s state, must be %s to unvalidate",
					ErrInvalidState, n.ID.String(), n.EpistemicState, schema.EpistemicValidated)
			} else {
				events = append(events, ledger.NewNodeUnvalidated(n.ID, reason, revokedBy))
				report.Count++
			}
			report.Items = append(report.Items, item)
		}
		return events, nil
	})
	if commitErr != nil {
		// Nothing was appended: the CAS refused the batch. Report the
		// conflict per node rather than a misleading partial success count.
		report.Count = 0
		for i := range report.Items {
			if report.Items[i].Err == "" {
				report.Items[i].Err = commitErr.Error()
			}
		}
		return report, wrapSequenceMismatch(commitErr, "UnvalidateBatch")
	}

	if !found {
		return report, ErrUnvalidateBatchNotFound
	}

	failed := 0
	for _, item := range report.Items {
		if item.Err != "" {
			failed++
		}
	}
	if failed > 0 {
		return report, fmt.Errorf("%w: %d of %d nodes in batch %s failed to unvalidate",
			ErrConcurrentModification, failed, len(report.Items), batchID)
	}
	return report, nil
}
