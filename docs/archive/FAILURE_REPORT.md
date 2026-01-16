# AF Tool Critical Failure Report

**Date**: 2026-01-14
**Test Case**: Dobinski's Formula Proof
**Rating**: 1/10 - Fundamentally broken, requires architectural overhaul

## Executive Summary

The AF (Adversarial Proof Framework) tool is architecturally flawed in ways that make it unusable for its intended purpose. The core adversarial workflow is inverted, agents lack necessary context, and basic usability issues pervade every interaction. This report documents 80+ distinct failures requiring immediate attention.

---

## Critical Architectural Failures

### FAILURE 1: Verifier Job Detection is Fundamentally Backwards

**Severity**: CRITICAL - Blocks entire workflow
**Location**: `internal/jobs/verifier.go`

**Current Behavior**:
```go
// Leaf nodes (no children) are NOT verifier jobs - they are prover jobs that need refinement.
// A node with no children is a prover job (needs refinement), not a verifier job.
```

Verifier jobs only appear when ALL children of a node are validated. This creates a **bottom-up** workflow where leaves must be accepted first.

**Required Behavior**:
Every newly created node should IMMEDIATELY be a verifier job. The workflow should be:
1. Prover creates claim → **Verifier job appears immediately**
2. Verifier challenges OR accepts
3. If challenged → Prover job appears for that node
4. Prover refines (adds children to address challenge)
5. Each new child is immediately a verifier job
6. Repeat breadth-first

**Why This Matters**:
- Currently there's NO WAY for verifiers to see new nodes until the entire subtree is validated
- This defeats the entire adversarial model
- Provers can keep refining forever without any verification
- The tool name says "adversarial" but the workflow is cooperative

**Fix Required**: Complete rewrite of job detection logic. A node becomes a verifier job when:
- It has a statement (was refined), AND
- It is not currently claimed, AND
- It has no unresolved challenges

### FAILURE 2: Agents Must Look Up Context - Catastrophic UX

**Severity**: CRITICAL - Makes agents nearly useless
**Location**: All job-related commands

**Observed Behavior**:
When my verifier agents tried to work, they had to:
```bash
af get 1.1.1           # Get the node
af defs                # Look up definitions
af externals           # Look up external references
af get 1.1 --full      # Get parent context
```

**Required Behavior**:
A single command should provide EVERYTHING an agent needs:
```
=== VERIFIER JOB: Node 1.1.1 ===

STATEMENT:
By definition of Stirling-second-kind, S(n,k) counts...

JUSTIFICATION:
by_definition

PARENT CONTEXT:
[1.1] Establish the combinatorial foundation...

DEFINITIONS REFERENCED:
- Stirling-second-kind: S(n,k) is the number of ways to partition...
- Bell-number: B_n is the number of partitions of an n-element set...
- partition: A partition of a set S is a collection of non-empty...

EXTERNALS REFERENCED:
(none cited)

OPEN CHALLENGES:
- ch-2e3785232b284295: "The step invokes 'the addition principle'..."

YOUR OPTIONS:
- af challenge 1.1.1 -r "REASON"   # Raise a challenge
- af accept 1.1.1                   # Accept as valid
```

**Why This Matters**:
- Agents waste turns looking up context
- Agents might miss relevant definitions/externals
- The "self-documenting CLI" principle is violated
- Every lookup is a potential error point

### FAILURE 3: No Mechanism to Mark Node as "Complete"

**Severity**: HIGH
**Location**: Missing functionality

**Problem**:
There's no way for a prover to signal "this node is done, ready for verification". The options are:
- Keep refining (adding children forever)
- Release and hope someone verifies

**Required**: Either:
- `af mark-complete <node-id>` command
- `af refine ... --terminal` flag for leaf nodes
- Automatic detection based on node type (some types are inherently terminal)

### FAILURE 4: Claim/Refine/Release Creates Massive Contention

**Severity**: HIGH
**Location**: Claim-based workflow

**Observed Behavior**:
To add 4 children to node 1, I had to:
1. Claim node 1
2. Refine (creates 1.1)
3. Release node 1
4. Claim node 1 again (MIGHT FAIL - another agent could grab it!)
5. Refine (creates 1.2)
6. Release...
7. (repeat 2 more times)

