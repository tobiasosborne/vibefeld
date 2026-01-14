# Handoff - 2026-01-14 (Session 35)

## What Was Accomplished This Session

### Bug Fix: vibefeld-99ab - Verifier Jobs Not Showing

Fixed a critical bug where `af jobs` showed refined-and-released nodes as "prover jobs" instead of "verifier jobs".

**Root Cause**: The old logic required `WorkflowState="claimed"` for verifier jobs. When a prover releases a node after refining it, the node goes back to `WorkflowState="available"`, so it couldn't appear as a verifier job.

**The Fix**:
- **Verifier jobs** now require: `pending + not blocked + HAS children + all children validated`
- **Prover jobs** now exclude nodes that qualify as verifier jobs
- Both `available` and `claimed` nodes can now be verifier jobs

| File | Changes |
|------|---------|
| `internal/jobs/verifier.go` | Removed claimed requirement, require has children + all validated |
| `internal/jobs/prover.go` | Added nodeMap param, exclude verifier-ready nodes |
| `internal/jobs/jobs.go` | Pass nodeMap to both functions |
| `internal/jobs/*_test.go` | Updated tests + regression test for exact bug scenario |
| `cmd/af/jobs.go` | Updated help text to document new criteria |

**Issue Closed**: vibefeld-99ab

## Current State

### Test Status
```bash
go build ./cmd/af    # PASSES
go test ./...        # All tests PASS
```

### Key Design Decision
Leaf nodes (no children) remain **prover jobs**, not verifier jobs. A verifier job must have children and all children must be validated. This ensures fresh nodes need refinement before verification.

## Next Steps (Priority Order)

### P1 - Discoverability Improvements
1. **vibefeld-435t** - Add `af inferences` command (agents need this)

### P2 - Implementation for TDD Tests Ready
1. **vibefeld-swn9** - Implement af verify-external (tests ready)
2. **vibefeld-hmnt** - Implement af extract-lemma (tests ready)

### P2 - Bug Fixes
1. **vibefeld-yxhf** - Add state validation to accept/admit/archive/refute commands
2. **vibefeld-dwdh** - Fix nil pointer panic in refute test
3. **vibefeld-o9op** - Auto-compute taint after validation events

### P2 - Additional Work
1. **vibefeld-23e6** - Add `af types` command
2. **vibefeld-rimp** - Show valid inferences in `af refine --help`
3. **vibefeld-b0yc** - Write tests for lemma independence criteria
4. **vibefeld-t1io** - Implement lemma independence validation
5. **vibefeld-v0ux** - Better error messages when refining unclaimed nodes

## Session History

**Session 35:** Fixed vibefeld-99ab - verifier jobs not showing for released refined nodes
**Session 34:** √2 proof with adversarial agents + 5 improvement issues filed
**Session 33:** 8 issues + readiness assessment + √2 proof demo + supervisor prompts
**Session 32:** Fixed init bug across 14 test files, created 2 issues for remaining failures
**Session 31:** 4 issues via 4 parallel agents
**Session 30:** 11 issues total (7 via 5 agents + 4 via 4 agents)
**Session 29:** 7 issues total (5 via parallel agents + 2 P0 bug fixes)
**Session 28:** 5 issues via 5 parallel agents + architecture fix
**Session 27:** 5 issues via 5 parallel agents
**Session 26:** 5 issues via 5 parallel agents + lock manager fix
**Session 25:** 9 issues via parallel agents
**Session 24:** 5 E2E test files via parallel agents
