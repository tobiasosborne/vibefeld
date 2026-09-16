// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/fs"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/lemma"
	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/taint"
	"github.com/tobiasosborne/vibefeld/internal/types"
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

// ErrParentIDMismatch is returned when a caller-supplied ParentID does not
// match the structural parent encoded in the child's ID. The committed node's
// structure is derived from the child ID, so accepting a mismatched ParentID
// would validate one graph and commit another.
// Exit code: 3 (logic error)
var ErrParentIDMismatch = aferrors.New(aferrors.INVALID_PARENT, "parent ID does not match child ID")

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

	// beforeAppend is a test-only hook invoked by commit() between build()
	// and AppendBatchIfSequence(). It lets tests append a concurrent event in
	// the exact window the optimistic commit protocol is designed to close.
	// It is nil in production.
	beforeAppend func()
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

	svc := &ProofService{path: path}

	// Refuse a workspace whose stamped format this binary cannot read. This is
	// the format gate at the service entry point; the replay CLI bypasses
	// ProofService and performs the same check itself.
	cfg, err := svc.LoadConfig()
	if err != nil {
		return nil, err
	}
	if err := config.CheckFormat(cfg); err != nil {
		return nil, err
	}

	return svc, nil
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

	svc, err := NewProofService(proofDir)
	if err != nil {
		return err
	}

	// Create the root node (node "1") with the conjecture as the statement.
	// Building it before the commit is fine: the emptiness check and both
	// events share one commit closure, so two concurrent inits cannot both
	// pass an emptiness check and land.
	rootID, err := types.Parse("1")
	if err != nil {
		return err
	}

	// The root node's Author is the proof's author (recorded at creation),
	// same driver-supplied-provenance convention as any other node's Author.
	rootNode, err := node.NewNodeWithOptions(rootID, schema.NodeTypeClaim, conjecture, schema.InferenceAssumption, node.NodeOptions{Author: author})
	if err != nil {
		return err
	}

	// Append the initialization and root-node events through the same
	// optimistic commit primitive as every other mutating path. The ledger
	// emptiness check happens inside the closure against the same state read
	// that supplies the CAS sequence, so a concurrent init is refused.
	_, err = svc.commit(func(st *state.State) ([]ledger.Event, error) {
		if st.LatestSeq() != 0 {
			return nil, fmt.Errorf("%w: proof already initialized", ErrAlreadyExists)
		}
		return []ledger.Event{
			ledger.NewProofInitialized(conjecture, author),
			ledger.NewNodeCreated(*rootNode),
		}, nil
	})
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

	_, err = s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if node already exists
		if st.GetNode(id) != nil {
			return nil, fmt.Errorf("%w: node %s", ErrAlreadyExists, id.String())
		}

		// Validate child count for parent (if not root)
		if parentID, hasParent := id.Parent(); hasParent {
			if err := s.validateChildCount(st, parentID); err != nil {
				return nil, err
			}
		}

		// Create the node
		n, err := node.NewNode(id, nodeType, statement, inference)
		if err != nil {
			return nil, err
		}

		// D1: run the same support check every creation path uses. The overlay
		// parent is derived from the child ID (there is no caller-supplied
		// ParentID here), matching the committed NodeCreated's structure. A node
		// created without dependencies cannot itself close a cycle, but this
		// keeps the invariant in one place as the graph grows.
		parent, hasParent := id.Parent()
		pn := support.ProspectiveNode{ID: id, Type: nodeType}
		if hasParent {
			pn.ParentID = parent
		}
		if err := checkSupportBatch(st, []support.ProspectiveNode{pn}); err != nil {
			return nil, err
		}

		return []ledger.Event{ledger.NewNodeCreated(*n)}, nil
	})
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

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Check if node is available
		if n.WorkflowState != schema.WorkflowAvailable {
			return nil, fmt.Errorf("%w: node %s is not available", ErrInvalidState, id.String())
		}

		// Calculate timeout timestamp
		timeoutTS := types.FromTime(time.Now().Add(timeout))
		return []ledger.Event{ledger.NewNodesClaimed([]types.NodeID{id}, owner, timeoutTS)}, nil
	})
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

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Check if node is claimed
		if n.WorkflowState != schema.WorkflowClaimed {
			return nil, ErrNotClaimed
		}

		// Check if owner matches
		if n.ClaimedBy != owner {
			return nil, fmt.Errorf("%w: node is claimed by %s, not %s", ErrOwnerMismatch, n.ClaimedBy, owner)
		}

		// Calculate new timeout timestamp
		newTimeoutTS := types.FromTime(time.Now().Add(timeout))
		return []ledger.Event{ledger.NewClaimRefreshed(id, owner, newTimeoutTS)}, nil
	})
	return wrapSequenceMismatch(err, "RefreshClaim")
}

