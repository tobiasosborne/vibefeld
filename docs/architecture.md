# AF (Adversarial Proof Framework) Architecture

## Overview

AF is a command-line tool for collaborative construction of natural-language mathematical proofs. It implements an event-sourced adversarial workflow engine where multiple AI agents work concurrently as provers and verifiers, refining proof steps until rigorous acceptance.

**Key characteristics:**
- **Event-sourced architecture**: All state changes go through an append-only ledger
- **Adversarial verification**: Distinct prover and verifier roles with opposing objectives
- **Filesystem concurrency**: ACID guarantees via POSIX atomics, no database server required
- **Hierarchical proof structure**: Lamport-style structured proofs with hierarchical node IDs

**What AF is**: A socio-technical protocol for producing machine-auditable proof trees with full history preservation.

**What AF is not**: A proof assistant, logic kernel, or theorem prover. It provides procedural rigor, not semantic soundness.

---

## Design Philosophy: The 9 Core Principles

### 1. Adversarial Verification
Provers convince, verifiers attack. No agent plays both roles. This separation ensures that acceptance decisions are explicit and scrutinized.

### 2. Agent Isolation
Each agent spawns fresh, claims one job, works, terminates. No context bleeding between invocations. This prevents state corruption and ensures reproducibility.

### 3. Append-Only Truth
The ledger is the source of truth. Current state is derived by replaying events. Full history is preserved including rejected paths, making the proof a scientific record.

### 4. Filesystem Concurrency
ACID guarantees via POSIX atomics (O_CREAT | O_EXCL, rename()). No database server required. Lock files with agent ID and timestamp enable concurrent operation.

### 5. Tool-Controlled Structure
Hierarchical IDs, child assignment, and state transitions are enforced by the tool, never by agents. This prevents structural corruption.

### 6. Verifier Control
Verifiers explicitly control all acceptance decisions. No automatic state transitions. Every validation requires explicit verifier action.

### 7. Serialized Writes
Ledger writes are serialized via file-based locking. No gaps in sequence numbers. This ensures total ordering of all events.

### 8. Taint Propagation
Epistemic uncertainty propagates through dependencies. If a node is admitted without proof, all dependent nodes are marked as tainted.

### 9. Self-Documenting CLI
The tool provides complete context; agents need no external documentation. Every command output suggests next steps.

---

## Module Diagram

```
                                 +-------------------+
                                 |     cmd/af/       |
                                 |  (CLI Commands)   |
                                 +--------+----------+
                                          |
                                          v
+-----------------------------------------------------------------------------+
|                              internal/service/                               |
|                         (Proof Service Facade)                               |
|  Coordinates operations across ledger, state, locks, and filesystem          |
+--------+----------+----------+----------+----------+----------+-------------+
         |          |          |          |          |          |
         v          v          v          v          v          v
+--------+--+ +-----+----+ +---+----+ +---+----+ +---+---+ +----+----+
|  ledger/  | |  state/  | | lock/  | |  node/ | | taint/| |  jobs/  |
| (Events)  | | (Replay) | |(Claims)| |(Model) | |(Prop) | |(Detect) |
+-----------+ +----------+ +--------+ +--------+ +-------+ +---------+
         |          |          |          |
         v          v          v          v
+--------+--+ +-----+----+ +---+----+ +---+--------+
|   types/  | | schema/  | |  hash/ | |  scope/    |
|(NodeID,TS)| | (States) | |(SHA256)| |(Tracking)  |
+-----------+ +----------+ +--------+ +------------+
                  |
                  v
+-----------------------------------------------------------------------------+
|                                internal/fs/                                  |
|                    (Filesystem Operations - Nodes, Defs, etc.)               |
+-----------------------------------------------------------------------------+
                                          |
                                          v
                              +------------------------+
                              |     proof/ directory   |
                              |  (ledger/, nodes/,     |
                              |   locks/, defs/, ...)  |
                              +------------------------+
```

### Module Responsibilities

| Module | Responsibility |
|--------|----------------|
| `cmd/af/` | CLI entry point and Cobra command definitions |
| `service/` | High-level proof operations facade; coordinates all subsystems |
| `ledger/` | Event sourcing: append, read, scan, batch operations with file locking |
| `state/` | Derived state from event replay; maintains node map, challenges, definitions |
| `lock/` | Node-level claim locks with timeout and refresh support |
| `node/` | Node and Challenge data structures, validation, content hashing |
| `taint/` | Taint computation and propagation logic |
| `jobs/` | Job detection for prover and verifier agents |
| `schema/` | Type definitions: NodeType, InferenceType, WorkflowState, EpistemicState |
| `types/` | Core types: NodeID (hierarchical), Timestamp (RFC3339) |
| `hash/` | SHA256 content hashing for integrity verification |
| `scope/` | Local assumption scope tracking and inheritance |
| `fs/` | Filesystem I/O for nodes, definitions, assumptions, lemmas, externals |
| `fuzzy/` | Levenshtein distance for fuzzy command/flag matching |
| `config/` | Configuration loading from meta.json |
| `render/` | Human-readable and JSON output formatting |
| `errors/` | Structured error types with exit codes |

