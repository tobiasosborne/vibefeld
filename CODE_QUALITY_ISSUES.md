# Code Quality Issues Summary - Vibefeld

**Generated:** 2026-01-19
**Codebase:** vibefeld (AF CLI)
**Total cmd/af LOC:** ~19,700 lines across 121 files
**Total render package LOC:** ~5,000 lines

---

## Executive Summary

| Category | Critical | High | Medium | Total Issues |
|----------|----------|------|--------|--------------|
| Code Duplication | 2 | 3 | 0 | 5 |
| God Objects | 1 | 0 | 0 | 1 |
| Code Complexity | 0 | 1 | 3 | 4 |
| Test Coverage Gaps | 0 | 3 | 2 | 5 |
| **TOTAL** | **3** | **7** | **5** | **15** |

**Estimated Technical Debt:** ~2,000 duplicated lines (8-12% of cmd/af)
**Potential Savings:** 1,200-1,500 lines (60-75% reduction)
**Total Refactoring Effort:** 8-12 hours

---

## 1. Code Duplication

### CRITICAL-DUP-001: Destructive Confirmation Prompt Pattern
**Files:**
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/refute.go:74-101` (28 lines)
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/archive.go:76-104` (29 lines)

**Similarity:** 95%

**Code Pattern (duplicated):**
```go
// Handle confirmation for destructive action
if !skipConfirm {
    stat, err := os.Stdin.Stat()
    if err != nil {
        return fmt.Errorf("stdin is not a terminal; use --yes flag...")
    }
    mode := stat.Mode()
    isTerminal := (mode & os.ModeCharDevice) != 0
    isPipe := (mode & os.ModeNamedPipe) != 0

    if !isTerminal || isPipe {
        return fmt.Errorf("stdin is not a terminal...")
    }
    // ... reader.ReadString, response handling ...
}
```

**Recommendation:** Extract to `internal/cli/confirm.go`:
```go
func ConfirmDestructiveAction(actionName, nodeID string, skipConfirm bool) error
```

**Effort:** 20 minutes
**Impact:** Eliminates 25 duplicate lines, centralizes terminal detection logic

---

### CRITICAL-DUP-002: Flag Parsing Boilerplate
**Files:** 36+ command files in `/home/tobiasosborne/Projects/vibefeld/cmd/af/`
**Occurrences:** 100 instances of `cmd.Flags().GetString()`

**Pattern:**
```go
dir, err := cmd.Flags().GetString("dir")
if err != nil {
    return err
}
format, err := cmd.Flags().GetString("format")
if err != nil {
    return err
}
```

**Recommendation:** Create flag helper structs in `internal/cli/flags.go`:
```go
type CommonFlags struct {
    Dir    string
    Format string
}

func ParseCommonFlags(cmd *cobra.Command) (CommonFlags, error)
```

**Effort:** 1.5 hours
**Impact:** Reduces boilerplate by ~300 lines across codebase

---

### HIGH-DUP-003: refute.go / archive.go Command Structure
**Files:**
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/refute.go` (140 lines)
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/archive.go` (143 lines)

**Similarity:** 85%

**Differences:**
- Command name/description
- Service method called (`RefuteNode` vs `ArchiveNode`)
- JSON output field names (`refuted` vs `archived`)

**Recommendation:** Create generic "epistemic state change" command factory:
```go
func newEpistemicChangeCmd(name, action string, svcMethod func(*service.ProofService, types.NodeID) error) *cobra.Command
```

**Effort:** 45 minutes
**Impact:** Eliminates ~100 duplicate lines

---

