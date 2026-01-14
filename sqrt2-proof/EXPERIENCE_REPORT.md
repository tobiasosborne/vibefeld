# Experience Report: Proving √2 is Irrational with AF

## Executive Summary

Successfully completed a formal proof that √2 is irrational using the Adversarial Proof Framework (AF). The proof was constructed through an adversarial protocol with spawned Prover and Verifier subagents, resulting in 12 validated nodes with no challenges or refutations.

**Duration**: ~7 minutes (12:54:47 - 13:01:56)
**Result**: PROOF COMPLETE - All 12 nodes validated

---

## Protocol Execution

### Phase 1: Initialization

The supervisor (myself) performed the following setup:

1. **Cleared workspace**: Removed existing proof artifacts from `sqrt2-proof/`
2. **Initialized proof**: Created the conjecture node with:
   ```
   "The square root of 2 is irrational: there do not exist coprime integers p, q with q ≠ 0 such that (p/q)² = 2"
   ```
3. **Added definitions** (5 total):
   - `rational`: A real number r is rational iff ∃ integers p,q with q≠0 such that r = p/q
   - `irrational`: A real number is irrational iff it is not rational
   - `coprime`: Integers p,q are coprime iff gcd(p,q) = 1
   - `even`: An integer n is even iff ∃k∈ℤ such that n = 2k
   - `odd`: An integer n is odd iff ∃k∈ℤ such that n = 2k + 1

4. **Added external references** (4 total):
   - `even-square-lemma`: n² even ⟹ n even
   - `parity-dichotomy`: Every integer is either even or odd, not both
   - `gcd-exists`: Any rational can be reduced to coprime form
   - `integers-closed-multiplication`: ab ∈ ℤ for a,b ∈ ℤ

### Phase 2: Prover Agent

A general-purpose subagent was spawned with the "Prover" role. The agent:

1. Checked proof status and available jobs
2. Claimed node 1.1 (the assumption node already existed)
3. Systematically refined the proof structure by adding nodes 1.1.1 through 1.1.9
4. Created node 1.2 as the final discharge/conclusion
5. Released all claims when complete

**Nodes created by Prover**:
| Node | Type | Inference | Purpose |
|------|------|-----------|---------|
| 1.1.1 | claim | by_definition | WLOG gcd(p,q)=1 via gcd-exists |
| 1.1.2 | claim | modus_ponens | Derive p² = 2q² |
| 1.1.3 | claim | by_definition | p² is even (witness k=q²) |
| 1.1.4 | claim | modus_ponens | p is even via even-square-lemma |
| 1.1.5 | claim | existential_instantiation | p = 2m for some m |
| 1.1.6 | claim | modus_ponens | Derive 2m² = q² |
| 1.1.7 | claim | by_definition | q² is even (witness k=m²) |
| 1.1.8 | claim | modus_ponens | q is even via even-square-lemma |
| 1.1.9 | claim | contradiction | Both even contradicts gcd=1 |
| 1.2 | local_discharge | contradiction | QED by reductio ad absurdum |

### Phase 3: Verifier Agent

A separate general-purpose subagent was spawned with the "Verifier" role. The agent:

1. Examined each node using `af get NODE_ID`
2. Verified that justifications were explicit and cited appropriate definitions/lemmas
3. Checked algebraic manipulations for validity
4. Confirmed witnesses for evenness were properly identified as integers
5. Validated all 12 nodes with no challenges raised

**Verification order** (bottom-up, as instructed):
1.1.3 → 1.1.4 → 1.1.5 → 1.1.6 → 1.1.7 → 1.1.8 → 1.1.1 → 1.1.2 → 1.1.9 → 1.1 → 1.2 → 1

---

## Observations and Lessons Learned

### What Worked Well

1. **Subagent delegation**: Spawning separate Prover and Verifier agents cleanly separated concerns. Each agent focused on its role without confusion.

2. **Explicit instructions**: Providing detailed command references and valid inference types in the agent prompts prevented errors and reduced back-and-forth.

3. **AF tool design**: The CLI was intuitive with helpful "Next steps" suggestions after each command. The fuzzy matching for commands was forgiving.

4. **Proof structure**: The classical proof by contradiction mapped naturally to the AF node hierarchy (local_assume → claims → contradiction → local_discharge).

### Challenges Encountered

1. **Initial confusion about spawning**: I initially started executing the proof directly as the supervisor rather than spawning subagents. The user correctly reminded me that the protocol requires agent spawning.

2. **Command syntax discovery**: The `add-external` command required `-n` and `-s` flags rather than positional arguments. The prompt's example syntax didn't match the actual CLI.

3. **Inference type discovery**: The prompt suggested `lemma_application` as an inference type, but the actual valid types differ. Had to read the source code to find the correct types.

4. **Taint state remains "unresolved"**: After validation, nodes show `[validated/unresolved]` - the taint computation didn't automatically run to set them to `clean`. This is a minor cosmetic issue.

### Suggestions for Protocol Improvement

1. **Include valid inference types in prompt**: The supervisor prompt should list the actual valid inference types (modus_ponens, by_definition, etc.) rather than suggested names like `lemma_application`.

2. **Provide exact command syntax**: CLI flag syntax should be documented precisely to avoid trial-and-error.

3. **Automatic taint computation**: Consider auto-computing taint after validation to show `[validated/clean]` for a fully clean state.

---

## Final Proof Statistics

```
Nodes: 12 total
  Epistemic: 0 pending, 12 validated, 0 admitted, 0 refuted, 0 archived
  Challenges: 0 raised, 0 resolved
  Events: 36 total in ledger
```

---

## Proof Event Timeline

| # | Event | Time | Description |
|---|-------|------|-------------|
| 1 | ProofInitialized | 12:54:47 | Started proof |
| 2-7 | DefAdded | 12:55:05-09 | Added 5 definitions |
| 8-24 | Node operations | 12:56:00-12:59:02 | Prover constructed 12 nodes |
| 25-36 | NodeValidated | 13:01:32-56 | Verifier validated all nodes |

---

## Conclusion

The adversarial proof protocol successfully produced a rigorous, formally verified proof of the irrationality of √2. The spawned agents worked autonomously with minimal supervisor intervention. The AF framework effectively enforced separation of concerns between prover and verifier roles, ensuring that every logical step was explicitly justified and adversarially verified.

The proof is now a permanent artifact in the ledger, fully traceable and auditable.

---

*Report generated: 2026-01-14*
*Supervisor: Claude Opus 4.5*
*Prover Agent ID: a46b158*
*Verifier Agent ID: af335c5*
