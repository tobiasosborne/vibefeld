package service

import (
	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/export"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/fuzzy"
	"github.com/tobias/vibefeld/internal/lemma"
	"github.com/tobias/vibefeld/internal/metrics"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/state"
	"github.com/tobias/vibefeld/internal/strategy"
	"github.com/tobias/vibefeld/internal/templates"
	"github.com/tobias/vibefeld/internal/types"
)

// Re-exported types from internal/types to reduce cmd/af import count.
// Consumers should use service.NodeID and service.ParseNodeID instead of
// importing the types package directly.

// NodeID is an alias for types.NodeID.
type NodeID = types.NodeID

// ParseNodeID parses a string to a NodeID.
// Re-export of types.Parse.
var ParseNodeID = types.Parse

// ToStringSlice converts a slice of NodeIDs to strings.
// Re-export of types.ToStringSlice.
var ToStringSlice = types.ToStringSlice

// Timestamp represents an ISO8601 timestamp for use in the AF ledger.
// Re-export of types.Timestamp.
type Timestamp = types.Timestamp

// Now returns a Timestamp representing the current time in UTC.
// Re-export of types.Now.
var Now = types.Now

// FromTime converts a time.Time to a Timestamp.
// Re-export of types.FromTime.
var FromTime = types.FromTime

// ParseTimestamp parses an ISO8601 formatted timestamp string.
// Re-export of types.ParseTimestamp.
var ParseTimestamp = types.ParseTimestamp

// InitProofDir initializes a proof directory structure at the given path.
// This is a re-export of fs.InitProofDir to reduce cmd/af imports.
// See fs.InitProofDir for full documentation.
var InitProofDir = fs.InitProofDir

// Re-exported types from internal/schema to reduce cmd/af import count.
// Consumers should use service.NodeType, service.InferenceType, etc. instead of
// importing the schema package directly.

// NodeType is an alias for schema.NodeType.
type NodeType = schema.NodeType

// NodeType constants.
const (
	NodeTypeClaim          = schema.NodeTypeClaim
	NodeTypeLocalAssume    = schema.NodeTypeLocalAssume
	NodeTypeLocalDischarge = schema.NodeTypeLocalDischarge
	NodeTypeCase           = schema.NodeTypeCase
	NodeTypeQED            = schema.NodeTypeQED
)

// InferenceType is an alias for schema.InferenceType.
type InferenceType = schema.InferenceType

// InferenceType constants.
const (
	InferenceModusPonens               = schema.InferenceModusPonens
	InferenceModusTollens              = schema.InferenceModusTollens
	InferenceUniversalInstantiation    = schema.InferenceUniversalInstantiation
	InferenceExistentialInstantiation  = schema.InferenceExistentialInstantiation
	InferenceUniversalGeneralization   = schema.InferenceUniversalGeneralization
	InferenceExistentialGeneralization = schema.InferenceExistentialGeneralization
	InferenceByDefinition              = schema.InferenceByDefinition
	InferenceAssumption                = schema.InferenceAssumption
	InferenceLocalAssume               = schema.InferenceLocalAssume
	InferenceLocalDischarge            = schema.InferenceLocalDischarge
	InferenceContradiction             = schema.InferenceContradiction
)

// EpistemicState is an alias for schema.EpistemicState.
type EpistemicState = schema.EpistemicState

// EpistemicState constants.
const (
	EpistemicPending   = schema.EpistemicPending
	EpistemicValidated = schema.EpistemicValidated
	EpistemicAdmitted  = schema.EpistemicAdmitted
	EpistemicRefuted   = schema.EpistemicRefuted
	EpistemicArchived  = schema.EpistemicArchived
)

// WorkflowState is an alias for schema.WorkflowState.
type WorkflowState = schema.WorkflowState

// WorkflowState constants.
const (
	WorkflowAvailable = schema.WorkflowAvailable
	WorkflowClaimed   = schema.WorkflowClaimed
	WorkflowBlocked   = schema.WorkflowBlocked
)

// ChallengeTarget is an alias for schema.ChallengeTarget.
type ChallengeTarget = schema.ChallengeTarget

// ChallengeTarget constants.
const (
	TargetStatement    = schema.TargetStatement
	TargetInference    = schema.TargetInference
	TargetContext      = schema.TargetContext
	TargetDependencies = schema.TargetDependencies
	TargetScope        = schema.TargetScope
	TargetGap          = schema.TargetGap
	TargetTypeError    = schema.TargetTypeError
	TargetDomain       = schema.TargetDomain
	TargetCompleteness = schema.TargetCompleteness
)

