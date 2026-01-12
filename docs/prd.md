# AF: Adversarial Proof Framework

## Product Requirements Document v3

### Overview

AF is a command-line tool for collaborative construction of natural-language mathematical proofs. Multiple AI agents work concurrently as adversarial provers and verifiers, refining proof steps until rigorous acceptance. Proofs follow Lamport's structured proof notation with hierarchical numbering.

**What AF is**: An event-sourced adversarial workflow engine for natural-language formal proofs. It provides procedural rigor, not semantic soundness. It is a socio-technical protocol for producing machine-auditable proof trees.

**What AF is not**: A proof assistant, logic kernel, or theorem prover. It does not compete with Lean or Coq. It preserves failed lines, disagreements, and full history — functioning as a scientific record, not just a proof.

### Core Principles

1. **Adversarial verification**: Provers convince, verifiers attack. No agent plays both roles.
2. **Agent isolation**: Each agent spawns fresh, claims one job, works, terminates. No context bleeding.
3. **Append-only truth**: Ledger is source of truth. Current state is derived. Full history preserved including rejected paths.
4. **Filesystem concurrency**: ACID guarantees via POSIX atomics. No database server.
5. **Tool-controlled structure**: Hierarchical IDs, child assignment, and state transitions are enforced by the tool, never by agents.
6. **Verifier control**: Verifiers explicitly control all acceptance decisions. No automatic state transitions.
7. **Serialized writes**: Ledger writes are serialized. No gaps in sequence numbers.
8. **Taint propagation**: Epistemic uncertainty propagates through dependencies.
9. **Self-documenting CLI**: The tool provides complete context; agents need no external documentation.

---

### Operational Model

| Role | Responsibility |
|------|----------------|
| **Orchestrator** | Automated loop. Polls `af jobs`, spawns agents, runs `af reap`. Terminates when no jobs. |
| **Agents** | Ephemeral. Claim, read, act, release/terminate. No state across invocations. |
| **Human** | Supervises. Resolves definition requests, archives stuck branches, applies escape hatches, declares victory or defeat. |

---

### Data Model

#### Node

A proof step in the hierarchical structure.

```json
{
  "id": "1.2.1",
  "parent": "1.2",
  "type": "claim",
  "statement": "Since p is prime and p = 2k, we have 2 | p, so 2 ∈ {1, p}.",
  "latex": "\\text{Since } p \\text{ is prime and } p = 2k...",
  "inference": "by_definition",
  "context": ["DEF-prime", "DEF-divides"],
  "dependencies": ["1.2", "1.1"],
  "scope": ["1.1.A"],
  "addresses_challenges": ["ch-002"],
  
  "content_hash": "a3f2b1c7d8e9...",
  
  "workflow_state": "available",
  "epistemic_state": "pending",
  "taint": "clean",
  
  "created_by": "agent-12",
  "created_at": "2025-01-11T10:05:00Z",
  
  "challenges": [
    {
      "id": "ch-003",
      "by": "agent-15",
      "at": "2025-01-11T10:06:00Z",
      "objection": "You assume 2 | p without justification.",
      "targets": ["inference"],
      "state": "open",
      "addressed_by": []
    }
  ],
  
  "children": ["1.2.1.1"],
  
  "validated_by": null,
  "validated_at": null,
  "admitted_by": null,
  "admitted_reason": null,
  "refuted_by": null,
  "refuted_reason": null,
  "archived_by": null,
  "archived_reason": null
}
```

#### Node Types

| Type | Meaning |
|------|---------|
| `claim` | A mathematical assertion to be justified |
| `local_assume` | Introduce a local hypothesis (opens scope) |
| `local_discharge` | Conclude from local hypothesis (closes scope) |
| `case` | One branch of a case split |
| `qed` | Final step concluding the proof or subproof |

#### Scope Tracking

Scopes track which local assumptions are active at each node.

- `local_assume` nodes create a scope entry: `"{node_id}.A"`
- `local_discharge` nodes must reference the scope entry they close
- A node's `scope` field lists all active scope entries it depends on
- Child nodes inherit parent's scope unless they discharge

Example:
```
1.1     local_assume "Suppose p = 2"        scope: []         creates: "1.1.A"
1.1.1   claim "Then p is even"              scope: ["1.1.A"]
1.1.2   local_discharge "Contradicts odd"   scope: []         closes: "1.1.A"
```

**Invariant**: A node cannot reference dependencies whose scope entries are not in the node's own scope.

#### Content Hash

Computed as: `sha256(type + statement + latex + inference + sorted(context) + sorted(dependencies))`

Used for:
- Integrity verification during replay
- Detecting silent mutations
- Future: content-addressed storage

#### Workflow State

| State | Meaning |
|-------|---------|
| `available` | Open for prover or verifier work |
| `claimed` | Agent holds exclusive access |
| `blocked` | Awaiting definition resolution |

#### Epistemic State

| State | Meaning |
|-------|---------|
| `pending` | Not yet evaluated |
| `validated` | Verifier accepted |
| `admitted` | Assumed without proof (escape hatch) |
| `refuted` | Proven false |
| `archived` | Abandoned strategy, kept for history |

#### Taint

Computed property reflecting epistemic uncertainty propagation.

| Taint | Meaning |
|-------|---------|
| `clean` | All ancestors validated |
| `self_admitted` | This node is admitted |
| `tainted` | Some ancestor is admitted or tainted |
| `unresolved` | Some ancestor is pending |

Taint computation:
```
if node.epistemic_state == admitted:
    taint = self_admitted
elif any(ancestor.taint in [self_admitted, tainted] for ancestor in dependencies):
    taint = tainted
elif any(ancestor.epistemic_state == pending for ancestor in dependencies):
    taint = unresolved
else:
    taint = clean
```

Taint is recomputed on every status change and propagated to descendants.

#### Challenge State

