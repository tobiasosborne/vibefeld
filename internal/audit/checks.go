package audit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/lemma"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// amendmentHotspotLimit is how many AMENDMENTS_PER_NODE hotspots are reported.
const amendmentHotspotLimit = 10

// checksInput bundles the one immutable state read and the ordered ledger pass
// (which may be nil for a state-only audit).
type checksInput struct {
	st   *state.State
	pass *Pass
	snap *snapshot
}

// computeFindings runs the ordered audit pass. want reports whether a code's
// producer should run; callers that only consume a subset (health) use it to
// skip the rest of the work.
func computeFindings(st *state.State, pass *Pass, want func(code string) bool) []Finding {
	if st == nil {
		return nil
	}
	in := checksInput{st: st, pass: pass, snap: newSnapshot(st)}
	var fs []Finding
	if want(CodeSupportNotCurrent) {
		fs = append(fs, supportNotCurrent(st)...)
	}
	if want(CodeHashMismatch) {
		fs = append(fs, hashMismatch(in)...)
	}
	if want(CodeCycle) {
		fs = append(fs, cycles(st)...)
	}
	if want(CodeScopeLeak) {
		fs = append(fs, scopeLeaks(st)...)
	}
	if want(CodeCitesSevered) {
		fs = append(fs, citesSevered(st)...)
	}
	if want(CodeAmendedNotReverified) {
		fs = append(fs, amendedNotReverified(in)...)
	}
	if want(CodeSelfAccept) {
		fs = append(fs, selfAccepts(in)...)
	}
	if want(CodeValidatedWithOpenBlockingChallenge) {
		fs = append(fs, validatedWithBlockingChallenge(in)...)
	}
	if want(CodeAdmitted) {
		fs = append(fs, admittedNodes(in)...)
	}
	if want(CodeArchivedWithOpenChallenge) {
		fs = append(fs, archivedWithOpenChallenge(in)...)
	}
	if want(CodeAmendmentsPerNode) {
		fs = append(fs, amendmentHotspots(in)...)
	}
	if want(CodePendingExternalCitedByValidated) {
		fs = append(fs, pendingExternalCitedByValidated(in)...)
	}
	if want(CodeUnknownProvenance) {
		fs = append(fs, unknownProvenance(in)...)
	}
	return seedFindings(fs)
}

// supportNotCurrent emits one current finding per validated or admitted node
// whose derived support_current is false. The finding lists the offending node
// first and the responsible node second (supplied by support.Current) and
// carries the stable cause.
func supportNotCurrent(st *state.State) []Finding {
	if st == nil {
		return nil
	}
	statuses := support.Current(st)
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated && n.EpistemicState != schema.EpistemicAdmitted {
			continue
		}
		sup := statuses[n.ID.String()]
		if sup.Current || sup.Cause == "" {
			continue
		}
		responsible := sup.Node
		nodes := []types.NodeID{n.ID}
		if responsible.String() != "" {
			nodes = append(nodes, responsible)
		}
		var seqs []int
		if sup.Seq > 0 {
			seqs = []int{sup.Seq}
		}
		fs = append(fs, Finding{
			Code:     CodeSupportNotCurrent,
			Severity: SeverityError,
			Status:   StatusCurrent,
			Nodes:    nodes,
			Seqs:     seqs,
			Cause:    sup.Cause,
			Message: fmt.Sprintf("validated node %s is not support_current: %s (responsible: %s)",
				n.ID.String(), sup.Cause, responsible.String()),
			Remediation: RemediationFor(CodeSupportNotCurrent),
		})
	}
	return fs
}

// hashMismatch compares the accepted content hash with a fresh recompute of the
// node's content on FINAL state. The accepted hash is the recorded
// ValidatedContentHash when present, else the hash reconstructed at the
// acceptance sequence from the ordered pass. A recorded mismatch is a current,
// strict finding; a reconstructed one is historical and never gates. No finding
// is emitted when the accepted hash equals the final content hash -- which is
// what removes the historical false positives on the pre-D3 corpus.
func hashMismatch(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		finalHash := n.ComputeContentHash()
		accepted := n.ValidatedContentHash
		reconstructed := false
		if accepted == "" {
			reconstructed = true
			if in.pass != nil {
				if a, ok := in.pass.Acceptance(n.ID); ok {
					accepted = a.ContentHash
				}
			}
		}
		if accepted == "" {
			// No recorded hash and no acceptance on the ledger to reconstruct
			// from. Report the unverifiable acceptance, never gating.
			fs = append(fs, Finding{
				Code:     CodeHashMismatch,
				Severity: SeverityWarning,
				Status:   StatusHistorical,
				Nodes:    []types.NodeID{n.ID},
				Message: fmt.Sprintf("node %s was validated without a recorded content hash; "+
					"the comparison is reconstructed and never gates --strict", n.ID.String()),
				Remediation: RemediationFor(CodeHashMismatch),
			})
			continue
		}
		if accepted == finalHash {
			continue
		}
		status := StatusCurrent
		severity := SeverityError
		source := "recorded"
		if reconstructed {
			status = StatusHistorical
			severity = SeverityWarning
			source = "reconstructed"
		}
		fs = append(fs, Finding{
			Code:     CodeHashMismatch,
			Severity: severity,
			Status:   status,
			Nodes:    []types.NodeID{n.ID},
			Message: fmt.Sprintf("%s hash %s does not match current content hash %s for node %s",
				source, shortHash(accepted), shortHash(finalHash), n.ID.String()),
			Remediation: RemediationFor(CodeHashMismatch),
		})
	}
	return fs
}

