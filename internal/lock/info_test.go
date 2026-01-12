//go:build integration
// +build integration

// These tests define expected behavior for GetLockInfo and LockInfo.
// Run with: go test -tags=integration ./internal/lock/...

package lock_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// TestGetLockInfo_Valid verifies GetLockInfo returns correct info from a valid lock
func TestGetLockInfo_Valid(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node", "1", "agent-001", 5 * time.Minute},
		{"child node", "1.1", "agent-002", 10 * time.Second},
		{"deep node", "1.2.3.4", "prover-alpha", 1 * time.Hour},
		{"short timeout", "1.1.1", "verifier-beta", 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			info, err := lock.GetLockInfo(lk)
			if err != nil {
				t.Fatalf("GetLockInfo() unexpected error: %v", err)
			}

			// Verify NodeID
			if info.NodeID != tt.nodeID {
				t.Errorf("NodeID = %q, want %q", info.NodeID, tt.nodeID)
			}

			// Verify Owner
			if info.Owner != tt.owner {
				t.Errorf("Owner = %q, want %q", info.Owner, tt.owner)
			}

			// Verify Acquired is set (not zero)
			if info.Acquired.IsZero() {
				t.Error("Acquired is zero, want non-zero time")
			}

			// Verify Expires is after Acquired
			if !info.Expires.After(info.Acquired) {
				t.Errorf("Expires = %v should be after Acquired = %v",
					info.Expires, info.Acquired)
			}

			// Verify IsExpired matches lock state
			if info.IsExpired != lk.IsExpired() {
				t.Errorf("IsExpired = %v, want %v", info.IsExpired, lk.IsExpired())
			}

			// Verify Remaining is positive for non-expired lock
			if !info.IsExpired && info.Remaining <= 0 {
				t.Errorf("Remaining = %v, want positive for non-expired lock", info.Remaining)
			}
		})
	}
}

// TestGetLockInfo_NilLock verifies GetLockInfo handles nil lock
func TestGetLockInfo_NilLock(t *testing.T) {
	info, err := lock.GetLockInfo(nil)

	if err == nil {
		t.Error("GetLockInfo(nil) expected error, got nil")
	}

	if info != nil {
		t.Errorf("GetLockInfo(nil) returned non-nil info: %+v", info)
	}
}

// TestLockInfo_RemainingTime verifies remaining time calculation
func TestLockInfo_RemainingTime(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		sleepDuration  time.Duration
		expectPositive bool
	}{
		{"fresh lock with long timeout", 1 * time.Hour, 0, true},
		{"fresh lock with short timeout", 1 * time.Minute, 0, true},
		{"after some time elapsed", 5 * time.Second, 100 * time.Millisecond, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewLock(nodeID, "agent-001", tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if tt.sleepDuration > 0 {
				time.Sleep(tt.sleepDuration)
			}

			info, err := lock.GetLockInfo(lk)
			if err != nil {
				t.Fatalf("GetLockInfo() unexpected error: %v", err)
			}

			if tt.expectPositive && info.Remaining <= 0 {
				t.Errorf("Remaining = %v, want positive", info.Remaining)
			}

			// Remaining should be less than or equal to original timeout
			if info.Remaining > tt.timeout {
				t.Errorf("Remaining = %v, should be <= timeout %v", info.Remaining, tt.timeout)
			}

			// Remaining should have decreased if we slept
			if tt.sleepDuration > 0 {
				maxExpected := tt.timeout - tt.sleepDuration + 50*time.Millisecond // allow some slack
				if info.Remaining > maxExpected {
					t.Errorf("Remaining = %v, should be <= %v after sleeping %v",
						info.Remaining, maxExpected, tt.sleepDuration)
				}
			}
		})
	}
}

// TestLockInfo_RemainingTime_Expired verifies remaining time for expired lock
func TestLockInfo_RemainingTime_Expired(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	info, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() unexpected error: %v", err)
	}

	// Remaining should be zero or negative for expired lock
	if info.Remaining > 0 {
		t.Errorf("Remaining = %v, want <= 0 for expired lock", info.Remaining)
	}

	if !info.IsExpired {
		t.Error("IsExpired = false, want true for expired lock")
	}
}

