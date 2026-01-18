package service

import "github.com/tobias/vibefeld/internal/types"

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
