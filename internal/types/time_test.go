package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParse_ValidTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"basic UTC", "2025-01-11T10:05:00Z"},
		{"midnight", "2025-01-01T00:00:00Z"},
		{"end of day", "2025-12-31T23:59:59Z"},
		{"with fractional seconds", "2025-01-11T10:05:00.123456Z"},
		{"different year", "2020-06-15T14:30:45Z"},
		{"leap year date", "2024-02-29T12:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := ParseTimestamp(tt.input)
			if err != nil {
				t.Errorf("ParseTimestamp(%q) unexpected error: %v", tt.input, err)
			}
			if ts.IsZero() {
				t.Errorf("ParseTimestamp(%q) returned zero timestamp", tt.input)
			}
		})
	}
}

func TestParse_InvalidTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"garbage", "not-a-timestamp"},
		{"missing timezone", "2025-01-11T10:05:00"},
		{"wrong format", "01/11/2025 10:05:00"},
		{"invalid date", "2025-02-30T10:05:00Z"},
		{"invalid month", "2025-13-01T10:05:00Z"},
		{"invalid day", "2025-01-32T10:05:00Z"},
		{"invalid hour", "2025-01-11T25:05:00Z"},
		{"invalid minute", "2025-01-11T10:65:00Z"},
		{"invalid second", "2025-01-11T10:05:65Z"},
		{"partial timestamp", "2025-01-11"},
		{"just time", "10:05:00Z"},
		{"unix timestamp", "1736592300"},
		{"whitespace", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := ParseTimestamp(tt.input)
			if err == nil {
				t.Errorf("ParseTimestamp(%q) expected error, got timestamp: %v", tt.input, ts)
			}
		})
	}
}

func TestTimestamp_String_Roundtrip(t *testing.T) {
	tests := []string{
		"2025-01-11T10:05:00Z",
		"2020-12-25T00:00:00Z",
		"2024-06-15T18:30:45Z",
		"2025-01-01T12:00:00Z",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			ts, err := ParseTimestamp(input)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", input, err)
			}

			output := ts.String()
			// Allow fractional seconds to be truncated
			if output != input && output != input[:19]+"Z" {
				t.Errorf("String() roundtrip failed: input=%q, output=%q", input, output)
			}

			// Parse again to ensure it's still valid
			ts2, err := ParseTimestamp(output)
			if err != nil {
				t.Errorf("ParseTimestamp(output=%q) error: %v", output, err)
			}

			if !ts.Equal(ts2) {
				t.Errorf("Roundtrip not equal: original=%v, reparsed=%v", ts, ts2)
			}
		})
	}
}

func TestNow_NotZero(t *testing.T) {
	ts := Now()
	if ts.IsZero() {
		t.Error("Now() returned zero timestamp")
	}

	// Now should return a time close to actual current time
	goNow := time.Now()
	diff := goNow.Sub(ts.t)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("Now() timestamp %v differs from time.Now() by %v", ts, diff)
	}
}

func TestBefore_True(t *testing.T) {
	tests := []struct {
		name   string
		early  string
		later  string
	}{
		{"same day different hours", "2025-01-11T10:00:00Z", "2025-01-11T11:00:00Z"},
		{"different days", "2025-01-10T23:59:59Z", "2025-01-11T00:00:01Z"},
		{"different years", "2024-12-31T23:59:59Z", "2025-01-01T00:00:00Z"},
		{"one second apart", "2025-01-11T10:05:00Z", "2025-01-11T10:05:01Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			early, err := ParseTimestamp(tt.early)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.early, err)
			}

			later, err := ParseTimestamp(tt.later)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.later, err)
			}

			if !early.Before(later) {
				t.Errorf("%q should be before %q", tt.early, tt.later)
			}
		})
	}
}

