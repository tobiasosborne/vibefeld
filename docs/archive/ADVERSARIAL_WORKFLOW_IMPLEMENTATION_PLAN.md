# Adversarial Workflow Implementation Plan

**Created**: 2026-01-16
**Reference**: ADVERSARIAL_WORKFLOW_FIX_PLAN.md, FAILURE_REPORT_SESSION53.md

---

## Overview

This plan breaks down the 5 fixes into granular implementation steps. Each step is designed to be:
- Independently testable
- Small enough for a single session
- TDD-compliant (tests first)

---

## PHASE 1: Challenge Enforcement at Acceptance (P0)

### Step 1.1: Add GetBlockingChallengesForNode to State

**File**: `internal/state/state.go`
**Type**: Feature

Add a method to retrieve open challenges with blocking severity for a node.

```go
// GetBlockingChallengesForNode returns open challenges with critical/major severity
func (s *State) GetBlockingChallengesForNode(nodeID types.NodeID) []*ChallengeInfo {
```

**Test file**: `internal/state/state_test.go`
**Tests to add**:
- `TestGetBlockingChallengesForNode_ReturnsCritical`
- `TestGetBlockingChallengesForNode_ReturnsMajor`
- `TestGetBlockingChallengesForNode_ExcludesMinor`
- `TestGetBlockingChallengesForNode_ExcludesNote`
- `TestGetBlockingChallengesForNode_ExcludesResolved`
- `TestGetBlockingChallengesForNode_EmptyForNoNode`

**Dependencies**: None

---

### Step 1.2: Add blocking challenge check to AcceptNode

**File**: `internal/service/proof.go`
**Type**: Bug fix

Modify `AcceptNode()` to reject if blocking challenges exist.

**Location**: After line 738 (after validation deps check)

```go
// Check for blocking challenges
blockingChallenges := st.GetBlockingChallengesForNode(id)
if len(blockingChallenges) > 0 {
    return fmt.Errorf("cannot accept node %s: %d blocking challenge(s) open",
        id.String(), len(blockingChallenges))
}
```

**Test file**: `internal/service/proof_test.go`
**Tests to add**:
- `TestAcceptNode_RejectsWithCriticalChallenge`
- `TestAcceptNode_RejectsWithMajorChallenge`
- `TestAcceptNode_AllowsWithMinorChallenge`
- `TestAcceptNode_AllowsWithNoteChallenge`
- `TestAcceptNode_AllowsAfterChallengeResolved`

**Dependencies**: Step 1.1

---

### Step 1.3: Add blocking challenge check to AcceptNodeWithNote

**File**: `internal/service/proof.go`
**Type**: Bug fix

Modify `AcceptNodeWithNote()` to reject if blocking challenges exist.

**Location**: After line 794 (after validation deps check)

Same logic as Step 1.2.

**Test file**: `internal/service/proof_test.go`
**Tests to add**:
- `TestAcceptNodeWithNote_RejectsWithBlockingChallenge`
- `TestAcceptNodeWithNote_AllowsWithMinorChallenge`

**Dependencies**: Step 1.1

---

### Step 1.4: Add blocking challenge check to AcceptNodeBulk

**File**: `internal/service/proof.go`
**Type**: Bug fix

Modify `AcceptNodeBulk()` to check all nodes for blocking challenges before accepting any.

**Location**: In the validation loop before appending events

```go
// Check all nodes for blocking challenges first
for _, id := range ids {
    blockingChallenges := st.GetBlockingChallengesForNode(id)
    if len(blockingChallenges) > 0 {
        return fmt.Errorf("cannot accept node %s: %d blocking challenge(s) open",
            id.String(), len(blockingChallenges))
    }
}
```

**Test file**: `internal/service/proof_test.go`
**Tests to add**:
- `TestAcceptNodeBulk_RejectsIfAnyHasBlockingChallenge`
- `TestAcceptNodeBulk_AllowsWithOnlyMinorChallenges`

**Dependencies**: Step 1.1

---

### Step 1.5: Update accept CLI to show blocking challenges on failure

**File**: `cmd/af/accept.go`
**Type**: Enhancement

When acceptance fails due to blocking challenges, show which challenges need resolution.

```go
if strings.Contains(err.Error(), "blocking challenge") {
    cmd.Println("\nBlocking challenges must be resolved first:")
    // List the blocking challenges with IDs and reasons
}
```

**Test file**: `cmd/af/accept_test.go`
**Tests to add**:
- `TestAcceptCommand_ShowsBlockingChallengeDetails`

