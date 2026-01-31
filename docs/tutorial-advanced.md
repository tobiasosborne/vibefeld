# AF Advanced Tutorial: Complex Proof Techniques

This tutorial covers advanced proof techniques in AF (Adversarial Proof Framework). You should already be familiar with the basic workflow from the introductory tutorial. Here we explore sophisticated proof patterns including case analysis, contradiction, induction, definitions, lemma extraction, challenge handling, multi-agent workflows, and debugging techniques.

## Prerequisites

- Completed the basic AF tutorial
- Built the `af` binary: `go build ./cmd/af`
- Familiarity with basic commands: `init`, `claim`, `refine`, `release`, `accept`

---

## 1. Proof by Cases

Proof by cases (case analysis or case split) is used when you need to prove a statement by considering all possible scenarios exhaustively.

### When to Use Case Analysis

Use case analysis when:
- A statement naturally splits into disjoint cases (e.g., "n is even or n is odd")
- You need to prove P(x) for all x in a finite set
- An assumption leads to multiple possibilities that must all be addressed

### How AF Tracks Case Scopes

AF uses the `case` node type for case analysis. Each case branch is a separate child node. The key points:

1. **Case nodes are siblings**: All cases of a split are children of the same parent
2. **Independence**: Each case can be worked on independently by different agents
3. **Completeness**: A verifier will challenge if cases don't exhaust all possibilities
4. **Inference type**: Use `case_split` or `disjunction_elim` as the inference

### Complete Example: Every Integer is Even or Odd

```bash
# Initialize the proof
mkdir case-proof && cd case-proof
af init --conjecture "For every integer n, n is either even or odd" --author "tutorial"

# Prover claims the root
af claim 1 --owner prover-1

# Set up the case split - create case nodes as children
af refine 1 --owner prover-1 --type case --justification case_split \
  "Case 1: n mod 2 = 0" \
  "Case 2: n mod 2 = 1"

# Release the parent after setting up cases
af release 1 --owner prover-1

# Work on Case 1: even case
af claim 1.1 --owner prover-1

af refine 1.1 --owner prover-1 --justification by_definition \
  "If n mod 2 = 0, then n = 2k for some integer k" \
  "By definition of even, n is even"

af release 1.1 --owner prover-1

# Work on Case 2: odd case
af claim 1.2 --owner prover-2

af refine 1.2 --owner prover-2 --justification by_definition \
  "If n mod 2 = 1, then n = 2k + 1 for some integer k" \
  "By definition of odd, n is odd"

af release 1.2 --owner prover-2
```

### Verification of Case Completeness

Verifiers must check that cases are exhaustive. If cases overlap or miss scenarios, raise a challenge:

```bash
af challenge 1 \
  --reason "Cases do not exhaust all possibilities - what about n mod 2 producing negative results?" \
  --target completeness
```

---

## 2. Proof by Contradiction

Proof by contradiction (reductio ad absurdum) assumes the negation of what you want to prove, then derives a logical impossibility.

### Setting Up the Contradiction Assumption

In AF, use `local_assume` to introduce the negated hypothesis. This opens a **scope** that tracks the temporary assumption.

```bash
# Initialize proof
mkdir sqrt2-proof && cd sqrt2-proof
af init --conjecture "The square root of 2 is irrational" --author "tutorial"

# Claim and begin the contradiction structure
af claim 1 --owner prover-1

# Introduce the contradiction assumption with local_assume
af refine 1 --owner prover-1 --type local_assume --justification local_assume \
  "Assume for contradiction that sqrt(2) is rational. Then sqrt(2) = a/b where a, b are coprime integers with b != 0"

af release 1 --owner prover-1
```

### Deriving the Contradiction

Continue building the proof under the assumption's scope:

