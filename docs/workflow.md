# Prover/Verifier Workflow Guide

This document describes the detailed workflows for provers and verifiers in AF (Adversarial Proof Framework). It covers the adversarial model, role-specific workflows, job detection logic, multi-agent coordination, common patterns, and troubleshooting.

---

## 1. The Adversarial Model

### Why Adversarial?

AF operates on the principle of **adversarial verification** rather than collaborative proof construction. This design choice serves several critical purposes:

1. **Catches Errors**: When the same agent both writes and checks a proof step, confirmation bias can cause errors to slip through. Separating roles ensures genuine scrutiny.

2. **Builds Trust**: A proof that has survived adversarial attack from skeptical verifiers carries more epistemic weight than one that was simply constructed and self-certified.

3. **Prevents Collusion**: Role isolation prevents a single agent from rubber-stamping its own work, maintaining the integrity of the verification process.

4. **Creates Audit Trail**: The challenge-response cycle creates a detailed record of which issues were raised and how they were addressed.

### Separation of Concerns

| Role | Goal | Mindset |
|------|------|---------|
| **Prover** | Convince verifiers that statements are true | Constructive: build and justify |
| **Verifier** | Ensure only valid proofs are accepted | Skeptical: question and validate |

### No Dual Roles

**Critical Rule**: No agent ever plays both roles on the same node. An agent cannot:
- Create a node and then accept it
- Raise a challenge and then address it
- Both attack and defend the same claim

This separation is enforced by the tool through claim ownership and role tracking.

---

## 2. Prover Workflow

### Overview

As a prover, your goal is to convince verifiers that mathematical statements are true. You do this by:
1. Developing proof nodes with child steps that provide rigorous justification
2. Addressing challenges raised by verifiers
3. Ensuring every inference is properly justified with definitions and dependencies

### Step 1: Check Status

Before claiming work, understand the current proof state:

```bash
af status
```

This displays:
- The node tree with hierarchical IDs
- Epistemic states (pending, validated, admitted, refuted, archived)
- Taint states (clean, self_admitted, tainted, unresolved)
- Statistics summary
- Available jobs

### Step 2: Find Available Work

List jobs available for provers:

```bash
af jobs --role prover
```

**You see a prover job when a node has:**
- `EpistemicState = "pending"` (not yet verified)
- `WorkflowState != "blocked"` (can be available or claimed)
- One or more open blocking challenges (critical/major severity)

Example output:
```
PROVER JOBS AVAILABLE

NODE 1.2.1
  Statement: Since p is prime and 2 | p, we have 2 in {1, p}
  Reason: Has open challenge needing response
  Challenge: ch-003 "You assume 2 | p without justification"

  To work on this:
    af claim 1.2.1 --owner prover-42 --role prover
```

### Step 3: Claim a Job

Claim the node to begin work:

```bash
af claim 1.2.1 --owner prover-42 --role prover
```

**Claim options:**
- `--timeout 2h` - Set custom claim duration (default: 1h)
- `--refresh` - Extend an existing claim without releasing

The claim provides exclusive access to prevent concurrent modifications.

### Step 4: Understand the Context

After claiming, the tool provides context about what needs to be done. Use `af get` to retrieve node details:

```bash
af get 1.2.1 --full
```

This shows:
- The target node's statement, type, and status
- Open challenges that must be addressed
- Ancestor chain (parent nodes leading to root)
- Active scope (local assumptions in effect)
- Available definitions and assumptions
- Valid inference types

### Step 5: Refine the Node

Add child nodes to develop the proof or address challenges:

**Single child:**
```bash
af refine 1.2.1 \
  --owner prover-42 \
  --statement "By definition of even, p = 2k implies 2 | p" \
  --justification by_definition \
  --type claim
```

**Multiple children (quick form):**
```bash
af refine 1.2.1 "Step A" "Step B" "Step C" --owner prover-42
```

**Multiple children (JSON form):**
```bash
af refine 1.2.1 --owner prover-42 --children '[
  {"statement": "First substep", "type": "claim", "justification": "by_definition"},
  {"statement": "Second substep", "type": "claim", "justification": "modus_ponens"}
]'
```

