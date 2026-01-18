package service

import (
	"github.com/tobias/vibefeld/internal/config"
	"github.com/tobias/vibefeld/internal/errors"
	"github.com/tobias/vibefeld/internal/fs"
	"github.com/tobias/vibefeld/internal/fuzzy"
	"github.com/tobias/vibefeld/internal/node"
	"github.com/tobias/vibefeld/internal/schema"
	"github.com/tobias/vibefeld/internal/scope"
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
