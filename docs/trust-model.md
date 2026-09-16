# Trust Model and Known Gaps

*Recorded 2026-09-02 while fixing the 0.1.7 taint bug. Read this before
relying on an af proof you have not personally re-read.*

af's promise is that when the root is `validated` with taint `clean`, every
step was accepted by a verifier and nothing was taken on faith. The ledger
records everything agents did, so any gap below can be **audited after the
fact**; the point of this page is to say where an agent can "win" without
rigor so that audits look in the right places, and to track which gaps are
closed structurally.

## Closed

| Gap | Closed in | How |
|-----|-----------|-----|
| Admitted or pending descendants left a validated root `clean` | 0.1.7 | Taint now propagates upward; replay recomputes taint authoritatively, so old workspaces self-heal on load |
| A reopened (`needs_refinement`) node left the root `clean` | 0.1.7 | `needs_refinement` counts as unresolved for the node, its ancestors, and its descendants |
| Statement changed after acceptance | always | `af amend` only works on `pending` nodes |

## Open (ordered by how easily an agent can exploit them)

1. **Archive-the-hard-step.** `af accept` requires every child cleared, and
   `archived` counts as cleared. A prover facing an unanswerable challenge can
   archive that child; the parent becomes acceptable with one fewer step, and
   archived branches are clean by design. `af archive` does not check for
   open challenges. *Audit:* `af audit` reports `ARCHIVED_WITH_OPEN_CHALLENGE`
   (historical, never gating) for archived nodes with an open challenge on the
   node or an active descendant. D9's archival obligations are not on this
   branch, so this is derived from current state rather than the challenge set
   at archival time. *Fix candidates:* refuse archive while a challenge is open
   unless `--force` with a reason; flag "parent accepted after child
   archived" in `af health`. Tracked: vibefeld-a7p5.
2. **Cross-references do not carry taint.** Taint flows along the tree only.
   A node's reference `dependencies` and external references are not
   consulted, so lemma A can cite an admitted lemma B and stay `clean`. This
   matters for DAG-shaped arguments (many lemmas citing each other) far more
   than for tree-shaped ones. *Audit:* `af audit` reports `CITES_SEVERED`
   (missing/archived/refuted dependency targets), `SUPPORT_NOT_CURRENT`
   (`TARGET_PENDING`, `TARGET_REFUTED`, `TARGET_REVISED` causes) and
   `PENDING_EXTERNAL_CITED_BY_VALIDATED`; `af pending-refs` lists the pending
   externals. *Fix:* include dependency targets in the down-component of taint
   (the cycle package already guarantees a DAG). Tracked: vibefeld-0ry1.
3. **Roles are convention, not enforcement.** Nothing stops one process from
   calling `refine` and `accept` on the same node. Author and verifier
   identities are recorded (0.1.6) but `accept` does not refuse when they
   match; `resolve-challenge` is a prover action. *Audit:* `af audit` reports
   `SELF_ACCEPT` (verifier equals author/proof author/amender), `UNKNOWN_PROVENANCE`
   (validated with no recorded verifier) and `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE`.
   *Fix:* `accept` refuses self-acceptance unless `--allow-self` (for
   single-agent use). Tracked: vibefeld-gwps.
4. **Ledger is append-only but not tamper-evident.** Node content is hashed
   but events are not hash-chained, so an agent with shell access to the
   workspace could rewrite history without replay noticing. Relevant when
   agents have write access beyond the `af` binary. *Audit:* `af audit` reports
   `HASH_MISMATCH` for a validated node whose current content hash differs from
   the hash recorded at acceptance, but that detects content moving after
   acceptance, not a coordinated ledger rewrite. *Fix:* per-event hash
   chain, verified by `af replay --verify`. Belongs to the v0.2 kernel work
   (docs/prd.md, "v0.2 Target"). Tracked: vibefeld-8x16.

## Audit

`af audit` is the read-only audit these gaps point to (0.1.10). It is computed
from one ordered pass over derived state, appends no events and takes no lock.
It reports stable codes with a `current | historical` status; only current
findings with a strict code fail `af audit --strict` (exit 3, `AUDIT_FAILED`).

| Gap | Finding codes |
|-----|---------------|
| Archive-the-hard-step | `ARCHIVED_WITH_OPEN_CHALLENGE` (historical) |
| Cross-references do not carry taint | `CITES_SEVERED`, `SUPPORT_NOT_CURRENT`, `PENDING_EXTERNAL_CITED_BY_VALIDATED` |
| Roles are convention, not enforcement | `SELF_ACCEPT`, `UNKNOWN_PROVENANCE`, `VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE` |
| Ledger is append-only but not tamper-evident | `HASH_MISMATCH` (content moved after acceptance only) |

`af health` reads its `support_current` blockers from the same findings engine,
and `af amend-deps --file` runs the audit as a migration preflight/postcheck
(`--dry-run` and the real run both end with a one-line summary and the exact
`af audit --strict` command). Tracked: vibefeld-5qrx.
