package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/service"
)

// TestHandleRefineError_DistinguishesNotClaimedFromOwnerMismatch is a
// regression test: service.ErrNotClaimed and service.ErrOwnerMismatch share
// the NOT_CLAIM_HOLDER code, and AFError.Is compares codes only, so
// errors.Is matched both and a wrong-owner refine was reported as "parent node
// is not claimed".
func TestHandleRefineError_DistinguishesNotClaimedFromOwnerMismatch(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    string
		notWant string
	}{
		{
			name:    "not claimed (wrapped, as Refine returns it)",
			err:     fmt.Errorf("%w: parent node must be claimed", service.ErrNotClaimed),
			want:    "parent node is not claimed",
			notWant: "owner does not match",
		},
		{
			name:    "owner mismatch (bare, as Refine returns it)",
			err:     service.ErrOwnerMismatch,
			want:    "owner does not match the claim owner for node 1",
			notWant: "not claimed",
		},
		{
			name:    "owner mismatch (wrapped, as ClaimNode-style callers return it)",
			err:     fmt.Errorf("%w: node is claimed by a, not b", service.ErrOwnerMismatch),
			want:    "owner does not match the claim owner for node 1",
			notWant: "not claimed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handleRefineError(tt.err, "1", "agent-b").Error()
			if !strings.Contains(got, tt.want) {
				t.Errorf("handleRefineError() = %q, want it to contain %q", got, tt.want)
			}
			if strings.Contains(got, tt.notWant) {
				t.Errorf("handleRefineError() = %q, must not contain %q", got, tt.notWant)
			}
		})
	}
}

// TestHandleRefineError_HintUsesPositionalStatement checks the not-claimed hint
// shows the current refine syntax (the -s/--statement flag was removed).
func TestHandleRefineError_HintUsesPositionalStatement(t *testing.T) {
	got := handleRefineError(service.ErrNotClaimed, "1.2", "prover").Error()
	if strings.Contains(got, " -s ") {
		t.Errorf("hint uses removed -s flag: %q", got)
	}
	if !strings.Contains(got, `af refine 1.2 "..." -o prover`) {
		t.Errorf("hint should show positional statement syntax, got: %q", got)
	}
}

// TestHandleRefineError_PassesThroughOtherErrors checks unrelated errors are
// returned unchanged.
func TestHandleRefineError_PassesThroughOtherErrors(t *testing.T) {
	other := fmt.Errorf("%w: 1.9", service.ErrNodeNotFound)
	if got := handleRefineError(other, "1", "agent"); got != other {
		t.Errorf("handleRefineError() = %v, want the original error %v", got, other)
	}
}

// newDeepRefineFixture initializes a proof whose meta.json sets warn_depth=1
// and max_depth=2, with the root claimed by "prover". Refining the root
// creates a depth-2 node: past warn_depth, at max_depth.
func newDeepRefineFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := service.Init(dir, "Deep conjecture", "author"); err != nil {
		t.Fatal(err)
	}
	meta := `{"title":"Deep","conjecture":"Deep conjecture","lock_timeout":300000000000,` +
		`"max_depth":2,"warn_depth":1,"max_children":10,"auto_correct_threshold":0.8,` +
		`"version":"1.0","created":"2024-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := service.ParseNodeID("1")
	if err := svc.ClaimNode(root, "prover", time.Hour); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runRefineForTest(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := newRefineCmd()
	out, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// TestRefinePositional_WarnsPastWarnDepth is a regression test: since the
// --statement flag was replaced by positional statements (88dd188), a plain
// `af refine <id> "stmt"` goes through RefineNodeBulk, which skipped the
// warn_depth advisory that the single-statement path printed.
func TestRefinePositional_WarnsPastWarnDepth(t *testing.T) {
	dir := newDeepRefineFixture(t)

	stdout, stderr, err := runRefineForTest(t, "1", "Deep step", "-o", "prover", "-d", dir)
	if err != nil {
		t.Fatalf("refine unexpected error: %v", err)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "Warning: Creating node at depth 2") ||
		!strings.Contains(combined, "Consider adding siblings") {
		t.Errorf("expected depth warning, got stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestRefinePositional_DepthWarningKeepsJSONStdoutClean checks the advisory
// goes to stderr, so -f json output stays parseable.
func TestRefinePositional_DepthWarningKeepsJSONStdoutClean(t *testing.T) {
	dir := newDeepRefineFixture(t)

	stdout, stderr, err := runRefineForTest(t, "1", "Deep step", "-o", "prover", "-d", dir, "-f", "json")
	if err != nil {
		t.Fatalf("refine unexpected error: %v", err)
	}
	var v interface{}
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Errorf("stdout is not valid JSON: %v\nstdout=%q", err, stdout)
	}
	if !strings.Contains(stderr, "Warning: Creating node at depth 2") {
		t.Errorf("expected depth warning on stderr, got %q", stderr)
	}
}

// TestRefinePositional_MaxDepthErrorSuggestsBreadth is a regression test for
// the same refactor: exceeding max_depth lost its "add breadth instead" hint.
// The service error (and its DEPTH_EXCEEDED code) must be kept.
func TestRefinePositional_MaxDepthErrorSuggestsBreadth(t *testing.T) {
	dir := newDeepRefineFixture(t)
	if _, _, err := runRefineForTest(t, "1", "Depth two", "-o", "prover", "-d", dir); err != nil {
		t.Fatalf("refine at max depth unexpected error: %v", err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatal(err)
	}
	child, _ := service.ParseNodeID("1.1")
	if err := svc.ClaimNode(child, "prover", time.Hour); err != nil {
		t.Fatal(err)
	}

	_, _, err = runRefineForTest(t, "1.1", "Depth three", "-o", "prover", "-d", dir)
	if err == nil {
		t.Fatal("expected error past max depth, got nil")
	}
	if !strings.Contains(err.Error(), "add breadth instead") {
		t.Errorf("expected breadth hint, got: %q", err.Error())
	}
	if !errorsIsMaxDepth(err) {
		t.Errorf("expected error to still wrap service.ErrMaxDepthExceeded, got: %v", err)
	}
}

func errorsIsMaxDepth(err error) bool {
	return wrapsSentinel(err, service.ErrMaxDepthExceeded)
}
