package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/audit"
	"github.com/tobiasosborne/vibefeld/internal/service"
)

// analyzeSupportHealth appends D4's support_current failures to a health report:
// every validated or admitted node whose derived support is no longer current,
// grouped by stable cause code and naming the responsible node (and revision
// sequence where one applies). The findings come from the one shared audit
// engine (internal/audit, code SUPPORT_NOT_CURRENT) rather than a second
// support.Current call, so health and audit cannot disagree. It never leaves the
// status healthier than warning. Kept in its own file so the rest of health's
// analysis is untouched.
func analyzeSupportHealth(st *service.State, status string, blockers []Blocker) (string, []Blocker) {
	if st == nil {
		return status, blockers
	}
	// Only the two producers health reads run: SUPPORT_NOT_CURRENT for the
	// blockers below, and VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE so the open
	// challenge producer shares this one snapshot. Every other producer is
	// skipped before computation.
	report := audit.Run(st, audit.Options{Codes: []string{
		audit.CodeSupportNotCurrent,
		audit.CodeValidatedWithOpenBlockingChallenge,
	}})

	type failure struct {
		node        string
		responsible string
		seq         int
	}
	byCause := make(map[string][]failure)
	for _, f := range report.Findings {
		if f.Code != audit.CodeSupportNotCurrent {
			continue
		}
		if len(f.Nodes) == 0 {
			continue
		}
		responsible := ""
		if len(f.Nodes) > 1 {
			responsible = f.Nodes[1].String()
		}
		seq := 0
		if len(f.Seqs) > 0 {
			seq = f.Seqs[0]
		}
		byCause[f.Cause] = append(byCause[f.Cause], failure{
			node:        f.Nodes[0].String(),
			responsible: responsible,
			seq:         seq,
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
