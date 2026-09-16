package audit

import (
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// snapshot is the one immutable read of derived state the audit runs on. Every
// producer takes its nodes, amendments and challenges from here, so no state
// getter locks or populates a cache mid-run and two audits of the same
// workspace produce byte-identical output.
type snapshot struct {
	nodes      []*node.Node
	children   map[string][]*node.Node
	amendments map[string][]state.Amendment

	// allChallenges is sorted by (node ID, sequence, challenge ID).
	allChallenges  []*state.Challenge
	openByNode     map[string][]*state.Challenge
	blockingByNode map[string][]*state.Challenge
}

// newSnapshot takes the single state read. It pre-populates the state's
// challenge-by-node cache exactly once before any producer runs; every later
// challenge lookup is served from that cache, so no check mutates the state
// while the audit is in flight.
func newSnapshot(st *state.State) *snapshot {
	s := &snapshot{
		children:       make(map[string][]*node.Node),
		amendments:     make(map[string][]state.Amendment),
		openByNode:     make(map[string][]*state.Challenge),
		blockingByNode: make(map[string][]*state.Challenge),
	}
	if st == nil {
		return s
	}

	// The one permitted mutation: fill the challenge cache up front.
	st.ChallengesByNodeID()

	s.nodes = st.AllNodes()
	sort.Slice(s.nodes, func(i, j int) bool { return s.nodes[i].ID.Less(s.nodes[j].ID) })
	for _, n := range s.nodes {
		if parent, ok := n.ID.Parent(); ok {
			key := parent.String()
			s.children[key] = append(s.children[key], n)
		}
		s.amendments[n.ID.String()] = st.GetAmendmentHistory(n.ID)
	}
	for _, cs := range s.children {
		sort.Slice(cs, func(i, j int) bool { return cs[i].ID.Less(cs[j].ID) })
	}

	s.allChallenges = st.AllChallenges()
	sort.Slice(s.allChallenges, func(i, j int) bool {
		a, b := s.allChallenges[i], s.allChallenges[j]
		if !a.NodeID.Equal(b.NodeID) {
			return a.NodeID.Less(b.NodeID)
		}
		if a.Seq != b.Seq {
			return a.Seq < b.Seq
		}
		return a.ID < b.ID
	})
	for _, c := range s.allChallenges {
		key := c.NodeID.String()
		if c.Status == state.ChallengeStatusOpen {
			s.openByNode[key] = append(s.openByNode[key], c)
			if schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
				s.blockingByNode[key] = append(s.blockingByNode[key], c)
			}
		}
	}
	return s
}

// latestRevisionSeq returns the latest amendment sequence recorded for a node
// in the snapshot, and whether any amendment was recorded.
func (s *snapshot) latestRevisionSeq(id types.NodeID) (int, bool) {
	best := 0
	found := false
	for _, a := range s.amendments[id.String()] {
		if a.Seq > best {
			best = a.Seq
			found = true
		}
	}
	return best, found
}

// openChallengesAt returns the node itself and every active (non-severed)
// descendant with at least one open challenge, in hierarchical order, using
// the immutable children index and challenge snapshot.
func (s *snapshot) openChallengesAt(root *node.Node) []*node.Node {
	if root == nil {
		return nil
	}
	var out []*node.Node
	var walk func(n *node.Node, isRoot bool)
	walk = func(n *node.Node, isRoot bool) {
		if n == nil {
			return
		}
		if !isRoot && (n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted) {
			return
		}
		if len(s.openByNode[n.ID.String()]) > 0 {
			out = append(out, n)
		}
		for _, c := range s.children[n.ID.String()] {
			walk(c, false)
		}
	}
	walk(root, true)
	return out
}
