//go:build !integration

package shell

import (
	"bytes"
	"strings"
	"testing"
)

// =============================================================================
// Shell Struct Tests
// =============================================================================

// TestNewShell_DefaultPrompt verifies default prompt is set.
func TestNewShell_DefaultPrompt(t *testing.T) {
	s := New(nil)

	if s.Prompt != "af> " {
		t.Errorf("expected default prompt 'af> ', got %q", s.Prompt)
	}
}

// TestNewShell_CustomPrompt verifies custom prompt can be set.
func TestNewShell_CustomPrompt(t *testing.T) {
	s := New(nil, WithPrompt("custom> "))

	if s.Prompt != "custom> " {
		t.Errorf("expected custom prompt 'custom> ', got %q", s.Prompt)
	}
}

// TestNewShell_CustomIO verifies custom I/O can be set.
func TestNewShell_CustomIO(t *testing.T) {
	in := strings.NewReader("exit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	if s.input == nil {
		t.Error("expected input to be set")
	}
	if s.output == nil {
		t.Error("expected output to be set")
	}
}

// =============================================================================
// ParseLine Tests
// =============================================================================

// TestParseLine verifies command line parsing.
func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantCmd  string
		wantArgs []string
	}{
		{
			name:     "simple command",
			line:     "status",
			wantCmd:  "status",
			wantArgs: []string{},
		},
		{
			name:     "command with args",
			line:     "claim 1.1 prover",
			wantCmd:  "claim",
			wantArgs: []string{"1.1", "prover"},
		},
		{
			name:     "command with flags",
			line:     "status --format json",
			wantCmd:  "status",
			wantArgs: []string{"--format", "json"},
		},
		{
			name:     "empty line",
			line:     "",
			wantCmd:  "",
			wantArgs: nil,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			wantCmd:  "",
			wantArgs: nil,
		},
		{
			name:     "extra whitespace",
			line:     "  status   --format   json  ",
			wantCmd:  "status",
			wantArgs: []string{"--format", "json"},
		},
		{
			name:     "help builtin",
			line:     "help",
			wantCmd:  "help",
			wantArgs: []string{},
		},
		{
			name:     "exit builtin",
			line:     "exit",
			wantCmd:  "exit",
			wantArgs: []string{},
		},
		{
			name:     "quit builtin",
			line:     "quit",
			wantCmd:  "quit",
			wantArgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args := ParseLine(tt.line)

			if cmd != tt.wantCmd {
				t.Errorf("ParseLine(%q) cmd = %q, want %q", tt.line, cmd, tt.wantCmd)
			}

			if tt.wantArgs == nil {
				if args != nil {
					t.Errorf("ParseLine(%q) args = %v, want nil", tt.line, args)
				}
			} else {
				if len(args) != len(tt.wantArgs) {
					t.Errorf("ParseLine(%q) args len = %d, want %d", tt.line, len(args), len(tt.wantArgs))
				} else {
					for i, arg := range args {
						if arg != tt.wantArgs[i] {
							t.Errorf("ParseLine(%q) args[%d] = %q, want %q", tt.line, i, arg, tt.wantArgs[i])
						}
					}
				}
			}
		})
	}
}

// =============================================================================
// Built-in Command Tests
// =============================================================================

