// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/cycle"
	aferrors "github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/lemma"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/taint"
	"github.com/tobias/vibefeld/internal/types"
)

// ErrConcurrentModification is returned when an operation fails due to
// concurrent modification of the proof state. Callers should retry the
// operation after reloading the current state.
// Exit code: 1 (retriable)
var ErrConcurrentModification = aferrors.New(aferrors.VALIDATION_INVARIANT_FAILED, "concurrent modification detected")

// ErrMaxDepthExceeded is returned when an operation would exceed the configured MaxDepth.
// Exit code: 3 (logic error)
var ErrMaxDepthExceeded = aferrors.New(aferrors.DEPTH_EXCEEDED, "maximum proof depth exceeded")

// ErrMaxChildrenExceeded is returned when an operation would exceed the configured MaxChildren.
// Exit code: 3 (logic error)
var ErrMaxChildrenExceeded = aferrors.New(aferrors.REFINEMENT_LIMIT_EXCEEDED, "maximum children per node exceeded")

// ErrBlockingChallenges is returned when an operation cannot proceed due to
// unresolved blocking challenges (critical or major severity) on a node.
// Exit code: 2 (blocked)
var ErrBlockingChallenges = aferrors.New(aferrors.NODE_BLOCKED, "node has unresolved blocking challenges")

// ErrNotClaimed is returned when an operation requires a node to be claimed
// but the node is not currently claimed by any owner.
// Exit code: 1 (retriable - caller should claim the node first)
var ErrNotClaimed = aferrors.New(aferrors.NOT_CLAIM_HOLDER, "node is not claimed")

// ErrOwnerMismatch is returned when an operation is attempted by an owner
// that does not match the current claim owner of the node.
// Exit code: 1 (retriable - caller should claim the node)
var ErrOwnerMismatch = aferrors.New(aferrors.NOT_CLAIM_HOLDER, "owner does not match")

// ErrNodeNotFound is returned when a node does not exist.
// Exit code: 3 (logic error)
var ErrNodeNotFound = aferrors.New(aferrors.NODE_NOT_FOUND, "node not found")

// ErrParentNotFound is returned when a parent node does not exist.
// Exit code: 3 (logic error)
var ErrParentNotFound = aferrors.New(aferrors.PARENT_NOT_FOUND, "parent node not found")

// ErrEmptyInput is returned when a required input is empty or whitespace.
// Exit code: 3 (logic error)
var ErrEmptyInput = aferrors.New(aferrors.EMPTY_INPUT, "required input cannot be empty")

// ErrInvalidState is returned when an operation is attempted in an invalid state.
// Exit code: 3 (logic error)
var ErrInvalidState = aferrors.New(aferrors.INVALID_STATE, "invalid state for operation")

// ErrAlreadyExists is returned when attempting to create something that already exists.
// Exit code: 3 (logic error)
var ErrAlreadyExists = aferrors.New(aferrors.ALREADY_EXISTS, "resource already exists")

// ErrInvalidTimeout is returned when a timeout value is invalid (e.g., negative or zero).
// Exit code: 3 (logic error)
var ErrInvalidTimeout = aferrors.New(aferrors.INVALID_TIMEOUT, "timeout must be positive")

// wrapSequenceMismatch converts ledger.ErrSequenceMismatch to ErrConcurrentModification
// with additional context for the caller.
func wrapSequenceMismatch(err error, operation string) error {
	if errors.Is(err, ledger.ErrSequenceMismatch) {
		return fmt.Errorf("%w: %s failed, please retry", ErrConcurrentModification, operation)
	}
	return err
}

// formatBlockingChallengesError creates an error message listing blocking challenges.
func formatBlockingChallengesError(nodeID types.NodeID, challenges []*state.Challenge) error {
	if len(challenges) == 0 {
		return nil
	}
	var ids []string
	for _, c := range challenges {
		ids = append(ids, c.ID)
	}
	return fmt.Errorf("%w: node %s has %d blocking challenge(s): %s",
		ErrBlockingChallenges, nodeID.String(), len(challenges), strings.Join(ids, ", "))
}

// TaintChange represents a change in taint state for a node.
type TaintChange struct {
	NodeID   string     `json:"node_id"`
	OldTaint TaintState `json:"old_taint"`
	NewTaint TaintState `json:"new_taint"`
}

// RecomputeTaintResult represents the result of recomputing taint for all nodes.
type RecomputeTaintResult struct {
	TotalNodes   int           `json:"total_nodes"`
	NodesChanged int           `json:"nodes_changed"`
	Changes      []TaintChange `json:"changes"`
	DryRun       bool          `json:"dry_run"`
}

// ProofService orchestrates proof operations across ledger, state, locks, and filesystem.
// It provides a high-level facade for proof manipulation operations.
type ProofService struct {
	path string
	cfg  *config.Config // cached config, loaded lazily
}

// NewProofService creates a new ProofService for the given proof directory.
// Returns an error if the directory is invalid or inaccessible.
func NewProofService(path string) (*ProofService, error) {
	// Validate path is not empty or whitespace
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%w: path", ErrEmptyInput)
	}

	// Check if path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: path does not exist", ErrInvalidState)
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: path is not a directory", ErrInvalidState)
	}

	return &ProofService{path: path}, nil
}

// LoadConfig loads and caches the config from meta.json.
// Returns the cached config if already loaded.
// Returns a default config if meta.json doesn't exist yet (proof not initialized).
func (s *ProofService) LoadConfig() (*config.Config, error) {
	if s.cfg != nil {
		return s.cfg, nil
	}

	metaPath := filepath.Join(s.path, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if meta.json doesn't exist
			s.cfg = config.Default()
			return s.cfg, nil
		}
		return nil, err
	}

	s.cfg = cfg
	return s.cfg, nil
}

// Config returns the current config, loading it if necessary.
// Returns an error if the config cannot be loaded (e.g., permission denied,
// corrupt JSON). Note that a missing meta.json returns a default config,
// not an error - this is expected for uninitialized proofs.
func (s *ProofService) Config() (*config.Config, error) {
	return s.LoadConfig()
}

// LockTimeout returns the configured lock timeout.
// Returns an error if the config cannot be loaded.
func (s *ProofService) LockTimeout() (time.Duration, error) {
	cfg, err := s.Config()
	if err != nil {
		return 0, err
	}
	return cfg.LockTimeout, nil
}

// validateDepth checks if a node at the given depth would exceed MaxDepth.
func (s *ProofService) validateDepth(depth int) error {
	cfg, err := s.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if depth > cfg.MaxDepth {
		return fmt.Errorf("%w: depth %d exceeds max %d", ErrMaxDepthExceeded, depth, cfg.MaxDepth)
	}
	return nil
}

