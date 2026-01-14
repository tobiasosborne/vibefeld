# AF Architectural Remediation Plan

**Version**: 1.0
**Date**: 2026-01-14
**Status**: Active
**Tracking**: All steps tracked in beads issue tracker with prefix `vibefeld-`

## Executive Summary

This plan addresses the architectural failures identified in the Dobinski proof attempt and subsequent deep analysis. The AF tool has sound foundational infrastructure (event sourcing, append-only ledger, CAS concurrency) but suffers from:

1. **Inverted workflow model** - bottom-up instead of breadth-first adversarial verification
2. **Incomplete validation invariant** - only 1 of 4 PRD requirements implemented
3. **Disconnected rendering infrastructure** - comprehensive context rendering built but never wired to CLI
4. **Missing state machine triggers** - taint, challenge supersession not automatically propagated

## Guiding Principles

1. **Fix the model first** - workflow inversion must be corrected before other fixes make sense
2. **Wire before writing** - use existing infrastructure before building new
3. **Test adversarially** - add E2E tests that verify the adversarial workflow
4. **Incremental validation** - each phase should produce a testable improvement

---

## Phase 0: Workflow Model Correction

**Goal**: Transform from bottom-up cooperative model to breadth-first adversarial model.

### Step 0.1: Redefine Verifier Job Detection
**Issue**: vibefeld-9jgk (existing P0)
**Location**: `internal/jobs/verifier.go`

**Current Logic**:
```go
// A node is a verifier job when:
// - Not blocked
// - Pending epistemic state
// - Has children AND all children validated
```

**New Logic**:
```go
// A node is a verifier job when:
// - Not blocked
// - Has a statement (was refined/created)
// - Pending epistemic state
// - Not currently claimed
// - Has no unresolved challenges OR has addressed challenges ready for review
```

**Rationale**: Every new node should immediately be reviewable by verifiers. Challenges create prover jobs; acceptance clears the node.

### Step 0.2: Redefine Prover Job Detection
**Issue**: vibefeld-9jgk (same issue - both detections change together)
**Location**: `internal/jobs/prover.go`

**Current Logic**:
```go
// A node is a prover job when:
// - Available workflow state
// - Pending epistemic state
// - NOT a verifier job (doesn't have all validated children)
```

**New Logic**:
```go
// A node is a prover job when:
// - Available or claimed by a prover
// - Pending epistemic state
// - Has unaddressed challenges requiring response
```

**Rationale**: Provers work on challenged nodes. Unchallenged nodes are verifier territory.

### Step 0.3: Remove "Mark Complete" Requirement
**Issue**: vibefeld-heir (existing P0) - may be CLOSED if Step 0.1 makes it unnecessary
**Evaluation**: With breadth-first model, every node is immediately verifiable. The concept of "marking complete" becomes unnecessary - verifiers decide when a node is complete by accepting it.

**Action**: Evaluate after Step 0.1/0.2 implementation. May close as "won't fix - addressed by model change."

### Step 0.4: Add E2E Test for Adversarial Workflow
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `e2e/adversarial_workflow_test.go`

**Test Scenario**:
```
1. Init proof with conjecture
2. Root node (1) should be verifier job immediately
3. Verifier claims, challenges root → now prover job
4. Prover refines (adds 1.1) → 1.1 is verifier job, 1 is still prover job (challenge open)
5. Verifier accepts 1.1 → 1.1 validated
6. Prover adds 1.2 addressing challenge → 1.2 is verifier job
7. Verifier accepts 1.2, resolves challenge on 1 → 1 becomes verifier job again
8. Verifier accepts 1 → proof complete
```

---

## Phase 1: Validation Invariant Completion

**Goal**: Implement all 4 PRD-specified requirements for node validation.

### Step 1.1: Fix Admitted Children Bug
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `internal/node/validate_invariant.go:47`

**Current Code**:
```go
if child.EpistemicState != schema.EpistemicValidated {
    return fmt.Errorf("validation invariant violated")
}
```

**Fixed Code**:
```go
if child.EpistemicState != schema.EpistemicValidated &&
   child.EpistemicState != schema.EpistemicAdmitted {
    return fmt.Errorf("validation invariant violated: child %s has state %s (requires validated or admitted)",
        child.ID, child.EpistemicState)
}
```

**Rationale**: PRD (p.185) explicitly states children may be "validated OR admitted." This is the escape hatch mechanism.

### Step 1.2: Add Challenge State Validation
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `internal/node/validate_invariant.go`

**New Function Signature**:
```go
func CheckValidationInvariant(
    n *Node,
    getChildren func(types.NodeID) []*Node,
    getChallenges func(types.NodeID) []*Challenge,  // NEW
) error
```

