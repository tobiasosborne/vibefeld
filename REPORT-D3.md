# REPORT D3 — Acceptance records what it accepted, and whether it was checked

Bead: `vibefeld-w3vz` · Plan: `docs/plans/scale-hardening.md` §D3

## What landed

Optional, `omitempty` only; no shape changes; the legacy hash algorithm was not
touched.

### Events (`internal/ledger/event.go`)
- `NodeValidated.ContentHash string  json:"content_hash,omitempty"` — the hash
  accepted, read from the same state the accept commits against.
- `NodeValidated.ExpectedHashChecked bool  json:"expected_hash_checked,omitempty"` —
  true only when a caller-supplied expectation was compared.
- `ClaimTested.ContentHash string  json:"content_hash,omitempty"` — the node's
  hash at test time.
- New `NewNodeValidatedWithHash(...)`; the existing constructors leave the new
  fields unrecorded (legacy events replay identically).

### State / node
- `node.Node.ValidatedContentHash` / `ValidatedHashChecked`
  (`internal/node/node.go`), set by `applyNodeValidated`, cleared by
  `applyNodeUnvalidated`.
- `state.ClaimTestResult.ContentHash`, set by `applyClaimTested`.
- `HasPassingClaimTestForContent(id, hash)` counts a passing test only when its
  recorded hash is empty (legacy) or equals the current hash.
  `HasStalePassingClaimTest(id, hash)` names the stale case. `HasPassingClaimTest`
  is kept (deprecated) as a legacy-compatible wrapper.

### Accept paths (`internal/service`)
- `buildAcceptEvents` now takes `expectHash`; when non-empty it is compared in
  the same state read (mismatch → `ErrInvalidState`), and the event records
  `n.ContentHash` plus `ExpectedHashChecked = expectHash != ""`.
- `AcceptNodeWithExpectation` is the new single-node entry point;
  `AcceptNodeWithVerifier` delegates with no expectation (checked=false).
- Bulk accept records each node's hash with checked=false (no per-node flag).
- `applyAcceptVerdict` passes `item.ExpectHash` through.
- Both crux gates now use `HasPassingClaimTestForContent`; a stale-only pass
  is refused with an error that says the test is stale.
- `RunClaimTest` records the node's current `ContentHash`.

### CLI
- `af accept --expect-hash <hash>` (single node only, enforced in validation).
- `af get` (verbose text + JSON) shows
  `Accepted content hash: <h> (checked|recorded)` when present.
- `af claim-tests` marks a test `[stale: recorded for hash <h>]` / `"stale": true`
  when its hash differs from the node's current content. (The brief's
  "af get marks a claim test as stale" is implemented on the claim-test view,
  `af claim-tests`, because `af get` does not display claim tests; `af get`
  shows the accepted hash as required.)
- `af export --graph json` adds `validated_content_hash` and
  `validated_hash_checked` per node, omitted when unset.

### Docs
- `docs/concepts.md`: paragraph on exactly what the accepted hash covers (own
  fields + dependency IDs; not dependency contents, children, scope, evidence or
  external references; `support_current` handles result-use staleness).
- `cmd/af/changelog.go`: 0.1.9 entry.

## Tests (all in `go test ./...`, no build tag except where noted)
- `internal/ledger/content_hash_events_test.go` — constructors, wire key names,
  additive omission, legacy JSON decode for both events.
- `internal/state/content_hash_apply_test.go` — apply sets node/test fields;
  legacy event and legacy JSON replay leave them empty; unvalidate clears;
  matching/legacy/stale claim-test counting.
- `internal/service/accept_content_hash_test.go` — plain accept records hash and
  checked=false; `AcceptNodeWithExpectation` records checked=true and refuses a
  mismatch without writing an event; verdict `expect_hash` records checked=true;
  a stale crux claim test is ignored and its error names the staleness, while a
  legacy test counts.
- `internal/export/graph_content_hash_test.go` — export includes the new fields
  and omits them when unset.
- `cmd/af/accept_expect_hash_test.go` — the new CLI flag threads through
  (checked=true) and a mismatch is refused.

## Gates
`gofmt` clean · `go build ./cmd/af` · `go vet ./...` · `go test ./...` all pass.

## Notes / follow-ups
- `af export --graph json` does not advertise a new capability token for these
  fields; D2's plan text scopes capability flags to the amendment/validation-dep
  fields. Add one if a driver needs to detect the capability.
- The `af get` "stale claim test" wording in the brief was mapped to
  `af claim-tests`; see above.

## Review fixes

An independent review of the D3 branch found four gaps; all were fixed on
`work/d3-accept-hash` (tests added first, `gofmt`/`go vet`/`go test ./...`
green).

1. **`state.HasPassingClaimTest` rejected hash-bearing passes.** The deprecated
   legacy wrapper delegated to `HasPassingClaimTestForContent(id, "")`, and an
   empty current hash never equals a non-empty recorded hash, so every new
   hash-bearing passing test counted as stale. Restored the original
   any-passing-test loop. Regression test
   `TestHasPassingClaimTest_CountsHashBearingPass` covers a non-empty test hash.
2. **Single-node accept discarded the stale diagnosis.** `af accept` matched
   `"claim-test"` in the error string and printed the generic "no passing
   claim-test" text, and `ErrClaimTestRequired` shared the `NODE_BLOCKED` code
   with `ErrBlockingChallenges`, so `errors.Is` could not tell them apart. Added
   a distinct `CLAIM_TEST_REQUIRED` error code, kept `ErrClaimTestRequired` on it,
   and added a plain `ErrClaimTestStale` sentinel wrapped alongside it. The CLI
   and `verdicts_apply` now classify with `errors.Is`; single and bulk accept
   preserve the service error and name the staleness.
   `TestAcceptCmd_StaleCruxClaimTestNamesStaleness` asserts the CLI output names
   the stale test and does not fall back to the generic message.
3. **Hash recording in `RunClaimTest` was untested.** Added
   `TestRunClaimTest_RecordsHashAndAcceptRejectsAfterAmend`: it runs a real
   passing script, asserts the recorded `ClaimTested.ContentHash` equals the
   node's `ContentHash`, amends the node (changing its hash), and asserts accept
   now rejects the test as stale.
4. **Bulk accept had no hash/staleness coverage.** Added
   `TestAcceptNodeBulk_RecordsHashCheckedFalse` (every bulk `NodeValidated`
   records its node's hash with `checked=false`, on the event and applied node)
   and `TestAcceptNodeBulk_CruxStaleClaimTestRejected` (the bulk crux gate
   refuses a stale-only passing test and writes no event).
