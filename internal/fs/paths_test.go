package fs

import (
	"path/filepath"
	"testing"
)

func TestPathResolver(t *testing.T) {
	basePath := "/proof/dir"
	resolver := NewPathResolver(basePath)

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "Base",
			method:   resolver.Base,
			expected: basePath,
		},
		{
			name:     "Ledger",
			method:   resolver.Ledger,
			expected: filepath.Join(basePath, "ledger"),
		},
		{
			name:     "Nodes",
			method:   resolver.Nodes,
			expected: filepath.Join(basePath, "nodes"),
		},
		{
			name:     "Defs",
			method:   resolver.Defs,
			expected: filepath.Join(basePath, "defs"),
		},
		{
			name:     "Assumptions",
			method:   resolver.Assumptions,
			expected: filepath.Join(basePath, "assumptions"),
		},
		{
			name:     "Externals",
			method:   resolver.Externals,
			expected: filepath.Join(basePath, "externals"),
		},
		{
			name:     "Lemmas",
			method:   resolver.Lemmas,
			expected: filepath.Join(basePath, "lemmas"),
		},
		{
			name:     "Locks",
			method:   resolver.Locks,
			expected: filepath.Join(basePath, "locks"),
		},
		{
			name:     "Meta",
			method:   resolver.Meta,
			expected: filepath.Join(basePath, "meta.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.expected {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestNewPathResolver(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
	}{
		{
			name:     "absolute path",
			basePath: "/home/user/proof",
		},
		{
			name:     "relative path",
			basePath: "./proof",
		},
		{
			name:     "empty path",
			basePath: "",
		},
		{
			name:     "path with trailing slash",
			basePath: "/proof/dir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewPathResolver(tt.basePath)
			if resolver == nil {
				t.Fatal("NewPathResolver returned nil")
			}
			if resolver.Base() != tt.basePath {
				t.Errorf("Base() = %q, want %q", resolver.Base(), tt.basePath)
			}
		})
	}
}