### HIGH-DUP-004: resolve_challenge.go / withdraw_challenge.go
**Files:**
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/resolve_challenge.go` (227 lines)
- `/home/tobiasosborne/Projects/vibefeld/cmd/af/withdraw_challenge.go` (181 lines)

**Similarity:** 90%

**Duplicated Logic (lines 82-175 in both):**
- Directory validation
- Ledger initialization
- Challenge state scanning via `ldg.Scan()`
- State validation (`exists`, `status == "resolved"`, etc.)
- JSON output formatting

**Recommendation:** Extract common challenge state machine to `internal/service/challenge_ops.go`:
```go
func (s *ProofService) ValidateChallengeState(challengeID string) (*ChallengeState, error)
func (s *ProofService) ResolveChallenge(challengeID, response string) error
func (s *ProofService) WithdrawChallenge(challengeID string) error
```

**Effort:** 1 hour
**Impact:** Eliminates ~150 duplicate lines, moves validation to service layer

---

### HIGH-DUP-005: JSON Output Formatting
**Files:** 52 files with `json.Marshal` usage (72 occurrences)
**Pattern:**
```go
switch strings.ToLower(format) {
case "json":
    result := map[string]interface{}{...}
    output, err := json.Marshal(result)
    if err != nil {
        return fmt.Errorf("error marshaling JSON: %w", err)
    }
    fmt.Fprintln(cmd.OutOrStdout(), string(output))
default:
    // Text format...
}
```

**Recommendation:** Create output formatter in `internal/render/output.go`:
```go
type OutputFormatter struct {
    Format string
    Writer io.Writer
}

func (f *OutputFormatter) Output(textFn func() string, jsonData interface{}) error
```

**Effort:** 2 hours
**Impact:** Standardizes output across all commands, ~200 lines saved

---

## 2. God Objects

### CRITICAL-GOD-001: ProofService God Object
**File:** `/home/tobiasosborne/Projects/vibefeld/internal/service/proof.go`
**Lines:** 1,980

**Exported Methods (30+):**
- Lifecycle: `Init`, `LoadState`, `Status`
- Node Operations: `CreateNode`, `ClaimNode`, `ReleaseNode`, `RefreshClaim`, `RefineNode`, `RefineNodeBulk`
- Epistemic: `AcceptNode`, `AcceptNodeBulk`, `AdmitNode`, `RefuteNode`, `ArchiveNode`
- Definitions: `AddDefinition`, `ReadPendingDef`, `WritePendingDef`, `ListPendingDefs`, `DeletePendingDef`
- Assumptions: `AddAssumption`, `ReadAssumption`, `ListAssumptions`
- Externals: `AddExternal`, `ReadExternal`, `WriteExternal`, `ListExternals`
- Lemmas: `ExtractLemma`
- Amendments: `AmendNode`, `LoadAmendmentHistory`
- Cycles: `CheckCycles`, `CheckAllCycles`, `WouldCreateCycle`
- Taint: `RecomputeAllTaint`, `emitTaintRecomputedEvents`

**Recommendation:** Split into focused services:
```
internal/service/
  proof.go          (~400 lines) - Core state, lifecycle
  node_ops.go       (~400 lines) - CreateNode, Claim, Release, Refine
  epistemic.go      (~300 lines) - Accept, Admit, Refute, Archive
  definitions.go    (~200 lines) - Definition CRUD
  references.go     (~200 lines) - Assumptions, Externals, Lemmas
  amendments.go     (~150 lines) - Amend, history
  cycles.go         (~150 lines) - Cycle detection
  taint.go          (~200 lines) - Taint computation
