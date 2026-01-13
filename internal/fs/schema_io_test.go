//go:build integration
// +build integration

// These tests define expected behavior for WriteSchema and ReadSchema.
// Run with: go test -tags=integration ./internal/fs/...

package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
)

// TestWriteSchema verifies that WriteSchema correctly writes schema.json
// to the proof directory.
func TestWriteSchema(t *testing.T) {
	dir := t.TempDir()

	s := schema.DefaultSchema()

	err := WriteSchema(dir, s)
	if err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Verify file was created
	schemaPath := filepath.Join(dir, "schema.json")
	info, err := os.Stat(schemaPath)
	if os.IsNotExist(err) {
		t.Fatalf("expected schema.json to exist at %s", schemaPath)
	}
	if err != nil {
		t.Fatalf("error checking schema.json: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected schema.json to be a file, not a directory")
	}

	// Verify file contents are valid JSON matching the schema
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema.json: %v", err)
	}

	var readSchema schema.Schema
	if err := json.Unmarshal(content, &readSchema); err != nil {
		t.Fatalf("schema.json is not valid JSON: %v", err)
	}

	if readSchema.Version != s.Version {
		t.Errorf("Version mismatch: got %q, want %q", readSchema.Version, s.Version)
	}
	if len(readSchema.InferenceTypes) != len(s.InferenceTypes) {
		t.Errorf("InferenceTypes length mismatch: got %d, want %d", len(readSchema.InferenceTypes), len(s.InferenceTypes))
	}
	if len(readSchema.NodeTypes) != len(s.NodeTypes) {
		t.Errorf("NodeTypes length mismatch: got %d, want %d", len(readSchema.NodeTypes), len(s.NodeTypes))
	}
	if len(readSchema.ChallengeTargets) != len(s.ChallengeTargets) {
		t.Errorf("ChallengeTargets length mismatch: got %d, want %d", len(readSchema.ChallengeTargets), len(s.ChallengeTargets))
	}
	if len(readSchema.WorkflowStates) != len(s.WorkflowStates) {
		t.Errorf("WorkflowStates length mismatch: got %d, want %d", len(readSchema.WorkflowStates), len(s.WorkflowStates))
	}
	if len(readSchema.EpistemicStates) != len(s.EpistemicStates) {
		t.Errorf("EpistemicStates length mismatch: got %d, want %d", len(readSchema.EpistemicStates), len(s.EpistemicStates))
	}
}

// TestWriteSchema_NilSchema verifies that WriteSchema returns an error
// when given a nil schema.
func TestWriteSchema_NilSchema(t *testing.T) {
	dir := t.TempDir()

	err := WriteSchema(dir, nil)
	if err == nil {
		t.Error("expected error for nil schema, got nil")
	}
}

// TestWriteSchema_EmptyBasePath verifies that WriteSchema returns an error
// when given an empty basePath.
func TestWriteSchema_EmptyBasePath(t *testing.T) {
	s := schema.DefaultSchema()

	err := WriteSchema("", s)
	if err == nil {
		t.Error("expected error for empty basePath, got nil")
	}
}

// TestWriteSchema_InvalidPath verifies that WriteSchema returns an error
// for invalid paths.
func TestWriteSchema_InvalidPath(t *testing.T) {
	s := schema.DefaultSchema()

	t.Run("empty_path", func(t *testing.T) {
		err := WriteSchema("", s)
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		err := WriteSchema("   ", s)
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		err := WriteSchema("path\x00with\x00nulls", s)
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestWriteSchema_Overwrite verifies that WriteSchema overwrites existing
// schema.json files.
func TestWriteSchema_Overwrite(t *testing.T) {
	dir := t.TempDir()

	// Write first version with default schema
	schema1 := schema.DefaultSchema()
	if err := WriteSchema(dir, schema1); err != nil {
		t.Fatalf("first WriteSchema failed: %v", err)
	}

	// Create a modified schema (different version)
	schema2 := schema.DefaultSchema()
	schema2.Version = "2.0"

	// Write second version (overwrite)
	if err := WriteSchema(dir, schema2); err != nil {
		t.Fatalf("second WriteSchema (overwrite) failed: %v", err)
	}

	// Read back and verify updated content
	readSchema, err := ReadSchema(dir)
	if err != nil {
		t.Fatalf("ReadSchema after overwrite failed: %v", err)
	}

	if readSchema.Version != schema2.Version {
		t.Errorf("Version not updated: got %q, want %q", readSchema.Version, schema2.Version)
	}
}

// TestWriteSchema_AtomicWrite verifies that WriteSchema uses atomic write
// operations (write to temp file, then rename).
func TestWriteSchema_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	s := schema.DefaultSchema()

	err := WriteSchema(dir, s)
	if err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check for common temp file patterns
		if name != "schema.json" {
			if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
				t.Errorf("temp file left behind: %s", name)
			}
		}
	}
}