```bash
# Claim the assumption node to derive consequences
af claim 1.1 --owner prover-1

af refine 1.1 --owner prover-1 --justification modus_ponens --depends 1.1 \
  "Squaring both sides: 2 = a^2/b^2, therefore a^2 = 2b^2"

af refine 1.1 --owner prover-1 --justification modus_ponens \
  "Since a^2 = 2b^2, a^2 is even. An integer squared is even only if the integer itself is even, therefore a is even"

# Continue deriving...
af refine 1.1 --owner prover-1 --justification by_definition \
  "Since a is even, write a = 2k for some integer k"

af refine 1.1 --owner prover-1 --justification modus_ponens \
  "Substituting: (2k)^2 = 2b^2, so 4k^2 = 2b^2, therefore b^2 = 2k^2"

af refine 1.1 --owner prover-1 --justification modus_ponens \
  "Since b^2 = 2k^2, b^2 is even, therefore b is even"

# State the contradiction
af refine 1.1 --owner prover-1 --justification contradiction \
  "Both a and b are even, meaning they share factor 2. This contradicts the assumption that a, b are coprime"

af release 1.1 --owner prover-1
```

### Discharging the Assumption

After establishing the contradiction, discharge the local assumption:

```bash
af claim 1 --owner prover-1

af refine 1 --owner prover-1 --type local_discharge --justification local_discharge \
  "The assumption that sqrt(2) is rational leads to contradiction. Therefore sqrt(2) is irrational"

af release 1 --owner prover-1
```

### How AF Handles Scope

AF tracks scopes automatically:

- `local_assume` nodes create scope entries (e.g., `"1.1.A"`)
- Child nodes inherit the parent's active scope
- `local_discharge` nodes close scopes using `--discharges`
- **Scope invariant**: A node cannot reference dependencies whose scope is not in the node's own scope

View active scopes:
```bash
af get 1.1.5 --scope
```

---

## 3. Proof by Induction

Mathematical induction proves statements for all natural numbers (or other well-ordered structures).

### Base Case and Inductive Step

AF uses specific inference types for induction:
- `induction_base`: Proves P(0) or P(1)
- `induction_step`: Proves P(n) implies P(n+1)

### Example: Sum of First n Natural Numbers

```bash
mkdir induction-proof && cd induction-proof
af init --conjecture "For all n >= 1, the sum 1 + 2 + ... + n = n(n+1)/2" --author "tutorial"

af claim 1 --owner prover-1

# Base case
af refine 1 --owner prover-1 --justification induction_base \
  "Base case: When n = 1, the sum is 1. And 1(1+1)/2 = 2/2 = 1. So P(1) holds."

# Inductive step setup
af refine 1 --owner prover-1 --type local_assume --justification local_assume \
  "Inductive step: Assume P(k) holds for some k >= 1, i.e., 1 + 2 + ... + k = k(k+1)/2"

af release 1 --owner prover-1

# Prove P(k+1) using P(k)
af claim 1.2 --owner prover-1

af refine 1.2 --owner prover-1 --justification modus_ponens \
  "We must show: 1 + 2 + ... + k + (k+1) = (k+1)(k+2)/2"

af refine 1.2 --owner prover-1 --justification substitution --depends 1.2 \
  "By the inductive hypothesis: 1 + 2 + ... + k + (k+1) = k(k+1)/2 + (k+1)"

af refine 1.2 --owner prover-1 --justification direct_computation \
  "Simplifying: k(k+1)/2 + (k+1) = k(k+1)/2 + 2(k+1)/2 = (k+1)(k+2)/2"

af refine 1.2 --owner prover-1 --type qed --justification induction_step \
  "Therefore P(k) implies P(k+1). By induction, P(n) holds for all n >= 1."

af release 1.2 --owner prover-1
```

### Structuring Induction in AF

Best practices for induction proofs:

1. **Clearly label base case and inductive step**: Use appropriate inference types
2. **Use `local_assume` for the inductive hypothesis**: This creates a trackable scope
3. **Reference the inductive hypothesis explicitly**: Use dependencies to link to it
4. **Discharge properly**: The induction step should close the assumption scope

