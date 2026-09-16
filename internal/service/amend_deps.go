package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Amend-deps outcomes. The strings are part of the CLI/manifest contract.
const (
	AmendDepsApplied        = "applied"
	AmendDepsAppliedAlready = "applied-already"
	AmendDepsUnchanged      = "unchanged"
)

// Sentinel precondition errors. Each carries an AFError code so the manifest
// form can report a stable rejected:<code> and the single command exits 3
// (logic) or 1 (retriable) per the code.
var (
	// ErrAmendDepsHashMismatch: --expect-hash does not match the node's
	// current content hash. Exit 3.
	ErrAmendDepsHashMismatch = aferrors.New(aferrors.INVALID_STATE, "content hash does not match --expect-hash")
	// ErrAmendDepsContradiction: an id appears in both an add and a remove
	// list for the same edge kind. Exit 3.
	ErrAmendDepsContradiction = aferrors.New(aferrors.INVALID_TARGET, "contradictory add and remove of the same dependency")
	// ErrAmendDepsStrictNoChange: --strict and the requested change is a no-op
	// (add of an existing edge or remove of an absent one). Exit 3.
	ErrAmendDepsStrictNoChange = aferrors.New(aferrors.INVALID_TARGET, "requested dependency change is already satisfied")
	// ErrAmendDepsAdmitted: an admitted node must be unadmitted before its
	// edges can be corrected. Exit 3.
	ErrAmendDepsAdmitted = aferrors.New(aferrors.INVALID_STATE, "node is admitted; run `af unadmit` first")
	// ErrAmendDepsOperationIDConflict: an operation id was already committed for
	// a different node or a different change set. Exit 3.
	ErrAmendDepsOperationIDConflict = aferrors.New(aferrors.OPERATION_ID_CONFLICT, "operation id already committed for a different request")
)

// AmendDepsRequest is one edge correction. Empty lists are no-ops; Reopen
// additionally performs validated -> pending in the same event; ExpectHash is
// the revision the caller authored against; Strict turns a no-op edge change
// into an error; OperationID makes a retry discover its committed result.
type AmendDepsRequest struct {
	Add             []types.NodeID
	Remove          []types.NodeID
	AddValidated    []types.NodeID
	RemoveValidated []types.NodeID
	Reopen          bool
	ExpectHash      string
	Strict          bool
	Owner           string
	Reason          string
	OperationID     string
}

// AmendDepsResult is the per-operation report, shared by the single command and
// each manifest item.
type AmendDepsResult struct {
	Outcome          string         `json:"outcome"`
	Seq              int            `json:"seq,omitempty"`
	OldHash          string         `json:"old_hash,omitempty"`
	NewHash          string         `json:"new_hash,omitempty"`
	Reopened         bool           `json:"reopened,omitempty"`
	Added            []types.NodeID `json:"added,omitempty"`
	Removed          []types.NodeID `json:"removed,omitempty"`
	AddedValidated   []types.NodeID `json:"added_validated,omitempty"`
	RemovedValidated []types.NodeID `json:"removed_validated,omitempty"`
	// Reverify is true when the node is now pending because this operation
	// reopened a validated node, i.e. the caller must re-accept it.
	Reverify bool `json:"reverify,omitempty"`
}

// depsPlan is the pure outcome of planning one edge correction against one
// state read. Event is nil when nothing should be appended (unchanged or
// already applied).
type depsPlan struct {
	Event  ledger.Event
	Result AmendDepsResult
}

// AmendDeps corrects one node's dependency edges as exactly one ledger event.
//
// Preconditions run in this order inside the single commit closure: node
// exists; operation id already committed (short-circuit to applied-already);
// epistemic state admits the change; claim ownership; --expect-hash matches;
// every target node exists; add/remove lists do not contradict; then the change
// set, then the D1 cycle/scope check over the prospective node. Only if all of
// that passes is one NodeDepsAmended event built. Reopen and the edge
// replacement are the same event.
func (s *ProofService) AmendDeps(nodeID types.NodeID, req AmendDepsRequest) (AmendDepsResult, error) {
	var plan depsPlan
	seqs, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		p, err := planDepsAmendment(st, nodeID, req)
		if err != nil {
			return nil, err
		}
		plan = p
		if p.Event == nil {
			return nil, nil
		}
		return []ledger.Event{p.Event}, nil
	})
	if err != nil {
		return AmendDepsResult{}, wrapSequenceMismatch(err, "AmendDeps")
	}
	if len(seqs) > 0 {
		plan.Result.Seq = seqs[0]
	}
	return plan.Result, nil
}

