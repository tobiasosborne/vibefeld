package service

import (
	"strings"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// ErrOpenChallengeObligation refuses an archive that would abandon a node (or
// an active descendant) while a challenge is open on it, unless --force and a
// non-empty --reason were given.
var ErrOpenChallengeObligation = aferrors.New(aferrors.NODE_BLOCKED,
	"cannot archive while a challenge is open on this node or an active descendant; pass --force --reason to abandon the obligation")

// setFencedClaimRelease stamps D5's claim-release fields on a terminal state
// event when the caller holds the node's current claim. Caller identity is a
// driver-supplied string and the generation is the ledger sequence of the
// NodesClaimed event that created the claim; both are checked at replay, so a
// delayed or retried event cannot evict a later claim, even under the same
// owner string. A caller with no identity (empty) never releases.
func setFencedClaimRelease(n *node.Node, caller string, release *bool, claimSeq *int) {
	if n == nil || caller == "" {
		return
	}
	if n.WorkflowState != schema.WorkflowClaimed {
		return
	}
	if n.ClaimedBy != caller || n.ClaimSeq == 0 {
		return
	}
	*release = true
	*claimSeq = n.ClaimSeq
}

// newFencedNodesReleased builds an explicit NodesReleased event, stamping each
// node's current claim generation (read from st, the same state read the
// commit was built against) into the positionally aligned ClaimSeqs slice.
// Every explicit release path goes through this so a delayed or retried
// release cannot evict a later claim. When no target has a claim generation
// the slice is left nil, which replays as a legacy unfenced release.
func newFencedNodesReleased(st *state.State, ids []types.NodeID) ledger.NodesReleased {
	ev := ledger.NewNodesReleased(ids)
	if st == nil {
		return ev
	}
	seqs := make([]int, len(ids))
	any := false
	for i, id := range ids {
		if n := st.GetNode(id); n != nil {
			seqs[i] = n.ClaimSeq
			if n.ClaimSeq != 0 {
				any = true
			}
		}
	}
	if any {
		ev.ClaimSeqs = seqs
	}
	return ev
}

// obligationStrings renders node IDs as strings for the durable
// abandoned_obligations snapshot on NodeArchived.
func obligationStrings(ids []types.NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// formatObligations renders node IDs for an error line.
func formatObligations(ids []types.NodeID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ", ")
}
