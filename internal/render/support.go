package render

import (
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
)

// supportStatuses computes support_current once per render. The status command
// computes it once and passes the map down, so rendering a large tree stays
// linear rather than re-walking the DAG per node.
func supportStatuses(s *state.State) map[string]support.SupportStatus {
	if s == nil {
		return nil
	}
	return support.Current(s)
}

// supportMarker renders the D4 marker for a node's support status: "!" when a
// validated or admitted node's recorded verdict is no longer currently
// supported. Non-validated nodes (cause NOT_VALIDATED) carry no marker, since
// they were never "validated but unsupported".
func supportMarker(st support.SupportStatus) string {
	if st.Cause == "" || st.Cause == support.CauseNotValidated {
		return ""
	}
	return " !"
}
