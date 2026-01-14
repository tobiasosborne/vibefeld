# Role-Specific Workflow Documentation

This document describes the detailed workflows for each role in AF (Adversarial Proof Framework). It answers the key questions agents need to know: when to take which actions, what their responsibilities are, and how to complete their tasks successfully.

---

## Overview: The Two Roles

AF operates on the principle of adversarial verification. Every proof is refined through the interaction of two distinct roles:

| Role | Goal | Mindset |
|------|------|---------|
| **Prover** | Convince verifiers that statements are true | Constructive: build and justify |
| **Verifier** | Ensure only valid proofs are accepted | Skeptical: question and validate |

**Critical Rule**: No agent ever plays both roles. Provers convince, verifiers attack.

---

## Prover Role

### Your Goal

As a prover, your goal is to **convince verifiers that mathematical statements are true**. You do this by:
1. Developing proof nodes with child steps that provide rigorous justification
2. Addressing challenges raised by verifiers
3. Ensuring every inference is properly justified with definitions and dependencies

### When You See a Prover Job

A prover job appears when a node needs development or has unaddressed challenges:

```bash
$ af jobs --role prover

PROVER JOBS AVAILABLE

NODE 1.2.1
  Statement: Since p is prime and 2 | p, we have 2 in {1, p}
  Reason: Has open challenge needing response
  Challenge: ch-003 "You assume 2 | p without justification"

  To work on this:
    af claim 1.2.1 --role prover --agent <your-agent-id>
```

**You see a prover job when:**
- A node has no children (needs development)
- A node has open challenges with empty `addressed_by` fields

### Actions Available

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `af claim --role prover` | Claim a node for work | First step before any work |
| `af refine` | Add child nodes | To develop the proof or address challenges |
| `af request-def` | Request a new definition | When you need a definition that doesn't exist |
| `af release` | Release a claimed node | When you're done or cannot continue |

### Basic Prover Workflow

```bash
# 1. Find available work
af jobs --role prover

# 2. Claim a node (this provides full context)
af claim 1.2.1 --role prover --agent prover-42

# 3. Add child nodes to develop the proof
af refine 1.2.1 \
  --statement "By definition of even, p = 2k implies 2 | p" \
  --inference by_definition \
  --context DEF-even,DEF-divides \
  --dependencies 1.2 \
  --agent prover-42

# 4. Node is automatically released after successful refine
# (or explicitly release if needed: af release 1.2.1 --agent prover-42)
```

### How to Address Challenges

When a node has an open challenge, you must create child nodes that address it:

1. **Read the challenge carefully**: Understand exactly what the verifier is objecting to
2. **Create addressing nodes**: Add children that directly respond to the objection
3. **Use `addresses_challenges`**: Link your new nodes to the challenge they address

**Example: Addressing a challenge**

Given challenge `ch-003`: "You assume 2 | p without justification"

```bash
af refine 1.2.1 \
  --statement "Since p is even, by definition there exists k such that p = 2k, hence 2 | p" \
  --inference by_definition \
  --context DEF-even,DEF-divides \
  --addresses ch-003 \
  --agent prover-42
```

Or using JSON for multiple children:

```bash
af refine 1.2.1 --children children.json --agent prover-42
```

Where `children.json` contains:
```json
[
  {
    "type": "claim",
    "statement": "Since p is even, by definition there exists k such that p = 2k",
    "inference": "by_definition",
    "context": ["DEF-even"],
    "dependencies": ["1.2"],
    "addresses_challenges": ["ch-003"]
  },
  {
    "type": "claim",
    "statement": "By definition of divides, p = 2k implies 2 | p",
    "inference": "by_definition",
    "context": ["DEF-divides"],
    "dependencies": ["1.2.1.1"],
    "addresses_challenges": ["ch-003"]
  }
]
```

### When a Node is "Complete Enough"

A node is ready for verifier review when:
- It has children that provide logical justification for its statement
- All open challenges have been addressed (child nodes with `addresses_challenges` pointing to each challenge)
- The inference type matches what the children actually demonstrate