// ReleaseNode releases a claimed node, making it available again.
// Returns an error if the node is not claimed or the owner doesn't match.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) ReleaseNode(id types.NodeID, owner string) error {
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Check if node is claimed
		if n.WorkflowState != schema.WorkflowClaimed {
			return nil, ErrNotClaimed
		}

		// Check if owner matches
		if n.ClaimedBy != owner {
			return nil, ErrOwnerMismatch
		}

		return []ledger.Event{ledger.NewNodesReleased([]types.NodeID{id})}, nil
	})
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

	var oldTaints map[string]node.TaintState
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// The committed NodeCreated derives structure from the child ID, so a
		// caller-supplied ParentID that disagrees would validate one graph and
		// commit another. Reject the mismatch and derive the overlay parent from
		// the child ID below.
		derivedParent, hasDerived := spec.ChildID.Parent()
		if !hasDerived || derivedParent.String() != spec.ParentID.String() {
			return nil, fmt.Errorf("%w: parent %s does not match child %s's parent %s",
				ErrParentIDMismatch, spec.ParentID.String(), spec.ChildID.String(), derivedParent.String())
		}

		// Check if parent node exists
		parent := st.GetNode(spec.ParentID)
		if parent == nil {
			return nil, fmt.Errorf("%w: %s", ErrParentNotFound, spec.ParentID.String())
		}

		// Check if parent is claimed
		if parent.WorkflowState != schema.WorkflowClaimed {
			return nil, fmt.Errorf("%w: parent node must be claimed", ErrNotClaimed)
		}

		// Check if owner matches
		if parent.ClaimedBy != spec.Owner {
			return nil, ErrOwnerMismatch
		}

		// Check if child already exists
		if st.GetNode(spec.ChildID) != nil {
			return nil, fmt.Errorf("%w: node %s", ErrAlreadyExists, spec.ChildID.String())
		}

		// Validate child count for parent
		if err := s.validateChildCount(st, spec.ParentID); err != nil {
			return nil, err
		}

		// Validate external citations in the statement
		if err := lemma.ValidateExtCitations(spec.Statement, st); err != nil {
			return nil, err
		}

		// Validate that all explicit dependencies exist. Existence is a
		// creation-path invariant; the support graph itself keeps a
		// dependency on a missing/severed node as a sink edge for audit.
		for _, depID := range spec.Dependencies {
			if st.GetNode(depID) == nil {
				return nil, fmt.Errorf("invalid dependency: node %s not found", depID.String())
			}
		}
		for _, valDepID := range spec.ValidationDeps {
			if st.GetNode(valDepID) == nil {
				return nil, fmt.Errorf("invalid validation dependency: node %s not found", valDepID.String())
			}
		}

		// Cycle and scope checks over result-use edges, from the child's own
		// position (vibefeld-0ko0): a child may not result-use an ancestor
		// claim, but may hypothesis-use an enclosing local_assume.
		if err := checkSupportBatch(st, []support.ProspectiveNode{{
			ID:             spec.ChildID,
			ParentID:       derivedParent,
			Type:           spec.NodeType,
			Dependencies:   spec.Dependencies,
			ValidationDeps: spec.ValidationDeps,
		}}); err != nil {
			return nil, err
		}

		// Create the child node with both dependency types.
		opts := node.NodeOptions{
			Dependencies:   spec.Dependencies,
			ValidationDeps: spec.ValidationDeps,
			Draft:          spec.Draft,
			Crux:           spec.Crux,
			Author:         spec.Owner,
		}
		child, err := node.NewNodeWithOptions(spec.ChildID, spec.NodeType, spec.Statement, spec.Inference, opts)
		if err != nil {
			return nil, err
		}

		oldTaints = snapshotTaintStates(st)
		return []ledger.Event{ledger.NewNodeCreated(*child)}, nil
	})
	if err != nil {
		return wrapSequenceMismatch(err, "Refine")
	}

	// Node creation and taint-audit emission are intentionally non-atomic, like
	// epistemic transitions. Replay still derives correct state if emission fails.
	return s.emitTaintRecomputedEvents(spec.ChildID, oldTaints)
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
//  1. If taint emission fails after validation succeeds, the validation stands
//     but taint state in the ledger may be stale until the next state replay.
//  2. Concurrent readers may briefly see the validated node with outdated taint.
//  3. The taint package computes correct taint on replay, so eventual consistency
//     is guaranteed - the ledger just won't contain explicit taint events.
//
// This is acceptable because taint is derived state (can be recomputed from
// epistemic states) and the validation event is the authoritative record.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AcceptNodeWithNote(id types.NodeID, note string) error {
	return s.AcceptNodeWithVerifier(id, note, "", "")
}

// AcceptNodeWithVerifier validates a node with an optional acceptance note,
// verifier identity, and batch id. It is the full-featured form of
// AcceptNode/AcceptNodeWithNote (both of which call this with empty
// verifiedBy/batchID) and is the kernel surface rk's C3 batch verification
// mode (`af verdicts apply`, item V2 — not implemented in this package) is
// expected to call directly, passing the batch's own id so every node it
// validates carries the same batchID.
//
// verifiedBy and batchID are DRIVER-SUPPLIED PROVENANCE: recorded on the
// NodeValidated event and on the node's ValidatedBy/ValidationBatchID
// fields, mechanically checkable (e.g. by comparing against the node's
// Author for a reviewer≠author check), but never verified against any
// external credential — the trust anchor remains the driver's process
// discipline, same as it always has been for ClaimedBy.
//
// Returns an error if the node doesn't exist.
// Returns ErrBlockingChallenges if the node has unresolved critical or major challenges.
//
// After validation, automatically recomputes and emits taint state changes
// for the node and any affected descendants.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) AcceptNodeWithVerifier(id types.NodeID, note, verifiedBy, batchID string) error {
	return s.acceptNodeWithExpectation(id, note, verifiedBy, batchID, "")
}

// AcceptNodeWithExpectation is AcceptNodeWithVerifier with an expected content
// hash. When expectHash is non-empty it is compared against the node's current
// content hash under the same state read the accept commits against, and the
// resulting NodeValidated records ExpectedHashChecked=true. A mismatch is
// refused; a plain accept (empty expectHash) records ExpectedHashChecked=false.
func (s *ProofService) AcceptNodeWithExpectation(id types.NodeID, note, verifiedBy, batchID, expectHash string) error {
	return s.acceptNodeWithExpectation(id, note, verifiedBy, batchID, expectHash)
}

// acceptNodeWithExpectation is the shared body of AcceptNodeWithVerifier and
// AcceptNodeWithExpectation.
func (s *ProofService) acceptNodeWithExpectation(id types.NodeID, note, verifiedBy, batchID, expectHash string) error {
	var oldTaints map[string]node.TaintState
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		events, err := s.buildAcceptEvents(st, id, note, verifiedBy, batchID, expectHash)
		if err != nil {
			return nil, err
		}
		oldTaints = snapshotTaintStates(st)
		return events, nil
	})
	if err != nil {
		return wrapSequenceMismatch(err, "AcceptNodeWithNote")
	}

	// Auto-compute and emit taint events after successful validation.
	return s.emitTaintRecomputedEvents(id, oldTaints)
}

