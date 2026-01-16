package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/patterns"
	"github.com/tobias/vibefeld/internal/service"
)

// newPatternsCmd creates the patterns command with subcommands.
func newPatternsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Manage challenge pattern library",
		Long: `Analyze resolved challenges and extract common mistake patterns.

The pattern library helps future provers avoid common mistakes by learning
from resolved challenges. It tracks four types of patterns:

  - logical_gap:        Missing justification or logical gaps in reasoning
  - scope_violation:    Using assumptions outside their valid scope
  - circular_reasoning: Circular or self-referential dependencies
  - undefined_term:     Using terms that are not defined

Subcommands:
  list      Show known patterns from resolved challenges
  analyze   Analyze current proof for potential issues
  stats     Show statistics on common challenge types
  extract   Extract patterns from resolved challenges

Examples:
  af patterns list                    List all known patterns
  af patterns list --type logical_gap Filter by pattern type
  af patterns analyze                 Analyze current proof
  af patterns stats                   Show pattern statistics
  af patterns extract                 Extract patterns from challenges`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to showing help
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newPatternsListCmd())
	cmd.AddCommand(newPatternsAnalyzeCmd())
	cmd.AddCommand(newPatternsStatsCmd())
	cmd.AddCommand(newPatternsExtractCmd())

	return cmd
}

