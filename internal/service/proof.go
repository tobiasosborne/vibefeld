// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
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
	// Validate path is not empty or whitespace
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path cannot be empty")
	}

	// Check if path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("path does not exist")
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("path is not a directory")
	}

	return &ProofService{path: path}, nil
}

// Init initializes a new proof with the given conjecture and author.
// Creates the initial proof structure and ledger event.
// Returns an error if the proof is already initialized or validation fails.
func Init(proofDir, conjecture, author string) error {
	// Validate inputs
	if strings.TrimSpace(conjecture) == "" {
		return errors.New("conjecture cannot be empty")
	}
	if strings.TrimSpace(author) == "" {
		return errors.New("author cannot be empty")
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
		return errors.New("proof already initialized")
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
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Load externals from filesystem
	if err := s.loadExternalsIntoState(st); err != nil {
		// Ignore errors if directory doesn't exist
		if !os.IsNotExist(err) {
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
func (s *ProofService) CreateNode(id types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	// Check if initialized
	init, err := s.isInitialized()
	if err != nil {
		return err
	}
	if !init {
		return errors.New("proof not initialized")
	}

	// Check if node already exists
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	if st.GetNode(id) != nil {
		return errors.New("node already exists")
	}

	// Create the node
	n, err := node.NewNode(id, nodeType, statement, inference)
	if err != nil {
		return err
	}

	// Get ledger and append event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeCreated(*n)
	_, err = ldg.Append(event)
	return err
}

// ClaimNode claims a node for an agent with the given timeout.
// Returns an error if the node doesn't exist, is already claimed, or validation fails.
func (s *ProofService) ClaimNode(id types.NodeID, owner string, timeout time.Duration) error {
	// Validate owner
	if strings.TrimSpace(owner) == "" {
		return errors.New("owner cannot be empty")
	}

	// Validate timeout
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return errors.New("node not found")
	}

	// Check if node is available
	if n.WorkflowState != schema.WorkflowAvailable {
		return errors.New("node is not available")
	}

	// Get ledger and append claim event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	// Calculate timeout timestamp
	timeoutTS := types.FromTime(time.Now().Add(timeout))

	event := ledger.NewNodesClaimed([]types.NodeID{id}, owner, timeoutTS)
	_, err = ldg.Append(event)
	return err
}

// ReleaseNode releases a claimed node, making it available again.
// Returns an error if the node is not claimed or the owner doesn't match.
func (s *ProofService) ReleaseNode(id types.NodeID, owner string) error {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return errors.New("node not found")
	}

	// Check if node is claimed
	if n.WorkflowState != schema.WorkflowClaimed {
		return errors.New("node is not claimed")
	}

	// Check if owner matches
	if n.ClaimedBy != owner {
		return errors.New("owner does not match")
	}

	// Get ledger and append release event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodesReleased([]types.NodeID{id})
	_, err = ldg.Append(event)
	return err
}

// RefineNode adds a child node to a claimed parent node.
// Returns an error if the parent is not claimed by the owner or validation fails.
func (s *ProofService) RefineNode(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if parent node exists
	parent := st.GetNode(parentID)
	if parent == nil {
		return errors.New("parent node not found")
	}

	// Check if parent is claimed
	if parent.WorkflowState != schema.WorkflowClaimed {
		return errors.New("parent node is not claimed")
	}

	// Check if owner matches
	if parent.ClaimedBy != owner {
		return errors.New("owner does not match")
	}

	// Check if child already exists
	if st.GetNode(childID) != nil {
		return errors.New("child node already exists")
	}

	// Create the child node
	child, err := node.NewNode(childID, nodeType, statement, inference)
	if err != nil {
		return err
	}

	// Get ledger and append event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeCreated(*child)
	_, err = ldg.Append(event)
	return err
}

// AcceptNode validates a node, marking it as verified correct.
// Returns an error if the node doesn't exist.
func (s *ProofService) AcceptNode(id types.NodeID) error {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return errors.New("node not found")
	}

	// Get ledger and append validation event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeValidated(id)
	_, err = ldg.Append(event)
	return err
}

// AdmitNode admits a node without full verification.
// Returns an error if the node doesn't exist.
func (s *ProofService) AdmitNode(id types.NodeID) error {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return errors.New("node not found")
	}

	// Get ledger and append admit event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeAdmitted(id)
	_, err = ldg.Append(event)
	return err
}

// RefuteNode refutes a node, marking it as incorrect.
// Returns an error if the node doesn't exist.
func (s *ProofService) RefuteNode(id types.NodeID) error {
	// Load current state
	st, err := s.LoadState()
	if err != nil {
		return err
	}

	// Check if node exists
	n := st.GetNode(id)
	if n == nil {
		return errors.New("node not found")
	}

	// Get ledger and append refute event
	ldg, err := s.getLedger()
	if err != nil {
		return err
	}

	event := ledger.NewNodeRefuted(id)
	_, err = ldg.Append(event)
	return err
}

// AddDefinition adds a new definition to the proof.
// Returns the definition ID and any error.
func (s *ProofService) AddDefinition(name, content string) (string, error) {
	// Create the definition (validates inputs)
	def, err := node.NewDefinition(name, content)
	if err != nil {
		return "", err
	}

	// Get ledger and append event
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
	_, err = ldg.Append(event)
	if err != nil {
		return "", err
	}

	return def.ID, nil
}

// AddAssumption adds a new assumption to the proof.
// Returns the assumption ID and any error.
func (s *ProofService) AddAssumption(statement string) (string, error) {
	// Validate statement
	if strings.TrimSpace(statement) == "" {
		return "", errors.New("assumption statement cannot be empty")
	}

	// Create the assumption
	asm := node.NewAssumption(statement)

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
		return "", errors.New("external reference name cannot be empty")
	}
	if strings.TrimSpace(source) == "" {
		return "", errors.New("external reference source cannot be empty")
	}

	// Create the external
	ext := node.NewExternal(name, source)

	// Store in filesystem (base path is the proof directory)
	if err := fs.WriteExternal(s.path, &ext); err != nil {
		return "", err
	}

	return ext.ID, nil
}

// ExtractLemma extracts a lemma from a source node.
// Returns the lemma ID and any error.
func (s *ProofService) ExtractLemma(sourceNodeID types.NodeID, statement string) (string, error) {
	// Validate statement
	if strings.TrimSpace(statement) == "" {
		return "", errors.New("lemma statement cannot be empty")
	}

	// Check if source node exists
	st, err := s.LoadState()
	if err != nil {
		return "", err
	}

	n := st.GetNode(sourceNodeID)
	if n == nil {
		return "", errors.New("source node not found")
	}

	// Create the lemma
	lemma, err := node.NewLemma(statement, sourceNodeID)
	if err != nil {
		return "", err
	}

	// Append to ledger
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
	_, err = ldg.Append(event)
	if err != nil {
		return "", err
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

// GetAvailableNodes returns all nodes in the available workflow state.
func (s *ProofService) GetAvailableNodes() ([]*node.Node, error) {
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
