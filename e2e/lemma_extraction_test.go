//go:build integration

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/types"
)

// setupLemmaTest creates a temporary directory for the test and returns the proof directory path
// and a cleanup function.
func setupLemmaTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-lemma-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initLemmaProof initializes a proof with the given conjecture.
func initLemmaProof(t *testing.T, proofDir, conjecture string) *service.ProofService {
	t.Helper()
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("failed to initialize proof dir: %v", err)
	}
	err := service.Init(proofDir, conjecture, "test-author")
	if err != nil {
		t.Fatalf("failed to initialize proof: %v", err)
	}
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create proof service: %v", err)
	}
	return svc
}

// mustParseLemmaNodeID parses a node ID string and fails the test if it fails.
func mustParseLemmaNodeID(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse node ID %q: %v", s, err)
	}
	return id
}

// LemmaState tracks the state of lemmas from ledger events.
type LemmaState struct {
	ID        string
	Statement string
	NodeID    types.NodeID
}

// getLemmaStates replays the ledger and returns the current state of all lemmas.
func getLemmaStates(t *testing.T, proofDir string) map[string]*LemmaState {
	t.Helper()
	lemmas := make(map[string]*LemmaState)

	ledgerDir := filepath.Join(proofDir, "ledger")
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	err = ldg.Scan(func(seq int, data []byte) error {
		var base struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return err
		}

		if base.Type == ledger.EventLemmaExtracted {
			var e ledger.LemmaExtracted
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			lemmas[e.Lemma.ID] = &LemmaState{
				ID:        e.Lemma.ID,
				Statement: e.Lemma.Statement,
				NodeID:    e.Lemma.NodeID,
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to scan ledger: %v", err)
	}

	return lemmas
}

// TestLemma_ExtractFromValidatedSubtree tests extracting a lemma from a validated subtree.
// This is the primary workflow: create a proof, validate all nodes, then extract a lemma.
func TestLemma_ExtractFromValidatedSubtree(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create proof with a complete structure: root -> child -> grandchild
	conjecture := "If n is even and m is even, then n+m is even"
	svc := initLemmaProof(t, proofDir, conjecture)

	// Parse node IDs
	rootID := mustParseLemmaNodeID(t, "1")
	childID := mustParseLemmaNodeID(t, "1.1")
	grandchildID := mustParseLemmaNodeID(t, "1.1.1")

	// Claim root and add child
	proverOwner := "prover-agent"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// Refine root with child node
	if err := svc.RefineNode(rootID, proverOwner, childID, schema.NodeTypeClaim,
		"Let n = 2k for some integer k", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child) failed: %v", err)
	}

	// Release root so we can claim child
	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode (root) failed: %v", err)
	}

	// Claim child and add grandchild
	if err := svc.ClaimNode(childID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (child) failed: %v", err)
	}

	// Refine child with grandchild node
	if err := svc.RefineNode(childID, proverOwner, grandchildID, schema.NodeTypeClaim,
		"Then n+m = 2(k+j), which is even by definition", schema.InferenceModusPonens); err != nil {
		t.Fatalf("RefineNode (grandchild) failed: %v", err)
	}

	// Release child
	if err := svc.ReleaseNode(childID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode (child) failed: %v", err)
	}

	// 2. Validate all nodes (bottom-up is typical, but order shouldn't matter for validation)
	if err := svc.AcceptNode(grandchildID); err != nil {
		t.Fatalf("AcceptNode (grandchild) failed: %v", err)
	}
	if err := svc.AcceptNode(childID); err != nil {
		t.Fatalf("AcceptNode (child) failed: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// 3. Verify all nodes are validated
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, nodeID := range []types.NodeID{rootID, childID, grandchildID} {
		n := state.GetNode(nodeID)
		if n == nil {
			t.Fatalf("node %s not found", nodeID)
		}
		if n.EpistemicState != schema.EpistemicValidated {
			t.Errorf("node %s epistemic state = %v, want %v",
				nodeID, n.EpistemicState, schema.EpistemicValidated)
		}
	}

	// 4. Extract a lemma from the validated subtree
	lemmaStatement := "The sum of two even numbers is even"
	lemmaID, err := svc.ExtractLemma(childID, lemmaStatement)
	if err != nil {
		t.Fatalf("ExtractLemma failed: %v", err)
	}

	if lemmaID == "" {
		t.Fatal("ExtractLemma returned empty ID")
	}

	t.Logf("Extracted lemma with ID: %s", lemmaID)

	// 5. Verify lemma is in state
	state, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState after lemma extraction failed: %v", err)
	}

	lemma := state.GetLemma(lemmaID)
	if lemma == nil {
		t.Fatal("Lemma not found in state after extraction")
	}

	if lemma.Statement != lemmaStatement {
		t.Errorf("lemma statement = %q, want %q", lemma.Statement, lemmaStatement)
	}

	if lemma.SourceNodeID.String() != childID.String() {
		t.Errorf("lemma source node ID = %v, want %v", lemma.SourceNodeID, childID)
	}

	t.Log("Lemma extraction from validated subtree: SUCCESS")
}

// TestLemma_CreatesLemmaExtractedEvent tests that lemma extraction creates
// the correct LemmaExtracted event in the ledger.
func TestLemma_CreatesLemmaExtractedEvent(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create and initialize a simple proof
	conjecture := "Test conjecture for lemma event"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")

	// Validate the root node
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// 2. Extract a lemma
	lemmaStatement := "A validated statement that can be reused"
	lemmaID, err := svc.ExtractLemma(rootID, lemmaStatement)
	if err != nil {
		t.Fatalf("ExtractLemma failed: %v", err)
	}

	// 3. Verify LemmaExtracted event is in the ledger
	lemmas := getLemmaStates(t, proofDir)

	if len(lemmas) != 1 {
		t.Fatalf("expected 1 lemma in ledger, got %d", len(lemmas))
	}

	lemmaState := lemmas[lemmaID]
	if lemmaState == nil {
		t.Fatal("lemma not found in ledger events")
	}

	if lemmaState.Statement != lemmaStatement {
		t.Errorf("lemma statement in ledger = %q, want %q", lemmaState.Statement, lemmaStatement)
	}

	if lemmaState.NodeID.String() != rootID.String() {
		t.Errorf("lemma node ID in ledger = %v, want %v", lemmaState.NodeID, rootID)
	}

	t.Log("LemmaExtracted event verified in ledger: SUCCESS")
}

// TestLemma_MustReferenceExistingNode tests that lemma extraction fails
// if the source node does not exist.
func TestLemma_MustReferenceExistingNode(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create a simple proof
	conjecture := "Test conjecture for invalid node reference"
	svc := initLemmaProof(t, proofDir, conjecture)

	// 2. Try to extract a lemma from a non-existent node
	// Use "1.99" which is a valid node ID format (child of root) but doesn't exist
	nonExistentID := mustParseLemmaNodeID(t, "1.99")
	_, err := svc.ExtractLemma(nonExistentID, "Some statement")

	if err == nil {
		t.Fatal("ExtractLemma should fail for non-existent node")
	}

	t.Logf("ExtractLemma correctly failed for non-existent node: %v", err)
}

// TestLemma_EmptyStatementFails tests that lemma extraction fails
// if the statement is empty.
func TestLemma_EmptyStatementFails(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create a simple proof
	conjecture := "Test conjecture for empty statement"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")

	// Validate the root node
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// 2. Try to extract a lemma with empty statement
	_, err := svc.ExtractLemma(rootID, "")
	if err == nil {
		t.Fatal("ExtractLemma should fail for empty statement")
	}

	// 3. Try with whitespace-only statement
	_, err = svc.ExtractLemma(rootID, "   ")
	if err == nil {
		t.Fatal("ExtractLemma should fail for whitespace-only statement")
	}

	t.Log("ExtractLemma correctly fails for empty/whitespace statements: SUCCESS")
}

// TestLemma_MultipleExtractions tests extracting multiple lemmas from different nodes.
func TestLemma_MultipleExtractions(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create proof with multiple validated nodes
	conjecture := "Multiple lemma test"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")
	child1ID := mustParseLemmaNodeID(t, "1.1")
	child2ID := mustParseLemmaNodeID(t, "1.2")

	proverOwner := "prover"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, child1ID, schema.NodeTypeClaim,
		"First proof branch", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child1) failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, child2ID, schema.NodeTypeClaim,
		"Second proof branch", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode (child2) failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Validate all nodes
	if err := svc.AcceptNode(child1ID); err != nil {
		t.Fatalf("AcceptNode (child1) failed: %v", err)
	}
	if err := svc.AcceptNode(child2ID); err != nil {
		t.Fatalf("AcceptNode (child2) failed: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// 2. Extract multiple lemmas
	lemma1ID, err := svc.ExtractLemma(child1ID, "Lemma from first branch")
	if err != nil {
		t.Fatalf("ExtractLemma (child1) failed: %v", err)
	}

	lemma2ID, err := svc.ExtractLemma(child2ID, "Lemma from second branch")
	if err != nil {
		t.Fatalf("ExtractLemma (child2) failed: %v", err)
	}

	lemma3ID, err := svc.ExtractLemma(rootID, "Lemma from root")
	if err != nil {
		t.Fatalf("ExtractLemma (root) failed: %v", err)
	}

	// 3. Verify all lemmas have unique IDs
	ids := map[string]bool{lemma1ID: true, lemma2ID: true, lemma3ID: true}
	if len(ids) != 3 {
		t.Error("Lemma IDs should be unique")
	}

	// 4. Verify all lemmas exist in state
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	for _, lemmaID := range []string{lemma1ID, lemma2ID, lemma3ID} {
		if state.GetLemma(lemmaID) == nil {
			t.Errorf("lemma %s not found in state", lemmaID)
		}
	}

	// 5. Verify all events in ledger
	lemmas := getLemmaStates(t, proofDir)
	if len(lemmas) != 3 {
		t.Errorf("expected 3 lemmas in ledger, got %d", len(lemmas))
	}

	t.Logf("Multiple lemma extraction: SUCCESS (IDs: %s, %s, %s)", lemma1ID, lemma2ID, lemma3ID)
}

// TestLemma_ExtractFromRootNode tests extracting a lemma from the root node.
func TestLemma_ExtractFromRootNode(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create and validate a simple proof (root only)
	conjecture := "A simple validated claim"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")

	// Validate root node
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// 2. Extract lemma from root
	lemmaID, err := svc.ExtractLemma(rootID, "Root lemma statement")
	if err != nil {
		t.Fatalf("ExtractLemma from root failed: %v", err)
	}

	// 3. Verify lemma references root
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	lemma := state.GetLemma(lemmaID)
	if lemma == nil {
		t.Fatal("Lemma not found in state")
	}

	if lemma.SourceNodeID.String() != rootID.String() {
		t.Errorf("lemma source = %v, want %v", lemma.SourceNodeID, rootID)
	}

	t.Log("Lemma extraction from root node: SUCCESS")
}

// TestLemma_LemmaHasCorrectStructure tests that extracted lemmas have the expected structure.
// Note: ContentHash is computed locally during lemma creation but not persisted to the ledger,
// so we verify other structural properties instead.
func TestLemma_LemmaHasCorrectStructure(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create and validate a proof
	conjecture := "Structure test"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// 2. Extract lemma with a specific statement
	lemmaStatement := "A statement for structure verification"
	lemmaID, err := svc.ExtractLemma(rootID, lemmaStatement)
	if err != nil {
		t.Fatalf("ExtractLemma failed: %v", err)
	}

	// 3. Verify lemma structure in state
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	lemma := state.GetLemma(lemmaID)
	if lemma == nil {
		t.Fatal("Lemma not found")
	}

	// Verify ID is set
	if lemma.ID == "" {
		t.Error("Lemma ID should not be empty")
	}

	// Verify statement is preserved
	if lemma.Statement != lemmaStatement {
		t.Errorf("Lemma statement = %q, want %q", lemma.Statement, lemmaStatement)
	}

	// Verify source node ID is set
	if lemma.SourceNodeID.String() != rootID.String() {
		t.Errorf("Lemma SourceNodeID = %v, want %v", lemma.SourceNodeID, rootID)
	}

	// Verify Created timestamp is set (non-zero)
	if lemma.Created.IsZero() {
		t.Error("Lemma Created timestamp should not be zero")
	}

	t.Logf("Lemma structure verified: ID=%s, Statement=%q, SourceNodeID=%s",
		lemma.ID, lemma.Statement, lemma.SourceNodeID)
}

// TestLemma_SameStatementDifferentIDs tests that extracting lemmas with the same
// statement but from different nodes produces different lemma IDs.
func TestLemma_SameStatementDifferentIDs(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create proof with two nodes
	conjecture := "Same statement test"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")
	childID := mustParseLemmaNodeID(t, "1.1")

	proverOwner := "prover"
	if err := svc.ClaimNode(rootID, proverOwner, 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode failed: %v", err)
	}

	if err := svc.RefineNode(rootID, proverOwner, childID, schema.NodeTypeClaim,
		"Child node", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	if err := svc.ReleaseNode(rootID, proverOwner); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	if err := svc.AcceptNode(childID); err != nil {
		t.Fatalf("AcceptNode (child) failed: %v", err)
	}
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode (root) failed: %v", err)
	}

	// 2. Extract two lemmas with the same statement from different nodes
	sameStatement := "A reusable mathematical fact"

	lemma1ID, err := svc.ExtractLemma(rootID, sameStatement)
	if err != nil {
		t.Fatalf("ExtractLemma (root) failed: %v", err)
	}

	lemma2ID, err := svc.ExtractLemma(childID, sameStatement)
	if err != nil {
		t.Fatalf("ExtractLemma (child) failed: %v", err)
	}

	// 3. Verify IDs are different
	if lemma1ID == lemma2ID {
		t.Error("Lemmas with same statement should have different IDs")
	}

	// 4. Verify both exist
	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	lemma1 := state.GetLemma(lemma1ID)
	lemma2 := state.GetLemma(lemma2ID)

	if lemma1 == nil || lemma2 == nil {
		t.Fatal("Both lemmas should exist in state")
	}

	// 5. Verify both have the same statement
	if lemma1.Statement != lemma2.Statement {
		t.Error("Both lemmas should have the same statement")
	}

	// 6. Verify they reference different nodes
	if lemma1.SourceNodeID.String() == lemma2.SourceNodeID.String() {
		t.Error("Lemmas should reference different source nodes")
	}

	t.Log("Same statement, different lemma IDs: SUCCESS")
}

// TestLemma_LemmaIDFormat tests that lemma IDs have the expected format.
func TestLemma_LemmaIDFormat(t *testing.T) {
	proofDir, cleanup := setupLemmaTest(t)
	defer cleanup()

	// 1. Create and validate a proof
	conjecture := "ID format test"
	svc := initLemmaProof(t, proofDir, conjecture)

	rootID := mustParseLemmaNodeID(t, "1")
	if err := svc.AcceptNode(rootID); err != nil {
		t.Fatalf("AcceptNode failed: %v", err)
	}

	// 2. Extract lemma
	lemmaID, err := svc.ExtractLemma(rootID, "Test statement")
	if err != nil {
		t.Fatalf("ExtractLemma failed: %v", err)
	}

	// 3. Verify ID format starts with "LEM-"
	if len(lemmaID) < 4 || lemmaID[:4] != "LEM-" {
		t.Errorf("Lemma ID should start with 'LEM-', got %q", lemmaID)
	}

	t.Logf("Lemma ID format verified: %s", lemmaID)
}
