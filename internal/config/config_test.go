package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault_HasCorrectValues(t *testing.T) {
	cfg := Default()

	// Check all default values are set correctly
	if cfg.LockTimeout != 5*time.Minute {
		t.Errorf("Default() LockTimeout = %v, want %v", cfg.LockTimeout, 5*time.Minute)
	}

	if cfg.MaxDepth != 20 {
		t.Errorf("Default() MaxDepth = %d, want 20", cfg.MaxDepth)
	}

	if cfg.MaxChildren != 20 {
		t.Errorf("Default() MaxChildren = %d, want 20", cfg.MaxChildren)
	}

	if cfg.WarnDepth != 3 {
		t.Errorf("Default() WarnDepth = %d, want 3", cfg.WarnDepth)
	}

	if cfg.AutoCorrectThreshold != 0.8 {
		t.Errorf("Default() AutoCorrectThreshold = %f, want 0.8", cfg.AutoCorrectThreshold)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Default() Version = %q, want %q", cfg.Version, "1.0")
	}

	// Title and Conjecture should be empty in default
	if cfg.Title != "" {
		t.Errorf("Default() Title = %q, want empty string", cfg.Title)
	}

	if cfg.Conjecture != "" {
		t.Errorf("Default() Conjecture = %q, want empty string", cfg.Conjecture)
	}

	// SchemaPath is optional and should be empty
	if cfg.SchemaPath != "" {
		t.Errorf("Default() SchemaPath = %q, want empty string", cfg.SchemaPath)
	}

	// Created time should be non-zero
	if cfg.Created.IsZero() {
		t.Error("Default() Created is zero time")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	created := time.Now().UTC().Truncate(time.Second)
	testConfig := Config{
		Title:                "Test Proof",
		Conjecture:           "P implies Q",
		LockTimeout:          10 * time.Minute,
		MaxDepth:             15,
		MaxChildren:          8,
		AutoCorrectThreshold: 0.75,
		SchemaPath:           "/custom/schema.json",
		Created:              created,
		Version:              "1.0",
	}

	// Write test config to file
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	loaded, err := Load(metaPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify all fields
	if loaded.Title != testConfig.Title {
		t.Errorf("Load() Title = %q, want %q", loaded.Title, testConfig.Title)
	}
	if loaded.Conjecture != testConfig.Conjecture {
		t.Errorf("Load() Conjecture = %q, want %q", loaded.Conjecture, testConfig.Conjecture)
	}
	if loaded.LockTimeout != testConfig.LockTimeout {
		t.Errorf("Load() LockTimeout = %v, want %v", loaded.LockTimeout, testConfig.LockTimeout)
	}
	if loaded.MaxDepth != testConfig.MaxDepth {
		t.Errorf("Load() MaxDepth = %d, want %d", loaded.MaxDepth, testConfig.MaxDepth)
	}
	if loaded.MaxChildren != testConfig.MaxChildren {
		t.Errorf("Load() MaxChildren = %d, want %d", loaded.MaxChildren, testConfig.MaxChildren)
	}
	if loaded.AutoCorrectThreshold != testConfig.AutoCorrectThreshold {
		t.Errorf("Load() AutoCorrectThreshold = %f, want %f", loaded.AutoCorrectThreshold, testConfig.AutoCorrectThreshold)
	}
	if loaded.SchemaPath != testConfig.SchemaPath {
		t.Errorf("Load() SchemaPath = %q, want %q", loaded.SchemaPath, testConfig.SchemaPath)
	}
	if !loaded.Created.Equal(testConfig.Created) {
		t.Errorf("Load() Created = %v, want %v", loaded.Created, testConfig.Created)
	}
	if loaded.Version != testConfig.Version {
		t.Errorf("Load() Version = %q, want %q", loaded.Version, testConfig.Version)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	nonExistentPath := "/nonexistent/path/meta.json"

	_, err := Load(nonExistentPath)
	if err == nil {
		t.Error("Load() with non-existent file should return error")
	}

	// Should contain some indication of file not found
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("Load() error should wrap os.ErrNotExist, got: %v", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	// Write invalid JSON
	invalidJSON := []byte(`{
		"title": "Test",
		"conjecture": "P implies Q"
		"lock_timeout": "5m"   // Missing comma - invalid JSON
	}`)
	if err := os.WriteFile(metaPath, invalidJSON, 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	_, err := Load(metaPath)
	if err == nil {
		t.Error("Load() with invalid JSON should return error")
	}
}

func TestLoad_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	// Config with only required fields, missing optional ones
	minimalConfig := map[string]interface{}{
		"title":      "Minimal Test",
		"conjecture": "Basic conjecture",
		"version":    "1.0",
		"created":    time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(minimalConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal minimal config: %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("Failed to write minimal config: %v", err)
	}

	loaded, err := Load(metaPath)
	if err != nil {
		t.Fatalf("Load() with missing fields error = %v, want nil", err)
	}

	// Should use defaults for missing fields
	if loaded.LockTimeout != 5*time.Minute {
		t.Errorf("Load() with missing LockTimeout = %v, want default %v", loaded.LockTimeout, 5*time.Minute)
	}
	if loaded.MaxDepth != 20 {
		t.Errorf("Load() with missing MaxDepth = %d, want default 20", loaded.MaxDepth)
	}
	if loaded.MaxChildren != 20 {
		t.Errorf("Load() with missing MaxChildren = %d, want default 20", loaded.MaxChildren)
	}
	if loaded.AutoCorrectThreshold != 0.8 {
		t.Errorf("Load() with missing AutoCorrectThreshold = %f, want default 0.8", loaded.AutoCorrectThreshold)
	}
	if loaded.WarnDepth != 3 {
		t.Errorf("Load() with missing WarnDepth = %d, want default 3", loaded.WarnDepth)
	}
}

func TestConfig_WarnDepthDefault(t *testing.T) {
	// Test that WarnDepth defaults to 3 in Default()
	cfg := Default()
	if cfg.WarnDepth != 3 {
		t.Errorf("Default() WarnDepth = %d, want 3", cfg.WarnDepth)
	}
}

func TestConfig_WarnDepthCustom(t *testing.T) {
	// Test that a custom WarnDepth value is preserved when loading from file
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	// Config with custom WarnDepth
	customConfig := map[string]interface{}{
		"title":      "Test With Custom WarnDepth",
		"conjecture": "Test conjecture",
		"version":    "1.0",
		"created":    time.Now().UTC().Format(time.RFC3339),
		"warn_depth": 5,
	}

	data, err := json.MarshalIndent(customConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal custom config: %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("Failed to write custom config: %v", err)
	}

	loaded, err := Load(metaPath)
	if err != nil {
		t.Fatalf("Load() with custom WarnDepth error = %v, want nil", err)
	}

	if loaded.WarnDepth != 5 {
		t.Errorf("Load() with custom WarnDepth = %d, want 5", loaded.WarnDepth)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	validConfigs := []struct {
		name   string
		config *Config
	}{
		{
			name: "all default values",
			config: &Config{
				Title:                "Valid Title",
				Conjecture:           "Valid Conjecture",
				LockTimeout:          5 * time.Minute,
				MaxDepth:             20,
				MaxChildren:          20,
				AutoCorrectThreshold: 0.8,
				Version:              "1.0",
				Created:              time.Now(),
			},
		},
		{
			name: "minimum valid values",
			config: &Config{
				Title:                "T",
				Conjecture:           "C",
				LockTimeout:          1 * time.Second,
				MaxDepth:             1,
				MaxChildren:          1,
				AutoCorrectThreshold: 0.0,
				Version:              "1.0",
				Created:              time.Now(),
			},
		},
		{
			name: "maximum valid values",
			config: &Config{
				Title:                "Max Title",
				Conjecture:           "Max Conjecture",
				LockTimeout:          1 * time.Hour,
				MaxDepth:             100,
				MaxChildren:          100,
				AutoCorrectThreshold: 1.0,
				Version:              "1.0",
				Created:              time.Now(),
			},
		},
		{
			name: "with optional schema path",
			config: &Config{
				Title:                "With Schema",
				Conjecture:           "Test",
				LockTimeout:          5 * time.Minute,
				MaxDepth:             20,
				MaxChildren:          20,
				AutoCorrectThreshold: 0.8,
				SchemaPath:           "/path/to/schema.json",
				Version:              "1.0",
				Created:              time.Now(),
			},
		},
	}

	for _, tt := range validConfigs {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if err != nil {
				t.Errorf("Validate() with valid config %q error = %v, want nil", tt.name, err)
			}
		})
	}
}

func TestValidate_EmptyTitle(t *testing.T) {
	cfg := Default()
	cfg.Title = ""
	cfg.Conjecture = "Valid conjecture"

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() with empty Title should return error")
	}
}

func TestValidate_EmptyConjecture(t *testing.T) {
	cfg := Default()
	cfg.Title = "Valid title"
	cfg.Conjecture = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() with empty Conjecture should return error")
	}
}

func TestValidate_LockTimeoutTooShort(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -1 * time.Second},
		{"less than 1 second", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.LockTimeout = tt.timeout

			err := Validate(cfg)
			if err == nil {
				t.Errorf("Validate() with LockTimeout %v should return error", tt.timeout)
			}
		})
	}
}