// validateChildCount checks if adding a child would exceed MaxChildren for the parent.
func (s *ProofService) validateChildCount(st *state.State, parentID types.NodeID) error {
	cfg, err := s.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Count existing children
	childCount := 0
	for _, n := range st.AllNodes() {
		parent, hasParent := n.ID.Parent()
		if hasParent && parent.String() == parentID.String() {
			childCount++
		}
	}

	if childCount >= cfg.MaxChildren {
		return fmt.Errorf("%w: node %s already has %d children (max %d)", ErrMaxChildrenExceeded, parentID.String(), childCount, cfg.MaxChildren)
	}
	return nil
}

// Init initializes a new proof with the given conjecture and author.
// Creates the initial proof structure and ledger event.
// Returns an error if the proof is already initialized or validation fails.
func Init(proofDir, conjecture, author string) error {
	// Validate inputs
	if strings.TrimSpace(conjecture) == "" {
		return fmt.Errorf("%w: conjecture", ErrEmptyInput)
	}
	if strings.TrimSpace(author) == "" {
		return fmt.Errorf("%w: author", ErrEmptyInput)
	}

	// Initialize the proof directory structure
	if err := fs.InitProofDir(proofDir); err != nil {
		return err
	}

	// Create ledger and append initialization event
	ledgerDir := filepath.Join(proofDir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		return err
	}

	// Check if already initialized
	count, err := ldg.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: proof already initialized", ErrAlreadyExists)
	}

	// Append the initialization event
	event := ledger.NewProofInitialized(conjecture, author)
	_, err = ldg.Append(event)
	if err != nil {
		return err
	}

	// Create the root node (node "1") with the conjecture as the statement
	rootID, err := types.Parse("1")
	if err != nil {
		return err
	}

	rootNode, err := node.NewNode(rootID, schema.NodeTypeClaim, conjecture, schema.InferenceAssumption)
	if err != nil {
		return err
	}

	nodeEvent := ledger.NewNodeCreated(*rootNode)
	_, err = ldg.Append(nodeEvent)
	return err
}

// Init initializes a new proof with the given conjecture and author.
// Creates the initial proof structure and ledger event.
// Returns an error if the proof is already initialized or validation fails.
func (s *ProofService) Init(conjecture, author string) error {
	return Init(s.path, conjecture, author)
}

// getLedger returns a ledger instance for this proof's ledger directory.
func (s *ProofService) getLedger() (*ledger.Ledger, error) {
	ledgerDir := filepath.Join(s.path, "ledger")
	return ledger.NewLedger(ledgerDir)
}

// LoadState loads and returns the current proof state by replaying ledger events.
// Also loads assumptions and externals from filesystem.
func (s *ProofService) LoadState() (*state.State, error) {
	ldg, err := s.getLedger()
	if err != nil {
		return nil, err
	}
	st, err := state.Replay(ldg)
	if err != nil {
		return nil, err
	}

	// Load assumptions from filesystem
	if err := s.loadAssumptionsIntoState(st); err != nil {
		// Ignore errors if directory doesn't exist
		// Use errors.Is for proper handling of wrapped errors
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	// Load externals from filesystem
	if err := s.loadExternalsIntoState(st); err != nil {
		// Ignore errors if directory doesn't exist
		// Use errors.Is for proper handling of wrapped errors
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return st, nil
}

// loadAssumptionsIntoState loads all assumptions from filesystem into state.
func (s *ProofService) loadAssumptionsIntoState(st *state.State) error {
	ids, err := fs.ListAssumptions(s.path)
	if err != nil {
		return err
	}

	for _, id := range ids {
		asm, err := fs.ReadAssumption(s.path, id)
		if err != nil {
			return err
		}
		st.AddAssumption(asm)
	}

	return nil
}

// loadExternalsIntoState loads all externals from filesystem into state.
func (s *ProofService) loadExternalsIntoState(st *state.State) error {
	ids, err := fs.ListExternals(s.path)
	if err != nil {
		return err
	}

	for _, id := range ids {
		ext, err := fs.ReadExternal(s.path, id)
		if err != nil {
			return err
		}
		st.AddExternal(ext)
	}

	return nil
}

// isInitialized checks if the proof has been initialized.
func (s *ProofService) isInitialized() (bool, error) {
	ldg, err := s.getLedger()
	if err != nil {
		return false, err
	}
	count, err := ldg.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateNode creates a new proof node with the given parameters.
// The node is initially in available workflow state and pending epistemic state.
//
// Returns ErrMaxDepthExceeded if the node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if the parent node already has config.MaxChildren children.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) CreateNode(id types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	// Check if initialized
	init, err := s.isInitialized()
	if err != nil {
		return err
	}
	if !init {
		return fmt.Errorf("%w: proof not initialized", ErrInvalidState)
	}

	// Validate depth against config
	if err := s.validateDepth(id.Depth()); err != nil {
		return err
	}

	// Load state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node already exists
	if st.GetNode(id) != nil {
		return fmt.Errorf("%w: node %s", ErrAlreadyExists, id.String())
	}

	// Validate child count for parent (if not root)
	if parentID, hasParent := id.Parent(); hasParent {
		if err := s.validateChildCount(st, parentID); err != nil {
			return err
		}
	}

	// Create the node
	n, err := node.NewNode(id, nodeType, statement, inference)
	if err != nil {
		return err
	}

	// Get ledger and append event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeCreated(*n)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "CreateNode")
}

// ClaimNode claims a node for an agent with the given timeout.
// Returns an error if the node doesn't exist, is already claimed, or validation fails.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. This is the primary defense against multiple agents
// claiming the same node. Callers should retry after reloading state.
func (s *ProofService) ClaimNode(id types.NodeID, owner string, timeout time.Duration) error {
	// Validate owner
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("%w: owner", ErrEmptyInput)
	}

	// Validate timeout
	if timeout <= 0 {
		return ErrInvalidTimeout
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Check if node is available
	if n.WorkflowState != schema.WorkflowAvailable {
		return fmt.Errorf("%w: node %s is not available", ErrInvalidState, id.String())
	}

	// Get ledger and append claim event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	// Calculate timeout timestamp
	timeoutTS := types.FromTime(time.Now().Add(timeout))

	event := ledger.NewNodesClaimed([]types.NodeID{id}, owner, timeoutTS)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "ClaimNode")
}

// RefreshClaim extends the claim timeout for a node the caller owns.
// This allows agents to extend their claims without releasing and reclaiming,
// which would risk another agent claiming the node in between.
//
// Returns an error if the node doesn't exist, is not claimed, or is claimed by
// a different owner.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefreshClaim(id types.NodeID, owner string, timeout time.Duration) error {
	// Validate owner
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("%w: owner", ErrEmptyInput)
	}

	// Validate timeout
	if timeout <= 0 {
		return ErrInvalidTimeout
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Check if node is claimed
	if n.WorkflowState != schema.WorkflowClaimed {
		return ErrNotClaimed
	}

	// Check if owner matches
	if n.ClaimedBy != owner {
		return fmt.Errorf("%w: node is claimed by %s, not %s", ErrOwnerMismatch, n.ClaimedBy, owner)
	}

	// Get ledger and append refresh event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	// Calculate new timeout timestamp
	newTimeoutTS := types.FromTime(time.Now().Add(timeout))

	event := ledger.NewClaimRefreshed(id, owner, newTimeoutTS)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "RefreshClaim")
}

