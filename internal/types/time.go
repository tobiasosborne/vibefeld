package types

import (
	"errors"
	"time"
)

// Timestamp represents an ISO8601 timestamp for use in the AF ledger.
// All timestamps are stored in UTC.
type Timestamp struct {
	t time.Time
}

// Now returns a Timestamp representing the current time in UTC.
func Now() Timestamp {
	// TODO: implement
	return Timestamp{}
}

// ParseTimestamp parses an ISO8601 formatted timestamp string.
// Expected format: "2025-01-11T10:05:00Z"
func ParseTimestamp(s string) (Timestamp, error) {
	// TODO: implement
	return Timestamp{}, errors.New("not implemented")
}

// String returns the ISO8601 string representation of the timestamp.
// Format: "2025-01-11T10:05:00Z"
func (ts Timestamp) String() string {
	// TODO: implement
	return ""
}

// Before returns true if ts is before other.
func (ts Timestamp) Before(other Timestamp) bool {
	// TODO: implement
	return false
}

// After returns true if ts is after other.
func (ts Timestamp) After(other Timestamp) bool {
	// TODO: implement
	return false
}

// Equal returns true if ts and other represent the same instant in time.
func (ts Timestamp) Equal(other Timestamp) bool {
	// TODO: implement
	return false
}

// IsZero returns true if ts is the zero value.
func (ts Timestamp) IsZero() bool {
	// TODO: implement
	return false
}

// MarshalJSON implements json.Marshaler.
// Timestamps are serialized as ISO8601 strings.
func (ts Timestamp) MarshalJSON() ([]byte, error) {
	// TODO: implement
	return nil, errors.New("not implemented")
}

// UnmarshalJSON implements json.Unmarshaler.
// Expects an ISO8601 formatted timestamp string.
func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	// TODO: implement
	return errors.New("not implemented")
}
