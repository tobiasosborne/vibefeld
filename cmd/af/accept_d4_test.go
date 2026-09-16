//go:build integration

package main

import (
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

// setupD4AcceptTest initializes a proof with node 1 and a single pending child
// 1.2, the minimal shape the D4 scheduling tests need.
func setupD4AcceptTest(t *testing.T) (string, *service.ProofService) {
	t.Helper()
	dir := t.TempDir()
	if err := service.InitProofDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := service.Init(dir, "D4 accept fixture", "test-author"); err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateNode(nid("1.2"), service.NodeTypeClaim, "child 1.2", service.InferenceAssumption); err != nil {
		t.Fatal(err)
	}
	return dir, svc
}

// TestAcceptCmd_D4_SchedulesChildBeforeParent covers `af accept 1 1.2` with 1.2
// pending: 1.2 is accepted first, then 1, in one batch.
func TestAcceptCmd_D4_SchedulesChildBeforeParent(t *testing.T) {
	dir, _ := setupD4AcceptTest(t)

	output, err := executeBulkAcceptCommand(t, "1", "1.2", "-d", dir)
	if err != nil {
		t.Fatalf("accept 1 1.2: %v\n%s", err, output)
	}
	if strings.Index(output, "1.2 - validated") > strings.Index(output, "1 - validated") {
		t.Errorf("expected 1.2 scheduled before 1, got:\n%s", output)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "1.2"} {
		if got := st.GetNode(nid(id)).EpistemicState; got != service.EpistemicValidated {
			t.Errorf("node %s = %s, want validated", id, got)
		}
	}
}

// TestAcceptCmd_D4_BlockedPrerequisitePending covers a parent whose pending
// child is not in the batch: blocked:prerequisite-pending and not validated.
func TestAcceptCmd_D4_BlockedPrerequisitePending(t *testing.T) {
	dir, _ := setupD4AcceptTest(t)

	// Create a grandchild that this batch does not include. 1.2 then cannot be
	// accepted, and neither can 1.
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateNode(nid("1.2.1"), service.NodeTypeClaim, "grandchild", service.InferenceAssumption); err != nil {
		t.Fatal(err)
	}

	output, err := executeBulkAcceptCommand(t, "1", "1.2", "-d", dir)
	if err == nil {
		t.Fatalf("expected a partial/none exit error:\n%s", output)
	}
	if !strings.Contains(output, "blocked:prerequisite-pending") {
		t.Fatalf("expected blocked:prerequisite-pending, got:\n%s", output)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "1.2"} {
		if got := st.GetNode(nid(id)).EpistemicState; got == service.EpistemicValidated {
			t.Errorf("node %s was validated despite a pending prerequisite", id)
		}
	}
}

// TestAcceptCmd_D4_AllSchedulesChildren covers `--all`: every pending node is
// scheduled by prerequisite, so a parent and child both get accepted.
func TestAcceptCmd_D4_AllSchedulesChildren(t *testing.T) {
	dir, _ := setupD4AcceptTest(t)

	output, err := executeBulkAcceptCommand(t, "--all", "-d", dir)
	if err != nil {
		t.Fatalf("accept --all: %v\n%s", err, output)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "1.2"} {
		if got := st.GetNode(nid(id)).EpistemicState; got != service.EpistemicValidated {
			t.Errorf("node %s = %s, want validated", id, got)
		}
	}
}
