package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Stable acceptance-eligibility codes. They are the machine contract the
// verdict-file and bulk-accept reports are built from; message text may change,
// these may not.
const (
	CodeChildrenNotValidated       = "children-not-validated"
	CodeValidationDepsNotValidated = "validation-deps-not-validated"
	CodeClaimTestRequired          = "claim-test-required"
	CodeBlockingChallenge          = "blocking-challenge"
	CodeNeedsRefinementNoChildren  = "needs-refinement-no-children"
	CodeContentHashMismatch        = "content-hash-mismatch"
	CodeNotVerifierReady           = "not-verifier-ready"
	CodeReviewerIsAuthor           = "reviewer-equals-author"
	CodeNodeNotFound               = "node-not-found"
	CodeInvalidTransition          = "invalid-transition"
)

// AcceptBlockedError is an accept that is legitimate but not yet eligible: a
// prerequisite (a child or validation dependency) is pending, a crux node has
// no current passing claim-test, a blocking challenge is open, or a
// needs_refinement node has no children yet. Bulk accept defers these by
// prerequisite; a verdict file reports them as blocked-by:<Code>.
type AcceptBlockedError struct {
	Node   types.NodeID
	Code   string
	Detail string
	Err    error
}

func (e *AcceptBlockedError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Err.Error()
}

func (e *AcceptBlockedError) Unwrap() error { return e.Err }

// IsPrerequisitePending reports whether the block is a pending child or
// validation dependency — the only kind bulk accept can resolve by accepting
// another item first.
func (e *AcceptBlockedError) IsPrerequisitePending() bool {
	return e.Code == CodeChildrenNotValidated || e.Code == CodeValidationDepsNotValidated
}

// AcceptRejectedError is an accept that is invalid regardless of proof state:
// a failed hash or workflow-readiness check, a reviewer equal to the recorded
// author, an illegal epistemic transition, or a missing node. It is never
// resolved by reordering a batch.
type AcceptRejectedError struct {
	Node   types.NodeID
	Code   string
	Detail string
	Err    error
}

func (e *AcceptRejectedError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Err.Error()
}

func (e *AcceptRejectedError) Unwrap() error { return e.Err }

// AcceptOptions configures checkAcceptEligibility. The zero value is the
// single `af accept` path (no expected hash, no reviewer check).
type AcceptOptions struct {
	// Note is recorded on the resulting NodeValidated event (not used by the
	// eligibility checks themselves; carried here so one options struct flows
	// through the shared validator and event builder).
	Note string
	// VerifiedBy is the verifier identity recorded on the event (driver-
	// supplied provenance).
	VerifiedBy string
	// BatchID is recorded on the event as the originating batch.
	BatchID string
	// ExpectHash, when non-empty, is compared against the node's current content
	// hash; a mismatch is rejected.
	ExpectHash string
	// RequireVerifierReady additionally requires the node's workflow state to be
	// available when an expected hash was supplied. Verdict files set this; the
	// interactive and bulk accept paths do not.
	RequireVerifierReady bool
	// AllowSelf lets a verifier who equals a recorded contributor accept
	// anyway, recording SelfAccepted=true on the resulting NodeValidated so the
	// choice is auditable provenance. It is the only bypass of the
	// reviewer-contributor check.
	AllowSelf bool
	// AcceptedInBatch names nodes this same bulk batch has already scheduled for
	// acceptance (or already appended), so prerequisite checks treat them as
	// validated. Verdict files leave it nil and keep file order.
	AcceptedInBatch map[string]bool
}

// clearedForAccept reports whether a prerequisite target no longer blocks an
// accept: it is validated, admitted, or (for children) archived in state, or it
// was accepted earlier in this same bulk batch.
func clearedForAccept(id types.NodeID, st *state.State, opts AcceptOptions) bool {
	if opts.AcceptedInBatch[id.String()] {
		return true
	}
	n := st.GetNode(id)
	if n == nil {
		return false
	}
	switch n.EpistemicState {
	case schema.EpistemicValidated, schema.EpistemicAdmitted:
		return true
	default:
		return false
	}
}