---

## 4. Working with Definitions

Definitions are globally registered, immutable references that provers and verifiers can cite.

### Adding Definitions Mid-Proof

Provers can request new definitions when needed:

```bash
# Request a definition (prover requests while working on a node)
af request-def --node 1.1 --term "coprime"
```

This creates a pending definition request. A human operator must approve it:

```bash
# Operator reviews pending definitions
af pending-defs

# Operator adds the definition
af def-add coprime "Two integers a and b are coprime if gcd(a, b) = 1"

# Or rejects it
af def-reject coprime --reason "Definition already exists as 'relatively_prime'"
```

### Definition Dependencies

When creating a node, reference definitions in the `--context` flag:

```bash
af refine 1.2 --owner prover-1 --justification by_definition --depends 1.1 \
  "Since gcd(a, b) = 1 by assumption, a and b are coprime"
```

### Best Practices for Definitions

1. **Request definitions early**: Don't wait until deep in the proof
2. **Be precise**: Include LaTeX for mathematical notation
3. **Cite sources**: Help verifiers understand provenance
4. **Check existing definitions first**: Use `af defs` to list available definitions
5. **Use consistent naming**: Follow project conventions (e.g., `DEF-prime`, `DEF-even`)

### Listing and Viewing Definitions

```bash
# List all definitions
af defs

# View a specific definition
af def DEF-prime
```

---

## 5. Lemma Extraction

Lemma extraction allows you to package a validated subproof for reuse elsewhere.

### When to Extract a Lemma

Extract a lemma when:
- A subproof is self-contained and could be reused
- The subproof proves a general fact beyond the current context
- You want to simplify the main proof by isolating a result

### The Extraction Workflow

Lemmas can only be extracted from **validated** nodes with certain criteria:

1. All nodes in the extraction set must be validated
2. Internal dependencies must be satisfied within the set
3. Scope entries opened in the set must be closed within it
4. Only the root can be depended on from outside

```bash
# First, ensure the subproof nodes are validated
af status

# Extract a lemma from a validated node
af extract-lemma 1.3 --statement "If n^2 is even, then n is even" --name "Even squares imply even roots"
```

### Using Extracted Lemmas

Once extracted, reference lemmas in your proof:

```bash
af refine 1.5 --owner prover-1 --justification lemma_application \
  "Since a^2 is even, a must be even by LEM-001"
```

### Viewing Lemmas

```bash
# List all extracted lemmas
af lemmas

# View a specific lemma
af lemma LEM-001
```

---

## 6. Handling Challenges

Challenges are how verifiers raise objections. Proper challenge handling is crucial for adversarial verification.

### Different Challenge Types (Targets)

| Target | Description | Example Objection |
|--------|-------------|-------------------|
| `statement` | The claim itself is wrong or unclear | "The statement uses undefined notation" |
| `inference` | The reasoning step is invalid | "Modus ponens doesn't apply here - P->Q is not established" |
| `context` | Missing or incorrect definitions | "DEF-prime is not referenced but used" |
| `dependencies` | Wrong or missing step references | "Step depends on 1.3 but doesn't list it" |
| `scope` | Scope violation | "Uses assumption from discharged scope 1.1.A" |
| `gap` | Unstated substeps required | "This step jumps from A to C without establishing B" |
| `type_error` | Mathematical type mismatch | "Treating integer as real without justification" |
| `domain` | Domain restriction issue | "Division by zero possible when x = 0" |
| `completeness` | Missing cases | "Only covers n > 0, what about n = 0?" |

### Raising Challenges

```bash
# Claim the node first
af claim 1.2.1 --owner verifier-1

# Raise a challenge with specific target
af challenge 1.2.1 \
  --reason "The claim uses p > 0 but only p > 2 is established" \
  --target domain \
  --severity major

af release 1.2.1 --owner verifier-1
```