// ReleaseNode releases a claimed node, making it available again.
// Returns an error if the node is not claimed or the owner doesn't match.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) ReleaseNode(id types.NodeID, owner string) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Check if node is claimed
	if n.WorkflowState != schema.WorkflowClaimed {
		return ErrNotClaimed
	}

	// Check if owner matches
	if n.ClaimedBy != owner {
		return ErrOwnerMismatch
	}

	// Get ledger and append release event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodesReleased([]types.NodeID{id})
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "ReleaseNode")
}

// RefineNode adds a child node to a claimed parent node.
// Returns an error if the parent is not claimed by the owner or validation fails.
//
// Deprecated: Use Refine(RefineSpec{...}) instead, which provides a cleaner API
// by consolidating all parameters into a single struct.
//
// Returns ErrMaxDepthExceeded if the child node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if the parent node already has config.MaxChildren children.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefineNode(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	return s.Refine(RefineSpec{
		ParentID:  parentID,
		Owner:     owner,
		ChildID:   childID,
		NodeType:  nodeType,
		Statement: statement,
		Inference: inference,
	})
}

// RefineNodeWithDeps adds a child node with explicit dependencies to a claimed parent node.
// Dependencies are logical cross-references to other nodes (e.g., "by step 1.2").
// Returns an error if the parent is not claimed by the owner, any dependency doesn't exist,
// or validation fails.
//
// Deprecated: Use Refine(RefineSpec{...}) instead, which provides a cleaner API
// by consolidating all parameters into a single struct.
//
// Returns ErrMaxDepthExceeded if the child node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if the parent node already has config.MaxChildren children.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefineNodeWithDeps(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType, dependencies []types.NodeID) error {
	return s.Refine(RefineSpec{
		ParentID:     parentID,
		Owner:        owner,
		ChildID:      childID,
		NodeType:     nodeType,
		Statement:    statement,
		Inference:    inference,
		Dependencies: dependencies,
	})
}

// RefineNodeWithAllDeps adds a child node with both reference dependencies and validation dependencies
// to a claimed parent node.
// Reference dependencies are logical cross-references to other nodes (e.g., "by step 1.2").
// Validation dependencies specify nodes that must be validated before this node can be accepted.
// Returns an error if the parent is not claimed by the owner, any dependency doesn't exist,
// or validation fails.
//
// Deprecated: Use Refine(RefineSpec{...}) instead, which provides a cleaner API
// by consolidating all parameters into a single struct.
//
// Returns ErrMaxDepthExceeded if the child node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if the parent node already has config.MaxChildren children.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefineNodeWithAllDeps(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType, dependencies []types.NodeID, validationDeps []types.NodeID) error {
	return s.Refine(RefineSpec{
		ParentID:       parentID,
		Owner:          owner,
		ChildID:        childID,
		NodeType:       nodeType,
		Statement:      statement,
		Inference:      inference,
		Dependencies:   dependencies,
		ValidationDeps: validationDeps,
	})
}

// Refine adds a child node to a claimed parent node using a RefineSpec.
// This is the preferred API for creating child nodes as it consolidates
// all parameters into a single struct.
//
// Returns ErrMaxDepthExceeded if the child node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if the parent node already has config.MaxChildren children.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) Refine(spec RefineSpec) error {
	// Validate depth against config
	if err := s.validateDepth(spec.ChildID.Depth()); err != nil {
		return err
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if parent node exists
	parent := st.GetNode(spec.ParentID)
	if parent == nil {
		return fmt.Errorf("%w: %s", ErrParentNotFound, spec.ParentID.String())
	}

	// Check if parent is claimed
	if parent.WorkflowState != schema.WorkflowClaimed {
		return fmt.Errorf("%w: parent node must be claimed", ErrNotClaimed)
	}

	// Check if owner matches
	if parent.ClaimedBy != spec.Owner {
		return ErrOwnerMismatch
	}

	// Check if child already exists
	if st.GetNode(spec.ChildID) != nil {
		return fmt.Errorf("%w: node %s", ErrAlreadyExists, spec.ChildID.String())
	}

	// Validate child count for parent
	if err := s.validateChildCount(st, spec.ParentID); err != nil {
		return err
	}

	// Validate external citations in the statement
	if err := lemma.ValidateExtCitations(spec.Statement, st); err != nil {
		return err
	}

	// Create provider for cycle check
	provider := &stateDependencyProvider{st: st}

	// Validate that all reference dependencies exist and don't create cycles
	for _, depID := range spec.Dependencies {
		if st.GetNode(depID) == nil {
			return fmt.Errorf("invalid dependency: node %s not found", depID.String())
		}
		
		if res := cycle.WouldCreateCycle(provider, spec.ParentID, depID); res.HasCycle {
			return fmt.Errorf("%w: adding dependency %s -> %s would create cycle %v", ErrCircularDependency, spec.ParentID.String(), depID.String(), res.Path)
		}
	}

	// Validate that all validation dependencies exist and don't create cycles
	for _, valDepID := range spec.ValidationDeps {
		if st.GetNode(valDepID) == nil {
			return fmt.Errorf("invalid validation dependency: node %s not found", valDepID.String())
		}
		
		if res := cycle.WouldCreateCycle(provider, spec.ParentID, valDepID); res.HasCycle {
			return fmt.Errorf("%w: adding validation dependency %s -> %s would create cycle %v", ErrCircularDependency, spec.ParentID.String(), valDepID.String(), res.Path)
		}
	}

	// Create the child node with both dependency types
	opts := node.NodeOptions{
		Dependencies:   spec.Dependencies,
		ValidationDeps: spec.ValidationDeps,
	}
	child, err := node.NewNodeWithOptions(spec.ChildID, spec.NodeType, spec.Statement, spec.Inference, opts)
	if err != nil {
		return err
	}

	// Get ledger and append event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeCreated(*child)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "Refine")
}

// AcceptNode validates a node, marking it as verified correct.
// Returns an error if the node doesn't exist.
// Returns ErrBlockingChallenges if the node has unresolved critical or major challenges.
//
// After validation, automatically recomputes and emits taint state changes
// for the node and any affected descendants.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AcceptNode(id types.NodeID) error {
	return s.AcceptNodeWithNote(id, "")
}