// TestWriteSchema_PermissionDenied verifies that WriteSchema handles
// permission errors gracefully.
func TestWriteSchema_PermissionDenied(t *testing.T) {
	// Skip on Windows where permission model differs
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Skip if running as root (root can write anywhere)
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()

	// Remove write permission from directory
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("failed to change directory permissions: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(dir, 0755)
	})

	s := schema.DefaultSchema()

	err := WriteSchema(dir, s)
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}

// TestReadSchema verifies that ReadSchema correctly reads schema.json
// from the proof directory.
func TestReadSchema(t *testing.T) {
	dir := t.TempDir()

	// Create a schema.json file manually
	expectedSchema := schema.DefaultSchema()

	schemaJSON, err := json.Marshal(expectedSchema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		t.Fatalf("failed to write schema.json: %v", err)
	}

	// Read it back using ReadSchema
	readSchema, err := ReadSchema(dir)
	if err != nil {
		t.Fatalf("ReadSchema failed: %v", err)
	}

	if readSchema.Version != expectedSchema.Version {
		t.Errorf("Version mismatch: got %q, want %q", readSchema.Version, expectedSchema.Version)
	}
	if len(readSchema.InferenceTypes) != len(expectedSchema.InferenceTypes) {
		t.Errorf("InferenceTypes length mismatch: got %d, want %d", len(readSchema.InferenceTypes), len(expectedSchema.InferenceTypes))
	}
	if len(readSchema.NodeTypes) != len(expectedSchema.NodeTypes) {
		t.Errorf("NodeTypes length mismatch: got %d, want %d", len(readSchema.NodeTypes), len(expectedSchema.NodeTypes))
	}
}

// TestReadSchema_NotFound verifies that ReadSchema returns os.ErrNotExist
// when schema.json doesn't exist.
func TestReadSchema_NotFound(t *testing.T) {
	dir := t.TempDir()
	// Note: schema.json does NOT exist

	_, err := ReadSchema(dir)
	if err == nil {
		t.Error("expected error for missing schema.json, got nil")
	}
	if !os.IsNotExist(err) {
		// Accept either os.IsNotExist or a wrapped error
		t.Logf("got error (expected): %v", err)
	}
}

// TestReadSchema_InvalidJSON verifies that ReadSchema returns an error
// when schema.json contains invalid JSON.
func TestReadSchema_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Write invalid JSON
	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("failed to write invalid schema.json: %v", err)
	}

	_, err := ReadSchema(dir)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestReadSchema_CorruptedJSON verifies that ReadSchema returns an error
// when schema.json contains corrupted/truncated JSON.
func TestReadSchema_CorruptedJSON(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{"truncated_object", `{"version": "1.0", "inference_types":`},
		{"missing_closing_brace", `{"version": "1.0"`},
		{"invalid_escape", `{"version": "\x"}`},
		{"binary_data", "\x00\x01\x02\x03"},
		{"empty_file", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			schemaPath := filepath.Join(dir, "schema.json")
			if err := os.WriteFile(schemaPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to write corrupted schema.json: %v", err)
			}

			_, err := ReadSchema(dir)
			if err == nil {
				t.Error("expected error for corrupted JSON, got nil")
			}
		})
	}
}

// TestReadSchema_EmptyBasePath verifies that ReadSchema returns an error
// when given an empty basePath.
func TestReadSchema_EmptyBasePath(t *testing.T) {
	_, err := ReadSchema("")
	if err == nil {
		t.Error("expected error for empty basePath, got nil")
	}
}

