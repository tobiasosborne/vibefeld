package scope

// GetActiveEntries filters the given entries and returns only those that are
// active (not discharged). Returns an empty slice if entries is nil or empty.
// Nil entries in the slice are skipped.
func GetActiveEntries(entries []*Entry) []*Entry {
	// TODO: implement
	return []*Entry{}
}

// InheritScope returns the scope entries that a child node should inherit
// from its parent. Only active (non-discharged) entries are inherited.
// The parentScope contains NodeID strings that reference entries.
// Returns an empty slice if parentScope is nil/empty or entries is nil.
func InheritScope(parentScope []string, entries []*Entry) []string {
	// TODO: implement
	return []string{}
}