// TestIsBuiltin verifies built-in command detection.
func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"help", true},
		{"exit", true},
		{"quit", true},
		{"status", false},
		{"claim", false},
		{"", false},
		{"HELP", true},   // case insensitive
		{"EXIT", true},   // case insensitive
		{"Help", true},   // case insensitive
		{"ExIt", true},   // case insensitive
		{"QUIT", true},   // case insensitive
		{"QuIt", true},   // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := IsBuiltin(tt.cmd)
			if got != tt.want {
				t.Errorf("IsBuiltin(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// TestIsExitCommand verifies exit command detection.
func TestIsExitCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"exit", true},
		{"quit", true},
		{"EXIT", true},
		{"QUIT", true},
		{"help", false},
		{"status", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := IsExitCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("IsExitCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Execute Tests
// =============================================================================

// TestShell_ExecuteBuiltinHelp verifies help command execution.
func TestShell_ExecuteBuiltinHelp(t *testing.T) {
	out := &bytes.Buffer{}
	s := New(nil, WithOutput(out))

	err := s.ExecuteBuiltin("help", nil)
	if err != nil {
		t.Errorf("ExecuteBuiltin(help) returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "help") {
		t.Errorf("help output should mention 'help', got: %s", output)
	}
	if !strings.Contains(output, "exit") {
		t.Errorf("help output should mention 'exit', got: %s", output)
	}
}

// TestShell_ExecuteBuiltinExit verifies exit command returns ErrExit.
func TestShell_ExecuteBuiltinExit(t *testing.T) {
	out := &bytes.Buffer{}
	s := New(nil, WithOutput(out))

	err := s.ExecuteBuiltin("exit", nil)
	if err != ErrExit {
		t.Errorf("ExecuteBuiltin(exit) should return ErrExit, got: %v", err)
	}
}

// TestShell_ExecuteBuiltinQuit verifies quit command returns ErrExit.
func TestShell_ExecuteBuiltinQuit(t *testing.T) {
	out := &bytes.Buffer{}
	s := New(nil, WithOutput(out))

	err := s.ExecuteBuiltin("quit", nil)
	if err != ErrExit {
		t.Errorf("ExecuteBuiltin(quit) should return ErrExit, got: %v", err)
	}
}

// =============================================================================
// Run Tests (Integration-style)
// =============================================================================

// TestShell_RunWithExit verifies shell exits on exit command.
func TestShell_RunWithExit(t *testing.T) {
	in := strings.NewReader("exit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "af> ") {
		t.Errorf("output should contain prompt 'af> ', got: %s", output)
	}
}

// TestShell_RunWithQuit verifies shell exits on quit command.
func TestShell_RunWithQuit(t *testing.T) {
	in := strings.NewReader("quit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

// TestShell_RunWithEOF verifies shell exits on EOF.
func TestShell_RunWithEOF(t *testing.T) {
	in := strings.NewReader("") // Empty input = EOF
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

// TestShell_RunWithHelp verifies help command works in REPL.
func TestShell_RunWithHelp(t *testing.T) {
	in := strings.NewReader("help\nexit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Available commands") {
		t.Errorf("output should contain help text, got: %s", output)
	}
}

// TestShell_RunWithEmptyLines verifies empty lines are ignored.
func TestShell_RunWithEmptyLines(t *testing.T) {
	in := strings.NewReader("\n\n\nexit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	// Should have printed prompt multiple times
	output := out.String()
	promptCount := strings.Count(output, "af> ")
	if promptCount < 4 {
		t.Errorf("expected at least 4 prompts, got %d in output: %s", promptCount, output)
	}
}

// TestShell_RunWithCustomPrompt verifies custom prompt is used.
func TestShell_RunWithCustomPrompt(t *testing.T) {
	in := strings.NewReader("exit\n")
	out := &bytes.Buffer{}

	s := New(nil, WithInput(in), WithOutput(out), WithPrompt("test> "))

	err := s.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "test> ") {
		t.Errorf("output should contain custom prompt 'test> ', got: %s", output)
	}
}

// =============================================================================
// Command Execution Tests (with mock RootCmd)
// =============================================================================

// mockCmd creates a simple test command.
func mockCmd(name string, run func() error) *testCommand {
	return &testCommand{name: name, run: run}
}

type testCommand struct {
	name string
	run  func() error
}

// TestShell_ExecuteCommand verifies af subcommands are called.
func TestShell_ExecuteCommand(t *testing.T) {
	executed := false

	// Create a shell with a mock executor
	out := &bytes.Buffer{}
	s := New(nil, WithOutput(out))

	// Override the executor for testing
	s.Executor = func(args []string) error {
		if len(args) > 0 && args[0] == "status" {
			executed = true
		}
		return nil
	}

	err := s.Execute("status")
	if err != nil {
		t.Errorf("Execute(status) returned error: %v", err)
	}

	if !executed {
		t.Error("expected executor to be called with 'status'")
	}
}

// TestShell_ExecuteCommandWithArgs verifies args are passed correctly.
func TestShell_ExecuteCommandWithArgs(t *testing.T) {
	var capturedArgs []string

	out := &bytes.Buffer{}
	s := New(nil, WithOutput(out))

	s.Executor = func(args []string) error {
		capturedArgs = args
		return nil
	}

	err := s.Execute("status --format json")
	if err != nil {
		t.Errorf("Execute() returned error: %v", err)
	}

	expected := []string{"status", "--format", "json"}
	if len(capturedArgs) != len(expected) {
		t.Errorf("expected args %v, got %v", expected, capturedArgs)
	} else {
		for i, arg := range expected {
			if capturedArgs[i] != arg {
				t.Errorf("expected args[%d] = %q, got %q", i, arg, capturedArgs[i])
			}
		}
	}
}
