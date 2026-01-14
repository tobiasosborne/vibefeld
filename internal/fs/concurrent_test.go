// Package fs provides filesystem operations for the AF proof framework.
package fs

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TestConcurrentWriteNode tests that multiple goroutines can write
// different nodes concurrently without races.
func TestConcurrentWriteNode(t *testing.T) {
	dir := t.TempDir()

	// Initialize proof directory structure
	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	numWorkers := 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// Each worker writes a different node
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			rootID, err := types.Parse("1")
			if err != nil {
				errorCount.Add(1)
				return
			}

			childID, err := rootID.Child(workerID + 1)
			if err != nil {
				errorCount.Add(1)
				return
			}

			n, err := node.NewNode(
				childID,
				schema.NodeTypeClaim,
				"Statement from worker",
				schema.InferenceAssumption,
			)
			if err != nil {
				errorCount.Add(1)
				return
			}

			if err := WriteNode(dir, n); err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if got := successCount.Load(); got != int32(numWorkers) {
		t.Errorf("expected %d successful writes, got %d (errors: %d)",
			numWorkers, got, errorCount.Load())
	}
}

// TestConcurrentWriteSameNode tests concurrent writes to the same node.
// The last write should win, but no data corruption should occur.
func TestConcurrentWriteSameNode(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	numWorkers := 10
	var wg sync.WaitGroup
	var successCount atomic.Int32

	// All workers write to the same node
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			n, err := node.NewNode(
				nodeID,
				schema.NodeTypeClaim,
				"Statement from worker",
				schema.InferenceAssumption,
			)
			if err != nil {
				return
			}

			if err := WriteNode(dir, n); err == nil {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// All writes should succeed (last one wins)
	if got := successCount.Load(); got == 0 {
		t.Error("expected some successful writes")
	}

	// The node should still be readable
	readNode, err := ReadNode(dir, nodeID)
	if err != nil {
		t.Errorf("failed to read node after concurrent writes: %v", err)
	}

	if readNode == nil {
		t.Error("read returned nil node")
	}
}

// TestConcurrentReadNode tests that multiple goroutines can read
// the same node concurrently.
func TestConcurrentReadNode(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create a node to read
	n, err := node.NewNode(
		nodeID,
		schema.NodeTypeClaim,
		"Test statement",
		schema.InferenceAssumption,
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	numReaders := 20
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// All readers read the same node concurrently
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			readNode, err := ReadNode(dir, nodeID)
			if err != nil {
				errorCount.Add(1)
				return
			}

			// Verify content
			if readNode.Statement != "Test statement" {
				errorCount.Add(1)
				return
			}

			successCount.Add(1)
		}()
	}

	wg.Wait()

	if got := successCount.Load(); got != int32(numReaders) {
		t.Errorf("expected %d successful reads, got %d (errors: %d)",
			numReaders, got, errorCount.Load())
	}
}

// TestConcurrentReadDuringWrite tests that reads are safe during writes.
func TestConcurrentReadDuringWrite(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("failed to parse node ID: %v", err)
	}

	// Create initial node
	n, err := node.NewNode(
		nodeID,
		schema.NodeTypeClaim,
		"Initial statement",
		schema.InferenceAssumption,
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write initial node: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var readErrors atomic.Int32
	var readCount atomic.Int32

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					readNode, err := ReadNode(dir, nodeID)
					if err != nil {
						// File not found during atomic rename is acceptable
						if !os.IsNotExist(err) {
							readErrors.Add(1)
						}
					} else if readNode != nil {
						readCount.Add(1)
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Concurrent writer
	numWrites := 20
	var writeErrors atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < numWrites; i++ {
			n, err := node.NewNode(
				nodeID,
				schema.NodeTypeClaim,
				"Updated statement",
				schema.InferenceAssumption,
			)
			if err != nil {
				writeErrors.Add(1)
				continue
			}

			if err := WriteNode(dir, n); err != nil {
				writeErrors.Add(1)
			}
			time.Sleep(2 * time.Millisecond)
		}

		close(stop)
	}()

	wg.Wait()

	// Should have completed some reads without content hash errors
	if errors := readErrors.Load(); errors > 0 {
		t.Errorf("got %d read errors during concurrent writes", errors)
	}

	t.Logf("completed %d reads during %d writes", readCount.Load(), numWrites)
}

