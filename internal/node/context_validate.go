// Package node provides data structures for proof nodes in the AF system.
package node

import (
	"strings"

	"github.com/tobias/vibefeld/internal/errors"
)

// ContextLookup is an interface for looking up context items by ID.
// This avoids import cycles by not depending on the state package directly.
// The state.State type implements this interface.
type ContextLookup interface {
	GetDefinition(id string) *Definition
	GetAssumption(id string) *Assumption
	GetExternal(id string) *External
}

// ValidateDefRefs checks that all definition references in a node exist in state.
// Returns nil if all references are valid, or DEF_NOT_FOUND error for missing definitions.
// Items that exist as assumptions or externals are skipped (they're not def refs).
func ValidateDefRefs(n *Node, s ContextLookup) error {
	if n == nil {
		return errors.New(errors.DEF_NOT_FOUND, "nil node")
	}
	if s == nil {
		return errors.New(errors.DEF_NOT_FOUND, "nil state")
	}

	for _, ref := range n.Context {
		// Skip empty or whitespace-only refs - they'll be caught by ValidateContextRefs
		if strings.TrimSpace(ref) == "" {
			continue
		}

		// Check if it's a valid definition
		if s.GetDefinition(ref) != nil {
			continue // Valid def ref
		}

		// Check if it exists as another type (skip if so)
		if s.GetAssumption(ref) != nil || s.GetExternal(ref) != nil {
			continue // It's not a def ref, skip
		}

		// Not found anywhere - report as missing definition
		return errors.Newf(errors.DEF_NOT_FOUND, "definition not found: %s", ref)
	}

	return nil
}

// ValidateAssnRefs checks that all assumption references in a node exist in state.
// Returns nil if all references are valid, or ASSUMPTION_NOT_FOUND error for missing assumptions.
// Items that exist as definitions or externals are skipped (they're not assn refs).
func ValidateAssnRefs(n *Node, s ContextLookup) error {
	if n == nil {
		return errors.New(errors.ASSUMPTION_NOT_FOUND, "nil node")
	}
	if s == nil {
		return errors.New(errors.ASSUMPTION_NOT_FOUND, "nil state")
	}

	for _, ref := range n.Context {
		// Skip empty or whitespace-only refs - they'll be caught by ValidateContextRefs
		if strings.TrimSpace(ref) == "" {
			continue
		}

		// Check if it's a valid assumption
		if s.GetAssumption(ref) != nil {
			continue // Valid assn ref
		}

		// Check if it exists as another type (skip if so)
		if s.GetDefinition(ref) != nil || s.GetExternal(ref) != nil {
			continue // It's not an assn ref, skip
		}

		// Not found anywhere - report as missing assumption
		return errors.Newf(errors.ASSUMPTION_NOT_FOUND, "assumption not found: %s", ref)
	}

	return nil
}

// ValidateExtRefs checks that all external references in a node exist in state.
// Returns nil if all references are valid, or EXTERNAL_NOT_FOUND error for missing externals.
// Items that exist as definitions or assumptions are skipped (they're not ext refs).
func ValidateExtRefs(n *Node, s ContextLookup) error {
	if n == nil {
		return errors.New(errors.EXTERNAL_NOT_FOUND, "nil node")
	}
	if s == nil {
		return errors.New(errors.EXTERNAL_NOT_FOUND, "nil state")
	}

	for _, ref := range n.Context {
		// Skip empty or whitespace-only refs - they'll be caught by ValidateContextRefs
		if strings.TrimSpace(ref) == "" {
			continue
		}

		// Check if it's a valid external
		if s.GetExternal(ref) != nil {
			continue // Valid ext ref
		}

		// Check if it exists as another type (skip if so)
		if s.GetDefinition(ref) != nil || s.GetAssumption(ref) != nil {
			continue // It's not an ext ref, skip
		}

		// Not found anywhere - report as missing external
		return errors.Newf(errors.EXTERNAL_NOT_FOUND, "external not found: %s", ref)
	}

	return nil
}