// buildAcceptEvents validates an accept of id against st and returns the
// NodeValidated event. The preconditions and the event construction share the
// same state read, so a verdict item cannot validate against one state and
// append against another. It is shared by AcceptNodeWithVerifier's commit
// closure and by applyAcceptVerdict's (which adds the verdict-file gates
// before calling it). expectHash, when non-empty, is compared against the
// node's content in this same read and sets ExpectedHashChecked on the event.
func (s *ProofService) buildAcceptEvents(st *state.State, id types.NodeID, note, verifiedBy, batchID, expectHash string) ([]ledger.Event, error) {
	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
	}

	// D3: when the caller supplied an expected hash, compare it here — against
	// the same state read the accept commits against — before anything else.
	if expectHash != "" && n.ContentHash != expectHash {
		return nil, fmt.Errorf("%w: node %s content hash changed since the expectation was recorded (expected %s, current %s)",
			ErrInvalidState, id.String(), expectHash, n.ContentHash)
	}

	// Check for blocking challenges (critical or major severity)
	blockingChallenges := st.GetBlockingChallengesForNode(id)
	if len(blockingChallenges) > 0 {
		return nil, formatBlockingChallengesError(id, blockingChallenges)
	}

	// Check crux nodes require a passing claim-test that matches the node's
	// current content. A legacy test (no recorded hash) still counts.
	if n.Crux && !st.HasPassingClaimTestForContent(id, n.ContentHash) {
		if st.HasStalePassingClaimTest(id, n.ContentHash) {
			return nil, fmt.Errorf("%w: %w: node %s (only passing claim-test is stale: it was run against an older content hash; re-run 'af claim-test')", ErrClaimTestRequired, ErrClaimTestStale, id.String())
		}
		return nil, fmt.Errorf("%w: node %s", ErrClaimTestRequired, id.String())
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
			return nil, fmt.Errorf("cannot accept node %s: validation dependencies not yet validated: %s",
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
		// Child must reach a terminal-cleared verdict: validated, admitted, or archived.
		// Archived = branch abandoned, parent no longer relies on it. Refuted is intentionally
		// excluded — refuted means the step is false, which is a real obstacle to the parent.
		if child.EpistemicState != schema.EpistemicValidated &&
			child.EpistemicState != schema.EpistemicAdmitted &&
			child.EpistemicState != schema.EpistemicArchived {
			unvalidatedChildren = append(unvalidatedChildren, child.ID.String())
		}
	}
	if len(unvalidatedChildren) > 0 {
		return nil, fmt.Errorf("cannot accept node %s: children not yet validated: %s",
			id.String(), strings.Join(unvalidatedChildren, ", "))
	}

	// For needs_refinement nodes, require that refinement actually happened (has children)
	if n.EpistemicState == schema.EpistemicNeedsRefinement && len(children) == 0 {
		return nil, fmt.Errorf("cannot accept node %s: node is in needs_refinement state but has no children; use 'af refine' to add child nodes first",
			id.String())
	}

	// Validate epistemic state transition (pending -> validated or needs_refinement -> validated)
	if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
		return nil, err
	}

	return []ledger.Event{ledger.NewNodeValidatedWithHash(id, note, verifiedBy, batchID, n.ContentHash, expectHash != "")}, nil
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
	return s.AcceptNodeBulkWithVerifier(ids, "", "")
}

// AcceptNodeBulkWithVerifier is AcceptNodeBulk with verifier identity and an
// optional batch id recorded on every resulting NodeValidated event. See
// AcceptNodeWithVerifier for the provenance caveats (driver-supplied,
// recorded-and-checkable, not adversary-proof) — the same apply here. This
// is the kernel surface rk's C3 batch verification mode is expected to use
// when a batch's verdict list accepts more than one item at once.
func (s *ProofService) AcceptNodeBulkWithVerifier(ids []types.NodeID, verifiedBy, batchID string) error {
	if len(ids) == 0 {
		return nil // Nothing to do
	}

	var oldTaints map[string]node.TaintState
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Validate all nodes exist, have no blocking challenges, and are in pending state before any mutation
		contentHashes := make([]string, len(ids))
		for i, id := range ids {
			n := st.GetNode(id)
			if n == nil {
				return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
			}
			contentHashes[i] = n.ContentHash

			// Check for blocking challenges (critical or major severity)
			blockingChallenges := st.GetBlockingChallengesForNode(id)
			if len(blockingChallenges) > 0 {
				return nil, formatBlockingChallengesError(id, blockingChallenges)
			}

			// Check crux nodes require a passing claim-test matching current content
			if n.Crux && !st.HasPassingClaimTestForContent(id, n.ContentHash) {
				if st.HasStalePassingClaimTest(id, n.ContentHash) {
					return nil, fmt.Errorf("%w: %w: node %s (only passing claim-test is stale: it was run against an older content hash; re-run 'af claim-test')", ErrClaimTestRequired, ErrClaimTestStale, id.String())
				}
				return nil, fmt.Errorf("%w: node %s", ErrClaimTestRequired, id.String())
			}

			// Validate epistemic state transition (only pending -> validated allowed)
			if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicValidated); err != nil {
				return nil, fmt.Errorf("node %s: %w", id.String(), err)
			}
		}

		// Create events for all nodes. Bulk accept has no per-node expectation,
		// so ExpectedHashChecked is always false (D3).
		events := make([]ledger.Event, len(ids))
		for i, id := range ids {
			events[i] = ledger.NewNodeValidatedWithHash(id, "", verifiedBy, batchID, contentHashes[i], false)
		}
		oldTaints = snapshotTaintStates(st)
		return events, nil
	})
	if err != nil {
		return wrapSequenceMismatch(err, "AcceptNodeBulk")
	}

	// Emit taint events for all accepted nodes. Reuse one pre-transition
	// snapshot so overlapping ancestor changes are emitted only once.
	for _, id := range ids {
		if err := s.emitTaintRecomputedEvents(id, oldTaints); err != nil {
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
//
// Deprecated: Use LoadPendingNodeSummaries instead, which returns a view model
// that doesn't leak internal domain types. This method is kept for backward
// compatibility but new code should prefer the summary version.
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

// LoadPendingNodeSummaries returns summaries of all nodes in the pending epistemic state.
// This returns a view model (NodeSummary) instead of the internal node.Node type,
// which prevents the CLI from depending on internal domain types.
//
// Note: This method performs I/O to load state from disk.
func (s *ProofService) LoadPendingNodeSummaries() ([]NodeSummary, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}

	var summaries []NodeSummary
	for _, n := range st.AllNodes() {
		if n.EpistemicState == schema.EpistemicPending {
			summaries = append(summaries, NodeSummary{
				ID:        n.ID,
				Type:      n.Type,
				Statement: n.Statement,
				Inference: n.Inference,
			})
		}
	}

	return summaries, nil
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
	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Validate epistemic state transition (only pending -> admitted allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicAdmitted); err != nil {
			return nil, err
		}

		return []ledger.Event{ledger.NewNodeAdmitted(id)}, nil
	})
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
	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Validate epistemic state transition (only pending -> refuted allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicRefuted); err != nil {
			return nil, err
		}

		return []ledger.Event{ledger.NewNodeRefuted(id)}, nil
	})
}

