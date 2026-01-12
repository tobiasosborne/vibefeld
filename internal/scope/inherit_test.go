//go:build integration

package scope

import (
	"testing"

	"github.com/tobias/vibefeld/internal/types"
)

func TestGetActiveEntries(t *testing.T) {
	t.Run("nil entries returns empty slice", func(t *testing.T) {
		result := GetActiveEntries(nil)
		if result == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("empty entries returns empty slice", func(t *testing.T) {
		result := GetActiveEntries([]*Entry{})
		if result == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("all active entries are returned", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")

		entries := []*Entry{entry1, entry2}
		result := GetActiveEntries(entries)

		if len(result) != 2 {
			t.Errorf("expected 2 active entries, got %d", len(result))
		}
	})

	t.Run("discharged entries are filtered out", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")
		_ = entry2.Discharge() // Discharge entry2

		entries := []*Entry{entry1, entry2}
		result := GetActiveEntries(entries)

		if len(result) != 1 {
			t.Errorf("expected 1 active entry, got %d", len(result))
		}
		if result[0].Statement != "Assume P" {
			t.Errorf("expected 'Assume P', got '%s'", result[0].Statement)
		}
	})

	t.Run("all discharged returns empty slice", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")
		_ = entry1.Discharge()
		_ = entry2.Discharge()

		entries := []*Entry{entry1, entry2}
		result := GetActiveEntries(entries)

		if len(result) != 0 {
			t.Errorf("expected 0 active entries, got %d", len(result))
		}
	})

	t.Run("nil entries in slice are skipped", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		entry1, _ := NewEntry(nodeID1, "Assume P")

		entries := []*Entry{entry1, nil, nil}
		result := GetActiveEntries(entries)

		if len(result) != 1 {
			t.Errorf("expected 1 active entry, got %d", len(result))
		}
	})
}

func TestInheritScope(t *testing.T) {
	t.Run("empty parent scope returns empty", func(t *testing.T) {
		result := InheritScope([]string{}, nil)
		if result == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("nil parent scope returns empty", func(t *testing.T) {
		result := InheritScope(nil, nil)
		if result == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("all active entries are inherited", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")

		parentScope := []string{"1.1", "1.2"}
		entries := []*Entry{entry1, entry2}

		result := InheritScope(parentScope, entries)

		if len(result) != 2 {
			t.Errorf("expected 2 inherited entries, got %d", len(result))
		}
		// Check both IDs are present
		found1, found2 := false, false
		for _, id := range result {
			if id == "1.1" {
				found1 = true
			}
			if id == "1.2" {
				found2 = true
			}
		}
		if !found1 || !found2 {
			t.Errorf("expected both 1.1 and 1.2 in result, got %v", result)
		}
	})

	t.Run("discharged entries are NOT inherited", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")
		_ = entry2.Discharge() // Discharge entry2

		parentScope := []string{"1.1", "1.2"}
		entries := []*Entry{entry1, entry2}

		result := InheritScope(parentScope, entries)

		if len(result) != 1 {
			t.Errorf("expected 1 inherited entry, got %d", len(result))
		}
		if result[0] != "1.1" {
			t.Errorf("expected '1.1', got '%s'", result[0])
		}
	})

	t.Run("mix of active and discharged entries", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		nodeID2, _ := types.Parse("1.2")
		nodeID3, _ := types.Parse("1.3")

		entry1, _ := NewEntry(nodeID1, "Assume P")
		entry2, _ := NewEntry(nodeID2, "Assume Q")
		entry3, _ := NewEntry(nodeID3, "Assume R")
		_ = entry1.Discharge() // Discharge entry1
		// entry2 active
		_ = entry3.Discharge() // Discharge entry3

		parentScope := []string{"1.1", "1.2", "1.3"}
		entries := []*Entry{entry1, entry2, entry3}

		result := InheritScope(parentScope, entries)

		if len(result) != 1 {
			t.Errorf("expected 1 inherited entry, got %d", len(result))
		}
		if result[0] != "1.2" {
			t.Errorf("expected '1.2', got '%s'", result[0])
		}
	})

	t.Run("nil entries handled gracefully", func(t *testing.T) {
		parentScope := []string{"1.1", "1.2"}

		result := InheritScope(parentScope, nil)

		// With nil entries, we can't determine which are active,
		// so we return empty (safe default)
		if result == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 entries when entries is nil, got %d", len(result))
		}
	})

	t.Run("parent scope entry not in entries map is excluded", func(t *testing.T) {
		nodeID1, _ := types.Parse("1.1")
		entry1, _ := NewEntry(nodeID1, "Assume P")

		// Parent scope references 1.1 and 1.2, but only entry for 1.1 exists
		parentScope := []string{"1.1", "1.2"}
		entries := []*Entry{entry1}

		result := InheritScope(parentScope, entries)

		// Only 1.1 should be inherited since 1.2 has no entry
		if len(result) != 1 {
			t.Errorf("expected 1 inherited entry, got %d", len(result))
		}
		if result[0] != "1.1" {
			t.Errorf("expected '1.1', got '%s'", result[0])
		}
	})
}
