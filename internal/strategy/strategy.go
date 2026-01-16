// Package strategy provides proof structure and strategy guidance for the AF framework.
// It helps provers structure their proofs with common strategies and templates including
// direct proof, proof by contradiction, proof by induction, proof by cases, and proof
// by contrapositive.
package strategy

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// titleCase converts the first character of a string to uppercase.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Step represents a single step in a proof strategy.
type Step struct {
	// Description explains what this step accomplishes
	Description string

	// Template provides a template for this step's content
	Template string
}

// Strategy represents a proof strategy with its structure and guidance.
type Strategy struct {
	// Name is the unique identifier for this strategy
	Name string

	// Description is a human-readable description of when to use this strategy
	Description string

	// Steps are the required steps/structure for this strategy
	Steps []Step

	// Example provides an example outline of using this strategy
	Example string
}

// GenerateSkeleton generates a proof skeleton using this strategy for the given conjecture.
func (s *Strategy) GenerateSkeleton(conjecture string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Proof Strategy: %s\n", titleCase(s.Name)))
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")
	sb.WriteString(fmt.Sprintf("Goal: %s\n\n", conjecture))
	sb.WriteString("Proof Structure:\n")
	sb.WriteString(strings.Repeat("-", 40) + "\n\n")

	for i, step := range s.Steps {
		sb.WriteString(fmt.Sprintf("Step %d: %s\n", i+1, step.Description))
		sb.WriteString(fmt.Sprintf("  %s\n\n", step.Template))
	}

	sb.WriteString("Conclusion:\n")
	sb.WriteString(fmt.Sprintf("  Therefore, %s. QED\n", conjecture))

	return sb.String()
}

// Suggestion represents a strategy suggestion with reasoning.
type Suggestion struct {
	// Strategy is the suggested strategy
	Strategy Strategy

	// Reason explains why this strategy is suggested
	Reason string

	// Confidence indicates how well the conjecture matches this strategy (0.0 to 1.0)
	Confidence float64
}