**You are done when**: You have added children that address all challenges. Release the node and let the verifier review.

### Prover Responsibilities Summary

1. **Develop proofs**: Add child nodes that justify parent claims
2. **Address challenges**: Create nodes that directly respond to verifier objections
3. **Use proper context**: Reference definitions and assumptions correctly
4. **Maintain scope**: Respect scope boundaries for local assumptions
5. **Be precise**: Use the correct inference type for each step

---

## Verifier Role

### Your Goal

As a verifier, your goal is to **ensure only valid proofs are accepted**. You do this by:
1. Scrutinizing every proof step for errors, gaps, and invalid reasoning
2. Raising challenges when you find issues
3. Accepting steps only when they are rigorously justified
4. Resolving or withdrawing challenges appropriately

### When You See a Verifier Job

A verifier job appears when a node is ready for review:

```bash
$ af jobs --role verifier

VERIFIER JOBS AVAILABLE

NODE 1.2.1
  Statement: Since p is prime and 2 | p, we have 2 in {1, p}
  Reason: Challenge ch-003 has been addressed, ready for resolution
  Children: 1.2.1.1 [validated]
  Pending challenges: ch-003 (addressed by 1.2.1.1)

  To work on this:
    af claim 1.2.1 --role verifier --agent <your-agent-id>
```

**You see a verifier job when:**
- A node has no open challenges and has children to evaluate
- All challenges on a node have non-empty `addressed_by` fields (ready for resolution)

### Actions Available

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `af claim --role verifier` | Claim a node for review | First step before any work |
| `af challenge` | Raise an objection | When you find an issue |
| `af resolve-challenge` | Mark challenge as resolved | When addressing nodes are satisfactory |
| `af withdraw-challenge` | Retract a challenge | When your challenge was wrong |
| `af accept` | Validate a node | When all criteria are met |
| `af release` | Release a claimed node | When you're done reviewing |

### Basic Verifier Workflow

```bash
# 1. Find available work
af jobs --role verifier

# 2. Claim a node (this provides full context and validation checklist)
af claim 1.2.1 --role verifier --agent verifier-42

# 3. Review the node and its children
# 4. Take appropriate action:

# If valid - resolve any pending challenges, then accept:
af resolve-challenge 1.2.1 --challenge ch-003 --agent verifier-42
af accept 1.2.1 --agent verifier-42

# If issues found - raise a challenge:
af challenge 1.2.1 \
  --objection "The step assumes x > 0 but only x >= 0 is established" \
  --targets domain \
  --agent verifier-42

# 5. Release the node
af release 1.2.1 --agent verifier-42
```

### Decision Criteria: When to Accept

**Accept a node when ALL of the following are true:**

1. **Statement follows logically**: The claim follows from its children and/or referenced dependencies
2. **Inference is valid**: The stated inference rule is correctly applied
3. **Context is complete**: All referenced definitions and assumptions exist and are used correctly
4. **Dependencies are valid**: All referenced nodes exist and support the claim
5. **Scope is respected**: No out-of-scope assumptions are used
6. **All challenges resolved**: Every challenge is in a terminal state (`resolved`, `withdrawn`, or `superseded`)
7. **All children are accepted**: Every child has `epistemic_state` in `{validated, admitted}`

### Decision Criteria: When to Challenge

**Challenge a node when you find:**

| Issue Type | Example | Challenge Target |
|------------|---------|------------------|
| Claim is unclear or wrong | "p is prime" when p could be composite | `statement` |
| Reasoning step is invalid | Claiming A implies B without justification | `inference` |
| Missing definition reference | Using "even" without citing DEF-even | `context` |
| Missing step reference | Depending on 1.3 when only 1.2 is cited | `dependencies` |
| Using discharged assumption | Using a local assumption outside its scope | `scope` |
| Missing substeps | Jumping from A to C without showing B | `gap` |
| Type mismatch | Treating an integer as a real number | `type_error` |
| Unjustified domain restriction | Assuming x > 0 when only x >= 0 is known | `domain` |
| Incomplete cases | Only handling n=1 when n could be any integer | `completeness` |

