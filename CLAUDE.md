# Vibefeld (AF) - Claude Code Project Guide

## Project Overview

AF (Adversarial Proof Framework) is a command-line tool for collaborative construction of natural-language mathematical proofs. Multiple AI agents work concurrently as adversarial provers and verifiers, refining proof steps until rigorous acceptance.

**Language**: Go 1.22+
**CLI Framework**: Cobra
**Estimated Size**: ~3000-3500 LOC

## Project Laws

### Core Principles (Non-Negotiable)

1. **Adversarial Verification**: Provers convince, verifiers attack. No agent plays both roles.
2. **Agent Isolation**: Each agent spawns fresh, claims one job, works, terminates. No context bleeding.
3. **Append-Only Truth**: Ledger is source of truth. Current state is derived. Full history preserved including rejected paths.
4. **Filesystem Concurrency**: ACID guarantees via POSIX atomics. No database server.
5. **Tool-Controlled Structure**: Hierarchical IDs, child assignment, and state transitions are enforced by the tool, never by agents.
6. **Verifier Control**: Verifiers explicitly control all acceptance decisions. No automatic state transitions.
7. **Serialized Writes**: Ledger writes are serialized. No gaps in sequence numbers.
8. **Taint Propagation**: Epistemic uncertainty propagates through dependencies.
9. **Self-Documenting CLI**: The tool provides complete context; agents need no external documentation.

### Development Principles

1. **TDD (Tests First)**: Write tests before implementation. No exceptions.
2. **Small Modules**: Target 200-300 LOC per module.
3. **Tracer Bullet First**: Get minimal working CLI (`init`, `status`, `claim`, `refine`, `release`, `accept`) before expanding.

### CLI Self-Documentation Requirements

The `af` CLI must be fully self-documenting:
- Zero external documentation required for agents
- Forgiving input (fuzzy matching for commands/flags)
- Guided workflow (every command output suggests next steps)
- Role-specific context on claim

## Directory Structure

```
cmd/af/           # CLI entry point and commands
internal/
  errors/         # Error types and codes
  types/          # Core types (NodeID, timestamps)
  hash/           # Content hash computation
  schema/         # Inference types, node types, states
  fuzzy/          # Levenshtein distance, fuzzy matching
  config/         # Configuration loading
  node/           # Node, Challenge, Definition structs
  ledger/         # Event sourcing (append, read, replay)
  lock/           # File-based lock manager
  state/          # Derived state from event replay
  scope/          # Scope tracking for local assumptions
  taint/          # Taint propagation
  jobs/           # Job detection (prover/verifier)
  render/         # Human-readable and JSON output
  fs/             # Filesystem operations
  service/        # Proof service facade
  cli/            # CLI utilities (arg parsing, prompts)
  lemma/          # Lemma extraction validation
  testutil/       # Test helpers
e2e/              # End-to-end tests
```

## Key Data Model

### Node States
- **Workflow**: `available`, `claimed`, `blocked`
- **Epistemic**: `pending`, `validated`, `admitted`, `refuted`, `archived`
- **Taint**: `clean`, `self_admitted`, `tainted`, `unresolved`

### Hierarchical IDs
- Root: `1`
- Children: `1.1`, `1.2`, etc.
- Grandchildren: `1.1.1`, `1.1.2`, etc.

### Event Sourcing
All state changes go through the ledger:
- `ProofInitialized`, `NodeCreated`, `NodesClaimed`, `NodesReleased`
- `ChallengeRaised`, `ChallengeResolved`, `ChallengeWithdrawn`
- `NodeValidated`, `NodeAdmitted`, `NodeRefuted`, `NodeArchived`
- `TaintRecomputed`, `DefAdded`, `LemmaExtracted`, etc.

## Error Codes

Exit codes:
- 1 = retriable
- 2 = blocked (pending definition)
- 3 = logic error
- 4 = corruption

## Beads Issue Tracker

This project uses beads (`bd`) for issue tracking.