| State | Meaning |
|-------|---------|
| `open` | Awaiting response |
| `resolved` | Verifier satisfied by addressing nodes |
| `withdrawn` | Verifier retracted |
| `superseded` | Parent archived/refuted, challenge moot |

#### Validation Invariant

A node `n` may be validated if and only if:

1. All challenges on `n` have `state ∈ {resolved, withdrawn, superseded}`
2. For each challenge with `state = resolved`, at least one node in `addressed_by` has `epistemic_state = validated`
3. All children of `n` have `epistemic_state ∈ {validated, admitted}`
4. All scope entries opened by `n` (if `local_assume`) are closed by a descendant

The `af accept` command enforces this invariant.

#### Definition

Globally registered, immutable, seeded at initialization.

```json
{
  "id": "DEF-prime",
  "name": "prime",
  "latex": "p \\text{ is prime} \\iff p > 1 \\land \\forall a,b: ab = p \\implies (a = 1 \\lor b = 1)",
  "source": "Euclid, Elements VII",
  "content_hash": "b4e5f6...",
  "created_by": "init",
  "created_at": "2025-01-11T09:00:00Z"
}
```

#### External Reference

For citing published results.

```json
{
  "id": "EXT-001",
  "doi": "10.1000/example",
  "claimed_statement": "Every continuous function on [a,b] is bounded.",
  "verification_status": "pending",
  "verified_statement": null,
  "bibdata": null,
  "content_hash": "c5f6a7...",
  "created_by": "agent-12",
  "created_at": "2025-01-11T10:00:00Z"
}
```

Verification statuses:
- `pending`: Not yet checked
- `verified`: DOI exists, statement matches
- `mismatch`: DOI exists, statement differs
- `not_found`: Cannot locate reference
- `metadata_only`: DOI exists but full text inaccessible

#### Assumption

Global hypothesis for the conjecture.

```json
{
  "id": "ASM-p-greater-2",
  "name": "p > 2",
  "latex": "p > 2",
  "source": "hypothesis",
  "content_hash": "d6a7b8...",
  "created_by": "init",
  "created_at": "2025-01-11T09:00:00Z"
}
```

#### Schema

Allowed inference types.

```json
{
  "inferences": [
    {"id": "modus_ponens", "name": "Modus Ponens", "form": "P, P → Q ⊢ Q"},
    {"id": "modus_tollens", "name": "Modus Tollens", "form": "¬Q, P → Q ⊢ ¬P"},
    {"id": "universal_instantiation", "name": "Universal Instantiation", "form": "∀x.P(x) ⊢ P(t)"},
    {"id": "existential_instantiation", "name": "Existential Instantiation", "form": "∃x.P(x) ⊢ P(c) for fresh c"},
    {"id": "universal_generalization", "name": "Universal Generalization", "form": "P(x) for arbitrary x ⊢ ∀x.P(x)"},
    {"id": "existential_generalization", "name": "Existential Generalization", "form": "P(c) ⊢ ∃x.P(x)"},
    {"id": "by_definition", "name": "By Definition", "form": "unfold definition"},
    {"id": "assumption", "name": "Assumption", "form": "global hypothesis"},
    {"id": "local_assume", "name": "Local Assumption", "form": "introduce local hypothesis"},
    {"id": "local_discharge", "name": "Local Discharge", "form": "conclude from local hypothesis"},
    {"id": "contradiction", "name": "Contradiction", "form": "P ∧ ¬P ⊢ ⊥"},
    {"id": "case_split", "name": "Case Split", "form": "P ∨ Q, P ⊢ R, Q ⊢ R ⊢ R"},
    {"id": "induction_base", "name": "Induction Base", "form": "P(0)"},
    {"id": "induction_step", "name": "Induction Step", "form": "P(n) → P(n+1)"},
    {"id": "direct_computation", "name": "Direct Computation", "form": "arithmetic or algebraic simplification"},
    {"id": "substitution", "name": "Substitution", "form": "a = b, P(a) ⊢ P(b)"},
    {"id": "conjunction_intro", "name": "Conjunction Introduction", "form": "P, Q ⊢ P ∧ Q"},
    {"id": "conjunction_elim", "name": "Conjunction Elimination", "form": "P ∧ Q ⊢ P"},
    {"id": "disjunction_intro", "name": "Disjunction Introduction", "form": "P ⊢ P ∨ Q"},
    {"id": "disjunction_elim", "name": "Disjunction Elimination", "form": "P ∨ Q, P → R, Q → R ⊢ R"},
    {"id": "implication_intro", "name": "Implication Introduction", "form": "P ⊢ Q under P ⊢ P → Q"},
    {"id": "external_application", "name": "External Application", "form": "apply cited result"},
    {"id": "lemma_application", "name": "Lemma Application", "form": "apply extracted lemma"},
    {"id": "qed", "name": "QED", "form": "proof complete"}
  ]
}
```

#### Challenge Targets

| Target | Meaning |
|--------|---------|
| `statement` | The claim itself is unclear or wrong |
| `inference` | The reasoning step is invalid |
| `context` | Missing or incorrect definition/assumption references |
| `dependencies` | Missing or incorrect step references |
| `scope` | Scope violation (using out-of-scope assumption) |
| `gap` | Unstated substeps required |
| `type_error` | Type mismatch in mathematical content |
| `domain` | Unjustified domain restriction |
| `completeness` | Incomplete case analysis or enumeration |

---

### Filesystem Structure

```
proof/
  meta.json
  
  ledger/
    000001-1736600000000-proof_initialized.json
    000002-1736600000001-node_created.json
    ...
  
  ledger.lock
  
  nodes/
    1.json
    1.1.json
    ...
  
  locks/
    1.1.lock
    ...
  
  defs/
    DEF-prime.json
    ...
  
  external/
    EXT-001.json
    ...
  
  pending-defs/
    REQ-001.json
    ...
  
  assumptions/
    ASM-p-greater-2.json
    ...
  
  lemmas/
    LEM-001.json
    ...
  
  schema.json
```

