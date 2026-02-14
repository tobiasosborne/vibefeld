# Proof Export

## Node 1

**Statement:** Dobinski's Formula: For all non-negative integers n, the Bell number B_n equals (1/e) * Σ_{k=0}^{∞} (k^n / k\!), where B_n counts the number of partitions of a set with n elements

**Type:** claim

**Inference:** assumption

**Status:** validated

**Taint:** clean

### Node 1.1

**Statement:** By definition, B_n = Σ_{k=0}^{n} S(n,k), the sum of Stirling numbers of the second kind

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

#### Node 1.1.1

**Statement:** For n ≥ 1, every partition of an n-set has exactly k non-empty blocks for some unique k with 1 ≤ k ≤ n. For n = 0, the unique partition (the empty partition) has k = 0 blocks.

**Type:** claim

**Inference:** assumption

**Status:** validated

**Taint:** clean

#### Node 1.1.2

**Statement:** The set of all partitions of an n-set is the disjoint union of partitions with exactly k blocks, for k = 1, 2, ..., n

**Type:** claim

**Inference:** assumption

**Status:** validated

**Taint:** clean

#### Node 1.1.3

**Statement:** By definition of S(n,k), there are exactly S(n,k) partitions with exactly k blocks

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

#### Node 1.1.4

**Statement:** By the addition principle for disjoint sets, B_n = Σ_{k=1}^{n} S(n,k). Since S(n,0) = 0 for n > 0, we can write B_n = Σ_{k=0}^{n} S(n,k)

**Type:** claim

**Inference:** assumption

**Status:** validated

**Taint:** clean

### Node 1.2

**Statement:** S(n,k) counts surjections from an n-set to a k-set divided by k!. Equivalently, S(n,k) = (1/k!) * Σ_{j=0}^{k} (-1)^{k-j} * C(k,j) * j^n

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

### Node 1.3

**Statement:** Substituting the explicit formula: B_n = Σ_{k=0}^{n} (1/k!) * Σ_{j=0}^{k} (-1)^{k-j} * C(k,j) * j^n

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

### Node 1.4

**Statement:** Exchanging the order of summation and extending k to infinity (terms with k > n contribute 0 to inner sum when j ≤ n): B_n = Σ_{j=0}^{∞} j^n * Σ_{k=j}^{∞} (-1)^{k-j} * C(k,j) / k!

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

#### Node 1.4.1

**Statement:** For the finite double sum Σ_{k=0}^{n} Σ_{j=0}^{k} f(j,k), we can exchange order of summation to Σ_{j=0}^{n} Σ_{k=j}^{n} f(j,k) since both are finite sums over the same triangular region {(j,k): 0 ≤ j ≤ k ≤ n}

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

#### Node 1.4.2

**Statement:** After exchange: B_n = Σ_{j=0}^{n} j^n * Σ_{k=j}^{n} (-1)^{k-j} * C(k,j) / k!

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

#### Node 1.4.3

**Statement:** For fixed j, the inner sum Σ_{k=j}^{∞} (-1)^{k-j} * C(k,j) / k! converges absolutely (by ratio test, similar to node 1.7), so we can extend the finite sum to infinity: Σ_{k=j}^{n} converges to Σ_{k=j}^{∞} as n → ∞

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

#### Node 1.4.4

**Statement:** The outer sum has only finitely many non-zero terms when j > 0 since j^n contributes only for j = 0, 1, ..., n. For j > n, we define 0^n = 0 (for n > 0). Thus extending j to infinity adds only zeros.

**Type:** claim

**Inference:** by_definition

**Status:** archived

**Taint:** clean

#### Node 1.4.5

**Statement:** Therefore B_n = Σ_{j=0}^{∞} j^n * Σ_{k=j}^{∞} (-1)^{k-j} * C(k,j) / k! with the extension being well-defined

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

##### Node 1.4.5.1

**Statement:** From 1.4.2 and 1.4.3: B_n = Σ_{j=0}^{n} j^n · [1/(e·j!)] = (1/e) · Σ_{j=0}^{n} j^n/j!

**Type:** claim

**Inference:** modus_ponens

**Status:** archived

**Taint:** clean

##### Node 1.4.5.2

**Statement:** The partial sum (1/e)·Σ_{j=0}^{n} j^n/j! is exactly the first n+1 terms of the convergent series (1/e)·Σ_{j=0}^{∞} j^n/j!. Since we derived B_n equals this partial sum, and B_n is a fixed finite value independent of how we write it, we have B_n = (1/e)·Σ_{j=0}^{∞} j^n/j!

**Type:** claim

**Inference:** modus_ponens

**Status:** archived

**Taint:** clean

##### Node 1.4.5.3

**Statement:** Powers can be written in terms of Stirling numbers: k^n = Σ_{j=0}^{n} S(n,j) · (k)_j where (k)_j = k(k-1)...(k-j+1) is the falling factorial. Equivalently, k^n = Σ_{j=0}^{n} S(n,j) · j! · C(k,j) where C(k,j)=0 for k<j.

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

##### Node 1.4.5.4

**Statement:** Substituting into Dobinski's sum: (1/e)·Σ_{k=0}^{∞} k^n/k\! = (1/e)·Σ_{k=0}^{∞} (1/k\!)·Σ_{j=0}^{n} S(n,j)·j\!·C(k,j) = (1/e)·Σ_{j=0}^{n} S(n,j)·j\!·Σ_{k=j}^{∞} C(k,j)/k\!

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

##### Node 1.4.5.5

**Statement:** For k≥j: C(k,j)/k! = 1/(j!·(k-j)!). So Σ_{k=j}^{∞} C(k,j)/k! = (1/j!)·Σ_{m=0}^{∞} 1/m! = e/j!

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

##### Node 1.4.5.6

**Statement:** Therefore (1/e)·Σ_{k=0}^{∞} k^n/k! = (1/e)·Σ_{j=0}^{n} S(n,j)·j!·(e/j!) = Σ_{j=0}^{n} S(n,j) = B_n

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

### Node 1.5

**Statement:** The inner sum Σ_{k=j}^{∞} (-1)^{k-j} * C(k,j) / k! = 1/(e * j!) by the series expansion of e^{-1}

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

### Node 1.6

**Statement:** Substituting: B_n = Σ_{j=0}^{∞} j^n * 1/(e * j!) = (1/e) * Σ_{j=0}^{∞} j^n / j!

**Type:** claim

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

### Node 1.7

**Statement:** The series Σ_{k=0}^{∞} k^n / k! converges absolutely for all n ≥ 0 by ratio test comparison with e^k

**Type:** claim

**Inference:** by_definition

**Status:** validated

**Taint:** clean

### Node 1.8

**Statement:** Therefore B_n = (1/e) * Σ_{k=0}^{∞} k^n / k\!, which is Dobinski's Formula. QED

**Type:** qed

**Inference:** modus_ponens

**Status:** validated

**Taint:** clean