// planDepsAmendment is the read-only core shared by AmendDeps and the manifest
// dry run. It never mutates st and never appends; it returns the event to
// append (nil when nothing changes) and the result to report.
func planDepsAmendment(st *state.State, nodeID types.NodeID, req AmendDepsRequest) (depsPlan, error) {
	if err := validateAmendDepsInput(req); err != nil {
		return depsPlan{}, err
	}

	n := st.GetNode(nodeID)
	if n == nil {
		return depsPlan{}, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
	}

	// A retried operation discovers its own committed result before any
	// precondition that the operation itself changed (state, hash). The id is
	// bound to the event type, node and canonical request fingerprint, so a
	// reuse of the id for a different request is a conflict rather than a
	// false applied-already.
	if req.OperationID != "" {
		if rec, ok := st.LookupOperation(req.OperationID); ok {
			fp := fingerprintAmendDepsRequest(nodeID, req)
			if rec.EventType != string(ledger.EventNodeDepsAmended) ||
				rec.NodeID != nodeID.String() || rec.RequestFingerprint != fp {
				return depsPlan{}, fmt.Errorf("%w: operation id %s belongs to %s %s",
					ErrAmendDepsOperationIDConflict, req.OperationID, operationRecordLabel(rec), rec.NodeID)
			}
			return depsPlan{Result: AmendDepsResult{
				Outcome:  AmendDepsAppliedAlready,
				Seq:      rec.Seq,
				OldHash:  rec.PreviousHash,
				NewHash:  rec.NewHash,
				Reopened: rec.Reopened,
				Reverify: rec.Reopened,
			}}, nil
		}
	}

	if err := checkAmendDepsState(n, req.Reopen); err != nil {
		return depsPlan{}, err
	}
	if err := checkAmendDepsOwnership(n, req.Owner); err != nil {
		return depsPlan{}, err
	}
	if req.ExpectHash != "" && n.ContentHash != req.ExpectHash {
		return depsPlan{}, fmt.Errorf("%w: node %s is at hash %s, expected %s",
			ErrAmendDepsHashMismatch, nodeID.String(), n.ContentHash, req.ExpectHash)
	}
	if err := checkAmendDepsTargets(st, req); err != nil {
		return depsPlan{}, err
	}
	if err := checkAmendDepsContradictions(req); err != nil {
		return depsPlan{}, err
	}

	newDeps, addedDeps, removedDeps, err := applyEdgeChange(n.Dependencies, req.Add, req.Remove, req.Strict)
	if err != nil {
		return depsPlan{}, err
	}
	newValDeps, addedVal, removedVal, err := applyEdgeChange(n.ValidationDeps, req.AddValidated, req.RemoveValidated, req.Strict)
	if err != nil {
		return depsPlan{}, err
	}

	edgesChanged := len(addedDeps)+len(removedDeps)+len(addedVal)+len(removedVal) > 0
	reopen := req.Reopen && n.EpistemicState == schema.EpistemicValidated
	if !edgesChanged && !reopen {
		return depsPlan{Result: AmendDepsResult{
			Outcome:          AmendDepsUnchanged,
			OldHash:          n.ContentHash,
			NewHash:          n.ContentHash,
			Added:            addedDeps,
			Removed:          removedDeps,
			AddedValidated:   addedVal,
			RemovedValidated: removedVal,
		}}, nil
	}

	// Compute the resulting hash without mutating the state copy: clone the
	// node and replace the edges before recomputing.
	preview := *n
	preview.Dependencies = newDeps
	preview.ValidationDeps = newValDeps

	result := AmendDepsResult{
		Outcome:          AmendDepsApplied,
		OldHash:          n.ContentHash,
		NewHash:          preview.ComputeContentHash(),
		Reopened:         reopen,
		Reverify:         reopen,
		Added:            addedDeps,
		Removed:          removedDeps,
		AddedValidated:   addedVal,
		RemovedValidated: removedVal,
	}

	// D1: cycle and scope checks over the node with its NEW edge lists. On
	// rejection the partial result is returned so the dry run can still show
	// the exact edge diff that was attempted.
	pn := support.ProspectiveNode{
		ID:             nodeID,
		Type:           n.Type,
		Dependencies:   newDeps,
		ValidationDeps: newValDeps,
	}
	if parent, ok := nodeID.Parent(); ok {
		pn.ParentID = parent
	}
	if err := checkSupportBatch(st, []support.ProspectiveNode{pn}); err != nil {
		return depsPlan{Result: result}, err
	}

	event := ledger.NewNodeDepsAmended(nodeID, n.Dependencies, newDeps, n.ValidationDeps, newValDeps,
		req.Owner, req.Reason, n.ContentHash, reopen)
	event.OperationID = req.OperationID
	event.RequestFingerprint = fingerprintAmendDepsRequest(nodeID, req)

	return depsPlan{Event: event, Result: result}, nil
}