Multiple agents racing to refine the same parent causes:
- `ErrConcurrentModification` errors
- Unpredictable child ID assignment
- Agents creating 1.3 when they expected to create 1.5

**Required**: One of:
- `af refine-bulk` - add multiple children atomically
- Remove claim requirement for refinement (append-only ledger already handles concurrency)
- Reservation system to pre-allocate child IDs

---

## Job Context Failures

### FAILURE 5: `af jobs` Output is Useless

**Current Output**:
```
[1.1] claim: "Establish the combinatorial foundation: The Bell number B..."
```

**Problems**:
- Statement truncated with "..."
- No justification shown
- No parent context
- No definitions/externals
- No challenges
- No indication of WHY this is a job

**Required**: Full context for each job (see Failure 2)

### FAILURE 6: `af get` Default Output is Truncated

**Current**:
```
[1.1.1] claim (pending): "By definition of Stirling-second-kind (def:Stirling-secon..."
```

**Required**: Default should show full text, or at minimum not truncate in the middle of a word.

### FAILURE 7: No `af challenges` Command

**Problem**: No way to list all open challenges across the proof.

**Current Workaround**: Must examine each node individually.

**Required**:
```bash
af challenges                    # List all open challenges
af challenges --node 1.1.1       # Challenges for specific node
af challenges --status open      # Filter by status
```

### FAILURE 8: Challenge Details Not Shown in Node View

When viewing a node with `af get 1.1.1 --full`, open challenges are not displayed.

**Required**: Show all challenges, their status, and any responses.

---

## Command Syntax Failures

### FAILURE 9: Inconsistent Flag Names

**Observed**:
- `af refine` uses `-i` for inference type, `-T` for node type
- The supervisor prompt template had `-j` for justification, `-t` for type
- Documentation and actual CLI don't match

**Required**: Consistent, intuitive flag names across all commands.

### FAILURE 10: `add-external` Positional vs Flag Confusion

**Tried**: `af add-external "name" "source"` (positional)
**Required**: `af add-external -n "name" -s "source"` (flags)

Help text should show BOTH syntaxes or be crystal clear about which is required.

### FAILURE 11: No Fuzzy Matching Despite Claims

CLAUDE.md claims "Forgiving input (fuzzy matching for commands/flags)" but:
- `af add-external "name" "source"` failed with unhelpful error
- No suggestion of correct syntax
- No fuzzy matching observed

---

## Workflow Failures

### FAILURE 12: No Clear Prover vs Verifier Workflow Documentation

Agents don't know:
- When should I challenge vs accept?
- How do I respond to a challenge?
- When is a node "complete enough"?
- What makes a good refinement?

**Required**: Role-specific workflow guides embedded in CLI output.

### FAILURE 13: Challenge Resolution Workflow is Opaque

After a verifier challenges:
1. How does the prover see the challenge? (Must check `af get` or `af log`?)
2. Does the prover need to claim the node?
3. After `af resolve-challenge`, what happens?
4. Does the verifier need to review the response?
5. How does the verifier withdraw the challenge if satisfied?

**Required**: Clear state machine documentation and CLI guidance.

### FAILURE 14: No "Stuck" Detection

If no progress is being made (all nodes challenged, no resolutions), the system doesn't detect or flag this.

**Required**:
- `af health` command showing blockers
- Warning when proof is stuck
- Suggestions for resolution

### FAILURE 15: Taint System Never Resolves

All nodes show `taint: unresolved`. The taint system is documented but apparently never runs.

**Required**:
- Automatic taint computation after state changes
- `af recompute-taint` should not be necessary (but should work when called)
- Clear explanation of when taint becomes "clean"

---

## Output and Display Failures

### FAILURE 16: Status Output Truncates Everything

```
1 [pending/unresolved] Dobinski's Formula: For all n >= 0, B_n = (1/e) * sum_{k=...
```

Mathematical proofs require PRECISION. Truncation is unacceptable.

**Required**:
- `af status --full` to show complete text
- Or wrap long lines instead of truncating
- NEVER truncate in the middle of a formula

### FAILURE 17: JSON Output Missing Fields

`af status -f json` output showed `"children": 0` for all nodes despite the tree clearly having parent-child relationships.

