package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitProofDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-initproofdir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proofDir := filepath.Join(tmpDir, "proof")

	// Test that InitProofDir creates the directory structure
	if err := InitProofDir(proofDir); err != nil {
		t.Fatalf("InitProofDir failed: %v", err)
	}

	// Verify the directory was created
	info, err := os.Stat(proofDir)
	if err != nil {
		t.Fatalf("proof dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("proof dir is not a directory")
	}

	// Verify subdirectories were created
	subdirs := []string{"ledger", "nodes", "defs", "assumptions", "externals", "lemmas", "locks"}
	for _, sub := range subdirs {
		subPath := filepath.Join(proofDir, sub)
		info, err := os.Stat(subPath)
		if err != nil {
			t.Errorf("subdir %s not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("subdir %s is not a directory", sub)
		}
	}

	// Verify meta.json was created
	metaPath := filepath.Join(proofDir, "meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("meta.json not created: %v", err)
	}
}

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
