# Second opinion on docs/plans/scale-hardening.md v1

Reviewer: codex exec, gpt-6-astra, reasoning xhigh, 2026-09-15, against HEAD c67a7e6. Read-only review; the disposable-workspace checks it cites were run in a scratch directory. Every factual claim about the code that the plan v2 relies on was re-verified by hand afterwards (see plan v2, section 'What the review changed').

---

# A. Decisions D1–D12

**Recommendation: do not approve this plan as written.** Ship dependency correction on 0.1.x, but revise the graph semantics, concurrency guarantees, and migration protocol first. The plan identifies real problems; several proposed fixes do not establish the invariants claimed for them. Its most consequential omission is the existing gap between checking a verifier's expected content hash and committing the acceptance.

Reviewed against repository HEAD **c67a7e62502581af7f6813f7f7c5cd244ffab85d**, following the requested reading order. References below are repository-relative file paths and line numbers, unless explicitly identified as the supplied issue. This is a review, not an implementation. I built the current CLI into the scratchpad and ran small disposable-workspace checks there; I did not run the whole test suite, change repository files, or run the AISM regression corpus. The supplied performance numbers are accepted as observations, not extrapolated throughput guarantees.

## D1 — Disagree with the unconditional hierarchy rule

**“Ancestors always do” and “child citing ancestor is circular reasoning” are false as general statements about structured proofs.** They are reasonable for a child using the *conclusion currently being proved by that child's subtree*. They are not reasonable for every use of an ancestor's assumptions or context.

A concrete counterexample already appears in **docs/concepts.md:418–435**:

    1.1     local_assume "Suppose p = 2"
    1.1.1   claim "Then p is even"

The child legitimately uses the ancestor's assumption. That assumption is introduced, not established by the child. Analogously, an induction step may use an explicitly introduced hypothesis P(k) to prove P(k+1), and a case branch may use its case hypothesis. In Lamport-style ASSUME/PROVE structure, using the ASSUME part is different from citing the PROVE conclusion as established. A bare reference to the theorem “for all n, P(n)” from its own induction proof is still unsound; “induction” is not a general exception to cycle checking.

The repository already distinguishes local assumptions, discharges, and claims (**docs/concepts.md:92–118, 403–477**). D1 erases that distinction. Simply exempting every ancestor or every ancestor whose inference label contains “induction” would be equally wrong: inference labels are free text, and scope semantics come from node type.

**Concrete alternative:** define an edge contract before selecting the DFS graph:

- Result-use dependencies consume established conclusions.
- Scoped assumption uses consume hypotheses available at the use site.
- Hierarchy records proof decomposition; acceptance prerequisites are a separate relation.
- Check conclusion-support cycles, and check scope containment/discharge separately. Use the existing local-assumption/scope representation where it suffices; if a single node label denotes both hypothesis and conclusion, explicitly separate those endpoints or represent the hypothesis with a local_assume node.

Parent-to-active-proof-child edges belong in the conclusion-support relation for ordinary claims. Do not blindly add them for an assumption's truth, abandoned alternatives, or every structural container. An archived branch is explicitly no longer relied on by its parent (**proof.go:886–892**); permanently retaining that hierarchy edge for logical-cycle rejection can forbid an otherwise legitimate dependency. Conversely, making support edges state-dependent requires checks on every transition that activates support.

There are also implementation omissions:

- Allocating the child's ID is insufficient. **cycle.go:274–277** returns “no cycle” when the source is absent. The overlay must include the prospective node, its dependencies, and the parent's prospective child edge.
- **proof.go:1733–1794** creates bulk children without the single-refine cycle check. Fix the shared creation path used by both RefineNodeBulk and RecordProof, and validate the complete prospective batch graph.
- “Any node unless cyclic” omits scope containment. A cousin edge can leak an assumption across cases without forming a cycle (**docs/concepts.md:429–450; docs/prd.md:1483**).
- Existing ancestor references will not vanish at upgrade. Audit them before enforcement; replay must remain readable, and a repair operation must be able to reduce a legacy invalid component without requiring the whole workspace to be clean first.

Fix the wrong-source bug at **proof.go:716,727**. Replace the skipped test with separate tests for forbidden ancestor-conclusion use and permitted scoped hypothesis use. Deleting it with the proposed blanket rationale would encode the wrong mathematics.

## D2 — Agree-with-changes; prefer explicit automatic reopening over mandatory command choreography

An append-only dependency amendment, stable node ID, previous/new edge sets, reason, attribution, hash recomputation, and validation-dependency support are right. **node.go:267–281** already hashes both lists; **apply.go:401–420** establishes the amendment precedent. The issue author's central request is justified.

**I disagree with choosing policy (a) simply because policy (b) is “(a) with the unvalidate hidden.”** The transition need not be hidden. An explicit option such as *--reopen*, with a preview/report stating “validation revoked; dependency revision changed; re-verification required,” can preserve exactly the same trust policy while avoiding a race-prone three-command handoff. Use policy (a) as the primitive if useful, but provide a safe combined operation for the issue's 21 validated nodes. Do not preserve current validation merely because the verifier identity field has been recorded: policy (c) needs a separate distinction between a historical verdict and current verification.

The dangerous concurrency window is practical: unvalidate makes a node pending and potentially verifier-ready; another agent can accept it before amend-deps runs. Claims and stale verdicts make this more than a verbosity complaint. A combined correction should be one logical commit, with explicit reopening and re-verification, after fixing the transaction primitive discussed in D5/D10 and section D.

