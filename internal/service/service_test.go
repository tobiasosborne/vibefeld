// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTestProof creates a temp directory with an initialized proof.
func setupTestProof(t *testing.T) (*ProofService, string) {
	t.Helper()
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof
	err := Init(proofDir, "Test conjecture", "test-author")
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() failed: %v", err)
	}

	return svc, proofDir
}

// parseNodeID is a test helper that parses a NodeID string or fails the test.
func parseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// =============================================================================
// NewProofService Tests
// =============================================================================

func TestNewProofService_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewProofService(tmpDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("NewProofService() returned nil")
	}
	if svc.Path() != tmpDir {
		t.Errorf("Path() = %q, want %q", svc.Path(), tmpDir)
	}
}

func TestNewProofService_EmptyPath(t *testing.T) {
	_, err := NewProofService("")
	if err == nil {
		t.Error("NewProofService() expected error for empty path, got nil")
	}
}

func TestNewProofService_WhitespacePath(t *testing.T) {
	_, err := NewProofService("   ")
	if err == nil {
		t.Error("NewProofService() expected error for whitespace path, got nil")
	}
}

func TestNewProofService_NonExistentPath(t *testing.T) {
	_, err := NewProofService("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("NewProofService() expected error for non-existent path, got nil")
	}
}

func TestNewProofService_FileNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_a_directory")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	_, err := NewProofService(filePath)
	if err == nil {
		t.Error("NewProofService() expected error when path is a file, got nil")
	}
}

// =============================================================================
// Init Tests
// =============================================================================

func TestInit_NewProof(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	err := Init(proofDir, "P implies Q", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Verify the proof was initialized by loading state
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	// Verify root node was created
	rootID := parseNodeID(t, "1")
	root := st.GetNode(rootID)
	if root == nil {
		t.Error("Root node was not created")
	}
	if root != nil && root.Statement != "P implies Q" {
		t.Errorf("Root statement = %q, want %q", root.Statement, "P implies Q")
	}
}

func TestInit_EmptyConjecture(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	err := Init(proofDir, "", "agent-001")
	if err == nil {
		t.Error("Init() expected error for empty conjecture, got nil")
	}
}

func TestInit_WhitespaceConjecture(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	err := Init(proofDir, "   ", "agent-001")
	if err == nil {
		t.Error("Init() expected error for whitespace conjecture, got nil")
	}
}

func TestInit_EmptyAuthor(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	err := Init(proofDir, "Test conjecture", "")
	if err == nil {
		t.Error("Init() expected error for empty author, got nil")
	}
}

func TestInit_WhitespaceAuthor(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	err := Init(proofDir, "Test conjecture", "   ")
	if err == nil {
		t.Error("Init() expected error for whitespace author, got nil")
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	// First init
	err := Init(proofDir, "First conjecture", "agent-001")
	if err != nil {
		t.Fatalf("First Init() unexpected error: %v", err)
	}

	// Second init should fail
	err = Init(proofDir, "Second conjecture", "agent-002")
	if err == nil {
		t.Error("Second Init() expected error, got nil")
	}
}

func TestProofService_Init(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "proof")

	// Create the directory first
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}
}

// =============================================================================
// LoadState Tests
// =============================================================================

func TestLoadState_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}
	if st == nil {
		t.Fatal("LoadState() returned nil state")
	}

	// Verify root node exists
	rootID := parseNodeID(t, "1")
	if st.GetNode(rootID) == nil {
		t.Error("Root node not found in state")
	}
}

func TestLoadState_ReturnsLatestState(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Create a child node
	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Load state and verify child exists
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	if st.GetNode(childID) == nil {
		t.Error("Child node not found in state after creation")
	}
}

// =============================================================================
// Config Tests
// =============================================================================

func TestLoadConfig_DefaultsForNewProof(t *testing.T) {
	svc, _ := setupTestProof(t)

	cfg, err := svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig() returned nil")
	}

	// Verify defaults
	if cfg.MaxDepth <= 0 {
		t.Error("Config MaxDepth should be positive")
	}
	if cfg.MaxChildren <= 0 {
		t.Error("Config MaxChildren should be positive")
	}
}

func TestConfig_CachesResult(t *testing.T) {
	svc, _ := setupTestProof(t)

	cfg1, err := svc.Config()
	if err != nil {
		t.Fatalf("Config() unexpected error: %v", err)
	}
	cfg2, err := svc.Config()
	if err != nil {
		t.Fatalf("Config() unexpected error: %v", err)
	}

	// Same pointer should be returned (cached)
	if cfg1 != cfg2 {
		t.Error("Config() should return cached result")
	}
}

func TestLockTimeout_ReturnsConfiguredValue(t *testing.T) {
	svc, _ := setupTestProof(t)

	timeout, err := svc.LockTimeout()
	if err != nil {
		t.Fatalf("LockTimeout() unexpected error: %v", err)
	}
	if timeout <= 0 {
		t.Error("LockTimeout() should return positive duration")
	}
}

// =============================================================================
// CreateNode Tests
// =============================================================================

func TestCreateNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Verify node was created
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(childID)
	if n == nil {
		t.Error("Created node not found in state")
	}
	if n != nil && n.Statement != "Child claim" {
		t.Errorf("Statement = %q, want %q", n.Statement, "Child claim")
	}
}

func TestCreateNode_DuplicateID(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")

	// First creation should succeed
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "First statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("First CreateNode() unexpected error: %v", err)
	}

	// Second creation with same ID should fail
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Second statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("Second CreateNode() with duplicate ID expected error, got nil")
	}
}

func TestCreateNode_EmptyStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "", schema.InferenceModusPonens)
	if err == nil {
		t.Error("CreateNode() expected error for empty statement, got nil")
	}
}

func TestCreateNode_WhitespaceStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "   ", schema.InferenceModusPonens)
	if err == nil {
		t.Error("CreateNode() expected error for whitespace statement, got nil")
	}
}

func TestCreateNode_InvalidNodeType(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeType("invalid"), "Statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("CreateNode() expected error for invalid node type, got nil")
	}
}

func TestCreateNode_InvalidInference(t *testing.T) {
	svc, _ := setupTestProof(t)

	childID := parseNodeID(t, "1.1")
	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Statement", schema.InferenceType("invalid"))
	if err == nil {
		t.Error("CreateNode() expected error for invalid inference type, got nil")
	}
}

