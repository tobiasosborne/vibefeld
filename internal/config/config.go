// Package config provides configuration loading and validation for AF proofs.
// Configuration is stored in meta.json in the proof directory.
package config

import (
	"errors"
	"time"
)

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
	return nil, errors.New("not implemented")
}

// Default returns a Config with default values.
// Title and Conjecture are left empty and must be set by the caller.
// Created is set to the current time.
func Default() *Config {
	return nil // TODO: implement
}

// Validate checks that all config values are within acceptable bounds.
// Returns an error describing the first validation failure, or nil if valid.
//
// Validation rules:
// - Title must not be empty
// - Conjecture must not be empty
// - LockTimeout must be between 1s and 1h
// - MaxDepth must be between 1 and 100
// - MaxChildren must be between 1 and 50
// - AutoCorrectThreshold must be between 0.0 and 1.0
// - Version must be "1.0"
func Validate(c *Config) error {
	return errors.New("not implemented")
}

// Save writes the config to the given path as formatted JSON.
// Returns an error if the file cannot be written.
func Save(c *Config, path string) error {
	return errors.New("not implemented")
}
