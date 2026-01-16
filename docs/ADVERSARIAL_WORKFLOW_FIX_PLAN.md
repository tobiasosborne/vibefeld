# AF Tool: Adversarial Workflow Fix Plan

**Date**: 2026-01-16
**Session**: Deep Analysis Session
**Triggered By**: FAILURE_REPORT_SESSION53.md

---

## Executive Summary

A deep multi-agent analysis of the AF codebase reveals that the **core workflow is broken** not due to incorrect logic in job detection (which is correct), but due to:

1. **Missing enforcement at acceptance boundary** - Acceptance doesn't check for blocking challenges
2. **Missing verification context/checklist** - Verifiers have no guidance on what to verify
3. **Tool guidance encourages depth over breadth** - "Next steps" suggests child refinement
4. **Severity system is dead code** - `SeverityBlocksAcceptance()` exists but is never called

**The job detection logic is CORRECT.** The failure report's claim that "nodes only become verifier jobs when ALL children are validated" is **FALSE** - that logic doesn't exist in the codebase. Tests pass (77/77).

---

## Root Cause Analysis

### ROOT CAUSE 1: Acceptance Doesn't Enforce Challenge Resolution (CRITICAL)

**File**: `internal/service/proof.go` lines 761-815

**Current Behavior**:
```go
func (s *ProofService) AcceptNodeWithNote(id types.NodeID, note string) error {
    // ...
    // Checks validation dependencies ✓ (lines 776-794)
    // Validates epistemic transition ✓ (lines 796-799)
    // MISSING: Check for blocking challenges ✗
    // ...
}
```

**Problem**: A verifier can accept a node that has OPEN challenges with `critical` or `major` severity. The acceptance gate has no challenge enforcement.

**Evidence**: `SeverityBlocksAcceptance()` exists in `internal/schema/severity.go:74-80` but is NEVER called anywhere in the codebase.

### ROOT CAUSE 2: No Verification Checklist on Claim (CRITICAL)

**File**: `internal/render/verifier_context.go`

**Current Behavior**: `RenderVerifierContext()` only renders context for **reviewing an existing challenge**, not for **verifying a node**.

**Problem**: When a verifier claims a node to verify it, they receive NO checklist of what to check:
- Statement is mathematically precise?
- Inference type matches the logical operation?
- All dependencies cited and validated?
- No hidden assumptions outside scope?
- Domain restrictions justified?

**Evidence**: There's no `RenderVerificationChecklist()` function or equivalent.

### ROOT CAUSE 3: Tool Guides Toward Depth-First Construction (HIGH)

**File**: `cmd/af/refine.go` line 310

**Current Output**:
```
Next steps:
  af refine 1.1.1    - Add more children to this node
```

**Problem**: This suggests refining the **child** node, which adds depth. A user following this guidance naturally creates a linked list:
```
1.1.1 → 1.1.1.1 → 1.1.1.1.1 → ...
```

**Missing**:
- No guidance for breadth-first: "af refine 1.1 - Add sibling to 1.1.1"
- No warning when depth > 3: "Your proof is getting deep"
- `MaxDepth` config exists but is never enforced

### ROOT CAUSE 4: Severity Blocking is Implemented But Disconnected (HIGH)

**File**: `internal/schema/severity.go`

**Implemented (but unused)**:
```go
SeverityCritical → BlocksAcceptance: true
SeverityMajor    → BlocksAcceptance: true
SeverityMinor    → BlocksAcceptance: false
SeverityNote     → BlocksAcceptance: false
```

**Problem**: `SeverityBlocksAcceptance()` is never called. Challenges can be raised with `critical` severity, but they don't actually block acceptance.

---

## Fix Plan

### FIX 1: Enforce Challenge Resolution at Acceptance (CRITICAL)

**Priority**: P0 - Blocks all testing
**Files**: `internal/service/proof.go`
**Estimated Lines Changed**: ~30

**Implementation**:

1. Add new method to get blocking challenges for a node:

   ```go
   // getBlockingChallenges returns open challenges that block acceptance
   func (s *ProofService) getBlockingChallenges(st *state.State, nodeID types.NodeID) []*node.Challenge {
       var blocking []*node.Challenge
       for _, c := range st.AllChallenges() {
           if c.TargetID.String() == nodeID.String() &&
              c.Status == node.ChallengeStatusOpen &&
              schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
               blocking = append(blocking, c)
           }
       }
       return blocking
   }
   ```

2. Add check in `AcceptNodeWithNote()` after line 794 (after validation deps check):

   ```go
   // Check for blocking challenges - must be resolved before acceptance
   blockingChallenges := s.getBlockingChallenges(st, id)
   if len(blockingChallenges) > 0 {
       var ids []string
       for _, c := range blockingChallenges {
           ids = append(ids, c.ID)
       }
       return fmt.Errorf("cannot accept node %s: %d blocking challenge(s) must be resolved: %s",
           id.String(), len(ids), strings.Join(ids, ", "))
   }
   ```