// ChallengeSeverity is an alias for schema.ChallengeSeverity.
type ChallengeSeverity = schema.ChallengeSeverity

// ChallengeSeverity constants.
const (
	SeverityCritical = schema.SeverityCritical
	SeverityMajor    = schema.SeverityMajor
	SeverityMinor    = schema.SeverityMinor
	SeverityNote     = schema.SeverityNote
)

// Schema validation functions re-exported from internal/schema.
var (
	ValidateNodeType          = schema.ValidateNodeType
	ValidateInference         = schema.ValidateInference
	ValidateEpistemicState    = schema.ValidateEpistemicState
	ValidateWorkflowState     = schema.ValidateWorkflowState
	ValidateChallengeTarget   = schema.ValidateChallengeTarget
	ValidateChallengeTargets  = schema.ValidateChallengeTargets
	ParseChallengeTargets     = schema.ParseChallengeTargets
	SuggestInference          = schema.SuggestInference
	AllInferences             = schema.AllInferences
	AllNodeTypes              = schema.AllNodeTypes
	AllEpistemicStates        = schema.AllEpistemicStates
	AllWorkflowStates         = schema.AllWorkflowStates
	AllChallengeTargets       = schema.AllChallengeTargets
	GetInferenceInfo          = schema.GetInferenceInfo
	GetNodeTypeInfo           = schema.GetNodeTypeInfo
	GetEpistemicStateInfo     = schema.GetEpistemicStateInfo
	GetWorkflowStateInfo      = schema.GetWorkflowStateInfo
	GetChallengeTargetInfo    = schema.GetChallengeTargetInfo
	OpensScope                = schema.OpensScope
	ClosesScope               = schema.ClosesScope
	IsFinal                   = schema.IsFinal
	IntroducesTaint           = schema.IntroducesTaint
	ValidateEpistemicTransition = schema.ValidateEpistemicTransition
	ValidateWorkflowTransition  = schema.ValidateWorkflowTransition
	CanClaim                    = schema.CanClaim
	ValidateChallengeSeverity   = schema.ValidateChallengeSeverity
	SeverityBlocksAcceptance    = schema.SeverityBlocksAcceptance
	GetChallengeSeverityInfo    = schema.GetChallengeSeverityInfo
	AllChallengeSeverities      = schema.AllChallengeSeverities
	DefaultChallengeSeverity    = schema.DefaultChallengeSeverity
)

// Info types re-exported from internal/schema.
type (
	InferenceInfo         = schema.InferenceInfo
	NodeTypeInfo          = schema.NodeTypeInfo
	EpistemicStateInfo    = schema.EpistemicStateInfo
	WorkflowStateInfo     = schema.WorkflowStateInfo
	ChallengeTargetInfo   = schema.ChallengeTargetInfo
	ChallengeSeverityInfo = schema.ChallengeSeverityInfo
)

// Re-exported types from internal/node to reduce cmd/af import count.

// TaintState represents the taint status of a node.
// Re-export of node.TaintState.
type TaintState = node.TaintState

// TaintState constants.
const (
	TaintClean        = node.TaintClean
	TaintSelfAdmitted = node.TaintSelfAdmitted
	TaintTainted      = node.TaintTainted
	TaintUnresolved   = node.TaintUnresolved
)

// Re-exported functions from internal/errors to reduce cmd/af import count.
// Consumers should use service.SanitizeError and service.ExitCode instead of
// importing the errors package directly.

// SanitizeError wraps an error with sanitized file paths in its message.
// Re-export of errors.SanitizeError.
var SanitizeError = errors.SanitizeError

// ExitCode returns the appropriate exit code for an error.
// Re-export of errors.ExitCode.
var ExitCode = errors.ExitCode

// Re-exported constants from internal/config to reduce cmd/af import count.
// Consumers should use service.DefaultClaimTimeout instead of
// importing the config package directly.

// DefaultClaimTimeout is the default duration for claim timeouts in CLI commands.
// Re-export of config.DefaultClaimTimeout.
const DefaultClaimTimeout = config.DefaultClaimTimeout

// Re-exported types from internal/scope to reduce cmd/af import count.
// Consumers should use service.ScopeEntry and service.ScopeInfo instead of
// importing the scope package directly.

// ScopeEntry represents a scope entry for a local assumption.
// Re-export of scope.Entry.
type ScopeEntry = scope.Entry

// ScopeInfo contains information about a node's scope context.
// Re-export of scope.ScopeInfo.
type ScopeInfo = scope.ScopeInfo

