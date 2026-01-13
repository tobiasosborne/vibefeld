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

// ProofService orchestrates proof operations across ledger, state, locks, and filesystem.
// It provides a high-level facade for proof manipulation operations.
type ProofService struct {
	path string
}

// NewProofService creates a new ProofService for the given proof directory.
// Returns an error if the directory is invalid or inaccessible.
func NewProofService(path string) (*ProofService, error) {
	panic("not implemented")
}

// Init initializes a new proof with the given conjecture and author.
// Creates the initial proof structure and ledger event.
// Returns an error if the proof is already initialized or validation fails.
func Init(proofDir, conjecture, author string) error {
	panic("not implemented")
}

// Init initializes a new proof with the given conjecture and author.
// Creates the initial proof structure and ledger event.
// Returns an error if the proof is already initialized or validation fails.
func (s *ProofService) Init(conjecture, author string) error {
	panic("not implemented")
}

// LoadState loads and returns the current proof state by replaying ledger events.
func (s *ProofService) LoadState() (*state.State, error) {
	panic("not implemented")
}

// CreateNode creates a new proof node with the given parameters.
// The node is initially in available workflow state and pending epistemic state.
func (s *ProofService) CreateNode(id types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	panic("not implemented")
}

// ClaimNode claims a node for an agent with the given timeout.
// Returns an error if the node doesn't exist, is already claimed, or validation fails.
func (s *ProofService) ClaimNode(id types.NodeID, owner string, timeout time.Duration) error {
	panic("not implemented")
}

// ReleaseNode releases a claimed node, making it available again.
// Returns an error if the node is not claimed or the owner doesn't match.
func (s *ProofService) ReleaseNode(id types.NodeID, owner string) error {
	panic("not implemented")
}

// RefineNode adds a child node to a claimed parent node.
// Returns an error if the parent is not claimed by the owner or validation fails.
func (s *ProofService) RefineNode(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	panic("not implemented")
}

// AcceptNode validates a node, marking it as verified correct.
// Returns an error if the node doesn't exist.
func (s *ProofService) AcceptNode(id types.NodeID) error {
	panic("not implemented")
}

// AdmitNode admits a node without full verification.
// Returns an error if the node doesn't exist.
func (s *ProofService) AdmitNode(id types.NodeID) error {
	panic("not implemented")
}

// RefuteNode refutes a node, marking it as incorrect.
// Returns an error if the node doesn't exist.
func (s *ProofService) RefuteNode(id types.NodeID) error {
	panic("not implemented")
}

// AddDefinition adds a new definition to the proof.
// Returns the definition ID and any error.
func (s *ProofService) AddDefinition(name, content string) (string, error) {
	panic("not implemented")
}

// AddAssumption adds a new assumption to the proof.
// Returns the assumption ID and any error.
func (s *ProofService) AddAssumption(statement string) (string, error) {
	panic("not implemented")
}

// AddExternal adds a new external reference to the proof.
// Returns the external ID and any error.
func (s *ProofService) AddExternal(name, source string) (string, error) {
	panic("not implemented")
}

// ExtractLemma extracts a lemma from a source node.
// Returns the lemma ID and any error.
func (s *ProofService) ExtractLemma(sourceNodeID types.NodeID, statement string) (string, error) {
	panic("not implemented")
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
	panic("not implemented")
}

// GetAvailableNodes returns all nodes in the available workflow state.
func (s *ProofService) GetAvailableNodes() ([]*node.Node, error) {
	panic("not implemented")
}

// Path returns the proof directory path.
func (s *ProofService) Path() string {
	return s.path
}