---

## Data Flow

### Write Path (Creating a Node)

```
1. CLI Command (af refine)
         |
         v
2. ProofService.RefineNode()
         |
         +--> Load current state via Replay()
         |
         +--> Validate: parent claimed by owner, depth limits, child count
         |
         +--> Create Node with content hash
         |
         v
3. Ledger.AppendIfSequence()
         |
         +--> Acquire ledger.lock
         |
         +--> Verify expected sequence (CAS semantics)
         |
         +--> Write event to temp file
         |
         +--> fsync() + rename() (atomic)
         |
         +--> Release lock
         |
         v
4. Event persisted, sequence number returned
```

### Read Path (Replaying State)

```
1. ProofService.LoadState()
         |
         v
2. Ledger.Scan() / ReadAll()
         |
         +--> List ledger files (sorted by sequence)
         |
         +--> Validate: no gaps, no duplicates, starts at 1
         |
         v
3. For each event:
         |
         +--> Parse event JSON
         |
         +--> state.Apply(event)
         |
         v
4. Apply() dispatches by event type:
         |
         +-- ProofInitialized -> (setup)
         +-- NodeCreated      -> AddNode()
         +-- NodesClaimed     -> SetWorkflowClaimed()
         +-- NodesReleased    -> SetWorkflowAvailable()
         +-- NodeValidated    -> SetEpistemicValidated() + ComputeTaint()
         +-- NodeAdmitted     -> SetEpistemicAdmitted() + ComputeTaint()
         +-- ChallengeRaised  -> AddChallenge()
         +-- ChallengeResolved-> SetChallengeResolved()
         +-- DefAdded         -> AddDefinition()
         +-- TaintRecomputed  -> SetTaintState()
         +-- ...
         |
         v
5. State object with all nodes, challenges, definitions ready for use
```

---

## Key Abstractions

### Node

A proof step in the hierarchical structure.

```go
type Node struct {
    ID             types.NodeID      // Hierarchical: "1", "1.1", "1.2.3"
    Type           schema.NodeType   // claim, local_assume, local_discharge, case, qed
    Statement      string            // Mathematical assertion
    Latex          string            // LaTeX rendering (optional)
    Inference      schema.InferenceType // modus_ponens, by_definition, etc.
    Context        []string          // Referenced definitions/assumptions
    Dependencies   []types.NodeID    // Referenced node IDs
    ValidationDeps []types.NodeID    // Cross-branch validation dependencies
    Scope          []string          // Active scope entries (e.g., "1.1.A")

    WorkflowState  schema.WorkflowState   // available, claimed, blocked
    EpistemicState schema.EpistemicState  // pending, validated, needs_refinement, admitted, refuted, archived
    TaintState     TaintState             // clean, self_admitted, tainted, unresolved

    ContentHash    string            // SHA256 for integrity
    Created        types.Timestamp
    ClaimedBy      string            // Agent ID if claimed
    ClaimedAt      types.Timestamp
}
```

### Challenge

A verifier's objection to a proof step.

```go
type Challenge struct {
    ID         string                 // Unique identifier
    TargetID   types.NodeID           // Node being challenged
    Target     schema.ChallengeTarget // statement, inference, gap, scope, etc.
    Reason     string                 // Specific objection
    Severity   string                 // critical, major, minor, note
    Status     ChallengeStatus        // open, resolved, withdrawn, superseded
    Raised     types.Timestamp
    ResolvedAt types.Timestamp
    Resolution string                 // How it was resolved
}
```

### Ledger Event

Immutable record of state change.

```go
type Event interface {
    Type() EventType
    Time() types.Timestamp
}

// Event types:
// ProofInitialized, NodeCreated, NodesClaimed, NodesReleased,
// ChallengeRaised, ChallengeResolved, ChallengeWithdrawn, ChallengeSuperseded,
// NodeValidated, NodeAdmitted, NodeRefuted, NodeArchived, NodeAmended,
// TaintRecomputed, DefAdded, LemmaExtracted, ClaimRefreshed, LockReaped,
// ScopeOpened, ScopeClosed, RefinementRequested
```

### State

Derived from replaying ledger events.

```go
type State struct {
    nodes       map[string]*node.Node
    challenges  map[string]*Challenge
    definitions map[string]*Definition
    lemmas      map[string]*Lemma
    latestSeq   int  // Last processed sequence number
}
```

### Job

Work item for an agent.

