/-
  No-Cloning Theorem (Formal Proof)

  Lamport structured proof mapping:
    1.1  ASSUME: a cloning isometry exists
    1.2  Inner products: unitarity gives ⟨φ,ψ⟩, cloning gives ⟨φ,ψ⟩²
    1.3  So ⟨φ,ψ⟩ = ⟨φ,ψ⟩² for all φ,ψ
    1.4  Algebraic lemma: x² = x implies x ∈ {0,1} (over any field)
    1.5  But dim ≥ 2 provides a witness with ⟨φ₀,ψ₀⟩ ∉ {0,1}
    1.6  QED: contradiction
-/

import Mathlib.Analysis.InnerProductSpace.Basic

/-!
## No-Cloning Theorem

The standard inner-product argument: if a map satisfies
⟨φ,ψ⟩² = ⟨φ,ψ⟩ for all states (the consequence of unitarity + cloning),
then every inner product is 0 or 1. This is impossible when dim ≥ 2.
-/

/-- **Step 1.4**: Over any integral domain, x² = x implies x = 0 or x = 1. -/
theorem sq_eq_self_of_field {R : Type*} [CommRing R] [NoZeroDivisors R]
    (x : R) (h : x ^ 2 = x) : x = 0 ∨ x = 1 := by
  have h1 : x * (x - 1) = 0 := by
    have hsub : x ^ 2 - x = 0 := sub_eq_zero.mpr h
    calc x * (x - 1) = x ^ 2 - x := by ring
      _ = 0 := hsub
  rcases mul_eq_zero.mp h1 with h0 | h1
  · left; exact h0
  · right; exact sub_eq_zero.mp h1

/--
**No-Cloning Theorem** (inner product formulation).

If every pair of states satisfies ⟨φ,ψ⟩² = ⟨φ,ψ⟩ (the consequence
of a unitary cloning map), and there exist states φ₀, ψ₀ with
⟨φ₀,ψ₀⟩ ∉ {0,1}, then we have a contradiction.

The hypothesis `h_clone` encodes steps 1.1–1.3 of the Lamport proof:
  - A unitary U exists that clones (1.1, ASSUME)
  - Unitarity preserves inner products: ⟨U(φ⊗0), U(ψ⊗0)⟩ = ⟨φ,ψ⟩ (1.2)
  - Cloning means U(φ⊗0) = φ⊗φ, so the inner product is ⟨φ,ψ⟩² (1.3)
  - Combining: ⟨φ,ψ⟩ = ⟨φ,ψ⟩² (1.3, conclusion)
-/
theorem no_cloning
    {E : Type*} [NormedAddCommGroup E] [InnerProductSpace ℂ E]
    -- Step 1.3: unitarity + cloning forces this identity
    (h_clone : ∀ φ ψ : E, @inner ℂ E _ φ ψ = (@inner ℂ E _ φ ψ) ^ 2)
    -- Step 1.5: a witness pair whose inner product is not 0 or 1
    {φ₀ ψ₀ : E}
    (h_not_zero : @inner ℂ E _ φ₀ ψ₀ ≠ 0)
    (h_not_one : @inner ℂ E _ φ₀ ψ₀ ≠ 1) :
    -- Step 1.6: QED
    False := by
  -- Step 1.4: apply the algebraic lemma
  have h_sq := h_clone φ₀ ψ₀
  have h_01 := sq_eq_self_of_field (@inner ℂ E _ φ₀ ψ₀) h_sq.symm
  -- Step 1.6: contradiction with the witness
  rcases h_01 with h0 | h1
  · exact h_not_zero h0
  · exact h_not_one h1
