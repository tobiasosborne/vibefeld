package schema

import (
	"testing"
)

// TestValidateNodeType_AllValid verifies all 5 valid node types pass validation
func TestValidateNodeType_AllValid(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
	}{
		{"claim", "claim"},
		{"local_assume", "local_assume"},
		{"local_discharge", "local_discharge"},
		{"case", "case"},
		{"qed", "qed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeType(tt.nodeType)
			if err != nil {
				t.Errorf("ValidateNodeType(%q) error = %v, want nil", tt.nodeType, err)
			}
		})
	}
}

// TestValidateNodeType_Invalid verifies invalid node types fail validation
func TestValidateNodeType_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
	}{
		{"empty string", ""},
		{"invalid word", "foo"},
		{"uppercase CLAIM", "CLAIM"},
		{"uppercase QED", "QED"},
		{"mixed case Claim", "Claim"},
		{"mixed case Local_Assume", "Local_Assume"},
		{"typo clam", "clam"},
		{"typo qued", "qued"},
		{"whitespace only", "   "},
		{"with spaces", "local assume"},
		{"with hyphen", "local-assume"},
		{"extra underscore", "local__assume"},
		{"prefix", "claim_extra"},
		{"suffix", "extra_claim"},
		{"numbers", "claim123"},
		{"special chars", "claim!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeType(tt.nodeType)
			if err == nil {
				t.Errorf("ValidateNodeType(%q) error = nil, want error", tt.nodeType)
			}
		})
	}
}

// TestGetNodeTypeInfo_Exists verifies metadata is returned for valid node types
func TestGetNodeTypeInfo_Exists(t *testing.T) {
	tests := []struct {
		name        string
		nodeType    NodeType
		wantID      NodeType
		wantOpens   bool
		wantCloses  bool
		wantDescLen bool // should have non-empty description
	}{
		{
			name:        "claim",
			nodeType:    NodeTypeClaim,
			wantID:      NodeTypeClaim,
			wantOpens:   false,
			wantCloses:  false,
			wantDescLen: true,
		},
		{
			name:        "local_assume",
			nodeType:    NodeTypeLocalAssume,
			wantID:      NodeTypeLocalAssume,
			wantOpens:   true,
			wantCloses:  false,
			wantDescLen: true,
		},
		{
			name:        "local_discharge",
			nodeType:    NodeTypeLocalDischarge,
			wantID:      NodeTypeLocalDischarge,
			wantOpens:   false,
			wantCloses:  true,
			wantDescLen: true,
		},
		{
			name:        "case",
			nodeType:    NodeTypeCase,
			wantID:      NodeTypeCase,
			wantOpens:   false,
			wantCloses:  false,
			wantDescLen: true,
		},
		{
			name:        "qed",
			nodeType:    NodeTypeQED,
			wantID:      NodeTypeQED,
			wantOpens:   false,
			wantCloses:  false,
			wantDescLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := GetNodeTypeInfo(tt.nodeType)
			if !ok {
				t.Fatalf("GetNodeTypeInfo(%q) ok = false, want true", tt.nodeType)
			}

			if info.ID != tt.wantID {
				t.Errorf("GetNodeTypeInfo(%q) ID = %q, want %q", tt.nodeType, info.ID, tt.wantID)
			}

			if info.OpensScope != tt.wantOpens {
				t.Errorf("GetNodeTypeInfo(%q) OpensScope = %v, want %v", tt.nodeType, info.OpensScope, tt.wantOpens)
			}

			if info.ClosesScope != tt.wantCloses {
				t.Errorf("GetNodeTypeInfo(%q) ClosesScope = %v, want %v", tt.nodeType, info.ClosesScope, tt.wantCloses)
			}

			if tt.wantDescLen && info.Description == "" {
				t.Errorf("GetNodeTypeInfo(%q) Description is empty, want non-empty", tt.nodeType)
			}
		})
	}
}

// TestGetNodeTypeInfo_NotExists verifies false is returned for invalid node types
func TestGetNodeTypeInfo_NotExists(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
	}{
		{"empty", NodeType("")},
		{"invalid", NodeType("foo")},
		{"uppercase", NodeType("CLAIM")},
		{"typo", NodeType("clam")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetNodeTypeInfo(tt.nodeType)
			if ok {
				t.Errorf("GetNodeTypeInfo(%q) ok = true, want false", tt.nodeType)
			}
		})
	}
}

// TestAllNodeTypes_Count verifies exactly 5 node types are returned
func TestAllNodeTypes_Count(t *testing.T) {
	all := AllNodeTypes()
	if len(all) != 5 {
		t.Errorf("AllNodeTypes() returned %d types, want 5", len(all))
	}
}

// TestAllNodeTypes_Complete verifies all expected node types are present
func TestAllNodeTypes_Complete(t *testing.T) {
	all := AllNodeTypes()

	// Build a map for easy lookup
	found := make(map[NodeType]bool)
	for _, info := range all {
		found[info.ID] = true
	}

	// Check all expected types are present
	expected := []NodeType{
		NodeTypeClaim,
		NodeTypeLocalAssume,
		NodeTypeLocalDischarge,
		NodeTypeCase,
		NodeTypeQED,
	}

	for _, nt := range expected {
		if !found[nt] {
			t.Errorf("AllNodeTypes() missing expected node type: %q", nt)
		}
	}
}

