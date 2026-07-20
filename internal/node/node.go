// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// TaintState represents the taint status of a node.
type TaintState string

const (
	TaintClean        TaintState = "clean"
	TaintSelfAdmitted TaintState = "self_admitted"
	TaintTainted      TaintState = "tainted"
	TaintUnresolved   TaintState = "unresolved"
)

// Node represents a proof node in the AF framework.
// Nodes form a hierarchical tree structure where each node
// represents a step in a mathematical proof.
type Node struct {
	// ID is the hierarchical identifier for this node (e.g., "1", "1.2", "1.2.3").
	ID types.NodeID `json:"id"`

	// Type is the type of this proof node (claim, local_assume, etc.).
	Type schema.NodeType `json:"type"`

	// Statement is the mathematical assertion or claim text.
	Statement string `json:"statement"`

	// Latex is the optional LaTeX representation of the statement.
	Latex string `json:"latex,omitempty"`

	// Inference is the inference rule used to justify this node.
	Inference schema.InferenceType `json:"inference"`

	// Context contains references to definitions, assumptions, externals used.
	Context []string `json:"context,omitempty"`

	// Dependencies lists the NodeIDs this node depends on (reference dependencies).
	Dependencies []types.NodeID `json:"dependencies,omitempty"`

	// ValidationDeps lists the NodeIDs that must be validated before this node
	// can be accepted. This enables cross-branch dependency tracking.
	ValidationDeps []types.NodeID `json:"validation_deps,omitempty"`

	// WorkflowState is the current workflow state (available, claimed, blocked).
	WorkflowState schema.WorkflowState `json:"workflow_state"`

	// EpistemicState is the epistemic verification state (pending, validated, etc.).
	EpistemicState schema.EpistemicState `json:"epistemic_state"`

	// TaintState represents the taint propagation status.
	TaintState TaintState `json:"taint_state"`

	// ContentHash is the SHA256 hash of the node's content fields.
	ContentHash string `json:"content_hash"`

	// Created is the timestamp when this node was created.
	Created types.Timestamp `json:"created"`

	// Scope contains the scope entries active at this node.
	Scope []string `json:"scope,omitempty"`

	// ClaimedBy is the agent ID that currently holds the claim (if any).
	ClaimedBy string `json:"claimed_by,omitempty"`

	// ClaimedAt is the timestamp when the node was claimed.
	ClaimedAt types.Timestamp `json:"claimed_at,omitempty"`

	// Crux marks this node as critical path — it cannot be validated
	// without a passing claim-test.
	Crux bool `json:"crux,omitempty"`

	// Author is the identity of the agent that authored this node's content,
	// recorded at creation time (or, for the root node, the proof's --author).
	// This is DRIVER-SUPPLIED PROVENANCE, not adversary-proof enforcement: af
	// records whatever identity string the caller passes and never verifies
	// it against any external credential. It exists so that reviewer≠author
	// checks (rk PRD C3, `af verdicts apply`) have something recorded and
	// mechanically checkable to compare against — the actual trust anchor
	// remains the driver's process discipline, same as ClaimedBy always was.
	// Empty for nodes created before this field existed, or created without
	// an author supplied; old ledgers replay identically (this is read as a
	// zero value, never required).
	Author string `json:"author,omitempty"`

	// ValidatedBy is the identity of the verifier who validated this node
	// (recorded from the NodeValidated event that moved it to the validated
	// epistemic state). Same provenance caveat as Author: driver-supplied,
	// recorded-and-checkable, not adversary-proof. Cleared if the node is
	// later unvalidated. Empty for nodes validated before this field existed.
	ValidatedBy string `json:"validated_by,omitempty"`

	// ValidationBatchID is the batch identifier recorded on the NodeValidated
	// event that validated this node, if the validation was applied as part
	// of a batch (rk PRD C3's batched verification mode, `af verdicts
	// apply`). Empty for singly-validated nodes and for nodes validated
	// before this field existed. Cleared if the node is later unvalidated.
	ValidationBatchID string `json:"validation_batch_id,omitempty"`

	// ProofAuthor is the identity of the prover that RECORDED THE PROOF of this
	// node — i.e. decomposed it into children via `af record-proof` (recorded
	// from the NodeProofAuthored event that fires as part of record-proof's
	// atomic write). It is DELIBERATELY DISTINCT from Author: Author records who
	// authored the node's own content (for a root, the `af init` --author; for a
	// child, the prover that CREATED it), whereas ProofAuthor records who proved
	// THIS node by decomposing it. The two coincide for a prover-created child it
	// later re-proves, but diverge for the ROOT: a root's Author is the init
	// author (often an unparseable orchestration stamp), while its ProofAuthor is
	// the prover-of-record that actually decomposed it.
	//
	// It exists so a reviewer≠author / cross-vendor check (rk PRD C9,
	// `af verdicts apply`) can compare the DECOMPOSER's family against the
	// verifier's when validating a decomposed node — the decomposition IS the
	// prover's proof of the parent, symmetric with the family stamp its children
	// already carry (rk GAP 9). Same DRIVER-SUPPLIED PROVENANCE caveat as Author:
	// recorded-and-checkable, not adversary-proof. record-proof sets it to the
	// acting prover on every decomposition (a later re-decomposition after a
	// challenge updates it to the new decomposer); it NEVER touches Author.
	// Empty for nodes never decomposed, and for nodes decomposed before this
	// field existed — old ledgers replay identically (read as a zero value,
	// never required).
	ProofAuthor string `json:"proof_author,omitempty"`
}

