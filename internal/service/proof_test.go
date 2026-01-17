//go:build integration

// Package service provides the proof service facade for coordinating
// proof operations across ledger, state, locks, and filesystem.
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTestDir creates a temporary directory for testing.
func setupTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// setupInitializedProof creates a temp directory with initialized proof structure.
func setupInitializedProof(t *testing.T) string {
	t.Helper()
	dir := setupTestDir(t)
	proofDir := filepath.Join(dir, "proof")
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("failed to initialize proof dir: %v", err)
	}
	return proofDir
}

// mustParseNodeID is a test helper that parses a NodeID string or fails the test.
func mustParseNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("Failed to parse NodeID %q: %v", s, err)
	}
	return id
}

// createTestNode creates a node for testing purposes.
func createTestNode(t *testing.T, id string, statement string) *node.Node {
	t.Helper()
	nodeID := mustParseNodeID(t, id)
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, statement, schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create test node: %v", err)
	}
	return n
}

// =============================================================================
// ProofService Creation Tests
// =============================================================================

// TestNewProofService_ValidPath verifies service creation with valid path.
func TestNewProofService_ValidPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"root proof dir", setupInitializedProof(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewProofService(tt.path)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}
			if svc == nil {
				t.Fatal("NewProofService() returned nil")
			}
		})
	}
}

// TestNewProofService_InvalidPath verifies error handling for invalid paths.
func TestNewProofService_InvalidPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"whitespace path", "   "},
		{"non-existent path", "/nonexistent/path/that/does/not/exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewProofService(tt.path)
			if err == nil {
				t.Error("NewProofService() expected error for invalid path, got nil")
			}
			if svc != nil {
				t.Error("NewProofService() should return nil service on error")
			}
		})
	}
}

// TestNewProofService_FileNotDirectory verifies error when path is a file.
func TestNewProofService_FileNotDirectory(t *testing.T) {
	dir := setupTestDir(t)
	filePath := filepath.Join(dir, "not_a_directory")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	svc, err := NewProofService(filePath)
	if err == nil {
		t.Error("NewProofService() expected error when path is a file, got nil")
	}
	if svc != nil {
		t.Error("NewProofService() should return nil service on error")
	}
}

// =============================================================================
// Proof Initialization Tests
// =============================================================================

// TestProofService_Init_NewProof verifies initializing a new proof.
func TestProofService_Init_NewProof(t *testing.T) {
	tests := []struct {
		name       string
		conjecture string
		author     string
	}{
		{"simple conjecture", "P implies Q", "agent-001"},
		{"math conjecture", "For all n, n^2 >= 0", "prover-alpha"},
		{"complex conjecture", "If P and (P implies Q), then Q", "verifier-beta"},
		{"unicode conjecture", "∀x. P(x) → Q(x)", "agent-unicode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestDir(t)
			proofDir := filepath.Join(dir, "proof")

			svc, err := NewProofService(proofDir)
			// Note: NewProofService might create the dir or fail if not exists
			// This tests the Init flow for new proofs
			if err != nil {
				// Try Init directly for new directory
				err = Init(proofDir, tt.conjecture, tt.author)
				if err != nil {
					t.Fatalf("Init() unexpected error: %v", err)
				}
				return
			}

			err = svc.Init(tt.conjecture, tt.author)
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}
		})
	}
}

// TestProofService_Init_AlreadyInitialized verifies error on double init.
func TestProofService_Init_AlreadyInitialized(t *testing.T) {
	proofDir := setupInitializedProof(t)

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	// First init should succeed
	err = svc.Init("First conjecture", "agent-001")
	if err != nil {
		t.Fatalf("First Init() unexpected error: %v", err)
	}

	// Second init should fail
	err = svc.Init("Second conjecture", "agent-002")
	if err == nil {
		t.Error("Second Init() expected error for already initialized proof, got nil")
	}
}

// TestProofService_Init_InvalidConjecture verifies validation of conjecture.
func TestProofService_Init_InvalidConjecture(t *testing.T) {
	tests := []struct {
		name       string
		conjecture string
		author     string
	}{
		{"empty conjecture", "", "agent-001"},
		{"whitespace conjecture", "   ", "agent-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)

			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init(tt.conjecture, tt.author)
			if err == nil {
				t.Error("Init() expected error for invalid conjecture, got nil")
			}
		})
	}
}

// TestProofService_Init_InvalidAuthor verifies validation of author.
func TestProofService_Init_InvalidAuthor(t *testing.T) {
	tests := []struct {
		name       string
		conjecture string
		author     string
	}{
		{"empty author", "Valid conjecture", ""},
		{"whitespace author", "Valid conjecture", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)

			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init(tt.conjecture, tt.author)
			if err == nil {
				t.Error("Init() expected error for invalid author, got nil")
			}
		})
	}
}

// =============================================================================
// State Loading Tests
// =============================================================================

// TestProofService_LoadState_EmptyProof verifies loading state from empty proof.
func TestProofService_LoadState_EmptyProof(t *testing.T) {
	proofDir := setupInitializedProof(t)

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	// Initialize the proof first
	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	if st == nil {
		t.Fatal("LoadState() returned nil state")
	}
}

// TestProofService_LoadState_WithNodes verifies loading state with nodes.
func TestProofService_LoadState_WithNodes(t *testing.T) {
	proofDir := setupInitializedProof(t)

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	// Initialize proof (creates root node "1" automatically)
	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Add a child node through the service (root "1" already exists from Init)
	childID := mustParseNodeID(t, "1.1")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Child claim", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Load state and verify nodes exist
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	// Verify root node from Init
	rootID := mustParseNodeID(t, "1")
	n := st.GetNode(rootID)
	if n == nil {
		t.Error("LoadState() should contain the root node from Init")
	}

	// Verify child node
	child := st.GetNode(childID)
	if child == nil {
		t.Error("LoadState() should contain the created child node")
	}
}

// TestProofService_LoadState_Uninitalized verifies error for uninitialized proof.
func TestProofService_LoadState_Uninitalized(t *testing.T) {
	proofDir := setupInitializedProof(t)

	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	// Don't initialize - try to load state directly
	st, err := svc.LoadState()
	// Behavior depends on implementation - might return empty state or error
	if err != nil {
		// This is acceptable - uninitialized proof can return error
		return
	}

	// If no error, state should be empty
	if st != nil {
		nodes := st.AllNodes()
		if len(nodes) != 0 {
			t.Errorf("LoadState() on uninitialized proof returned %d nodes, expected 0", len(nodes))
		}
	}
}

// =============================================================================
// Node Operations Tests
// =============================================================================

// TestProofService_CreateNode_ValidNode verifies creating valid nodes.
// Note: Init() already creates root node "1", so we test with child nodes.
func TestProofService_CreateNode_ValidNode(t *testing.T) {
	tests := []struct {
		name      string
		nodeID    string
		nodeType  schema.NodeType
		statement string
		inference schema.InferenceType
	}{
		{"child node", "1.1", schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens},
		{"sibling node", "1.2", schema.NodeTypeClaim, "Sibling statement", schema.InferenceModusPonens},
		{"local assume", "1.3", schema.NodeTypeLocalAssume, "Assume P", schema.InferenceLocalAssume},
		{"qed node", "1.1.1", schema.NodeTypeQED, "Therefore Q", schema.InferenceModusPonens},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			nodeID := mustParseNodeID(t, tt.nodeID)
			err = svc.CreateNode(nodeID, tt.nodeType, tt.statement, tt.inference)
			if err != nil {
				t.Fatalf("CreateNode() unexpected error: %v", err)
			}

			// Verify node was created
			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			n := st.GetNode(nodeID)
			if n == nil {
				t.Error("Created node not found in state")
			}
		})
	}
}

