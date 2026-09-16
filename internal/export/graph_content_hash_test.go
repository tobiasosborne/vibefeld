package export

import (
	"encoding/json"
	"testing"
)

// D3: the graph export surfaces the accepted content hash and whether an
// expected hash was checked, per node, additively (omitempty).
func TestExportGraph_IncludesValidatedContentHash(t *testing.T) {
	s := buildFixtureState(t)
	child1 := s.GetNode(mustParseGraphNodeID(t, "1.1"))
	child1.ValidatedContentHash = "hash-accepted"
	child1.ValidatedHashChecked = true

	out, err := ExportGraph(s, "ws", nil)
	if err != nil {
		t.Fatalf("ExportGraph unexpected error: %v", err)
	}

	var doc GraphExport
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	byID := make(map[string]GraphNode)
	for _, n := range doc.Nodes {
		byID[n.ID] = n
	}
	if byID["1.1"].ValidatedContentHash != "hash-accepted" {
		t.Errorf("1.1 validated_content_hash = %q, want %q", byID["1.1"].ValidatedContentHash, "hash-accepted")
	}
	if !byID["1.1"].ValidatedHashChecked {
		t.Error("1.1 validated_hash_checked = false, want true")
	}

	// Omitted, not empty, for a node with no recorded accepted hash.
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	nodes := raw["nodes"].([]interface{})
	for _, rawNode := range nodes {
		n := rawNode.(map[string]interface{})
		if n["id"] == "1.2" {
			if _, present := n["validated_content_hash"]; present {
				t.Error("1.2 must omit validated_content_hash when unset")
			}
			if _, present := n["validated_hash_checked"]; present {
				t.Error("1.2 must omit validated_hash_checked when unset")
			}
		}
	}
}
