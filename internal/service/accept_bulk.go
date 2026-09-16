package service

import (
	"fmt"
	"sort"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// BulkAcceptItem is the per-node outcome of a bulk accept. Status is
// "applied", "blocked:<code>" or "rejected:<code>". A blocked item is a
// legitimate accept that the batch could not schedule — currently only
// "blocked:prerequisite-pending" (a child or validation dependency that is
// neither already validated nor accepted by this batch) — while a rejected
// item is invalid regardless of scheduling.
type BulkAcceptItem struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// BulkAcceptReport is the full per-item result of a bulk accept, plus the
// aggregate counts the CLI uses to choose its exit code.
type BulkAcceptReport struct {
	Items    []BulkAcceptItem `json:"items"`
	Applied  int              `json:"applied"`
	Blocked  int              `json:"blocked"`
	Rejected int              `json:"rejected"`

	// cause is the first per-item error, kept so the aggregate exit error can
	// still be matched with errors.Is against the underlying sentinel (e.g.
	// ErrBlockingChallenges, ErrClaimTestStale) while the exit code stays the
	// batch outcome's (5 or 6). Unexported and not serialised.
	cause error
}

func (r *BulkAcceptReport) add(nodeID, status, detail string) {
	r.Items = append(r.Items, BulkAcceptItem{Node: nodeID, Status: status, Detail: detail})
	switch {
	case status == "applied":
		r.Applied++
	case len(status) >= 8 && status[:8] == "blocked:":
		r.Blocked++
	default:
		r.Rejected++
	}
}

// addError records an item outcome together with the error that produced it.
func (r *BulkAcceptReport) addError(nodeID, status, detail string, err error) {
	if r.cause == nil && err != nil {
		r.cause = err
	}
	r.add(nodeID, status, detail)
}

// bulkAcceptError carries both the batch-outcome AFError (for the exit code)
// and the first underlying per-item error (for errors.Is / diagnostic text).
type bulkAcceptError struct {
	aggregate *aferrors.AFError
	cause     error
}

func (e *bulkAcceptError) Error() string {
	return e.aggregate.Error() + ": " + e.cause.Error()
}

// Unwrap returns the aggregate first so errors.As/aferrors.Code picks the
// batch exit code (5 or 6), then the underlying per-item error so callers can
// still match sentinels like ErrBlockingChallenges or ErrClaimTestStale.
func (e *bulkAcceptError) Unwrap() []error { return []error{e.aggregate, e.cause} }

// ExitError selects the aggregate AFError for this report. nil means every item
// applied. When at least one item failed, the returned error also wraps the
// first underlying per-item error.
func (r *BulkAcceptReport) ExitError() error {
	total := len(r.Items)
	if total > 0 && r.Applied == total {
		return nil
	}
	var aggregate *aferrors.AFError
	if r.Applied == 0 {
		aggregate = aferrors.Newf(aferrors.VERDICTS_NONE_APPLIED, "0 of %d node(s) accepted", total)
	} else {
		aggregate = aferrors.Newf(aferrors.VERDICTS_PARTIALLY_APPLIED, "%d of %d node(s) accepted", r.Applied, total)
	}
	if r.cause != nil {
		return &bulkAcceptError{aggregate: aggregate, cause: r.cause}
	}
	return aggregate
}

// AcceptNodesBulk validates as many of ids as can be scheduled by their actual
// prerequisites, in one commit. A node is accepted only after every child and
// validation dependency it relies on is already validated/admitted (or
// archived, for a child) or is accepted earlier in this same batch. Among the
// nodes eligible at any step, the smallest hierarchical ID goes first, so the
// ordering is deterministic.
//
// Every event is validated and built against one state read and appended as a
// single batch; a blocked or rejected item never stops the rest. The returned
// report always has exactly one entry per distinct requested node. The returned
// error is nil iff every item applied, otherwise a VERDICTS_PARTIALLY_APPLIED
// (exit 5) or VERDICTS_NONE_APPLIED (exit 6) AFError.
func (s *ProofService) AcceptNodesBulk(ids []types.NodeID, verifiedBy, batchID string) (*BulkAcceptReport, error) {
	return s.acceptNodesBulkWithOptions(ids, AcceptOptions{VerifiedBy: verifiedBy, BatchID: batchID})
}

// AcceptNodesBulkInteractive is the interactive `af accept` bulk path. Like
// AcceptNodeInteractive it runs the reviewer≠contributor check and may allow
// an explicit self-accept (recorded as self_accepted on each event). Verdict
// files keep the same check and never allow self.
func (s *ProofService) AcceptNodesBulkInteractive(ids []types.NodeID, verifiedBy string, allowSelf bool) (*BulkAcceptReport, error) {
	return s.acceptNodesBulkWithOptions(ids, AcceptOptions{
		VerifiedBy: verifiedBy,
		AllowSelf:  allowSelf,
	})
}

// acceptNodesBulkWithOptions is the shared body of the bulk accept forms.
func (s *ProofService) acceptNodesBulkWithOptions(ids []types.NodeID, opts AcceptOptions) (*BulkAcceptReport, error) {
	report := &BulkAcceptReport{}
	if len(ids) == 0 {
		return report, nil
	}

	var oldTaints map[string]node.TaintState
	var acceptedIDs []types.NodeID
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		report.Items = nil
		report.Applied, report.Blocked, report.Rejected = 0, 0, 0
		report.cause = nil
		acceptedIDs = nil
		events, err := scheduleBulkAccept(st, ids, opts, report, &acceptedIDs)
		if err != nil {
			return nil, err
		}
		oldTaints = snapshotTaintStates(st)
		return events, nil
	})
	if err != nil {
		// The report describes a schedule that was not committed; report the
		// commit failure alone rather than a misleading outcome list.
		report.Items = nil
		report.Applied, report.Blocked, report.Rejected = 0, 0, 0
		report.cause = nil
		return report, wrapSequenceMismatch(err, "AcceptNodeBulk")
	}

	// Emit taint events for all accepted nodes. Reuse one pre-transition
	// snapshot so overlapping ancestor changes are emitted only once. Taint
	// emission is best-effort and never fails the accept (see AcceptNodeWithNote).
	for _, id := range acceptedIDs {
		_ = s.emitTaintRecomputedEvents(id, oldTaints)
	}
	return report, report.ExitError()
}

