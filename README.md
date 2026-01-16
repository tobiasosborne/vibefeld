# Vibefeld

**Adversarial proof verification for the AI age.**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-Passing-success)](https://github.com/tobias/vibefeld)

```
         _  _         __       _    _
 __ __ _(_)| |__  ___/ _| ___ | |__| |
 \ V  V /| || '_ \/ -_)  _/ -_)| / _` |
  \_/\_/ |_||_.__/\___|_| \___||_\__,_|

  Provers convince. Verifiers attack.
```

---

## The Dance

```
PROVER claims:  "Since a^2 = 2b^2, a must be even"

VERIFIER attacks:
  [CRITICAL] "Why does a^2 being equal to 2b^2 imply a is even?
              You assume without justification."

PROVER refines:
  1.3.1  "a^2 = 2b^2 means a^2 is divisible by 2"
  1.3.2  "If a were odd, a = 2k+1, then a^2 = 4k^2+4k+1 (odd)"
  1.3.3  "But a^2 = 2b^2 is even. Contradiction. So a is even."

VERIFIER validates: [ACCEPTED]
```

This is **adversarial verification**: AI agents that *attack* proofs, not just check them.

---

## What is Vibefeld?

Vibefeld (codename: `af`) is a command-line framework for constructing rigorous natural-language mathematical proofs through adversarial collaboration between AI agents.

Unlike traditional proof assistants that rely on formal logic kernels, Vibefeld orchestrates multiple AI agents in an **adversarial dance**: provers construct arguments, verifiers attack weaknesses. Every claim must survive scrutiny. Every gap gets challenged. The result is a machine-auditable proof tree with full history preserved -- including rejected paths and resolved disputes.

**What makes this different:**
- **Adversarial by design** -- Verifiers are *incentivized* to find flaws, not rubber-stamp claims
- **Natural language** -- Write mathematics as you think it, not in formal syntax
- **Event-sourced truth** -- Every state change is recorded; the proof is its history
- **Multi-agent ready** -- Spawn concurrent prover/verifier agents with filesystem-level isolation
- **Self-documenting** -- Agents need no external documentation; the CLI tells them exactly what to do

---

## Key Features

- **Adversarial Verification** -- Provers convince, verifiers attack. Role isolation prevents bias.
- **Event-Sourced Ledger** -- Append-only history. Current state derived from events. Nothing lost.
- **Hierarchical Proofs** -- Lamport-style structured proofs with automatic ID assignment (1, 1.1, 1.1.1, ...)
- **Taint Tracking** -- Epistemic uncertainty propagates. Admitted nodes taint their dependents.
- **Challenge System** -- Structured objections with severity levels (critical/major/minor/note)
- **Definition Management** -- Request, add, and track mathematical definitions
- **Scope Tracking** -- Local assumptions with proper discharge enforcement
- **Filesystem Concurrency** -- POSIX atomics for multi-agent safety. No database required.

---

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/tobias/vibefeld.git
cd vibefeld && go build ./cmd/af

# Or install directly
go install github.com/tobias/vibefeld/cmd/af@latest
```

### Your First Proof

```bash
# Initialize a proof
af init --claim "The square root of 2 is irrational"

# See what needs work
af status

# Claim the root node as a prover
af claim 1 --role prover --owner prover-001

# Break it down into steps
af refine 1 --statement "Assume sqrt(2) = a/b where a,b are coprime"
af refine 1 --statement "Then 2 = a^2/b^2, so a^2 = 2b^2"
af refine 1 --statement "Therefore a^2 is even, so a is even"
# ... continue building the proof

# Release the claim
af release 1

# As a verifier, challenge weak steps
af claim 1.3 --role verifier --owner verifier-001
af challenge 1.3 --reason "Why does a^2 even imply a is even?" --target inference

# Check progress
af progress
```

For a complete walkthrough, run `af tutorial`.

---

## How It Works

```
                     +------------------+
                     |   ORCHESTRATOR   |
                     |  (spawns agents) |
                     +--------+---------+
                              |
              +---------------+---------------+
              |                               |
      +-------v-------+               +-------v-------+
      |    PROVER     |               |   VERIFIER    |
      |  (constructs) |               |   (attacks)   |
      +-------+-------+               +-------+-------+
              |                               |
              |      af claim / refine        |
              |      af challenge / accept    |
              |                               |
              +---------------+---------------+
                              |
                     +--------v--------+
                     |     LEDGER      |
                     | (append-only)   |
                     | (event-sourced) |
                     +-----------------+
```

1. **Provers propose** -- Add child nodes that break down claims into justified steps
2. **Verifiers challenge** -- Raise objections targeting specific aspects (inference, scope, gaps, ...)
3. **Provers address** -- Refine nodes to answer challenges or admit defeat
4. **Verifiers accept** -- When all challenges resolved and children validated, node becomes validated
5. **Proof completes** -- When the root node reaches `validated` state

The **ledger** records every event. State is always derived from history. Full audit trail preserved.

---

## Node States

| Epistemic State | Meaning |
|-----------------|---------|
| `pending` | Awaiting proof or verification |
| `validated` | Accepted by adversarial verifier |
| `admitted` | Assumed without full proof (introduces taint) |
| `refuted` | Proven false |
| `archived` | Superseded or abandoned branch |

| Taint State | Meaning |
|-------------|---------|
| `clean` | No epistemic uncertainty in ancestry |
| `self_admitted` | This node was admitted |
| `tainted` | Depends on admitted/tainted ancestor |
| `unresolved` | Taint not yet computed |

---

## Example: Dobinski's Formula

A real proof constructed with Vibefeld (24 nodes, 7 challenges raised and resolved):

```
af status --dir examples/dobinski-proof

1 [validated/clean] Dobinski's Formula: B_n = (1/e) * Sum(k^n / k!)
  1.1 [validated/clean] By definition, B_n = Sum S(n,k)
    1.1.1 [validated/clean] Every partition has exactly k blocks for some k
    1.1.2 [validated/clean] Partitions form disjoint union by block count
    ...
  1.2 [validated/clean] S(n,k) counts surjections / k!
  1.3 [validated/clean] Substituting the explicit formula...
  1.4 [validated/clean] Exchanging order of summation...
    1.4.4 [archived/clean] (Flawed approach, abandoned after challenge)
    1.4.5 [validated/clean] Extension is well-defined
      1.4.5.3 [validated/clean] k^n in terms of Stirling numbers
      ...
  1.8 [validated/clean] QED
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Tutorial](docs/tutorial.md) | Step-by-step guide to your first proof |
| [CLI Reference](docs/cli-reference.md) | Complete command documentation |
| [Architecture](docs/architecture.md) | System design and data model |
| [PRD](docs/prd.md) | Full product requirements document |
| [Contributing](CONTRIBUTING.md) | How to contribute |

Or just run `af --help` -- the CLI is fully self-documenting.

---

## Common Commands

| Command | Description |
|---------|-------------|
| `af init` | Initialize a new proof workspace |
| `af status` | Show proof tree with states and taint |
| `af jobs` | List available prover/verifier work |
| `af claim` | Claim a node for work |
| `af refine` | Add child nodes to develop proof |
| `af challenge` | Raise objection against a node |
| `af accept` | Validate a node (verifier) |
| `af progress` | Show completion metrics |
| `af log` | View event history |

---

## Project Status

Vibefeld is under active development.

**Working:**
- Core proof workflow (init, refine, challenge, accept)
- Event-sourced ledger with replay verification
- Multi-agent concurrency via filesystem locks
- Taint propagation
- Scope tracking for local assumptions
- Export to Markdown/LaTeX/PDF

**Roadmap:**
- [ ] Web UI for proof visualization
- [ ] Export to Lean/Coq/Isabelle
- [ ] Reference checker agent integration
- [ ] Distributed storage backend
- [ ] Schema extension protocol

---

## Philosophy

> "Beware of bugs in the above code; I have only ~~proved it correct~~ *vibe coded it*, not tried it." -- ~~Donald Knuth~~ *every vibe coder*

Traditional proof assistants trust the prover to find their own errors. Vibefeld assumes you will miss things. That is why it deploys adversaries.

Every proof in Vibefeld is a record of intellectual combat: claims made, challenges raised, weaknesses exposed, arguments refined. The final validated proof is not just correct -- it is *battle-tested*.

---

## Requirements

- Go 1.22+
- POSIX-compliant filesystem (for atomic operations)

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

<p align="center">
<i>Provers convince. Verifiers attack. Truth emerges.</i>
</p>
