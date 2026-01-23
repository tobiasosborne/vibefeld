package render

import (
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// saveColorState saves the current color state and returns a restore function.
func saveColorState() func() {
	original := colorEnabled
	return func() {
		colorEnabled = original
	}
}

// TestColorize_Enabled tests that colorize adds ANSI codes when enabled.
func TestColorize_Enabled(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := colorize("test", ansiRed)

	if !strings.Contains(result, "\033[31m") {
		t.Errorf("colorize should add ANSI red code, got: %q", result)
	}
	if !strings.Contains(result, "\033[0m") {
		t.Errorf("colorize should add ANSI reset code, got: %q", result)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("colorize should preserve text, got: %q", result)
	}
}

// TestColorize_Disabled tests that colorize returns plain text when disabled.
func TestColorize_Disabled(t *testing.T) {
	restore := saveColorState()
	defer restore()
	DisableColor()

	result := colorize("test", ansiRed)

	if result != "test" {
		t.Errorf("colorize with color disabled should return plain text, got: %q", result)
	}
	if strings.Contains(result, "\033") {
		t.Errorf("colorize with color disabled should not contain escape codes, got: %q", result)
	}
}

// TestRed tests the Red function.
func TestRed(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Red("error")

	if !strings.Contains(result, "\033[31m") {
		t.Errorf("Red should use red color code, got: %q", result)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("Red should preserve text, got: %q", result)
	}
}

// TestGreen tests the Green function.
func TestGreen(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Green("success")

	if !strings.Contains(result, "\033[32m") {
		t.Errorf("Green should use green color code, got: %q", result)
	}
	if !strings.Contains(result, "success") {
		t.Errorf("Green should preserve text, got: %q", result)
	}
}

// TestYellow tests the Yellow function.
func TestYellow(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Yellow("warning")

	if !strings.Contains(result, "\033[33m") {
		t.Errorf("Yellow should use yellow color code, got: %q", result)
	}
	if !strings.Contains(result, "warning") {
		t.Errorf("Yellow should preserve text, got: %q", result)
	}
}

// TestBlue tests the Blue function.
func TestBlue(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Blue("info")

	if !strings.Contains(result, "\033[34m") {
		t.Errorf("Blue should use blue color code, got: %q", result)
	}
	if !strings.Contains(result, "info") {
		t.Errorf("Blue should preserve text, got: %q", result)
	}
}

// TestCyan tests the Cyan function.
func TestCyan(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Cyan("notice")

	if !strings.Contains(result, "\033[36m") {
		t.Errorf("Cyan should use cyan color code, got: %q", result)
	}
	if !strings.Contains(result, "notice") {
		t.Errorf("Cyan should preserve text, got: %q", result)
	}
}

// TestGray tests the Gray function.
func TestGray(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Gray("muted")

	if !strings.Contains(result, "\033[90m") {
		t.Errorf("Gray should use gray color code, got: %q", result)
	}
	if !strings.Contains(result, "muted") {
		t.Errorf("Gray should preserve text, got: %q", result)
	}
}

// TestBold tests the Bold function.
func TestBold(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := Bold("important")

	if !strings.Contains(result, "\033[1m") {
		t.Errorf("Bold should use bold code, got: %q", result)
	}
	if !strings.Contains(result, "important") {
		t.Errorf("Bold should preserve text, got: %q", result)
	}
}

// TestColorEpistemicState_Pending tests pending state is yellow.
func TestColorEpistemicState_Pending(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicPending)

	if !strings.Contains(result, "\033[33m") { // yellow
		t.Errorf("pending should be yellow, got: %q", result)
	}
	if !strings.Contains(result, "pending") {
		t.Errorf("result should contain 'pending', got: %q", result)
	}
}

// TestColorEpistemicState_Validated tests validated state is green.
func TestColorEpistemicState_Validated(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicValidated)

	if !strings.Contains(result, "\033[32m") { // green
		t.Errorf("validated should be green, got: %q", result)
	}
	if !strings.Contains(result, "validated") {
		t.Errorf("result should contain 'validated', got: %q", result)
	}
}

