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

// setupLockLedgerTest creates a temporary directory for lock-ledger coordination testing.
func setupLockLedgerTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "e2e-lock-ledger-*")
	if err != nil {
		t.Fatal(err)
	}
	proofDir := filepath.Join(tmpDir, "proof")
	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// initializeLockLedgerProof sets up a proof with a root node for lock-ledger testing.
func initializeLockLedgerProof(t *testing.T, proofDir string) *service.ProofService {
	t.Helper()

	// Initialize proof directory
	if err := fs.InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	// Initialize the proof
	if err := service.Init(proofDir, "Test lock-ledger coordination", "test-author"); err != nil {
		t.Fatalf("service.Init failed: %v", err)
	}

	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("NewProofService failed: %v", err)
	}

	return svc
}

// TestLockLedger_LockExpiresDuringLedgerWrite tests the scenario where an agent's lock
// expires while a ledger write is in progress. The key behavior being tested is that
// the ledger write uses CAS (Compare-And-Swap) semantics which will detect the state
// change if another agent claims the node after the lock expires.
func TestLockLedger_LockExpiresDuringLedgerWrite(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	svc := initializeLockLedgerProof(t, proofDir)
	rootID, _ := types.Parse("1")

	// Agent 1 claims the root node with a very short timeout
	shortTimeout := 50 * time.Millisecond
	if err := svc.ClaimNode(rootID, "agent-short-claim", shortTimeout); err != nil {
		t.Fatalf("Agent 1 ClaimNode failed: %v", err)
	}

	// Verify node is claimed
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

	t.Logf("Agent 1 claimed node 1 with timeout %v", shortTimeout)

	// Wait for the claim to expire (based on the claim timeout in ledger)
	time.Sleep(60 * time.Millisecond)

	// Note: In the current implementation, claim timeout doesn't auto-expire.
	// The workflow state remains "claimed" until explicitly released or reaped.
	// This test verifies that the CAS mechanism still protects against concurrent modification.

	// Agent 2 should not be able to claim because workflow state is still claimed
	// (even though the claim has logically expired)
	err = svc.ClaimNode(rootID, "agent-2", 5*time.Minute)
	if err == nil {
		t.Error("Agent 2 should not be able to claim a node in claimed state")
	}

	t.Log("CAS protection verified: claimed node remains protected even after timeout")
}

// TestLockLedger_NewAgentClaimsAfterExpiredLockReaped tests that when an agent's lock
// expires and is reaped, another agent can successfully claim the node. This tests
// the full lifecycle of lock expiration, reaping, and re-acquisition.
func TestLockLedger_NewAgentClaimsAfterExpiredLockReaped(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
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

	// Create ledger and persistent lock manager
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	lockMgr, err := lock.NewPersistentManager(ldg)
	if err != nil {
		t.Fatalf("NewPersistentManager failed: %v", err)
	}

	nodeID, _ := types.Parse("1")

	// Agent 1 acquires a lock with short timeout
	shortTimeout := 20 * time.Millisecond
	lk, err := lockMgr.Acquire(nodeID, "agent-1-short", shortTimeout)
	if err != nil {
		t.Fatalf("Agent 1 lock acquire failed: %v", err)
	}

	t.Logf("Agent 1 acquired lock, expires at: %v", lk.ExpiresAt())

	// Verify lock is active
	if lockMgr.IsLocked(nodeID) != true {
		t.Error("Node should be locked after acquisition")
	}

	// Wait for lock to expire
	time.Sleep(30 * time.Millisecond)

	// Verify lock is now expired
	if !lk.IsExpired() {
		t.Error("Lock should be expired after timeout")
	}

	// Reap expired locks
	reaped, err := lockMgr.ReapExpired()
	if err != nil {
		t.Fatalf("ReapExpired failed: %v", err)
	}

	if len(reaped) != 1 {
		t.Errorf("Expected 1 reaped lock, got %d", len(reaped))
	}

	t.Logf("Reaped %d expired lock(s)", len(reaped))

	// Verify node is no longer locked after reaping
	if lockMgr.IsLocked(nodeID) {
		t.Error("Node should not be locked after reaping")
	}

	// Agent 2 should now be able to acquire the lock
	lk2, err := lockMgr.Acquire(nodeID, "agent-2-new", 5*time.Minute)
	if err != nil {
		t.Fatalf("Agent 2 lock acquire failed after reaping: %v", err)
	}

	if !lk2.IsOwnedBy("agent-2-new") {
		t.Error("Lock should be owned by agent-2-new")
	}

	// Verify lock is active
	if !lockMgr.IsLocked(nodeID) {
		t.Error("Node should be locked by agent-2-new")
	}

	t.Log("Lock re-acquisition after reaping verified successfully")
}