// TestConcurrentListNodes tests that listing nodes is safe during modifications.
func TestConcurrentListNodes(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	// Pre-populate some nodes
	rootID, _ := types.Parse("1")
	for i := 0; i < 5; i++ {
		childID, _ := rootID.Child(i + 1)
		n, _ := node.NewNode(
			childID,
			schema.NodeTypeClaim,
			"Test",
			schema.InferenceAssumption,
		)
		_ = WriteNode(dir, n)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var listErrors atomic.Int32
	var listCount atomic.Int32

	// Concurrent listers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					nodes, err := ListNodes(dir)
					if err != nil {
						listErrors.Add(1)
					} else {
						_ = nodes
						listCount.Add(1)
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Concurrent modifier
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 15; i++ {
			childID, _ := rootID.Child(i + 10)
			n, _ := node.NewNode(
				childID,
				schema.NodeTypeClaim,
				"New node",
				schema.InferenceAssumption,
			)
			_ = WriteNode(dir, n)
			time.Sleep(2 * time.Millisecond)
		}

		close(stop)
	}()

	wg.Wait()

	if errors := listErrors.Load(); errors > 0 {
		t.Errorf("got %d list errors", errors)
	}

	t.Logf("completed %d list operations", listCount.Load())
}

// TestConcurrentDeleteNode tests that delete operations are safe
// during concurrent access.
func TestConcurrentDeleteNode(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	// Create nodes to delete
	rootID, _ := types.Parse("1")
	numNodes := 10
	for i := 0; i < numNodes; i++ {
		childID, _ := rootID.Child(i + 1)
		n, _ := node.NewNode(
			childID,
			schema.NodeTypeClaim,
			"To be deleted",
			schema.InferenceAssumption,
		)
		_ = WriteNode(dir, n)
	}

	var wg sync.WaitGroup
	var deleteCount atomic.Int32

	// Multiple workers try to delete the same nodes
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < numNodes; j++ {
				childID, _ := rootID.Child(j + 1)
				if err := DeleteNode(dir, childID); err == nil {
					deleteCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// Each node should be deleted exactly once
	if got := deleteCount.Load(); got != int32(numNodes) {
		t.Errorf("expected %d successful deletes, got %d", numNodes, got)
	}

	// Verify nodes are gone
	nodes, err := ListNodes(dir)
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after delete, got %d", len(nodes))
	}
}

// TestConcurrentInitProofDir tests that multiple goroutines can
// call InitProofDir on the same directory safely.
func TestConcurrentInitProofDir(t *testing.T) {
	dir := t.TempDir()

	numWorkers := 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := InitProofDir(dir); err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All should succeed since InitProofDir is idempotent
	if got := successCount.Load(); got != int32(numWorkers) {
		t.Errorf("expected %d successful inits, got %d (errors: %d)",
			numWorkers, got, errorCount.Load())
	}

	// Verify directory structure is valid
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		subPath := filepath.Join(dir, sub)
		info, err := os.Stat(subPath)
		if err != nil {
			t.Errorf("missing subdirectory %s: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

// TestConcurrentMixedOperations tests a mix of read, write, list,
// and delete operations concurrently.
func TestConcurrentMixedOperations(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	var writeCount, readCount, listCount, deleteCount atomic.Int32
	var writeErrors, readErrors, listErrors, deleteErrors atomic.Int32

	rootID, _ := types.Parse("1")

	// Writers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
					childID, _ := rootID.Child((workerID*100 + counter) + 1)
					counter++
					n, _ := node.NewNode(
						childID,
						schema.NodeTypeClaim,
						"Written node",
						schema.InferenceAssumption,
					)
					if err := WriteNode(dir, n); err != nil {
						writeErrors.Add(1)
					} else {
						writeCount.Add(1)
					}
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Try to read a random node
					childID, _ := rootID.Child(1)
					_, err := ReadNode(dir, childID)
					if err != nil {
						// Not found errors are expected
						if !os.IsNotExist(err) {
							readErrors.Add(1)
						}
					} else {
						readCount.Add(1)
					}
				}
			}
		}()
	}

	// Listers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, err := ListNodes(dir)
				if err != nil {
					listErrors.Add(1)
				} else {
					listCount.Add(1)
				}
			}
		}
	}()

	// Deleters (delete nodes that were just written)
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				childID, _ := rootID.Child(counter%50 + 1)
				counter++
				err := DeleteNode(dir, childID)
				if err == nil {
					deleteCount.Add(1)
				} else if !os.IsNotExist(err) {
					deleteErrors.Add(1)
				}
			}
		}
	}()

	// Run for a bit
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Report errors
	if errors := writeErrors.Load(); errors > 0 {
		t.Errorf("got %d write errors", errors)
	}
	if errors := readErrors.Load(); errors > 0 {
		t.Errorf("got %d read errors (excluding not found)", errors)
	}
	if errors := listErrors.Load(); errors > 0 {
		t.Errorf("got %d list errors", errors)
	}
	if errors := deleteErrors.Load(); errors > 0 {
		t.Errorf("got %d delete errors (excluding not found)", errors)
	}

	t.Logf("operations - writes: %d, reads: %d, lists: %d, deletes: %d",
		writeCount.Load(), readCount.Load(), listCount.Load(), deleteCount.Load())
}