#### Concurrency Guarantees

- **Node locks**: `O_CREAT | O_EXCL`. Contains agent ID and timestamp.
- **Node writes**: Write to `.tmp`, then `rename()`.
- **Ledger serialization**: `ledger.lock` acquired before sequence assignment and write.
- **Stale locks**: Configurable timeout. `af reap` clears expired.
- **Child ID assignment**: Atomic lock acquisition on candidate IDs.

---

### Ledger Events

```json
{
  "seq": 1235,
  "type": "NodeValidated",
  "observed_seq": 1230,
  "timestamp": "2025-01-11T10:07:00Z",
  "by": "agent-20",
  "payload": {
    "node": "1.2.1"
  }
}
```

#### Event Types

```
ProofInitialized       { conjecture, context, assumptions }
NodeCreated            { id, parent, type, statement, inference, context, dependencies, scope, addresses_challenges, content_hash }
NodesClaimed           { ids[], role }
NodesReleased          { ids[] }
ChallengeRaised        { node, challenge_id, objection, targets }
ChallengeResolved      { node, challenge_id }
ChallengeWithdrawn     { node, challenge_id }
NodeValidated          { id }
NodeAdmitted           { id, reason }
NodeRefuted            { id, reason }
NodeArchived           { id, reason }
TaintRecomputed        { nodes[], old_taints[], new_taints[] }
DefRequested           { request_id, name, latex, source }
DefAdded               { id, name, latex, source, content_hash }
DefRequestRejected     { request_id, reason }
ExternalRefAdded       { id, doi, claimed_statement, content_hash }
ExternalRefVerified    { id, status, verified_statement, bibdata }
LemmaExtracted         { id, name, statement, root_node, nodes[], content_hash }
AssumptionAdded        { id, name, latex, source, content_hash }
LockReaped             { node, original_agent }
```

---

### CLI Self-Documentation Requirements

The `af` command must be fully self-documenting. An agent with no prior knowledge of AF, given only access to the CLI, must be able to operate correctly. This is non-negotiable.

#### Principles

1. **Zero external documentation required**: The CLI is the complete interface specification.
2. **Forgiving input**: Accept misspellings, reordered arguments, alternative phrasings.
3. **Guided workflow**: Every command output suggests logical next steps.
4. **Role-specific context**: When an agent claims a job, the CLI provides everything needed to complete that job.

#### Fuzzy Command Matching

The CLI accepts approximate commands and suggests corrections:

```
$ af chalenge 1.2.1
Unknown command 'chalenge'. Did you mean 'challenge'?

$ af challenge 1.2.1 --agnet prover-42
Unknown flag '--agnet'. Did you mean '--agent'?
```

When unambiguous, auto-correct and proceed:

```
$ af stauts
(Interpreting as 'status')

[status output follows]
```

#### Argument Order Independence

All commands accept arguments in any order:

```bash
# These are equivalent:
af refine 1.2 --statement "..." --agent prover-1 --inference modus_ponens
af refine --agent prover-1 --inference modus_ponens --statement "..." 1.2
af refine --inference modus_ponens 1.2 --statement "..." --agent prover-1
```

#### Missing Argument Prompting

When required arguments are missing, prompt with explanation:

```
$ af refine 1.2
Missing required arguments for 'refine':

  --statement TEXT    The mathematical claim (required)
  --inference TYPE    Inference rule used (required)
  --agent ID          Your agent identifier (required)

Optional arguments:
  --type TYPE         Node type [claim|local_assume|local_discharge|case|qed] (default: claim)
  --latex TEXT        LaTeX rendering of statement
  --context IDS       Comma-separated definition/assumption IDs
  --dependencies IDS  Comma-separated node IDs this step depends on
  --addresses IDS     Comma-separated challenge IDs this addresses
  --discharges ID     Scope entry to close (for local_discharge)
  --children FILE     JSON file for multi-child creation

Run 'af refine --help' for full documentation.
Run 'af schema' to see valid inference types.
```

#### Contextual Help

Every command has `--help` with examples:

```
$ af challenge --help
Usage: af challenge <node-id> [options]

Raise a challenge against a proof step. Requires claim on the node.

Required:
  --objection TEXT    Specific issue with the step
  --targets TYPES     Comma-separated: statement,inference,context,dependencies,
                      scope,gap,type_error,domain,completeness
  --agent ID          Your agent identifier

Example:
  af challenge 1.2.1 \
    --objection "Claim uses p > 0 but only p > 2 established" \
    --targets domain,scope \
    --agent verifier-42

After challenging, the node remains available for provers to address.
Run 'af jobs --role prover' to see nodes needing attention.
```

#### Next Step Suggestions

Every command output ends with contextually relevant next steps:

```
$ af refine 1.2 --statement "..." --inference by_definition --agent prover-1

Created node 1.2.1
  Type: claim
  Status: pending
  Taint: unresolved

Next steps:
  • This node needs verification. A verifier should run:
      af claim 1.2.1 --role verifier --agent <agent-id>
  • To add more children to 1.2:
      af claim 1.2 --role prover --agent <agent-id>
  • To see current proof state:
      af status
```

```
$ af accept 1.2.1 --agent verifier-1

Error: VALIDATION_INVARIANT_FAILED

Cannot accept node 1.2.1:
  ✗ Challenge ch-003 has state 'open' (must be resolved/withdrawn/superseded)
  ✓ All children validated

To proceed, either:
  • Resolve the challenge (if addressing nodes are satisfactory):
      af resolve-challenge 1.2.1 --challenge ch-003 --agent verifier-1
  • Withdraw the challenge (if no longer valid):
      af withdraw-challenge 1.2.1 --challenge ch-003 --agent verifier-1
  • Wait for prover to address the challenge:
      af jobs --role prover
```

#### Claim Context Provision

