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

// computeFindings runs the ordered audit pass over derived state.
func computeFindings(st *state.State) []Finding {
	if st == nil {
		return nil
	}
	var fs []Finding
	fs = append(fs, supportNotCurrent(st)...)
	fs = append(fs, hashMismatch(st)...)
	fs = append(fs, cycles(st)...)
	fs = append(fs, scopeLeaks(st)...)
	fs = append(fs, citesSevered(st)...)
	fs = append(fs, amendedNotReverified(st)...)
	fs = append(fs, selfAccepts(st)...)
	fs = append(fs, validatedWithBlockingChallenge(st)...)
	fs = append(fs, admittedNodes(st)...)
	fs = append(fs, archivedWithOpenChallenge(st)...)
	fs = append(fs, amendmentHotspots(st)...)
	fs = append(fs, pendingExternalCitedByValidated(st)...)
	fs = append(fs, unknownProvenance(st)...)
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

// hashMismatch compares each validated node's current content hash with the
// hash recorded at acceptance. A recorded mismatch is a current, strict
// finding; an unrecorded hash yields a historical "reconstructed" note that
// never gates (D3).
func hashMismatch(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		if n.ValidatedContentHash == "" {
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
		if n.ContentHash != n.ValidatedContentHash {
			fs = append(fs, Finding{
				Code:     CodeHashMismatch,
				Severity: SeverityError,
				Status:   StatusCurrent,
				Nodes:    []types.NodeID{n.ID},
				Message: fmt.Sprintf("validated hash %s does not match current content hash %s for node %s",
					shortHash(n.ValidatedContentHash), shortHash(n.ContentHash), n.ID.String()),
				Remediation: RemediationFor(CodeHashMismatch),
			})
		}
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
// target whose content moved after the target's own verdict.
func amendedNotReverified(st *state.State) []Finding {
	provider := support.ResultUseEdges(st, nil)
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		if seq, ok := latestAmendmentSeq(st, n.ID); ok && seq > n.VerdictSeq {
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
			tn := st.GetNode(t)
			if tn == nil {
				continue
			}
			seq, ok := latestAmendmentSeq(st, t)
			if !ok || seq <= tn.VerdictSeq {
				continue
			}
			fs = append(fs, Finding{
				Code:     CodeAmendedNotReverified,
				Severity: SeverityError,
				Status:   StatusCurrent,
				Nodes:    nonZeroIDs(n.ID, t),
				Seqs:     []int{seq},
				Message: fmt.Sprintf("node %s relies on %s, which was amended at seq %d after its own verdict at seq %d",
					n.ID.String(), t.String(), seq, tn.VerdictSeq),
				Remediation: RemediationFor(CodeAmendedNotReverified),
			})
		}
	}
	return fs
}

// selfAccepts emits a current finding when a validated node's recorded verifier
// matches the node's author, proof author or any amendment owner. It only fires
// when both identities are recorded.
func selfAccepts(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated || n.ValidatedBy == "" {
			continue
		}
		contributors := []string{n.Author, n.ProofAuthor}
		for _, a := range st.GetAmendmentHistory(n.ID) {
			contributors = append(contributors, a.Owner)
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
				Message: fmt.Sprintf("verifier %s is also a recorded contributor (author/proof author/amender) of node %s",
					n.ValidatedBy, n.ID.String()),
				Remediation: RemediationFor(CodeSelfAccept),
			})
			break
		}
	}
	return fs
}

// validatedWithBlockingChallenge emits a current finding for each validated
// node carrying an open critical/major challenge.
func validatedWithBlockingChallenge(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		blocking := st.GetBlockingChallengesForNode(n.ID)
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
func admittedNodes(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
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
// has an open challenge on itself or on an active (non-severed) descendant. The
// D9 obligations helper is not on this branch, so the check is derived from
// current state; it is reported, never gating.
func archivedWithOpenChallenge(st *state.State) []Finding {
	var fs []Finding
	nodes := sortedNodes(st)
	for _, n := range nodes {
		if n.EpistemicState != schema.EpistemicArchived {
			continue
		}
		var challengeNodes []types.NodeID
		var challengeIDs []string
		add := func(target types.NodeID) {
			for _, c := range openChallengesFor(st, target) {
				challengeIDs = append(challengeIDs, c.ID)
				challengeNodes = append(challengeNodes, target)
			}
		}
		add(n.ID)
		for _, d := range nodes {
			if d.ID.Equal(n.ID) || !isDescendant(d.ID, n.ID) {
				continue
			}
			if d.EpistemicState == schema.EpistemicArchived || d.EpistemicState == schema.EpistemicRefuted {
				continue
			}
			add(d.ID)
		}
		if len(challengeIDs) == 0 {
			continue
		}
		sort.Strings(challengeIDs)
		findingNodes := append([]types.NodeID{n.ID}, challengeNodes...)
		fs = append(fs, Finding{
			Code:     CodeArchivedWithOpenChallenge,
			Severity: SeverityWarning,
			Status:   StatusHistorical,
			Nodes:    findingNodes,
			Message: fmt.Sprintf("archived node %s has %d open challenge(s) on itself or an active descendant: %s",
				n.ID.String(), len(challengeIDs), strings.Join(challengeIDs, ", ")),
			Remediation: RemediationFor(CodeArchivedWithOpenChallenge),
		})
	}
	return fs
}

// amendmentHotspots emits historical, non-gating info findings for the nodes
// with the most recorded amendments.
func amendmentHotspots(st *state.State) []Finding {
	type hotspot struct {
		id  types.NodeID
		seq []int
	}
	var spots []hotspot
	for _, n := range sortedNodes(st) {
		history := st.GetAmendmentHistory(n.ID)
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
func pendingExternalCitedByValidated(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
		if n.EpistemicState != schema.EpistemicValidated {
			continue
		}
		refs := citedExternalNames(st, n)
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
func unknownProvenance(st *state.State) []Finding {
	var fs []Finding
	for _, n := range sortedNodes(st) {
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

// openChallengesFor returns the open challenges on one node.
func openChallengesFor(st *state.State, id types.NodeID) []*state.Challenge {
	var out []*state.Challenge
	for _, c := range st.GetChallengesForNode(id) {
		if c.Status == state.ChallengeStatusOpen {
			out = append(out, c)
		}
	}
	return out
}

// isDescendant reports whether child is strictly below ancestor.
func isDescendant(child, ancestor types.NodeID) bool {
	return strings.HasPrefix(child.String(), ancestor.String()+".")
}

// latestAmendmentSeq returns the latest amendment sequence recorded for a node.
func latestAmendmentSeq(st *state.State, id types.NodeID) (int, bool) {
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
