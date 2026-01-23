package service

import (
	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/export"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/fuzzy"
	"github.com/tobias/vibefeld/internal/hooks"
	"github.com/tobias/vibefeld/internal/jobs"
	"github.com/tobias/vibefeld/internal/lemma"
	"github.com/tobias/vibefeld/internal/metrics"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/patterns"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
	"github.com/tobias/vibefeld/internal/shell"
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
	EpistemicPending          = schema.EpistemicPending
	EpistemicValidated        = schema.EpistemicValidated
	EpistemicAdmitted         = schema.EpistemicAdmitted
	EpistemicRefuted          = schema.EpistemicRefuted
	EpistemicArchived         = schema.EpistemicArchived
	EpistemicNeedsRefinement  = schema.EpistemicNeedsRefinement
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

// Re-exported types and functions from internal/patterns to reduce cmd/af import count.
// Consumers should use service.PatternType, service.Pattern, etc. instead of
// importing the patterns package directly.

// PatternType represents the type of mistake pattern.
// Re-export of patterns.PatternType.
type PatternType = patterns.PatternType

// PatternType constants.
const (
	PatternLogicalGap        = patterns.PatternLogicalGap
	PatternScopeViolation    = patterns.PatternScopeViolation
	PatternCircularReasoning = patterns.PatternCircularReasoning
	PatternUndefinedTerm     = patterns.PatternUndefinedTerm
)

// PatternTypeInfo provides metadata about a pattern type.
// Re-export of patterns.PatternTypeInfo.
type PatternTypeInfo = patterns.PatternTypeInfo

// ValidatePatternType validates a pattern type.
// Re-export of patterns.ValidatePatternType.
var ValidatePatternType = patterns.ValidatePatternType

// AllPatternTypes returns all valid pattern types.
// Re-export of patterns.AllPatternTypes.
var AllPatternTypes = patterns.AllPatternTypes

// GetPatternTypeInfo returns metadata for a pattern type.
// Re-export of patterns.GetPatternTypeInfo.
var GetPatternTypeInfo = patterns.GetPatternTypeInfo

// Pattern represents a detected mistake pattern from resolved challenges.
// Re-export of patterns.Pattern.
type Pattern = patterns.Pattern

// NewPattern creates a new Pattern instance.
// Re-export of patterns.NewPattern.
var NewPattern = patterns.NewPattern

// PatternLibrary stores collected patterns from resolved challenges.
// Re-export of patterns.PatternLibrary.
type PatternLibrary = patterns.PatternLibrary

// NewPatternLibrary creates a new empty pattern library.
// Re-export of patterns.NewPatternLibrary.
var NewPatternLibrary = patterns.NewPatternLibrary

// PatternStats contains statistics about patterns in the library.
// Re-export of patterns.PatternStats.
type PatternStats = patterns.PatternStats

// LoadPatternLibrary loads the pattern library from the proof directory.
// Re-export of patterns.LoadPatternLibrary.
var LoadPatternLibrary = patterns.LoadPatternLibrary

// PatternAnalyzer analyzes challenges and nodes to detect patterns.
// Re-export of patterns.Analyzer.
type PatternAnalyzer = patterns.Analyzer

// NewPatternAnalyzer creates a new analyzer with the given pattern library.
// Re-export of patterns.NewAnalyzer.
var NewPatternAnalyzer = patterns.NewAnalyzer

// PotentialIssue represents a potential issue detected in a node.
// Re-export of patterns.PotentialIssue.
type PotentialIssue = patterns.PotentialIssue

// Challenge status values for state.Challenge comparisons.
// Re-export of state.ChallengeStatus* constants.
const (
	ChallengeStatusOpen       = state.ChallengeStatusOpen
	ChallengeStatusResolved   = state.ChallengeStatusResolved
	ChallengeStatusWithdrawn  = state.ChallengeStatusWithdrawn
	ChallengeStatusSuperseded = state.ChallengeStatusSuperseded
)

// Re-exported types and functions from internal/shell to reduce cmd/af import count.
// Consumers should use service.Shell, service.NewShell, etc. instead of
// importing the shell package directly.

// Shell represents an interactive shell session.
// Re-export of shell.Shell.
type Shell = shell.Shell

// ShellOption is a functional option for configuring a Shell.
// Re-export of shell.Option.
type ShellOption = shell.Option

// NewShell creates a new Shell with the given options.
// Re-export of shell.New.
var NewShell = shell.New

// ShellWithPrompt sets a custom prompt for the shell.
// Re-export of shell.WithPrompt.
var ShellWithPrompt = shell.WithPrompt

// ShellWithInput sets the input reader for the shell.
// Re-export of shell.WithInput.
var ShellWithInput = shell.WithInput

// ShellWithOutput sets the output writer for the shell.
// Re-export of shell.WithOutput.
var ShellWithOutput = shell.WithOutput

// ShellWithExecutor sets the command executor function.
// Re-export of shell.WithExecutor.
var ShellWithExecutor = shell.WithExecutor

// ErrShellExit is returned when the user requests to exit the shell.
// Re-export of shell.ErrExit.
var ErrShellExit = shell.ErrExit

// Re-exported types and functions from internal/hooks to reduce cmd/af import count.
// Consumers should use service.Hook, service.HookType, etc. instead of
// importing the hooks package directly.

// HookType represents the type of hook.
// Re-export of hooks.HookType.
type HookType = hooks.HookType

// HookType constants.
const (
	HookTypeWebhook = hooks.HookTypeWebhook
	HookTypeCommand = hooks.HookTypeCommand
)

// HookEventType represents the type of event that can trigger hooks.
// Re-export of hooks.EventType.
type HookEventType = hooks.EventType

// HookEventType constants.
const (
	HookEventNodeCreated       = hooks.EventNodeCreated
	HookEventNodeValidated     = hooks.EventNodeValidated
	HookEventChallengeRaised   = hooks.EventChallengeRaised
	HookEventChallengeResolved = hooks.EventChallengeResolved
)

// Hook represents a configured hook.
// Re-export of hooks.Hook.
type Hook = hooks.Hook

// HookConfig holds the hooks configuration for a proof directory.
// Re-export of hooks.Config.
type HookConfig = hooks.Config

// HookManager manages hook execution for a proof directory.
// Re-export of hooks.Manager.
type HookManager = hooks.Manager

// ValidateHookType validates a hook type.
// Re-export of hooks.ValidateHookType.
var ValidateHookType = hooks.ValidateHookType

// ValidateHookEventType validates an event type.
// Re-export of hooks.ValidateEventType.
var ValidateHookEventType = hooks.ValidateEventType

// GenerateHookID generates a unique hook ID.
// Re-export of hooks.GenerateHookID.
var GenerateHookID = hooks.GenerateHookID

// LoadHookConfig loads the hooks config from the proof directory.
// Re-export of hooks.LoadConfig.
var LoadHookConfig = hooks.LoadConfig

// NewHookManager creates a new hook manager.
// Re-export of hooks.NewManager.
var NewHookManager = hooks.NewManager

// NewHookConfig creates a new default hooks config.
// Re-export of hooks.NewConfig.
var NewHookConfig = hooks.NewConfig

// Re-exported types and functions from internal/jobs to reduce cmd/af import count.
// Consumers should use service.JobResult, service.FindJobs, etc. instead of
// importing the jobs package directly.

// JobResult contains the results of finding all jobs.
// Re-export of jobs.JobResult.
type JobResult = jobs.JobResult

// FindJobs finds all prover and verifier jobs from the given nodes.
// Re-export of jobs.FindJobs.
var FindJobs = jobs.FindJobs

// FindProverJobs returns nodes available for provers to work on.
// Re-export of jobs.FindProverJobs.
var FindProverJobs = jobs.FindProverJobs

// FindVerifierJobs returns nodes ready for verifier review.
// Re-export of jobs.FindVerifierJobs.
var FindVerifierJobs = jobs.FindVerifierJobs

// Re-exported functions from internal/cli to reduce cmd/af import count.
// Consumers should use service.MustString, service.MustBool, etc. instead of
// importing the cli package directly.

// MustString retrieves a string flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
// Re-export of cli.MustString.
func MustString(cmd *cobra.Command, name string) string {
	return cli.MustString(cmd, name)
}

// MustBool retrieves a boolean flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
// Re-export of cli.MustBool.
func MustBool(cmd *cobra.Command, name string) bool {
	return cli.MustBool(cmd, name)
}

// MustInt retrieves an integer flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
// Re-export of cli.MustInt.
func MustInt(cmd *cobra.Command, name string) int {
	return cli.MustInt(cmd, name)
}

// MustStringSlice retrieves a string slice flag value from a cobra command.
// Panics if the flag was not registered, which is a programming error.
// Re-export of cli.MustStringSlice.
func MustStringSlice(cmd *cobra.Command, name string) []string {
	return cli.MustStringSlice(cmd, name)
}

// Re-exported types and functions from internal/fs and internal/node for pending def
// filesystem operations. Used by test code to set up test fixtures.

// PendingDef represents a pending definition request.
// Re-export of node.PendingDef.
type PendingDef = node.PendingDef

// PendingDefStatus constants.
// Re-export of node.PendingDefStatus* constants.
const (
	PendingDefStatusPending   = node.PendingDefStatusPending
	PendingDefStatusResolved  = node.PendingDefStatusResolved
	PendingDefStatusCancelled = node.PendingDefStatusCancelled
)

// NewPendingDefWithValidation creates a new pending definition with validation.
// Re-export of node.NewPendingDefWithValidation.
var NewPendingDefWithValidation = node.NewPendingDefWithValidation

// WritePendingDef writes a pending definition to the filesystem.
// Re-export of fs.WritePendingDef for test fixture setup.
var WritePendingDef = fs.WritePendingDef

// ReadPendingDef reads a pending definition from the filesystem.
// Re-export of fs.ReadPendingDef for test verification.
var ReadPendingDef = fs.ReadPendingDef

// ListPendingDefs lists all pending definitions in a proof directory.
// Re-export of fs.ListPendingDefs for test verification.
var ListPendingDefs = fs.ListPendingDefs

// Re-exported types and functions from internal/state to reduce cmd/af import count.
// Consumers should use service.State, service.Challenge, etc. instead of
// importing the state package directly.

// State represents the current derived state of a proof.
// Re-export of state.State.
type State = state.State

// Challenge represents a challenge tracked in the state.
// Re-export of state.Challenge.
type Challenge = state.Challenge

// Amendment represents a single amendment to a node's statement.
// Re-export of state.Amendment.
type Amendment = state.Amendment

// NewState creates a new empty State with all maps initialized.
// Re-export of state.NewState.
var NewState = state.NewState

// Replay replays all events from the ledger to rebuild the state.
// Re-export of state.Replay.
var Replay = state.Replay

// ReplayWithVerify replays events and verifies state consistency.
// Re-export of state.ReplayWithVerify.
var ReplayWithVerify = state.ReplayWithVerify
