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

	// Dependencies lists the NodeIDs this node depends on.
	Dependencies []types.NodeID `json:"dependencies,omitempty"`

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
	Latex        string
	Context      []string
	Dependencies []types.NodeID
	Scope        []string
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

	// Validate inference type
	if _, ok := schema.GetInferenceInfo(inference); !ok {
		return nil, errors.New("invalid inference type")
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
		WorkflowState:  schema.WorkflowAvailable,
		EpistemicState: schema.EpistemicPending,
		TaintState:     TaintUnresolved,
		Created:        types.Now(),
		Scope:          opts.Scope,
	}

	// Compute content hash
	node.ContentHash = node.ComputeContentHash()

	return node, nil
}

// ComputeContentHash computes the SHA256 hash of the node's content fields.
// The hash is computed from: type, statement, latex, inference, context, dependencies.
// Context and dependencies are sorted for deterministic ordering.
func (n *Node) ComputeContentHash() string {
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
		depStrings := make([]string, len(n.Dependencies))
		for i, dep := range n.Dependencies {
			depStrings[i] = dep.String()
		}
		sort.Strings(depStrings)
		sb.WriteString("|dependencies:")
		sb.WriteString(strings.Join(depStrings, ","))
	}

	// Compute hash from the built string
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// Validate checks if the Node is valid.
// Returns an error describing the validation failure, or nil if valid.
func (n *Node) Validate() error {
	// Check statement
	if strings.TrimSpace(n.Statement) == "" {
		return errors.New("node statement cannot be empty")
	}

	// Check node type
	if _, ok := schema.GetNodeTypeInfo(n.Type); !ok {
		return errors.New("invalid node type")
	}

	// Check inference type
	if _, ok := schema.GetInferenceInfo(n.Inference); !ok {
		return errors.New("invalid inference type")
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
func (n *Node) IsRoot() bool {
	return n.ID.IsRoot()
}

// Depth returns the depth of this node in the tree.
func (n *Node) Depth() int {
	return n.ID.Depth()
}

// VerifyContentHash returns true if the stored content hash matches
// the computed hash of the current content.
func (n *Node) VerifyContentHash() bool {
	return n.ContentHash == n.ComputeContentHash()
}