// checkAcceptEligibility is the single acceptance precondition validator,
// shared by buildAcceptEvents (interactive accept and verdict files) and the
// bulk scheduler. It runs against one caller-supplied state read; the caller
// is responsible for commit's CAS. It distinguishes an AcceptBlockedError (a
// legitimate accept that is not yet eligible) from an AcceptRejectedError (an
// accept that is invalid) so callers can defer, report or abort precisely.
func checkAcceptEligibility(st *state.State, n *node.Node, opts AcceptOptions) error {
	if n == nil {
		return &AcceptRejectedError{Code: CodeNodeNotFound, Err: fmt.Errorf("%w: node not found", ErrNodeNotFound)}
	}

	// Expected-hash and verdict-readiness checks, under the same read.
	if opts.ExpectHash != "" {
		if n.ContentHash != opts.ExpectHash {
			return &AcceptRejectedError{
				Node: n.ID, Code: CodeContentHashMismatch,
				Err: fmt.Errorf("%w: node %s content hash changed since the expectation was recorded (expected %s, current %s)",
					ErrInvalidState, n.ID.String(), opts.ExpectHash, n.ContentHash),
			}
		}
		if opts.RequireVerifierReady && n.WorkflowState != schema.WorkflowAvailable {
			return &AcceptRejectedError{
				Node: n.ID, Code: CodeNotVerifierReady,
				Err: fmt.Errorf("%w: node %s is no longer verifier-ready: workflow_state is %q, not %q",
					errVerdictNotReady, n.ID.String(), n.WorkflowState, schema.WorkflowAvailable),
			}
		}
	}

	// Reviewer ≠ contributor is enforced whenever a verifier identity is
	// recorded, on every accept path (interactive, bulk and verdict files);
	// AllowSelf is the only bypass and records self_accepted. A "contributor"
	// is any identity recorded as the node's author, its proof author
	// (record-proof), or an owner of a statement/edge amendment. This is
	// recorded provenance that can be mechanically checked, not proof of
	// independence: the identity strings are driver-supplied.
	if opts.VerifiedBy != "" && !opts.AllowSelf {
		if role := contributorRole(st, n, opts.VerifiedBy); role != "" {
			return &AcceptRejectedError{
				Node: n.ID, Code: CodeReviewerIsAuthor,
				Err: fmt.Errorf("%w: verifier %q is also the recorded %s of node %s; pass --allow-self to accept anyway",
					errVerdictReviewerIsAuthor, opts.VerifiedBy, role, n.ID.String()),
			}
		}
	}

	// Blocking challenges (critical/major severity).
	if blocking := st.GetBlockingChallengesForNode(n.ID); len(blocking) > 0 {
		return &AcceptBlockedError{
			Node: n.ID, Code: CodeBlockingChallenge,
			Err: formatBlockingChallengesError(n.ID, blocking),
		}
	}

	// A crux node needs a passing claim-test valid for its current content. A
	// legacy test (no recorded hash) still counts.
	if n.Crux && !st.HasPassingClaimTestForContent(n.ID, n.ContentHash) {
		if st.HasStalePassingClaimTest(n.ID, n.ContentHash) {
			return &AcceptBlockedError{
				Node: n.ID, Code: CodeClaimTestRequired,
				Err: fmt.Errorf("%w: %w: node %s (only passing claim-test is stale: it was run against an older content hash; re-run 'af claim-test')", ErrClaimTestRequired, ErrClaimTestStale, n.ID.String()),
			}
		}
		return &AcceptBlockedError{
			Node: n.ID, Code: CodeClaimTestRequired,
			Err: fmt.Errorf("%w: node %s", ErrClaimTestRequired, n.ID.String()),
		}
	}

	// Validation dependencies must be validated/admitted (or accepted earlier
	// in this batch) before this node can be accepted.
	var pendingDeps []string
	for _, depID := range n.ValidationDeps {
		if !clearedForAccept(depID, st, opts) {
			pendingDeps = append(pendingDeps, depID.String())
		}
	}
	if len(pendingDeps) > 0 {
		return &AcceptBlockedError{
			Node: n.ID, Code: CodeValidationDepsNotValidated,
			Err: fmt.Errorf("cannot accept node %s: validation dependencies not yet validated: %s",
				n.ID.String(), strings.Join(pendingDeps, ", ")),
		}
	}

	// Every direct child must reach a terminal-cleared verdict: validated,
	// admitted, or archived. Refuted children are deliberately excluded.
	var children []*node.Node
	var pendingChildren []string
	for _, child := range st.AllNodes() {
		parentID, hasParent := child.ID.Parent()
		if !hasParent || parentID.String() != n.ID.String() {
			continue
		}
		children = append(children, child)
		if child.EpistemicState == schema.EpistemicValidated ||
			child.EpistemicState == schema.EpistemicAdmitted ||
			child.EpistemicState == schema.EpistemicArchived {
			continue
		}
		if opts.AcceptedInBatch[child.ID.String()] {
			continue
		}
		pendingChildren = append(pendingChildren, child.ID.String())
	}
	if len(pendingChildren) > 0 {
		return &AcceptBlockedError{
			Node: n.ID, Code: CodeChildrenNotValidated,
			Err: fmt.Errorf("cannot accept node %s: children not yet validated: %s",
				n.ID.String(), strings.Join(pendingChildren, ", ")),
		}
	}

	// needs_refinement may only be re-accepted after refinement actually
	// happened (the node gained children).
	if n.EpistemicState == schema.EpistemicNeedsRefinement && len(children) == 0 {
		return &AcceptBlockedError{
			Node: n.ID, Code: CodeNeedsRefinementNoChildren,
			Err: fmt.Errorf("cannot accept node %s: node is in needs_refinement state but has no children; use 'af refine' to add child nodes first", n.ID.String()),
		}
	}

	// The epistemic transition itself (pending/needs_refinement -> validated).
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
		return &AcceptRejectedError{Node: n.ID, Code: CodeInvalidTransition, Err: err}
	}
	return nil
}

// contributorRole returns the role under which id is recorded as a
// contributor to n ("author", "proof author" or "amender"), or "" if the
// identity is not a recorded contributor. Amendment history covers statement
// and dependency-edge amendments alike.
func contributorRole(st *state.State, n *node.Node, id string) string {
	if id == "" || n == nil {
		return ""
	}
	if n.Author != "" && n.Author == id {
		return "author"
	}
	if n.ProofAuthor != "" && n.ProofAuthor == id {
		return "proof author"
	}
	if st != nil {
		for _, a := range st.GetAmendmentHistory(n.ID) {
			if a.Owner != "" && a.Owner == id {
				return "amender"
			}
		}
	}
	return ""
}

// asAcceptBlocked / asAcceptRejected are small helpers for callers that switch
// on the two typed outcomes.
func asAcceptBlocked(err error) (*AcceptBlockedError, bool) {
	var e *AcceptBlockedError
	ok := errors.As(err, &e)
	return e, ok
}

func asAcceptRejected(err error) (*AcceptRejectedError, bool) {
	var e *AcceptRejectedError
	ok := errors.As(err, &e)
	return e, ok
}
