# REPORT — Plan items D7 and D11 (scale-hardening v3.1, 0.1.10)

Worktree: `vibefeld-wt-d7`, branch `work/d7-d11-health-accuracy`, based on `f736885`
(handoff after v0.1.9). Work is committed locally; **not pushed** (per the brief).

Quality gates at HEAD (`4f0e46c`):

```
gofmt -l cmd internal e2e      # clean
go build ./cmd/af              # ok
go vet ./...                   # ok
go test ./...                  # all packages ok
go test -tags integration ./e2e/ -run TestAutoProveStub   # ok
bash -n scripts/auto-prove.sh scripts/test-auto-prove.sh  # ok
```

Commits (newest last / topic order):

| Commit | Topic |
|---|---|
| `1a2a92c` | D11: one jobs classifier for status/get; removed-flag help examples; acceptance/claim fields in `af get` |
| `2edeabc` | D7: descriptive per-node rework; blockers for open challenges and stalled/stale claims |
| `c0d039a` | D11: auto-prove `--with-note`, per-worker identities, real completion test; stub-agent regression test |
| `3c39dd5` | D7: `repair-stats` no longer infers falsity from sustained rework |
| `9da78bf` | D11: `af get` `expires_at` alias + acceptance/claim tests |
| `cbcc8a2` | D11: stub test also covers prover refine/amend and `accept --confirm` |
| `a66ba80` | docs: cli-reference refine examples use positional statements |
| `4f0e46c` | D7: health JSON emits `rework: []`, not `null` |

---

## D7 — Health stops calling scrutiny evidence of falsity

`cmd/af/health.go`, `internal/state/repair.go`, `cmd/af/repair_stats.go`.

### What changed

1. **Subtree alarm removed from health.** `analyzeHealth` no longer calls the
   absolute `FindFatiguedSubtrees`/`ClassifyFatigue` alarm, and health no longer
   prints "Fatigued subtrees" or any "probably false" suggestion. The
   `HealthStatistics.FatiguedSubtrees` field is kept (always 0) so the JSON
   schema does not lose a key; new fields were added instead of renaming.
2. **Descriptive per-node rework.** New `state.ReworkMetrics` /
   `(*State).GetReworkMetrics` counts, per node:
   - `resolved_challenges` (challenges with status `resolved`),
   - `amendments` (statement **and** dependency amendments from
     `GetAmendmentHistory`),
   - `refuted_children` (direct children with epistemic state `refuted`),
   - `rework` = the sum.
   Health lists the top N nodes by rework (`--hotspots`, default 5) and flags a
   hotspot as a warning when `rework >= --rework-warn` (default 5 per node).
   Text output explains each hotspot: *"N resolved challenge(s) on this node;
   this is rework, not evidence of falsity"*. Levels are only `info`/`warning`.
3. **Blockers** now include:
   - each open challenge, with `severity` and `age` (age from the challenge
     `created` timestamp), level `info`;
   - **stalled claims** — still active but held longer than the configured lock
     timeout — with owner, age and expiry, level `warning`;
   - **stale claims** — expiry in the past — with owner and expiry, level
     `warning`.
   To distinguish acquisition from expiry, `node.Node` gained
   `ClaimedSince types.Timestamp` (the `NodesClaimed` event time), set by
   `applyNodesClaimed` and cleared on release; `ClaimedAt` remains the expiry.
   This field is excluded from the content hash.
4. **Help + docs + changelog.** `af health` help documents `--hotspots` and
   `--rework-warn` and the "rework is not falsity" policy.
   `docs/cli-reference.md` health section rewritten. `cmd/af/changelog.go` has a
   new `0.1.10` entry marked `Unreleased: true` (so the version test keeps
   matching the newest released entry, `0.1.9`).
5. **`repair-stats` language softened.** The separate `af repair-stats` command
   keeps its (tested) subtree scan, but its help and alarm text no longer infer
   that the conjecture/parent is false.

### Tests

`cmd/af/health_rework_test.go`:

- `TestHealthCmd_ReworkFlags` — `--hotspots`/`--rework-warn` exist, default 5.
- `TestAnalyzeHealth_ReworkIsDescriptive` — 6 resolved challenges + 1
  dependency amendment → 7 rework, warned, no "probably false", no "Fatigued",
  explanation present.
- `TestAnalyzeHealth_HotspotLimit` — top-N ordering and total rework-node count.
- `TestAnalyzeHealth_ClaimBlockers` — stalled and stale claims with owner/expiry.
- `TestAnalyzeHealth_OpenChallengesHaveSeverityAndAge` — info-level blocker with
  severity and age.

---

## D11 — Agent-facing accuracy

### 5. One jobs classifier

The render-side workflow heuristic was deleted from every surface that drove a
Prover/Verifier count:

- `internal/render/status.go`: `renderJobs` and `FilterUrgentNodes` now call
  `internal/jobs.FindJobs` / `FindProverJobs` / `FindVerifierJobs` via the new
  `jobCountsForState` helper, computed over **the whole state** so it matches
  `af jobs` even when the tree display is paginated or filtered.