// TestProofService_CreateNode_InvalidInput verifies validation of node creation.
// Note: We use "1.1" since root "1" is already created by Init()
func TestProofService_CreateNode_InvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		nodeID    string
		nodeType  schema.NodeType
		statement string
		inference schema.InferenceType
	}{
		{"empty statement", "1.1", schema.NodeTypeClaim, "", schema.InferenceAssumption},
		{"whitespace statement", "1.1", schema.NodeTypeClaim, "   ", schema.InferenceAssumption},
		{"invalid node type", "1.1", schema.NodeType("invalid"), "Statement", schema.InferenceAssumption},
		{"invalid inference", "1.1", schema.NodeTypeClaim, "Statement", schema.InferenceType("invalid")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			nodeID := mustParseNodeID(t, tt.nodeID)
			err = svc.CreateNode(nodeID, tt.nodeType, tt.statement, tt.inference)
			if err == nil {
				t.Error("CreateNode() expected error for invalid input, got nil")
			}
		})
	}
}

// TestProofService_CreateNode_DuplicateID verifies error on duplicate node ID.
// Note: We use "1.1" since root "1" is already created by Init()
func TestProofService_CreateNode_DuplicateID(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1.1")

	// First creation should succeed
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "First statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("First CreateNode() unexpected error: %v", err)
	}

	// Second creation with same ID should fail
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Second statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("Second CreateNode() with duplicate ID expected error, got nil")
	}
}

// =============================================================================
// Claim/Release Tests
// =============================================================================

// TestProofService_ClaimNode_Success verifies claiming an available node.
// Note: Init() creates root node "1", so we use it directly or create child nodes.
func TestProofService_ClaimNode_Success(t *testing.T) {
	tests := []struct {
		name       string
		nodeID     string
		owner      string
		timeout    time.Duration
		needCreate bool // whether node needs to be created (root "1" doesn't)
	}{
		{"root node", "1", "agent-001", 5 * time.Minute, false},
		{"child node", "1.1", "prover-alpha", 10 * time.Minute, true},
		{"deep node", "1.2.3", "verifier-beta", 1 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			nodeID := mustParseNodeID(t, tt.nodeID)

			// Create the node if needed (root "1" is created by Init)
			if tt.needCreate {
				// For deep nodes, we need to create parent nodes first
				if tt.nodeID == "1.2.3" {
					parentID := mustParseNodeID(t, "1.2")
					err = svc.CreateNode(parentID, schema.NodeTypeClaim, "Parent statement", schema.InferenceModusPonens)
					if err != nil {
						t.Fatalf("CreateNode(parent) unexpected error: %v", err)
					}
				}
				err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceModusPonens)
				if err != nil {
					t.Fatalf("CreateNode() unexpected error: %v", err)
				}
			}

			// Claim the node
			err = svc.ClaimNode(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("ClaimNode() unexpected error: %v", err)
			}

			// Verify node is claimed
			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			n := st.GetNode(nodeID)
			if n == nil {
				t.Fatal("Node not found after claim")
			}

			if n.WorkflowState != schema.WorkflowClaimed {
				t.Errorf("Node WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowClaimed)
			}
		})
	}
}

// TestProofService_ClaimNode_AlreadyClaimed verifies error when claiming already claimed node.
// Note: Uses root node "1" created by Init()
func TestProofService_ClaimNode_AlreadyClaimed(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	// First claim should succeed
	err = svc.ClaimNode(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("First ClaimNode() unexpected error: %v", err)
	}

	// Second claim should fail
	err = svc.ClaimNode(nodeID, "agent-002", 5*time.Minute)
	if err == nil {
		t.Error("Second ClaimNode() on already claimed node expected error, got nil")
	}
}

// TestProofService_ClaimNode_InvalidOwner verifies error with invalid owner.
// Note: Uses root node "1" created by Init()
func TestProofService_ClaimNode_InvalidOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"empty owner", ""},
		{"whitespace owner", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			// Use root node "1" created by Init
			nodeID := mustParseNodeID(t, "1")

			err = svc.ClaimNode(nodeID, tt.owner, 5*time.Minute)
			if err == nil {
				t.Error("ClaimNode() with invalid owner expected error, got nil")
			}
		})
	}
}

// TestProofService_ClaimNode_InvalidTimeout verifies error with invalid timeout.
// Note: Uses root node "1" created by Init()
func TestProofService_ClaimNode_InvalidTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			// Use root node "1" created by Init
			nodeID := mustParseNodeID(t, "1")

			err = svc.ClaimNode(nodeID, "agent-001", tt.timeout)
			if err == nil {
				t.Error("ClaimNode() with invalid timeout expected error, got nil")
			}
		})
	}
}

// TestProofService_ClaimNode_NonExistent verifies error when claiming non-existent node.
// Note: Use node "2" which doesn't exist (Init creates "1")
func TestProofService_ClaimNode_NonExistent(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use node "2" which doesn't exist (Init creates "1")
	nodeID := mustParseNodeID(t, "1.99")
	err = svc.ClaimNode(nodeID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("ClaimNode() on non-existent node expected error, got nil")
	}
}

// TestProofService_ReleaseNode_Success verifies releasing a claimed node.
// Note: Uses root node "1" created by Init()
func TestProofService_ReleaseNode_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Release the node
	err = svc.ReleaseNode(nodeID, owner)
	if err != nil {
		t.Fatalf("ReleaseNode() unexpected error: %v", err)
	}

	// Verify node is available again
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node not found after release")
	}

	if n.WorkflowState != schema.WorkflowAvailable {
		t.Errorf("Node WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowAvailable)
	}
}

// TestProofService_ReleaseNode_WrongOwner verifies error when releasing with wrong owner.
// Note: Uses root node "1" created by Init()
func TestProofService_ReleaseNode_WrongOwner(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	err = svc.ClaimNode(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to release with different owner
	err = svc.ReleaseNode(nodeID, "agent-002")
	if err == nil {
		t.Error("ReleaseNode() with wrong owner expected error, got nil")
	}
}

// TestProofService_ReleaseNode_NotClaimed verifies error when releasing unclaimed node.
// Note: Uses root node "1" created by Init()
func TestProofService_ReleaseNode_NotClaimed(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init (not claimed yet)
	nodeID := mustParseNodeID(t, "1")

	// Try to release without claiming first
	err = svc.ReleaseNode(nodeID, "agent-001")
	if err == nil {
		t.Error("ReleaseNode() on unclaimed node expected error, got nil")
	}
}

// =============================================================================
// Refine Operation Tests
// =============================================================================

// TestProofService_RefineNode_Success verifies refining a claimed node.
// Note: Uses root node "1" created by Init()
func TestProofService_RefineNode_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Refine should add children to the claimed node
	childID := mustParseNodeID(t, "1.1")
	err = svc.RefineNode(nodeID, owner, childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
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
		t.Error("Refined child node not found in state")
	}
}

