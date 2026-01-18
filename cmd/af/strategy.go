// Package main contains the af strategy command for proof structure and strategy guidance.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newStrategyCmd creates the strategy command with subcommands.
func newStrategyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "strategy",
		GroupID: GroupUtil,
		Short:   "Proof structure and strategy guidance",
		Long: `Provides guidance on proof strategies and structure.

The strategy command helps provers plan their proofs by:
  - Listing available proof strategies (direct, contradiction, induction, etc.)
  - Suggesting strategies based on conjecture analysis
  - Generating proof skeletons using a chosen strategy

This command works without an initialized proof directory because
it provides static planning and reference information.

Subcommands:
  list     Show all available proof strategies
  suggest  Analyze a conjecture and suggest strategies
  apply    Generate a proof skeleton using a strategy

Examples:
  af strategy list                                  List all strategies
  af strategy suggest "For all n, P(n)"            Get suggestions for a conjecture
  af strategy apply induction "For all n, P(n)"   Generate an induction skeleton`,
	}

	// Add subcommands
	cmd.AddCommand(newStrategyListCmd())
	cmd.AddCommand(newStrategySuggestCmd())
	cmd.AddCommand(newStrategyApplyCmd())

	return cmd
}

// newStrategyListCmd creates the 'strategy list' subcommand.
func newStrategyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show available proof strategies",
		Long: `Display all available proof strategies with their descriptions.

Each strategy includes:
  - Name: The strategy identifier
  - Description: When to use this strategy
  - Steps: The required structure for this strategy

This helps provers understand the available approaches before
beginning their proof work.

Examples:
  af strategy list              List all strategies in text format
  af strategy list --format json   Output in JSON format`,
		RunE: runStrategyList,
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// newStrategySuggestCmd creates the 'strategy suggest' subcommand.
func newStrategySuggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest <conjecture>",
		Short: "Analyze a conjecture and suggest strategies",
		Long: `Analyze a conjecture and suggest appropriate proof strategies.

The suggest command examines the structure of a conjecture and recommends
strategies based on common patterns:
  - Universal quantification over numbers suggests induction
  - Non-existence claims suggest proof by contradiction
  - Disjunctions suggest case analysis
  - Implications suggest contrapositive or direct proof

Each suggestion includes a reason explaining why that strategy
may be appropriate for the given conjecture.

Examples:
  af strategy suggest "For all n, n + 0 = n"
  af strategy suggest "There is no largest prime"
  af strategy suggest --format json "If P then Q"`,
		Args: cobra.ExactArgs(1),
		RunE: runStrategySuggest,
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	return cmd
}

// newStrategyApplyCmd creates the 'strategy apply' subcommand.
func newStrategyApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <strategy> <conjecture>",
		Short: "Generate a proof skeleton using a strategy",
		Long: `Generate a proof skeleton for a conjecture using the specified strategy.

The apply command creates a structured outline for your proof based on
the chosen strategy. The skeleton includes:
  - The proof goal (your conjecture)
  - Required steps for the strategy
  - Template text for each step

Available strategies:
  direct        - Direct proof by logical deduction
  contradiction - Assume negation, derive contradiction
  induction     - Base case + inductive step
  cases         - Exhaustive case analysis
  contrapositive - Prove the contrapositive

Examples:
  af strategy apply induction "For all n, P(n)"
  af strategy apply contradiction "There is no largest prime"
  af strategy apply cases "Either A or B"`,
		Args: cobra.ExactArgs(2),
		RunE: runStrategyApply,
	}

	return cmd
}

// runStrategyList executes the 'strategy list' subcommand.
func runStrategyList(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	format = strings.ToLower(format)

	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	strategies := service.AllStrategies()
	out := cmd.OutOrStdout()

	if format == "json" {
		return outputStrategyListJSON(cmd, strategies)
	}

	// Text format
	fmt.Fprintln(out, "=== Proof Strategies ===")
	fmt.Fprintln(out)

	for _, s := range strategies {
		fmt.Fprintf(out, "%s\n", strings.ToUpper(s.Name))
		fmt.Fprintf(out, "  %s\n", s.Description)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Steps:")
		for i, step := range s.Steps {
			fmt.Fprintf(out, "    %d. %s\n", i+1, step.Description)
		}
		fmt.Fprintln(out)
	}

	return nil
}