```go
type JobResult struct {
    ProverJobs   []*node.Node  // Nodes with open challenges to address
    VerifierJobs []*node.Node  // Nodes ready for verification
}
```

---

## Concurrency Model

### File-Based Locking Strategy

AF uses two types of locks:

1. **Ledger Lock** (`ledger.lock`): Serializes all ledger writes
2. **Node Claim Locks** (`locks/<node-id>.lock`): Tracks agent ownership

### Ledger Write Serialization

```
Append(event):
    1. Acquire ledger.lock (O_CREAT | O_EXCL with spin/backoff)
    2. Read current max sequence
    3. Assign next sequence number
    4. Write event to temp file
    5. fsync() to ensure durability
    6. rename() temp -> final (atomic)
    7. Release ledger.lock
```

### Optimistic Concurrency Control

The `AppendIfSequence` operation implements CAS (Compare-And-Swap) semantics:

```go
func AppendIfSequence(event Event, expectedSeq int) (int, error) {
    // Under lock:
    actualSeq := currentSequence()
    if actualSeq != expectedSeq {
        return 0, ErrSequenceMismatch  // Concurrent modification detected
    }
    // Proceed with append
}
```

This prevents lost updates when multiple agents modify the proof concurrently.

### Node Claim Flow

```
Agent A claims node 1.2:
    1. Load state (records latestSeq)
    2. Verify node is available
    3. AppendIfSequence(NodesClaimed, latestSeq)
    4. If ErrSequenceMismatch: reload state, retry
    5. On success: agent has exclusive access until release
```

### Stale Lock Handling

Claim locks include timestamp and can be reaped:

```bash
af reap --older-than 5m  # Clear locks older than 5 minutes
```

---

## Event Sourcing

### The Ledger as Source of Truth

All state is derived from the event ledger. The ledger directory contains:

```
proof/ledger/
    000001.json  # ProofInitialized
    000002.json  # NodeCreated
    000003.json  # NodesClaimed
    ...
```

### Event Structure

```json
{
    "type": "NodeCreated",
    "timestamp": "2025-01-11T10:05:00Z",
    "node": {
        "id": "1.2.1",
        "type": "claim",
        "statement": "Since p is prime...",
        "inference": "by_definition",
        ...
    }
}
```

### Replay for State Recovery

```go
func Replay(ledger *Ledger) (*State, error) {
    state := NewState()

    err := ledger.Scan(func(seq int, data []byte) error {
        event, err := ParseEvent(data)
        if err != nil { return err }

        // Validate sequence continuity
        if seq != state.latestSeq + 1 {
            return fmt.Errorf("gap detected: expected %d, got %d",
                state.latestSeq+1, seq)
        }

        return Apply(state, event)
    })

    return state, err
}
```

### Hash Verification

Nodes include content hashes computed as:

```
SHA256(type + statement + latex + inference + sorted(context) + sorted(dependencies))
```

`ReplayWithVerify` validates hashes during replay to detect corruption.

---

## State Derivation

### State Machine: Workflow States

```
                    +------------------+
                    |    available     |
                    +--------+---------+
                             |
                        claim by agent
                             |
                             v
                    +--------+---------+
            +-------|     claimed      |-------+
            |       +--------+---------+       |
            |                |                 |
      blocked by       release by        blocked by
        dep               agent            dep req
            |                |                 |
            v                v                 v
    +-------+--------+  (back to       +------+------+
    |    blocked     |   available)    |   blocked   |
    +----------------+                 +-------------+
            |
    blocker resolved
            |
            v
    (back to available)
```

### State Machine: Epistemic States

```
                    +------------------+
                    |     pending      |
                    +--------+---------+
                             |
         +---------+---------+---------+---------+
         |         |         |         |         |
    validated   admitted   refuted  archived   (stay pending)
         |         |         |         |
         v         v         v         v
    +----+---+ +---+----+ +--+-----+ +-+-------+
    |validated| |admitted| |refuted| |archived |
    |(FINAL)  | |(FINAL) | |(FINAL)| |(FINAL)  |
    +---------+ +--------+ +-------+ +---------+
```

### Taint Computation

Taint propagates epistemic uncertainty through the proof tree:

```go
func ComputeTaint(n *Node, ancestors []*Node) TaintState {
    // Rule 1: Pending nodes are unresolved
    if n.EpistemicState == pending { return unresolved }

    // Rule 2: Unresolved ancestors propagate
    for _, a := range ancestors {
        if a.TaintState == unresolved { return unresolved }
    }

    // Rule 3: Admitted nodes are self_admitted
    if n.EpistemicState == admitted { return self_admitted }

    // Rule 4: Tainted ancestors propagate
    for _, a := range ancestors {
        if a.TaintState == tainted || a.TaintState == self_admitted {
            return tainted
        }
    }

    // Rule 5: Clean
    return clean
}
```

