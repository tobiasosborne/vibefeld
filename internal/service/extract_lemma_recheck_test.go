package service

import (
	"errors"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

// TestExtractLemma_RechecksSourceInCommit verifies the source-validated and
// no-open-scope checks live in ExtractLemma's commit closure, not only in the
// CLI. A caller that observed a validated node earlier must still be refused
// if the node was unvalidated before the commit's state read, and no
// LemmaExtracted event may be appended.
func TestExtractLemma_RechecksSourceInCommit(t *testing.T) {
	svc, _ := setupVerdictTestProof(t)
	childID := parseNodeID(t, "1.1")

	if err := svc.AcceptNodeWithVerifier(childID, "", "verifier-1", ""); err != nil {
		t.Fatalf("AcceptNodeWithVerifier: %v", err)
	}
	// Simulate an unvalidation landing between the CLI's "is validated" check
	// and ExtractLemma's own commit read.
	if err := svc.UnvalidateNode(childID, "revoked", "verifier-1"); err != nil {
		t.Fatalf("UnvalidateNode: %v", err)
	}

	before, err := ledger.Count(svc.ledgerDir())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}

	if _, err := svc.ExtractLemma(childID, "should not be extracted"); err == nil {
		t.Fatal("ExtractLemma on an unvalidated source unexpectedly succeeded")
	} else if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("ExtractLemma err = %v, want ErrInvalidState", err)
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(st.AllLemmas()) != 0 {
		t.Fatalf("state has %d lemmas, want 0", len(st.AllLemmas()))
	}
	after, err := ledger.Count(svc.ledgerDir())
	if err != nil {
		t.Fatalf("Count after: %v", err)
	}
	if after != before {
		t.Fatalf("ledger grew from %d to %d; no LemmaExtracted should have been appended", before, after)
	}
}
