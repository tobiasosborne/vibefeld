# Experience Report: Proving sqrt(2) is Irrational with AF

## Overview

This report documents my experience using the AF (Adversarial Proof Framework) CLI tool to construct and verify a mathematical proof that the square root of 2 is irrational.

## The Proof Session

### Initialization

```
./af init -c "The square root of 2 is irrational" -a "Human" -d ./temp
```

The tool created:
- A root node (1) with the conjecture as the statement
- Initial epistemic state: `pending`
- Initial workflow state: `available`

### Prover Phase (Prover-1)

Acting as Prover-1, I claimed node 1 and constructed the classic proof by contradiction:

| Node | Type | Statement Summary |
|------|------|-------------------|
| 1.1 | local_assume | Assume sqrt(2) = p/q in lowest terms |
| 1.2 | claim | Square both sides: p² = 2q² |
| 1.3 | claim | p² even implies p even |
| 1.4 | claim | Let p = 2k |
| 1.5 | claim | Substitute: q² = 2k² |
| 1.6 | claim | q² even implies q even |
| 1.7 | claim | Contradiction: gcd(p,q) ≥ 2 but assumed = 1 |
| 1.8 | local_discharge | Our assumption was false |
| 1.9 | qed | Therefore sqrt(2) is irrational |

The refinement process was smooth. The `--type` and `--inference` flags properly captured the logical structure.

### Verifier Phase (Verifier-1)

Acting as Verifier-1, I identified two gaps:

**Challenge 1 (node 1.3)**: The lemma "n² even implies n even" was used without justification.

**Challenge 2 (node 1.6)**: The phrase "same reasoning as for p" was too informal.

Both challenges targeted the `gap` aspect, which correctly identifies missing justification.

### Resolution Phase

Prover-2 (or any prover) could resolve challenges:

**Resolution 1**: Provided contrapositive proof: If n is odd (n = 2m+1), then n² = 4m² + 4m + 1 is odd.

**Resolution 2**: Made the reasoning explicit by citing the lemma from 1.3.

### Validation Phase

After challenges were resolved, Verifier-1 accepted all nodes in order:
- Children first (1.1 through 1.9)
- Root last (1)

Final state: All nodes validated, no jobs remaining.

## Observations

### What Worked Well

1. **CLI Self-Documentation**: The `--help` on each command was clear and included examples.

2. **Event Sourcing**: The ledger captured 27 events showing complete proof history:
   - `proof_initialized`
   - `node_created` (10 nodes)
   - `nodes_claimed`
   - `nodes_released`
   - `challenge_raised` (2)
   - `challenge_resolved` (2)
   - `node_validated` (10)

3. **Challenge/Response Flow**: The adversarial model worked as intended:
   - Verifier identified genuine logical gaps
   - Prover provided rigorous responses
   - This improved the proof quality

4. **Node Types**: The distinction between `claim`, `local_assume`, `local_discharge`, and `qed` properly captured proof structure.

5. **Inference Types**: Types like `modus_ponens`, `contradiction`, and `by_definition` added semantic meaning.

### Issues Encountered

1. **No Verifier Jobs Displayed**: `af jobs` showed all nodes as "prover jobs" even after refinement. Expected to see pending nodes as "verifier jobs" awaiting review.

2. **Challenge Responses Not Attached to Nodes**: The challenge resolution text isn't visible in the proof tree. Ideally, resolved challenges would become part of the node's context or create supplementary nodes.

3. **No Proof Tree Visualization**: Without `af status` implemented, I couldn't see the hierarchical structure. Had to infer it from node IDs.

4. **Linear Acceptance Required**: Had to accept nodes individually. A `--recursive` flag or automatic propagation would help.

5. **Challenge Resolution Ownership**: The `resolve-challenge` command doesn't require `--owner`. In a multi-agent scenario, any agent could resolve any challenge.

### Suggestions for Improvement

1. **Implement `af status`**: Critical for understanding proof state at a glance.

2. **Show Challenge Context**: When displaying a node, show associated challenges and resolutions.

3. **Verifier Job Detection**: Nodes in `pending` epistemic state with no open challenges should show as verifier jobs.

4. **Rich Challenge Resolution**: Option to add a child node as part of challenge resolution (for lemma subproofs).

5. **Proof Export**: `af export` to generate a formatted proof document with all nodes and challenge responses.

## Ledger Analysis

The 27 events form a complete audit trail:

```
Events 1-2:   Initialization (proof + root node)
Event 3:     Prover-1 claims node 1
Events 4-12: 9 child nodes created
Event 13:    Node 1 released
Event 14:    Challenge on 1.3
Event 15:    Challenge 1 resolved
Event 16:    Challenge on 1.6
Event 17:    Challenge 2 resolved
Events 18-27: All 10 nodes validated
```

This append-only structure ensures:
- Full reproducibility
- Blame tracing (who made each decision)
- Rollback capability (conceptually)

## Conclusion

The AF framework successfully supported construction of a non-trivial mathematical proof through adversarial collaboration. The challenge/response mechanism genuinely improved proof quality by forcing explicit justification of lemmas.

The core workflow (init → claim → refine → challenge → resolve → accept) is solid. The main gaps are in observability (status, tree view) and workflow polish (verifier job detection, recursive operations).

**Proof Validity**: The resulting proof correctly establishes that sqrt(2) is irrational through:
1. Proof by contradiction assumption
2. Algebraic manipulation showing p² = 2q²
3. Parity lemma (challenged and proven)
4. Contradiction derivation
5. Discharge of assumption

The adversarial process caught real gaps that would weaken an informal proof.

---

*Report generated: 2026-01-13*
*Tool version: af (vibefeld tracer bullet)*
*Proof directory: ./temp*
