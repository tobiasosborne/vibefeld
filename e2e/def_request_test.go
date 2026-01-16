//go:build integration

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// setupDefRequestTest creates a temporary directory for testing definition requests.
// Returns the proof directory path and a cleanup function.
func setupDefRequestTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-def-request-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initializeProofWithNodeForDefRequest creates a proof directory with ledger and a root node.
// Also initializes the .af directory structure for pending definitions.
func initializeProofWithNodeForDefRequest(t *testing.T, proofDir string) (*ledger.Ledger, types.NodeID) {
	t.Helper()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("failed to create ledger directory: %v", err)
	}

	// Create .af directory for pending definitions
	if err := os.MkdirAll(filepath.Join(proofDir, ".af", "pending_defs"), 0755); err != nil {
		t.Fatalf("failed to create .af/pending_defs directory: %v", err)
	}

	// Create ledger instance
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test conjecture requiring definitions", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("failed to append proof initialized event: %v", err)
	}

	// Create root node
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	rootNode, err := node.NewNode(nodeID, schema.NodeTypeClaim, "Test statement", schema.InferenceAssumption)
	if err != nil {
		t.Fatalf("failed to create root node: %v", err)
	}

	nodeEvent := ledger.NewNodeCreated(*rootNode)
	if _, err := ldg.Append(nodeEvent); err != nil {
		t.Fatalf("failed to append node created event: %v", err)
	}

	return ldg, nodeID
}

// TestDefRequest_RaiseAndSatisfy tests the basic workflow:
// 1. Raising a definition request on a node (blocks node)
// 2. Satisfying the definition request with a definition (unblocks node)
func TestDefRequest_RaiseAndSatisfy(t *testing.T) {
	proofDir, cleanup := setupDefRequestTest(t)
	defer cleanup()

	// Setup proof with a node
	ldg, nodeID := initializeProofWithNodeForDefRequest(t, proofDir)

	// ==========================================================================
	// Step 1: Raise a definition request (this blocks the node conceptually)
	// ==========================================================================
	t.Log("Step 1: Raise a definition request")

	// Create a pending definition request for the term "group"
	pd, err := node.NewPendingDef("group", nodeID)
	if err != nil {
		t.Fatalf("NewPendingDef failed: %v", err)
	}

	// Verify the pending def is in pending state
	if !pd.IsPending() {
		t.Fatal("newly created PendingDef should be in pending state")
	}
	if pd.Term != "group" {
		t.Errorf("PendingDef.Term = %q, want %q", pd.Term, "group")
	}
	if pd.RequestedBy.String() != nodeID.String() {
		t.Errorf("PendingDef.RequestedBy = %q, want %q", pd.RequestedBy.String(), nodeID.String())
	}

	// Persist the pending definition to filesystem
	err = fs.WritePendingDef(proofDir, nodeID, pd)
	if err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Verify it can be read back
	readPd, err := fs.ReadPendingDef(proofDir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}
	if readPd.ID != pd.ID {
		t.Errorf("ReadPendingDef.ID = %q, want %q", readPd.ID, pd.ID)
	}
	if !readPd.IsPending() {
		t.Error("read PendingDef should still be in pending state")
	}

	t.Logf("  Definition request raised for term %q by node %s", pd.Term, nodeID)

	// ==========================================================================
	// Step 2: Satisfy the definition request
	// ==========================================================================
	t.Log("Step 2: Satisfy the definition request")

	// Create the actual definition
	def, err := node.NewDefinition("group", "A group is a set G with a binary operation * satisfying closure, associativity, identity, and inverse properties.")
	if err != nil {
		t.Fatalf("NewDefinition failed: %v", err)
	}

	// Record the definition in the ledger
	defEvent := ledger.NewDefAdded(ledger.Definition{
		ID:         def.ID,
		Name:       def.Name,
		Definition: def.Content,
		Created:    def.Created,
	})
	if _, err := ldg.Append(defEvent); err != nil {
		t.Fatalf("failed to append def added event: %v", err)
	}

	// Resolve the pending definition
	err = pd.Resolve(def.ID)
	if err != nil {
		t.Fatalf("PendingDef.Resolve failed: %v", err)
	}

	// Verify the pending def is now resolved
	if pd.IsPending() {
		t.Error("PendingDef should no longer be pending after Resolve")
	}
	if pd.Status != node.PendingDefStatusResolved {
		t.Errorf("PendingDef.Status = %q, want %q", pd.Status, node.PendingDefStatusResolved)
	}
	if pd.ResolvedBy != def.ID {
		t.Errorf("PendingDef.ResolvedBy = %q, want %q", pd.ResolvedBy, def.ID)
	}

	// Update the persisted pending definition
	err = fs.WritePendingDef(proofDir, nodeID, pd)
	if err != nil {
		t.Fatalf("WritePendingDef (update) failed: %v", err)
	}

	// Verify the resolved state persists
	readPd, err = fs.ReadPendingDef(proofDir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef (after resolve) failed: %v", err)
	}
	if readPd.Status != node.PendingDefStatusResolved {
		t.Errorf("persisted PendingDef.Status = %q, want %q", readPd.Status, node.PendingDefStatusResolved)
	}

	t.Logf("  Definition request satisfied with definition %q", def.ID)
	t.Log("")
	t.Log("  Definition request workflow completed successfully")
}