// registry contains all available strategies
var registry = map[string]Strategy{
	"direct": {
		Name:        "direct",
		Description: "Prove the statement directly by logical deduction from known facts and definitions",
		Steps: []Step{
			{
				Description: "State the assumptions and what needs to be proved",
				Template:    "Let [assumptions]. We want to prove [goal].",
			},
			{
				Description: "Apply definitions and known results",
				Template:    "By definition of [term], we have [consequence].",
			},
			{
				Description: "Chain logical steps to reach the conclusion",
				Template:    "From [previous], it follows that [next step].",
			},
			{
				Description: "Conclude the proof",
				Template:    "Therefore, [goal] holds.",
			},
		},
		Example: `Example: Prove that 2 + 2 = 4

Step 1: By definition, 2 = S(S(0)) where S is the successor function.
Step 2: 2 + 2 = S(S(0)) + S(S(0))
Step 3: By the definition of addition, this equals S(S(S(S(0)))) = 4
Step 4: Therefore, 2 + 2 = 4.`,
	},

	"contradiction": {
		Name:        "contradiction",
		Description: "Assume the negation of the statement and derive a contradiction",
		Steps: []Step{
			{
				Description: "Assume the negation of what you want to prove",
				Template:    "Assume for contradiction that [negation of goal].",
			},
			{
				Description: "Derive consequences from the assumption",
				Template:    "From this assumption, it follows that [consequence].",
			},
			{
				Description: "Show these consequences lead to a contradiction",
				Template:    "But this contradicts [known fact or earlier result].",
			},
			{
				Description: "Conclude the original statement must be true",
				Template:    "Therefore, our assumption was false, and [goal] holds.",
			},
		},
		Example: `Example: Prove there is no largest prime number

Step 1: Assume for contradiction that there exists a largest prime p.
Step 2: Let P = (2 * 3 * 5 * ... * p) + 1 (product of all primes up to p, plus 1).
Step 3: P is not divisible by any prime <= p, so either P is prime or has a prime factor > p.
Step 4: This contradicts p being the largest prime.
Step 5: Therefore, there is no largest prime number.`,
	},

	"induction": {
		Name:        "induction",
		Description: "Prove for a base case, assume it holds for n, prove it holds for n+1",
		Steps: []Step{
			{
				Description: "Prove the base case (usually n=0 or n=1)",
				Template:    "Base case: When n = [base], we have [verification].",
			},
			{
				Description: "State the inductive hypothesis - assume P(k) holds",
				Template:    "Inductive hypothesis: Assume [statement] holds for some k >= [base].",
			},
			{
				Description: "Prove the inductive step - show P(k) implies P(k+1)",
				Template:    "Inductive step: We must show [statement] holds for k+1.",
			},
			{
				Description: "Use the hypothesis to complete the inductive step",
				Template:    "By the inductive hypothesis, [consequence]. Therefore [k+1 case].",
			},
			{
				Description: "Conclude by the principle of mathematical induction",
				Template:    "By induction, [statement] holds for all n >= [base].",
			},
		},
		Example: `Example: Prove for all n >= 0, 0 + n = n

Base case: When n = 0, we have 0 + 0 = 0 by definition of addition.

Inductive hypothesis: Assume 0 + k = k holds for some k >= 0.

Inductive step: We must show 0 + (k+1) = k+1.
  0 + (k+1) = 0 + S(k)     [definition of successor]
            = S(0 + k)     [definition of addition]
            = S(k)         [by inductive hypothesis]
            = k+1          [definition of successor]

By induction, 0 + n = n holds for all n >= 0.`,
	},

	"cases": {
		Name:        "cases",
		Description: "Exhaustively analyze all possible cases and prove the statement in each case",
		Steps: []Step{
			{
				Description: "Identify all possible cases (must be exhaustive)",
				Template:    "We consider all cases: [list of cases].",
			},
			{
				Description: "Prove the statement for Case 1",
				Template:    "Case 1: [case description]. Then [proof for this case].",
			},
			{
				Description: "Prove the statement for Case 2",
				Template:    "Case 2: [case description]. Then [proof for this case].",
			},
			{
				Description: "Continue for remaining cases and conclude",
				Template:    "Since all cases are covered, [goal] holds.",
			},
		},
		Example: `Example: Prove n^2 >= n for all integers n

We consider three cases: n < 0, n = 0, and n > 0.

Case 1: n < 0
  Then n^2 > 0 > n, so n^2 >= n.

Case 2: n = 0
  Then n^2 = 0 = n, so n^2 >= n.

Case 3: n > 0
  Then n^2 = n * n >= n * 1 = n (since n >= 1), so n^2 >= n.

Since all cases are covered, n^2 >= n for all integers n.`,
	},

	"contrapositive": {
		Name:        "contrapositive",
		Description: "Prove the contrapositive: instead of 'if P then Q', prove 'if not Q then not P' (logically equivalent via negation)",
		Steps: []Step{
			{
				Description: "State the contrapositive of the implication",
				Template:    "We prove the contrapositive: if not [Q], then not [P].",
			},
			{
				Description: "Assume the negation of the consequent",
				Template:    "Assume [not Q].",
			},
			{
				Description: "Derive the negation of the antecedent",
				Template:    "From [not Q], it follows that [intermediate steps].",
			},
			{
				Description: "Conclude the negation of the antecedent, thus proving the contrapositive",
				Template:    "Therefore, [not P]. The contrapositive holds, so if [P] then [Q].",
			},
		},
		Example: `Example: Prove if n^2 is even, then n is even

We prove the contrapositive: if n is odd, then n^2 is odd.

Assume n is odd. Then n = 2k + 1 for some integer k.

n^2 = (2k + 1)^2
    = 4k^2 + 4k + 1
    = 2(2k^2 + 2k) + 1

Since 2k^2 + 2k is an integer, n^2 = 2m + 1 for some integer m.
Therefore, n^2 is odd.

The contrapositive holds, so if n^2 is even, then n is even.`,
	},
}

