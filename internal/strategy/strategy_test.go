// Package strategy_test provides TDD tests for the proof strategy guidance system.
// The strategy package helps provers structure their proofs with common strategies
// and templates for direct proof, contradiction, induction, cases, and contrapositive.
package strategy_test

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/strategy"
)

// =============================================================================
// Strategy Type Tests
// =============================================================================

// TestAllStrategies_ReturnsRequiredStrategies tests that all required strategies exist.
func TestAllStrategies_ReturnsRequiredStrategies(t *testing.T) {
	strategies := strategy.All()

	// Should have at least 5 strategies
	if len(strategies) < 5 {
		t.Errorf("expected at least 5 strategies, got %d", len(strategies))
	}

	// Check that required strategies are present
	names := make(map[string]bool)
	for _, s := range strategies {
		names[s.Name] = true
	}

	required := []string{"direct", "contradiction", "induction", "cases", "contrapositive"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("expected strategy %q to be in list", name)
		}
	}
}

// TestGet_DirectProof tests getting the direct proof strategy.
func TestGet_DirectProof(t *testing.T) {
	s, ok := strategy.Get("direct")
	if !ok {
		t.Fatal("expected direct proof strategy to exist")
	}

	if s.Name != "direct" {
		t.Errorf("expected name 'direct', got %q", s.Name)
	}

	if s.Description == "" {
		t.Error("expected description to be non-empty")
	}

	if len(s.Steps) == 0 {
		t.Error("expected at least one step")
	}
}

// TestGet_Contradiction tests getting the contradiction strategy.
func TestGet_Contradiction(t *testing.T) {
	s, ok := strategy.Get("contradiction")
	if !ok {
		t.Fatal("expected contradiction strategy to exist")
	}

	if s.Name != "contradiction" {
		t.Errorf("expected name 'contradiction', got %q", s.Name)
	}

	// Contradiction should mention negation and contradiction
	descLower := strings.ToLower(s.Description)
	if !strings.Contains(descLower, "negat") && !strings.Contains(descLower, "contradict") {
		t.Error("contradiction description should mention negation or contradiction")
	}

	// Should have steps for assuming negation and deriving contradiction
	if len(s.Steps) < 2 {
		t.Error("contradiction should have at least 2 steps")
	}
}

// TestGet_Induction tests getting the induction strategy.
func TestGet_Induction(t *testing.T) {
	s, ok := strategy.Get("induction")
	if !ok {
		t.Fatal("expected induction strategy to exist")
	}

	if s.Name != "induction" {
		t.Errorf("expected name 'induction', got %q", s.Name)
	}

	// Check for base case, inductive hypothesis, and inductive step
	hasBase := false
	hasHypothesis := false
	hasStep := false
	for _, step := range s.Steps {
		stepLower := strings.ToLower(step.Description)
		if strings.Contains(stepLower, "base") {
			hasBase = true
		}
		if strings.Contains(stepLower, "hypothesis") || strings.Contains(stepLower, "assume") {
			hasHypothesis = true
		}
		if strings.Contains(stepLower, "inductive step") || strings.Contains(stepLower, "successor") {
			hasStep = true
		}
	}

	if !hasBase {
		t.Error("induction should have a base case step")
	}
	if !hasHypothesis {
		t.Error("induction should have an inductive hypothesis step")
	}
	if !hasStep {
		t.Error("induction should have an inductive step")
	}
}

// TestGet_Cases tests getting the cases strategy.
func TestGet_Cases(t *testing.T) {
	s, ok := strategy.Get("cases")
	if !ok {
		t.Fatal("expected cases strategy to exist")
	}

	if s.Name != "cases" {
		t.Errorf("expected name 'cases', got %q", s.Name)
	}

	// Should mention exhaustive or case analysis
	descLower := strings.ToLower(s.Description)
	if !strings.Contains(descLower, "case") && !strings.Contains(descLower, "exhaust") {
		t.Error("cases description should mention case analysis or exhaustive")
	}
}

// TestGet_Contrapositive tests getting the contrapositive strategy.
func TestGet_Contrapositive(t *testing.T) {
	s, ok := strategy.Get("contrapositive")
	if !ok {
		t.Fatal("expected contrapositive strategy to exist")
	}

	if s.Name != "contrapositive" {
		t.Errorf("expected name 'contrapositive', got %q", s.Name)
	}

	// Should mention negation and implication
	descLower := strings.ToLower(s.Description)
	if !strings.Contains(descLower, "negat") && !strings.Contains(descLower, "contrapositive") {
		t.Error("contrapositive description should mention negation or contrapositive")
	}
}