// TestDefRequest_MultipleRequestsOnSameNode tests multiple definition requests
// on the same node.
func TestDefRequest_MultipleRequestsOnSameNode(t *testing.T) {
	proofDir, cleanup := setupDefRequestTest(t)
	defer cleanup()

	// Setup proof with a node (root node ID not used directly in this test)
	ldg, _ := initializeProofWithNodeForDefRequest(t, proofDir)

	// ==========================================================================
	// Step 1: Create multiple pending definitions for different child nodes
	// ==========================================================================
	t.Log("Step 1: Create multiple pending definitions for different child nodes")

	// Note: In the current implementation, each node can have one pending def file.
	// Multiple definitions are tracked by creating pending defs for different nodes.

	// Create child nodes
	child1ID, _ := types.Parse("1.1")
	child2ID, _ := types.Parse("1.2")
	child3ID, _ := types.Parse("1.3")

	child1, _ := node.NewNode(child1ID, schema.NodeTypeClaim, "Statement using group", schema.InferenceModusPonens)
	child2, _ := node.NewNode(child2ID, schema.NodeTypeClaim, "Statement using homomorphism", schema.InferenceModusPonens)
	child3, _ := node.NewNode(child3ID, schema.NodeTypeClaim, "Statement using kernel", schema.InferenceModusPonens)

	// Add child nodes to ledger
	for _, n := range []*node.Node{child1, child2, child3} {
		nodeEvent := ledger.NewNodeCreated(*n)
		if _, err := ldg.Append(nodeEvent); err != nil {
			t.Fatalf("failed to append node created event for %s: %v", n.ID, err)
		}
	}

	// Create pending definitions for each child node
	pd1, err := node.NewPendingDef("group", child1ID)
	if err != nil {
		t.Fatalf("NewPendingDef (pd1) failed: %v", err)
	}
	pd2, err := node.NewPendingDef("homomorphism", child2ID)
	if err != nil {
		t.Fatalf("NewPendingDef (pd2) failed: %v", err)
	}
	pd3, err := node.NewPendingDef("kernel", child3ID)
	if err != nil {
		t.Fatalf("NewPendingDef (pd3) failed: %v", err)
	}

	// Write all pending definitions
	for _, pd := range []*node.PendingDef{pd1, pd2, pd3} {
		if err := fs.WritePendingDef(proofDir, pd.RequestedBy, pd); err != nil {
			t.Fatalf("WritePendingDef failed for %s: %v", pd.Term, err)
		}
	}

	// Verify all pending definitions are listed
	nodeIDs, err := fs.ListPendingDefs(proofDir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}
	if len(nodeIDs) != 3 {
		t.Errorf("ListPendingDefs returned %d items, want 3", len(nodeIDs))
	}

	t.Logf("  Created 3 pending definition requests")

	// ==========================================================================
	// Step 2: Satisfy definitions one by one
	// ==========================================================================
	t.Log("Step 2: Satisfy definitions one by one")

	// Satisfy first definition
	def1, _ := node.NewDefinition("group", "A set with binary operation satisfying group axioms")
	defEvent1 := ledger.NewDefAdded(ledger.Definition{
		ID: def1.ID, Name: def1.Name, Definition: def1.Content, Created: def1.Created,
	})
	if _, err := ldg.Append(defEvent1); err != nil {
		t.Fatalf("failed to append def added event: %v", err)
	}
	if err := pd1.Resolve(def1.ID); err != nil {
		t.Fatalf("pd1.Resolve failed: %v", err)
	}
	if err := fs.WritePendingDef(proofDir, pd1.RequestedBy, pd1); err != nil {
		t.Fatalf("WritePendingDef (pd1 resolve) failed: %v", err)
	}

	// Count pending vs resolved
	pending := 0
	resolved := 0
	for _, nid := range []types.NodeID{child1ID, child2ID, child3ID} {
		pd, err := fs.ReadPendingDef(proofDir, nid)
		if err != nil {
			t.Fatalf("ReadPendingDef failed for %s: %v", nid, err)
		}
		if pd.IsPending() {
			pending++
		} else {
			resolved++
		}
	}
	if pending != 2 || resolved != 1 {
		t.Errorf("After first resolve: pending=%d, resolved=%d; want pending=2, resolved=1", pending, resolved)
	}

	t.Log("  After first definition: 2 pending, 1 resolved")

	// Satisfy remaining definitions
	def2, _ := node.NewDefinition("homomorphism", "A structure-preserving map between algebraic structures")
	defEvent2 := ledger.NewDefAdded(ledger.Definition{
		ID: def2.ID, Name: def2.Name, Definition: def2.Content, Created: def2.Created,
	})
	if _, err := ldg.Append(defEvent2); err != nil {
		t.Fatalf("failed to append def added event: %v", err)
	}
	if err := pd2.Resolve(def2.ID); err != nil {
		t.Fatalf("pd2.Resolve failed: %v", err)
	}
	if err := fs.WritePendingDef(proofDir, pd2.RequestedBy, pd2); err != nil {
		t.Fatalf("WritePendingDef (pd2 resolve) failed: %v", err)
	}

	def3, _ := node.NewDefinition("kernel", "The preimage of the identity element under a homomorphism")
	defEvent3 := ledger.NewDefAdded(ledger.Definition{
		ID: def3.ID, Name: def3.Name, Definition: def3.Content, Created: def3.Created,
	})
	if _, err := ldg.Append(defEvent3); err != nil {
		t.Fatalf("failed to append def added event: %v", err)
	}
	if err := pd3.Resolve(def3.ID); err != nil {
		t.Fatalf("pd3.Resolve failed: %v", err)
	}
	if err := fs.WritePendingDef(proofDir, pd3.RequestedBy, pd3); err != nil {
		t.Fatalf("WritePendingDef (pd3 resolve) failed: %v", err)
	}

	// Verify all are resolved
	allResolved := true
	for _, nid := range []types.NodeID{child1ID, child2ID, child3ID} {
		pd, err := fs.ReadPendingDef(proofDir, nid)
		if err != nil {
			t.Fatalf("ReadPendingDef failed for %s: %v", nid, err)
		}
		if pd.IsPending() {
			allResolved = false
		}
	}
	if !allResolved {
		t.Error("Expected all pending definitions to be resolved")
	}

	t.Log("  All 3 definition requests satisfied")
	t.Log("")
	t.Log("  Multiple definition requests workflow completed successfully")
}