**Dependencies**: Steps 1.2, 1.3, 1.4

---

## PHASE 2: Verification Checklist (P0)

### Step 2.1: Create RenderVerificationChecklist function

**File**: `internal/render/verification_checklist.go` (new)
**Type**: Feature

Create a function that generates a verification checklist for a node.

```go
// RenderVerificationChecklist generates a checklist for verifiers
func RenderVerificationChecklist(n *node.Node, s *state.State) string
```

Checklist items:
1. Statement precision
2. Inference validity (show actual inference type)
3. Dependencies (list each with validation status)
4. Hidden assumptions
5. Domain restrictions
6. Notation consistency

**Test file**: `internal/render/verification_checklist_test.go`
**Tests to add**:
- `TestRenderVerificationChecklist_IncludesStatementCheck`
- `TestRenderVerificationChecklist_IncludesInferenceType`
- `TestRenderVerificationChecklist_ListsDependencies`
- `TestRenderVerificationChecklist_ShowsDependencyStatus`
- `TestRenderVerificationChecklist_SuggestsChallengeCommand`
- `TestRenderVerificationChecklist_HandlesNilNode`
- `TestRenderVerificationChecklist_HandlesNilState`

**Dependencies**: None

---

### Step 2.2: Add JSON format for verification checklist

**File**: `internal/render/verification_checklist.go`
**Type**: Feature

Add `RenderVerificationChecklistJSON()` for machine-readable output.

```go
type VerificationChecklistJSON struct {
    NodeID       string              `json:"node_id"`
    Items        []ChecklistItem     `json:"items"`
    Dependencies []DependencyStatus  `json:"dependencies"`
    ChallengeCmd string              `json:"challenge_command"`
}

func RenderVerificationChecklistJSON(n *node.Node, s *state.State) (string, error)
```

**Test file**: `internal/render/verification_checklist_test.go`
**Tests to add**:
- `TestRenderVerificationChecklistJSON_ValidJSON`
- `TestRenderVerificationChecklistJSON_IncludesAllFields`

**Dependencies**: Step 2.1

---

### Step 2.3: Show checklist when verifier claims a node

**File**: `cmd/af/claim.go`
**Type**: Feature

After successful claim with `--role verifier`, display the verification checklist.

**Location**: After successful claim output, before next steps

```go
if role == "verifier" {
    checklist := render.RenderVerificationChecklist(targetNode, st)
    cmd.Println(checklist)
}
```

**Test file**: `cmd/af/claim_test.go`
**Tests to add**:
- `TestClaimCommand_VerifierSeesChecklist`
- `TestClaimCommand_ProverDoesNotSeeChecklist`
- `TestClaimCommand_ChecklistJSON_WhenFormatJSON`

**Dependencies**: Steps 2.1, 2.2

---

### Step 2.4: Add --checklist flag to af show command

**File**: `cmd/af/show.go`
**Type**: Feature

Add `--checklist` flag to show verification checklist for any node.

```bash
af show 1.1 --checklist
```

**Test file**: `cmd/af/show_test.go`
**Tests to add**:
- `TestShowCommand_ChecklistFlag`
- `TestShowCommand_ChecklistJSON`

**Dependencies**: Steps 2.1, 2.2

---

## PHASE 3: Breadth-First Guidance (P1)

### Step 3.1: Modify refine "Next steps" to show breadth option first

**File**: `cmd/af/refine.go`
**Type**: Enhancement

Change line 309-312 to show breadth-first option prominently.

**Current**:
```
Next steps:
  af refine 1.1.1    - Add more children to this node
```

**New**:
```
Next steps:
  af refine 1.1 -s "..."    - Add sibling (breadth-first, recommended)
  af refine 1.1.1 -s "..."  - Add child (depth-first)
```

**Test file**: `cmd/af/refine_test.go`
**Tests to add**:
- `TestRefineCommand_NextStepsShowsBreadthFirst`
- `TestRefineCommand_NextStepsShowsParentID`

**Dependencies**: None

---

### Step 3.2: Add depth warning when creating deep nodes

**File**: `cmd/af/refine.go`
**Type**: Enhancement

Before creating a node at depth > 3, print a warning.

**Location**: Before node creation (~line 243)

```go
depth := childID.Depth()
if depth > 3 {
    cmd.Printf("\n⚠️  Creating node at depth %d\n", depth)
    cmd.Printf("Consider adding siblings to %s instead.\n\n", parentID.String())
}
```

