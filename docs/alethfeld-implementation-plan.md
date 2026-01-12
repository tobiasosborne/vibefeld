# Alethfeld Implementation Plan

Greenfield Go implementation of the AF (Adversarial Proof Framework) CLI.

**Principles**: TDD (tests first), modules 200-300 LOC, tracer bullet first.

**Tracer Bullet Goal**: Minimal working CLI that can `init`, `status`, `claim`, `refine`, `release`, `accept` on a single node. Event sourcing works. Locks work.

---

## Phase 0: Project Bootstrap

### 1. Install Go toolchain
- Install Go 1.22+
- Verify `go version`
- **Deps**: None

### 2. Install cobra CLI generator
- `go install github.com/spf13/cobra-cli@latest`
- Verify `cobra-cli --version`
- **Deps**: 1

### 3. Initialize Go module
- `go mod init github.com/tobias/alethfeld` (or chosen module path)
- Create `go.mod`
- **Deps**: 1

### 4. Add cobra dependency
- `go get github.com/spf13/cobra@latest`
- **Deps**: 3

### 5. Create project directory structure
- Create: `cmd/af/`, `internal/` with subdirs: `ledger/`, `node/`, `lock/`, `schema/`, `jobs/`, `taint/`, `scope/`, `hash/`, `render/`, `fuzzy/`, `types/`, `errors/`, `config/`, `testutil/`
- **Deps**: 3

### 6. Create basic main.go scaffold
- `cmd/af/main.go` with cobra root command
- Just prints version, exits 0
- **Deps**: 4, 5

### 7. Verify build and run
- `go build ./cmd/af`
- `./af --version`
- **Deps**: 6

---

## Phase 1: Core Types and Error Handling

### 8. Write tests for error types
- Test file: `internal/errors/errors_test.go`
- Test error codes, exit codes, error messages
- Test `Is()` and `Unwrap()` behavior
- **Deps**: 5

### 9. Implement error types
- `internal/errors/errors.go`
- Define all error codes from PRD: `ALREADY_CLAIMED`, `NOT_CLAIM_HOLDER`, `NODE_BLOCKED`, `INVALID_PARENT`, `INVALID_TYPE`, `INVALID_INFERENCE`, etc.
- Include exit code mapping (1=retriable, 2=blocked, 3=logic, 4=corruption)
- **Deps**: 8

### 10. Write tests for ID types
- Test file: `internal/types/id_test.go`
- Test hierarchical ID parsing ("1.2.1")
- Test parent extraction, child generation
- Test validation (no gaps, proper format)
- **Deps**: 5

### 11. Implement ID types
- `internal/types/id.go`
- `NodeID` type with Parse, Parent, Child, IsRoot, Depth methods
- Validation logic
- **Deps**: 10

### 12. Write tests for timestamp handling
- Test file: `internal/types/time_test.go`
- Test ISO8601 parsing/formatting
- Test monotonic ordering
- **Deps**: 5

### 13. Implement timestamp types
- `internal/types/time.go`
- Consistent ISO8601 formatting
- **Deps**: 12

### 14. Write tests for content hash computation
- Test file: `internal/hash/hash_test.go`
- Test SHA256 computation from node fields
- Test deterministic ordering of context/dependencies
- **Deps**: 5

### 15. Implement content hash
- `internal/hash/hash.go`
- `ComputeNodeHash(type, statement, latex, inference, context, dependencies) string`
- **Deps**: 14

---

## Phase 2: Schema and Validation

### 16. Write tests for inference type validation
- Test file: `internal/schema/inference_test.go`
- Test all valid inference types from PRD
- Test invalid type rejection
- Test fuzzy matching suggestions
- **Deps**: 5, 9

### 17. Implement inference types
- `internal/schema/inference.go`
- Define all inference types with their forms
- Validation function
- **Deps**: 16

### 18. Write tests for node type validation
- Test file: `internal/schema/nodetype_test.go`
- Test: claim, local_assume, local_discharge, case, qed
- **Deps**: 5, 9

### 19. Implement node types
- `internal/schema/nodetype.go`
- Enum + validation
- **Deps**: 18

### 20. Write tests for challenge target validation
- Test file: `internal/schema/target_test.go`
- Test all valid targets: statement, inference, context, dependencies, scope, gap, type_error, domain, completeness
- **Deps**: 5, 9

### 21. Implement challenge targets
- `internal/schema/target.go`
- Enum + validation
- **Deps**: 20

### 22. Write tests for workflow state
- Test file: `internal/schema/workflow_test.go`
- Test: available, claimed, blocked
- Test valid transitions
- **Deps**: 5, 9

### 23. Implement workflow state
- `internal/schema/workflow.go`
- **Deps**: 22

### 24. Write tests for epistemic state
- Test file: `internal/schema/epistemic_test.go`
- Test: pending, validated, admitted, refuted, archived
- **Deps**: 5, 9

