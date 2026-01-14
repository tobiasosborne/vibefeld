# Challenge Workflow Documentation

This document describes the challenge-response workflow in AF (Adversarial Proof Framework). Challenges are the core mechanism by which verifiers identify issues with proof nodes, and provers address those issues.

## Overview

The challenge system implements adversarial verification where:
- **Verifiers** raise challenges to identify issues with proof steps
- **Provers** address challenges by adding child nodes that justify the disputed step
- **Verifiers** then review the addressing nodes and resolve or further challenge

This creates an iterative refinement process that produces rigorous, auditable proofs.

---

## Challenge Lifecycle

### Challenge States

A challenge can be in one of four states:

| State | Meaning |
|-------|---------|
| `open` | Challenge is active and awaiting response |
| `resolved` | Verifier is satisfied by the addressing nodes |
| `withdrawn` | Verifier retracted the challenge (they were wrong) |
| `superseded` | Parent node was archived/refuted, making challenge moot |

### State Transitions

```
                    +-----------+
                    |   open    |
                    +-----------+
                   /      |      \
                  /       |       \
                 v        v        v
         +---------+  +----------+  +-----------+
         |resolved |  |withdrawn |  |superseded |
         +---------+  +----------+  +-----------+
```

**Transitions:**
- `open` -> `resolved`: Verifier accepts the prover's addressing nodes
- `open` -> `withdrawn`: Verifier retracts the challenge
- `open` -> `superseded`: Parent node is archived or refuted (automatic)

All terminal states (`resolved`, `withdrawn`, `superseded`) are final.

---

## Raising Challenges (Verifier Action)

### When to Raise a Challenge

Verifiers raise challenges when they identify issues with a proof node. Each challenge must specify:
1. **Target**: What aspect of the node is problematic
2. **Reason**: A clear explanation of the issue

### Challenge Targets

| Target | When to Use |
|--------|-------------|
| `statement` | The claim text itself is unclear, ambiguous, or wrong |
| `inference` | The inference rule is invalid or misapplied |
| `context` | Missing or incorrect definition/assumption references |
| `dependencies` | Missing or incorrect step references |
| `scope` | Scope violation (using out-of-scope assumptions) |
| `gap` | Unstated substeps are required |
| `type_error` | Type mismatch in mathematical content |
| `domain` | Unjustified domain restriction (e.g., assuming x > 0) |
| `completeness` | Incomplete case analysis or enumeration |

### Command: `af challenge`

```bash
af challenge <node-id> --reason "Explanation of the issue" --target <target>
```

**Example:**
```bash
af challenge 1.2.1 \
  --reason "You assume 2 | p without justification. Need to show divisibility." \
  --target inference
```

**Output:**
```
Challenge raised against node 1.2.1
  Challenge ID: ch-a1b2c3d4e5f6g7h8
  Target:       inference
  Reason:       You assume 2 | p without justification. Need to show divisibility.

Next steps:
  af resolve-challenge  - Resolve this challenge with an explanation
  af withdraw-challenge - Withdraw this challenge if no longer relevant
```

### Ledger Event

When a challenge is raised, a `ChallengeRaised` event is appended to the ledger:

```json
{
  "type": "challenge_raised",
  "challenge_id": "ch-a1b2c3d4e5f6g7h8",
  "node_id": "1.2.1",
  "target": "inference",
  "reason": "You assume 2 | p without justification."
}
```

---

## Responding to Challenges (Prover Action)

### How Challenges Create Prover Jobs

When a node has open challenges, it becomes a **prover job**. The prover must:
1. Claim the challenged node
2. Add child nodes that address the challenge
3. Release the node for verifier review

### The `addresses_challenges` Field

When creating child nodes, the prover specifies which challenges the new node addresses using the `addresses_challenges` field:

```json
{
  "action": "refine",
  "parent": "1.2.1",
  "children": [
    {
      "type": "claim",
      "statement": "By definition of even, if p = 2k then 2 | p",
      "inference": "by_definition",
      "context": ["DEF-even", "DEF-divides"],
      "dependencies": ["1.2.1"],
      "addresses_challenges": ["ch-a1b2c3d4e5f6g7h8"]
    }
  ]
}
```

### The `addressed_by` Field (Automatic Update)

When a child node specifies `addresses_challenges`, the challenge's `addressed_by` field is automatically updated to include that child node's ID. This creates a bidirectional link:

- **Node** -> Challenge: via `addresses_challenges` on the node
- **Challenge** -> Node: via `addressed_by` on the challenge

This allows verifiers to easily find which nodes were created to address a challenge.

### Command: `af refine` with Challenge Addressing

```bash
af refine 1.2.1 \
  --owner prover-agent \
  --statement "By definition of even, if p = 2k then 2 | p" \
  --inference by_definition \
  --addresses ch-a1b2c3d4e5f6g7h8
```