```

**Effort:** 4-6 hours
**Impact:**
- Improved testability (can mock individual services)
- Reduced cognitive load
- Clearer dependency boundaries
- Better parallel development

---

## 3. Code Complexity

### HIGH-COMPLEX-001: wizard.go Long Functions
**File:** `/home/tobiasosborne/Projects/vibefeld/cmd/af/wizard.go`
**Lines:** 634 total

**Problematic Functions:**
- `runWizardNewProof`: 153 lines (lines 117-269)
- `runWizardRespondChallenge`: 98 lines (lines 307-404)
- `runWizardReview`: 88 lines (lines 441-528)

**Issue:** Each function handles input, validation, preview, confirmation, and execution.

**Recommendation:** Extract into wizard step pattern:
```go
type WizardStep interface {
    Prompt(out io.Writer) string
    Validate(input string) error
    Execute(ctx *WizardContext) error
}
```

**Effort:** 2 hours
**Impact:** Testable steps, reusable prompts, ~100 lines saved

---

### MEDIUM-COMPLEX-002: refine.go Conditional Complexity
**File:** `/home/tobiasosborne/Projects/vibefeld/cmd/af/refine.go`
**Lines:** 578 total

**Issues:**
- `runRefine` function: 143 lines with 3 code paths (single, multi-JSON, positional)
- Deep nesting for input validation (4+ levels)
- Multiple helper functions that could be methods

**Recommendation:**
1. Extract input mode enum: `RefineMode{Single, MultiJSON, Positional}`
2. Create strategy pattern for each mode
3. Reduce function length to <50 lines each

**Effort:** 1.5 hours
**Impact:** Improved readability, easier testing

---

### MEDIUM-COMPLEX-003: render_views.go Size
**File:** `/home/tobiasosborne/Projects/vibefeld/internal/render/render_views.go`
**Lines:** 981

**Issue:** Single file handles all view rendering.

**Recommendation:** Split by view type:
```
internal/render/
  views_node.go      - RenderNodeView, RenderNodeViewVerbose
  views_tree.go      - RenderTreeView, renderSubtreeView
  views_status.go    - RenderStatusView, renderStatisticsView
  views_context.go   - RenderProverContextView, RenderVerifierContextView
  views_search.go    - RenderSearchResultViews
  views_helpers.go   - Color functions, sorting
