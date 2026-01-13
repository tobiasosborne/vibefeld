//go:build integration

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// setupChallengeTest creates a temporary directory and returns it along with a cleanup function.
func setupChallengeTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-challenge-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initializeProofWithNode creates a proof directory with ledger and a root node.
func initializeProofWithNode(t *testing.T, proofDir string) (*ledger.Ledger, types.NodeID) {
	t.Helper()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("failed to create ledger directory: %v", err)
	}

	// Create ledger instance
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test conjecture", "test-author")
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

// ChallengeState tracks the state of challenges from ledger events.
type ChallengeState struct {
	ID       string
	NodeID   types.NodeID
	Target   string
	Reason   string
	Status   string // "open", "resolved", "withdrawn"
}

// getChallengeStates replays the ledger and returns the current state of all challenges.
func getChallengeStates(t *testing.T, ldg *ledger.Ledger) map[string]*ChallengeState {
	t.Helper()
	challenges := make(map[string]*ChallengeState)

	err := ldg.Scan(func(seq int, data []byte) error {
		var base struct {
			Type ledger.EventType `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			return err
		}

		switch base.Type {
		case ledger.EventChallengeRaised:
			var e ledger.ChallengeRaised
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			challenges[e.ChallengeID] = &ChallengeState{
				ID:     e.ChallengeID,
				NodeID: e.NodeID,
				Target: e.Target,
				Reason: e.Reason,
				Status: "open",
			}
		case ledger.EventChallengeResolved:
			var e ledger.ChallengeResolved
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			if c, ok := challenges[e.ChallengeID]; ok {
				c.Status = "resolved"
			}
		case ledger.EventChallengeWithdrawn:
			var e ledger.ChallengeWithdrawn
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			if c, ok := challenges[e.ChallengeID]; ok {
				c.Status = "withdrawn"
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to scan ledger: %v", err)
	}

	return challenges
}

func TestChallengeCycle_RaiseAndResolve(t *testing.T) {
	proofDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// 1. Setup proof with a node
	ldg, nodeID := initializeProofWithNode(t, proofDir)

	// 2. Verifier raises challenge with reason
	challengeID := "challenge-001"
	target := "statement"
	reason := "The statement requires additional justification"

	raiseEvent := ledger.NewChallengeRaised(challengeID, nodeID, target, reason)
	if _, err := ldg.Append(raiseEvent); err != nil {
		t.Fatalf("failed to raise challenge: %v", err)
	}

	// Verify challenge is open
	challenges := getChallengeStates(t, ldg)
	if len(challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(challenges))
	}

	challenge := challenges[challengeID]
	if challenge == nil {
		t.Fatal("challenge not found")
	}
	if challenge.Status != "open" {
		t.Errorf("expected challenge status 'open', got %q", challenge.Status)
	}
	if challenge.NodeID.String() != nodeID.String() {
		t.Errorf("expected challenge node ID %v, got %v", nodeID, challenge.NodeID)
	}
	if challenge.Reason != reason {
		t.Errorf("expected challenge reason %q, got %q", reason, challenge.Reason)
	}

	// 3. Prover resolves challenge with explanation
	resolveEvent := ledger.NewChallengeResolved(challengeID)
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("failed to resolve challenge: %v", err)
	}

	// 4. Verify challenge is resolved in state
	challenges = getChallengeStates(t, ldg)
	challenge = challenges[challengeID]
	if challenge == nil {
		t.Fatal("challenge not found after resolve")
	}
	if challenge.Status != "resolved" {
		t.Errorf("expected challenge status 'resolved', got %q", challenge.Status)
	}
}

func TestChallengeCycle_WithdrawChallenge(t *testing.T) {
	proofDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// 1. Setup proof with a node
	ldg, nodeID := initializeProofWithNode(t, proofDir)

	// 2. Verifier raises challenge
	challengeID := "challenge-withdraw-001"
	raiseEvent := ledger.NewChallengeRaised(challengeID, nodeID, "inference", "Inference rule seems incorrect")
	if _, err := ldg.Append(raiseEvent); err != nil {
		t.Fatalf("failed to raise challenge: %v", err)
	}

	// Verify challenge is open
	challenges := getChallengeStates(t, ldg)
	if challenges[challengeID].Status != "open" {
		t.Errorf("expected challenge status 'open', got %q", challenges[challengeID].Status)
	}

	// 3. Verifier withdraws challenge
	withdrawEvent := ledger.NewChallengeWithdrawn(challengeID)
	if _, err := ldg.Append(withdrawEvent); err != nil {
		t.Fatalf("failed to withdraw challenge: %v", err)
	}

	// 4. Verify challenge is withdrawn
	challenges = getChallengeStates(t, ldg)
	if challenges[challengeID].Status != "withdrawn" {
		t.Errorf("expected challenge status 'withdrawn', got %q", challenges[challengeID].Status)
	}
}

func TestChallengeCycle_MultipleChallenges(t *testing.T) {
	proofDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// 1. Setup proof
	ldg, nodeID := initializeProofWithNode(t, proofDir)

	// 2. Raise multiple challenges on same node
	challengeIDs := []string{"multi-challenge-001", "multi-challenge-002", "multi-challenge-003"}
	targets := []string{"statement", "inference", "gap"}
	reasons := []string{
		"Statement is unclear",
		"Inference rule does not apply here",
		"Logical gap between premise and conclusion",
	}

	for i, challengeID := range challengeIDs {
		raiseEvent := ledger.NewChallengeRaised(challengeID, nodeID, targets[i], reasons[i])
		if _, err := ldg.Append(raiseEvent); err != nil {
			t.Fatalf("failed to raise challenge %s: %v", challengeID, err)
		}
	}

	// Verify all challenges are open
	challenges := getChallengeStates(t, ldg)
	if len(challenges) != 3 {
		t.Fatalf("expected 3 challenges, got %d", len(challenges))
	}

	for _, challengeID := range challengeIDs {
		challenge := challenges[challengeID]
		if challenge == nil {
			t.Errorf("challenge %s not found", challengeID)
			continue
		}
		if challenge.Status != "open" {
			t.Errorf("challenge %s: expected status 'open', got %q", challengeID, challenge.Status)
		}
	}

	// 3. Resolve each challenge
	for _, challengeID := range challengeIDs {
		resolveEvent := ledger.NewChallengeResolved(challengeID)
		if _, err := ldg.Append(resolveEvent); err != nil {
			t.Fatalf("failed to resolve challenge %s: %v", challengeID, err)
		}
	}

	// 4. Verify all resolved
	challenges = getChallengeStates(t, ldg)
	for _, challengeID := range challengeIDs {
		challenge := challenges[challengeID]
		if challenge == nil {
			t.Errorf("challenge %s not found after resolve", challengeID)
			continue
		}
		if challenge.Status != "resolved" {
			t.Errorf("challenge %s: expected status 'resolved', got %q", challengeID, challenge.Status)
		}
	}
}

func TestChallengeCycle_MixedResolutions(t *testing.T) {
	proofDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// Setup proof
	ldg, nodeID := initializeProofWithNode(t, proofDir)

	// Raise multiple challenges
	raiseEvent1 := ledger.NewChallengeRaised("mixed-001", nodeID, "statement", "Reason 1")
	if _, err := ldg.Append(raiseEvent1); err != nil {
		t.Fatalf("failed to raise challenge: %v", err)
	}

	raiseEvent2 := ledger.NewChallengeRaised("mixed-002", nodeID, "inference", "Reason 2")
	if _, err := ldg.Append(raiseEvent2); err != nil {
		t.Fatalf("failed to raise challenge: %v", err)
	}

	raiseEvent3 := ledger.NewChallengeRaised("mixed-003", nodeID, "gap", "Reason 3")
	if _, err := ldg.Append(raiseEvent3); err != nil {
		t.Fatalf("failed to raise challenge: %v", err)
	}

	// Resolve one, withdraw one, leave one open
	resolveEvent := ledger.NewChallengeResolved("mixed-001")
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("failed to resolve challenge: %v", err)
	}

	withdrawEvent := ledger.NewChallengeWithdrawn("mixed-002")
	if _, err := ldg.Append(withdrawEvent); err != nil {
		t.Fatalf("failed to withdraw challenge: %v", err)
	}

	// Verify final states
	challenges := getChallengeStates(t, ldg)

	if challenges["mixed-001"].Status != "resolved" {
		t.Errorf("mixed-001: expected 'resolved', got %q", challenges["mixed-001"].Status)
	}
	if challenges["mixed-002"].Status != "withdrawn" {
		t.Errorf("mixed-002: expected 'withdrawn', got %q", challenges["mixed-002"].Status)
	}
	if challenges["mixed-003"].Status != "open" {
		t.Errorf("mixed-003: expected 'open', got %q", challenges["mixed-003"].Status)
	}
}

func TestChallengeCycle_ChallengeOnDifferentNodes(t *testing.T) {
	proofDir, cleanup := setupChallengeTest(t)
	defer cleanup()

	// Setup proof with root node
	ldg, rootNodeID := initializeProofWithNode(t, proofDir)

	// Create a second node
	childNodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("failed to parse child node ID: %v", err)
	}

	childNode, err := node.NewNode(childNodeID, schema.NodeTypeClaim, "Child statement", schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("failed to create child node: %v", err)
	}

	nodeEvent := ledger.NewNodeCreated(*childNode)
	if _, err := ldg.Append(nodeEvent); err != nil {
		t.Fatalf("failed to append child node created event: %v", err)
	}

	// Raise challenge on root node
	raiseEvent1 := ledger.NewChallengeRaised("node-challenge-001", rootNodeID, "statement", "Root node challenge")
	if _, err := ldg.Append(raiseEvent1); err != nil {
		t.Fatalf("failed to raise challenge on root: %v", err)
	}

	// Raise challenge on child node
	raiseEvent2 := ledger.NewChallengeRaised("node-challenge-002", childNodeID, "inference", "Child node challenge")
	if _, err := ldg.Append(raiseEvent2); err != nil {
		t.Fatalf("failed to raise challenge on child: %v", err)
	}

	// Verify challenges are associated with correct nodes
	challenges := getChallengeStates(t, ldg)

	if challenges["node-challenge-001"].NodeID.String() != rootNodeID.String() {
		t.Errorf("node-challenge-001: expected node ID %v, got %v", rootNodeID, challenges["node-challenge-001"].NodeID)
	}
	if challenges["node-challenge-002"].NodeID.String() != childNodeID.String() {
		t.Errorf("node-challenge-002: expected node ID %v, got %v", childNodeID, challenges["node-challenge-002"].NodeID)
	}

	// Resolve challenge on root node
	resolveEvent := ledger.NewChallengeResolved("node-challenge-001")
	if _, err := ldg.Append(resolveEvent); err != nil {
		t.Fatalf("failed to resolve root challenge: %v", err)
	}

	// Verify state after partial resolution
	challenges = getChallengeStates(t, ldg)
	if challenges["node-challenge-001"].Status != "resolved" {
		t.Errorf("node-challenge-001: expected 'resolved', got %q", challenges["node-challenge-001"].Status)
	}
	if challenges["node-challenge-002"].Status != "open" {
		t.Errorf("node-challenge-002: expected 'open', got %q", challenges["node-challenge-002"].Status)
	}
}
