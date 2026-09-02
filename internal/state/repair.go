// Package state provides derived state from replaying ledger events.
package state

import (
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/types"
)

// RepairMetrics holds repair fatigue metrics for a single node.
type RepairMetrics struct {
	NodeID             string `json:"node_id"`
	ChallengeCount     int    `json:"challenge_count"`      // Total challenges raised against this node
	OpenChallengeCount int    `json:"open_challenge_count"` // Currently open challenges
	AmendmentCount     int    `json:"amendment_count"`      // Number of amendments made
	ResolvedCount      int    `json:"resolved_count"`       // Challenges resolved (repair cycles)
	WithdrawnCount     int    `json:"withdrawn_count"`      // Challenges withdrawn
}

// SubtreeRepairMetrics holds aggregated repair metrics for a node and its descendants.
type SubtreeRepairMetrics struct {
	RootID                string `json:"root_id"`
	TotalChallenges       int    `json:"total_challenges"`        // Sum of all challenges in subtree
	TotalOpenChallenges   int    `json:"total_open_challenges"`   // Sum of open challenges in subtree
	TotalAmendments       int    `json:"total_amendments"`        // Sum of amendments in subtree
	TotalResolved         int    `json:"total_resolved"`          // Sum of resolved challenges in subtree
	RefutedDescendants    int    `json:"refuted_descendants"`     // Count of refuted nodes in subtree
	ArchivedDescendants   int    `json:"archived_descendants"`    // Count of archived nodes in subtree
	NodeCount             int    `json:"node_count"`              // Total nodes in subtree
	RepairCycles          int    `json:"repair_cycles"`           // resolved + amendments (proxy for rework)
	HighestNodeChallenges int    `json:"highest_node_challenges"` // Max challenges on any single node
	HighestNodeID         string `json:"highest_node_id"`         // Node with most challenges
}

// GetRepairMetrics returns repair fatigue metrics for a single node.
func (s *State) GetRepairMetrics(nodeID types.NodeID) RepairMetrics {
	m := RepairMetrics{
		NodeID: nodeID.String(),
	}

	challenges := s.GetChallengesForNode(nodeID)
	m.ChallengeCount = len(challenges)
	for _, c := range challenges {
		switch c.Status {
		case ChallengeStatusOpen:
			m.OpenChallengeCount++
		case ChallengeStatusResolved:
			m.ResolvedCount++
		case ChallengeStatusWithdrawn:
			m.WithdrawnCount++
		}
	}

	m.AmendmentCount = len(s.GetAmendmentHistory(nodeID))

	return m
}

// GetSubtreeRepairMetrics returns aggregated repair metrics for a subtree rooted at nodeID.
func (s *State) GetSubtreeRepairMetrics(nodeID types.NodeID) SubtreeRepairMetrics {
	m := SubtreeRepairMetrics{
		RootID: nodeID.String(),
	}

	for _, n := range s.nodes {
		// Include the root and all descendants
		if !n.ID.Equal(nodeID) && !nodeID.IsAncestorOf(n.ID) {
			continue
		}

		m.NodeCount++

		// Count epistemic states
		switch n.EpistemicState {
		case schema.EpistemicRefuted:
			m.RefutedDescendants++
		case schema.EpistemicArchived:
			m.ArchivedDescendants++
		}

		// Aggregate per-node metrics
		nodeChallenges := s.GetChallengesForNode(n.ID)
		m.TotalChallenges += len(nodeChallenges)
		for _, c := range nodeChallenges {
			switch c.Status {
			case ChallengeStatusOpen:
				m.TotalOpenChallenges++
			case ChallengeStatusResolved:
				m.TotalResolved++
			}
		}

		m.TotalAmendments += len(s.GetAmendmentHistory(n.ID))

		// Track node with most challenges
		if len(nodeChallenges) > m.HighestNodeChallenges {
			m.HighestNodeChallenges = len(nodeChallenges)
			m.HighestNodeID = n.ID.String()
		}
	}

	// Repair cycles = resolved challenges + amendments (proxy for rework effort)
	m.RepairCycles = m.TotalResolved + m.TotalAmendments

	return m
}

// FatigueLevel represents the severity of repair fatigue for a subtree.
type FatigueLevel int

const (
	FatigueNone    FatigueLevel = iota // Below warning threshold
	FatigueWarning                     // At or above warning threshold
	FatigueAlarm                       // At or above alarm threshold
)

// Default thresholds for repair fatigue detection.
const (
	DefaultRepairWarningThreshold = 3 // Repair cycles to trigger warning
	DefaultRepairAlarmThreshold   = 5 // Repair cycles to trigger alarm
)

// ClassifyFatigue returns the fatigue level for a given repair cycle count.
func ClassifyFatigue(repairCycles, warningThreshold, alarmThreshold int) FatigueLevel {
	if repairCycles >= alarmThreshold {
		return FatigueAlarm
	}
	if repairCycles >= warningThreshold {
		return FatigueWarning
	}
	return FatigueNone
}

// FatiguedSubtree identifies a subtree exhibiting repair fatigue.
type FatiguedSubtree struct {
	NodeID       string               `json:"node_id"`
	RepairCycles int                  `json:"repair_cycles"`
	Level        FatigueLevel         `json:"level"`
	Metrics      SubtreeRepairMetrics `json:"metrics"`
}

// FindFatiguedSubtrees scans all depth-1 subtrees (direct children of root)
// for repair fatigue. Returns subtrees exceeding the warning threshold,
// sorted by repair cycles descending.
func (s *State) FindFatiguedSubtrees(warningThreshold, alarmThreshold int) []FatiguedSubtree {
	var fatigued []FatiguedSubtree

	for _, n := range s.nodes {
		// Check depth-1 and depth-2 subtrees (main proof branches)
		if n.ID.Depth() > 2 {
			continue
		}

		metrics := s.GetSubtreeRepairMetrics(n.ID)
		level := ClassifyFatigue(metrics.RepairCycles, warningThreshold, alarmThreshold)
		if level == FatigueNone {
			continue
		}

		fatigued = append(fatigued, FatiguedSubtree{
			NodeID:       n.ID.String(),
			RepairCycles: metrics.RepairCycles,
			Level:        level,
			Metrics:      metrics,
		})
	}

	// Sort by repair cycles descending
	for i := 0; i < len(fatigued); i++ {
		for j := i + 1; j < len(fatigued); j++ {
			if fatigued[j].RepairCycles > fatigued[i].RepairCycles {
				fatigued[i], fatigued[j] = fatigued[j], fatigued[i]
			}
		}
	}

	return fatigued
}
