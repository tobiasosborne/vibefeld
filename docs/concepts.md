# AF Core Concepts

This document explains the foundational concepts of AF (Adversarial Proof Framework), a system for collaborative construction of natural-language mathematical proofs through adversarial verification.

## Table of Contents

1. [The Adversarial Model](#the-adversarial-model)
2. [Proof Structure](#proof-structure)
3. [Node States](#node-states)
4. [Challenges](#challenges)
5. [Definitions and Lemmas](#definitions-and-lemmas)
6. [Taint Propagation](#taint-propagation)
7. [Scope and Assumptions](#scope-and-assumptions)

---

## The Adversarial Model

### Why Adversarial Verification?

Traditional proof systems rely on a single agent to both construct and verify proofs. This creates a fundamental weakness: the same cognitive biases and blind spots that lead to errors in construction are present during verification. AF addresses this by separating construction from verification and making them adversarial.

The adversarial model provides several key benefits:

1. **Error Detection**: Verifiers actively seek flaws, not confirmation. This maximizes the probability of catching errors.

2. **Explicit Justification**: Every claim must survive scrutiny from an agent whose sole purpose is to find problems.

3. **Procedural Rigor**: Trust emerges from process, not from any single agent's capabilities. The system is designed so that correct proofs survive while flawed ones are challenged.

4. **Auditability**: The full history of challenges, responses, and decisions is preserved, creating a scientific record of the proof's development.

### Provers vs Verifiers

AF enforces a strict separation of roles. No agent may play both prover and verifier on the same proof node. This is a non-negotiable principle.

**Provers** are responsible for:
- Creating new proof nodes (claims, assertions, lemmas)
- Refining existing nodes by adding child steps
- Responding to challenges by providing additional justification
- Requesting new definitions when needed

**Verifiers** are responsible for:
- Examining proof nodes for correctness
- Raising challenges when they identify issues
- Resolving challenges when they are satisfied by responses
- Accepting (validating) nodes that meet all requirements
- Withdrawing challenges they determine were incorrect

### How Trust Emerges

Trust in AF is not granted to any single agent. Instead, trust emerges from the adversarial interaction:

1. A prover creates a claim
2. A verifier examines it and either:
   - Accepts it (if it passes all checks), or
   - Raises one or more challenges
3. The prover must address challenges with additional justification
4. The verifier evaluates the responses
5. This continues until the verifier is satisfied or the node is abandoned

A node achieves "validated" status only when a verifier explicitly accepts it after all challenges are resolved. This explicit verifier control ensures that no automatic transitions can bypass human (or agent) judgment.

---

## Proof Structure

### Hierarchical Nodes

AF proofs are organized as hierarchical trees. Each node represents a step in the proof and is identified by a hierarchical ID that encodes its position in the tree.

```
1                    (root - the main theorem)
├── 1.1              (first major step)
│   ├── 1.1.1        (substep of 1.1)
│   └── 1.1.2        (another substep)
├── 1.2              (second major step)
│   ├── 1.2.1
│   ├── 1.2.2
│   └── 1.2.3
└── 1.3              (third major step)
```

The hierarchical ID system has these properties:

- The root node is always `1`
- Children of node `X` are numbered `X.1`, `X.2`, `X.3`, etc.
- The depth of a node is the number of components in its ID (e.g., `1.2.3` has depth 3)
- Parent-child relationships are encoded in the ID structure
- IDs are assigned by the tool, never by agents (tool-controlled structure)

### Node Types

Each node has a type that indicates its role in the proof:

| Type | Description |
|------|-------------|
| `claim` | A mathematical assertion to be justified. This is the most common type. |
| `local_assume` | Introduces a local hypothesis, opening a new scope. Used in proof by contradiction or case analysis. |
| `local_discharge` | Concludes from a local hypothesis, closing the scope it opened. |
| `case` | One branch of a case split. Each case is proved independently. |
| `qed` | Final step concluding the proof or a subproof. |

Node types interact with the scope system:
- `local_assume` opens a scope
- `local_discharge` closes a scope
- Other types inherit their parent's scope

### Inference Types

Each node must declare the inference rule that justifies it. AF supports a rich set of inference types:

**Logical Inference Rules:**
| Inference | Form | Description |
|-----------|------|-------------|
| `modus_ponens` | P, P -> Q |- Q | From P and "P implies Q", conclude Q |
| `modus_tollens` | not-Q, P -> Q |- not-P | From "not Q" and "P implies Q", conclude "not P" |
| `contradiction` | P and not-P |- false | From a contradiction, conclude falsehood |

**Quantifier Rules:**
| Inference | Form | Description |
|-----------|------|-------------|
| `universal_instantiation` | forall x.P(x) |- P(t) | Instantiate a universal with a specific term |
| `existential_instantiation` | exists x.P(x) |- P(c) | Introduce a fresh constant witnessing existence |
| `universal_generalization` | P(x) for arbitrary x |- forall x.P(x) | Generalize from an arbitrary instance |
| `existential_generalization` | P(c) |- exists x.P(x) | From a witness, conclude existence |

**Proof Structure Rules:**
| Inference | Form | Description |
|-----------|------|-------------|
| `by_definition` | unfold definition | Apply a definition directly |
| `assumption` | global hypothesis | Use a stated assumption of the theorem |
| `local_assume` | introduce local hypothesis | Begin a subproof with an assumption |
| `local_discharge` | conclude from local hypothesis | End a subproof, discharging the assumption |

Each node's `context` field references the definitions and assumptions it uses, while the `dependencies` field references other nodes it builds upon.

---

## Node States

Nodes in AF have three orthogonal state dimensions: workflow state, epistemic state, and taint state. Understanding these is essential for understanding how proofs progress.

### Workflow States

Workflow state controls which agents can work on a node:

| State | Description |
|-------|-------------|
| `available` | The node is free for any agent to claim and work on. |
| `claimed` | An agent has exclusive access to this node. Other agents cannot modify it. |
| `blocked` | The node cannot be worked on, typically because it awaits a pending definition. |

Valid workflow state transitions:
```
available --> claimed    (agent claims the node)
claimed --> available    (agent releases the node)
claimed --> blocked      (dependency blocks progress)
blocked --> available    (blocker resolved)
```

The claim system prevents concurrent modification:
- Only one agent can claim a node at a time
- Claims are tracked with agent ID and timestamp
- Stale claims can be reaped by the `af reap` command

### Epistemic States

Epistemic state reflects the verification status of a node's content:

| State | Description | Terminal? |
|-------|-------------|-----------|
| `pending` | Not yet evaluated by a verifier. | No |
| `validated` | Accepted by a verifier as correct. | Yes |
| `admitted` | Accepted without full verification (escape hatch). Introduces taint. | Yes |
| `refuted` | Proven to be false. | Yes |
| `archived` | Abandoned as a proof strategy, preserved for history. | Yes |

Key points about epistemic states:

1. **Only `pending` can transition**: Once a node reaches any other state, it is terminal.

2. **Validation requires explicit acceptance**: A verifier must explicitly call `af accept` to validate a node.

3. **Admitted introduces taint**: This is an escape hatch for claims that are "obviously true" but tedious to prove fully. The epistemic cost is tracked via taint propagation.

4. **History is preserved**: Refuted and archived nodes are kept to maintain a complete record of the proof's development, including failed approaches.

### Taint States

Taint tracks epistemic uncertainty that propagates through the proof tree:

| State | Description |
|-------|-------------|
| `clean` | All ancestors are validated. This node has full epistemic certainty. |
| `self_admitted` | This node itself was admitted without proof. |
| `tainted` | Some ancestor was admitted, so this node inherits that uncertainty. |
| `unresolved` | Some ancestor is still pending, so the chain is incomplete. |

Taint is computed, not directly set. The computation follows these rules (in order):

1. If the node is pending, return `unresolved`
2. If any ancestor is unresolved, return `unresolved`
3. If the node's epistemic state is `admitted`, return `self_admitted`
4. If any ancestor is tainted or self_admitted, return `tainted`
5. Otherwise, return `clean`

A proof is fully verified only when the root node is `validated` with taint `clean`.

---

## Challenges

### Why Challenges Exist

Challenges are the mechanism by which verifiers express objections to proof nodes. Rather than simply rejecting a node, a verifier raises a challenge that specifies what is wrong and gives the prover an opportunity to respond.

Challenges serve several purposes:

1. **Precision**: The verifier must articulate exactly what is wrong.
2. **Dialogue**: Provers can respond with additional justification.
3. **Auditability**: The challenge-response history is preserved.
4. **Granularity**: Multiple independent issues can be raised and tracked separately.

### Challenge Targets

Each challenge identifies what aspect of the node is being challenged:

| Target | Description |
|--------|-------------|
| `statement` | The claim text itself is disputed or unclear. |
| `inference` | The inference type is inappropriate for this step. |
| `context` | Referenced definitions or assumptions are wrong or missing. |
| `dependencies` | Node dependencies (other steps referenced) are incorrect. |
| `scope` | Scope violation: using an assumption that is not in scope. |
| `gap` | Logical gap: unstated substeps are required. |
| `type_error` | Type mismatch in mathematical objects. |
| `domain` | Unjustified domain restriction (e.g., assuming x > 0). |
| `completeness` | Incomplete case analysis or missing cases. |

A single challenge can target multiple aspects (e.g., both `inference` and `gap`).

### Challenge Severity

Challenges have severity levels that determine their impact on the workflow:

| Severity | Description | Blocks Acceptance? |
|----------|-------------|-------------------|
| `critical` | Fundamental error that must be fixed. | Yes |
| `major` | Significant issue that should be addressed. | Yes |
| `minor` | Minor issue that could be improved. | No |
| `note` | Clarification request or suggestion. | No |

**Blocking challenges** (critical, major) prevent a node from being accepted until they are resolved or withdrawn. The prover must address them.

**Non-blocking challenges** (minor, note) are advisory. The verifier can accept the node even with these open, though they remain visible for future consideration.

### Challenge States

| State | Description |
|-------|-------------|
| `open` | Awaiting response from prover. |
| `resolved` | Verifier is satisfied by the prover's response. |
| `withdrawn` | Verifier retracted the challenge (determined it was wrong). |

A node can only be validated when all blocking challenges are resolved or withdrawn.

### Resolving Challenges

The typical flow for resolving a challenge:

1. Verifier raises challenge with specific objection
2. Node becomes a prover job (if blocking challenge)
3. Prover claims the node and adds child nodes addressing the challenge
4. Prover releases the node
5. Node becomes a verifier job again
6. Verifier reviews the response and either:
   - Resolves the challenge (satisfied), or
   - Raises additional challenges
7. Once all blocking challenges are resolved, verifier can accept

---

## Definitions and Lemmas

### How Definitions Work

Definitions provide the formal meaning of terms used in the proof. They are:

- **Immutable**: Once added, a definition cannot be changed.
- **Global**: Definitions are available to all nodes in the proof.
- **Content-hashed**: Each definition has a hash computed from its content, enabling integrity verification.

Definitions are typically seeded at proof initialization but can be added during the proof via `af request-def` (prover requests) and `af def-add` (human operator adds).

When a node uses a definition, it lists the definition ID in its `context` field. Verifiers check that the referenced definitions actually support the claim being made.

### Lemma Extraction

Once a subtree of the proof is validated, it may be extracted as a reusable lemma. A lemma captures a proven fact that can be referenced by other parts of the proof (or other proofs entirely).

Lemma extraction has strict requirements:

1. All nodes in the subtree must be `validated`
2. All internal dependencies must be satisfied within the subtree
3. Only the root of the subtree can be referenced from outside
4. All scopes opened within the subtree must be closed within it

This ensures that a lemma is a self-contained, proven fact.

### Dependency Tracking

AF tracks two types of dependencies:

1. **Reference Dependencies** (`dependencies` field): Nodes that this step logically builds upon. These represent "I use the result of node X in my reasoning."

2. **Validation Dependencies** (`validation_deps` field): Nodes that must be validated before this node can be accepted. This enables cross-branch dependency tracking.

Both types are subject to validation:
- Dependencies must exist
- Dependencies must be ancestors or siblings (no forward references)
- Circular dependencies are forbidden

---

## Taint Propagation

### What is Epistemic Taint?

Epistemic taint is a marker of uncertainty that propagates through the proof tree. When a node is "admitted" (accepted without full proof), it introduces epistemic uncertainty: we are taking this claim on faith rather than proof.

This uncertainty must propagate to all nodes that depend on the admitted node. If node B depends on node A, and A is admitted, then B inherits that uncertainty even if B is fully validated.

Taint answers the question: "If we were to verify this proof with maximum rigor, how much work remains?"

### How Taint Spreads

Taint propagates along the dependency tree. The computation is performed from root toward leaves, ensuring parents are computed before children.

The algorithm:

```
function ComputeTaint(node, ancestors):
    if node.epistemic_state == pending:
        return unresolved

    for ancestor in ancestors:
        if ancestor.taint == unresolved:
            return unresolved

    if node.epistemic_state == admitted:
        return self_admitted

    for ancestor in ancestors:
        if ancestor.taint in [tainted, self_admitted]:
            return tainted

    return clean
```

When a node's epistemic state changes, taint is recomputed for that node and all its descendants.

### Why It Matters for Proof Integrity

Taint provides a nuanced view of proof quality:

- **clean**: Full confidence. Every step is verified.
- **unresolved**: Work in progress. Some steps await verification.
- **self_admitted**: This specific step was taken on faith.
- **tainted**: Depends on faith-based steps, even if this step is verified.

A proof with a validated root but `tainted` status is complete in one sense (someone has verified the reasoning), but epistemically weaker than a `clean` proof.

This allows AF to accommodate pragmatic trade-offs:
- Admit well-known results that are tedious to prove
- Track the epistemic cost of those admissions
- Distinguish between "verified" and "verified with assumptions"

---

## Scope and Assumptions

### Local vs Global Assumptions

AF distinguishes two types of assumptions:

**Global Assumptions** are hypotheses of the theorem being proved. For example, when proving "All primes greater than 2 are odd," the assumption "p > 2 and p is prime" is global. These are declared at proof initialization and available to all nodes.

**Local Assumptions** are temporary hypotheses introduced during the proof for techniques like:
- Proof by contradiction: "Assume the negation..."
- Proof by cases: "In case A..." / "In case B..."
- Conditional reasoning: "Suppose X..."

Local assumptions are introduced by `local_assume` nodes and discharged by `local_discharge` nodes.

### Scope Tracking in Proofs

When a `local_assume` node is created, it opens a scope. All descendant nodes exist within that scope and may use the local assumption in their reasoning.

```
1.1     local_assume "Suppose p = 2"     scope: []         opens: "1.1"
1.1.1   claim "Then p is even"           scope: ["1.1"]    (can use "p = 2")
1.1.2   claim "But p > 2 contradicts"    scope: ["1.1"]    (can use "p = 2")
1.1.3   local_discharge "Contradiction"  scope: []         closes: "1.1"
```

Key scope rules:

1. **Scope Inheritance**: Child nodes inherit their parent's active scopes (unless discharging).

2. **Scope Containment**: A node can only reference dependencies whose scopes are contained within the node's own scope.

3. **Scope Closure**: Every `local_assume` must have a corresponding `local_discharge` among its descendants.

4. **Validation Requirement**: A node with an open scope cannot be fully validated until the scope is closed.

### Scope Violations

The following would be a scope violation:

```
1.1     local_assume "Suppose p = 2"
1.1.1   claim "Then p is even"
1.1.2   local_discharge "Therefore..."      (closes scope)
1.2     claim "Since p = 2, we have..."     ERROR: p = 2 is no longer in scope!
```

Node `1.2` cannot reference the assumption from `1.1` because that scope was closed at `1.1.2`. The scope tracker enforces this invariant.

### Proof by Cases and Contradiction

AF handles case splits through multiple `local_assume` nodes:

```
1.1     case_split "Either p = 2 or p > 2"
1.1.1   local_assume "Case: p = 2"         (scope for case 1)
1.1.1.1 claim "..."
1.1.1.2 local_discharge "Done with case 1"
1.1.2   local_assume "Case: p > 2"         (scope for case 2)
1.1.2.1 claim "..."
1.1.2.2 local_discharge "Done with case 2"
1.1.3   qed "In all cases, conclusion holds"
```

For proof by contradiction:

```
1.1     local_assume "Suppose for contradiction that sqrt(2) is rational"
1.1.1   claim "Then sqrt(2) = p/q for integers p, q"
1.1.2   claim "..."
1.1.3   claim "But p and q must both be even, contradiction"
1.1.4   local_discharge "Therefore sqrt(2) is irrational"
```

The scope system ensures that assumptions from one branch of a case split cannot "leak" into another, and that conclusions drawn under a contradictory assumption are properly bounded.

---

## Summary

AF provides a rigorous framework for collaborative proof construction through:

1. **Adversarial Verification**: Separating provers (who construct) from verifiers (who critique) ensures robust error detection.

2. **Hierarchical Structure**: Proofs are organized as trees with tool-controlled ID assignment.

3. **Multi-dimensional State**: Workflow, epistemic, and taint states provide complete visibility into proof progress.

4. **Challenge-Response Dialogue**: Verifiers raise specific challenges; provers respond; this continues until acceptance.

5. **Taint Propagation**: Epistemic uncertainty from admitted claims is tracked through the dependency tree.

6. **Scope Management**: Local assumptions are tracked to prevent scope violations in case splits and contradiction proofs.

The result is a system where trust emerges from process, not from any single agent's judgment. Correct proofs survive adversarial scrutiny; flawed proofs are challenged and refined.
