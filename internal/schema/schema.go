// Package schema defines the schema configuration for the AF proof framework.
package schema

import (
	"encoding/json"
	"fmt"
)

// Schema holds the configuration for valid types in the proof framework.
type Schema struct {
	Version          string            `json:"version"`
	InferenceTypes   []InferenceType   `json:"inference_types"`
	NodeTypes        []NodeType        `json:"node_types"`
	ChallengeTargets []ChallengeTarget `json:"challenge_targets"`
	WorkflowStates   []WorkflowState   `json:"workflow_states"`
	EpistemicStates  []EpistemicState  `json:"epistemic_states"`

	// Cache maps for O(1) lookups (not serialized)
	inferenceTypeMap   map[InferenceType]struct{}   `json:"-"`
	nodeTypeMap        map[NodeType]struct{}        `json:"-"`
	challengeTargetMap map[ChallengeTarget]struct{} `json:"-"`
	workflowStateMap   map[WorkflowState]struct{}   `json:"-"`
	epistemicStateMap  map[EpistemicState]struct{}  `json:"-"`
}

// DefaultSchema returns a schema with all default values.
func DefaultSchema() *Schema {
	s := &Schema{
		Version: "1.0",
		InferenceTypes: []InferenceType{
			InferenceModusPonens,
			InferenceModusTollens,
			InferenceUniversalInstantiation,
			InferenceExistentialInstantiation,
			InferenceUniversalGeneralization,
			InferenceExistentialGeneralization,
			InferenceByDefinition,
			InferenceAssumption,
			InferenceLocalAssume,
			InferenceLocalDischarge,
			InferenceContradiction,
		},
		NodeTypes: []NodeType{
			NodeTypeClaim,
			NodeTypeLocalAssume,
			NodeTypeLocalDischarge,
			NodeTypeCase,
			NodeTypeQED,
		},
		ChallengeTargets: []ChallengeTarget{
			TargetStatement,
			TargetInference,
			TargetContext,
			TargetDependencies,
			TargetScope,
			TargetGap,
			TargetTypeError,
			TargetDomain,
			TargetCompleteness,
		},
		WorkflowStates: []WorkflowState{
			WorkflowAvailable,
			WorkflowClaimed,
			WorkflowBlocked,
		},
		EpistemicStates: []EpistemicState{
			EpistemicPending,
			EpistemicValidated,
			EpistemicAdmitted,
			EpistemicRefuted,
			EpistemicArchived,
		},
	}
	s.buildCaches()
	return s
}

// LoadSchema loads a schema from JSON data.
// Returns an error if the JSON is invalid or any field contains invalid values.
func LoadSchema(data []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	s.buildCaches()
	return &s, nil
}

// buildCaches initializes the cache maps for O(1) lookups.
func (s *Schema) buildCaches() {
	s.inferenceTypeMap = make(map[InferenceType]struct{}, len(s.InferenceTypes))
	for _, it := range s.InferenceTypes {
		s.inferenceTypeMap[it] = struct{}{}
	}

	s.nodeTypeMap = make(map[NodeType]struct{}, len(s.NodeTypes))
	for _, nt := range s.NodeTypes {
		s.nodeTypeMap[nt] = struct{}{}
	}

	s.challengeTargetMap = make(map[ChallengeTarget]struct{}, len(s.ChallengeTargets))
	for _, ct := range s.ChallengeTargets {
		s.challengeTargetMap[ct] = struct{}{}
	}

	s.workflowStateMap = make(map[WorkflowState]struct{}, len(s.WorkflowStates))
	for _, ws := range s.WorkflowStates {
		s.workflowStateMap[ws] = struct{}{}
	}

	s.epistemicStateMap = make(map[EpistemicState]struct{}, len(s.EpistemicStates))
	for _, es := range s.EpistemicStates {
		s.epistemicStateMap[es] = struct{}{}
	}
}

// Validate validates the schema, ensuring all fields are properly populated
// with valid values.
func (s *Schema) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(s.InferenceTypes) == 0 {
		return fmt.Errorf("inference_types cannot be empty")
	}
	for _, it := range s.InferenceTypes {
		if err := ValidateInference(string(it)); err != nil {
			return fmt.Errorf("invalid inference type: %w", err)
		}
	}

	if len(s.NodeTypes) == 0 {
		return fmt.Errorf("node_types cannot be empty")
	}
	for _, nt := range s.NodeTypes {
		if err := ValidateNodeType(string(nt)); err != nil {
			return fmt.Errorf("invalid node type: %w", err)
		}
	}

	if len(s.ChallengeTargets) == 0 {
		return fmt.Errorf("challenge_targets cannot be empty")
	}
	for _, ct := range s.ChallengeTargets {
		if err := ValidateChallengeTarget(string(ct)); err != nil {
			return fmt.Errorf("invalid challenge target: %w", err)
		}
	}

	if len(s.WorkflowStates) == 0 {
		return fmt.Errorf("workflow_states cannot be empty")
	}
	for _, ws := range s.WorkflowStates {
		if err := ValidateWorkflowState(string(ws)); err != nil {
			return fmt.Errorf("invalid workflow state: %w", err)
		}
	}

	if len(s.EpistemicStates) == 0 {
		return fmt.Errorf("epistemic_states cannot be empty")
	}
	for _, es := range s.EpistemicStates {
		if err := ValidateEpistemicState(string(es)); err != nil {
			return fmt.Errorf("invalid epistemic state: %w", err)
		}
	}

	return nil
}

// ToJSON serializes the schema to JSON.
func (s *Schema) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// HasInferenceType returns true if the schema contains the given inference type.
func (s *Schema) HasInferenceType(t InferenceType) bool {
	_, ok := s.inferenceTypeMap[t]
	return ok
}

// HasNodeType returns true if the schema contains the given node type.
func (s *Schema) HasNodeType(t NodeType) bool {
	_, ok := s.nodeTypeMap[t]
	return ok
}

// HasChallengeTarget returns true if the schema contains the given challenge target.
func (s *Schema) HasChallengeTarget(t ChallengeTarget) bool {
	_, ok := s.challengeTargetMap[t]
	return ok
}

// HasWorkflowState returns true if the schema contains the given workflow state.
func (s *Schema) HasWorkflowState(t WorkflowState) bool {
	_, ok := s.workflowStateMap[t]
	return ok
}

// HasEpistemicState returns true if the schema contains the given epistemic state.
func (s *Schema) HasEpistemicState(t EpistemicState) bool {
	_, ok := s.epistemicStateMap[t]
	return ok
}

// Clone returns a deep copy of the schema.
func (s *Schema) Clone() *Schema {
	clone := &Schema{
		Version:          s.Version,
		InferenceTypes:   make([]InferenceType, len(s.InferenceTypes)),
		NodeTypes:        make([]NodeType, len(s.NodeTypes)),
		ChallengeTargets: make([]ChallengeTarget, len(s.ChallengeTargets)),
		WorkflowStates:   make([]WorkflowState, len(s.WorkflowStates)),
		EpistemicStates:  make([]EpistemicState, len(s.EpistemicStates)),
	}
	copy(clone.InferenceTypes, s.InferenceTypes)
	copy(clone.NodeTypes, s.NodeTypes)
	copy(clone.ChallengeTargets, s.ChallengeTargets)
	copy(clone.WorkflowStates, s.WorkflowStates)
	copy(clone.EpistemicStates, s.EpistemicStates)
	clone.buildCaches()
	return clone
}