**Allow amendments in needs_refinement.** It is already reopened and unresolved (**apply.go:441–450; taint/propagate.go:216–219**). Refusing corrections there creates a workflow trap: UnvalidateNode accepts validated nodes, not needs_refinement nodes (**proof.go:2305–2309; schema/epistemic.go:106–120**). The suggested request-refinement path can therefore put a node into a state where neither statement nor dependency correction is allowed. Keep it reopened, record the amendment, and require explicit acceptance afterwards. Decide separately whether an edge-only repair satisfies a “deeper proof requested” obligation; do not let amendments silently discharge that request. Draft correction should also be considered consistently.

**Do not make retries fail merely because their intended set change already happened.** In a many-agent CLI, “ensure edge present” should return a structured unchanged outcome and append no event if already satisfied. A strict mode can catch mistaken inputs. Reject contradictory add/remove instructions and malformed IDs; normalize duplicates deliberately. Include an operation ID for retry recognition and an optional expected hash/revision. If a stale expected hash is supplied, do not bypass it just because the requested edge now exists, unless the operation ID proves this is a retry of the committed operation. The ledger can contain only real changes without punishing successful retries.

Further changes:

- Validate new targets, scope, both edge kinds, and the *replacement* graph. Do not reject a removal-only repair because an unrelated legacy cycle remains.
- Preserve the old lists in their actual historical form; do not retroactively normalize the corpus. Verify the event's previous lists/revision against replay state, rather than copying applyNodeAmended's unconditional overwrite.
- Add **af diff**, including “since challenge,” to the surfaces. The issue explicitly asks for it; **cmd/af/diff.go:83–129,168 onward** currently models statement versions only. Keep existing statement-version numbering stable or version the new output contract.
- Provide batch correction input, per-item outcomes, old/new hashes, and a re-verification work list. “Batchable” is not implemented by the proposed flags.
- Do not claim additions are semantically harmless in all senses. They can add acceptance prerequisites, expose scope problems, or change the proof a verifier must read. “Does not clear uncertainty” is narrower than “does not need review.”

## D3 — Agree-with-changes

Record the accepted node content hash on every acceptance event, and retain it in the current validation projection and historical audit records. Missing fields must mean “not recorded,” not “matched.” This extends **ledger/event.go:156–162** without changing historical events.

**But a hash written at acceptance is not proof that the verifier inspected those bytes.** In **verdicts_apply.go:166–202**, the wrapper loads state and checks ExpectHash, then calls AcceptNodeWithVerifier, which loads state again at **proof.go:832** and takes a fresh CAS sequence. An amendment between the reads can produce a successful acceptance of different content. D3 would faithfully stamp the *wrongly accepted new hash*, and its proposed mismatch audit would see nothing wrong. The challenge path has the same two-read pattern (**verdicts_apply.go:239–258,282–299**).

Pass expected content and readiness preconditions into the service operation that performs the single authoritative read and append. Share that path across CLI, bulk acceptance, and verdict files. Test the interleaving with deterministic barriers, not a probabilistic sleep.

Also define what the hash covers. It covers the node's dependency **IDs**, not their current contents, children, scopes, evidence, or external-reference contents (**node.go:220–281**). Changing a child or a cited lemma can invalidate a proof without changing this hash. Preserve the legacy hash algorithm for compatibility; add a separate review-basis record/revision manifest for the support the verifier actually reviewed, or explicitly limit D3's guarantee to the node's own fields. Do not call it the complete verification invariant.

Finally, old events do not make every historical comparison impossible: replay can capture the content present at each legacy acceptance. Label that “reconstructed ledger content at acceptance,” distinct from a recorded verifier expectation. This can make audit useful immediately without fabricating historical attestations.

## D4 — Agree-with-changes; it does not establish “always”

Single/bulk acceptance should share one validator. The missing bulk gates are real (**proof.go:855–908 versus 965–986**). Preventing new proof children under an accepted claim without reopening is sensible.

Four corrections are necessary:

1. **Depth, then lexical order, is not a topological order for a DAG.** Two pending siblings 1.1 and 1.2 with 1.1 requiring 1.2 are already a counterexample. A deeper node may require a shallower cousin as well. Schedule acceptance by actual child/validation prerequisites, with hierarchical ID only as a deterministic tie-breaker. Distinguish blocked prerequisites from rejected input. Preserve verdicts apply's documented file order for mixed accept/challenge files (**verdicts_apply.go:111–119**); reordering such a file changes its meaning.
2. **The invariant can be broken without refining an accepted parent.** UnvalidateNode, RequestRefinement, UnadmitNode, and VetoNode change a child or prerequisite without changing dependent parents (**proof.go:2247–2370,1172–1207**). I confirmed: accept child and parent, then unvalidate child; parent remains validated/unresolved. Vetoing the reaccepted child leaves parent **validated/clean**, while graph export says closed=false. Taint deliberately severs refuted branches (**taint/propagate.go:151–159**), although acceptance refuses refuted children. D4's health finding detects this; it does not make “validated means children cleared, always” true.
3. **The admitted-parent remedy is wrong.** RequestRefinement only permits validated → needs_refinement (**proof.go:2261–2265**). An admitted parent needs unadmit. Explicitly decide behavior for draft, needs_refinement, archived, and refuted parents too; banning only two states leaves other nonsensical refinements available. Apply the gate to all creation entry points, including CreateNode if it remains callable (**proof.go:386–434**).
4. **“Unvalidated validation dependency” needs a definition.** Current acceptance allows admitted validation deps (**proof.go:865–866**). Either preserve that as “cleared with taint,” or deliberately tighten it with migration guidance. A health rule must not classify an intentionally admitted prerequisite as the same structural error as a missing/pending one.

