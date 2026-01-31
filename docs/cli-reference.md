# AF CLI Reference

AF (Adversarial Proof Framework) is a command-line tool for collaborative construction of natural-language mathematical proofs.

Multiple AI agents work concurrently as adversarial provers and verifiers, refining proof steps until rigorous acceptance. Provers convince, verifiers attack - no agent plays both roles.

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `init` | Initialize a new proof workspace |
| `status` | Show proof status and node tree |
| `claim` | Claim a job for work |
| `release` | Release a claimed job |
| `refine` | Add child node(s) to a claimed parent |
| `amend` | Amend a node's statement |
| `challenge` | Raise a challenge against a proof node |
| `resolve-challenge` | Resolve a challenge with a response |
| `refine-sibling` | Add sibling node (breadth expansion) |
| `accept` | Accept/validate proof nodes |
| `admit` | Admit a node without full verification (introduces taint) |
| `refute` | Refute a proof node (mark as disproven) |
| `archive` | Archive a proof node (abandon the branch) |
| `request-refinement` | Request deeper proof for validated node |
| `withdraw-challenge` | Withdraw an open challenge |
| `get` | Get node details by ID |
| `jobs` | List available jobs |
| `search` | Search and filter nodes |
| `history` | Show node evolution history |
| `log` | Show event ledger history |
| `replay` | Replay ledger to rebuild and verify state |
| `export` | Export proof to different formats |
| `scope` | Show scope information for a node |
| `deps` | Show dependency graph for a node |
| `challenges` | List challenges across the proof |
| `def-add` | Add a definition to the proof |
| `defs` | List all definitions |
| `def` | Show a specific definition |
| `request-def` | Request a definition for a term |
| `pending-defs` | List pending definition requests |
| `pending-def` | Show a specific pending definition |
| `def-reject` | Reject a pending definition request |
| `extract-lemma` | Extract a lemma from a validated node |
| `lemmas` | List all lemmas |
| `lemma` | Show a specific lemma |
| `add-external` | Add an external reference |
| `externals` | List all external references |
| `external` | Show a specific external reference |
| `verify-external` | Verify an external reference |
| `pending-refs` | List pending external references |
| `pending-ref` | Show a specific pending reference |
| `assumptions` | List assumptions in the proof |
| `assumption` | Show a specific assumption |
| `recompute-taint` | Recompute taint state for all nodes |
| `agents` | Show agent activity and claimed nodes |
| `extend-claim` | Extend duration of an existing claim |
| `reap` | Clean up stale/expired locks |
| `health` | Check proof health and detect stuck states |
| `progress` | Show proof progress metrics |
| `metrics` | Show proof quality metrics |
| `watch` | Stream events in real-time |
| `shell` | Start an interactive shell session |
| `wizard` | Guided workflow wizards |
| `strategy` | Proof structure and strategy guidance |
| `tutorial` | Show step-by-step proving guide |
| `schema` | Show proof schema information |
| `inferences` | List valid inference types |
| `types` | List valid node types |
| `hooks` | Manage hooks for external integrations |
| `patterns` | Manage challenge pattern library |
| `completion` | Generate shell completion scripts |
| `version` | Display version and build information |

---

## Global Flags

These flags apply to all commands:

| Flag | Description |
|------|-------------|
| `--verbose` | Enable verbose output for debugging |
| `--dry-run` | Preview changes without making them |
| `-h, --help` | Help for any command |

---

## Exit Codes

| Code | Category | Description |
|------|----------|-------------|
| 0 | Success | Command completed successfully |
| 1 | Retriable | Race conditions, transient failures (e.g., ALREADY_CLAIMED, NOT_CLAIM_HOLDER) |
| 2 | Blocked | Work cannot proceed (e.g., NODE_BLOCKED) |
| 3 | Logic Error | Invalid input, not found, scope violations |
| 4 | Corruption | Data integrity failures (e.g., CONTENT_HASH_MISMATCH) |

---

## Proof Initialization

### `init`

Initialize a new proof workspace with a conjecture to prove.

**Syntax:**
```
af init [flags]
```

**Flags:**

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--conjecture` | `-c` | string | Yes* | The mathematical conjecture to prove |
| `--author` | `-a` | string | Yes* | The author initiating the proof |
| `--dir` | `-d` | string | No | Directory to initialize (default: ".") |
| `--template` | `-t` | string | No | Proof template: contradiction, induction, cases |
| `--list-templates` | | bool | No | List available proof templates |

*Required unless using `--list-templates`

**Examples:**
```bash
# Basic initialization
af init --conjecture "All primes greater than 2 are odd" --author "Claude"

# With shorthand flags
af init -c "P = NP" -a "Alice" -d ./my-proof

# Using a proof template
af init -c "Sum of first n integers is n(n+1)/2" -a "Claude" --template induction