// TestGet_NotFound tests that Get returns false for unknown strategies.
func TestGet_NotFound(t *testing.T) {
	_, ok := strategy.Get("nonexistent")
	if ok {
		t.Error("expected nonexistent strategy to not be found")
	}
}

// =============================================================================
// Strategy Description Tests
// =============================================================================

// TestAllStrategies_HaveDescriptions tests that all strategies have descriptions.
func TestAllStrategies_HaveDescriptions(t *testing.T) {
	for _, s := range strategy.All() {
		if s.Description == "" {
			t.Errorf("strategy %q should have a description", s.Name)
		}
	}
}

// TestAllStrategies_HaveSteps tests that all strategies have steps.
func TestAllStrategies_HaveSteps(t *testing.T) {
	for _, s := range strategy.All() {
		if len(s.Steps) == 0 {
			t.Errorf("strategy %q should have at least one step", s.Name)
		}
	}
}

// TestAllStrategies_HaveExamples tests that all strategies have example outlines.
func TestAllStrategies_HaveExamples(t *testing.T) {
	for _, s := range strategy.All() {
		if s.Example == "" {
			t.Errorf("strategy %q should have an example outline", s.Name)
		}
	}
}

// =============================================================================
// Suggestion Tests
// =============================================================================

// TestSuggest_InductionForForAll tests suggestion for universal statements.
func TestSuggest_InductionForForAll(t *testing.T) {
	conjecture := "For all natural numbers n, n + 0 = n"
	suggestions := strategy.Suggest(conjecture)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should suggest induction for universal quantification over naturals
	hasInduction := false
	for _, s := range suggestions {
		if s.Strategy.Name == "induction" {
			hasInduction = true
			break
		}
	}

	if !hasInduction {
		t.Error("expected induction to be suggested for universal quantification over naturals")
	}
}

// TestSuggest_ContradictionForNotExists tests suggestion for non-existence.
func TestSuggest_ContradictionForNotExists(t *testing.T) {
	conjecture := "There does not exist a largest prime number"
	suggestions := strategy.Suggest(conjecture)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should suggest contradiction for non-existence proofs
	hasContradiction := false
	for _, s := range suggestions {
		if s.Strategy.Name == "contradiction" {
			hasContradiction = true
			break
		}
	}

	if !hasContradiction {
		t.Error("expected contradiction to be suggested for non-existence proof")
	}
}

// TestSuggest_CasesForOr tests suggestion for disjunctive statements.
func TestSuggest_CasesForOr(t *testing.T) {
	conjecture := "Either n is even or n is odd"
	suggestions := strategy.Suggest(conjecture)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should suggest cases for disjunctive statements
	hasCases := false
	for _, s := range suggestions {
		if s.Strategy.Name == "cases" {
			hasCases = true
			break
		}
	}

	if !hasCases {
		t.Error("expected cases to be suggested for disjunctive statement")
	}
}

// TestSuggest_ContrapositiveForImplication tests suggestion for implications.
func TestSuggest_ContrapositiveForImplication(t *testing.T) {
	conjecture := "If n^2 is even, then n is even"
	suggestions := strategy.Suggest(conjecture)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should suggest contrapositive for implications
	hasContrapositive := false
	for _, s := range suggestions {
		if s.Strategy.Name == "contrapositive" {
			hasContrapositive = true
			break
		}
	}

	if !hasContrapositive {
		t.Error("expected contrapositive to be suggested for implication")
	}
}

// TestSuggest_DirectForSimple tests suggestion for simple statements.
func TestSuggest_DirectForSimple(t *testing.T) {
	conjecture := "2 + 2 = 4"
	suggestions := strategy.Suggest(conjecture)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should suggest direct proof for simple statements
	hasDirect := false
	for _, s := range suggestions {
		if s.Strategy.Name == "direct" {
			hasDirect = true
			break
		}
	}

	if !hasDirect {
		t.Error("expected direct proof to be suggested")
	}
}

// TestSuggest_ReturnsReasons tests that suggestions include reasons.
func TestSuggest_ReturnsReasons(t *testing.T) {
	conjecture := "For all n, P(n)"
	suggestions := strategy.Suggest(conjecture)

	for _, s := range suggestions {
		if s.Reason == "" {
			t.Errorf("suggestion for %q should have a reason", s.Strategy.Name)
		}
	}
}

// =============================================================================
// Skeleton Generation Tests
// =============================================================================