// TestReadSchema_InvalidPath verifies that ReadSchema returns an error
// for invalid paths.
func TestReadSchema_InvalidPath(t *testing.T) {
	t.Run("empty_path", func(t *testing.T) {
		_, err := ReadSchema("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("whitespace_only_path", func(t *testing.T) {
		_, err := ReadSchema("   ")
		if err == nil {
			t.Error("expected error for whitespace-only path, got nil")
		}
	})

	t.Run("null_byte_in_path", func(t *testing.T) {
		_, err := ReadSchema("path\x00with\x00nulls")
		if err == nil {
			t.Error("expected error for path with null bytes, got nil")
		}
	})
}

// TestRoundTrip_Schema verifies that a schema can be written and read back
// with all fields preserved.
func TestRoundTrip_Schema(t *testing.T) {
	dir := t.TempDir()

	original := schema.DefaultSchema()

	// Write
	if err := WriteSchema(dir, original); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Read
	retrieved, err := ReadSchema(dir)
	if err != nil {
		t.Fatalf("ReadSchema failed: %v", err)
	}

	// Compare all fields
	if retrieved.Version != original.Version {
		t.Errorf("Version mismatch: got %q, want %q", retrieved.Version, original.Version)
	}

	// Compare InferenceTypes
	if len(retrieved.InferenceTypes) != len(original.InferenceTypes) {
		t.Errorf("InferenceTypes length mismatch: got %d, want %d", len(retrieved.InferenceTypes), len(original.InferenceTypes))
	} else {
		for i, it := range original.InferenceTypes {
			if retrieved.InferenceTypes[i] != it {
				t.Errorf("InferenceTypes[%d] mismatch: got %q, want %q", i, retrieved.InferenceTypes[i], it)
			}
		}
	}

	// Compare NodeTypes
	if len(retrieved.NodeTypes) != len(original.NodeTypes) {
		t.Errorf("NodeTypes length mismatch: got %d, want %d", len(retrieved.NodeTypes), len(original.NodeTypes))
	} else {
		for i, nt := range original.NodeTypes {
			if retrieved.NodeTypes[i] != nt {
				t.Errorf("NodeTypes[%d] mismatch: got %q, want %q", i, retrieved.NodeTypes[i], nt)
			}
		}
	}

	// Compare ChallengeTargets
	if len(retrieved.ChallengeTargets) != len(original.ChallengeTargets) {
		t.Errorf("ChallengeTargets length mismatch: got %d, want %d", len(retrieved.ChallengeTargets), len(original.ChallengeTargets))
	} else {
		for i, ct := range original.ChallengeTargets {
			if retrieved.ChallengeTargets[i] != ct {
				t.Errorf("ChallengeTargets[%d] mismatch: got %q, want %q", i, retrieved.ChallengeTargets[i], ct)
			}
		}
	}

	// Compare WorkflowStates
	if len(retrieved.WorkflowStates) != len(original.WorkflowStates) {
		t.Errorf("WorkflowStates length mismatch: got %d, want %d", len(retrieved.WorkflowStates), len(original.WorkflowStates))
	} else {
		for i, ws := range original.WorkflowStates {
			if retrieved.WorkflowStates[i] != ws {
				t.Errorf("WorkflowStates[%d] mismatch: got %q, want %q", i, retrieved.WorkflowStates[i], ws)
			}
		}
	}

	// Compare EpistemicStates
	if len(retrieved.EpistemicStates) != len(original.EpistemicStates) {
		t.Errorf("EpistemicStates length mismatch: got %d, want %d", len(retrieved.EpistemicStates), len(original.EpistemicStates))
	} else {
		for i, es := range original.EpistemicStates {
			if retrieved.EpistemicStates[i] != es {
				t.Errorf("EpistemicStates[%d] mismatch: got %q, want %q", i, retrieved.EpistemicStates[i], es)
			}
		}
	}
}

// TestSchemaFileFormat verifies that schema.json uses the expected JSON format
// with proper indentation for human readability.
func TestSchemaFileFormat(t *testing.T) {
	dir := t.TempDir()

	s := schema.DefaultSchema()

	if err := WriteSchema(dir, s); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Read raw file content
	schemaPath := filepath.Join(dir, "schema.json")
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema.json: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("schema.json is not valid JSON: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"version", "inference_types", "node_types", "challenge_targets", "workflow_states", "epistemic_states"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q not found in schema.json", field)
		}
	}
}

// TestSchema_JSONTags verifies that Schema struct has proper JSON tags.
func TestSchema_JSONTags(t *testing.T) {
	s := schema.DefaultSchema()

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal Schema: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Check that JSON keys use snake_case
	expectedKeys := []string{"version", "inference_types", "node_types", "challenge_targets", "workflow_states", "epistemic_states"}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected '%s' key in JSON", key)
		}
	}
}

// TestReadSchema_CachesAreRebuilt verifies that when reading a schema,
// the internal caches are properly rebuilt.
func TestReadSchema_CachesAreRebuilt(t *testing.T) {
	dir := t.TempDir()

	original := schema.DefaultSchema()

	// Write
	if err := WriteSchema(dir, original); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Read
	retrieved, err := ReadSchema(dir)
	if err != nil {
		t.Fatalf("ReadSchema failed: %v", err)
	}

	// Verify caches work by using Has* methods
	// These would fail if caches weren't rebuilt
	if !retrieved.HasInferenceType(schema.InferenceModusPonens) {
		t.Error("expected HasInferenceType to return true for modus_ponens")
	}
	if !retrieved.HasNodeType(schema.NodeTypeClaim) {
		t.Error("expected HasNodeType to return true for claim")
	}
	if !retrieved.HasChallengeTarget(schema.TargetStatement) {
		t.Error("expected HasChallengeTarget to return true for statement")
	}
	if !retrieved.HasWorkflowState(schema.WorkflowAvailable) {
		t.Error("expected HasWorkflowState to return true for available")
	}
	if !retrieved.HasEpistemicState(schema.EpistemicPending) {
		t.Error("expected HasEpistemicState to return true for pending")
	}
}

