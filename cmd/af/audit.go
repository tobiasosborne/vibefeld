// Package main contains the af audit command: a read-only trust audit over the
// current workspace, backed by internal/audit.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/audit"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// newAuditCmd creates the audit command.
func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "audit",
		GroupID: GroupAdmin,
		Short:   "Audit the proof for trust gaps (read-only)",
		Long: `Audit the proof for trust gaps, computed from an ordered ledger pass
plus one immutable snapshot of derived state.

The audit is read-only: it appends no events and takes no ledger lock. Findings
carry a stable code, a severity (error/warning/info), a current or historical
status, the node IDs and ledger sequences involved, and a remediation.

Current findings fail --strict; historical findings are always reported and
never gate.

Strict-current codes:
  SUPPORT_NOT_CURRENT                    a validated node's verdict is no longer
                                         supported (cause + responsible node)
  HASH_MISMATCH                          the validated content hash differs from
                                         the current content hash
  CYCLE                                  a legacy result-use cycle
  SCOPE_LEAK                             a citation escapes its local assumption
  CITES_SEVERED                          a dependency on an archived, refuted or
                                         missing target
  AMENDED_NOT_REVERIFIED                 a validated node (or a target it relies
                                         on) moved after its verdict
  SELF_ACCEPT                            verifier equals author/amender
  VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE a validated node is still challenged

Historical (never gating) codes:
  ADMITTED, ARCHIVED_WITH_OPEN_CHALLENGE, AMENDMENTS_PER_NODE,
  PENDING_EXTERNAL_CITED_BY_VALIDATED, UNKNOWN_PROVENANCE, and hash checks that
  were never recorded.

Exit codes:
  0  audit ran (always, without --strict; with --strict when it passed)
  3  --strict and a strict-current finding exists (AUDIT_FAILED)

Examples:
  af audit                       Print findings grouped by code
  af audit --strict              Exit 3 if any current finding gates
  af audit -f json               Machine-readable report (schema_version 1)
  af audit --code CYCLE,SCOPE_LEAK
  af audit --node 1.2 --status current
  af audit --limit 50`,
		RunE: runAudit,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	cmd.Flags().Bool("strict", false, "Exit 3 when a current finding would gate")
	cmd.Flags().StringSlice("code", nil, "Only findings with these codes (comma-separated)")
	cmd.Flags().String("node", "", "Only findings involving a node ID with this prefix")
	cmd.Flags().String("status", "", "Only findings with this status (current or historical)")
	cmd.Flags().Int("limit", audit.DefaultLimit, "Maximum findings to print (0 = unlimited)")

	return cmd
}

func runAudit(cmd *cobra.Command, _ []string) error {
	dir := service.MustString(cmd, "dir")
	format := strings.ToLower(service.MustString(cmd, "format"))
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	status := strings.ToLower(strings.TrimSpace(service.MustString(cmd, "status")))
	if status != "" && status != audit.StatusCurrent && status != audit.StatusHistorical {
		return fmt.Errorf("invalid status %q: must be 'current' or 'historical'", status)
	}

	limit := service.MustInt(cmd, "limit")
	if limit < 0 {
		return fmt.Errorf("invalid limit %d: must be non-negative", limit)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// One ledger replay supplies both the final state and the ordered pass, so
	// the audit never loads state twice and the sequence-sensitive checks agree
	// with the state they run on.
	st, pass, err := svc.LoadStateWithPass()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}
	if st.LatestSeq() == 0 {
		return fmt.Errorf("proof not initialized")
	}

	report := audit.RunWithPass(st, pass, audit.Options{
		Strict:     service.MustBool(cmd, "strict"),
		Codes:      service.MustStringSlice(cmd, "code"),
		NodePrefix: service.MustString(cmd, "node"),
		Status:     status,
		Limit:      limit,
	})

	if format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), renderAuditText(report))
	}

	if report.Strict && !report.Passed {
		return aferrors.Newf(aferrors.AUDIT_FAILED,
			"audit --strict found %d current finding(s) that gate", report.Summary.StrictCurrent)
	}
	return nil
}

// renderAuditText renders the report grouped by code with counts and
// remediation.
func renderAuditText(report audit.Report) string {
	var sb strings.Builder
	sb.WriteString("Audit report")
	if report.Strict {
		if report.Passed {
			sb.WriteString(" [strict: PASS]")
		} else {
			sb.WriteString(" [strict: FAIL]")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "%d finding(s) (%d current, %d historical; %d strict-current)\n\n",
		report.Summary.Total, report.Summary.Current, report.Summary.Historical, report.Summary.StrictCurrent)

	grouped := map[string][]audit.Finding{}
	for _, f := range report.Findings {
		grouped[f.Code] = append(grouped[f.Code], f)
	}
	codes := make([]string, 0, len(grouped))
	for code := range grouped {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	if len(codes) == 0 {
		sb.WriteString("No findings for this filter.\n")
		return sb.String()
	}

	for _, code := range codes {
		findings := grouped[code]
		current := 0
		for _, f := range findings {
			if f.Status == audit.StatusCurrent {
				current++
			}
		}
		fmt.Fprintf(&sb, "%s (%d shown; %d current)\n", code, len(findings), current)
		if rem := audit.RemediationFor(code); rem != "" {
			fmt.Fprintf(&sb, "  remediation: %s\n", rem)
		}
		for _, f := range findings {
			fmt.Fprintf(&sb, "  - [%s/%s] %s\n", f.Severity, f.Status, f.Message)
			if len(f.Nodes) > 0 {
				fmt.Fprintf(&sb, "    nodes: %s\n", joinNodeIDs(f.Nodes))
			}
			if len(f.Seqs) > 0 {
				fmt.Fprintf(&sb, "    seqs: %v\n", f.Seqs)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func joinNodeIDs(ids []types.NodeID) string {
	parts := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		s := id.String()
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func init() {
	rootCmd.AddCommand(newAuditCmd())
}
