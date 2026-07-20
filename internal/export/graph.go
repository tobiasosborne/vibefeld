// Package export provides proof export functionality to various formats.
package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/node"
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
	// Author is the driver-supplied identity of the agent that authored
	// this node's content (rk PRD C3's author-identity kernel surface),
	// omitted if never recorded. Added under v1's future-additive rule
	// (docs/export-graph-v1.md): a purely additive optional field, no
	// schema_version bump.
	Author string `json:"author,omitempty"`
	// ValidatedBy is the driver-supplied identity of the verifier who
	// validated this node, omitted if the node isn't validated or was
	// validated before this field existed.
	ValidatedBy string `json:"validated_by,omitempty"`
	// ValidationBatchID is the batch id recorded when this node was
	// validated as part of a batch (`af verdicts apply`), omitted for
	// singly-validated nodes.
	ValidationBatchID string `json:"validation_batch_id,omitempty"`
	// ProofAuthor is the driver-supplied identity of the prover that PROVED
	// this node by decomposing it (`af record-proof`), distinct from Author
	// (who authored the node's content). Omitted if the node was never
	// decomposed. An external driver's cross-vendor check (rk PRD C9) reads
	// this as the prover-of-record for a decomposed node — a root's Author is
	// its `af init` stamp, but its ProofAuthor is the prover that decomposed it
	// (rk GAP 9). Added under v1's future-additive rule (advertised via the
	// FeatureProofAuthor capability token): a purely additive optional field,
	// no schema_version bump.
	ProofAuthor string `json:"proof_author,omitempty"`
	// ProverReady is true iff af's OWN job detection (internal/jobs.FindJobs,
	// the same classifier `af jobs --role prover` uses) marks this node as a
	// prover job: not blocked, and either pending with an open blocking
	// (critical/major) challenge, or in draft/needs_refinement. An external
	// driver (rk) reads this instead of re-deriving af's job state machine.
	// Added under v1's additive-fields rule: optional (omitempty), no
	// schema_version bump. Omitted (false) for nodes needing no prover work.
	ProverReady bool `json:"prover_ready,omitempty"`
	// VerifierReady is true iff af's OWN job detection marks this node ready
	// for verifier review AND all its children are epistemically cleared —
	// exactly internal/jobs.FilterReadyVerifierJobs, the bottom-up-ready filter
	// `af jobs --role verifier --ready` uses (a statement, pending, available,
	// no open blocking challenge, every child validated/admitted/archived). A
	// driver dispatches a verifier turn on these and only these. Same additive-
	// field note. Omitted (false) when the node is not review-ready.
	VerifierReady bool `json:"verifier_ready,omitempty"`
	// Dependencies lists this node's reference dependency IDs (the nodes its
	// reasoning cites, e.g. "by step 1.2") in hierarchical-ID order, exactly
	// node.Node.Dependencies. Emitted so an external verifier judges a node
	// against the SAME dependency set the prover recorded, rather than a
	// children-substitution proxy. Additive field (omitempty), no
	// schema_version bump; omitted for a node with no recorded dependencies.
	Dependencies []string `json:"dependencies,omitempty"`
	// Closed is true iff the subtree rooted at this node is settled — its own
	// epistemic state is cleared (validated/admitted/archived), it is not
	// blocked, it carries no open blocking challenge, and every descendant is
	// itself closed (see computeClosedSet). This is the authoritative
	// convergence signal a driver reads on the ROOT: it goes false the instant
	// a blocking challenge lands on an already-validated node, which the bare
	// epistemic_state axis does not reflect. Additive field (omitempty), no
	// schema_version bump; omitted (false) for any not-yet-closed node.
	Closed bool `json:"closed,omitempty"`
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
	// Features is the fixed capability list this af build advertises
	// (GraphFeatures). Always present (never omitempty) so an external driver
	// can detect an af too old to emit a capability it depends on: an older af
	// omits this array entirely, and the driver fails loudly at preflight
	// rather than misreading an absent omitempty flag as "nothing ready".
	Features   []string        `json:"features"`
	Workspace  GraphWorkspace  `json:"workspace"`
	Nodes      []GraphNode     `json:"nodes"`
	Validation GraphValidation `json:"validation"`
}

// BuildGraphExport builds the deterministic in-memory GraphExport document
// for the given state. workspaceID identifies the workspace (typically the
// resolved --dir path); cfg may be nil, in which case Title/Conjecture are
// left empty. s may be nil (uninitialized proof), in which case an empty,
// still schema-versioned document is returned.
func BuildGraphExport(s *state.State, workspaceID string, cfg *config.Config) GraphExport {
	ge := GraphExport{
		SchemaVersion: GraphSchemaVersion,
		Features:      GraphFeatures,
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

	// Per-node readiness from af's OWN job detection (internal/jobs) — the same
	// classifier `af jobs` uses, so an external driver never re-derives af's
	// job state machine. proverReadySet = FindProverJobs; verifierReadySet =
	// FilterReadyVerifierJobs (verifier jobs whose children are all cleared, the
	// bottom-up-ready set a driver dispatches on). Built once over the full node
	// set; determinism is unaffected (these only set already-deterministic bool
	// fields on nodes already emitted in stable ID order).
	nodeMap := make(map[string]*node.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID.String()] = n
	}
	challengeMap := s.ChallengeMapForJobs()
	jr := jobs.FindJobs(nodes, nodeMap, challengeMap)
	proverReadySet := make(map[string]bool, len(jr.ProverJobs))
	for _, n := range jr.ProverJobs {
		proverReadySet[n.ID.String()] = true
	}
	verifierReadySet := make(map[string]bool, len(jr.VerifierJobs))
	for _, n := range jobs.FilterReadyVerifierJobs(jr.VerifierJobs, nodeMap) {
		verifierReadySet[n.ID.String()] = true
	}

	// Per-node bottom-up closure (the authoritative convergence signal); built
	// once over the full node set from the same challengeMap, deterministic.
	closedSet := computeClosedSet(nodes, nodeMap, challengeMap)

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
			ID:                n.ID.String(),
			Type:              string(n.Type),
			Statement:         n.Statement,
			Latex:             n.Latex,
			Inference:         string(n.Inference),
			ContentHash:       n.ContentHash,
			WorkflowState:     string(n.WorkflowState),
			EpistemicState:    string(n.EpistemicState),
			TaintState:        string(n.TaintState),
			Crux:              n.Crux,
			Created:           n.Created.String(),
			Author:            n.Author,
			ValidatedBy:       n.ValidatedBy,
			ValidationBatchID: n.ValidationBatchID,
			ProofAuthor:       n.ProofAuthor,
			ProverReady:       proverReadySet[n.ID.String()],
			VerifierReady:     verifierReadySet[n.ID.String()],
			Closed:            closedSet[n.ID.String()],
		}
		if len(n.Dependencies) > 0 {
			deps := make([]string, len(n.Dependencies))
			for i, d := range n.Dependencies {
				deps[i] = d.String()
			}
			gn.Dependencies = deps
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