// newPatternsListCmd creates the patterns list subcommand.
func newPatternsListCmd() *cobra.Command {
	var dir string
	var typeFilter string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List known patterns from resolved challenges",
		Long: `List all patterns that have been extracted from resolved challenges.

Patterns are categorized by type:
  - logical_gap:        Missing steps or justification
  - scope_violation:    Assumption scope issues
  - circular_reasoning: Self-referential dependencies
  - undefined_term:     Undefined terminology

Examples:
  af patterns list                     List all patterns
  af patterns list --type logical_gap  Filter by type
  af patterns list --json              Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsList(cmd, dir, typeFilter, jsonOutput)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "Filter by pattern type")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// runPatternsList executes the patterns list command.
func runPatternsList(cmd *cobra.Command, dir, typeFilter string, jsonOutput bool) error {
	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Validate type filter if provided
	if typeFilter != "" {
		if err := patterns.ValidatePatternType(patterns.PatternType(typeFilter)); err != nil {
			return fmt.Errorf("invalid pattern type %q: valid types are logical_gap, scope_violation, circular_reasoning, undefined_term", typeFilter)
		}
	}

	// Load pattern library
	lib, err := patterns.LoadPatternLibrary(dir)
	if err != nil {
		return fmt.Errorf("error loading pattern library: %w", err)
	}

	// Filter patterns if type specified
	var patternList []*patterns.Pattern
	if typeFilter != "" {
		patternList = lib.GetByType(patterns.PatternType(typeFilter))
	} else {
		patternList = lib.Patterns
	}

	// Sort by occurrences (descending)
	sort.Slice(patternList, func(i, j int) bool {
		return patternList[i].Occurrences > patternList[j].Occurrences
	})

	// Output
	if jsonOutput {
		output := patternsListJSON{
			Patterns: make([]patternJSON, len(patternList)),
			Total:    len(patternList),
		}
		for i, p := range patternList {
			output.Patterns[i] = patternJSON{
				Type:        string(p.Type),
				Description: p.Description,
				Example:     p.Example,
				Occurrences: p.Occurrences,
			}
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text format
	if len(patternList) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No patterns found in library.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af patterns extract  - Extract patterns from resolved challenges")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %s\n", "TYPE", "OCCURRENCES", "DESCRIPTION")
	for _, p := range patternList {
		desc := p.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12d %s\n", p.Type, p.Occurrences, desc)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d pattern(s)\n", len(patternList))

	return nil
}

// patternsListJSON is the JSON output format for patterns list.
type patternsListJSON struct {
	Patterns []patternJSON `json:"patterns"`
	Total    int           `json:"total"`
}

// patternJSON is the JSON representation of a pattern.
type patternJSON struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
	Occurrences int    `json:"occurrences"`
}

// newPatternsAnalyzeCmd creates the patterns analyze subcommand.
func newPatternsAnalyzeCmd() *cobra.Command {
	var dir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze current proof for potential issues",
		Long: `Analyze the current proof for potential issues based on known patterns.

This command examines nodes in the proof and identifies potential problems
that match patterns learned from previously resolved challenges.

Issues are ranked by confidence score based on how frequently the
pattern has been observed in the past.

Examples:
  af patterns analyze        Analyze current proof
  af patterns analyze --json Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsAnalyze(cmd, dir, jsonOutput)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// runPatternsAnalyze executes the patterns analyze command.
func runPatternsAnalyze(cmd *cobra.Command, dir string, jsonOutput bool) error {
	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Load pattern library
	lib, err := patterns.LoadPatternLibrary(dir)
	if err != nil {
		return fmt.Errorf("error loading pattern library: %w", err)
	}

	// Create analyzer and analyze state
	analyzer := patterns.NewAnalyzer(lib)
	issues := analyzer.AnalyzeState(st)

	// Output
	if jsonOutput {
		output := analyzeResultJSON{
			Issues: make([]issueJSON, len(issues)),
			Total:  len(issues),
		}
		for i, issue := range issues {
			output.Issues[i] = issueJSON{
				NodeID:      issue.NodeID.String(),
				PatternType: string(issue.PatternType),
				Description: issue.Description,
				Confidence:  issue.Confidence,
			}
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text format
	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No potential issues detected.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "The proof was analyzed against known patterns and no matches were found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %-10s %s\n", "NODE", "PATTERN", "CONFIDENCE", "DESCRIPTION")
	for _, issue := range issues {
		desc := issue.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		confidenceStr := fmt.Sprintf("%.0f%%", issue.Confidence*100)
		fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %-10s %s\n",
			issue.NodeID.String(), issue.PatternType, confidenceStr, desc)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d potential issue(s) detected\n", len(issues))

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  Review flagged nodes to determine if they need refinement")
	fmt.Fprintln(cmd.OutOrStdout(), "  af get <node-id>  - View node details")

	return nil
}

// analyzeResultJSON is the JSON output format for pattern analysis.
type analyzeResultJSON struct {
	Issues []issueJSON `json:"issues"`
	Total  int         `json:"total"`
}

// issueJSON is the JSON representation of a potential issue.
type issueJSON struct {
	NodeID      string  `json:"node_id"`
	PatternType string  `json:"pattern_type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

// newPatternsStatsCmd creates the patterns stats subcommand.
func newPatternsStatsCmd() *cobra.Command {
	var dir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show statistics on common challenge types",
		Long: `Show statistics on the pattern library.

Displays:
  - Total number of patterns
  - Total occurrences across all patterns
  - Breakdown by pattern type

Examples:
  af patterns stats        Show statistics
  af patterns stats --json Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsStats(cmd, dir, jsonOutput)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// runPatternsStats executes the patterns stats command.
func runPatternsStats(cmd *cobra.Command, dir string, jsonOutput bool) error {
	// Verify proof is initialized
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load pattern library
	lib, err := patterns.LoadPatternLibrary(dir)
	if err != nil {
		return fmt.Errorf("error loading pattern library: %w", err)
	}

	stats := lib.Stats()

	// Output
	if jsonOutput {
		output := statsResultJSON{
			TotalPatterns:    stats.TotalPatterns,
			TotalOccurrences: stats.TotalOccurrences,
			ByType:           make(map[string]int),
		}
		for pt, count := range stats.ByType {
			output.ByType[string(pt)] = count
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text format
	fmt.Fprintln(cmd.OutOrStdout(), "Pattern Library Statistics")
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 40))
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintf(cmd.OutOrStdout(), "Total Patterns:    %d\n", stats.TotalPatterns)
	fmt.Fprintf(cmd.OutOrStdout(), "Total Occurrences: %d\n", stats.TotalOccurrences)
	fmt.Fprintln(cmd.OutOrStdout())

	if len(stats.ByType) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Breakdown by Type:")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 40))

		// Sort by occurrences descending
		type typeCount struct {
			pt    patterns.PatternType
			count int
		}
		var typeCounts []typeCount
		for pt, count := range stats.ByType {
			typeCounts = append(typeCounts, typeCount{pt, count})
		}
		sort.Slice(typeCounts, func(i, j int) bool {
			return typeCounts[i].count > typeCounts[j].count
		})

		for _, tc := range typeCounts {
			info, _ := patterns.GetPatternTypeInfo(tc.pt)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %4d  (%s)\n",
				tc.pt, tc.count, info.Description)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No patterns recorded yet.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  af patterns extract  - Extract patterns from resolved challenges")
	}

	return nil
}

// statsResultJSON is the JSON output format for pattern stats.
type statsResultJSON struct {
	TotalPatterns    int            `json:"total_patterns"`
	TotalOccurrences int            `json:"total_occurrences"`
	ByType           map[string]int `json:"by_type"`
}

// newPatternsExtractCmd creates the patterns extract subcommand.
func newPatternsExtractCmd() *cobra.Command {
	var dir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract patterns from resolved challenges",
		Long: `Extract patterns from resolved challenges in the proof.

This command analyzes all resolved challenges and extracts common
mistake patterns to help future provers avoid similar issues.

Patterns are stored in .af/patterns.json and used by the analyze
command to detect potential issues in the proof.

Examples:
  af patterns extract        Extract patterns from challenges
  af patterns extract --json Output results as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsExtract(cmd, dir, jsonOutput)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// runPatternsExtract executes the patterns extract command.
func runPatternsExtract(cmd *cobra.Command, dir string, jsonOutput bool) error {
	// Create proof service
	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	// Check if proof is initialized
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("error checking proof status: %w", err)
	}
	if !status.Initialized {
		return fmt.Errorf("proof not initialized")
	}

	// Load state
	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading proof state: %w", err)
	}

	// Load existing pattern library (or create new one)
	lib, err := patterns.LoadPatternLibrary(dir)
	if err != nil {
		return fmt.Errorf("error loading pattern library: %w", err)
	}

	// Get all challenges (we'll filter for resolved ones)
	challenges := st.AllChallenges()

	// Count resolved challenges
	resolvedCount := 0
	for _, c := range challenges {
		if c.Status == "resolved" {
			resolvedCount++
		}
	}

	// Create analyzer and extract patterns
	analyzer := patterns.NewAnalyzer(lib)
	analyzer.ExtractPatterns(challenges)

	// Save updated library
	if err := lib.Save(dir); err != nil {
		return fmt.Errorf("error saving pattern library: %w", err)
	}

	// Get stats after extraction
	stats := lib.Stats()

	// Output
	if jsonOutput {
		output := extractResultJSON{
			ChallengesAnalyzed: resolvedCount,
			PatternsExtracted:  stats.TotalPatterns,
			TotalOccurrences:   stats.TotalOccurrences,
		}
		data, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text format
	fmt.Fprintln(cmd.OutOrStdout(), "Pattern extraction complete.")
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "Resolved challenges analyzed: %d\n", resolvedCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Patterns in library:          %d\n", stats.TotalPatterns)
	fmt.Fprintf(cmd.OutOrStdout(), "Total occurrences:            %d\n", stats.TotalOccurrences)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Patterns saved to .af/patterns.json")
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  af patterns list     - View extracted patterns")
	fmt.Fprintln(cmd.OutOrStdout(), "  af patterns analyze  - Analyze proof for potential issues")

	return nil
}

// extractResultJSON is the JSON output format for pattern extraction.
type extractResultJSON struct {
	ChallengesAnalyzed int `json:"challenges_analyzed"`
	PatternsExtracted  int `json:"patterns_extracted"`
	TotalOccurrences   int `json:"total_occurrences"`
}

func init() {
	rootCmd.AddCommand(newPatternsCmd())
}