My preference is to distinguish the recorded historical verdict from a derived *currently supported/verification-current* signal, and have acceptance eligibility, health, jobs, and exports consume a shared policy. Reopening a prerequisite must immediately make affected support non-current; changing its content must not silently restore confidence in consumers merely by reaccepting the new lemma. Preserve history; schedule any required re-review explicitly. If the project instead insists that the stored validated state itself is a live invariant, implement transitive revocation. The plan must choose; two read-only warnings do not resolve it.

## D5 — Agree-with-changes

Automatic release is useful, but **“append release after the state event” is not a lock protocol**.

The normal CLI claim is a ledger claim: **proof.go:443–483,548–580** never acquires/releases a per-node filesystem ClaimLock. The plan needs to identify the actual claim representation rather than assuming both exist. AdmitNode, RefuteNode, and ArchiveNode currently do not even receive the caller identity (**proof.go:1081,1127,1221**); thread that identity through every relevant surface, rather than treating verifier provenance as proof of claim ownership.

State change plus release must not interleave with reap/reclaim. NodesReleased carries only IDs (**event.go:98–102**); replay does not compare an expected holder (**apply.go:165–178**). A delayed release can otherwise release a newer claimant. Use a checked claim generation/owner in the transaction protocol, and a recoverable logical commit. Handle explicit release after auto-release as a documented idempotent outcome, so existing cleanup scripts remain usable.

Do not reuse appendBulkIfSequence unchanged. It CAS-checks the first event and then does unrestricted appends (**proof.go:1861–1927**). Its comment about “implied serialization through the sequence numbers” is false. See section D.

## D6 — Disagree with the proposed recurrence; agree that dependency taint must ship

The three-level internal join **clean < tainted < unresolved** agrees with **taint/propagate.go:10–16,222–229**. Keeping ancestor context separate from support propagated upward is essential to preserving the 0.1.7 sibling fix. But the proposed dependency component is too narrow.

**Counterexample:** A cites validated B; B has admitted proof child C. B has no explicit dependency edges. D6 says a validated dependency contributes its own *dependency component*, which is clean here, so A misses the admission that B's proof rests on. This is exactly the kind of cross-branch reuse a DAG proof needs. **docs/trust-model.md:31–38** explicitly includes tainted targets, not just directly admitted ones.

A second omission goes in the other direction: if child C acquires uncertainty through an explicit reference, its parent must inherit that support uncertainty. Existing upward computation uses child epistemic state plus child.up only (**taint/propagate.go:145–165**), not the child's new dependency component. Merely adding a third final component leaves this path unspecified.

**Alternative:** define intrinsic proof-support uncertainty recursively over active child-support and result-use dependency edges together. For a validated target, propagate the uncertainty of the proof supporting that result, including its active subtree and transitive cited results. Join ancestor-context uncertainty separately; never propagate the parent's fully combined taint down into siblings. Specify scoped assumption obligations at the same time as D1. Do not naively substitute the target's stored/final TaintState: that reintroduces stale-state and ancestor/sibling feedback problems.

For explicit references, admitted → tainted and unfinished → unresolved are sensible. Archived/refuted → unresolved is a conservative display value in the existing enum, **provided audit/eligibility distinguishes abandoned support from disproved support** and reports a blocker; a refuted lemma is not merely waiting for an agent. Missing targets need an explicit unresolved/error rule too. Do not silently replace a missing target with a clean leaf because cycle.DetectCycleFrom tolerates missing nodes (**cycle.go:68–69,123–127**). A severed *hierarchical alternative* and a retained *explicit citation* have different meanings.

Preserve boundary precedence: an admitted node's own unfinished descendants are intentionally ignored; archived nodes remain severed. An indiscriminate final join with every dependency can change self_admitted behavior. Write the complete rule order, rather than appending “rules 8–9” after today's otherwise-clean return.

**External references:** deferral is acceptable only as a stated limit of the trust signal, with used pending/unverified references visible in completion/audit policy. “Clean” cannot then mean “nothing taken on faith” (**trust-model.md:6–9**). Before adding external taint, define cited versus merely registered references, verified versus admitted imports, scope, and mutable external records. pending-refs alone does not state which conclusions rely on them. This need not block amend-deps, but it cannot disappear behind a clean root.

The fuzz strategy needs mixed hierarchy/reference/validation paths, local assumptions, missing targets, pre-existing cycles, and command sequences including amendment/reopen/archive—not just “5000 trees.” Equality between replay and incremental derivation, idempotence, and no false-clean support paths are the useful properties (**docs/prd.md:1549–1554**). A thousand-node DAG with few edges is not a stress test for transitive support.

D1 cannot guarantee that historical ledgers are DAGs under the new relation. Detect cyclic support components explicitly, mark their support invalid/unresolved, and retain a readable repair path; do not recurse indefinitely or let mutually validated nodes acquire clean support from an unconstrained fixed-point computation. New writes must not introduce such components.

Finally, update PropagateTaint's affected set and the service's audit-emission filter, both currently restricted to ancestors/descendants (**taint/propagate.go:38–53; proof.go:1515–1517**). Dependency edits and changes to referenced nodes must reach reverse dependents and their relevant parents. Replay self-heal alone leaves incremental callers and taint explanations inconsistent.

## D7 — Agree-with-changes

