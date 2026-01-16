# State Machine Reference

This document describes all state machines in the AF (Adversarial Proof Framework), their valid states, transitions, and the events that trigger them.

Source: `internal/schema/`, `internal/node/`, `internal/state/`, `internal/taint/`

---

## Table of Contents

1. [Overview](#overview)
2. [Workflow State Machine](#workflow-state-machine)
3. [Epistemic State Machine](#epistemic-state-machine)
4. [Taint State Machine](#taint-state-machine)
5. [Challenge Lifecycle](#challenge-lifecycle)
6. [State Interactions](#state-interactions)
7. [State Derivation](#state-derivation)

---

## Overview

### Why AF Uses State Machines

AF uses three interconnected state machines to track proof nodes:

1. **Workflow State**: Tracks who is working on a node (operational coordination)
2. **Epistemic State**: Tracks the verification status of a node (truth status)
3. **Taint State**: Tracks epistemic uncertainty propagation (dependency contamination)

This separation serves several purposes:

- **Clean responsibility boundaries**: Each state machine tracks a single concern
- **Concurrent agent coordination**: Workflow state prevents conflicts between agents
- **Adversarial verification**: Epistemic state ensures verifier control over acceptance
- **Uncertainty propagation**: Taint state tracks when proofs rely on unverified claims

All state is derived from an append-only event ledger, ensuring:
- Full auditability of all changes
- Reproducible state computation
- No hidden state mutations

---

## Workflow State Machine

Workflow states track the operational status of a proof node - who is working on it and whether it can be worked on.

Source: `internal/schema/workflow.go`

### States

| State | Description |
|-------|-------------|
| `available` | Node is free for any agent to claim |
| `claimed` | Node is currently owned by an agent |
| `blocked` | Node cannot be worked on (e.g., awaiting dependency) |

### State Diagram

```
                         +-------------+
                         |  available  |
                         +-------------+
                               |
                               | nodes_claimed
                               | (agent claims the node)
                               v
                         +-------------+
             +-----------|   claimed   |-----------+
             |           +-------------+           |
             |                                     |
             | nodes_released                      | (node blocked
             | (agent releases)                    |  due to dependency)
             |                                     |
             v                                     v
       +-------------+                       +-------------+
       |  available  |<----------------------|   blocked   |
       +-------------+                       +-------------+
                        nodes_released
                        (blocker resolved)
```

### Transitions

| From | To | Trigger | Ledger Event |
|------|----|---------|--------------|
| `available` | `claimed` | Agent claims the node | `nodes_claimed` |
| `claimed` | `available` | Agent releases the node | `nodes_released` |
| `claimed` | `blocked` | Node blocked due to dependency | (internal transition) |
| `blocked` | `available` | Blocker resolved | `nodes_released` |

### Invalid Transitions

The following transitions are explicitly disallowed:

- `available` -> `blocked`: A node must be claimed first before it can be blocked
- `blocked` -> `claimed`: A blocked node must become available first
- Same state -> Same state: No-op transitions are not allowed

### Example: Agent Claims and Releases Node

```
Event 1: nodes_claimed {node_ids: ["1.2"], owner: "prover-001", timeout: "..."}
  State: Node 1.2 transitions from 'available' to 'claimed'

Event 2: nodes_released {node_ids: ["1.2"]}
  State: Node 1.2 transitions from 'claimed' to 'available'
```

### Validation

```go
import "github.com/tobias/vibefeld/internal/schema"

// Check if a state is valid
err := schema.ValidateWorkflowState("available")

// Check if a transition is valid
err := schema.ValidateWorkflowTransition(schema.WorkflowAvailable, schema.WorkflowClaimed)

// Check if a node can be claimed
canClaim := schema.CanClaim(schema.WorkflowAvailable) // true
```

---

## Epistemic State Machine

Epistemic states track the verification status of a proof node - whether it has been validated, admitted, refuted, or archived.

Source: `internal/schema/epistemic.go`

### States

| State | Description | Terminal | Introduces Taint |
|-------|-------------|----------|------------------|
| `pending` | Awaiting verification | No | No |
| `validated` | Verified correct by verifier | Yes | No |
| `admitted` | Accepted without full verification | Yes | Yes |
| `refuted` | Proven incorrect | Yes | No |
| `archived` | No longer relevant (branch abandoned) | Yes | No |

### State Diagram

```
                              +-------------+
                              |   pending   |
                              +-------------+
                                    |
           +------------------------+------------------------+------------------------+
           |                        |                        |                        |
           | node_validated         | node_admitted          | node_refuted           | node_archived
           | (verifier accepts)     | (verifier admits       | (verifier rejects)     | (branch abandoned)
           |                        |  without full proof)   |                        |
           v                        v                        v                        v
     +------------+           +------------+           +------------+           +------------+
     | validated  |           |  admitted  |           |  refuted   |           |  archived  |
     |  (final)   |           |  (final)   |           |  (final)   |           |  (final)   |
     +------------+           +------------+           +------------+           +------------+
```

### Transitions

| From | To | Trigger | Ledger Event | Who |
|------|----|---------|--------------|-----|
| `pending` | `validated` | Verifier accepts the node | `node_validated` | Verifier |
| `pending` | `admitted` | Verifier admits without full proof | `node_admitted` | Verifier |
| `pending` | `refuted` | Verifier rejects the node | `node_refuted` | Verifier |
| `pending` | `archived` | Proof branch abandoned | `node_archived` | Verifier |

### Key Principles

1. **Terminal States**: All states except `pending` are terminal (final). Once a node enters a terminal state, no further epistemic transitions are allowed.

2. **Verifier Control**: Only verifiers can trigger epistemic state transitions. Provers cannot accept their own work.

3. **Taint Introduction**: Only `admitted` introduces taint, which propagates to all dependent nodes.

4. **Challenge Supersession**: When a node is `refuted` or `archived`, all open challenges on it are automatically superseded.

### Example: Verification Flow

```
Event 1: node_created {node: {id: "1.2", statement: "...", epistemic_state: "pending"}}
  State: Node 1.2 created with 'pending' epistemic state

Event 2: challenge_raised {challenge_id: "c1", node_id: "1.2", target: "gap", reason: "..."}
  State: Challenge c1 is open against node 1.2

Event 3: challenge_resolved {challenge_id: "c1"}
  State: Challenge c1 resolved

Event 4: node_validated {node_id: "1.2"}
  State: Node 1.2 transitions to 'validated'
         Taint recomputed automatically
```

### Validation

```go
import "github.com/tobias/vibefeld/internal/schema"

// Check if a state is valid
err := schema.ValidateEpistemicState("pending")

// Check if a transition is valid
err := schema.ValidateEpistemicTransition(schema.EpistemicPending, schema.EpistemicValidated)

// Check if a state is terminal
isFinal := schema.IsFinal(schema.EpistemicValidated) // true

// Check if a state introduces taint
introducesTaint := schema.IntroducesTaint(schema.EpistemicAdmitted) // true
```

---

## Taint State Machine

Taint states track epistemic uncertainty propagation through the proof tree. A node's taint indicates whether its validity depends on unverified assumptions.

Source: `internal/node/node.go`, `internal/taint/compute.go`, `internal/taint/propagate.go`

### States

| State | Description |
|-------|-------------|
| `clean` | Node and all ancestors are validated |
| `self_admitted` | This node was admitted (introduced taint) |
| `tainted` | Node inherits taint from an ancestor |
| `unresolved` | Node or an ancestor is still pending |

### Computation Rules

Unlike workflow and epistemic states, taint is **computed** (not directly transitioned) based on the node's epistemic state and its ancestors' taint states. The rules are applied in priority order:

```
1. IF node.epistemic_state == 'pending' THEN taint = 'unresolved'
2. IF any ancestor.taint == 'unresolved' THEN taint = 'unresolved'
3. IF node.epistemic_state == 'admitted' THEN taint = 'self_admitted'
4. IF any ancestor.taint IN ('tainted', 'self_admitted') THEN taint = 'tainted'
5. OTHERWISE taint = 'clean'
```

### Computation Flow Diagram

```
                          +---------------------------+
                          | Is node pending?          |
                          +---------------------------+
                                       |
                           +-----------+-----------+
                          Yes                      No
                           |                       |
                           v                       v
                     +------------+    +---------------------------+
                     | unresolved |    | Any ancestor unresolved?  |
                     +------------+    +---------------------------+
                                                    |
                                        +-----------+-----------+
                                       Yes                      No
                                        |                       |
                                        v                       v
                                  +------------+    +---------------------------+
                                  | unresolved |    | Is node admitted?         |
                                  +------------+    +---------------------------+
                                                                 |
                                                     +-----------+-----------+
                                                    Yes                      No
                                                     |                       |
                                                     v                       v
                                               +--------------+  +-------------------------------+
                                               | self_admitted|  | Ancestor tainted/self_admitted?|
                                               +--------------+  +-------------------------------+
                                                                              |
                                                                  +-----------+-----------+
                                                                 Yes                      No
                                                                  |                       |
                                                                  v                       v
                                                            +---------+            +-------+
                                                            | tainted |            | clean |
                                                            +---------+            +-------+
```

### Taint Propagation

When a node's epistemic state changes, taint must be recomputed for:
1. The node itself
2. All descendant nodes (in depth-first order, shallower first)

This propagation is automatic and happens in `internal/state/apply.go` after any epistemic state change.

### Taint Events

| Event | Description |
|-------|-------------|
| `taint_recomputed` | Node's taint state was recalculated |

### Example: Taint Propagation

Consider a proof tree:
```
1 (validated, clean)
  1.1 (admitted, self_admitted)
    1.1.1 (pending, unresolved)
    1.1.2 (validated, tainted)  <- tainted because 1.1 is self_admitted
  1.2 (validated, clean)
```

If node 1.1.1 becomes validated:
```
Before: 1.1.1 (pending, unresolved)
After:  1.1.1 (validated, tainted)  <- now tainted from parent 1.1
```

### API Reference

```go
import "github.com/tobias/vibefeld/internal/taint"

// Compute taint for a single node
taintState := taint.ComputeTaint(node, ancestors)

// Propagate taint to all descendants
changedNodes := taint.PropagateTaint(root, allNodes)

// Propagate and generate events
changedNodes, events := taint.PropagateAndGenerateEvents(root, allNodes)
```

---

## Challenge Lifecycle

Challenge states track the lifecycle of verifier challenges against proof nodes.

Source: `internal/node/challenge.go`, `internal/schema/severity.go`

### Challenge States

| State | Description |
|-------|-------------|
| `open` | Challenge is active and awaiting response |
| `resolved` | Challenge was addressed by the prover |
| `withdrawn` | Verifier withdrew the challenge |
| `superseded` | Challenge became moot (parent node archived/refuted) |

### State Diagram

```
                              +-------------+
                              |    open     |
                              +-------------+
                                    |
           +------------------------+------------------------+
           |                        |                        |
           | challenge_resolved     | challenge_withdrawn    | challenge_superseded
           | (prover answers)       | (verifier retracts)    | (node archived/refuted)
           |                        |                        |
           v                        v                        v
     +------------+           +------------+           +-------------+
     |  resolved  |           | withdrawn  |           | superseded  |
     |   (final)  |           |   (final)  |           |   (final)   |
     +------------+           +------------+           +-------------+
```

### Transitions

| From | To | Trigger | Ledger Event |
|------|----|---------|--------------|
| `open` | `resolved` | Prover provides resolution | `challenge_resolved` |
| `open` | `withdrawn` | Verifier withdraws challenge | `challenge_withdrawn` |
| `open` | `superseded` | Parent node archived or refuted | `challenge_superseded` |

### Challenge Severity

Challenges have severity levels that determine whether they block node acceptance:

| Severity | Description | Blocks Acceptance |
|----------|-------------|-------------------|
| `critical` | Fundamental error that must be fixed | Yes |
| `major` | Significant issue that should be addressed | Yes |
| `minor` | Minor issue that could be improved | No |
| `note` | Clarification request or suggestion | No |

### Challenge Targets

When raising a challenge, the verifier must specify which aspect of the node is being challenged:

| Target | Description |
|--------|-------------|
| `statement` | The claim text itself is disputed |
| `inference` | The inference type is inappropriate |
| `context` | Referenced definitions are wrong/missing |
| `dependencies` | Node dependencies are incorrect |
| `scope` | Scope/local assumption issues |
| `gap` | Logical gap in reasoning |
| `type_error` | Type mismatch in mathematical objects |
| `domain` | Domain restriction violation |
| `completeness` | Missing cases or incomplete argument |

### Example: Challenge Lifecycle

```
Event 1: challenge_raised {
  challenge_id: "c1",
  node_id: "1.2",
  target: "gap",
  reason: "Missing justification for step",
  severity: "major"
}
  State: Challenge c1 created with status 'open'

Event 2: challenge_resolved {challenge_id: "c1"}
  State: Challenge c1 transitions to 'resolved'
```

### API Reference

```go
import "github.com/tobias/vibefeld/internal/node"

// Create a new challenge
challenge, err := node.NewChallenge(id, targetID, target, reason)

// Resolve a challenge
err := challenge.Resolve("Resolution explanation")

// Withdraw a challenge
err := challenge.Withdraw()

// Check if challenge is open
isOpen := challenge.IsOpen()
```

---

## State Interactions

### Workflow + Epistemic

The workflow and epistemic state machines operate on different concerns but interact in important ways:

1. **Claiming enables work**: A prover must claim a node before refining it
2. **Verifiers make decisions**: A verifier can validate/refute a node (changing epistemic state) regardless of workflow state
3. **Release after completion**: Nodes should be released after epistemic transitions are complete

```
                    Workflow                          Epistemic
                    --------                          ---------
   Start:           available                         pending

   Prover claims:   claimed                           pending
                       |                                 |
   Prover refines:     |                                 |
                       |                                 |
   Verifier validates: |                              validated
                       v                                 |
   Prover releases: available                            |
                                                         v
                                                     (terminal)
```

### Epistemic + Taint

Epistemic state changes automatically trigger taint recomputation:

| Epistemic Transition | Taint Effect |
|---------------------|--------------|
| `pending` -> `validated` | Taint may change from `unresolved` to `clean` (if no tainted ancestors) |
| `pending` -> `admitted` | Taint changes to `self_admitted` |
| `pending` -> `refuted`/`archived` | Taint remains `unresolved` (node is invalid anyway) |

Taint then propagates to all descendants.

### Epistemic + Challenge

Challenges and epistemic state are tightly coupled:

1. **Challenges on pending nodes**: Challenges can only be raised against nodes in `pending` epistemic state
2. **Blocking acceptance**: Open challenges with `critical` or `major` severity block acceptance
3. **Auto-supersession**: When a node transitions to `refuted` or `archived`, all open challenges on it automatically become `superseded`

### Combined State Diagram

```
Node Created
     |
     v
+--------------------------------------------+
|            PENDING / AVAILABLE             |
|              taint: unresolved             |
+--------------------------------------------+
     |
     | Prover claims
     v
+--------------------------------------------+
|            PENDING / CLAIMED               |
|              taint: unresolved             |
+--------------------------------------------+
     |
     | Verifier raises challenges
     | Prover refines, resolves challenges
     |
     +-----------------+-----------------+
     |                 |                 |
     | validate        | admit           | refute/archive
     v                 v                 v
+---------------+ +---------------+ +---------------+
| VALIDATED     | | ADMITTED      | | REFUTED       |
| taint: clean  | | taint: self_  | | taint: n/a    |
| or tainted*   | | admitted      | |               |
+---------------+ +---------------+ +---------------+

* taint depends on ancestors
```

### Invariants

The following invariants must always hold:

1. **Terminal states are final**: Nodes in `validated`, `admitted`, `refuted`, or `archived` cannot change epistemic state
2. **Taint follows computation rules**: Taint is always computed (never manually set)
3. **Challenges cannot block terminal nodes**: Challenges on non-pending nodes are superseded
4. **Workflow transitions are validated**: Invalid workflow transitions are rejected

### Edge Cases

1. **Orphaned challenges**: If a node is archived/refuted, all open challenges become superseded
2. **Taint with archived ancestors**: Archived nodes don't propagate taint (they're not valid)
3. **Concurrent claims**: Only one agent can claim a node at a time
4. **Claim timeout**: Stale claims are reaped, returning nodes to available state

---

## State Derivation

### Event Sourcing Model

AF uses event sourcing: all state changes are recorded as immutable events in the ledger, and current state is derived by replaying events.

### Event Types

The full list of events that affect state:

| Event | State Change |
|-------|--------------|
| `proof_initialized` | Creates the proof context |
| `node_created` | Adds a node to state (pending/available/unresolved) |
| `nodes_claimed` | Workflow: available -> claimed |
| `nodes_released` | Workflow: claimed/blocked -> available |
| `claim_refreshed` | Updates claim timeout (no state change) |
| `challenge_raised` | Adds challenge with status 'open' |
| `challenge_resolved` | Challenge status -> 'resolved' |
| `challenge_withdrawn` | Challenge status -> 'withdrawn' |
| `challenge_superseded` | Challenge status -> 'superseded' |
| `node_validated` | Epistemic: pending -> validated; triggers taint recompute |
| `node_admitted` | Epistemic: pending -> admitted; triggers taint recompute |
| `node_refuted` | Epistemic: pending -> refuted; auto-supersedes challenges |
| `node_archived` | Epistemic: pending -> archived; auto-supersedes challenges |
| `node_amended` | Updates node statement; recomputes content hash |
| `taint_recomputed` | Updates node taint state |
| `def_added` | Adds a definition to state |
| `lemma_extracted` | Adds a lemma to state |
| `scope_opened` | Opens assumption scope at node |
| `scope_closed` | Closes assumption scope |
| `lock_reaped` | Records cleanup of stale claim |

### Replay Process

State is rebuilt by replaying events in sequence order:

```go
func Replay(ldg *ledger.Ledger) (*State, error) {
    state := NewState()

    err := ldg.Scan(func(seq int, data []byte) error {
        // 1. Parse event from JSON
        event, err := parseEvent(data)
        if err != nil {
            return err
        }

        // 2. Apply event to state
        if err := Apply(state, event); err != nil {
            return err
        }

        // 3. Track sequence for concurrency control
        state.SetLatestSeq(seq)

        return nil
    })

    return state, err
}
```

### Validation During Replay

The replay process validates:

1. **Sequence continuity**: No gaps in sequence numbers
2. **Transition validity**: Each state transition is legal
3. **Reference integrity**: Events reference existing nodes/challenges
4. **Content hashes**: Optional verification of node content integrity

### Example Replay

Given this event sequence:
```json
{"seq": 1, "type": "proof_initialized", "conjecture": "P implies Q"}
{"seq": 2, "type": "node_created", "node": {"id": "1", "statement": "P implies Q"}}
{"seq": 3, "type": "nodes_claimed", "node_ids": ["1"], "owner": "prover-1"}
{"seq": 4, "type": "node_created", "node": {"id": "1.1", "statement": "Assume P"}}
{"seq": 5, "type": "challenge_raised", "challenge_id": "c1", "node_id": "1.1", "target": "gap"}
{"seq": 6, "type": "challenge_resolved", "challenge_id": "c1"}
{"seq": 7, "type": "node_validated", "node_id": "1.1"}
```

State after replay:
```
Nodes:
  1:   workflow=claimed, epistemic=pending, taint=unresolved
  1.1: workflow=available, epistemic=validated, taint=clean

Challenges:
  c1: status=resolved, node_id=1.1

Latest sequence: 7
```

### Optimistic Concurrency Control

The `latestSeq` field enables optimistic concurrency:

```go
// When appending new events:
// 1. Read current state
state, _ := Replay(ledger)

// 2. Check sequence hasn't changed
currentSeq := state.LatestSeq()

// 3. Append with CAS (compare-and-swap)
err := ledger.AppendWithSeq(event, currentSeq+1)
```

This ensures no events are lost if multiple agents modify the ledger concurrently.

---

## Summary

AF's state machine architecture provides:

1. **Separation of concerns**: Three independent but interacting state machines
2. **Auditability**: All changes recorded in append-only ledger
3. **Reproducibility**: State computed deterministically from events
4. **Concurrency safety**: Workflow state coordinates agent access
5. **Epistemic rigor**: Clear verification workflow with verifier control
6. **Uncertainty tracking**: Taint propagation through dependencies

This design supports the adversarial proof framework's core principle: provers convince, verifiers attack, and the tool enforces the rules.
