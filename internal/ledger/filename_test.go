//go:build integration

package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateFilename tests filename generation for event files.
func TestGenerateFilename(t *testing.T) {
	tests := []struct {
		name     string
		seq      int
		expected string
	}{
		{"first event", 1, "000001.json"},
		{"single digit", 5, "000005.json"},
		{"double digit", 42, "000042.json"},
		{"triple digit", 123, "000123.json"},
		{"four digits", 1234, "001234.json"},
		{"five digits", 12345, "012345.json"},
		{"six digits", 123456, "123456.json"},
		{"seven digits", 1234567, "1234567.json"},
		{"zero", 0, "000000.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateFilename(tt.seq)
			if result != tt.expected {
				t.Errorf("GenerateFilename(%d) = %q, want %q", tt.seq, result, tt.expected)
			}
		})
	}
}

// TestGenerateFilenameFormat verifies the filename format constraints.
func TestGenerateFilenameFormat(t *testing.T) {
	t.Run("has json extension", func(t *testing.T) {
		result := GenerateFilename(1)
		if filepath.Ext(result) != ".json" {
			t.Errorf("GenerateFilename(1) = %q, want .json extension", result)
		}
	})

	t.Run("minimum 6 digit padding", func(t *testing.T) {
		result := GenerateFilename(1)
		// Should be "000001.json" (6 digits + .json = 11 chars)
		if len(result) < 11 {
			t.Errorf("GenerateFilename(1) = %q, want at least 6 digit padding", result)
		}
	})
}

// TestParseFilename tests extraction of sequence number from filename.
func TestParseFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		expected  int
		wantError bool
	}{
		{"first event", "000001.json", 1, false},
		{"single digit value", "000005.json", 5, false},
		{"double digit value", "000042.json", 42, false},
		{"triple digit value", "000123.json", 123, false},
		{"four digit value", "001234.json", 1234, false},
		{"five digit value", "012345.json", 12345, false},
		{"six digit value", "123456.json", 123456, false},
		{"seven digit value", "1234567.json", 1234567, false},
		{"zero value", "000000.json", 0, false},
		{"invalid extension", "000001.txt", 0, true},
		{"no extension", "000001", 0, true},
		{"non-numeric", "abcdef.json", 0, true},
		{"empty string", "", 0, true},
		{"just extension", ".json", 0, true},
		{"negative in name", "-00001.json", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFilename(tt.filename)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParseFilename(%q) = %d, want error", tt.filename, result)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseFilename(%q) error = %v, want nil", tt.filename, err)
				return
			}

			if result != tt.expected {
				t.Errorf("ParseFilename(%q) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestParseFilenameRoundtrip verifies that parsing reverses generation.
func TestParseFilenameRoundtrip(t *testing.T) {
	testSeqs := []int{0, 1, 42, 123, 1234, 12345, 123456, 1234567, 999999}

	for _, seq := range testSeqs {
		filename := GenerateFilename(seq)
		parsed, err := ParseFilename(filename)
		if err != nil {
			t.Errorf("Roundtrip failed for seq %d: ParseFilename(%q) error = %v", seq, filename, err)
			continue
		}
		if parsed != seq {
			t.Errorf("Roundtrip failed for seq %d: GenerateFilename -> %q -> ParseFilename = %d", seq, filename, parsed)
		}
	}
}

// TestNextSequence tests determination of next sequence number in a directory.
func TestNextSequence(t *testing.T) {
	t.Run("empty directory returns 1", func(t *testing.T) {
		dir := t.TempDir()

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 1 {
			t.Errorf("NextSequence(%q) = %d, want 1", dir, seq)
		}
	})

	t.Run("single file returns max+1", func(t *testing.T) {
		dir := t.TempDir()

		// Create a single event file
		if err := os.WriteFile(filepath.Join(dir, "000001.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 2 {
			t.Errorf("NextSequence(%q) = %d, want 2", dir, seq)
		}
	})

	t.Run("multiple files returns max+1", func(t *testing.T) {
		dir := t.TempDir()

		// Create multiple event files
		files := []string{"000001.json", "000002.json", "000005.json", "000003.json"}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0644); err != nil {
				t.Fatalf("Failed to create test file %s: %v", f, err)
			}
		}

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 6 {
			t.Errorf("NextSequence(%q) = %d, want 6 (max was 5)", dir, seq)
		}
	})

	t.Run("ignores non-json files", func(t *testing.T) {
		dir := t.TempDir()

		// Create mix of files
		if err := os.WriteFile(filepath.Join(dir, "000001.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "000002.txt"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 2 {
			t.Errorf("NextSequence(%q) = %d, want 2", dir, seq)
		}
	})

	t.Run("ignores non-numeric json files", func(t *testing.T) {
		dir := t.TempDir()

		// Create files with non-numeric names
		if err := os.WriteFile(filepath.Join(dir, "000003.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 4 {
			t.Errorf("NextSequence(%q) = %d, want 4 (max was 3)", dir, seq)
		}
	})

	t.Run("non-existent directory returns error", func(t *testing.T) {
		_, err := NextSequence("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("NextSequence for non-existent directory should return error")
		}
	})

	t.Run("handles large sequence numbers", func(t *testing.T) {
		dir := t.TempDir()

		// Create a file with a large sequence number
		if err := os.WriteFile(filepath.Join(dir, "999999.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		seq, err := NextSequence(dir)
		if err != nil {
			t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
		}
		if seq != 1000000 {
			t.Errorf("NextSequence(%q) = %d, want 1000000", dir, seq)
		}
	})
}

// TestNextSequenceWithSubdirectories verifies subdirectories are ignored.
func TestNextSequenceWithSubdirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a file
	if err := os.WriteFile(filepath.Join(dir, "000002.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a subdirectory named like a json file
	if err := os.Mkdir(filepath.Join(dir, "000010.json"), 0755); err != nil {
		t.Fatalf("Failed to create test subdirectory: %v", err)
	}

	seq, err := NextSequence(dir)
	if err != nil {
		t.Fatalf("NextSequence(%q) error = %v, want nil", dir, err)
	}
	if seq != 3 {
		t.Errorf("NextSequence(%q) = %d, want 3 (should ignore subdirectory)", dir, seq)
	}
}