// strategyListJSON represents the JSON output for strategy list.
type strategyListJSON struct {
	Strategies []strategyJSON `json:"strategies"`
}

type strategyJSON struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Steps       []stepJSON `json:"steps"`
	Example     string     `json:"example"`
}

type stepJSON struct {
	Description string `json:"description"`
	Template    string `json:"template"`
}

// outputStrategyListJSON outputs strategies in JSON format.
func outputStrategyListJSON(cmd *cobra.Command, strategies []service.Strategy) error {
	output := strategyListJSON{
		Strategies: make([]strategyJSON, len(strategies)),
	}

	for i, s := range strategies {
		output.Strategies[i] = strategyJSON{
			Name:        s.Name,
			Description: s.Description,
			Steps:       make([]stepJSON, len(s.Steps)),
			Example:     s.Example,
		}
		for j, step := range s.Steps {
			output.Strategies[i].Steps[j] = stepJSON{
				Description: step.Description,
				Template:    step.Template,
			}
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// runStrategySuggest executes the 'strategy suggest' subcommand.
func runStrategySuggest(cmd *cobra.Command, args []string) error {
	conjecture := args[0]
	format, _ := cmd.Flags().GetString("format")
	format = strings.ToLower(format)

	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}

	suggestions := service.SuggestStrategies(conjecture)
	out := cmd.OutOrStdout()

	if format == "json" {
		return outputSuggestJSON(cmd, conjecture, suggestions)
	}

	// Text format
	fmt.Fprintln(out, "=== Strategy Suggestions ===")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Conjecture: %s\n\n", conjecture)

	if len(suggestions) == 0 {
		fmt.Fprintln(out, "No specific strategy suggested. Consider using direct proof.")
		return nil
	}

	fmt.Fprintln(out, "Suggested strategies (in order of confidence):")
	fmt.Fprintln(out)

	for i, s := range suggestions {
		fmt.Fprintf(out, "%d. %s (confidence: %.0f%%)\n", i+1, strings.ToUpper(s.Strategy.Name), s.Confidence*100)
		fmt.Fprintf(out, "   Reason: %s\n\n", s.Reason)
	}

	fmt.Fprintln(out, "Next step: Run 'af strategy apply <strategy> \"<conjecture>\"' to generate a skeleton.")

	return nil
}

// suggestJSON represents the JSON output for strategy suggest.
type suggestJSON struct {
	Conjecture  string           `json:"conjecture"`
	Suggestions []suggestionJSON `json:"suggestions"`
}

type suggestionJSON struct {
	Strategy   string  `json:"strategy"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// outputSuggestJSON outputs suggestions in JSON format.
func outputSuggestJSON(cmd *cobra.Command, conjecture string, suggestions []service.StrategySuggestion) error {
	output := suggestJSON{
		Conjecture:  conjecture,
		Suggestions: make([]suggestionJSON, len(suggestions)),
	}

	for i, s := range suggestions {
		output.Suggestions[i] = suggestionJSON{
			Strategy:   s.Strategy.Name,
			Reason:     s.Reason,
			Confidence: s.Confidence,
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// runStrategyApply executes the 'strategy apply' subcommand.
func runStrategyApply(cmd *cobra.Command, args []string) error {
	strategyName := args[0]
	conjecture := args[1]

	s, ok := service.GetStrategy(strategyName)
	if !ok {
		// List available strategies in error message
		names := service.StrategyNames()
		return fmt.Errorf("unknown strategy %q: valid strategies are %s", strategyName, strings.Join(names, ", "))
	}

	skeleton := s.GenerateSkeleton(conjecture)
	fmt.Fprint(cmd.OutOrStdout(), skeleton)

	return nil
}

func init() {
	rootCmd.AddCommand(newStrategyCmd())
}