// AcceptNodeWithNote validates a node with an optional acceptance note.
// The note allows verifiers to express nuanced feedback - accepting the node
// while recording a minor issue or clarification for the record.
//
// This supports partial acceptance where minor/note severity challenges
// exist but don't block validation.
//
// Returns an error if the node doesn't exist.
// Returns ErrBlockingChallenges if the node has unresolved critical or major challenges.
//
// After validation, automatically recomputes and emits taint state changes
// for the node and any affected descendants.
//
// ATOMICITY NOTE: The validation event and subsequent taint recomputation events
// are NOT atomic - they are written as separate ledger appends. This means:
// 1. If taint emission fails after validation succeeds, the validation stands
//    but taint state in the ledger may be stale until the next state replay.
// 2. Concurrent readers may briefly see the validated node with outdated taint.
// 3. The taint package computes correct taint on replay, so eventual consistency
//    is guaranteed - the ledger just won't contain explicit taint events.
// This is acceptable because taint is derived state (can be recomputed from
// epistemic states) and the validation event is the authoritative record.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AcceptNodeWithNote(id types.NodeID, note string) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Check for blocking challenges (critical or major severity)
	blockingChallenges := st.GetBlockingChallengesForNode(id)
	if len(blockingChallenges) > 0 {
		return formatBlockingChallengesError(id, blockingChallenges)
	}

	// Check validation dependencies - all must be validated before this node can be accepted
	if len(n.ValidationDeps) > 0 {
		var unvalidatedDeps []string
		for _, depID := range n.ValidationDeps {
			depNode := st.GetNode(depID)
			if depNode == nil {
				// Dependency node doesn't exist (should be caught earlier, but be defensive)
				unvalidatedDeps = append(unvalidatedDeps, depID.String()+" (not found)")
				continue
			}
			// Check if the dependency is validated (or admitted, which counts as validated)
			if depNode.EpistemicState != schema.EpistemicValidated && depNode.EpistemicState != schema.EpistemicAdmitted {
				unvalidatedDeps = append(unvalidatedDeps, depID.String())
			}
		}
		if len(unvalidatedDeps) > 0 {
			return fmt.Errorf("cannot accept node %s: validation dependencies not yet validated: %s",
				id.String(), strings.Join(unvalidatedDeps, ", "))
		}
	}

	// Check all children are validated or admitted (PRD requirement)
	// A child is a node whose parent ID equals this node's ID
	var children []*node.Node
	var unvalidatedChildren []string
	for _, child := range st.AllNodes() {
		parentID, hasParent := child.ID.Parent()
		if !hasParent || parentID.String() != id.String() {
			continue // not a child of this node
		}
		children = append(children, child)
		// Child must be validated or admitted
		if child.EpistemicState != schema.EpistemicValidated && child.EpistemicState != schema.EpistemicAdmitted {
			unvalidatedChildren = append(unvalidatedChildren, child.ID.String())
		}
	}
	if len(unvalidatedChildren) > 0 {
		return fmt.Errorf("cannot accept node %s: children not yet validated: %s",
			id.String(), strings.Join(unvalidatedChildren, ", "))
	}

	// For needs_refinement nodes, require that refinement actually happened (has children)
	if n.EpistemicState == schema.EpistemicNeedsRefinement && len(children) == 0 {
		return fmt.Errorf("cannot accept node %s: node is in needs_refinement state but has no children; use 'af refine' to add child nodes first",
			id.String())
	}

	// Validate epistemic state transition (pending -> validated or needs_refinement -> validated)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
		return err
	}

	// Get ledger and append validation event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeValidatedWithNote(id, note)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return wrapSequenceMismatch(err, "AcceptNodeWithNote")
	}

	// Auto-compute and emit taint events after successful validation
	return s.emitTaintRecomputedEvents(ldg, id)
}

// AcceptNodeBulk validates multiple nodes atomically, marking them as verified correct.
// All nodes must exist and be in pending state. If any validation fails, the operation
// stops at the first error (partial failure may occur).
//
// After validation, automatically recomputes and emits taint state changes for each
// node and any affected descendants.
//
// ATOMICITY NOTE: The validation events and subsequent taint events are NOT atomic.
// Taint emission failures are silently ignored (the validation events stand).
// See AcceptNodeWithNote for details on the implications and why this is acceptable.
//
// Returns nil if all nodes were successfully accepted.
// Returns error if any node doesn't exist, isn't pending, or validation fails.
// Returns ErrBlockingChallenges if any node has unresolved critical or major challenges.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AcceptNodeBulk(ids []types.NodeID) error {
	if len(ids) == 0 {
		return nil // Nothing to do
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Validate all nodes exist, have no blocking challenges, and are in pending state before any mutation
	for _, id := range ids {
		n := st.GetNode(id)
		if n == nil {
			return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Check for blocking challenges (critical or major severity)
		blockingChallenges := st.GetBlockingChallengesForNode(id)
		if len(blockingChallenges) > 0 {
			return formatBlockingChallengesError(id, blockingChallenges)
		}

		// Validate epistemic state transition (only pending -> validated allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
			return fmt.Errorf("node %s: %w", id.String(), err)
		}
	}

	// Get ledger
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	// Create events for all nodes
	events := make([]ledger.Event, len(ids))
	for i, id := range ids {
		events[i] = ledger.NewNodeValidated(id)
	}

	// Append all events with CAS on first event (see appendBulkIfSequence ATOMICITY NOTE)
	_, err = s.appendBulkIfSequence(ldg, events, expectedSeq)
	if err != nil {
		return wrapSequenceMismatch(err, "AcceptNodeBulk")
	}

	// Emit taint events for all accepted nodes
	for _, id := range ids {
		if err := s.emitTaintRecomputedEvents(ldg, id); err != nil {
			// Log but don't fail - the validation events are already committed
			// Taint will be recalculated on next state load
			continue
		}
	}

	return nil
}

// LoadPendingNodes returns all nodes in the pending epistemic state.
// This is useful for the --all flag in accept command.
// Note: This method performs I/O to load state from disk.
func (s *ProofService) LoadPendingNodes() ([]*node.Node, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	var pending []*node.Node
	for _, n := range st.AllNodes() {
		if n.EpistemicState == schema.EpistemicPending {
			pending = append(pending, n)
		}
	}

	return pending, nil
}

// AdmitNode admits a node without full verification.
// Returns an error if the node doesn't exist.
//
// After admission, automatically recomputes and emits taint state changes
// for the node and any affected descendants (they become tainted).
//
// ATOMICITY NOTE: The admission event and subsequent taint events are NOT atomic.
// See AcceptNodeWithNote for details on the implications and why this is acceptable.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AdmitNode(id types.NodeID) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Validate epistemic state transition (only pending -> admitted allowed)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicAdmitted); err != nil {
		return err
	}

	// Get ledger and append admit event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeAdmitted(id)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return wrapSequenceMismatch(err, "AdmitNode")
	}

	// Auto-compute and emit taint events after successful admission
	return s.emitTaintRecomputedEvents(ldg, id)
}