**Required**: Accurate JSON that includes:
- children array (actual child node IDs)
- challenges array
- parent ID
- full statement text

### FAILURE 18: No Color/Formatting for Readability

All output is plain text. No visual hierarchy.

**Required**:
- Color-coded states (pending=yellow, validated=green, challenged=red)
- Indentation for tree structure
- Bold for important information

### FAILURE 19: Unicode/Math Symbols Mangled

In JSON output: `k\u003e=0` instead of `k>=0`

**Required**: Proper Unicode handling or ASCII alternatives.

---

## Missing Essential Features

### FAILURE 20: No Bulk Operations

Can't do:
- Add multiple children in one command
- Accept multiple nodes
- Close multiple challenges

**Required**: Bulk variants of all mutation commands.

### FAILURE 21: No Proof Template/Skeleton

For common proof patterns (induction, contradiction, cases), there should be templates.

**Required**:
```bash
af init --template contradiction -c "CONJECTURE"
# Creates structure:
# 1: Conjecture
# 1.1: Assume negation
# 1.2: Derive contradiction
# 1.3: Conclude original
```

### FAILURE 22: No Search/Filter

Can't search for:
- Nodes containing specific text
- Nodes referencing specific definitions
- Nodes in specific states

**Required**: `af search` command with filters.

### FAILURE 23: No Proof Export

Can't export to:
- LaTeX
- PDF
- Markdown
- JSON (complete structured export)

**Required**: `af export --format latex` etc.

### FAILURE 24: No Undo/Rollback

If a prover makes a mistake, there's no recovery:
- Can't delete a node
- Can't edit a statement
- Can archive, but that's permanent

**Required**:
- `af amend <node-id>` to fix statements
- Or clear archive-and-retry workflow

### FAILURE 25: No Partial Acceptance

Verifier must fully accept or challenge. Can't say "I accept the logic but need notation clarification."

**Required**:
- `af accept --with-note "MINOR ISSUE"`
- Or challenge severity levels

### FAILURE 26: No Challenge Categories

All challenges are equal. But some are:
- Critical (logical error)
- Major (missing justification)
- Minor (notation, typo)

**Required**: Challenge severity/category classification.

### FAILURE 27: No Dependency Tracking Between Branches

Can't express that node 1.5 depends on 1.1, 1.2, 1.3, 1.4 all being validated.

**Required**: Cross-branch dependency declarations.

### FAILURE 28: No Progress Metrics

Can't see:
- Percentage complete
- Critical path to completion
- Estimated remaining work

**Required**: `af progress` or enhanced `af stats`.

### FAILURE 29: No Agent Activity Tracking

Can't see:
- Which agents are currently active
- What nodes are claimed by whom
- Historical agent activity

**Required**: `af agents` or enhanced `af status`.

### FAILURE 30: No Diff/History View

Can't see what changed over time. The ledger has this info but it's not exposed.

**Required**: `af history <node-id>` showing evolution.

---

## Documentation Failures

### FAILURE 31: Help Text Doesn't Match Reality

The supervisor prompt I was given had completely wrong command syntax:
- `-j JUSTIFICATION` doesn't exist (it's part of `-s` statement)
- `-t TYPE` should be `-T TYPE`
- Positional args for `add-external` don't work

**Required**: Authoritative, tested help text.

### FAILURE 32: No Examples in Help

`af refine --help` doesn't show realistic examples with all required flags.

**Required**: Complete working examples for each command.

### FAILURE 33: No Workflow Tutorial

No guide for "how to prove something from start to finish."

**Required**: Step-by-step tutorial in `af help tutorial` or similar.

### FAILURE 34: Error Messages Don't Guide Action

When something fails, the error should suggest what to do.

**Current**: `Error: name is required and cannot be empty`
**Required**: `Error: name is required. Usage: af add-external -n "name" -s "source"`

---

## Concurrency and Locking Failures

### FAILURE 35: Claim Timeout is Invisible

Claims expire after timeout, but:
- Agent doesn't know the timeout
- No warning before expiry
- Node just becomes available again silently

**Required**:
- Show timeout in claim confirmation
- Warning when timeout approaching
- Event when claim expires

