package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/node"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

func archiveChecklistNode(t *testing.T, id string, es schema.EpistemicState) *node.Node {
	t.Helper()
	nodeID, err := types.Parse(id)
	if err != nil {
		t.Fatalf("parse %q: %v", id, err)
	}
	n, err := node.NewNode(nodeID, schema.NodeTypeClaim, "step "+id, schema.InferenceModusPonens)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	n.EpistemicState = es
	return n
}

// D9: the parent's checklist surfaces a child archived with an open challenge
// so the next accept acknowledges the abandoned obligation.
func TestChecklist_ListsArchivedChildWithAbandonedChallenge(t *testing.T) {
	st := state.NewState()
	parent := archiveChecklistNode(t, "1", schema.EpistemicPending)
	child := archiveChecklistNode(t, "1.1", schema.EpistemicArchived)
	st.AddNode(parent)
	st.AddNode(child)
	st.AddChallenge(&state.Challenge{
		ID:       "ch-abandoned",
		NodeID:   child.ID,
		Status:   state.ChallengeStatusSuperseded,
		Severity: "major",
		Target:   "statement",
		Reason:   "unanswered",
	})

	text := RenderVerificationChecklist(parent, st)
	if !strings.Contains(text, "ABANDONED OBLIGATIONS") {
		t.Errorf("checklist missing abandoned-obligations section:\n%s", text)
	}
	if !strings.Contains(text, child.ID.String()) {
		t.Errorf("checklist missing archived child %s:\n%s", child.ID, text)
	}

	data := RenderVerificationChecklistJSON(parent, st)
	var parsed struct {
		ArchiveObligations []struct {
			NodeID string `json:"node_id"`
		} `json:"archive_obligations"`
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("unmarshal checklist JSON: %v (%s)", err, data)
	}
	if len(parsed.ArchiveObligations) != 1 || parsed.ArchiveObligations[0].NodeID != child.ID.String() {
		t.Errorf("archive_obligations = %+v, want [%s]", parsed.ArchiveObligations, child.ID)
	}
}

// D9: a descendant-only obligation (challenge on 1.1.1 abandoned by archiving
// 1.1) is read from the durable snapshot on 1.1's NodeArchived event and still
// surfaces on 1's checklist.
func TestChecklist_ListsDescendantAbandonedObligation(t *testing.T) {
	st := state.NewState()
	parent := archiveChecklistNode(t, "1", schema.EpistemicPending)
	child := archiveChecklistNode(t, "1.1", schema.EpistemicArchived)
	grandchild := archiveChecklistNode(t, "1.1.1", schema.EpistemicPending)
	child.AbandonedObligations = []string{"1.1.1"}
	st.AddNode(parent)
	st.AddNode(child)
	st.AddNode(grandchild)

	text := RenderVerificationChecklist(parent, st)
	if !strings.Contains(text, "ABANDONED OBLIGATIONS") {
		t.Errorf("checklist missing abandoned-obligations section:\n%s", text)
	}
	if !strings.Contains(text, "1.1.1") {
		t.Errorf("checklist missing descendant obligation 1.1.1:\n%s", text)
	}

	data := RenderVerificationChecklistJSON(parent, st)
	var parsed struct {
		ArchiveObligations []struct {
			NodeID string `json:"node_id"`
		} `json:"archive_obligations"`
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("unmarshal checklist JSON: %v (%s)", err, data)
	}
	if len(parsed.ArchiveObligations) != 1 || parsed.ArchiveObligations[0].NodeID != "1.1.1" {
		t.Errorf("archive_obligations = %+v, want [1.1.1]", parsed.ArchiveObligations)
	}
}

// A child archived with no challenge trace is not listed.
func TestChecklist_OmitsArchivedChildWithoutChallenge(t *testing.T) {
	st := state.NewState()
	parent := archiveChecklistNode(t, "1", schema.EpistemicPending)
	child := archiveChecklistNode(t, "1.1", schema.EpistemicArchived)
	st.AddNode(parent)
	st.AddNode(child)

	text := RenderVerificationChecklist(parent, st)
	if strings.Contains(text, "ABANDONED OBLIGATIONS") {
		t.Errorf("checklist should not list a challenge-free archived child:\n%s", text)
	}
}

// A resolved challenge on an archived child is not an abandoned obligation.
func TestChecklist_OmitsResolvedChallengeOnArchivedChild(t *testing.T) {
	st := state.NewState()
	parent := archiveChecklistNode(t, "1", schema.EpistemicPending)
	child := archiveChecklistNode(t, "1.1", schema.EpistemicArchived)
	st.AddNode(parent)
	st.AddNode(child)
	st.AddChallenge(&state.Challenge{
		ID:       "ch-resolved",
		NodeID:   child.ID,
		Status:   state.ChallengeStatusResolved,
		Severity: "major",
	})

	text := RenderVerificationChecklist(parent, st)
	if strings.Contains(text, "ABANDONED OBLIGATIONS") {
		t.Errorf("resolved challenge should not be an abandoned obligation:\n%s", text)
	}
}