### 25. Implement epistemic state
- `internal/schema/epistemic.go`
- **Deps**: 24

### 26. Write tests for schema loading
- Test file: `internal/schema/schema_test.go`
- Test loading schema.json
- Test default schema
- **Deps**: 17, 19, 21, 23, 25

### 27. Implement schema loader
- `internal/schema/schema.go`
- Load from file or use defaults
- **Deps**: 26

---

## Phase 3: Fuzzy Matching

### 28. Write tests for Levenshtein distance
- Test file: `internal/fuzzy/levenshtein_test.go`
- Test distance calculation
- Test edge cases (empty strings, identical strings)
- **Deps**: 5

### 29. Implement Levenshtein distance
- `internal/fuzzy/levenshtein.go`
- Standard DP implementation
- **Deps**: 28

### 30. Write tests for fuzzy command matching
- Test file: `internal/fuzzy/match_test.go`
- Test "chalenge" → "challenge"
- Test "stauts" → "status"
- Test threshold for auto-correction vs suggestion
- **Deps**: 29

### 31. Implement fuzzy matcher
- `internal/fuzzy/match.go`
- `SuggestCommand(input string, candidates []string) (match string, autoCorrect bool)`
- **Deps**: 30

---

## Phase 4: Configuration

### 32. Write tests for config loading
- Test file: `internal/config/config_test.go`
- Test loading meta.json
- Test default values
- Test validation (lock_timeout, max_depth, etc.)
- **Deps**: 5, 9

### 33. Implement config types and loader
- `internal/config/config.go`
- `Config` struct with all fields from PRD
- Load/Save functions
- **Deps**: 32

---

## Phase 5: Node Data Model

### 34. Write tests for challenge struct
- Test file: `internal/node/challenge_test.go`
- Test challenge creation, state transitions
- Test addressed_by updates
- **Deps**: 11, 13, 21

### 35. Implement challenge struct
- `internal/node/challenge.go`
- Challenge struct with all fields
- State transition methods
- **Deps**: 34

### 36. Write tests for node struct
- Test file: `internal/node/node_test.go`
- Test node creation with all fields
- Test JSON serialization/deserialization
- Test content hash verification
- **Deps**: 11, 13, 15, 17, 19, 23, 25, 35

### 37. Implement node struct
- `internal/node/node.go`
- Node struct with all fields from PRD
- JSON tags, marshal/unmarshal
- **Deps**: 36

### 38. Write tests for definition struct
- Test file: `internal/node/definition_test.go`
- Test definition creation, immutability
- **Deps**: 13, 15

### 39. Implement definition struct
- `internal/node/definition.go`
- **Deps**: 38

### 40. Write tests for assumption struct
- Test file: `internal/node/assumption_test.go`
- **Deps**: 13, 15

### 41. Implement assumption struct
- `internal/node/assumption.go`
- **Deps**: 40

### 42. Write tests for external reference struct
- Test file: `internal/node/external_test.go`
- Test verification status transitions
- **Deps**: 13, 15

### 43. Implement external reference struct
- `internal/node/external.go`
- **Deps**: 42

### 44. Write tests for lemma struct
- Test file: `internal/node/lemma_test.go`
- **Deps**: 13, 15

### 45. Implement lemma struct
- `internal/node/lemma.go`
- **Deps**: 44

### 46. Write tests for pending definition request
- Test file: `internal/node/pending_def_test.go`
- **Deps**: 13

### 47. Implement pending definition request
- `internal/node/pending_def.go`
- **Deps**: 46

---

## Phase 6: Event Ledger (Core of Event Sourcing)

### 48. Write tests for event types
- Test file: `internal/ledger/event_test.go`
- Test all event types from PRD
- Test JSON serialization
- Test sequence number handling
- **Deps**: 11, 13, 37

### 49. Implement event types
- `internal/ledger/event.go`
- All event structs: ProofInitialized, NodeCreated, NodesClaimed, NodesReleased, ChallengeRaised, etc.
- Common Event interface/struct
- **Deps**: 48

### 50. Write tests for event filename generation
- Test file: `internal/ledger/filename_test.go`
- Test format: `000001-1736600000000-proof_initialized.json`
- Test parsing
- Test ordering
- **Deps**: 49

### 51. Implement event filename handling
- `internal/ledger/filename.go`
- Generate, parse, compare filenames
- **Deps**: 50

### 52. Write tests for ledger lock acquisition
- Test file: `internal/ledger/lock_test.go`
- Test exclusive lock via `ledger.lock`
- Test timeout behavior
- Test concurrent access blocking
- **Deps**: 5

### 53. Implement ledger lock
- `internal/ledger/lock.go`
- File-based mutex using O_CREAT|O_EXCL
- **Deps**: 52

