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
// Nanosecond precision is preserved through JSON and string roundtrips.
func Now() Timestamp {
	return Timestamp{t: time.Now().UTC()}
}

// FromTime converts a time.Time to a Timestamp.
// The time is converted to UTC. Nanosecond precision is preserved
// through JSON and string roundtrips.
func FromTime(t time.Time) Timestamp {
	return Timestamp{t: t.UTC()}
}

// ParseTimestamp parses an ISO8601 formatted timestamp string.
// Accepts both RFC3339 ("2025-01-11T10:05:00Z") and RFC3339Nano
// ("2025-01-11T10:05:00.123456789Z") formats, preserving nanosecond precision.
func ParseTimestamp(s string) (Timestamp, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return Timestamp{}, err
	}
	return Timestamp{t: t.UTC()}, nil
}

// String returns the ISO8601 string representation of the timestamp.
// Format: "2025-01-11T10:05:00Z" (without nanoseconds) or
// "2025-01-11T10:05:00.123456789Z" (with nanoseconds).
// Uses RFC3339Nano to preserve nanosecond precision.
func (ts Timestamp) String() string {
	return ts.t.Format(time.RFC3339Nano)
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
// Timestamps are serialized as ISO8601 strings with nanosecond precision.
// When nanoseconds are zero, the output is identical to RFC3339.
func (ts Timestamp) MarshalJSON() ([]byte, error) {
	// Format as RFC3339Nano to preserve nanosecond precision.
	// When nanoseconds are zero, this outputs the same as RFC3339.
	s := ts.t.Format(time.RFC3339Nano)
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts both RFC3339 and RFC3339Nano formatted timestamp strings,
// preserving nanosecond precision when present.
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

	// Parse the timestamp using RFC3339Nano to preserve nanosecond precision.
	// RFC3339Nano also parses timestamps without fractional seconds.
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}

	ts.t = t.UTC()
	return nil
}