When an agent claims a job, the CLI outputs everything needed to complete the job.

**Prover claim output:**

```
$ af claim 1.2.1 --role prover --agent prover-42

Claimed node 1.2.1 for prover work.

═══════════════════════════════════════════════════════════════════════════════
NODE CONTEXT FOR PROVER
═══════════════════════════════════════════════════════════════════════════════

TARGET NODE: 1.2.1
  Statement: Since p is prime and p = 2k, we have 2 | p
  Type: claim
  Status: pending
  Inference: by_definition
  Context: DEF-prime, DEF-divides
  Dependencies: 1.2, 1.1
  Scope: [1.1.A]
  Taint: unresolved

OPEN CHALLENGES (must address):
  ch-003: "You assume 2 | p without justification."
          Target: inference
          Raised by: agent-15 at 2025-01-11T10:06:00Z
          Addressed by: (none)

ANCESTOR CHAIN:
  1    [validated] All primes greater than 2 are odd
  1.2  [pending]   Let p > 2 be prime. Suppose p is even.
    → 1.2.1 (you are here)

ACTIVE SCOPE:
  1.1.A: "Assume p is even" (from node 1.1, type: local_assume)

AVAILABLE DEFINITIONS:
  DEF-prime: p is prime ⟺ p > 1 ∧ ∀a,b: ab = p ⟹ (a = 1 ∨ b = 1)
  DEF-divides: a | b ⟺ ∃k: b = ak
  DEF-even: n is even ⟺ 2 | n
  DEF-odd: n is odd ⟺ ∃k: n = 2k + 1

AVAILABLE ASSUMPTIONS:
  ASM-p-gt-2: p > 2

VALID INFERENCES:
  modus_ponens, modus_tollens, universal_instantiation, existential_instantiation,
  universal_generalization, existential_generalization, by_definition, assumption,
  local_assume, local_discharge, contradiction, case_split, induction_base,
  induction_step, direct_computation, substitution, conjunction_intro,
  conjunction_elim, disjunction_intro, disjunction_elim, implication_intro,
  external_application, lemma_application, qed

═══════════════════════════════════════════════════════════════════════════════
YOUR TASK
═══════════════════════════════════════════════════════════════════════════════

Address challenge ch-003 by adding child node(s) that justify the inference.

Output JSON in this format:
{
  "action": "refine",
  "parent": "1.2.1",
  "children": [
    {
      "type": "claim",
      "statement": "Your mathematical claim",
      "latex": "LaTeX version",
      "inference": "inference_type",
      "context": ["DEF-xxx"],
      "dependencies": ["1.2.1"],
      "addresses_challenges": ["ch-003"]
    }
  ],
  "rationale": "Brief explanation of approach"
}

Or request a definition:
{
  "action": "request_def",
  "name": "definition_name",
  "latex": "LaTeX definition",
  "source": "source reference",
  "rationale": "Why needed"
}

═══════════════════════════════════════════════════════════════════════════════
AFTER COMPLETION
═══════════════════════════════════════════════════════════════════════════════

Parse your JSON output and run:
  af refine 1.2.1 --children <file.json> --agent prover-42

Or for single child:
  af refine 1.2.1 --statement "..." --inference "..." --context "..." \
     --dependencies "..." --addresses ch-003 --agent prover-42

Your claim will be released automatically after successful refine.
If you need to abort: af release 1.2.1 --agent prover-42
```

**Verifier claim output:**

```
$ af claim 1.2.1 --role verifier --agent verifier-42

Claimed node 1.2.1 for verifier work.

═══════════════════════════════════════════════════════════════════════════════
NODE CONTEXT FOR VERIFIER
═══════════════════════════════════════════════════════════════════════════════

TARGET NODE: 1.2.1
  Statement: Since p is prime and p = 2k, we have 2 | p
  Type: claim
  Status: pending
  Inference: by_definition
  Context: DEF-prime, DEF-divides
  Dependencies: 1.2, 1.1
  Scope: [1.1.A]
  Taint: unresolved
  Content hash: a3f2b1c7...

CHALLENGES ON THIS NODE:
  ch-003: "You assume 2 | p without justification."
          Target: inference
          State: open
          Addressed by:
            1.2.1.1 [validated] "By definition of even, p = 2k implies 2 | p"

CHILDREN:
  1.2.1.1 [validated] By definition of even, p = 2k implies 2 | p
            Inference: by_definition
            Context: DEF-even, DEF-divides
            Addresses: ch-003

ANCESTOR CHAIN:
  1    [validated] All primes greater than 2 are odd
  1.2  [pending]   Let p > 2 be prime. Suppose p is even.
    → 1.2.1 (you are here)

ACTIVE SCOPE:
  1.1.A: "Assume p is even" (from node 1.1)

REFERENCED DEFINITIONS:
  DEF-prime: p is prime ⟺ p > 1 ∧ ∀a,b: ab = p ⟹ (a = 1 ∨ b = 1)
  DEF-divides: a | b ⟺ ∃k: b = ak

VALIDATION CHECKLIST:
  ✓ All children validated
  ? Challenge ch-003: addressed by 1.2.1.1 (validated) — you must resolve or reject

═══════════════════════════════════════════════════════════════════════════════
YOUR TASK
═══════════════════════════════════════════════════════════════════════════════

Verify this node. Check for:

STRUCTURAL (reject if found):
  • Dependencies reference undefined nodes
  • Scope violations (using discharged assumptions)
  • Invalid inference type
  • Circular dependencies

SEMANTIC (challenge if found):
  • Claim does not follow from dependencies
  • Inference rule misapplied
  • Hidden quantifiers or type errors
  • Domain restrictions without justification (e.g., assuming x > 0)
  • Incomplete case analysis
  • Sign errors in inequalities

Output JSON in this format:

Accept (if valid):
{
  "action": "accept",
  "node": "1.2.1",
  "resolved_challenges": ["ch-003"],
  "rationale": "Child 1.2.1.1 correctly justifies the divisibility claim"
}

Challenge (if semantic issue):
{
  "action": "challenge",
  "node": "1.2.1",
  "objection": "Specific issue description",
  "targets": ["inference", "domain"],
  "rationale": "Why this is wrong"
}

Reject (if structural error):
{
  "action": "reject",
  "node": "1.2.1",
  "reason": "Structural problem description",
  "targets": ["dependencies"]
}

═══════════════════════════════════════════════════════════════════════════════
AFTER COMPLETION
═══════════════════════════════════════════════════════════════════════════════

For accept:
  af resolve-challenge 1.2.1 --challenge ch-003 --agent verifier-42
  af accept 1.2.1 --agent verifier-42

For challenge:
  af challenge 1.2.1 --objection "..." --targets "..." --agent verifier-42

Then release:
  af release 1.2.1 --agent verifier-42
```