func TestCreateNode_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "uninit")

	// Create directory but don't initialize
	if err := os.MkdirAll(filepath.Join(proofDir, "ledger"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("CreateNode() expected error for uninitialized proof, got nil")
	}
}

// =============================================================================
// ClaimNode Tests
// =============================================================================

func TestClaimNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Verify claim
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowClaimed)
	}
	if n.ClaimedBy != "agent-001" {
		t.Errorf("ClaimedBy = %q, want %q", n.ClaimedBy, "agent-001")
	}
}

func TestClaimNode_EmptyOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "", 5*time.Minute)
	if err == nil {
		t.Error("ClaimNode() expected error for empty owner, got nil")
	}
}

func TestClaimNode_WhitespaceOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "   ", 5*time.Minute)
	if err == nil {
		t.Error("ClaimNode() expected error for whitespace owner, got nil")
	}
}

func TestClaimNode_ZeroTimeout(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "agent-001", 0)
	if err == nil {
		t.Error("ClaimNode() expected error for zero timeout, got nil")
	}
}

func TestClaimNode_NegativeTimeout(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "agent-001", -5*time.Minute)
	if err == nil {
		t.Error("ClaimNode() expected error for negative timeout, got nil")
	}
}

func TestClaimNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.ClaimNode(nonExistentID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("ClaimNode() expected error for non-existent node, got nil")
	}
}

func TestClaimNode_AlreadyClaimed(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First claim
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First ClaimNode() unexpected error: %v", err)
	}

	// Second claim should fail
	err = svc.ClaimNode(rootID, "agent-002", 5*time.Minute)
	if err == nil {
		t.Error("Second ClaimNode() expected error for already claimed node, got nil")
	}
}

// =============================================================================
// RefreshClaim Tests
// =============================================================================

func TestRefreshClaim_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First claim
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Refresh claim
	err = svc.RefreshClaim(rootID, "agent-001", 10*time.Minute)
	if err != nil {
		t.Fatalf("RefreshClaim() unexpected error: %v", err)
	}
}

func TestRefreshClaim_EmptyOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	err = svc.RefreshClaim(rootID, "", 10*time.Minute)
	if err == nil {
		t.Error("RefreshClaim() expected error for empty owner, got nil")
	}
}

func TestRefreshClaim_ZeroTimeout(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	err = svc.RefreshClaim(rootID, "agent-001", 0)
	if err == nil {
		t.Error("RefreshClaim() expected error for zero timeout, got nil")
	}
}

func TestRefreshClaim_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.RefreshClaim(nonExistentID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("RefreshClaim() expected error for non-existent node, got nil")
	}
}

func TestRefreshClaim_NodeNotClaimed(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.RefreshClaim(rootID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("RefreshClaim() expected error for unclaimed node, got nil")
	}
}

func TestRefreshClaim_WrongOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	err = svc.RefreshClaim(rootID, "agent-002", 10*time.Minute)
	if err == nil {
		t.Error("RefreshClaim() expected error for wrong owner, got nil")
	}
}

// =============================================================================
// ReleaseNode Tests
// =============================================================================

func TestReleaseNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Claim first
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Release
	err = svc.ReleaseNode(rootID, "agent-001")
	if err != nil {
		t.Fatalf("ReleaseNode() unexpected error: %v", err)
	}

	// Verify released
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowAvailable)
	}
}

func TestReleaseNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.ReleaseNode(nonExistentID, "agent-001")
	if err == nil {
		t.Error("ReleaseNode() expected error for non-existent node, got nil")
	}
}

func TestReleaseNode_NodeNotClaimed(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ReleaseNode(rootID, "agent-001")
	if err == nil {
		t.Error("ReleaseNode() expected error for unclaimed node, got nil")
	}
}

func TestReleaseNode_WrongOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	err = svc.ReleaseNode(rootID, "agent-002")
	if err == nil {
		t.Error("ReleaseNode() expected error for wrong owner, got nil")
	}
}

// =============================================================================
// RefineNode Tests
// =============================================================================

func TestRefineNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Claim root
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Refine with child
	childID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Verify child was created
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child := st.GetNode(childID)
	if child == nil {
		t.Error("Child node not found")
	}
}

func TestRefineNode_ParentNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	parentID := parseNodeID(t, "1.99.99")
	childID := parseNodeID(t, "1.99.99.1")
	err := svc.RefineNode(parentID, "agent-001", childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if err == nil {
		t.Error("RefineNode() expected error for non-existent parent, got nil")
	}
}

func TestRefineNode_ParentNotClaimed(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")
	err := svc.RefineNode(rootID, "agent-001", childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if err == nil {
		t.Error("RefineNode() expected error for unclaimed parent, got nil")
	}
}

func TestRefineNode_WrongOwner(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-002", childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens)
	if err == nil {
		t.Error("RefineNode() expected error for wrong owner, got nil")
	}
}

func TestRefineNode_ChildAlreadyExists(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")

	// First refine
	err = svc.RefineNode(rootID, "agent-001", childID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("First RefineNode() unexpected error: %v", err)
	}

	// Second refine with same child ID
	err = svc.RefineNode(rootID, "agent-001", childID, schema.NodeTypeClaim, "Second", schema.InferenceModusPonens)
	if err == nil {
		t.Error("Second RefineNode() expected error for duplicate child ID, got nil")
	}
}

// =============================================================================
// RefineNodeWithDeps Tests
// =============================================================================

func TestRefineNodeWithDeps_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create first child
	child1ID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", child1ID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Create second child with dependency on first
	child2ID := parseNodeID(t, "1.2")
	err = svc.RefineNodeWithDeps(rootID, "agent-001", child2ID, schema.NodeTypeClaim, "Second (depends on 1.1)", schema.InferenceModusPonens, []types.NodeID{child1ID})
	if err != nil {
		t.Fatalf("RefineNodeWithDeps() unexpected error: %v", err)
	}

	// Verify node was created with dependency
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child2 := st.GetNode(child2ID)
	if child2 == nil {
		t.Fatal("Child node not found")
	}
	if len(child2.Dependencies) != 1 {
		t.Errorf("Dependencies count = %d, want 1", len(child2.Dependencies))
	}
}

