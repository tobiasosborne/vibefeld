package schema

import (
	"testing"
)

// TestValidateInference_AllValid tests that all 11 valid inference types pass validation
func TestValidateInference_AllValid(t *testing.T) {
	tests := []struct {
		name      string
		inference string
	}{
		{"modus_ponens", "modus_ponens"},
		{"modus_tollens", "modus_tollens"},
		{"universal_instantiation", "universal_instantiation"},
		{"existential_instantiation", "existential_instantiation"},
		{"universal_generalization", "universal_generalization"},
		{"existential_generalization", "existential_generalization"},
		{"by_definition", "by_definition"},
		{"assumption", "assumption"},
		{"local_assume", "local_assume"},
		{"local_discharge", "local_discharge"},
		{"contradiction", "contradiction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInference(tt.inference)
			if err != nil {
				t.Errorf("ValidateInference(%q) returned error: %v, want nil", tt.inference, err)
			}
		})
	}
}

// TestValidateInference_Invalid tests that invalid inference types fail validation
func TestValidateInference_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		inference string
	}{
		{"unknown type", "foo"},
		{"empty string", ""},
		{"space in name", "modus ponens"},
		{"uppercase", "MODUS_PONENS"},
		{"partial match", "modus"},
		{"mixed case", "Modus_Ponens"},
		{"typo", "modus_pones"},
		{"extra chars", "modus_ponens_extra"},
		{"hyphen instead of underscore", "modus-ponens"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInference(tt.inference)
			if err == nil {
				t.Errorf("ValidateInference(%q) returned nil, want error", tt.inference)
			}
		})
	}
}

// TestGetInferenceInfo_Exists tests that valid inference types return correct metadata
func TestGetInferenceInfo_Exists(t *testing.T) {
	tests := []struct {
		inference InferenceType
		wantName  string
		wantForm  string
	}{
		{
			inference: InferenceModusPonens,
			wantName:  "Modus Ponens",
			wantForm:  "P, P → Q ⊢ Q",
		},
		{
			inference: InferenceModusTollens,
			wantName:  "Modus Tollens",
			wantForm:  "¬Q, P → Q ⊢ ¬P",
		},
		{
			inference: InferenceUniversalInstantiation,
			wantName:  "Universal Instantiation",
			wantForm:  "∀x.P(x) ⊢ P(t)",
		},
		{
			inference: InferenceExistentialInstantiation,
			wantName:  "Existential Instantiation",
			wantForm:  "∃x.P(x) ⊢ P(c) for fresh c",
		},
		{
			inference: InferenceUniversalGeneralization,
			wantName:  "Universal Generalization",
			wantForm:  "P(x) for arbitrary x ⊢ ∀x.P(x)",
		},
		{
			inference: InferenceExistentialGeneralization,
			wantName:  "Existential Generalization",
			wantForm:  "P(c) ⊢ ∃x.P(x)",
		},
		{
			inference: InferenceByDefinition,
			wantName:  "By Definition",
			wantForm:  "unfold definition",
		},
		{
			inference: InferenceAssumption,
			wantName:  "Assumption",
			wantForm:  "global hypothesis",
		},
		{
			inference: InferenceLocalAssume,
			wantName:  "Local Assume",
			wantForm:  "introduce local hypothesis",
		},
		{
			inference: InferenceLocalDischarge,
			wantName:  "Local Discharge",
			wantForm:  "conclude from local hypothesis",
		},
		{
			inference: InferenceContradiction,
			wantName:  "Contradiction",
			wantForm:  "P ∧ ¬P ⊢ ⊥",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.inference), func(t *testing.T) {
			info, ok := GetInferenceInfo(tt.inference)
			if !ok {
				t.Fatalf("GetInferenceInfo(%q) returned ok=false, want true", tt.inference)
			}

			if info.ID != tt.inference {
				t.Errorf("GetInferenceInfo(%q).ID = %q, want %q", tt.inference, info.ID, tt.inference)
			}

			if info.Name != tt.wantName {
				t.Errorf("GetInferenceInfo(%q).Name = %q, want %q", tt.inference, info.Name, tt.wantName)
			}

			if info.Form != tt.wantForm {
				t.Errorf("GetInferenceInfo(%q).Form = %q, want %q", tt.inference, info.Form, tt.wantForm)
			}
		})
	}
}