### FAILURE 36: No Claim Extension

Can't extend a claim without release-and-reclaim (risking loss to another agent).

**Required**: `af extend-claim <node-id>`

### FAILURE 37: Stale Lock Accumulation

`af reap` exists but unclear when/how stale locks accumulate.

**Required**: Automatic reaping or clear documentation.

### FAILURE 38: Race Condition in ID Assignment

When multiple provers refine the same parent concurrently, child IDs are assigned unpredictably:
- Prover A expects to create 1.5, gets 1.3
- Prover B expects 1.3, gets 1.4

**Required**: Either prevent concurrent refinement of same parent OR make IDs predictable/reservable.

---

## Verification Logic Failures

### FAILURE 39: No Definition Application Verification

When a prover cites `def:Stirling-second-kind`, the system doesn't verify:
- The definition exists
- The application is syntactically correct
- The definition content matches the claim

**Required**:
- Syntax: `def:NAME` should be validated
- Warning if NAME doesn't exist
- Optional: semantic validation

### FAILURE 40: No External Reference Verification

Same as above for `external:NAME` citations.

### FAILURE 41: No Cross-Reference Validation

When node A claims to depend on node B, this isn't validated or tracked.

**Required**:
- `depends-on: 1.1.3` should be validated
- Dependency graph should be maintained
- Cycles should be detected

### FAILURE 42: No Circular Reasoning Detection

If the proof structure contains logical cycles, they're not detected.

**Required**: Cycle detection in logical dependencies (not just tree structure).

### FAILURE 43: No Scope Tracking for Assumptions

When an assumption is made (proof by contradiction), its scope isn't tracked:
- Which nodes are "inside" the assumption?
- When does the scope end?
- Are conclusions properly scoped?

**Required**: Assumption scope tracking and validation.

---

## API and Integration Failures

### FAILURE 44: JSON API Incomplete

JSON output is available but:
- Missing fields (children, challenges)
- Inconsistent between commands
- No documented schema

**Required**: Complete JSON schema with all fields, documented.

### FAILURE 45: No Programmatic Lock Management

CLI locks files, but no API for programmatic access with proper locking.

**Required**: Document lock protocol for external tools.

### FAILURE 46: No Event Stream

Can't watch for events in real-time.

**Required**: `af watch` or event streaming API.

### FAILURE 47: No Webhook/Hook Support

Can't trigger external actions on events.

**Required**: Hook system for external integrations.

---

## Performance and Scalability Failures

### FAILURE 48: Linear Scan for Job Detection

Job detection appears to scan all nodes. For large proofs, this will be slow.

**Required**: Index or cache for job detection.

### FAILURE 49: Full Ledger Replay on Every Operation

State is rebuilt by replaying entire ledger. For large proofs, this will be slow.

**Required**: Checkpoint/snapshot system.

### FAILURE 50: No Pagination

`af status` shows all nodes. For large proofs, this will be unusable.

**Required**: `af status --limit 10 --offset 20` or similar.

---

## State Machine Failures

### FAILURE 51: Epistemic State Transitions Undocumented

What causes transitions between:
- pending → validated
- pending → refuted
- pending → admitted
- pending → archived

**Required**: Clear state machine documentation.

### FAILURE 52: Workflow State Transitions Undocumented

What causes transitions between:
- available → claimed
- claimed → blocked
- blocked → available

**Required**: Clear state machine documentation.

### FAILURE 53: Taint State Transitions Undocumented

What causes transitions between:
- unresolved → clean
- clean → self_admitted
- clean → tainted

**Required**: Clear state machine documentation.

### FAILURE 54: Challenge State Machine Missing

Challenges have states but they're undocumented:
- open → resolved
- open → withdrawn
- What triggers each?

**Required**: Clear state machine documentation.

---

## Edge Case Failures

### FAILURE 55: Empty Proof Handling

What happens with 0 nodes? Is this valid?

### FAILURE 56: Single Node Proof

Can a proof be just the conjecture with no refinement?

### FAILURE 57: Deep Nesting Limits

What's the maximum depth? (1.1.1.1.1.1.1...)

### FAILURE 58: Wide Branching Limits

What's the maximum children per node? (1.1, 1.2, ... 1.1000?)

