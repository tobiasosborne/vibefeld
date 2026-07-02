// Package main contains tests for the af jobs --ready filter wiring
// (vibefeld-r0k9). Behavioral coverage of the underlying predicate lives in
// internal/jobs (AllChildrenCleared / FilterReadyVerifierJobs); these tests
// cover the CLI flag surface.
package main

import (
	"strings"
	"testing"
)

// TestJobsCmd_HasReadyFlag verifies the --ready flag is registered.
func TestJobsCmd_HasReadyFlag(t *testing.T) {
	cmd := newJobsCmd()
	if cmd.Flags().Lookup("ready") == nil {
		t.Error("expected jobs command to have a --ready flag")
	}
}

// TestJobsCmd_HelpMentionsReady verifies the help documents --ready.
func TestJobsCmd_HelpMentionsReady(t *testing.T) {
	cmd := newJobsCmd()
	if !strings.Contains(cmd.Long, "--ready") {
		t.Error("expected jobs command Long help to document --ready")
	}
}