// TestDefRequest_StateTransitions tests definition request state transitions
// through ledger events.
func TestDefRequest_StateTransitions(t *testing.T) {
	proofDir, cleanup := setupDefRequestTest(t)
	defer cleanup()

	// Setup proof with a node
	ldg, nodeID := initializeProofWithNodeForDefRequest(t, proofDir)

	// ==========================================================================
	// Test pending -> resolved transition
	// ==========================================================================
	t.Log("Test 1: pending -> resolved transition")

	pd, err := node.NewPendingDef("group", nodeID)
	if err != nil {
		t.Fatalf("NewPendingDef failed: %v", err)
	}

	// Verify initial state
	if pd.Status != node.PendingDefStatusPending {
		t.Errorf("initial status = %q, want %q", pd.Status, node.PendingDefStatusPending)
	}

	// Persist
	if err := fs.WritePendingDef(proofDir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Create and add definition to ledger
	def, _ := node.NewDefinition("group", "A group definition")
	defEvent := ledger.NewDefAdded(ledger.Definition{
		ID: def.ID, Name: def.Name, Definition: def.Content, Created: def.Created,
	})
	if _, err := ldg.Append(defEvent); err != nil {
		t.Fatalf("failed to append def added event: %v", err)
	}

	// Resolve the pending definition
	if err := pd.Resolve(def.ID); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Verify transition
	if pd.Status != node.PendingDefStatusResolved {
		t.Errorf("after resolve, status = %q, want %q", pd.Status, node.PendingDefStatusResolved)
	}

	// Persist resolved state
	if err := fs.WritePendingDef(proofDir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef (resolved) failed: %v", err)
	}

	// Verify persisted state
	readPd, _ := fs.ReadPendingDef(proofDir, nodeID)
	if readPd.Status != node.PendingDefStatusResolved {
		t.Errorf("persisted status = %q, want %q", readPd.Status, node.PendingDefStatusResolved)
	}

	t.Log("  pending -> resolved transition verified")

	// ==========================================================================
	// Test pending -> cancelled transition
	// ==========================================================================
	t.Log("Test 2: pending -> cancelled transition")

	// Create new node for this test
	cancelNodeID, _ := types.Parse("1.1")
	cancelNode, _ := node.NewNode(cancelNodeID, schema.NodeTypeClaim, "Cancelled node", schema.InferenceModusPonens)
	nodeEvent := ledger.NewNodeCreated(*cancelNode)
	if _, err := ldg.Append(nodeEvent); err != nil {
		t.Fatalf("failed to append node created event: %v", err)
	}

	pd2, err := node.NewPendingDef("ring", cancelNodeID)
	if err != nil {
		t.Fatalf("NewPendingDef (pd2) failed: %v", err)
	}

	// Verify initial state
	if pd2.Status != node.PendingDefStatusPending {
		t.Errorf("initial status = %q, want %q", pd2.Status, node.PendingDefStatusPending)
	}

	// Persist
	if err := fs.WritePendingDef(proofDir, cancelNodeID, pd2); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Cancel the pending definition
	if err := pd2.Cancel(); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// Verify transition
	if pd2.Status != node.PendingDefStatusCancelled {
		t.Errorf("after cancel, status = %q, want %q", pd2.Status, node.PendingDefStatusCancelled)
	}

	// Persist cancelled state
	if err := fs.WritePendingDef(proofDir, cancelNodeID, pd2); err != nil {
		t.Fatalf("WritePendingDef (cancelled) failed: %v", err)
	}

	// Verify persisted state
	readPd2, _ := fs.ReadPendingDef(proofDir, cancelNodeID)
	if readPd2.Status != node.PendingDefStatusCancelled {
		t.Errorf("persisted status = %q, want %q", readPd2.Status, node.PendingDefStatusCancelled)
	}

	t.Log("  pending -> cancelled transition verified")

	// ==========================================================================
	// Test invalid transitions
	// ==========================================================================
	t.Log("Test 3: invalid state transitions")

	// Try to resolve an already resolved pending def
	err = pd.Resolve("another-def-id")
	if err == nil {
		t.Error("expected error when resolving already resolved pending def")
	}

	// Try to cancel an already resolved pending def
	err = pd.Cancel()
	if err == nil {
		t.Error("expected error when cancelling already resolved pending def")
	}

	// Try to resolve an already cancelled pending def
	err = pd2.Resolve("some-def-id")
	if err == nil {
		t.Error("expected error when resolving already cancelled pending def")
	}

	// Try to cancel an already cancelled pending def
	err = pd2.Cancel()
	if err == nil {
		t.Error("expected error when cancelling already cancelled pending def")
	}

	t.Log("  Invalid transitions correctly rejected")
	t.Log("")
	t.Log("  State transition tests completed successfully")
}

// TestDefRequest_DeletePendingDef tests that pending definitions can be deleted.
func TestDefRequest_DeletePendingDef(t *testing.T) {
	proofDir, cleanup := setupDefRequestTest(t)
	defer cleanup()

	// Setup proof with a node
	_, nodeID := initializeProofWithNodeForDefRequest(t, proofDir)

	// Create and persist a pending definition
	pd, err := node.NewPendingDef("field", nodeID)
	if err != nil {
		t.Fatalf("NewPendingDef failed: %v", err)
	}
	if err := fs.WritePendingDef(proofDir, nodeID, pd); err != nil {
		t.Fatalf("WritePendingDef failed: %v", err)
	}

	// Verify it exists
	_, err = fs.ReadPendingDef(proofDir, nodeID)
	if err != nil {
		t.Fatalf("ReadPendingDef failed: %v", err)
	}

	// List should show 1 pending def
	nodeIDs, err := fs.ListPendingDefs(proofDir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("ListPendingDefs returned %d items, want 1", len(nodeIDs))
	}

	// Delete the pending definition
	if err := fs.DeletePendingDef(proofDir, nodeID); err != nil {
		t.Fatalf("DeletePendingDef failed: %v", err)
	}

	// Verify it no longer exists
	_, err = fs.ReadPendingDef(proofDir, nodeID)
	if err == nil {
		t.Error("expected error reading deleted pending def")
	}

	// List should be empty
	nodeIDs, err = fs.ListPendingDefs(proofDir)
	if err != nil {
		t.Fatalf("ListPendingDefs failed: %v", err)
	}
	if len(nodeIDs) != 0 {
		t.Errorf("ListPendingDefs returned %d items, want 0", len(nodeIDs))
	}

	// Delete should be idempotent
	if err := fs.DeletePendingDef(proofDir, nodeID); err != nil {
		t.Errorf("DeletePendingDef (second call) should be idempotent, got error: %v", err)
	}

	t.Log("DeletePendingDef test completed successfully")
}

// TestDefRequest_LedgerDefAddedEvents tests that DefAdded events are properly
// recorded in the ledger during definition request satisfaction.
func TestDefRequest_LedgerDefAddedEvents(t *testing.T) {
	proofDir, cleanup := setupDefRequestTest(t)
	defer cleanup()

	// Setup proof
	ldg, nodeID := initializeProofWithNodeForDefRequest(t, proofDir)

	// Create multiple definitions
	definitions := []struct {
		name    string
		content string
	}{
		{"vector space", "A set V with field F and operations satisfying vector space axioms"},
		{"linear map", "A function between vector spaces preserving addition and scalar multiplication"},
		{"basis", "A linearly independent set that spans the vector space"},
	}

	defIDs := make([]string, 0, len(definitions))

	for _, d := range definitions {
		def, err := node.NewDefinition(d.name, d.content)
		if err != nil {
			t.Fatalf("NewDefinition(%q) failed: %v", d.name, err)
		}
		defIDs = append(defIDs, def.ID)

		defEvent := ledger.NewDefAdded(ledger.Definition{
			ID:         def.ID,
			Name:       def.Name,
			Definition: def.Content,
			Created:    def.Created,
		})
		if _, err := ldg.Append(defEvent); err != nil {
			t.Fatalf("failed to append def added event for %q: %v", d.name, err)
		}
	}

	// Read all events and count DefAdded events
	defAddedCount := 0
	defNames := make([]string, 0)

	err := ldg.Scan(func(seq int, data []byte) error {
		// Quick check for event type
		var base struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return err
		}

		if base.Type == ledger.EventDefAdded {
			defAddedCount++
			var e ledger.DefAdded
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			defNames = append(defNames, e.Definition.Name)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify we have the expected number of DefAdded events
	if defAddedCount != len(definitions) {
		t.Errorf("DefAdded event count = %d, want %d", defAddedCount, len(definitions))
	}

	// Verify all definition names are present
	for _, d := range definitions {
		found := false
		for _, name := range defNames {
			if name == d.name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefAdded event for %q not found in ledger", d.name)
		}
	}

	// Create pending defs and resolve them using the definitions
	pd, err := node.NewPendingDef("vector space", nodeID)
	if err != nil {
		t.Fatalf("NewPendingDef failed: %v", err)
	}
	if err := pd.Resolve(defIDs[0]); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	t.Logf("Verified %d DefAdded events in ledger", defAddedCount)
	t.Log("Ledger DefAdded events test completed successfully")
}