### 54. Write tests for ledger append
- Test file: `internal/ledger/append_test.go`
- Test atomic write (tmp + rename)
- Test sequence number assignment
- Test no gaps invariant
- **Deps**: 51, 53

### 55. Implement ledger append
- `internal/ledger/append.go`
- Acquire lock, get next seq, write tmp, rename
- **Deps**: 54

### 56. Write tests for ledger read/scan
- Test file: `internal/ledger/read_test.go`
- Test reading all events
- Test reading since seq N
- Test ordering
- **Deps**: 51

### 57. Implement ledger reader
- `internal/ledger/read.go`
- Scan ledger directory, parse events
- **Deps**: 56

### 58. Write tests for ledger struct (facade)
- Test file: `internal/ledger/ledger_test.go`
- Integration tests: append, read, replay
- **Deps**: 55, 57

### 59. Implement ledger facade
- `internal/ledger/ledger.go`
- `Ledger` struct combining append/read
- **Deps**: 58

---

## Phase 7: File-based Lock Manager

### 60. Write tests for node lock acquisition
- Test file: `internal/lock/lock_test.go`
- Test O_CREAT|O_EXCL semantics
- Test lock file contents (agent ID, timestamp)
- Test already-claimed error
- **Deps**: 9, 11, 13

### 61. Implement node lock acquisition
- `internal/lock/lock.go`
- `Acquire(nodeID, agentID) error`
- **Deps**: 60

### 62. Write tests for node lock release
- Test file: `internal/lock/release_test.go`
- Test release by owner
- Test NOT_CLAIM_HOLDER error
- **Deps**: 61

### 63. Implement node lock release
- `internal/lock/release.go`
- `Release(nodeID, agentID) error`
- **Deps**: 62

### 64. Write tests for lock info reading
- Test file: `internal/lock/info_test.go`
- Test reading lock holder, timestamp
- Test non-existent lock
- **Deps**: 61

### 65. Implement lock info
- `internal/lock/info.go`
- `Info(nodeID) (*LockInfo, error)`
- **Deps**: 64

### 66. Write tests for stale lock detection
- Test file: `internal/lock/stale_test.go`
- Test timeout-based staleness
- **Deps**: 65, 33

### 67. Implement stale lock detection
- `internal/lock/stale.go`
- `IsStale(nodeID, timeout) bool`
- **Deps**: 66

### 68. Write tests for lock reaping
- Test file: `internal/lock/reap_test.go`
- Test removing stale locks
- Test generating LockReaped events
- **Deps**: 67, 59

### 69. Implement lock reaper
- `internal/lock/reap.go`
- `Reap(timeout) []ReapResult`
- **Deps**: 68

### 70. Write tests for lock manager facade
- Test file: `internal/lock/manager_test.go`
- Integration tests
- **Deps**: 61, 63, 65, 69

### 71. Implement lock manager
- `internal/lock/manager.go`
- `LockManager` struct
- **Deps**: 70

---

## Phase 8: State Derivation (Replay)

### 72. Write tests for state struct
- Test file: `internal/state/state_test.go`
- Test state holding nodes, defs, assumptions, externals, lemmas
- **Deps**: 37, 39, 41, 43, 45

### 73. Implement state struct
- `internal/state/state.go`
- In-memory derived state
- **Deps**: 72

### 74. Write tests for event application
- Test file: `internal/state/apply_test.go`
- Test applying each event type to state
- Test idempotency where applicable
- **Deps**: 49, 73

### 75. Implement event application
- `internal/state/apply.go`
- `Apply(state *State, event Event) error`
- **Deps**: 74

### 76. Write tests for full replay
- Test file: `internal/state/replay_test.go`
- Test building state from event stream
- Test verification mode (hash checking)
- **Deps**: 59, 75

### 77. Implement replay
- `internal/state/replay.go`
- `Replay(ledger *Ledger, verify bool) (*State, error)`
- **Deps**: 76

---

## Phase 9: Scope Tracking

### 78. Write tests for scope entry creation
- Test file: `internal/scope/scope_test.go`
- Test creating scope entries from local_assume nodes
- Test scope entry format "{node_id}.A"
- **Deps**: 11

### 79. Implement scope entry
- `internal/scope/scope.go`
- Scope entry type and creation
- **Deps**: 78

### 80. Write tests for scope inheritance
- Test file: `internal/scope/inherit_test.go`
- Test child inheriting parent scope
- Test scope modification on discharge
- **Deps**: 79, 37

### 81. Implement scope inheritance
- `internal/scope/inherit.go`
- `InheritScope(parent *Node, childType NodeType, discharges string) []string`
- **Deps**: 80

### 82. Write tests for scope validation
- Test file: `internal/scope/validate_test.go`
- Test dependency scope checking
- Test SCOPE_VIOLATION error
- Test SCOPE_UNCLOSED error
- **Deps**: 81, 9

