package render

import (
	"testing"
)

func TestJobListView_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		jl       JobListView
		expected bool
	}{
		{
			name:     "empty job list",
			jl:       JobListView{},
			expected: true,
		},
		{
			name: "prover jobs only",
			jl: JobListView{
				ProverJobs: []NodeView{{ID: "1"}},
			},
			expected: false,
		},
		{
			name: "verifier jobs only",
			jl: JobListView{
				VerifierJobs: []NodeView{{ID: "1"}},
			},
			expected: false,
		},
		{
			name: "both jobs",
			jl: JobListView{
				ProverJobs:   []NodeView{{ID: "1"}},
				VerifierJobs: []NodeView{{ID: "2"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.jl.IsEmpty(); got != tt.expected {
				t.Errorf("JobListView.IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestJobListView_TotalCount(t *testing.T) {
	tests := []struct {
		name     string
		jl       JobListView
		expected int
	}{
		{
			name:     "empty",
			jl:       JobListView{},
			expected: 0,
		},
		{
			name: "prover jobs only",
			jl: JobListView{
				ProverJobs: []NodeView{{ID: "1"}, {ID: "2"}},
			},
			expected: 2,
		},
		{
			name: "verifier jobs only",
			jl: JobListView{
				VerifierJobs: []NodeView{{ID: "1"}, {ID: "2"}, {ID: "3"}},
			},
			expected: 3,
		},
		{
			name: "both jobs",
			jl: JobListView{
				ProverJobs:   []NodeView{{ID: "1"}, {ID: "2"}},
				VerifierJobs: []NodeView{{ID: "3"}},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.jl.TotalCount(); got != tt.expected {
				t.Errorf("JobListView.TotalCount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetNodeViewParentID(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		wantParent   string
		wantHasParent bool
	}{
		{
			name:         "root node",
			nodeID:       "1",
			wantParent:   "",
			wantHasParent: false,
		},
		{
			name:         "first level child",
			nodeID:       "1.2",
			wantParent:   "1",
			wantHasParent: true,
		},
		{
			name:         "second level child",
			nodeID:       "1.2.3",
			wantParent:   "1.2",
			wantHasParent: true,
		},
		{
			name:         "deep child",
			nodeID:       "1.2.3.4.5",
			wantParent:   "1.2.3.4",
			wantHasParent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NodeView{ID: tt.nodeID}
			gotParent, gotHasParent := GetNodeViewParentID(v)
			if gotParent != tt.wantParent {
				t.Errorf("GetNodeViewParentID() parent = %v, want %v", gotParent, tt.wantParent)
			}
			if gotHasParent != tt.wantHasParent {
				t.Errorf("GetNodeViewParentID() hasParent = %v, want %v", gotHasParent, tt.wantHasParent)
			}
		})
	}
}

// TestIsNodeViewRoot is in adapters_test.go to avoid duplication
