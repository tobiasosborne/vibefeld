package service

import (
	"github.com/tobias/vibefeld/internal/fs"
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

// InitProofDir initializes a proof directory structure at the given path.
// This is a re-export of fs.InitProofDir to reduce cmd/af imports.
// See fs.InitProofDir for full documentation.
var InitProofDir = fs.InitProofDir