#### Status Output with Guidance

```
$ af status

═══════════════════════════════════════════════════════════════════════════════
PROOF STATUS: All primes greater than 2 are odd
═══════════════════════════════════════════════════════════════════════════════

1 [validated] [clean] All primes greater than 2 are odd
├─ 1.1 [validated] [clean] Assume p > 2 is prime
│  └─ 1.1.1 [validated] [clean] Suppose for contradiction p is even
│     ├─ 1.1.1.1 [validated] [clean] Then p = 2k for some k
│     ├─ 1.1.1.2 [pending] [unresolved] (!) Since p is prime and 2 | p...
│     │  └─ 1.1.1.2.1 [validated] [clean] By def of even, 2 | p
│     └─ 1.1.1.3 [pending] [unresolved] Therefore p = 2, contradicting p > 2
└─ 1.2 [archived] [clean] (Alternative approach, abandoned)

LEGEND:
  [validated] = accepted by verifier    [pending] = awaiting verification
  [admitted] = assumed without proof    [archived] = abandoned
  [clean] = all ancestors validated     [tainted] = depends on admitted
  [unresolved] = ancestors pending      (!) = has open challenges

SUMMARY:
  Nodes: 8 total (5 validated, 2 pending, 1 archived)
  Challenges: 1 open
  Taint: 0 tainted, 2 unresolved
  Depth: 4 / 20 max

BLOCKING ISSUES:
  • Node 1.1.1.2 has open challenge ch-003 (needs prover or verifier attention)

NEXT STEPS:
  • 1 prover job available:  af jobs --role prover
  • 1 verifier job available: af jobs --role verifier
  • To see blocking challenge: af get 1.1.1.2 --challenges
```

#### Jobs Output with Instructions

```
$ af jobs --role prover

═══════════════════════════════════════════════════════════════════════════════
PROVER JOBS AVAILABLE
═══════════════════════════════════════════════════════════════════════════════

NODE 1.1.1.2
  Statement: Since p is prime and 2 | p, we have 2 ∈ {1, p}
  Reason: Has open challenge needing response
  Challenge: ch-003 "You assume 2 | p without justification"

  To work on this:
    af claim 1.1.1.2 --role prover --agent <your-agent-id>

───────────────────────────────────────────────────────────────────────────────

Total: 1 prover job

To claim a job, run the 'af claim' command shown above.
The claim output will provide full context for completing the task.
```

```
$ af jobs --role verifier

═══════════════════════════════════════════════════════════════════════════════
VERIFIER JOBS AVAILABLE
═══════════════════════════════════════════════════════════════════════════════

NODE 1.1.1.2
  Statement: Since p is prime and 2 | p, we have 2 ∈ {1, p}
  Reason: Challenge ch-003 has been addressed, ready for resolution
  Children: 1.1.1.2.1 [validated]
  Pending challenges: ch-003 (addressed by 1.1.1.2.1)

  To work on this:
    af claim 1.1.1.2 --role verifier --agent <your-agent-id>

NODE 1.1.1.3
  Statement: Therefore p = 2, contradicting p > 2
  Reason: No challenges, leaf node ready for acceptance
  Children: (none)

  To work on this:
    af claim 1.1.1.3 --role verifier --agent <your-agent-id>

───────────────────────────────────────────────────────────────────────────────

Total: 2 verifier jobs

To claim a job, run the 'af claim' command shown above.
The claim output will provide full context for completing the task.
```

#### Error Messages with Recovery

```
$ af refine 1.2.1 --statement "..." --inference magic --agent prover-1

Error: INVALID_INFERENCE

'magic' is not a valid inference type.

Valid inferences:
  modus_ponens              P, P → Q ⊢ Q
  modus_tollens             ¬Q, P → Q ⊢ ¬P
  universal_instantiation   ∀x.P(x) ⊢ P(t)
  by_definition             unfold definition
  ... (run 'af schema' for full list)

Did you mean one of these?
  • direct_computation (for arithmetic/algebraic steps)
  • by_definition (for unfolding definitions)
  • assumption (for using hypotheses)
```

```
$ af challenge 1.2.1 --objection "Bad step" --targets wrong --agent v-1

Error: INVALID_TARGET

'wrong' is not a valid challenge target.

Valid targets:
  statement     The claim itself is unclear or wrong
  inference     The reasoning step is invalid
  context       Missing or incorrect definition/assumption references
  dependencies  Missing or incorrect step references
  scope         Scope violation (using out-of-scope assumption)
  gap           Unstated substeps required
  type_error    Type mismatch in mathematical content
  domain        Unjustified domain restriction
  completeness  Incomplete case analysis or enumeration

Multiple targets can be specified: --targets inference,domain
```

```
$ af refine 1.2.1 --statement "..." --inference by_definition --dependencies 1.9 --agent p-1

Error: INVALID_DEPENDENCY

Node '1.9' does not exist.

Available nodes that 1.2.1 can depend on:
  1       [validated] All primes greater than 2 are odd
  1.1     [validated] Assume p > 2 is prime  
  1.2     [pending]   Let p > 2 be prime. Suppose p is even.
  1.2.1   [pending]   (current node - cannot self-reference)

To see full node details: af get <node-id>
```

