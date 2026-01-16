package cli

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		flagNames        []string
		wantPositional   []string
		wantFlags        map[string]string
	}{
		// Empty args
		{
			name:           "empty args",
			args:           []string{},
			flagNames:      []string{"owner", "verbose"},
			wantPositional: []string{},
			wantFlags:      map[string]string{},
		},

		// Only positional args
		{
			name:           "only positional args",
			args:           []string{"1.2", "1.3"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2", "1.3"},
			wantFlags:      map[string]string{},
		},

		// Only flags - long flags with space
		{
			name:           "only long flag with space",
			args:           []string{"--owner", "alice"},
			flagNames:      []string{"owner"},
			wantPositional: []string{},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Only flags - long flags with equals
		{
			name:           "only long flag with equals",
			args:           []string{"--owner=alice"},
			flagNames:      []string{"owner"},
			wantPositional: []string{},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Flags after positional args (standard order)
		{
			name:           "flags after positional",
			args:           []string{"1.2", "--owner", "alice"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Flags before positional args (reversed order)
		{
			name:           "flags before positional",
			args:           []string{"--owner", "alice", "1.2"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Mixed order with multiple positional
		{
			name:           "mixed order multiple positional",
			args:           []string{"--owner", "alice", "1.2", "1.3"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2", "1.3"},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Mixed order with flags in between
		{
			name:           "flags in between positional",
			args:           []string{"1.2", "--owner", "alice", "1.3"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2", "1.3"},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Multiple flags
		{
			name:           "multiple flags",
			args:           []string{"--owner", "alice", "--format", "json"},
			flagNames:      []string{"owner", "format"},
			wantPositional: []string{},
			wantFlags:      map[string]string{"owner": "alice", "format": "json"},
		},

		// Multiple flags mixed with positional
		{
			name:           "multiple flags mixed with positional",
			args:           []string{"--owner", "alice", "1.2", "--format", "json", "1.3"},
			flagNames:      []string{"owner", "format"},
			wantPositional: []string{"1.2", "1.3"},
			wantFlags:      map[string]string{"owner": "alice", "format": "json"},
		},

		// Short flag with space
		{
			name:           "short flag with space",
			args:           []string{"-o", "alice", "1.2"},
			flagNames:      []string{"o"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"o": "alice"},
		},

		// Boolean flags (no value) - without explicit boolean info, parser treats next arg as value
		// To get boolean behavior, use ParseArgsWithBoolFlags or =true syntax
		{
			name:           "boolean flag",
			args:           []string{"--verbose=true", "1.2"},
			flagNames:      []string{"verbose"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": "true"},
		},

		// Boolean flag at end
		{
			name:           "boolean flag at end",
			args:           []string{"1.2", "--verbose"},
			flagNames:      []string{"verbose"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": ""},
		},

		// Boolean flag with value flags mixed
		{
			name:           "boolean and value flags mixed",
			args:           []string{"--verbose", "--owner", "alice", "1.2"},
			flagNames:      []string{"verbose", "owner"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": "", "owner": "alice"},
		},

		// Short flag without explicit boolean - parser consumes next arg as value
		// For boolean short flags, use ParseArgsWithBoolFlags
		{
			name:           "short flag consumes value",
			args:           []string{"-v", "1.2"},
			flagNames:      []string{"v"},
			wantPositional: []string{},
			wantFlags:      map[string]string{"v": "1.2"},
		},

		// Flag value that looks like a flag (starts with dash)
		{
			name:           "flag value starting with dash",
			args:           []string{"--owner", "-alice-", "1.2"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"owner": "-alice-"},
		},

		// Unknown flag treated as positional
		{
			name:           "unknown flag treated as positional",
			args:           []string{"--unknown", "value", "1.2"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"--unknown", "value", "1.2"},
			wantFlags:      map[string]string{},
		},

		// Flag with equals and no value
		{
			name:           "flag with equals empty value",
			args:           []string{"--owner=", "1.2"},
			flagNames:      []string{"owner"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"owner": ""},
		},

		// Complex real-world example
		{
			name:           "complex af refine example",
			args:           []string{"--owner", "alice", "1.2", "--inference", "modus_ponens"},
			flagNames:      []string{"owner", "inference", "format"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"owner": "alice", "inference": "modus_ponens"},
		},

		// Double dash terminates flag parsing
		{
			name:           "double dash terminates flags",
			args:           []string{"--owner", "alice", "--", "--not-a-flag", "1.2"},
			flagNames:      []string{"owner", "not-a-flag"},
			wantPositional: []string{"--not-a-flag", "1.2"},
			wantFlags:      map[string]string{"owner": "alice"},
		},

		// Flag name with hyphen - without boolean info, consumes next arg as value
		{
			name:           "flag name with hyphen",
			args:           []string{"--dry-run=true", "1.2"},
			flagNames:      []string{"dry-run"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"dry-run": "true"},
		},

		// Flag value with equals sign
		{
			name:           "flag value with equals sign",
			args:           []string{"--config=key=value", "1.2"},
			flagNames:      []string{"config"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"config": "key=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPositional, gotFlags := ParseArgs(tt.args, tt.flagNames)

			if !reflect.DeepEqual(gotPositional, tt.wantPositional) {
				t.Errorf("ParseArgs() positional = %v, want %v", gotPositional, tt.wantPositional)
			}

			if !reflect.DeepEqual(gotFlags, tt.wantFlags) {
				t.Errorf("ParseArgs() flags = %v, want %v", gotFlags, tt.wantFlags)
			}
		})
	}
}

func TestParseArgs_BooleanFlags(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		flagNames        []string
		boolFlags        []string
		wantPositional   []string
		wantFlags        map[string]string
	}{
		// Boolean flag explicitly marked
		{
			name:           "explicit boolean flag",
			args:           []string{"--verbose", "1.2"},
			flagNames:      []string{"verbose", "owner"},
			boolFlags:      []string{"verbose"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": "true"},
		},

		// Boolean flag followed by non-flag looking value
		{
			name:           "boolean flag with value flag after",
			args:           []string{"--verbose", "--owner", "alice"},
			flagNames:      []string{"verbose", "owner"},
			boolFlags:      []string{"verbose"},
			wantPositional: []string{},
			wantFlags:      map[string]string{"verbose": "true", "owner": "alice"},
		},

		// Boolean flag with explicit false
		{
			name:           "boolean flag with equals false",
			args:           []string{"--verbose=false", "1.2"},
			flagNames:      []string{"verbose"},
			boolFlags:      []string{"verbose"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": "false"},
		},

		// Multiple boolean flags
		{
			name:           "multiple boolean flags",
			args:           []string{"--verbose", "--force", "1.2"},
			flagNames:      []string{"verbose", "force"},
			boolFlags:      []string{"verbose", "force"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"verbose": "true", "force": "true"},
		},

		// Short boolean flag
		{
			name:           "short boolean flag explicit",
			args:           []string{"-v", "1.2"},
			flagNames:      []string{"v", "o"},
			boolFlags:      []string{"v"},
			wantPositional: []string{"1.2"},
			wantFlags:      map[string]string{"v": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPositional, gotFlags := ParseArgsWithBoolFlags(tt.args, tt.flagNames, tt.boolFlags)

			if !reflect.DeepEqual(gotPositional, tt.wantPositional) {
				t.Errorf("ParseArgsWithBoolFlags() positional = %v, want %v", gotPositional, tt.wantPositional)
			}

			if !reflect.DeepEqual(gotFlags, tt.wantFlags) {
				t.Errorf("ParseArgsWithBoolFlags() flags = %v, want %v", gotFlags, tt.wantFlags)
			}
		})
	}
}

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		flagNames []string
		want      []string
	}{
		// Empty args
		{
			name:      "empty args",
			args:      []string{},
			flagNames: []string{"owner"},
			want:      []string{},
		},

		// Already normalized (positional first, then flags)
		{
			name:      "already normalized",
			args:      []string{"1.2", "--owner", "alice"},
			flagNames: []string{"owner"},
			want:      []string{"1.2", "--owner", "alice"},
		},

		// Flags before positional
		{
			name:      "flags before positional",
			args:      []string{"--owner", "alice", "1.2"},
			flagNames: []string{"owner"},
			want:      []string{"1.2", "--owner", "alice"},
		},

		// Multiple flags before positional
		{
			name:      "multiple flags before positional",
			args:      []string{"--owner", "alice", "--format", "json", "1.2", "1.3"},
			flagNames: []string{"owner", "format"},
			want:      []string{"1.2", "1.3", "--owner", "alice", "--format", "json"},
		},

		// Flags interleaved with positional
		{
			name:      "flags interleaved",
			args:      []string{"--owner", "alice", "1.2", "--format", "json", "1.3"},
			flagNames: []string{"owner", "format"},
			want:      []string{"1.2", "1.3", "--owner", "alice", "--format", "json"},
		},

		// Short flags
		{
			name:      "short flags before positional",
			args:      []string{"-o", "alice", "1.2"},
			flagNames: []string{"o"},
			want:      []string{"1.2", "-o", "alice"},
		},

		// Boolean flags - without explicit bool flag info, --verbose consumes 1.2 as value
		// For proper boolean handling, use NormalizeArgsWithBoolFlags
		{
			name:      "boolean flag before positional",
			args:      []string{"--verbose=true", "1.2"},
			flagNames: []string{"verbose"},
			want:      []string{"1.2", "--verbose=true"},
		},

		// Flag with equals syntax
		{
			name:      "flag with equals before positional",
			args:      []string{"--owner=alice", "1.2"},
			flagNames: []string{"owner"},
			want:      []string{"1.2", "--owner=alice"},
		},

		// Only positional
		{
			name:      "only positional args",
			args:      []string{"1.2", "1.3", "1.4"},
			flagNames: []string{"owner"},
			want:      []string{"1.2", "1.3", "1.4"},
		},

		// Only flags
		{
			name:      "only flags",
			args:      []string{"--owner", "alice", "--format", "json"},
			flagNames: []string{"owner", "format"},
			want:      []string{"--owner", "alice", "--format", "json"},
		},

		// Double dash preserved
		{
			name:      "double dash preserved",
			args:      []string{"--owner", "alice", "--", "--not-flag", "1.2"},
			flagNames: []string{"owner"},
			want:      []string{"--not-flag", "1.2", "--owner", "alice", "--"},
		},

		// Unknown flags kept as positional
		{
			name:      "unknown flags as positional",
			args:      []string{"--unknown", "value", "1.2"},
			flagNames: []string{"owner"},
			want:      []string{"--unknown", "value", "1.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeArgs(tt.args, tt.flagNames)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeArgsWithBoolFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		flagNames []string
		boolFlags []string
		want      []string
	}{
		// Boolean flag followed by positional
		{
			name:      "boolean flag then positional",
			args:      []string{"--verbose", "1.2"},
			flagNames: []string{"verbose", "owner"},
			boolFlags: []string{"verbose"},
			want:      []string{"1.2", "--verbose"},
		},

		// Boolean flag with value flag
		{
			name:      "boolean and value flags",
			args:      []string{"--verbose", "--owner", "alice", "1.2"},
			flagNames: []string{"verbose", "owner"},
			boolFlags: []string{"verbose"},
			want:      []string{"1.2", "--verbose", "--owner", "alice"},
		},

		// Multiple boolean flags
		{
			name:      "multiple boolean flags",
			args:      []string{"--verbose", "--force", "1.2"},
			flagNames: []string{"verbose", "force"},
			boolFlags: []string{"verbose", "force"},
			want:      []string{"1.2", "--verbose", "--force"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeArgsWithBoolFlags(tt.args, tt.flagNames, tt.boolFlags)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeArgsWithBoolFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFlag(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"long flag", "--owner", true},
		{"short flag", "-o", true},
		{"positional", "1.2", false},
		{"empty string", "", false},
		{"single dash", "-", false},
		{"double dash", "--", true},
		{"triple dash", "---flag", true},
		{"negative number", "-123", false},
		{"negative decimal", "-1.23", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsFlag(tt.arg)
			if got != tt.want {
				t.Errorf("IsFlag(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestExtractFlagName(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantVal string
		wantOk  bool
	}{
		{"long flag", "--owner", "owner", "", true},
		{"short flag", "-o", "o", "", true},
		{"flag with equals", "--owner=alice", "owner", "alice", true},
		{"flag with equals empty value", "--owner=", "owner", "", true},
		{"flag with equals value has equals", "--config=key=value", "config", "key=value", true},
		{"not a flag", "1.2", "", "", false},
		{"empty string", "", "", "", false},
		{"single dash", "-", "", "", false},
		{"double dash only", "--", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotVal, gotOk := ExtractFlagName(tt.arg)
			if gotName != tt.want {
				t.Errorf("ExtractFlagName(%q) name = %q, want %q", tt.arg, gotName, tt.want)
			}
			if gotVal != tt.wantVal {
				t.Errorf("ExtractFlagName(%q) value = %q, want %q", tt.arg, gotVal, tt.wantVal)
			}
			if gotOk != tt.wantOk {
				t.Errorf("ExtractFlagName(%q) ok = %v, want %v", tt.arg, gotOk, tt.wantOk)
			}
		})
	}
}

func TestParseArgs_DoesNotMutateInput(t *testing.T) {
	args := []string{"--owner", "alice", "1.2"}
	flagNames := []string{"owner"}

	// Make copies to compare after
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)

	ParseArgs(args, flagNames)

	for i, v := range args {
		if v != argsCopy[i] {
			t.Errorf("ParseArgs() mutated args slice: original[%d]=%q, now=%q", i, argsCopy[i], v)
		}
	}
}

func TestNormalizeArgs_DoesNotMutateInput(t *testing.T) {
	args := []string{"--owner", "alice", "1.2"}
	flagNames := []string{"owner"}

	// Make copies to compare after
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)

	NormalizeArgs(args, flagNames)

	for i, v := range args {
		if v != argsCopy[i] {
			t.Errorf("NormalizeArgs() mutated args slice: original[%d]=%q, now=%q", i, argsCopy[i], v)
		}
	}
}

func TestParseArgs_NilInputs(t *testing.T) {
	// Should handle nil inputs gracefully
	positional, flags := ParseArgs(nil, nil)

	if positional == nil || len(positional) != 0 {
		t.Errorf("ParseArgs(nil, nil) positional = %v, want empty slice", positional)
	}

	if flags == nil || len(flags) != 0 {
		t.Errorf("ParseArgs(nil, nil) flags = %v, want empty map", flags)
	}
}

func TestNormalizeArgs_NilInputs(t *testing.T) {
	// Should handle nil inputs gracefully
	result := NormalizeArgs(nil, nil)

	if result == nil || len(result) != 0 {
		t.Errorf("NormalizeArgs(nil, nil) = %v, want empty slice", result)
	}
}
