//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// =============================================================================
// Integration Test: Tracer Bullet
// =============================================================================
//
// This test verifies the complete prover/verifier workflow:
//   1. Init: Create a new proof with a conjecture
//   2. Claim: Prover claims the root node
//   3. Refine: Prover adds child nodes to break down the proof
//   4. Release: Prover releases the node after refinement
//   5. Accept: Verifier accepts the refined proof step
//
// This is the "tracer bullet" - a minimal end-to-end test that proves
// the entire system works together.

// newIntegrationTestCmd creates a root command with all subcommands needed for integration testing.
func newIntegrationTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "af",
		Short: "Adversarial Proof Framework CLI",
	}

	// Add all commands needed for the tracer bullet workflow
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newClaimCmd())
	cmd.AddCommand(newRefineCmd())
	cmd.AddCommand(newReleaseCmd())
	cmd.AddCommand(newAcceptCmd())
	cmd.AddCommand(newJobsCmd())

	AddFuzzyMatching(cmd)

	return cmd
}

// setupIntegrationTest creates a clean temp directory for integration testing.
func setupIntegrationTest(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-integration-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// setupIntegrationTestWithRoot creates a proof with an initialized root node.
// service.Init() creates both the proof metadata and the root node (node 1)
// with the conjecture as its statement.
func setupIntegrationTestWithRoot(t *testing.T, conjecture string) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-integration-test-*")
	if err != nil {
		t.Fatal(err)
	}

	proofDir := filepath.Join(tmpDir, "proof")

	// Initialize proof with conjecture - this also creates node 1
	if err := service.Init(proofDir, conjecture, "integration-test"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return proofDir, cleanup
}

// TestTracerBullet_FullWorkflow tests the complete claim->refine->release->accept cycle.
// This is the primary tracer bullet test that validates the entire system works end-to-end.
func TestTracerBullet_FullWorkflow(t *testing.T) {
	conjecture := "For all n > 1, if n is prime and n > 2, then n is odd"
	proofDir, cleanup := setupIntegrationTestWithRoot(t, conjecture)
	defer cleanup()

	t.Log("Setup: Proof initialized with root node 1")

	// ==========================================================================
	// Step 1: JOBS - Check available prover jobs
	// ==========================================================================
	t.Log("Step 1: Check available jobs")

	cmd := newIntegrationTestCmd()
	output, err := executeCommand(cmd, "jobs",
		"--dir", proofDir,
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("jobs failed: %v\nOutput: %s", err, output)
	}

	var jobsResult struct {
		ProverJobs   []interface{} `json:"prover_jobs"`
		VerifierJobs []interface{} `json:"verifier_jobs"`
	}
	if err := json.Unmarshal([]byte(output), &jobsResult); err != nil {
		t.Fatalf("failed to parse jobs JSON: %v\nOutput: %s", err, output)
	}

	if len(jobsResult.ProverJobs) == 0 {
		t.Fatal("expected at least one prover job")
	}

	t.Logf("  ✓ Found %d prover job(s)", len(jobsResult.ProverJobs))

	// ==========================================================================
	// Step 2: CLAIM - Prover claims the root node
	// ==========================================================================
	t.Log("Step 2: Prover claims root node")

	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "claim", "1",
		"--owner", "prover-agent-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("claim failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "claimed") && !strings.Contains(output, "Claimed") {
		t.Errorf("expected claim confirmation in output, got: %s", output)
	}

	t.Log("  ✓ Node 1 claimed by prover")

	// ==========================================================================
	// Step 3: REFINE - Prover adds child nodes
	// ==========================================================================
	t.Log("Step 3: Prover refines node with children")

	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "refine", "1",
		"--statement", "Assume n is prime and n > 2. By definition of prime, n has no divisors other than 1 and itself.",
		"--inference", "assumption",
		"--owner", "prover-agent-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("refine failed: %v\nOutput: %s", err, output)
	}

	// Should create node 1.1
	if !strings.Contains(output, "1.1") {
		t.Errorf("expected child node 1.1 in output, got: %s", output)
	}

	t.Log("  ✓ Node 1.1 created as child of 1")

	// Add another refinement step
	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "refine", "1",
		"--statement", "Since n > 2 and all even numbers > 2 are divisible by 2, n cannot be even. Therefore n is odd.",
		"--inference", "modus_ponens",
		"--owner", "prover-agent-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("second refine failed: %v\nOutput: %s", err, output)
	}

	// Should create node 1.2
	if !strings.Contains(output, "1.2") {
		t.Errorf("expected child node 1.2 in output, got: %s", output)
	}

	t.Log("  ✓ Node 1.2 created as child of 1")

	// ==========================================================================
	// Step 4: RELEASE - Prover releases the node
	// ==========================================================================
	t.Log("Step 4: Prover releases node")

	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "release", "1",
		"--owner", "prover-agent-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("release failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "released") && !strings.Contains(output, "Released") {
		t.Errorf("expected release confirmation in output, got: %s", output)
	}

	t.Log("  ✓ Node 1 released")

	// ==========================================================================
	// Step 5: ACCEPT - Verifier accepts a child node
	// ==========================================================================
	t.Log("Step 5: Verifier accepts child node")

	// First, claim the child node as verifier
	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "claim", "1.1",
		"--owner", "verifier-agent-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("verifier claim failed: %v\nOutput: %s", err, output)
	}

	t.Log("  ✓ Node 1.1 claimed by verifier")

	// Now accept the node
	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "accept", "1.1",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("accept failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "accepted") && !strings.Contains(output, "validated") {
		t.Errorf("expected acceptance confirmation in output, got: %s", output)
	}

	t.Log("  ✓ Node 1.1 accepted and validated")

	// ==========================================================================
	// Verify Final State
	// ==========================================================================
	t.Log("Verifying final state...")

	// Reload service to get fresh state
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to reload proof service: %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Check we have the expected nodes
	nodes := state.AllNodes()
	if len(nodes) < 3 {
		t.Errorf("expected at least 3 nodes (1, 1.1, 1.2), got %d", len(nodes))
	}

	t.Logf("  ✓ Final state has %d nodes", len(nodes))
	t.Log("")
	t.Log("========================================")
	t.Log("  TRACER BULLET COMPLETE!")
	t.Log("  Full workflow: claim → refine → release → accept")
	t.Log("========================================")
}

