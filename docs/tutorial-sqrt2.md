# Tutorial: Proving the Square Root of 2 is Irrational with AF

**Level**: Beginner
**Time**: 30-45 minutes
**Goal**: Learn the complete AF workflow by constructing a classic mathematical proof

---

## What is AF?

AF (Adversarial Proof Framework) is a command-line tool for building mathematical proofs through collaboration between AI agents. What makes AF special is its **adversarial verification** model:

- **Provers** construct proof steps, breaking complex claims into smaller, justified pieces
- **Verifiers** attack and challenge those steps, looking for gaps or errors
- No agent plays both roles on the same step

This separation creates rigorous verification - every claim must survive scrutiny before being accepted. The result is a machine-auditable proof tree with complete history, including rejected attempts and resolved challenges.

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Node** | A single proof step with a statement and justification |
| **Hierarchical IDs** | Nodes form a tree: 1 is root, 1.1 and 1.2 are children, 1.1.1 is a grandchild |
| **Ledger** | Append-only event log recording all actions - the source of truth |
| **Claim** | Temporary lock on a node while working on it |
| **Challenge** | An objection raised by a verifier about a proof step |
| **Epistemic State** | Whether a node is pending, validated, admitted, refuted, or archived |

---

## Prerequisites

Before starting, you need:

1. **Go 1.22 or later** - Check with `go version`
2. **A terminal** - Any Unix-like shell (bash, zsh, etc.)
3. **Basic math knowledge** - We will prove a classic result you may have seen before

---

## Installation

Clone and build the AF tool:

```bash
# Clone the repository
git clone https://github.com/yourusername/vibefeld.git
cd vibefeld

# Build the tool
go build ./cmd/af

# Verify it works
./af --version
```

You should see version information. If so, you are ready to go.

---

## Your First Proof: The Square Root of 2 is Irrational

We will prove one of the oldest and most beautiful results in mathematics: there is no fraction equal to the square root of 2. The proof uses contradiction - we assume the opposite and derive something impossible.

### The Mathematical Argument (Preview)

Here is the proof we will construct:

1. Assume for contradiction that sqrt(2) is rational, so sqrt(2) = a/b where a and b are coprime integers
2. Squaring both sides: 2 = a^2/b^2, so a^2 = 2b^2
3. Since a^2 = 2b^2, a^2 is even, which means a is even
4. Since a is even, write a = 2k for some integer k
5. Substituting: (2k)^2 = 2b^2, so 4k^2 = 2b^2, therefore b^2 = 2k^2
6. Since b^2 = 2k^2, b^2 is even, which means b is even
7. Both a and b are even, contradicting our assumption that they are coprime
8. Therefore sqrt(2) is irrational. QED

Now let's build this proof step-by-step using AF.

---

## Step 1: Initialize the Proof

Create a new directory for your proof and initialize it:

```bash
# Create a working directory
mkdir sqrt2-proof
cd sqrt2-proof

# Initialize the proof with your conjecture
../af init --conjecture "The square root of 2 is irrational" --author "Tutorial User"
```

You should see output like:

```
Proof initialized successfully!
  Conjecture: The square root of 2 is irrational
  Author: Tutorial User
  Directory: .

Next steps:
  1. Run 'af status' to see the proof tree
  2. Run 'af claim 1 --role prover --owner <your-name>' to start proving
  3. Run 'af refine 1 ...' to break down the root claim
```

AF has created:
- A `.af/` directory containing the proof's event ledger
- A root node (ID: `1`) containing your conjecture
- The node starts as `pending` (not yet verified) and `available` (not claimed)

### Check the Status

```bash
../af status
```

Output:

```
=== Proof Status ===

1 [pending/unresolved] The square root of 2 is irrational

--- Statistics ---
Nodes: 1 total
  Epistemic: 1 pending, 0 validated, 0 admitted, 0 refuted, 0 archived
  Taint: 0 clean, 0 self_admitted, 0 tainted, 1 unresolved

--- Jobs ---
  Prover: 0 nodes awaiting refinement
  Verifier: 1 nodes ready for review
```

The root node is waiting to be proved. Since it has no children yet, a verifier could look at it, but there is nothing to verify until we break it down.

---

## Step 2: Claim the Root Node (Prover)

Before modifying a node, you must claim it. This prevents concurrent modifications:

```bash
../af claim 1 --role prover --owner prover-agent-1
```

Output:

