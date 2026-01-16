# AF Tool Critical Failure Report - Session 53

**Date**: 2026-01-16
**Test Case**: √2 Irrationality Proof
**Rating**: 3/10 - Same fundamental issues persist despite prior feedback

## Executive Summary

Despite a previous 80+ failure report (Session Dobinski), the core architectural problems remain unaddressed. The √2 irrationality proof was "completed" but the workflow was fundamentally broken: the adversarial verification loop never engaged, and the proof structure devolved into a deeply nested linked list instead of a breadth-first tree.

---

## Critical Failures Observed

### FAILURE 1: Verifier Jobs Still Don't Appear Immediately

**Severity**: CRITICAL - Blocks adversarial workflow
**Status**: UNRESOLVED (identical to prior FAILURE_REPORT.md FAILURE 1)

**Observed Behavior**:
After the prover created node 1.1.1, I checked jobs:
```
=== Verifier Jobs (3 available) ===
  [1] claim: "√2 is irrational"
  [1.1] local_assume: "Assume the negation of the conjecture"
  [1.2] claim: "Derive a contradiction from the assumption"

Summary: 0 prover job(s), 3 verifier job(s)
```

But after the prover built out 10 nested nodes (1.1.1 → 1.1.1.1 → ... → 1.1.1.1.1.1.1.1.1.1):
```
--- Jobs ---
  Prover: 3 nodes awaiting refinement
  Verifier: 0 nodes ready for review
```

**Expected Behavior** (from PRD):
> Every newly created node should IMMEDIATELY be a verifier job. The workflow should be:
> 1. Prover creates claim → **Verifier job appears immediately**
> 2. Verifier challenges OR accepts
> 3. If challenged → Prover job appears for that node

**Impact**:
- The prover built the ENTIRE proof (10 nested nodes) without ANY verification
- Verifier was completely excluded until the prover was "done"
- This is cooperative proof construction, NOT adversarial verification
- The tool name "Adversarial Proof Framework" is false advertising

### FAILURE 2: Proof Structure is a Linked List, Not a Tree

**Severity**: CRITICAL - Violates proof organization principles
**Status**: NEW FAILURE

**Observed Structure**:
```
1 √2 is irrational
├── 1.1 Assume the negation
│   └── 1.1.1 Assume √2 = p/q with gcd(p,q) = 1
│       └── 1.1.1.1 Squaring: 2q² = p²
│           └── 1.1.1.1.1 p² is even
│               └── 1.1.1.1.1.1 p is even
│                   └── 1.1.1.1.1.1.1 p = 2k
│                       └── 1.1.1.1.1.1.1.1 q² = 2k²
│                           └── 1.1.1.1.1.1.1.1.1 q is even
│                               └── 1.1.1.1.1.1.1.1.1.1 CONTRADICTION
└── 1.2 Derive contradiction
    └── 1.2.1 Conclusion
```

This is a **linked list** with depth 10, not a proper proof tree.

**Expected Structure** (breadth-first):
```
1 √2 is irrational
├── 1.1 Assume √2 is rational: √2 = p/q with gcd(p,q) = 1
├── 1.2 Square both sides: 2 = p²/q², so 2q² = p²
├── 1.3 p² is even (divisible by 2)
├── 1.4 p is even (contrapositive: odd² is odd)
├── 1.5 Let p = 2k for some integer k
├── 1.6 Substituting: 2q² = 4k², so q² = 2k²
├── 1.7 q is even (same reasoning as 1.4)
├── 1.8 Both p,q even contradicts gcd(p,q) = 1
└── 1.9 QED: √2 is irrational
```