// cycles emits one current finding per legacy result-use strongly-connected
// component.
func cycles(st *state.State) []Finding {
	g := support.Prepare(support.ResultUseEdges(st, nil))
	var fs []Finding
	for _, comp := range g.CyclicComponents() {
		ids := make([]string, len(comp))
		for i, id := range comp {
			ids[i] = id.String()
		}
		fs = append(fs, Finding{
			Code:        CodeCycle,
			Severity:    SeverityError,
			Status:      StatusCurrent,
			Nodes:       comp,
			Message:     "result-use cycle: " + strings.Join(ids, " -> ") + " -> " + ids[0],
			Remediation: RemediationFor(CodeCycle),
		})
	}
	return fs
}

// scopeLeaks emits one current finding per existing node that result-uses a
// node inside a local assumption that does not enclose it.
func scopeLeaks(st *state.State) []Finding {
	var fs []Finding
	for _, leak := range support.ScopeLeaks(st) {
		nodes := nonZeroIDs(leak.Node, leak.Dep, leak.Assumption)
		fs = append(fs, Finding{
			Code:     CodeScopeLeak,
			Severity: SeverityError,
			Status:   StatusCurrent,
			Nodes:    nodes,
			Message: fmt.Sprintf("node %s cites %s inside local assumption %s, whose scope does not enclose it",
				leak.Node.String(), leak.Dep.String(), leak.Assumption.String()),
			Remediation: RemediationFor(CodeScopeLeak),
		})
	}
	return fs
}

// citesSevered emits one current finding per dependency on a severed
// (archived/refuted) or missing target.
func citesSevered(st *state.State) []Finding {
	var fs []Finding
	for _, d := range support.DanglingDeps(st, nil) {
		kind := "missing"
		if d.Severed {
			kind = "severed (archived or refuted)"
		}
		fs = append(fs, Finding{
			Code:     CodeCitesSevered,
			Severity: SeverityError,
			Status:   StatusCurrent,
			Nodes:    nonZeroIDs(d.From, d.To),
			Message: fmt.Sprintf("node %s depends on %s %s, which cannot support it",
				d.From.String(), kind, d.To.String()),
			Remediation: RemediationFor(CodeCitesSevered),
		})
	}
	return fs
}

// amendedNotReverified emits a current finding for a validated node whose own
// content moved after its verdict, and for a validated node that relies on a
// target whose latest revision moved after the CONSUMER's verdict. Target
// currentness is left to SUPPORT_NOT_CURRENT; here the question is only whether
// the consumer's recorded verdict predates a revision it depends on.
func amendedNotReverified(in checksInput) []Finding {
	provider := support.ResultUseEdges(in.st, nil)
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		if n.VerdictSeq <= 0 {
			continue
		}
		if seq, ok := in.snap.latestRevisionSeq(n.ID); ok && seq > n.VerdictSeq {
			fs = append(fs, Finding{
				Code:     CodeAmendedNotReverified,
				Severity: SeverityError,
				Status:   StatusCurrent,
				Nodes:    []types.NodeID{n.ID},
				Seqs:     []int{seq},
				Message: fmt.Sprintf("node %s was amended at seq %d after its verdict at seq %d",
					n.ID.String(), seq, n.VerdictSeq),
				Remediation: RemediationFor(CodeAmendedNotReverified),
			})
		}
		targets, ok := provider.GetNodeDependencies(n.ID)
		if !ok {
			continue
		}
		for _, t := range sortedIDs(targets) {
			if in.st.GetNode(t) == nil {
				continue
			}
			seq, ok := in.snap.latestRevisionSeq(t)
			if !ok || seq <= n.VerdictSeq {
				continue
			}
			fs = append(fs, Finding{
				Code:     CodeAmendedNotReverified,
				Severity: SeverityError,
				Status:   StatusCurrent,
				Nodes:    nonZeroIDs(n.ID, t),
				Seqs:     []int{seq},
				Message: fmt.Sprintf("node %s relies on %s, which was amended at seq %d after the consumer's verdict at seq %d",
					n.ID.String(), t.String(), seq, n.VerdictSeq),
				Remediation: RemediationFor(CodeAmendedNotReverified),
			})
		}
	}
	return fs
}