// TestLockInfo_Stringer verifies string representation of lock info
func TestLockInfo_Stringer(t *testing.T) {
	tests := []struct {
		name           string
		nodeID         string
		owner          string
		timeout        time.Duration
		expectContains []string
	}{
		{
			name:    "basic lock info",
			nodeID:  "1",
			owner:   "agent-001",
			timeout: 5 * time.Minute,
			expectContains: []string{
				"1",           // node ID
				"agent-001",   // owner
			},
		},
		{
			name:    "child node",
			nodeID:  "1.2.3",
			owner:   "prover-alpha",
			timeout: 1 * time.Hour,
			expectContains: []string{
				"1.2.3",
				"prover-alpha",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			info, err := lock.GetLockInfo(lk)
			if err != nil {
				t.Fatalf("GetLockInfo() unexpected error: %v", err)
			}

			str := info.String()

			for _, substr := range tt.expectContains {
				if !strings.Contains(str, substr) {
					t.Errorf("String() = %q, want to contain %q", str, substr)
				}
			}

			// String should not be empty
			if str == "" {
				t.Error("String() returned empty string")
			}
		})
	}
}

// TestLockInfo_Stringer_Expired verifies string representation includes expiration status
func TestLockInfo_Stringer_Expired(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	info, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() unexpected error: %v", err)
	}

	str := info.String()

	// Should indicate expiration in some way
	hasExpiredIndicator := strings.Contains(strings.ToLower(str), "expired") ||
		strings.Contains(str, "-") // negative duration

	if !hasExpiredIndicator {
		t.Errorf("String() = %q, want to indicate expired status", str)
	}
}

// TestLockInfo_JSON verifies JSON serialization of lock info
func TestLockInfo_JSON(t *testing.T) {
	tests := []struct {
		name   string
		nodeID string
		owner  string
	}{
		{"root node", "1", "agent-001"},
		{"child node", "1.1", "prover-alpha"},
		{"deep node", "1.2.3.4", "verifier-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewLock(nodeID, tt.owner, 5*time.Minute)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			info, err := lock.GetLockInfo(lk)
			if err != nil {
				t.Fatalf("GetLockInfo() unexpected error: %v", err)
			}

			// Serialize to JSON
			data, err := json.Marshal(info)
			if err != nil {
				t.Fatalf("json.Marshal() unexpected error: %v", err)
			}

			// Parse as generic map to verify structure
			var m map[string]interface{}
			err = json.Unmarshal(data, &m)
			if err != nil {
				t.Fatalf("json.Unmarshal() to map unexpected error: %v", err)
			}

			// Check expected fields exist
			requiredFields := []string{"node_id", "owner", "acquired", "expires", "remaining", "is_expired"}
			for _, field := range requiredFields {
				if _, ok := m[field]; !ok {
					t.Errorf("JSON missing required field %q", field)
				}
			}

			// Verify field values
			if nodeIDVal, ok := m["node_id"].(string); !ok || nodeIDVal != tt.nodeID {
				t.Errorf("JSON node_id = %v, want %q", m["node_id"], tt.nodeID)
			}

			if ownerVal, ok := m["owner"].(string); !ok || ownerVal != tt.owner {
				t.Errorf("JSON owner = %v, want %q", m["owner"], tt.owner)
			}
		})
	}
}

// TestLockInfo_JSON_Roundtrip verifies JSON serialization roundtrip
func TestLockInfo_JSON_Roundtrip(t *testing.T) {
	nodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2.3\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "roundtrip-agent", 30*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	original, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() unexpected error: %v", err)
	}

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	// Deserialize
	var restored lock.LockInfo
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	// Compare fields
	if restored.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch: got %q, want %q", restored.NodeID, original.NodeID)
	}

	if restored.Owner != original.Owner {
		t.Errorf("Owner mismatch: got %q, want %q", restored.Owner, original.Owner)
	}

	// Times should be equal (within JSON precision)
	if !restored.Acquired.Equal(original.Acquired) {
		t.Errorf("Acquired mismatch: got %v, want %v", restored.Acquired, original.Acquired)
	}

	if !restored.Expires.Equal(original.Expires) {
		t.Errorf("Expires mismatch: got %v, want %v", restored.Expires, original.Expires)
	}

	if restored.IsExpired != original.IsExpired {
		t.Errorf("IsExpired mismatch: got %v, want %v", restored.IsExpired, original.IsExpired)
	}
}