```
Claimed node 1 as prover (owner: prover-agent-1)
Claim expires: [timestamp]

Node context:
  ID: 1
  Statement: The square root of 2 is irrational
  Type: claim
  Epistemic: pending
  Children: (none)

Next steps:
  - Use 'af refine 1 ...' to add child steps
  - Use 'af release 1 --owner prover-agent-1' when done
```

You now have exclusive access to node 1. The `--owner` flag identifies who holds the claim.

---

## Step 3: Refine with Proof Steps (Prover)

Now break down the root claim into the proof steps. AF supports adding multiple children at once:

```bash
../af refine 1 --owner prover-agent-1 \
  "Assume for contradiction that sqrt(2) is rational. Then sqrt(2) = a/b where a, b are coprime integers with b != 0" \
  "Squaring both sides: 2 = a^2/b^2, therefore a^2 = 2b^2" \
  "Since a^2 = 2b^2, a^2 is even. An integer squared is even only if the integer itself is even, therefore a is even" \
  "Since a is even, write a = 2k for some integer k" \
  "Substituting a = 2k into a^2 = 2b^2: (2k)^2 = 2b^2, so 4k^2 = 2b^2, therefore b^2 = 2k^2" \
  "Since b^2 = 2k^2, b^2 is even. An integer squared is even only if the integer itself is even, therefore b is even" \
  "Both a and b are even, which means they share a common factor of 2. This contradicts our assumption that a and b are coprime" \
  "Therefore our assumption that sqrt(2) is rational must be false. Hence sqrt(2) is irrational. QED"
```

Output:

```
Created 8 children of node 1:
  1.1 [pending] Assume for contradiction that sqrt(2) is rational...
  1.2 [pending] Squaring both sides: 2 = a^2/b^2...
  1.3 [pending] Since a^2 = 2b^2, a^2 is even...
  1.4 [pending] Since a is even, write a = 2k...
  1.5 [pending] Substituting a = 2k into a^2 = 2b^2...
  1.6 [pending] Since b^2 = 2k^2, b^2 is even...
  1.7 [pending] Both a and b are even...
  1.8 [pending] Therefore our assumption... QED

Next steps:
  - Release your claim: 'af release 1 --owner prover-agent-1'
  - Check status: 'af status'
```

All 8 proof steps are now children of node 1, each with a unique hierarchical ID.

### Release the Claim

Once you are done refining, release the node so verifiers can review it:

```bash
../af release 1 --owner prover-agent-1
```

---

## Step 4: Check Available Jobs

Let's see what work is available:

```bash
../af jobs
```

Output:

```
=== Available Jobs ===

VERIFIER JOBS (9 nodes ready for review):
  1    The square root of 2 is irrational
  1.1  Assume for contradiction that sqrt(2) is rational...
  1.2  Squaring both sides: 2 = a^2/b^2...
  1.3  Since a^2 = 2b^2, a^2 is even...
  1.4  Since a is even, write a = 2k...
  1.5  Substituting a = 2k into a^2 = 2b^2...
  1.6  Since b^2 = 2k^2, b^2 is even...
  1.7  Both a and b are even...
  1.8  Therefore our assumption... QED

PROVER JOBS (0 nodes needing refinement):
  (none)
```

All nodes are waiting for verification. No prover jobs yet because no challenges have been raised.

---

## Step 5: Verify the Proof Steps (Verifier)

Now switch to the verifier role. A verifier examines each step and either:
- **Accepts** it if the reasoning is sound
- **Challenges** it if there's a gap or error

### Verify Step 1.1 (The Assumption)

The first step sets up proof by contradiction:

```bash
# First, look at the node in detail
../af get 1.1
```

Output:

```
Node 1.1
  Statement: Assume for contradiction that sqrt(2) is rational. Then sqrt(2) = a/b
             where a, b are coprime integers with b != 0
  Type: claim
  Inference: assumption
  Epistemic State: pending
  Workflow State: available
  Taint: unresolved
  Children: (none)
```

This is a valid setup for proof by contradiction. Accept it:

```bash
../af accept 1.1 --with-note "Valid assumption for proof by contradiction"
```

Output:

```
Accepted node 1.1
  Epistemic state: pending -> validated
  Note: Valid assumption for proof by contradiction
```

### Verify Remaining Steps

Continue verifying each step. Here's a quick way to verify simple steps:

```bash
# Verify step 1.2 (algebraic manipulation)
../af accept 1.2 --with-note "Correct algebraic manipulation: squaring both sides"

# Verify step 1.3 (even property)
../af accept 1.3 --with-note "Valid: a^2 even implies a even (contrapositive of odd^2 is odd)"

# Verify step 1.4 (definition of even)
../af accept 1.4 --with-note "Definition of even number"

# Verify step 1.5 (substitution)
../af accept 1.5 --with-note "Correct substitution and simplification"

# Verify step 1.6 (even property again)
../af accept 1.6 --with-note "Same logic as 1.3: b^2 even implies b even"

# Verify step 1.7 (contradiction)
../af accept 1.7 --with-note "Valid contradiction: both even contradicts coprime assumption"

# Verify step 1.8 (conclusion)
../af accept 1.8 --with-note "Valid conclusion from proof by contradiction"
```

### Verify the Root Node

Once all children are validated, verify the root:

```bash
../af accept 1 --with-note "Proof complete: all steps validated"
```

---

## Step 6: Check the Completed Proof

```bash
../af status
```

Output:

```
=== Proof Status ===

1 [validated/clean] The square root of 2 is irrational
  1.1 [validated/clean] Assume for contradiction that sqrt(2) is rational...
  1.2 [validated/clean] Squaring both sides: 2 = a^2/b^2...
  1.3 [validated/clean] Since a^2 = 2b^2, a^2 is even...
  1.4 [validated/clean] Since a is even, write a = 2k...
  1.5 [validated/clean] Substituting a = 2k into a^2 = 2b^2...
  1.6 [validated/clean] Since b^2 = 2k^2, b^2 is even...
  1.7 [validated/clean] Both a and b are even...
  1.8 [validated/clean] Therefore our assumption... QED

--- Statistics ---
Nodes: 9 total
  Epistemic: 0 pending, 9 validated, 0 admitted, 0 refuted, 0 archived
  Taint: 9 clean, 0 self_admitted, 0 tainted, 0 unresolved

--- Jobs ---
  Prover: 0 nodes awaiting refinement
  Verifier: 0 nodes ready for review
```

Congratulations! All nodes are `validated` and `clean`. The proof is complete.

---

## Bonus: What if a Verifier Finds an Issue?

Let's see what happens when a verifier challenges a step. We will simulate starting fresh and challenging step 1.3.

### Raising a Challenge

Suppose a verifier thinks step 1.3 needs more justification:

```bash
../af challenge 1.3 \
  --reason "Why does a^2 being even imply a is even? This needs proof." \
  --target gap \
  --severity major
```

Output:

```
Challenge raised on node 1.3
  ID: chal-abc123
  Target: gap
  Severity: major
  Reason: Why does a^2 being even imply a is even? This needs proof.

Node 1.3 now has 1 open challenge(s).
```

### Checking Jobs After a Challenge

```bash
../af jobs --role prover
```

Output:

```
=== Prover Jobs ===

1.3 [pending] Since a^2 = 2b^2, a^2 is even...
    Challenge: chal-abc123 (major)
    Reason: Why does a^2 being even imply a is even? This needs proof.
```

The challenged node now appears as a prover job.

### Responding to the Challenge (Prover)

The prover can respond by refining the step further:

```bash
# Claim the node
../af claim 1.3 --role prover --owner prover-agent-2

# Add a sub-step explaining the logic
../af refine 1.3 --owner prover-agent-2 \
  --statement "Proof: If a were odd, then a = 2m+1 for some integer m. Then a^2 = (2m+1)^2 = 4m^2 + 4m + 1 = 2(2m^2 + 2m) + 1, which is odd. But a^2 = 2b^2 is even. Contradiction, so a must be even." \
  --justification contradiction

# Release the claim
../af release 1.3 --owner prover-agent-2
```

### Resolving the Challenge (Verifier)

The verifier reviews the new sub-step and resolves the challenge:

```bash
# First accept the new sub-step
../af accept 1.3.1 --with-note "Valid proof by contradiction that even square implies even base"

# Then resolve the challenge
../af resolve-challenge chal-abc123 --response "Challenge addressed by node 1.3.1"
```

Now node 1.3 can be accepted:

```bash
../af accept 1.3 --with-note "Gap addressed in child node 1.3.1"
```

---

## Understanding the Ledger

Every action in AF is recorded in an append-only ledger. You can view it:

```bash
../af log
```

Output (abbreviated):