```
$ af claim 1.2.1 --role prover --agent prover-1

Error: ALREADY_CLAIMED

Node 1.2.1 is currently claimed by agent 'prover-99' (since 2025-01-11T10:05:00Z).

Options:
  • Wait for the agent to complete and release
  • If the agent has crashed, an operator can run:
      af reap --older-than 5m
  • Choose a different job:
      af jobs --role prover
```

#### Global Help

```
$ af

AF: Adversarial Proof Framework v1.0

Usage: af <command> [options]

Proof Management:
  init          Initialize a new proof
  status        Show proof tree with states and taint

Job Discovery:
  jobs          List available prover/verifier jobs
  pending-defs  List unresolved definition requests
  pending-refs  List unverified external references

Agent Operations:
  claim         Claim a node for work (provides full context)
  release       Release a claimed node
  
Prover Commands:
  refine        Add child node(s) to develop proof
  request-def   Request a new definition

Verifier Commands:
  challenge     Raise objection to a node
  resolve-challenge   Mark challenge as resolved
  withdraw-challenge  Retract a challenge
  accept        Validate a node (requires all invariants met)
  verify-external     Record external reference verification

Escape Hatches:
  admit         Accept node without full proof
  refute        Mark node as false
  archive       Abandon proof branch

Reference Data:
  get           Retrieve node with context
  defs          List definitions
  def           Show single definition
  assumptions   List assumptions
  assumption    Show single assumption
  externals     List external references
  external      Show single external reference
  lemmas        List extracted lemmas
  lemma         Show single lemma
  schema        Show valid inference types

Administration:
  log           Show event ledger
  replay        Rebuild state from ledger
  reap          Clear stale locks
  recompute-taint   Force taint recalculation
  def-add       Add definition (human operator)
  def-reject    Reject definition request (human operator)
  extract-lemma Extract reusable subproof

Run 'af <command> --help' for detailed usage of any command.

Quick Start:
  1. af init "Your theorem statement" --defs defs.json --assumptions assumptions.json
  2. af jobs --role prover
  3. af claim <node-id> --role prover --agent <agent-id>
  4. (follow instructions in claim output)
```

#### Machine-Readable Output

All commands support `--format json` for orchestrator consumption:

```
$ af jobs --role prover --format json
```

```json
{
  "jobs": [
    {
      "node_id": "1.1.1.2",
      "role": "prover",
      "reason": "open_challenge",
      "statement": "Since p is prime and 2 | p, we have 2 ∈ {1, p}",
      "challenges": ["ch-003"],
      "claim_command": "af claim 1.1.1.2 --role prover --agent <agent-id>"
    }
  ],
  "total": 1
}
```

```
$ af claim 1.2.1 --role prover --agent prover-1 --format json
```

```json
{
  "claimed": true,
  "node_id": "1.2.1",
  "role": "prover",
  "agent": "prover-1",
  "context": {
    "node": { },
    "challenges": [ ],
    "ancestors": [ ],
    "scope": [ ],
    "definitions": { },
    "assumptions": { },
    "valid_inferences": [ ]
  },
  "task": {
    "description": "Address challenge ch-003 by adding child node(s)",
    "output_format": "refine_json",
    "example": { }
  },
  "commands": {
    "refine": "af refine 1.2.1 --children <file> --agent prover-1",
    "release": "af release 1.2.1 --agent prover-1"
  }
}
```

---

### CLI Interface Reference

#### Initialization

```bash
af init "Theorem statement" \
   --defs defs.json \
   --assumptions assumptions.json \
   --schema schema.json \
   --external external.json
```

Creates proof directory, root node `1` with `type: claim`, `workflow_state: available`, `epistemic_state: pending`.

#### Job Discovery

```bash
af status                             # tree view with states and taint
af status --format json

af jobs                               # all available jobs
af jobs --role prover
af jobs --role verifier
af jobs --format json

af pending-defs                       # unresolved definition requests
af pending-refs                       # unverified external references
```

#### Claiming and Releasing

```bash
af claim <node-id> --role <role> --agent <agent-id>
af release <node-id> --agent <agent-id>
```

#### Reading State

```bash
af get <node-id>                      # node JSON
af get <node-id> --ancestors          # ancestor chain
af get <node-id> --subtree            # descendants
af get <node-id> --challenges         # open challenges highlighted
af get <node-id> --context            # referenced defs, assumptions, externals
af get <node-id> --scope              # active scope entries
af get <node-id> --full               # all of the above

af defs
af def <def-id>
af assumptions
af assumption <asm-id>
af externals
af external <ext-id>
af lemmas
af lemma <lemma-id>
af schema
```

#### Prover Actions

```bash
af refine <parent-id> \
   --type claim \
   --statement "..." \
   --latex "..." \
   --inference modus_ponens \
   --context DEF-x,DEF-y \
   --dependencies 1.1,1.2 \
   --addresses ch-001,ch-002 \
   --agent <agent-id>
```

For local assumptions:
```bash
af refine <parent-id> \
   --type local_assume \
   --statement "Suppose p = 2" \
   --latex "..." \
   --inference local_assume \
   --agent <agent-id>
```

For discharging:
```bash
af refine <parent-id> \
   --type local_discharge \
   --statement "Therefore, if p = 2 then contradiction" \
   --latex "..." \
   --inference local_discharge \
   --discharges 1.1.A \
   --agent <agent-id>
```

Multi-child (case split):
```bash
af refine <parent-id> \
   --children children.json \
   --agent <agent-id>
```

Definition and reference requests:
```bash
af request-def <name> --latex "..." --source "..." --agent <agent-id>
af add-external --doi "..." --statement "..." --agent <agent-id>
```

#### Verifier Actions

