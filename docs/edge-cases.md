# AF Edge Cases and Limits

This document describes edge case behaviors and system limits for the AF (Adversarial Proof Framework).

## Table of Contents

1. [Empty Proof Behavior](#empty-proof-behavior)
2. [Single Node Proof](#single-node-proof)
3. [Maximum Depth Limits](#maximum-depth-limits)
4. [Maximum Children Limits](#maximum-children-limits)
5. [Maximum Statement Length](#maximum-statement-length)
6. [Special Character Handling](#special-character-handling)
7. [Concurrent Access Edge Cases](#concurrent-access-edge-cases)

---

## Empty Proof Behavior

### Definition
An "empty proof" refers to a proof directory with no events in the ledger (0 nodes, no initialized state).

### Behavior

| Operation | Behavior |
|-----------|----------|
| `af status` | Returns error: proof not initialized |
| `af init` | Creates root node (ID `1`) as part of initialization |
| Ledger `ReadAll()` | Returns empty slice `[]` with no error |
| Ledger `Count()` | Returns `0` |
| Ledger `HasGaps()` | Returns `false` (no gaps in empty sequence) |
| State replay | Returns empty `State` with `LatestSeq() == 0` |

### Key Points

- A valid AF proof always has at least one event (`ProofInitialized`) after `af init`
- The root node (ID `1`) is created during initialization with the conjecture as its statement
- An uninitialized proof directory is not the same as an empty ledger - it lacks `meta.json`

---

## Single Node Proof

### Definition
A proof with only the root node (ID `1`) created during `af init`.

### Behavior

| Aspect | Behavior |
|--------|----------|
| Node count | 1 (root only) |
| Node ID | `"1"` |
| Depth | 1 |
| Parent | None (root has no parent) |
| Children | None initially |
| State | `AllChildrenValidated(rootID)` returns `true` (no children to validate) |

### Valid Operations on Single-Node Proof

- **Claim**: Root node can be claimed for refinement
- **Refine**: Add child nodes (e.g., `1.1`, `1.2`)
- **Challenge**: Verifiers can raise challenges
- **Validate/Admit**: Verifiers can accept the proof

### Completion Criteria

A single-node proof is considered complete when:
- The root node reaches epistemic state `validated` or `admitted`
- No open challenges remain

---

## Maximum Depth Limits

### Configuration

| Parameter | Default | Minimum | Maximum |
|-----------|---------|---------|---------|
| `MaxDepth` | 20 | 1 | 100 |

The maximum allowed `MaxDepth` configuration value is defined by `MaxDepthLimit = 100`.

### Depth Calculation

- Root node `"1"` has depth 1
- First-level children `"1.1"`, `"1.2"` have depth 2
- Depth equals the number of parts in the hierarchical ID

### Enforcement

| Operation | Validation |
|-----------|------------|
| `CreateNode` | Fails with `ErrMaxDepthExceeded` if new node depth > `MaxDepth` |
| `RefineNode` | Fails with `ErrMaxDepthExceeded` if child depth > `MaxDepth` |
| `RefineNodeBulk` | Fails with `ErrMaxDepthExceeded` if any child depth > `MaxDepth` |

### Error Handling

When depth is exceeded:
- Error code: `DEPTH_EXCEEDED`
- Exit code: 3 (logic error)
- Error message includes current depth and configured maximum

### Example

With default `MaxDepth = 20`:
- Node `"1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1"` (depth 20): Allowed
- Adding a child to create depth 21: Rejected with `ErrMaxDepthExceeded`

---

## Maximum Children Limits

### Configuration

| Parameter | Default | Minimum | Maximum |
|-----------|---------|---------|---------|
| `MaxChildren` | 10 | 1 | 50 |

### Enforcement

| Operation | Validation |
|-----------|------------|
| `CreateNode` | Fails with `ErrMaxChildrenExceeded` if parent already has `MaxChildren` children |
| `RefineNode` | Fails with `ErrMaxChildrenExceeded` if adding child would exceed limit |
| `RefineNodeBulk` | Fails with `ErrMaxChildrenExceeded` if `existing_children + new_children > MaxChildren` |

### Child Count Calculation

Children are counted by finding all nodes whose parent ID matches the target node.
Only direct children are counted, not descendants.

### Example

With default `MaxChildren = 10`:
- Node `"1"` can have children `"1.1"` through `"1.10"`
- Attempting to create `"1.11"` fails with `ErrMaxChildrenExceeded`

---

## Maximum Statement Length

### Current Implementation

**No explicit statement length limit is enforced.**

The statement is stored as a Go `string` in JSON format:
- Validation only checks that the statement is not empty or whitespace-only
- The practical limit is bounded by:
  - Available memory for JSON marshaling
  - Filesystem limits for individual event files
  - JSON encoding overhead for special characters

### Validation Rules

| Check | Behavior |
|-------|----------|
| Empty statement | Rejected: `"node statement cannot be empty"` |
| Whitespace-only | Rejected: Trimmed and considered empty |
| Very long statements | Allowed (no explicit limit) |

### Recommendations

While no limit is enforced, extremely long statements may:
- Impact display rendering
- Cause memory issues during state replay with many nodes
- Affect content hash computation performance

---

## Special Character Handling

### Supported Characters

AF uses standard Go JSON encoding, which supports:
- Full Unicode (UTF-8)
- All printable characters
- Newlines and whitespace
- Mathematical symbols
- LaTeX notation (in the `latex` field)

### JSON Encoding

Special characters are handled by Go's `encoding/json`:

| Character | Encoding |
|-----------|----------|
| `"` (quote) | Escaped as `\"` |
| `\` (backslash) | Escaped as `\\` |
| Newline | Escaped as `\n` |
| Tab | Escaped as `\t` |
| Control chars | Escaped as `\uXXXX` |
| Unicode | Preserved as UTF-8 |

### Content Hash Computation

The content hash (`SHA256`) is computed from the raw statement string:
- Unicode characters are hashed as their UTF-8 bytes
- The hash is deterministic for identical content

### Terminal/CLI Display

The `internal/render` package handles display formatting:
- ANSI escape codes for color are available
- Long statements may be truncated or wrapped depending on terminal width
- JSON output mode preserves full content with proper escaping

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Empty string `""` | Rejected during node creation |
| Null bytes `\x00` | Technically allowed but may cause display issues |
| Very long lines | No line length limit; may affect CLI display |
| RTL characters | Preserved; display depends on terminal support |

---

## Concurrent Access Edge Cases

### File-Based Locking

AF uses file-based locking (`ledger.lock`) for write serialization:

| Parameter | Default |
|-----------|---------|
| Lock timeout (append) | 5 seconds |
| Lock poll interval | 10 milliseconds |
| Config `LockTimeout` (claim) | 5 minutes (1s - 1h configurable) |

### Concurrent Write Behavior

| Scenario | Behavior |
|----------|----------|
| Concurrent `Append` | Serialized via lock; all succeed |
| Lock acquisition timeout | Returns `"timeout waiting for lock"` |
| Lock holder crash | Stale lock file may block others |
| Concurrent `AppendBatch` | Entire batch is atomic; serialized |

### Optimistic Concurrency (CAS)

`AppendIfSequence` provides Compare-And-Swap semantics:

| Scenario | Behavior |
|----------|----------|
| Expected sequence matches | Append succeeds |
| Concurrent modification | Returns `ErrSequenceMismatch` |
| Multiple CAS attempts | Exactly one succeeds; others retry |

### Concurrent Read Behavior

| Operation | Safety |
|-----------|--------|
| `ReadAll` during write | Safe; reads committed events only |
| `Scan` during write | Safe; may see partial updates |
| `Count` during write | Safe; returns committed count |

### Atomic Write Guarantees

All ledger writes follow this pattern:
1. Write to temporary file (`.event-*.tmp`)
2. `fsync()` to ensure durability
3. Atomic rename to final path
4. Lock release

This ensures:
- No partial writes visible to readers
- Crash recovery leaves ledger consistent
- No gaps in sequence numbers

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Process killed holding lock | Lock file remains; manual cleanup may be needed |
| Concurrent claim on same node | First claimant wins; others fail |
| Network filesystem (NFS) | Not recommended; POSIX atomics may not work correctly |
| Simultaneous `af init` | One succeeds; others fail (directory/file exists) |

### Lock Cleanup

After all operations complete:
- Lock file (`ledger.lock`) should be removed
- Stale locks may require manual removal or timeout-based reaping
- The `LockReaped` event records automatic lock cleanup

---

## Summary Table

| Limit | Default | Min | Max | Configurable |
|-------|---------|-----|-----|--------------|
| Max Depth | 20 | 1 | 100 | Yes (`max_depth`) |
| Max Children | 10 | 1 | 50 | Yes (`max_children`) |
| Statement Length | Unlimited | 1 char | - | No |
| Lock Timeout (config) | 5 minutes | 1 second | 1 hour | Yes (`lock_timeout`) |
| Lock Timeout (append) | 5 seconds | - | - | No (hardcoded) |

---

## Related Documentation

- Configuration: `internal/config/config.go`
- Depth validation: `internal/node/depth.go`
- Ledger operations: `internal/ledger/`
- Concurrent tests: `internal/ledger/concurrent_test.go`