// selfAccepts emits a current finding when a validated node's recorded verifier
// is one of the identities that had contributed to the accepted revision by the
// verdict sequence, from the ordered pass. Without a pass it falls back to the
// final-state identities. It only fires when both identities are recorded.
func selfAccepts(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated || n.ValidatedBy == "" {
			continue
		}
		contributors := contributorsFromState(n, in.snap)
		if in.pass != nil {
			if a, ok := in.pass.Acceptance(n.ID); ok {
				contributors = a.Contributors
			}
		}
		for _, c := range contributors {
			if c == "" || c != n.ValidatedBy {
				continue
			}
			fs = append(fs, Finding{
				Code:     CodeSelfAccept,
				Severity: SeverityError,
				Status:   StatusCurrent,
				Nodes:    []types.NodeID{n.ID},
				Message: fmt.Sprintf("verifier %s is also a recorded contributor (author/proof author/amender) of node %s at its verdict",
					n.ValidatedBy, n.ID.String()),
				Remediation: RemediationFor(CodeSelfAccept),
			})
			break
		}
	}
	return fs
}

// contributorsFromState is the state-only fallback for SELF_ACCEPT: the current
// author, proof author and amendment owners.
func contributorsFromState(n *node.Node, snap *snapshot) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(n.Author)
	add(n.ProofAuthor)
	for _, a := range snap.amendments[n.ID.String()] {
		add(a.Owner)
	}
	return out
}

// validatedWithBlockingChallenge emits a current finding for each validated
// node carrying an open critical/major challenge.
func validatedWithBlockingChallenge(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		blocking := in.snap.blockingByNode[n.ID.String()]
		if len(blocking) == 0 {
			continue
		}
		ids := make([]string, 0, len(blocking))
		var seqs []int
		for _, c := range blocking {
			ids = append(ids, c.ID)
			if c.Seq > 0 {
				seqs = append(seqs, c.Seq)
			}
		}
		sort.Strings(ids)
		sort.Ints(seqs)
		fs = append(fs, Finding{
			Code:     CodeValidatedWithOpenBlockingChallenge,
			Severity: SeverityError,
			Status:   StatusCurrent,
			Nodes:    []types.NodeID{n.ID},
			Seqs:     seqs,
			Message: fmt.Sprintf("validated node %s has %d open blocking challenge(s): %s",
				n.ID.String(), len(blocking), strings.Join(ids, ", ")),
			Remediation: RemediationFor(CodeValidatedWithOpenBlockingChallenge),
		})
	}
	return fs
}

// admittedNodes emits one historical, non-gating finding per admitted node.
func admittedNodes(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicAdmitted {
			continue
		}
		fs = append(fs, Finding{
			Code:        CodeAdmitted,
			Severity:    SeverityInfo,
			Status:      StatusHistorical,
			Nodes:       []types.NodeID{n.ID},
			Message:     fmt.Sprintf("node %s is admitted (cleared with taint)", n.ID.String()),
			Remediation: RemediationFor(CodeAdmitted),
		})
	}
	return fs
}

// archivedWithOpenChallenge emits a historical finding per archived node that
// abandoned an open challenge obligation. It prefers the durable
// abandoned_obligations snapshot recorded on the NodeArchived event (D9), then
// the ordered-pass open set at the archival sequence. The final challenge state
// is never consulted when a pass is available; the state-only compatibility
// path (no pass) falls back to the current open set.
func archivedWithOpenChallenge(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicArchived {
			continue
		}
		var obligations []types.NodeID
		archivalSeq := n.ArchivedSeq
		switch {
		case len(n.AbandonedObligations) > 0:
			obligations = parseNodeIDs(n.AbandonedObligations)
		case in.pass != nil:
			if a, ok := in.pass.Archival(n.ID); ok {
				obligations = a.OpenAtArchive
				if a.Seq > 0 {
					archivalSeq = a.Seq
				}
			}
		default:
			for _, d := range in.snap.openChallengesAt(n) {
				obligations = append(obligations, d.ID)
			}
		}
		obligations = dedupeSortedIDs(obligations)
		if len(obligations) == 0 {
			continue
		}
		findingNodes := append([]types.NodeID{n.ID}, obligations...)
		var seqs []int
		if archivalSeq > 0 {
			seqs = append(seqs, archivalSeq)
		}
		fs = append(fs, Finding{
			Code:     CodeArchivedWithOpenChallenge,
			Severity: SeverityWarning,
			Status:   StatusHistorical,
			Nodes:    findingNodes,
			Seqs:     seqs,
			Message: fmt.Sprintf("archived node %s has %d abandoned open challenge obligation(s) on itself or an active descendant: %s",
				n.ID.String(), len(obligations), joinNodeIDs(obligations)),
			Remediation: RemediationFor(CodeArchivedWithOpenChallenge),
		})
	}
	return fs
}

