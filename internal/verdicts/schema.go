// Package verdicts defines af's own batch verdict-file schema — the
// machine-readable document a verifier (or an orchestrating driver) submits
// to `af verdicts apply <file>` (rk PRD C3 / IMPLEMENTATION_PLAN.md item V2:
// batched verification as a native mode).
//
// This schema is versioned independently of rk's schemas/verdict.v1.json
// (drafted concurrently in the sibling ../rk repo as of 2026-07-19). The two
// are aligned IN SPIRIT — the same verdict vocabulary (accept/challenge), the
// same idea of a batch id shared across a whole verdict list, and the same
// "no blanket accepts" mandatory-justification rule — but af owns this
// schema and versions it on its own schedule. Do not assume wire-
// compatibility between the two files without checking both documents; see
// docs/verdicts-apply.md for the seam this creates for rk's M3.4 driver.
//
// This package is pure: it only parses and validates bytes. It has no
// dependency on the ledger, state, or service packages, so a malformed file
// can be diagnosed ("file-invalid") without touching a proof directory at
// all.
package verdicts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// CurrentSchemaVersion is the only schema_version value this build accepts.
// Bump alongside a docs/verdicts-apply.md update and a fixture when the wire
// format changes (rk CLAUDE.md L10's compat-event discipline, mirrored here
// even though af is not the rk repo: a verdict file is a cross-repo compat
// surface).
const CurrentSchemaVersion = "1"

// Verdict is the per-item disposition. There is no third option: a verdict
// file never encodes a blanket "accept everything," and it never encodes a
// no-op — every item is a decision.
type Verdict string

const (
	VerdictAccept    Verdict = "accept"
	VerdictChallenge Verdict = "challenge"
)

// Item is a single per-node verdict.
//
// Reason is MANDATORY for every item regardless of Verdict — this is the
// file-schema enforcement of PRD C3's "mandatory per-item justification (no
// blanket accepts)": an accept item with an empty Reason is rejected at
// parse time, exactly like an unjustified challenge would be. For a
// challenge item, Reason doubles as the challenge's reason text (the same
// field `af challenge --reason` already requires).
//
// Target, Severity, and Category apply to challenge items only. Validate
// rejects them on accept items — a mixed/ambiguous item is a file error, not
// a per-item runtime outcome, so it is caught here rather than surfacing as
// a confusing "rejected:*" result at apply time.
type Item struct {
	Node     string  `json:"node"`
	Verdict  Verdict `json:"verdict"`
	Reason   string  `json:"reason"`
	Target   string  `json:"target,omitempty"`
	Severity string  `json:"severity,omitempty"`
	Category string  `json:"category,omitempty"`
	// ExpectHash is the node content hash this verdict was authored against
	// (rk B1). Optional and additive: when non-empty, `af verdicts apply`
	// re-reads the node under its own state read and REJECTS the item if the
	// node's current content hash differs (the node was edited between the
	// verifier's dispatch and this apply) or, for an accept, if the node is no
	// longer verifier-ready (workflow_state != available). This makes the
	// readiness/hash re-check atomic in af's kernel rather than a racy
	// second export on the driver side. An empty ExpectHash preserves the
	// legacy behavior (no hash/availability gate).
	ExpectHash string `json:"expect_hash,omitempty"`
}

// File is the top-level verdict-file document accepted by
// `af verdicts apply <file>`.
//
// BatchID and VerifiedBy are both mandatory: every event this file produces
// carries BatchID (so `af unvalidate --batch` can later find them), and
// VerifiedBy is the verifier identity checked against each node's recorded
// Author for the reviewer≠author rule (PRD C3) — driver-supplied provenance,
// recorded-and-checkable, not adversary-proof, same caveat as the rest of
// V1's identity fields.
//
// Items is order-significant: `af verdicts apply` applies items in file
// order, and that order is what makes "children before parent" accepts
// possible. The engine does not reorder the file.
type File struct {
	SchemaVersion string `json:"schema_version"`
	BatchID       string `json:"batch_id"`
	VerifiedBy    string `json:"verified_by"`
	Items         []Item `json:"items"`
}