// NewNode creates a new Node with the given parameters.
// It computes the content hash automatically.
// Returns an error if validation fails.
func NewNode(
	id types.NodeID,
	nodeType schema.NodeType,
	statement string,
	inference schema.InferenceType,
) (*Node, error) {
	return NewNodeWithOptions(id, nodeType, statement, inference, NodeOptions{})
}

// NodeOptions contains optional parameters for node creation.
type NodeOptions struct {
	Latex          string
	Context        []string
	Dependencies   []types.NodeID
	ValidationDeps []types.NodeID
	Scope          []string
	Draft          bool // If true, node starts in draft state instead of pending
	Crux           bool // If true, node requires passing claim-test before acceptance

	// Author records the identity of the agent authoring this node's
	// content, if the caller supplies one. Driver-supplied provenance, not
	// adversary-proof enforcement; see Node.Author.
	Author string
}

// NewNodeWithOptions creates a new Node with the given parameters and options.
// It computes the content hash automatically.
// Returns an error if validation fails.
func NewNodeWithOptions(
	id types.NodeID,
	nodeType schema.NodeType,
	statement string,
	inference schema.InferenceType,
	opts NodeOptions,
) (*Node, error) {
	// Validate statement is not empty
	if strings.TrimSpace(statement) == "" {
		return nil, errors.New("node statement cannot be empty")
	}

	// Validate node type
	if _, ok := schema.GetNodeTypeInfo(nodeType); !ok {
		return nil, errors.New("invalid node type")
	}

	// Validate inference/justification: a free-text derivation label; any
	// non-blank string is accepted and stored verbatim (registry values keep
	// their identity but are not required). Blank is rejected.
	if strings.TrimSpace(string(inference)) == "" {
		return nil, errors.New("node inference cannot be blank")
	}

	// Create the node
	node := &Node{
		ID:             id,
		Type:           nodeType,
		Statement:      statement,
		Latex:          opts.Latex,
		Inference:      inference,
		Context:        opts.Context,
		Dependencies:   opts.Dependencies,
		ValidationDeps: opts.ValidationDeps,
		WorkflowState:  schema.WorkflowAvailable,
		EpistemicState: schema.EpistemicPending,
		TaintState:     TaintUnresolved,
		Created:        types.Now(),
		Scope:          opts.Scope,
		Author:         opts.Author,
	}

	if opts.Draft {
		node.EpistemicState = schema.EpistemicDraft
	}
	if opts.Crux {
		node.Crux = true
	}

	// Compute content hash
	node.ContentHash = node.ComputeContentHash()

	return node, nil
}

