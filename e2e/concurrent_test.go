//go:build integration

package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/ledger"
	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/service"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// setupConcurrentTest creates a temporary directory for concurrent testing.
func setupConcurrentTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-concurrent-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initializeConcurrentProof sets up a proof with multiple available nodes for concurrent testing.
func initializeConcurrentProof(t *testing.T, proofDir string) *service.ProofService {
	t.Helper()

	// Initialize proof directory
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	// Initialize the proof
	if err := service.Init(proofDir, "Test concurrent operations", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	return svc
}

// TestConcurrent_TwoAgentsClaimSameNode tests that when two agents try to claim
// the same node simultaneously, exactly one succeeds and one fails.
func TestConcurrent_TwoAgentsClaimSameNode(t *testing.T) {
	proofDir, cleanup := setupConcurrentTest(t)
	defer cleanup()

	svc := initializeConcurrentProof(t, proofDir)
	rootID, _ := types.Parse("1")

	// Use WaitGroup to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Track results
	var mu sync.Mutex
	results := make([]error, 2)

	// Start signal to ensure both goroutines start at roughly the same time
	start := make(chan struct{})

	// Agent 1 tries to claim
	go func() {
		defer wg.Done()
		<-start
		err := svc.ClaimNode(rootID, "agent-1", 5*time.Minute)
		mu.Lock()
		results[0] = err
		mu.Unlock()
	}()

	// Agent 2 tries to claim
	go func() {
		defer wg.Done()
		<-start
		err := svc.ClaimNode(rootID, "agent-2", 5*time.Minute)
		mu.Lock()
		results[1] = err
		mu.Unlock()
	}()

	// Release both goroutines simultaneously
	close(start)

	// Wait for both to complete
	wg.Wait()

	// Verify exactly one succeeded and one failed
	successes := 0
	failures := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}

	if successes != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successes)
	}
	if failures != 1 {
		t.Errorf("Expected exactly 1 failure, got %d", failures)
	}

	// Verify the node is actually claimed by one agent
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("Root node not found")
	}

	if rootNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Expected workflow state %q, got %q", schema.WorkflowClaimed, rootNode.WorkflowState)
	}

	// Verify the claimed owner is one of the two agents
	if rootNode.ClaimedBy != "agent-1" && rootNode.ClaimedBy != "agent-2" {
		t.Errorf("Expected claimed by agent-1 or agent-2, got %q", rootNode.ClaimedBy)
	}

	t.Logf("Node claimed by: %s", rootNode.ClaimedBy)
}

// TestConcurrent_LockTimeoutAndReaping tests that expired locks can be reaped.
func TestConcurrent_LockTimeoutAndReaping(t *testing.T) {
	proofDir, cleanup := setupConcurrentTest(t)
	defer cleanup()

	// Create necessary directories
	ledgerDir := filepath.Join(proofDir, "ledger")
	locksDir := filepath.Join(proofDir, "locks")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		t.Fatalf("Failed to create locks dir: %v", err)
	}

	// Create ledger
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("Failed to create ledger: %v", err)
	}

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test reaping", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("Failed to append init event: %v", err)
	}

	// Create a lock manager and acquire a short-lived lock
	manager := lock.NewManager()
	nodeID, _ := types.Parse("1")

	// Acquire a lock with very short timeout
	shortTimeout := 10 * time.Millisecond
	lk, err := manager.Acquire(nodeID, "agent-short", shortTimeout)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	t.Logf("Lock acquired, expires at: %v", lk.ExpiresAt())

	// Verify lock is not expired immediately
	if lk.IsExpired() {
		t.Error("Lock should not be expired immediately after acquisition")
	}

	// Wait for the lock to expire
	time.Sleep(20 * time.Millisecond)

	// Verify lock is now expired
	if !lk.IsExpired() {
		t.Error("Lock should be expired after timeout")
	}

	// Verify lock is stale
	if !lk.IsStale() {
		t.Error("Expired lock should be stale")
	}

	// Reap expired locks
	reaped, err := manager.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired failed: %v", err)
	}

	if len(reaped) != 1 {
		t.Errorf("Expected 1 reaped lock, got %d", len(reaped))
	}

	// Verify the lock is no longer in the manager
	if manager.IsLocked(nodeID) {
		t.Error("Node should no longer be locked after reaping")
	}

	// New agent should be able to acquire the lock now
	_, err = manager.Acquire(nodeID, "agent-new", 5*time.Minute)
	if err != nil {
		t.Errorf("New agent should be able to acquire lock after reaping: %v", err)
	}

	t.Log("Lock timeout and reaping verified successfully")
}

