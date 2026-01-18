package render

// CommandExamples provides example usage strings for CLI commands.
// These are used in error messages to help users understand correct syntax.
var CommandExamples = map[string][]string{
	// Core workflow commands
	"af claim": {
		"af claim 1 --owner agent1 --role prover",
		"af claim 1.2 --owner agent1 --role verifier --timeout 30m",
	},
	"af release": {
		"af release 1 --owner agent1",
		"af release 1.2 -o agent1",
	},
	"af refine": {
		"af refine 1 --owner agent1 --statement \"First step\"",
		"af refine 1 -o agent1 -s \"Step\" --type claim --justification modus_ponens",
		"af refine 1 --owner agent1 --children '[{\"statement\":\"Step 1\"},{\"statement\":\"Step 2\"}]'",
	},
	"af accept": {
		"af accept 1",
		"af accept 1.2.3 --dir ./proof",
	},
	"af challenge": {
		"af challenge 1 --reason \"The inference is invalid\"",
		"af challenge 1.2 --reason \"Missing case\" --target completeness",
	},
	"af resolve-challenge": {
		"af resolve-challenge ch-abc123 --resolution \"Addressed by adding case analysis\"",
	},
	"af withdraw-challenge": {
		"af withdraw-challenge ch-abc123",
	},
	"af admit": {
		"af admit 1 --reason \"Accepted as axiom\"",
		"af admit 1.2 -r \"Well-known result\"",
	},
	"af refute": {
		"af refute 1 --reason \"Counterexample: x=0\"",
		"af refute 1.2 -r \"Contradicts axiom 3\"",
	},
	"af archive": {
		"af archive 1",
		"af archive 1.2 --reason \"Abandoned approach\"",
	},

	// Definition commands
	"af def-add": {
		"af def-add group \"A group is a set with a binary operation.\"",
		"af def-add homomorphism --file definition.txt",
	},
	"af request-def": {
		"af request-def group",
		"af request-def vector_space --context \"Need formal definition\"",
	},

	// Query commands
	"af status": {
		"af status",
		"af status 1.2",
		"af status --dir ./proof",
	},
	"af get": {
		"af get 1",
		"af get 1.2.3 --format json",
	},
	"af jobs": {
		"af jobs",
		"af jobs --role prover",
		"af jobs --role verifier",
	},

	// Initialization
	"af init": {
		"af init \"Prove theorem X\"",
		"af init \"Statement\" --dir ./my-proof",
	},

	// Search commands
	"af search": {
		"af search \"convergence\"",
		"af search --state pending",
		"af search --workflow available",
		"af search --state validated --json",
	},
}

// ValidRoles contains the valid role values for commands that accept --role.
var ValidRoles = []string{"prover", "verifier"}

// ValidNodeTypes contains the valid node type values.
var ValidNodeTypes = []string{"claim", "case", "qed", "local_assume", "local_discharge"}

// ValidInferenceTypes contains the valid inference/justification types.
var ValidInferenceTypes = []string{
	"modus_ponens", "modus_tollens", "by_definition",
	"assumption", "local_assume", "local_discharge", "contradiction",
	"universal_instantiation", "existential_instantiation",
	"universal_generalization", "existential_generalization",
}

// ValidChallengeTargets contains the valid challenge target values.
var ValidChallengeTargets = []string{
	"statement", "inference", "context", "dependencies", "scope",
	"gap", "type_error", "domain", "completeness",
}

// ValidFormats contains the valid output format values.
var ValidFormats = []string{"text", "json"}

// ValidEpistemicStates contains the valid epistemic state values.
var ValidEpistemicStates = []string{"pending", "validated", "admitted", "refuted", "archived"}

// ValidWorkflowStates contains the valid workflow state values.
var ValidWorkflowStates = []string{"available", "claimed", "blocked"}

// GetExamples returns example usage for a command, or nil if not found.
func GetExamples(command string) []string {
	return CommandExamples[command]
}