- `internal/render/json.go` (`statusToJSON`, `statusToJSONWithPagination`) and
  `internal/render/adapters.go` (`StateToStatusView`) use the same helper.
- `internal/jobs/verifier.go` exports `IsVerifierJob`; `internal/service`
  re-exports `IsProverJob`/`IsVerifierJob`.
- `af get` now reports `prover_ready`/`verifier_ready` from that classifier.
- `af export --graph json` already used `internal/jobs`; `af health` continues
  to use it via `service.FindJobs`.

`cmd/af/status_jobs_consistency_test.go` (`TestStatusAndJobsCountsAgree`) builds
a mixed-state workspace and asserts the `af status` JSON job counts equal the
`af jobs` JSON list lengths.

### 6. Removed-flag help examples

- `cmd/af/main.go` workflow: `af refine 1 --owner prover-1 -s "..."` →
  `af refine 1 "..." --owner prover-1` (both occurrences); `af challenge 1.1
  --owner verifier-1 ...` → `af challenge 1.1 --reason ...` (challenge has no
  `--owner`).
- `internal/render/examples.go` refine examples use positional statements;
  `resolve-challenge` example uses the real `--response` flag.
- `docs/cli-reference.md` `refine` flag table/examples no longer document the
  removed `--statement`/`-s`/`--sibling` flags.

### 7. `af get` surfaces acceptance and claim detail

`cmd/af/get.go` (JSON) and `internal/render/node.go` (`RenderNodeVerbose`, text):

- `validated_by`, `validation_batch_id` (when recorded),
- `claimed_by`, `claimed_at` (acquisition, `ClaimedSince`),
  `claim_expires_at` / `expires_at` (expiry),
- `author`, `proof_author`, and `prover_ready`/`verifier_ready`.

`cmd/af/get_acceptance_fields_test.go` covers the JSON keys end to end.

### 8. `scripts/auto-prove.sh`

- `accept --note` → `accept --agent <worker> --with-note ... --confirm`.
- Every generated command carries a per-worker identity: `--owner` on
  `claim`/`release`/`refine`/`amend`, `--agent` on `accept`, and
  `AF_AGENT_ID=<worker>` for `challenge`/`resolve-challenge` (which have no
  identity flag). The generated `resolve-challenge` uses `--response`, not the
  removed `--note`.
- Completion no longer reads the root's epistemic state alone. The new
  `proof_complete` requires root `validated` **and** root taint `clean` **and**
  no pending external reference cited by a validated node (cross-check of
  `af status -f json` context against `af pending-refs -f json`). D4's
  `support_current` is feature-detected: if the field is present on the root it
  must be `true`, otherwise it is not required.
- `AF_CMD` may be supplied by the caller (needed by the test) instead of only
  being auto-detected.
- `scripts/test-auto-prove.sh`: creates a disposable workspace, installs a stub
  `claude` that extracts the generated af commands from the prompt and executes
  them, runs both a verifier scenario and a prover scenario, and fails on any
  `unknown flag` / `flag provided but not defined` / `unknown shorthand flag`,
  or if no generated command ran.
- `e2e/auto_prove_stub_test.go` (`TestAutoProveStub`, build tag `integration`)
  runs that script and is skipped unless `bash >= 4` is available.

### 9. Docs / changelog

`docs/cli-reference.md` updated for `status`, `get` and `health`; changelog
lines added under the `0.1.10` (unreleased) entry.

---

## Notes / deviations

- The brief said not to implement `support_current` and not to touch
  `internal/support` or the accept paths. Neither was touched. Health's
  `analyzeHealth` signature gained an options value
  (`healthOptions{ReworkWarn, Hotspots, LockTimeout}`); the D4 branch's
  `cmd/af/health_support.go` will need to pass those options (or call the new
  helper) when it lands.
- `FindFatiguedSubtrees`/`ClassifyFatigue` still exist for the separate
  `af repair-stats` command (its tests depend on them); only the falsity
  inference was removed there. Health no longer uses them at all.
- `--hotspots 0` currently means "show all"; `--hotspots N>0` means top N.
- The `0.1.10` changelog entry is `Unreleased: true`, so
  `TestVersionInfo_MatchesLatestChangelogEntry` still anchors `VersionInfo`
  (still `0.1.9`) to the newest released entry.

## Handoff

- Close `vibefeld-429g` (D7) with `ywsu`; close `vibefeld-7ze3` (D11) with
  `ujp4`, `c8yb`, `xr7g`, `0l3d` once this branch is reviewed/merged.
- Merge coordination: D4 (`support_current`) also touches `cmd/af/health.go` via
  a new `cmd/af/health_support.go`. Rebase D4 onto this branch and pass
  `healthOptions` (or thread `support_current` through `analyzeHealth`) before
  adding its blockers.
- No push was performed, per the brief.