func TestValidate_LockTimeoutTooLong(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"just over 1 hour", 1*time.Hour + 1*time.Nanosecond},
		{"2 hours", 2 * time.Hour},
		{"much too long", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.LockTimeout = tt.timeout

			err := Validate(cfg)
			if err == nil {
				t.Errorf("Validate() with LockTimeout %v should return error", tt.timeout)
			}
		})
	}
}

func TestValidate_MaxDepthBounds(t *testing.T) {
	tests := []struct {
		name     string
		maxDepth int
		wantErr  bool
	}{
		{"zero depth", 0, true},
		{"negative depth", -1, true},
		{"valid minimum", 1, false},
		{"valid middle", 50, false},
		{"valid maximum", 100, false},
		{"exceeds maximum", 101, true},
		{"far exceeds maximum", 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.MaxDepth = tt.maxDepth

			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with MaxDepth %d error = %v, wantErr %v", tt.maxDepth, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_MaxChildrenBounds(t *testing.T) {
	tests := []struct {
		name        string
		maxChildren int
		wantErr     bool
	}{
		{"zero children", 0, true},
		{"negative children", -1, true},
		{"valid minimum", 1, false},
		{"valid middle", 50, false},
		{"valid maximum", 100, false},
		{"exceeds maximum", 101, true},
		{"far exceeds maximum", 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.MaxChildren = tt.maxChildren

			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with MaxChildren %d error = %v, wantErr %v", tt.maxChildren, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_AutoCorrectThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		wantErr   bool
	}{
		{"negative threshold", -0.1, true},
		{"large negative", -1.0, true},
		{"valid zero", 0.0, false},
		{"valid middle", 0.5, false},
		{"valid 0.8", 0.8, false},
		{"valid maximum", 1.0, false},
		{"exceeds maximum", 1.1, true},
		{"far exceeds maximum", 5.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.AutoCorrectThreshold = tt.threshold

			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with AutoCorrectThreshold %f error = %v, wantErr %v", tt.threshold, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty version", ""},
		{"wrong version 2.0", "2.0"},
		{"wrong version 0.9", "0.9"},
		{"wrong version 1.1", "1.1"},
		{"wrong format", "1"},
		{"wrong format v1.0", "v1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Title = "Valid"
			cfg.Conjecture = "Valid"
			cfg.Version = tt.version

			err := Validate(cfg)
			if err == nil {
				t.Errorf("Validate() with Version %q should return error", tt.version)
			}
		})
	}
}

func TestSave_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	cfg := Default()
	cfg.Title = "Test Save"
	cfg.Conjecture = "Test Conjecture"

	err := Save(cfg, metaPath)
	if err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("Save() did not create file")
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Errorf("Save() created invalid JSON: %v", err)
	}

	// Verify basic fields were saved
	if loaded.Title != cfg.Title {
		t.Errorf("Save() saved Title = %q, want %q", loaded.Title, cfg.Title)
	}
	if loaded.Conjecture != cfg.Conjecture {
		t.Errorf("Save() saved Conjecture = %q, want %q", loaded.Conjecture, cfg.Conjecture)
	}
}