// TestColorEpistemicState_Admitted tests admitted state is cyan.
func TestColorEpistemicState_Admitted(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicAdmitted)

	if !strings.Contains(result, "\033[36m") { // cyan
		t.Errorf("admitted should be cyan, got: %q", result)
	}
	if !strings.Contains(result, "admitted") {
		t.Errorf("result should contain 'admitted', got: %q", result)
	}
}

// TestColorEpistemicState_Refuted tests refuted state is red.
func TestColorEpistemicState_Refuted(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicRefuted)

	if !strings.Contains(result, "\033[31m") { // red
		t.Errorf("refuted should be red, got: %q", result)
	}
	if !strings.Contains(result, "refuted") {
		t.Errorf("result should contain 'refuted', got: %q", result)
	}
}

// TestColorEpistemicState_Archived tests archived state is gray.
func TestColorEpistemicState_Archived(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicArchived)

	if !strings.Contains(result, "\033[90m") { // gray
		t.Errorf("archived should be gray, got: %q", result)
	}
	if !strings.Contains(result, "archived") {
		t.Errorf("result should contain 'archived', got: %q", result)
	}
}

// TestColorEpistemicState_NeedsRefinement tests needs_refinement state is magenta.
func TestColorEpistemicState_NeedsRefinement(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicNeedsRefinement)

	if !strings.Contains(result, "\033[35m") { // magenta
		t.Errorf("needs_refinement should be magenta, got: %q", result)
	}
	if !strings.Contains(result, "needs_refinement") {
		t.Errorf("result should contain 'needs_refinement', got: %q", result)
	}
}

// TestColorEpistemicState_Unknown tests unknown state returns plain text.
func TestColorEpistemicState_Unknown(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorEpistemicState(schema.EpistemicState("unknown"))

	// Should not contain any color codes
	if result != "unknown" {
		t.Errorf("unknown state should return plain text, got: %q", result)
	}
}

// TestColorEpistemicState_Disabled tests that color is not applied when disabled.
func TestColorEpistemicState_Disabled(t *testing.T) {
	restore := saveColorState()
	defer restore()
	DisableColor()

	result := ColorEpistemicState(schema.EpistemicPending)

	if result != "pending" {
		t.Errorf("with color disabled should return plain 'pending', got: %q", result)
	}
	if strings.Contains(result, "\033") {
		t.Errorf("with color disabled should not contain escape codes, got: %q", result)
	}
}

// TestColorTaintState_Clean tests clean state is green.
func TestColorTaintState_Clean(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorTaintState(node.TaintClean)

	if !strings.Contains(result, "\033[32m") { // green
		t.Errorf("clean should be green, got: %q", result)
	}
	if !strings.Contains(result, "clean") {
		t.Errorf("result should contain 'clean', got: %q", result)
	}
}

// TestColorTaintState_SelfAdmitted tests self_admitted state is cyan.
func TestColorTaintState_SelfAdmitted(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorTaintState(node.TaintSelfAdmitted)

	if !strings.Contains(result, "\033[36m") { // cyan
		t.Errorf("self_admitted should be cyan, got: %q", result)
	}
	if !strings.Contains(result, "self_admitted") {
		t.Errorf("result should contain 'self_admitted', got: %q", result)
	}
}

// TestColorTaintState_Tainted tests tainted state is red.
func TestColorTaintState_Tainted(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorTaintState(node.TaintTainted)

	if !strings.Contains(result, "\033[31m") { // red
		t.Errorf("tainted should be red, got: %q", result)
	}
	if !strings.Contains(result, "tainted") {
		t.Errorf("result should contain 'tainted', got: %q", result)
	}
}

// TestColorTaintState_Unresolved tests unresolved state is yellow.
func TestColorTaintState_Unresolved(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorTaintState(node.TaintUnresolved)

	if !strings.Contains(result, "\033[33m") { // yellow
		t.Errorf("unresolved should be yellow, got: %q", result)
	}
	if !strings.Contains(result, "unresolved") {
		t.Errorf("result should contain 'unresolved', got: %q", result)
	}
}

