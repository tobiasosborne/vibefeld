package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestInitProofDir_CreatesAllDirectories verifies that InitProofDir creates
// the complete directory structure required for a proof workspace.
func TestInitProofDir_CreatesAllDirectories(t *testing.T) {
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	err := InitProofDir(proofDir)
	if err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	// Verify proof root exists
	if _, err := os.Stat(proofDir); os.IsNotExist(err) {
		t.Error("expected proof directory to exist")
	}

	// Check each required subdirectory exists
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		path := filepath.Join(proofDir, sub)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", sub)
			continue
		}
		if err != nil {
			t.Errorf("error checking directory %s: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory, got file", sub)
		}
	}
}

// TestInitProofDir_Idempotent verifies that calling InitProofDir multiple times
// on the same path succeeds without error or corruption.
func TestInitProofDir_Idempotent(t *testing.T) {
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	// First call
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("first InitProofDir failed: %v", err)
	}

	// Create a marker file to verify it's not deleted
	markerPath := filepath.Join(proofDir, "nodes", "marker.txt")
	if err := os.WriteFile(markerPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create marker file: %v", err)
	}

	// Second call - should succeed
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("second InitProofDir failed: %v", err)
	}

	// Third call - should also succeed
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("third InitProofDir failed: %v", err)
	}

	// Verify marker file still exists (not corrupted/deleted)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("marker file was deleted during idempotent call")
	}

	// Verify all directories still exist
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		path := filepath.Join(proofDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("directory %s missing after idempotent calls", sub)
		}
	}
}

// TestInitProofDir_AlreadyExists verifies that InitProofDir works correctly
// when some or all directories already exist.
func TestInitProofDir_AlreadyExists(t *testing.T) {
	t.Run("all_dirs_exist", func(t *testing.T) {
		dir := t.TempDir()
		proofDir := filepath.Join(dir, "proof")

		// Pre-create all directories
		subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
		for _, sub := range subdirs {
			path := filepath.Join(proofDir, sub)
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("failed to pre-create directory %s: %v", sub, err)
			}
		}

		// InitProofDir should succeed
		if err := InitProofDir(proofDir); err != nil {
			t.Fatalf("InitProofDir failed when all dirs exist: %v", err)
		}
	})

	t.Run("some_dirs_exist", func(t *testing.T) {
		dir := t.TempDir()
		proofDir := filepath.Join(dir, "proof")

		// Pre-create only some directories
		existingDirs := []string{"ledger", "nodes"}
		for _, sub := range existingDirs {
			path := filepath.Join(proofDir, sub)
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("failed to pre-create directory %s: %v", sub, err)
			}
		}

		// InitProofDir should create the missing ones
		if err := InitProofDir(proofDir); err != nil {
			t.Fatalf("InitProofDir failed when some dirs exist: %v", err)
		}

		// Verify all directories now exist
		allDirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
		for _, sub := range allDirs {
			path := filepath.Join(proofDir, sub)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("expected directory %s to exist", sub)
			}
		}
	})

	t.Run("proof_root_exists_empty", func(t *testing.T) {
		dir := t.TempDir()
		proofDir := filepath.Join(dir, "proof")

		// Pre-create only the root proof directory
		if err := os.MkdirAll(proofDir, 0755); err != nil {
			t.Fatalf("failed to pre-create proof directory: %v", err)
		}

		// InitProofDir should create all subdirectories
		if err := InitProofDir(proofDir); err != nil {
			t.Fatalf("InitProofDir failed when proof root exists: %v", err)
		}

		// Verify all subdirectories were created
		subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
		for _, sub := range subdirs {
			path := filepath.Join(proofDir, sub)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("expected directory %s to exist", sub)
			}
		}
	})
}

// TestInitProofDir_InvalidPath verifies that InitProofDir returns an error
// for invalid path inputs.
func TestInitProofDir_InvalidPath(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		err := InitProofDir("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only", func(t *testing.T) {
		err := InitProofDir("   ")
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := InitProofDir("proof\x00dir")
		if err == nil {
			t.Error("expected error for path with null byte, got nil")
		}
	})
}

// TestInitProofDir_PermissionDenied verifies that InitProofDir handles
// permission errors gracefully.
func TestInitProofDir_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	restrictedDir := filepath.Join(dir, "restricted")

	// Create a directory with no write permission
	if err := os.MkdirAll(restrictedDir, 0555); err != nil {
		t.Fatalf("failed to create restricted directory: %v", err)
	}
	// Ensure we restore permissions for cleanup
	t.Cleanup(func() {
		os.Chmod(restrictedDir, 0755)
	})

	proofDir := filepath.Join(restrictedDir, "proof")
	err := InitProofDir(proofDir)
	if err == nil {
		t.Error("expected error when creating in read-only directory, got nil")
	}
}