Remove absolute subtree fatigue alarms now. The proposed per-node aggregation is an improvement, but **resolved challenges plus amendments is still not evidence that a conjecture is false**. One successful repair can count twice, and twelve independent verification rounds naturally accumulate findings. The current metric really is resolved + amendments, not just resolved challenges (**state/repair.go:101–102**); the alarming claim is at **cmd/af/health.go:278–281**.

Report rework as descriptive data: node-local counts, distinct review rounds when known, recent reopenings, recurrence of the same unresolved issue, severity, and progress since the last accepted revision. Keep lifetime counts but do not alarm permanently on them. A mean per node hides hotspots and changes with decomposition granularity; show hotspots/distribution too. Make thresholds configurable and explain their units. If round information is unavailable in old ledgers, say so.

The cheap first-release fix is to remove the unsupported “probably false” conclusion and separate rework information from current blockers. A different magic denominator is not a principled diagnostic.

## D8 — Agree-with-changes; move the minimal audit into 0.1.9

Audit is necessary before migration and before dependency taint. But **the current State is insufficient for the proposed historical findings**:

- applyNodeArchived overwrites epistemic state and supersedes challenges (**apply.go:239–251**).
- Validation identity/batch fields are cleared by unvalidation (**apply.go:481–483**).
- State has no archival/validation timeline (**state.go:148–219**).
- Current superseded challenges do not reliably reconstruct the exact open set at archival, especially after later disposition events.

Use a single ordered ledger analysis, or add replay-derived historical indexes carrying sequence numbers, prior validation revisions, and challenge status at the relevant event. Do not sort cross-event history only by timestamps. This remains read-only; “over State” must mean an explicitly richer projection.

Do not make strict mode fail on *any* finding. An intentional admitted import, an ordinary pending reference during development, or an edge amended after an earlier validation and then reverified are not necessarily current errors. D2's prescribed migration would itself create “edges amended after validation,” so D8 as written can permanently fail a successfully repaired proof.

Give findings stable codes, severity, current/historical status, node IDs, supporting event sequences, and remediation. Gate on specified current error classes or an explicit policy profile. Include unknown provenance, support cycles, scope leaks, missing deps, stale support revisions, and validated nodes with newly open blocking challenges. Keep health and audit on one findings engine to prevent divergent definitions.

## D9 — Agree-with-changes; these are guardrails, not exploit closure

The archive guard catches a common mistake, but a prover can resolve its own challenge and then archive, or archive an unchallenged ancestor of the challenged hard step. The tool explicitly offers prover-side resolution; RecordProof even resolves all open challenges on its parent (**record_proof.go:115–125**). Checking only the target's currently open challenges cannot close archive-the-hard-step.

Retain force/reason as an auditable escape hatch, inspect affected active descendants and incoming references, and show the verifier what was removed from the parent's proof basis. A subsequent acceptance should acknowledge relevant abandoned obligations or a changed proof basis. Do not make archived history itself permanently taint every future replacement proof. Update trust-model.md to say which shortcut is prevented and which still requires verifier judgment.

The self-accept guard already exists in verdicts apply (**verdicts_apply.go:193–200**), so centralizing it is right. However:

- The CLI still permits no verifier identity (**accept.go:55–57,85**).
- Equality with original Author misses ProofAuthor on decomposed nodes (**node.go:109–131**) and the agent who amended content or dependencies.
- Driver-supplied aliases do not establish independent identities or independent model families (**event.go:147–154**).
- Allow-self must be explicit, attributed, and preserved in history; decide whether verdict files can opt out at all, rather than accidentally loosening their current contract.

Compare against relevant proof contributors for the reviewed revision, and make missing attribution visible. Require meaningful identities for new automated verification workflows; preserve clearly labeled legacy/manual behavior where necessary. Do not claim this enforces adversarial independence. Independent-round/model-family policy belongs in the orchestrator with recorded evidence.

## D10 — Disagree with the claimed safety of the migration trigger

A format gate is useful; **the factual premise that the existing binaries are fenced by meta.json's validation check is not supported by the load paths.**

Yes, **config.go:147–149** rejects version != 1.0 *when Validate is called*. But config.Load does not call Validate (**config.go:57–89**), ProofService.LoadConfig only calls Load and caches the result (**proof.go:154–171**), and LoadState skips config altogether (**proof.go:299–304**). I found no production call to config.Validate. The direct replay CLI bypasses LoadState too (**cmd/af/replay.go:92–98**).

**Confirmed on the current 0.1.8 build:** I initialized a disposable workspace, changed its meta.json version to 1.1, and ran status, claim, and replay --verify. All returned exit 0; claim appended an event. Evidence is in **review-checks/format-check-results.json** beside this review. This tests current 0.1.8, not every historical binary; it is enough to invalidate the plan's blanket assurance. Unknown event parsing still fails later (**state/replay.go:189–192; state/apply.go:104–105**).

“Bump atomically before append” also conflates one-file atomic replacement with a two-resource protocol. Required behavior:

1. Parse/validate/preflight the operation before initiating an upgrade.
2. Under the same workspace write critical section, reread the format/policy requirement, check the ledger sequence, monotonically persist and sync the required format, then append the incompatible event with a defined recovery boundary.
3. Specify crash outcomes. Stamp 1.1 with no new event is conservative but can unnecessarily lock out a workspace; format 1.0 with a durably published incompatible event is unacceptable. Directory durability matters, not just renaming a temp file. config.Save is currently a direct WriteFile (**config.go:154–170**).
4. Never downgrade the stamp via cached config. Long-lived shell/service instances must recheck at commit; readers must handle the race between reading metadata and scanning a newer ledger prefix.
5. Apply the check to every supported writer and direct replay entry point, not merely LoadState. Interactive challenge performs a raw append after a separate state read (**cmd/af/challenge.go:155–181**).