// TestColorTaintState_Unknown tests unknown state returns plain text.
func TestColorTaintState_Unknown(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	result := ColorTaintState(node.TaintState("unknown"))

	// Should not contain any color codes
	if result != "unknown" {
		t.Errorf("unknown state should return plain text, got: %q", result)
	}
}

// TestColorTaintState_Disabled tests that color is not applied when disabled.
func TestColorTaintState_Disabled(t *testing.T) {
	restore := saveColorState()
	defer restore()
	DisableColor()

	result := ColorTaintState(node.TaintClean)

	if result != "clean" {
		t.Errorf("with color disabled should return plain 'clean', got: %q", result)
	}
	if strings.Contains(result, "\033") {
		t.Errorf("with color disabled should not contain escape codes, got: %q", result)
	}
}

// TestStripANSI_NoEscapes tests StripANSI with plain text.
func TestStripANSI_NoEscapes(t *testing.T) {
	result := StripANSI("plain text")

	if result != "plain text" {
		t.Errorf("StripANSI should preserve plain text, got: %q", result)
	}
}

// TestStripANSI_WithEscapes tests StripANSI removes escape codes.
func TestStripANSI_WithEscapes(t *testing.T) {
	colored := "\033[31mred text\033[0m"
	result := StripANSI(colored)

	if result != "red text" {
		t.Errorf("StripANSI should remove escape codes, got: %q", result)
	}
}

// TestStripANSI_MultipleEscapes tests StripANSI with multiple codes.
func TestStripANSI_MultipleEscapes(t *testing.T) {
	colored := "\033[1m\033[31mBold Red\033[0m Normal \033[32mGreen\033[0m"
	result := StripANSI(colored)

	if result != "Bold Red Normal Green" {
		t.Errorf("StripANSI should remove all escape codes, got: %q", result)
	}
}

// TestStripANSI_EmptyString tests StripANSI with empty string.
func TestStripANSI_EmptyString(t *testing.T) {
	result := StripANSI("")

	if result != "" {
		t.Errorf("StripANSI of empty string should be empty, got: %q", result)
	}
}

// TestStripANSI_OnlyEscapes tests StripANSI with only escape codes.
func TestStripANSI_OnlyEscapes(t *testing.T) {
	result := StripANSI("\033[31m\033[0m")

	if result != "" {
		t.Errorf("StripANSI of only escape codes should be empty, got: %q", result)
	}
}

// TestEnableDisableColor tests the Enable/Disable toggle functions.
func TestEnableDisableColor(t *testing.T) {
	restore := saveColorState()
	defer restore()

	// Test enable
	EnableColor()
	if !IsColorEnabled() {
		t.Error("IsColorEnabled should return true after EnableColor")
	}

	// Test disable
	DisableColor()
	if IsColorEnabled() {
		t.Error("IsColorEnabled should return false after DisableColor")
	}

	// Test re-enable
	EnableColor()
	if !IsColorEnabled() {
		t.Error("IsColorEnabled should return true after re-enabling")
	}
}

// TestAllColorsProduceDistinctOutput tests that each color function produces unique output.
func TestAllColorsProduceDistinctOutput(t *testing.T) {
	restore := saveColorState()
	defer restore()
	EnableColor()

	colors := map[string]func(string) string{
		"Red":     Red,
		"Green":   Green,
		"Yellow":  Yellow,
		"Blue":    Blue,
		"Cyan":    Cyan,
		"Magenta": Magenta,
		"Gray":    Gray,
	}

	seen := make(map[string]string)
	for name, colorFunc := range colors {
		result := colorFunc("test")
		for otherName, otherResult := range seen {
			if result == otherResult {
				t.Errorf("%s and %s produce the same output", name, otherName)
			}
		}
		seen[name] = result
	}
}