// TestProofService_RefineNode_NotOwner verifies error when refining with wrong owner.
// Note: Uses root node "1" created by Init()
func TestProofService_RefineNode_NotOwner(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	err = svc.ClaimNode(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to refine with different owner
	childID := mustParseNodeID(t, "1.1")
	err = svc.RefineNode(nodeID, "agent-002", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("RefineNode() with wrong owner expected error, got nil")
	}
}

// TestProofService_RefineNode_NotClaimed verifies error when refining unclaimed node.
// Note: Uses root node "1" created by Init()
func TestProofService_RefineNode_NotClaimed(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init (not claimed)
	nodeID := mustParseNodeID(t, "1")

	// Try to refine without claiming first
	childID := mustParseNodeID(t, "1.1")
	err = svc.RefineNode(nodeID, "agent-001", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err == nil {
		t.Error("RefineNode() on unclaimed node expected error, got nil")
	}
}

// TestProofService_RefineNodeWithDeps_Success verifies refining with dependencies.
// Note: Uses root node "1" created by Init()
func TestProofService_RefineNodeWithDeps_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// First create a child node without dependencies
	childID1 := mustParseNodeID(t, "1.1")
	err = svc.RefineNode(nodeID, owner, childID1, schema.NodeTypeClaim, "First step", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Now create a second child with dependency on the first
	childID2 := mustParseNodeID(t, "1.2")
	deps := []types.NodeID{childID1}
	err = svc.RefineNodeWithDeps(nodeID, owner, childID2, schema.NodeTypeClaim, "By step 1.1, we have...", schema.InferenceModusPonens, deps)
	if err != nil {
		t.Fatalf("RefineNodeWithDeps() unexpected error: %v", err)
	}

	// Verify child was created with dependencies
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child := st.GetNode(childID2)
	if child == nil {
		t.Fatal("Refined child node not found in state")
	}

	if len(child.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(child.Dependencies))
	}

	if len(child.Dependencies) > 0 && child.Dependencies[0].String() != "1.1" {
		t.Errorf("Expected dependency on '1.1', got %q", child.Dependencies[0].String())
	}
}

// TestProofService_RefineNodeWithDeps_NonExistentDep verifies error for missing dependency.
func TestProofService_RefineNodeWithDeps_NonExistentDep(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to create a node with dependency on non-existent node
	childID := mustParseNodeID(t, "1.1")
	nonExistentDep := mustParseNodeID(t, "1.5")
	deps := []types.NodeID{nonExistentDep}
	err = svc.RefineNodeWithDeps(nodeID, owner, childID, schema.NodeTypeClaim, "By step 1.5...", schema.InferenceModusPonens, deps)
	if err == nil {
		t.Error("RefineNodeWithDeps() with non-existent dependency expected error, got nil")
	}

	// Error should mention the missing node
	if err != nil && !strings.Contains(err.Error(), "1.5") {
		t.Errorf("Error should mention missing node '1.5', got: %q", err.Error())
	}
}

// TestProofService_RefineNodeWithDeps_MultipleDeps verifies refining with multiple dependencies.
func TestProofService_RefineNodeWithDeps_MultipleDeps(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create two initial children
	childID1 := mustParseNodeID(t, "1.1")
	err = svc.RefineNode(nodeID, owner, childID1, schema.NodeTypeClaim, "First step", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	childID2 := mustParseNodeID(t, "1.2")
	err = svc.RefineNode(nodeID, owner, childID2, schema.NodeTypeClaim, "Second step", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("RefineNode() unexpected error: %v", err)
	}

	// Now create a third child with dependencies on both
	childID3 := mustParseNodeID(t, "1.3")
	deps := []types.NodeID{childID1, childID2}
	err = svc.RefineNodeWithDeps(nodeID, owner, childID3, schema.NodeTypeClaim, "Combining steps 1.1 and 1.2", schema.InferenceModusPonens, deps)
	if err != nil {
		t.Fatalf("RefineNodeWithDeps() unexpected error: %v", err)
	}

	// Verify child was created with both dependencies
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	child := st.GetNode(childID3)
	if child == nil {
		t.Fatal("Refined child node not found in state")
	}

	if len(child.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(child.Dependencies))
	}
}

// =============================================================================
// Accept/Validate/Refute Tests
// =============================================================================

// TestProofService_AcceptNode_Success verifies accepting (validating) a node.
// Note: Uses root node "1" created by Init()
func TestProofService_AcceptNode_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	// Accept the node
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	// Verify epistemic state changed
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node not found after accept")
	}

	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// TestProofService_AcceptNode_NonExistent verifies error when accepting non-existent node.
// Note: Use node "2" which doesn't exist (Init creates "1")
func TestProofService_AcceptNode_NonExistent(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use node "2" which doesn't exist (Init creates "1")
	nodeID := mustParseNodeID(t, "1.99")
	err = svc.AcceptNode(nodeID)
	if err == nil {
		t.Error("AcceptNode() on non-existent node expected error, got nil")
	}
}

// TestProofService_AdmitNode_Success verifies admitting a node without full verification.
// Note: Uses root node "1" created by Init()
func TestProofService_AdmitNode_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	// Admit the node
	err = svc.AdmitNode(nodeID)
	if err != nil {
		t.Fatalf("AdmitNode() unexpected error: %v", err)
	}

	// Verify epistemic state changed
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node not found after admit")
	}

	if n.EpistemicState != schema.EpistemicAdmitted {
		t.Errorf("Node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicAdmitted)
	}
}

// TestProofService_RefuteNode_Success verifies refuting a node.
// Note: Uses root node "1" created by Init()
func TestProofService_RefuteNode_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	// Refute the node
	err = svc.RefuteNode(nodeID)
	if err != nil {
		t.Fatalf("RefuteNode() unexpected error: %v", err)
	}

	// Verify epistemic state changed
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(nodeID)
	if n == nil {
		t.Fatal("Node not found after refute")
	}

	if n.EpistemicState != schema.EpistemicRefuted {
		t.Errorf("Node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicRefuted)
	}
}

// =============================================================================
// Definition Management Tests
// =============================================================================

// TestProofService_AddDefinition_Success verifies adding a definition.
func TestProofService_AddDefinition_Success(t *testing.T) {
	tests := []struct {
		name    string
		defName string
		content string
	}{
		{"simple definition", "Triangle", "A polygon with three sides"},
		{"math definition", "Prime", "A natural number greater than 1 with no divisors other than 1 and itself"},
		{"unicode definition", "Continuous", "f is continuous at a if lim_{x→a} f(x) = f(a)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			defID, err := svc.AddDefinition(tt.defName, tt.content)
			if err != nil {
				t.Fatalf("AddDefinition() unexpected error: %v", err)
			}

			if defID == "" {
				t.Error("AddDefinition() returned empty definition ID")
			}

			// Verify definition can be retrieved
			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			def := st.GetDefinition(defID)
			if def == nil {
				t.Error("Definition not found in state")
			}
		})
	}
}

// TestProofService_AddDefinition_Invalid verifies validation of definitions.
func TestProofService_AddDefinition_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		defName string
		content string
	}{
		{"empty name", "", "Valid content"},
		{"whitespace name", "   ", "Valid content"},
		{"empty content", "ValidName", ""},
		{"whitespace content", "ValidName", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			_, err = svc.AddDefinition(tt.defName, tt.content)
			if err == nil {
				t.Error("AddDefinition() expected error for invalid input, got nil")
			}
		})
	}
}

// =============================================================================
// Assumption Management Tests
// =============================================================================