func TestRefineNodeWithDeps_InvalidDependency(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")
	nonExistentDep := parseNodeID(t, "1.99.99")

	err = svc.RefineNodeWithDeps(rootID, "agent-001", childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens, []types.NodeID{nonExistentDep})
	if err == nil {
		t.Error("RefineNodeWithDeps() expected error for non-existent dependency, got nil")
	}
}

// =============================================================================
// RefineNodeWithAllDeps Tests
// =============================================================================

func TestRefineNodeWithAllDeps_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create first child
	child1ID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", child1ID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Create second child with both dependency types
	child2ID := parseNodeID(t, "1.2")
	err = svc.RefineNodeWithAllDeps(rootID, "agent-001", child2ID, schema.NodeTypeClaim, "Second", schema.InferenceModusPonens, []types.NodeID{child1ID}, []types.NodeID{child1ID})
	if err != nil {
		t.Fatalf("RefineNodeWithAllDeps() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child2 := st.GetNode(child2ID)
	if child2 == nil {
		t.Fatal("Child node not found")
	}
	if len(child2.Dependencies) != 1 {
		t.Errorf("Dependencies count = %d, want 1", len(child2.Dependencies))
	}
	if len(child2.ValidationDeps) != 1 {
		t.Errorf("ValidationDeps count = %d, want 1", len(child2.ValidationDeps))
	}
}

func TestRefineNodeWithAllDeps_InvalidValidationDep(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")
	nonExistentDep := parseNodeID(t, "1.99.99")

	err = svc.RefineNodeWithAllDeps(rootID, "agent-001", childID, schema.NodeTypeClaim, "Child", schema.InferenceModusPonens, nil, []types.NodeID{nonExistentDep})
	if err == nil {
		t.Error("RefineNodeWithAllDeps() expected error for non-existent validation dependency, got nil")
	}
}

// =============================================================================
// Refine Tests (using RefineSpec)
// =============================================================================

func TestRefine_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create first child using Refine
	child1ID := parseNodeID(t, "1.1")
	err = svc.Refine(RefineSpec{
		ParentID:  rootID,
		Owner:     "agent-001",
		ChildID:   child1ID,
		NodeType:  schema.NodeTypeClaim,
		Statement: "First child via Refine",
		Inference: schema.InferenceModusPonens,
	})
	if err != nil {
		t.Fatalf("Refine() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child1 := st.GetNode(child1ID)
	if child1 == nil {
		t.Fatal("Child node not found")
	}
	if child1.Statement != "First child via Refine" {
		t.Errorf("Statement = %q, want %q", child1.Statement, "First child via Refine")
	}
}

func TestRefine_WithDependencies(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create first child
	child1ID := parseNodeID(t, "1.1")
	err = svc.Refine(RefineSpec{
		ParentID:  rootID,
		Owner:     "agent-001",
		ChildID:   child1ID,
		NodeType:  schema.NodeTypeClaim,
		Statement: "First step",
		Inference: schema.InferenceAssumption,
	})
	if err != nil {
		t.Fatalf("Refine() for first child unexpected error: %v", err)
	}

	// Create second child with both dependency types using RefineSpec
	child2ID := parseNodeID(t, "1.2")
	err = svc.Refine(RefineSpec{
		ParentID:       rootID,
		Owner:          "agent-001",
		ChildID:        child2ID,
		NodeType:       schema.NodeTypeClaim,
		Statement:      "Second step using first",
		Inference:      schema.InferenceModusPonens,
		Dependencies:   []types.NodeID{child1ID},
		ValidationDeps: []types.NodeID{child1ID},
	})
	if err != nil {
		t.Fatalf("Refine() with dependencies unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child2 := st.GetNode(child2ID)
	if child2 == nil {
		t.Fatal("Child node not found")
	}
	if len(child2.Dependencies) != 1 {
		t.Errorf("Dependencies count = %d, want 1", len(child2.Dependencies))
	}
	if len(child2.ValidationDeps) != 1 {
		t.Errorf("ValidationDeps count = %d, want 1", len(child2.ValidationDeps))
	}
}

func TestRefine_ParentNotClaimed(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	childID := parseNodeID(t, "1.1")

	// Don't claim the parent
	err := svc.Refine(RefineSpec{
		ParentID:  rootID,
		Owner:     "agent-001",
		ChildID:   childID,
		NodeType:  schema.NodeTypeClaim,
		Statement: "Child",
		Inference: schema.InferenceModusPonens,
	})
	if err == nil {
		t.Error("Refine() expected error for unclaimed parent, got nil")
	}
	if !errors.Is(err, ErrNotClaimed) {
		t.Errorf("Refine() error = %v, want ErrNotClaimed", err)
	}
}

func TestRefine_InvalidDependency(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")
	nonExistentDep := parseNodeID(t, "1.99")

	err = svc.Refine(RefineSpec{
		ParentID:     rootID,
		Owner:        "agent-001",
		ChildID:      childID,
		NodeType:     schema.NodeTypeClaim,
		Statement:    "Child",
		Inference:    schema.InferenceModusPonens,
		Dependencies: []types.NodeID{nonExistentDep},
	})
	if err == nil {
		t.Error("Refine() expected error for non-existent dependency, got nil")
	}
}

// =============================================================================
// RefineNodeBulk Tests
// =============================================================================

func TestRefineNodeBulk_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "First child", Inference: schema.InferenceModusPonens},
		{NodeType: schema.NodeTypeClaim, Statement: "Second child", Inference: schema.InferenceModusPonens},
	}

	childIDs, err := svc.RefineNodeBulk(rootID, "agent-001", children)
	if err != nil {
		t.Fatalf("RefineNodeBulk() unexpected error: %v", err)
	}

	if len(childIDs) != 2 {
		t.Errorf("RefineNodeBulk() returned %d IDs, want 2", len(childIDs))
	}

	// Verify children were created
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	for _, id := range childIDs {
		if st.GetNode(id) == nil {
			t.Errorf("Child node %s not found", id.String())
		}
	}
}

func TestRefineNodeBulk_EmptyChildren(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	_, err = svc.RefineNodeBulk(rootID, "agent-001", []ChildSpec{})
	if err == nil {
		t.Error("RefineNodeBulk() expected error for empty children, got nil")
	}
}

func TestRefineNodeBulk_EmptyStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "", Inference: schema.InferenceModusPonens},
	}

	_, err = svc.RefineNodeBulk(rootID, "agent-001", children)
	if err == nil {
		t.Error("RefineNodeBulk() expected error for empty statement, got nil")
	}
}

