package service

import (
	"crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"strings"

	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/types"
	"github.com/tobias/vibefeld/internal/verdicts"
)

// ParseVerdictFile parses and validates raw verdict-file bytes (rk PRD C3 /
// af verdicts apply, item V2 — see internal/verdicts for the file schema).
// Any failure here means "file-invalid": the caller must not attempt to
// apply anything from a file ParseVerdictFile rejects. The returned error is
// an AFError carrying VERDICTS_FILE_INVALID so main.go's exit-code
// machinery (service.ExitCode) picks exit code 3 automatically, the same
// tier as af's other invalid-input errors.
func ParseVerdictFile(data []byte) (*verdicts.File, error) {
	f, err := verdicts.ParseFile(data)
	if err != nil {
		return nil, aferrors.Newf(aferrors.VERDICTS_FILE_INVALID, "%v", err)
	}
	return f, nil
}

// NewVerdictsFileReadError wraps a filesystem error encountered while
// reading a verdict file (missing file, permission denied, etc.) as
// VERDICTS_FILE_INVALID — from the CLI's point of view, a verdict file that
// cannot be read is exactly as "file-invalid" as one that fails to parse,
// and both must produce the same exit code (3) without attempting to apply
// anything.
func NewVerdictsFileReadError(path string, readErr error) error {
	return aferrors.Newf(aferrors.VERDICTS_FILE_INVALID, "cannot read verdict file %s: %v", path, readErr)
}

