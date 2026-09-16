package service

import (
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
)

// checkSupportBatch is the single cycle+scope validation entry point for every
// node creation path (Refine, RefineNodeBulk, RecordProof, CreateNode) and for
// the future D2 `amend-deps` path. It runs over a complete prospective batch
// against the one state read inside the caller's commit closure.
//
// For amend-deps (D2) the batch holds one ProspectiveNode per amended node
// whose Dependencies/ValidationDeps REPLACE the node's current edges; a
// removal-only batch therefore drops the removed edge from the graph and is
// accepted even if an unrelated legacy cycle remains in state.
func checkSupportBatch(st *state.State, batch []support.ProspectiveNode) error {
	return support.CheckCreation(st, batch)
}
