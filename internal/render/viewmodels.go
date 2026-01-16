// Package render provides human-readable formatting for AF framework types.
// This file defines view model types that decouple render from domain packages.
// The render package receives these view models instead of importing domain types.
package render

// NodeView is a view model representing a proof node for rendering.
// This decouples render from the node package.
type NodeView struct {
	ID             string   // Hierarchical ID (e.g., "1", "1.2", "1.2.3")
	Type           string   // Node type (claim, local_assume, etc.)
	Statement      string   // Mathematical assertion text
	Latex          string   // Optional LaTeX representation
	Inference      string   // Inference rule used
	WorkflowState  string   // available, claimed, blocked
	EpistemicState string   // pending, validated, admitted, refuted, archived
	TaintState     string   // clean, self_admitted, tainted, unresolved
	ContentHash    string   // SHA256 hash of content
	Created        string   // ISO8601 timestamp
	Context        []string // References to definitions, assumptions, externals
	Dependencies   []string // NodeIDs this node depends on
	ValidationDeps []string // NodeIDs that must be validated first
	Scope          []string // Scope entries active at this node
	ClaimedBy      string   // Agent ID holding the claim
	ClaimedAt      string   // When the node was claimed
	Depth          int      // Depth in the tree (root = 1)
}

// ChallengeView is a view model representing a challenge for rendering.
type ChallengeView struct {
	ID         string // Unique challenge identifier
	TargetID   string // Node ID being challenged
	Target     string // What aspect is challenged (statement, inference, etc.)
	TargetDesc string // Description of the challenge target
	Reason     string // Explanation of the challenge
	Status     string // open, resolved, withdrawn
	Severity   string // critical, major, minor, note
	Raised     string // ISO8601 timestamp when raised
	Resolution string // Resolution text (if resolved)
}

// DefinitionView is a view model representing a definition for rendering.
type DefinitionView struct {
	ID      string // Unique definition identifier
	Name    string // Human-readable name
	Content string // Definition content/body
}

// AssumptionView is a view model representing an assumption for rendering.
type AssumptionView struct {
	ID            string // Unique assumption identifier
	Statement     string // The assumption statement
	Justification string // Why this assumption is made
}

// ExternalView is a view model representing an external reference for rendering.
type ExternalView struct {
	ID     string // Unique external identifier
	Name   string // Human-readable name
	Source string // Source reference (book, paper, URL, etc.)
	Notes  string // Additional notes
}

// JobListView is a view model representing available jobs for rendering.
type JobListView struct {
	ProverJobs   []NodeView // Nodes needing prover attention
	VerifierJobs []NodeView // Nodes ready for verifier review
}

// IsEmpty returns true if there are no jobs of either type.
func (j *JobListView) IsEmpty() bool {
	return len(j.ProverJobs) == 0 && len(j.VerifierJobs) == 0
}

// TotalCount returns the total number of jobs across both types.
func (j *JobListView) TotalCount() int {
	return len(j.ProverJobs) + len(j.VerifierJobs)
}

// StatusView is a view model for rendering proof status.
type StatusView struct {
	Nodes            []NodeView
	Challenges       []ChallengeView
	ProverJobCount   int
	VerifierJobCount int
}

// ProverContextView is a view model for rendering prover context.
type ProverContextView struct {
	Node        NodeView
	Parent      *NodeView        // nil if root
	Siblings    []NodeView       // sibling nodes
	Dependencies []NodeView      // dependency nodes
	Definitions []DefinitionView // definitions in scope
	Assumptions []AssumptionView // assumptions in scope
	Externals   []ExternalView   // externals in scope
	Challenges  []ChallengeView  // challenges on this node
}

// VerifierContextView is a view model for rendering verifier context.
type VerifierContextView struct {
	Challenge   ChallengeView
	Node        NodeView          // The challenged node
	Parent      *NodeView         // Parent of challenged node (nil if root)
	Siblings    []NodeView        // Sibling nodes
	Dependencies []NodeView       // Dependency nodes
	Definitions []DefinitionView  // definitions in scope
	Assumptions []AssumptionView  // assumptions in scope
	Externals   []ExternalView    // externals in scope
}

// TreeView is a view model for rendering a proof tree.
type TreeView struct {
	Root       *NodeView   // The root node to render (nil renders all roots)
	Nodes      []NodeView  // All nodes in the tree
	NodeLookup map[string]NodeView // Quick lookup by ID string
}

// SearchResultView is a view model representing a node match from a search query.
type SearchResultView struct {
	Node        NodeView
	MatchReason string // Describes why this node matched
}