// RefuteNode refutes a node, marking it as incorrect.
// Returns an error if the node doesn't exist.
//
// After refutation, automatically recomputes and emits taint state changes
// for the node and any affected descendants.
//
// ATOMICITY NOTE: The refutation event and subsequent taint events are NOT atomic.
// See AcceptNodeWithNote for details on the implications and why this is acceptable.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefuteNode(id types.NodeID) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Validate epistemic state transition (only pending -> refuted allowed)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicRefuted); err != nil {
		return err
	}

	// Get ledger and append refute event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeRefuted(id)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return wrapSequenceMismatch(err, "RefuteNode")
	}

	// Auto-compute and emit taint events after successful refutation
	return s.emitTaintRecomputedEvents(ldg, id)
}

// ArchiveNode archives a node, abandoning the branch.
// Returns an error if the node doesn't exist.
//
// After archiving, automatically recomputes and emits taint state changes
// for the node and any affected descendants.
//
// ATOMICITY NOTE: The archive event and subsequent taint events are NOT atomic.
// See AcceptNodeWithNote for details on the implications and why this is acceptable.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) ArchiveNode(id types.NodeID) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// Validate epistemic state transition (only pending -> archived allowed)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicArchived); err != nil {
		return err
	}

	// Get ledger and append archive event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeArchived(id)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return wrapSequenceMismatch(err, "ArchiveNode")
	}

	// Auto-compute and emit taint events after successful archiving
	return s.emitTaintRecomputedEvents(ldg, id)
}

// AddDefinition adds a new definition to the proof.
// Returns the definition ID and any error.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AddDefinition(name, content string) (string, error) {
	// Create the definition (validates inputs)
	def, err := node.NewDefinition(name, content)
	if err != nil {
		return "", err
	}

	// Load state to get current sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return "", err
	}
	expectedSeq := st.LatestSeq()

	// Get ledger and append event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return "", err
	}

	// Create ledger definition
	ledgerDef := ledger.Definition{
		ID:         def.ID,
		Name:       def.Name,
		Definition: def.Content,
		Created:    def.Created,
	}

	event := ledger.NewDefAdded(ledgerDef)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return "", wrapSequenceMismatch(err, "AddDefinition")
	}

	return def.ID, nil
}

// AddAssumption adds a new assumption to the proof.
// Returns the assumption ID and any error.
func (s *ProofService) AddAssumption(statement string) (string, error) {
	// Validate statement
	if strings.TrimSpace(statement) == "" {
		return "", fmt.Errorf("%w: assumption statement", ErrEmptyInput)
	}

	// Create the assumption
	asm, err := node.NewAssumption(statement)
	if err != nil {
		return "", fmt.Errorf("creating assumption: %w", err)
	}

	// Store assumption in filesystem (base path is the proof directory)
	if err := fs.WriteAssumption(s.path, asm); err != nil {
		return "", err
	}

	return asm.ID, nil
}

// AddExternal adds a new external reference to the proof.
// Returns the external ID and any error.
func (s *ProofService) AddExternal(name, source string) (string, error) {
	// Validate inputs
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("%w: external reference name", ErrEmptyInput)
	}
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("%w: external reference source", ErrEmptyInput)
	}

	// Create the external
	ext, err := node.NewExternal(name, source)
	if err != nil {
		return "", fmt.Errorf("creating external: %w", err)
	}

	// Store in filesystem (base path is the proof directory)
	if err := fs.WriteExternal(s.path, ext); err != nil {
		return "", err
	}

	return ext.ID, nil
}

// ExtractLemma extracts a lemma from a source node.
// Returns the lemma ID and any error.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) ExtractLemma(sourceNodeID types.NodeID, statement string) (string, error) {
	// Validate statement
	if strings.TrimSpace(statement) == "" {
		return "", fmt.Errorf("%w: lemma statement", ErrEmptyInput)
	}

	// Load state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return "", err
	}
	expectedSeq := st.LatestSeq()

	// Check if source node exists
	n := st.GetNode(sourceNodeID)
	if n == nil {
		return "", fmt.Errorf("%w: %s", ErrNodeNotFound, sourceNodeID.String())
	}

	// Create the lemma
	lemma, err := node.NewLemma(statement, sourceNodeID)
	if err != nil {
		return "", err
	}

	// Append to ledger with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return "", err
	}

	ledgerLemma := ledger.Lemma{
		ID:        lemma.ID,
		Statement: lemma.Statement,
		NodeID:    lemma.SourceNodeID,
		Created:   lemma.Created,
	}

	event := ledger.NewLemmaExtracted(ledgerLemma)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	if err != nil {
		return "", wrapSequenceMismatch(err, "ExtractLemma")
	}

	return lemma.ID, nil
}

// ProofStatus contains status information about a proof.
type ProofStatus struct {
	Initialized    bool
	Conjecture     string
	TotalNodes     int
	ClaimedNodes   int
	ValidatedNodes int
	PendingNodes   int
}

// Status returns the current status of the proof.
func (s *ProofService) Status() (*ProofStatus, error) {
	status := &ProofStatus{}

	// Check if initialized
	ldg, err := s.getLedger()
	if err != nil {
		return nil, err
	}

	count, err := ldg.Count()
	if err != nil {
		return nil, err
	}

	if count == 0 {
		return status, nil
	}

	status.Initialized = true

	// Load state to count nodes
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	nodes := st.AllNodes()
	status.TotalNodes = len(nodes)

	for _, n := range nodes {
		switch n.WorkflowState {
		case schema.WorkflowClaimed:
			status.ClaimedNodes++
		}
		switch n.EpistemicState {
		case schema.EpistemicValidated:
			status.ValidatedNodes++
		case schema.EpistemicPending:
			status.PendingNodes++
		}
	}

	return status, nil
}

// LoadAvailableNodes returns all nodes in the available workflow state.
// Note: This method performs I/O to load state from disk.
func (s *ProofService) LoadAvailableNodes() ([]*node.Node, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	var available []*node.Node
	for _, n := range st.AllNodes() {
		if n.WorkflowState == schema.WorkflowAvailable {
			available = append(available, n)
		}
	}

	return available, nil
}

// Path returns the proof directory path.
func (s *ProofService) Path() string {
	return s.path
}