func TestBefore_False(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{"same time", "2025-01-11T10:05:00Z", "2025-01-11T10:05:00Z"},
		{"reversed order", "2025-01-11T11:00:00Z", "2025-01-11T10:00:00Z"},
		{"later is earlier", "2025-01-12T00:00:00Z", "2025-01-11T00:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := ParseTimestamp(tt.first)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.first, err)
			}

			second, err := ParseTimestamp(tt.second)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.second, err)
			}

			if first.Before(second) {
				t.Errorf("%q should not be before %q", tt.first, tt.second)
			}
		})
	}
}

func TestAfter_True(t *testing.T) {
	tests := []struct {
		name   string
		later  string
		early  string
	}{
		{"same day different hours", "2025-01-11T11:00:00Z", "2025-01-11T10:00:00Z"},
		{"different days", "2025-01-11T00:00:01Z", "2025-01-10T23:59:59Z"},
		{"different years", "2025-01-01T00:00:00Z", "2024-12-31T23:59:59Z"},
		{"one second apart", "2025-01-11T10:05:01Z", "2025-01-11T10:05:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			later, err := ParseTimestamp(tt.later)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.later, err)
			}

			early, err := ParseTimestamp(tt.early)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.early, err)
			}

			if !later.After(early) {
				t.Errorf("%q should be after %q", tt.later, tt.early)
			}
		})
	}
}

func TestAfter_False(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{"same time", "2025-01-11T10:05:00Z", "2025-01-11T10:05:00Z"},
		{"reversed order", "2025-01-11T10:00:00Z", "2025-01-11T11:00:00Z"},
		{"earlier is later", "2025-01-11T00:00:00Z", "2025-01-12T00:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := ParseTimestamp(tt.first)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.first, err)
			}

			second, err := ParseTimestamp(tt.second)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.second, err)
			}

			if first.After(second) {
				t.Errorf("%q should not be after %q", tt.first, tt.second)
			}
		})
	}
}

func TestEqual_Same(t *testing.T) {
	tests := []string{
		"2025-01-11T10:05:00Z",
		"2020-12-25T00:00:00Z",
		"2024-06-15T18:30:45Z",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			ts1, err := ParseTimestamp(input)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", input, err)
			}

			ts2, err := ParseTimestamp(input)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", input, err)
			}

			if !ts1.Equal(ts2) {
				t.Errorf("Equal() should return true for same timestamp %q", input)
			}
		})
	}
}

func TestEqual_Different(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{"one second diff", "2025-01-11T10:05:00Z", "2025-01-11T10:05:01Z"},
		{"different days", "2025-01-11T00:00:00Z", "2025-01-12T00:00:00Z"},
		{"different years", "2024-01-11T10:05:00Z", "2025-01-11T10:05:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts1, err := ParseTimestamp(tt.first)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.first, err)
			}

			ts2, err := ParseTimestamp(tt.second)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", tt.second, err)
			}

			if ts1.Equal(ts2) {
				t.Errorf("Equal() should return false for %q and %q", tt.first, tt.second)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var ts Timestamp
		if !ts.IsZero() {
			t.Error("Zero-value Timestamp should return true for IsZero()")
		}
	})

	t.Run("parsed timestamp", func(t *testing.T) {
		ts, err := ParseTimestamp("2025-01-11T10:05:00Z")
		if err != nil {
			t.Fatalf("ParseTimestamp error: %v", err)
		}
		if ts.IsZero() {
			t.Error("Parsed timestamp should not return true for IsZero()")
		}
	})

	t.Run("Now timestamp", func(t *testing.T) {
		ts := Now()
		if ts.IsZero() {
			t.Error("Now() timestamp should not return true for IsZero()")
		}
	})
}

func TestJSON_Roundtrip(t *testing.T) {
	tests := []string{
		"2025-01-11T10:05:00Z",
		"2020-12-25T00:00:00Z",
		"2024-06-15T18:30:45Z",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			original, err := ParseTimestamp(input)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error: %v", input, err)
			}

			// Marshal to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal error: %v", err)
			}

			// Unmarshal from JSON
			var restored Timestamp
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal error: %v", err)
			}

			// Compare
			if !original.Equal(restored) {
				t.Errorf("JSON roundtrip failed: original=%v, restored=%v", original, restored)
			}
		})
	}
}

