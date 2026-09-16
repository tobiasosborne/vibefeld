package state

import (
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// OpenChallengeObligations returns the IDs of id and any active (non-severed)
// descendant that has at least one open challenge. A severed node (archived
// or refuted) prunes its subtree: abandoning a branch that is already
// abandoned is not a new obligation.
//
// It is used by the archive guard (refuse unless forced with a reason) and by
// the verification checklist (surface an abandoned obligation to the next
// verifier). Replay does not supersede an open challenge when its node is
// merely being considered for archival, so the check is against live state.
func (s *State) OpenChallengeObligations(id types.NodeID) []types.NodeID {
	if s == nil {
		return nil
	}

	// Group nodes by parent so the walk can prune severed subtrees.
	children := make(map[string][]*node.Node)
	for _, n := range s.AllNodes() {
		parent, ok := n.ID.Parent()
		if !ok {
			continue
		}
		children[parent.String()] = append(children[parent.String()], n)
	}

	var out []types.NodeID
	var walk func(cur types.NodeID)
	walk = func(cur types.NodeID) {
		n := s.GetNode(cur)
		if n == nil {
			return
		}
		if nodeSevered(n) {
			return
		}
		if s.HasOpenChallenge(cur) {
			out = append(out, cur)
		}
		for _, child := range children[cur.String()] {
			walk(child.ID)
		}
	}
	walk(id)
	return out
}

// HasOpenChallenge reports whether any challenge on id is still open.
func (s *State) HasOpenChallenge(id types.NodeID) bool {
	for _, c := range s.GetChallengesForNode(id) {
		if c.Status == ChallengeStatusOpen {
			return true
		}
	}
	return false
}

// nodeSevered reports whether a node's branch has been abandoned.
func nodeSevered(n *node.Node) bool {
	return n.EpistemicState == schema.EpistemicArchived || n.EpistemicState == schema.EpistemicRefuted
}

// ArchivedChildrenWithAbandonedChallenges returns the direct children of
// parentID that are archived and still carry the trace of a challenge that was
// open when they were archived. Replay auto-supersedes a node's open
// challenges on archive, so a superseded challenge on an archived child is the
// durable record of an abandoned obligation; a still-open challenge (legacy or
// raised after archival) also counts. The verification checklist surfaces
// these so the next accept acknowledges them (D9).
func (s *State) ArchivedChildrenWithAbandonedChallenges(parentID types.NodeID) []types.NodeID {
	if s == nil {
		return nil
	}
	var out []types.NodeID
	for _, n := range s.AllNodes() {
		parent, ok := n.ID.Parent()
		if !ok || parent.String() != parentID.String() {
			continue
		}
		if n.EpistemicState != schema.EpistemicArchived {
			continue
		}
		for _, c := range s.GetChallengesForNode(n.ID) {
			if c.Status == ChallengeStatusSuperseded || c.Status == ChallengeStatusOpen {
				out = append(out, n.ID)
				break
			}
		}
	}
	return out
}