// VetoNode is a human expert force-refute that bypasses normal adversarial
// workflow. Unlike RefuteNode, this can override any non-terminal state
// including validated and admitted nodes.
//
// Requires a non-empty reason for audit trail.
//
// Returns ErrEmptyInput if the reason is empty.
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrInvalidState if the node is already refuted or archived.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) VetoNode(id types.NodeID, reason, vetoedBy string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: reason is required for veto", ErrEmptyInput)
	}

	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Veto can override any state except already-refuted and archived
		if n.EpistemicState == schema.EpistemicRefuted {
			return nil, fmt.Errorf("%w: node %s is already refuted", ErrInvalidState, id.String())
		}
		if n.EpistemicState == schema.EpistemicArchived {
			return nil, fmt.Errorf("%w: node %s is archived and cannot be vetoed", ErrInvalidState, id.String())
		}

		return []ledger.Event{ledger.NewNodeVetoed(id, reason, vetoedBy)}, nil
	})
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
	return s.commitThenTaint(id, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(id)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id.String())
		}

		// Validate epistemic state transition (only pending -> archived allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicArchived); err != nil {
			return nil, err
		}

		return []ledger.Event{ledger.NewNodeArchived(id)}, nil
	})
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

	// Create ledger definition
	ledgerDef := ledger.Definition{
		ID:         def.ID,
		Name:       def.Name,
		Definition: def.Content,
		Created:    def.Created,
	}

	_, err = s.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{ledger.NewDefAdded(ledgerDef)}, nil
	})
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

	// Load state and capture sequence for CAS. The source must be validated
	// and free of an open local scope; both checks run inside the commit
	// closure against the same state read the CAS uses, so a concurrent
	// unvalidation/scope change is caught rather than overwritten.
	var lemmaID string
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if source node exists
		n := st.GetNode(sourceNodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, sourceNodeID.String())
		}

		// Check if node is validated
		if n.EpistemicState != schema.EpistemicValidated {
			return nil, fmt.Errorf("%w: node %s is not validated (current state: %s); only validated nodes can be extracted as lemmas",
				ErrInvalidState, sourceNodeID.String(), n.EpistemicState)
		}

		// Check for independence: the node must not depend on local
		// assumptions from a parent scope.
		if len(n.Scope) > 0 {
			return nil, fmt.Errorf("%w: node %s is not independent: it depends on local assumptions (%s); lemmas cannot rely on local scope",
				ErrInvalidState, sourceNodeID.String(), strings.Join(n.Scope, ", "))
		}

		// Create the lemma
		lemma, err := node.NewLemma(statement, sourceNodeID)
		if err != nil {
			return nil, err
		}

		ledgerLemma := ledger.Lemma{
			ID:        lemma.ID,
			Statement: lemma.Statement,
			NodeID:    lemma.SourceNodeID,
			Created:   lemma.Created,
		}
		lemmaID = lemma.ID
		return []ledger.Event{ledger.NewLemmaExtracted(ledgerLemma)}, nil
	})
	if err != nil {
		return "", wrapSequenceMismatch(err, "ExtractLemma")
	}

	return lemmaID, nil
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

// emitTaintRecomputedEvents computes taint for a node, its ancestors, and its
// descendants after an epistemic state change, then emits TaintRecomputed
// events for every changed node.
//
// This is called automatically after validation events (AcceptNode, AdmitNode,
// RefuteNode, ArchiveNode) to ensure the ledger contains explicit taint state
// records for audit and replay purposes.
//
// IMPORTANT: This function is intentionally NOT atomic with the preceding epistemic
// state change event. The taint events are appended separately after the validation
// event is committed. This means:
//   - If this function fails, the epistemic state change still stands
//   - Taint state is derived (computable from epistemic states), so eventual consistency
//     is guaranteed on the next state replay even if these events are never written
//   - The ledger may lack explicit taint records, but the taint package will compute
//     correct taint on replay
func (s *ProofService) emitTaintRecomputedEvents(nodeID types.NodeID, oldTaints map[string]node.TaintState) error {
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Get the node that was just transitioned
		n := st.GetNode(nodeID)
		if n == nil {
			// Node should exist - this would be a logic error
			return nil, nil
		}

		allNodes := st.AllNodes()

		// Recompute in memory first. LoadState already performs an authoritative
		// full recompute, but keeping this targeted call here makes the affected-set
		// contract explicit and protects non-replay callers.
		taint.PropagateTaint(n, allNodes)

		if oldTaints == nil {
			oldTaints = make(map[string]node.TaintState)
		}

		// Compare against the caller's pre-transition snapshot. This is necessary
		// because replay derives correct taint before this audit-emission step runs.
		var events []ledger.Event
		for _, changed := range allNodes {
			if changed == nil || (!changed.ID.Equal(nodeID) && !nodeID.IsAncestorOf(changed.ID) && !changed.ID.IsAncestorOf(nodeID)) {
				continue
			}
			key := changed.ID.String()
			oldTaint, ok := oldTaints[key]
			if !ok {
				// Node absent from the pre-transition snapshot was created by this
				// operation; its NodeCreated event already records its taint.
				oldTaints[key] = changed.TaintState
				continue
			}
			if oldTaint == changed.TaintState {
				continue
			}
			events = append(events, ledger.NewTaintRecomputed(changed.ID, changed.TaintState))
			oldTaints[key] = changed.TaintState
		}
		return events, nil
	})
	return err
}

// SubmitNode transitions a draft node to pending state, making it ready for
// formal verification. Only draft nodes can be submitted.
//
// Returns ErrConcurrentModification if the proof was modified by another process
// since state was loaded. Callers should retry after reloading state.
func (s *ProofService) SubmitNode(nodeID types.NodeID, owner string) error {
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("node %s not found", nodeID.String())
		}

		if n.EpistemicState != schema.EpistemicDraft {
			return nil, fmt.Errorf("node %s is in state %q, not %q: only draft nodes can be submitted",
				nodeID.String(), n.EpistemicState, schema.EpistemicDraft)
		}

		return []ledger.Event{ledger.NewNodeSubmitted(nodeID, owner)}, nil
	})
	return wrapSequenceMismatch(err, "SubmitNode")
}

