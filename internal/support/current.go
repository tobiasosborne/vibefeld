package support

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Stable cause codes reported by SupportStatus.Cause. They are the machine
// contract `af status`, `af health`, `af get` and `af export` surface; message
// text may change, these may not.
const (
	CauseNotValidated          = "NOT_VALIDATED"
	CauseOpenBlockingChallenge = "OPEN_BLOCKING_CHALLENGE"
	CauseTargetNotCurrent      = "TARGET_NOT_CURRENT"
	CauseTargetPending         = "TARGET_PENDING"
	CauseTargetRefuted         = "TARGET_REFUTED"
	CauseTargetRevised         = "TARGET_REVISED"
	CauseSelfRevised           = "SELF_REVISED"
	CauseCycle                 = "CYCLE"
)

// SupportStatus is whether a node's recorded verdict is currently supported by
// the proof under it, and if not, why and which node is responsible. Cause is
// empty and Node is the zero value when Current is true. Seq is the ledger
// sequence of the event that broke support when one is known (a revision
// sequence for the *_REVISED causes; 0 otherwise).
type SupportStatus struct {
	Current bool         `json:"current"`
	Cause   string       `json:"cause,omitempty"`
	Node    types.NodeID `json:"node,omitempty"`
	Seq     int          `json:"seq,omitempty"`
}

// Current computes support_current for every node in st, memoised over the
// result-use graph with the one shared Walk (v3.1 amendment 4). The definition
// is recursive and revision-aware:
//
//   - n is validated or admitted;
//   - n has no open blocking challenge;
//   - n itself has no content revision (statement or dependency amendment)
//     after its recorded validation sequence;
//   - every result-use target t is validated, admitted, or — for children only
//     — archived, AND support_current(t); a refuted, pending, draft or
//     needs_refinement target fails it, and a missing target fails it;
//   - no target has a content revision later than n's validation sequence.
//
// A legacy result-use cycle is reported as CYCLE rather than erroring:
// validation remains a recorded verdict, it just cannot be current.
func Current(st *state.State) map[string]SupportStatus {
	if st == nil {
		return map[string]SupportStatus{}
	}

	// The result-use relation severs archived and refuted children (0.1.7) so
	// that taint does not pass through an abandoned branch. support_current is
	// stricter: an archived child is cleared, but a refuted child is a real
	// obstacle to its parent. The walk therefore cannot see a severed child, so
	// gather direct children up front and let the fold inspect them explicitly.
	children := make(map[string][]*node.Node)
	for _, n := range st.AllNodes() {
		parent, ok := n.ID.Parent()
		if !ok {
			continue
		}
		children[parent.String()] = append(children[parent.String()], n)
	}
	for id := range children {
		sort.Slice(children[id], func(i, j int) bool { return children[id][i].ID.Less(children[id][j].ID) })
	}

	p := ResultUseEdges(st, nil)
	return Walk(p, func(n *node.Node, targets []Folded[SupportStatus]) SupportStatus {
		return currentFor(st, n, targets, children[n.ID.String()])
	})
}

func currentFor(st *state.State, n *node.Node, targets []Folded[SupportStatus], children []*node.Node) SupportStatus {
	if n.EpistemicState != schema.EpistemicValidated && n.EpistemicState != schema.EpistemicAdmitted {
		return SupportStatus{Cause: CauseNotValidated, Node: n.ID}
	}
	if len(st.GetBlockingChallengesForNode(n.ID)) > 0 {
		return SupportStatus{Cause: CauseOpenBlockingChallenge, Node: n.ID}
	}
	if seq, ok := latestRevisionSeq(st, n.ID); ok && n.ValidatedSeq > 0 && seq > n.ValidatedSeq {
		return SupportStatus{Cause: CauseSelfRevised, Node: n.ID, Seq: seq}
	}

	// Direct children, including the ones the result-use relation severs.
	for _, c := range children {
		switch c.EpistemicState {
		case schema.EpistemicRefuted:
			return SupportStatus{Cause: CauseTargetRefuted, Node: c.ID}
		case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
			return SupportStatus{Cause: CauseTargetPending, Node: c.ID}
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
func classifyTarget(st *state.State, n *node.Node, t Folded[SupportStatus]) (SupportStatus, bool) {
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

	if seq, ok := latestRevisionSeq(st, t.ID); ok && n.ValidatedSeq > 0 && seq > n.ValidatedSeq {
		return SupportStatus{Cause: CauseTargetRevised, Node: t.ID, Seq: seq}, true
	}
	if !t.Resolved || !t.Value.Current {
		return SupportStatus{Cause: CauseTargetNotCurrent, Node: t.ID}, true
	}
	return SupportStatus{}, false
}

// latestRevisionSeq returns the latest ledger sequence among a node's recorded
// content revisions (statement and dependency amendments, including reopened
// ones), and whether any revision was recorded.
func latestRevisionSeq(st *state.State, id types.NodeID) (int, bool) {
	best := 0
	found := false
	for _, a := range st.GetAmendmentHistory(id) {
		if a.Seq > best {
			best = a.Seq
			found = true
		}
	}
	return best, found
}
