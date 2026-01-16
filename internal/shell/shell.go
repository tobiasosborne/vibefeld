// Package shell provides an interactive REPL for the af CLI.
package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrExit is returned when the user requests to exit the shell.
var ErrExit = errors.New("exit requested")

// Shell represents an interactive shell session.
type Shell struct {
	// Prompt is the string displayed before each input line.
	Prompt string

	// Executor is the function called to execute af subcommands.
	// If nil, commands are passed to the default cobra root command.
	Executor func(args []string) error

	input   io.Reader
	output  io.Writer
	scanner *bufio.Scanner
}

// Option is a functional option for configuring a Shell.
type Option func(*Shell)

// WithPrompt sets a custom prompt for the shell.
func WithPrompt(prompt string) Option {
	return func(s *Shell) {
		s.Prompt = prompt
	}
}

// WithInput sets the input reader for the shell.
func WithInput(r io.Reader) Option {
	return func(s *Shell) {
		s.input = r
	}
}

// WithOutput sets the output writer for the shell.
func WithOutput(w io.Writer) Option {
	return func(s *Shell) {
		s.output = w
	}
}

// WithExecutor sets the command executor function.
func WithExecutor(exec func(args []string) error) Option {
	return func(s *Shell) {
		s.Executor = exec
	}
}

// New creates a new Shell with the given options.
// The rootCmd parameter is the cobra root command (can be nil for testing).
func New(rootCmd interface{}, opts ...Option) *Shell {
	s := &Shell{
		Prompt: "af> ",
		input:  os.Stdin,
		output: os.Stdout,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the interactive shell loop.
// It returns nil on normal exit (exit/quit command or EOF).
func (s *Shell) Run() error {
	s.scanner = bufio.NewScanner(s.input)

	for {
		// Print prompt
		fmt.Fprint(s.output, s.Prompt)

		// Read line
		if !s.scanner.Scan() {
			// EOF or error
			if err := s.scanner.Err(); err != nil {
				return fmt.Errorf("read error: %w", err)
			}
			// EOF - normal exit
			fmt.Fprintln(s.output)
			return nil
		}

		line := s.scanner.Text()

		// Execute command
		err := s.Execute(line)
		if err == ErrExit {
			return nil
		}
		if err != nil {
			fmt.Fprintf(s.output, "Error: %v\n", err)
		}
	}
}

// Execute parses and executes a single command line.
func (s *Shell) Execute(line string) error {
	cmd, args := ParseLine(line)

	// Skip empty lines
	if cmd == "" {
		return nil
	}

	// Handle built-in commands
	if IsBuiltin(cmd) {
		return s.ExecuteBuiltin(cmd, args)
	}

	// Execute af subcommand
	if s.Executor != nil {
		fullArgs := append([]string{cmd}, args...)
		return s.Executor(fullArgs)
	}

	return fmt.Errorf("no executor configured for command: %s", cmd)
}

// ExecuteBuiltin executes a built-in shell command.
func (s *Shell) ExecuteBuiltin(cmd string, args []string) error {
	cmd = strings.ToLower(cmd)

	switch cmd {
	case "help":
		s.printHelp()
		return nil
	case "exit", "quit":
		return ErrExit
	default:
		return fmt.Errorf("unknown builtin command: %s", cmd)
	}
}

// printHelp prints the shell help message.
func (s *Shell) printHelp() {
	help := `Interactive AF Shell

Available commands:
  help        Show this help message
  exit, quit  Exit the shell

All other commands are passed to af. For example:
  status              Run 'af status'
  claim 1.1 prover    Run 'af claim 1.1 prover'
  jobs --format json  Run 'af jobs --format json'

Type any af command without the 'af' prefix.
`
	fmt.Fprint(s.output, help)
}

// ParseLine splits a command line into command and arguments.
// Returns empty command for empty/whitespace-only lines.
func ParseLine(line string) (cmd string, args []string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", nil
	}

	cmd = parts[0]
	if len(parts) > 1 {
		args = parts[1:]
	} else {
		args = []string{}
	}

	return cmd, args
}

// IsBuiltin returns true if the command is a built-in shell command.
func IsBuiltin(cmd string) bool {
	cmd = strings.ToLower(cmd)
	switch cmd {
	case "help", "exit", "quit":
		return true
	default:
		return false
	}
}

// IsExitCommand returns true if the command is an exit command.
func IsExitCommand(cmd string) bool {
	cmd = strings.ToLower(cmd)
	return cmd == "exit" || cmd == "quit"
}