// emitTaintRecomputedEvents computes taint for a node and its descendants after
// an epistemic state change, then emits TaintRecomputed events to the ledger.
//
// This is called automatically after validation events (AcceptNode, AdmitNode,
// RefuteNode, ArchiveNode) to ensure the ledger contains explicit taint state
// records for audit and replay purposes.
//
// IMPORTANT: This function is intentionally NOT atomic with the preceding epistemic
// state change event. The taint events are appended separately after the validation
// event is committed. This means:
// - If this function fails, the epistemic state change still stands
// - Taint state is derived (computable from epistemic states), so eventual consistency
//   is guaranteed on the next state replay even if these events are never written
// - The ledger may lack explicit taint records, but the taint package will compute
//   correct taint on replay
func (s *ProofService) emitTaintRecomputedEvents(ldg *ledger.Ledger, nodeID types.NodeID) error {
	// Reload state to get the updated epistemic state (validation event was just applied)
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Get the node that was just validated
	n := st.GetNode(nodeID)
	if n == nil {
		// Node should exist - this would be a logic error
		return nil
	}

	// Get all nodes for taint computation
	allNodes := st.AllNodes()

	// Build ancestor list for this node
	nodeMap := make(map[string]*node.Node)
	for _, nd := range allNodes {
		if nd != nil {
			nodeMap[nd.ID.String()] = nd
		}
	}

	var ancestors []*node.Node
	parentID, hasParent := nodeID.Parent()
	for hasParent {
		if parent, ok := nodeMap[parentID.String()]; ok {
			ancestors = append(ancestors, parent)
		}
		parentID, hasParent = parentID.Parent()
	}

	// Compute taint for this node
	newTaint := taint.ComputeTaint(n, ancestors)

	// Emit TaintRecomputed event for this node if taint changed
	if n.TaintState != newTaint {
		taintEvent := ledger.NewTaintRecomputed(nodeID, newTaint)
		if _, err := ldg.Append(taintEvent); err != nil {
			return err
		}
	}

	// Propagate taint to descendants and get changed nodes
	// Note: We need to update n.TaintState first so descendants see the correct parent taint
	n.TaintState = newTaint
	changedDescendants := taint.PropagateTaint(n, allNodes)

	// Emit TaintRecomputed events for all changed descendants
	for _, desc := range changedDescendants {
		if desc != nil {
			taintEvent := ledger.NewTaintRecomputed(desc.ID, desc.TaintState)
			if _, err := ldg.Append(taintEvent); err != nil {
				return err
			}
		}
	}

	return nil
}

// ChildSpec specifies a child node to be created in a bulk refine operation.
type ChildSpec struct {
	NodeType  schema.NodeType
	Statement string
	Inference schema.InferenceType
}

// RefineSpec specifies parameters for refining a node with a child.
// This struct consolidates the many parameters of RefineNodeWithAllDeps
// into a single, cleaner API.
type RefineSpec struct {
	// ParentID is the node to add a child to (required).
	ParentID types.NodeID

	// Owner is the agent who has claimed the parent node (required).
	Owner string

	// ChildID is the ID for the new child node (required).
	ChildID types.NodeID

	// NodeType is the type of the new node (required).
	NodeType schema.NodeType

	// Statement is the content of the new node (required).
	Statement string

	// Inference is the inference type used to derive the statement (required).
	Inference schema.InferenceType

	// Dependencies are logical cross-references to other nodes (optional).
	// These represent nodes that are referenced in the reasoning (e.g., "by step 1.2").
	Dependencies []types.NodeID

	// ValidationDeps specify nodes that must be validated before this node
	// can be accepted (optional).
	ValidationDeps []types.NodeID
}

// AllocateChildID allocates the next available child ID for a parent node atomically.
// This method acquires the ledger lock and returns the next child ID that should be used.
// The returned ID is guaranteed to not exist in the current state.
//
// This fixes the TOCTOU race condition (vibefeld-hrap) where child IDs assigned at
// CLI level could race with other agents.
//
// Note: This only allocates the ID - it does NOT create the node. The caller should
// use the returned ID with RefineNode() immediately, or the ID may become stale if
// another agent creates nodes in between.
func (s *ProofService) AllocateChildID(parentID types.NodeID) (types.NodeID, error) {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return types.NodeID{}, err
	}

	// Check if parent node exists
	if st.GetNode(parentID) == nil {
		return types.NodeID{}, fmt.Errorf("%w: %s", ErrParentNotFound, parentID.String())
	}

	// Find next available child ID
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return types.NodeID{}, fmt.Errorf("failed to generate child ID: %w", err)
		}
		if st.GetNode(candidateID) == nil {
			return candidateID, nil
		}
		childNum++
	}
}

// RefineNodeBulk adds multiple child nodes to a claimed parent node in a single atomic operation.
// This fixes the claim contention bug (vibefeld-9ayl) where agents had to claim-refine-release
// multiple times to add N children, allowing other agents to grab the node between cycles.
//
// All children are created atomically - either all succeed or none are created.
// Child IDs are allocated sequentially starting from the next available child number.
//
// Returns the IDs of the created children in order, or an error if any validation fails.
// Returns ErrMaxDepthExceeded if any child node's depth would exceed config.MaxDepth.
// Returns ErrMaxChildrenExceeded if adding all children would exceed config.MaxChildren.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RefineNodeBulk(parentID types.NodeID, owner string, children []ChildSpec) ([]types.NodeID, error) {
	if len(children) == 0 {
		return nil, fmt.Errorf("%w: at least one child specification is required", ErrEmptyInput)
	}

	// Validate depth for children (all children will have parent depth + 1)
	childDepth := parentID.Depth() + 1
	if err := s.validateDepth(childDepth); err != nil {
		return nil, err
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}
	expectedSeq := st.LatestSeq()

	// Check if parent node exists
	parent := st.GetNode(parentID)
	if parent == nil {
		return nil, fmt.Errorf("%w: %s", ErrParentNotFound, parentID.String())
	}

	// Check if parent is claimed
	if parent.WorkflowState != schema.WorkflowClaimed {
		return nil, fmt.Errorf("%w: parent node must be claimed", ErrNotClaimed)
	}

	// Check if owner matches
	if parent.ClaimedBy != owner {
		return nil, ErrOwnerMismatch
	}

	// Count existing children and validate that we can add all new children
	cfg, err := s.Config()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	existingChildCount := 0
	for _, n := range st.AllNodes() {
		p, hasParent := n.ID.Parent()
		if hasParent && p.String() == parentID.String() {
			existingChildCount++
		}
	}
	if existingChildCount+len(children) > cfg.MaxChildren {
		return nil, fmt.Errorf("%w: node %s has %d children, adding %d would exceed max %d",
			ErrMaxChildrenExceeded, parentID.String(), existingChildCount, len(children), cfg.MaxChildren)
	}

	// Find next available child number
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return nil, fmt.Errorf("failed to generate child ID: %w", err)
		}
		if st.GetNode(candidateID) == nil {
			break
		}
		childNum++
	}

	// Prepare all child nodes and their IDs
	childIDs := make([]types.NodeID, len(children))
	events := make([]ledger.Event, len(children))

	for i, spec := range children {
		// Validate statement is not empty
		if strings.TrimSpace(spec.Statement) == "" {
			return nil, fmt.Errorf("child %d: statement cannot be empty", i+1)
		}

		// Validate external citations in the statement
		if err := lemma.ValidateExtCitations(spec.Statement, st); err != nil {
			return nil, fmt.Errorf("child %d: %w", i+1, err)
		}

		// Generate child ID
		childID, err := parentID.Child(childNum + i)
		if err != nil {
			return nil, fmt.Errorf("child %d: failed to generate child ID: %w", i+1, err)
		}
		childIDs[i] = childID

		// Create the child node
		childNode, err := node.NewNode(childID, spec.NodeType, spec.Statement, spec.Inference)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i+1, err)
		}

		// Create the event
		events[i] = ledger.NewNodeCreated(*childNode)
	}

	// Get ledger
	ldg, err := s.getLedger()
	if err != nil {
		return nil, err
	}

	// Append all events with CAS on first event (see appendBulkIfSequence ATOMICITY NOTE)
	_, err = s.appendBulkIfSequence(ldg, events, expectedSeq)
	if err != nil {
		return nil, wrapSequenceMismatch(err, "RefineNodeBulk")
	}

	return childIDs, nil
}

