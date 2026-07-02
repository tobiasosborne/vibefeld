package schema

import "testing"

func TestValidateChallengeCategory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty is valid (optional)", "", false},
		{"gap", "gap", false},
		{"missing", "missing", false},
		{"dependency", "dependency", false},
		{"incorrect", "incorrect", false},
		{"unclear", "unclear", false},
		{"other", "other", false},
		{"unknown value", "bogus", true},
		{"wrong case", "Gap", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChallengeCategory(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateChallengeCategory(%q) = nil, want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateChallengeCategory(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestAllChallengeCategories_SortedAndComplete(t *testing.T) {
	all := AllChallengeCategories()
	if len(all) != 6 {
		t.Fatalf("expected 6 categories, got %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ID >= all[i].ID {
			t.Errorf("categories not sorted: %q >= %q", all[i-1].ID, all[i].ID)
		}
	}
	for _, info := range all {
		if info.Description == "" {
			t.Errorf("category %q has empty description", info.ID)
		}
	}
}

func TestValidChallengeCategoryStrings(t *testing.T) {
	got := ValidChallengeCategoryStrings()
	if len(got) != 6 {
		t.Fatalf("expected 6 category strings, got %d: %v", len(got), got)
	}
	if got[0] != "dependency" {
		t.Errorf("expected first (sorted) category to be 'dependency', got %q", got[0])
	}
}
