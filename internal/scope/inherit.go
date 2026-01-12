package scope

// GetActiveEntries filters the given entries and returns only those that are
// active (not discharged). Returns an empty slice if entries is nil or empty.
// Nil entries in the slice are skipped.
func GetActiveEntries(entries []*Entry) []*Entry {
	result := make([]*Entry, 0, len(entries))
	for _, entry := range entries {
		if entry != nil && entry.IsActive() {
			result = append(result, entry)
		}
	}
	return result
}

// InheritScope returns the scope entries that a child node should inherit
// from its parent. Only active (non-discharged) entries are inherited.
// The parentScope contains NodeID strings that reference entries.
// Returns an empty slice if parentScope is nil/empty or entries is nil.
func InheritScope(parentScope []string, entries []*Entry) []string {
	result := make([]string, 0, len(parentScope))

	// If entries is nil, we can't determine which are active
	if entries == nil {
		return result
	}

	// Build a map of NodeID string -> Entry for quick lookup
	entryMap := make(map[string]*Entry)
	for _, entry := range entries {
		if entry != nil {
			entryMap[entry.NodeID.String()] = entry
		}
	}

	// Filter parentScope to only include active entries
	for _, nodeIDStr := range parentScope {
		if entry, exists := entryMap[nodeIDStr]; exists && entry.IsActive() {
			result = append(result, nodeIDStr)
		}
	}

	return result
}