### 83. Implement scope validation
- `internal/scope/validate.go`
- `ValidateDependencyScope(node, dep, state) error`
- `ValidateScopesClosed(node, state) error`
- **Deps**: 82

---

## Phase 10: Taint Propagation

### 84. Write tests for single node taint computation
- Test file: `internal/taint/compute_test.go`
- Test clean, self_admitted, tainted, unresolved states
- Test based on epistemic state and ancestor taints
- **Deps**: 25, 37

### 85. Implement single node taint computation
- `internal/taint/compute.go`
- `ComputeTaint(node, getDependencies func) Taint`
- **Deps**: 84

### 86. Write tests for taint propagation
- Test file: `internal/taint/propagate_test.go`
- Test propagation to descendants on status change
- Test TaintRecomputed event generation
- **Deps**: 85, 73

### 87. Implement taint propagation
- `internal/taint/propagate.go`
- `Propagate(state, changedNodeID) []TaintChange`
- **Deps**: 86

---

## Phase 11: Job Calculation

### 88. Write tests for prover job detection
- Test file: `internal/jobs/prover_test.go`
- Test: available + pending + (no children OR open challenges with empty addressed_by)
- **Deps**: 73, 37

### 89. Implement prover job detection
- `internal/jobs/prover.go`
- `FindProverJobs(state) []Job`
- **Deps**: 88

### 90. Write tests for verifier job detection
- Test file: `internal/jobs/verifier_test.go`
- Test: available + pending + (no open challenges with children OR all challenges have addressed_by)
- **Deps**: 73, 37

### 91. Implement verifier job detection
- `internal/jobs/verifier.go`
- `FindVerifierJobs(state) []Job`
- **Deps**: 90

### 92. Write tests for job facade
- Test file: `internal/jobs/jobs_test.go`
- Integration tests
- **Deps**: 89, 91

### 93. Implement job facade
- `internal/jobs/jobs.go`
- **Deps**: 92

---

## Phase 12: Validation Invariant

### 94. Write tests for validation invariant checking
- Test file: `internal/node/validate_invariant_test.go`
- Test: all challenges resolved/withdrawn/superseded
- Test: addressed challenges have validated addressing nodes
- Test: all children validated/admitted
- Test: all scopes closed
- **Deps**: 37, 83

### 95. Implement validation invariant checker
- `internal/node/validate_invariant.go`
- `CheckValidationInvariant(node, state) (bool, []string)`
- Returns pass/fail and list of violations
- **Deps**: 94

---

## Phase 13: Rendering

### 96. Write tests for node rendering (human-readable)
- Test file: `internal/render/node_test.go`
- Test single node display format
- **Deps**: 37

### 97. Implement node renderer
- `internal/render/node.go`
- **Deps**: 96

### 98. Write tests for tree rendering
- Test file: `internal/render/tree_test.go`
- Test tree structure with proper indentation
- Test status/taint indicators
- **Deps**: 73, 97

### 99. Implement tree renderer
- `internal/render/tree.go`
- **Deps**: 98

### 100. Write tests for claim context rendering (prover)
- Test file: `internal/render/prover_context_test.go`
- Test full prover context output as per PRD
- **Deps**: 73, 97

### 101. Implement prover context renderer
- `internal/render/prover_context.go`
- **Deps**: 100

### 102. Write tests for claim context rendering (verifier)
- Test file: `internal/render/verifier_context_test.go`
- Test full verifier context output as per PRD
- **Deps**: 73, 97

### 103. Implement verifier context renderer
- `internal/render/verifier_context.go`
- **Deps**: 102

### 104. Write tests for status rendering
- Test file: `internal/render/status_test.go`
- Test full status output with legend, summary, next steps
- **Deps**: 99, 93

### 105. Implement status renderer
- `internal/render/status.go`
- **Deps**: 104

### 106. Write tests for jobs rendering
- Test file: `internal/render/jobs_test.go`
- Test jobs list with instructions
- **Deps**: 93

### 107. Implement jobs renderer
- `internal/render/jobs.go`
- **Deps**: 106

### 108. Write tests for error rendering
- Test file: `internal/render/error_test.go`
- Test error messages with recovery suggestions
- **Deps**: 9

### 109. Implement error renderer
- `internal/render/error.go`
- **Deps**: 108

### 110. Write tests for JSON output
- Test file: `internal/render/json_test.go`
- Test --format json output for various commands
- **Deps**: 37, 93

### 111. Implement JSON renderer
- `internal/render/json.go`
- **Deps**: 110

---

## Phase 14: Filesystem Operations

### 112. Write tests for proof directory structure creation
- Test file: `internal/fs/init_test.go`
- Test creating all required directories
- **Deps**: 5

### 113. Implement proof directory initialization
- `internal/fs/init.go`
- Create: `proof/`, subdirs
- **Deps**: 112