// ValidateContextRefs validates all context references in a node.
// It checks that each reference exists as a definition, assumption, or external.
// Returns the first error found. The error type is determined by the position
// of the missing reference relative to valid references of known types.
func ValidateContextRefs(n *Node, s ContextLookup) error {
	if n == nil {
		return errors.New(errors.DEF_NOT_FOUND, "nil node")
	}
	if s == nil {
		return errors.New(errors.DEF_NOT_FOUND, "nil state")
	}

	// First pass: categorize all items and find missing ones
	for i, ref := range n.Context {
		// Check for empty or whitespace-only refs
		if strings.TrimSpace(ref) == "" {
			return errors.Newf(errors.DEF_NOT_FOUND, "empty context reference")
		}

		// Check if it exists in any of the three stores
		isDef := s.GetDefinition(ref) != nil
		isAssn := s.GetAssumption(ref) != nil
		isExt := s.GetExternal(ref) != nil

		if !isDef && !isAssn && !isExt {
			// Item not found - determine error type based on context
			errorCode := determineErrorType(n.Context, i, s)
			switch errorCode {
			case errors.ASSUMPTION_NOT_FOUND:
				return errors.Newf(errors.ASSUMPTION_NOT_FOUND, "assumption not found: %s", ref)
			case errors.EXTERNAL_NOT_FOUND:
				return errors.Newf(errors.EXTERNAL_NOT_FOUND, "external not found: %s", ref)
			default:
				return errors.Newf(errors.DEF_NOT_FOUND, "definition not found: %s", ref)
			}
		}
	}

	return nil
}

// determineErrorType figures out what type a missing context reference should be
// based on its position relative to valid references of known types.
// The expected order is: definitions, then assumptions, then externals.
// The error type is determined by where the missing item falls in this order.
func determineErrorType(context []string, missingIdx int, s ContextLookup) errors.ErrorCode {
	// Find the first and last indices of each type
	firstDefIdx := -1
	lastDefIdx := -1
	firstAssnIdx := -1
	lastAssnIdx := -1
	firstExtIdx := -1

	for i, ref := range context {
		if s.GetDefinition(ref) != nil {
			if firstDefIdx == -1 {
				firstDefIdx = i
			}
			lastDefIdx = i
		}
		if s.GetAssumption(ref) != nil {
			if firstAssnIdx == -1 {
				firstAssnIdx = i
			}
			lastAssnIdx = i
		}
		if s.GetExternal(ref) != nil {
			if firstExtIdx == -1 {
				firstExtIdx = i
			}
		}
	}

	// Determine the error type based on where the missing item is positioned
	// relative to valid items of each type.
	//
	// Expected order: [definitions] [assumptions] [externals]
	//
	// The missing item is:
	// - A definition if it comes before any assumption or external
	// - An assumption if it comes after definitions but before externals
	// - An external if it comes after assumptions

	// Check if we're in the external zone: at or after the first external
	if firstExtIdx >= 0 && missingIdx >= firstExtIdx {
		return errors.EXTERNAL_NOT_FOUND
	}

	// Check if we're in the assumption zone: after the last def, before the first ext
	// This includes the case where there are no assumptions but the item is between defs and exts
	if lastDefIdx >= 0 && missingIdx > lastDefIdx {
		// We're after the last definition
		// If there's an external later, we're in the assumption zone
		if firstExtIdx >= 0 && missingIdx < firstExtIdx {
			return errors.ASSUMPTION_NOT_FOUND
		}
		// If there are no externals, check if there are assumptions
		if firstAssnIdx >= 0 && missingIdx >= firstAssnIdx {
			// We're in or after the assumption section
			if missingIdx > lastAssnIdx {
				return errors.EXTERNAL_NOT_FOUND
			}
			return errors.ASSUMPTION_NOT_FOUND
		}
		// After defs but no assumptions and no exts after -> it's an assumption zone
		if firstExtIdx < 0 && firstAssnIdx < 0 {
			return errors.ASSUMPTION_NOT_FOUND
		}
	}

	// Check if we're in the assumption zone based on assumption positions
	if firstAssnIdx >= 0 && missingIdx >= firstAssnIdx {
		// We're at or after the first assumption
		if missingIdx > lastAssnIdx {
			return errors.EXTERNAL_NOT_FOUND
		}
		return errors.ASSUMPTION_NOT_FOUND
	}

	// Default: we're in the definition zone (before any assumptions or externals)
	return errors.DEF_NOT_FOUND
}
