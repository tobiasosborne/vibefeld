// Package export provides proof export functionality to various formats.
package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/state"
)

// GraphSchemaVersion is the current schema version of the `af export --graph
// json` projection document (rk PRD C5's join target). Bump this string and
// add a fixture in graph_test.go whenever the document shape changes in a
// way an existing consumer would need to account for.
const GraphSchemaVersion = "1"

// ValidateGraphFormat checks if the given graph export format string is
// valid. Valid formats: json (case-insensitive). This validates the
// `--graph` flag, which is a separate flag from `--format` (markdown/latex).
func ValidateGraphFormat(format string) error {
	f := strings.ToLower(format)
	if f != "json" {
		return fmt.Errorf("invalid graph format %q: must be one of: json", format)
	}
	return nil
}

// GraphWorkspace identifies the proof workspace a graph document was
// exported from. ID is supplied by the caller (the resolved --dir path) and
// is the value rk's registry locates via its shard's `workspace:` field
// (PRD C5) before byte-matching contract text against node statements here.
type GraphWorkspace struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Conjecture string `json:"conjecture,omitempty"`
}

// GraphNode is a single proof node in the graph export. Statement (and
// Latex, if present) is the exact contract text used for byte-match joins
// against the registry — it is never reformatted or truncated.
type GraphNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// ParentID is empty for the root node.
	ParentID string `json:"parent_id,omitempty"`
	// ChildIDs is sorted in hierarchical-ID order; omitted for leaf nodes.
	ChildIDs  []string `json:"child_ids,omitempty"`
	Statement string   `json:"statement"`
	Latex     string   `json:"latex,omitempty"`
	Inference string   `json:"inference"`
	// ContentHash is the node's own SHA256 content hash (type/statement/
	// latex/inference/context/dependencies), independent of this export.
	ContentHash string `json:"content_hash"`
	// The three recorded epistemic axes (PRD/CLAUDE.md "three-axis state").
	WorkflowState  string `json:"workflow_state"`
	EpistemicState string `json:"epistemic_state"`
	TaintState     string `json:"taint_state"`
	// Crux marks nodes on the critical path (PRD C9 batch-exclusion rule).
	Crux bool `json:"crux,omitempty"`
	// Created is the node's ledger-recorded creation time, not an
	// export-time timestamp; it is part of the node's identity-bearing
	// content and reproduces byte-identically across repeated exports.
	Created string `json:"created"`
}

// GraphValidation summarizes validation-relevant events cheaply derivable
// from already-computed state (node and challenge tallies), without
// re-walking the raw ledger.
type GraphValidation struct {
	TotalNodes            int            `json:"total_nodes"`
	EpistemicCounts       map[string]int `json:"epistemic_counts"`
	TaintCounts           map[string]int `json:"taint_counts"`
	TotalChallenges       int            `json:"total_challenges"`
	ChallengeStatusCounts map[string]int `json:"challenge_status_counts"`
}

// GraphExport is the top-level document produced by `af export --graph
// json`. SchemaVersion is D6's explicit compat marker: rk's M2.2 store
// reader checks this field before parsing the rest of the document.
type GraphExport struct {
	SchemaVersion string          `json:"schema_version"`
	Workspace     GraphWorkspace  `json:"workspace"`
	Nodes         []GraphNode     `json:"nodes"`
	Validation    GraphValidation `json:"validation"`
}

// BuildGraphExport builds the deterministic in-memory GraphExport document
// for the given state. workspaceID identifies the workspace (typically the
// resolved --dir path); cfg may be nil, in which case Title/Conjecture are
// left empty. s may be nil (uninitialized proof), in which case an empty,
// still schema-versioned document is returned.
func BuildGraphExport(s *state.State, workspaceID string, cfg *config.Config) GraphExport {
	ge := GraphExport{
		SchemaVersion: GraphSchemaVersion,
		Workspace:     GraphWorkspace{ID: workspaceID},
		Nodes:         []GraphNode{},
		Validation: GraphValidation{
			EpistemicCounts:       map[string]int{},
			TaintCounts:           map[string]int{},
			ChallengeStatusCounts: map[string]int{},
		},
	}

	if cfg != nil {
		ge.Workspace.Title = cfg.Title
		ge.Workspace.Conjecture = cfg.Conjecture
	}

	if s == nil {
		return ge
	}

	// sortNodesByID is defined in export.go (same package) and guarantees
	// stable hierarchical-ID ordering, the backbone of this export's
	// determinism.
	nodes := sortNodesByID(s.AllNodes())

	// child_ids per parent, built from the already-sorted node list so each
	// parent's children slice comes out in hierarchical-ID order too.
	childrenOf := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		if parentID, ok := n.ID.Parent(); ok {
			pStr := parentID.String()
			childrenOf[pStr] = append(childrenOf[pStr], n.ID.String())
		}
	}

	ge.Nodes = make([]GraphNode, 0, len(nodes))
	for _, n := range nodes {
		gn := GraphNode{
			ID:             n.ID.String(),
			Type:           string(n.Type),
			Statement:      n.Statement,
			Latex:          n.Latex,
			Inference:      string(n.Inference),
			ContentHash:    n.ContentHash,
			WorkflowState:  string(n.WorkflowState),
			EpistemicState: string(n.EpistemicState),
			TaintState:     string(n.TaintState),
			Crux:           n.Crux,
			Created:        n.Created.String(),
		}
		if parentID, ok := n.ID.Parent(); ok {
			gn.ParentID = parentID.String()
		}
		if kids := childrenOf[gn.ID]; len(kids) > 0 {
			gn.ChildIDs = kids
		}

		ge.Nodes = append(ge.Nodes, gn)

		ge.Validation.EpistemicCounts[string(n.EpistemicState)]++
		ge.Validation.TaintCounts[string(n.TaintState)]++
	}
	ge.Validation.TotalNodes = len(nodes)

	for _, c := range s.AllChallenges() {
		if c == nil {
			continue
		}
		ge.Validation.TotalChallenges++
		ge.Validation.ChallengeStatusCounts[c.Status]++
	}

	return ge
}

// ExportGraph renders the graph export document as a deterministic JSON
// string. Determinism rests on three properties: (1) nodes are emitted in
// stable hierarchical-ID order (sortNodesByID), never map-iteration order;
// (2) all object keys come either from fixed struct field order or from
// Go's encoding/json, which always sorts map[string]... keys alphabetically
// on marshal; (3) no field carries an export-time (wall-clock) timestamp —
// Created is the node's ledger-recorded creation time, which is part of the
// content being exported, not export metadata. Two calls against unchanged
// state therefore produce byte-identical output.
func ExportGraph(s *state.State, workspaceID string, cfg *config.Config) (string, error) {
	ge := BuildGraphExport(s, workspaceID, cfg)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ge); err != nil {
		return "", fmt.Errorf("failed to marshal graph export: %w", err)
	}

	b := buf.Bytes()
	// Encode appends a trailing newline; strip it for consistency with
	// ToMarkdown/ToLaTeX, whose callers control their own trailing newlines.
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return string(b), nil
}