// ComputeContentHash computes the SHA256 hash of the node's content fields.
// The hash is computed from: type, statement, latex, inference, context, dependencies.
// Context and dependencies are sorted for deterministic ordering.
// Returns an empty string if the node is nil.
//
// Deliberately excluded: WorkflowState, EpistemicState, TaintState,
// ClaimedBy/ClaimedAt, Crux, Scope, and (as of the author/verifier-identity
// schema addition) Author, ValidatedBy, ValidationBatchID, ProofAuthor. These are
// workflow/provenance metadata, not mathematical content — the same
// exclusion rationale that already applied to ClaimedBy. Excluding them
// keeps ComputeContentHash, and therefore VerifyContentHash and `af replay
// --verify`, byte-identical on the historical ledger corpus regardless of
// whether these new fields are populated.
func (n *Node) ComputeContentHash() string {
	if n == nil {
		return ""
	}
	// Build a deterministic string representation of content fields using strings.Builder
	var sb strings.Builder

	// Add type
	sb.WriteString("type:")
	sb.WriteString(string(n.Type))

	// Add statement
	sb.WriteString("|statement:")
	sb.WriteString(n.Statement)

	// Add latex if present
	if n.Latex != "" {
		sb.WriteString("|latex:")
		sb.WriteString(n.Latex)
	}

	// Add inference
	sb.WriteString("|inference:")
	sb.WriteString(string(n.Inference))

	// Add sorted context
	if len(n.Context) > 0 {
		sortedContext := make([]string, len(n.Context))
		copy(sortedContext, n.Context)
		sort.Strings(sortedContext)
		sb.WriteString("|context:")
		sb.WriteString(strings.Join(sortedContext, ","))
	}

	// Add sorted dependencies
	if len(n.Dependencies) > 0 {
		depStrings := types.ToStringSlice(n.Dependencies)
		sort.Strings(depStrings)
		sb.WriteString("|dependencies:")
		sb.WriteString(strings.Join(depStrings, ","))
	}

	// Add sorted validation dependencies
	if len(n.ValidationDeps) > 0 {
		valDepStrings := types.ToStringSlice(n.ValidationDeps)
		sort.Strings(valDepStrings)
		sb.WriteString("|validation_deps:")
		sb.WriteString(strings.Join(valDepStrings, ","))
	}

	// Compute hash from the built string
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// Validate checks if the Node is valid.
// Returns an error describing the validation failure, or nil if valid.
func (n *Node) Validate() error {
	if n == nil {
		return errors.New("nil node")
	}

	// Check statement
	if strings.TrimSpace(n.Statement) == "" {
		return errors.New("node statement cannot be empty")
	}

	// Check node type
	if _, ok := schema.GetNodeTypeInfo(n.Type); !ok {
		return errors.New("invalid node type")
	}

	// Check inference/justification: free-text label, non-blank required
	// (registry membership not required; see schema.ValidateJustification).
	if strings.TrimSpace(string(n.Inference)) == "" {
		return errors.New("node inference cannot be blank")
	}

	// Check workflow state
	if _, ok := schema.GetWorkflowStateInfo(n.WorkflowState); !ok {
		return errors.New("invalid workflow state")
	}

	// Check epistemic state
	if _, ok := schema.GetEpistemicStateInfo(n.EpistemicState); !ok {
		return errors.New("invalid epistemic state")
	}

	return nil
}

// IsRoot returns true if this is the root node (ID is "1").
// Returns false if the node is nil.
func (n *Node) IsRoot() bool {
	if n == nil {
		return false
	}
	return n.ID.IsRoot()
}

// Depth returns the depth of this node in the tree.
// Returns 0 if the node is nil.
func (n *Node) Depth() int {
	if n == nil {
		return 0
	}
	return n.ID.Depth()
}

// VerifyContentHash returns true if the stored content hash matches
// the computed hash of the current content.
// Returns false if the node is nil.
func (n *Node) VerifyContentHash() bool {
	if n == nil {
		return false
	}
	return n.ContentHash == n.ComputeContentHash()
}