```
Seq  Type              Timestamp                 Details
---  ----------------  ------------------------  --------------------------------
1    proof_initialized 2026-01-16T20:53:16.158Z  The square root of 2 is irrational
2    node_created      2026-01-16T20:53:16.162Z  Node 1 (claim)
3    nodes_claimed     2026-01-16T20:53:54.464Z  [1] by prover-agent-1
4    node_created      2026-01-16T20:54:36.617Z  Node 1.1 (claim)
5    node_created      2026-01-16T20:54:36.617Z  Node 1.2 (claim)
...
20   node_validated    2026-01-16T20:56:30.845Z  Node 1.1
21   node_validated    2026-01-16T20:56:35.123Z  Node 1.2
...
```

The ledger is the **source of truth**. The current state of the proof is derived by replaying these events. This provides:

- **Auditability**: Complete history of how the proof was constructed
- **Immutability**: Past events cannot be changed
- **Recovery**: State can be rebuilt from the ledger

### Event Types

| Event Type | Description |
|------------|-------------|
| `proof_initialized` | Proof created with conjecture |
| `node_created` | New proof step added |
| `nodes_claimed` | Agent locked a node for work |
| `nodes_released` | Agent released their lock |
| `challenge_raised` | Verifier objected to a step |
| `challenge_resolved` | Challenge was addressed |
| `node_validated` | Step accepted as correct |
| `node_refuted` | Step proven incorrect |
| `node_archived` | Branch abandoned |

---

## Node States Explained

### Epistemic States (What We Know)

| State | Meaning |
|-------|---------|
| `pending` | Not yet evaluated - needs verification |
| `validated` | Accepted by a verifier as correct |
| `admitted` | Accepted without full proof (escape hatch) |
| `refuted` | Proven to be incorrect |
| `archived` | Abandoned approach, kept for history |

### Taint States (Epistemic Confidence)

| Taint | Meaning |
|-------|---------|
| `clean` | All ancestors are validated - full confidence |
| `self_admitted` | This node was admitted without proof |
| `tainted` | Depends on an admitted or tainted ancestor |
| `unresolved` | Some ancestor is still pending |

A proof is truly complete when the root is `validated` and `clean`.

---

## Command Reference

### Essential Commands

```bash
# Initialize a proof
af init --conjecture "Your theorem" --author "Your name"

# Check proof status
af status

# See available work
af jobs
af jobs --role prover
af jobs --role verifier

# Claim a node
af claim <id> --role prover --owner <name>
af claim <id> --role verifier --owner <name>

# Release a claim
af release <id> --owner <name>

# Add proof steps (prover)
af refine <id> --owner <name> "Step 1" "Step 2" "Step 3"
af refine <id> --owner <name> --statement "Single step" --justification modus_ponens

# Accept a step (verifier)
af accept <id>
af accept <id> --with-note "Optional acceptance note"

# Challenge a step (verifier)
af challenge <id> --reason "What's wrong" --target gap --severity major

# Resolve a challenge
af resolve-challenge <challenge-id> --response "How it was addressed"

# View event history
af log

# Get node details
af get <id>
```

### Getting Help

```bash
# General help
af --help

# Command-specific help
af <command> --help

# Built-in tutorial
af tutorial

# List inference types
af schema
```

---

## Next Steps

Now that you have completed your first proof, here are some ways to continue:

1. **Try another proof**: Prove that there are infinitely many primes, or that the sum of angles in a triangle is 180 degrees

2. **Explore templates**: Use `af init --list-templates` to see proof structure templates for contradiction, induction, and case analysis

3. **Use inference types**: Specify reasoning methods like `modus_ponens`, `contradiction`, `by_definition` with the `--justification` flag

4. **Add definitions**: Use `af def-add` to define terms precisely (e.g., "even", "coprime")

5. **Read the PRD**: See `docs/prd.md` for the complete specification including all commands and data model

6. **Run the orchestrator**: In production, an automated orchestrator spawns prover and verifier agents concurrently

---

## Summary

In this tutorial you learned:

1. **AF uses adversarial verification** - Provers build, verifiers attack
2. **Proofs are trees** - Each node has a hierarchical ID (1, 1.1, 1.1.1, etc.)
3. **Claims prevent conflicts** - Lock nodes before modifying them
4. **Challenges drive rigor** - Verifiers can demand more justification
5. **The ledger records everything** - Complete audit trail of the proof construction
6. **State flows through validation** - Nodes go from pending to validated (or refuted)

Happy proving!
