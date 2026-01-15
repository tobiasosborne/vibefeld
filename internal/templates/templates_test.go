package templates

import (
	"testing"

	"github.com/tobias/vibefeld/internal/schema"
)

func TestGetTemplate_Contradiction(t *testing.T) {
	tmpl, ok := Get("contradiction")
	if !ok {
		t.Fatal("expected contradiction template to exist")
	}

	if tmpl.Name != "contradiction" {
		t.Errorf("expected name 'contradiction', got %q", tmpl.Name)
	}

	if len(tmpl.Children) != 2 {
		t.Errorf("expected 2 children in contradiction template, got %d", len(tmpl.Children))
	}

	// First child should be assumption (local_assume type)
	if tmpl.Children[0].NodeType != schema.NodeTypeLocalAssume {
		t.Errorf("expected first child to be local_assume, got %s", tmpl.Children[0].NodeType)
	}

	// Second child should be claim to derive contradiction
	if tmpl.Children[1].NodeType != schema.NodeTypeClaim {
		t.Errorf("expected second child to be claim, got %s", tmpl.Children[1].NodeType)
	}
}

func TestGetTemplate_Induction(t *testing.T) {
	tmpl, ok := Get("induction")
	if !ok {
		t.Fatal("expected induction template to exist")
	}

	if tmpl.Name != "induction" {
		t.Errorf("expected name 'induction', got %q", tmpl.Name)
	}

	if len(tmpl.Children) != 2 {
		t.Errorf("expected 2 children in induction template, got %d", len(tmpl.Children))
	}

	// Verify base case and inductive step exist
	hasBase := false
	hasInductive := false
	for _, child := range tmpl.Children {
		if child.StatementTemplate != "" {
			lower := child.StatementTemplate
			if contains(lower, "base") {
				hasBase = true
			}
			if contains(lower, "inductive") {
				hasInductive = true
			}
		}
	}

	if !hasBase {
		t.Error("expected induction template to have a base case child")
	}
	if !hasInductive {
		t.Error("expected induction template to have an inductive step child")
	}
}

func TestGetTemplate_Cases(t *testing.T) {
	tmpl, ok := Get("cases")
	if !ok {
		t.Fatal("expected cases template to exist")
	}

	if tmpl.Name != "cases" {
		t.Errorf("expected name 'cases', got %q", tmpl.Name)
	}

	if len(tmpl.Children) < 2 {
		t.Errorf("expected at least 2 children in cases template, got %d", len(tmpl.Children))
	}

	// All children should be case type
	for i, child := range tmpl.Children {
		if child.NodeType != schema.NodeTypeCase {
			t.Errorf("expected child %d to be case type, got %s", i+1, child.NodeType)
		}
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	_, ok := Get("nonexistent")
	if ok {
		t.Error("expected nonexistent template to not be found")
	}
}

func TestList(t *testing.T) {
	templates := List()

	// Should have at least 3 templates
	if len(templates) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(templates))
	}

	// Check that required templates are present
	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}

	required := []string{"contradiction", "induction", "cases"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("expected template %q to be in list", name)
		}
	}
}

func TestTemplateDescription(t *testing.T) {
	templates := List()

	for _, tmpl := range templates {
		if tmpl.Description == "" {
			t.Errorf("template %q should have a description", tmpl.Name)
		}
	}
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsLower(toLower(s), toLower(substr)))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