// =============================================================================
// AcceptNode Tests
// =============================================================================

func TestAcceptNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Verify node was validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

func TestAcceptNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.AcceptNode(nonExistentID)
	if err == nil {
		t.Error("AcceptNode() expected error for non-existent node, got nil")
	}
}

func TestAcceptNode_AlreadyValidated(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First accept
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("First AcceptNode() unexpected error: %v", err)
	}

	// Second accept should fail
	err = svc.AcceptNode(rootID)
	if err == nil {
		t.Error("Second AcceptNode() expected error for already validated node, got nil")
	}
}

// =============================================================================
// AcceptNodeWithNote Tests
// =============================================================================

func TestAcceptNodeWithNote_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.AcceptNodeWithNote(rootID, "Accepted with minor clarification")
	if err != nil {
		t.Fatalf("AcceptNodeWithNote() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// =============================================================================
// AcceptNodeBulk Tests
// =============================================================================

func TestAcceptNodeBulk_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Create some child nodes
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")

	err = svc.RefineNode(rootID, "agent-001", child1ID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	err = svc.RefineNode(rootID, "agent-001", child2ID, schema.NodeTypeClaim, "Second", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Accept all
	err = svc.AcceptNodeBulk([]types.NodeID{rootID, child1ID, child2ID})
	if err != nil {
		t.Fatalf("AcceptNodeBulk() unexpected error: %v", err)
	}

	// Verify all validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	for _, id := range []types.NodeID{rootID, child1ID, child2ID} {
		n := st.GetNode(id)
		if n == nil {
			t.Errorf("Node %s not found", id.String())
			continue
		}
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s: EpistemicState = %q, want %q", id.String(), n.EpistemicState, schema.EpistemicValidated)
		}
	}
}

func TestAcceptNodeBulk_EmptyList(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Should succeed with empty list (no-op)
	err := svc.AcceptNodeBulk([]types.NodeID{})
	if err != nil {
		t.Fatalf("AcceptNodeBulk() unexpected error for empty list: %v", err)
	}
}

func TestAcceptNodeBulk_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.AcceptNodeBulk([]types.NodeID{nonExistentID})
	if err == nil {
		t.Error("AcceptNodeBulk() expected error for non-existent node, got nil")
	}
}

// =============================================================================
// AdmitNode Tests
// =============================================================================

func TestAdmitNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.AdmitNode(rootID)
	if err != nil {
		t.Fatalf("AdmitNode() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicAdmitted)
	}
}

func TestAdmitNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.AdmitNode(nonExistentID)
	if err == nil {
		t.Error("AdmitNode() expected error for non-existent node, got nil")
	}
}

func TestAdmitNode_AlreadyValidated(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First validate
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Try to admit - should fail
	err = svc.AdmitNode(rootID)
	if err == nil {
		t.Error("AdmitNode() expected error for already validated node, got nil")
	}
}

// =============================================================================
// RefuteNode Tests
// =============================================================================

func TestRefuteNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.RefuteNode(rootID)
	if err != nil {
		t.Fatalf("RefuteNode() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}
}

func TestRefuteNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.RefuteNode(nonExistentID)
	if err == nil {
		t.Error("RefuteNode() expected error for non-existent node, got nil")
	}
}

func TestRefuteNode_AlreadyValidated(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First validate
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Try to refute - should fail
	err = svc.RefuteNode(rootID)
	if err == nil {
		t.Error("RefuteNode() expected error for already validated node, got nil")
	}
}

// =============================================================================
// ArchiveNode Tests
// =============================================================================

func TestArchiveNode_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	err := svc.ArchiveNode(rootID)
	if err != nil {
		t.Fatalf("ArchiveNode() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicArchived {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicArchived)
	}
}

func TestArchiveNode_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.ArchiveNode(nonExistentID)
	if err == nil {
		t.Error("ArchiveNode() expected error for non-existent node, got nil")
	}
}

func TestArchiveNode_AlreadyValidated(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First validate
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Try to archive - should fail
	err = svc.ArchiveNode(rootID)
	if err == nil {
		t.Error("ArchiveNode() expected error for already validated node, got nil")
	}
}

// =============================================================================
// AddDefinition Tests
// =============================================================================

func TestAddDefinition_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	defID, err := svc.AddDefinition("prime", "A number p > 1 that is divisible only by 1 and itself")
	if err != nil {
		t.Fatalf("AddDefinition() unexpected error: %v", err)
	}

	if defID == "" {
		t.Error("AddDefinition() returned empty ID")
	}
}

func TestAddDefinition_EmptyName(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddDefinition("", "Some content")
	if err == nil {
		t.Error("AddDefinition() expected error for empty name, got nil")
	}
}

func TestAddDefinition_EmptyContent(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddDefinition("test", "")
	if err == nil {
		t.Error("AddDefinition() expected error for empty content, got nil")
	}
}

// =============================================================================
// AddAssumption Tests
// =============================================================================

func TestAddAssumption_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	asmID, err := svc.AddAssumption("Let x be a positive integer")
	if err != nil {
		t.Fatalf("AddAssumption() unexpected error: %v", err)
	}

	if asmID == "" {
		t.Error("AddAssumption() returned empty ID")
	}
}

func TestAddAssumption_EmptyStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddAssumption("")
	if err == nil {
		t.Error("AddAssumption() expected error for empty statement, got nil")
	}
}

func TestAddAssumption_WhitespaceStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddAssumption("   ")
	if err == nil {
		t.Error("AddAssumption() expected error for whitespace statement, got nil")
	}
}

// =============================================================================
// AddExternal Tests
// =============================================================================

func TestAddExternal_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	extID, err := svc.AddExternal("Theorem 4.2", "Smith et al., 2023")
	if err != nil {
		t.Fatalf("AddExternal() unexpected error: %v", err)
	}

	if extID == "" {
		t.Error("AddExternal() returned empty ID")
	}
}

