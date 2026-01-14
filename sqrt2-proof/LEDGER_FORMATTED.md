# Proof Ledger: √2 is Irrational

## Proof Metadata

| Field | Value |
|-------|-------|
| **Conjecture** | The square root of 2 is irrational: there do not exist coprime integers p, q with q ≠ 0 such that (p/q)² = 2 |
| **Author** | supervisor |
| **Started** | 2026-01-14 12:54:47 UTC |
| **Completed** | 2026-01-14 13:01:56 UTC |
| **Duration** | 7 minutes 9 seconds |
| **Total Events** | 36 |

---

## Definitions

| Name | Definition |
|------|------------|
| **rational** | A real number r is rational iff there exist integers p, q with q ≠ 0 such that r = p/q |
| **irrational** | A real number is irrational iff it is not rational |
| **coprime** | Integers p, q are coprime (gcd(p,q) = 1) iff their only common positive divisor is 1 |
| **even** | An integer n is even iff there exists an integer k such that n = 2k |
| **odd** | An integer n is odd iff there exists an integer k such that n = 2k + 1 |

---

## External References (Axioms/Lemmas)

| Name | Statement |
|------|-----------|
| **even-square-lemma** | For any integer n: n² even implies n even (contrapositive: n odd implies n² odd) |
| **parity-dichotomy** | Every integer is either even or odd, but not both |
| **gcd-exists** | For any integers p, q with q ≠ 0, there exist p', q' with gcd(p',q') = 1 and p/q = p'/q' |
| **integers-closed-multiplication** | The integers are closed under multiplication: if a, b ∈ ℤ then ab ∈ ℤ |

---

## Proof Tree

```
1 [VALIDATED] Conjecture
└── 1.1 [VALIDATED] Assumption for contradiction
    ├── 1.1.1 [VALIDATED] WLOG gcd(p,q) = 1
    ├── 1.1.2 [VALIDATED] Derive p² = 2q²
    ├── 1.1.3 [VALIDATED] p² is even
    ├── 1.1.4 [VALIDATED] p is even
    ├── 1.1.5 [VALIDATED] p = 2m
    ├── 1.1.6 [VALIDATED] Derive 2m² = q²
    ├── 1.1.7 [VALIDATED] q² is even
    ├── 1.1.8 [VALIDATED] q is even
    └── 1.1.9 [VALIDATED] Contradiction
└── 1.2 [VALIDATED] QED
```

---

## Proof Nodes (Full Text)

### Node 1 — Conjecture
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | assumption |
| **Status** | VALIDATED |

> **Statement**: The square root of 2 is irrational: there do not exist coprime integers p, q with q ≠ 0 such that (p/q)² = 2

---

### Node 1.1 — Assumption for Contradiction
| Field | Value |
|-------|-------|
| **Type** | local_assume |
| **Inference** | assumption |
| **Status** | VALIDATED |

> **Statement**: Assume for contradiction that √2 is rational. By definition of 'rational', there exist integers p, q with q ≠ 0 such that √2 = p/q.

---

### Node 1.1.1 — WLOG Coprimality
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | by_definition |
| **Status** | VALIDATED |

> **Statement**: Without loss of generality, we may assume gcd(p, q) = 1, i.e., p and q are coprime. If they share a common factor d > 1, we can divide both by d to obtain an equivalent representation p/d and q/d. By 'gcd-exists', every pair of integers has a well-defined greatest common divisor, so we can always reduce to this coprime form.

---

### Node 1.1.2 — Algebraic Derivation
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | modus_ponens |
| **Status** | VALIDATED |

> **Statement**: From (p/q)² = 2, by algebraic manipulation we derive p² = 2q². Multiplying both sides by q²: p² = 2q². This equation will be the foundation for our parity argument.

---

### Node 1.1.3 — p² is Even
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | by_definition |
| **Status** | VALIDATED |

> **Statement**: p² is even. By definition of 'even', an integer n is even if there exists an integer k such that n = 2k. Here p² = 2q², so taking k = q², we have p² = 2k. Since q is an integer, by 'integers-closed-multiplication', q² is also an integer, so k = q² is a valid witness. Therefore p² is even.

---

### Node 1.1.4 — p is Even
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | modus_ponens |
| **Status** | VALIDATED |

> **Statement**: p is even. By 'even-square-lemma': for any integer n, if n² is even then n is even. We have established that p² is even (node 1.1.3). Applying the lemma with n = p, we conclude p is even.

---

### Node 1.1.5 — Existential Witness for p
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | existential_instantiation |
| **Status** | VALIDATED |

> **Statement**: p = 2m for some integer m. By definition of 'even': an integer n is even if and only if there exists an integer k such that n = 2k. Since p is even (node 1.1.4), by existential instantiation there exists such an integer. Let m denote this witness, so p = 2m.

---

### Node 1.1.6 — Substitution
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | modus_ponens |
| **Status** | VALIDATED |

> **Statement**: Substituting p = 2m into p² = 2q²: (2m)² = 2q², which gives 4m² = 2q². Dividing both sides by 2: 2m² = q². This algebraic manipulation is valid since we are working with integer equations.