### FAILURE 59: Long Statement Handling

What's the maximum statement length? How are long statements displayed?

### FAILURE 60: Special Characters in Statements

How are quotes, newlines, Unicode math symbols handled?

---

## Missing Validation

### FAILURE 61: No Statement Validation

Statements can be empty or nonsensical.

**Required**: Minimum length, format validation.

### FAILURE 62: No Justification Validation

Justifications can be empty or invalid.

**Required**: Validate inference types, referenced definitions.

### FAILURE 63: No Challenge Reason Validation

Challenge reasons can be empty.

**Required**: Minimum length for challenges.

### FAILURE 64: No Response Validation

Challenge responses can be empty.

**Required**: Minimum length for responses.

---

## Usability Failures

### FAILURE 65: No Interactive Mode

Every operation requires a full command. No REPL or interactive session.

**Required**: `af interactive` or `af shell`.

### FAILURE 66: No Guided Workflow

No wizard for "start a new proof" or "respond to challenge."

**Required**: `af wizard` or guided prompts.

### FAILURE 67: No Tab Completion for Node IDs

Must type full node IDs.

**Required**: Tab completion that knows valid node IDs.

### FAILURE 68: No Confirmation for Destructive Actions

`af refute` and `af archive` are permanent with no confirmation.

**Required**: `--yes` flag or interactive confirmation.

### FAILURE 69: No Dry Run Mode

Can't preview what a command would do.

**Required**: `--dry-run` flag.

### FAILURE 70: No Verbose Mode

Can't see detailed operation logs.

**Required**: `--verbose` or `-v` flag.

---

## Testing and Reliability Failures

### FAILURE 71: Concurrent Modification Tests Missing

The claim system is designed for concurrency but failure modes aren't well-tested.

### FAILURE 72: No Fuzz Testing

What happens with malformed input?

### FAILURE 73: No Stress Testing

What happens with 1000 nodes? 10000?

### FAILURE 74: No Recovery Testing

What happens if process is killed mid-operation?

### FAILURE 75: No Corruption Detection

How is ledger corruption detected?

---

## Design Philosophy Failures

### FAILURE 76: Prover-Centric Design

The tool is designed around provers, not the adversarial model. Verifiers are second-class.

**Required**: Equal tooling for both roles.

### FAILURE 77: No Role Isolation Enforcement

Nothing prevents an agent from running both prover and verifier commands.

**Required**: `--role prover` enforcement or separate binaries.

### FAILURE 78: No Proof Strategy Guidance

The tool doesn't help structure proofs. It's just a ledger.

**Required**: Proof planning features.

### FAILURE 79: No Quality Metrics

No way to measure proof quality:
- Depth of refinement
- Challenge density
- Definition coverage

**Required**: Quality metrics and reports.

### FAILURE 80: No Learning from Challenges

When a challenge reveals a common mistake, this isn't captured for future provers.

**Required**: Challenge pattern library.

---

## Summary of Required Changes

### Immediate (Blocks All Use)
1. Fix verifier job detection - breadth-first, not bottom-up
2. Add full context to job output
3. Add `af challenges` command
4. Fix command syntax inconsistencies

### High Priority
5. Add bulk operations
6. Add node completion marking
7. Fix JSON output completeness
8. Add proper error messages with guidance
9. Document all state machines

### Medium Priority
10. Add search/filter
11. Add export
12. Add progress metrics
13. Add agent tracking
14. Add proof templates

### Lower Priority
15. Add interactive mode
16. Add guided wizards
17. Add proof quality metrics
18. Add learning from challenges

---

## Conclusion

The AF tool has a sound theoretical foundation (append-only ledger, adversarial verification) but the implementation fails at almost every level of practical use:

1. **The core workflow is inverted** - verifiers can't see new nodes until leaves are validated
2. **Agents lack context** - every operation requires multiple lookups
3. **Output is unusable** - truncated, incomplete, poorly formatted
4. **Documentation is wrong** - help text doesn't match reality
5. **Concurrency is broken** - claiming creates more problems than it solves

This is not a usable tool. It's a proof-of-concept that needs 6-12 months of focused development to become production-ready.

**Rating: 1/10** - Architectural redesign required.
