package service

import (
	"strings"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
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

// formatObligations renders node IDs for an error line.
func formatObligations(ids []types.NodeID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ", ")
}