// TestGenerateSkeleton_Direct tests skeleton generation for direct proof.
func TestGenerateSkeleton_Direct(t *testing.T) {
	s, _ := strategy.Get("direct")
	skeleton := s.GenerateSkeleton("P implies Q")

	if skeleton == "" {
		t.Fatal("expected non-empty skeleton")
	}

	// Should mention the conjecture
	if !strings.Contains(skeleton, "P implies Q") {
		t.Error("skeleton should include the conjecture")
	}

	// Should have proof structure
	lower := strings.ToLower(skeleton)
	if !strings.Contains(lower, "prove") && !strings.Contains(lower, "goal") {
		t.Error("skeleton should reference the proof goal")
	}
}

// TestGenerateSkeleton_Induction tests skeleton generation for induction.
func TestGenerateSkeleton_Induction(t *testing.T) {
	s, _ := strategy.Get("induction")
	skeleton := s.GenerateSkeleton("For all n, P(n)")

	if skeleton == "" {
		t.Fatal("expected non-empty skeleton")
	}

	lower := strings.ToLower(skeleton)

	// Should have base case
	if !strings.Contains(lower, "base") {
		t.Error("induction skeleton should have base case")
	}

	// Should have inductive step
	if !strings.Contains(lower, "inductive") {
		t.Error("induction skeleton should have inductive step")
	}
}

// TestGenerateSkeleton_Contradiction tests skeleton generation for contradiction.
func TestGenerateSkeleton_Contradiction(t *testing.T) {
	s, _ := strategy.Get("contradiction")
	skeleton := s.GenerateSkeleton("There is no largest prime")

	if skeleton == "" {
		t.Fatal("expected non-empty skeleton")
	}

	lower := strings.ToLower(skeleton)

	// Should assume negation
	if !strings.Contains(lower, "assume") {
		t.Error("contradiction skeleton should assume the negation")
	}

	// Should derive contradiction
	if !strings.Contains(lower, "contradict") {
		t.Error("contradiction skeleton should derive a contradiction")
	}
}

// TestGenerateSkeleton_Cases tests skeleton generation for case analysis.
func TestGenerateSkeleton_Cases(t *testing.T) {
	s, _ := strategy.Get("cases")
	skeleton := s.GenerateSkeleton("Either A or B")

	if skeleton == "" {
		t.Fatal("expected non-empty skeleton")
	}

	lower := strings.ToLower(skeleton)

	// Should have case 1
	if !strings.Contains(lower, "case 1") && !strings.Contains(lower, "case:") {
		t.Error("cases skeleton should have Case 1")
	}
}

// TestGenerateSkeleton_Contrapositive tests skeleton generation for contrapositive.
func TestGenerateSkeleton_Contrapositive(t *testing.T) {
	s, _ := strategy.Get("contrapositive")
	skeleton := s.GenerateSkeleton("If P then Q")

	if skeleton == "" {
		t.Fatal("expected non-empty skeleton")
	}

	lower := strings.ToLower(skeleton)

	// Should mention negation
	if !strings.Contains(lower, "not") && !strings.Contains(lower, "negat") {
		t.Error("contrapositive skeleton should mention negation")
	}
}

// =============================================================================
// Names Tests
// =============================================================================

// TestNames_ReturnsAllNames tests that Names returns all strategy names.
func TestNames_ReturnsAllNames(t *testing.T) {
	names := strategy.Names()

	if len(names) < 5 {
		t.Errorf("expected at least 5 names, got %d", len(names))
	}

	// Check required names
	required := map[string]bool{
		"direct":        false,
		"contradiction": false,
		"induction":     false,
		"cases":         false,
		"contrapositive": false,
	}

	for _, name := range names {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}

	for name, found := range required {
		if !found {
			t.Errorf("expected %q to be in names list", name)
		}
	}
}

// =============================================================================
// Step Tests
// =============================================================================

// TestStep_HasDescription tests that steps have descriptions.
func TestStep_HasDescription(t *testing.T) {
	for _, s := range strategy.All() {
		for i, step := range s.Steps {
			if step.Description == "" {
				t.Errorf("strategy %q step %d should have a description", s.Name, i+1)
			}
		}
	}
}

// TestStep_HasTemplate tests that steps have templates.
func TestStep_HasTemplate(t *testing.T) {
	for _, s := range strategy.All() {
		for i, step := range s.Steps {
			if step.Template == "" {
				t.Errorf("strategy %q step %d should have a template", s.Name, i+1)
			}
		}
	}
}