### Resolution Strategies

**Strategy 1: Add addressing nodes (prover response)**

```bash
# Prover claims the challenged node
af claim 1.2.1 --owner prover-1

# Add child nodes that address the challenge
af refine 1.2.1 --owner prover-1 --justification modus_ponens \
  "Since p > 2 by hypothesis, we have p > 0 a fortiori"

af release 1.2.1 --owner prover-1
```

**Strategy 2: Verifier resolves (challenge was addressed)**

```bash
af claim 1.2.1 --owner verifier-1

# Resolve the challenge after reviewing addressing nodes
af resolve-challenge ch-003 --response "The prover has added node 1.2.1.1 which addresses the concern"

af release 1.2.1 --owner verifier-1
```

**Strategy 3: Verifier withdraws (challenge was unfounded)**

```bash
af withdraw-challenge ch-003
```

### When to Admit vs Prove

Sometimes complete proof is impractical. AF provides **escape hatches**:

**Admit**: Accept without full proof (introduces taint)
```bash
af admit 1.2.1
```

**Note:** Admitted nodes should reference external sources. Add an external reference first:
```bash
af add-external "Rudin Theorem 4.2" "Rudin, Principles of Mathematical Analysis, 3rd ed., Theorem 4.2"
```

**When to admit**:
- Well-known standard results
- Results proven elsewhere (with proper citation)
- Time constraints on complex but accepted facts

**Consequences of admitting**:
- Node gets `self_admitted` taint
- Descendants get `tainted` status
- The proof is complete but not fully verified

**Better alternative**: Use external references:
```bash
af add-external "Extreme Value Theorem" "Every continuous function on [a,b] is bounded (standard analysis result)"
```

---

## 7. Multi-Agent Workflows

AF is designed for concurrent work by multiple agents.

### Concurrent Prover/Verifier Sessions

Multiple agents can work simultaneously on different nodes:

```bash
# Terminal 1: Prover works on node 1.1
af claim 1.1 --owner prover-agent-001
af refine 1.1 --owner prover-agent-001 "Step 1.1 content"
af release 1.1 --owner prover-agent-001

# Terminal 2: Different prover works on node 1.2
af claim 1.2 --owner prover-agent-002
af refine 1.2 --owner prover-agent-002 "Step 1.2 content"
af release 1.2 --owner prover-agent-002

# Terminal 3: Verifier reviews completed work
af claim 1.1 --owner verifier-001
af accept 1.1
af release 1.1 --owner verifier-001
```

### Resolving Conflicts

**Lock contention**: If two agents try to claim the same node:

```bash
$ af claim 1.1 --owner prover-agent-002
Error: ALREADY_CLAIMED

Node 1.1 is currently claimed by agent 'prover-agent-001' (since 2025-01-11T10:05:00Z).

Options:
  - Wait for the agent to complete and release
  - If the agent has crashed, an operator can run:
      af reap --older-than 5m
  - Choose a different job:
      af jobs --role prover
```

**Stale locks**: Clean up abandoned claims:

```bash
# Operator clears stale locks older than 5 minutes
af reap --older-than 5m
```

### Best Practices for Team Proofs

1. **Use consistent naming**: Agent IDs should be meaningful (e.g., `prover-alice`, `verifier-bob`)

2. **Coordinate on scope**: When working on related nodes, communicate about dependencies

3. **Work breadth-first when possible**: Complete sibling nodes before going deeper

4. **Monitor blocking issues**:
   ```bash
   af status  # Shows blocked nodes
   af pending-defs  # Shows definition requests
   ```

5. **Regular synchronization**: Check `af jobs` frequently to see available work

6. **Use JSON output for automation**:
   ```bash
   af jobs --format json | jq '.prover_jobs[] | .id'
   ```

### Orchestrator Pattern

For automated multi-agent workflows:

