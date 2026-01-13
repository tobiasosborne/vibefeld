# AF Tutorial: Your First Proof

This tutorial walks through a complete proof workflow using the `af` CLI tool. You'll learn how prover and verifier agents collaborate to construct a rigorous mathematical proof.

## Overview

AF (Adversarial Proof Framework) uses an adversarial model where:
- **Provers** break down claims into smaller, more defensible steps
- **Verifiers** challenge and ultimately accept or reject those steps

The core workflow is:

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│  init   │ ──▶ │  claim  │ ──▶ │ refine  │ ──▶ │ release │ ──▶ │ accept  │
└─────────┘     └─────────┘     └─────────┘     └─────────┘     └─────────┘
   Setup         Prover          Prover          Prover         Verifier
                 claims          adds            releases       accepts
                 node            children        node           children
```

## Prerequisites

Build the `af` binary:

```bash
go build ./cmd/af
```

## Step 1: Initialize a Proof

Create a new proof workspace with your conjecture:

```bash
mkdir my-proof && cd my-proof

af init \
  --conjecture "All prime numbers greater than 2 are odd" \
  --author "tutorial-user"
```

**Output:**
```
Proof initialized successfully in .
Conjecture: All prime numbers greater than 2 are odd
Author: tutorial-user

Next steps:
  af status    - View proof status
  af claim     - Claim a job to work on
```

This creates the ledger directory structure where all proof events are recorded.

## Step 2: Create the Root Goal

The conjecture needs to become a provable goal node. Use the service layer to create the root node:

```bash
# In a real system, this would be done automatically or via a dedicated command
# For now, we use the proof programmatically or the init could be extended
```

> **Note:** Currently `af init` creates the proof metadata but not the root node. A future version will create the root goal automatically.

## Step 3: Check Available Jobs

See what work is available for agents:

```bash
af jobs
```

**Output:**
```
Prover Jobs (1):
  1. Node 1 [pending] - "All prime numbers greater than 2 are odd"

Verifier Jobs (0):
  (none available)

Summary: 1 prover job, 0 verifier jobs
```

Use `--format json` for machine-readable output:

```bash
af jobs --format json
```

```json
{
  "prover_jobs": [
    {"id": "1", "statement": "All prime numbers greater than 2 are odd", "state": "pending"}
  ],
  "verifier_jobs": []
}
```

## Step 4: Claim a Node (Prover)

As a prover agent, claim the root node to work on it:

```bash
af claim 1 --owner prover-agent-001
```

**Output:**
```
Node 1 claimed successfully.
Owner: prover-agent-001
Timeout: 1h0m0s

You have claimed this node as a PROVER.
Your task: Break down the claim into smaller, defensible steps.

Context:
  Statement: All prime numbers greater than 2 are odd
  Type: claim
  State: pending

Next steps:
  af refine 1 --statement "..." --owner prover-agent-001
  af release 1 --owner prover-agent-001  (if you need to abandon)
```

The claim prevents other agents from modifying this node while you work.

## Step 5: Refine the Node (Prover)

Break down the claim into smaller proof steps:

```bash
# First child: establish the key property
af refine 1 \
  --statement "Let n be a prime number where n > 2. By definition, n has exactly two divisors: 1 and n." \
  --inference assumption \
  --owner prover-agent-001
```

**Output:**
```
Child node created: 1.1
Parent: 1
Statement: Let n be a prime number where n > 2. By definition, n has exactly two divisors: 1 and n.
Inference: assumption
Type: claim

Next steps:
  af refine 1 --statement "..." --owner prover-agent-001  (add more children)
  af release 1 --owner prover-agent-001  (when done refining)
```

Add another refinement:

```bash
af refine 1 \
  --statement "If n were even, then n would be divisible by 2. Since n > 2, this would mean n has a divisor other than 1 and n, contradicting primality. Therefore n must be odd." \
  --inference modus_ponens \
  --owner prover-agent-001
```

**Output:**
```
Child node created: 1.2
...
```

## Step 6: Release the Node (Prover)

When you're done refining, release the node so verifiers can review:

```bash
af release 1 --owner prover-agent-001
```

**Output:**
```
Node 1 released successfully.
Previous owner: prover-agent-001

