// Package audit computes the read-only trust audit for a proof workspace. The
// sequence-sensitive checks (accepted content hash, acceptance-time
// contributors, open challenges at an archival) run over an ordered ledger pass
// built by BuildPass; the rest run over one immutable snapshot of derived
// state. It is the single findings engine shared by `af audit`, `af health` and
// the `af amend-deps` migration preflight/postcheck.
//
// Findings carry a stable code, a severity, a current/historical status, the
// node IDs and ledger sequences involved, a message and a remediation. Only
// current findings whose code is in strictCurrentCodes fail `af audit --strict`;
// historical classes are reported and never gate. `af audit` itself performs no
// ledger writes and takes no lock.
package audit

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// SchemaVersion is the audit report schema version.
const SchemaVersion = 1

// DefaultLimit is the default number of findings `af audit` prints. The engine
// itself does not bound its result; the CLI applies this bound.
const DefaultLimit = 200

// Finding is one audit finding.
type Finding struct {
	Code        string         `json:"code"`
	Severity    string         `json:"severity"`
	Status      string         `json:"status"`
	Nodes       []types.NodeID `json:"nodes,omitempty"`
	Seqs        []int          `json:"seqs,omitempty"`
	Cause       string         `json:"cause,omitempty"`
	Message     string         `json:"message"`
	Remediation string         `json:"remediation,omitempty"`
}

// Summary counts findings. It is computed over the filtered set before the
// output limit is applied, so counts are never truncated even when the printed
// finding list is.
type Summary struct {
	Total         int            `json:"total"`
	Current       int            `json:"current"`
	Historical    int            `json:"historical"`
	StrictCurrent int            `json:"strict_current"`
	BySeverity    map[string]int `json:"by_severity,omitempty"`
	ByCode        map[string]int `json:"by_code,omitempty"`
}

// Report is the machine-readable audit result.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Findings      []Finding `json:"findings"`
	Summary       Summary   `json:"summary"`
	Strict        bool      `json:"strict"`
	Passed        bool      `json:"passed"`
}

// Options controls filtering, strictness and output bounding.
type Options struct {
	// Strict marks the report strict. Passed is true when Strict is false or
	// when no current finding has a strict-current code.
	Strict bool
	// Codes filters to the given codes (case-insensitive). Empty means all.
	Codes []string
	// NodePrefix filters to findings that involve a node ID at or below the
	// prefix (dotted-segment aware). Empty means all.
	NodePrefix string
	// Status filters to "current" or "historical". Empty means both.
	Status string
	// Limit bounds Report.Findings after filtering. Zero means no limit; the
	// CLI default is DefaultLimit.
	Limit int
}

// Run computes the audit for st from derived state alone. Sequence-sensitive
// checks fall back to the best reconstruction available in final state. It
// performs no I/O and never fails; a nil state produces an empty, passing
// report.
func Run(st *state.State, opts Options) Report {
	return run(st, nil, opts)
}

// RunWithPass computes the audit using the ordered ledger pass for the
// sequence-sensitive checks: the acceptance-time content hash and
// contributors, and the open challenges at each archival sequence. It is the
// path `af audit` and the amend-deps summary take.
func RunWithPass(st *state.State, pass *Pass, opts Options) Report {
	return run(st, pass, opts)
}

func run(st *state.State, pass *Pass, opts Options) Report {
	findings := computeFindings(st, pass, codeSelector(opts.Codes))
	findings = applyFilters(findings, opts)

	summary := summarize(findings)
	passed := !opts.Strict || summary.StrictCurrent == 0

	findings = applyLimit(findings, opts.Limit)
	if findings == nil {
		findings = []Finding{}
	}

	return Report{
		SchemaVersion: SchemaVersion,
		Findings:      findings,
		Summary:       summary,
		Strict:        opts.Strict,
		Passed:        passed,
	}
}