// TestProofService_AddAssumption_Success verifies adding an assumption.
func TestProofService_AddAssumption_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	asmID, err := svc.AddAssumption("Assume P is true")
	if err != nil {
		t.Fatalf("AddAssumption() unexpected error: %v", err)
	}

	if asmID == "" {
		t.Error("AddAssumption() returned empty assumption ID")
	}

	// Verify assumption can be retrieved
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	asm := st.GetAssumption(asmID)
	if asm == nil {
		t.Error("Assumption not found in state")
	}
}

// TestProofService_AddAssumption_Invalid verifies validation of assumptions.
func TestProofService_AddAssumption_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		statement string
	}{
		{"empty statement", ""},
		{"whitespace statement", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			_, err = svc.AddAssumption(tt.statement)
			if err == nil {
				t.Error("AddAssumption() expected error for invalid input, got nil")
			}
		})
	}
}

// =============================================================================
// External Reference Management Tests
// =============================================================================

// TestProofService_AddExternal_Success verifies adding an external reference.
func TestProofService_AddExternal_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	extID, err := svc.AddExternal("Fermat's Last Theorem", "Wiles, A. (1995)")
	if err != nil {
		t.Fatalf("AddExternal() unexpected error: %v", err)
	}

	if extID == "" {
		t.Error("AddExternal() returned empty external ID")
	}

	// Verify external can be retrieved
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	ext := st.GetExternal(extID)
	if ext == nil {
		t.Error("External not found in state")
	}
}

// TestProofService_AddExternal_Invalid verifies validation of externals.
func TestProofService_AddExternal_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		extName string
		source string
	}{
		{"empty name", "", "Valid source"},
		{"whitespace name", "   ", "Valid source"},
		{"empty source", "ValidName", ""},
		{"whitespace source", "ValidName", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = svc.Init("Test conjecture", "agent-001")
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			_, err = svc.AddExternal(tt.extName, tt.source)
			if err == nil {
				t.Error("AddExternal() expected error for invalid input, got nil")
			}
		})
	}
}

// =============================================================================
// Lemma Extraction Tests
// =============================================================================

// TestProofService_ExtractLemma_Success verifies extracting a lemma from a node.
// Note: Uses root node "1" created by Init()
func TestProofService_ExtractLemma_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	// Validate the node first (lemmas typically extracted from validated nodes)
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error: %v", err)
	}

	lemmaID, err := svc.ExtractLemma(nodeID, "Extracted lemma statement")
	if err != nil {
		t.Fatalf("ExtractLemma() unexpected error: %v", err)
	}

	if lemmaID == "" {
		t.Error("ExtractLemma() returned empty lemma ID")
	}

	// Verify lemma can be retrieved
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	lemma := st.GetLemma(lemmaID)
	if lemma == nil {
		t.Error("Lemma not found in state")
	}
}

// TestProofService_ExtractLemma_InvalidStatement verifies validation of lemma statement.
// Note: Uses root node "1" created by Init()
func TestProofService_ExtractLemma_InvalidStatement(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use root node "1" created by Init
	nodeID := mustParseNodeID(t, "1")

	_, err = svc.ExtractLemma(nodeID, "")
	if err == nil {
		t.Error("ExtractLemma() with empty statement expected error, got nil")
	}
}

// TestProofService_ExtractLemma_NonExistentNode verifies error for non-existent source node.
// Note: Use node "2" which doesn't exist (Init creates "1")
func TestProofService_ExtractLemma_NonExistentNode(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Use node "2" which doesn't exist (Init creates "1")
	nodeID := mustParseNodeID(t, "1.99")
	_, err = svc.ExtractLemma(nodeID, "Lemma statement")
	if err == nil {
		t.Error("ExtractLemma() on non-existent node expected error, got nil")
	}
}

// =============================================================================
// Table-Driven Comprehensive Tests
// =============================================================================

// TestProofService_NodeOperationsFlow verifies complete node operation workflow.
// Note: Uses root node "1" created by Init()
func TestProofService_NodeOperationsFlow(t *testing.T) {
	tests := []struct {
		name            string
		conjecture      string
		nodeID          string
		statement       string
		owner           string
		expectedWorkflow schema.WorkflowState
	}{
		{
			name:            "complete workflow",
			conjecture:      "For all x, P(x) implies Q(x)",
			nodeID:          "1",
			statement:       "Root claim",
			owner:           "prover-001",
			expectedWorkflow: schema.WorkflowClaimed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			// Step 1: Initialize proof (creates root node "1")
			err = svc.Init(tt.conjecture, tt.owner)
			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}

			// Step 2: Use root node "1" created by Init (no CreateNode needed)
			nodeID := mustParseNodeID(t, tt.nodeID)

			// Step 3: Claim node
			err = svc.ClaimNode(nodeID, tt.owner, 5*time.Minute)
			if err != nil {
				t.Fatalf("ClaimNode() unexpected error: %v", err)
			}

			// Step 4: Verify state
			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			n := st.GetNode(nodeID)
			if n == nil {
				t.Fatal("Node not found")
			}

			if n.WorkflowState != tt.expectedWorkflow {
				t.Errorf("WorkflowState = %q, want %q", n.WorkflowState, tt.expectedWorkflow)
			}

			// Step 5: Refine (add child)
			childID := mustParseNodeID(t, "1.1")
			err = svc.RefineNode(nodeID, tt.owner, childID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
			if err != nil {
				t.Fatalf("RefineNode() unexpected error: %v", err)
			}

			// Step 6: Release node
			err = svc.ReleaseNode(nodeID, tt.owner)
			if err != nil {
				t.Fatalf("ReleaseNode() unexpected error: %v", err)
			}

			// Step 7: Verify available again
			st, err = svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			n = st.GetNode(nodeID)
			if n.WorkflowState != schema.WorkflowAvailable {
				t.Errorf("After release WorkflowState = %q, want %q", n.WorkflowState, schema.WorkflowAvailable)
			}

			// Step 8: Accept child node
			err = svc.AcceptNode(childID)
			if err != nil {
				t.Fatalf("AcceptNode() unexpected error: %v", err)
			}

			// Step 9: Verify epistemic state
			st, err = svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState() unexpected error: %v", err)
			}

			child := st.GetNode(childID)
			if child == nil {
				t.Fatal("Child node not found")
			}
			if child.EpistemicState != schema.EpistemicValidated {
				t.Errorf("Child EpistemicState = %q, want %q", child.EpistemicState, schema.EpistemicValidated)
			}
		})
	}
}

// TestProofService_MultipleNodesConcurrentClaim verifies claiming multiple nodes.
// Note: Node "1" is created by Init(), so only create child nodes
func TestProofService_MultipleNodesConcurrentClaim(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create child nodes (root "1" is created by Init)
	childNodeIDs := []string{"1.1", "1.2", "1.1.1"}
	for i, idStr := range childNodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("CreateNode(%d) unexpected error: %v", i, err)
		}
	}

	// All nodes including root
	allNodeIDs := []string{"1", "1.1", "1.2", "1.1.1"}

	// Claim each node with different owners
	for i, idStr := range allNodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		owner := "agent-" + idStr
		err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
		if err != nil {
			t.Fatalf("ClaimNode(%d) unexpected error: %v", i, err)
		}
	}

	// Verify all nodes are claimed
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	for _, idStr := range allNodeIDs {
		nodeID := mustParseNodeID(t, idStr)
		n := st.GetNode(nodeID)
		if n == nil {
			t.Errorf("Node %s not found", idStr)
			continue
		}
		if n.WorkflowState != schema.WorkflowClaimed {
			t.Errorf("Node %s WorkflowState = %q, want %q", idStr, n.WorkflowState, schema.WorkflowClaimed)
		}
	}
}