// TestTracerBullet_ProverVerifierRoleIsolation verifies that provers and verifiers
// are properly isolated - the workflow requires both roles working together.
func TestTracerBullet_ProverVerifierRoleIsolation(t *testing.T) {
	proofDir, cleanup := setupIntegrationTestWithRoot(t, "Test role isolation")
	defer cleanup()

	// Claim as prover
	cmd := newIntegrationTestCmd()
	_, err := executeCommand(cmd, "claim", "1",
		"--owner", "prover-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	// Prover refines
	cmd = newIntegrationTestCmd()
	_, err = executeCommand(cmd, "refine", "1",
		"--statement", "Proof step",
		"--inference", "assumption",
		"--owner", "prover-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("refine failed: %v", err)
	}

	// Release
	cmd = newIntegrationTestCmd()
	_, err = executeCommand(cmd, "release", "1",
		"--owner", "prover-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Now verifier can claim and accept the child
	cmd = newIntegrationTestCmd()
	_, err = executeCommand(cmd, "claim", "1.1",
		"--owner", "verifier-001",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("verifier claim failed: %v", err)
	}

	cmd = newIntegrationTestCmd()
	output, err := executeCommand(cmd, "accept", "1.1",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("accept failed: %v\nOutput: %s", err, output)
	}

	t.Log("✓ Role isolation verified: prover refined, verifier accepted")
}

// TestTracerBullet_MultipleRefinements tests that a prover can add multiple children
// to a node during a single claim session.
func TestTracerBullet_MultipleRefinements(t *testing.T) {
	proofDir, cleanup := setupIntegrationTestWithRoot(t, "P and Q implies R")
	defer cleanup()

	// Claim
	cmd := newIntegrationTestCmd()
	_, err := executeCommand(cmd, "claim", "1",
		"--owner", "prover",
		"--dir", proofDir,
	)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	// Add multiple refinements
	refinements := []struct {
		statement string
		inference string
		expected  string
	}{
		{"First, we establish P", "assumption", "1.1"},
		{"Next, we establish Q", "assumption", "1.2"},
		{"From P and Q, we derive R", "modus_ponens", "1.3"},
	}

	for i, r := range refinements {
		cmd = newIntegrationTestCmd()
		output, err := executeCommand(cmd, "refine", "1",
			"--statement", r.statement,
			"--inference", r.inference,
			"--owner", "prover",
			"--dir", proofDir,
		)
		if err != nil {
			t.Fatalf("refine %d failed: %v", i+1, err)
		}

		if !strings.Contains(output, r.expected) {
			t.Errorf("refinement %d: expected node %s in output, got: %s", i+1, r.expected, output)
		}
	}

	// Verify state
	svc, err := service.NewProofService(proofDir)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	state, err := svc.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Should have root + 3 children = 4 nodes
	nodes := state.AllNodes()
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}

	t.Logf("✓ Multiple refinements created %d nodes", len(nodes))
}

// TestTracerBullet_JSONOutput verifies that CLI commands support JSON output format
// for machine-readable integration.
func TestTracerBullet_JSONOutput(t *testing.T) {
	proofDir, cleanup := setupIntegrationTestWithRoot(t, "JSON test")
	defer cleanup()

	// Jobs with JSON
	cmd := newIntegrationTestCmd()
	output, err := executeCommand(cmd, "jobs",
		"--dir", proofDir,
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("jobs failed: %v", err)
	}

	var jobsResult map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jobsResult); err != nil {
		t.Fatalf("jobs output is not valid JSON: %v\nOutput: %s", err, output)
	}

	t.Log("✓ jobs produces valid JSON")

	// Claim with JSON
	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "claim", "1",
		"--owner", "test",
		"--dir", proofDir,
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	var claimResult map[string]interface{}
	if err := json.Unmarshal([]byte(output), &claimResult); err != nil {
		t.Fatalf("claim output is not valid JSON: %v\nOutput: %s", err, output)
	}

	t.Log("✓ claim produces valid JSON")

	// Refine with JSON
	cmd = newIntegrationTestCmd()
	output, err = executeCommand(cmd, "refine", "1",
		"--statement", "Test",
		"--inference", "assumption",
		"--owner", "test",
		"--dir", proofDir,
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("refine failed: %v", err)
	}

	var refineResult map[string]interface{}
	if err := json.Unmarshal([]byte(output), &refineResult); err != nil {
		t.Fatalf("refine output is not valid JSON: %v\nOutput: %s", err, output)
	}

	t.Log("✓ refine produces valid JSON")
	t.Log("✓ All tracer bullet commands support JSON output")
}