# List available templates
af init --list-templates
```

**Next Steps:** After initialization, use `af status` to view the proof tree, then `af claim` to start working.

---

## Status and Navigation

### `status`

Show the current proof status including the node tree, statistics, and available jobs.

**Syntax:**
```
af status [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format: text or json |
| `--limit` | `-l` | int | 0 | Maximum nodes to display (0 = unlimited) |
| `--offset` | `-o` | int | 0 | Number of nodes to skip |

**Examples:**
```bash
af status                        # Show status in current directory
af status --dir /path/to/proof   # Specific proof directory
af status --format json          # JSON output
af status --limit 10             # Show first 10 nodes
af status --limit 10 --offset 5  # Pagination: 10 nodes starting from 6th
```

**Next Steps:** Use `af jobs` to see available work, or `af get <node-id>` for node details.

---

### `get`

Get detailed information about a proof node.

**Syntax:**
```
af get <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The hierarchical node ID (e.g., "1", "1.2", "1.2.3") |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory |
| `--format` | `-f` | string | "text" | Output format: text or json |
| `--ancestors` | `-a` | bool | false | Show ancestor chain |
| `--subtree` | `-s` | bool | false | Show all descendants |
| `--full` | `-F` | bool | false | Show full node details |
| `--checklist` | `-c` | bool | false | Show verification checklist |

**Examples:**
```bash
af get 1                    # Show node 1
af get 1.2 --ancestors      # Node with ancestor chain
af get 1 --subtree          # Node and all descendants
af get 1 --full             # Full details
af get 1.1 -a -F            # Ancestors with full details
af get 1 -s -f json         # Subtree in JSON
af get 1.1 --checklist      # Verification checklist for verifiers
```

**Next Steps:** Use `af claim` to work on the node, or `af challenge` to raise an objection.

---

### `jobs`

List available prover and verifier jobs in the proof.

**Syntax:**
```
af jobs [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format: text or json |
| `--role` | `-r` | string | | Filter by role: prover or verifier |

**Job Types:**

- **Verifier jobs**: Nodes ready for review (pending, available, no open challenges)
- **Prover jobs**: Nodes with open challenges that need addressing

**Examples:**
```bash
af jobs                     # List all available jobs
af jobs --role prover       # Only prover jobs
af jobs --role verifier     # Only verifier jobs
af jobs --format json       # JSON output
```

**Next Steps:** Use `af claim` to claim a job and start working.

---

### `search`

Search for proof nodes by text content, state, or definition references.

**Syntax:**
```
af search [query] [flags]
```

**Flags:**

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--dir` | `-d` | string | Proof directory (default: ".") |
| `--text` | `-t` | string | Search node content/statements |
| `--state` | `-s` | string | Filter by epistemic state |
| `--workflow` | `-w` | string | Filter by workflow state |
| `--def` | | string | Search nodes referencing a definition |
| `--json` | | bool | Output in JSON format |

**State Values:**
- Epistemic: `pending`, `validated`, `admitted`, `refuted`, `archived`
- Workflow: `available`, `claimed`, `blocked`

**Examples:**
```bash
af search "convergence"              # Text search
af search --state pending            # All pending nodes
af search --workflow available       # All available nodes
af search --def "continuity"         # Nodes referencing definition
af search --state validated --json   # Validated nodes as JSON
af search -t "limit" -s pending      # Combined filters
```

---

### `history`

Display the complete history of events affecting a specific node.

**Syntax:**
```
af history <node-id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--json` | | bool | false | Output in JSON format |

**Examples:**
```bash
af history 1                # History of root node
af history 1.2.3            # History of node 1.2.3
af history 1 --json         # JSON format
af history 1 -d ./proof     # Specific proof directory
```

---

### `log`

Display the event ledger history for the proof.

**Syntax:**
```
af log [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format: text or json |
| `--limit` | `-n` | int | 0 | Limit output to N events (0 = unlimited) |
| `--since` | | int | 0 | Show events after sequence number N |
| `--reverse` | | bool | false | Show newest events first |

**Examples:**
```bash
af log                      # Show all events
af log --since 10           # Events after sequence 10
af log -n 5                 # First 5 events
af log --reverse            # Newest first
af log --reverse -n 10      # 10 newest events
af log -f json              # JSON output
```

---

## Claiming and Releasing Work

### `claim`

Claim a proof node to work on as a prover or verifier.

**Syntax:**
```
af claim <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to claim |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--owner` | `-o` | string | Yes | | Owner identity for the claim |
| `--role` | `-r` | string | No | "prover" | Agent role: prover or verifier |
| `--timeout` | `-t` | string | No | "1h" | Claim timeout (e.g., 30m, 1h, 2h30m) |
| `--dir` | `-d` | string | No | "." | Proof directory |
| `--format` | `-f` | string | No | "text" | Output format: text or json |
| `--refresh` | | bool | No | false | Extend existing claim timeout |

**Examples:**
```bash
af claim 1 --owner prover-001 --role prover
af claim 1.2 --owner verifier-alpha --timeout 30m --role verifier
af claim 1 -o prover-001 -r prover -t 2h --format json
af claim 1 --owner prover-001 --refresh --timeout 2h  # Extend claim
```

**Exit Codes:**
- 0: Success
- 1: ALREADY_CLAIMED (retriable - another agent has the claim)

**Next Steps:** Use `af refine` (prover) or `af accept`/`af challenge` (verifier).

---

### `release`

Release a claimed job, making it available for other agents.

**Syntax:**
```
af release <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to release |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--owner` | `-o` | string | Yes | | Agent ID that owns the claim |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format: text or json |

**Examples:**
```bash
af release 1 --owner prover-001           # Release root node
af release 1.2.3 -o prover-001            # Short owner flag
af release 1 -o prover-001 -d ./proof     # Specific directory
af release 1 -o prover-001 -f json        # JSON output
```

**Exit Codes:**
- 0: Success
- 1: NOT_CLAIM_HOLDER (you don't own this claim)

---

### `extend-claim`

Extend the timeout of a claimed node without releasing and reclaiming.

**Syntax:**
```
af extend-claim <node-id> [flags]
```

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--owner` | `-o` | string | Yes | | Owner identity for the claim |
| `--duration` | | string | No | "1h" | New duration from now (e.g., 30m, 1h) |
| `--dir` | `-d` | string | No | "." | Proof directory |
| `--format` | `-f` | string | No | "text" | Output format: text or json |

**Examples:**
```bash
af extend-claim 1 --owner prover-001
af extend-claim 1.2 --owner verifier-alpha --duration 2h
af extend-claim 1 -o prover-001 --duration 30m --format json
```

---

## Prover Actions

### `refine`

Add a child node to a claimed parent node.

**Syntax:**
```
af refine <parent-id> [statement]... [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `parent-id` | Yes | The parent node ID |
| `statement...` | No | One or more child statements (creates multiple children) |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--owner` | `-o` | string | Yes | | Agent/owner name (must match claim) |
| `--statement` | `-s` | string | No | | (Deprecated) Use positional args instead |
| `--type` | `-t` | string | No | "claim" | Node type: claim, local_assume, local_discharge, case, qed |
| `--justification` | `-j` | string | No | "assumption" | Inference type |
| `--depends` | | string | No | | Comma-separated node IDs this node depends on |
| `--requires-validated` | | string | No | | Node IDs that must be validated before acceptance |
| `--sibling` | `-b` | bool | No | false | (Deprecated) Use `refine-sibling` command instead |
| `--children` | | string | No | | JSON array of child specifications |
| `--dir` | `-d` | string | No | "." | Proof directory |
| `--format` | `-f` | string | No | "text" | Output format: text or json |

**Note:** Prefer positional arguments over `--statement`. The `--statement` flag is deprecated.

**Inference Types:**
`modus_ponens`, `modus_tollens`, `by_definition`, `assumption`, `local_assume`, `local_discharge`, `contradiction`, `universal_instantiation`, `existential_instantiation`, `universal_generalization`, `existential_generalization`

**Examples:**
```bash
# Single child
af refine 1 --owner agent1 --statement "First subgoal"

# Multiple children via positional args
af refine 1 "Step A" "Step B" "Step C" --owner agent1

# With type and justification
af refine 1 -o agent1 -s "Case 1" --type case --justification local_assume

# With dependencies
af refine 1 -o agent1 -s "By step 1.1, we have..." --depends 1.1

# Multiple dependencies
af refine 1 -o agent1 -s "Combining steps 1.1 and 1.2..." --depends 1.1,1.2

# Validation dependencies
af refine 1.5 -o agent1 -s "Step 1.5" --requires-validated 1.1,1.2,1.3,1.4

# JSON children specification
af refine 1 --owner agent1 --children '[{"statement":"Child 1"},{"statement":"Child 2","type":"case"}]'
```

**Next Steps:** Use `af status` to see the updated tree, then continue refining or release the claim.

---

### `refine-sibling`

Add a sibling node at the same level as the specified node (breadth instead of depth).

**Syntax:**
```
af refine-sibling <node-id> [statement]... [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The sibling node ID (new node added at same level) |
| `statement...` | No | One or more sibling statements |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--owner` | `-o` | string | Yes | | Agent/owner name (must match claim on parent) |
| `--type` | `-t` | string | No | "claim" | Node type: claim, local_assume, local_discharge, case, qed |
| `--justification` | `-j` | string | No | "assumption" | Inference type |
| `--depends` | | string | No | | Comma-separated node IDs this node depends on |
| `--dir` | `-d` | string | No | "." | Proof directory |
| `--format` | `-f` | string | No | "text" | Output format |

**Examples:**
```bash
# Add sibling to node 1.1
af refine-sibling 1.1 "Alternative approach" --owner prover-001

# Multiple siblings
af refine-sibling 1.2 "Case A" "Case B" --owner prover-001 --type case
```

**Note:** You must hold a claim on the parent node to add siblings.

---

### `amend`

Amend a node's statement to correct mistakes.

**Syntax:**
```
af amend <node-id> [flags]
```

**Flags:**

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--owner` | `-o` | string | Yes | Agent/owner name |
| `--statement` | `-s` | string | Yes | New statement text |
| `--dir` | `-d` | string | No | Proof directory (default: ".") |
| `--format` | `-f` | string | No | Output format (default: "text") |

**Requirements:**
- You must be the owner of the node
- Node must be in 'pending' epistemic state
- Node must not be claimed by another agent

**Examples:**
```bash
af amend 1.1 --owner agent1 --statement "Corrected claim about X"
af amend 1.2 -o agent1 -s "Fixed typo in the proof step"
af amend 1.1 --owner agent1 --statement "Clarified statement" --format json
```

---

### `resolve-challenge`

Resolve a previously raised challenge by providing a response.

**Syntax:**
```
af resolve-challenge <challenge-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `challenge-id` | Yes | The challenge ID to resolve |

**Flags:**

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--response` | `-r` | string | Yes | Response text for resolving |
| `--dir` | `-d` | string | No | Proof directory (default: ".") |
| `--format` | `-f` | string | No | Output format (default: "text") |

**Examples:**
```bash
af resolve-challenge chal-001 --response "The statement is clarified..."
af resolve-challenge ch-abc123 -r "Here is the proof of the step..." -d ./proof
```

**Next Steps:** The challenge is now resolved. The verifier may accept the node or raise new challenges.

---

## Verifier Actions

### `challenge`

Raise a challenge (objection) against a proof node.

**Syntax:**
```
af challenge <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to challenge |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--reason` | `-r` | string | Yes | | Reason for the challenge |
| `--severity` | `-s` | string | No | "major" | Severity: critical, major, minor, note |
| `--target` | `-t` | string | No | "statement" | Target aspect |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format |

**Severity Levels:**
| Severity | Blocks Acceptance | Description |
|----------|-------------------|-------------|
| `critical` | Yes | Fundamental error that must be fixed |
| `major` | Yes | Significant issue that should be addressed |
| `minor` | No | Minor issue that could be improved |
| `note` | No | Clarification request or suggestion |

**Valid Targets:**
`statement`, `inference`, `context`, `dependencies`, `scope`, `gap`, `type_error`, `domain`, `completeness`

**Examples:**
```bash
af challenge 1 --reason "The inference is invalid"
af challenge 1.2 --reason "Missing case" --target completeness
af challenge 1 --severity critical --reason "This is fundamentally wrong"
af challenge 1 --severity note --reason "Consider clarifying this step"
af challenge 1 -r "Statement is unclear" -t statement -d ./proof
```

**Next Steps:** Wait for the prover to resolve the challenge or refine the proof.

---

### `accept`

Accept validates proof nodes, marking them as verified correct.

**Syntax:**
```
af accept [node-id]... [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id...` | No* | One or more node IDs to accept |

*Required unless using `--all`

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--all` | `-a` | bool | false | Accept all pending nodes |
| `--agent` | | string | | Agent ID for challenge verification |
| `--confirm` | | bool | false | Confirm acceptance without having raised challenges |
| `--with-note` | | string | | Optional acceptance note (partial acceptance) |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af accept 1              # Accept root node
af accept 1.2.3          # Accept specific node
af accept 1.1 1.2        # Accept multiple nodes
af accept --all          # Accept all pending nodes
af accept -a             # Short form
af accept 1 --with-note "Consider clarifying step 2"
af accept 1 -d ./proof   # Specific directory
af accept 1 --agent verifier-1  # With agent verification
af accept 1 --agent v1 --confirm  # Accept without having raised challenges
```

**Next Steps:** Check `af progress` to see overall completion status.

---

### `request-refinement`

Request deeper refinement for a validated node that needs more detail.

**Syntax:**
```
af request-refinement <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The validated node to request refinement for |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--reason` | `-r` | string | No | | Reason for requesting refinement |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format |

**Requirements:**
- Node must be in `validated` epistemic state
- Transitions node to `needs_refinement` state

**Examples:**
```bash
af request-refinement 1.2 --reason "Step needs more detail"
af request-refinement 1.1.1 -r "Please elaborate on the convergence argument"
```

**Note:** This reopens a validated node for further proof work without invalidating it.

---

### `withdraw-challenge`

Withdraw a previously raised challenge that is still open.

**Syntax:**
```
af withdraw-challenge <challenge-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `challenge-id` | Yes | The challenge ID to withdraw |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Requirements:**
- Challenge must be in `open` status (not already resolved or withdrawn)

**Examples:**
```bash
af withdraw-challenge chal-001
af withdraw-challenge ch-abc123 -d ./proof
```

**Note:** Use this when a challenge was raised in error or is no longer relevant.

---

### `admit`

Admit accepts a proof node without full verification (introduces epistemic taint).

**Syntax:**
```
af admit <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to admit |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af admit 1          # Admit root node
af admit 1.2.3      # Admit specific node
af admit 1 -d ./proof  # Specific directory
```

**Note:** Admitted nodes introduce taint. Any nodes depending on admitted nodes will inherit taint.

**Next Steps:** Use `af recompute-taint` to see taint propagation effects.

---

### `refute`

Refute marks a proof node as disproven or incorrect.

**Syntax:**
```
af refute <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to refute |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--reason` | | string | | Reason for refutation |
| `--yes` | `-y` | bool | false | Skip confirmation prompt |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Warning:** This is a DESTRUCTIVE action. Confirmation required unless `--yes` is provided.

**Examples:**
```bash
af refute 1          # Refute root (prompts for confirmation)
af refute 1 -y       # Refute without confirmation
af refute 1.2.3      # Refute specific node
af refute 1 --reason "Contradicts theorem 3.2"
```

---

### `archive`

Archive marks a proof node as archived, abandoning the branch.

**Syntax:**
```
af archive <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The node ID to archive |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--reason` | | string | | Reason for archiving |
| `--yes` | `-y` | bool | false | Skip confirmation prompt |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Warning:** This is a DESTRUCTIVE action. Confirmation required unless `--yes` is provided.

**Examples:**
```bash
af archive 1          # Archive root (prompts for confirmation)
af archive 1 -y       # Archive without confirmation
af archive 1.2.3      # Archive specific node
af archive 1 --reason "Taking different approach"
```

---

### `challenges`

List challenges across the proof.

**Syntax:**
```
af challenges [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--node` | `-n` | string | | Filter by target node ID |
| `--status` | `-s` | string | | Filter by status: open, resolved, withdrawn |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af challenges                    # List all challenges
af challenges --node 1.1.1       # Challenges on specific node
af challenges --status open      # Only open challenges
af challenges --format json      # JSON output
```

---

## Definitions

### `def-add`

Add a definition to the proof.

**Syntax:**
```
af def-add <name> [content] [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Name of the definition |
| `content` | No* | Definition content |

*Required unless using `--file`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | | Read content from file |
| `--dir` | string | "." | Proof directory path |
| `--format` | string | "text" | Output format |

**Examples:**
```bash
af def-add group "A group is a set with a binary operation."
af def-add homomorphism --file definition.txt
af def-add kernel "The kernel of f is ker(f) = {x : f(x) = e}" --format json
```

---

### `defs`

List all definitions in the proof.

**Syntax:**
```
af defs [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |
| `--verbose` | | bool | false | Show verbose output |

**Examples:**
```bash
af defs                     # List all definitions
af defs --format json       # JSON output
af defs -d /path/to/proof   # Specific directory
```

---

### `def`

Show details of a specific definition.

**Syntax:**
```
af def <name> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--full` | `-F` | bool | false | Show full details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af def group                # Show definition "group"
af def homomorphism -F      # Full details
af def kernel -f json       # JSON output
```

---

### `request-def`

Request a definition for a term needed during proof work.

**Syntax:**
```
af request-def [flags]
```

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--node` | `-n` | string | Yes | | Node ID requesting the definition |
| `--term` | `-t` | string | Yes | | Term to define |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format |

**Examples:**
```bash
af request-def --node 1 --term "group"
af request-def -n 1.2 -t "homomorphism" -d ./proof
af request-def --node 1 --term "kernel" --format json
```

---

### `pending-defs`

List all pending definition requests.

**Syntax:**
```
af pending-defs [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

### `pending-def`

Show a specific pending definition request.

**Syntax:**
```
af pending-def <term|node-id|id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `term\|node-id\|id` | Yes | Term name, node ID, or pending def ID |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--full` | `-F` | bool | false | Show full details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af pending-def group                # By term name
af pending-def 1.1                  # By requesting node
af pending-def abc123               # By ID prefix
af pending-def homomorphism -F      # Full details
```

---

### `def-reject`

Reject a pending definition request.

**Syntax:**
```
af def-reject <term|node-id|id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--reason` | `-r` | string | | Reason for rejection |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af def-reject group                    # Reject by term
af def-reject 1.1                      # Reject by node ID
af def-reject group --reason "Not needed"
```

---

## Lemmas

### `extract-lemma`

Extract a reusable lemma from a validated proof node.

**Syntax:**
```
af extract-lemma <node-id> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | Yes | The validated node to extract from |

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--statement` | `-s` | string | Yes | | Lemma statement |
| `--name` | `-n` | string | No | | Custom lemma name |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format |

**Requirements:**
- Node must be in 'validated' epistemic state
- Node must be independent (no local assumptions from parent scopes)

**Examples:**
```bash
af extract-lemma 1 --statement "For all n >= 0, n! >= 1"
af extract-lemma 1.2 -s "P implies Q" -d ./proof
af extract-lemma 1.1.1 --statement "Base case" --format json
```

---

### `lemmas`

List all lemmas extracted from the proof.

**Syntax:**
```
af lemmas [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

### `lemma`

Show detailed information about a specific lemma.

**Syntax:**
```
af lemma <id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--full` | `-F` | bool | false | Show full details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af lemma LEM-abc123              # Show lemma
af lemma LEM-abc123 --format json
af lemma LEM-abc123 --full
```

---

## External References

### `add-external`

Add an external reference (axiom, theorem, paper citation) to the proof.

**Syntax:**
```
af add-external [NAME SOURCE] [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `NAME` | No* | Name of the external reference |
| `SOURCE` | No* | Source citation |

*Can use flags instead

**Flags:**

| Flag | Short | Type | Required | Default | Description |
|------|-------|------|----------|---------|-------------|
| `--name` | `-n` | string | Yes* | | Name of the reference |
| `--source` | `-s` | string | Yes* | | Source citation |
| `--dir` | `-d` | string | No | "." | Proof directory path |
| `--format` | `-f` | string | No | "text" | Output format |

*Required if not using positional arguments

**Examples:**
```bash
af add-external "Fermat's Last Theorem" "Wiles, A. (1995)"
af add-external "Prime Number Theorem" "de la Vallee Poussin (1896)"
af add-external --name "Theorem 3.1" --source "Paper citation" --format json
```

---

### `externals`

List all external references in the proof.

**Syntax:**
```
af externals [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |
| `--verbose` | | bool | false | Show verbose output |

---

### `external`

Show details of a specific external reference.

**Syntax:**
```
af external <name> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--full` | `-F` | bool | false | Show full details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

### `verify-external`

Mark an external reference as verified by a human reviewer.

**Syntax:**
```
af verify-external <ext-id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

### `pending-refs`

List all pending external references that have not yet been verified.

**Syntax:**
```
af pending-refs [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |
| `--verbose` | | bool | false | Show verbose output |

---

### `pending-ref`

Show details of a specific pending external reference.

**Syntax:**
```
af pending-ref <name> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--full` | `-F` | bool | false | Show full details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

## Assumptions and Scope

### `assumptions`

List assumptions in the proof, or assumptions in scope for a specific node.

**Syntax:**
```
af assumptions [node-id] [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | No | Node ID to show assumptions in scope for |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af assumptions                  # List all assumptions
af assumptions 1                # Assumptions in scope for node 1
af assumptions 1.2 --format json
```

---

### `assumption`

Show detailed information about a specific assumption.

**Syntax:**
```
af assumption <id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

---

### `scope`

Show assumption scope information for a proof node.

**Syntax:**
```
af scope [node-id] [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `node-id` | No | Node ID to show scope for |

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--all` | `-a` | bool | false | Show all active scopes |
| `--dir` | `-d` | string | "." | Proof directory |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af scope 1.2.3              # Scope info for node
af scope 1.2.3 --format json
af scope --all              # All active scopes
```

---

### `deps`

Show the dependency graph for a proof node.

**Syntax:**
```
af deps <node-id> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af deps 1.3              # Show dependencies for node
af deps 1.3 -f json      # JSON output
```

---

## Taint Management

### `recompute-taint`

Recompute taint state for all nodes in the proof tree.

**Syntax:**
```
af recompute-taint [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dry-run` | | bool | false | Preview changes without applying |
| `--verbose` | `-v` | bool | false | Verbose output with details |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Taint States:**
| State | Description |
|-------|-------------|
| `clean` | Validated nodes |
| `self_admitted` | Admitted nodes |
| `tainted` | Children of self_admitted/tainted nodes |
| `unresolved` | Pending nodes |

**Examples:**
```bash
af recompute-taint                    # Recompute in current directory
af recompute-taint --dir /path        # Specific directory
af recompute-taint --dry-run          # Preview changes
af recompute-taint -v                 # Verbose output
af recompute-taint -f json            # JSON output
```

---

## Agent Management

### `agents`

Display agent activity including currently claimed nodes and historical claim/release events.

**Syntax:**
```
af agents [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |
| `--limit` | `-l` | int | 50 | Limit activity history to N events |

**Examples:**
```bash
af agents                     # Show agent activity
af agents --dir /path/to/proof
af agents --format json
af agents --limit 20          # Limit to 20 events
```

---

### `reap`

Clean up stale or expired locks from claimed nodes.

**Syntax:**
```
af reap [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | false | Reap all locks regardless of expiration |
| `--dry-run` | bool | false | Preview what would be reaped |
| `--dir` | string | "." | Proof directory path |
| `--format` | string | "text" | Output format |

**Examples:**
```bash
af reap                    # Reap expired locks
af reap --dry-run          # Preview
af reap --all              # Reap all locks
af reap -d ./proof         # Specific directory
af reap -f json            # JSON output
```

---

## Progress and Health

### `progress`

Show proof progress including completion percentage and blockers.

**Syntax:**
```
af progress [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Displays:**
- Completion percentage (validated + admitted / total)
- Node counts by epistemic state
- Open challenges count
- Pending definition requests count
- Blocked nodes count
- Critical path (deepest pending branch)

---

### `health`

Analyze the proof state to detect if the proof is stuck or making progress.

**Syntax:**
```
af health [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Health Statuses:**
| Status | Description |
|--------|-------------|
| `healthy` | Proof has available work and is making progress |
| `warning` | Proof has potential issues but is not stuck |
| `stuck` | Proof cannot make progress without intervention |

**Detects:**
- All leaf nodes have open challenges
- No available prover or verifier jobs
- Circular dependencies

---

### `metrics`

Analyze the proof and display quality metrics.

**Syntax:**
```
af metrics [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--node` | `-n` | string | | Node ID to focus metrics on (subtree) |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Metrics:**
- Refinement depth: Maximum depth of the proof tree
- Challenge density: Number of challenges per node
- Definition coverage: Percentage of referenced terms with definitions
- Quality score: Composite score (0-100)

**Examples:**
```bash
af metrics                     # Entire proof
af metrics --node 1.2          # Subtree rooted at 1.2
af metrics --format json       # JSON output
```

---

## Real-time Monitoring

### `watch`

Watch the event ledger for new events in real-time.

**Syntax:**
```
af watch [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--interval` | duration | 1s | Poll interval (e.g., 1s, 500ms) |
| `--since` | int | 0 | Start from sequence number |
| `--filter` | string | | Filter events by type (partial match) |
| `--json` | bool | false | Output as NDJSON |
| `--once` | bool | false | Show current events and exit |
| `--dir` | string | "." | Proof directory path |

**Examples:**
```bash
af watch                      # Default 1s interval
af watch --interval 500ms     # 500ms polling
af watch --json               # NDJSON output
af watch --filter node_created  # Filter by event type
af watch --since 10           # Start from sequence 10
af watch --once               # Show current and exit
```

Press Ctrl+C to stop watching.

---

## Interactive Tools

### `shell`

Start an interactive shell session for running af commands.

**Syntax:**
```
af shell [flags]
```

**Aliases:** `shell`, `repl`

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--prompt` | `-p` | string | "af> " | Shell prompt string |

**Built-in Commands:**
- `help` - Show shell help
- `exit`, `quit` - Exit the shell

**Example Session:**
```
$ af shell
af> status
af> claim 1.1 --owner prover-001
af> help
af> exit
```

---

### `wizard`

Interactive wizards that guide you through common AF workflows.

**Syntax:**
```
af wizard [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `new-proof` | Guide through initializing a new proof |
| `respond-challenge` | Guide through responding to challenges |
| `review` | Guide a verifier through reviewing pending nodes |

#### `wizard new-proof`

```
af wizard new-proof [flags]
```

**Flags:**

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--conjecture` | `-c` | string | The conjecture to prove |
| `--author` | `-a` | string | The author |
| `--dir` | `-d` | string | Directory (default: ".") |
| `--template` | `-t` | string | Proof template |
| `--no-confirm` | | bool | Skip confirmation prompts |
| `--preview` | | bool | Preview only |

#### `wizard respond-challenge`

```
af wizard respond-challenge [flags]
```

**Flags:**

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--dir` | `-d` | string | Proof directory (default: ".") |

#### `wizard review`

```
af wizard review [flags]
```

**Flags:**

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--dir` | `-d` | string | Proof directory (default: ".") |

---

## Strategy and Guidance

### `strategy`

Provides guidance on proof strategies and structure.

**Syntax:**
```
af strategy [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `list` | Show all available proof strategies |
| `suggest` | Analyze a conjecture and suggest strategies |
| `apply` | Generate a proof skeleton using a strategy |

#### `strategy list`

```
af strategy list [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--format` | `-f` | string | "text" | Output format |

#### `strategy suggest`

Analyze a conjecture and suggest appropriate proof strategies.

```
af strategy suggest <conjecture> [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af strategy suggest "For all n, n + 0 = n"
af strategy suggest "There is no largest prime"
af strategy suggest --format json "If P then Q"
```

#### `strategy apply`

Generate a proof skeleton for a conjecture using a strategy.

```
af strategy apply <strategy> <conjecture> [flags]
```

**Available Strategies:**
- `direct` - Direct proof by logical deduction
- `contradiction` - Assume negation, derive contradiction
- `induction` - Base case + inductive step
- `cases` - Exhaustive case analysis
- `contrapositive` - Prove the contrapositive

**Examples:**
```bash
af strategy apply induction "For all n, P(n)"
af strategy apply contradiction "There is no largest prime"
af strategy apply cases "Either A or B"
```

---

### `tutorial`

Display a comprehensive tutorial on proving with AF.

**Syntax:**
```
af tutorial [flags]
```

This command works without an initialized proof directory.

---

## Schema and Reference

### `schema`

Display schema information for the AF proof framework.

**Syntax:**
```
af schema [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--section` | `-s` | string | | Filter to specific section |
| `--dir` | `-d` | string | "." | Proof directory (ignored) |
| `--format` | `-f` | string | "text" | Output format |

**Sections:**
- `inference-types` - Valid inference types
- `states` - Workflow, epistemic, and taint states

This command works without an initialized proof directory.

---

### `inferences`

List all valid inference types for use with `af refine -j TYPE`.

**Syntax:**
```
af inferences [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--format` | `-f` | string | "text" | Output format |

**Inference Types:**

| ID | Name | Description |
|----|------|-------------|
| `modus_ponens` | Modus Ponens | If P and P->Q, then Q |
| `modus_tollens` | Modus Tollens | If not Q and P->Q, then not P |
| `by_definition` | By Definition | Follows from a definition |
| `assumption` | Assumption | Assumed premise |
| `local_assume` | Local Assumption | Introduce local hypothesis |
| `local_discharge` | Local Discharge | Conclude from local hypothesis |
| `contradiction` | Contradiction | Derive from contradiction |
| `universal_instantiation` | Universal Instantiation | For all x, P(x) -> P(a) |
| `existential_instantiation` | Existential Instantiation | Exists x, P(x) -> P(c) |
| `universal_generalization` | Universal Generalization | P(a) for arbitrary a -> For all x, P(x) |
| `existential_generalization` | Existential Generalization | P(a) -> Exists x, P(x) |

---

### `types`

List all valid node types for use with `af refine -t TYPE`.

**Syntax:**
```
af types [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--format` | `-f` | string | "text" | Output format |

**Node Types:**

| Type | Description |
|------|-------------|
| `claim` | A mathematical assertion to be justified |
| `local_assume` | Introduce a local hypothesis (opens scope) |
| `local_discharge` | Conclude from local hypothesis (closes scope) |
| `case` | One branch of a case split |
| `qed` | Final step concluding the proof or subproof |

---

## Export and Import

### `export`

Export the proof tree to various document formats.

**Syntax:**
```
af export [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--format` | `-f` | string | "markdown" | Output format: markdown, md, latex, tex |
| `--output` | `-o` | string | | Output file path (default: stdout) |
| `--dir` | `-d` | string | "." | Proof directory path |

**Examples:**
```bash
af export                           # Markdown to stdout
af export --format latex            # LaTeX to stdout
af export -o proof.md               # Markdown to file
af export --format latex -o proof.tex  # LaTeX to file
```

---

### `replay`

Replay all events from the ledger to rebuild and verify the proof state.

**Syntax:**
```
af replay [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--verify` | | bool | false | Verify content hashes during replay |
| `--verbose` | `-v` | bool | false | Show detailed replay progress |
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

**Examples:**
```bash
af replay                         # Replay ledger
af replay --verify                # Verify content hashes
af replay --verbose               # Detailed progress
af replay --format json           # JSON output
```

---

## Hooks

### `hooks`

Manage webhook and command hooks for external integrations.

**Syntax:**
```
af hooks [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `list` | List configured hooks |
| `add` | Add a new hook |
| `remove` | Remove a hook |
| `test` | Test a hook with sample event |

**Hook Types:**
- `webhook` - HTTP POST to URL with event JSON payload
- `command` - Execute shell command with event data as env vars

**Events:**
- `node_created` - Fired when a new node is added
- `node_validated` - Fired when a verifier validates a node
- `challenge_raised` - Fired when a verifier raises a challenge
- `challenge_resolved` - Fired when a challenge is resolved

#### `hooks list`

```
af hooks list [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | "." | Proof directory path |
| `--format` | `-f` | string | "text" | Output format |

#### `hooks add`

```
af hooks add <event-type> <hook-type> <target> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `event-type` | Yes | Event to trigger on |
| `hook-type` | Yes | webhook or command |
| `target` | Yes | URL or shell command |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--description` | string | | Optional description |
| `--disabled` | bool | false | Create in disabled state |
| `--dir` | string | "." | Proof directory path |

**Environment Variables (for commands):**
- `AF_EVENT_TYPE` - The event type
- `AF_NODE_ID` - The affected node ID
- `AF_CHALLENGE_ID` - The challenge ID (if applicable)
- `AF_TIMESTAMP` - When the event occurred

**Examples:**
```bash
af hooks add node_created webhook https://example.com/hook
af hooks add challenge_raised command "notify-send 'Challenge on $AF_NODE_ID'"
af hooks add node_validated webhook https://slack.com/... --description "Slack notification"
```

#### `hooks remove`

```
af hooks remove <hook-id> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | string | "." | Proof directory path |

#### `hooks test`

```
af hooks test <hook-id> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | string | "." | Proof directory path |

---

## Patterns

### `patterns`

Analyze resolved challenges and extract common mistake patterns.

**Syntax:**
```
af patterns [command]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `list` | Show known patterns from resolved challenges |
| `analyze` | Analyze current proof for potential issues |
| `stats` | Show statistics on common challenge types |
| `extract` | Extract patterns from resolved challenges |

**Pattern Types:**
| Type | Description |
|------|-------------|
| `logical_gap` | Missing justification or logical gaps |
| `scope_violation` | Using assumptions outside valid scope |
| `circular_reasoning` | Circular or self-referential dependencies |
| `undefined_term` | Using terms that are not defined |

#### `patterns list`

```
af patterns list [flags]
```

**Flags:**

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--type` | `-t` | string | Filter by pattern type |
| `--json` | | bool | Output in JSON format |
| `--dir` | `-d` | string | Proof directory (default: ".") |

#### `patterns analyze`

```
af patterns analyze [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |
| `--dir` | string | Proof directory (default: ".") |

#### `patterns stats`

```
af patterns stats [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |
| `--dir` | string | Proof directory (default: ".") |

#### `patterns extract`

```
af patterns extract [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |
| `--dir` | string | Proof directory (default: ".") |

---

## Shell Completion

### `completion`

Generate shell completion scripts for af commands.

**Syntax:**
```
af completion [bash|zsh|fish|powershell]
```

**Installation:**

**Bash:**
```bash
# Add to ~/.bashrc:
source <(af completion bash)

# Or save to a file:
af completion bash > /etc/bash_completion.d/af
```

**Zsh:**
```bash
# Add to ~/.zshrc (before compinit):
source <(af completion zsh)

# Or if using oh-my-zsh:
af completion zsh > ~/.oh-my-zsh/completions/_af
```

**Fish:**
```bash
af completion fish > ~/.config/fish/completions/af.fish
```

**PowerShell:**
```powershell
af completion powershell | Out-String | Invoke-Expression
```

**Features:**
- Command name completion
- Flag name completion
- Node ID completion
- Context-aware completions based on proof state

---

## Version

### `version`

Display version and build information.

**Syntax:**
```
af version [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Shows:**
- Version string
- Git commit hash
- Build date
- Go version

---

## Workflow Examples

### Starting a New Proof (Prover)

```bash
# Initialize proof workspace
af init -c "The square root of 2 is irrational" -a "Claude"

# Check status
af status

# List available jobs
af jobs

# Claim the root node
af claim 1 --owner prover-001 --role prover

# Add proof structure
af refine 1 "Assume sqrt(2) = p/q in lowest terms" --owner prover-001 --type local_assume
af refine 1 "Then 2q^2 = p^2" --owner prover-001
af refine 1 "Therefore p is even" --owner prover-001
af refine 1 "And q is even" --owner prover-001
af refine 1 "Contradiction: p/q not in lowest terms" --owner prover-001 --type local_discharge

# Release the claim
af release 1 --owner prover-001
```

### Reviewing a Proof (Verifier)

```bash
# Check available verifier jobs
af jobs --role verifier

# Claim a node for review
af claim 1.1 --owner verifier-001 --role verifier

# Get detailed view with checklist
af get 1.1 --checklist

# Option A: Accept the node
af accept 1.1

# Option B: Raise a challenge
af challenge 1.1 --reason "Step missing justification" --severity major --target inference

# Release the claim
af release 1.1 --owner verifier-001
```

### Responding to a Challenge (Prover)

```bash
# List open challenges
af challenges --status open

# Claim the challenged node
af claim 1.1 --owner prover-001 --role prover

# Option A: Resolve the challenge directly
af resolve-challenge chal-001 --response "The justification follows from..."

# Option B: Add refinement to address the challenge
af refine 1.1 "Additional justification step" --owner prover-001

# Release the claim
af release 1.1 --owner prover-001
```

### Using the Interactive Wizard

```bash
# Start new proof wizard
af wizard new-proof

# Review pending nodes as verifier
af wizard review

# Respond to challenges as prover
af wizard respond-challenge
```

---

## State Reference

### Epistemic States

| State | Description |
|-------|-------------|
| `pending` | Not yet verified (default) |
| `validated` | Verified correct by verifier |
| `needs_refinement` | Validated but reopened for deeper proof |
| `admitted` | Accepted without full verification (introduces taint) |
| `refuted` | Marked as disproven |
| `archived` | Branch abandoned |

### Workflow States

| State | Description |
|-------|-------------|
| `available` | Ready to be claimed |
| `claimed` | Currently being worked on |
| `blocked` | Cannot proceed (e.g., pending definition) |

### Taint States

| State | Description |
|-------|-------------|
| `clean` | No epistemic uncertainty |
| `self_admitted` | Node itself was admitted |
| `tainted` | Depends on admitted node |
| `unresolved` | Pending verification |

---

## Error Reference

### Error Codes

| Code | Name | Exit | Description |
|------|------|------|-------------|
| 1 | ALREADY_CLAIMED | 1 | Node is claimed by another agent |
| 2 | NOT_CLAIM_HOLDER | 1 | You don't own the claim |
| 3 | NODE_BLOCKED | 2 | Node is blocked and cannot proceed |
| 4 | INVALID_PARENT | 3 | Parent node does not exist |
| 5 | INVALID_TYPE | 3 | Invalid node type |
| 6 | INVALID_INFERENCE | 3 | Invalid inference type |
| 7 | INVALID_TARGET | 3 | Invalid challenge target |
| 8 | CHALLENGE_NOT_FOUND | 3 | Challenge does not exist |
| 9 | DEF_NOT_FOUND | 3 | Definition does not exist |
| 10 | ASSUMPTION_NOT_FOUND | 3 | Assumption does not exist |
| 11 | EXTERNAL_NOT_FOUND | 3 | External reference does not exist |
| 12 | SCOPE_VIOLATION | 3 | Assumption used outside valid scope |
| 13 | SCOPE_UNCLOSED | 3 | Local assumption not discharged |
| 14 | DEPENDENCY_CYCLE | 3 | Circular dependency detected |
| 15 | CONTENT_HASH_MISMATCH | 4 | Data integrity failure |
| 16 | LEDGER_INCONSISTENT | 4 | Ledger corruption detected |
| 17 | VALIDATION_INVARIANT_FAILED | 1 | Validation invariant violated |
| 18 | DEPTH_EXCEEDED | 3 | Maximum proof depth exceeded |
| 19 | CHALLENGE_LIMIT_EXCEEDED | 3 | Too many challenges on node |
| 20 | REFINEMENT_LIMIT_EXCEEDED | 3 | Too many children on node |
| 21 | EXTRACTION_INVALID | 3 | Cannot extract lemma from node |

---

## See Also

- `af tutorial` - Comprehensive proof workflow guide
- `af schema` - Full schema reference
- `af strategy list` - Available proof strategies
- `af wizard` - Interactive guided workflows