// =============================================================================
// Status/Query Tests
// =============================================================================

// TestProofService_Status verifies getting proof status.
func TestProofService_Status(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	// Status should contain information about the proof
	if status == nil {
		t.Error("Status() returned nil")
	}
}

// TestProofService_GetAvailableNodes verifies listing available nodes.
// Note: Node "1" is created by Init(), so only create child nodes
func TestProofService_GetAvailableNodes(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create child nodes (root "1" is created by Init)
	for _, idStr := range []string{"1.1", "1.2"} {
		nodeID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement "+idStr, schema.InferenceModusPonens)
		if err != nil {
			t.Fatalf("CreateNode() unexpected error: %v", err)
		}
	}

	// Claim root node
	err = svc.ClaimNode(mustParseNodeID(t, "1"), "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Get available nodes - should exclude claimed node
	available, err := svc.GetAvailableNodes()
	if err != nil {
		t.Fatalf("GetAvailableNodes() unexpected error: %v", err)
	}

	// Should have 2 available nodes (1.1 and 1.2)
	if len(available) != 2 {
		t.Errorf("GetAvailableNodes() returned %d nodes, want 2", len(available))
	}
}

// =============================================================================
// Service Path/Directory Tests
// =============================================================================

// TestProofService_Path verifies getting the proof directory path.
func TestProofService_Path(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	path := svc.Path()
	if path != proofDir {
		t.Errorf("Path() = %q, want %q", path, proofDir)
	}
}

// =============================================================================
// Error Propagation Tests
// =============================================================================

// TestProofService_ErrorPropagation verifies that underlying errors are propagated correctly.
func TestProofService_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name      string
		operation func(svc *ProofService) error
		wantErr   bool
	}{
		{
			name: "load state before init",
			operation: func(svc *ProofService) error {
				_, err := svc.LoadState()
				return err
			},
			wantErr: false, // May return empty state
		},
		{
			name: "create node without init",
			operation: func(svc *ProofService) error {
				nodeID, _ := types.Parse("1")
				return svc.CreateNode(nodeID, schema.NodeTypeClaim, "Statement", schema.InferenceAssumption)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofDir := setupInitializedProof(t)
			svc, err := NewProofService(proofDir)
			if err != nil {
				t.Fatalf("NewProofService() unexpected error: %v", err)
			}

			err = tt.operation(svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("operation error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// Interface Compliance Tests (used to detect missing methods)
// =============================================================================

// These tests ensure ProofService will have the expected method signatures.
// They compile-check the interface but don't test behavior.

var _ interface {
	Init(conjecture, author string) error
	LoadState() (*state.State, error)
	CreateNode(id types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error
	ClaimNode(id types.NodeID, owner string, timeout time.Duration) error
	ReleaseNode(id types.NodeID, owner string) error
	RefineNode(parentID types.NodeID, owner string, childID types.NodeID, nodeType schema.NodeType, statement string, inference schema.InferenceType) error
	AcceptNode(id types.NodeID) error
	AdmitNode(id types.NodeID) error
	RefuteNode(id types.NodeID) error
	AddDefinition(name, content string) (string, error)
	AddAssumption(statement string) (string, error)
	AddExternal(name, source string) (string, error)
	ExtractLemma(sourceNodeID types.NodeID, statement string) (string, error)
	Status() (*ProofStatus, error)
	GetAvailableNodes() ([]*node.Node, error)
	Path() string
} = (*ProofService)(nil)

// Note: ProofStatus is defined in proof.go, not duplicated here.
// Tests use the ProofStatus type from the implementation.

// =============================================================================
// AllocateChildID Tests (vibefeld-hrap fix)
// =============================================================================

// TestProofService_AllocateChildID_Success verifies allocating child IDs atomically.
func TestProofService_AllocateChildID_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Root node "1" is created by Init
	parentID := mustParseNodeID(t, "1")

	// Allocate first child ID
	childID, err := svc.AllocateChildID(parentID)
	if err != nil {
		t.Fatalf("AllocateChildID() unexpected error: %v", err)
	}

	// Should return "1.1" since no children exist yet
	expectedID := mustParseNodeID(t, "1.1")
	if childID.String() != expectedID.String() {
		t.Errorf("AllocateChildID() = %q, want %q", childID.String(), expectedID.String())
	}
}

// TestProofService_AllocateChildID_SkipsExisting verifies that allocated IDs skip existing children.
func TestProofService_AllocateChildID_SkipsExisting(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")

	// Create first two children manually
	child1ID := mustParseNodeID(t, "1.1")
	child2ID := mustParseNodeID(t, "1.2")
	err = svc.CreateNode(child1ID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode(1.1) unexpected error: %v", err)
	}
	err = svc.CreateNode(child2ID, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode(1.2) unexpected error: %v", err)
	}

	// Allocate next child ID - should skip 1.1 and 1.2
	childID, err := svc.AllocateChildID(parentID)
	if err != nil {
		t.Fatalf("AllocateChildID() unexpected error: %v", err)
	}

	// Should return "1.3"
	expectedID := mustParseNodeID(t, "1.3")
	if childID.String() != expectedID.String() {
		t.Errorf("AllocateChildID() = %q, want %q", childID.String(), expectedID.String())
	}
}

// TestProofService_AllocateChildID_NonExistentParent verifies error for non-existent parent.
func TestProofService_AllocateChildID_NonExistentParent(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Try to allocate child for non-existent parent
	parentID := mustParseNodeID(t, "1.99")
	_, err = svc.AllocateChildID(parentID)
	if err == nil {
		t.Error("AllocateChildID() on non-existent parent expected error, got nil")
	}
}

// =============================================================================
// RefineNodeBulk Tests (vibefeld-9ayl fix)
// =============================================================================

// TestProofService_RefineNodeBulk_Success verifies creating multiple children atomically.
func TestProofService_RefineNodeBulk_Success(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Claim root node
	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"
	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Create multiple children at once
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "First child", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeCase, Statement: "Second child (case)", Inference: schema.InferenceLocalAssume},
		{NodeType: schema.NodeTypeClaim, Statement: "Third child", Inference: schema.InferenceModusPonens},
	}

	childIDs, err := svc.RefineNodeBulk(parentID, owner, children)
	if err != nil {
		t.Fatalf("RefineNodeBulk() unexpected error: %v", err)
	}

	// Should return 3 child IDs
	if len(childIDs) != 3 {
		t.Errorf("RefineNodeBulk() returned %d IDs, want 3", len(childIDs))
	}

	// Verify the IDs are sequential: 1.1, 1.2, 1.3
	expectedIDs := []string{"1.1", "1.2", "1.3"}
	for i, childID := range childIDs {
		if childID.String() != expectedIDs[i] {
			t.Errorf("Child %d ID = %q, want %q", i+1, childID.String(), expectedIDs[i])
		}
	}

	// Verify all children exist in state
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	for i, childID := range childIDs {
		child := st.GetNode(childID)
		if child == nil {
			t.Errorf("Child %d (%s) not found in state", i+1, childID.String())
			continue
		}
		if child.Statement != children[i].Statement {
			t.Errorf("Child %d statement = %q, want %q", i+1, child.Statement, children[i].Statement)
		}
		if child.Type != children[i].NodeType {
			t.Errorf("Child %d type = %q, want %q", i+1, child.Type, children[i].NodeType)
		}
	}
}

// TestProofService_RefineNodeBulk_SkipsExisting verifies bulk refine skips existing children.
func TestProofService_RefineNodeBulk_SkipsExisting(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"

	// Create first child manually
	child1ID := mustParseNodeID(t, "1.1")
	err = svc.CreateNode(child1ID, schema.NodeTypeClaim, "Existing child", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode(1.1) unexpected error: %v", err)
	}

	// Claim parent
	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Bulk create more children - should start from 1.2
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "New child 1", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "New child 2", Inference: schema.InferenceAssumption},
	}

	childIDs, err := svc.RefineNodeBulk(parentID, owner, children)
	if err != nil {
		t.Fatalf("RefineNodeBulk() unexpected error: %v", err)
	}

	// Should return 1.2 and 1.3 (skipping existing 1.1)
	expectedIDs := []string{"1.2", "1.3"}
	for i, childID := range childIDs {
		if childID.String() != expectedIDs[i] {
			t.Errorf("Child %d ID = %q, want %q", i+1, childID.String(), expectedIDs[i])
		}
	}
}

// TestProofService_RefineNodeBulk_EmptyChildren verifies error for empty children list.
func TestProofService_RefineNodeBulk_EmptyChildren(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to create with empty children list
	_, err = svc.RefineNodeBulk(parentID, owner, []ChildSpec{})
	if err == nil {
		t.Error("RefineNodeBulk() with empty children expected error, got nil")
	}
}

// TestProofService_RefineNodeBulk_NotClaimed verifies error when parent is not claimed.
func TestProofService_RefineNodeBulk_NotClaimed(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Don't claim the node
	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"

	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child", Inference: schema.InferenceAssumption},
	}

	_, err = svc.RefineNodeBulk(parentID, owner, children)
	if err == nil {
		t.Error("RefineNodeBulk() on unclaimed parent expected error, got nil")
	}
}