**Why This Matters**:
- Breadth-first allows verifiers to check each step independently
- Breadth-first enables parallel verification
- Deep nesting creates false dependencies (each step doesn't NEED to be a child)
- A 10-level deep proof is unreadable and unverifiable
- The PRD shows sibling steps (1.1, 1.2, 1.3...), not nested children

**Root Cause**:
The `af refine` command creates CHILDREN of the claimed node. The prover agent used:
```
af claim 1.1.1 → af refine 1.1.1 (creates 1.1.1.1)
af claim 1.1.1.1 → af refine 1.1.1.1 (creates 1.1.1.1.1)
...
```

What SHOULD have happened:
```
af claim 1 → af refine 1 (creates 1.1)
af claim 1 → af refine 1 (creates 1.2)
af claim 1 → af refine 1 (creates 1.3)
...
```

**The tool provides no guidance** on when to use depth vs breadth. The PRD says "Provers convince, verifiers attack" but provides no guidance on proof STRUCTURE.

### FAILURE 3: Verifier Accepted Everything Without Challenge

**Severity**: HIGH - Undermines adversarial model
**Status**: NEW FAILURE

**Observed**:
The verifier agent accepted all 12 nodes without raising a single challenge:
```
All 12 nodes have been validated. The proof of "sqrt(2) is irrational" is now fully verified.
...
**Issues found**: None
```

**Problem**:
The verifier is supposed to be ADVERSARIAL. A real adversarial verifier would challenge:
- Node 1.1.1.1.1: "Why does p² even imply p even? Justify the contrapositive."
- Node 1.1.1.1.1.1: "What is the definition of 'even'? Cite it."
- Node 1.2.1: "This node claims dependency on 1.1.1.1.1.1.1.1.1.1 but the graph doesn't show explicit dependency tracking."

**Root Cause**:
1. No adversarial incentive - verifier has no motivation to find errors
2. No verification criteria - what makes a step "valid"?
3. No challenge examples - verifier doesn't know WHAT to challenge
4. The claim context output doesn't show verification checklist items

### FAILURE 4: No Breadth-First Guidance or Enforcement

**Severity**: HIGH - Design flaw
**Status**: NEW FAILURE

The tool provides:
- `af strategy` - only lists strategies (contradiction, induction, etc.)
- No `af strategy apply contradiction` with breadth-first skeleton
- No warning when proof depth exceeds reasonable limits
- No "Did you mean to add a sibling instead of a child?" prompt

**Required**:
When depth > 3, warn: "Your proof is getting deep. Consider adding siblings to the parent instead."

### FAILURE 5: Claim-Per-Refinement Creates Serial Bottleneck

**Severity**: MEDIUM
**Status**: UNRESOLVED (identical to prior FAILURE 4)

The prover had to:
1. Claim 1.1.1
2. Refine (creates 1.1.1.1)
3. Release 1.1.1
4. Claim 1.1.1.1
5. Refine (creates 1.1.1.1.1)
6. Release 1.1.1.1
... repeat 8 more times

This is incredibly inefficient. The prover should be able to:
```bash
af refine 1 --children '[
  {"statement": "Step 1", ...},
  {"statement": "Step 2", ...},
  {"statement": "Step 3", ...}
]' --agent prover-1
```

But even if `--children` worked, the claim-per-node model means only ONE agent can work on a node at a time, forcing serialization.

---

## What Should Have Happened

### Correct Adversarial Workflow

```
Round 1:
  Prover: Creates 1.1 "Assume √2 = p/q with gcd(p,q)=1"
  → IMMEDIATELY visible as verifier job
  Verifier: Reviews 1.1, CHALLENGES: "What do you mean by 'lowest terms'? Define gcd."

Round 2:
  Prover: Sees challenge, adds definition request for gcd
  Human: Provides gcd definition
  Prover: Refines 1.1 to cite DEF-gcd
  → IMMEDIATELY visible as verifier job
  Verifier: Accepts 1.1

Round 3:
  Prover: Creates 1.2 "Squaring: 2q² = p²"
  → IMMEDIATELY visible as verifier job
  Verifier: Reviews 1.2, ACCEPTS (straightforward algebra)

Round 4:
  Prover: Creates 1.3 "p² is even"
  → IMMEDIATELY visible as verifier job
  Verifier: CHALLENGES: "You claim p² is even. What's your definition of 'even'?"

... and so on, BREADTH-FIRST, with IMMEDIATE verification ...
```

### Correct Proof Structure

```
1 √2 is irrational [validated]
├── 1.1 Assume √2 = p/q, gcd(p,q) = 1 [validated]
├── 1.2 2q² = p² [validated]
├── 1.3 p² even [validated]
├── 1.4 p even [validated]
├── 1.5 p = 2k [validated]
├── 1.6 q² = 2k² [validated]
├── 1.7 q even [validated]
├── 1.8 Contradiction: gcd(p,q) ≥ 2 [validated]
└── 1.9 QED [validated]
```

9 nodes at depth 2, not 12 nodes at depth 10.

---

## Architectural Changes Required

### 1. Fix Verifier Job Detection (CRITICAL)

Current logic (internal/jobs/verifier.go):
```go
// A node is a verifier job when ALL children are validated
```

Required logic:
```go
// A node is a verifier job when:
// - It has been refined (has statement content)
// - It is not currently claimed
// - It has no unresolved challenges with unanswered responses
```

**Every newly created node should be a verifier job IMMEDIATELY.**

### 2. Add Breadth-First Guidance

When `af refine` creates a child at depth > 3:
```
Warning: Creating node at depth 6.

Deep proofs are harder to verify. Consider:
  - Adding siblings to 1.1 instead of children to 1.1.1.1.1
  - Using 'af refine 1 --statement "Next step"' to add breadth

Continue anyway? (y/N)
```

### 3. Add Multi-Child Refinement

```bash
af refine 1 --bulk <<EOF
[
  {"statement": "Step 1", "inference": "local_assume"},
  {"statement": "Step 2", "inference": "derive"},
  {"statement": "Step 3", "inference": "derive"}
]
EOF
```

Creates 1.1, 1.2, 1.3 in a single atomic operation.

### 4. Add Verification Checklist to Verifier Context

When verifier claims a node, show:
```
VERIFICATION CHECKLIST:
  □ Statement is mathematically precise
  □ Inference type matches the logical step
  □ All dependencies are cited and validated
  □ No hidden assumptions
  □ Domain restrictions justified
  □ Notation consistent with definitions
```

### 5. Add Challenge Incentive

Require verifiers to check off items or explain why they're valid:
```json
{
  "action": "accept",
  "checklist": {
    "statement_precise": true,
    "inference_valid": true,
    "dependencies_cited": true,
    "no_hidden_assumptions": "checked - only uses local assumption from 1.1",
    "domain_restrictions": "N/A"
  }
}
```

---

## Comparison with Prior FAILURE_REPORT.md

| Issue | Prior Report | This Session | Status |
|-------|--------------|--------------|--------|
| Verifier jobs don't appear immediately | FAILURE 1 | Reproduced | UNRESOLVED |
| Agents must look up context | FAILURE 2 | Not tested | UNKNOWN |
| No mechanism to mark node complete | FAILURE 3 | Not tested | UNKNOWN |
| Claim/Refine/Release contention | FAILURE 4 | Reproduced | UNRESOLVED |
| Deep nesting instead of breadth | Not reported | NEW | NEW ISSUE |
| Verifier accepts everything | Not reported | NEW | NEW ISSUE |
| No breadth-first guidance | Not reported | NEW | NEW ISSUE |

**Conclusion**: Of the 80+ failures documented in the prior report, the TWO most critical ones (verifier job detection, claim contention) remain completely unresolved.

---

## Recommendations

### Immediate (Block Further Testing)

1. **Fix verifier job detection** - Nodes should be verifier jobs IMMEDIATELY after creation
2. **Add breadth-first warnings** - Discourage deep nesting
3. **Add multi-child refinement** - Enable efficient breadth-first construction

### Before Next Test Session

4. **Add verification checklist** - Make adversarial review meaningful
5. **Fix claim model** - Consider claimless refinement since ledger handles concurrency

### Design Review Required

6. **Re-evaluate adversarial model** - Current implementation is cooperative, not adversarial
7. **Add proof structure templates** - Show CORRECT structure for contradiction proofs

---

## Session Summary

The √2 irrationality proof was "completed" but demonstrated that the tool is fundamentally broken:

- **Adversarial verification never occurred** - All 12 nodes were created by prover, then rubber-stamped by verifier
- **Proof structure was wrong** - Linked list instead of breadth-first tree
- **Core issues from prior report persist** - Verifier job detection is still backwards

The tool produces a **validated proof tree** but the process that produced it was **not adversarial**. This makes the "validated" status meaningless - it's just a prover talking to itself.

**Rating: 3/10** - Marginally functional but defeats its own purpose.

---

*End of Failure Report*
