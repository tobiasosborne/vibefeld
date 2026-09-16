// Package config provides configuration loading and validation for AF proofs.
// Configuration is stored in meta.json in the proof directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
)

// Workspace format and policy versions.
//
// FormatCurrent is the workspace format this binary writes. It is stored in
// meta.json's "version" field and gates which event types may be appended.
// PolicyVersion is the acceptance/claim policy number advertised by
// `af version`; it is deliberately separate from the workspace format so that
// 0.1.9 and 0.1.10 can both read format 1.1 while differing in policy.
const (
	// FormatCurrent is the newest workspace format this binary can read and write.
	FormatCurrent = "1.1"

	// PolicyVersion is the acceptance/claim policy generation. It starts at
	// 0.1.9 and changes independently of the workspace format.
	PolicyVersion = "0.1.9"
)

// FormatsReadable lists every workspace format this binary can read. The
// first entry is the original format; FormatCurrent is the newest.
var FormatsReadable = []string{"1.0", FormatCurrent}

// ErrFormatTooNew is returned by CheckFormat when meta.json names a workspace
// format this binary cannot read (newer than FormatCurrent, or unknown).
var ErrFormatTooNew = aferrors.New(aferrors.FORMAT_TOO_NEW, "workspace format is not readable by this af")

// IsFormatReadable reports whether v is one of FormatsReadable.
func IsFormatReadable(v string) bool {
	for _, f := range FormatsReadable {
		if v == f {
			return true
		}
	}
	return false
}

// parseFormat parses a "major.minor" workspace format. It returns ok=false for
// anything that is not exactly two non-negative integers separated by a dot.
func parseFormat(v string) (major, minor int, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, false
	}
	return major, minor, true
}

// CompareFormats orders two workspace formats: -1 if a < b, 0 if equal, 1 if
// a > b. Formats that do not parse as major.minor fall back to a byte-wise
// string comparison so callers still get a stable ordering.
func CompareFormats(a, b string) int {
	amaj, amin, aok := parseFormat(a)
	bmaj, bmin, bok := parseFormat(b)
	if !aok || !bok {
		return strings.Compare(a, b)
	}
	if amaj != bmaj {
		if amaj < bmaj {
			return -1
		}
		return 1
	}
	if amin != bmin {
		if amin < bmin {
			return -1
		}
		return 1
	}
	return 0
}

// CheckFormat returns a structured error (exit code 3, FORMAT_TOO_NEW) when
// cfg names a workspace format this binary cannot read. It is a no-op for a
// nil config or any readable format. The message names both the workspace
// format and the running binary's format.
func CheckFormat(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if IsFormatReadable(cfg.Version) {
		return nil
	}
	return aferrors.Newf(aferrors.FORMAT_TOO_NEW,
		"workspace format %q is not readable by this af (running format %s); upgrade af to a newer binary, or run `af workspace upgrade --to %s` on an older workspace",
		cfg.Version, FormatCurrent, FormatCurrent)
}

// MaxDepthLimit is the maximum allowed value for MaxDepth configuration.
// This prevents excessively deep proof trees that could cause performance issues.
const MaxDepthLimit = 100

// DefaultClaimTimeout is the default duration for claim timeouts in CLI commands.
// Used by claim and extend-claim commands when no explicit timeout is specified.
const DefaultClaimTimeout = "1h"

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

	// MaxChildren is the maximum number of children per node (default: 100)
	MaxChildren int `json:"max_children"`

	// WarnDepth is the depth at which warnings are issued for deep nodes (default: 3)
	WarnDepth int `json:"warn_depth"`

	// AutoCorrectThreshold is the fuzzy match threshold for auto-correction (default: 0.8)
	AutoCorrectThreshold float64 `json:"auto_correct_threshold"`

	// SchemaPath is an optional custom schema path
	SchemaPath string `json:"schema_path,omitempty"`

	// Created is the timestamp when the proof was initialized
	Created time.Time `json:"created"`

	// Version is the workspace format stored in meta.json. It names the format
	// of the events this workspace may contain — see FormatCurrent and
	// FormatsReadable. Use CheckFormat to refuse formats this binary cannot read.
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
		cfg.MaxChildren = 100
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
		MaxChildren:          100,
		WarnDepth:            3,
		AutoCorrectThreshold: 0.8,
		Version:              FormatCurrent,
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
// - MaxChildren must be between 1 and 100
// - AutoCorrectThreshold must be between 0.0 and 1.0
// - Version must be one of FormatsReadable
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

	if c.MaxChildren < 1 || c.MaxChildren > 100 {
		return fmt.Errorf("max_children must be between 1 and 100, got %d", c.MaxChildren)
	}

	if c.AutoCorrectThreshold < 0.0 || c.AutoCorrectThreshold > 1.0 {
		return fmt.Errorf("auto_correct_threshold must be between 0.0 and 1.0, got %f", c.AutoCorrectThreshold)
	}

	if !IsFormatReadable(c.Version) {
		return fmt.Errorf("version must be a readable workspace format (%s), got %q", strings.Join(FormatsReadable, ", "), c.Version)
	}

	return nil
}

// Save writes the config to the given path as formatted JSON using atomic
// semantics: marshal, write to a temp file in the same directory, fsync it,
// rename over the target, then fsync the directory so the rename survives a
// crash. Returns an error if the file cannot be written.
func Save(c *Config, path string) error {
	if path == "" {
		return fmt.Errorf("config path cannot be empty")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".meta-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp config: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp config: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to set config permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return fsyncDir(dir)
}

// fsyncDir opens dir read-only and fsyncs it so a preceding rename is durable.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