// TestProofService_RefineNodeBulk_WrongOwner verifies error when owner doesn't match.
func TestProofService_RefineNodeBulk_WrongOwner(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")

	// Claim with agent-001
	err = svc.ClaimNode(parentID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to refine with agent-002
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child", Inference: schema.InferenceAssumption},
	}

	_, err = svc.RefineNodeBulk(parentID, "agent-002", children)
	if err == nil {
		t.Error("RefineNodeBulk() with wrong owner expected error, got nil")
	}
}

// TestProofService_RefineNodeBulk_NonExistentParent verifies error for non-existent parent.
func TestProofService_RefineNodeBulk_NonExistentParent(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Non-existent parent
	parentID := mustParseNodeID(t, "1.99")
	owner := "agent-001"

	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child", Inference: schema.InferenceAssumption},
	}

	_, err = svc.RefineNodeBulk(parentID, owner, children)
	if err == nil {
		t.Error("RefineNodeBulk() on non-existent parent expected error, got nil")
	}
}

// TestProofService_RefineNodeBulk_EmptyStatement verifies error for empty statement in child spec.
func TestProofService_RefineNodeBulk_EmptyStatement(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// One child has empty statement
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Valid child", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "", Inference: schema.InferenceAssumption}, // Empty!
	}

	_, err = svc.RefineNodeBulk(parentID, owner, children)
	if err == nil {
		t.Error("RefineNodeBulk() with empty statement expected error, got nil")
	}
}

// TestProofService_RefineNodeBulk_SingleChild verifies single-child case works.
func TestProofService_RefineNodeBulk_SingleChild(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"

	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Single child
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Only child", Inference: schema.InferenceAssumption},
	}

	childIDs, err := svc.RefineNodeBulk(parentID, owner, children)
	if err != nil {
		t.Fatalf("RefineNodeBulk() unexpected error: %v", err)
	}

	if len(childIDs) != 1 {
		t.Errorf("RefineNodeBulk() returned %d IDs, want 1", len(childIDs))
	}

	expectedID := mustParseNodeID(t, "1.1")
	if childIDs[0].String() != expectedID.String() {
		t.Errorf("Child ID = %q, want %q", childIDs[0].String(), expectedID.String())
	}
}

// =============================================================================
// Config Integration Tests (vibefeld-de47)
// =============================================================================

// TestProofService_LoadConfig verifies that config is loaded correctly.
func TestProofService_LoadConfig(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	// LoadConfig should return default config before meta.json exists
	cfg, err := svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Verify default values
	if cfg.MaxDepth != 20 {
		t.Errorf("Default MaxDepth = %d, want 20", cfg.MaxDepth)
	}
	if cfg.MaxChildren != 20 {
		t.Errorf("Default MaxChildren = %d, want 20", cfg.MaxChildren)
	}
	if cfg.LockTimeout != 5*time.Minute {
		t.Errorf("Default LockTimeout = %v, want 5m", cfg.LockTimeout)
	}
}

// TestProofService_LockTimeout verifies the LockTimeout helper method.
func TestProofService_LockTimeout(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	timeout, err := svc.LockTimeout()
	if err != nil {
		t.Fatalf("LockTimeout() unexpected error: %v", err)
	}
	if timeout != 5*time.Minute {
		t.Errorf("LockTimeout() = %v, want 5m", timeout)
	}
}

// TestProofService_CreateNode_MaxDepthExceeded verifies depth validation in CreateNode.
func TestProofService_CreateNode_MaxDepthExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Default MaxDepth is 20, try to create a node at depth 21
	// Build an ID at depth 21: "1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1"
	deepID := "1"
	for i := 1; i <= 20; i++ {
		deepID += ".1"
	}

	nodeID := mustParseNodeID(t, deepID)
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Deep node", schema.InferenceAssumption)

	if err == nil {
		t.Error("CreateNode() at depth 21 should fail with MaxDepthExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("CreateNode() error = %v, want ErrMaxDepthExceeded", err)
	}
}

// TestProofService_CreateNode_AtMaxDepth verifies nodes at exactly MaxDepth are allowed.
func TestProofService_CreateNode_AtMaxDepth(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Default MaxDepth is 20, create a node at exactly depth 20
	// Build an ID at depth 20: "1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1.1"
	deepID := "1"
	for i := 1; i < 20; i++ {
		deepID += ".1"
	}

	nodeID := mustParseNodeID(t, deepID)
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Max depth node", schema.InferenceAssumption)

	if err != nil {
		t.Errorf("CreateNode() at max depth should succeed, got error: %v", err)
	}
}

// TestProofService_CreateNode_MaxChildrenExceeded verifies children limit in CreateNode.
func TestProofService_CreateNode_MaxChildrenExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Default MaxChildren is 20, create 20 children of root
	for i := 1; i <= 20; i++ {
		idStr := fmt.Sprintf("1.%d", i)
		childID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Child %d", i), schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", idStr, err)
		}
	}

	// 21st child should fail
	childID := mustParseNodeID(t, "1.21")
	err = svc.CreateNode(childID, schema.NodeTypeClaim, "Child 21", schema.InferenceAssumption)

	if err == nil {
		t.Error("CreateNode() with 21st child should fail with MaxChildrenExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxChildrenExceeded) {
		t.Errorf("CreateNode() error = %v, want ErrMaxChildrenExceeded", err)
	}
}