A new binary cannot retroactively teach old writers to respect a new gate. Drain/stop old writers for the upgrade and require driver-level executable/capability preflight. Keep an immutable backup/checkpoint and state that downgrade after new events requires restoring a prefix/copy, not editing the stamp.

I prefer an explicit, once-per-workspace feature/format activation with dry-run and an exact migration report. Lazy activation at the first incompatible write is defensible to minimize churn **only if it is documented, deliberate, recoverable, and protected by the protocol above**. It is not sufficient to label a rename “atomic.”

Also separate event-format compatibility from writer-policy/derived-semantics compatibility. 0.1.9 and 0.1.10 can both read 1.1 while disagreeing on taint and self-accept rules; an optional prev_hash field would not trigger this gate at all. A format number does not mean old writers enforce new trust policy. Advertise required capabilities/policy versions; do not invent an exact “af ≥ X” for an unknown future version unless that mapping is available.

Agree with real tags/releases and a checked-in deterministic generator. Reject a single machine-specific wall-time assertion as the scale evidence. Measure representative workflows, event/edge counts, and concurrent writers; keep ordinary CI correctness checks deterministic and timing budgets broad or in a controlled benchmark job.

## D11 — Agree-with-changes

Help and provenance display fixes belong with the behavior they describe. The D11 classifier wording is backwards if it means adopting the renderer's classifier: **render/status.go:167–184** still counts available pending nodes as prover jobs and claimed pending nodes as verifier jobs, while **jobs/verifier.go:50–68** uses available pending nodes without blocking challenges. Export intentionally uses internal/jobs (**export/graph.go:186–205**).

Consolidate classification in a shared policy used by jobs, status, health, get/checklist, export, and orchestrators. Include validation prerequisites and the needs_refinement-to-verifier handoff; FilterReadyVerifierJobs presently checks children only (**jobs/verifier.go:95–105**). Otherwise agents will repeatedly dispatch work that accept refuses.

Preserve CLI/JSON contracts while adding provenance. Repair auto-prove.sh's actual generated prompts, not just root --help (section B). Update hash documentation to distinguish stable node identity, content revision, and reviewed proof basis. “The ledger is the identity” is too vague: the stable logical identifier is the node ID within a workspace; the ledger records its revisions.

## D12 — Disagree with shipping this as a dedicated 0.1.11 release

A local unanchored chain is **not the promised solution to an agent with arbitrary workspace write access**. Such an agent can rewrite the event and recompute the suffix; it can truncate the ledger to a valid prefix. The current final event has no successor authenticating it. An external trusted head/checkpoint is needed to detect those changes. Denying workers arbitrary filesystem writes is a useful deployment boundary, but it does not make the chain an independent trust anchor (**trust-model.md:46–51**).

The proposal also leaves important details undefined:

- Canonical JSON rules, preservation of unknown fields, algorithm identifiers, and sequence/workspace binding.
- Whether the first chained event commits the *whole* legacy prefix or merely the immediately previous event; the latter leaves earlier unchained history unauthenticated.
- Rejection of a missing prev_hash after chaining begins. Older writers can append known event types without it, and D10's new-event-type trigger will not stop that.
- Batch chain calculation under one lock, crash recovery, and durable external head publication.

Either defer this to the ledger/kernel work, or implement a clearly scoped accidental-corruption check with an explicit checkpoint contract. Do not allocate a patch release to a security claim the mechanism cannot meet. The plan itself calls D12 the item v0.2 will revisit, contradicting its introductory “nothing here is undone” assurance.

# B. Migration and compatibility risks

## Mixed binaries and activation

The most urgent compatibility problem is **semantic split-brain**, not just unknown-event parsing. Older binaries can compute old taint, permit old acceptance/refinement behavior, omit accepted hashes, or append unchained events. A 1.1 stamp alone cannot distinguish 0.1.9 from 0.1.10 policy. Enforce a supported writer set in the driver, stop ongoing writers for activation, and report executable version/commit, workspace format, and policy/capability version in machine-readable output. Do not promise ongoing mixed-writer safety until it is tested.

Test old-read/new-write and old-write/new-read paths with actual release binaries, including a process already holding a stale state/config, a paused batch, direct ledger-appending commands, and interrupted upgrade. Optional JSON fields usually preserve parsing; they do not preserve enforcement.

## AISM and historical replay

The plan conflates unchanged input bytes, successful replay, identical replay-summary JSON, identical derived state, and identical graph export. They are different checks. **CONTRIBUTING.md:64–81** describes both replay and graph regression, and notes that replay output is a stats summary. **handoff.md:31–33** records 211 workspaces, whereas CONTRIBUTING still says 44; the live external corpus is not a pinned fixture.

Pin or snapshot a read-only manifest with workspace list and ledger digests. Run old/new binaries on exactly that input. Preserve historical node hashes and event bytes, separately assert parse/hash verification, and review an explicit semantic projection diff. Add representative in-repo minimal regression fixtures for continued CI, with no assumption that an external research checkout exists.

D6 cannot promise the full export diff is limited to taint_state: **graph.go:260–261** also changes validation.taint_counts. Correct changes through mixed support paths may affect nodes without a direct edge to a non-validated node. Validation/readiness changes can affect other fields too. Name the actual allowlist and explain each changed support path.