// ChildSpec specifies a child node to be created in a bulk refine operation.
type ChildSpec struct {
	NodeType  schema.NodeType
	Statement string
	Inference schema.InferenceType
	Draft     bool
	Crux      bool
	// Dependencies are this child's reference dependency IDs, in RAW,
	// unresolved form (rk B2 / rk-2zj: a prover's declared per-child depends).
	// Each entry is EITHER an existing node id (e.g. "1.2", validated to
	// exist) OR a backward sibling reference "#N" (0-based index into THIS
	// batch's children, strictly earlier than the current child — children
	// don't have IDs until this bulk op allocates them, so a sibling is named
	// by its position and resolved at creation to the allocated id).
	// Resolution + validation happen inside RefineNodeBulk. Empty for a child
	// with no dependencies.
	Dependencies []string
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

	// Draft creates the node in draft state instead of pending.
	// Draft nodes are work-in-progress; challenges on them are non-blocking.
	Draft bool

	// Crux marks the node as critical path. Crux nodes cannot be
	// validated without a passing claim-test.
	Crux bool
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
// resolveChildDeps resolves one bulk-refine child's raw dependency strings
// (rk B2) into concrete node IDs. childIndex is this child's 0-based position
// in the batch; allocatedIDs holds the ids assigned to every child in the
// batch (entries at positions >= childIndex are not yet meaningful). st is the
// current state, used to validate real (non-sibling) node ids exist.
//
// Each raw entry is one of:
//   - "#N": a backward sibling reference (0 <= N < childIndex). A forward or
//     self reference (N >= childIndex) is rejected — a prover must order
//     children so each depends only on earlier ones. A non-numeric or
//     out-of-range N is rejected.
//   - anything else: parsed as an existing node id, which must exist in st.
//
// Nothing is ever silently dropped: an unresolvable entry is an error.
func resolveChildDeps(raw []string, childIndex int, allocatedIDs []types.NodeID, st *State) ([]types.NodeID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	deps := make([]types.NodeID, 0, len(raw))
	for _, entry := range raw {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "#") {
			n, err := strconv.Atoi(entry[1:])
			if err != nil {
				return nil, fmt.Errorf("invalid sibling dependency reference %q: not a %q + integer index", entry, "#")
			}
			if n < 0 || n >= childIndex {
				return nil, fmt.Errorf("sibling dependency reference %q out of range: only earlier children (#0..#%d) may be referenced", entry, childIndex-1)
			}
			deps = append(deps, allocatedIDs[n])
			continue
		}
		depID, err := types.Parse(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency id %q: %w", entry, err)
		}
		if st.GetNode(depID) == nil {
			return nil, fmt.Errorf("dependency node %s does not exist", entry)
		}
		deps = append(deps, depID)
	}
	return deps, nil
}

// buildChildEvents validates a batch of ChildSpecs against the current state
// and produces the NodeCreated events plus the allocated child IDs, WITHOUT
// touching the ledger or checking the parent's workflow/claim state (each
// caller applies its own workflow gate first). Shared by RefineNodeBulk (which
// requires the parent claimed by owner) and RecordProof (rk B1/FU3, which
// gates on prover-job classification instead). Enforces max-children, sequential
// child-ID allocation, non-empty statements, external citations, and per-child
// dependency resolution (rk B2). owner is recorded as each child's Author.
func (s *ProofService) buildChildEvents(st *State, parentID types.NodeID, owner string, children []ChildSpec) ([]ledger.Event, []types.NodeID, error) {
	cfg, err := s.Config()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	existingChildCount := 0
	for _, n := range st.AllNodes() {
		p, hasParent := n.ID.Parent()
		if hasParent && p.String() == parentID.String() {
			existingChildCount++
		}
	}
	if existingChildCount+len(children) > cfg.MaxChildren {
		return nil, nil, fmt.Errorf("%w: node %s has %d children, adding %d would exceed max %d",
			ErrMaxChildrenExceeded, parentID.String(), existingChildCount, len(children), cfg.MaxChildren)
	}

	// Find next available child number
	childNum := 1
	for {
		candidateID, err := parentID.Child(childNum)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate child ID: %w", err)
		}
		if st.GetNode(candidateID) == nil {
			break
		}
		childNum++
	}

	childIDs := make([]types.NodeID, len(children))
	events := make([]ledger.Event, len(children))
	batch := make([]support.ProspectiveNode, len(children))

	for i, spec := range children {
		if strings.TrimSpace(spec.Statement) == "" {
			return nil, nil, fmt.Errorf("child %d: statement cannot be empty", i+1)
		}
		if err := lemma.ValidateExtCitations(spec.Statement, st); err != nil {
			return nil, nil, fmt.Errorf("child %d: %w", i+1, err)
		}

		childID, err := parentID.Child(childNum + i)
		if err != nil {
			return nil, nil, fmt.Errorf("child %d: failed to generate child ID: %w", i+1, err)
		}
		childIDs[i] = childID
		// The committed child's structure comes from its ID; derive the overlay
		// parent from that ID and never from the caller-supplied parentID.
		derivedParent, ok := childID.Parent()
		if !ok || derivedParent.String() != parentID.String() {
			return nil, nil, fmt.Errorf("child %d: %w: %s is not a child of %s",
				i+1, ErrParentIDMismatch, childID.String(), parentID.String())
		}

		// Resolve per-child dependencies (rk B2): "#N" is a backward sibling
		// ref into THIS batch (only known now, at allocation), anything else an
		// existing node id.
		deps, err := resolveChildDeps(spec.Dependencies, i, childIDs, st)
		if err != nil {
			return nil, nil, fmt.Errorf("child %d: %w", i+1, err)
		}

		childNode, err := node.NewNodeWithOptions(childID, spec.NodeType, spec.Statement, spec.Inference, node.NodeOptions{Dependencies: deps, Draft: spec.Draft, Crux: spec.Crux, Author: owner})
		if err != nil {
			return nil, nil, fmt.Errorf("child %d: %w", i+1, err)
		}
		events[i] = ledger.NewNodeCreated(*childNode)
		batch[i] = support.ProspectiveNode{
			ID:             childID,
			ParentID:       derivedParent,
			Type:           spec.NodeType,
			Dependencies:   childNode.Dependencies,
			ValidationDeps: childNode.ValidationDeps,
		}
	}

	// D1: cycle and scope checks over the WHOLE prospective child batch, so a
	// cycle formed only between two children of this batch is caught, and from
	// each child's own position so the error names the actual path.
	if err := checkSupportBatch(st, batch); err != nil {
		return nil, nil, err
	}
	return events, childIDs, nil
}