// TestProofService_RefineNode_MaxDepthExceeded verifies depth validation in RefineNode.
func TestProofService_RefineNode_MaxDepthExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create nodes up to depth 19, claim node at depth 19
	parentID := "1"
	for i := 1; i < 19; i++ {
		nextID := parentID + ".1"
		nodeID := mustParseNodeID(t, nextID)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Node at depth", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", nextID, err)
		}
		parentID = nextID
	}

	// Create node at depth 20
	depth20ID := parentID + ".1"
	nodeID := mustParseNodeID(t, depth20ID)
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Node at depth 20", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode(%s) unexpected error: %v", depth20ID, err)
	}

	// Claim node at depth 20
	owner := "agent-001"
	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to refine (add child at depth 21)
	childID := mustParseNodeID(t, depth20ID+".1")
	err = svc.RefineNode(nodeID, owner, childID, schema.NodeTypeClaim, "Child", schema.InferenceAssumption)

	if err == nil {
		t.Error("RefineNode() at depth 21 should fail with MaxDepthExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("RefineNode() error = %v, want ErrMaxDepthExceeded", err)
	}
}

// TestProofService_RefineNode_MaxChildrenExceeded verifies children limit in RefineNode.
func TestProofService_RefineNode_MaxChildrenExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create 20 children of root directly
	for i := 1; i <= 20; i++ {
		idStr := fmt.Sprintf("1.%d", i)
		childID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Child %d", i), schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", idStr, err)
		}
	}

	// Claim root
	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"
	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to refine (add 21st child)
	childID := mustParseNodeID(t, "1.21")
	err = svc.RefineNode(parentID, owner, childID, schema.NodeTypeClaim, "Child 21", schema.InferenceAssumption)

	if err == nil {
		t.Error("RefineNode() with 21st child should fail with MaxChildrenExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxChildrenExceeded) {
		t.Errorf("RefineNode() error = %v, want ErrMaxChildrenExceeded", err)
	}
}

// TestProofService_RefineNodeBulk_MaxDepthExceeded verifies depth validation in RefineNodeBulk.
func TestProofService_RefineNodeBulk_MaxDepthExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create nodes up to depth 20
	parentID := "1"
	for i := 1; i < 20; i++ {
		nextID := parentID + ".1"
		nodeID := mustParseNodeID(t, nextID)
		err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Node at depth", schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", nextID, err)
		}
		parentID = nextID
	}

	// Claim node at depth 20
	nodeID := mustParseNodeID(t, parentID)
	owner := "agent-001"
	err = svc.ClaimNode(nodeID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to bulk refine (add children at depth 21)
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child 1", Inference: schema.InferenceAssumption},
	}

	_, err = svc.RefineNodeBulk(nodeID, owner, children)

	if err == nil {
		t.Error("RefineNodeBulk() at depth 21 should fail with MaxDepthExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("RefineNodeBulk() error = %v, want ErrMaxDepthExceeded", err)
	}
}

// TestProofService_RefineNodeBulk_MaxChildrenExceeded verifies children limit in RefineNodeBulk.
func TestProofService_RefineNodeBulk_MaxChildrenExceeded(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create 18 children of root directly
	for i := 1; i <= 18; i++ {
		idStr := fmt.Sprintf("1.%d", i)
		childID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Child %d", i), schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", idStr, err)
		}
	}

	// Claim root
	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"
	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Try to bulk add 3 more children (would make 21, exceeding max 20)
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child 19", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "Child 20", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "Child 21", Inference: schema.InferenceAssumption},
	}

	_, err = svc.RefineNodeBulk(parentID, owner, children)

	if err == nil {
		t.Error("RefineNodeBulk() adding 3 children to parent with 18 should fail with MaxChildrenExceeded")
	}
	if err != nil && !errors.Is(err, ErrMaxChildrenExceeded) {
		t.Errorf("RefineNodeBulk() error = %v, want ErrMaxChildrenExceeded", err)
	}
}

// TestProofService_RefineNodeBulk_AtMaxChildren verifies bulk refine at exactly max children.
func TestProofService_RefineNodeBulk_AtMaxChildren(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create 18 children of root directly
	for i := 1; i <= 18; i++ {
		idStr := fmt.Sprintf("1.%d", i)
		childID := mustParseNodeID(t, idStr)
		err = svc.CreateNode(childID, schema.NodeTypeClaim, fmt.Sprintf("Child %d", i), schema.InferenceAssumption)
		if err != nil {
			t.Fatalf("CreateNode(%s) unexpected error: %v", idStr, err)
		}
	}

	// Claim root
	parentID := mustParseNodeID(t, "1")
	owner := "agent-001"
	err = svc.ClaimNode(parentID, owner, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimNode() unexpected error: %v", err)
	}

	// Add exactly 2 more children (makes 20, exactly at max)
	children := []ChildSpec{
		{NodeType: schema.NodeTypeClaim, Statement: "Child 19", Inference: schema.InferenceAssumption},
		{NodeType: schema.NodeTypeClaim, Statement: "Child 20", Inference: schema.InferenceAssumption},
	}

	childIDs, err := svc.RefineNodeBulk(parentID, owner, children)

	if err != nil {
		t.Errorf("RefineNodeBulk() at exactly max children should succeed, got error: %v", err)
	}
	if len(childIDs) != 2 {
		t.Errorf("RefineNodeBulk() returned %d IDs, want 2", len(childIDs))
	}
}

// =============================================================================
// Blocking Challenge Tests
// =============================================================================

// addChallengeToNode is a test helper that adds a challenge to a node by directly
// appending to the ledger. This bypasses the service layer to set up test fixtures.
func addChallengeToNode(t *testing.T, proofDir string, nodeID types.NodeID, challengeID, severity string) {
	t.Helper()
	ldg, err := ledger.NewLedger(filepath.Join(proofDir, "ledger"))
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	event := ledger.NewChallengeRaisedWithSeverity(challengeID, nodeID, "statement", "test challenge", severity, "")
	if _, err := ldg.Append(event); err != nil {
		t.Fatalf("failed to append challenge: %v", err)
	}
}

// TestProofService_AcceptNode_BlockedByCriticalChallenge verifies that AcceptNode
// fails when the node has an unresolved critical challenge.
func TestProofService_AcceptNode_BlockedByCriticalChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a critical challenge to the node
	addChallengeToNode(t, proofDir, nodeID, "chal-001", "critical")

	// Try to accept the node - should fail
	err = svc.AcceptNode(nodeID)
	if err == nil {
		t.Fatal("AcceptNode() expected error for node with critical challenge, got nil")
	}

	if !errors.Is(err, ErrBlockingChallenges) {
		t.Errorf("AcceptNode() error = %v, want error wrapping ErrBlockingChallenges", err)
	}

	if !strings.Contains(err.Error(), "chal-001") {
		t.Errorf("AcceptNode() error should mention challenge ID, got: %v", err)
	}
}

// TestProofService_AcceptNode_BlockedByMajorChallenge verifies that AcceptNode
// fails when the node has an unresolved major challenge.
func TestProofService_AcceptNode_BlockedByMajorChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a major challenge to the node
	addChallengeToNode(t, proofDir, nodeID, "chal-002", "major")

	// Try to accept the node - should fail
	err = svc.AcceptNode(nodeID)
	if err == nil {
		t.Fatal("AcceptNode() expected error for node with major challenge, got nil")
	}

	if !errors.Is(err, ErrBlockingChallenges) {
		t.Errorf("AcceptNode() error = %v, want error wrapping ErrBlockingChallenges", err)
	}
}

