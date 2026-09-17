package support

import (
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Stable cause codes reported by SupportStatus.Cause. They are the machine
// contract `af status`, `af health`, `af get` and `af export` surface; message
// text may change, these may not.
const (
	CauseNotValidated              = "NOT_VALIDATED"
	CauseOpenBlockingChallenge     = "OPEN_BLOCKING_CHALLENGE"
	CauseTargetNotCurrent          = "TARGET_NOT_CURRENT"
	CauseTargetPending             = "TARGET_PENDING"
	CauseTargetRefuted             = "TARGET_REFUTED"
	CauseTargetRevised             = "TARGET_REVISED"
	CauseSelfRevised               = "SELF_REVISED"
	CauseChildArchivedAfterVerdict = "CHILD_ARCHIVED_AFTER_VERDICT"
	CauseCycle                     = "CYCLE"
)

// SupportStatus is whether a node's recorded verdict is currently supported by
// the proof under it, and if not, why and which node is responsible. Cause is
// empty and Node is the zero value when Current is true. Seq is the ledger
// sequence of the event that broke support when one is known (a revision
// sequence for the *_REVISED causes; 0 otherwise).
//
// LatestRevisionSeq is fold plumbing, not part of the reported status: it is
// the latest content-revision ledger sequence at or below this node (its own
// revisions plus every result-use target's carried value). It is carried so an
// ancestor's verdict can be compared against a descendant revision even when
// the intermediate target has since been re-accepted and is current again. It
// is excluded from JSON so the status wire shape is unchanged.
type SupportStatus struct {
	Current           bool         `json:"current"`
	Cause             string       `json:"cause,omitempty"`
	Node              types.NodeID `json:"node,omitempty"`
	Seq               int          `json:"seq,omitempty"`
	LatestRevisionSeq int          `json:"-"`
}

// Current computes support_current for every node in st, memoised over the
// result-use graph with the one shared Walk (v3.1 amendment 4). The graph and
// its SCC condensation are prepared once; only the fold varies. The definition
// is recursive and revision-aware:
//
//   - n is validated or admitted;
//   - n has no open blocking challenge;
//   - n itself has no content revision (statement or dependency amendment)
//     after its recorded verdict sequence;
//   - every result-use target t is validated, admitted, or — for children only
//     — archived, AND support_current(t); a refuted, pending, draft or
//     needs_refinement target fails it, and a missing target fails it;
//   - no target, nor any descendant reachable through a target, has a content
//     revision later than n's verdict sequence.
//
// A legacy result-use cycle is reported as CYCLE rather than erroring:
// validation remains a recorded verdict, it just cannot be current.
func Current(st State) map[string]SupportStatus {
	if st == nil {
		return map[string]SupportStatus{}
	}

	g := Prepare(ResultUseEdges(st, nil))
	return Walk(g, func(n *node.Node, targets []Folded[SupportStatus]) SupportStatus {
		return currentFor(st, n, targets, g.children[n.ID.String()])
	})
}

func currentFor(st State, n *node.Node, targets []Folded[SupportStatus], children []*node.Node) SupportStatus {
	// Carry the latest content revision at or below this node: its own
	// revisions plus every result-use target's carried value. This is what lets
	// an older ancestor verdict see a descendant revision through a target that
	// has since been re-accepted (the R -> B -> C regression).
	latest := 0
	if seq, ok := st.LatestAmendmentSeq(n.ID); ok {
		latest = seq
	}
	for _, t := range targets {
		if t.Cycle || t.Missing {
			continue
		}
		if t.Value.LatestRevisionSeq > latest {
			latest = t.Value.LatestRevisionSeq
		}
	}
	// A direct child archived after this node's verdict is a revision to the
	// decomposition just like an amendment: carry its archival sequence so an
	// ancestor's older verdict can see it through this node even if the child's
	// severed edge would otherwise hide it.
	for _, c := range children {
		if c != nil && c.EpistemicState == schema.EpistemicArchived && c.ArchivedSeq > latest {
			latest = c.ArchivedSeq
		}
	}

	status := classifyCurrent(st, n, targets, children)
	status.LatestRevisionSeq = latest
	return status
}

// classifyCurrent decides the status of one node from its own state and its
// fold targets, ignoring LatestRevisionSeq plumbing (which currentFor sets).
func classifyCurrent(st State, n *node.Node, targets []Folded[SupportStatus], children []*node.Node) SupportStatus {
	if n.EpistemicState != schema.EpistemicValidated && n.EpistemicState != schema.EpistemicAdmitted {
		return SupportStatus{Cause: CauseNotValidated, Node: n.ID}
	}
	if st.HasBlockingChallenges(n.ID) {
		return SupportStatus{Cause: CauseOpenBlockingChallenge, Node: n.ID}
	}
	if seq, ok := st.LatestAmendmentSeq(n.ID); ok && n.VerdictSeq > 0 && seq > n.VerdictSeq {
		return SupportStatus{Cause: CauseSelfRevised, Node: n.ID, Seq: seq}
	}

	// The result-use relation severs archived and refuted children (0.1.7) so
	// that taint does not pass through an abandoned branch. support_current is
	// stricter for the severed child: archived is cleared, but refuted is a real
	// obstacle to its parent and so must be inspected explicitly here. Every
	// child is a result-use edge whatever its type (v3.2 amendment), so a
	// local_assume child and the children of a local_assume are inspected like
	// any other. Ordinary (unsevered) pending children arrive through targets
	// below.
	for _, c := range children {
		if c.EpistemicState == schema.EpistemicRefuted {
			return SupportStatus{Cause: CauseTargetRefuted, Node: c.ID}
		}
		// A child archived after this node's recorded verdict invalidates that
		// verdict: the parent relied on a decomposition that has since been
		// revised by the child's abandonment, so the parent is not current until
		// it is re-accepted. A fresh accept after the archive clears it because
		// the parent's verdict sequence then post-dates the child's ArchivSeq.
		if c.EpistemicState == schema.EpistemicArchived && n.VerdictSeq > 0 && c.ArchivedSeq > n.VerdictSeq {
			return SupportStatus{Cause: CauseChildArchivedAfterVerdict, Node: c.ID, Seq: c.ArchivedSeq}
		}
	}

	// Result-use targets: dependencies (severed or not) and non-severed
	// children.
	for _, t := range targets {
		if s, bad := classifyTarget(st, n, t); bad {
			return s
		}
	}
	return SupportStatus{Current: true}
}

// classifyTarget reports whether one folded result-use target breaks n's
// support, and if so the status naming the target as responsible.
func classifyTarget(st State, n *node.Node, t Folded[SupportStatus]) (SupportStatus, bool) {
	if t.Cycle {
		return SupportStatus{Cause: CauseCycle, Node: t.ID}, true
	}
	if t.Missing {
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	}
	tn := st.GetNode(t.ID)
	if tn == nil {
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	}
	isChild := false
	if parent, ok := t.ID.Parent(); ok && parent.String() == n.ID.String() {
		isChild = true
	}

	switch tn.EpistemicState {
	case schema.EpistemicRefuted:
		return SupportStatus{Cause: CauseTargetRefuted, Node: t.ID}, true
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return SupportStatus{Cause: CauseTargetPending, Node: t.ID}, true
	case schema.EpistemicArchived:
		if isChild {
			return SupportStatus{}, false // abandoned child: parent no longer relies on it
		}
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	case schema.EpistemicValidated, schema.EpistemicAdmitted:
		// The target's own verdict is terminal; the checks below decide whether
		// it is still current.
	default:
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	}

	// Revision guard: the target's carried latest revision covers its own
	// revisions AND every descendant revision below it, so a consumer is
	// TARGET_REVISED when any descendant revision is newer than the consumer's
	// verdict — even if the target itself was re-accepted afterwards.
	if n.VerdictSeq > 0 && t.Value.LatestRevisionSeq > n.VerdictSeq {
		return SupportStatus{Cause: CauseTargetRevised, Node: t.ID, Seq: t.Value.LatestRevisionSeq}, true
	}
	if !t.Resolved || !t.Value.Current {
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	}
	return SupportStatus{}, false
}

// latestRevisionSeq returns the latest ledger sequence among a node's recorded
// content revisions (statement and dependency amendments, including reopened
// ones), and whether any revision was recorded.
func latestRevisionSeq(st State, id types.NodeID) (int, bool) {
	return st.LatestAmendmentSeq(id)
}
