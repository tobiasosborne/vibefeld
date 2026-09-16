package main

import (
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func TestAnalyzeSupportHealth_FlagsNotCurrent(t *testing.T) {
	st := state.NewState()
	rootID, _ := types.Parse("1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "root", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	root.EpistemicState = schema.EpistemicValidated
	root.VerdictSeq = 2
	st.AddNode(root)

	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	child.EpistemicState = schema.EpistemicPending
	st.AddNode(child)

	status, blockers := analyzeSupportHealth(st, HealthStatusHealthy, nil)
	if status != HealthStatusWarning {
		t.Errorf("status = %q, want warning", status)
	}
	if len(blockers) != 1 {
		t.Fatalf("blockers = %+v, want 1", blockers)
	}
	b := blockers[0]
	if !strings.Contains(b.Type, "TARGET_PENDING") {
		t.Errorf("blocker type = %q, want cause TARGET_PENDING", b.Type)
	}
	if len(b.NodeIDs) != 1 || b.NodeIDs[0] != "1" {
		t.Errorf("blocker node ids = %v, want [1]", b.NodeIDs)
	}
}

func TestAnalyzeSupportHealth_HealthyWhenCurrent(t *testing.T) {
	st := state.NewState()
	rootID, _ := types.Parse("1")
	root, err := node.NewNode(rootID, schema.NodeTypeClaim, "root", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	root.EpistemicState = schema.EpistemicValidated
	root.VerdictSeq = 2
	st.AddNode(root)
	// A validated leaf child makes the tree current.
	childID, _ := types.Parse("1.1")
	child, err := node.NewNode(childID, schema.NodeTypeClaim, "child", schema.InferenceModusPonens)
	if err != nil {
		t.Fatal(err)
	}
	child.EpistemicState = schema.EpistemicValidated
	child.VerdictSeq = 1
	st.AddNode(child)

	status, blockers := analyzeSupportHealth(st, HealthStatusHealthy, nil)
	if status != HealthStatusHealthy || len(blockers) != 0 {
		t.Fatalf("expected healthy, got status %q blockers %+v", status, blockers)
	}
}
