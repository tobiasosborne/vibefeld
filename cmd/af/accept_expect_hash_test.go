package main

import (
	"bytes"
	"testing"
)

// D3: `af accept --expect-hash` threads the caller's expected hash through to
// the accept and records expected_hash_checked=true. A mismatch is refused.
func TestAcceptCmd_ExpectHashFlag(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)
	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	h := st.GetNode(nid("1")).ContentHash

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "--dir", dir, "--expect-hash", h})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("accept --expect-hash failed: %v\n%s", err, buf.String())
	}

	st, err = svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	n := st.GetNode(nid("1"))
	if n.ValidatedContentHash != h {
		t.Errorf("ValidatedContentHash = %q, want %q", n.ValidatedContentHash, h)
	}
	if !n.ValidatedHashChecked {
		t.Error("ValidatedHashChecked = false, want true")
	}
}

func TestAcceptCmd_ExpectHashMismatchRefused(t *testing.T) {
	dir, svc := setupTaintTraceTest(t)

	cmd := newAcceptCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"1", "--dir", dir, "--expect-hash", "deadbeef"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected a mismatch error, got nil\n%s", buf.String())
	}

	st, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.GetNode(nid("1")).EpistemicState == "validated" {
		t.Error("node was validated despite a hash mismatch")
	}
}
