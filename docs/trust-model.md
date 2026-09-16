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
   archived branches are clean by design. *Audit:* archived nodes that had an
   open challenge at archival time. *Fix candidates:* refuse archive while a
   challenge is open unless `--force` with a reason; flag "parent accepted after
   child archived" in `af health`. Tracked: vibefeld-a7p5.

   *Narrowed in 0.1.10:* `af archive` now refuses when a challenge is open on
   the node or on an active (non-severed) descendant, unless `--force --reason`
   is given; `NodeArchived` records `reason` and `forced`, and the parent's
   verification checklist lists children archived with a challenge open so the
   next accept acknowledges the abandoned obligation. This prevents the silent
   version and leaves an attributed audit trail. It does **not** prove the
   obligation was discharged, that a forced archive was justified, or that the
   accepting verifier read the checklist: `--force`, the reason text and the
   accept are all recorded provenance, not enforcement of rigor.
2. **Cross-references do not carry taint.** Taint flows along the tree only.
   A node's reference `dependencies` and external references are not
   consulted, so lemma A can cite an admitted lemma B and stay `clean`. This
   matters for DAG-shaped arguments (many lemmas citing each other) far more
   than for tree-shaped ones. *Audit:* nodes whose dependencies include an
   admitted/tainted/pending node; `af pending-refs`. *Fix:* include
   dependency targets in the down-component of taint (the cycle package
   already guarantees a DAG). Tracked: vibefeld-0ry1.
3. **Roles are convention, not enforcement.** Nothing stops one process from
   calling `refine` and `accept` on the same node. Author and verifier
   identities are recorded (0.1.6) but `accept` does not refuse when they
   match; `resolve-challenge` is a prover action. *Audit:* accepts where
   verifier == author; accepts seconds after a resolve with no new challenge.
   *Fix:* `accept` refuses self-acceptance unless `--allow-self` (for
   single-agent use). Tracked: vibefeld-gwps.

   *Narrowed in 0.1.10:* with an `--agent`/`AF_AGENT_ID` identity, the shared
   accept validator refuses when the verifier is recorded as the node's author,
   its proof author (`af record-proof`), or an owner in its amendment history;
   `--allow-self` overrides and records `self_accepted: true`. Verdict files run
   the same check and cannot opt out, and from 0.1.11 the identity is required.
   This is **recorded provenance, not proof of independence**: the identity
   strings are driver-supplied, so it makes an accidental or off-the-record
   self-accept visible and refusable, but a driver can still supply different
   strings or pass `--allow-self`. It does not enforce role separation between
   processes, and `resolve-challenge` remains a prover action.
4. **Ledger is append-only but not tamper-evident.** Node content is hashed
   but events are not hash-chained, so an agent with shell access to the
   workspace could rewrite history without replay noticing. Relevant when
   agents have write access beyond the `af` binary. *Fix:* per-event hash
   chain, verified by `af replay --verify`. Belongs to the v0.2 kernel work
   (docs/prd.md, "v0.2 Target"). Tracked: vibefeld-8x16.

## Audit script (planned)

A read-only `af audit` (or script over `af export --graph json` + the ledger)
reporting: admitted nodes; archived nodes with challenges open at archival;
self-accepts; nodes citing admitted/pending/tainted nodes; pending external
refs; amendments per node. Tracked: vibefeld-5qrx.