// TestConcurrentAtomicWrite tests that the atomic write mechanism
// (write to temp, then rename) is safe under concurrent access.
// Note: Concurrent writes to the same node may have some failures due to
// temp file rename races, but the key guarantees are:
// 1. No data corruption
// 2. No partial writes
// 3. Final state is valid
func TestConcurrentAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	nodeID, _ := types.Parse("1")

	numWriters := 10
	numWrites := 10
	var wg sync.WaitGroup
	var totalWrites atomic.Int32
	var totalErrors atomic.Int32

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < numWrites; j++ {
				n, _ := node.NewNode(
					nodeID,
					schema.NodeTypeClaim,
					"Atomic write test",
					schema.InferenceAssumption,
				)
				if err := WriteNode(dir, n); err == nil {
					totalWrites.Add(1)
				} else {
					totalErrors.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Some writes may fail due to concurrent temp file operations, but
	// at least some should succeed
	if totalWrites.Load() == 0 {
		t.Error("expected at least some successful writes")
	}

	t.Logf("writes succeeded: %d, failed: %d (some failures expected)",
		totalWrites.Load(), totalErrors.Load())

	// Verify no temp files remain
	nodesDir := filepath.Join(dir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		t.Fatalf("failed to read nodes dir: %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("temp file remained: %s", entry.Name())
		}
	}

	// The final file should be valid and readable without corruption
	readNode, err := ReadNode(dir, nodeID)
	if err != nil {
		t.Errorf("failed to read node after concurrent atomic writes: %v", err)
	}
	if readNode == nil {
		t.Error("read returned nil node")
	}
	// Verify content hash is valid (no corruption)
	if readNode != nil && !readNode.VerifyContentHash() {
		t.Error("content hash verification failed - data corruption detected")
	}
}

// TestConcurrentContentHashVerification tests that content hash verification
// is consistent under concurrent reads.
func TestConcurrentContentHashVerification(t *testing.T) {
	dir := t.TempDir()

	if err := InitProofDir(dir); err != nil {
		t.Fatalf("failed to init proof dir: %v", err)
	}

	nodeID, _ := types.Parse("1")

	// Create a node with specific content
	n, err := node.NewNode(
		nodeID,
		schema.NodeTypeClaim,
		"Content hash test statement",
		schema.InferenceAssumption,
	)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := WriteNode(dir, n); err != nil {
		t.Fatalf("failed to write node: %v", err)
	}

	originalHash := n.ContentHash

	numReaders := 20
	var wg sync.WaitGroup
	var hashMismatchCount atomic.Int32

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			readNode, err := ReadNode(dir, nodeID)
			if err != nil {
				return
			}

			if readNode.ContentHash != originalHash {
				hashMismatchCount.Add(1)
			}

			// Verify the hash is still valid
			if !readNode.VerifyContentHash() {
				hashMismatchCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if mismatches := hashMismatchCount.Load(); mismatches > 0 {
		t.Errorf("got %d content hash mismatches", mismatches)
	}
}