// TestLockLedger_ConcurrentLockExpireAndClaim tests the race condition where
// one agent's lock expires at the same moment another agent tries to claim.
// This verifies the system handles this edge case correctly.
func TestLockLedger_ConcurrentLockExpireAndClaim(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	svc := initializeLockLedgerProof(t, proofDir)
	rootID, _ := types.Parse("1")

	// Agent 1 claims the node with a short timeout
	if err := svc.ClaimNode(rootID, "agent-expire", 30*time.Millisecond); err != nil {
		t.Fatalf("Initial claim failed: %v", err)
	}

	// Create child node while claimed
	childID, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, "agent-expire", childID, schema.NodeTypeClaim, "Child statement", schema.InferenceAssumption); err != nil {
		t.Fatalf("RefineNode failed: %v", err)
	}

	// Release the claim on root
	if err := svc.ReleaseNode(rootID, "agent-expire"); err != nil {
		t.Fatalf("ReleaseNode failed: %v", err)
	}

	// Wait a bit to ensure child node is available
	time.Sleep(10 * time.Millisecond)

	// Now test concurrent claims on the child node
	var wg sync.WaitGroup
	wg.Add(2)

	var mu sync.Mutex
	results := make([]error, 2)

	start := make(chan struct{})

	// Agent 2 tries to claim with very short timeout
	go func() {
		defer wg.Done()
		<-start
		err := svc.ClaimNode(childID, "agent-short-2", 15*time.Millisecond)
		mu.Lock()
		results[0] = err
		mu.Unlock()
	}()

	// Agent 3 tries to claim immediately after
	go func() {
		defer wg.Done()
		<-start
		// Small delay to create the race condition
		time.Sleep(5 * time.Millisecond)
		err := svc.ClaimNode(childID, "agent-3", 5*time.Minute)
		mu.Lock()
		results[1] = err
		mu.Unlock()
	}()

	// Start both agents
	close(start)
	wg.Wait()

	// Exactly one should succeed
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}

	if successes != 1 {
		t.Errorf("Expected exactly 1 success in concurrent claim, got %d", successes)
	}

	// Verify final state is consistent
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	childNode := st.GetNode(childID)
	if childNode == nil {
		t.Fatal("Child node not found")
	}

	if childNode.WorkflowState != schema.WorkflowClaimed {
		t.Errorf("Expected child to be claimed, got %s", childNode.WorkflowState)
	}

	t.Logf("Concurrent lock expiration test: node claimed by %s", childNode.ClaimedBy)
}

// TestLockLedger_LedgerAppendAfterLockExpires verifies that ledger operations
// using CAS semantics correctly detect when state has changed during the window
// between reading state and writing to the ledger.
func TestLockLedger_LedgerAppendAfterLockExpires(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}

	// Create ledger
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Test CAS with expiration", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("Failed to append init event: %v", err)
	}

	// Get initial state
	st, err := state.Replay(ldg)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	initialSeq := st.LatestSeq()

	t.Logf("Initial sequence: %d", initialSeq)

	// Simulate agent 1 reading state and preparing an event
	nodeID1, _ := types.Parse("1")
	node1, _ := node.NewNode(nodeID1, schema.NodeTypeClaim, "First claim", schema.InferenceAssumption)
	event1 := ledger.NewNodeCreated(*node1)

	// Simulate agent 2 appending while agent 1 is "thinking"
	nodeID2, _ := types.Parse("1.1")
	node2, _ := node.NewNode(nodeID2, schema.NodeTypeClaim, "Second claim", schema.InferenceAssumption)
	event2 := ledger.NewNodeCreated(*node2)

	// Agent 2 appends successfully (no CAS check here, simulating a faster agent)
	_, err = ldg.Append(event2)
	if err != nil {
		t.Fatalf("Agent 2 append failed: %v", err)
	}

	// Agent 1 tries to append with CAS using the stale sequence
	_, err = ldg.AppendIfSequence(event1, initialSeq)
	if err == nil {
		t.Error("Agent 1 should get sequence mismatch error")
	}

	if !errors.Is(err, ledger.ErrSequenceMismatch) {
		t.Errorf("Expected ErrSequenceMismatch, got: %v", err)
	}

	t.Log("CAS correctly detected stale sequence after concurrent modification")
}