### 114. Write tests for node file I/O
- Test file: `internal/fs/node_io_test.go`
- Test atomic write (tmp + rename)
- Test read with hash verification
- **Deps**: 37

### 115. Implement node file I/O
- `internal/fs/node_io.go`
- **Deps**: 114

### 116. Write tests for definition file I/O
- Test file: `internal/fs/def_io_test.go`
- **Deps**: 39

### 117. Implement definition file I/O
- `internal/fs/def_io.go`
- **Deps**: 116

### 118. Write tests for assumption file I/O
- Test file: `internal/fs/assumption_io_test.go`
- **Deps**: 41

### 119. Implement assumption file I/O
- `internal/fs/assumption_io.go`
- **Deps**: 118

### 120. Write tests for external reference file I/O
- Test file: `internal/fs/external_io_test.go`
- **Deps**: 43

### 121. Implement external reference file I/O
- `internal/fs/external_io.go`
- **Deps**: 120

### 122. Write tests for lemma file I/O
- Test file: `internal/fs/lemma_io_test.go`
- **Deps**: 45

### 123. Implement lemma file I/O
- `internal/fs/lemma_io.go`
- **Deps**: 122

### 124. Write tests for pending def file I/O
- Test file: `internal/fs/pending_def_io_test.go`
- **Deps**: 47

### 125. Implement pending def file I/O
- `internal/fs/pending_def_io.go`
- **Deps**: 124

### 126. Write tests for meta.json I/O
- Test file: `internal/fs/meta_io_test.go`
- **Deps**: 33

### 127. Implement meta.json I/O
- `internal/fs/meta_io.go`
- **Deps**: 126

### 128. Write tests for schema.json I/O
- Test file: `internal/fs/schema_io_test.go`
- **Deps**: 27

### 129. Implement schema.json I/O
- `internal/fs/schema_io.go`
- **Deps**: 128

---

## Phase 15: Core Service Layer

### 130. Write tests for proof service (orchestrates operations)
- Test file: `internal/service/proof_test.go`
- Test loading proof, getting state
- **Deps**: 59, 77, 71, 113-129

### 131. Implement proof service
- `internal/service/proof.go`
- `ProofService` struct combining all components
- **Deps**: 130

---

## Phase 16: CLI Commands - Tracer Bullet

This phase implements the minimal commands needed for a working proof cycle.

### 132. Write tests for root command with fuzzy matching
- Test file: `cmd/af/root_test.go`
- Test unknown command suggestions
- Test auto-correction
- **Deps**: 6, 31

### 133. Implement root command with fuzzy matching
- `cmd/af/root.go`
- **Deps**: 132

### 134. Write tests for `af init` command
- Test file: `cmd/af/init_test.go`
- Test creating proof with conjecture
- Test loading defs/assumptions from files
- Test default schema creation
- **Deps**: 131

### 135. Implement `af init` command
- `cmd/af/init.go`
- **Deps**: 134

### 136. Write tests for `af status` command
- Test file: `cmd/af/status_test.go`
- Test tree output
- Test --format json
- **Deps**: 131, 105, 111

### 137. Implement `af status` command
- `cmd/af/status.go`
- **Deps**: 136

### 138. Write tests for `af claim` command
- Test file: `cmd/af/claim_test.go`
- Test claiming for prover/verifier
- Test context output
- Test ALREADY_CLAIMED error
- **Deps**: 131, 101, 103

### 139. Implement `af claim` command
- `cmd/af/claim.go`
- **Deps**: 138

### 140. Write tests for `af release` command
- Test file: `cmd/af/release_test.go`
- Test releasing owned lock
- Test NOT_CLAIM_HOLDER error
- **Deps**: 131

### 141. Implement `af release` command
- `cmd/af/release.go`
- **Deps**: 140

### 142. Write tests for `af refine` command (single child)
- Test file: `cmd/af/refine_test.go`
- Test adding single child node
- Test validation of inference, type, dependencies
- Test content hash generation
- **Deps**: 131

### 143. Implement `af refine` command (single child)
- `cmd/af/refine.go`
- **Deps**: 142

### 144. Write tests for `af accept` command
- Test file: `cmd/af/accept_test.go`
- Test validation invariant enforcement
- Test VALIDATION_INVARIANT_FAILED with details
- **Deps**: 131, 95

### 145. Implement `af accept` command
- `cmd/af/accept.go`
- **Deps**: 144

### 146. Tracer bullet integration test
- Test file: `cmd/af/integration_test.go`
- Test: init → claim → refine → release → claim (verifier) → accept
- Full cycle on filesystem
- **Deps**: 135, 137, 139, 141, 143, 145

---

## Phase 17: CLI Commands - Job Discovery

### 147. Write tests for `af jobs` command
- Test file: `cmd/af/jobs_test.go`
- Test --role prover, --role verifier
- Test output format
- Test --format json
- **Deps**: 131, 93, 107