func (s *ProofService) RefineNodeBulk(parentID types.NodeID, owner string, children []ChildSpec) ([]types.NodeID, error) {
	if len(children) == 0 {
		return nil, fmt.Errorf("%w: at least one child specification is required", ErrEmptyInput)
	}

	// Validate depth for children (all children will have parent depth + 1)
	childDepth := parentID.Depth() + 1
	if err := s.validateDepth(childDepth); err != nil {
		return nil, err
	}

	var childIDs []types.NodeID
	var oldTaints map[string]node.TaintState
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
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

		events, ids, err := s.buildChildEvents(st, parentID, owner, children)
		if err != nil {
			return nil, err
		}
		childIDs = ids
		oldTaints = snapshotTaintStates(st)
		return events, nil
	})
	if err != nil {
		return nil, wrapSequenceMismatch(err, "RefineNodeBulk")
	}

	// Emit audit events once for the parent: the affected set (parent, its
	// ancestors, and all its descendants) already covers every new child, so a
	// single reload suffices. The NodeCreated events are committed at this
	// point and taint is derived, so an audit-emission failure must not hide
	// the created IDs from the caller (mirrors AcceptNodeBulk).
	if err := s.emitTaintRecomputedEvents(parentID, oldTaints); err != nil {
		return childIDs, err
	}

	return childIDs, nil
}

// ErrCircularDependency is returned when a cycle is detected in node dependencies.
// Exit code: 3 (logic error)
var ErrCircularDependency = aferrors.New(aferrors.DEPENDENCY_CYCLE, "circular dependency detected")

// ErrClaimTestRequired is returned when a crux node is accepted without a passing claim-test.
var ErrClaimTestRequired = aferrors.New(aferrors.CLAIM_TEST_REQUIRED, "crux node requires passing claim-test before acceptance")

// ErrClaimTestStale is returned when a crux node's only passing claim-test was
// run against a different content hash than the node's current content, so it
// cannot gate acceptance. It is a plain sentinel wrapped alongside
// ErrClaimTestRequired so errors.Is can name the staleness specifically while
// callers that only care that a claim-test is missing still match.
var ErrClaimTestStale = errors.New("crux claim-test is stale relative to current content")

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

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		// Check epistemic state - can only amend pending nodes
		if n.EpistemicState != schema.EpistemicPending {
			return nil, fmt.Errorf("cannot amend node: epistemic state is %s, must be pending", n.EpistemicState)
		}

		// Check ownership - either unclaimed or owned by the caller
		if n.WorkflowState == schema.WorkflowClaimed {
			if n.ClaimedBy != owner {
				return nil, fmt.Errorf("node is claimed by %s, not %s", n.ClaimedBy, owner)
			}
		}
		// If unclaimed, any owner can amend (they're taking responsibility)

		return []ledger.Event{ledger.NewNodeAmended(nodeID, n.Statement, newStatement, owner)}, nil
	})
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

// UpdateExternal updates fields of an existing external reference.
// Only non-empty values are applied. Returns the updated external.
func (s *ProofService) UpdateExternal(id string, name, source, notes string) (*node.External, error) {
	ext, err := fs.ReadExternal(s.path, id)
	if err != nil {
		return nil, fmt.Errorf("external %q not found: %w", id, err)
	}

	if strings.TrimSpace(name) != "" {
		ext.Name = name
	}
	if strings.TrimSpace(source) != "" {
		ext.Source = source
		// Recompute content hash from new source
		sum := sha256.Sum256([]byte(source))
		ext.ContentHash = hex.EncodeToString(sum[:])
	}
	if notes != "" {
		ext.Notes = notes
	}

	if err := fs.WriteExternal(s.path, ext); err != nil {
		return nil, fmt.Errorf("writing updated external: %w", err)
	}

	return ext, nil
}