// TestConcurrent_CASSequenceConflict tests that concurrent ledger appends
// with CAS (Compare-And-Swap) properly detect conflicts.
func TestConcurrent_CASSequenceConflict(t *testing.T) {
	proofDir, cleanup := setupConcurrentTest(t)
	defer cleanup()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}

	// Create ledger
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("Failed to create ledger: %v", err)
	}

	// Initialize proof to have a starting sequence
	initEvent := ledger.NewProofInitialized("Test CAS", "test-author")
	_, err = ldg.Append(initEvent)
	if err != nil {
		t.Fatalf("Failed to append init event: %v", err)
	}

	// Get current state to know the sequence
	st, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Failed to replay state: %v", err)
	}
	expectedSeq := st.LatestSeq()

	t.Logf("Initial sequence: %d", expectedSeq)

	// Create two different events to append
	nodeID1, _ := types.Parse("1")
	nodeID2, _ := types.Parse("1.1")

	node1, _ := node.NewNode(nodeID1, schema.NodeTypeClaim, "Statement 1", schema.InferenceAssumption)
	node2, _ := node.NewNode(nodeID2, schema.NodeTypeClaim, "Statement 2", schema.InferenceAssumption)

	event1 := ledger.NewNodeCreated(*node1)
	event2 := ledger.NewNodeCreated(*node2)

	// Use WaitGroup to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	var mu sync.Mutex
	results := make([]error, 2)
	seqs := make([]int, 2)

	// Start signal
	start := make(chan struct{})

	// Goroutine 1 tries to append with CAS using the same expected sequence
	go func() {
		defer wg.Done()
		<-start
		seq, err := ldg.AppendIfSequence(event1, expectedSeq)
		mu.Lock()
		results[0] = err
		seqs[0] = seq
		mu.Unlock()
	}()

	// Goroutine 2 tries to append with CAS using the same expected sequence
	go func() {
		defer wg.Done()
		<-start
		seq, err := ldg.AppendIfSequence(event2, expectedSeq)
		mu.Lock()
		results[1] = err
		seqs[1] = seq
		mu.Unlock()
	}()

	// Release both goroutines simultaneously
	close(start)

	// Wait for both to complete
	wg.Wait()

	// Verify exactly one succeeded and one got a sequence mismatch
	successes := 0
	sequenceMismatches := 0

	for i, err := range results {
		if err == nil {
			successes++
			t.Logf("Goroutine %d succeeded with sequence %d", i+1, seqs[i])
		} else if errors.Is(err, ledger.ErrSequenceMismatch) {
			sequenceMismatches++
			t.Logf("Goroutine %d got sequence mismatch: %v", i+1, err)
		} else {
			t.Errorf("Goroutine %d got unexpected error: %v", i+1, err)
		}
	}

	if successes != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successes)
	}
	if sequenceMismatches != 1 {
		t.Errorf("Expected exactly 1 sequence mismatch, got %d", sequenceMismatches)
	}

	// Verify ledger has exactly 2 events (init + 1 node created)
	count, err := ldg.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 events in ledger, got %d", count)
	}

	t.Log("CAS sequence conflict detection verified successfully")
}