Or using the JSON children format:

```bash
af refine 1.2.1 \
  --owner prover-agent \
  --children '[{
    "statement": "By definition of even, if p = 2k then 2 | p",
    "inference": "by_definition",
    "addresses_challenges": ["ch-a1b2c3d4e5f6g7h8"]
  }]'
```

---

## Reviewing Addressed Challenges (Verifier Action)

### When a Challenge is Ready for Review

A challenge is ready for review when:
1. The challenge is in `open` state
2. The `addressed_by` field is non-empty (prover has added addressing nodes)

### Verifier Options

After reviewing the addressing nodes, the verifier can:

1. **Resolve the challenge** - Accept that the addressing nodes adequately respond to the objection
2. **Raise new challenges** - If the addressing nodes themselves have issues
3. **Withdraw the challenge** - If the verifier realizes their original objection was invalid

### Command: `af resolve-challenge`

```bash
af resolve-challenge <challenge-id> --response "Explanation of why resolved"
```

**Example:**
```bash
af resolve-challenge ch-a1b2c3d4e5f6g7h8 \
  --response "Child node 1.2.1.1 correctly justifies the divisibility claim using the definition of even."
```

**Ledger Event:**
```json
{
  "type": "challenge_resolved",
  "challenge_id": "ch-a1b2c3d4e5f6g7h8"
}
```

### Command: `af withdraw-challenge`

Use when the verifier realizes their challenge was wrong:

```bash
af withdraw-challenge <challenge-id>
```

**Example:**
```bash
af withdraw-challenge ch-a1b2c3d4e5f6g7h8
```

**Ledger Event:**
```json
{
  "type": "challenge_withdrawn",
  "challenge_id": "ch-a1b2c3d4e5f6g7h8"
}
```

---

## Validation Invariant

A node can only be validated (`af accept`) when ALL of the following are true:

1. **All challenges are closed**: Every challenge on the node has state in `{resolved, withdrawn, superseded}`
2. **Resolved challenges have validated addressing nodes**: For each challenge with `state = resolved`, at least one node in `addressed_by` has `epistemic_state = validated`
3. **All children are accepted**: All children have `epistemic_state` in `{validated, admitted}`
4. **Scope entries are closed**: All scope entries opened by the node (if `local_assume`) are closed by a descendant

### What Happens When Validation Fails

```
$ af accept 1.2.1

Error: VALIDATION_INVARIANT_FAILED

Cannot accept node 1.2.1:
  x Challenge ch-003 has state 'open' (must be resolved/withdrawn/superseded)
  v All children validated

To proceed, either:
  * Resolve the challenge (if addressing nodes are satisfactory):
      af resolve-challenge ch-003 --response "..."
  * Withdraw the challenge (if no longer valid):
      af withdraw-challenge ch-003
  * Wait for prover to address the challenge:
      af jobs --role prover
```

---

## Automatic Supersession

Challenges are automatically superseded when their parent node is archived or refuted.

### When Supersession Occurs

- Node is archived: `af archive <node-id> --reason "..."`
- Node is refuted: `af refute <node-id> --reason "..."`

All open challenges on the archived/refuted node (and its descendants) become `superseded`.

### Why Supersession Exists

When a proof branch is abandoned (archived) or proven wrong (refuted), any pending challenges on that branch become moot. There's no point in addressing challenges on nodes that are no longer part of the proof.

### Example

```
Node 1.2 [archived] "Alternative approach"
  Challenge ch-001 [superseded] (automatically, because 1.2 is archived)
  Child 1.2.1 [archived] (automatically archived with parent)
    Challenge ch-002 [superseded] (automatically)
```

---

## Complete Example Workflow

This example shows a full challenge-response cycle.

### Step 1: Initial State

```
1 [pending] "All primes greater than 2 are odd"
  1.1 [pending] "Let p > 2 be prime"
    1.1.1 [pending] "Since p is prime and p = 2k, we have 2 | p"
```

### Step 2: Verifier Raises Challenge

The verifier identifies that node 1.1.1 assumes divisibility without justification.

```bash
$ af challenge 1.1.1 \
    --reason "You assume 2 | p without justification. Where does p = 2k come from?" \
    --target inference

Challenge raised against node 1.1.1
  Challenge ID: ch-abc123
  Target:       inference
  Reason:       You assume 2 | p without justification. Where does p = 2k come from?
```

### Step 3: Node Becomes Prover Job

```bash
$ af jobs --role prover

PROVER JOBS AVAILABLE

NODE 1.1.1
  Statement: Since p is prime and p = 2k, we have 2 | p
  Reason: Has open challenge needing response
  Challenge: ch-abc123 "You assume 2 | p without justification..."

  To work on this:
    af claim 1.1.1 --role prover --agent <your-agent-id>

Total: 1 prover job
```

### Step 4: Prover Claims and Addresses

