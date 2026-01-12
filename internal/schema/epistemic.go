package schema

import "fmt"

// EpistemicState represents the epistemic verification status of a node.
type EpistemicState string

const (
	EpistemicPending   EpistemicState = "pending"
	EpistemicValidated EpistemicState = "validated"
	EpistemicAdmitted  EpistemicState = "admitted"
	EpistemicRefuted   EpistemicState = "refuted"
	EpistemicArchived  EpistemicState = "archived"
)

// EpistemicStateInfo provides metadata about an epistemic state.
type EpistemicStateInfo struct {
	ID              EpistemicState
	Description     string
	IsFinal         bool // true for validated, admitted, refuted, archived
	IntroducesTaint bool // true only for admitted
}

var epistemicStateRegistry = map[EpistemicState]EpistemicStateInfo{
	EpistemicPending: {
		ID:              EpistemicPending,
		Description:     "Awaiting verification",
		IsFinal:         false,
		IntroducesTaint: false,
	},
	EpistemicValidated: {
		ID:              EpistemicValidated,
		Description:     "Verified correct by verifier",
		IsFinal:         true,
		IntroducesTaint: false,
	},
	EpistemicAdmitted: {
		ID:              EpistemicAdmitted,
		Description:     "Accepted without full verification (introduces taint)",
		IsFinal:         true,
		IntroducesTaint: true,
	},
	EpistemicRefuted: {
		ID:              EpistemicRefuted,
		Description:     "Proven incorrect",
		IsFinal:         true,
		IntroducesTaint: false,
	},
	EpistemicArchived: {
		ID:              EpistemicArchived,
		Description:     "No longer relevant (branch abandoned)",
		IsFinal:         true,
		IntroducesTaint: false,
	},
}

// ValidateEpistemicState checks if the given string is a valid epistemic state.
func ValidateEpistemicState(s string) error {
	state := EpistemicState(s)
	if _, ok := epistemicStateRegistry[state]; !ok {
		return fmt.Errorf("invalid epistemic state: %q, must be one of: %s, %s, %s, %s, %s",
			s, EpistemicPending, EpistemicValidated, EpistemicAdmitted,
			EpistemicRefuted, EpistemicArchived)
	}
	return nil
}

// GetEpistemicStateInfo returns metadata for the given epistemic state.
// Returns false if the state does not exist.
func GetEpistemicStateInfo(s EpistemicState) (EpistemicStateInfo, bool) {
	info, ok := epistemicStateRegistry[s]
	return info, ok
}

// AllEpistemicStates returns metadata for all epistemic states.
func AllEpistemicStates() []EpistemicStateInfo {
	states := []EpistemicStateInfo{
		epistemicStateRegistry[EpistemicPending],
		epistemicStateRegistry[EpistemicValidated],
		epistemicStateRegistry[EpistemicAdmitted],
		epistemicStateRegistry[EpistemicRefuted],
		epistemicStateRegistry[EpistemicArchived],
	}
	return states
}

// ValidateEpistemicTransition checks if a transition from one epistemic state
// to another is allowed.
//
// Valid transitions:
// - pending → validated (verifier accepts)
// - pending → admitted (verifier admits without proof)
// - pending → refuted (verifier rejects)
// - pending → archived (proof path abandoned)
// - validated/admitted/refuted/archived are terminal (no transitions out)
func ValidateEpistemicTransition(from, to EpistemicState) error {
	// Validate both states exist
	if err := ValidateEpistemicState(string(from)); err != nil {
		return err
	}
	if err := ValidateEpistemicState(string(to)); err != nil {
		return err
	}

	// Check if from state is terminal
	fromInfo, _ := GetEpistemicStateInfo(from)
	if fromInfo.IsFinal {
		return fmt.Errorf("cannot transition from terminal state %q to %q: state is final", from, to)
	}

	// Only pending can transition
	if from != EpistemicPending {
		return fmt.Errorf("invalid transition from %q to %q: only pending state can transition", from, to)
	}

	// Pending can transition to validated, admitted, refuted, or archived
	validTargets := []EpistemicState{
		EpistemicValidated,
		EpistemicAdmitted,
		EpistemicRefuted,
		EpistemicArchived,
	}
	for _, valid := range validTargets {
		if to == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %q to %q: pending can only transition to: %s, %s, %s, %s",
		from, to, EpistemicValidated, EpistemicAdmitted, EpistemicRefuted, EpistemicArchived)
}

// IsFinal returns true if the given epistemic state is terminal
// (no transitions allowed out of it).
func IsFinal(s EpistemicState) bool {
	info, ok := GetEpistemicStateInfo(s)
	if !ok {
		return false
	}
	return info.IsFinal
}

// IntroducesTaint returns true if the given epistemic state introduces
// taint (epistemic uncertainty).
func IntroducesTaint(s EpistemicState) bool {
	info, ok := GetEpistemicStateInfo(s)
	if !ok {
		return false
	}
	return info.IntroducesTaint
}