```

**Effort:** 1 hour
**Impact:** Better organization, easier navigation

---

### MEDIUM-COMPLEX-004: Deep Nesting in Wizard Functions
**File:** `/home/tobiasosborne/Projects/vibefeld/cmd/af/wizard.go`
**Lines:** 148-168, 170-188

**Pattern:**
```go
if conjecture == "" {
    if interactive {
        // 4 levels deep
        if err := validateWizardConjecture(conjecture); err != nil {
            return err
        }
    }
} else {
    if err := validateWizardConjecture(conjecture); err != nil {
        return err
    }
}
```

**Recommendation:** Early returns and guard clauses:
```go
if !interactive && conjecture == "" {
    return errors.New("conjecture required in non-interactive mode")
}
if interactive && conjecture == "" {
    conjecture = readInput(...)
}
if err := validateWizardConjecture(conjecture); err != nil {
    return err
}
```

**Effort:** 30 minutes
**Impact:** Reduced cognitive complexity

---

## 4. Test Coverage Gaps

### HIGH-COV-001: service Package Low Coverage
**File:** `/home/tobiasosborne/Projects/vibefeld/internal/service/proof.go`
**Current Ratio:** 1.94x test-to-source (lowest in codebase)
**Compare:** taint (13.17x), fs (10.31x), state (7.03x)

**Missing Test Scenarios:**
1. `AcceptNodeBulk` atomicity (partial failure scenarios)
2. Concurrent modification race conditions
3. `emitTaintRecomputedEvents` failure recovery
4. Config loading edge cases

**Recommendation:** Add test file `proof_atomicity_test.go`:
```go
func TestAcceptNodeBulk_PartialFailure(t *testing.T)
func TestRefineNodeBulk_ConcurrentModification(t *testing.T)
func TestTaintEmission_FailureRecovery(t *testing.T)
```

**Effort:** 3 hours
**Impact:** Catches race conditions, validates error handling

---

### HIGH-COV-002: Missing Concurrent Modification Tests
**Files:** All service methods using `AppendIfSequence`

**Pattern Not Tested:**
```go
expectedSeq := st.LatestSeq()
// ... modifications ...
_, err = ldg.AppendIfSequence(event, expectedSeq)
return wrapSequenceMismatch(err, "operation")
```

**Missing Tests:**
- Simulated concurrent writer
- Retry behavior after `ErrConcurrentModification`
- Sequence gap detection

**Effort:** 2 hours
**Impact:** Validates ACID guarantees

---

### HIGH-COV-003: Error Injection for Filesystem Failures
**Files:** `internal/service/proof.go`, `internal/fs/*.go`

**Untested Failure Modes:**
- Disk full during ledger append
- Permission denied on proof directory
- Corrupted JSON in meta.json
- Interrupted write (partial file)

**Recommendation:** Use error injection via interface:
```go
type FileSystem interface {
    WriteAtomic(path string, data []byte) error
    // ...
}

type ErrorInjectingFS struct {
    FailOn map[string]error
}
```

**Effort:** 2 hours
**Impact:** Validates error recovery paths

---

### MEDIUM-COV-004: Challenge State Machine Edge Cases
**Files:** `cmd/af/resolve_challenge.go`, `cmd/af/withdraw_challenge.go`

**Missing Tests:**
- Challenge that doesn't exist
- Already resolved challenge
- Already withdrawn challenge
- Empty response on resolve

**Effort:** 1 hour
**Impact:** Validates user error handling

---

### MEDIUM-COV-005: Wizard Input Validation
**File:** `/home/tobiasosborne/Projects/vibefeld/cmd/af/wizard.go`

**Missing Tests:**
- EOF on stdin during prompt
- Invalid template name
- Empty conjecture/author
- Non-interactive mode without required flags

**Effort:** 1 hour
**Impact:** Validates CLI UX

---

## Quick Wins (< 30 minutes each)

| Item | File | Change | Time | Lines Saved |
|------|------|--------|------|-------------|
| 1 | refute.go:74-101 | Extract `ConfirmDestructiveAction` | 20 min | 25 |
| 2 | archive.go:76-104 | Use extracted function | 5 min | 25 |
| 3 | wizard.go:148-188 | Early returns for validation | 30 min | 0 (clarity) |
| 4 | Multiple | Add `//nolint:dupl` where intentional | 10 min | 0 (suppress noise) |

---

## Refactoring Priority Matrix

| Priority | Item | Effort | Impact | ROI |
|----------|------|--------|--------|-----|
| P0 | CRITICAL-GOD-001 (proof.go split) | 4-6h | High | High |
| P0 | HIGH-COV-001 (service tests) | 3h | High | High |
| P1 | CRITICAL-DUP-001 (confirm prompt) | 20m | Medium | Very High |
| P1 | HIGH-DUP-004 (challenge commands) | 1h | Medium | High |
| P1 | HIGH-COV-002 (concurrent tests) | 2h | High | High |
| P2 | CRITICAL-DUP-002 (flag parsing) | 1.5h | Low | Medium |
| P2 | HIGH-COMPLEX-001 (wizard steps) | 2h | Medium | Medium |
| P2 | HIGH-DUP-005 (JSON output) | 2h | Low | Medium |
| P3 | MEDIUM-COMPLEX-003 (render split) | 1h | Low | Low |

---

## Tracking Issues to Create

```bash
# Critical
bd q "REFACTOR: Split proof.go god object into focused services (1980 LOC)" --priority critical
bd q "TEST: Add service package atomicity and concurrent modification tests" --priority critical

# High
bd q "REFACTOR: Extract destructive action confirmation to internal/cli" --priority high
bd q "REFACTOR: Consolidate challenge resolve/withdraw commands" --priority high
bd q "TEST: Add error injection tests for filesystem failures" --priority high

# Medium
bd q "REFACTOR: Extract common flag parsing to internal/cli/flags.go" --priority medium
bd q "REFACTOR: Split wizard.go into step-based architecture" --priority medium
bd q "REFACTOR: Standardize JSON output formatting across commands" --priority medium
```

---

## Metrics Summary

| Metric | Current | Target | Improvement |
|--------|---------|--------|-------------|
| Duplicate code | ~2,000 lines | ~500 lines | 75% reduction |
| Largest file | 1,980 lines | <500 lines | 75% reduction |
| service test ratio | 1.94x | 5.0x | 158% increase |
| Max function length | 153 lines | <50 lines | 67% reduction |
| Files with duplication | 10+ | 2-3 | 70-80% reduction |