**Test file**: `cmd/af/refine_test.go`
**Tests to add**:
- `TestRefineCommand_WarnsAtDepth4`
- `TestRefineCommand_NoWarningAtDepth3`

**Dependencies**: None

---

### Step 3.3: Add --warn-depth config option

**File**: `internal/config/config.go`
**Type**: Feature

Add configurable warning threshold for depth.

```go
// WarnDepth is the depth at which to warn about deep nesting (default: 3)
WarnDepth int `json:"warn_depth"`
```

**File**: `cmd/af/refine.go`
**Type**: Enhancement

Use config value instead of hardcoded 3.

**Test file**: `internal/config/config_test.go`
**Tests to add**:
- `TestConfig_WarnDepthDefault`
- `TestConfig_WarnDepthCustom`

**Dependencies**: Step 3.2

---

### Step 3.4: Enforce MaxDepth config in refine

**File**: `cmd/af/refine.go`
**Type**: Bug fix

Check `MaxDepth` config before creating nodes and reject if exceeded.

```go
cfg, _ := svc.LoadConfig()
if depth > cfg.MaxDepth {
    return fmt.Errorf("depth %d exceeds MaxDepth %d; add breadth instead", depth, cfg.MaxDepth)
}
```

**Test file**: `cmd/af/refine_test.go`
**Tests to add**:
- `TestRefineCommand_RejectsExceedingMaxDepth`
- `TestRefineCommand_AllowsAtMaxDepth`

**Dependencies**: None

---

### Step 3.5: Add --sibling flag to refine command

**File**: `cmd/af/refine.go`
**Type**: Feature

Add `--sibling` flag that adds a sibling to the specified node instead of a child.

```bash
af refine 1.1.1 --sibling -s "Next step"  # Creates 1.1.4 (sibling of 1.1.3)
```

Implementation: Find parent of specified node, then create child of parent.

**Test file**: `cmd/af/refine_test.go`
**Tests to add**:
- `TestRefineCommand_SiblingFlag_CreatesAtSameLevel`
- `TestRefineCommand_SiblingFlag_ErrorOnRoot`

**Dependencies**: None

---

## PHASE 4: Connect Severity System (P1)

### Step 4.1: Add HasBlockingChallenges helper to state

**File**: `internal/state/state.go`
**Type**: Feature

Add boolean helper that wraps GetBlockingChallengesForNode.

```go
// HasBlockingChallenges returns true if node has open critical/major challenges
func (s *State) HasBlockingChallenges(nodeID types.NodeID) bool {
    return len(s.GetBlockingChallengesForNode(nodeID)) > 0
}
```

**Test file**: `internal/state/state_test.go`
**Tests to add**:
- `TestHasBlockingChallenges_TrueForCritical`
- `TestHasBlockingChallenges_FalseForMinor`

**Dependencies**: Step 1.1

---

### Step 4.2: Update verifier job detection to use severity

**File**: `internal/jobs/verifier.go`
**Type**: Enhancement

Modify `hasOpenChallenges()` to only count blocking challenges.

**Current**: Any open challenge makes node a prover job
**New**: Only blocking (critical/major) challenges make node a prover job

```go
func hasBlockingChallenges(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
    challenges := challengeMap[n.ID.String()]
    for _, c := range challenges {
        if c.Status == node.ChallengeStatusOpen &&
           schema.SeverityBlocksAcceptance(schema.ChallengeSeverity(c.Severity)) {
            return true
        }
    }
    return false
}
```

**Test file**: `internal/jobs/verifier_test.go`
**Tests to add**:
- `TestVerifierJob_TrueWithMinorChallenge`
- `TestVerifierJob_FalseWithCriticalChallenge`
- `TestVerifierJob_TrueWithResolvedCritical`

**Dependencies**: None

---

### Step 4.3: Update prover job detection to use severity

**File**: `internal/jobs/prover.go`
**Type**: Enhancement

Modify to only count blocking challenges as requiring prover attention.

**Test file**: `internal/jobs/prover_test.go`
**Tests to add**:
- `TestProverJob_TrueWithCriticalChallenge`
- `TestProverJob_FalseWithOnlyMinorChallenge`

**Dependencies**: Step 4.2

---

### Step 4.4: Add challenge severity to jobs output

**File**: `cmd/af/jobs.go`
**Type**: Enhancement

Show severity when listing challenges in job output.

```
Prover Jobs:
  1.1.1 - "Some claim" [1 critical, 2 minor challenges]
```

**Test file**: `cmd/af/jobs_test.go`
**Tests to add**:
- `TestJobsCommand_ShowsChallengeSeverityCounts`