### Quick Reference
```bash
bd list                    # List all open issues
bd list --status open      # Filter by status
bd ready                   # Show ready-to-work tasks
bd show <issue-id>         # Show issue details
bd update <id> --status in_progress  # Start work
bd close <id>              # Close completed issue
bd dep add <id> --needs <dep-id>     # Add dependency
bd blocked                 # Show blocked issues
```

### Issue Naming
Issues use prefix `vibefeld-` followed by hash (e.g., `vibefeld-a3f2dd`).

### Workflow
1. `bd ready` to find unblocked tasks
2. `bd update <id> --status in_progress` to claim
3. Implement with TDD (tests first!)
4. `bd close <id>` when done

## Implementation Phases

See `docs/vibefeld-implementation-plan.md` for full details.

**Critical Path to Tracer Bullet**:
1. Phase 0: Project Bootstrap (steps 1-7)
2. Phase 1: Core Types and Error Handling (steps 8-15)
3. Phase 2-5: Schema, Fuzzy, Config, Node Model
4. Phase 6-8: Ledger, Locks, State/Replay
5. Phase 9-14: Scope, Taint, Jobs, Validation, Rendering, Filesystem
6. Phase 15-16: Service Layer and CLI Tracer Bullet Commands

**Parallelizable after tracer bullet**: All remaining CLI commands (Phase 17-22).

## Adversarial Workflow Fix (PRIORITY)

The core adversarial workflow has critical issues identified in Session 53. See:
- `docs/FAILURE_REPORT_SESSION53.md` - What went wrong
- `docs/ADVERSARIAL_WORKFLOW_FIX_PLAN.md` - Root cause analysis
- `docs/ADVERSARIAL_WORKFLOW_IMPLEMENTATION_PLAN.md` - 22-step fix plan

### Key Issues Found
1. **Acceptance doesn't check blocking challenges** - `SeverityBlocksAcceptance()` exists but is never called
2. **No verification checklist** - Verifiers have no guidance on what to check
3. **Tool guides toward depth** - "Next steps" encourages child refinement over breadth
4. **Job detection is CORRECT** - The bug is NOT in `internal/jobs/`

### Critical Path (P0)
Start with these two issues (no dependencies):
1. `vibefeld-eo90` - Add `GetBlockingChallengesForNode` to State
2. `vibefeld-45nt` - Create `RenderVerificationChecklist` function

### Quick Reference
```bash
bd ready                    # See 10 unblocked issues
bd blocked                  # See 12 blocked issues with dependencies
bd show vibefeld-eo90       # View issue details
```

## Testing

- Test files: `*_test.go` alongside implementation
- Use table-driven tests
- E2E tests in `e2e/` directory
- Run: `go test ./...`

## Build

```bash
go build ./cmd/af
./af --version
```

## Landing the Plane

When completing a work session or reaching a milestone, follow this **mandatory workflow**:

### 1. Update Issues
```bash
bd close <id>                           # Close completed work
bd update <id> --status in_progress     # Mark WIP items
bd q "Remaining task description"       # File issues for follow-up work
```

### 2. Run Quality Gates (if code changed)
```bash
go test ./...
go build ./cmd/af
```

### 3. Write `handoff.md`
Create/update `handoff.md` with:
- **What was accomplished**: Completed issues/tasks with brief descriptions
- **Current state**: What's working, partially done, or broken
- **Next steps**: Prioritized list of what to do next
- **Blockers/Decisions needed**: Issues requiring human input
- **Key files changed**: Summary of modified/created files
- **Testing status**: What tests pass, what's untested
- **Known issues**: Bugs, edge cases, or technical debt

### 4. Sync and Push (MANDATORY)
```bash
git add -A
git commit -m "Description of changes"
git pull --rebase
bd sync
git push
git status   # MUST show "up to date with origin"
```

**CRITICAL**: Work is NOT complete until `git push` succeeds. Never stop before pushing.
