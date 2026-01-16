package cli

import (
	"strings"
	"testing"
)

func TestArgSpec_Validation(t *testing.T) {
	tests := []struct {
		name string
		spec ArgSpec
	}{
		{
			name: "complete spec with examples",
			spec: ArgSpec{
				Name:        "node-id",
				Description: "The ID of the node to claim",
				Examples:    []string{"1", "1.2", "1.2.3"},
				Required:    true,
			},
		},
		{
			name: "spec without examples",
			spec: ArgSpec{
				Name:        "message",
				Description: "The challenge message",
				Examples:    nil,
				Required:    true,
			},
		},
		{
			name: "optional spec",
			spec: ArgSpec{
				Name:        "format",
				Description: "Output format",
				Examples:    []string{"json", "text"},
				Required:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ArgSpec should be constructible
			spec := tt.spec
			if spec.Name == "" {
				t.Error("Name should not be empty")
			}
		})
	}
}

func TestMissingArgError_Error(t *testing.T) {
	tests := []struct {
		name        string
		err         *MissingArgError
		wantContain string
	}{
		{
			name: "basic error message",
			err: &MissingArgError{
				Command: "claim",
				Arg: ArgSpec{
					Name:        "node-id",
					Description: "The ID of the node to claim",
					Examples:    []string{"1", "1.2"},
					Required:    true,
				},
			},
			wantContain: "node-id",
		},
		{
			name: "error includes command",
			err: &MissingArgError{
				Command: "challenge",
				Arg: ArgSpec{
					Name:        "message",
					Description: "The challenge message",
					Examples:    nil,
					Required:    true,
				},
			},
			wantContain: "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			if errStr == "" {
				t.Error("Error() returned empty string")
			}
			if !strings.Contains(errStr, tt.wantContain) {
				t.Errorf("Error() = %q, want to contain %q", errStr, tt.wantContain)
			}
		})
	}
}

func TestMissingArgError_HelpText(t *testing.T) {
	tests := []struct {
		name         string
		err          *MissingArgError
		wantContains []string
	}{
		{
			name: "help text with examples",
			err: &MissingArgError{
				Command: "claim",
				Arg: ArgSpec{
					Name:        "node-id",
					Description: "The ID of the node to claim",
					Examples:    []string{"1", "1.2", "1.2.3"},
					Required:    true,
				},
			},
			wantContains: []string{
				"Missing required argument: node-id",
				"The ID of the node to claim",
				"Examples:",
				"af claim 1",
				"af claim 1.2",
				"af claim 1.2.3",
			},
		},
		{
			name: "help text without examples",
			err: &MissingArgError{
				Command: "refine",
				Arg: ArgSpec{
					Name:        "content",
					Description: "The proof step content",
					Examples:    nil,
					Required:    true,
				},
			},
			wantContains: []string{
				"Missing required argument: content",
				"The proof step content",
			},
		},
		{
			name: "help text single example",
			err: &MissingArgError{
				Command: "status",
				Arg: ArgSpec{
					Name:        "node-id",
					Description: "Node to show status for",
					Examples:    []string{"1"},
					Required:    true,
				},
			},
			wantContains: []string{
				"Missing required argument: node-id",
				"Node to show status for",
				"Examples:",
				"af status 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpText := tt.err.HelpText()
			if helpText == "" {
				t.Error("HelpText() returned empty string")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(helpText, want) {
					t.Errorf("HelpText() = %q, want to contain %q", helpText, want)
				}
			}
		})
	}
}

func TestMissingArgError_HelpText_NoExamplesSection(t *testing.T) {
	// When there are no examples, "Examples:" section should not appear
	err := &MissingArgError{
		Command: "test",
		Arg: ArgSpec{
			Name:        "arg",
			Description: "Test argument",
			Examples:    nil,
			Required:    true,
		},
	}

	helpText := err.HelpText()
	if strings.Contains(helpText, "Examples:") {
		t.Errorf("HelpText() should not contain 'Examples:' when no examples provided, got %q", helpText)
	}
}

func TestCheckRequiredArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		specs   []ArgSpec
		wantErr bool
		wantArg string // The name of the missing arg, if error expected
	}{
		{
			name: "all required args present",
			args: []string{"1.2"},
			specs: []ArgSpec{
				{Name: "node-id", Description: "Node ID", Required: true},
			},
			wantErr: false,
		},
		{
			name: "one required arg missing",
			args: []string{},
			specs: []ArgSpec{
				{Name: "node-id", Description: "Node ID", Required: true},
			},
			wantErr: true,
			wantArg: "node-id",
		},
		{
			name: "multiple required args first missing",
			args: []string{},
			specs: []ArgSpec{
				{Name: "node-id", Description: "Node ID", Required: true},
				{Name: "message", Description: "Message", Required: true},
			},
			wantErr: true,
			wantArg: "node-id",
		},
		{
			name: "multiple required args second missing",
			args: []string{"1.2"},
			specs: []ArgSpec{
				{Name: "node-id", Description: "Node ID", Required: true},
				{Name: "message", Description: "Message", Required: true},
			},
			wantErr: true,
			wantArg: "message",
		},
		{
			name:    "no required args",
			args:    []string{},
			specs:   []ArgSpec{},
			wantErr: false,
		},
		{
			name: "empty args with required",
			args: []string{},
			specs: []ArgSpec{
				{Name: "target", Description: "Target", Required: true},
			},
			wantErr: true,
			wantArg: "target",
		},
		{
			name: "optional arg missing is ok",
			args: []string{},
			specs: []ArgSpec{
				{Name: "format", Description: "Format", Required: false},
			},
			wantErr: false,
		},
		{
			name: "required then optional - required present",
			args: []string{"value1"},
			specs: []ArgSpec{
				{Name: "required-arg", Description: "Required", Required: true},
				{Name: "optional-arg", Description: "Optional", Required: false},
			},
			wantErr: false,
		},
		{
			name: "more args than specs is ok",
			args: []string{"arg1", "arg2", "arg3"},
			specs: []ArgSpec{
				{Name: "first", Description: "First", Required: true},
			},
			wantErr: false,
		},
		{
			name: "nil specs",
			args: []string{"something"},
			specs: nil,
			wantErr: false,
		},
		{
			name: "nil args with required",
			args: nil,
			specs: []ArgSpec{
				{Name: "target", Description: "Target", Required: true},
			},
			wantErr: true,
			wantArg: "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRequiredArgs(tt.args, tt.specs)
			if tt.wantErr {
				if err == nil {
					t.Error("CheckRequiredArgs() returned nil, want error")
					return
				}
				if err.Arg.Name != tt.wantArg {
					t.Errorf("CheckRequiredArgs() error arg = %q, want %q", err.Arg.Name, tt.wantArg)
				}
			} else {
				if err != nil {
					t.Errorf("CheckRequiredArgs() returned error %v, want nil", err)
				}
			}
		})
	}
}

func TestFormatArgHelp(t *testing.T) {
	tests := []struct {
		name         string
		spec         ArgSpec
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "with examples",
			spec: ArgSpec{
				Name:        "node-id",
				Description: "The node identifier",
				Examples:    []string{"1", "1.2", "1.2.3"},
				Required:    true,
			},
			wantContains: []string{
				"node-id",
				"The node identifier",
				"1",
				"1.2",
				"1.2.3",
			},
		},
		{
			name: "without examples",
			spec: ArgSpec{
				Name:        "message",
				Description: "A description message",
				Examples:    nil,
				Required:    true,
			},
			wantContains: []string{
				"message",
				"A description message",
			},
			wantMissing: []string{
				"Examples",
			},
		},
		{
			name: "empty examples slice",
			spec: ArgSpec{
				Name:        "target",
				Description: "Target node",
				Examples:    []string{},
				Required:    true,
			},
			wantContains: []string{
				"target",
				"Target node",
			},
			wantMissing: []string{
				"Examples",
			},
		},
		{
			name: "optional arg",
			spec: ArgSpec{
				Name:        "format",
				Description: "Output format",
				Examples:    []string{"json", "text"},
				Required:    false,
			},
			wantContains: []string{
				"format",
				"Output format",
				"json",
				"text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help := FormatArgHelp(tt.spec)
			if help == "" {
				t.Error("FormatArgHelp() returned empty string")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(help, want) {
					t.Errorf("FormatArgHelp() = %q, want to contain %q", help, want)
				}
			}
			for _, notWant := range tt.wantMissing {
				if strings.Contains(help, notWant) {
					t.Errorf("FormatArgHelp() = %q, should not contain %q", help, notWant)
				}
			}
		})
	}
}

func TestCheckRequiredArgs_PreservesArgSpec(t *testing.T) {
	// Verify that the returned error contains the full ArgSpec
	specs := []ArgSpec{
		{
			Name:        "node-id",
			Description: "The ID of the node",
			Examples:    []string{"1", "1.2"},
			Required:    true,
		},
	}

	err := CheckRequiredArgs([]string{}, specs)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Arg.Name != "node-id" {
		t.Errorf("Arg.Name = %q, want %q", err.Arg.Name, "node-id")
	}
	if err.Arg.Description != "The ID of the node" {
		t.Errorf("Arg.Description = %q, want %q", err.Arg.Description, "The ID of the node")
	}
	if len(err.Arg.Examples) != 2 {
		t.Errorf("Arg.Examples length = %d, want 2", len(err.Arg.Examples))
	}
	if !err.Arg.Required {
		t.Error("Arg.Required = false, want true")
	}
}

func TestCheckRequiredArgs_CommandInError(t *testing.T) {
	// The command field should be set (though CheckRequiredArgs doesn't set it)
	// This test documents that the Command must be set by the caller
	specs := []ArgSpec{
		{Name: "arg", Description: "Test", Required: true},
	}

	err := CheckRequiredArgs([]string{}, specs)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Command should be empty since CheckRequiredArgs doesn't know the command
	if err.Command != "" {
		t.Errorf("Command = %q, want empty (caller should set)", err.Command)
	}
}

func TestMissingArgError_Implements_error(t *testing.T) {
	// Verify MissingArgError implements the error interface
	var _ error = &MissingArgError{}
}

func TestFormatArgHelp_Indentation(t *testing.T) {
	spec := ArgSpec{
		Name:        "node-id",
		Description: "The ID of the node",
		Examples:    []string{"1"},
		Required:    true,
	}

	help := FormatArgHelp(spec)
	lines := strings.Split(help, "\n")

	// Check that description is indented
	foundDescription := false
	for _, line := range lines {
		if strings.Contains(line, "The ID of the node") {
			foundDescription = true
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("Description line should be indented, got %q", line)
			}
		}
	}
	if !foundDescription {
		t.Error("Description not found in help text")
	}
}