// codeSelector returns the producer gate for an options code filter. An empty
// (or all-blank) filter runs every producer.
func codeSelector(codes []string) func(code string) bool {
	set := map[string]bool{}
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" {
			set[c] = true
		}
	}
	if len(set) == 0 {
		return func(string) bool { return true }
	}
	return func(code string) bool { return set[code] }
}

// IsStrictCurrent reports whether a finding would fail --strict.
func IsStrictCurrent(f Finding) bool {
	return f.Status == StatusCurrent && strictCurrentCodes[f.Code]
}

// RemediationFor returns the remediation text for a code.
func RemediationFor(code string) string {
	return remediation[code]
}

// summarize computes the summary counts over findings.
func summarize(findings []Finding) Summary {
	s := Summary{
		Total:      len(findings),
		BySeverity: map[string]int{},
		ByCode:     map[string]int{},
	}
	for _, f := range findings {
		s.ByCode[f.Code]++
		s.BySeverity[f.Severity]++
		switch f.Status {
		case StatusCurrent:
			s.Current++
		case StatusHistorical:
			s.Historical++
		}
		if IsStrictCurrent(f) {
			s.StrictCurrent++
		}
	}
	return s
}

// applyFilters returns the findings matching code, node-prefix and status.
func applyFilters(findings []Finding, opts Options) []Finding {
	codes := map[string]bool{}
	for _, c := range opts.Codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" {
			codes[c] = true
		}
	}
	status := strings.ToLower(strings.TrimSpace(opts.Status))

	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if len(codes) > 0 && !codes[strings.ToUpper(f.Code)] {
			continue
		}
		if status != "" && strings.ToLower(f.Status) != status {
			continue
		}
		if opts.NodePrefix != "" && !findingMatchesNodePrefix(f, opts.NodePrefix) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// findingMatchesNodePrefix reports whether any node in the finding is at or
// below prefix. Matching is dotted-segment aware, so "1.2" matches "1.2" and
// "1.2.3" but not "1.20".
func findingMatchesNodePrefix(f Finding, prefix string) bool {
	for _, n := range f.Nodes {
		s := n.String()
		if s == prefix || strings.HasPrefix(s, prefix+".") {
			return true
		}
	}
	return false
}

// applyLimit bounds the finding list. A limit <= 0 means no limit.
func applyLimit(findings []Finding, limit int) []Finding {
	if limit <= 0 || limit >= len(findings) {
		return findings
	}
	return findings[:limit]
}

// sortedNodes returns the state's nodes in stable hierarchical-ID order.
func sortedNodes(st *state.State) []*node.Node {
	if st == nil {
		return nil
	}
	nodes := st.AllNodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID.Less(nodes[j].ID) })
	return nodes
}

// seedFindings sorts findings deterministically: by code, then first node ID,
// then the full node list, then sequences, then message. It also normalizes
// every finding's sequence list into ascending order so output is
// order-independent of the producer's internal iteration.
func seedFindings(findings []Finding) []Finding {
	for i := range findings {
		if len(findings[i].Seqs) > 1 {
			sort.Ints(findings[i].Seqs)
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		a, b := nodeKey(findings[i]), nodeKey(findings[j])
		if a != b {
			return a < b
		}
		if c, d := seqKey(findings[i]), seqKey(findings[j]); c != d {
			return c < d
		}
		return findings[i].Message < findings[j].Message
	})
	return findings
}

// nodeKey renders a finding's nodes as a stable comparison key.
func nodeKey(f Finding) string {
	parts := make([]string, 0, len(f.Nodes))
	for _, n := range f.Nodes {
		parts = append(parts, n.String())
	}
	return strings.Join(parts, "|")
}

// seqKey renders a finding's sequences as a stable comparison key.
func seqKey(f Finding) string {
	parts := make([]string, 0, len(f.Seqs))
	for _, s := range f.Seqs {
		parts = append(parts, strconv.Itoa(s))
	}
	return strings.Join(parts, ",")
}