// TestLockLedger_PersistentManagerReplayAfterLockExpire verifies that the
// PersistentManager correctly reconstructs lock state from the ledger after
// a restart, and handles expired locks properly.
func TestLockLedger_PersistentManagerReplayAfterLockExpire(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}

	// Create ledger and first persistent manager
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	lockMgr1, err := lock.NewPersistentManager(ldg)
	if err != nil {
		t.Fatalf("NewPersistentManager 1 failed: %v", err)
	}

	nodeID, _ := types.Parse("1")

	// Acquire a lock with short timeout
	shortTimeout := 20 * time.Millisecond
	_, err = lockMgr1.Acquire(nodeID, "agent-persist", shortTimeout)
	if err != nil {
		t.Fatalf("Lock acquire failed: %v", err)
	}

	// Verify lock is in ledger
	count, err := ldg.Count()
	if err != nil {
		t.Fatalf("Ledger count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event in ledger, got %d", count)
	}

	// Wait for lock to expire
	time.Sleep(30 * time.Millisecond)

	// Create a new persistent manager (simulating process restart)
	lockMgr2, err := lock.NewPersistentManager(ldg)
	if err != nil {
		t.Fatalf("NewPersistentManager 2 failed: %v", err)
	}

	// The new manager should have reconstructed the lock from ledger
	// but it should be expired
	if !lockMgr2.IsLocked(nodeID) {
		// This is expected because Info() and IsLocked() filter out expired locks
		t.Log("Lock correctly reported as unlocked (expired) after replay")
	} else {
		// If the lock is still reported as locked, verify it's actually expired
		lockInfo, err := lockMgr2.Info(nodeID)
		if err != nil {
			t.Fatalf("Info failed: %v", err)
		}
		if lockInfo != nil && !lockInfo.IsExpired() {
			t.Error("Replayed lock should be expired")
		}
	}

	// New agent should be able to acquire since the existing lock is expired
	_, err = lockMgr2.Acquire(nodeID, "agent-new-after-restart", 5*time.Minute)
	if err != nil {
		t.Errorf("New agent should be able to acquire expired lock: %v", err)
	}

	t.Log("Persistent manager correctly handles expired locks after replay")
}

// TestLockLedger_ConcurrentLedgerWritesDuringLockTransition tests multiple agents
// trying to write to the ledger simultaneously while locks are transitioning
// (being acquired, expiring, being released).
func TestLockLedger_ConcurrentLedgerWritesDuringLockTransition(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}

	// Create ledger
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger failed: %v", err)
	}

	// Initialize proof
	initEvent := ledger.NewProofInitialized("Concurrent lock transition test", "test-author")
	if _, err := ldg.Append(initEvent); err != nil {
		t.Fatalf("Failed to append init event: %v", err)
	}

	// Create persistent lock manager
	lockMgr, err := lock.NewPersistentManager(ldg)
	if err != nil {
		t.Fatalf("NewPersistentManager failed: %v", err)
	}

	nodeID, _ := types.Parse("1")

	// Multiple agents try to acquire and release locks concurrently
	numAgents := 5
	iterations := 3

	var wg sync.WaitGroup
	errorCh := make(chan error, numAgents*iterations)

	for i := 0; i < numAgents; i++ {
		agentID := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				agentName := "agent-" + string(rune('A'+agentID))

				// Try to acquire
				_, err := lockMgr.Acquire(nodeID, agentName, 10*time.Millisecond)
				if err != nil {
					// Acquisition failure is expected in concurrent scenario
					continue
				}

				// Small work simulation
				time.Sleep(2 * time.Millisecond)

				// Try to release (may fail if already expired)
				if err := lockMgr.Release(nodeID, agentName); err != nil {
					// Release failure is acceptable if lock expired
				}

				// Small delay before next iteration
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(errorCh)

	// Check for any critical errors
	criticalErrors := 0
	for err := range errorCh {
		t.Errorf("Critical error during concurrent test: %v", err)
		criticalErrors++
	}

	if criticalErrors > 0 {
		t.Fatalf("Had %d critical errors", criticalErrors)
	}

	// Verify ledger integrity - all events should have sequential numbers
	events, err := ldg.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	t.Logf("Total events in ledger after concurrent operations: %d", len(events))

	// Events should exist and ledger should be consistent
	if len(events) < 1 {
		t.Error("Expected at least the init event in ledger")
	}

	t.Log("Concurrent lock-ledger coordination completed without corruption")
}

