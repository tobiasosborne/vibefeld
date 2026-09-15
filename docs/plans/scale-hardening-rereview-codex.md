# Re-review of docs/plans/scale-hardening.md v2

Reviewer: codex exec, gpt-6-astra, reasoning xhigh, 2026-09-15, against HEAD c67a7e6. The plan was revised to v3 in response; see the final section of the plan for the disposition of each point.

---

# Re-review: scale-hardening v2

**Verdict: no, not yet a plan of record.** V2 incorporates much of the review, but several replacement mechanisms still fail their advertised guarantees.

Compared against the verbatim scratchpad review. Decision references below name v2 sections; code paths are repository-relative. I reread code only for specific claims; no implementation, repository edits, or new runtime tests were performed. “ADDRESSED” assesses the specification, not completed implementation.

## 1. Disposition of the original review

### A. Decisions

| Critique | Status | Assessment |
|---|---|---|
| D1: support/scope semantics | PARTIALLY | D1 adds prospective graphs, shared creation checks and legacy repair. It misclassifies `case`, leaves discharge/active-branch semantics unspecified, and conflicts with D6’s graph (N2). |
| D2: dependency correction | PARTIALLY | D2 supplies reopening, broader states, manifests, preconditions, historical lists and dependency-aware diff. Durable retry identity, partial-item recovery, refinement obligations and statement-version compatibility remain unspecified. |
| D3: accepted revision | PARTIALLY | D0/D3 fix the two-read design and distinguish reconstructed hashes. D3 still overclaims verifier expectation and protection against changed support (N3/N6). |
| D4: live validation meaning | PARTIALLY | D4 correctly separates verdict history, preserves admitted prerequisites and specifies prerequisite ordering/file order. Its shallow `support_current` predicate does not establish current support (N3). |
| D5: claim release | PARTIALLY | D5 threads identity and preserves cleanup idempotence. Owner equality is not claim-generation fencing; D0 also leaves state-plus-release incomplete on crash (N1/N5). |
| D6: support taint | PARTIALLY | D6 fixes the admitted-child-of-cited-lemma recurrence, reverse propagation, missing targets and fuzz coverage. Hypothesis edges, ordering and boundary precedence remain inconsistent or incomplete (N2). |
| D7: fatigue diagnosis | ADDRESSED | D7 removes the falsity inference and subtree alarm, replacing them with descriptive counts, rounds, hotspots and explained thresholds. |
| D8: historical audit | PARTIALLY | D8 specifies ordered history, sequence-backed findings and current-only strict policy. It lacks reviewed-support revisions, actual strict-code selection and an implementable first-release dependency schedule. |
| D9: guardrails | PARTIALLY | D9 covers active descendants, proof authors and explicit self-accept provenance, with honest independence limits. Amendment contributors, incoming references, abandoned-obligation acknowledgement and verdict-file opt-out policy remain absent. |
| D10: migration protocol | PARTIALLY | D10 adds deliberate activation, backup, policy capabilities, corpus pinning and benchmarks. Crash/read races and actual mixed-binary tests remain unspecified; the acceptance test contradicts its old-binary finding (N4/N7). |
| D11: agent surfaces | PARTIALLY | D11 chooses `internal/jobs`, fixes prompts and specifies a stub-agent test. It never defines the `needs_refinement` handoff or complete readiness policy; current `internal/jobs/verifier.go:63` excludes that state. |
| D12: hash chain | ADDRESSED | “Deferred: tamper-evident ledger” gives the correct unanchored-chain limitation and moves the work to v0.2. |

### B. Migration risks

