// Package main provides role-specific help filtering for the af CLI.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// RoleAnnotation is the key used in cobra.Command.Annotations to mark command roles.
const RoleAnnotation = "role"

// Role constants for command categorization.
const (
	RoleProver   = "prover"
	RoleVerifier = "verifier"
	RoleShared   = "shared"   // Both roles use these (claim, release, etc.)
	RoleInfo     = "info"     // Information/reference commands (status, get, etc.)
	RoleOperator = "operator" // Human operator commands (def-add, init, etc.)
)

// commandRoles maps command names to their roles.
// Commands not in this map are shown to all roles.
var commandRoles = map[string]string{
	// Prover-specific commands
	"refine":            RoleProver,
	"amend":             RoleProver,
	"request-def":       RoleProver,
	"resolve-challenge": RoleProver,

	// Verifier-specific commands
	"accept":    RoleVerifier,
	"challenge": RoleVerifier,
	// Note: withdraw-challenge command exists in code but is not registered

	// Shared commands (both roles need these)
	"claim":        RoleShared,
	"release":      RoleShared,
	"extend-claim": RoleShared,
	"jobs":         RoleShared,

	// Escape hatches (typically operator, but sometimes agent-used)
	"admit":   RoleOperator,
	"refute":  RoleOperator,
	"archive": RoleOperator,

	// Information/reference commands
	"status":       RoleInfo,
	"get":          RoleInfo,
	"scope":        RoleInfo,
	"deps":         RoleInfo,
	"challenges":   RoleInfo,
	"defs":         RoleInfo,
	"def":          RoleInfo,
	"assumptions":  RoleInfo,
	"assumption":   RoleInfo,
	"externals":    RoleInfo,
	"external":     RoleInfo,
	"lemmas":       RoleInfo,
	"lemma":        RoleInfo,
	"pending-defs": RoleInfo,
	"pending-def":  RoleInfo,
	"pending-refs": RoleInfo,
	"pending-ref":  RoleInfo,
	"progress":     RoleInfo,
	"health":       RoleInfo,
	"schema":       RoleInfo,
	"inferences":   RoleInfo,
	"types":        RoleInfo,
	"history":      RoleInfo,
	"search":       RoleInfo,
	"log":          RoleInfo,
	"metrics":      RoleInfo,
	"strategy":     RoleInfo,
	"patterns":     RoleInfo,
	"agents":       RoleInfo,

	// Operator/admin commands
	"init":            RoleOperator,
	"def-add":         RoleOperator,
	"def-reject":      RoleOperator,
	"add-external":    RoleOperator,
	"verify-external": RoleOperator,
	"extract-lemma":   RoleOperator,
	"replay":          RoleOperator,
	"reap":            RoleOperator,
	"recompute-taint": RoleOperator,
	"export":          RoleOperator,
	"hooks":           RoleOperator,
	"watch":           RoleOperator,

	// Interactive/guidance commands
	"shell":    RoleShared,
	"wizard":   RoleShared,
	"tutorial": RoleShared,

	// Utility commands (always shown)
	"version":    "",
	"completion": "",
	"help":       "",
}

// newHelpCmd creates a custom help command with role filtering.
func newHelpCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.

Use --role to filter commands by agent role:
  af help --role prover     Show prover-relevant commands
  af help --role verifier   Show verifier-relevant commands

Without --role, all commands are shown.

