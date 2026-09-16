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

// ArchivedObligations returns the IDs of open-challenge obligations abandoned
// by the archive of a direct child of parentID. It reads the durable
// abandoned_obligations snapshot recorded on the child's NodeArchived event
// (D9), so a descendant-only obligation (e.g. a challenge on 1.1.1 abandoned
// by archiving 1.1) still surfaces on 1's checklist. Legacy archives that
// predate the snapshot fall back to the challenge-trace derivation: a
// superseded (auto-closed on archive) or still-open challenge directly on the
// archived child is the durable record of an abandoned obligation.
func (s *State) ArchivedObligations(parentID types.NodeID) []types.NodeID {
	if s == nil {
		return nil
	}
	var out []types.NodeID
	seen := make(map[string]bool)
	add := func(id types.NodeID) {
		if seen[id.String()] {
			return
		}
		seen[id.String()] = true
		out = append(out, id)
	}

	for _, n := range s.AllNodes() {
		parent, ok := n.ID.Parent()
		if !ok || parent.String() != parentID.String() {
			continue
		}
		if n.EpistemicState != schema.EpistemicArchived {
			continue
		}

		if len(n.AbandonedObligations) > 0 {
			for _, raw := range n.AbandonedObligations {
				if id, err := types.Parse(raw); err == nil {
					add(id)
				}
			}
			continue
		}

		// Legacy fallback: no durable snapshot, so a challenge trace on the
		// archived child itself is the only recoverable obligation.
		for _, c := range s.GetChallengesForNode(n.ID) {
			if c.Status == ChallengeStatusSuperseded || c.Status == ChallengeStatusOpen {
				add(n.ID)
				break
			}
		}
	}
	return out
}
