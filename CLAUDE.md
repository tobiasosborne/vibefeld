# Vibefeld (AF) - Claude Code Project Guide

## Project Overview

AF (Adversarial Proof Framework) is a CLI tool for collaborative construction of natural-language mathematical proofs. Multiple AI agents work as adversarial provers and verifiers, refining proof steps until rigorous acceptance.

**Language**: Go 1.25.5+
**CLI Framework**: Cobra
**Status**: Feature-complete, in hardening phase

## Quick Commands

```bash
# Build
go build ./cmd/af

# Test
go test ./...

# Run
./af [command]
./af --help
```

## Core Principles (The 9 Laws)

1. **Adversarial Verification**: Provers convince, verifiers attack. No agent plays both roles.
2. **Agent Isolation**: Each agent spawns fresh, claims one job, works, terminates. No context bleeding.
3. **Append-Only Truth**: Ledger is source of truth. Current state is derived. Full history preserved.
4. **Filesystem Concurrency**: ACID guarantees via POSIX atomics. No database server.
5. **Tool-Controlled Structure**: Hierarchical IDs and state transitions enforced by tool, not agents.
6. **Verifier Control**: Verifiers explicitly control all acceptance decisions.
7. **Serialized Writes**: Ledger writes serialized. No gaps in sequence numbers.
8. **Taint Propagation**: Epistemic uncertainty propagates through dependencies.
9. **Self-Documenting CLI**: The tool provides complete context; agents need no external documentation.

## Directory Structure

```
cmd/af/           # CLI commands (60+ commands)
internal/
  cli/            # CLI utilities (arg parsing, prompts)
  config/         # Configuration loading
  cycle/          # Cycle detection in dependencies
  errors/         # Error types and exit codes
  export/         # Export to LaTeX, Markdown
  fs/             # Filesystem operations (atomic writes)
  fuzzy/          # Levenshtein distance, fuzzy matching
  hash/           # Content hash computation (SHA256)
  hooks/          # External integration hooks
  jobs/           # Job detection (prover/verifier)
  ledger/         # Event sourcing (append, read, replay)
  lemma/          # Lemma extraction validation
  lock/           # File-based lock manager
  metrics/        # Proof quality metrics
  node/           # Node, Challenge, Definition structs
  patterns/       # Challenge pattern library
  render/         # Human-readable and JSON output
  schema/         # Inference types, node types, states
  scope/          # Scope tracking for local assumptions
  service/        # Proof service facade
  shell/          # Interactive shell
  state/          # Derived state from event replay
  strategy/       # Proof strategy guidance
  taint/          # Taint propagation
  templates/      # Output templates
  types/          # Core types (NodeID, timestamps)
e2e/              # End-to-end tests
examples/         # Example proofs (sqrt2, Dobinski)
docs/             # Documentation (PRD, archived plans)
```

## Key Data Model

### Node States

| Category | States |
|----------|--------|
| **Workflow** | `available`, `claimed`, `blocked` |
| **Epistemic** | `pending`, `validated`, `admitted`, `refuted`, `archived` |
| **Taint** | `clean`, `self_admitted`, `tainted`, `unresolved` |

### Hierarchical IDs

- Root: `1`
- Children: `1.1`, `1.2`, etc.
- Grandchildren: `1.1.1`, `1.1.2`, etc.

### Event Types

`ProofInitialized`, `NodeCreated`, `NodesClaimed`, `NodesReleased`, `ChallengeRaised`, `ChallengeResolved`, `ChallengeWithdrawn`, `NodeValidated`, `NodeAdmitted`, `NodeRefuted`, `NodeArchived`, `TaintRecomputed`, `DefAdded`, `LemmaExtracted`

## Error Codes

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | Retriable (lock conflict, etc.) |
| 2 | Blocked (pending definition) |
| 3 | Logic error (invalid input) |
| 4 | Corruption (ledger inconsistent) |

## CLI Commands