// Patterns for suggestion matching
var (
	universalPattern     = regexp.MustCompile(`(?i)\b(for all|every|all|any)\b.*\b(natural|integer|number|n\b)`)
	nonExistencePattern  = regexp.MustCompile(`(?i)(\bno\b|\bnot\b|\bcannot\b|not exist|does not exist|there is no|impossible|irrational)`)
	disjunctionPattern   = regexp.MustCompile(`(?i)\b(either|or)\b`)
	implicationPattern   = regexp.MustCompile(`(?i)\b(if|then|implies|whenever|when)\b`)
	evenOddPattern       = regexp.MustCompile(`(?i)\b(even|odd)\b`)
	inductionKeywords    = regexp.MustCompile(`(?i)\b(natural number|n\s*\+\s*1|successor|induction)\b`)
)

// Get retrieves a strategy by name.
// Returns (strategy, true) if found, (zero, false) if not found.
func Get(name string) (Strategy, bool) {
	s, ok := registry[name]
	return s, ok
}

// All returns all available strategies in a consistent order.
func All() []Strategy {
	return []Strategy{
		registry["direct"],
		registry["contradiction"],
		registry["induction"],
		registry["cases"],
		registry["contrapositive"],
	}
}

// Names returns all available strategy names in a consistent order.
func Names() []string {
	return []string{"direct", "contradiction", "induction", "cases", "contrapositive"}
}

// Suggest analyzes a conjecture and suggests appropriate proof strategies.
func Suggest(conjecture string) []Suggestion {
	var suggestions []Suggestion

	// Check for induction indicators
	if universalPattern.MatchString(conjecture) || inductionKeywords.MatchString(conjecture) {
		suggestions = append(suggestions, Suggestion{
			Strategy:   registry["induction"],
			Reason:     "The conjecture quantifies over natural numbers or integers, making induction a natural choice",
			Confidence: 0.8,
		})
	}

	// Check for non-existence/impossibility claims
	if nonExistencePattern.MatchString(conjecture) {
		suggestions = append(suggestions, Suggestion{
			Strategy:   registry["contradiction"],
			Reason:     "The conjecture asserts non-existence or impossibility, which is often proved by contradiction",
			Confidence: 0.85,
		})
	}

	// Check for disjunctive statements
	if disjunctionPattern.MatchString(conjecture) {
		suggestions = append(suggestions, Suggestion{
			Strategy:   registry["cases"],
			Reason:     "The conjecture involves a disjunction (either/or), suggesting case analysis",
			Confidence: 0.75,
		})
	}

	// Check for implications, especially with even/odd
	if implicationPattern.MatchString(conjecture) {
		// Special case: even/odd proofs often use contrapositive
		if evenOddPattern.MatchString(conjecture) {
			suggestions = append(suggestions, Suggestion{
				Strategy:   registry["contrapositive"],
				Reason:     "Implications involving even/odd properties are often easier to prove via contrapositive",
				Confidence: 0.8,
			})
		} else {
			suggestions = append(suggestions, Suggestion{
				Strategy:   registry["contrapositive"],
				Reason:     "The conjecture is an implication; consider proving the contrapositive if direct proof is difficult",
				Confidence: 0.6,
			})
		}

		// Direct proof is also valid for implications
		suggestions = append(suggestions, Suggestion{
			Strategy:   registry["direct"],
			Reason:     "Implications can be proved directly by assuming the antecedent and deriving the consequent",
			Confidence: 0.5,
		})
	}

	// Always suggest direct proof as a fallback if no other matches
	if len(suggestions) == 0 {
		suggestions = append(suggestions, Suggestion{
			Strategy:   registry["direct"],
			Reason:     "Direct proof is the simplest approach when no special structure is apparent",
			Confidence: 0.5,
		})
	}

	// Sort by confidence (descending)
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Confidence > suggestions[i].Confidence {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	return suggestions
}