Regression scripts must inspect JSON validity as well as process status: **cmd/af/replay.go:139–145** currently prints a failed replay report and returns nil in JSON mode. A shell loop checking exit status alone can declare an invalid corpus pass. Fix that error contract with explicit release notes, and assert both valid and hash_verification.valid in the migration checker.

Do not enforce new creation-time graph or scope constraints as unconditional historical replay rejection. Existing workspaces may contain ancestor edges, pending children under validations, or cycles under the new interpretation. They need readable audit and repair paths. Corpus compliance must not force retaining wrong taint semantics merely to keep an output file identical.

## rk and external verifiers

Graph v1 already has schema_version and a capabilities list (**graph.go:138–151; docs/export-graph-v1.md:16–17**). Use them. Dependency amendment history without **validation_deps** is incomplete: GraphNode currently exports Dependencies but not ValidationDeps, scope, or context (**graph.go:46–125; docs/export-graph-v1.md:60 onward**). A consumer cannot explain an acceptance block or all newly load-bearing edges from this projection.

Add optional capabilities/fields for validation dependencies, revised verification basis, and dependency-aware taint. Preserve stable ordering, omitted-empty conventions, statement bytes, node IDs, and existing readiness/closure meaning, or explicitly version an incompatible semantic change. Decide how consumers invalidate cached verdicts and compute completion. The existing closed flag means settled subtree, not clean proof (**graph.go:116–124**); neither closed nor validated alone promises rigor.

D4's bulk CLI report is a contract change: today's output has accepted/count/status (**accept.go:280–287**), while verdicts apply has blocked and rejected outcomes and exits 5/6 (**verdicts_apply.go:42–108**). Specify compatible output and partial-success semantics. Do not silently change verdict-file order or introduce blanket accepts to avoid migration work.

## auto-prove.sh

This is a shipped integration, and it currently declares success on the root's epistemic state alone (**scripts/auto-prove.sh:658–666**). D6 can correctly make a validated root tainted/unresolved without stopping that false success. The script should use an explicit completion policy incorporating support/closure, taint, current blocking challenges, and any required external-reference status.

Its generated prompts also use accept --note (CLI uses --with-note), omit claim/release owner identity, and do not supply acceptance identity (**auto-prove.sh:522–550; accept.go:80–85; amend.go:49–50**). D5 cannot release “the caller's own claim” when the caller has no consistent identity. Test the generated commands with a stub agent and disposable workspace, including terminal auto-release and explicit cleanup. Update all pending/refinement transitions and dependency repair instructions there.

## expect_hash after dependency correction

Changing either edge set must change the current content hash. In-flight accept **and challenge** verdicts authored against the previous hash should be rejected and regenerated after review; never mechanically replace expect_hash in an old verdict file. The same applies to record-proof --expect-hash (**record_proof.go:28–31,84–86**).

Fix the two-read race in section A/D3. Add expected revision/operation identity to amendments, because an internal ledger CAS protects only the service call's brief read/write interval, not an LLM's earlier reasoning interval. A hash can also return to an earlier value after an edit/revert; whether that is acceptable for a verdict depends on whether the rest of its review basis changed.

## Replay self-heal and audit history

Preserve the single authoritative end-of-replay derivation (**state/replay.go:80–83**), so stale TaintRecomputed events cannot override current rules. But version/explain the derivation and preserve historical audit values as historical facts. New and old binaries will otherwise disagree on the same unchanged ledger.

Update taint-trace, which currently explains only ancestor and descendant sources (**cmd/af/taint_trace.go:127–183**), and reverse-dependency audit emission. It should name an actual path and source revision, not say there are no sources when the new dependency component is tainted.

The proposed migration commands are not a script. Audit does not exist in 0.1.9, so “audit before and after upgrading to 0.1.10” needs the new binary running read-only against a pinned copy/checkpoint. Unvalidate --batch revokes a *previous validation batch ID*, not an arbitrary list of the 21 corrected nodes (**cmd/af/unvalidate.go:34–45**). It may revoke unrelated nodes or match none. Supply explicit node selection, resumable per-item correction results, newly generated review inputs, and a final graph/audit comparison.

# C. Band-aids and work v0.2 would have to revisit

The assertion that adding optional fields/events guarantees an enduring fix is wrong. Syntax compatibility says nothing about mathematical semantics or the write protocol.

The principal band-aids are:

1. **D1's universal hierarchy edge:** it substitutes a tree-shaped shortcut for a typed support/scope model. A trustworthy kernel would have to distinguish introduced assumptions from proven conclusions.
2. **D4's health warnings as an “always” invariant:** diagnostics expose violations but do not prevent or derive away unsupported validation. Reopening and refutation still break the claimed meaning.
3. **D6's dependency-only transitivity:** it misses proofs of reused results and dependencies of proof children. Passing a reference implementation that repeats that recurrence would reproduce the 0.1.7 process failure described at **prd.md:1532–1536**.
4. **D7's threshold relocation:** it still treats successful scrutiny as evidence of falsity. Make the metric descriptive until there is a defensible diagnostic.
5. **D9's literal author equality and open-challenge archive check:** useful safeguards, insufficient independence or proof-basis enforcement. Keep their limitations visible.
6. **D10's stamp and D12's unanchored chain:** neither substitutes for a common, crash-tested writer protocol and a defined trust boundary.

Borrow the first independently shippable v0.2 deliverables now: a small executable model for changed transitions/support rules, and deterministic concurrency/crash tests around a shared commit primitive (**prd.md:1547–1562**). This does not require a language rewrite or waiting for the v0.2 release.

