package schema_test

import (
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
)

// TestChallengeSeverity_Constants verifies severity constants exist and are distinct.
func TestChallengeSeverity_Constants(t *testing.T) {
	severities := []schema.ChallengeSeverity{
		schema.SeverityCritical,
		schema.SeverityMajor,
		schema.SeverityMinor,
		schema.SeverityNote,
	}

	// All severities must be non-empty and unique
	seen := make(map[schema.ChallengeSeverity]bool)
	for _, s := range severities {
		if s == "" {
			t.Error("severity constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate severity value: %q", s)
		}
		seen[s] = true
	}
}

// TestValidateChallengeSeverity_ValidValues tests validation accepts valid severities.
func TestValidateChallengeSeverity_ValidValues(t *testing.T) {
	tests := []struct {
		name     string
		severity string
	}{
		{"critical", "critical"},
		{"major", "major"},
		{"minor", "minor"},
		{"note", "note"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := schema.ValidateChallengeSeverity(tc.severity)
			if err != nil {
				t.Errorf("ValidateChallengeSeverity(%q) expected no error, got: %v", tc.severity, err)
			}
		})
	}
}

// TestValidateChallengeSeverity_InvalidValues tests validation rejects invalid severities.
func TestValidateChallengeSeverity_InvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		severity string
	}{
		{"empty string", ""},
		{"unknown", "unknown"},
		{"typo", "critica"},
		{"uppercase", "CRITICAL"},
		{"mixed case", "Critical"},
		{"whitespace", " critical "},
		{"random", "high"},
		{"numeric", "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := schema.ValidateChallengeSeverity(tc.severity)
			if err == nil {
				t.Errorf("ValidateChallengeSeverity(%q) expected error, got nil", tc.severity)
			}
		})
	}
}

// TestChallengeSeverityBlocksAcceptance tests which severities block acceptance.
func TestChallengeSeverityBlocksAcceptance(t *testing.T) {
	tests := []struct {
		severity     schema.ChallengeSeverity
		shouldBlock  bool
		description  string
	}{
		{schema.SeverityCritical, true, "critical severity blocks acceptance"},
		{schema.SeverityMajor, true, "major severity blocks acceptance"},
		{schema.SeverityMinor, false, "minor severity does not block acceptance"},
		{schema.SeverityNote, false, "note severity does not block acceptance"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			blocks := schema.SeverityBlocksAcceptance(tc.severity)
			if blocks != tc.shouldBlock {
				t.Errorf("SeverityBlocksAcceptance(%q) = %v, want %v", tc.severity, blocks, tc.shouldBlock)
			}
		})
	}
}

// TestAllChallengeSeverities returns all valid severity levels.
func TestAllChallengeSeverities(t *testing.T) {
	severities := schema.AllChallengeSeverities()

	if len(severities) != 4 {
		t.Errorf("AllChallengeSeverities() returned %d severities, want 4", len(severities))
	}

	// Verify each expected severity is present
	expected := map[schema.ChallengeSeverity]bool{
		schema.SeverityCritical: false,
		schema.SeverityMajor:    false,
		schema.SeverityMinor:    false,
		schema.SeverityNote:     false,
	}

	for _, s := range severities {
		if _, ok := expected[s.ID]; !ok {
			t.Errorf("unexpected severity in result: %q", s.ID)
		}
		expected[s.ID] = true
	}

	for id, found := range expected {
		if !found {
			t.Errorf("severity %q not found in result", id)
		}
	}
}

// TestGetChallengeSeverityInfo returns info for valid severities.
func TestGetChallengeSeverityInfo(t *testing.T) {
	tests := []struct {
		severity    schema.ChallengeSeverity
		wantExists  bool
		wantBlocks  bool
		description string
	}{
		{schema.SeverityCritical, true, true, "critical exists and blocks"},
		{schema.SeverityMajor, true, true, "major exists and blocks"},
		{schema.SeverityMinor, true, false, "minor exists and does not block"},
		{schema.SeverityNote, true, false, "note exists and does not block"},
		{schema.ChallengeSeverity("invalid"), false, false, "invalid does not exist"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			info, exists := schema.GetChallengeSeverityInfo(tc.severity)
			if exists != tc.wantExists {
				t.Errorf("GetChallengeSeverityInfo(%q) exists = %v, want %v", tc.severity, exists, tc.wantExists)
			}
			if exists && info.BlocksAcceptance != tc.wantBlocks {
				t.Errorf("GetChallengeSeverityInfo(%q).BlocksAcceptance = %v, want %v", tc.severity, info.BlocksAcceptance, tc.wantBlocks)
			}
		})
	}
}

// TestDefaultChallengeSeverity returns major as the default.
func TestDefaultChallengeSeverity(t *testing.T) {
	defaultSeverity := schema.DefaultChallengeSeverity()
	if defaultSeverity != schema.SeverityMajor {
		t.Errorf("DefaultChallengeSeverity() = %q, want %q", defaultSeverity, schema.SeverityMajor)
	}
}

// TestChallengeSeverityInfo_Description ensures all severities have descriptions.
func TestChallengeSeverityInfo_Description(t *testing.T) {
	severities := schema.AllChallengeSeverities()
	for _, s := range severities {
		if s.Description == "" {
			t.Errorf("severity %q has empty description", s.ID)
		}
	}
}