### 148. Implement `af jobs` command
- `cmd/af/jobs.go`
- **Deps**: 147

---

## Phase 18: CLI Commands - Verifier Actions

### 149. Write tests for `af challenge` command
- Test file: `cmd/af/challenge_test.go`
- Test challenge creation
- Test target validation
- Test objection required
- **Deps**: 131

### 150. Implement `af challenge` command
- `cmd/af/challenge.go`
- **Deps**: 149

### 151. Write tests for `af resolve-challenge` command
- Test file: `cmd/af/resolve_challenge_test.go`
- Test resolving existing challenge
- Test CHALLENGE_NOT_FOUND error
- **Deps**: 131

### 152. Implement `af resolve-challenge` command
- `cmd/af/resolve_challenge.go`
- **Deps**: 151

### 153. Write tests for `af withdraw-challenge` command
- Test file: `cmd/af/withdraw_challenge_test.go`
- **Deps**: 131

### 154. Implement `af withdraw-challenge` command
- `cmd/af/withdraw_challenge.go`
- **Deps**: 153

---

## Phase 19: CLI Commands - Prover Actions

### 155. Write tests for `af refine` command (multi-child via JSON)
- Test file: `cmd/af/refine_multi_test.go`
- Test --children file.json
- Test case splits
- **Deps**: 143

### 156. Implement `af refine` multi-child support
- Update `cmd/af/refine.go`
- **Deps**: 155

### 157. Write tests for `af request-def` command
- Test file: `cmd/af/request_def_test.go`
- Test creating pending definition request
- Test NODE_BLOCKED behavior on affected branches
- **Deps**: 131

### 158. Implement `af request-def` command
- `cmd/af/request_def.go`
- **Deps**: 157

### 159. Write tests for `af add-external` command
- Test file: `cmd/af/add_external_test.go`
- **Deps**: 131

### 160. Implement `af add-external` command
- `cmd/af/add_external.go`
- **Deps**: 159

---

## Phase 20: CLI Commands - State Reading

### 161. Write tests for `af get` command
- Test file: `cmd/af/get_test.go`
- Test basic node retrieval
- Test --ancestors, --subtree, --challenges, --context, --scope, --full
- **Deps**: 131

### 162. Implement `af get` command
- `cmd/af/get.go`
- **Deps**: 161

### 163. Write tests for `af defs` and `af def` commands
- Test file: `cmd/af/defs_test.go`
- **Deps**: 131

### 164. Implement `af defs` and `af def` commands
- `cmd/af/defs.go`
- **Deps**: 163

### 165. Write tests for `af assumptions` and `af assumption` commands
- Test file: `cmd/af/assumptions_test.go`
- **Deps**: 131

### 166. Implement `af assumptions` and `af assumption` commands
- `cmd/af/assumptions.go`
- **Deps**: 165

### 167. Write tests for `af externals` and `af external` commands
- Test file: `cmd/af/externals_test.go`
- **Deps**: 131

### 168. Implement `af externals` and `af external` commands
- `cmd/af/externals.go`
- **Deps**: 167

### 169. Write tests for `af lemmas` and `af lemma` commands
- Test file: `cmd/af/lemmas_test.go`
- **Deps**: 131

### 170. Implement `af lemmas` and `af lemma` commands
- `cmd/af/lemmas.go`
- **Deps**: 169

### 171. Write tests for `af schema` command
- Test file: `cmd/af/schema_test.go`
- **Deps**: 131

### 172. Implement `af schema` command
- `cmd/af/schema.go`
- **Deps**: 171

### 173. Write tests for `af pending-defs` command
- Test file: `cmd/af/pending_defs_test.go`
- **Deps**: 131

### 174. Implement `af pending-defs` command
- `cmd/af/pending_defs.go`
- **Deps**: 173

### 175. Write tests for `af pending-refs` command
- Test file: `cmd/af/pending_refs_test.go`
- **Deps**: 131

### 176. Implement `af pending-refs` command
- `cmd/af/pending_refs.go`
- **Deps**: 175

---

## Phase 21: CLI Commands - Escape Hatches

### 177. Write tests for `af admit` command
- Test file: `cmd/af/admit_test.go`
- Test epistemic state change
- Test taint propagation triggered
- **Deps**: 131, 87

### 178. Implement `af admit` command
- `cmd/af/admit.go`
- **Deps**: 177

### 179. Write tests for `af refute` command
- Test file: `cmd/af/refute_test.go`
- **Deps**: 131

### 180. Implement `af refute` command
- `cmd/af/refute.go`
- **Deps**: 179

### 181. Write tests for `af archive` command
- Test file: `cmd/af/archive_test.go`
- Test challenge supersession
- **Deps**: 131

### 182. Implement `af archive` command
- `cmd/af/archive.go`
- **Deps**: 181

---

## Phase 22: CLI Commands - Administration

