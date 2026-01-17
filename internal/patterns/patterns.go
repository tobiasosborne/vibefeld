// Package patterns provides a challenge pattern library for the AF proof framework.
// It analyzes resolved challenges to extract common mistake patterns and helps
// future provers avoid similar issues.
package patterns

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/types"
)

// PatternType represents the type of mistake pattern.
type PatternType string

// Pattern types that can be detected from resolved challenges.
const (
	PatternLogicalGap       PatternType = "logical_gap"
	PatternScopeViolation   PatternType = "scope_violation"
	PatternCircularReasoning PatternType = "circular_reasoning"
	PatternUndefinedTerm    PatternType = "undefined_term"
)

// patternTypeRegistry maps each valid pattern type to its metadata.
var patternTypeRegistry = map[PatternType]PatternTypeInfo{
	PatternLogicalGap:       {Type: PatternLogicalGap, Description: "Missing justification or logical gap in reasoning"},
	PatternScopeViolation:   {Type: PatternScopeViolation, Description: "Using assumptions outside their valid scope"},
	PatternCircularReasoning: {Type: PatternCircularReasoning, Description: "Circular or self-referential dependency"},
	PatternUndefinedTerm:    {Type: PatternUndefinedTerm, Description: "Using terms that are not defined"},
}

// PatternTypeInfo provides metadata about a pattern type.
type PatternTypeInfo struct {
	Type        PatternType
	Description string
}

// ValidatePatternType validates a pattern type.
func ValidatePatternType(pt PatternType) error {
	if _, exists := patternTypeRegistry[pt]; !exists {
		return errors.New("invalid pattern type")
	}
	return nil
}