Do not silently change the legacy content hash while doing that. Besides incomplete review coverage, its free-text delimiter encoding is not a general canonical tuple encoding (**node.go:240–280**). For example, statement “s|inference:x” with inference “y” and statement “s” with inference “x|inference:y” serialize identically. Free inference text is allowed (**node.go:182–186**). A future canonical content encoding needs its own versioned migration; a new review-basis digest can use an unambiguous encoding immediately. This is another reason not to promise v0.2 will merely rubber-stamp the resulting ledger semantics.

# D. Missing work at the stated scale

## 1. Repair the commit protocol before extending it

**appendBulkIfSequence is unsafe under concurrent writers, not merely under rare disk failures.** After its first CAS succeeds, every later event takes a fresh independent append lock without checking the state used to authorize it (**proof.go:1905–1925; ledger/append.go:153–183**).

Example: a bulk operation plans to accept X then Y. It commits X; another agent raises a blocking challenge on Y; the bulk operation unconditionally appends Y's validation. Replay checks Y's epistemic transition but not its acceptance prerequisites (**apply.go:184–197**), so the stale authorization survives. Similar interleavings can affect sibling allocation and delayed claim release. RecordProof calls the same helper despite claiming atomicity (**record_proof.go:41–52,140**).

Provide one shared writer critical section with sequence/precondition checks over the planned batch and precise committed-result reporting. **AppendBatch is not a complete replacement:** it lacks CAS, publishes files by separate renames, and attempts rollback by deleting already visible files (**ledger/append.go:242–331**), while readers scan without the writer lock (**ledger/read.go:100–120**). It is neither crash-atomic nor reader-atomic.

Choose and specify a recoverable commit boundary—such as a committed batch manifest/journal protocol respected by readers and writers—or explicitly supported durable prefixes with resumable operations where atomicity is unnecessary. For semantic units such as reopen+amend or verdict+claim disposition, define one logical transition and crash behavior. Do not leave comments promising ACID while silently depending on success. Test process death between publications, fsync/rename failures, CAS conflict, and retry after a committed operation whose response was lost.

## 2. Define claim expiry, reaping, and fencing

A crashed writer can leave ledger.lock behind: **ledger/lock.go:95–116** creates an exclusive file, and Acquire only polls until timeout (**49–79**). Metadata has an operation label/time, not a unique process/lease generation (**23–26; append.go:165**). A recovery command needs to distinguish abandoned ownership from a slow/live writer and prevent a displaced writer from continuing.

The ledger claim path and filesystem lock utilities are separate; health should expose which is blocking work. Check claim generation/expiry consistently at mutation time, document supported filesystem assumptions, and report stale claims separately from stalled proof reasoning. The recent **handoff.md:34–37** already lists the LockInfo/tolerance issue; omission from this “scale hardening” plan needs justification, even if its production reach is limited.

## 3. Measure round cost, not one status call

A fast 3202-event read is good evidence against urgently adding persistent snapshots. It is not evidence that performance is irrelevant for many concurrent agents.

AcceptNodeBulk emits taint events by invoking a full state reload **once per accepted node** (**proof.go:1007–1015,1492–1496**). verdicts apply also performs multiple replays per item. Each append scans the ledger directory to find the next sequence (**ledger/filename.go:54–79**). These costs and global CAS conflicts accumulate during verification rounds. Dependency fan-out can create many taint audit events.

Measure 100/1000 nodes across multiple rounds, high dependency fan-out, challenge/claim volume, and 10–50 concurrent writers; report event count, bytes, p50/p95 command latency, batch latency, retries, lock wait, and replay count. Reuse one authoritative state per logical batch and derive/report changes once before introducing a persistent cache. Set a measured threshold for snapshots later; 1000 nodes after many rounds can exceed 5000 events easily. Do not stop validation planning at the synthetic event count.

## 4. Make multi-round verification revisions explicit

A current ValidatedBy plus ValidationBatchID is not a record of independent review rounds: it is the last currently projected acceptance (**apply.go:193–195,481–483**). Audit needs round/batch attribution, proof contributor history, the reviewed revision/support basis, unresolved disagreements, and which verdicts became stale.

A changed cited statement can leave every consumer's own ContentHash unchanged. A changed dependency set can invalidate old computational evidence too: HasPassingClaimTest is consulted on acceptance, while recorded claim-test results carry no node hash (**proof.go:850–852; apply.go:640–655**). Clarify which evidence remains applicable after correction. These are direct consequences of multi-round verification, not future formal-kernel luxuries.

## 5. Treat the machine interface as a product surface

Ship:

- Capability/policy negotiation and version/commit reporting.
- Stable structured error codes for blocked, stale, invalid, incompatible, and partially committed outcomes. Avoid new message-substring classifiers like **verdicts_apply.go:210–231**.
- Per-item committed event sequences, returned node revisions/hashes, operation IDs, and retry guidance.
- Dry-run/preflight of a correction manifest with scope/cycle paths and impacted consumers.
- Bounded, filterable audit/trace output and shared completion/readiness predicates.
- A current-support failure for a validated node with new blocking challenges, not merely a clean taint bit. The graph closed projection already recognizes this distinction (**graph.go:116–124**).

These reduce wasted LLM turns and make an operator able to diagnose the proof without rereading thousands of events.

# E. Release ordering

**The corrections-before-taint principle is right. The proposed contents of 0.1.9 and the dedicated hash-chain 0.1.11 are wrong.**