// amendmentHotspots emits historical, non-gating info findings for the nodes
// with the most recorded amendments.
func amendmentHotspots(in checksInput) []Finding {
	type hotspot struct {
		id  types.NodeID
		seq []int
	}
	var spots []hotspot
	for _, n := range in.snap.nodes {
		history := in.snap.amendments[n.ID.String()]
		if len(history) == 0 {
			continue
		}
		seqs := make([]int, 0, len(history))
		for _, a := range history {
			if a.Seq > 0 {
				seqs = append(seqs, a.Seq)
			}
		}
		sort.Ints(seqs)
		spots = append(spots, hotspot{id: n.ID, seq: seqs})
	}
	sort.SliceStable(spots, func(i, j int) bool {
		if len(spots[i].seq) != len(spots[j].seq) {
			return len(spots[i].seq) > len(spots[j].seq)
		}
		return spots[i].id.Less(spots[j].id)
	})
	if len(spots) > amendmentHotspotLimit {
		spots = spots[:amendmentHotspotLimit]
	}
	var fs []Finding
	for _, s := range spots {
		fs = append(fs, Finding{
			Code:        CodeAmendmentsPerNode,
			Severity:    SeverityInfo,
			Status:      StatusHistorical,
			Nodes:       []types.NodeID{s.id},
			Seqs:        s.seq,
			Message:     fmt.Sprintf("node %s has %d recorded amendment(s)", s.id.String(), len(s.seq)),
			Remediation: RemediationFor(CodeAmendmentsPerNode),
		})
	}
	return fs
}

// pendingExternalCitedByValidated emits a historical warning per validated node
// that cites an external reference. External verification is not implemented,
// so every cited external is pending.
func pendingExternalCitedByValidated(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		refs := citedExternalNames(in.st, n)
		if len(refs) == 0 {
			continue
		}
		fs = append(fs, Finding{
			Code:     CodePendingExternalCitedByValidated,
			Severity: SeverityWarning,
			Status:   StatusHistorical,
			Nodes:    []types.NodeID{n.ID},
			Message: fmt.Sprintf("validated node %s cites unverified external reference(s): %s",
				n.ID.String(), strings.Join(refs, ", ")),
			Remediation: RemediationFor(CodePendingExternalCitedByValidated),
		})
	}
	return fs
}

// unknownProvenance emits a historical info finding per validated node with no
// recorded verifier identity.
func unknownProvenance(in checksInput) []Finding {
	var fs []Finding
	for _, n := range in.snap.nodes {
		if n.EpistemicState != schema.EpistemicValidated || n.ValidatedBy != "" {
			continue
		}
		fs = append(fs, Finding{
			Code:        CodeUnknownProvenance,
			Severity:    SeverityInfo,
			Status:      StatusHistorical,
			Nodes:       []types.NodeID{n.ID},
			Message:     fmt.Sprintf("validated node %s has no recorded verifier identity", n.ID.String()),
			Remediation: RemediationFor(CodeUnknownProvenance),
		})
	}
	return fs
}

// citedExternalNames returns the external references a node cites, by name or
// ID, in stable order. It mirrors export's nodeExternalIDs: statement
// `external:NAME` citations resolved by name plus context entries that resolve
// to an external.
func citedExternalNames(st *state.State, n *node.Node) []string {
	seen := map[string]bool{}
	var out []string
	add := func(e *node.External) {
		if e == nil {
			return
		}
		key := e.Name
		if key == "" {
			key = e.ID
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, key)
	}
	for _, name := range lemma.ParseExtCitations(n.Statement) {
		add(st.GetExternalByName(name))
	}
	for _, ref := range n.Context {
		add(st.GetExternal(ref))
		add(st.GetExternalByName(ref))
	}
	sort.Strings(out)
	return out
}

// nonZeroIDs returns the distinct, non-empty IDs in argument order.
func nonZeroIDs(ids ...types.NodeID) []types.NodeID {
	out := make([]types.NodeID, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		s := id.String()
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, id)
	}
	return out
}

// dedupeSortedIDs returns the distinct IDs in stable hierarchical order.
func dedupeSortedIDs(ids []types.NodeID) []types.NodeID {
	out := make([]types.NodeID, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		s := id.String()
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out
}

// sortedIDs returns IDs in stable hierarchical order.
func sortedIDs(ids []types.NodeID) []types.NodeID {
	out := append([]types.NodeID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out
}

// shortHash abbreviates a content hash for messages.
func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

// joinNodeIDs renders node IDs for messages.
func joinNodeIDs(ids []types.NodeID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id.String())
	}
	return strings.Join(parts, ", ")
}