**How to raise a challenge:**

```bash
af challenge 1.2.1 \
  --objection "The claim uses p > 0 but only p > 2 is in scope. The domain restriction is not justified." \
  --targets domain,scope \
  --agent verifier-42
```

### Resolving vs Withdrawing Challenges

**Resolve a challenge when:**
- The prover has added child nodes that address the challenge
- Those addressing nodes are valid and have been validated
- The original objection has been adequately answered

```bash
af resolve-challenge 1.2.1 --challenge ch-003 --agent verifier-42
```

**Withdraw a challenge when:**
- You realize your original objection was wrong
- The step was actually valid and you misunderstood it

```bash
af withdraw-challenge 1.2.1 --challenge ch-003 --agent verifier-42
```

### What to Check Before Accepting

The `af claim --role verifier` output provides a validation checklist:

```
VALIDATION CHECKLIST:
  v All children validated
  ? Challenge ch-003: addressed by 1.2.1.1 (validated) - you must resolve or reject
```

Before running `af accept`:
1. All checkmarks must be present (no `x` items)
2. All `?` items must be resolved (resolve or withdraw each challenge)

### Verifier Responsibilities Summary

1. **Scrutinize every step**: Look for errors, gaps, and invalid reasoning
2. **Raise specific challenges**: Be clear about what is wrong and why
3. **Review addressing nodes**: Verify that prover responses actually fix the issues
4. **Accept only valid proofs**: Never accept a step you're not confident is correct
5. **Withdraw when wrong**: If your challenge was mistaken, withdraw it

---

## Challenge Targets Reference

When raising a challenge, you must specify which aspect of the node is problematic:

| Target | When to Use | Example Objection |
|--------|-------------|-------------------|
| `statement` | The claim itself is unclear, ambiguous, or incorrect | "The statement 'p is even' is undefined for p not an integer" |
| `inference` | The reasoning rule is invalid or misapplied | "Modus ponens requires P and P->Q, but only P is provided" |
| `context` | Missing or incorrect definition/assumption references | "Uses 'prime' but doesn't cite DEF-prime" |
| `dependencies` | Missing or incorrect step references | "Claims to follow from 1.2 but 1.2 doesn't establish this" |
| `scope` | Scope violation using out-of-scope assumptions | "Uses assumption from 1.1.A which was discharged at 1.1.3" |
| `gap` | Unstated substeps required | "How do you get from 'p is prime' to 'p has exactly two divisors'?" |
| `type_error` | Type mismatch in mathematical content | "Treats n as a real but it was introduced as an integer" |
| `domain` | Unjustified domain restriction | "Assumes x > 0 but the hypothesis only gives x >= 0" |
| `completeness` | Incomplete case analysis or enumeration | "Only handles the case n > 0 but n could be negative or zero" |

**You can specify multiple targets:**
```bash
--targets inference,domain,gap
```

---

## Example Workflows

### Example 1: Simple Acceptance Flow

A leaf node with no challenges, ready for validation.

**Initial State:**
```
1.2.1 [pending] "By definition of even, 2 | p"
  (no children, no challenges)
```

**Verifier Workflow:**
```bash
# 1. Find the job
$ af jobs --role verifier
NODE 1.2.1 - Ready for validation (no challenges, leaf node)

# 2. Claim and review
$ af claim 1.2.1 --role verifier --agent verifier-42
# (review the context provided)

# 3. Accept if valid
$ af accept 1.2.1 --agent verifier-42
Node 1.2.1 accepted and validated.

# 4. Release
$ af release 1.2.1 --agent verifier-42
```

**Final State:**
```
1.2.1 [validated] "By definition of even, 2 | p"
```

---

### Example 2: Challenge-Response Cycle