**Key flags:**
- `--type` - Node type: claim, local_assume, local_discharge, case, qed
- `--justification` - Inference type (see `af inferences` for full list)
- `--depends` - Comma-separated node IDs this step depends on
- `--sibling` - Add as sibling instead of child

### Step 6: Request Definitions (if needed)

If you need a definition that does not exist:

```bash
af request-def "coprime" --latex "\\gcd(a, b) = 1"
```

This blocks the node until a human operator provides the definition.

### Step 7: Release When Done

After adding children or if you cannot continue:

```bash
af release 1.2.1 --owner prover-42
```

**Note**: Successful refine operations automatically release the claim.

### Prover Best Practices

1. **Address challenges directly**: Create nodes that specifically respond to what the verifier objected to
2. **Use correct inference types**: Match the justification to what the children actually demonstrate
3. **Maintain scope**: Do not use assumptions that have been discharged
4. **Declare dependencies**: Use `--depends` to explicitly track which steps a proof relies on
5. **Be precise**: Vague or ambiguous statements invite challenges
6. **Work incrementally**: Smaller, more focused steps are easier to verify

---

## 3. Verifier Workflow

### Overview

As a verifier, your goal is to ensure only valid proofs are accepted. You do this by:
1. Scrutinizing every proof step for errors, gaps, and invalid reasoning
2. Raising challenges when you find issues
3. Accepting steps only when they are rigorously justified
4. Resolving or withdrawing challenges appropriately

### Step 1: Check Status

```bash
af status
```

Review the proof tree to understand context and identify nodes ready for verification.

### Step 2: Find Available Work

List jobs available for verifiers:

```bash
af jobs --role verifier
```

**You see a verifier job when a node has:**
- A non-empty statement
- `EpistemicState = "pending"` (not yet verified)
- `WorkflowState = "available"` (not claimed or blocked)
- No open blocking challenges (critical/major severity)

Example output:
```
VERIFIER JOBS AVAILABLE

NODE 1.2.1
  Statement: Since p is prime and 2 | p, we have 2 in {1, p}
  Reason: No blocking challenges, ready for verification
  Children: 1.2.1.1 [pending]

  To work on this:
    af claim 1.2.1 --owner verifier-42 --role verifier
```

### Step 3: Claim a Job

```bash
af claim 1.2.1 --owner verifier-42 --role verifier
```

### Step 4: Review the Node

Use `af get` to examine the node in detail:

```bash
af get 1.2.1 --full
```

**What to check (Verification Checklist):**

**Structural Issues (raise critical/major challenge):**
- Dependencies reference undefined nodes
- Scope violations (using discharged assumptions)
- Invalid inference type
- Circular dependencies

**Semantic Issues (raise challenge):**
- Claim does not follow from dependencies
- Inference rule misapplied
- Hidden quantifiers or type errors
- Domain restrictions without justification
- Incomplete case analysis
- Sign errors in inequalities

### Step 5: Take Action

#### Option A: Accept (if valid)

When all children are validated and no blocking issues exist:

```bash
af accept 1.2.1
```

**Acceptance with agent verification:**
```bash
af accept 1.2.1 --agent verifier-42
```
This checks if you have raised any challenges. If not, use `--confirm` to acknowledge you are accepting without having challenged.

**Acceptance with note:**
```bash
af accept 1.2.1 --with-note "Minor issue but acceptable"
```

#### Option B: Challenge (if semantic issue found)

When you find an error, gap, or invalid reasoning:

```bash
af challenge 1.2.1 \
  --reason "The step assumes x > 0 but only x >= 0 is established" \
  --target domain \
  --severity major
```

**Severity levels:**
| Severity | Meaning | Blocks Acceptance |
|----------|---------|-------------------|
| `critical` | Fundamental error that must be fixed | Yes |
| `major` | Significant issue that should be addressed | Yes |
| `minor` | Minor issue that could be improved | No |
| `note` | Clarification request or suggestion | No |