// RecomputeAllTaint re-synchronizes the ledger's TaintRecomputed audit trail
// with authoritative derived state. If dryRun is true, it reports stale audit
// values without appending their corrected events.
//
// Taint propagates through the proof tree based on epistemic states:
// - Admitted nodes are self_admitted
// - Admitted ancestors or descendants taint validated nodes
// - Pending ancestors or descendants make validated nodes unresolved
// - Archived and refuted branches are severed from upward propagation
func (s *ProofService) RecomputeAllTaint(dryRun bool) (*RecomputeTaintResult, error) {
	ldg, err := s.getLedger()
	if err != nil {
		return nil, err
	}
	auditTaints, err := lastAuditedTaintStates(ldg)
	if err != nil {
		return nil, err
	}

	var result *RecomputeTaintResult
	_, err = s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Get all nodes
		allNodes := st.AllNodes()
		if len(allNodes) == 0 {
			return nil, fmt.Errorf("proof not initialized or empty")
		}

		// LoadState has already derived correct taint, but use the shared primitive
		// here too so repair and replay cannot acquire different semantics.
		taint.RecomputeAll(allNodes)
		var changes []TaintChange
		var events []ledger.Event
		for _, n := range allNodes {
			oldTaint, ok := auditTaints[n.ID.String()]
			if !ok {
				// A valid ledger has a NodeCreated event for every node. Treat a
				// missing baseline defensively as already synchronized.
				oldTaint = n.TaintState
			}
			if oldTaint == n.TaintState {
				continue
			}
			changes = append(changes, TaintChange{
				NodeID:   n.ID.String(),
				OldTaint: TaintState(oldTaint),
				NewTaint: TaintState(n.TaintState),
			})
			if !dryRun {
				events = append(events, ledger.NewTaintRecomputed(n.ID, n.TaintState))
			}
		}

		result = &RecomputeTaintResult{
			TotalNodes:   len(allNodes),
			NodesChanged: len(changes),
			Changes:      changes,
			DryRun:       dryRun,
		}
		return events, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// lastAuditedTaintStates returns the last explicitly recorded taint for each
// node. Until the first TaintRecomputed event, the NodeCreated value is the
// audit baseline because that is what applyNodeCreated places in state.
func lastAuditedTaintStates(ldg *ledger.Ledger) (map[string]node.TaintState, error) {
	result := make(map[string]node.TaintState)
	err := ldg.Scan(func(seq int, data []byte) error {
		var envelope struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("parsing ledger event %d for taint audit: %w", seq, err)
		}

		switch envelope.Type {
		case ledger.EventNodeCreated:
			var event ledger.NodeCreated
			if err := json.Unmarshal(data, &event); err != nil {
				return fmt.Errorf("parsing NodeCreated event %d for taint audit: %w", seq, err)
			}
			result[event.Node.ID.String()] = event.Node.TaintState
		case ledger.EventTaintRecomputed:
			var event ledger.TaintRecomputed
			if err := json.Unmarshal(data, &event); err != nil {
				return fmt.Errorf("parsing TaintRecomputed event %d for taint audit: %w", seq, err)
			}
			result[event.NodeID.String()] = event.NewTaint
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
	return s.commitThenTaint(nodeID, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		// Validate epistemic state transition (only validated -> needs_refinement allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicNeedsRefinement); err != nil {
			return nil, fmt.Errorf("%w: node %s is in %s state, must be %s to request refinement",
				ErrInvalidState, nodeID.String(), n.EpistemicState, schema.EpistemicValidated)
		}

		return []ledger.Event{ledger.NewRefinementRequested(nodeID, reason, requestedBy)}, nil
	})
}

// UnvalidateNode revokes validation on a node, reverting it from validated
// back to pending for re-examination. This is a verifier action used when
// a validation error is discovered after acceptance.
//
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrInvalidState if the node is not in validated state.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) UnvalidateNode(nodeID types.NodeID, reason, revokedBy string) error {
	return s.commitThenTaint(nodeID, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		// Validate epistemic state transition (only validated -> pending allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
			return nil, fmt.Errorf("%w: node %s is in %s state, must be %s to unvalidate",
				ErrInvalidState, nodeID.String(), n.EpistemicState, schema.EpistemicValidated)
		}

		return []ledger.Event{ledger.NewNodeUnvalidated(nodeID, reason, revokedBy)}, nil
	})
}

// UnadmitNode revokes an admission on a node, reverting it from admitted back
// to pending. Admit is a temporary, taint-introducing escape hatch — once the
// underlying claim has been rigorously verified, unadmit clears the admission
// so the node can be properly accepted with af accept.
//
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrInvalidState if the node is not in admitted state.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) UnadmitNode(nodeID types.NodeID, reason, revokedBy string) error {
	return s.commitThenTaint(nodeID, func(st *state.State) ([]ledger.Event, error) {
		// Check if node exists
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		// Validate epistemic state transition (only admitted -> pending allowed)
		if err := schema.ValidateEpistemicTransition(n.EpistemicState, schema.EpistemicPending); err != nil {
			return nil, fmt.Errorf("%w: node %s is in %s state, must be %s to unadmit",
				ErrInvalidState, nodeID.String(), n.EpistemicState, schema.EpistemicAdmitted)
		}

		return []ledger.Event{ledger.NewNodeUnadmitted(nodeID, reason, revokedBy)}, nil
	})
}

func snapshotTaintStates(st *state.State) map[string]node.TaintState {
	taints := make(map[string]node.TaintState)
	if st == nil {
		return taints
	}
	for _, n := range st.AllNodes() {
		if n != nil {
			taints[n.ID.String()] = n.TaintState
		}
	}
	return taints
}

// RecordApproachTried records a failed proof approach for a node.
// This prevents other agents from re-attempting the same dead end.
//
// Returns ErrEmptyInput if the approach description is empty.
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) RecordApproachTried(nodeID types.NodeID, approach, outcome, triedBy string) error {
	if strings.TrimSpace(approach) == "" {
		return fmt.Errorf("%w: approach description", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		return []ledger.Event{ledger.NewApproachTried(nodeID, approach, outcome, triedBy)}, nil
	})
	return wrapSequenceMismatch(err, "RecordApproachTried")
}

// ProposeStrategy records a proposed proof strategy for a node.
// This helps track what strategies have been considered before committing.
//
// Returns ErrEmptyInput if the strategy description is empty.
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) ProposeStrategy(nodeID types.NodeID, strategy, novelty, rationale, proposedBy string) error {
	if strings.TrimSpace(strategy) == "" {
		return fmt.Errorf("%w: strategy description", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		return []ledger.Event{ledger.NewStrategyProposed(nodeID, strategy, novelty, rationale, proposedBy)}, nil
	})
	return wrapSequenceMismatch(err, "ProposeStrategy")
}

// AddPattern registers a failure pattern in the workspace.
// Failure patterns capture recurring proof anti-patterns so agents can
// recognize and avoid them.
//
// Returns ErrEmptyInput if the name or description is empty.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) AddPattern(name, description, indicators, remediation, addedBy string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: pattern name", ErrEmptyInput)
	}
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("%w: pattern description", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{ledger.NewPatternAdded(name, description, indicators, remediation, addedBy)}, nil
	})
	return wrapSequenceMismatch(err, "AddPattern")
}

// AddHint adds a directional hint from a domain expert to a node.
// Hints provide guidance that provers must address (incorporate or explain why not).
//
// Returns ErrEmptyInput if the hint text is empty.
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) AddHint(nodeID types.NodeID, text, hintBy string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: hint text", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		return []ledger.Event{ledger.NewHintAdded(nodeID, text, hintBy)}, nil
	})
	return wrapSequenceMismatch(err, "AddHint")
}