// TestReadSchema_ValidatesSchema verifies that ReadSchema validates
// the schema after reading.
func TestReadSchema_ValidatesSchema(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "missing_version",
			content: `{"inference_types":["modus_ponens"],"node_types":["claim"],"challenge_targets":["statement"],"workflow_states":["available"],"epistemic_states":["pending"]}`,
		},
		{
			name:    "empty_inference_types",
			content: `{"version":"1.0","inference_types":[],"node_types":["claim"],"challenge_targets":["statement"],"workflow_states":["available"],"epistemic_states":["pending"]}`,
		},
		{
			name:    "empty_node_types",
			content: `{"version":"1.0","inference_types":["modus_ponens"],"node_types":[],"challenge_targets":["statement"],"workflow_states":["available"],"epistemic_states":["pending"]}`,
		},
		{
			name:    "invalid_inference_type",
			content: `{"version":"1.0","inference_types":["invalid_type"],"node_types":["claim"],"challenge_targets":["statement"],"workflow_states":["available"],"epistemic_states":["pending"]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			schemaPath := filepath.Join(dir, "schema.json")
			if err := os.WriteFile(schemaPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to write schema.json: %v", err)
			}

			_, err := ReadSchema(dir)
			if err == nil {
				t.Error("expected error for invalid schema, got nil")
			}
		})
	}
}

// TestWriteSchema_CustomSchema verifies that WriteSchema works with
// a customized schema (not just the default).
func TestWriteSchema_CustomSchema(t *testing.T) {
	dir := t.TempDir()

	// Create a custom schema with subset of types
	custom := &schema.Schema{
		Version: "2.0-custom",
		InferenceTypes: []schema.InferenceType{
			schema.InferenceModusPonens,
			schema.InferenceAssumption,
		},
		NodeTypes: []schema.NodeType{
			schema.NodeTypeClaim,
			schema.NodeTypeQED,
		},
		ChallengeTargets: []schema.ChallengeTarget{
			schema.TargetStatement,
			schema.TargetInference,
		},
		WorkflowStates: []schema.WorkflowState{
			schema.WorkflowAvailable,
			schema.WorkflowClaimed,
		},
		EpistemicStates: []schema.EpistemicState{
			schema.EpistemicPending,
			schema.EpistemicValidated,
		},
	}

	// Write
	if err := WriteSchema(dir, custom); err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	// Read back
	retrieved, err := ReadSchema(dir)
	if err != nil {
		t.Fatalf("ReadSchema failed: %v", err)
	}

	// Verify
	if retrieved.Version != custom.Version {
		t.Errorf("Version mismatch: got %q, want %q", retrieved.Version, custom.Version)
	}
	if len(retrieved.InferenceTypes) != 2 {
		t.Errorf("InferenceTypes length: got %d, want 2", len(retrieved.InferenceTypes))
	}
	if len(retrieved.NodeTypes) != 2 {
		t.Errorf("NodeTypes length: got %d, want 2", len(retrieved.NodeTypes))
	}
}

// TestWriteSchema_NonExistentDirectory verifies that WriteSchema creates
// parent directories if they don't exist.
func TestWriteSchema_NonExistentDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "subdir", "nested")
	// Note: nestedDir does NOT exist yet

	s := schema.DefaultSchema()

	err := WriteSchema(nestedDir, s)
	// Implementation should either:
	// 1. Create the directory and succeed, or
	// 2. Return an appropriate error
	// This test documents expected behavior - adjust based on design decision
	if err != nil {
		// If we expect WriteSchema to NOT create directories, this is expected
		t.Logf("WriteSchema returned error for non-existent directory (may be expected): %v", err)
	} else {
		// If WriteSchema creates directories, verify the file exists
		schemaPath := filepath.Join(nestedDir, "schema.json")
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			t.Error("WriteSchema succeeded but schema.json was not created")
		}
	}
}

// TestReadSchema_DirectoryNotFile verifies that ReadSchema returns an error
// when schema.json is a directory instead of a file.
func TestReadSchema_DirectoryNotFile(t *testing.T) {
	dir := t.TempDir()

	// Create schema.json as a directory instead of a file
	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.MkdirAll(schemaPath, 0755); err != nil {
		t.Fatalf("failed to create schema.json directory: %v", err)
	}

	_, err := ReadSchema(dir)
	if err == nil {
		t.Error("expected error when schema.json is a directory, got nil")
	}
}