```bash
#!/bin/bash
# Simple orchestrator loop

while true; do
  # Check for blocking conditions
  PENDING_DEFS=$(af pending-defs --format json | jq -r '.[]')
  if [ -n "$PENDING_DEFS" ]; then
    echo "Blocked on definition requests"
    sleep 60
    continue
  fi

  # Spawn provers for available prover jobs
  for job in $(af jobs --role prover --format json | jq -r '.[] | .id'); do
    spawn_prover_agent "$job" &
  done

  # Spawn verifiers for available verifier jobs
  for job in $(af jobs --role verifier --format json | jq -r '.[] | .id'); do
    spawn_verifier_agent "$job" &
  done

  # Clean up stale locks
  af reap --older-than 5m

  # Check completion
  ROOT_STATE=$(af get 1 --format json | jq -r '.epistemic_state')
  if [ "$ROOT_STATE" = "validated" ]; then
    echo "Proof complete!"
    af status
    exit 0
  fi

  # Check if stuck
  JOBS=$(af jobs --format json | jq -r '.[] | .id')
  if [ -z "$JOBS" ] && [ "$ROOT_STATE" = "pending" ]; then
    echo "Proof stuck - no available jobs"
    af status
    exit 1
  fi

  sleep 10
done
```

---

## 8. Debugging Proofs

When proofs get stuck or confused, AF provides tools to diagnose issues.

### Using `af status` Effectively

The status command is your primary debugging tool:

```bash
af status
```

Output shows:
- Proof tree with epistemic states (`validated`, `pending`, `admitted`, etc.)
- Taint status (`clean`, `tainted`, `unresolved`)
- Open challenges (marked with `(!)`)
- Summary statistics

**Filtering status**:
```bash
af status --pending-only    # Show only pending nodes
af status --challenged-only # Show only nodes with open challenges
```

### Understanding Taint

Taint tracks epistemic uncertainty through dependencies:

| Taint State | Meaning | Cause |
|-------------|---------|-------|
| `clean` | Fully verified | All ancestors validated |
| `self_admitted` | This node was admitted | Used `af admit` on this node |
| `tainted` | Depends on admitted node | Ancestor has `self_admitted` or `tainted` |
| `unresolved` | Not yet decidable | Some ancestor is `pending` |

**Taint propagation rules**:
1. If node is `pending` -> taint is `unresolved`
2. If any ancestor has `unresolved` taint -> `unresolved`
3. If this node is `admitted` -> `self_admitted`
4. If any ancestor has `tainted` or `self_admitted` -> `tainted`
5. Otherwise -> `clean`

**Finding tainted nodes**:
```bash
af status | grep "tainted"
```

### Finding Blocked Nodes

Nodes can be blocked for several reasons:

1. **Pending definition**: Node references a definition that's requested but not added
2. **Claimed by another agent**: Node is locked
3. **Open challenges**: Must be resolved before acceptance

**Diagnose blocking**:
```bash
# See why a specific node is blocked
af get 1.2.1 --full

# See all pending definition requests
af pending-defs

# See who has claims
af get 1.2.1  # Shows claim owner if claimed
```

### Common Issues and Solutions

**Issue: Node can't be accepted**
```bash
$ af accept 1.2.1 --agent verifier-1
Error: VALIDATION_INVARIANT_FAILED

Cannot accept node 1.2.1:
  x Challenge ch-003 has state 'open' (must be resolved/withdrawn/superseded)
  / All children validated
```

**Solution**: Resolve the challenge first:
```bash
af resolve-challenge ch-003 --response "Issue addressed in child nodes"
af accept 1.2.1 --agent verifier-1
```

**Issue: Scope violation**
```bash
$ af refine 1.3 --owner prover-1 --depends 1.1.2 "Statement depending on 1.1.2"
Error: SCOPE_VIOLATION

Node 1.3 cannot depend on 1.1.2: dependency uses scope entry 1.1.A which is not active in node's scope
```