// AttachEvidence links computational evidence (a script or result file) to a node.
// The file content is hashed for reproducibility verification.
//
// Returns ErrEmptyInput if the file path is empty.
// Returns ErrNodeNotFound if the node doesn't exist.
// Returns an error if the file cannot be read or hashed.
// Returns ErrConcurrentModification if the proof was modified by another process.
func (s *ProofService) AttachEvidence(nodeID types.NodeID, filePath, evidenceType, description, attachedBy string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("%w: file path", ErrEmptyInput)
	}

	// Compute content hash of the file
	absPath := filepath.Join(s.path, filePath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		// Try as absolute path
		data, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("cannot read evidence file: %w", err)
		}
	}
	sum := sha256.Sum256(data)
	contentHash := hex.EncodeToString(sum[:])

	_, err = s.commit(func(st *state.State) ([]ledger.Event, error) {
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		return []ledger.Event{ledger.NewEvidenceAttached(nodeID, filePath, contentHash, evidenceType, description, attachedBy)}, nil
	})
	return wrapSequenceMismatch(err, "AttachEvidence")
}

// SetOutline sets the proof outline with the given stages.
// This replaces any previous outline entirely.
func (s *ProofService) SetOutline(stages []ledger.OutlineStage, setBy string) error {
	if len(stages) == 0 {
		return fmt.Errorf("%w: at least one stage is required", ErrEmptyInput)
	}

	seen := make(map[string]bool)
	for i, stage := range stages {
		if strings.TrimSpace(stage.Label) == "" {
			return fmt.Errorf("%w: stage %d label cannot be empty", ErrEmptyInput, i+1)
		}
		if strings.TrimSpace(stage.Description) == "" {
			return fmt.Errorf("%w: stage %d description cannot be empty", ErrEmptyInput, i+1)
		}
		crit := strings.ToLower(stage.Criticality)
		if crit != "critical" && crit != "important" && crit != "routine" {
			return fmt.Errorf("stage %d: invalid criticality %q (must be critical, important, or routine)", i+1, stage.Criticality)
		}
		stages[i].Criticality = crit
		if seen[stage.Label] {
			return fmt.Errorf("duplicate stage label: %q", stage.Label)
		}
		seen[stage.Label] = true
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		return []ledger.Event{ledger.NewOutlineSet(stages, setBy)}, nil
	})
	return wrapSequenceMismatch(err, "SetOutline")
}

// LinkOutlineStage maps an outline stage to a subtree root node.
func (s *ProofService) LinkOutlineStage(label string, nodeID types.NodeID) error {
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("%w: label", ErrEmptyInput)
	}

	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		if !st.HasOutline() {
			return nil, fmt.Errorf("no outline set; use 'af outline set' first")
		}

		found := false
		for _, stage := range st.GetOutlineStages() {
			if stage.Label == label {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("outline stage %q not found in current outline", label)
		}

		if st.GetNode(nodeID) == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		return []ledger.Event{ledger.NewOutlineStageLinked(label, nodeID)}, nil
	})
	return wrapSequenceMismatch(err, "LinkOutlineStage")
}

// GetOutlineCoverage loads state and returns the outline coverage report.
func (s *ProofService) GetOutlineCoverage() (*state.OutlineCoverageReport, error) {
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}
	if !st.HasOutline() {
		return nil, fmt.Errorf("no outline set; use 'af outline set' first")
	}
	return st.GetOutlineCoverage(), nil
}

// RunClaimTest executes a computational falsification test against a node's claim.
// It runs the specified script or sympy expression, captures the result, and
// records a ClaimTested event in the ledger.
// Returns whether the test passed, the captured output, and any error.
func (s *ProofService) RunClaimTest(nodeID types.NodeID, engine, scriptPath, expression, agent string) (bool, string, error) {
	var passed bool
	var output string
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Validate node exists
		n := st.GetNode(nodeID)
		if n == nil {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID.String())
		}

		// Execute the test
		var execErr error
		switch engine {
		case "sympy":
			if strings.TrimSpace(expression) == "" {
				return nil, fmt.Errorf("%w: expression", ErrEmptyInput)
			}
			passed, output, execErr = executeSympyExpression(expression, s.path, 0)
		case "script", "":
			if strings.TrimSpace(scriptPath) == "" {
				return nil, fmt.Errorf("%w: script path", ErrEmptyInput)
			}
			passed, output, execErr = executeScript(scriptPath, s.path, 0)
			if engine == "" {
				engine = "script"
			}
		default:
			return nil, fmt.Errorf("unsupported engine %q: use 'script' or 'sympy'", engine)
		}
		if execErr != nil {
			return nil, fmt.Errorf("test execution failed: %w", execErr)
		}

		// D3: record the node's content hash at test time so acceptance can
		// tell whether this passing test still applies to the current content.
		event := ledger.NewClaimTested(nodeID, engine, scriptPath, expression, passed, output, agent)
		event.ContentHash = n.ContentHash
		return []ledger.Event{event}, nil
	})
	if err != nil {
		return passed, output, wrapSequenceMismatch(err, "RunClaimTest")
	}

	return passed, output, nil
}

// RunDefCheck executes a stress test against a registered definition.
// It resolves the definition by name or ID, runs the script, and records
// a DefChecked event in the ledger.
// Returns whether the check passed, the captured output, and any error.
func (s *ProofService) RunDefCheck(defNameOrID, checkType, scriptPath, agent string) (bool, string, error) {
	var passed bool
	var output string
	_, err := s.commit(func(st *state.State) ([]ledger.Event, error) {
		// Resolve definition by name or ID
		defName := defNameOrID
		def := st.GetDefinitionByName(defNameOrID)
		if def == nil {
			def = st.GetDefinition(defNameOrID)
		}
		if def == nil {
			return nil, fmt.Errorf("definition %q not found", defNameOrID)
		}
		defName = def.Name

		if strings.TrimSpace(scriptPath) == "" {
			return nil, fmt.Errorf("%w: script path", ErrEmptyInput)
		}
		if strings.TrimSpace(checkType) == "" {
			checkType = "script"
		}

		// Execute the check
		var execErr error
		passed, output, execErr = executeScript(scriptPath, s.path, 0)
		if execErr != nil {
			return nil, fmt.Errorf("check execution failed: %w", execErr)
		}

		return []ledger.Event{ledger.NewDefChecked(defName, checkType, scriptPath, passed, output, agent)}, nil
	})
	if err != nil {
		return passed, output, wrapSequenceMismatch(err, "RunDefCheck")
	}

	return passed, output, nil
}
