package types

import (
	"encoding/json"
	"errors"
	"time"
)

// Timestamp represents an ISO8601 timestamp for use in the AF ledger.
// All timestamps are stored in UTC.
type Timestamp struct {
	t time.Time
}

// Now returns a Timestamp representing the current time in UTC.
// Timestamps are truncated to second precision to ensure JSON roundtrip
// equality (RFC3339 format does not preserve nanoseconds).
func Now() Timestamp {
	return Timestamp{t: time.Now().UTC().Truncate(time.Second)}
}

// FromTime converts a time.Time to a Timestamp.
// The time is converted to UTC. Note that when serialized to JSON,
// precision is truncated to seconds (RFC3339 format).
func FromTime(t time.Time) Timestamp {
	return Timestamp{t: t.UTC()}
}

// ParseTimestamp parses an ISO8601 formatted timestamp string.
// Expected format: "2025-01-11T10:05:00Z"
func ParseTimestamp(s string) (Timestamp, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return Timestamp{}, err
	}
	return Timestamp{t: t.UTC()}, nil
}

// String returns the ISO8601 string representation of the timestamp.
// Format: "2025-01-11T10:05:00Z"
func (ts Timestamp) String() string {
	return ts.t.Format(time.RFC3339)
}

// Before returns true if ts is before other.
func (ts Timestamp) Before(other Timestamp) bool {
	return ts.t.Before(other.t)
}

// After returns true if ts is after other.
func (ts Timestamp) After(other Timestamp) bool {
	return ts.t.After(other.t)
}

// Equal returns true if ts and other represent the same instant in time.
func (ts Timestamp) Equal(other Timestamp) bool {
	return ts.t.Equal(other.t)
}

// IsZero returns true if ts is the zero value.
func (ts Timestamp) IsZero() bool {
	return ts.t.IsZero()
}

// MarshalJSON implements json.Marshaler.
// Timestamps are serialized as ISO8601 strings.
func (ts Timestamp) MarshalJSON() ([]byte, error) {
	// Format as RFC3339 and use json.Marshal for proper escaping
	s := ts.t.Format(time.RFC3339)
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler.
// Expects an ISO8601 formatted timestamp string.
func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	// Handle null case
	if string(data) == "null" {
		return nil
	}

	// Strip quotes from JSON string
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("invalid JSON timestamp: not a string")
	}
	s := string(data[1 : len(data)-1])

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}

	ts.t = t.UTC()
	return nil
}