// TestLockInfo_JSON_ExpiredLock verifies JSON serialization of expired lock info
func TestLockInfo_JSON_ExpiredLock(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewLock(nodeID, "expired-agent", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	info, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() unexpected error: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	// Parse and verify is_expired is true
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	if err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	isExpired, ok := m["is_expired"].(bool)
	if !ok {
		t.Fatalf("is_expired field is not a bool: %T", m["is_expired"])
	}

	if !isExpired {
		t.Error("is_expired = false in JSON, want true for expired lock")
	}
}

// TestGetLockInfo_PreservesData verifies GetLockInfo returns accurate data
func TestGetLockInfo_PreservesData(t *testing.T) {
	nodeID, err := types.Parse("1.5.10")
	if err != nil {
		t.Fatalf("types.Parse(\"1.5.10\") unexpected error: %v", err)
	}

	owner := "test-preservation-agent"
	timeout := 15 * time.Minute

	lk, err := lock.NewLock(nodeID, owner, timeout)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	info, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() unexpected error: %v", err)
	}

	// Verify data matches lock
	if info.NodeID != lk.NodeID().String() {
		t.Errorf("NodeID = %q, want %q from lock", info.NodeID, lk.NodeID().String())
	}

	if info.Owner != lk.Owner() {
		t.Errorf("Owner = %q, want %q from lock", info.Owner, lk.Owner())
	}

	if info.IsExpired != lk.IsExpired() {
		t.Errorf("IsExpired = %v, want %v from lock", info.IsExpired, lk.IsExpired())
	}
}

// TestGetLockInfo_MultipleCalls verifies multiple calls return consistent data
func TestGetLockInfo_MultipleCalls(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewLock(nodeID, "multi-call-agent", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Call GetLockInfo multiple times
	info1, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() call 1 unexpected error: %v", err)
	}

	info2, err := lock.GetLockInfo(lk)
	if err != nil {
		t.Fatalf("GetLockInfo() call 2 unexpected error: %v", err)
	}

	// Static fields should be identical
	if info1.NodeID != info2.NodeID {
		t.Errorf("NodeID mismatch between calls: %q vs %q", info1.NodeID, info2.NodeID)
	}

	if info1.Owner != info2.Owner {
		t.Errorf("Owner mismatch between calls: %q vs %q", info1.Owner, info2.Owner)
	}

	if !info1.Acquired.Equal(info2.Acquired) {
		t.Errorf("Acquired mismatch between calls: %v vs %v", info1.Acquired, info2.Acquired)
	}

	if !info1.Expires.Equal(info2.Expires) {
		t.Errorf("Expires mismatch between calls: %v vs %v", info1.Expires, info2.Expires)
	}

	// Remaining may differ slightly due to time elapsed between calls
	// but should both be positive for non-expired lock
	if info1.Remaining <= 0 || info2.Remaining <= 0 {
		t.Errorf("Remaining should be positive: %v, %v", info1.Remaining, info2.Remaining)
	}
}

// TestLockInfo_ZeroValue verifies behavior with zero-value LockInfo
func TestLockInfo_ZeroValue(t *testing.T) {
	var info lock.LockInfo

	// Zero value should have empty strings and zero times
	if info.NodeID != "" {
		t.Errorf("Zero-value NodeID = %q, want empty", info.NodeID)
	}

	if info.Owner != "" {
		t.Errorf("Zero-value Owner = %q, want empty", info.Owner)
	}

	if !info.Acquired.IsZero() {
		t.Errorf("Zero-value Acquired = %v, want zero time", info.Acquired)
	}

	if !info.Expires.IsZero() {
		t.Errorf("Zero-value Expires = %v, want zero time", info.Expires)
	}

	if info.Remaining != 0 {
		t.Errorf("Zero-value Remaining = %v, want 0", info.Remaining)
	}

	if info.IsExpired {
		t.Error("Zero-value IsExpired = true, want false")
	}
}