Examples:
  af help                   Show all commands
  af help --role prover     Show only prover commands
  af help --role verifier   Show only verifier commands
  af help refine            Show help for the refine command`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHelp(cmd, args, role)
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "Filter commands by role (prover or verifier)")

	return cmd
}

// runHelp executes the help command with optional role filtering.
func runHelp(cmd *cobra.Command, args []string, role string) error {
	parent := cmd.Parent()
	if parent == nil {
		return fmt.Errorf("no parent command")
	}

	// Validate role if provided
	role = strings.ToLower(role)
	if role != "" && role != RoleProver && role != RoleVerifier {
		return fmt.Errorf("invalid role %q: must be 'prover' or 'verifier'", role)
	}

	// If a specific command is requested, show its help
	if len(args) > 0 {
		targetCmd, _, err := parent.Find(args)
		if err != nil || targetCmd == nil {
			return fmt.Errorf("unknown command %q", args[0])
		}
		return targetCmd.Help()
	}

	// Show filtered command list
	return showFilteredHelp(cmd, parent, role)
}

// showFilteredHelp displays the help with commands filtered by role.
func showFilteredHelp(cmd *cobra.Command, root *cobra.Command, role string) error {
	out := cmd.OutOrStdout()

	// Print header
	fmt.Fprintln(out, root.Long)
	fmt.Fprintln(out)

	if role != "" {
		fmt.Fprintf(out, "Commands for %s role:\n\n", role)
	} else {
		fmt.Fprintln(out, "Available Commands:")
		fmt.Fprintln(out)
	}

	// Collect and categorize commands
	var proverCmds, verifierCmds, sharedCmds, infoCmds, operatorCmds, otherCmds []*cobra.Command

	for _, sub := range root.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}

		cmdRole := commandRoles[sub.Name()]
		switch cmdRole {
		case RoleProver:
			proverCmds = append(proverCmds, sub)
		case RoleVerifier:
			verifierCmds = append(verifierCmds, sub)
		case RoleShared:
			sharedCmds = append(sharedCmds, sub)
		case RoleInfo:
			infoCmds = append(infoCmds, sub)
		case RoleOperator:
			operatorCmds = append(operatorCmds, sub)
		default:
			otherCmds = append(otherCmds, sub)
		}
	}

	// Sort each category
	sortCmds := func(cmds []*cobra.Command) {
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name() < cmds[j].Name()
		})
	}
	sortCmds(proverCmds)
	sortCmds(verifierCmds)
	sortCmds(sharedCmds)
	sortCmds(infoCmds)
	sortCmds(operatorCmds)
	sortCmds(otherCmds)

	// Print based on role filter
	switch role {
	case RoleProver:
		printSection(out, "Prover Actions", proverCmds)
		printSection(out, "Job Management", sharedCmds)
		printSection(out, "Reference", infoCmds)
	case RoleVerifier:
		printSection(out, "Verifier Actions", verifierCmds)
		printSection(out, "Job Management", sharedCmds)
		printSection(out, "Reference", infoCmds)
	default:
		printSection(out, "Prover Actions", proverCmds)
		printSection(out, "Verifier Actions", verifierCmds)
		printSection(out, "Job Management", sharedCmds)
		printSection(out, "Reference", infoCmds)
		printSection(out, "Administration", operatorCmds)
		if len(otherCmds) > 0 {
			printSection(out, "Other", otherCmds)
		}
	}

	// Print footer
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintln(out, "      --dry-run   Preview changes without making them")
	fmt.Fprintln(out, "  -h, --help      help for af")
	fmt.Fprintln(out, "      --verbose   Enable verbose output for debugging")
	fmt.Fprintln(out, "  -v, --version   version for af")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Use \"af [command] --help\" for more information about a command.")
	if role == "" {
		fmt.Fprintln(out, "Use \"af help --role prover\" or \"af help --role verifier\" to see role-specific commands.")
	}

	return nil
}

// printSection prints a section of commands with a header.
func printSection(out interface{ Write([]byte) (int, error) }, header string, cmds []*cobra.Command) {
	if len(cmds) == 0 {
		return
	}

	fmt.Fprintf(out, "  %s:\n", header)
	for _, c := range cmds {
		fmt.Fprintf(out, "    %-18s %s\n", c.Name(), c.Short)
	}
	fmt.Fprintln(out)
}

func init() {
	// Replace the default help command with our custom one
	rootCmd.SetHelpCommand(newHelpCmd())
}