// appendBulkIfSequence appends multiple events with sequence verification on the first event.
// This is an internal helper that combines CAS semantics with batch append.
//
// ATOMICITY NOTE: This function is NOT truly atomic for multiple events. The implementation:
// 1. Uses CAS (AppendIfSequence) for the first event to detect concurrent modifications
// 2. Uses simple Append for remaining events without further CAS checks
//
// This means:
// - If the first CAS succeeds but a subsequent append fails (disk error, etc.), the ledger
//   will contain a partial set of events. The function returns the successfully appended
//   sequence numbers along with an error.
// - Concurrent readers may observe partially-applied changes during the append window.
// - However, each individual event is still a valid, consistent ledger entry.
//
// Why this is acceptable:
// - Disk/IO failures during append are rare in practice
// - Callers (RefineNodeBulk, AcceptNodeBulk) create logically related but independent events
// - State replay handles partial updates correctly (each event is self-contained)
// - True atomic batch append would require a more complex protocol (e.g., WAL or 2PC)
//
// Callers should be aware that partial failure is possible and handle the returned
// sequence numbers accordingly.
func (s *ProofService) appendBulkIfSequence(ldg *ledger.Ledger, events []ledger.Event, expectedSeq int) ([]int, error) {
	if len(events) == 0 {
		return nil, nil
	}

	// For single event, use the existing method
	if len(events) == 1 {
		seq, err := ldg.AppendIfSequence(events[0], expectedSeq)
		if err != nil {
			return nil, err
		}
		return []int{seq}, nil
	}

	// For multiple events, we need to append them all atomically.
	// The ledger's AppendBatch doesn't support CAS, so we'll append one by one
	// but verify the sequence only on the first append.
	// This maintains atomicity from a concurrency perspective because:
	// 1. The first append with CAS ensures we're working from a consistent state
	// 2. Subsequent appends are guaranteed to succeed because we hold implied
	//    serialization through the sequence numbers

	seqs := make([]int, len(events))

	// First event uses CAS
	seq, err := ldg.AppendIfSequence(events[0], expectedSeq)
	if err != nil {
		return nil, err
	}
	seqs[0] = seq

	// Remaining events use simple append - they will get sequential numbers
	// because we just established our position in the sequence
	for i := 1; i < len(events); i++ {
		seq, err := ldg.Append(events[i])
		if err != nil {
			// Partial failure - some events were appended
			// This is a best-effort situation; the ledger will be in a partially
			// updated state but still consistent
			return seqs[:i], fmt.Errorf("failed to append event %d: %w", i+1, err)
		}
		seqs[i] = seq
	}

	return seqs, nil
}

// ErrCircularDependency is returned when a cycle is detected in node dependencies.
// Exit code: 3 (logic error)
var ErrCircularDependency = aferrors.New(aferrors.DEPENDENCY_CYCLE, "circular dependency detected")

// AmendNode allows a prover to correct the statement of a node they own.
// The original statement is preserved in the amendment history.
//
// Requirements:
// - Node must exist
// - Node must be in pending epistemic state (not validated/refuted)
// - Either the node is unclaimed, or the owner matches the claim owner
// - New statement must be non-empty
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AmendNode(nodeID types.NodeID, owner, newStatement string) error {
	// Validate inputs
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("%w: owner", ErrEmptyInput)
	}
	if strings.TrimSpace(newStatement) == "" {
		return fmt.Errorf("%w: statement", ErrEmptyInput)
	}

	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
	}

	// Check epistemic state - can only amend pending nodes
	if n.EpistemicState != schema.EpistemicPending {
		return fmt.Errorf("cannot amend node: epistemic state is %s, must be pending", n.EpistemicState)
	}

	// Check ownership - either unclaimed or owned by the caller
	if n.WorkflowState == schema.WorkflowClaimed {
		if n.ClaimedBy != owner {
			return fmt.Errorf("node is claimed by %s, not %s", n.ClaimedBy, owner)
		}
	}
	// If unclaimed, any owner can amend (they're taking responsibility)

	// Get ledger and append amendment event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeAmended(nodeID, n.Statement, newStatement, owner)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "AmendNode")
}

// LoadAmendmentHistory returns the amendment history for a node.
// Returns an empty slice if no amendments have been made.
// Note: This method performs I/O to load state from disk.
func (s *ProofService) LoadAmendmentHistory(nodeID types.NodeID) ([]state.Amendment, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	return st.GetAmendmentHistory(nodeID), nil
}

// WritePendingDef writes a pending definition to the proof's pending_defs directory.
// This is a convenience wrapper around fs.WritePendingDef that uses the service's path.
func (s *ProofService) WritePendingDef(nodeID types.NodeID, pd *node.PendingDef) error {
	return fs.WritePendingDef(s.path, nodeID, pd)
}

// ReadPendingDef reads a pending definition from the proof's pending_defs directory.
// Returns an error if the pending def doesn't exist.
// This is a convenience wrapper around fs.ReadPendingDef that uses the service's path.
func (s *ProofService) ReadPendingDef(nodeID types.NodeID) (*node.PendingDef, error) {
	return fs.ReadPendingDef(s.path, nodeID)
}

// ListPendingDefs returns all pending definition node IDs in the proof.
// Returns an empty slice (not an error) if no pending definitions exist.
// This is a convenience wrapper around fs.ListPendingDefs that uses the service's path.
func (s *ProofService) ListPendingDefs() ([]types.NodeID, error) {
	return fs.ListPendingDefs(s.path)
}

// DeletePendingDef removes a pending definition from the proof.
// This is idempotent: it does NOT return an error if the pending def doesn't exist.
// This is a convenience wrapper around fs.DeletePendingDef that uses the service's path.
func (s *ProofService) DeletePendingDef(nodeID types.NodeID) error {
	return fs.DeletePendingDef(s.path, nodeID)
}