// VerdictItemResult is the per-item outcome of applying one verdict from a
// batch file. Status is one of the three outcome strings PRD C3 mandates:
//
//	"applied"           — the accept or challenge was recorded as a normal
//	                      per-node ledger event.
//	"blocked-by:<reason>" — the item is a legitimate future acceptance, just
//	                      not yet: children/validation-deps not cleared, a
//	                      blocking challenge exists (possibly raised earlier
//	                      in this same batch), or the batch aborted before
//	                      this item was attempted.
//	"rejected:<reason>"   — the item itself is invalid regardless of proof
//	                      state: node not found, or (accept only) the
//	                      verifier is the node's recorded author.
type VerdictItemResult struct {
	Node    string `json:"node"`
	Verdict string `json:"verdict"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

// VerdictReport is ApplyVerdicts' full per-item report, plus the aggregate
// counts the caller uses to choose between all-applied / partially-applied /
// nothing-applied. Items always has exactly one entry per item in the
// source file, in file order, even when the batch aborts partway through —
// unattempted trailing items are recorded as "blocked-by:batch-aborted"
// rather than silently omitted, so the ledger's state is never ambiguous
// about what was and wasn't tried.
type VerdictReport struct {
	BatchID     string              `json:"batch_id"`
	VerifiedBy  string              `json:"verified_by"`
	Items       []VerdictItemResult `json:"items"`
	Applied     int                 `json:"applied"`
	Blocked     int                 `json:"blocked"`
	Rejected    int                 `json:"rejected"`
	Aborted     bool                `json:"aborted,omitempty"`
	AbortReason string              `json:"abort_reason,omitempty"`
}

func (r *VerdictReport) record(item verdicts.Item, status, detail string) {
	r.Items = append(r.Items, VerdictItemResult{
		Node:    item.Node,
		Verdict: string(item.Verdict),
		Status:  status,
		Detail:  detail,
	})
	switch {
	case status == "applied":
		r.Applied++
	case strings.HasPrefix(status, "blocked-by:"):
		r.Blocked++
	default:
		r.Rejected++
	}
}

// exitError picks the AFError that reports this report's aggregate outcome.
// A nil return means every item applied (exit 0, the caller's normal
// success path).
func (r *VerdictReport) exitError() error {
	total := len(r.Items)
	if total > 0 && r.Applied == total {
		return nil
	}
	if r.Applied == 0 {
		return aferrors.Newf(aferrors.VERDICTS_NONE_APPLIED, "0 of %d verdict(s) applied", total)
	}
	return aferrors.Newf(aferrors.VERDICTS_PARTIALLY_APPLIED, "%d of %d verdict(s) applied", r.Applied, total)
}

// ApplyVerdicts applies every item in f, in file order — order-dependence
// (children before parent for accepts) is a property of that order, which
// this method never changes. It applies what it can: a blocked or rejected
// item does not stop the batch. The single exception is a concurrent-
// modification race (some other process wrote to the ledger mid-batch):
// that aborts every remaining item, because the batch's ordering guarantee
// can no longer be trusted past that point. Every applied item is recorded
// as a normal per-node NodeValidated or ChallengeRaised event, attributed to
// f.VerifiedBy and f.BatchID — never a wholesale subtree accept.
//
// The returned error is nil iff every item applied. Otherwise it is an
// AFError with code VERDICTS_PARTIALLY_APPLIED (exit 5) or
// VERDICTS_NONE_APPLIED (exit 6). The report is always populated regardless
// of the returned error.
func (s *ProofService) ApplyVerdicts(f *verdicts.File) (*VerdictReport, error) {
	report := &VerdictReport{BatchID: f.BatchID, VerifiedBy: f.VerifiedBy}
	aborted := false

	for _, item := range f.Items {
		if aborted {
			report.record(item, "blocked-by:batch-aborted",
				fmt.Sprintf("batch aborted before this item was attempted: %s", report.AbortReason))
			continue
		}

		nodeID, parseErr := types.Parse(item.Node)
		if parseErr != nil {
			// Defensive: ParseFile already validated every node id.
			report.record(item, "rejected:invalid-node-id", parseErr.Error())
			continue
		}

		var status, detail string
		var abortErr error
		if item.Verdict == verdicts.VerdictAccept {
			status, detail, abortErr = s.applyAcceptVerdict(nodeID, item, f)
		} else {
			status, detail, abortErr = s.applyChallengeVerdict(nodeID, item, f)
		}
		report.record(item, status, detail)

		if abortErr != nil {
			aborted = true
			report.Aborted = true
			report.AbortReason = abortErr.Error()
		}
	}

	return report, report.exitError()
}

// applyAcceptVerdict handles a single accept item. abortErr is non-nil only
// for a concurrent-modification race; all other failure modes are reported
// via status/detail and do not stop the batch.
func (s *ProofService) applyAcceptVerdict(nodeID types.NodeID, item verdicts.Item, f *verdicts.File) (status, detail string, abortErr error) {
	st, err := s.LoadState()
	if err != nil {
		return "rejected:state-load-failed", err.Error(), nil
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return "rejected:node-not-found", fmt.Sprintf("node %s does not exist", item.Node), nil
	}

	// Reviewer != author, honestly stated: recorded-and-checkable provenance
	// (PRD C3), not adversary-proof enforcement. Both identities are
	// driver-supplied; if Author was never recorded (legacy node, or none
	// supplied), this check simply cannot fire — see node.Node.Author.
	if n.Author != "" && n.Author == f.VerifiedBy {
		return "rejected:reviewer-equals-author",
			fmt.Sprintf("verifier %q is also the recorded author of node %s", f.VerifiedBy, item.Node), nil
	}

	err = s.AcceptNodeWithVerifier(nodeID, item.Reason, f.VerifiedBy, f.BatchID)
	if err == nil {
		return "applied", "", nil
	}

	if stderrors.Is(err, ErrConcurrentModification) {
		return "rejected:concurrent-modification", err.Error(), err
	}
	// NOTE: ErrClaimTestRequired and ErrBlockingChallenges both carry the
	// same underlying aferrors.NODE_BLOCKED code (see internal/errors), and
	// AFError.Is compares codes only — errors.Is cannot distinguish them.
	// The claim-test and "children/deps not yet validated" cases are
	// therefore classified by their (unique, stable) message substrings
	// FIRST; errors.Is(ErrBlockingChallenges) is the fallback once those
	// more specific cases are ruled out. Same technique cmd/af/accept.go
	// already uses for claim-test vs. blocking-challenges.
	switch {
	case stderrors.Is(err, ErrNodeNotFound):
		return "rejected:node-not-found", err.Error(), nil
	case strings.Contains(err.Error(), "claim-test"):
		return "blocked-by:claim-test-required", err.Error(), nil
	case strings.Contains(err.Error(), "children not yet validated"):
		return "blocked-by:children-not-validated", err.Error(), nil
	case strings.Contains(err.Error(), "validation dependencies not yet validated"):
		return "blocked-by:validation-deps-not-validated", err.Error(), nil
	case strings.Contains(err.Error(), "needs_refinement"):
		return "blocked-by:needs-refinement-no-children", err.Error(), nil
	case stderrors.Is(err, ErrBlockingChallenges):
		return "blocked-by:blocking-challenge", err.Error(), nil
	default:
		return "rejected:apply-error", err.Error(), nil
	}
}

// applyChallengeVerdict handles a single challenge item. There is no
// reviewer≠author check here — PRD C3 scopes that rule to accepts only.
func (s *ProofService) applyChallengeVerdict(nodeID types.NodeID, item verdicts.Item, f *verdicts.File) (status, detail string, abortErr error) {
	st, err := s.LoadState()
	if err != nil {
		return "rejected:state-load-failed", err.Error(), nil
	}
	if st.GetNode(nodeID) == nil {
		return "rejected:node-not-found", fmt.Sprintf("node %s does not exist", item.Node), nil
	}

	challengeID := generateVerdictChallengeID()
	err = s.RaiseChallengeWithBatch(nodeID, challengeID, item.Target, item.Reason, item.Severity, f.VerifiedBy, item.Category, f.BatchID)
	if err == nil {
		return "applied", "challenge " + challengeID, nil
	}
	if stderrors.Is(err, ErrConcurrentModification) {
		return "rejected:concurrent-modification", err.Error(), err
	}
	if stderrors.Is(err, ErrNodeNotFound) {
		return "rejected:node-not-found", err.Error(), nil
	}
	return "rejected:apply-error", err.Error(), nil
}

// RaiseChallengeWithBatch raises a challenge against nodeID with an optional
// batch id, CAS-protected like af's other mutating service methods (unlike
// the interactive `af challenge` command, which appends directly without a
// compare-and-swap). This is the kernel surface rk's C3 batch verification
// mode uses so every challenge in a mixed verdict list shares the batch's
// id, exactly like AcceptNodeWithVerifier/AcceptNodeBulkWithVerifier do for
// accepts (see NewChallengeRaisedWithBatch and those methods' doc comments).
//
// Returns ErrNodeNotFound if nodeID doesn't exist. Returns
// ErrConcurrentModification if the proof was modified by another process
// since state was loaded.
func (s *ProofService) RaiseChallengeWithBatch(nodeID types.NodeID, challengeID, target, reason, severity, raisedBy, category, batchID string) error {
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	if st.GetNode(nodeID) == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
	}

	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewChallengeRaisedWithBatch(challengeID, nodeID, target, reason, severity, raisedBy, category, batchID)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "RaiseChallengeWithBatch")
}

// generateVerdictChallengeID generates a unique challenge id for challenges
// raised via ApplyVerdicts. Same "ch-" + random-hex shape as the interactive
// `af challenge` command's generator (cmd/af/challenge.go), kept as a
// separate copy here so internal/service does not depend on cmd/af.
func generateVerdictChallengeID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ch-%v", Now())
	}
	return "ch-" + hex.EncodeToString(b)
}