```bash
af challenge <node-id> \
   --objection "..." \
   --targets inference,domain \
   --agent <agent-id>

af resolve-challenge <node-id> --challenge <challenge-id> --agent <agent-id>
af withdraw-challenge <node-id> --challenge <challenge-id> --agent <agent-id>

af accept <node-id> --agent <agent-id>

af verify-external <ext-id> \
   --status verified \
   --verified-statement "..." \
   --bibdata bibdata.json \
   --agent <agent-id>
```

#### Escape Hatches

```bash
af admit <node-id> --reason "..." --agent <agent-id>
af refute <node-id> --reason "..." --agent <agent-id>
af archive <node-id> --reason "..." --agent <agent-id>
```

#### Lemma Extraction

```bash
af extract-lemma \
   --name "Lemma Name" \
   --root 1.2.1 \
   --nodes 1.2.1,1.2.1.1,1.2.1.2 \
   --agent <agent-id>
```

Independence criteria (checked by tool):
1. All internal dependencies satisfied within node set
2. Only root is depended on from outside
3. All scope entries opened in set are closed in set
4. All nodes are `validated`

#### Administrative

```bash
af log
af log --since <seq>

af replay
af replay --verify                    # exit 0: consistent, exit 1: mismatch

af reap --older-than 5m

af recompute-taint                    # force taint recalculation

af def-add <name> --latex "..." --source "..."
af def-reject <request-id> --reason "..."
```

---

### Agent Response Formats

#### Prover Response

```json
{
  "action": "refine",
  "parent": "1.2",
  "children": [
    {
      "type": "claim",
      "statement": "Since p is odd, p = 2k+1 for some integer k.",
      "latex": "p = 2k + 1 \\text{ for some } k \\in \\mathbb{Z}",
      "inference": "by_definition",
      "context": ["DEF-odd"],
      "dependencies": ["1.1"],
      "addresses_challenges": []
    }
  ],
  "rationale": "Expanding the definition of odd to make divisibility explicit."
}
```

Or for definition requests:

```json
{
  "action": "request_def",
  "name": "coprime",
  "latex": "\\gcd(a, b) = 1",
  "source": "standard definition",
  "rationale": "Need coprimality for the main argument."
}
```

#### Verifier Response

```json
{
  "action": "accept",
  "node": "1.2.1",
  "resolved_challenges": ["ch-001", "ch-002"],
  "rationale": "All challenges adequately addressed by child nodes."
}
```

Or:

```json
{
  "action": "challenge",
  "node": "1.2.1",
  "objection": "Claim uses p > 0 but only p > 2 is in scope. The case p ∈ (0, 2] is not excluded.",
  "targets": ["domain", "scope"],
  "rationale": "Domain restriction not justified from assumptions."
}
```

Or:

```json
{
  "action": "reject",
  "node": "1.2.1",
  "reason": "Circular dependency: step references itself through 1.2.1.3.",
  "targets": ["dependencies"]
}
```

---

### Job Availability Rules

**Prover jobs** (`af jobs --role prover`):

Nodes where:
- `workflow_state = available`
- `epistemic_state = pending`
- AND one of:
  - Node has no children (needs development), OR
  - Node has challenges with `state = open` that have empty `addressed_by`

**Verifier jobs** (`af jobs --role verifier`):

Nodes where:
- `workflow_state = available`
- `epistemic_state = pending`
- Either:
  - Node has no open challenges and has children to evaluate, OR
  - All challenges have non-empty `addressed_by` (verifier can resolve and potentially accept)

---

### Workflows

#### Orchestrator (Automated)

```bash
#!/bin/bash

while true; do
  # Check for blocking conditions
  if [ -n "$(af pending-defs --format json | jq -r '.[]')" ]; then
    echo "Blocked on definition requests"
    sleep 60
    continue
  fi
  
  # Spawn provers
  for job in $(af jobs --role prover --format json | jq -r '.[].id'); do
    spawn_prover_agent "$job" &
  done
  
  # Spawn verifiers
  for job in $(af jobs --role verifier --format json | jq -r '.[].id'); do
    spawn_verifier_agent "$job" &
  done
  
  # Maintenance
  af reap --older-than 5m
  
  # Check completion
  ROOT_STATE=$(af get 1 --format json | jq -r '.epistemic_state')
  if [ "$ROOT_STATE" != "pending" ]; then
    echo "Proof complete: $ROOT_STATE"
    af status
    exit 0
  fi
  
  # Check stuck
  if [ -z "$(af jobs --format json | jq -r '.[]')" ]; then
    echo "Proof stuck"
    af status
    exit 1
  fi
  
  sleep 10
done
```

#### Human (Supervisor)

```bash
# Monitor
af status
af log --since 100

# Handle definition requests
af pending-defs
af def-add "even" --latex "..." --source "..."

# Handle external references
af pending-refs
af verify-external EXT-001 --status verified --verified-statement "..." --bibdata bib.json

# Intervene on stuck branches
af archive "1.3" --reason "Unproductive approach" --agent human

# Escape hatches
af admit "1.2.1" --reason "Standard result, see Rudin Theorem 4.2" --agent human

# Extract reusable lemmas
af extract-lemma --name "Divisibility lemma" --root 1.2.1 --nodes 1.2.1,1.2.1.1,1.2.1.2

# Check completion
af status
```

#### Prover Agent

```bash
#!/bin/bash
NODE_ID="$1"
AGENT_ID="prover-$$"

af claim "$NODE_ID" --role prover --agent "$AGENT_ID" || exit 1

CONTEXT=$(af get "$NODE_ID" --full --format json)

# Invoke LLM with CONTEXT, receive RESPONSE JSON

echo "$RESPONSE" > /tmp/response.json
ACTION=$(jq -r '.action' /tmp/response.json)

case "$ACTION" in
  refine)
    jq '.children' /tmp/response.json > /tmp/children.json
    af refine "$NODE_ID" --children /tmp/children.json --agent "$AGENT_ID"
    ;;
  request_def)
    af request-def "$(jq -r '.name' /tmp/response.json)" \
      --latex "$(jq -r '.latex' /tmp/response.json)" \
      --source "$(jq -r '.source' /tmp/response.json)" \
      --agent "$AGENT_ID"
    af release "$NODE_ID" --agent "$AGENT_ID"
    ;;
esac
```