**New Checks**:
1. All challenges on node have state ∈ {resolved, withdrawn, superseded}
2. For each resolved challenge, at least one node in `addressed_by` has epistemic_state = validated

### Step 1.3: Add Scope Entry Validation
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `internal/node/validate_invariant.go`
**Depends on**: vibefeld-6um6 (assumption scope tracking)

**New Check**: If node type is `local_assume`, verify all scope entries it opens are closed by descendants before validation.

### Step 1.4: Implement Challenge Supersession
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `internal/ledger/events.go`, `internal/state/apply.go`

**New Event**: `ChallengeSuperseded`
**Trigger**: When a node is archived or refuted, all challenges on that node and descendants automatically become superseded.

**Implementation**:
```go
func applyNodeArchived(s *State, e *Event) error {
    // ... existing logic ...
    // NEW: Supersede all challenges on this node and descendants
    for _, challenge := range node.Challenges {
        if challenge.Status == "open" {
            challenge.Status = "superseded"
        }
    }
    return nil
}
```

---

## Phase 2: CLI-Rendering Integration

**Goal**: Wire existing rendering infrastructure to CLI commands.

### Step 2.1: Wire Prover Context to Claim Command
**Issue**: vibefeld-h0ck (existing P0) - partial coverage
**Location**: `cmd/af/claim.go`

**Current Output** (lines ~110-120):
```go
cmd.Printf("Claimed node %s\n", nodeID.String())
cmd.Printf("  Owner:   %s\n", owner)
```

**New Implementation**:
```go
// After successful claim, render full context
ctx := render.BuildProverContext(node, state, definitions, assumptions, externals)
output := render.RenderProverContext(ctx)
cmd.Println(output)
```

### Step 2.2: Wire Verifier Context to Claim Command
**Issue**: vibefeld-h0ck (same issue)
**Location**: `cmd/af/claim.go`

Detect role from `--role` flag and render appropriate context:
```go
switch role {
case "prover":
    ctx := render.BuildProverContext(...)
    cmd.Println(render.RenderProverContext(ctx))
case "verifier":
    ctx := render.BuildVerifierContext(...)
    cmd.Println(render.RenderVerifierContext(ctx))
}
```

### Step 2.3: Wire Error Recovery to All Commands
**Issue**: vibefeld-04p8 (existing P2 - upgrade to P1)
**Location**: All `cmd/af/*.go` files

**Pattern to Apply**:
```go
// Current:
return fmt.Errorf("failed to claim node: %w", err)

// New:
errInfo := render.FormatCLI(err, errors.CodeFromError(err))
cmd.PrintErrln(errInfo)
return err
```

### Step 2.4: Enhance Jobs Output with Full Context
**Issue**: vibefeld-we4t (existing P1)
**Location**: `cmd/af/jobs.go`, `internal/render/jobs.go`

**Enhancement**: Don't truncate. Show full statement, challenges, referenced definitions.

---

## Phase 3: State Machine Completion

**Goal**: Ensure all state machines are properly triggered and propagated.

### Step 3.1: Add Workflow Transition Validation During Replay
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `internal/state/apply.go`

**Current Code** (applyNodesClaimed):
```go
n.WorkflowState = schema.WorkflowClaimed  // No validation!
```

**Fixed Code**:
```go
if err := schema.ValidateWorkflowTransition(n.WorkflowState, schema.WorkflowClaimed); err != nil {
    return fmt.Errorf("invalid workflow transition for node %s: %w", nodeID, err)
}
n.WorkflowState = schema.WorkflowClaimed
```

### Step 3.2: Auto-Trigger Taint Recomputation
**Issue**: vibefeld-lyz0 (existing P1)
**Location**: `internal/state/apply.go`

**Trigger Points**: After any epistemic state change event:
- `applyNodeValidated()`
- `applyNodeAdmitted()`
- `applyNodeRefuted()`
- `applyNodeArchived()`

**Implementation**:
```go
func applyNodeValidated(s *State, e *Event) error {
    // ... existing logic ...

    // NEW: Recompute taint for this node and propagate to descendants
    ancestors := s.GetAncestors(node.ID)
    newTaint := taint.ComputeTaint(node, ancestors)
    if node.TaintState != newTaint {
        node.TaintState = newTaint
        changed := taint.PropagateTaint(node, s.AllNodes())
        // Optionally append TaintRecomputed event
    }
    return nil
}
```

### Step 3.3: Document State Machine Transitions
**Issue**: vibefeld-wuo4 (existing P2)
**Location**: `docs/state-machines.md` (new file)

Create comprehensive documentation of:
- Workflow state machine (available ↔ claimed ↔ blocked)
- Epistemic state machine (pending → validated/admitted/refuted/archived)
- Taint computation rules
- Challenge state machine (open → resolved/withdrawn/superseded)