```bash
$ af claim 1.1.1 --role prover --agent prover-42

Claimed node 1.1.1 for prover work.
...
```

```bash
$ af refine 1.1.1 \
    --owner prover-42 \
    --statement "By definition of even, p = 2k implies 2 | p" \
    --inference by_definition \
    --context DEF-even,DEF-divides

Node refined successfully.
  Parent: 1.1.1
  Child:  1.1.1.1
```

```bash
$ af release 1.1.1 --agent prover-42

Node 1.1.1 released.
```

### Step 5: Verifier Reviews and Accepts Child

```bash
$ af claim 1.1.1.1 --role verifier --agent verifier-99

Claimed node 1.1.1.1 for verifier work.
...
```

```bash
$ af accept 1.1.1.1

Node 1.1.1.1 accepted and validated.
```

### Step 6: Verifier Resolves Challenge

```bash
$ af resolve-challenge ch-abc123 \
    --response "Child node 1.1.1.1 correctly justifies the divisibility claim"

Challenge ch-abc123 resolved successfully.
```

### Step 7: Verifier Accepts Original Node

```bash
$ af accept 1.1.1

Node 1.1.1 accepted and validated.
```

### Final State

```
1 [pending] "All primes greater than 2 are odd"
  1.1 [pending] "Let p > 2 be prime"
    1.1.1 [validated] "Since p is prime and p = 2k, we have 2 | p"
      1.1.1.1 [validated] "By definition of even, p = 2k implies 2 | p"
```

Challenge ch-abc123 is now `resolved`, and both nodes are validated.

---

## Job Detection with Challenges

### Prover Jobs

A node appears as a prover job when:
- `workflow_state = available`
- `epistemic_state = pending`
- AND one of:
  - Node has no children (needs development)
  - Node has challenges with `state = open` that have empty `addressed_by`

### Verifier Jobs

A node appears as a verifier job when:
- `workflow_state = available`
- `epistemic_state = pending`
- Either:
  - Node has no open challenges and has children to evaluate
  - All challenges have non-empty `addressed_by` (verifier can resolve and potentially accept)

---

## Challenge Data Model

### Challenge Structure

```json
{
  "id": "ch-abc123",
  "target_id": "1.2.1",
  "target": "inference",
  "reason": "You assume 2 | p without justification.",
  "raised": "2025-01-11T10:06:00Z",
  "status": "open",
  "resolved_at": null,
  "resolution": null,
  "addressed_by": []
}
```

### Challenge in Node Context

Challenges are associated with nodes:

```json
{
  "id": "1.2.1",
  "statement": "Since p is prime and p = 2k, we have 2 | p",
  "challenges": [
    {
      "id": "ch-abc123",
      "target": "inference",
      "reason": "You assume 2 | p without justification.",
      "status": "open",
      "addressed_by": []
    }
  ]
}
```

---

## Best Practices

### For Verifiers

1. **Be specific**: Challenge reasons should clearly identify what is wrong
2. **Choose the right target**: Use the most specific target that applies
3. **One issue per challenge**: Raise separate challenges for distinct issues
4. **Review thoroughly**: Before resolving, verify the addressing nodes actually fix the issue
5. **Withdraw gracefully**: If you realize your challenge was wrong, withdraw it

### For Provers

1. **Address directly**: Create nodes that directly respond to the challenge
2. **Use `addresses_challenges`**: Always specify which challenge(s) a node addresses
3. **Don't over-refine**: Address just what's needed for the challenge
4. **Provide justification**: The addressing node's inference and context should make the fix clear

---

## Error Handling

### Common Errors

| Error | Cause | Resolution |
|-------|-------|------------|
| `CHALLENGE_NOT_FOUND` | Challenge ID doesn't exist | Check the challenge ID with `af status` |
| `CHALLENGE_ALREADY_RESOLVED` | Trying to resolve/withdraw a closed challenge | Challenge is already in terminal state |
| `VALIDATION_INVARIANT_FAILED` | Trying to accept node with open challenges | Resolve or withdraw all challenges first |
| `NODE_NOT_FOUND` | Challenge references non-existent node | Verify node exists |

### Exit Codes

- `1` = Retriable error (e.g., concurrent modification)
- `2` = Blocked (waiting for definition)
- `3` = Logic error (invalid challenge target, etc.)
- `4` = Corruption (ledger inconsistency)

---

## Summary

The challenge workflow is the heart of adversarial verification in AF:

1. **Verifiers identify issues** by raising challenges with specific targets and reasons
2. **Provers address challenges** by adding child nodes with `addresses_challenges`
3. **Verifiers review responses** and resolve, withdraw, or further challenge
4. **Nodes can only be validated** once all challenges are in terminal states with validated addressing nodes

This iterative process ensures that every step in the proof is rigorously justified and any gaps are explicitly identified and addressed.