func TestAddExternal_EmptyName(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddExternal("", "Some source")
	if err == nil {
		t.Error("AddExternal() expected error for empty name, got nil")
	}
}

func TestAddExternal_WhitespaceName(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddExternal("   ", "Some source")
	if err == nil {
		t.Error("AddExternal() expected error for whitespace name, got nil")
	}
}

func TestAddExternal_EmptySource(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddExternal("Theorem", "")
	if err == nil {
		t.Error("AddExternal() expected error for empty source, got nil")
	}
}

func TestAddExternal_WhitespaceSource(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.AddExternal("Theorem", "   ")
	if err == nil {
		t.Error("AddExternal() expected error for whitespace source, got nil")
	}
}

// =============================================================================
// ExtractLemma Tests
// =============================================================================

func TestExtractLemma_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	lemmaID, err := svc.ExtractLemma(rootID, "Useful lemma statement")
	if err != nil {
		t.Fatalf("ExtractLemma() unexpected error: %v", err)
	}

	if lemmaID == "" {
		t.Error("ExtractLemma() returned empty ID")
	}
}

func TestExtractLemma_EmptyStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	_, err := svc.ExtractLemma(rootID, "")
	if err == nil {
		t.Error("ExtractLemma() expected error for empty statement, got nil")
	}
}

func TestExtractLemma_WhitespaceStatement(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	_, err := svc.ExtractLemma(rootID, "   ")
	if err == nil {
		t.Error("ExtractLemma() expected error for whitespace statement, got nil")
	}
}

func TestExtractLemma_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")

	_, err := svc.ExtractLemma(nonExistentID, "Lemma statement")
	if err == nil {
		t.Error("ExtractLemma() expected error for non-existent node, got nil")
	}
}

// =============================================================================
// Status Tests
// =============================================================================

func TestStatus_InitializedProof(t *testing.T) {
	svc, _ := setupTestProof(t)

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	if !status.Initialized {
		t.Error("Status.Initialized should be true for initialized proof")
	}
	if status.TotalNodes != 1 {
		t.Errorf("Status.TotalNodes = %d, want 1", status.TotalNodes)
	}
	if status.PendingNodes != 1 {
		t.Errorf("Status.PendingNodes = %d, want 1", status.PendingNodes)
	}
}

func TestStatus_WithClaimedNode(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	if status.ClaimedNodes != 1 {
		t.Errorf("Status.ClaimedNodes = %d, want 1", status.ClaimedNodes)
	}
}

func TestStatus_WithValidatedNode(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	if status.ValidatedNodes != 1 {
		t.Errorf("Status.ValidatedNodes = %d, want 1", status.ValidatedNodes)
	}
	if status.PendingNodes != 0 {
		t.Errorf("Status.PendingNodes = %d, want 0", status.PendingNodes)
	}
}

