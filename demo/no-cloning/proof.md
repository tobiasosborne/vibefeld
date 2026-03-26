# Proof Export

## Node 1

**Statement:** There is no unitary operator U acting on two qubits such that U|psi>|0> = |psi>|psi> for every single-qubit state |psi>.

**Type:** claim

**Inference:** assumption

**Status:** pending

**Taint:** unresolved

### Node 1.1

**Statement:** Assume for contradiction that such a unitary U exists: U|psi>|0> = |psi>|psi> for all single-qubit states |psi>.

**Type:** local_assume

**Inference:** local_assume

**Status:** pending

**Taint:** unresolved

### Node 1.2

**Statement:** We derive a contradiction by applying U to |+> = (1/sqrt2)(|0>+|1>) in two different ways.

**Type:** claim

**Inference:** contradiction

**Status:** pending

**Taint:** unresolved

#### Node 1.2.1

**Statement:** Applying U to the computational basis states: U|0>|0> = |0>|0> and U|1>|0> = |1>|1>.

**Type:** claim

**Inference:** assumption

**Status:** pending

**Taint:** unresolved

#### Node 1.2.2

**Statement:** Since U is linear, U|+>|0> = (1/sqrt2)(U|0>|0> + U|1>|0>) = (1/sqrt2)(|00> + |11>). This is an entangled state.

**Type:** claim

**Inference:** assumption

**Status:** pending

**Taint:** unresolved

#### Node 1.2.3

**Statement:** But the cloning assumption gives U|+>|0> = |+>|+> = (1/2)(|00> + |01> + |10> + |11>). This is a product state.

**Type:** claim

**Inference:** assumption

**Status:** pending

**Taint:** unresolved

#### Node 1.2.4

**Statement:** Steps 1.2.2 and 1.2.3 give different results for U|+>|0>: (1/sqrt2)(|00>+|11>) != (1/2)(|00>+|01>+|10>+|11>). The left side is entangled (Schmidt rank 2); the right is a product (Schmidt rank 1). Contradiction.

**Type:** claim

**Inference:** assumption

**Status:** pending

**Taint:** unresolved

