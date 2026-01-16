# Contributing to Vibefeld (AF)

This guide covers everything you need to know to contribute to the Adversarial Proof Framework.

## Table of Contents

1. [Development Setup](#development-setup)
2. [Code Organization](#code-organization)
3. [Development Principles](#development-principles)
4. [Testing](#testing)
5. [Adding a New Command](#adding-a-new-command)
6. [Adding to Internal Packages](#adding-to-internal-packages)
7. [Issue Tracking with Beads](#issue-tracking-with-beads)
8. [Code Review Guidelines](#code-review-guidelines)
9. [Release Process](#release-process)

---

## Development Setup

### Prerequisites

- **Go 1.22+** (the project uses Go 1.25.5)
- **Git** for version control
- **beads (`bd`)** for issue tracking (optional but recommended)

### Cloning the Repository

```bash
git clone https://github.com/tobias/vibefeld.git
cd vibefeld
```

### Installing Go

If you don't have Go installed, follow the official instructions at [golang.org/doc/install](https://golang.org/doc/install).

Verify your installation:

```bash
go version
# Should show: go version go1.22+ (or later)
```

### Building the Project

```bash
go build ./cmd/af
./af --version
# Should show: af version 0.1.0
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/schema/...

# Run integration/E2E tests (requires build tag)
go test -tags=integration ./e2e/...
```

### Verifying Your Setup

After cloning and building, run the test suite to verify everything is working:

```bash
go test ./...
go build ./cmd/af
./af --help
```

---

## Code Organization

### Directory Structure

```
vibefeld/
├── cmd/af/               # CLI entry point and commands
│   ├── main.go          # Application entry point
│   ├── root.go          # Root command and fuzzy matching
│   ├── init.go          # `af init` command
│   ├── status.go        # `af status` command
│   └── ...              # Other commands
│
├── internal/            # Private packages (not importable externally)
│   ├── cli/             # CLI utilities (arg parsing, prompts, fuzzy flags)
│   ├── config/          # Configuration loading (meta.json)
│   ├── cycle/           # Dependency cycle detection
│   ├── errors/          # Error types and codes (exit codes 1-4)
│   ├── export/          # Proof export functionality
│   ├── fs/              # Filesystem operations (init, read/write nodes)
│   ├── fuzzy/           # Levenshtein distance, command/flag suggestions
│   ├── hash/            # Content hash computation (SHA256)
│   ├── hooks/           # Event hooks system
│   ├── jobs/            # Job detection (prover vs verifier)
│   ├── ledger/          # Event sourcing (append, read, replay)
│   ├── lemma/           # Lemma extraction validation
│   ├── lock/            # File-based lock manager
│   ├── metrics/         # Proof metrics collection
│   ├── node/            # Node, Challenge, Definition structs
│   ├── patterns/        # Common proof patterns
│   ├── render/          # Human-readable and JSON output
│   ├── schema/          # Inference types, node types, states
│   ├── scope/           # Scope tracking for local assumptions
│   ├── service/         # Proof service facade (main API)
│   ├── shell/           # Interactive shell support
│   ├── state/           # Derived state from event replay
│   ├── strategy/        # Proof strategy suggestions
│   ├── taint/           # Taint propagation
│   ├── templates/       # Proof templates (contradiction, induction)
│   └── types/           # Core types (NodeID, timestamps)
│
├── e2e/                 # End-to-end integration tests
│
├── docs/                # Documentation
│
└── go.mod               # Go module definition
```

### Module Responsibilities

Each internal package has a specific responsibility and should stay within **200-300 lines of code**. If a package grows beyond this, consider splitting it.

| Package | Responsibility | Key Types/Functions |
|---------|---------------|---------------------|
| `errors` | Structured error types with exit codes | `AFError`, `ErrorCode`, `ExitCode()` |
| `types` | Core value types | `NodeID`, `Timestamp`, `Parse()` |
| `hash` | Content hash computation | `ComputeNodeHash()` |
| `schema` | Type definitions and validation | `NodeType`, `InferenceType`, `WorkflowState`, `EpistemicState` |
| `fuzzy` | Fuzzy string matching | `SuggestCommand()`, `SuggestFlag()`, `Levenshtein()` |
| `node` | Domain model | `Node`, `Challenge`, `Definition`, `Lemma` |
| `ledger` | Event sourcing | `Ledger`, `Append()`, `Read()`, `Replay()` |
| `lock` | File-based locking | `Manager`, `Acquire()`, `Release()` |
| `state` | State derived from events | `State`, `Replay()`, `GetNode()`, `AllNodes()` |
| `scope` | Local assumption tracking | `Tracker`, `ValidateScope()` |
| `taint` | Epistemic uncertainty propagation | `ComputeTaint()`, `PropagateTaint()` |
| `jobs` | Job detection | `FindJobs()`, `JobResult` |
| `render` | Output formatting | `RenderNode()`, `RenderTree()`, `RenderJSON()` |
| `fs` | Filesystem operations | `InitProofDir()`, `WriteNode()`, `ReadNode()` |
| `service` | High-level facade | `ProofService`, `Init()`, `ClaimNode()`, `RefineNode()` |
| `cli` | CLI helpers | `ParseArgs()`, `Prompt()` |

### Import Hierarchy

Imports should follow a strict hierarchy to prevent cycles:

```
cmd/af
   └── internal/service
          └── internal/{ledger, state, fs, lock, node, ...}
                 └── internal/{types, errors, schema, hash}
```

Lower-level packages should never import higher-level packages. The dependency flow is:

1. **Foundation**: `types`, `errors`, `hash`, `schema`
2. **Domain**: `node`, `scope`, `taint`, `fuzzy`
3. **Infrastructure**: `ledger`, `lock`, `fs`, `config`
4. **State**: `state`, `jobs`
5. **Presentation**: `render`, `cli`
6. **Facade**: `service`
7. **Entry**: `cmd/af`

---

## Development Principles

### TDD: Tests First, No Exceptions

This project follows strict Test-Driven Development:

1. **Write a failing test first** that describes the expected behavior
2. **Implement the minimum code** to make the test pass
3. **Refactor** while keeping tests green

Never commit code without corresponding tests. The test file lives alongside the implementation:

```
internal/hash/hash.go       # Implementation
internal/hash/hash_test.go  # Tests
```

### Small Modules

Target **200-300 lines of code** per file. Benefits:

- Easier to understand and review
- Faster test runs
- Clearer responsibilities
- Easier to refactor

If a file grows beyond 300 lines, look for opportunities to extract a new package.

### Tracer Bullet First

When implementing new features:

1. Get the **minimal working version** first (tracer bullet)
2. Add error handling
3. Add edge cases
4. Optimize if needed

This ensures you have a working solution before adding complexity.

---

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/schema/...

# With coverage
go test -cover ./...

# With verbose output
go test -v ./internal/node/...

# Integration tests (E2E)
go test -tags=integration ./e2e/...
```

### Table-Driven Tests Pattern

Use table-driven tests for comprehensive coverage. This is the standard pattern throughout the codebase:

```go
func TestComputeNodeHash_Format(t *testing.T) {
    tests := []struct {
        name         string
        nodeType     string
        statement    string
        wantErr      bool
    }{
        {
            name:      "valid input",
            nodeType:  "step",
            statement: "Test statement",
            wantErr:   false,
        },
        {
            name:      "empty statement",
            nodeType:  "step",
            statement: "",
            wantErr:   false,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ComputeNodeHash(tt.nodeType, tt.statement, "", "", nil, nil)
            if (err != nil) != tt.wantErr {
                t.Errorf("ComputeNodeHash() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // ... assertions
        })
    }
}
```

### E2E Tests

End-to-end tests live in `e2e/` and test complete workflows:

```go
//go:build integration

package e2e

func TestSimpleProof_FullCompletion(t *testing.T) {
    proofDir, cleanup := setupTest(t)
    defer cleanup()

    // Step 1: Initialize proof
    // Step 2: Claim node
    // Step 3: Refine
    // Step 4: Release
    // Step 5: Accept
    // ...
}
```

Run E2E tests with:

```bash
go test -tags=integration ./e2e/...
```

### Test Helpers

Use `t.Helper()` for test helper functions:

```go
func setupTest(t *testing.T) (string, func()) {
    t.Helper()
    tmpDir, err := os.MkdirTemp("", "test-*")
    if err != nil {
        t.Fatal(err)
    }
    return tmpDir, func() { os.RemoveAll(tmpDir) }
}
```

---

## Adding a New Command

### Step 1: Create the Command File

Create a new file in `cmd/af/`:

```go
// cmd/af/mycommand.go
package main

import (
    "github.com/spf13/cobra"
    "github.com/tobias/vibefeld/internal/service"
)

func newMyCommandCmd() *cobra.Command {
    var flagValue string

    cmd := &cobra.Command{
        Use:   "mycommand",
        Short: "Brief description",
        Long: `Detailed description of what this command does.

Examples:
  af mycommand --flag value`,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runMyCommand(cmd, flagValue)
        },
    }

    cmd.Flags().StringVarP(&flagValue, "flag", "f", "", "Description")

    return cmd
}

func runMyCommand(cmd *cobra.Command, flagValue string) error {
    // Use the service layer
    svc, err := service.NewProofService(".")
    if err != nil {
        return err
    }

    // Implement command logic
    // ...

    // Provide next steps guidance
    cmd.Println("\nNext steps:")
    cmd.Println("  af status    - View current state")

    return nil
}

func init() {
    rootCmd.AddCommand(newMyCommandCmd())
}
```

### Step 2: Add Tests

Create corresponding test file:

```go
// cmd/af/mycommand_test.go
package main

import (
    "testing"
)

func TestMyCommand_ValidInput(t *testing.T) {
    // Setup
    // Execute
    // Assert
}

func TestMyCommand_InvalidInput(t *testing.T) {
    // ...
}
```

### Step 3: Use the Service Layer

Commands should use `internal/service.ProofService` as the facade:

```go
svc, err := service.NewProofService(proofDir)
if err != nil {
    return err
}

// Use service methods
err = svc.ClaimNode(nodeID, owner, timeout)
state, err := svc.LoadState()
```

### CLI Conventions

1. **Self-documenting**: Every command output suggests next steps
2. **Forgiving input**: Use fuzzy matching for commands and flags
3. **Role-specific context**: Show relevant information based on role (prover/verifier)
4. **Exit codes**: Use structured error codes (see `internal/errors`)

---

## Adding to Internal Packages

### Module Boundaries

Each package has clear boundaries:

- **DO**: Export only what's necessary
- **DON'T**: Import from higher-level packages
- **DO**: Keep packages focused on a single responsibility

### Error Handling Conventions

Use the structured error types from `internal/errors`:

```go
import "github.com/tobias/vibefeld/internal/errors"

// Creating errors with codes
if nodeID == "" {
    return errors.New(errors.INVALID_PARENT, "node ID cannot be empty")
}

// Wrapping errors
if err != nil {
    return errors.Wrap(err, "failed to load state")
}

// Checking error types
if errors.Code(err) == errors.ALREADY_CLAIMED {
    // Handle specific error
}

// Using exit codes
exitCode := errors.ExitCode(err)
// 1 = retriable, 2 = blocked, 3 = logic error, 4 = corruption
```

### Error Code Categories

| Exit Code | Category | Examples |
|-----------|----------|----------|
| 1 | Retriable | `ALREADY_CLAIMED`, `NOT_CLAIM_HOLDER` |
| 2 | Blocked | `NODE_BLOCKED` |
| 3 | Logic Error | `INVALID_PARENT`, `SCOPE_VIOLATION`, `DEPTH_EXCEEDED` |
| 4 | Corruption | `CONTENT_HASH_MISMATCH`, `LEDGER_INCONSISTENT` |

### Adding a New Error Code

Add to `internal/errors/errors.go`:

```go
const (
    // ... existing codes
    MY_NEW_ERROR ErrorCode = iota + 100 // Use a new range
)

var errorCodeNames = map[ErrorCode]string{
    // ... existing names
    MY_NEW_ERROR: "MY_NEW_ERROR",
}

// Update ExitCode() if needed
func (c ErrorCode) ExitCode() int {
    switch c {
    // ... handle new code
    }
}
```

---

## Issue Tracking with Beads

This project uses **beads (`bd`)** for issue tracking. Issues are stored in the repository and synced with git.

### Quick Reference

```bash
# List all open issues
bd list

# Show ready-to-work tasks (no blockers)
bd ready

# Show issue details
bd show vibefeld-a3f2dd

# Start working on an issue
bd update vibefeld-a3f2dd --status in_progress

# Close a completed issue
bd close vibefeld-a3f2dd

# Create a new issue
bd q "Description of the task"

# Add a dependency
bd dep add vibefeld-abc123 --needs vibefeld-def456

# Show blocked issues
bd blocked
```

### Issue Workflow

1. **Find work**: `bd ready` to see unblocked tasks
2. **Claim it**: `bd update <id> --status in_progress`
3. **Implement**: Follow TDD (write tests first!)
4. **Close**: `bd close <id>` when done
5. **File follow-ups**: `bd q "Remaining work..."` for new issues

### Issue Naming Convention

Issues use the prefix `vibefeld-` followed by a hash (e.g., `vibefeld-a3f2dd`).

### Syncing Issues

After making changes to issues:

```bash
bd sync
git add -A
git commit -m "Issue updates"
git push
```

---

## Code Review Guidelines

### What Reviewers Look For

1. **Tests exist and pass**
   - Every change must have corresponding tests
   - Tests should cover edge cases, not just happy paths

2. **Module size**
   - Files should be under 300 lines
   - If larger, consider splitting

3. **Error handling**
   - Uses structured errors from `internal/errors`
   - Appropriate error codes and messages

4. **Documentation**
   - Exported functions have godoc comments
   - Complex logic has inline comments

5. **Imports follow hierarchy**
   - No import cycles
   - Lower packages don't import higher ones

6. **CLI conventions**
   - Commands provide next steps guidance
   - Uses fuzzy matching for forgiving input

### Common Pitfalls

1. **Missing tests**: Don't submit code without tests
2. **Large PRs**: Keep changes focused and reviewable
3. **Import cycles**: Check your import graph
4. **Hardcoded paths**: Use `filepath.Join()` for cross-platform compatibility
5. **Ignoring errors**: Always handle errors explicitly
6. **Mutating input**: Functions should not modify their input parameters
7. **Missing error codes**: New errors should use the structured error system

### PR Checklist

Before submitting a PR:

- [ ] Tests pass: `go test ./...`
- [ ] Build succeeds: `go build ./cmd/af`
- [ ] No linting issues: `go vet ./...`
- [ ] Files under 300 lines
- [ ] Imports follow hierarchy
- [ ] Errors use structured error types
- [ ] CLI commands provide next steps
- [ ] Issue updated in beads

---

## Release Process

### Versioning

The project follows semantic versioning (SemVer):

- **MAJOR**: Breaking changes to CLI or API
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

Current version is defined in `cmd/af/main.go`:

```go
const Version = "0.1.0"
```

### Building a Release

```bash
# Build for current platform
go build -o af ./cmd/af

# Cross-compile for multiple platforms
GOOS=linux GOARCH=amd64 go build -o af-linux-amd64 ./cmd/af
GOOS=darwin GOARCH=amd64 go build -o af-darwin-amd64 ./cmd/af
GOOS=darwin GOARCH=arm64 go build -o af-darwin-arm64 ./cmd/af
GOOS=windows GOARCH=amd64 go build -o af-windows-amd64.exe ./cmd/af
```

### Pre-Release Checklist

Before releasing:

1. [ ] All tests pass: `go test ./...`
2. [ ] E2E tests pass: `go test -tags=integration ./e2e/...`
3. [ ] Version updated in `main.go`
4. [ ] CHANGELOG updated (if maintained)
5. [ ] All issues closed or deferred
6. [ ] Documentation up to date

### Creating a Release

1. Update the version in `cmd/af/main.go`
2. Commit and tag:
   ```bash
   git add -A
   git commit -m "Release v0.2.0"
   git tag v0.2.0
   git push origin main --tags
   ```
3. Build release binaries
4. Create GitHub release with binaries attached

---

## Getting Help

- Read the project documentation in `docs/`
- Check existing issues with `bd list`
- Review the codebase - it's designed to be self-documenting
- The CLI itself provides guidance: `af --help`, `af <command> --help`

---

## Summary

Key points for contributors:

1. **TDD always**: Write tests first
2. **Small modules**: Target 200-300 LOC
3. **Use the service layer**: `internal/service.ProofService`
4. **Structured errors**: Use `internal/errors`
5. **Track with beads**: Use `bd` for issue management
6. **Self-documenting CLI**: Provide next steps in output
7. **Follow the hierarchy**: Don't create import cycles

Thank you for contributing to Vibefeld!