### 183. Write tests for `af log` command
- Test file: `cmd/af/log_test.go`
- Test full log output
- Test --since N
- **Deps**: 131

### 184. Implement `af log` command
- `cmd/af/log.go`
- **Deps**: 183

### 185. Write tests for `af replay` command
- Test file: `cmd/af/replay_test.go`
- Test replay without verify
- Test replay --verify (exits 0 or 4)
- **Deps**: 131

### 186. Implement `af replay` command
- `cmd/af/replay.go`
- **Deps**: 185

### 187. Write tests for `af reap` command
- Test file: `cmd/af/reap_test.go`
- Test --older-than parsing
- Test event generation
- **Deps**: 131

### 188. Implement `af reap` command
- `cmd/af/reap.go`
- **Deps**: 187

### 189. Write tests for `af recompute-taint` command
- Test file: `cmd/af/recompute_taint_test.go`
- **Deps**: 131, 87

### 190. Implement `af recompute-taint` command
- `cmd/af/recompute_taint.go`
- **Deps**: 189

### 191. Write tests for `af def-add` command
- Test file: `cmd/af/def_add_test.go`
- Test adding definition (resolves pending request)
- **Deps**: 131

### 192. Implement `af def-add` command
- `cmd/af/def_add.go`
- **Deps**: 191

### 193. Write tests for `af def-reject` command
- Test file: `cmd/af/def_reject_test.go`
- **Deps**: 131

### 194. Implement `af def-reject` command
- `cmd/af/def_reject.go`
- **Deps**: 193

### 195. Write tests for `af verify-external` command
- Test file: `cmd/af/verify_external_test.go`
- Test status transitions
- **Deps**: 131

### 196. Implement `af verify-external` command
- `cmd/af/verify_external.go`
- **Deps**: 195

### 197. Write tests for `af extract-lemma` command
- Test file: `cmd/af/extract_lemma_test.go`
- Test independence criteria checking
- Test EXTRACTION_INVALID error
- **Deps**: 131

### 198. Implement `af extract-lemma` command
- `cmd/af/extract_lemma.go`
- **Deps**: 197

---

## Phase 23: Argument Handling and UX

### 199. Write tests for argument order independence
- Test file: `internal/cli/argparse_test.go`
- Test that flags work in any order
- **Deps**: 133

### 200. Implement argument order independence in cobra setup
- `internal/cli/argparse.go` or update command files
- **Deps**: 199

### 201. Write tests for missing argument prompting
- Test file: `internal/cli/prompt_test.go`
- Test missing required args show help
- **Deps**: 133

### 202. Implement missing argument prompting
- Add to each command
- **Deps**: 201

### 203. Write tests for fuzzy flag matching
- Test file: `internal/cli/fuzzy_flag_test.go`
- Test --agnet → --agent suggestion
- **Deps**: 31

### 204. Implement fuzzy flag matching
- `internal/cli/fuzzy_flag.go`
- **Deps**: 203

### 205. Write tests for next-step suggestions
- Test file: `internal/render/next_steps_test.go`
- Test contextual next steps after each command
- **Deps**: 131

### 206. Implement next-step suggestions
- `internal/render/next_steps.go`
- Integrate into command outputs
- **Deps**: 205

---

## Phase 24: Dependency Validation

### 207. Write tests for dependency cycle detection
- Test file: `internal/node/cycle_test.go`
- Test DEPENDENCY_CYCLE error
- **Deps**: 73

### 208. Implement dependency cycle detection
- `internal/node/cycle.go`
- **Deps**: 207

### 209. Write tests for dependency existence validation
- Test file: `internal/node/dep_validate_test.go`
- Test INVALID_DEPENDENCY error
- **Deps**: 73

### 210. Implement dependency existence validation
- `internal/node/dep_validate.go`
- **Deps**: 209

### 211. Write tests for context validation (defs, assumptions, externals)
- Test file: `internal/node/context_validate_test.go`
- Test DEF_NOT_FOUND, ASSUMPTION_NOT_FOUND, EXTERNAL_NOT_FOUND
- **Deps**: 73

### 212. Implement context validation
- `internal/node/context_validate.go`
- **Deps**: 211

---

## Phase 25: Advanced Validation

### 213. Write tests for max depth checking
- Test file: `internal/node/depth_test.go`
- Test DEPTH_EXCEEDED error
- **Deps**: 11, 33

### 214. Implement max depth checking
- `internal/node/depth.go`
- **Deps**: 213

### 215. Write tests for max challenges per node
- Test file: `internal/node/challenge_limit_test.go`
- Test CHALLENGE_LIMIT_EXCEEDED error
- **Deps**: 37, 33

### 216. Implement max challenges checking
- `internal/node/challenge_limit.go`
- **Deps**: 215

