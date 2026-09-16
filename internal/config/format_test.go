package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatConstants(t *testing.T) {
	if FormatCurrent != "1.1" {
		t.Errorf("FormatCurrent = %q, want 1.1", FormatCurrent)
	}
	if PolicyVersion == "" {
		t.Error("PolicyVersion must not be empty")
	}
	if !IsFormatReadable("1.0") || !IsFormatReadable("1.1") {
		t.Error("both 1.0 and 1.1 must be readable")
	}
	if IsFormatReadable("2.0") || IsFormatReadable("") {
		t.Error("2.0 and empty must not be readable")
	}
}

func TestValidate_AcceptsReadableFormats(t *testing.T) {
	for _, v := range []string{"1.0", "1.1"} {
		cfg := Default()
		cfg.Title = "T"
		cfg.Conjecture = "C"
		cfg.Version = v
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate() with format %q = %v, want nil", v, err)
		}
	}
}

func TestCheckFormat(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"format 1.0", "1.0", false},
		{"format 1.1", "1.1", false},
		{"newer format 2.0", "2.0", true},
		{"unknown format", "9.9", true},
		{"garbage", "banana", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFormat(&Config{Version: tt.version})
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckFormat(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrFormatTooNew) {
				t.Errorf("CheckFormat(%q) error = %v, want ErrFormatTooNew", tt.version, err)
			}
		})
	}
}

func TestCheckFormat_NilIsNoOp(t *testing.T) {
	if err := CheckFormat(nil); err != nil {
		t.Errorf("CheckFormat(nil) = %v, want nil", err)
	}
}

func TestCompareFormats(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "1.1", -1},
		{"1.1", "1.0", 1},
		{"1.2", "1.10", -1},
		{"2.0", "1.99", 1},
		{"1.0", "9.9", -1},
	}
	for _, tt := range tests {
		if got := CompareFormats(tt.a, tt.b); got != tt.want {
			t.Errorf("CompareFormats(%q,%q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSave_AtomicAndDurable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.json")

	cfg := Default()
	cfg.Title = "T"
	cfg.Conjecture = "C"

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if loaded.Version != FormatCurrent {
		t.Errorf("roundtrip Version = %q, want %q", loaded.Version, FormatCurrent)
	}

	// No temp files left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "meta.json" {
			t.Errorf("unexpected leftover file %q after Save()", e.Name())
		}
	}
}
