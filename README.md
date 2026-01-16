# Vibefeld (af)

CLI for adversarial proof verification. Multiple AI agents work concurrently as adversarial provers and verifiers, refining natural-language mathematical proof steps until rigorous acceptance.

## Quick Start

### Install

```bash
# Install directly
go install github.com/tobias/vibefeld/cmd/af@latest

# Or build from source
git clone https://github.com/tobias/vibefeld.git
cd vibefeld
go build ./cmd/af
```

### Initialize a Proof

```bash
# Start a new proof
af init --claim "The sum of first n natural numbers is n(n+1)/2"

# Check proof status
af status

# Claim work as a prover
af claim 1 --role prover

# Refine a node with sub-claims
af refine 1 --children "Base case: n=1" "Inductive step"

# Release a claim
af release 1

# Accept a node (as verifier)
af accept 1
```

## Key Concepts

### Prover/Verifier Roles

- **Provers** construct and refine proof steps, convincing verifiers of correctness
- **Verifiers** challenge claims and attack weak arguments; they control acceptance
- No agent plays both roles simultaneously

### Node States

Nodes progress through workflow and epistemic states:

- **Workflow**: `available`, `claimed`, `blocked`
- **Epistemic**: `pending`, `validated`, `admitted`, `refuted`, `archived`

### Adversarial Verification

The framework ensures rigorous proofs through adversarial interaction:

1. Provers submit claims and refinements
2. Verifiers challenge questionable steps
3. Provers must resolve challenges or see nodes refuted
4. Only verifier-accepted nodes become validated

## Common Commands

| Command | Description |
|---------|-------------|
| `af init` | Initialize a new proof with a root claim |
| `af status` | Show current proof state and available work |
| `af claim` | Claim a node to work on (as prover or verifier) |
| `af refine` | Break a claimed node into sub-claims |
| `af release` | Release a claimed node |
| `af accept` | Accept a node as valid (verifier only) |
| `af challenge` | Raise a challenge against a node |
| `af resolve-challenge` | Respond to an open challenge |

Run `af --help` or `af <command> --help` for detailed usage.

## Requirements

- Go 1.22+

## License

MIT License - see [LICENSE](LICENSE) for details.
