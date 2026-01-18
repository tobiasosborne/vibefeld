// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// ProofQueryOperations defines read-only query operations for proof state.
// Both provers and verifiers need these operations to understand proof state.
type ProofQueryOperations interface {
	// LoadState loads and returns the current proof state by replaying ledger events.
	// Also loads assumptions and externals from filesystem.
	LoadState() (*state.State, error)

	// LoadPendingNodes returns all nodes in the pending epistemic state.
	// Note: This method performs I/O to load state from disk.
	LoadPendingNodes() ([]*node.Node, error)

	// LoadAvailableNodes returns all nodes in the available workflow state.
	// Note: This method performs I/O to load state from disk.
	LoadAvailableNodes() ([]*node.Node, error)

	// Status returns the current status of the proof.
	Status() (*ProofStatus, error)

	// Path returns the proof directory path.
	Path() string
}

// ProverOperations defines operations that prover agents perform.
// These are operations for developing and extending the proof tree.
type ProverOperations interface {
	// ClaimNode claims a node for an agent with the given timeout.
	// Returns an error if the node doesn't exist, is already claimed, or validation fails.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. This is the primary defense against multiple agents
	// claiming the same node. Callers should retry after reloading state.
	ClaimNode(id types.NodeID, owner string, timeout time.Duration) error

	// RefreshClaim extends the claim timeout for a node the caller owns.
	// This allows agents to extend their claims without releasing and reclaiming,
	// which would risk another agent claiming the node in between.
	//
	// Returns an error if the node doesn't exist, is not claimed, or is claimed by
	// a different owner.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	RefreshClaim(id types.NodeID, owner string, timeout time.Duration) error

	// ReleaseNode releases a claimed node, making it available again.
	// Returns an error if the node is not claimed or the owner doesn't match.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	ReleaseNode(id types.NodeID, owner string) error

	// RefineNode adds a child node to a claimed parent node.
	// Returns an error if the parent is not claimed by the owner or validation fails.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	RefineNode(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error

	// AddDefinition adds a new definition to the proof.
	// Returns the definition ID and any error.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	AddDefinition(name, content string) (string, error)

	// AddAssumption adds a new assumption to the proof.
	// Returns the assumption ID and any error.
	AddAssumption(statement string) (string, error)

	// AddExternal adds a new external reference to the proof.
	// Returns the external ID and any error.
	AddExternal(name, source string) (string, error)

	// ExtractLemma extracts a lemma from a source node.
	// Returns the lemma ID and any error.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	ExtractLemma(sourceNodeID types.NodeID, statement string) (string, error)
}

// VerifierOperations defines operations that verifier agents perform.
// These are operations for validating, rejecting, or managing proof nodes.
type VerifierOperations interface {
	// AcceptNode validates a node, marking it as verified correct.
	// Returns an error if the node doesn't exist.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	AcceptNode(id types.NodeID) error

	// AcceptNodeBulk validates multiple nodes atomically, marking them as verified correct.
	// All nodes must exist and be in pending state.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	AcceptNodeBulk(ids []types.NodeID) error

	// AdmitNode admits a node without full verification.
	// Returns an error if the node doesn't exist.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	AdmitNode(id types.NodeID) error

	// RefuteNode refutes a node, marking it as incorrect.
	// Returns an error if the node doesn't exist.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	RefuteNode(id types.NodeID) error

	// ArchiveNode archives a node, abandoning the branch.
	// Returns an error if the node doesn't exist.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	ArchiveNode(id types.NodeID) error
}

// AdminOperations defines administrative operations for proof setup.
// These are typically used by the CLI for initial setup and node creation.
type AdminOperations interface {
	// Init initializes a new proof with the given conjecture and author.
	// Creates the initial proof structure and ledger event.
	// Returns an error if the proof is already initialized or validation fails.
	Init(conjecture, author string) error

	// CreateNode creates a new proof node with the given parameters.
	// The node is initially in available workflow state and pending epistemic state.
	//
	// Returns ErrConcurrentModification if the proof was modified by another process
	// since state was loaded. Callers should retry after reloading state.
	CreateNode(id types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error
}

// ProofOperations defines the full interface for proof manipulation operations.
// This is a composition of all role-based interfaces for convenience.
// Prefer using the smaller, focused interfaces when possible for better testability.
type ProofOperations interface {
	ProofQueryOperations
	ProverOperations
	VerifierOperations
	AdminOperations
}

// Ensure ProofService implements all interfaces
var _ ProofOperations = (*ProofService)(nil)
var _ ProofQueryOperations = (*ProofService)(nil)
var _ ProverOperations = (*ProofService)(nil)
var _ VerifierOperations = (*ProofService)(nil)
var _ AdminOperations = (*ProofService)(nil)