| Original risk | Status | Assessment |
|---|---|---|
| Mixed binaries/activation | PARTIALLY | D10 specifies stopping writers and capability preflight, but lacks the old/new, stale-process, interrupted-upgrade test matrix and later-policy rollout enforcement. |
| AISM/historical replay | PARTIALLY | D1/D10 preserve readable history, pin inputs and separate checks; D6 includes `validation.taint_counts`. P1 still demands identical replay despite changed derived state; readiness/classifier changes need explicit projection allowances. |
| rk/external verifiers | PARTIALLY | D2/D4 add validation edges, capabilities and compatible bulk fields. Scope/review-basis export, partial-success exit semantics and preservation of ordering/statement-version contracts remain undefined. |
| auto-prove.sh | PARTIALLY | D11 fixes actual commands and tests them, but root `support_current && clean` still misses descendant challenges/refutations (N3); external-reference completion policy is absent. |
| `expect_hash` after correction | PARTIALLY | D0/D2 reject stale accept/challenge/record-proof inputs without rewriting hashes. Operation identity and edit/revert or changed-support validity remain unresolved. |
| Replay self-heal/audit history | PARTIALLY | D6/D8 add traces, reverse dependents and historical projections; D2 replaces the invalid batch migration. Preserve the authoritative final derivation explicitly (`internal/state/replay.go:80`) and report source revisions, not merely paths. |

### C. Band-aids

| Original concern | Status | Assessment |
|---|---|---|
| C1: universal hierarchy | PARTIALLY | D1 attempts typing but D6 restores the raw hierarchy union; see N2. |
| C2: warnings masquerading as invariant | PARTIALLY | D4 introduces a derived signal, but its definition permits unsupported roots; see N3. |
| C3: dependency-only recurrence | PARTIALLY | D6 fixes mixed result-support paths but lacks consistent hypothesis/discharge semantics. |
| C4: threshold relocation | ADDRESSED | D7 removes the unsupported diagnosis itself. |
| C5: identity/archive safeguards | PARTIALLY | D9 labels limitations honestly but omits contributor and changed-proof-basis checks. |
| C6: stamp/chain replacing protocol | PARTIALLY | D10 and “Deferred” correct the trust boundary; D0 still lacks safe semantic-unit recovery. |
| Small executable model/crash tests | PARTIALLY | D6 supplies an independent support model; D0 offers interleaving barriers, not the requested process-death/fsync/recovery suite. |
| Legacy hash/canonical encoding | ADDRESSED | D3 preserves the legacy algorithm and explicitly defers its versioned replacement. |
| “v0.2 will not undo this” assurance | NOT ADDRESSED | “Why” repeats that promise without resolved support or commit semantics; additive JSON syntax cannot justify it. |

### D. Missing work at scale

| Item | Status | Assessment |
|---|---|---|
| 1: commit protocol | PARTIALLY | D0 fixes live-writer serialization; crash atomicity is deliberately deferred to v0.2. The required safe, resumable alternative for semantic units is missing (N1). |
| 2: expiry/reaping/fencing | NOT ADDRESSED | D5’s owner field supplies neither generations nor ledger-lock recovery; D7’s “stalled claims” finding supplies no protocol (`internal/ledger/lock.go:95`). |
| 3: round cost | PARTIALLY | D10 adds fan-out/concurrent multi-round benchmarks. Event/byte/replay counts, batch latency, 100-node comparison and a decision threshold are omitted. |
| 4: verification revisions/evidence | NOT ADDRESSED | D3/D4/D8 never bind verdicts or computational evidence to reviewed support. Acceptance still relies on any historical passing test (`internal/state/state.go:614`; `internal/service/proof.go:851`). |
| 5: machine interface | PARTIALLY | P5/D2/D8/D10 add codes, preview and capabilities. Durable operation IDs, committed sequences, partial-item outcomes, impacted consumers and bounded/filterable audit output are missing. |

### E. Ordering