// TestGetInferenceInfo_NotExists tests that invalid inference types return false
func TestGetInferenceInfo_NotExists(t *testing.T) {
	tests := []struct {
		name      string
		inference InferenceType
	}{
		{"empty", InferenceType("")},
		{"invalid", InferenceType("invalid")},
		{"uppercase", InferenceType("MODUS_PONENS")},
		{"typo", InferenceType("modus_ponen")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetInferenceInfo(tt.inference)
			if ok {
				t.Errorf("GetInferenceInfo(%q) returned ok=true, want false", tt.inference)
			}
		})
	}
}

// TestAllInferences_Count tests that AllInferences returns exactly 11 types
func TestAllInferences_Count(t *testing.T) {
	all := AllInferences()
	if len(all) != 11 {
		t.Errorf("AllInferences() returned %d types, want 11", len(all))
	}
}

// TestAllInferences_Complete tests that AllInferences contains all expected IDs
func TestAllInferences_Complete(t *testing.T) {
	expected := []InferenceType{
		InferenceModusPonens,
		InferenceModusTollens,
		InferenceUniversalInstantiation,
		InferenceExistentialInstantiation,
		InferenceUniversalGeneralization,
		InferenceExistentialGeneralization,
		InferenceByDefinition,
		InferenceAssumption,
		InferenceLocalAssume,
		InferenceLocalDischarge,
		InferenceContradiction,
	}

	all := AllInferences()

	// Create a map for efficient lookup
	found := make(map[InferenceType]bool)
	for _, info := range all {
		found[info.ID] = true
	}

	// Check that all expected types are present
	for _, exp := range expected {
		if !found[exp] {
			t.Errorf("AllInferences() missing expected type: %q", exp)
		}
	}

	// Check that there are no extra types
	if len(found) != len(expected) {
		t.Errorf("AllInferences() returned %d unique types, expected %d", len(found), len(expected))
	}
}

// TestAllInferences_NoDuplicates tests that AllInferences contains no duplicate IDs
func TestAllInferences_NoDuplicates(t *testing.T) {
	all := AllInferences()
	seen := make(map[InferenceType]bool)

	for _, info := range all {
		if seen[info.ID] {
			t.Errorf("AllInferences() contains duplicate ID: %q", info.ID)
		}
		seen[info.ID] = true
	}
}

// TestAllInferences_ValidMetadata tests that all returned inferences have valid metadata
func TestAllInferences_ValidMetadata(t *testing.T) {
	all := AllInferences()

	for _, info := range all {
		t.Run(string(info.ID), func(t *testing.T) {
			if info.ID == "" {
				t.Error("InferenceInfo has empty ID")
			}
			if info.Name == "" {
				t.Error("InferenceInfo has empty Name")
			}
			if info.Form == "" {
				t.Error("InferenceInfo has empty Form")
			}
		})
	}
}