---

## Phase 4: Concurrency Hardening

**Goal**: Eliminate race conditions and improve multi-agent scenarios.

### Step 4.1: Add Service-Level Child ID Allocation
**Issue**: vibefeld-hrap (existing P1)
**Location**: `internal/service/proof.go`

**New Method**:
```go
func (s *ProofService) AllocateChildID(parentID types.NodeID) (types.NodeID, error) {
    // Within ledger lock:
    // 1. Load state
    // 2. Find next available child number
    // 3. Reserve it (append reservation event or return with sequence)
    // 4. Return allocated ID
}
```

### Step 4.2: Add Bulk Refinement
**Issue**: vibefeld-9ayl (existing P0), vibefeld-q9ez (existing P2)
**Location**: `internal/service/proof.go`

**New Method**:
```go
func (s *ProofService) RefineNodeBulk(
    parentID types.NodeID,
    owner string,
    children []ChildSpec,
) ([]types.NodeID, error) {
    // Single CAS operation that:
    // 1. Validates parent is claimed by owner
    // 2. Allocates child IDs atomically
    // 3. Creates all child nodes in one ledger append
}
```

### Step 4.3: Implement Automatic Claim Reaping
**Issue**: vibefeld-pbtp (existing P1) - related
**Location**: `internal/service/proof.go`

**Implementation**: Claims store expiration timestamp. `LoadState()` or separate `ReapExpiredClaims()` automatically releases claims past their timeout.

---

## Phase 5: Challenge Workflow Clarity

**Goal**: Make the challenge-response workflow explicit and documented.

### Step 5.1: Document Challenge Workflow
**Issue**: vibefeld-ccvo (existing P1)
**Location**: `docs/challenge-workflow.md` (new file)

Document the complete flow:
1. Verifier raises challenge (node gets challenge, becomes prover job)
2. Prover sees challenge in job context
3. Prover adds children that `addresses_challenges: [ch-xxx]`
4. Challenge `addressed_by` updated automatically
5. Verifier sees addressed challenge, can resolve or raise new challenge
6. Resolution requires: resolved challenge + addressed_by nodes validated
7. Withdrawn: verifier retracts (acknowledges was wrong)
8. Superseded: automatic when parent archived/refuted

### Step 5.2: Add `af challenges` Command
**Issue**: vibefeld-vyus (existing P1)
**Location**: `cmd/af/challenges.go` (new file)

**Features**:
```bash
af challenges                    # All open challenges
af challenges --node 1.1.1       # Challenges on specific node
af challenges --status open      # Filter by status
af challenges --format json      # Machine-readable
```

### Step 5.3: Show Challenges in Node View
**Issue**: vibefeld-uevz (existing P1)
**Location**: `cmd/af/get.go`, `internal/render/node.go`

Ensure `af get <node-id>` shows:
- All challenges on the node
- Challenge status
- Addressed_by nodes
- For addressed challenges: status of addressing nodes

---

## Phase 6: Integration Testing

**Goal**: Ensure the adversarial workflow works end-to-end.

### Step 6.1: Add Adversarial Workflow E2E Test Suite
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `e2e/`

Test scenarios:
1. **Happy path**: Prover creates, verifier accepts, proof complete
2. **Challenge path**: Prover creates, verifier challenges, prover addresses, verifier accepts
3. **Multi-challenge**: Multiple challenges on same node
4. **Nested challenges**: Challenge on child of challenged node
5. **Supersession**: Archive parent, verify challenges superseded
6. **Escape hatch**: Admit node, verify taint propagates
7. **Concurrent agents**: Multiple provers/verifiers, no races

### Step 6.2: Add Regression Test for Dobinski Scenario
**Issue**: NEW - vibefeld-XXXX (to be created)
**Location**: `e2e/dobinski_regression_test.go`

Reproduce the exact failure scenario from the Dobinski proof attempt and verify it now works.

---

## Dependency Graph