// TestAllNodeTypes_NoDuplicates verifies no duplicate node types in the list
func TestAllNodeTypes_NoDuplicates(t *testing.T) {
	all := AllNodeTypes()

	seen := make(map[NodeType]bool)
	for _, info := range all {
		if seen[info.ID] {
			t.Errorf("AllNodeTypes() contains duplicate node type: %q", info.ID)
		}
		seen[info.ID] = true
	}
}

// TestAllNodeTypes_ValidDescriptions verifies all node types have descriptions
func TestAllNodeTypes_ValidDescriptions(t *testing.T) {
	all := AllNodeTypes()

	for _, info := range all {
		if info.Description == "" {
			t.Errorf("AllNodeTypes() node type %q has empty description", info.ID)
		}
	}
}

// TestOpensScope_LocalAssume verifies local_assume opens scope
func TestOpensScope_LocalAssume(t *testing.T) {
	if !OpensScope(NodeTypeLocalAssume) {
		t.Error("OpensScope(NodeTypeLocalAssume) = false, want true")
	}
}

// TestOpensScope_Others verifies other node types do not open scope
func TestOpensScope_Others(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
	}{
		{"claim", NodeTypeClaim},
		{"local_discharge", NodeTypeLocalDischarge},
		{"case", NodeTypeCase},
		{"qed", NodeTypeQED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if OpensScope(tt.nodeType) {
				t.Errorf("OpensScope(%q) = true, want false", tt.nodeType)
			}
		})
	}
}

// TestOpensScope_Invalid verifies invalid node types do not open scope
func TestOpensScope_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
	}{
		{"empty", NodeType("")},
		{"invalid", NodeType("foo")},
		{"uppercase", NodeType("LOCAL_ASSUME")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if OpensScope(tt.nodeType) {
				t.Errorf("OpensScope(%q) = true, want false", tt.nodeType)
			}
		})
	}
}

// TestClosesScope_LocalDischarge verifies local_discharge closes scope
func TestClosesScope_LocalDischarge(t *testing.T) {
	if !ClosesScope(NodeTypeLocalDischarge) {
		t.Error("ClosesScope(NodeTypeLocalDischarge) = false, want true")
	}
}

// TestClosesScope_Others verifies other node types do not close scope
func TestClosesScope_Others(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
	}{
		{"claim", NodeTypeClaim},
		{"local_assume", NodeTypeLocalAssume},
		{"case", NodeTypeCase},
		{"qed", NodeTypeQED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ClosesScope(tt.nodeType) {
				t.Errorf("ClosesScope(%q) = true, want false", tt.nodeType)
			}
		})
	}
}

// TestClosesScope_Invalid verifies invalid node types do not close scope
func TestClosesScope_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
	}{
		{"empty", NodeType("")},
		{"invalid", NodeType("foo")},
		{"uppercase", NodeType("LOCAL_DISCHARGE")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ClosesScope(tt.nodeType) {
				t.Errorf("ClosesScope(%q) = true, want false", tt.nodeType)
			}
		})
	}
}

// TestNodeTypeConstants verifies constants have expected string values
func TestNodeTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant NodeType
		want     string
	}{
		{"NodeTypeClaim", NodeTypeClaim, "claim"},
		{"NodeTypeLocalAssume", NodeTypeLocalAssume, "local_assume"},
		{"NodeTypeLocalDischarge", NodeTypeLocalDischarge, "local_discharge"},
		{"NodeTypeCase", NodeTypeCase, "case"},
		{"NodeTypeQED", NodeTypeQED, "qed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.want)
			}
		})
	}
}

// TestNodeTypeInfo_ScopeConsistency verifies OpensScope and ClosesScope are never both true
func TestNodeTypeInfo_ScopeConsistency(t *testing.T) {
	all := AllNodeTypes()

	for _, info := range all {
		if info.OpensScope && info.ClosesScope {
			t.Errorf("NodeTypeInfo for %q has both OpensScope and ClosesScope true, should be mutually exclusive",
				info.ID)
		}
	}
}

// TestGetNodeTypeInfo_ConsistentWithHelpers verifies GetNodeTypeInfo matches OpensScope/ClosesScope helpers
func TestGetNodeTypeInfo_ConsistentWithHelpers(t *testing.T) {
	all := AllNodeTypes()

	for _, info := range all {
		t.Run(string(info.ID), func(t *testing.T) {
			opensHelper := OpensScope(info.ID)
			if opensHelper != info.OpensScope {
				t.Errorf("OpensScope(%q) = %v, but GetNodeTypeInfo.OpensScope = %v",
					info.ID, opensHelper, info.OpensScope)
			}

			closesHelper := ClosesScope(info.ID)
			if closesHelper != info.ClosesScope {
				t.Errorf("ClosesScope(%q) = %v, but GetNodeTypeInfo.ClosesScope = %v",
					info.ID, closesHelper, info.ClosesScope)
			}
		})
	}
}