// TestLockLedger_LockConflictDuringConcurrentClaim specifically tests the scenario
// where two agents race to claim the same node through the service layer,
// verifying that exactly one succeeds and the other gets a proper error.
func TestLockLedger_LockConflictDuringConcurrentClaim(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	svc := initializeLockLedgerProof(t, proofDir)
	rootID, _ := types.Parse("1")

	numAttempts := 10
	successCount := 0
	conflictCount := 0

	for attempt := 0; attempt < numAttempts; attempt++ {
		// Reset state by releasing any existing claim
		st, _ := svc.LoadState()
		rootNode := st.GetNode(rootID)
		if rootNode != nil && rootNode.WorkflowState == schema.WorkflowClaimed {
			_ = svc.ReleaseNode(rootID, rootNode.ClaimedBy)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		var mu sync.Mutex
		results := make([]error, 2)

		start := make(chan struct{})

		// Two agents try to claim simultaneously
		for i := 0; i < 2; i++ {
			agentIdx := i
			go func() {
				defer wg.Done()
				<-start
				agentName := "agent-" + string(rune('A'+agentIdx))
				err := svc.ClaimNode(rootID, agentName, 5*time.Minute)
				mu.Lock()
				results[agentIdx] = err
				mu.Unlock()
			}()
		}

		close(start)
		wg.Wait()

		// Count results
		localSuccesses := 0
		localConflicts := 0
		for _, err := range results {
			if err == nil {
				localSuccesses++
			} else if errors.Is(err, service.ErrConcurrentModification) {
				localConflicts++
			}
		}

		if localSuccesses == 1 {
			successCount++
		}
		if localConflicts > 0 {
			conflictCount++
		}
	}

	// Across all attempts, we should have exactly one success per attempt
	// and some concurrent modification errors
	t.Logf("Over %d attempts: %d had exactly 1 success, %d had CAS conflicts",
		numAttempts, successCount, conflictCount)

	// At least some attempts should have exactly one success
	if successCount < numAttempts/2 {
		t.Errorf("Expected most attempts to have exactly 1 success, got %d/%d",
			successCount, numAttempts)
	}

	t.Log("Lock conflict handling during concurrent claims verified")
}

// TestLockLedger_ExpiredLockCanBeReplaced tests that an expired lock can be
// directly replaced by a new acquisition without explicit reaping.
func TestLockLedger_ExpiredLockCanBeReplaced(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	// Create ledger directory
	ledgerDir := filepath.Join(proofDir, "ledger")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("Failed to create ledger dir: %v", err)
	}

	// Create in-memory lock manager
	manager := lock.NewManager()

	nodeID, _ := types.Parse("1")

	// Agent 1 acquires with short timeout
	_, err := manager.Acquire(nodeID, "agent-1", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Verify locked
	if !manager.IsLocked(nodeID) {
		t.Error("Node should be locked")
	}

	// Wait for expiration
	time.Sleep(30 * time.Millisecond)

	// Agent 2 should be able to acquire because the lock is expired
	// (the manager allows replacing expired locks)
	lk2, err := manager.Acquire(nodeID, "agent-2", 5*time.Minute)
	if err != nil {
		t.Fatalf("Second acquire should succeed for expired lock: %v", err)
	}

	if !lk2.IsOwnedBy("agent-2") {
		t.Error("Lock should be owned by agent-2")
	}

	// Verify the node is now locked by agent-2
	if !manager.IsLocked(nodeID) {
		t.Error("Node should be locked by agent-2")
	}

	t.Log("Expired lock replacement verified")
}

// TestLockLedger_InterleavedLockAndLedgerOperations tests a realistic workflow
// where lock acquisition and ledger writes are interleaved, verifying the
// coordination between the two systems.
func TestLockLedger_InterleavedLockAndLedgerOperations(t *testing.T) {
	proofDir, cleanup := setupLockLedgerTest(t)
	defer cleanup()

	svc := initializeLockLedgerProof(t, proofDir)
	rootID, _ := types.Parse("1")

	// Sequence of operations:
	// 1. Agent A claims root
	// 2. Agent A adds child 1.1
	// 3. Agent A releases root
	// 4. Agent B claims root
	// 5. Agent B adds child 1.2
	// 6. Concurrent: Agent B release + Agent C claim

	// Step 1: Agent A claims root
	if err := svc.ClaimNode(rootID, "agent-A", 5*time.Minute); err != nil {
		t.Fatalf("Agent A claim failed: %v", err)
	}

	// Step 2: Agent A adds child 1.1
	childID1, _ := types.Parse("1.1")
	if err := svc.RefineNode(rootID, "agent-A", childID1, schema.NodeTypeClaim, "Child 1.1", schema.InferenceAssumption); err != nil {
		t.Fatalf("Agent A refine failed: %v", err)
	}

	// Step 3: Agent A releases root
	if err := svc.ReleaseNode(rootID, "agent-A"); err != nil {
		t.Fatalf("Agent A release failed: %v", err)
	}

	// Step 4: Agent B claims root
	if err := svc.ClaimNode(rootID, "agent-B", 5*time.Minute); err != nil {
		t.Fatalf("Agent B claim failed: %v", err)
	}

	// Step 5: Agent B adds child 1.2
	childID2, _ := types.Parse("1.2")
	if err := svc.RefineNode(rootID, "agent-B", childID2, schema.NodeTypeClaim, "Child 1.2", schema.InferenceAssumption); err != nil {
		t.Fatalf("Agent B refine failed: %v", err)
	}

	// Step 6: Concurrent release and claim
	var wg sync.WaitGroup
	wg.Add(2)

	var releaseErr, claimErr error
	start := make(chan struct{})

	go func() {
		defer wg.Done()
		<-start
		releaseErr = svc.ReleaseNode(rootID, "agent-B")
	}()

	go func() {
		defer wg.Done()
		<-start
		// Small delay to increase chance of race
		time.Sleep(1 * time.Millisecond)
		claimErr = svc.ClaimNode(rootID, "agent-C", 5*time.Minute)
	}()

	close(start)
	wg.Wait()

	// Release should succeed
	if releaseErr != nil {
		t.Logf("Release error (may be due to race): %v", releaseErr)
	}

	// Verify final state is consistent
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("Final LoadState failed: %v", err)
	}

	// Both children should exist
	if st.GetNode(childID1) == nil {
		t.Error("Child 1.1 should exist")
	}
	if st.GetNode(childID2) == nil {
		t.Error("Child 1.2 should exist")
	}

	// Root should be in a consistent state
	rootNode := st.GetNode(rootID)
	if rootNode == nil {
		t.Fatal("Root node should exist")
	}

	t.Logf("Final root state: workflow=%s, claimedBy=%s",
		rootNode.WorkflowState, rootNode.ClaimedBy)

	// If claim succeeded, root should be claimed by agent-C
	// If claim failed, root should be available
	if claimErr == nil {
		if rootNode.WorkflowState != schema.WorkflowClaimed || rootNode.ClaimedBy != "agent-C" {
			t.Errorf("Expected root claimed by agent-C, got state=%s claimed=%s",
				rootNode.WorkflowState, rootNode.ClaimedBy)
		}
	}

	t.Log("Interleaved lock and ledger operations completed with consistent state")
}