// Re-exported types and functions from internal/fuzzy to reduce cmd/af import count.
// Consumers should use service.MatchResult, service.SuggestCommand, and
// service.SuggestFlag instead of importing the fuzzy package directly.

// MatchResult contains the result of fuzzy matching.
// Re-export of fuzzy.MatchResult.
type MatchResult = fuzzy.MatchResult

// SuggestCommand finds the best match for input among command names.
// Re-export of fuzzy.SuggestCommand.
var SuggestCommand = fuzzy.SuggestCommand

// SuggestFlag suggests similar flags for a mistyped flag name.
// Re-export of fuzzy.SuggestFlag.
var SuggestFlag = fuzzy.SuggestFlag

// Re-exported functions from internal/lemma to reduce cmd/af import count.
// Consumers should use service.ValidateDefCitations instead of
// importing the lemma package directly.

// ValidateDefCitations validates that all def:NAME citations in the statement
// reference definitions that exist in the current state.
// Re-export of lemma.ValidateDefCitations.
var ValidateDefCitations = lemma.ValidateDefCitations

// Re-exported functions from internal/export to reduce cmd/af import count.
// Consumers should use service.ValidateExportFormat and service.ExportProof
// instead of importing the export package directly.

// ValidateExportFormat checks if the given format string is valid.
// Valid formats: markdown, md, latex, tex (case-insensitive).
// Re-export of export.ValidateFormat.
var ValidateExportFormat = export.ValidateFormat

// ExportProof exports the proof state to the specified format.
// Returns an error if the format is invalid.
// Re-export of export.Export.
func ExportProof(s *state.State, format string) (string, error) {
	return export.Export(s, format)
}

// Re-exported types and functions from internal/metrics to reduce cmd/af import count.
// Consumers should use service.QualityReport, service.OverallQuality, and
// service.SubtreeQuality instead of importing the metrics package directly.

// QualityReport contains comprehensive quality metrics for a proof or subtree.
// Re-export of metrics.QualityReport.
type QualityReport = metrics.QualityReport

// OverallQuality computes comprehensive quality metrics for the entire proof.
// Re-export of metrics.OverallQuality.
func OverallQuality(s *state.State) *QualityReport {
	return metrics.OverallQuality(s)
}

// SubtreeQuality computes quality metrics for a specific subtree.
// Returns an empty report if the root node doesn't exist.
// Re-export of metrics.SubtreeQuality.
func SubtreeQuality(s *state.State, rootID NodeID) *QualityReport {
	return metrics.SubtreeQuality(s, rootID)
}

// Re-exported types and functions from internal/templates to reduce cmd/af import count.
// Consumers should use service.Template, service.GetTemplate, etc. instead of
// importing the templates package directly.
// Note: templates.ChildSpec is NOT re-exported because service.ChildSpec already exists
// for a different purpose (bulk refine operations). The templates ChildSpec is
// accessed indirectly through Template.Children.

// Template defines a proof structure template.
// Re-export of templates.Template.
type Template = templates.Template

// GetTemplate retrieves a template by name.
// Re-export of templates.Get.
var GetTemplate = templates.Get

// ListTemplates returns all available templates in a consistent order.
// Re-export of templates.List.
var ListTemplates = templates.List

// TemplateNames returns all available template names in a consistent order.
// Re-export of templates.Names.
var TemplateNames = templates.Names

// Re-exported types and functions from internal/strategy to reduce cmd/af import count.
// Consumers should use service.Strategy, service.StrategyStep, etc. instead of
// importing the strategy package directly.

// Strategy represents a proof strategy with its structure and guidance.
// Re-export of strategy.Strategy.
type Strategy = strategy.Strategy

// StrategyStep represents a single step in a proof strategy.
// Re-export of strategy.Step.
type StrategyStep = strategy.Step

// StrategySuggestion represents a strategy suggestion with reasoning.
// Re-export of strategy.Suggestion.
type StrategySuggestion = strategy.Suggestion

// AllStrategies returns all available strategies in a consistent order.
// Re-export of strategy.All.
var AllStrategies = strategy.All

// GetStrategy retrieves a strategy by name.
// Re-export of strategy.Get.
var GetStrategy = strategy.Get

// StrategyNames returns all available strategy names in a consistent order.
// Re-export of strategy.Names.
var StrategyNames = strategy.Names

// SuggestStrategies analyzes a conjecture and suggests appropriate proof strategies.
// Re-export of strategy.Suggest.
var SuggestStrategies = strategy.Suggest