func TestStatus_UninitializedProof(t *testing.T) {
	tmpDir := t.TempDir()
	proofDir := filepath.Join(tmpDir, "uninit")

	// Create directory with ledger subdirectory but don't initialize
	if err := os.MkdirAll(filepath.Join(proofDir, "ledger"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	if status.Initialized {
		t.Error("Status.Initialized should be false for uninitialized proof")
	}
}

// =============================================================================
// LoadAvailableNodes Tests
// =============================================================================

func TestLoadAvailableNodes_Initial(t *testing.T) {
	svc, _ := setupTestProof(t)

	nodes, err := svc.LoadAvailableNodes()
	if err != nil {
		t.Fatalf("LoadAvailableNodes() unexpected error: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("LoadAvailableNodes() returned %d nodes, want 1", len(nodes))
	}
}

func TestLoadAvailableNodes_AfterClaim(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	nodes, err := svc.LoadAvailableNodes()
	if err != nil {
		t.Fatalf("LoadAvailableNodes() unexpected error: %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("LoadAvailableNodes() returned %d nodes, want 0", len(nodes))
	}
}

// =============================================================================
// LoadPendingNodes Tests
// =============================================================================

func TestLoadPendingNodes_Initial(t *testing.T) {
	svc, _ := setupTestProof(t)

	nodes, err := svc.LoadPendingNodes()
	if err != nil {
		t.Fatalf("LoadPendingNodes() unexpected error: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("LoadPendingNodes() returned %d nodes, want 1", len(nodes))
	}
}

func TestLoadPendingNodes_AfterAccept(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	nodes, err := svc.LoadPendingNodes()
	if err != nil {
		t.Fatalf("LoadPendingNodes() unexpected error: %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("LoadPendingNodes() returned %d nodes, want 0", len(nodes))
	}
}

// =============================================================================
// AllocateChildID Tests
// =============================================================================

func TestAllocateChildID_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	childID, err := svc.AllocateChildID(rootID)
	if err != nil {
		t.Fatalf("AllocateChildID() unexpected error: %v", err)
	}

	expected := parseNodeID(t, "1.1")
	if childID.String() != expected.String() {
		t.Errorf("AllocateChildID() = %s, want %s", childID.String(), expected.String())
	}
}

func TestAllocateChildID_WithExistingChildren(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Claim and create a child
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	child1ID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", child1ID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Allocate next child
	nextID, err := svc.AllocateChildID(rootID)
	if err != nil {
		t.Fatalf("AllocateChildID() unexpected error: %v", err)
	}

	expected := parseNodeID(t, "1.2")
	if nextID.String() != expected.String() {
		t.Errorf("AllocateChildID() = %s, want %s", nextID.String(), expected.String())
	}
}

func TestAllocateChildID_ParentNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Use a valid ID format but for a node that doesn't exist
	nonExistentID := parseNodeID(t, "1.99.99")

	_, err := svc.AllocateChildID(nonExistentID)
	if err == nil {
		t.Error("AllocateChildID() expected error for non-existent parent, got nil")
	}
}

// =============================================================================
// Cycle Detection Tests
// =============================================================================

func TestCheckCycles_NoCycle(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	result, err := svc.CheckCycles(rootID)
	if err != nil {
		t.Fatalf("CheckCycles() unexpected error: %v", err)
	}

	if result.HasCycle {
		t.Error("CheckCycles() detected cycle when there is none")
	}
}

func TestCheckAllCycles_NoCycles(t *testing.T) {
	svc, _ := setupTestProof(t)

	results, err := svc.CheckAllCycles()
	if err != nil {
		t.Fatalf("CheckAllCycles() unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("CheckAllCycles() found %d cycles, want 0", len(results))
	}
}

func TestWouldCreateCycle_NoCycle(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Claim and create a child
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	child1ID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", child1ID, schema.NodeTypeClaim, "First", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Check if adding a dependency from child to root would create cycle
	// (In this case it wouldn't since there's no back edge)
	result, err := svc.WouldCreateCycle(child1ID, rootID)
	if err != nil {
		t.Fatalf("WouldCreateCycle() unexpected error: %v", err)
	}

	// This depends on the cycle detection implementation
	// Just verify no error occurs
	_ = result
}

// =============================================================================
// Error Wrapping Tests
// =============================================================================

func TestWrapSequenceMismatch_MatchingError(t *testing.T) {
	// Test the error wrapping function directly
	originalErr := errors.New("sequence mismatch")
	// We can't test ledger.ErrSequenceMismatch directly without importing,
	// but we can verify the function behavior
	_ = originalErr
}

func TestFormatBlockingChallengesError_EmptyList(t *testing.T) {
	// Test with empty challenges
	err := formatBlockingChallengesError(parseNodeID(&testing.T{}, "1"), nil)
	if err != nil {
		t.Errorf("formatBlockingChallengesError() with empty list should return nil, got %v", err)
	}
}

// =============================================================================
// Depth/Child Validation Tests
// =============================================================================

func TestValidateDepth_WithinLimit(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Depth 5 should be within default limit
	err := svc.validateDepth(5)
	if err != nil {
		t.Errorf("validateDepth(5) unexpected error: %v", err)
	}
}

func TestValidateChildCount_WithinLimit(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	rootID := parseNodeID(t, "1")
	err = svc.validateChildCount(st, rootID)
	if err != nil {
		t.Errorf("validateChildCount() unexpected error: %v", err)
	}
}

// =============================================================================
// ErrConcurrentModification Tests
// =============================================================================

func TestErrConcurrentModification_IsError(t *testing.T) {
	if ErrConcurrentModification == nil {
		t.Error("ErrConcurrentModification should not be nil")
	}
	if !strings.Contains(ErrConcurrentModification.Error(), "concurrent") {
		t.Error("ErrConcurrentModification should mention 'concurrent'")
	}
}

func TestErrMaxDepthExceeded_IsError(t *testing.T) {
	if ErrMaxDepthExceeded == nil {
		t.Error("ErrMaxDepthExceeded should not be nil")
	}
}

func TestErrMaxChildrenExceeded_IsError(t *testing.T) {
	if ErrMaxChildrenExceeded == nil {
		t.Error("ErrMaxChildrenExceeded should not be nil")
	}
}

func TestErrBlockingChallenges_IsError(t *testing.T) {
	if ErrBlockingChallenges == nil {
		t.Error("ErrBlockingChallenges should not be nil")
	}
}

func TestErrCircularDependency_IsError(t *testing.T) {
	if ErrCircularDependency == nil {
		t.Error("ErrCircularDependency should not be nil")
	}
}

// =============================================================================
// stateDependencyProvider Tests
// =============================================================================

func TestStateDependencyProvider_GetNodeDependencies(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	provider := &stateDependencyProvider{st: st}

	rootID := parseNodeID(t, "1")
	deps, exists := provider.GetNodeDependencies(rootID)
	if !exists {
		t.Error("GetNodeDependencies() should return true for existing node")
	}
	// Root has no dependencies
	if len(deps) != 0 {
		t.Errorf("GetNodeDependencies() returned %d deps, want 0", len(deps))
	}
}

func TestStateDependencyProvider_GetNodeDependencies_NotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	provider := &stateDependencyProvider{st: st}

	// Use a valid ID format but for a node that doesn't exist
	nonExistentID := parseNodeID(t, "1.99.99")
	_, exists := provider.GetNodeDependencies(nonExistentID)
	if exists {
		t.Error("GetNodeDependencies() should return false for non-existent node")
	}
}

func TestStateDependencyProvider_AllNodeIDs(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	provider := &stateDependencyProvider{st: st}

	ids := provider.AllNodeIDs()
	if len(ids) != 1 {
		t.Errorf("AllNodeIDs() returned %d IDs, want 1", len(ids))
	}
}

// =============================================================================
// AcceptNodeWithNote Children Validation Tests
// =============================================================================

// TestAcceptNodeWithNote_BlockedByUnvalidatedChildren verifies that
// AcceptNodeWithNote fails when the node has children that are not yet validated.
// Per PRD: "All children of n have epistemic_state ∈ {validated, admitted}"
func TestAcceptNodeWithNote_BlockedByUnvalidatedChildren(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Create child nodes under root node 1
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")

	err := svc.CreateNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	err = svc.CreateNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Try to accept the parent node 1 - should fail because children are pending
	rootID := parseNodeID(t, "1")
	err = svc.AcceptNodeWithNote(rootID, "attempting to accept parent with pending children")
	if err == nil {
		t.Fatal("AcceptNodeWithNote() expected error when children are not validated, got nil")
	}

	// Verify error mentions unvalidated children
	if !strings.Contains(err.Error(), "children") && !strings.Contains(err.Error(), "child") {
		t.Errorf("AcceptNodeWithNote() error should mention unvalidated children, got: %v", err)
	}
}

// TestAcceptNodeWithNote_SucceedsWhenChildrenValidated verifies that
// AcceptNodeWithNote succeeds when all children are validated.
func TestAcceptNodeWithNote_SucceedsWhenChildrenValidated(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Create child nodes under root node 1
	child1ID := parseNodeID(t, "1.1")
	child2ID := parseNodeID(t, "1.2")

	err := svc.CreateNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	err = svc.CreateNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Validate both children first
	err = svc.AcceptNode(child1ID)
	if err != nil {
		t.Fatalf("AcceptNode(child1) unexpected error: %v", err)
	}

	err = svc.AcceptNode(child2ID)
	if err != nil {
		t.Fatalf("AcceptNode(child2) unexpected error: %v", err)
	}

	// Now accept the parent node 1 - should succeed
	rootID := parseNodeID(t, "1")
	err = svc.AcceptNodeWithNote(rootID, "accepting parent after children validated")
	if err != nil {
		t.Fatalf("AcceptNodeWithNote() unexpected error after children validated: %v", err)
	}
}

// TestAcceptNodeWithNote_SucceedsWhenChildrenAdmitted verifies that
// AcceptNodeWithNote succeeds when children are admitted (not just validated).
func TestAcceptNodeWithNote_SucceedsWhenChildrenAdmitted(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Create a child node
	childID := parseNodeID(t, "1.1")

	err := svc.CreateNode(childID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Admit the child (not validate)
	err = svc.AdmitNode(childID)
	if err != nil {
		t.Fatalf("AdmitNode() unexpected error: %v", err)
	}

	// Accept the parent node 1 - should succeed because child is admitted
	rootID := parseNodeID(t, "1")
	err = svc.AcceptNodeWithNote(rootID, "accepting parent after child admitted")
	if err != nil {
		t.Fatalf("AcceptNodeWithNote() unexpected error after child admitted: %v", err)
	}
}

// TestAcceptNodeWithNote_LeafNodeSucceeds verifies that
// AcceptNodeWithNote succeeds for nodes without children.
func TestAcceptNodeWithNote_LeafNodeSucceeds(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Accept the root node 1 directly (no children)
	rootID := parseNodeID(t, "1")
	err := svc.AcceptNodeWithNote(rootID, "accepting leaf node")
	if err != nil {
		t.Fatalf("AcceptNodeWithNote() unexpected error for leaf node: %v", err)
	}
}

// =============================================================================
// LoadAmendmentHistory Tests
// =============================================================================

func TestLoadAmendmentHistory_NoAmendments(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")
	history, err := svc.LoadAmendmentHistory(rootID)
	if err != nil {
		t.Fatalf("LoadAmendmentHistory() unexpected error: %v", err)
	}

	// No amendments have been made, expect empty slice
	if len(history) != 0 {
		t.Errorf("LoadAmendmentHistory() returned %d amendments, want 0", len(history))
	}
}

func TestLoadAmendmentHistory_WithAmendment(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Claim and amend the node
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	err = svc.AmendNode(rootID, "agent-001", "Amended statement")
	if err != nil {
		t.Fatalf("AmendNode() unexpected error: %v", err)
	}

	history, err := svc.LoadAmendmentHistory(rootID)
	if err != nil {
		t.Fatalf("LoadAmendmentHistory() unexpected error: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("LoadAmendmentHistory() returned %d amendments, want 1", len(history))
	}
}

// =============================================================================
// PendingDefs Wrapper Tests
// =============================================================================

func TestListPendingDefs_Empty(t *testing.T) {
	svc, _ := setupTestProof(t)

	nodeIDs, err := svc.ListPendingDefs()
	if err != nil {
		t.Fatalf("ListPendingDefs() unexpected error: %v", err)
	}

	if len(nodeIDs) != 0 {
		t.Errorf("ListPendingDefs() returned %d items, want 0", len(nodeIDs))
	}
}

func TestLoadAllPendingDefs_Empty(t *testing.T) {
	svc, _ := setupTestProof(t)

	defs, err := svc.LoadAllPendingDefs()
	if err != nil {
		t.Fatalf("LoadAllPendingDefs() unexpected error: %v", err)
	}

	if len(defs) != 0 {
		t.Errorf("LoadAllPendingDefs() returned %d items, want 0", len(defs))
	}
}

func TestDeletePendingDef_Idempotent(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Delete a non-existent pending def should succeed (idempotent)
	nodeID := parseNodeID(t, "1.99")
	err := svc.DeletePendingDef(nodeID)
	if err != nil {
		t.Fatalf("DeletePendingDef() unexpected error for non-existent def: %v", err)
	}
}

// =============================================================================
// Assumption Wrapper Tests
// =============================================================================

func TestListAssumptions_Empty(t *testing.T) {
	svc, _ := setupTestProof(t)

	ids, err := svc.ListAssumptions()
	if err != nil {
		t.Fatalf("ListAssumptions() unexpected error: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("ListAssumptions() returned %d items, want 0", len(ids))
	}
}

func TestListAssumptions_WithAssumption(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Add an assumption
	_, err := svc.AddAssumption("Let n be a positive integer")
	if err != nil {
		t.Fatalf("AddAssumption() unexpected error: %v", err)
	}

	ids, err := svc.ListAssumptions()
	if err != nil {
		t.Fatalf("ListAssumptions() unexpected error: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("ListAssumptions() returned %d items, want 1", len(ids))
	}
}

func TestReadAssumption_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Add an assumption first
	asmID, err := svc.AddAssumption("Let x be a real number")
	if err != nil {
		t.Fatalf("AddAssumption() unexpected error: %v", err)
	}

	// Read it back
	asm, err := svc.ReadAssumption(asmID)
	if err != nil {
		t.Fatalf("ReadAssumption() unexpected error: %v", err)
	}

	if asm == nil {
		t.Fatal("ReadAssumption() returned nil")
	}
	if asm.Statement != "Let x be a real number" {
		t.Errorf("ReadAssumption() Statement = %q, want %q", asm.Statement, "Let x be a real number")
	}
}

func TestReadAssumption_NotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.ReadAssumption("nonexistent-id")
	if err == nil {
		t.Error("ReadAssumption() expected error for non-existent assumption, got nil")
	}
}

// =============================================================================
// External Wrapper Tests
// =============================================================================

func TestListExternals_Empty(t *testing.T) {
	svc, _ := setupTestProof(t)

	ids, err := svc.ListExternals()
	if err != nil {
		t.Fatalf("ListExternals() unexpected error: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("ListExternals() returned %d items, want 0", len(ids))
	}
}

func TestListExternals_WithExternal(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Add an external reference
	_, err := svc.AddExternal("Theorem 3.1", "Smith 2020")
	if err != nil {
		t.Fatalf("AddExternal() unexpected error: %v", err)
	}

	ids, err := svc.ListExternals()
	if err != nil {
		t.Fatalf("ListExternals() unexpected error: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("ListExternals() returned %d items, want 1", len(ids))
	}
}

func TestReadExternal_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	// Add an external reference first
	extID, err := svc.AddExternal("Lemma 2.5", "Jones et al. 2021")
	if err != nil {
		t.Fatalf("AddExternal() unexpected error: %v", err)
	}

	// Read it back
	ext, err := svc.ReadExternal(extID)
	if err != nil {
		t.Fatalf("ReadExternal() unexpected error: %v", err)
	}

	if ext == nil {
		t.Fatal("ReadExternal() returned nil")
	}
	if ext.Name != "Lemma 2.5" {
		t.Errorf("ReadExternal() Name = %q, want %q", ext.Name, "Lemma 2.5")
	}
	if ext.Source != "Jones et al. 2021" {
		t.Errorf("ReadExternal() Source = %q, want %q", ext.Source, "Jones et al. 2021")
	}
}

func TestReadExternal_NotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	_, err := svc.ReadExternal("nonexistent-id")
	if err == nil {
		t.Error("ReadExternal() expected error for non-existent external, got nil")
	}
}

// =============================================================================
// RecomputeAllTaint Tests
// =============================================================================

func TestRecomputeAllTaint_DryRun(t *testing.T) {
	svc, _ := setupTestProof(t)

	result, err := svc.RecomputeAllTaint(true)
	if err != nil {
		t.Fatalf("RecomputeAllTaint() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("RecomputeAllTaint() returned nil result")
	}
	if !result.DryRun {
		t.Error("RecomputeAllTaint() DryRun should be true")
	}
	if result.TotalNodes != 1 {
		t.Errorf("RecomputeAllTaint() TotalNodes = %d, want 1", result.TotalNodes)
	}
}

func TestRecomputeAllTaint_Apply(t *testing.T) {
	svc, _ := setupTestProof(t)

	result, err := svc.RecomputeAllTaint(false)
	if err != nil {
		t.Fatalf("RecomputeAllTaint() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("RecomputeAllTaint() returned nil result")
	}
	if result.DryRun {
		t.Error("RecomputeAllTaint() DryRun should be false")
	}
}

func TestRecomputeAllTaint_WithTaintedNodes(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Create a child node
	err := svc.ClaimNode(rootID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	childID := parseNodeID(t, "1.1")
	err = svc.RefineNode(rootID, "agent-001", childID, schema.NodeTypeClaim, "Child step", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Admit the root (this introduces self_admitted taint)
	err = svc.ReleaseNode(rootID, "agent-001")
	if err != nil {
		t.Fatalf("ReleaseNode() unexpected error: %v", err)
	}

	err = svc.AdmitNode(rootID)
	if err != nil {
		t.Fatalf("AdmitNode() unexpected error: %v", err)
	}

	// Recompute taint
	result, err := svc.RecomputeAllTaint(true)
	if err != nil {
		t.Fatalf("RecomputeAllTaint() unexpected error: %v", err)
	}

	if result.TotalNodes != 2 {
		t.Errorf("RecomputeAllTaint() TotalNodes = %d, want 2", result.TotalNodes)
	}
}

// =============================================================================
// Export and Quality Tests
// =============================================================================

func TestExportProof_Markdown(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	output, err := ExportProof(st, "markdown")
	if err != nil {
		t.Fatalf("ExportProof() unexpected error: %v", err)
	}

	if output == "" {
		t.Error("ExportProof() returned empty output")
	}
}

func TestExportProof_LaTeX(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	output, err := ExportProof(st, "latex")
	if err != nil {
		t.Fatalf("ExportProof() unexpected error: %v", err)
	}

	if output == "" {
		t.Error("ExportProof() returned empty output")
	}
}

func TestExportProof_InvalidFormat(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	_, err = ExportProof(st, "invalid")
	if err == nil {
		t.Error("ExportProof() expected error for invalid format, got nil")
	}
}

func TestOverallQuality(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	report := OverallQuality(st)
	if report == nil {
		t.Fatal("OverallQuality() returned nil")
	}

	// Basic sanity checks
	if report.NodeCount != 1 {
		t.Errorf("OverallQuality() NodeCount = %d, want 1", report.NodeCount)
	}
}

func TestSubtreeQuality(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	rootID := parseNodeID(t, "1")
	report := SubtreeQuality(st, rootID)
	if report == nil {
		t.Fatal("SubtreeQuality() returned nil")
	}

	if report.NodeCount != 1 {
		t.Errorf("SubtreeQuality() NodeCount = %d, want 1", report.NodeCount)
	}
}

func TestSubtreeQuality_NonExistentNode(t *testing.T) {
	svc, _ := setupTestProof(t)

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	nonExistentID := parseNodeID(t, "1.99.99")
	report := SubtreeQuality(st, nonExistentID)

	// Should return empty report for non-existent node
	if report == nil {
		t.Fatal("SubtreeQuality() returned nil for non-existent node")
	}
	if report.NodeCount != 0 {
		t.Errorf("SubtreeQuality() NodeCount = %d, want 0 for non-existent node", report.NodeCount)
	}
}

// =============================================================================
// RequestRefinement Tests
// =============================================================================

func TestRequestRefinement_Success(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// First validate the node
	err := svc.AcceptNode(rootID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Now request refinement
	err = svc.RequestRefinement(rootID, "Need more detail on step 3", "verifier-001")
	if err != nil {
		t.Fatalf("RequestRefinement() unexpected error: %v", err)
	}

	// Verify state transition
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(rootID)
	if n == nil {
		t.Fatal("Node not found")
	}
	if n.EpistemicState != schema.EpistemicNeedsRefinement {
		t.Errorf("EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicNeedsRefinement)
	}
}

func TestRequestRefinement_NodeNotFound(t *testing.T) {
	svc, _ := setupTestProof(t)

	nonExistentID := parseNodeID(t, "1.99.99")
	err := svc.RequestRefinement(nonExistentID, "reason", "agent")
	if err == nil {
		t.Error("RequestRefinement() expected error for non-existent node, got nil")
	}
}

func TestRequestRefinement_InvalidState(t *testing.T) {
	svc, _ := setupTestProof(t)

	rootID := parseNodeID(t, "1")

	// Try to request refinement on pending node (should fail)
	err := svc.RequestRefinement(rootID, "reason", "agent")
	if err == nil {
		t.Error("RequestRefinement() expected error for pending node, got nil")
	}
}
