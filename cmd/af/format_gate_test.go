//go:build !integration

package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// newFormatGateRoot builds a root command with the read/claim/replay commands
// used to exercise the workspace format gate.
func newFormatGateRoot() *cobra.Command {
	root := newTestRootCmd()
	root.AddCommand(newStatusCmd())
	root.AddCommand(newClaimCmd())
	root.AddCommand(newReplayCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newHistoryCmd())
	root.AddCommand(newWatchCmd())
	return root
}

func executeFormatGate(t *testing.T, root *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestFormatGate_UnreadableWorkspaceRefused(t *testing.T) {
	dir := setupFormatWorkspace(t, "9.9")
	root := newFormatGateRoot()

	tests := []struct {
		name string
		args []string
	}{
		{"status", []string{"status", "--dir", dir}},
		{"claim", []string{"claim", "1", "--owner", "o", "--dir", dir}},
		{"replay", []string{"replay", "--dir", dir}},
		{"log", []string{"log", "--dir", dir}},
		{"history", []string{"history", "1", "--dir", dir}},
		{"watch", []string{"watch", "--dir", dir, "--once"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeFormatGate(t, root, tt.args...)
			if err == nil {
				t.Fatalf("%s on a 9.9 workspace must be refused", tt.name)
			}
			if aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
				t.Errorf("%s error code = %v, want FORMAT_TOO_NEW", tt.name, aferrors.Code(err))
			}
			if aferrors.ExitCode(err) != 3 {
				t.Errorf("%s exit code = %d, want 3", tt.name, aferrors.ExitCode(err))
			}
		})
	}
}

func TestReplay_InvalidExitsCorruption(t *testing.T) {
	dir := setupFormatWorkspace(t, "1.1")

	// Append an event type replay does not know. The workspace format gate
	// passes (unknown types default to 1.0); replay itself fails, which must
	// surface as corruption (exit 4), not exit 0.
	ldg, err := ledger.NewLedger(filepath.Join(dir, "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ldg.Append(ledger.BaseEvent{EventType: "totally_unknown_event", EventTime: types.Now()}); err != nil {
		t.Fatal(err)
	}

	root := newFormatGateRoot()
	out, err := executeFormatGate(t, root, "replay", "--dir", dir, "-f", "json")
	if err == nil {
		t.Fatalf("invalid replay must return an error; output=%s", out)
	}
	if aferrors.Code(err) != aferrors.LEDGER_INCONSISTENT {
		t.Errorf("replay error code = %v, want LEDGER_INCONSISTENT", aferrors.Code(err))
	}
	if aferrors.ExitCode(err) != 4 {
		t.Errorf("replay exit code = %d, want 4", aferrors.ExitCode(err))
	}

	// The JSON payload must still be printed.
	if !bytes.Contains([]byte(out), []byte("\"valid\": false")) {
		t.Errorf("expected JSON payload with valid:false, got %q", out)
	}
}

func TestVersionCmd_FormatFlagJSON(t *testing.T) {
	cmd := newTestVersionCmd()
	output, err := executeVersionCommand(cmd, "version", "-f", "json")
	if err != nil {
		t.Fatalf("version -f json error = %v", err)
	}
	for _, key := range []string{`"format"`, `"policy"`, `"` + "1.1" + `"`} {
		if !bytes.Contains([]byte(output), []byte(key)) {
			t.Errorf("version -f json output missing %s: %s", key, output)
		}
	}
}

func TestVersionCmd_FormatFlagInvalid(t *testing.T) {
	cmd := newTestVersionCmd()
	if _, err := executeVersionCommand(cmd, "version", "-f", "xml"); err == nil {
		t.Error("version -f xml must error")
	}
}