// TestProofService_AcceptNode_SucceedsWithMinorChallenge verifies that AcceptNode
// succeeds when the node only has minor challenges (non-blocking).
func TestProofService_AcceptNode_SucceedsWithMinorChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge to the node (non-blocking)
	addChallengeToNode(t, proofDir, nodeID, "chal-003", "minor")

	// Accept the node - should succeed
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error for node with minor challenge: %v", err)
	}

	// Verify node was validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	n := st.GetNode(nodeID)
	if n.EpistemicState != schema.EpistemicValidated {
		t.Errorf("Node EpistemicState = %q, want %q", n.EpistemicState, schema.EpistemicValidated)
	}
}

// TestProofService_AcceptNode_SucceedsWithNoteChallenge verifies that AcceptNode
// succeeds when the node only has note-level challenges (non-blocking).
func TestProofService_AcceptNode_SucceedsWithNoteChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a note challenge to the node (non-blocking)
	addChallengeToNode(t, proofDir, nodeID, "chal-004", "note")

	// Accept the node - should succeed
	err = svc.AcceptNode(nodeID)
	if err != nil {
		t.Fatalf("AcceptNode() unexpected error for node with note challenge: %v", err)
	}
}

// TestProofService_AcceptNodeWithNote_BlockedByCriticalChallenge verifies that
// AcceptNodeWithNote fails when the node has an unresolved critical challenge.
func TestProofService_AcceptNodeWithNote_BlockedByCriticalChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a critical challenge to the node
	addChallengeToNode(t, proofDir, nodeID, "chal-005", "critical")

	// Try to accept the node with a note - should fail
	err = svc.AcceptNodeWithNote(nodeID, "acceptance note")
	if err == nil {
		t.Fatal("AcceptNodeWithNote() expected error for node with critical challenge, got nil")
	}

	if !errors.Is(err, ErrBlockingChallenges) {
		t.Errorf("AcceptNodeWithNote() error = %v, want error wrapping ErrBlockingChallenges", err)
	}
}

// TestProofService_AcceptNodeWithNote_SucceedsWithMinorChallenge verifies that
// AcceptNodeWithNote succeeds when the node only has minor challenges.
func TestProofService_AcceptNodeWithNote_SucceedsWithMinorChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add a minor challenge to the node (non-blocking)
	addChallengeToNode(t, proofDir, nodeID, "chal-006", "minor")

	// Accept the node with a note - should succeed
	err = svc.AcceptNodeWithNote(nodeID, "accepting with minor issue noted")
	if err != nil {
		t.Fatalf("AcceptNodeWithNote() unexpected error for node with minor challenge: %v", err)
	}
}

// TestProofService_AcceptNodeBulk_BlockedByCriticalChallenge verifies that
// AcceptNodeBulk fails when any node has an unresolved critical challenge.
func TestProofService_AcceptNodeBulk_BlockedByCriticalChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create multiple child nodes
	nodeID1 := mustParseNodeID(t, "1.1")
	nodeID2 := mustParseNodeID(t, "1.2")

	err = svc.CreateNode(nodeID1, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	err = svc.CreateNode(nodeID2, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Add a critical challenge to the second node
	addChallengeToNode(t, proofDir, nodeID2, "chal-007", "critical")

	// Try to accept both nodes - should fail because of the challenge on nodeID2
	err = svc.AcceptNodeBulk([]types.NodeID{nodeID1, nodeID2})
	if err == nil {
		t.Fatal("AcceptNodeBulk() expected error for node with critical challenge, got nil")
	}

	if !errors.Is(err, ErrBlockingChallenges) {
		t.Errorf("AcceptNodeBulk() error = %v, want error wrapping ErrBlockingChallenges", err)
	}

	if !strings.Contains(err.Error(), "1.2") {
		t.Errorf("AcceptNodeBulk() error should mention blocked node ID, got: %v", err)
	}
}

// TestProofService_AcceptNodeBulk_BlockedByMajorChallenge verifies that
// AcceptNodeBulk fails when any node has an unresolved major challenge.
func TestProofService_AcceptNodeBulk_BlockedByMajorChallenge(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1.1")
	err = svc.CreateNode(nodeID, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Add a major challenge to the node
	addChallengeToNode(t, proofDir, nodeID, "chal-008", "major")

	// Try to accept the node - should fail
	err = svc.AcceptNodeBulk([]types.NodeID{nodeID})
	if err == nil {
		t.Fatal("AcceptNodeBulk() expected error for node with major challenge, got nil")
	}

	if !errors.Is(err, ErrBlockingChallenges) {
		t.Errorf("AcceptNodeBulk() error = %v, want error wrapping ErrBlockingChallenges", err)
	}
}

// TestProofService_AcceptNodeBulk_SucceedsWithMinorChallenges verifies that
// AcceptNodeBulk succeeds when nodes only have minor/note challenges.
func TestProofService_AcceptNodeBulk_SucceedsWithMinorChallenges(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	// Create multiple child nodes
	nodeID1 := mustParseNodeID(t, "1.1")
	nodeID2 := mustParseNodeID(t, "1.2")

	err = svc.CreateNode(nodeID1, schema.NodeTypeClaim, "Child 1", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	err = svc.CreateNode(nodeID2, schema.NodeTypeClaim, "Child 2", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("CreateNode() unexpected error: %v", err)
	}

	// Add minor/note challenges to the nodes (non-blocking)
	addChallengeToNode(t, proofDir, nodeID1, "chal-009", "minor")
	addChallengeToNode(t, proofDir, nodeID2, "chal-010", "note")

	// Accept both nodes - should succeed
	err = svc.AcceptNodeBulk([]types.NodeID{nodeID1, nodeID2})
	if err != nil {
		t.Fatalf("AcceptNodeBulk() unexpected error for nodes with minor/note challenges: %v", err)
	}

	// Verify nodes were validated
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState() unexpected error: %v", err)
	}

	for _, nodeID := range []types.NodeID{nodeID1, nodeID2} {
		n := st.GetNode(nodeID)
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("Node %s EpistemicState = %q, want %q", nodeID, n.EpistemicState, schema.EpistemicValidated)
		}
	}
}

// TestProofService_AcceptNode_MultipleBlockingChallenges verifies the error message
// when a node has multiple blocking challenges.
func TestProofService_AcceptNode_MultipleBlockingChallenges(t *testing.T) {
	proofDir := setupInitializedProof(t)
	svc, err := NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService() unexpected error: %v", err)
	}

	err = svc.Init("Test conjecture", "agent-001")
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	nodeID := mustParseNodeID(t, "1")

	// Add multiple blocking challenges
	addChallengeToNode(t, proofDir, nodeID, "chal-a", "critical")
	addChallengeToNode(t, proofDir, nodeID, "chal-b", "major")

	// Try to accept the node - should fail
	err = svc.AcceptNode(nodeID)
	if err == nil {
		t.Fatal("AcceptNode() expected error for node with multiple blocking challenges, got nil")
	}

	// Verify error mentions the count of challenges
	if !strings.Contains(err.Error(), "2 blocking challenge") {
		t.Errorf("AcceptNode() error should mention 2 blocking challenges, got: %v", err)
	}
}
