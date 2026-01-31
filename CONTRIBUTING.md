# Contributing to Vibefeld

Welcome to Vibefeld! We appreciate your interest in contributing to the adversarial proof verification framework. This guide will help you get started.

## Development Environment Setup

### Prerequisites

- Go 1.25.5 or later
- Git
- [Beads](https://github.com/beads-project/beads) (`bd`) for issue tracking (optional but recommended)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/tobias/vibefeld.git
cd vibefeld

# Verify your Go version
go version  # Should be 1.25.5+

# Run tests to ensure everything works
go test ./...

# Build the CLI
go build ./cmd/af
./af --version
```

## Running Tests

Tests are the backbone of this project. Run them frequently.

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/ledger/...

# Run a specific test
go test -v ./internal/errors -run TestErrorCodesExist

# Run end-to-end tests
go test ./e2e/...
```

## Building

```bash
# Build the CLI binary
go build ./cmd/af

# Run the built binary
./af --help
./af --version
```

## Code Style and Conventions

### TDD (Test-Driven Development)

This project strictly follows TDD. **Write tests before implementation. No exceptions.**

1. Write a failing test that describes the expected behavior
2. Write the minimum code to make the test pass
3. Refactor while keeping tests green

### Table-Driven Tests

Use table-driven tests for comprehensive coverage. Example pattern from our codebase:

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "expected",
            wantErr: false,
        },
        {
            name:    "empty input",
            input:   "",
            want:    "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Something() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Something() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Module Size

Target 200-300 lines of code per module. If a module grows larger, consider splitting it.

### File Organization

- Test files: `*_test.go` alongside the implementation they test
- Package per directory in `internal/`
- E2E tests go in `e2e/`

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

## Issue Tracking with Beads

This project uses [Beads](https://github.com/beads-project/beads) (`bd`) for issue tracking. Issues are stored in the repository itself.

### Finding Work

```bash
# List all open issues
bd list

# Show issues ready to work on (no blockers)
bd ready

# Show blocked issues
bd blocked

# View details of a specific issue
bd show vibefeld-a3f2dd
```

### Working on an Issue

```bash
# 1. Find a task
bd ready

# 2. Claim it by marking as in progress
bd update vibefeld-a3f2dd --status in_progress

# 3. Do the work (TDD - tests first!)
# ... write tests, implement, verify ...

# 4. Close when done
bd close vibefeld-a3f2dd
```

### Creating Issues

```bash
# Quick issue creation
bd q "Description of the task"

# Add dependencies between issues
bd dep add vibefeld-abc123 --needs vibefeld-def456
```

### Issue Naming

Issues use the prefix `vibefeld-` followed by a hash (e.g., `vibefeld-a3f2dd`).

## Pull Request Process

### Before Submitting

1. **Run all tests**: `go test ./...`
2. **Build successfully**: `go build ./cmd/af`
3. **Follow TDD**: New code must have tests written first
4. **Keep commits focused**: One logical change per commit

### Submitting a PR

1. Create a feature branch from `main`
2. Make your changes following TDD
3. Ensure all tests pass
4. Push your branch and open a PR
5. Describe what changed and why
6. Link any related issues

### PR Description Template

```
## Summary
Brief description of what this PR does.

## Changes
- List of specific changes made

## Testing
- How were these changes tested?
- Any new tests added?

## Related Issues
Closes vibefeld-xxxxxx
```

## Quality Gates

Before any PR can be merged, ensure:

```bash
# All tests pass
go test ./...

# Build succeeds
go build ./cmd/af
```

## Core Principles

When contributing, keep these project principles in mind:

1. **Adversarial Verification**: Provers convince, verifiers attack
2. **Agent Isolation**: Each agent spawns fresh, claims one job, works, terminates
3. **Append-Only Truth**: Ledger is source of truth; full history preserved
4. **Self-Documenting CLI**: The tool provides complete context via `--help`

## Getting Help

- Read `CLAUDE.md` for detailed project conventions
- Check existing tests for usage examples
- Use `af --help` and `af <command> --help` for CLI documentation
- Look at `docs/` for implementation plans and design documents

## License

By contributing to Vibefeld, you agree that your contributions will be licensed under the MIT License.