func scheduleBulkAccept(st *state.State, ids []types.NodeID, base AcceptOptions, report *BulkAcceptReport, acceptedIDs *[]types.NodeID) ([]ledger.Event, error) {
	type candidate struct {
		id types.NodeID
		n  *node.Node
	}

	// Reject missing nodes up front; they can never be scheduled. Deduplicate
	// so the report has exactly one entry per requested node.
	seen := make(map[string]bool, len(ids))
	items := make([]candidate, 0, len(ids))
	for _, id := range ids {
		if seen[id.String()] {
			continue
		}
		seen[id.String()] = true
		n := st.GetNode(id)
		if n == nil {
			report.addError(id.String(), "rejected:"+CodeNodeNotFound, fmt.Sprintf("node %s does not exist", id.String()), fmt.Errorf("%w: %s", ErrNodeNotFound, id.String()))
			continue
		}
		items = append(items, candidate{id: id, n: n})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id.Less(items[j].id) })

	accepted := make(map[string]bool)
	var events []ledger.Event
	undecided := items
	for len(undecided) > 0 {
		var still []candidate
		progressed := false
		for _, it := range undecided {
			opts := base
			opts.AcceptedInBatch = accepted
			err := checkAcceptEligibility(st, it.n, opts)
			if err == nil {
				ev := ledger.NewNodeValidatedWithHash(it.id, "", base.VerifiedBy, base.BatchID, it.n.ContentHash, false)
				if base.AllowSelf && contributorRole(st, it.n, base.VerifiedBy) != "" {
					ev.SelfAccepted = true
				}
				setFencedClaimRelease(it.n, base.VerifiedBy, &ev.ReleaseClaim, &ev.ClaimSeq)
				events = append(events, ev)
				accepted[it.id.String()] = true
				*acceptedIDs = append(*acceptedIDs, it.id)
				report.add(it.id.String(), "applied", "")
				progressed = true
				continue
			}
			if blocked, ok := asAcceptBlocked(err); ok && blocked.IsPrerequisitePending() {
				still = append(still, it)
				continue
			}
			if blocked, ok := asAcceptBlocked(err); ok {
				report.addError(it.id.String(), "blocked:"+blocked.Code, err.Error(), err)
			} else if rejected, ok := asAcceptRejected(err); ok {
				report.addError(it.id.String(), "rejected:"+rejected.Code, err.Error(), err)
			} else {
				report.addError(it.id.String(), "rejected:apply-error", err.Error(), err)
			}
			progressed = true
		}
		if !progressed {
			// No remaining node can be unlocked: a prerequisite is outside the
			// batch, is itself blocked, or the batch is cyclic.
			for _, it := range still {
				report.add(it.id.String(), "blocked:prerequisite-pending",
					"a prerequisite (child or validation dependency) is pending and was not accepted in this batch")
			}
			break
		}
		undecided = still
	}
	return events, nil
}