3. Add same check to `AcceptNode()` and `AcceptNodeBulk()`.

**Tests**:
- `TestAcceptNodeWithNote_BlocksOnCriticalChallenge`
- `TestAcceptNodeWithNote_AllowsMinorChallenge`
- `TestAcceptNodeBulk_FailsOnBlockingChallenge`

---

### FIX 2: Add Verification Checklist to Claim Output (CRITICAL)

**Priority**: P0 - Core workflow enabler
**Files**:
- `internal/render/node_verification_checklist.go` (new)
- `cmd/af/claim.go`
**Estimated Lines Changed**: ~100

**Implementation**:

1. Create `RenderVerificationChecklist()` in new file:

   ```go
   func RenderVerificationChecklist(n *node.Node, s *state.State) string {
       var sb strings.Builder
       sb.WriteString("\n=== VERIFICATION CHECKLIST ===\n")
       sb.WriteString("Before accepting this node, verify:\n\n")

       sb.WriteString("  □ STATEMENT PRECISE\n")
       sb.WriteString("    Is the statement mathematically unambiguous?\n\n")

       sb.WriteString("  □ INFERENCE VALID\n")
       sb.WriteString("    Does the inference type (")
       sb.WriteString(string(n.Inference))
       sb.WriteString(") match the logical operation?\n\n")

       sb.WriteString("  □ DEPENDENCIES JUSTIFIED\n")
       if len(n.Dependencies) == 0 {
           sb.WriteString("    (No dependencies declared)\n\n")
       } else {
           for _, dep := range n.Dependencies {
               depNode := s.GetNode(dep)
               if depNode != nil {
                   sb.WriteString("    - ")
                   sb.WriteString(dep.String())
                   sb.WriteString(" [")
                   sb.WriteString(string(depNode.EpistemicState))
                   sb.WriteString("]\n")
               }
           }
           sb.WriteString("\n")
       }

       sb.WriteString("  □ NO HIDDEN ASSUMPTIONS\n")
       sb.WriteString("    Does this step rely on anything not explicitly stated?\n\n")

       sb.WriteString("  □ DOMAIN RESTRICTIONS\n")
       sb.WriteString("    Are all domain constraints (e.g., x > 0) justified?\n\n")

       sb.WriteString("If any check fails, raise a challenge:\n")
       sb.WriteString("  af challenge ")
       sb.WriteString(n.ID.String())
       sb.WriteString(" --reason \"<reason>\" --severity critical\n")

       return sb.String()
   }
   ```

2. Modify `claim.go` to show checklist when verifier claims a node:

   ```go
   // After successful claim, show role-specific context
   if role == "verifier" {
       checklist := render.RenderVerificationChecklist(n, st)
       cmd.Println(checklist)
   }
   ```

**Tests**:
- `TestRenderVerificationChecklist_ShowsAllItems`
- `TestClaim_VerifierSeesChecklist`

---

### FIX 3: Add Breadth-First Guidance to Refine Output (HIGH)

**Priority**: P1 - Prevents structural issues
**Files**: `cmd/af/refine.go`
**Estimated Lines Changed**: ~25

**Implementation**:

1. Modify "Next steps" section (around line 309-312):

   ```go
   cmd.Println("\nNext steps:")

   // Show breadth-first option FIRST
   parentIDStr := parentID.String()
   cmd.Printf("  af claim %s && af refine %s -s \"<sibling step>\"  - Add breadth (recommended)\n",
       parentIDStr, parentIDStr)

   // Then show depth option
   cmd.Printf("  af claim %s && af refine %s -s \"<sub-step>\"      - Add depth (child of %s)\n",
       childID.String(), childID.String(), childID.String())

   cmd.Printf("  af status         - View proof status\n")
   ```

2. Add depth warning (insert before node creation, ~line 243):

   ```go
   // Warn about deep nesting
   depth := childID.Depth()
   if depth > 3 {
       cmd.Printf("\n⚠️  WARNING: Creating node at depth %d\n", depth)
       cmd.Printf("Deep proofs are harder to verify. Consider:\n")
       cmd.Printf("  - Adding siblings to %s instead of children to %s\n",
           parentID.String(), parentID.String())
       cmd.Printf("  - Most proof steps should be siblings (same depth), not children\n\n")
   }
   ```

3. Enforce MaxDepth config (add check before creation):

   ```go
   cfg, _ := svc.LoadConfig()
   if depth > cfg.MaxDepth {
       return fmt.Errorf("ERROR: depth %d exceeds MaxDepth %d. Add breadth instead:\n"+
           "  af refine %s -s \"<step>\"",
           depth, cfg.MaxDepth, parentID.Parent().String())
   }
   ```

**Tests**:
- `TestRefine_ShowsBreadthFirstGuidance`
- `TestRefine_WarnsOnDeepNesting`
- `TestRefine_EnforcesMaxDepth`