**Dependencies**: Steps 4.2, 4.3

---

## PHASE 5: Minimum Challenge Workflow (P2)

### Step 5.1: Track verifier challenge history per claim session

**File**: `internal/state/state.go`
**Type**: Feature

Add method to check if a verifier has raised any challenges for a node.

```go
// VerifierRaisedChallengeForNode returns true if the given agent raised
// any challenge (now resolved or not) for the specified node
func (s *State) VerifierRaisedChallengeForNode(nodeID types.NodeID, agentID string) bool
```

**Test file**: `internal/state/state_test.go`
**Tests to add**:
- `TestVerifierRaisedChallengeForNode_TrueIfRaised`
- `TestVerifierRaisedChallengeForNode_TrueEvenIfResolved`
- `TestVerifierRaisedChallengeForNode_FalseForDifferentAgent`

**Dependencies**: None

---

### Step 5.2: Add --confirm flag to accept command

**File**: `cmd/af/accept.go`
**Type**: Feature

Add `--confirm` flag that skips the "no challenges raised" prompt.

```go
var confirmFlag bool
acceptCmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Confirm acceptance without challenges")
```

**Test file**: `cmd/af/accept_test.go`
**Tests to add**:
- `TestAcceptCommand_ConfirmFlag`

**Dependencies**: None

---

### Step 5.3: Require --confirm if verifier raised no challenges

**File**: `cmd/af/accept.go`
**Type**: Feature

Before accepting, check if the current agent raised any challenges. If not, require `--confirm`.

```go
if !st.VerifierRaisedChallengeForNode(nodeID, owner) && !confirmFlag {
    return fmt.Errorf("you haven't raised any challenges for %s; use --confirm to accept anyway", nodeID.String())
}
```

**Test file**: `cmd/af/accept_test.go`
**Tests to add**:
- `TestAcceptCommand_RequiresConfirmIfNoChallenges`
- `TestAcceptCommand_NoConfirmNeededIfChallengeRaised`
- `TestAcceptCommand_NoConfirmNeededIfChallengeResolved`

**Dependencies**: Steps 5.1, 5.2

---

### Step 5.4: Add verification summary to accept output

**File**: `cmd/af/accept.go`
**Type**: Enhancement

After successful acceptance, show what was verified.

```
✓ Node 1.1 accepted

Verification summary:
  - 2 challenges raised and resolved
  - Dependencies: 1.1.1 [validated], 1.1.2 [validated]
  - Acceptance note: "Valid by modus ponens"
```

**Test file**: `cmd/af/accept_test.go`
**Tests to add**:
- `TestAcceptCommand_ShowsVerificationSummary`

**Dependencies**: None

---

## Implementation Schedule

### Batch 1: Core Enforcement (Steps 1.1-1.5)
- **Priority**: P0
- **Blocks**: All meaningful testing
- **Can parallelize**: Steps 1.2, 1.3, 1.4 after 1.1 completes

### Batch 2: Verification Checklist (Steps 2.1-2.4)
- **Priority**: P0
- **Blocks**: Meaningful verification
- **Can parallelize**: Step 2.4 independent of 2.3

### Batch 3: Breadth-First Guidance (Steps 3.1-3.5)
- **Priority**: P1
- **Blocks**: Correct proof structure
- **Can parallelize**: Steps 3.1, 3.2, 3.4, 3.5 are independent

### Batch 4: Severity Connection (Steps 4.1-4.4)
- **Priority**: P1
- **Blocks**: Graduated challenge handling
- **Can parallelize**: Step 4.1 after 1.1; Steps 4.2, 4.3 independent

### Batch 5: Challenge Friction (Steps 5.1-5.4)
- **Priority**: P2
- **Blocks**: Adversarial incentive
- **Can parallelize**: Steps 5.1, 5.2, 5.4 are independent

---

## Total Steps: 22

| Phase | Steps | Priority |
|-------|-------|----------|
| 1. Challenge Enforcement | 5 | P0 |
| 2. Verification Checklist | 4 | P0 |
| 3. Breadth-First Guidance | 5 | P1 |
| 4. Severity Connection | 4 | P1 |
| 5. Minimum Challenge | 4 | P2 |

---

## Validation Criteria

After all steps complete, the √2 proof should:
1. Fail acceptance if blocking challenges exist
2. Show verification checklist on verifier claim
3. Warn at depth > 3, reject at depth > MaxDepth
4. Distinguish blocking vs non-blocking challenges
5. Require `--confirm` if no challenges raised