| Original point | Status | Assessment |
|---|---|---|
| 0.1.9 contents | PARTIALLY | “Releases” brings correction, commit repair and minimal audit forward, but postpones shared eligibility and schedules unavailable prerequisites (N4). |
| 0.1.10 contents | PARTIALLY | “Releases” expands audit/guardrails, but moves support taint and export capabilities to 0.1.11. |
| 0.1.11 purpose | PARTIALLY | Hash chaining is removed, but “Order of work” leaves benchmarking until after that tag and never schedules verification-basis work. |
| Early D7/minimal D8 | ADDRESSED | “Releases” explicitly includes both in 0.1.9. |
| D5 only with claim/commit fix | PARTIALLY | D5 follows D0, but generation/recovery work is absent. |
| Pull forward existing D9 check | NOT ADDRESSED | “Order of work” item 11 leaves centralization after 0.1.9 without explaining the delay. |
| Activation versus deployed workers | PARTIALLY | D10 correctly requires operator coordination; later release instructions and the acceptance test undermine it. |
| Safe fallback if atomicity delays correction | NOT ADDRESSED | D0 declines atomicity, while D2 offers no exclusive resumable migration fallback. |
| Correctness before bucket preservation | PARTIALLY | P6 preserves corrections-before-taint, but “Releases” fixes an order with broken dependencies and unbounded first-release scope. |

### F. Author experience

| Original point | Status | Assessment |
|---|---|---|
| Clear policy preserving IDs/history | ADDRESSED | D2 gives an explicit reopening policy and attributed amendment history. |
| Verify all 21 corrections | PARTIALLY | “Acceptance test” requests the real ledger; four example fixtures cannot demonstrate all 21 succeed. |
| Avoid manual command choreography | PARTIALLY | D2 replaces it with a manifest; crash recovery remains unspecified. |
| Do not misuse `unvalidate --batch` | ADDRESSED | D2 uses explicit manifest node selection; “What the review changed” correctly explains the old mistake. |
| Dependency diff since challenge | ADDRESSED | D2 explicitly includes it. |
| No permanent audit failure after repair | ADDRESSED | D8 distinguishes historical findings and current strict failures. |
| Correct taint and exported prerequisites | PARTIALLY | D2 adds validation dependencies; D6 still conflicts with typed hypotheses. |
| Do not delay narrow offered patch | NOT ADDRESSED | “Order of work” puts broad D0/D1/D10/D11 work before release and author contact. |
| No surprise incompatibility/broken orchestration | PARTIALLY | D10/D11 improve migration and prompts; N3/N4/N7 remain. |
| First-class reviewed correction manifest | PARTIALLY | P7/D2 promise preview, hashes and re-verification; durable resume, impacted consumers and a concrete final graph/audit diff contract remain missing. |
| Real-ledger validation | ADDRESSED | “Acceptance test” makes a copy of the author’s ledger the preferred fixture, without pretending it is already available. |
| Prompt answer/tests-first invitation | NOT ADDRESSED | “Order of work” item 8 postpones replying until the tag; the review asked for a prompt decision/spec invitation. |

### G. Original top five

1. **PARTIALLY — Commit/stale-verdict races:** D0 fixes the read boundary, not recoverable logical commits (N1).
2. **PARTIALLY — Typed support/scope:** D1 adopts the distinction but misclassifies `case` and conflicts with D6 (N2).
3. **PARTIALLY — Complete support propagation:** D6 improves transitivity; D4 still permits false completion (N3).
4. **PARTIALLY — Resumable correction:** D2 supplies the interface without durable retry/recovery semantics (N1).
5. **PARTIALLY — Compatibility and defer D12:** D10/“Deferred” adopt the direction, but the compatibility acceptance test is impossible as stated (N7).

## 2. New problems introduced by v2

**N1 — “One commit” is not one correction.** D0 allows a crash after unvalidate but before amendment: D2 leaves a pending, unchanged node that another verifier can accept after recovery. Lost successful responses have no specified durable recognition mechanism for `--resume`: no operation ID/result location exists, and retrying the old expected hash rejects a completed correction. “Durable prefix” also needs directory fsync; current appends sync the temporary file then rename (`internal/ledger/append.go:209–235`), and readers do not take the writer lock (`internal/ledger/read.go:100`).

**N2 — Two incompatible mathematical graphs.** `case` has `OpensScope:false` (`internal/schema/nodetype.go:47`); case hypotheses use `local_assume` (`docs/concepts.md`, “Proof by Cases and Contradiction”). D1 permits hypothesis child→ancestor, but D6 includes ancestor→child again and declares the resulting legal proof cyclic. “Using internal/scope” also needs a derivation/population rule: current Refine supplies neither Scope nor ScopeOpened (`internal/service/proof.go:735`). Specify discharge exports, active branches and both edge kinds; D6 also needs dependency-topological ordering and explicit self-admitted/severed precedence, not its unqualified “no clean support path through a non-validated node” property.