---

### FIX 4: Connect Severity System to Workflow (HIGH)

**Priority**: P1 - Enables graduated challenges
**Files**:
- `internal/state/state.go` (add helper method)
- `internal/jobs/verifier.go` (update job detection)
**Estimated Lines Changed**: ~20

**Implementation**:

1. Add method to `state.State`:

   ```go
   // HasBlockingChallenges returns true if the node has open challenges
   // with critical or major severity.
   func (s *State) HasBlockingChallenges(nodeID types.NodeID) bool {
       nodeIDStr := nodeID.String()
       for _, c := range s.challenges {
           if c.NodeID.String() == nodeIDStr &&
              c.Status == "open" &&
              schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
               return true
           }
       }
       return false
   }
   ```

2. Update `isVerifierJob()` in `verifier.go` to distinguish blocking vs non-blocking:

   ```go
   // Verifier jobs should show notes/minor challenges (they don't block)
   // but not show if there are blocking challenges (prover needs to address those)
   if hasBlockingChallenges(n, challengeMap) {
       return false // Has blocking challenges → prover job
   }
   return true // May have minor challenges, but verifier can still evaluate
   ```

**Tests**:
- `TestHasBlockingChallenges_Critical`
- `TestHasBlockingChallenges_Minor`
- `TestVerifierJob_IgnoresMinorChallenges`

---

### FIX 5: Add "Minimum Challenge" Workflow (MEDIUM)

**Priority**: P2 - Improves adversarial incentive
**Files**:
- `cmd/af/accept.go`
**Estimated Lines Changed**: ~30

**Implementation**:

Optionally require verifier to explicitly confirm no challenges:

```go
// If no challenges were raised, prompt for confirmation
if !hasRaisedAnyChallenges(st, nodeID, claimedBy) {
    cmd.Println("\nNo challenges raised for this node.")
    cmd.Println("As a verifier, you should scrutinize every step.")
    cmd.Println("")
    cmd.Println("Confirm you have verified:")
    cmd.Println("  - Statement precision")
    cmd.Println("  - Inference validity")
    cmd.Println("  - Dependencies")
    cmd.Println("")
    if !confirmFlag {
        return fmt.Errorf("use --confirm to accept without challenges")
    }
}
```

This creates friction for "rubber stamp" acceptance.

**Tests**:
- `TestAccept_RequiresConfirmIfNoChallenges`
- `TestAccept_NoPromptIfChallengesResolved`

---

## Implementation Order

| Order | Fix | Priority | Blocks |
|-------|-----|----------|--------|
| 1 | FIX 1: Enforce challenge resolution | P0 | All testing |
| 2 | FIX 2: Verification checklist | P0 | Meaningful verification |
| 3 | FIX 3: Breadth-first guidance | P1 | Correct proof structure |
| 4 | FIX 4: Connect severity system | P1 | Graduated challenges |
| 5 | FIX 5: Minimum challenge workflow | P2 | Adversarial incentive |

---

## Validation Plan

After implementing all fixes:

1. **Re-run √2 irrationality proof test case**
   - Verifier should receive checklist on claim
   - Verifier should raise at least 1 challenge per node
   - Acceptance should fail if blocking challenges open
   - Proof structure should be breadth-first (depth ≤ 3)

2. **Metrics to track**:
   - Challenges raised per node (target: ≥ 0.5 average)
   - Max proof depth (target: ≤ 4)
   - Acceptance rejections due to blocking challenges (expected during testing)

---

## Appendix: Code Locations Reference

| Component | File | Lines |
|-----------|------|-------|
| AcceptNodeWithNote | `internal/service/proof.go` | 761-815 |
| AcceptNode | `internal/service/proof.go` | 719-752 |
| AcceptNodeBulk | `internal/service/proof.go` | 817-875 |
| SeverityBlocksAcceptance | `internal/schema/severity.go` | 74-80 |
| isVerifierJob | `internal/jobs/verifier.go` | 49-67 |
| isProverJob | `internal/jobs/prover.go` | 47-60 |
| RenderVerifierContext | `internal/render/verifier_context.go` | 17-67 |
| Refine next steps | `cmd/af/refine.go` | 309-312 |
| NodeID.Depth | `internal/types/id.go` | 133-136 |
| MaxDepth config | `internal/config/config.go` | 28-29 |

---

## Summary

The AF tool's adversarial workflow is broken because:

1. **Acceptance has no verification gate** - Blocking challenges are ignored
2. **No verification guidance** - Verifiers don't know what to check
3. **Wrong guidance** - Tool encourages depth over breadth
4. **Dead code** - Severity blocking is implemented but disconnected

The job detection logic is CORRECT. The tests pass. The state model works. The problem is entirely in the **CLI/service layer enforcement** and **UX guidance**.

These 5 fixes will close the adversarial loop and make AF function as designed.