#### Verifier Agent

```bash
#!/bin/bash
NODE_ID="$1"
AGENT_ID="verifier-$$"

CONTEXT=$(af get "$NODE_ID" --full --format json)

# Invoke LLM with CONTEXT, receive RESPONSE JSON

echo "$RESPONSE" > /tmp/response.json
ACTION=$(jq -r '.action' /tmp/response.json)

af claim "$NODE_ID" --role verifier --agent "$AGENT_ID" || exit 1

case "$ACTION" in
  accept)
    for ch_id in $(jq -r '.resolved_challenges[]' /tmp/response.json); do
      af resolve-challenge "$NODE_ID" --challenge "$ch_id" --agent "$AGENT_ID"
    done
    af accept "$NODE_ID" --agent "$AGENT_ID"
    ;;
  challenge)
    af challenge "$NODE_ID" \
      --objection "$(jq -r '.objection' /tmp/response.json)" \
      --targets "$(jq -r '.targets | join(",")' /tmp/response.json)" \
      --agent "$AGENT_ID"
    ;;
  reject)
    af challenge "$NODE_ID" \
      --objection "STRUCTURAL: $(jq -r '.reason' /tmp/response.json)" \
      --targets "$(jq -r '.targets | join(",")' /tmp/response.json)" \
      --agent "$AGENT_ID"
    ;;
esac

af release "$NODE_ID" --agent "$AGENT_ID"
```

---

### Configuration

`proof/meta.json`:

```json
{
  "conjecture": "All primes greater than 2 are odd",
  "created_at": "2025-01-11T09:00:00Z",
  "config": {
    "lock_timeout_seconds": 300,
    "max_proof_depth": 20,
    "max_challenges_per_node": 10,
    "max_refinements_per_node": 15,
    "require_content_hash_verification": true
  }
}
```

---

### Invariants

1. **Hierarchical IDs**: Children of `1.2` are `1.2.1`, `1.2.2`, etc. Assigned by tool.
2. **Definitions immutable**: Once added, never modified.
3. **Content hashes immutable**: Computed at creation, verified on read.
4. **Challenges tracked**: State transitions preserved, never deleted.
5. **Ledger append-only**: No modifications, no gaps.
6. **Derived state consistent**: `af replay --verify` passes.
7. **Exclusive claims**: One agent per node.
8. **Agent isolation**: No concurrent multi-job agents.
9. **Validation invariant**: Enforced by `af accept`.
10. **Scope validity**: Nodes cannot use out-of-scope dependencies.
11. **Taint propagation**: Recomputed on every status change.
12. **Verifier control**: All acceptance decisions explicit.
13. **Definition blocking**: Unresolved requests block affected branches.

---

### Error Conditions

| Error | Cause | Exit Code |
|-------|-------|-----------|
| `ALREADY_CLAIMED` | Lock exists | 1 |
| `NOT_CLAIM_HOLDER` | Wrong agent | 1 |
| `NODE_BLOCKED` | Pending definition | 2 |
| `INVALID_PARENT` | Parent missing | 3 |
| `INVALID_TYPE` | Unknown node type | 3 |
| `INVALID_INFERENCE` | Unknown inference | 3 |
| `INVALID_TARGET` | Unknown challenge target | 3 |
| `CHALLENGE_NOT_FOUND` | Bad challenge ID | 3 |
| `DEF_NOT_FOUND` | Missing definition | 3 |
| `ASSUMPTION_NOT_FOUND` | Missing assumption | 3 |
| `EXTERNAL_NOT_FOUND` | Missing external ref | 3 |
| `SCOPE_VIOLATION` | Using out-of-scope dep | 3 |
| `SCOPE_UNCLOSED` | Local assume not discharged | 3 |
| `DEPENDENCY_CYCLE` | Circular reference | 3 |
| `CONTENT_HASH_MISMATCH` | Integrity failure | 4 |
| `VALIDATION_INVARIANT_FAILED` | Accept preconditions unmet | 1 |
| `DEPTH_EXCEEDED` | Max depth reached | 3 |
| `CHALLENGE_LIMIT_EXCEEDED` | Too many challenges | 3 |
| `REFINEMENT_LIMIT_EXCEEDED` | Too many refinements | 3 |
| `EXTRACTION_INVALID` | Lemma criteria unmet | 3 |
| `LEDGER_INCONSISTENT` | Replay failed | 4 |

Exit codes: 1 = retriable, 2 = blocked, 3 = logic error, 4 = corruption.

---

### Success Criteria

**Complete**: Root node `epistemic_state ∈ {validated, admitted, refuted}`.

**Stuck**: No jobs available, root still `pending`.

**Blocked**: Pending definition or external reference requests.

---

### Future Extensions (Out of Scope)

- Schema extension protocol
- Agent authentication
- Input sanitization
- Distributed storage
- Web UI
- Export to Lean/Coq/Isabelle
- Proof merging
- `af diagnose` for cycle/stuck detection
- Reference checker agent integration
- Formalizer agent integration

---

### Implementation Notes

**Language**: Go

**Structure**:
```
cmd/af/main.go
internal/ledger/
internal/node/
internal/lock/
internal/schema/
internal/jobs/
internal/taint/
internal/scope/
internal/hash/
internal/render/
internal/fuzzy/
```

**Dependencies**: Standard library, `cobra` for CLI, Levenshtein for fuzzy matching.

**Estimated size**: ~3000-3500 LOC

---

*End of PRD v3*