```
Phase 0 (Workflow)
├── Step 0.1: Verifier job detection
├── Step 0.2: Prover job detection
├── Step 0.3: Evaluate mark-complete (depends on 0.1, 0.2)
└── Step 0.4: E2E test (depends on 0.1, 0.2)

Phase 1 (Validation) - can parallel with Phase 0
├── Step 1.1: Admitted children bug
├── Step 1.2: Challenge state validation
├── Step 1.3: Scope validation (depends on vibefeld-6um6)
└── Step 1.4: Challenge supersession

Phase 2 (CLI Integration) - depends on Phase 0
├── Step 2.1: Wire prover context
├── Step 2.2: Wire verifier context
├── Step 2.3: Wire error recovery
└── Step 2.4: Enhanced jobs output

Phase 3 (State Machines) - can parallel with Phase 2
├── Step 3.1: Workflow validation during replay
├── Step 3.2: Auto-trigger taint
└── Step 3.3: Document state machines

Phase 4 (Concurrency) - depends on Phase 0
├── Step 4.1: Child ID allocation
├── Step 4.2: Bulk refinement
└── Step 4.3: Automatic claim reaping

Phase 5 (Challenge Workflow) - depends on Phase 1
├── Step 5.1: Document workflow
├── Step 5.2: af challenges command
└── Step 5.3: Show challenges in node view

Phase 6 (Integration Testing) - depends on all above
├── Step 6.1: E2E test suite
└── Step 6.2: Dobinski regression
```

---

## Issue Mapping

### Existing Issues Covered by This Plan

| Issue ID | Title | Plan Step |
|----------|-------|-----------|
| vibefeld-9jgk | Verifier job detection inverted | 0.1, 0.2 |
| vibefeld-h0ck | Job output lacks context | 2.1, 2.2 |
| vibefeld-heir | No mark-complete mechanism | 0.3 (may close) |
| vibefeld-9ayl | Claim contention | 4.2 |
| vibefeld-hrap | Child ID race condition | 4.1 |
| vibefeld-lyz0 | Taint never resolves | 3.2 |
| vibefeld-ccvo | Challenge workflow opaque | 5.1 |
| vibefeld-vyus | Missing af challenges | 5.2 |
| vibefeld-uevz | Challenges not in node view | 5.3 |
| vibefeld-we4t | Jobs output truncates | 2.4 |
| vibefeld-04p8 | Error messages no guidance | 2.3 |
| vibefeld-wuo4 | State machines undocumented | 3.3 |
| vibefeld-q9ez | No bulk operations | 4.2 |
| vibefeld-pbtp | Claim timeout invisible | 4.3 |

### New Issues Created by This Plan

| Plan Step | Issue ID | Title |
|-----------|----------|-------|
| 0.4 | vibefeld-9tth | E2E test for adversarial breadth-first workflow |
| 1.1 | vibefeld-y0pf | Validation invariant rejects admitted children (PRD violation) |
| 1.2 | vibefeld-ru2t | Validation invariant missing challenge state check |
| 1.3 | vibefeld-1jo3 | Validation invariant missing scope entry check |
| 1.4 | vibefeld-g58b | Challenge supersession not implemented |
| 3.1 | vibefeld-f353 | Workflow transitions not validated during state replay |
| 6.1 | vibefeld-wzwp | Comprehensive E2E test suite for multi-agent scenarios |
| 6.2 | vibefeld-om5f | Dobinski proof regression test |

---

## Success Criteria

### Phase 0 Complete When:
- [ ] New node immediately appears as verifier job
- [ ] Challenged node appears as prover job
- [ ] E2E test demonstrates breadth-first workflow

### Phase 1 Complete When:
- [ ] `af accept` works with admitted children
- [ ] `af accept` fails if open challenges exist
- [ ] `af accept` fails if scope not closed
- [ ] Challenges auto-supersede on parent archive/refute

### Phase 2 Complete When:
- [ ] `af claim --role prover` shows full prover context
- [ ] `af claim --role verifier` shows full verifier context
- [ ] All error messages include recovery suggestions

### Phase 3 Complete When:
- [ ] Invalid ledger events rejected during replay
- [ ] Taint updates automatically on state changes
- [ ] State machines documented

### Phase 4 Complete When:
- [ ] No child ID race conditions under concurrent load
- [ ] Multi-child refinement is atomic
- [ ] Expired claims auto-released

### Phase 5 Complete When:
- [ ] `af challenges` command works
- [ ] Challenge workflow fully documented
- [ ] Challenges visible in `af get` output

### Phase 6 Complete When:
- [ ] All E2E tests pass
- [ ] Dobinski scenario works correctly

---

## Estimated Effort

| Phase | Sessions | Dependencies |
|-------|----------|--------------|
| Phase 0 | 1-2 | None |
| Phase 1 | 1-2 | None |
| Phase 2 | 1 | Phase 0 |
| Phase 3 | 1 | None |
| Phase 4 | 1-2 | Phase 0 |
| Phase 5 | 1 | Phase 1 |
| Phase 6 | 1 | All above |

**Total**: 7-11 sessions for full remediation.

**Minimum Viable**: Phases 0 + 1 + 2 (3-5 sessions) produces a usable tool.

---

## References

- PRD: `docs/prd.md`
- Failure Report: `dobinski-proof/FAILURE_REPORT.md`
- Architectural Analysis: Session 37 analysis (this session)