| Release | Recommended changes to the proposed scope |
|---|---|
| **0.1.9** | Keep dependency correction and minimal validation-hash provenance. Fix the actual expect_hash race and shared commit path they depend on. Resolve D1's assumption/result distinction with focused fixtures. Include the small useful audit/preflight subset and remove false fatigue language. Keep single/bulk eligibility aligned and correct admitted-parent instructions. Ship a usable resumable correction workflow, format activation protocol, consumer/help updates, and a real tag. |
| **0.1.10** | Ship corrected dependency-support taint with its independent spec, reverse-dependent updates, trace explanations, export capability changes, and migration diff tooling. Expand audit and contributor/archive safeguards. Coordinate supported writers; do not rely on a shared 1.1 format to enforce policy. |
| **0.1.11** | Do not reserve this release for D12. Use it for measured remaining concurrency/round-scale problems or completion of verification-basis/evidence tracking. Defer cryptographic tamper evidence until there is an anchor and an agreed durability protocol. |

Move D7's cheap diagnostic correction and D8's minimal preflight **forward**, since migration without inspection is poor operations. Move D5 only with the claim/commit fix, not as two convenient appends. Centralizing D9's existing reviewer check is small enough to pull forward, but do not let an expanded identity system delay the edge fix.

Do not stage “format support first” as if deploying a tag upgrades running workers. The compatibility mechanism and the first incompatible operation can ship together, with deliberate workspace activation after writers are upgraded. Conversely, if implementing genuinely atomic multi-event correction would delay the first fix substantially, ship a clearly scoped pending/reopened-node primitive with an exclusive, resumable operator migration tool, not a falsely atomic reopen option.

Cut blanket assumptions, a hard-coded timing budget, and security marketing before cutting correctness. The requirement is to ship on 0.1.x; it does not require preserving the exact three release buckets.

# F. The issue author's likely reaction

The author asked for a decision and a way to put **21 already audited corrections on the actual graph**, preserving IDs/history. They explicitly offered a tests-first patch and accepted any clear policy. I would not pretend they demanded policy (c), nor that re-verification itself would surprise them.

What would disappoint them:

- “All 21 can be corrected” without testing the actual corrections against the new scope/cycle interpretation.
- Requiring manual unvalidate/amend/reaccept choreography after twelve review rounds, with no real arbitrary-node batch correction path.
- Calling unvalidate --batch the migration solution when their nodes may not share a recorded batch ID.
- No dependency-aware diff since the challenge, leaving the verifier unable to see exactly what was repaired.
- An audit that keeps flagging corrected-and-reverified nodes forever.
- Dependency taint arriving with a recurrence that misses admitted proof children of reused lemmas, or export lacking validation prerequisites.
- Spending the first release on broad hardening while delaying the narrow patch they offered.
- A surprise workspace incompatibility or instructions that do not work with their existing orchestration.

**The one change that would most improve their experience:** make “apply this reviewed correction manifest to these 21 existing nodes” the acceptance test and the first-class workflow. It should preview exact edge diffs, retain IDs/challenges/history, explicitly reopen affected current validations, apply each item safely and idempotently, return new review inputs and hashes, and produce a final graph diff plus an audit showing which findings are resolved. Re-verification remains explicit and can focus on the changed reasoning/support, rather than rediscovering the corrections from hints.

Validate this on a copy of their actual ledger if made available; the supplied issue is not the full 21-item correction dataset. Publish the concrete policy/spec and invite the offered tests-first contribution promptly. The missing beads are an internal tracking repair, not a reason to delay the GitHub answer (**plan: “What the issue author gets”; supplied issue: final proposal/design questions**).

# G. Top five changes, ranked

1. **Fix the real commit and stale-verdict races first.** Put expected-hash/readiness validation and commit on one state version; replace first-CAS/rest-unconditional batches with a defined recoverable protocol. These bugs can silently accept stale proofs today and undermine the proposed migration (**verdicts_apply.go:166–202; proof.go:1883–1927**).
2. **Replace D1's blanket ancestor prohibition with typed support and scope semantics.** Permit introduced hypotheses in induction/cases; reject use of an ancestor's unproved conclusion. Cover every creation/amendment path and give legacy workspaces a repair route (**concepts.md:418–477; proof.go:716,727,1733–1794**).
3. **Correct D6 to propagate complete intrinsic proof support through mixed child/dependency paths.** Preserve sibling isolation, make missing/refuted support explicit, and share the derived support/completion policy across audit, eligibility, exports, and auto-prove (**taint/propagate.go:145–171; graph.go:116–124; auto-prove.sh:663–666**).
4. **Make dependency correction a resumable batch workflow for validated and reopened nodes.** Support explicit reopening, idempotent no-op outcomes, revision preconditions, dependency-aware diff, and a minimal migration audit in the first release. This is the fastest route to the issue author's actual success criterion (**proof.go:1970–1990,2305–2309; cmd/af/diff.go:83–129; supplied issue: 21 corrected nodes**).
5. **Replace the metadata-stamp assurance with a tested compatibility/activation protocol, and defer D12.** Verify actual old-binary behavior, coordinate writers, pin corpus inputs, advertise semantic capabilities, and specify crash/downgrade behavior. The scratch test demonstrates that the existing 1.1 stamp is not a fence (**config.go:57–89,147–149; proof.go:154–171,299–304; review-checks/format-check-results.json**).

**Validation performed:** successful build of ./cmd/af into the scratchpad; disposable CLI checks for 1.1 stamp behavior, prerequisite reopening, and child veto. Detailed command/output records are in review-checks/format-check-results.json and review-checks/invariant-check-results.json. The concurrency findings are established by source-level interleavings, not claimed as stress-test results.