// AllPatternTypes returns all valid pattern types.
func AllPatternTypes() []PatternType {
	result := make([]PatternType, 0, len(patternTypeRegistry))
	for pt := range patternTypeRegistry {
		result = append(result, pt)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// GetPatternTypeInfo returns metadata for a pattern type.
func GetPatternTypeInfo(pt PatternType) (PatternTypeInfo, bool) {
	info, exists := patternTypeRegistry[pt]
	return info, exists
}

// Pattern represents a detected mistake pattern from resolved challenges.
type Pattern struct {
	// Type is the category of mistake.
	Type PatternType `json:"type"`

	// Description is a human-readable description of the pattern.
	Description string `json:"description"`

	// Example is a concrete example of this pattern from resolved challenges.
	Example string `json:"example"`

	// Occurrences is the number of times this pattern has been observed.
	Occurrences int `json:"occurrences"`

	// ChallengeID is the ID of the challenge this pattern was extracted from (if any).
	ChallengeID string `json:"challenge_id,omitempty"`

	// NodeID is the node where this pattern was first observed.
	NodeID types.NodeID `json:"node_id,omitempty"`
}

// NewPattern creates a new Pattern instance.
func NewPattern(pt PatternType, description, example string) *Pattern {
	return &Pattern{
		Type:        pt,
		Description: description,
		Example:     example,
		Occurrences: 0,
	}
}

// Increment increases the occurrence count by one.
func (p *Pattern) Increment() {
	p.Occurrences++
}

// PatternLibrary stores collected patterns from resolved challenges.
type PatternLibrary struct {
	// Version is the schema version of the library.
	Version string `json:"version"`

	// Patterns is the list of observed patterns.
	Patterns []*Pattern `json:"patterns"`
}

// NewPatternLibrary creates a new empty pattern library.
func NewPatternLibrary() *PatternLibrary {
	return &PatternLibrary{
		Version:  "1.0",
		Patterns: make([]*Pattern, 0),
	}
}

// AddPattern adds a pattern to the library.
func (lib *PatternLibrary) AddPattern(p *Pattern) {
	lib.Patterns = append(lib.Patterns, p)
}

// GetByType returns all patterns of a specific type.
func (lib *PatternLibrary) GetByType(pt PatternType) []*Pattern {
	var result []*Pattern
	for _, p := range lib.Patterns {
		if p.Type == pt {
			result = append(result, p)
		}
	}
	return result
}

// PatternStats contains statistics about patterns in the library.
type PatternStats struct {
	TotalPatterns    int                `json:"total_patterns"`
	TotalOccurrences int                `json:"total_occurrences"`
	ByType           map[PatternType]int `json:"by_type"`
}

// Stats computes statistics about the patterns in the library.
func (lib *PatternLibrary) Stats() *PatternStats {
	stats := &PatternStats{
		TotalPatterns:    len(lib.Patterns),
		TotalOccurrences: 0,
		ByType:           make(map[PatternType]int),
	}

	for _, p := range lib.Patterns {
		stats.TotalOccurrences += p.Occurrences
		stats.ByType[p.Type] += p.Occurrences
	}

	return stats
}

// patternsFilename is the name of the patterns file in the .af directory.
const patternsFilename = "patterns.json"

// Save saves the pattern library to the proof directory.
func (lib *PatternLibrary) Save(proofDir string) error {
	afDir := filepath.Join(proofDir, ".af")

	// Ensure .af directory exists
	if err := os.MkdirAll(afDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(afDir, patternsFilename)

	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadPatternLibrary loads the pattern library from the proof directory.
// If no patterns file exists, returns an empty library (not an error).
func LoadPatternLibrary(proofDir string) (*PatternLibrary, error) {
	path := filepath.Join(proofDir, ".af", patternsFilename)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Return empty library if file doesn't exist
		return NewPatternLibrary(), nil
	}
	if err != nil {
		return nil, err
	}

	var lib PatternLibrary
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, err
	}

	// Ensure Patterns is not nil
	if lib.Patterns == nil {
		lib.Patterns = make([]*Pattern, 0)
	}

	return &lib, nil
}

// DetectPatternType determines the pattern type from a challenge's target and reason.
func DetectPatternType(target, reason string) PatternType {
	// Normalize inputs
	targetLower := strings.ToLower(target)
	reasonLower := strings.ToLower(reason)

	// Check for scope violations
	if targetLower == "scope" || strings.Contains(reasonLower, "scope") {
		return PatternScopeViolation
	}

	// Check for circular reasoning
	if strings.Contains(reasonLower, "circular") ||
		strings.Contains(reasonLower, "self-referential") ||
		strings.Contains(reasonLower, "depends on itself") {
		return PatternCircularReasoning
	}

	// Check for undefined terms
	if targetLower == "context" ||
		strings.Contains(reasonLower, "not defined") ||
		strings.Contains(reasonLower, "undefined") ||
		strings.Contains(reasonLower, "missing definition") ||
		strings.Contains(reasonLower, "unknown term") {
		return PatternUndefinedTerm
	}

	// Check for logical gaps
	if targetLower == "gap" ||
		strings.Contains(reasonLower, "gap") ||
		strings.Contains(reasonLower, "missing") ||
		strings.Contains(reasonLower, "justification") {
		return PatternLogicalGap
	}

	// Default to logical gap for other targets
	return PatternLogicalGap
}

// Analyzer analyzes challenges and nodes to detect patterns.
type Analyzer struct {
	library *PatternLibrary
}

// NewAnalyzer creates a new analyzer with the given pattern library.
func NewAnalyzer(lib *PatternLibrary) *Analyzer {
	return &Analyzer{
		library: lib,
	}
}

// AnalyzeChallenge analyzes a single challenge and returns a pattern.
func (a *Analyzer) AnalyzeChallenge(c *state.Challenge) *Pattern {
	pt := DetectPatternType(c.Target, c.Reason)

	p := &Pattern{
		Type:        pt,
		Description: c.Reason,
		Example:     c.Resolution,
		Occurrences: 1,
		ChallengeID: c.ID,
		NodeID:      c.NodeID,
	}

	return p
}

// AnalyzeResolvedChallenges analyzes all resolved challenges and returns patterns.
func (a *Analyzer) AnalyzeResolvedChallenges(challenges []*state.Challenge) []*Pattern {
	var patterns []*Pattern

	for _, c := range challenges {
		// Only analyze resolved challenges (not open or withdrawn)
		if c.Status != state.ChallengeStatusResolved {
			continue
		}

		p := a.AnalyzeChallenge(c)
		patterns = append(patterns, p)
	}

	return patterns
}

// ExtractPatterns extracts patterns from resolved challenges and adds them to the library.
func (a *Analyzer) ExtractPatterns(challenges []*state.Challenge) {
	patterns := a.AnalyzeResolvedChallenges(challenges)

	// Group by pattern type and description to avoid duplicates
	seen := make(map[string]*Pattern)

	for _, p := range patterns {
		key := string(p.Type) + ":" + p.Description
		if existing, ok := seen[key]; ok {
			existing.Increment()
		} else {
			seen[key] = p
			a.library.AddPattern(p)
		}
	}
}

// PotentialIssue represents a potential issue detected in a node.
type PotentialIssue struct {
	NodeID      types.NodeID `json:"node_id"`
	PatternType PatternType  `json:"pattern_type"`
	Description string       `json:"description"`
	Confidence  float64      `json:"confidence"` // 0.0 to 1.0
}

// AnalyzeNode analyzes a node for potential issues based on known patterns.
// contextNodes provides additional nodes for context (e.g., assumption nodes).
func (a *Analyzer) AnalyzeNode(n *node.Node, contextNodes map[string]*node.Node) []PotentialIssue {
	var issues []PotentialIssue

	// Check for patterns based on statement content
	stmtLower := strings.ToLower(n.Statement)

	// Check for potential logical gaps
	gapIndicators := []string{
		"trivially", "immediately", "obviously", "clearly",
		"it follows", "follows that", "hence", "thus",
	}

	gapPatterns := a.library.GetByType(PatternLogicalGap)
	if len(gapPatterns) > 0 {
		for _, indicator := range gapIndicators {
			if strings.Contains(stmtLower, indicator) {
				// Calculate confidence based on pattern frequency
				confidence := calculateConfidence(gapPatterns)
				issues = append(issues, PotentialIssue{
					NodeID:      n.ID,
					PatternType: PatternLogicalGap,
					Description: "Statement uses vague justification language that may indicate a logical gap",
					Confidence:  confidence,
				})
				break
			}
		}
	}

	// Check for potential scope violations
	if len(n.Scope) > 0 {
		scopePatterns := a.library.GetByType(PatternScopeViolation)
		if len(scopePatterns) > 0 {
			// Check if any scope references might be invalid
			for _, scopeRef := range n.Scope {
				scopeID, err := types.Parse(scopeRef)
				if err != nil {
					continue
				}

				// Check if this is a valid scope reference
				// A scope violation might occur if the node is not a descendant of the assumption
				if !isDescendantOrSibling(n.ID, scopeID) {
					confidence := calculateConfidence(scopePatterns)
					issues = append(issues, PotentialIssue{
						NodeID:      n.ID,
						PatternType: PatternScopeViolation,
						Description: "Node references scope from " + scopeRef + " but may not be within that scope",
						Confidence:  confidence,
					})
				}
			}
		}
	}

	// Check for potential undefined terms
	undefinedPatterns := a.library.GetByType(PatternUndefinedTerm)
	if len(undefinedPatterns) > 0 && len(n.Context) == 0 {
		// Node has no context references but uses technical-looking terms
		if containsTechnicalTerms(stmtLower) {
			confidence := calculateConfidence(undefinedPatterns)
			issues = append(issues, PotentialIssue{
				NodeID:      n.ID,
				PatternType: PatternUndefinedTerm,
				Description: "Statement may contain undefined technical terms",
				Confidence:  confidence * 0.5, // Lower confidence for this heuristic
			})
		}
	}

	return issues
}

// AnalyzeState analyzes an entire proof state for potential issues.
func (a *Analyzer) AnalyzeState(st *state.State) []PotentialIssue {
	var allIssues []PotentialIssue

	// Build context map for all nodes
	allNodes := st.AllNodes()
	contextNodes := make(map[string]*node.Node)
	for _, n := range allNodes {
		contextNodes[n.ID.String()] = n
	}

	// Analyze each node
	for _, n := range allNodes {
		issues := a.AnalyzeNode(n, contextNodes)
		allIssues = append(allIssues, issues...)
	}

	// Sort by confidence (highest first)
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].Confidence > allIssues[j].Confidence
	})

	return allIssues
}

