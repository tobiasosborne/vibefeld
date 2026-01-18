package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/service"
)

// newShellCmd creates the shell command.
// It takes the root command to pass subcommands to the shell executor.
func newShellCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "shell",
		GroupID: GroupUtil,
		Aliases: []string{"repl"},
		Short:   "Start an interactive shell session",
		Long: `Start an interactive shell session for running af commands.

The shell provides a REPL (Read-Eval-Print Loop) where you can run
af commands without typing 'af' each time.

Built-in commands:
  help        Show shell help
  exit, quit  Exit the shell

Examples:
  af shell                    Start interactive shell with default prompt
  af shell --prompt "proof> " Start with custom prompt

In the shell:
  af> status                  Runs 'af status'
  af> claim 1.1 prover        Runs 'af claim 1.1 prover'
  af> help                    Shows shell help
  af> exit                    Exits the shell`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(cmd, root)
		},
	}

	cmd.Flags().StringP("prompt", "p", "af> ", "Shell prompt string")

	return cmd
}

// runShell executes the interactive shell.
func runShell(cmd *cobra.Command, root *cobra.Command) error {
	prompt, _ := cmd.Flags().GetString("prompt")

	// Create shell with executor that runs af subcommands
	s := service.NewShell(
		root,
		service.ShellWithPrompt(prompt),
		service.ShellWithInput(os.Stdin),
		service.ShellWithOutput(cmd.OutOrStdout()),
		service.ShellWithExecutor(func(args []string) error {
			return executeSubcommand(root, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
		}),
	)

	// Print welcome message
	fmt.Fprintln(cmd.OutOrStdout(), "AF Interactive Shell - Type 'help' for commands, 'exit' to quit")
	fmt.Fprintln(cmd.OutOrStdout())

	return s.Run()
}

// executeSubcommand runs an af subcommand by setting args on the root command.
func executeSubcommand(root *cobra.Command, args []string, stdout, stderr io.Writer) error {
	// Create a new buffer for output to avoid polluting the shell
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	// Save and restore the root's output
	origOut := root.OutOrStdout()
	origErr := root.ErrOrStderr()
	defer func() {
		root.SetOut(origOut)
		root.SetErr(origErr)
	}()

	root.SetOut(outBuf)
	root.SetErr(errBuf)

	// Set args and execute
	root.SetArgs(args)
	err := root.Execute()

	// Print output
	if outBuf.Len() > 0 {
		fmt.Fprint(stdout, outBuf.String())
	}
	if errBuf.Len() > 0 {
		fmt.Fprint(stderr, errBuf.String())
	}

	return err
}

func init() {
	rootCmd.AddCommand(newShellCmd(rootCmd))
}