Taint is recomputed whenever a node's epistemic state changes, and propagates to all descendants.

---

## Operational Roles

### Orchestrator (Automated)

Polls for jobs, spawns agents, runs maintenance:

```bash
while true; do
    # Spawn provers for open challenges
    for job in $(af jobs --role prover); do
        spawn_prover_agent "$job" &
    done

    # Spawn verifiers for nodes ready to accept
    for job in $(af jobs --role verifier); do
        spawn_verifier_agent "$job" &
    done

    # Maintenance
    af reap --older-than 5m

    # Check completion
    if [[ $(af status --root-state) != "pending" ]]; then
        echo "Proof complete"
        exit 0
    fi
done
```

### Prover Agent

Addresses challenges, adds child nodes:

```bash
af claim "$NODE_ID" --role prover --agent "$AGENT_ID"
# CLI provides full context: challenges, definitions, scope
# Agent produces children nodes addressing challenges
af refine "$NODE_ID" --children children.json --agent "$AGENT_ID"
```

### Verifier Agent

Reviews nodes, raises challenges or accepts:

```bash
af claim "$NODE_ID" --role verifier --agent "$AGENT_ID"
# CLI provides verification checklist
# Agent either challenges or accepts
af accept "$NODE_ID" --agent "$AGENT_ID"
# OR
af challenge "$NODE_ID" --objection "..." --targets inference,gap
```

### Human Supervisor

Handles escape hatches and definition requests:

```bash
af status                           # Monitor progress
af pending-defs                     # Handle definition requests
af def-add "prime" --latex "..."    # Add definitions
af admit "1.2.1" --reason "..."     # Escape hatch: accept without proof
af archive "1.3" --reason "..."     # Abandon unproductive branch
```

---

## Filesystem Structure

```
proof/
    meta.json                        # Proof metadata and config

    ledger/
        000001.json                  # ProofInitialized event
        000002.json                  # NodeCreated event
        ...

    ledger.lock                      # Write serialization lock

    nodes/
        1.json                       # Root node (derived, optional cache)
        1.1.json
        1.2.json
        ...

    locks/
        1.1.lock                     # Node claim lock (agent ID, timestamp)
        ...

    defs/
        DEF-prime.json               # Definitions
        ...

    assumptions/
        ASM-p-greater-2.json         # Global hypotheses
        ...

    lemmas/
        LEM-001.json                 # Extracted lemmas
        ...

    external/
        EXT-001.json                 # External references (DOIs)
        ...

    pending-defs/
        REQ-001.json                 # Pending definition requests
        ...

    schema.json                      # Allowed inference types
```

---

## Error Handling

### Exit Codes

| Code | Meaning | Example |
|------|---------|---------|
| 0 | Success | Operation completed |
| 1 | Retriable | Lock conflict, concurrent modification |
| 2 | Blocked | Pending definition request |
| 3 | Logic error | Invalid node type, missing dependency |
| 4 | Corruption | Hash mismatch, ledger inconsistency |

### Common Errors

| Error | Cause | Recovery |
|-------|-------|----------|
| `ErrConcurrentModification` | Ledger modified during operation | Reload state, retry |
| `ErrSequenceMismatch` | CAS check failed | Retry with fresh state |
| `ErrBlockingChallenges` | Node has unresolved critical challenges | Resolve challenges first |
| `ALREADY_CLAIMED` | Another agent holds the lock | Wait or reap stale locks |
| `VALIDATION_INVARIANT_FAILED` | Accept preconditions not met | Resolve challenges, validate children |

---

## Design Rationale

### Why Event Sourcing?

1. **Full history**: Every change is recorded, enabling audit trails and debugging
2. **Crash recovery**: State can always be rebuilt from events
3. **Concurrent-safe**: Single-writer semantics via ledger lock
4. **Testability**: State at any point can be recreated by replaying events

### Why File-Based Concurrency?

1. **No external dependencies**: Works anywhere POSIX is available
2. **Simple operations model**: Standard file operations (create, rename)
3. **Natural durability**: fsync() for crash safety
4. **Easy debugging**: Events and state are human-readable JSON

### Why Adversarial Roles?

1. **Separation of concerns**: Provers build, verifiers attack
2. **Explicit acceptance**: Every validation is a conscious decision
3. **Challenge tracking**: Disagreements are first-class objects
4. **Scientific record**: Both successful and failed paths preserved

---

## Related Documentation

- `docs/prd.md` - Product Requirements Document
- `docs/vibefeld-implementation-plan.md` - Implementation phases
- `docs/ADVERSARIAL_WORKFLOW_FIX_PLAN.md` - Workflow improvements
- `CLAUDE.md` - Development guidelines and issue tracker usage