**Valid challenge targets:**
| Target | When to Use |
|--------|-------------|
| `statement` | The claim itself is unclear or wrong |
| `inference` | The reasoning step is invalid |
| `context` | Missing or incorrect definition/assumption references |
| `dependencies` | Missing or incorrect step references |
| `scope` | Scope violation (using out-of-scope assumption) |
| `gap` | Unstated substeps required |
| `type_error` | Type mismatch in mathematical content |
| `domain` | Unjustified domain restriction |
| `completeness` | Incomplete case analysis or enumeration |

#### Option C: Resolve Challenge

When a prover has adequately addressed a challenge:

```bash
af resolve-challenge ch-003 --response "Child 1.2.1.1 correctly justifies the claim"
```

#### Option D: Reject/Refute (if structurally invalid)

For fundamental structural errors:

```bash
af refute 1.2.1 --reason "Circular dependency detected" --yes
```

### Step 6: Release When Done

```bash
af release 1.2.1 --owner verifier-42
```

### Verifier Best Practices

1. **Be thorough**: Check all aspects before accepting
2. **Be specific**: Clearly describe what is wrong in challenges
3. **Use appropriate severity**: Reserve `critical` for fundamental errors
4. **Review addressing nodes**: Before resolving, ensure the prover's response actually fixes the issue
5. **Withdraw when wrong**: If your challenge was mistaken, withdraw it
6. **Accept incrementally**: Validate leaf nodes first, then work up to parents

---

## 4. Job Detection Logic

### How AF Determines Available Jobs

The job detection system in `internal/jobs/` determines which nodes need prover or verifier attention based on their state and challenges.

### Prover Job Conditions

A node qualifies as a **prover job** when:

1. `EpistemicState = "pending"` (not yet verified)
2. `WorkflowState != "blocked"` (can be available or claimed)
3. Has at least one **open blocking challenge** (critical or major severity)

```go
// Simplified from internal/jobs/prover.go
func isProverJob(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
    if n.WorkflowState == schema.WorkflowBlocked {
        return false
    }
    if n.EpistemicState != schema.EpistemicPending {
        return false
    }
    return hasBlockingChallenges(n, challengeMap)
}
```

### Verifier Job Conditions

A node qualifies as a **verifier job** when:

1. Has a non-empty statement
2. `EpistemicState = "pending"` (not yet verified)
3. `WorkflowState = "available"` (not claimed or blocked)
4. Has **no open blocking challenges** (critical or major severity)

```go
// Simplified from internal/jobs/verifier.go
func isVerifierJob(n *node.Node, challengeMap map[string][]*node.Challenge) bool {
    if n.Statement == "" {
        return false
    }
    if n.WorkflowState != schema.WorkflowAvailable {
        return false
    }
    if n.EpistemicState != schema.EpistemicPending {
        return false
    }
    return !hasBlockingChallenges(n, challengeMap)
}
```

### Priority Rules

1. **New nodes go to verifiers first**: In the breadth-first adversarial model, every new node is immediately a verifier job.

2. **Challenges shift work to provers**: When a verifier raises a blocking challenge, the node becomes a prover job.

3. **Resolution returns to verifiers**: When all blocking challenges are resolved/withdrawn, the node returns to verifier territory for final acceptance.

4. **Non-blocking challenges do not shift jobs**: Minor and note severity challenges do not create prover jobs and do not block acceptance.

### Blocking Conditions

A node becomes **blocked** (not available as any job) when:
- It has a pending definition request
- It is claimed by another agent
- Its workflow state is explicitly set to "blocked"

### Severity and Acceptance

Challenge severity determines whether acceptance is blocked:

| Severity | Creates Prover Job | Blocks Acceptance |
|----------|-------------------|-------------------|
| `critical` | Yes | Yes |
| `major` | Yes | Yes |
| `minor` | No | No |
| `note` | No | No |

---

## 5. Multi-Agent Coordination

### Concurrent Sessions

AF supports multiple agents working simultaneously through:

1. **Exclusive Claims**: Only one agent can claim a node at a time
2. **Lock-Based Coordination**: File system locks prevent race conditions
3. **Event Sourcing**: All state changes are recorded in the append-only ledger

### Lock Management

#### Acquiring Locks

When an agent claims a node:
- A lock file is created with the agent's ID and expiration time
- Other agents attempting to claim the same node receive an error
- Locks have configurable timeouts (default: 1 hour)

#### Lock Timeouts

Locks automatically expire after their timeout period. This prevents abandoned work from blocking progress indefinitely.

```bash
# Extend an existing claim
af claim 1.2.1 --owner prover-42 --refresh --timeout 2h
```

#### Reaping Stale Locks

When agents crash or abandon work, locks can become stale:

```bash
# Preview what would be reaped
af reap --dry-run

# Reap expired locks
af reap

# Reap all locks regardless of expiration
af reap --all
```

### Conflict Resolution

#### Optimistic Concurrency

AF uses optimistic concurrency control:
1. Read the current state (including latest sequence number)
2. Perform work
3. Write changes with the observed sequence number
4. If the sequence has changed, the write fails

#### Handling Conflicts

If a conflict occurs:
1. Re-read the current state
2. Merge or discard changes as appropriate
3. Retry the operation

The ledger's append-only nature means conflicts are detected at write time, preventing data loss.

### Orchestrator Pattern

For automated proof attempts, an orchestrator can:

```bash
#!/bin/bash
while true; do
  # Spawn provers for available jobs
  for job in $(af jobs --role prover --format json | jq -r '.[].id'); do
    spawn_prover_agent "$job" &
  done

  # Spawn verifiers for available jobs
  for job in $(af jobs --role verifier --format json | jq -r '.[].id'); do
    spawn_verifier_agent "$job" &
  done

  # Clean up stale locks
  af reap

  # Check completion
  ROOT_STATE=$(af get 1 --format json | jq -r '.epistemic_state')
  if [ "$ROOT_STATE" != "pending" ]; then
    echo "Proof complete: $ROOT_STATE"
    exit 0
  fi

  sleep 10
done
```

---

## 6. Common Patterns

### Quick Validation Cycle

For leaf nodes or simple claims:

```bash
# Verifier checks and accepts
af claim 1.2.1 --owner verifier-42 --role verifier
af accept 1.2.1 --agent verifier-42
af release 1.2.1 --owner verifier-42
```

### Deep Refinement Session

For complex claims requiring multiple steps:

```bash
# Prover develops multi-level structure
af claim 1.1 --owner prover-42 --role prover
af refine 1.1 "Assume for contradiction" "Then X" "But Y" "Contradiction" --owner prover-42

# Children need verification
# Verifier validates leaves first
af claim 1.1.4 --owner verifier-42 --role verifier
af accept 1.1.4
# ... work up the tree
```

### Challenge Resolution Workflow

**Step 1: Verifier raises challenge**
```bash
af claim 1.2.1 --owner verifier-42 --role verifier
af challenge 1.2.1 --reason "Missing justification for X" --target gap --severity major
af release 1.2.1 --owner verifier-42
```

**Step 2: Prover addresses challenge**
```bash
af claim 1.2.1 --owner prover-42 --role prover
af refine 1.2.1 --owner prover-42 --statement "Here is the missing step..."
# (auto-released after refine)
```

**Step 3: Verifier reviews and accepts**
```bash
# First verify the new child
af claim 1.2.1.1 --owner verifier-42 --role verifier
af accept 1.2.1.1
af release 1.2.1.1 --owner verifier-42

# Then resolve challenge and accept parent
af claim 1.2.1 --owner verifier-42 --role verifier
af resolve-challenge ch-xxx --response "Adequately addressed by 1.2.1.1"
af accept 1.2.1
af release 1.2.1 --owner verifier-42
```

### Escape Hatches

**Admit without full proof** (introduces taint):
```bash
af admit 1.2.1  # Node becomes "admitted" with "self_admitted" taint
```

**Archive abandoned branch** (preserves history):
```bash
af archive 1.3 --reason "Taking different approach" --yes
```

