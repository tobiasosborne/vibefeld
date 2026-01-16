// Package config provides configuration loading and validation for AF proofs.
// Configuration is stored in meta.json in the proof directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// MaxDepthLimit is the maximum allowed value for MaxDepth configuration.
// This prevents excessively deep proof trees that could cause performance issues.
const MaxDepthLimit = 100

// Config holds the configuration for an AF proof.
// It is stored in meta.json in the proof directory.
type Config struct {
	// Title is a human-readable title for the proof
	Title string `json:"title"`

	// Conjecture is the statement to be proved
	Conjecture string `json:"conjecture"`

	// LockTimeout is the maximum duration a lock can be held (default: 5m)
	LockTimeout time.Duration `json:"lock_timeout"`

	// MaxDepth is the maximum depth of the proof tree (default: 20)
	MaxDepth int `json:"max_depth"`

	// MaxChildren is the maximum number of children per node (default: 10)
	MaxChildren int `json:"max_children"`

	// WarnDepth is the depth at which warnings are issued for deep nodes (default: 3)
	WarnDepth int `json:"warn_depth"`

	// AutoCorrectThreshold is the fuzzy match threshold for auto-correction (default: 0.8)
	AutoCorrectThreshold float64 `json:"auto_correct_threshold"`

	// SchemaPath is an optional custom schema path
	SchemaPath string `json:"schema_path,omitempty"`

	// Created is the timestamp when the proof was initialized
	Created time.Time `json:"created"`

	// Version is the schema version (must be "1.0")
	Version string `json:"version"`
}

// Load reads and parses a config file from the given path.
// Missing optional fields are filled with defaults.
// Returns an error if the file cannot be read or parsed.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Fill missing optional fields with defaults
	if cfg.LockTimeout == 0 {
		cfg.LockTimeout = 5 * time.Minute
	}
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 20
	}
	if cfg.MaxChildren == 0 {
		cfg.MaxChildren = 10
	}
	if cfg.WarnDepth == 0 {
		cfg.WarnDepth = 3
	}
	if cfg.AutoCorrectThreshold == 0 {
		cfg.AutoCorrectThreshold = 0.8
	}

	return &cfg, nil
}

// Default returns a Config with default values.
// Title and Conjecture are left empty and must be set by the caller.
// Created is set to the current time.
func Default() *Config {
	return &Config{
		LockTimeout:          5 * time.Minute,
		MaxDepth:             20,
		MaxChildren:          10,
		WarnDepth:            3,
		AutoCorrectThreshold: 0.8,
		Version:              "1.0",
		Created:              time.Now(),
	}
}

// Validate checks that all config values are within acceptable bounds.
// Returns an error describing the first validation failure, or nil if valid.
//
// Validation rules:
// - Title must not be empty
// - Conjecture must not be empty
// - LockTimeout must be between 1s and 1h
// - MaxDepth must be between 1 and MaxDepthLimit (100)
// - MaxChildren must be between 1 and 50
// - AutoCorrectThreshold must be between 0.0 and 1.0
// - Version must be "1.0"
func Validate(c *Config) error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if c.Title == "" {
		return fmt.Errorf("title must not be empty")
	}

	if c.Conjecture == "" {
		return fmt.Errorf("conjecture must not be empty")
	}

	if c.LockTimeout < time.Second || c.LockTimeout > time.Hour {
		return fmt.Errorf("lock_timeout must be between 1s and 1h, got %v", c.LockTimeout)
	}

	if c.MaxDepth < 1 || c.MaxDepth > MaxDepthLimit {
		return fmt.Errorf("max_depth must be between 1 and %d, got %d", MaxDepthLimit, c.MaxDepth)
	}

	if c.MaxChildren < 1 || c.MaxChildren > 50 {
		return fmt.Errorf("max_children must be between 1 and 50, got %d", c.MaxChildren)
	}

	if c.AutoCorrectThreshold < 0.0 || c.AutoCorrectThreshold > 1.0 {
		return fmt.Errorf("auto_correct_threshold must be between 0.0 and 1.0, got %f", c.AutoCorrectThreshold)
	}

	if c.Version != "1.0" {
		return fmt.Errorf("version must be \"1.0\", got %q", c.Version)
	}

	return nil
}

// Save writes the config to the given path as formatted JSON.
// Returns an error if the file cannot be written.
func Save(c *Config, path string) error {
	if path == "" {
		return fmt.Errorf("config path cannot be empty")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}