A node has an issue that needs to be addressed.

**Initial State:**
```
1.2.1 [pending] "Since p is prime and 2 | p, we have 2 in {1, p}"
  (no children, no challenges)
```

**Step 1: Verifier raises challenge**
```bash
$ af claim 1.2.1 --role verifier --agent verifier-42

$ af challenge 1.2.1 \
  --objection "You assume 2 | p without justification. Where does this come from?" \
  --targets inference \
  --agent verifier-42

Challenge raised: ch-abc123

$ af release 1.2.1 --agent verifier-42
```

**State after challenge:**
```
1.2.1 [pending] "Since p is prime and 2 | p, we have 2 in {1, p}"
  Challenge ch-abc123 [open]: "You assume 2 | p without justification"
```

**Step 2: Prover addresses challenge**
```bash
$ af jobs --role prover
NODE 1.2.1 - Has open challenge needing response

$ af claim 1.2.1 --role prover --agent prover-42

$ af refine 1.2.1 \
  --statement "Since p is even, by DEF-even there exists k with p = 2k, hence 2 | p" \
  --inference by_definition \
  --context DEF-even,DEF-divides \
  --addresses ch-abc123 \
  --agent prover-42

Created node 1.2.1.1
```

**State after addressing:**
```
1.2.1 [pending] "Since p is prime and 2 | p, we have 2 in {1, p}"
  Challenge ch-abc123 [open]: addressed_by [1.2.1.1]
  1.2.1.1 [pending] "Since p is even, by DEF-even..."
```

**Step 3: Verifier reviews addressing node**
```bash
$ af claim 1.2.1.1 --role verifier --agent verifier-42

# Review and accept the child
$ af accept 1.2.1.1 --agent verifier-42

$ af release 1.2.1.1 --agent verifier-42
```

**Step 4: Verifier resolves challenge and accepts parent**
```bash
$ af claim 1.2.1 --role verifier --agent verifier-42

$ af resolve-challenge 1.2.1 --challenge ch-abc123 --agent verifier-42

$ af accept 1.2.1 --agent verifier-42

$ af release 1.2.1 --agent verifier-42
```

**Final State:**
```
1.2.1 [validated] "Since p is prime and 2 | p, we have 2 in {1, p}"
  Challenge ch-abc123 [resolved]
  1.2.1.1 [validated] "Since p is even, by DEF-even..."
```

---

### Example 3: Multi-Round Refinement

A complex proof step requires multiple rounds of refinement.

**Initial State:**
```
1.1 [pending] "Every prime p > 2 is odd"
  (no children)
```

**Round 1: Prover develops initial structure**
```bash
$ af claim 1.1 --role prover --agent prover-42

$ af refine 1.1 --children '[
  {"type": "local_assume", "statement": "Suppose p is even", "inference": "local_assume"},
  {"type": "claim", "statement": "Then p = 2k for some k", "inference": "by_definition", "context": ["DEF-even"]},
  {"type": "claim", "statement": "Since p is prime and 2 | p, we have p = 2", "inference": "by_definition", "context": ["DEF-prime"]},
  {"type": "local_discharge", "statement": "But p > 2, contradiction. So p is odd", "inference": "contradiction", "discharges": "1.1.1.A"}
]' --agent prover-42

Created nodes: 1.1.1, 1.1.2, 1.1.3, 1.1.4
```

**State after Round 1:**
```
1.1 [pending] "Every prime p > 2 is odd"
  1.1.1 [pending] "Suppose p is even" (local_assume)
  1.1.2 [pending] "Then p = 2k for some k"
  1.1.3 [pending] "Since p is prime and 2 | p, we have p = 2"
  1.1.4 [pending] "But p > 2, contradiction. So p is odd" (local_discharge)
```

**Round 2: Verifier challenges 1.1.3**
```bash
$ af claim 1.1.3 --role verifier --agent verifier-42

$ af challenge 1.1.3 \
  --objection "The jump from '2 | p and p is prime' to 'p = 2' needs justification" \
  --targets gap \
  --agent verifier-42

$ af release 1.1.3 --agent verifier-42
```