**N3 — False completion survives.** For validated R→B→C, veto C or open a blocking challenge on B: D4 still sees R’s direct child B as validated, and taint can remain clean. D11 then announces completion. Separately, amend and reaccept a cited lemma: its consumers’ hashes and D4 predicates recover unchanged, although nobody re-reviewed their reasoning. D3 falsely says D4/D6 “carry that” change.

**N4 — Release dependencies contradict the table.** D8 minimal and D11 require D4’s signal in 0.1.9, but D4 lands in 0.1.10; D2’s exported capabilities are postponed to 0.1.11. D10 says 0.1.9/0.1.10 disagree on taint although D6 lands in 0.1.11. The 0.1.10 title also contradicts P8. Broadening 0.1.9 without a cut line risks delaying the correction indefinitely.

**N5 — Owner is not a generation.** After expiry/reclaim under the same owner string, delayed old work still matches D5’s owner check. Serialization prevents interleaving within an uninterrupted commit; it does not identify stale callers or recovered/retried releases.

**N6 — Hash recording is not expectation recording.** D3 says every acceptance hash matched a verifier expectation, but plain `accept` has no expected-hash flag (`cmd/af/accept.go:80`), and verdict-file expectations are optional (`internal/service/verdicts_apply.go:182`). Preserve the distinction between recording current content and checking supplied review input.

**N7 — Impossible acceptance test.** “Acceptance test for 0.1.9” requires an old binary’s structured format refusal after D10 establishes that 0.1.8 ignores the stamp. Unknown-event parse failure is not that contract. D10 also leaves backup consistency, directory durability, stale-reader handling and interrupted activation insufficiently specified.

## 3. Accuracy of “What the review changed”

The principal factual corrections—two reads, first-event CAS, batch-ID semantics, missing bulk checks, old format behavior and script flags—are accurate. Its summary overstates these responses:

- **D1:** the review requested introduced hypotheses; it did not endorse treating every `case` node as one (N2).
- **D2:** “one commit” means serialized only; it does not establish a safely resumable correction (N1).
- **D4:** “correct remedies” overlooks D4’s contradictory refusal outside pending/draft followed by “needs_refinement → allowed.”
- **D5:** “cannot evict a newer claimant” is false for reused owner identities (N5).
- **D6:** the cited-lemma subtree fix is real, but the summary hides its incompatible hypothesis graph.

These are proposed changes, not implemented fixes; the remaining bullets fairly summarize the proposals.

## 4. Ranked edits required for approval

1. **D0/D2:** specify one recoverable logical transition for reopen+amend, durable operation IDs/results, partial-prefix recovery and crash/fsync/lost-response tests; otherwise use the exclusive operator migration fallback.
2. **D1/D6:** define one typed active-support relation, represent case hypotheses with `local_assume`, specify discharge export and boundary rules, and use dependency-topological evaluation.
3. **D3/D4/D11:** make support recursively current and revision-aware, invalidate stale computational evidence, and test descendant blockers plus changed/reaccepted lemmas. Distinguish recorded hashes from checked expectations.
4. **D0/D5:** add claim generations, expiry checks and safe abandoned-ledger-lock recovery, with stale-worker tests.
5. **D10/acceptance test:** replace old-binary refusal with actual compatibility observations plus driver exclusion; specify consistent backup, publication durability and metadata/ledger reader races.
6. **Releases/order:** move required support/eligibility/capability work into 0.1.9, or remove dependent promises; correct taint versions, benchmark timing and the misleading release title. Define a bounded correction milestone.
7. **D2/D8/D11:** specify refinement handoff, unchanged/precondition precedence, statement-version preservation, strict codes, partial-result exits and final migration diff; explicitly scope external-reference completion.
8. **D9/order:** cover amendment contributors and abandoned obligations, decide verdict-file self-accept policy, and move the author’s decision/spec/tests invitation before implementation and tagging.