// ParseFile decodes and validates raw verdict-file bytes. Any failure
// returned here means "file-invalid": af verdicts apply must not attempt to
// apply anything from a file that fails ParseFile — no partial reading of a
// broken file.
//
// Unknown JSON fields are rejected (DisallowUnknownFields): a typo'd field
// name (e.g. "resaon") is a schema violation, not a silently-ignored no-op.
func ParseFile(data []byte) (*File, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("verdict file is not valid JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("verdict file contains trailing content after the JSON document")
	}

	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Validate checks every structural invariant of the file. It also applies
// defaults to challenge items (empty Target -> "statement", empty Severity
// -> "major", mirroring af challenge's own CLI defaults) so callers never
// see an empty Target/Severity on a challenge item that passed Validate.
func (f *File) Validate() error {
	if f.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q (expected %q)", f.SchemaVersion, CurrentSchemaVersion)
	}
	// batch_id is REQUIRED for a multi-item (genuine batch) file: a batch needs
	// a shared id so `af unvalidate --batch <id>` can revoke it as a unit. It is
	// OPTIONAL for a single-item, non-batch apply (rk's per-node verdict): an
	// absent/empty batch_id then records NO batch provenance on the node (the
	// event's BatchID and the node's ValidationBatchID are both omitempty), so a
	// per-node apply never carries a batch id that would trip rk's critical-path
	// Check 13. rk-qxp / rk-mq2.
	if strings.TrimSpace(f.BatchID) == "" && len(f.Items) > 1 {
		return fmt.Errorf("batch_id is required for a multi-item verdict file (a single-item non-batch apply may omit it)")
	}
	if strings.TrimSpace(f.VerifiedBy) == "" {
		return fmt.Errorf("verified_by is required")
	}
	if len(f.Items) == 0 {
		return fmt.Errorf("items must be non-empty")
	}

	seen := make(map[string]int, len(f.Items))
	for i := range f.Items {
		if err := f.Items[i].validate(); err != nil {
			return fmt.Errorf("item %d (node %q): %w", i, f.Items[i].Node, err)
		}
		if prev, ok := seen[f.Items[i].Node]; ok {
			return fmt.Errorf("item %d: node %q already has a verdict at item %d in this file — one verdict per node per batch", i, f.Items[i].Node, prev)
		}
		seen[f.Items[i].Node] = i
	}
	return nil
}

// validate checks a single item and fills in challenge defaults. Pointer
// receiver: called as f.Items[i].validate() so defaults are written back
// into the file's own slice.
func (it *Item) validate() error {
	if strings.TrimSpace(it.Node) == "" {
		return fmt.Errorf("node id is required")
	}
	if _, err := types.Parse(it.Node); err != nil {
		return fmt.Errorf("invalid node id %q: %w", it.Node, err)
	}

	if it.Verdict != VerdictAccept && it.Verdict != VerdictChallenge {
		return fmt.Errorf("verdict must be %q or %q, got %q", VerdictAccept, VerdictChallenge, it.Verdict)
	}

	// Mandatory per-item justification: PRD C3, "no blanket accepts."
	if strings.TrimSpace(it.Reason) == "" {
		return fmt.Errorf("reason (justification) is required for every item, including accepts")
	}

	switch it.Verdict {
	case VerdictAccept:
		if it.Target != "" || it.Severity != "" || it.Category != "" {
			return fmt.Errorf("target/severity/category are challenge-only fields and must be empty for an accept item")
		}
	case VerdictChallenge:
		if it.Target == "" {
			it.Target = string(schema.TargetStatement)
		} else if err := schema.ValidateChallengeTarget(it.Target); err != nil {
			return err
		}
		if it.Severity == "" {
			it.Severity = string(schema.SeverityMajor)
		} else if err := schema.ValidateChallengeSeverity(it.Severity); err != nil {
			return err
		}
		if err := schema.ValidateChallengeCategory(it.Category); err != nil {
			return err
		}
	}
	return nil
}
