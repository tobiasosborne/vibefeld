package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMustString(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("test", "default", "test flag")

	// Test retrieving a registered flag
	val := MustString(cmd, "test")
	if val != "default" {
		t.Errorf("MustString() = %q, want %q", val, "default")
	}

	// Test with a set value
	cmd.Flags().Set("test", "custom")
	val = MustString(cmd, "test")
	if val != "custom" {
		t.Errorf("MustString() = %q, want %q", val, "custom")
	}
}

func TestMustString_Panics(t *testing.T) {
	cmd := &cobra.Command{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustString() did not panic for unregistered flag")
		}
	}()

	MustString(cmd, "nonexistent")
}

func TestMustBool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("verbose", false, "verbose flag")

	// Test default value
	val := MustBool(cmd, "verbose")
	if val != false {
		t.Errorf("MustBool() = %v, want %v", val, false)
	}

	// Test with a set value
	cmd.Flags().Set("verbose", "true")
	val = MustBool(cmd, "verbose")
	if val != true {
		t.Errorf("MustBool() = %v, want %v", val, true)
	}
}

func TestMustBool_Panics(t *testing.T) {
	cmd := &cobra.Command{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustBool() did not panic for unregistered flag")
		}
	}()

	MustBool(cmd, "nonexistent")
}

func TestMustInt(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("count", 10, "count flag")

	// Test default value
	val := MustInt(cmd, "count")
	if val != 10 {
		t.Errorf("MustInt() = %d, want %d", val, 10)
	}

	// Test with a set value
	cmd.Flags().Set("count", "42")
	val = MustInt(cmd, "count")
	if val != 42 {
		t.Errorf("MustInt() = %d, want %d", val, 42)
	}
}

func TestMustInt_Panics(t *testing.T) {
	cmd := &cobra.Command{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustInt() did not panic for unregistered flag")
		}
	}()

	MustInt(cmd, "nonexistent")
}

func TestMustStringSlice(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("items", []string{"a", "b"}, "items flag")

	// Test default value
	val := MustStringSlice(cmd, "items")
	if len(val) != 2 || val[0] != "a" || val[1] != "b" {
		t.Errorf("MustStringSlice() = %v, want [a b]", val)
	}
}

func TestMustStringSlice_Panics(t *testing.T) {
	cmd := &cobra.Command{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustStringSlice() did not panic for unregistered flag")
		}
	}()

	MustStringSlice(cmd, "nonexistent")
}