The node is now available for other agents to claim.

Next steps:
  af jobs  - See available work
```

## Step 7: Check Jobs Again

Now verifier jobs should be available:

```bash
af jobs
```

**Output:**
```
Prover Jobs (2):
  1. Node 1.1 [pending] - "Let n be a prime number..."
  2. Node 1.2 [pending] - "If n were even..."

Verifier Jobs (0):
  (none yet - children need validation first)
```

## Step 8: Claim as Verifier

A verifier claims a child node to review it:

```bash
af claim 1.1 --owner verifier-agent-001
```

**Output:**
```
Node 1.1 claimed successfully.
Owner: verifier-agent-001

You have claimed this node as a VERIFIER.
Your task: Verify the claim is correct and well-justified.

Context:
  Statement: Let n be a prime number where n > 2...
  Type: claim
  Inference: assumption
  State: pending

Next steps:
  af accept 1.1  (if the claim is valid)
  af challenge 1.1 --reason "..." --target statement  (if there's an issue)
```

## Step 9: Accept the Node (Verifier)

If the proof step is valid, accept it:

```bash
af accept 1.1
```

**Output:**
```
Node 1.1 accepted and validated.

The node's epistemic state is now: validated

Next steps:
  af jobs  - Find more work
```

## Step 10: Verify the Final State

Check the proof structure:

```bash
af jobs --format json
```

The validated node (1.1) is no longer in the job queues. Continue the process:
1. Claim and accept node 1.2
2. Once all children are validated, the parent (node 1) becomes a verifier job
3. Accept the parent to complete that branch

## Complete Workflow Summary

```
┌──────────────────────────────────────────────────────────────────────┐
│                         PROOF WORKFLOW                                │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  1. af init --conjecture "..." --author "..."                        │
│     └── Creates proof workspace                                       │
│                                                                       │
│  2. af jobs                                                          │
│     └── Shows available prover/verifier work                         │
│                                                                       │
│  3. af claim <node> --owner <agent-id>                               │
│     └── Locks node for exclusive work                                │
│                                                                       │
│  4. af refine <node> --statement "..." --owner <agent-id>            │
│     └── Adds child nodes (can repeat multiple times)                 │
│                                                                       │
│  5. af release <node> --owner <agent-id>                             │
│     └── Unlocks node for others                                      │
│                                                                       │
│  6. af claim <child> --owner <verifier-id>                           │
│     └── Verifier claims child node                                   │
│                                                                       │
│  7. af accept <child>                                                │
│     └── Validates the proof step                                     │
│                                                                       │
│  (Repeat 6-7 for all children, then parent becomes verifier job)     │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

## Handling Challenges

If a verifier finds an issue with a proof step:

```bash
# Raise a challenge
af challenge 1.1 \
  --reason "The statement assumes n > 2 but doesn't establish this from the hypothesis" \
  --target statement

# The prover can resolve by providing clarification
af resolve-challenge ch-abc123 \
  --response "The n > 2 condition is inherited from the parent conjecture which states 'greater than 2'"

# Or the verifier can withdraw if they realize the challenge was unfounded
af withdraw-challenge ch-abc123
```

## JSON Output for Automation

All commands support `--format json` for scripting:

```bash
af claim 1 --owner agent-001 --format json
```

```json
{
  "node_id": "1",
  "owner": "agent-001",
  "claimed": true,
  "timeout": "1h0m0s"
}
```

## Key Principles

1. **Adversarial Verification**: Provers convince, verifiers attack. No agent plays both roles on the same node.

2. **Agent Isolation**: Each agent claims one job, works, releases. No context bleeding between agents.

3. **Append-Only Ledger**: All events are recorded. The current state is derived from history.

4. **Hierarchical IDs**: Root is `1`, children are `1.1`, `1.2`, grandchildren are `1.1.1`, etc.

5. **Claim Ownership**: The `--owner` flag must match across claim, refine, and release operations.

## Next Steps

- Explore `af --help` for all available commands
- Check `af <command> --help` for detailed flag information
- Review the ledger files in `./ledger/` to see the event history