// calculateConfidence calculates a confidence score based on pattern frequency.
func calculateConfidence(patterns []*Pattern) float64 {
	if len(patterns) == 0 {
		return 0.0
	}

	totalOccurrences := 0
	for _, p := range patterns {
		totalOccurrences += p.Occurrences
	}

	// Base confidence on number of occurrences
	// More occurrences = higher confidence that this pattern is real
	if totalOccurrences >= 10 {
		return 0.9
	} else if totalOccurrences >= 5 {
		return 0.7
	} else if totalOccurrences >= 2 {
		return 0.5
	}
	return 0.3
}

// isDescendantOrSibling checks if nodeID is a descendant or sibling of scopeID.
func isDescendantOrSibling(nodeID, scopeID types.NodeID) bool {
	nodeStr := nodeID.String()
	scopeStr := scopeID.String()

	// Node is descendant if it starts with scope ID followed by a dot
	if strings.HasPrefix(nodeStr, scopeStr+".") {
		return true
	}

	// Node is sibling if they share a common parent
	nodeParent, hasNodeParent := nodeID.Parent()
	scopeParent, hasScopeParent := scopeID.Parent()

	if hasNodeParent && hasScopeParent {
		if nodeParent.String() == scopeParent.String() {
			return true
		}
	}

	// Check if node and scope are on related branches
	// (e.g., 1.1.2 can use assumption from 1.1)
	if strings.HasPrefix(nodeStr, scopeStr) {
		return true
	}

	return false
}

// containsTechnicalTerms checks if a statement contains technical-looking terms.
func containsTechnicalTerms(s string) bool {
	// Simple heuristic: look for Greek letters or mathematical symbols/words
	technicalIndicators := []string{
		"epsilon", "delta", "alpha", "beta", "gamma",
		"theorem", "lemma", "corollary", "proposition",
		"continuous", "convergent", "bounded", "compact",
		"isomorphism", "homomorphism", "bijection",
	}

	for _, term := range technicalIndicators {
		if strings.Contains(s, term) {
			return true
		}
	}

	return false
}