**Solution**: The dependency is from a discharged assumption. Either:
- Reference a different node that's in scope
- Re-derive the result within the current scope

**Issue: Circular dependency**
```bash
$ af refine 1.2 --owner prover-1 --depends 1.2.1 "Statement"
Error: DEPENDENCY_CYCLE

Circular reference detected: 1.2 -> 1.2.1 -> 1.2
```

**Solution**: Remove the circular dependency. Children can depend on parents, but parents shouldn't depend on descendants.

### Verifying Ledger Integrity

If you suspect corruption:

```bash
# Replay ledger and verify consistency
af replay --verify

# Exit code 0: consistent
# Exit code 1: mismatch detected
```

### Force Taint Recalculation

If taint states seem wrong:

```bash
af recompute-taint
```

This recalculates taint for all nodes based on current epistemic states.

---

## Summary of Advanced Commands

| Command | Purpose |
|---------|---------|
| `af refine --type case` | Create case split nodes |
| `af refine --type local_assume` | Introduce local hypothesis (opens scope) |
| `af refine --type local_discharge --discharges ID` | Discharge local hypothesis |
| `af request-def` | Request a new definition |
| `af def-add` | Add definition (operator only) |
| `af extract-lemma` | Package validated subproof as lemma |
| `af challenge --target X` | Raise specific challenge type |
| `af resolve-challenge` | Close a challenge as addressed |
| `af withdraw-challenge` | Retract a challenge |
| `af admit` | Accept without full proof (escape hatch) |
| `af reap` | Clear stale locks |
| `af replay --verify` | Check ledger consistency |
| `af recompute-taint` | Force taint recalculation |

---

## Appendix: Inference Types Reference

| Inference | Form | Usage |
|-----------|------|-------|
| `modus_ponens` | P, P -> Q |- Q | Applying implications |
| `modus_tollens` | ~Q, P -> Q |- ~P | Contrapositive reasoning |
| `universal_instantiation` | forall x.P(x) |- P(t) | Applying universal statements |
| `existential_instantiation` | exists x.P(x) |- P(c) | Naming an existential witness |
| `universal_generalization` | P(x) for arbitrary x |- forall x.P(x) | Proving for all |
| `existential_generalization` | P(c) |- exists x.P(x) | Proving existence |
| `by_definition` | unfold definition | Using definitions |
| `assumption` | global hypothesis | Using proof assumptions |
| `local_assume` | introduce local hypothesis | Starting subproof |
| `local_discharge` | conclude from local hypothesis | Ending subproof |
| `contradiction` | P and ~P |- false | Proof by contradiction |
| `case_split` | P or Q, P |- R, Q |- R |- R | Case analysis |
| `induction_base` | P(0) | Base case of induction |
| `induction_step` | P(n) -> P(n+1) | Inductive step |
| `direct_computation` | arithmetic/algebraic simplification | Calculations |
| `substitution` | a = b, P(a) |- P(b) | Substituting equals |
| `conjunction_intro` | P, Q |- P and Q | Building conjunctions |
| `conjunction_elim` | P and Q |- P | Extracting from conjunctions |
| `disjunction_intro` | P |- P or Q | Building disjunctions |
| `disjunction_elim` | P or Q, P -> R, Q -> R |- R | Case analysis |
| `implication_intro` | P |- Q under P |- P -> Q | Conditional proof |
| `external_application` | apply cited result | Using external references |
| `lemma_application` | apply extracted lemma | Using extracted lemmas |
| `qed` | proof complete | Final step |

---

## Next Steps

- Study the example proofs in `examples/sqrt2-proof/` and `examples/dobinski-proof/`
- Practice with increasingly complex theorems
- Experiment with multi-agent coordination
- Explore the ledger files to understand event sourcing
- Read `docs/prd.md` for complete specification details