// fingerprintAmendDepsRequest is the canonical digest that binds an operation
// id to the request that created it: the node, the reopen flag, and the four
// sorted edge lists. It deliberately excludes owner, reason, expect-hash and
// strict, which are provenance/precondition fields rather than the change set.
func fingerprintAmendDepsRequest(nodeID types.NodeID, req AmendDepsRequest) string {
	var b strings.Builder
	b.WriteString("node=")
	b.WriteString(nodeID.String())
	if req.Reopen {
		b.WriteString("\nreopen=1")
	} else {
		b.WriteString("\nreopen=0")
	}
	for _, list := range [][]types.NodeID{req.Add, req.Remove, req.AddValidated, req.RemoveValidated} {
		b.WriteString("\n")
		b.WriteString(canonicalIDList(list))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// canonicalIDList renders an ID slice as a sorted, comma-joined string.
func canonicalIDList(ids []types.NodeID) string {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	sort.Strings(strs)
	return strings.Join(strs, ",")
}

// operationRecordLabel describes a committed operation's event type for a
// conflict error.
func operationRecordLabel(rec state.OperationRecord) string {
	if rec.EventType == "" {
		return "an event"
	}
	return rec.EventType
}

// validateAmendDepsInput enforces the required provenance fields.
func validateAmendDepsInput(req AmendDepsRequest) error {
	if req.Owner == "" {
		return fmt.Errorf("%w: owner", ErrEmptyInput)
	}
	if req.Reason == "" {
		return fmt.Errorf("%w: reason", ErrEmptyInput)
	}
	return nil
}

// checkAmendDepsState enforces the allowed epistemic states: pending, draft and
// needs_refinement directly; validated only with reopen; admitted only after
// `af unadmit`; refuted and archived never.
func checkAmendDepsState(n *node.Node, reopen bool) error {
	switch n.EpistemicState {
	case schema.EpistemicPending, schema.EpistemicDraft, schema.EpistemicNeedsRefinement:
		return nil
	case schema.EpistemicValidated:
		if reopen {
			return nil
		}
		return fmt.Errorf("%w: node %s is validated; pass --reopen to correct its dependencies and reopen it",
			ErrInvalidState, n.ID.String())
	case schema.EpistemicAdmitted:
		return fmt.Errorf("%w: node %s", ErrAmendDepsAdmitted, n.ID.String())
	case schema.EpistemicRefuted, schema.EpistemicArchived:
		return fmt.Errorf("%w: node %s is %s and cannot be amended", ErrInvalidState, n.ID.String(), n.EpistemicState)
	default:
		return fmt.Errorf("%w: node %s is in state %s", ErrInvalidState, n.ID.String(), n.EpistemicState)
	}
}

// checkAmendDepsOwnership mirrors af amend: a claimed node may be corrected
// only by its claim holder; an unclaimed node may be corrected by anyone.
func checkAmendDepsOwnership(n *node.Node, owner string) error {
	if n.WorkflowState == schema.WorkflowClaimed && n.ClaimedBy != owner {
		return fmt.Errorf("%w: node %s is claimed by %s, not %s", ErrOwnerMismatch, n.ID.String(), n.ClaimedBy, owner)
	}
	return nil
}

// checkAmendDepsTargets requires every referenced target in an ADD list to
// exist, so an amendment cannot introduce a dangling edge silently. Remove
// lists are exempt: removing a present edge must always be possible, including
// a dangling edge whose target was never created or has since been
// archived/refuted (D1 keeps such edges as sinks; D2 must be able to clear
// them). A remove of an edge that is not present is a no-op (or a --strict
// error), handled by applyEdgeChange.
func checkAmendDepsTargets(st *state.State, req AmendDepsRequest) error {
	all := make([]types.NodeID, 0, len(req.Add)+len(req.AddValidated))
	all = append(all, req.Add...)
	all = append(all, req.AddValidated...)
	for _, id := range all {
		if st.GetNode(id) == nil {
			return fmt.Errorf("%w: dependency target %s", ErrNodeNotFound, id.String())
		}
	}
	return nil
}

// checkAmendDepsContradictions rejects an id in both the add and remove list
// for the same edge kind.
func checkAmendDepsContradictions(req AmendDepsRequest) error {
	if id, ok := intersect(req.Add, req.Remove); ok {
		return fmt.Errorf("%w: %s", ErrAmendDepsContradiction, id.String())
	}
	if id, ok := intersect(req.AddValidated, req.RemoveValidated); ok {
		return fmt.Errorf("%w: %s", ErrAmendDepsContradiction, id.String())
	}
	return nil
}

// applyEdgeChange returns the new edge list after add/remove. An add of an
// existing edge or a remove of an absent one is ignored unless strict, in
// which case it is an error. added/removed report only the edges that actually
// changed.
func applyEdgeChange(current, add, remove []types.NodeID, strict bool) (newList, added, removed []types.NodeID, err error) {
	currentSet := make(map[string]bool, len(current))
	for _, id := range current {
		currentSet[id.String()] = true
	}

	removeSet := make(map[string]bool, len(remove))
	for _, id := range remove {
		removeSet[id.String()] = true
	}
	addSet := make(map[string]bool, len(add))
	for _, id := range add {
		addSet[id.String()] = true
	}

	if strict {
		for _, id := range remove {
			if !currentSet[id.String()] {
				return nil, nil, nil, fmt.Errorf("%w: remove %s (not a current dependency)", ErrAmendDepsStrictNoChange, id.String())
			}
		}
		for _, id := range add {
			if currentSet[id.String()] {
				return nil, nil, nil, fmt.Errorf("%w: add %s (already a dependency)", ErrAmendDepsStrictNoChange, id.String())
			}
		}
	}

	addedSet := make(map[string]bool, len(add))
	for _, id := range add {
		if !currentSet[id.String()] && !addedSet[id.String()] {
			addedSet[id.String()] = true
			added = append(added, id)
		}
	}
	for _, id := range remove {
		if currentSet[id.String()] {
			removed = append(removed, id)
		}
	}

	seen := make(map[string]bool, len(current)+len(add))
	for _, id := range current {
		key := id.String()
		if removeSet[key] || seen[key] {
			continue
		}
		seen[key] = true
		newList = append(newList, id)
	}
	for _, id := range add {
		key := id.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		newList = append(newList, id)
	}
	return newList, added, removed, nil
}

// intersect returns the first id present in both slices, and whether any was.
func intersect(a, b []types.NodeID) (types.NodeID, bool) {
	set := make(map[string]bool, len(a))
	for _, id := range a {
		set[id.String()] = true
	}
	for _, id := range b {
		if set[id.String()] {
			return id, true
		}
	}
	return types.NodeID{}, false
}
