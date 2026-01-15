# State Machine Reference

This document describes all state machines in the AF (Adversarial Proof Framework), their valid states, transitions, and the events that trigger them.

Source: `internal/schema/` and `internal/node/`

---

## Table of Contents

1. [Workflow States](#workflow-states)
2. [Epistemic States](#epistemic-states)
3. [Taint States](#taint-states)
4. [Challenge States](#challenge-states)

---

## Workflow States

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
                    ┌─────────────┐
                    │  available  │
                    └─────────────┘
                          │
                          │ NodesClaimed event
                          │ (agent claims the node)
                          ▼
                    ┌─────────────┐
        ┌──────────│   claimed   │──────────┐
        │          └─────────────┘          │
        │                                   │
        │ NodesReleased event               │ (node blocked
        │ (agent releases the node)         │  due to dependency)
        │                                   │
        ▼                                   ▼
  ┌─────────────┐                    ┌─────────────┐
  │  available  │◄───────────────────│   blocked   │
  └─────────────┘                    └─────────────┘
                   NodesReleased event
                   (blocker resolved)
```

### Transitions

| From | To | Trigger | Ledger Event |
|------|----|---------|--------------|
| `available` | `claimed` | Agent claims the node | `nodes_claimed` |
| `claimed` | `available` | Agent releases the node | `nodes_released` |
| `claimed` | `blocked` | Node blocked due to dependency | (internal) |
| `blocked` | `available` | Blocker resolved | `nodes_released` |

### Invalid Transitions

The following transitions are explicitly disallowed:

- `available` -> `blocked`: A node must be claimed first before it can be blocked
- `blocked` -> `claimed`: A blocked node must become available first
- Same state -> Same state: No-op transitions are not allowed

### Validation

Use `schema.ValidateWorkflowTransition(from, to)` to check if a transition is valid.

---

## Epistemic States

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
                         ┌─────────────┐
                         │   pending   │
                         └─────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          │ NodeValidated      │ NodeAdmitted       │ NodeRefuted
          │ (verifier          │ (verifier admits   │ (verifier
          │  accepts)          │  without proof)    │  rejects)
          │                    │                    │
          ▼                    ▼                    ▼
    ┌───────────┐       ┌───────────┐       ┌───────────┐
    │ validated │       │ admitted  │       │  refuted  │
    │  (final)  │       │  (final)  │       │  (final)  │
    └───────────┘       └───────────┘       └───────────┘

                               │
                               │ NodeArchived
                               │ (proof path abandoned)
                               ▼
                        ┌───────────┐
                        │ archived  │
                        │  (final)  │
                        └───────────┘
```

### Transitions

| From | To | Trigger | Ledger Event |
|------|----|---------|--------------|
| `pending` | `validated` | Verifier accepts the node | `node_validated` |
| `pending` | `admitted` | Verifier admits without full proof | `node_admitted` |
| `pending` | `refuted` | Verifier rejects the node | `node_refuted` |
| `pending` | `archived` | Proof branch abandoned | `node_archived` |

### Terminal States

All states except `pending` are terminal (final). Once a node enters a terminal state, no further epistemic transitions are allowed.

**Key insight**: Only `admitted` introduces taint, which propagates to dependent nodes.

---

## Taint States

Taint states track epistemic uncertainty propagation through the proof tree. A node's taint indicates whether its validity depends on unverified assumptions.

Source: `internal/node/node.go`, `internal/taint/compute.go`

### States

| State | Description |
|-------|-------------|
| `clean` | Node and all ancestors are validated |
| `self_admitted` | This node was admitted (introduced taint) |
| `tainted` | Node inherits taint from an ancestor |
| `unresolved` | Node or an ancestor is still pending |

### Computation Rules

Taint is computed (not directly transitioned) based on the node's epistemic state and its ancestors' taint states. The rules are applied in order:

1. **Rule 1**: If the node's epistemic state is `pending`, taint = `unresolved`
2. **Rule 2**: If any ancestor has taint = `unresolved`, taint = `unresolved`
3. **Rule 3**: If the node's epistemic state is `admitted`, taint = `self_admitted`
4. **Rule 4**: If any ancestor has taint = `tainted` or `self_admitted`, taint = `tainted`
5. **Rule 5**: Otherwise, taint = `clean`

### Taint Propagation Diagram

```
                     ┌─────────────────────────────────────┐
                     │       Is node pending?              │
                     └─────────────────────────────────────┘
                                      │
                          ┌───────────┴───────────┐
                         Yes                      No
                          │                       │
                          ▼                       ▼
                    ┌───────────┐    ┌─────────────────────────┐
                    │unresolved │    │ Any ancestor unresolved?│
                    └───────────┘    └─────────────────────────┘
                                                  │
                                      ┌───────────┴───────────┐
                                     Yes                      No
                                      │                       │
                                      ▼                       ▼
                                ┌───────────┐    ┌─────────────────────────┐
                                │unresolved │    │ Is node admitted?       │
                                └───────────┘    └─────────────────────────┘
                                                              │
                                                  ┌───────────┴───────────┐
                                                 Yes                      No
                                                  │                       │
                                                  ▼                       ▼
                                            ┌─────────────┐  ┌───────────────────────────┐
                                            │self_admitted│  │ Ancestor tainted/admitted?│
                                            └─────────────┘  └───────────────────────────┘
                                                                          │
                                                              ┌───────────┴───────────┐
                                                             Yes                      No
                                                              │                       │
                                                              ▼                       ▼
                                                        ┌─────────┐            ┌───────────┐
                                                        │ tainted │            │   clean   │
                                                        └─────────┘            └───────────┘
```

### Taint Events

When taint changes due to epistemic state changes or propagation:

| Event | Description |
|-------|-------------|
| `taint_recomputed` | Node's taint state was recalculated |

### Taint Propagation

When a node's epistemic state changes, taint must be recomputed for:
1. The node itself
2. All descendant nodes

Use `taint.PropagateTaint(root, allNodes)` to propagate taint changes through the tree.

---

## Challenge States

Challenge states track the lifecycle of verifier challenges against proof nodes.

Source: `internal/node/challenge.go`

### States

| State | Description |
|-------|-------------|
| `open` | Challenge is active and awaiting response |
| `resolved` | Challenge was addressed by the prover |
| `withdrawn` | Verifier withdrew the challenge |
| `superseded` | Challenge became moot (parent node archived/refuted) |

### State Diagram

```
                         ┌─────────────┐
                         │    open     │
                         └─────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          │ ChallengeResolved  │ ChallengeWithdrawn │ ChallengeSuperseded
          │ (prover answers)   │ (verifier retracts)│ (parent archived/
          │                    │                    │  refuted)
          ▼                    ▼                    ▼
    ┌───────────┐       ┌───────────┐       ┌───────────┐
    │ resolved  │       │ withdrawn │       │superseded │
    │  (final)  │       │  (final)  │       │  (final)  │
    └───────────┘       └───────────┘       └───────────┘
```

### Transitions

| From | To | Trigger | Ledger Event |
|------|----|---------|--------------|
| `open` | `resolved` | Prover provides resolution | `challenge_resolved` |
| `open` | `withdrawn` | Verifier withdraws challenge | `challenge_withdrawn` |
| `open` | `superseded` | Parent node archived or refuted | `challenge_superseded` |

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

Source: `internal/schema/target.go`

---

## State Interactions

### Workflow + Epistemic

- A node in `claimed` workflow state can have its epistemic state changed by a verifier
- Epistemic state changes trigger taint recomputation
- When a node is `refuted` or `archived`, descendant nodes may also be archived

### Epistemic + Taint

- `pending` -> `validated`: taint may change from `unresolved` to `clean`
- `pending` -> `admitted`: taint changes to `self_admitted`
- `pending` -> `refuted`/`archived`: taint remains `unresolved` (doesn't matter for invalid nodes)
- Taint propagates to all descendants when any node's epistemic state changes

### Epistemic + Challenge

- When a node transitions to `refuted` or `archived`, all open challenges on that node become `superseded`
- Challenges can only be raised against nodes in `pending` epistemic state
- Resolving all challenges is typically required before a node can be `validated`

---

## API Reference

### Workflow State Validation

```go
import "github.com/tobias/vibefeld/internal/schema"

// Check if a state is valid
err := schema.ValidateWorkflowState("available")

// Check if a transition is valid
err := schema.ValidateWorkflowTransition(schema.WorkflowAvailable, schema.WorkflowClaimed)

// Check if a node can be claimed
canClaim := schema.CanClaim(schema.WorkflowAvailable) // true
```

### Epistemic State Validation

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

### Taint Computation

```go
import "github.com/tobias/vibefeld/internal/taint"

// Compute taint for a single node
taintState := taint.ComputeTaint(node, ancestors)

// Propagate taint to all descendants
changedNodes := taint.PropagateTaint(root, allNodes)

// Propagate and generate events
changedNodes, events := taint.PropagateAndGenerateEvents(root, allNodes)
```

### Challenge State Management

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