// TestInitProofDir_CreatesMetaJson verifies that InitProofDir creates
// the meta.json file with default configuration.
func TestInitProofDir_CreatesMetaJson(t *testing.T) {
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	err := InitProofDir(proofDir)
	if err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	metaPath := filepath.Join(proofDir, "meta.json")
	info, err := os.Stat(metaPath)
	if os.IsNotExist(err) {
		t.Fatal("expected meta.json to be created")
	}
	if err != nil {
		t.Fatalf("error checking meta.json: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected meta.json to be a file, not a directory")
	}

	// Verify it's valid JSON
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta.json: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("meta.json is empty")
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}

	// Check for expected fields (version at minimum)
	if _, ok := meta["version"]; !ok {
		t.Error("meta.json should contain a 'version' field")
	}
}

// TestInitProofDir_MetaJsonNotOverwritten verifies that InitProofDir does not
// overwrite an existing meta.json file.
func TestInitProofDir_MetaJsonNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	// First init
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("first InitProofDir failed: %v", err)
	}

	metaPath := filepath.Join(proofDir, "meta.json")

	// Modify meta.json with custom content
	customContent := `{"version": "1.0.0", "custom": "value"}`
	if err := os.WriteFile(metaPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write custom meta.json: %v", err)
	}

	// Second init - should not overwrite
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("second InitProofDir failed: %v", err)
	}

	// Read back and verify custom content preserved
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta.json: %v", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}

	if meta["custom"] != "value" {
		t.Error("meta.json was overwritten, custom field lost")
	}
}

// TestInitProofDir_DirectoryPermissions verifies that created directories
// have appropriate permissions.
func TestInitProofDir_DirectoryPermissions(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	dir := t.TempDir()
	proofDir := filepath.Join(dir, "proof")

	err := InitProofDir(proofDir)
	if err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	// Check root proof directory permissions
	info, err := os.Stat(proofDir)
	if err != nil {
		t.Fatalf("failed to stat proof directory: %v", err)
	}

	// Expect at least read/write/execute for owner, and read/execute for group/others
	// Typical expected: 0755 (rwxr-xr-x)
	mode := info.Mode().Perm()
	if mode&0700 != 0700 {
		t.Errorf("proof directory owner permissions too restrictive: %o", mode)
	}

	// Check subdirectory permissions
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		path := filepath.Join(proofDir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("failed to stat %s: %v", sub, err)
			continue
		}

		mode := info.Mode().Perm()
		// Owner should have full access
		if mode&0700 != 0700 {
			t.Errorf("directory %s owner permissions too restrictive: %o", sub, mode)
		}
	}
}

// TestInitProofDir_NestedPath verifies that InitProofDir can create
// a proof directory in a deeply nested path that doesn't exist yet.
func TestInitProofDir_NestedPath(t *testing.T) {
	dir := t.TempDir()
	proofDir := filepath.Join(dir, "deeply", "nested", "path", "proof")

	err := InitProofDir(proofDir)
	if err != nil {
		t.Fatalf("InitProofDir failed for nested path: %v", err)
	}

	// Verify structure was created
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		path := filepath.Join(proofDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist in nested path", sub)
		}
	}
}

// TestInitProofDir_AbsoluteAndRelativePath verifies behavior with both
// absolute and relative paths.
func TestInitProofDir_AbsoluteAndRelativePath(t *testing.T) {
	t.Run("absolute_path", func(t *testing.T) {
		dir := t.TempDir()
		proofDir := filepath.Join(dir, "proof")

		// Ensure it's absolute
		absPath, err := filepath.Abs(proofDir)
		if err != nil {
			t.Fatalf("failed to get absolute path: %v", err)
		}

		err = InitProofDir(absPath)
		if err != nil {
			t.Fatalf("InitProofDir failed for absolute path: %v", err)
		}

		if _, err := os.Stat(filepath.Join(absPath, "nodes")); os.IsNotExist(err) {
			t.Error("expected nodes directory to exist")
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		// Save current directory
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		t.Cleanup(func() {
			os.Chdir(origDir)
		})

		// Change to temp directory
		tempDir := t.TempDir()
		if err := os.Chdir(tempDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Use relative path
		err = InitProofDir("proof")
		if err != nil {
			t.Fatalf("InitProofDir failed for relative path: %v", err)
		}

		// Verify using absolute path for checking
		proofDir := filepath.Join(tempDir, "proof")
		if _, err := os.Stat(filepath.Join(proofDir, "nodes")); os.IsNotExist(err) {
			t.Error("expected nodes directory to exist")
		}
	})
}

// TestInitProofDir_SymlinkPath verifies behavior when path contains symlinks.
func TestInitProofDir_SymlinkPath(t *testing.T) {
	// Skip on Windows where symlink creation may require elevated privileges
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows")
	}

	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	linkDir := filepath.Join(dir, "link")

	// Create the real directory
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("failed to create real directory: %v", err)
	}

	// Create symlink
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	proofDir := filepath.Join(linkDir, "proof")
	err := InitProofDir(proofDir)
	if err != nil {
		t.Fatalf("InitProofDir failed for symlink path: %v", err)
	}

	// Verify directories exist via the symlink
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		path := filepath.Join(proofDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist via symlink", sub)
		}
	}

	// Also verify via the real path
	realProofDir := filepath.Join(realDir, "proof")
	for _, sub := range subdirs {
		path := filepath.Join(realProofDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist in real path", sub)
		}
	}
}
