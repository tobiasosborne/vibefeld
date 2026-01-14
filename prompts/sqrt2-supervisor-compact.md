# √2 Irrationality Proof - Supervisor Agent

You are orchestrating a rigorous mathematical proof using the `af` adversarial proof tool.

## Setup (Run Once)

```bash
mkdir -p sqrt2-proof && cd sqrt2-proof

af init -c "Theorem: √2 is irrational. Formally: ¬∃p,q∈ℤ (q≠0 ∧ gcd(p,q)=1 ∧ p²=2q²)" -a "supervisor"

# Definitions
af def-add "rational" "r∈ℚ iff ∃p,q∈ℤ, q≠0: r=p/q"
af def-add "coprime" "gcd(p,q)=1 iff the only positive common divisor is 1"
af def-add "even" "n is even iff ∃k∈ℤ: n=2k"

# Lemmas (external references)
af add-external "even-square" "∀n∈ℤ: n² even ⟹ n even"
af add-external "parity" "∀n∈ℤ: n even ⊕ n odd"
```

## Spawn Prover Agent

Give this to a Prover subagent:

```
ROLE: Prover for √2 irrationality proof
TOOL: af (run `af --help` for commands)
DIRECTORY: sqrt2-proof/

PROOF STRUCTURE (refine node 1 with these children):
1.1: "Assume for contradiction: ∃p,q∈ℤ with q≠0, gcd(p,q)=1, and √2=p/q"
  1.1.1: "Squaring: 2 = p²/q², thus p² = 2q²"
  1.1.2: "p² = 2q² means p² is even (def: k=q²)"
  1.1.3: "p² even ⟹ p even (by even-square)"
  1.1.4: "p even ⟹ p=2m for some m∈ℤ (def of even)"
  1.1.5: "Substituting: (2m)²=2q² ⟹ 4m²=2q² ⟹ q²=2m²"
  1.1.6: "q²=2m² means q² is even (def: k=m²)"
  1.1.7: "q² even ⟹ q even (by even-square)"
  1.1.8: "p even ∧ q even ⟹ 2|gcd(p,q), contradicting gcd(p,q)=1"
1.2: "Contradiction ⟹ ¬∃ such p,q ⟹ √2 is irrational ∎"

COMMANDS:
af claim NODE -o prover1
af refine NODE -o prover1 -s "STATEMENT" -j "JUSTIFICATION" -t derivation
af release NODE -o prover1
af resolve-challenge CHALLENGE_ID -r "RESPONSE"

RULES:
- Justify EVERY step (cite definitions/lemmas by name)
- No "obvious" or "clearly" - spell it out
- Respond to challenges with MORE detail
```

## Spawn Verifier Agent

Give this to a Verifier subagent:

```
ROLE: Adversarial Verifier for √2 proof
TOOL: af (run `af --help` for commands)
DIRECTORY: sqrt2-proof/

YOUR JOB: Find flaws. Be harsh. Mathematical rigor means ZERO gaps.

CHALLENGE IF YOU SEE:
- Unstated assumptions ("where does q≠0 come from?")
- Missing justification ("why does p² even imply p even?")
- Logical leaps ("show the algebra step by step")
- Undefined terms ("what does 'coprime' mean here?")
- Implicit domain ("are we in ℤ, ℚ, or ℝ?")

COMMANDS:
af status                          # see proof tree
af get NODE                        # examine a step
af challenge NODE -r "OBJECTION"   # raise challenge
af accept NODE                     # if step is rigorous
af withdraw-challenge CH_ID        # if prover fixes it

ONLY ACCEPT when the step is LOGICALLY AIRTIGHT with explicit justification.
```

## Orchestration Loop

```bash
while true; do
  af status
  # If all nodes validated and no challenges: DONE
  # If pending nodes exist: ensure prover is working
  # If unchallenged pending nodes: spawn verifier
  # If open challenges: ensure prover responds
  sleep 5
done
```

## Victory Condition

```bash
af status --format json | jq '.nodes | map(select(.epistemic_state != "validated")) | length'
# Must return 0

af status  # Should show all nodes validated, no taint
```

The proof is complete when every node is `validated` with `taint: clean`.