### Proof Management
| Command | Description |
|---------|-------------|
| `init` | Initialize a new proof workspace |
| `status` | Show proof tree with states and taint |
| `progress` | Show completion metrics |
| `health` | Detect stuck states |

### Job Discovery
| Command | Description |
|---------|-------------|
| `jobs` | List available prover/verifier jobs |
| `pending-defs` | List unresolved definition requests |
| `pending-refs` | List unverified external references |

### Agent Operations
| Command | Description |
|---------|-------------|
| `claim` | Claim a node for work |
| `release` | Release a claimed node |
| `extend-claim` | Extend claim duration |

### Prover Commands
| Command | Description |
|---------|-------------|
| `refine` | Add child node(s) to develop proof |
| `request-def` | Request a new definition |
| `amend` | Correct a node statement |

### Verifier Commands
| Command | Description |
|---------|-------------|
| `challenge` | Raise objection to a node |
| `resolve-challenge` | Mark challenge as resolved |
| `withdraw-challenge` | Retract a challenge |
| `accept` | Validate a node |

### Escape Hatches
| Command | Description |
|---------|-------------|
| `admit` | Accept without full proof (introduces taint) |
| `refute` | Mark node as disproven |
| `archive` | Abandon proof branch |

### Reference Data
| Command | Description |
|---------|-------------|
| `get` | Retrieve node with context |
| `defs`, `def` | List/show definitions |
| `assumptions`, `assumption` | List/show assumptions |
| `schema`, `inferences`, `types` | Show valid inference rules and types |
| `challenges` | List challenges across proof |
| `deps` | Show dependency graph |
| `scope` | Show scope information |

### Administration
| Command | Description |
|---------|-------------|
| `log` | Show event ledger |
| `replay` | Rebuild state from ledger |
| `reap` | Clear stale locks |
| `recompute-taint` | Force taint recalculation |
| `def-add`, `def-reject` | Human operator actions |
| `extract-lemma` | Extract reusable subproof |
| `export` | Export to LaTeX/Markdown |

## Development Workflow

### TDD Always

1. Write test first
2. Run test (should fail)
3. Implement minimal code
4. Run test (should pass)
5. Refactor

### Small Modules

Target 200-300 LOC per module. Current packages average ~400 LOC.

### Beads Issue Tracker

```bash
bd list                    # List all open issues
bd ready                   # Show unblocked tasks
bd show <id>               # View issue details
bd update <id> --status in_progress  # Start work
bd close <id>              # Complete issue
bd q "Task description"    # Quick-create issue
```

Issue naming: `vibefeld-<hash>` (e.g., `vibefeld-a3f2dd`)

### Current Priorities

1. **P0 Critical**: Lock error handling, test coverage for service/taint packages
2. **P1 High**: Performance optimizations (challenge caching), TOCTOU fixes
3. **P2 Medium**: CLI UX improvements, edge case tests

## Documentation

| Document | Location |
|----------|----------|
| PRD (full spec) | `docs/prd.md` |
| Example proofs | `examples/sqrt2-proof/`, `examples/dobinski-proof/` |
| State machines | `docs/archive/state-machines.md` |
| Challenge workflow | `docs/archive/challenge-workflow.md` |
| Lock protocol | `docs/archive/lock-protocol.md` |

## Session Close Protocol

When completing a work session, follow this **mandatory workflow**:

### 1. Update Issues

```bash
bd close <id>                           # Close completed work
bd update <id> --status in_progress     # Mark WIP items
bd q "Remaining task description"       # File issues for follow-up
```

### 2. Run Quality Gates

```bash
go test ./...
go build ./cmd/af
```

### 3. Update `handoff.md`

Include:
- What was accomplished
- Current state (what works, what's broken)
- Next steps (prioritized)
- Key files changed
- Testing status
- Known issues

### 4. Sync and Push (MANDATORY)

```bash
git add -A
git commit -m "Description of changes"
git pull --rebase
bd sync
git push
git status   # MUST show "up to date with origin"
```

**CRITICAL**: Work is NOT complete until `git push` succeeds.