func TestSave_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "meta.json")

	// Create a config with all fields populated
	original := &Config{
		Title:                "Roundtrip Test",
		Conjecture:           "For all x, P(x) implies Q(x)",
		LockTimeout:          7 * time.Minute,
		MaxDepth:             25,
		MaxChildren:          12,
		AutoCorrectThreshold: 0.85,
		SchemaPath:           "/custom/path/schema.json",
		Created:              time.Now().UTC().Truncate(time.Second),
		Version:              "1.0",
	}

	// Save the config
	err := Save(original, metaPath)
	if err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Load it back
	loaded, err := Load(metaPath)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v, want nil", err)
	}

	// Compare all fields
	if loaded.Title != original.Title {
		t.Errorf("Roundtrip Title = %q, want %q", loaded.Title, original.Title)
	}
	if loaded.Conjecture != original.Conjecture {
		t.Errorf("Roundtrip Conjecture = %q, want %q", loaded.Conjecture, original.Conjecture)
	}
	if loaded.LockTimeout != original.LockTimeout {
		t.Errorf("Roundtrip LockTimeout = %v, want %v", loaded.LockTimeout, original.LockTimeout)
	}
	if loaded.MaxDepth != original.MaxDepth {
		t.Errorf("Roundtrip MaxDepth = %d, want %d", loaded.MaxDepth, original.MaxDepth)
	}
	if loaded.MaxChildren != original.MaxChildren {
		t.Errorf("Roundtrip MaxChildren = %d, want %d", loaded.MaxChildren, original.MaxChildren)
	}
	if loaded.AutoCorrectThreshold != original.AutoCorrectThreshold {
		t.Errorf("Roundtrip AutoCorrectThreshold = %f, want %f", loaded.AutoCorrectThreshold, original.AutoCorrectThreshold)
	}
	if loaded.SchemaPath != original.SchemaPath {
		t.Errorf("Roundtrip SchemaPath = %q, want %q", loaded.SchemaPath, original.SchemaPath)
	}
	if !loaded.Created.Equal(original.Created) {
		t.Errorf("Roundtrip Created = %v, want %v", loaded.Created, original.Created)
	}
	if loaded.Version != original.Version {
		t.Errorf("Roundtrip Version = %q, want %q", loaded.Version, original.Version)
	}
}

func TestSave_InvalidDirectory(t *testing.T) {
	// Try to save to a path where parent directory doesn't exist
	invalidPath := "/nonexistent/directory/meta.json"

	cfg := Default()
	cfg.Title = "Test"
	cfg.Conjecture = "Test"

	err := Save(cfg, invalidPath)
	if err == nil {
		t.Error("Save() to invalid directory should return error")
	}
}

func TestValidate_NilConfig(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Error("Validate() with nil config should return error")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Error("Load() with empty path should return error")
	}
}

func TestSave_EmptyPath(t *testing.T) {
	cfg := Default()
	cfg.Title = "Test"
	cfg.Conjecture = "Test"

	err := Save(cfg, "")
	if err == nil {
		t.Error("Save() with empty path should return error")
	}
}
