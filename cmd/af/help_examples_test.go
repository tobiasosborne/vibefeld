//go:build !integration

package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// extractAFExamples returns every `af ...` command example found in a help
// string, as the token list following the leading `af`. Leading `VAR=value`
// environment assignments (e.g. `AF_AGENT_ID=prover-1 af ...`) are skipped so
// those examples are still checked. Whitespace splitting is deliberate: the
// test only needs to catch unknown flags, and quoting never hides a flag name.
func extractAFExamples(help string) [][]string {
	var out [][]string
	for _, line := range strings.Split(help, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		i := 0
		for i < len(fields) && strings.Contains(fields[i], "=") && !strings.HasPrefix(fields[i], "-") {
			i++
		}
		if i >= len(fields) || fields[i] != "af" {
			continue
		}
		if rest := fields[i+1:]; len(rest) > 0 {
			out = append(out, rest)
		}
	}
	return out
}

func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, fn)
	}
}

// TestHelpExamplesUseKnownFlags parses every `af ...` example in `af --help`
// and every subcommand's `--help` through cobra's own flag parser, so a help
// example can never name a flag the command does not define. This is the D11
// review guard against the `resolve-challenge --owner` class of stale example.
func TestHelpExamplesUseKnownFlags(t *testing.T) {
	if len(rootCmd.Commands()) == 0 {
		t.Fatal("root command has no registered subcommands")
	}

	checked := 0
	walkCommands(rootCmd, func(c *cobra.Command) {
		help := c.Long
		if help == "" {
			help = c.Short
		}
		for _, example := range extractAFExamples(help) {
			target, args, err := rootCmd.Find(example)
			if err != nil || target == nil {
				// Not an af command we can resolve (e.g. prose that merely
				// starts with "af"); skip.
				continue
			}
			// ParseFlags deliberately ignores positional args, so it only
			// reports unknown/malformed flags.
			if err := target.ParseFlags(args); err != nil {
				t.Errorf("help example %q in %q does not parse: %v",
					strings.Join(append([]string{"af"}, example...), " "), c.CommandPath(), err)
			}
			checked++
		}
	})

	if checked == 0 {
		t.Fatal("no help examples were parsed; extraction is broken")
	}
}