func TestJSON_Format(t *testing.T) {
	ts, err := ParseTimestamp("2025-01-11T10:05:00Z")
	if err != nil {
		t.Fatalf("ParseTimestamp error: %v", err)
	}

	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// JSON should be quoted string in ISO8601 format
	expected := `"2025-01-11T10:05:00Z"`
	if string(data) != expected {
		t.Errorf("JSON format incorrect: got %q, want %q", string(data), expected)
	}
}

func TestJSON_UnmarshalInvalid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty string", `""`},
		{"invalid format", `"not-a-timestamp"`},
		{"not a string", `123`},
		{"null", `null`},
		{"malformed json", `"incomplete`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts Timestamp
			err := json.Unmarshal([]byte(tt.json), &ts)
			// Either error or result in zero timestamp
			if err == nil && !ts.IsZero() {
				t.Errorf("Unmarshal(%q) should fail or return zero, got: %v", tt.json, ts)
			}
		})
	}
}

func TestMonotonicOrdering(t *testing.T) {
	// Create a sequence of timestamps
	timestamps := []string{
		"2025-01-11T10:00:00Z",
		"2025-01-11T10:05:00Z",
		"2025-01-11T10:10:00Z",
		"2025-01-11T10:15:00Z",
		"2025-01-11T10:20:00Z",
	}

	parsed := make([]Timestamp, len(timestamps))
	for i, s := range timestamps {
		ts, err := ParseTimestamp(s)
		if err != nil {
			t.Fatalf("ParseTimestamp(%q) error: %v", s, err)
		}
		parsed[i] = ts
	}

	// Verify monotonic ordering
	for i := 0; i < len(parsed)-1; i++ {
		if !parsed[i].Before(parsed[i+1]) {
			t.Errorf("Timestamp %d (%v) should be before timestamp %d (%v)",
				i, parsed[i], i+1, parsed[i+1])
		}
		if !parsed[i+1].After(parsed[i]) {
			t.Errorf("Timestamp %d (%v) should be after timestamp %d (%v)",
				i+1, parsed[i+1], i, parsed[i])
		}
		if parsed[i].Equal(parsed[i+1]) {
			t.Errorf("Timestamp %d (%v) should not equal timestamp %d (%v)",
				i, parsed[i], i+1, parsed[i+1])
		}
	}
}

func TestTimestamp_Transitivity(t *testing.T) {
	// Test transitivity of Before/After
	ts1, _ := ParseTimestamp("2025-01-11T10:00:00Z")
	ts2, _ := ParseTimestamp("2025-01-11T10:05:00Z")
	ts3, _ := ParseTimestamp("2025-01-11T10:10:00Z")

	if !ts1.Before(ts2) || !ts2.Before(ts3) || !ts1.Before(ts3) {
		t.Error("Before() does not satisfy transitivity: ts1 < ts2 < ts3 => ts1 < ts3")
	}

	if !ts3.After(ts2) || !ts2.After(ts1) || !ts3.After(ts1) {
		t.Error("After() does not satisfy transitivity: ts3 > ts2 > ts1 => ts3 > ts1")
	}
}

func TestTimestamp_ZeroComparison(t *testing.T) {
	var zero Timestamp
	now := Now()

	if !zero.IsZero() {
		t.Error("Zero timestamp should report IsZero() == true")
	}

	if zero.Equal(now) {
		t.Error("Zero timestamp should not equal non-zero timestamp")
	}

	// Zero timestamp behavior in comparisons (implementation-defined, but test it)
	// Most implementations treat zero as earliest possible time
	if !zero.Before(now) {
		t.Log("Note: Zero timestamp is not before Now() - this may be valid depending on implementation")
	}
}