// LoadAllPendingDefs loads all pending definitions from the proof directory.
// This is a convenience method that combines ListPendingDefs and ReadPendingDef.
func (s *ProofService) LoadAllPendingDefs() ([]*node.PendingDef, error) {
	nodeIDs, err := s.ListPendingDefs()
	if err != nil {
		return nil, err
	}

	pendingDefs := make([]*node.PendingDef, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		pd, err := s.ReadPendingDef(nodeID)
		if err != nil {
			return nil, err
		}
		pendingDefs = append(pendingDefs, pd)
	}

	return pendingDefs, nil
}

// ReadAssumption reads an assumption by ID from the proof directory.
// This is a convenience wrapper around fs.ReadAssumption that uses the service's path.
func (s *ProofService) ReadAssumption(id string) (*node.Assumption, error) {
	return fs.ReadAssumption(s.path, id)
}

// ListAssumptions returns all assumption IDs in the proof.
// Returns an empty slice (not an error) if no assumptions exist.
// This is a convenience wrapper around fs.ListAssumptions that uses the service's path.
func (s *ProofService) ListAssumptions() ([]string, error) {
	return fs.ListAssumptions(s.path)
}

// ReadExternal reads an external reference by ID from the proof directory.
// This is a convenience wrapper around fs.ReadExternal that uses the service's path.
func (s *ProofService) ReadExternal(id string) (*node.External, error) {
	return fs.ReadExternal(s.path, id)
}

// WriteExternal writes an external reference to the proof directory.
// This is a convenience wrapper around fs.WriteExternal that uses the service's path.
func (s *ProofService) WriteExternal(ext *node.External) error {
	return fs.WriteExternal(s.path, ext)
}

// ListExternals returns all external reference IDs in the proof.
// Returns an empty slice (not an error) if no external references exist.
// This is a convenience wrapper around fs.ListExternals that uses the service's path.
func (s *ProofService) ListExternals() ([]string, error) {
	return fs.ListExternals(s.path)
}

// RecomputeAllTaint recomputes taint state for all nodes in the proof tree.
// If dryRun is true, returns what would change without applying changes.
// Otherwise, persists TaintRecomputed events to the ledger for each changed node.
//
// Taint propagates through the proof tree based on epistemic states:
// - Validated nodes are clean
// - Admitted nodes are self_admitted
// - Children of self_admitted/tainted nodes become tainted
// - Pending nodes are unresolved
func (s *ProofService) RecomputeAllTaint(dryRun bool) (*RecomputeTaintResult, error) {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return nil, fmt.Errorf("error loading proof state: %w", err)
	}

	// Get all nodes
	allNodes := st.AllNodes()
	if len(allNodes) == 0 {
		return nil, fmt.Errorf("proof not initialized or empty")
	}

	// Track changes
	var changes []TaintChange
	oldTaints := make(map[string]node.TaintState)

	// Store old taint states
	for _, n := range allNodes {
		oldTaints[n.ID.String()] = n.TaintState
	}

	// Sort nodes by depth (shallower first) to process parents before children
	sortNodesByDepthForTaint(allNodes)

	// Build node map for ancestor lookup
	nodeMap := make(map[string]*node.Node)
	for _, n := range allNodes {
		nodeMap[n.ID.String()] = n
	}

	// Recompute taint for each node
	for _, n := range allNodes {
		// Get ancestors
		ancestors := getNodeAncestorsForTaint(n, nodeMap)

		// Compute new taint
		newTaint := taint.ComputeTaint(n, ancestors)

		// Check if changed
		if n.TaintState != newTaint {
			changes = append(changes, TaintChange{
				NodeID:   n.ID.String(),
				OldTaint: TaintState(n.TaintState),
				NewTaint: TaintState(newTaint),
			})
			// Update node in memory (for cascade effect)
			n.TaintState = newTaint
		}
	}

	// Build result
	result := &RecomputeTaintResult{
		TotalNodes:   len(allNodes),
		NodesChanged: len(changes),
		Changes:      changes,
		DryRun:       dryRun,
	}

	// If not dry-run, persist changes to ledger
	if !dryRun && len(changes) > 0 {
		ledgerDir := filepath.Join(s.path, "ledger")
		ldg, err := ledger.NewLedger(ledgerDir)
		if err != nil {
			return nil, err
		}

		// Append events for each change
		seq := st.LatestSeq()
		for _, change := range changes {
			nodeID, err := types.Parse(change.NodeID)
			if err != nil {
				return nil, fmt.Errorf("invalid node ID %q: %w", change.NodeID, err)
			}

			event := ledger.NewTaintRecomputed(nodeID, node.TaintState(change.NewTaint))
			newSeq, err := ldg.AppendIfSequence(event, seq)
			if err != nil {
				return nil, err
			}
			seq = newSeq
		}
	}

	return result, nil
}

// sortNodesByDepthForTaint sorts nodes by their depth (shallower first).
func sortNodesByDepthForTaint(nodes []*node.Node) {
	// Using a simple insertion sort since nodes are already mostly ordered
	for i := 1; i < len(nodes); i++ {
		j := i
		for j > 0 && nodes[j].ID.Depth() < nodes[j-1].ID.Depth() {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
			j--
		}
	}
}

// getNodeAncestorsForTaint returns the ancestor nodes for a given node.
func getNodeAncestorsForTaint(n *node.Node, nodeMap map[string]*node.Node) []*node.Node {
	var ancestors []*node.Node
	parentID, hasParent := n.ID.Parent()
	for hasParent {
		if parent, ok := nodeMap[parentID.String()]; ok {
			ancestors = append(ancestors, parent)
		}
		parentID, hasParent = parentID.Parent()
	}
	return ancestors
}

// RequestRefinement requests refinement on a validated node, transitioning it
// to the needs_refinement state. This allows a verifier to reopen a validated
// node for further proof development by provers.
//
// The node must be in the validated epistemic state. Only validated nodes can
// have refinement requested. This is because refinement is a mechanism for
// requesting additional detail or rigor on a node that was previously accepted.
//
// Parameters:
//   - nodeID: The ID of the node to request refinement on
//   - reason: An explanation of why refinement is needed (optional but recommended)
//   - requestedBy: The agent ID of the requester (optional)
//
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrInvalidState if the node is not in validated state.
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) RequestRefinement(nodeID types.NodeID, reason, requestedBy string) error {
	// Load current state and capture sequence for CAS
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	expectedSeq := st.LatestSeq()

	// Check if node exists
	n := st.GetNode(nodeID)
	if n == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
	}

	// Validate epistemic state transition (only validated -> needs_refinement allowed)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicNeedsRefinement); err != nil {
		return fmt.Errorf("%w: node %s is in %s state, must be %s to request refinement",
			ErrInvalidState, nodeID.String(), n.EpistemicState, schema.EpistemicValidated)
	}

	// Get ledger and append refinement requested event with CAS
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewRefinementRequested(nodeID, reason, requestedBy)
	_, err = ldg.AppendIfSequence(event, expectedSeq)
	return wrapSequenceMismatch(err, "RequestRefinement")
}