---

### Node 1.1.7 — q² is Even
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | by_definition |
| **Status** | VALIDATED |

> **Statement**: q² is even. By definition of 'even', an integer n is even if there exists an integer k such that n = 2k. Here q² = 2m², so taking k = m², we have q² = 2k. Since m is an integer, by 'integers-closed-multiplication', m² is also an integer, so k = m² is a valid witness. Therefore q² is even.

---

### Node 1.1.8 — q is Even
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | modus_ponens |
| **Status** | VALIDATED |

> **Statement**: q is even. By 'even-square-lemma': for any integer n, if n² is even then n is even. We have established that q² is even (node 1.1.7). Applying the lemma with n = q, we conclude q is even.

---

### Node 1.1.9 — Contradiction
| Field | Value |
|-------|-------|
| **Type** | claim |
| **Inference** | contradiction |
| **Status** | VALIDATED |

> **Statement**: Contradiction: Both p and q are even, meaning 2 divides both p and q. Therefore 2 divides gcd(p, q), which means gcd(p, q) ≥ 2. But we established in node 1.1.1 that gcd(p, q) = 1. Since 2 ≥ 2 and gcd(p, q) = 1 cannot both hold, we have derived a contradiction from our assumption that √2 is rational.

---

### Node 1.2 — Conclusion (QED)
| Field | Value |
|-------|-------|
| **Type** | local_discharge |
| **Inference** | contradiction |
| **Status** | VALIDATED |

> **Statement**: Therefore √2 is irrational. We assumed √2 was rational (node 1.1) and derived a contradiction (node 1.1.9). By the principle of proof by contradiction (reductio ad absurdum), our assumption must be false. Hence √2 is not rational, i.e., √2 is irrational. By definition of 'irrational', a real number is irrational if and only if it is not rational. QED.

---

## Event Log

| # | Time | Event | Details |
|---|------|-------|---------|
| 1 | 12:54:47 | ProofInitialized | Conjecture established |
| 2 | 12:54:47 | NodeCreated | Node 1 (root conjecture) |
| 3 | 12:55:05 | DefAdded | "rational" |
| 4 | 12:55:06 | DefAdded | "irrational" |
| 5 | 12:55:07 | DefAdded | "coprime" |
| 6 | 12:55:08 | DefAdded | "even" |
| 7 | 12:55:09 | DefAdded | "odd" |
| 8 | 12:56:00 | NodesClaimed | Node 1 by prover1 |
| 9 | 12:56:17 | NodeCreated | Node 1.1 (assumption) |
| 10 | 12:57:02 | NodesReleased | Node 1 |
| 11 | 12:57:37 | NodesClaimed | Node 1.1 by prover1 |
| 12 | 12:57:44 | NodeCreated | Node 1.1.1 (WLOG gcd) |
| 13 | 12:57:52 | NodeCreated | Node 1.1.2 (p² = 2q²) |
| 14 | 12:57:57 | NodeCreated | Node 1.1.3 (p² even) |
| 15 | 12:58:05 | NodeCreated | Node 1.1.4 (p even) |
| 16 | 12:58:11 | NodeCreated | Node 1.1.5 (p = 2m) |
| 17 | 12:58:16 | NodeCreated | Node 1.1.6 (2m² = q²) |
| 18 | 12:58:23 | NodeCreated | Node 1.1.7 (q² even) |
| 19 | 12:58:30 | NodeCreated | Node 1.1.8 (q even) |
| 20 | 12:58:37 | NodeCreated | Node 1.1.9 (contradiction) |
| 21 | 12:58:47 | NodesReleased | Node 1.1 |
| 22 | 12:58:51 | NodesClaimed | Node 1 by prover1 |
| 23 | 12:58:56 | NodeCreated | Node 1.2 (QED) |
| 24 | 12:59:02 | NodesReleased | Node 1 |
| 25 | 13:01:32 | NodeValidated | Node 1.1.3 |
| 26 | 13:01:33 | NodeValidated | Node 1.1.4 |
| 27 | 13:01:33 | NodeValidated | Node 1.1.5 |
| 28 | 13:01:35 | NodeValidated | Node 1.1.6 |
| 29 | 13:01:36 | NodeValidated | Node 1.1.7 |
| 30 | 13:01:37 | NodeValidated | Node 1.1.8 |
| 31 | 13:01:44 | NodeValidated | Node 1.1.1 |
| 32 | 13:01:45 | NodeValidated | Node 1.1.2 |
| 33 | 13:01:46 | NodeValidated | Node 1.1.9 |
| 34 | 13:01:54 | NodeValidated | Node 1.1 |
| 35 | 13:01:55 | NodeValidated | Node 1.2 |
| 36 | 13:01:56 | NodeValidated | Node 1 |

---

## Summary Statistics

| Metric | Value |
|--------|-------|
| Total Nodes | 12 |
| Validated | 12 |
| Pending | 0 |
| Challenged | 0 |
| Refuted | 0 |
| Definitions Used | 5 |
| External References | 4 |
| Proof Technique | Contradiction (Reductio ad Absurdum) |

---

*Generated from AF ledger on 2026-01-14*
