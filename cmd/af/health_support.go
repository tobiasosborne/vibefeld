package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/support"
)

// analyzeSupportHealth appends D4's support_current failures to a health report:
// every validated or admitted node whose derived support is no longer current,
// grouped by stable cause code and naming the responsible node (and revision
// sequence where one applies). It never leaves the status healthier than
// warning. Kept in its own file so the rest of health's analysis is untouched.
func analyzeSupportHealth(st *service.State, status string, blockers []Blocker) (string, []Blocker) {
	if st == nil {
		return status, blockers
	}
	statuses := support.Current(st)

	type failure struct {
		node        string
		responsible string
		seq         int
	}
	byCause := make(map[string][]failure)
	for _, n := range st.AllNodes() {
		if n.EpistemicState != schema.EpistemicValidated && n.EpistemicState != schema.EpistemicAdmitted {
			continue
		}
		sup := statuses[n.ID.String()]
		if sup.Current || sup.Cause == "" {
			continue
		}
		byCause[sup.Cause] = append(byCause[sup.Cause], failure{
			node:        n.ID.String(),
			responsible: sup.Node.String(),
			seq:         sup.Seq,
		})
	}
	if len(byCause) == 0 {
		return status, blockers
	}

	causes := make([]string, 0, len(byCause))
	for cause := range byCause {
		causes = append(causes, cause)
	}
	sort.Strings(causes)

	for _, cause := range causes {
		failures := byCause[cause]
		sort.Slice(failures, func(i, j int) bool { return failures[i].node < failures[j].node })
		var ids, details []string
		seen := make(map[string]bool, len(failures))
		for _, f := range failures {
			if !seen[f.node] {
				ids = append(ids, f.node)
				seen[f.node] = true
			}
			detail := fmt.Sprintf("%s via %s", f.node, f.responsible)
			if f.seq > 0 {
				detail += fmt.Sprintf(" (seq %d)", f.seq)
			}
			details = append(details, detail)
		}
		blockers = append(blockers, Blocker{
			Type: "support_not_current_" + cause,
			Message: fmt.Sprintf("%d validated node(s) have support_current false (%s): %s",
				len(failures), cause, strings.Join(details, ", ")),
			Suggestion: "Re-verify the named nodes against the current dependencies (af request-refinement then re-accept), or restore the dependency revision",
			NodeIDs:    ids,
		})
	}

	if status == HealthStatusHealthy {
		status = HealthStatusWarning
	}
	return status, blockers
}