**Round 3: Prover addresses the gap**
```bash
$ af claim 1.1.3 --role prover --agent prover-42

$ af refine 1.1.3 --children '[
  {"type": "claim", "statement": "By definition of prime, the only divisors of p are 1 and p", "inference": "by_definition", "context": ["DEF-prime"]},
  {"type": "claim", "statement": "Since 2 | p and 2 is not 1, we must have 2 = p", "inference": "disjunction_elim", "dependencies": ["1.1.3.1"], "addresses_challenges": ["ch-xyz789"]}
]' --agent prover-42
```

**Round 4: Verifier accepts the refinements**
```bash
# Accept leaf nodes first
$ af claim 1.1.3.1 --role verifier --agent verifier-42
$ af accept 1.1.3.1 --agent verifier-42
$ af release 1.1.3.1 --agent verifier-42

$ af claim 1.1.3.2 --role verifier --agent verifier-42
$ af accept 1.1.3.2 --agent verifier-42
$ af release 1.1.3.2 --agent verifier-42

# Resolve challenge and accept parent
$ af claim 1.1.3 --role verifier --agent verifier-42
$ af resolve-challenge 1.1.3 --challenge ch-xyz789 --agent verifier-42
$ af accept 1.1.3 --agent verifier-42
$ af release 1.1.3 --agent verifier-42

# Continue up the tree...
```

**Final State:**
```
1.1 [validated] "Every prime p > 2 is odd"
  1.1.1 [validated] "Suppose p is even"
  1.1.2 [validated] "Then p = 2k for some k"
  1.1.3 [validated] "Since p is prime and 2 | p, we have p = 2"
    1.1.3.1 [validated] "By definition of prime..."
    1.1.3.2 [validated] "Since 2 | p and 2 is not 1..."
  1.1.4 [validated] "But p > 2, contradiction. So p is odd"
```

---

## Common Mistakes to Avoid

### Prover Mistakes

1. **Not addressing the challenge directly**: Create nodes that actually respond to what the verifier objected to
2. **Forgetting `addresses_challenges`**: Always link addressing nodes to the challenges they address
3. **Using wrong inference type**: Match your inference to what you're actually doing
4. **Ignoring scope**: Don't use assumptions that have been discharged
5. **Circular dependencies**: Don't create nodes that depend on themselves or their descendants

### Verifier Mistakes

1. **Accepting too quickly**: Always verify all criteria before accepting
2. **Vague challenges**: Be specific about what is wrong and which target applies
3. **Not reviewing addressing nodes**: Before resolving, ensure the prover's response actually fixes the issue
4. **Forgetting to resolve challenges**: All challenges must be resolved/withdrawn before accepting
5. **Not withdrawing wrong challenges**: If you made a mistake, withdraw the challenge

---

## Quick Reference Card

### Prover Commands
```bash
af jobs --role prover                    # Find work
af claim <id> --role prover --agent X    # Claim node
af refine <id> --statement "..." ...     # Add children
af release <id> --agent X                # Release node
```

### Verifier Commands
```bash
af jobs --role verifier                  # Find work
af claim <id> --role verifier --agent X  # Claim node
af challenge <id> --objection "..." ...  # Raise issue
af resolve-challenge <id> --challenge Y  # Resolve issue
af withdraw-challenge <id> --challenge Y # Retract issue
af accept <id> --agent X                 # Validate node
af release <id> --agent X                # Release node
```

### Challenge Targets (One Word Each)
- `statement` - Claim is wrong
- `inference` - Reasoning invalid
- `context` - Missing definitions
- `dependencies` - Missing steps
- `scope` - Out-of-scope use
- `gap` - Missing substeps
- `type_error` - Type mismatch
- `domain` - Bad domain assumption
- `completeness` - Incomplete cases