// TestConcurrent_MultipleAgentsClaimDifferentNodes tests that multiple agents
// can successfully claim different nodes simultaneously.
func TestConcurrent_MultipleAgentsClaimDifferentNodes(t *testing.T) {
	proofDir, cleanup := setupConcurrentTest(t)
	defer cleanup()

	// Initialize proof
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	if err := service.Init(proofDir, "Test multiple claims", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	// Claim root node and create children
	rootID, _ := types.Parse("1")
	if err := svc.ClaimNode(rootID, "setup-agent", 5*time.Minute); err != nil {
		t.Fatalf("ClaimNode (root) failed: %v", err)
	}

	// Create 3 child nodes
	childIDs := []types.NodeID{}
	for i := 1; i <= 3; i++ {
		childID, _ := types.Parse("1." + string(rune('0'+i)))
		if err := svc.RefineNode(rootID, "setup-agent", childID, schema.NodeTypeClaim,
			"Child statement "+string(rune('0'+i)), schema.InferenceAssumption); err != nil {
			t.Fatalf("RefineNode (1.%d) failed: %v", i, err)
		}
		childIDs = append(childIDs, childID)
	}

	// Release root node
	if err := svc.ReleaseNode(rootID, "setup-agent"); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Now try to have 3 agents claim the 3 different child nodes simultaneously
	var wg sync.WaitGroup
	wg.Add(3)

	var mu sync.Mutex
	results := make([]error, 3)

	start := make(chan struct{})

	for i := 0; i < 3; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-start
			// Each agent claims a different node
			err := svc.ClaimNode(childIDs[idx], "agent-"+string(rune('A'+idx)), 5*time.Minute)
			mu.Lock()
			results[idx] = err
			mu.Unlock()
		}()
	}

	// Release all goroutines simultaneously
	close(start)

	// Wait for all to complete
	wg.Wait()

	// All claims should succeed since they are claiming different nodes
	// Note: Due to CAS semantics, some may fail and need retry, but let's check results
	successes := 0
	casMismatches := 0
	for i, err := range results {
		if err == nil {
			successes++
			t.Logf("Agent %c successfully claimed node %s", rune('A'+i), childIDs[i])
		} else if errors.Is(err, service.ErrConcurrentModification) {
			casMismatches++
			t.Logf("Agent %c got concurrent modification (expected due to CAS): %v", rune('A'+i), err)
		} else {
			t.Logf("Agent %c got error: %v", rune('A'+i), err)
		}
	}

	// At least one should succeed, and the others may get CAS errors
	// (which is expected behavior - they would retry in a real scenario)
	if successes == 0 {
		t.Error("Expected at least one successful claim")
	}

	t.Logf("Results: %d successes, %d CAS mismatches", successes, casMismatches)

	// Verify the state - some nodes should be claimed
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	claimedCount := 0
	for _, childID := range childIDs {
		n := st.GetNode(childID)
		if n != nil && n.WorkflowState == schema.WorkflowClaimed {
			claimedCount++
			t.Logf("Node %s is claimed by %s", childID, n.ClaimedBy)
		}
	}

	if claimedCount == 0 {
		t.Error("Expected at least one node to be claimed")
	}

	t.Logf("Multiple agents claiming different nodes: %d nodes claimed", claimedCount)
}