**Refute disproven claim**:
```bash
af refute 1.2.1 --reason "Found counterexample" --yes
```

---

## 7. Troubleshooting

### "No jobs available"

**Symptoms**: `af jobs` returns empty results.

**Possible causes and solutions:**

1. **All nodes are claimed**
   ```bash
   af agents  # Check active claims
   af reap    # Clean up stale locks
   ```

2. **All nodes are blocked**
   ```bash
   af pending-defs   # Check for pending definition requests
   af status         # Review node states
   ```

3. **Proof is complete**
   ```bash
   af status         # Check root node state
   ```

4. **All pending nodes have blocking challenges (for verifiers)**
   - Switch to prover role to address challenges

5. **No nodes have blocking challenges (for provers)**
   - Switch to verifier role to add challenges or accept nodes

### Blocked Nodes

**Symptoms**: Node has `WorkflowState = "blocked"`.

**Possible causes:**

1. **Pending definition request**
   ```bash
   af pending-defs
   # Human operator must resolve:
   af def-add "term" --latex "..."
   # or
   af def-reject req-xxx --reason "..."
   ```

2. **External reference pending verification**
   ```bash
   af pending-refs
   af verify-external ext-xxx --status verified
   ```

### Stuck Proofs

**Symptoms**: No progress being made, but proof is not complete.

**Diagnosis:**

1. **Check overall status**
   ```bash
   af status
   af health  # Detect stuck states
   ```

2. **Identify blocking issues**
   ```bash
   af challenges  # List all challenges
   ```

3. **Review job availability**
   ```bash
   af jobs --role prover
   af jobs --role verifier
   ```

**Common stuck scenarios:**

| Scenario | Solution |
|----------|----------|
| Circular challenges | Human intervenes to break cycle |
| Unaddressable challenge | Verifier withdraws or challenge is reconsidered |
| Definition deadlock | Human provides definition |
| Depth limit reached | Archive branch, try different approach |

### Lock Issues

**Symptoms**: Cannot claim a node that should be available.

**Solutions:**

1. **Check if already claimed**
   ```bash
   af agents  # List active claims
   ```

2. **Reap stale locks**
   ```bash
   af reap --dry-run  # Preview
   af reap            # Execute
   ```

3. **Force reap (emergency)**
   ```bash
   af reap --all  # Reap all locks regardless of expiration
   ```

### Challenge States

| State | Meaning | Next Step |
|-------|---------|-----------|
| `open` | Awaiting response | Prover addresses, or verifier withdraws |
| `resolved` | Verifier satisfied | Node can proceed to acceptance |
| `withdrawn` | Verifier retracted | Node can proceed to acceptance |
| `superseded` | Parent archived/refuted | Challenge is moot |

---

## Quick Reference

### Prover Commands

```bash
af status                              # View proof state
af jobs --role prover                  # Find prover work
af claim <id> --owner X --role prover  # Claim for prover work
af get <id> --full                     # Get node context
af refine <id> --owner X --statement "..." --justification Y  # Add child
af request-def "term"                  # Request definition
af release <id> --owner X              # Release claim
```

### Verifier Commands

```bash
af status                               # View proof state
af jobs --role verifier                 # Find verifier work
af claim <id> --owner X --role verifier # Claim for verifier work
af get <id> --full                      # Get node context
af challenge <id> --reason "..." --target T --severity S  # Raise challenge
af resolve-challenge <ch-id> --response "..."  # Resolve challenge
af accept <id>                          # Accept valid node
af refute <id> --reason "..." --yes     # Reject invalid node
af release <id> --owner X               # Release claim
```

### Escape Hatches

```bash
af admit <id>                           # Accept without full proof (introduces taint)
af archive <id> --reason "..." --yes    # Abandon branch
af refute <id> --reason "..." --yes     # Mark as disproven
```

### Administration

```bash
af agents                               # Show active claims
af reap                                 # Clean stale locks
af health                               # Check proof health
af pending-defs                         # List pending definitions
af pending-refs                         # List pending externals
```
