package audit

// Stable finding codes. These are the machine contract `af audit`, `af health`
// and the amend-deps audit summary surface; message text may change, these may
// not.
const (
	CodeSupportNotCurrent                  = "SUPPORT_NOT_CURRENT"
	CodeHashMismatch                       = "HASH_MISMATCH"
	CodeCycle                              = "CYCLE"
	CodeScopeLeak                          = "SCOPE_LEAK"
	CodeCitesSevered                       = "CITES_SEVERED"
	CodeAmendedNotReverified               = "AMENDED_NOT_REVERIFIED"
	CodeSelfAccept                         = "SELF_ACCEPT"
	CodeValidatedWithOpenBlockingChallenge = "VALIDATED_WITH_OPEN_BLOCKING_CHALLENGE"

	// Historical classes: always reported, never gating.
	CodeAdmitted                        = "ADMITTED"
	CodeArchivedWithOpenChallenge       = "ARCHIVED_WITH_OPEN_CHALLENGE"
	CodeAmendmentsPerNode               = "AMENDMENTS_PER_NODE"
	CodePendingExternalCitedByValidated = "PENDING_EXTERNAL_CITED_BY_VALIDATED"
	CodeUnknownProvenance               = "UNKNOWN_PROVENANCE"
)

// strictCurrentCodes is the set of codes whose current findings fail
// `af audit --strict`. Historical findings never gate, so a code listed here
// still passes when its finding's Status is historical.
var strictCurrentCodes = map[string]bool{
	CodeSupportNotCurrent:                  true,
	CodeHashMismatch:                       true,
	CodeCycle:                              true,
	CodeScopeLeak:                          true,
	CodeCitesSevered:                       true,
	CodeAmendedNotReverified:               true,
	CodeSelfAccept:                         true,
	CodeValidatedWithOpenBlockingChallenge: true,
}

// Severity values.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Status values.
const (
	StatusCurrent    = "current"
	StatusHistorical = "historical"
)

// AllCodes lists every stable code in a deterministic order (strict-current
// codes first, then historical). It is used by docs/tests that enumerate the
// contract.
func AllCodes() []string {
	return []string{
		CodeSupportNotCurrent,
		CodeHashMismatch,
		CodeCycle,
		CodeScopeLeak,
		CodeCitesSevered,
		CodeAmendedNotReverified,
		CodeSelfAccept,
		CodeValidatedWithOpenBlockingChallenge,
		CodeAdmitted,
		CodeArchivedWithOpenChallenge,
		CodeAmendmentsPerNode,
		CodePendingExternalCitedByValidated,
		CodeUnknownProvenance,
	}
}

// remediation maps each code to the action that clears it. Messages are the
// stable human contract for what to do about a finding.
var remediation = map[string]string{
	CodeSupportNotCurrent: "Re-verify the named node against the current dependencies " +
		"(`af request-refinement` then re-accept), or restore the dependency revision.",
	CodeHashMismatch: "A validated node's content moved after acceptance. Re-read the proof, " +
		"then `af unvalidate` and re-accept it (or `af amend-deps`/`af amend` deliberately). " +
		"Hash checks that were never recorded are reported as historical.",
	CodeCycle: "A legacy result-use cycle cannot be current. Remove one edge of the cycle with " +
		"`af amend-deps`, then re-verify the nodes it supported.",
	CodeScopeLeak: "A node cites a result inside a local assumption whose scope does not enclose it. " +
		"Correct the edge with `af amend-deps` (or move the citation inside the scope).",
	CodeCitesSevered: "A node depends on a missing, archived or refuted target. Restore the target, " +
		"or remove the dependency with `af amend-deps` and re-verify.",
	CodeAmendedNotReverified: "A validated node (or a target it relies on) was amended after its " +
		"verdict. Re-verify the named node with `af request-refinement` then re-accept.",
	CodeSelfAccept: "Verifier equals author/proof-author/amender. Re-verify independently, or record " +
		"the self-acceptance deliberately with `af accept --allow-self`.",
	CodeValidatedWithOpenBlockingChallenge: "A validated node has an open critical/major challenge. " +
		"Resolve or withdraw the challenge, then re-verify if the verdict depended on it.",
	CodeAdmitted: "Admitted nodes are cleared with taint; they are reported for audit only. " +
		"Replace the admission with a real verification when the dependency is proven.",
	CodeArchivedWithOpenChallenge: "An archived node (or an active descendant) has an open challenge. " +
		"The abandoned obligation is historical; record a forced archive reason or re-open the work.",
	CodeAmendmentsPerNode: "Repeated amendment is normal on a hard node; listed for audit only. " +
		"Re-verify the node if an amendment postdates its verdict.",
	CodePendingExternalCitedByValidated: "A validated node cites an external reference that is still " +
		"pending. Verify it with `af verify-external`, or do not rely on it.",
	CodeUnknownProvenance: "A validated node has no recorded verifier identity. Re-verify it with " +
		"`af accept --agent <id>` so the verdict carries provenance.",
}