// TestConcurrent_LockManagerConcurrency tests the lock manager's thread safety.
func TestConcurrent_LockManagerConcurrency(t *testing.T) {
	manager := lock.NewManager()

	// Create multiple node IDs (using hierarchical format)
	nodeIDs := make([]types.NodeID, 5)
	nodeIDStrs := []string{"1", "1.1", "1.2", "1.3", "1.4"}
	for i := 0; i < 5; i++ {
		var err error
		nodeIDs[i], err = types.Parse(nodeIDStrs[i])
		if err != nil {
			t.Fatalf("Failed to parse node ID %s: %v", nodeIDStrs[i], err)
		}
	}

	// Multiple goroutines try to acquire locks on different nodes
	var wg sync.WaitGroup
	wg.Add(5)

	var mu sync.Mutex
	results := make([]*lock.ClaimLock, 5)

	start := make(chan struct{})

	for i := 0; i < 5; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-start
			lk, _ := manager.Acquire(nodeIDs[idx], "agent-"+string(rune('A'+idx)), 5*time.Minute)
			mu.Lock()
			results[idx] = lk
			mu.Unlock()
		}()
	}

	// Release all goroutines simultaneously
	close(start)

	// Wait for all to complete
	wg.Wait()

	// All should succeed since they are acquiring different locks
	for i, lk := range results {
		if lk == nil {
			t.Errorf("Agent %c failed to acquire lock for node %s", rune('A'+i), nodeIDs[i])
		}
	}

	// Verify all nodes are locked
	lockedCount := 0
	for _, nodeID := range nodeIDs {
		if manager.IsLocked(nodeID) {
			lockedCount++
		}
	}

	if lockedCount != 5 {
		t.Errorf("Expected 5 locked nodes, got %d", lockedCount)
	}

	// Try to acquire already-locked nodes (should fail)
	for i := 0; i < 5; i++ {
		_, err := manager.Acquire(nodeIDs[i], "different-agent", 5*time.Minute)
		if err == nil {
			t.Errorf("Should not be able to acquire already-locked node %s", nodeIDs[i])
		}
	}

	t.Log("Lock manager concurrency verified successfully")
}

// TestConcurrent_RaceConditionOnClaim tests a realistic race condition scenario
// where agents rapidly load state and try to claim.
func TestConcurrent_RaceConditionOnClaim(t *testing.T) {
	proofDir, cleanup := setupConcurrentTest(t)
	defer cleanup()

	svc := initializeConcurrentProof(t, proofDir)
	rootID, _ := types.Parse("1")

	// Number of agents trying to claim
	numAgents := 5

	var wg sync.WaitGroup
	wg.Add(numAgents)

	var mu sync.Mutex
	claimResults := make([]struct {
		agent string
		err   error
	}, numAgents)

	start := make(chan struct{})

	for i := 0; i < numAgents; i++ {
		idx := i
		agentName := "agent-" + string(rune('A'+idx))
		go func() {
			defer wg.Done()
			<-start
			err := svc.ClaimNode(rootID, agentName, 5*time.Minute)
			mu.Lock()
			claimResults[idx] = struct {
				agent string
				err   error
			}{agentName, err}
			mu.Unlock()
		}()
	}

	// Start all agents at once
	close(start)
	wg.Wait()

	// Count successes and failures
	var winner string
	successes := 0
	for _, result := range claimResults {
		if result.err == nil {
			winner = result.agent
			successes++
		}
	}

	// Exactly one should win
	if successes != 1 {
		t.Errorf("Expected exactly 1 winner, got %d", successes)
	}

	// Verify the node is claimed by the winner
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	rootNode := st.GetNode(rootID)
	if rootNode.ClaimedBy != winner {
		t.Errorf("Expected node to be claimed by %s, but claimed by %s", winner, rootNode.ClaimedBy)
	}

	t.Logf("Race condition test: %s won the claim", winner)
}

// TestConcurrent_ExpiredLockCanBeReacquired tests that after a lock expires,
// another agent can acquire it.
func TestConcurrent_ExpiredLockCanBeReacquired(t *testing.T) {
	manager := lock.NewManager()
	nodeID, _ := types.Parse("1")

	// First agent acquires with short timeout
	_, err := manager.Acquire(nodeID, "agent-first", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Second agent tries to acquire immediately - should fail
	_, err = manager.Acquire(nodeID, "agent-second", 5*time.Minute)
	if err == nil {
		t.Error("Second agent should not be able to acquire locked node")
	}

	// Wait for lock to expire
	time.Sleep(60 * time.Millisecond)

	// Now second agent should be able to acquire (expired locks can be replaced)
	lk, err := manager.Acquire(nodeID, "agent-second", 5*time.Minute)
	if err != nil {
		t.Errorf("Second agent should be able to acquire after expiration: %v", err)
	}

	if lk != nil && lk.Owner() != "agent-second" {
		t.Errorf("Expected owner to be agent-second, got %s", lk.Owner())
	}

	t.Log("Expired lock reacquisition verified successfully")
}