### 217. Write tests for max refinements per node
- Test file: `internal/node/refinement_limit_test.go`
- Test REFINEMENT_LIMIT_EXCEEDED error
- **Deps**: 73, 33

### 218. Implement max refinements checking
- `internal/node/refinement_limit.go`
- **Deps**: 217

---

## Phase 26: Lemma Extraction Validation

### 219. Write tests for lemma independence criteria
- Test file: `internal/lemma/independence_test.go`
- Test: all internal deps satisfied
- Test: only root depended on externally
- Test: all scopes closed within set
- Test: all nodes validated
- **Deps**: 73, 83

### 220. Implement lemma independence validation
- `internal/lemma/independence.go`
- **Deps**: 219

---

## Phase 27: Definition Blocking

### 221. Write tests for definition blocking propagation
- Test file: `internal/node/blocking_test.go`
- Test nodes become blocked when they reference pending def
- Test unblocking when def added
- **Deps**: 73, 47

### 222. Implement definition blocking
- `internal/node/blocking.go`
- **Deps**: 221

---

## Phase 28: End-to-End Tests

### 223. Write E2E test: simple proof completion
- Test file: `e2e/simple_proof_test.go`
- Full cycle: init → develop → verify → root validated
- **Deps**: 146, 148

### 224. Write E2E test: challenge and response cycle
- Test file: `e2e/challenge_cycle_test.go`
- challenge → refine to address → resolve → accept
- **Deps**: 150, 152

### 225. Write E2E test: scope tracking
- Test file: `e2e/scope_test.go`
- local_assume → children → local_discharge
- Test scope violation detection
- **Deps**: 83, 143

### 226. Write E2E test: taint propagation
- Test file: `e2e/taint_test.go`
- admit node → verify taint propagates to descendants
- **Deps**: 87, 178

### 227. Write E2E test: concurrent agents (simulated)
- Test file: `e2e/concurrent_test.go`
- Multiple agents claiming different nodes
- Test lock conflicts
- **Deps**: 139, 141

### 228. Write E2E test: definition request workflow
- Test file: `e2e/def_request_test.go`
- request-def → blocking → def-add → unblocking
- **Deps**: 158, 192, 222

### 229. Write E2E test: lemma extraction
- Test file: `e2e/lemma_extraction_test.go`
- Build valid subproof → extract → verify lemma usable
- **Deps**: 198, 220

### 230. Write E2E test: replay verification
- Test file: `e2e/replay_test.go`
- Run operations → replay --verify → ensure consistency
- **Deps**: 186

### 231. Write E2E test: stale lock reaping
- Test file: `e2e/reap_test.go`
- Create stale lock → reap → verify freed
- **Deps**: 188

---

## Phase 29: Polish and Documentation

### 232. Add --help to all commands with examples
- Update each command file
- **Deps**: All command implementations

### 233. Write completion script support (bash/zsh/fish)
- Use cobra's built-in completion generation
- **Deps**: 133

### 234. Add version command with build info
- `cmd/af/version.go`
- **Deps**: 6

### 235. Create README.md with quick start
- **Deps**: 223

### 236. Create CONTRIBUTING.md
- **Deps**: 235

---

## Dependency Graph Summary

**Critical Path to Tracer Bullet (Steps 1-146)**:
- Bootstrap: 1-7
- Types/Errors: 8-15
- Schema: 16-27
- Fuzzy: 28-31
- Config: 32-33
- Node model: 34-47
- Ledger: 48-59
- Locks: 60-71
- State/Replay: 72-77
- Scope: 78-83
- Taint: 84-87
- Jobs: 88-93
- Validation: 94-95
- Rendering: 96-111
- Filesystem: 112-129
- Service: 130-131
- CLI Tracer: 132-146

**Parallelizable Groups** (after bootstrap complete):

Group A (no deps on each other):
- 8-9 (errors)
- 10-13 (ID/time types)
- 14-15 (hash)
- 28-31 (fuzzy)
- 32-33 (config)

Group B (after Group A):
- 16-27 (schema) - needs errors
- 34-47 (node structs) - needs types, hash, schema
- 48-59 (ledger) - needs types

Group C (after Group B):
- 60-71 (locks) - needs ledger
- 72-77 (state/replay) - needs ledger, node structs
- 78-83 (scope) - needs node structs
- 84-87 (taint) - needs node structs
- 88-93 (jobs) - needs state

Group D (after state exists):
- 94-95 (validation invariant)
- 96-111 (rendering)
- 112-129 (filesystem I/O)

Group E (service layer):
- 130-131 (proof service) - needs all above

Group F (CLI commands - highly parallelizable after service):
- All CLI commands can be developed in parallel once service layer exists
- 132-145 are tracer bullet critical path
- 147-198 can be parallelized after tracer bullet

**Agents can work in parallel on**:
- Different phases once dependencies met
- Different commands within a phase
- Test file + implementation can be same agent or split