// TestSuggestInference_ExactMatch tests that exact input returns exact match
func TestSuggestInference_ExactMatch(t *testing.T) {
	tests := []struct {
		input string
		want  InferenceType
	}{
		{"modus_ponens", InferenceModusPonens},
		{"modus_tollens", InferenceModusTollens},
		{"universal_instantiation", InferenceUniversalInstantiation},
		{"existential_instantiation", InferenceExistentialInstantiation},
		{"universal_generalization", InferenceUniversalGeneralization},
		{"existential_generalization", InferenceExistentialGeneralization},
		{"by_definition", InferenceByDefinition},
		{"assumption", InferenceAssumption},
		{"local_assume", InferenceLocalAssume},
		{"local_discharge", InferenceLocalDischarge},
		{"contradiction", InferenceContradiction},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := SuggestInference(tt.input)
			if !ok {
				t.Fatalf("SuggestInference(%q) returned ok=false, want true", tt.input)
			}
			if got != tt.want {
				t.Errorf("SuggestInference(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSuggestInference_FuzzyMatch tests that fuzzy matching works for near-matches
func TestSuggestInference_FuzzyMatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  InferenceType
	}{
		{"typo one char", "modus_ponen", InferenceModusPonens},
		{"typo two chars", "modus_tone", InferenceModusTollens},
		{"missing underscore", "modusponens", InferenceModusPonens},
		{"extra char", "modus_ponenss", InferenceModusPonens},
		{"transposition", "modus_ponnes", InferenceModusPonens},
		{"missing final char", "assumptio", InferenceAssumption},
		{"single char typo", "contradictio", InferenceContradiction},
		{"prefix match", "by_def", InferenceByDefinition},
		{"abbreviated", "univ_inst", InferenceUniversalInstantiation},
		{"local_assum typo", "local_assum", InferenceLocalAssume},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SuggestInference(tt.input)
			if !ok {
				t.Fatalf("SuggestInference(%q) returned ok=false, want true for fuzzy match", tt.input)
			}
			if got != tt.want {
				t.Errorf("SuggestInference(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSuggestInference_NoMatch tests that very different input returns false
func TestSuggestInference_NoMatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"random string", "xyz"},
		{"empty", ""},
		{"completely different", "foobar"},
		{"too short", "a"},
		{"too different", "zzzzzzzzz"},
		{"numbers", "12345"},
		{"special chars", "@#$%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := SuggestInference(tt.input)
			if ok {
				t.Errorf("SuggestInference(%q) returned ok=true, want false (no match)", tt.input)
			}
		})
	}
}

// TestSuggestInference_CaseInsensitive tests that fuzzy matching handles case variations
func TestSuggestInference_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
		want  InferenceType
	}{
		{"MODUS_PONENS", InferenceModusPonens},
		{"Modus_Ponens", InferenceModusPonens},
		{"Assumption", InferenceAssumption},
		{"ASSUMPTION", InferenceAssumption},
		{"Local_Assume", InferenceLocalAssume},
		{"BY_DEFINITION", InferenceByDefinition},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := SuggestInference(tt.input)
			if !ok {
				t.Fatalf("SuggestInference(%q) returned ok=false, want true (case insensitive match)", tt.input)
			}
			if got != tt.want {
				t.Errorf("SuggestInference(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSuggestInference_ReturnsClosest tests that fuzzy matching returns the closest match
func TestSuggestInference_ReturnsClosest(t *testing.T) {
	// "assumpt" is closer to "assumption" (1 char away) than "local_assume" (7 chars away)
	got, ok := SuggestInference("assumpt")
	if !ok {
		t.Fatal("SuggestInference('assumpt') returned ok=false, want true")
	}
	if got != InferenceAssumption {
		t.Errorf("SuggestInference('assumpt') = %q, want %q (closest match)", got, InferenceAssumption)
	}

	// "by_def" is closer to "by_definition" (7 chars away) than others
	got, ok = SuggestInference("by_def")
	if !ok {
		t.Fatal("SuggestInference('by_def') returned ok=false, want true")
	}
	if got != InferenceByDefinition {
		t.Errorf("SuggestInference('by_def') = %q, want %q (closest match)", got, InferenceByDefinition)
	}
}

// TestValidateInference_ThenGetInfo tests the workflow of validating then getting info
func TestValidateInference_ThenGetInfo(t *testing.T) {
	inference := "modus_ponens"

	// First validate
	err := ValidateInference(inference)
	if err != nil {
		t.Fatalf("ValidateInference(%q) returned error: %v", inference, err)
	}

	// Then get info
	info, ok := GetInferenceInfo(InferenceType(inference))
	if !ok {
		t.Fatalf("GetInferenceInfo(%q) returned ok=false after validation passed", inference)
	}

	if info.ID != InferenceModusPonens {
		t.Errorf("GetInferenceInfo(%q).ID = %q, want %q", inference, info.ID, InferenceModusPonens)
	}
}

// TestSuggestInference_ThenValidate tests the workflow of suggesting then validating
func TestSuggestInference_ThenValidate(t *testing.T) {
	input := "modus_ponen" // typo

	// First suggest
	suggestion, ok := SuggestInference(input)
	if !ok {
		t.Fatalf("SuggestInference(%q) returned ok=false", input)
	}

	// Then validate the suggestion
	err := ValidateInference(string(suggestion))
	if err != nil {
		t.Errorf("ValidateInference(%q) returned error: %v, want nil (suggested inference should be valid)", suggestion, err)
	}

	// Should be modus_ponens
	if suggestion != InferenceModusPonens {
		t.Errorf("SuggestInference(%q) = %q, want %q", input, suggestion, InferenceModusPonens)
	}
}

// TestInferenceType_Constants tests that all constants have expected string values
func TestInferenceType_Constants(t *testing.T) {
	tests := []struct {
		constant InferenceType
		want     string
	}{
		{InferenceModusPonens, "modus_ponens"},
		{InferenceModusTollens, "modus_tollens"},
		{InferenceUniversalInstantiation, "universal_instantiation"},
		{InferenceExistentialInstantiation, "existential_instantiation"},
		{InferenceUniversalGeneralization, "universal_generalization"},
		{InferenceExistentialGeneralization, "existential_generalization"},
		{InferenceByDefinition, "by_definition"},
		{InferenceAssumption, "assumption"},
		{InferenceLocalAssume, "local_assume"},
		{InferenceLocalDischarge, "local_discharge"},
		{InferenceContradiction, "contradiction"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.constant) != tt.want {
				t.Errorf("constant %q has value %q, want %q", tt.want, string(tt.constant), tt.want)
			}
		})
	}
}
