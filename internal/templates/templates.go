// Package templates provides predefined proof structure templates for common
// mathematical proof patterns like induction, contradiction, and case analysis.
package templates

import (
	"github.com/tobias/vibefeld/internal/schema"
)

// ChildSpec defines a child node to be created by a template.
type ChildSpec struct {
	// NodeType is the type of node to create (e.g., claim, local_assume, case)
	NodeType schema.NodeType

	// StatementTemplate is the default statement for this child.
	// It may contain placeholders that can be customized.
	StatementTemplate string

	// Inference is the inference type for this child node.
	Inference schema.InferenceType
}

// Template defines a proof structure template.
type Template struct {
	// Name is the unique identifier for this template
	Name string

	// Description is a human-readable description of the template
	Description string

	// RootStatement is an optional override for the root node statement.
	// If empty, the user's conjecture is used directly.
	RootStatement string

	// Children are the child nodes to create under the root
	Children []ChildSpec
}

// registry contains all available templates
var registry = map[string]Template{
	"contradiction": {
		Name:        "contradiction",
		Description: "Proof by contradiction - assume the negation and derive a contradiction",
		Children: []ChildSpec{
			{
				NodeType:          schema.NodeTypeLocalAssume,
				StatementTemplate: "Assume the negation of the conjecture",
				Inference:         schema.InferenceLocalAssume,
			},
			{
				NodeType:          schema.NodeTypeClaim,
				StatementTemplate: "Derive a contradiction from the assumption",
				Inference:         schema.InferenceContradiction,
			},
		},
	},
	"induction": {
		Name:        "induction",
		Description: "Proof by induction - prove base case and inductive step",
		Children: []ChildSpec{
			{
				NodeType:          schema.NodeTypeClaim,
				StatementTemplate: "Base case: prove the statement holds for the base case",
				Inference:         schema.InferenceAssumption,
			},
			{
				NodeType:          schema.NodeTypeClaim,
				StatementTemplate: "Inductive step: assume P(k) holds, prove P(k+1)",
				Inference:         schema.InferenceUniversalGeneralization,
			},
		},
	},
	"cases": {
		Name:        "cases",
		Description: "Proof by case analysis - exhaustively analyze all cases",
		Children: []ChildSpec{
			{
				NodeType:          schema.NodeTypeCase,
				StatementTemplate: "Case 1: [describe first case]",
				Inference:         schema.InferenceAssumption,
			},
			{
				NodeType:          schema.NodeTypeCase,
				StatementTemplate: "Case 2: [describe second case]",
				Inference:         schema.InferenceAssumption,
			},
		},
	},
}

// Get retrieves a template by name.
// Returns (template, true) if found, (zero, false) if not found.
func Get(name string) (Template, bool) {
	tmpl, ok := registry[name]
	return tmpl, ok
}

// List returns all available templates in a consistent order.
func List() []Template {
	// Return in alphabetical order for consistency
	return []Template{
		registry["cases"],
		registry["contradiction"],
		registry["induction"],
	}
}

// Names returns all available template names in a consistent order.
func Names() []string {
	return []string{"cases", "contradiction", "induction"}
}
