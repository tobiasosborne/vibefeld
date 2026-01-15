// Package render provides human-readable formatting for AF framework types.
package render

import (
	"os"
	"strings"

	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
)

// ANSI escape codes for terminal colors
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
)

// colorEnabled tracks whether color output is enabled.
// Defaults to true, but can be disabled via NO_COLOR environment variable
// or by calling DisableColor().
var colorEnabled = true

func init() {
	// Check NO_COLOR environment variable (https://no-color.org/)
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		colorEnabled = false
	}
	// Also check TERM=dumb which indicates no color support
	if os.Getenv("TERM") == "dumb" {
		colorEnabled = false
	}
}

// EnableColor enables color output.
func EnableColor() {
	colorEnabled = true
}

// DisableColor disables color output.
func DisableColor() {
	colorEnabled = false
}

// IsColorEnabled returns whether color output is currently enabled.
func IsColorEnabled() bool {
	return colorEnabled
}

// colorize wraps text with ANSI color codes if color is enabled.
func colorize(text, colorCode string) string {
	if !colorEnabled {
		return text
	}
	return colorCode + text + ansiReset
}

// Red returns text colored red.
func Red(text string) string {
	return colorize(text, ansiRed)
}

// Green returns text colored green.
func Green(text string) string {
	return colorize(text, ansiGreen)
}

// Yellow returns text colored yellow.
func Yellow(text string) string {
	return colorize(text, ansiYellow)
}

// Blue returns text colored blue.
func Blue(text string) string {
	return colorize(text, ansiBlue)
}

// Cyan returns text colored cyan.
func Cyan(text string) string {
	return colorize(text, ansiCyan)
}

// Magenta returns text colored magenta.
func Magenta(text string) string {
	return colorize(text, ansiMagenta)
}

// Gray returns text colored gray.
func Gray(text string) string {
	return colorize(text, ansiGray)
}

// Bold returns text in bold.
func Bold(text string) string {
	return colorize(text, ansiBold)
}

// ColorEpistemicState returns the epistemic state string with appropriate color coding.
// Color mapping:
//   - pending = yellow (requires attention)
//   - validated = green (success)
//   - admitted = cyan (accepted but with epistemic uncertainty)
//   - refuted = red (proven false)
//   - archived = gray (inactive/superseded)
func ColorEpistemicState(state schema.EpistemicState) string {
	s := string(state)
	switch state {
	case schema.EpistemicPending:
		return Yellow(s)
	case schema.EpistemicValidated:
		return Green(s)
	case schema.EpistemicAdmitted:
		return Cyan(s)
	case schema.EpistemicRefuted:
		return Red(s)
	case schema.EpistemicArchived:
		return Gray(s)
	default:
		return s
	}
}

// ColorTaintState returns the taint state string with appropriate color coding.
// Color mapping:
//   - clean = green (no epistemic uncertainty)
//   - self_admitted = cyan (contains admitted node)
//   - tainted = red (depends on tainted/refuted node)
//   - unresolved = yellow (taint not yet computed)
func ColorTaintState(state node.TaintState) string {
	s := string(state)
	switch state {
	case node.TaintClean:
		return Green(s)
	case node.TaintSelfAdmitted:
		return Cyan(s)
	case node.TaintTainted:
		return Red(s)
	case node.TaintUnresolved:
		return Yellow(s)
	default:
		return s
	}
}

// StripANSI removes ANSI escape codes from a string.
// Useful for testing and plain-text output.
func StripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
