package service

import (
	"testing"
)

func TestParseNodeID(t *testing.T) {
	id, err := ParseNodeID("1.2.3")
	if err != nil {
		t.Fatalf("ParseNodeID failed: %v", err)
	}
	if id.String() != "1.2.3" {
		t.Errorf("expected 1.2.3, got %s", id.String())
	}
}

func TestNodeIDAlias(t *testing.T) {
	id, _ := ParseNodeID("1.1")
	var nodeID NodeID = id
	if nodeID.String() != "1.1" {
		t.Errorf("expected 1.1, got %s", nodeID.String())
	}
}

func TestToStringSlice(t *testing.T) {
	id1, _ := ParseNodeID("1.1")
	id2, _ := ParseNodeID("1.2")
	ids := []NodeID{id1, id2}

	strs := ToStringSlice(ids)
	if len(strs) != 2 {
		t.Fatalf("expected 2 strings, got %d", len(strs))
	}
	if strs[0] != "1.1" || strs[1] != "1.2" {
		t.Errorf("unexpected strings: %v", strs)
	}
}
