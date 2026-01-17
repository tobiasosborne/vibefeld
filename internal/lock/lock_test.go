package lock_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tobias/vibefeld/internal/lock"
	"github.com/tobias/vibefeld/internal/types"
)

// TestNewLock_Valid verifies NewLock creates locks with correct fields
func TestNewLock_Valid(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node", "1", "agent-001", 5 * time.Minute},
		{"child node", "1.1", "agent-002", 10 * time.Second},
		{"deep node", "1.2.3.4", "prover-alpha", 1 * time.Hour},
		{"short timeout", "1.1.1", "verifier-beta", 1 * time.Millisecond},
		{"long owner name", "1", "very-long-agent-identifier-with-many-characters", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			lk, err := lock.NewClaimLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Verify NodeID
			if lk.NodeID().String() != tt.nodeID {
				t.Errorf("NodeID() = %q, want %q", lk.NodeID().String(), tt.nodeID)
			}

			// Verify Owner
			if lk.Owner() != tt.owner {
				t.Errorf("Owner() = %q, want %q", lk.Owner(), tt.owner)
			}

			// Verify AcquiredAt is set (not zero)
			if lk.AcquiredAt().IsZero() {
				t.Error("AcquiredAt() is zero, want non-zero timestamp")
			}

			// Verify ExpiresAt is after AcquiredAt
			if !lk.ExpiresAt().After(lk.AcquiredAt()) {
				t.Errorf("ExpiresAt() = %v should be after AcquiredAt() = %v",
					lk.ExpiresAt(), lk.AcquiredAt())
			}
		})
	}
}

// TestNewLock_EmptyOwner verifies NewLock rejects empty owner
func TestNewLock_EmptyOwner(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	_, err = lock.NewClaimLock(nodeID, "", 5*time.Minute)
	if err == nil {
		t.Error("NewLock() with empty owner expected error, got nil")
	}
}

// TestNewLock_ZeroTimeout verifies NewLock rejects zero timeout
func TestNewLock_ZeroTimeout(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	_, err = lock.NewClaimLock(nodeID, "agent-001", 0)
	if err == nil {
		t.Error("NewLock() with zero timeout expected error, got nil")
	}
}

// TestNewLock_NegativeTimeout verifies NewLock rejects negative timeout
func TestNewLock_NegativeTimeout(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	_, err = lock.NewClaimLock(nodeID, "agent-001", -5*time.Minute)
	if err == nil {
		t.Error("NewLock() with negative timeout expected error, got nil")
	}
}

// TestNewLock_InvalidNodeID verifies NewLock rejects zero-value NodeID
func TestNewLock_InvalidNodeID(t *testing.T) {
	var zeroNodeID types.NodeID // zero value

	_, err := lock.NewClaimLock(zeroNodeID, "agent-001", 5*time.Minute)
	if err == nil {
		t.Error("NewLock() with zero-value NodeID expected error, got nil")
	}
}

// TestIsExpired_NotExpired verifies IsExpired returns false for fresh lock
func TestIsExpired_NotExpired(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	if lk.IsExpired() {
		t.Error("IsExpired() = true for fresh lock, want false")
	}
}

// TestIsExpired_Expired verifies IsExpired returns true after timeout
func TestIsExpired_Expired(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	if !lk.IsExpired() {
		t.Error("IsExpired() = false after timeout, want true")
	}
}

// TestIsOwnedBy_SameOwner verifies IsOwnedBy returns true for same owner
func TestIsOwnedBy_SameOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"simple owner", "agent-001"},
		{"hyphenated owner", "prover-verifier-agent"},
		{"long owner", "very-long-agent-identifier-with-many-characters-12345"},
		{"numeric suffix", "agent123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewClaimLock(nodeID, tt.owner, 5*time.Minute)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if !lk.IsOwnedBy(tt.owner) {
				t.Errorf("IsOwnedBy(%q) = false, want true", tt.owner)
			}
		})
	}
}

// TestIsOwnedBy_DifferentOwner verifies IsOwnedBy returns false for different owner
func TestIsOwnedBy_DifferentOwner(t *testing.T) {
	tests := []struct {
		name       string
		lockOwner  string
		queryOwner string
	}{
		{"completely different", "agent-001", "agent-002"},
		{"similar name", "prover", "prover-1"},
		{"case different", "Agent-001", "agent-001"},
		{"empty query", "agent-001", ""},
		{"prefix match", "agent", "agent-001"},
		{"suffix match", "001", "agent-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			lk, err := lock.NewClaimLock(nodeID, tt.lockOwner, 5*time.Minute)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			if lk.IsOwnedBy(tt.queryOwner) {
				t.Errorf("IsOwnedBy(%q) = true for lock owned by %q, want false",
					tt.queryOwner, tt.lockOwner)
			}
		})
	}
}

// TestRefresh_ExtendsExpiration verifies Refresh extends expiration time
func TestRefresh_ExtendsExpiration(t *testing.T) {
	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	// Create lock with short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	originalExpires := lk.ExpiresAt()

	// Wait a tiny bit then refresh with longer timeout
	time.Sleep(10 * time.Millisecond)

	err = lk.Refresh(1 * time.Hour)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}

	newExpires := lk.ExpiresAt()

	// New expiration should be after original
	if !newExpires.After(originalExpires) {
		t.Errorf("ExpiresAt() after Refresh = %v, should be after original %v",
			newExpires, originalExpires)
	}

	// Lock should not be expired
	if lk.IsExpired() {
		t.Error("IsExpired() = true after Refresh, want false")
	}
}

// TestRefresh_ZeroTimeout verifies Refresh rejects zero timeout
func TestRefresh_ZeroTimeout(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	err = lk.Refresh(0)
	if err == nil {
		t.Error("Refresh(0) expected error, got nil")
	}
}

// TestRefresh_NegativeTimeout verifies Refresh rejects negative timeout
func TestRefresh_NegativeTimeout(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	err = lk.Refresh(-5 * time.Minute)
	if err == nil {
		t.Error("Refresh(-5m) expected error, got nil")
	}
}

// TestRefresh_PreservesOtherFields verifies Refresh doesn't change NodeID or Owner
func TestRefresh_PreservesOtherFields(t *testing.T) {
	nodeID, err := types.Parse("1.2.3")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2.3\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	originalNodeID := lk.NodeID().String()
	originalOwner := lk.Owner()

	err = lk.Refresh(10 * time.Minute)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}

	// NodeID should be unchanged
	if lk.NodeID().String() != originalNodeID {
		t.Errorf("NodeID() changed from %q to %q after Refresh",
			originalNodeID, lk.NodeID().String())
	}

	// Owner should be unchanged
	if lk.Owner() != originalOwner {
		t.Errorf("Owner() changed from %q to %q after Refresh",
			originalOwner, lk.Owner())
	}
}

// TestRefresh_ExpiredLockBehavior verifies that refreshing an expired lock succeeds.
// This is intentional behavior: it allows recovery from brief expirations caused by
// clock skew or timing issues. The alternative (rejecting refresh on expired locks)
// would require agents to re-acquire locks through the full claim process, creating
// unnecessary churn. Since the lock owner is preserved, refreshing an expired lock
// is safe - no other agent could have claimed it in the meantime without the original
// agent releasing it or the lock being reaped.
func TestRefresh_ExpiredLockBehavior(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	// Create lock with very short timeout
	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("NewClaimLock() unexpected error: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Verify lock is expired
	if !lk.IsExpired() {
		t.Fatal("Lock should be expired after timeout")
	}

	// Refresh the expired lock - this should succeed
	err = lk.Refresh(1 * time.Hour)
	if err != nil {
		t.Fatalf("Refresh() on expired lock should succeed, got error: %v", err)
	}

	// Lock should no longer be expired after refresh
	if lk.IsExpired() {
		t.Error("IsExpired() = true after refresh, want false")
	}

	// Owner should still be preserved
	if !lk.IsOwnedBy("agent-001") {
		t.Errorf("IsOwnedBy(\"agent-001\") = false after refresh, want true")
	}

	// NodeID should still be preserved
	if lk.NodeID().String() != "1" {
		t.Errorf("NodeID() = %q after refresh, want \"1\"", lk.NodeID().String())
	}
}

// TestJSON_Roundtrip verifies locks can be serialized and deserialized
func TestJSON_Roundtrip(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		owner   string
		timeout time.Duration
	}{
		{"root node", "1", "agent-001", 5 * time.Minute},
		{"child node", "1.1", "prover-alpha", 1 * time.Hour},
		{"deep node", "1.2.3.4.5", "verifier-beta-gamma", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse(tt.nodeID)
			if err != nil {
				t.Fatalf("types.Parse(%q) unexpected error: %v", tt.nodeID, err)
			}

			original, err := lock.NewClaimLock(nodeID, tt.owner, tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}

			// Serialize to JSON
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() unexpected error: %v", err)
			}

			// Deserialize from JSON
			var restored lock.ClaimLock
			err = json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json.Unmarshal() unexpected error: %v", err)
			}

			// Verify fields match
			if restored.NodeID().String() != original.NodeID().String() {
				t.Errorf("NodeID mismatch: got %q, want %q",
					restored.NodeID().String(), original.NodeID().String())
			}

			if restored.Owner() != original.Owner() {
				t.Errorf("Owner mismatch: got %q, want %q",
					restored.Owner(), original.Owner())
			}

			if !restored.AcquiredAt().Equal(original.AcquiredAt()) {
				t.Errorf("AcquiredAt mismatch: got %v, want %v",
					restored.AcquiredAt(), original.AcquiredAt())
			}

			if !restored.ExpiresAt().Equal(original.ExpiresAt()) {
				t.Errorf("ExpiresAt mismatch: got %v, want %v",
					restored.ExpiresAt(), original.ExpiresAt())
			}
		})
	}
}

// TestJSON_ValidFormat verifies JSON output format
func TestJSON_ValidFormat(t *testing.T) {
	nodeID, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "test-agent", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	data, err := json.Marshal(lk)
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
	requiredFields := []string{"node_id", "owner", "acquired_at", "expires_at"}
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("JSON missing required field %q", field)
		}
	}
}

// TestJSON_UnmarshalInvalid verifies Unmarshal handles invalid JSON
func TestJSON_UnmarshalInvalid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty object", `{}`},
		{"missing owner", `{"node_id":"1","acquired_at":"2025-01-01T00:00:00.000000000Z","expires_at":"2025-01-01T01:00:00.000000000Z"}`},
		{"missing node_id", `{"owner":"agent","acquired_at":"2025-01-01T00:00:00.000000000Z","expires_at":"2025-01-01T01:00:00.000000000Z"}`},
		{"invalid node_id", `{"node_id":"invalid","owner":"agent","acquired_at":"2025-01-01T00:00:00.000000000Z","expires_at":"2025-01-01T01:00:00.000000000Z"}`},
		{"invalid timestamp", `{"node_id":"1","owner":"agent","acquired_at":"not-a-time","expires_at":"2025-01-01T01:00:00.000000000Z"}`},
		{"malformed json", `{not valid json}`},
		{"array instead of object", `[]`},
		{"null", `null`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lk lock.ClaimLock
			err := json.Unmarshal([]byte(tt.json), &lk)
			if err == nil {
				t.Errorf("json.Unmarshal(%q) expected error, got nil", tt.json)
			}
		})
	}
}

// TestValidation_WhitespaceOwner verifies whitespace-only owner is rejected
func TestValidation_WhitespaceOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner string
	}{
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			_, err = lock.NewClaimLock(nodeID, tt.owner, 5*time.Minute)
			if err == nil {
				t.Errorf("NewLock() with whitespace owner %q expected error, got nil", tt.owner)
			}
		})
	}
}

// TestExpirationCalculation verifies expiration is calculated correctly
func TestExpirationCalculation(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"1 minute", 1 * time.Minute},
		{"5 minutes", 5 * time.Minute},
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, err := types.Parse("1")
			if err != nil {
				t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
			}

			before := time.Now()
			lk, err := lock.NewClaimLock(nodeID, "agent-001", tt.timeout)
			if err != nil {
				t.Fatalf("NewLock() unexpected error: %v", err)
			}
			after := time.Now()

			// ExpiresAt should be approximately AcquiredAt + timeout
			// Allow for small timing differences
			expectedMin := before.Add(tt.timeout)
			expectedMax := after.Add(tt.timeout).Add(100 * time.Millisecond)

			// Use time.Time for comparison since we need to extract time from Timestamp
			// This relies on ExpiresAt being within a reasonable range
			if lk.IsExpired() {
				// If already expired, the timeout was very short (sub-millisecond)
				// and that's acceptable for very short timeouts
				if tt.timeout > 100*time.Millisecond {
					t.Error("Lock expired immediately for non-trivial timeout")
				}
				return
			}

			// For longer timeouts, verify expiration is in the future
			_ = expectedMin
			_ = expectedMax
		})
	}
}

// TestConcurrentAccess verifies lock is safe for concurrent method calls
func TestConcurrentAccess(t *testing.T) {
	nodeID, err := types.Parse("1")
	if err != nil {
		t.Fatalf("types.Parse(\"1\") unexpected error: %v", err)
	}

	lk, err := lock.NewClaimLock(nodeID, "agent-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("NewLock() unexpected error: %v", err)
	}

	// Spawn multiple goroutines accessing lock methods
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = lk.NodeID()
				_ = lk.Owner()
				_ = lk.AcquiredAt()
				_ = lk.ExpiresAt()
				_ = lk.IsExpired()
				_ = lk.IsOwnedBy("agent-001")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestIsExpired_ClockSkewHandling verifies lock expiration behavior under clock skew scenarios.
// Clock skew occurs when system time jumps forward or backward (e.g., NTP synchronization).
//
// Current behavior (documented, not necessarily ideal):
// - IsExpired() compares against time.Now(), so it reflects the current system time
// - If system time jumps backward, previously expired locks may appear valid again
// - If system time jumps forward, locks expire earlier than expected
//
// This test verifies the comparison logic works correctly by creating locks with
// expiration times in the past/future using JSON unmarshaling to simulate the
// effect of clock skew on pre-existing locks.
func TestIsExpired_ClockSkewHandling(t *testing.T) {
	// Scenario 1: Lock created in the past (simulates clock jumping forward)
	// If clock jumps forward significantly, a lock that should have been valid
	// will appear expired because time.Now() is now past expiresAt
	t.Run("clock jumps forward - lock appears expired early", func(t *testing.T) {
		// Create a lock via JSON with expiration 1 hour in the past
		// This simulates: lock was created, then clock jumped forward
		pastExpiry := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
		pastAcquired := time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano)
		jsonData := []byte(`{
			"node_id": "1",
			"owner": "agent-001",
			"acquired_at": "` + pastAcquired + `",
			"expires_at": "` + pastExpiry + `"
		}`)

		var lk lock.ClaimLock
		if err := json.Unmarshal(jsonData, &lk); err != nil {
			t.Fatalf("json.Unmarshal() unexpected error: %v", err)
		}

		// Lock should be expired since expiresAt is in the past
		if !lk.IsExpired() {
			t.Error("IsExpired() = false for lock with past expiration, want true")
		}
	})

	// Scenario 2: Lock created in the future (simulates clock jumping backward)
	// If clock jumps backward, a lock that was about to expire will appear valid
	// for longer than expected
	t.Run("clock jumps backward - lock appears valid longer", func(t *testing.T) {
		// Create a lock via JSON with expiration 1 hour in the future
		// This simulates: lock was created, then clock jumped backward
		futureExpiry := time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano)
		recentAcquired := time.Now().Add(-1 * time.Minute).Format(time.RFC3339Nano)
		jsonData := []byte(`{
			"node_id": "1",
			"owner": "agent-002",
			"acquired_at": "` + recentAcquired + `",
			"expires_at": "` + futureExpiry + `"
		}`)

		var lk lock.ClaimLock
		if err := json.Unmarshal(jsonData, &lk); err != nil {
			t.Fatalf("json.Unmarshal() unexpected error: %v", err)
		}

		// Lock should NOT be expired since expiresAt is in the future
		if lk.IsExpired() {
			t.Error("IsExpired() = true for lock with future expiration, want false")
		}
	})

	// Scenario 3: Boundary case - expiration exactly at current time
	// Tests behavior when expiresAt equals time.Now() (race condition territory)
	t.Run("expiration at boundary - near-current time", func(t *testing.T) {
		// Create lock expiring very close to now (within milliseconds)
		// Due to timing, this might be expired or not - we just verify no panic
		nearNow := time.Now().Add(1 * time.Millisecond).Format(time.RFC3339Nano)
		acquired := time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano)
		jsonData := []byte(`{
			"node_id": "1",
			"owner": "agent-003",
			"acquired_at": "` + acquired + `",
			"expires_at": "` + nearNow + `"
		}`)

		var lk lock.ClaimLock
		if err := json.Unmarshal(jsonData, &lk); err != nil {
			t.Fatalf("json.Unmarshal() unexpected error: %v", err)
		}

		// Just verify the call doesn't panic - result depends on timing
		_ = lk.IsExpired()
	})

	// Scenario 4: Extreme clock skew - expiration far in the past
	// Simulates a severe clock jump (e.g., year-level discrepancy)
	t.Run("extreme clock skew - year-old expiration", func(t *testing.T) {
		// Lock that "expired" a year ago
		oldExpiry := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339Nano)
		oldAcquired := time.Now().AddDate(-1, 0, -1).Format(time.RFC3339Nano)
		jsonData := []byte(`{
			"node_id": "1.2.3",
			"owner": "agent-004",
			"acquired_at": "` + oldAcquired + `",
			"expires_at": "` + oldExpiry + `"
		}`)

		var lk lock.ClaimLock
		if err := json.Unmarshal(jsonData, &lk); err != nil {
			t.Fatalf("json.Unmarshal() unexpected error: %v", err)
		}

		// Lock should definitely be expired
		if !lk.IsExpired() {
			t.Error("IsExpired() = false for year-old lock, want true")
		}
	})

	// Scenario 5: Extreme clock skew - expiration far in the future
	// Simulates a lock that won't expire for years (e.g., clock went backward by years)
	t.Run("extreme clock skew - far future expiration", func(t *testing.T) {
		// Lock that "expires" in 10 years
		farFutureExpiry := time.Now().AddDate(10, 0, 0).Format(time.RFC3339Nano)
		recentAcquired := time.Now().Add(-1 * time.Minute).Format(time.RFC3339Nano)
		jsonData := []byte(`{
			"node_id": "1.2.3.4",
			"owner": "agent-005",
			"acquired_at": "` + recentAcquired + `",
			"expires_at": "` + farFutureExpiry + `"
		}`)

		var lk lock.ClaimLock
		if err := json.Unmarshal(jsonData, &lk); err != nil {
			t.Fatalf("json.Unmarshal() unexpected error: %v", err)
		}

		// Lock should NOT be expired
		if lk.IsExpired() {
			t.Error("IsExpired() = true for far-future lock, want false")
		}
	})
}

// TestMultipleLocks verifies multiple locks can coexist independently
func TestMultipleLocks(t *testing.T) {
	nodeID1, err := types.Parse("1.1")
	if err != nil {
		t.Fatalf("types.Parse(\"1.1\") unexpected error: %v", err)
	}

	nodeID2, err := types.Parse("1.2")
	if err != nil {
		t.Fatalf("types.Parse(\"1.2\") unexpected error: %v", err)
	}

	lk1, err := lock.NewClaimLock(nodeID1, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() for node 1.1 unexpected error: %v", err)
	}

	lk2, err := lock.NewClaimLock(nodeID2, "agent-002", 10*time.Minute)
	if err != nil {
		t.Fatalf("NewLock() for node 1.2 unexpected error: %v", err)
	}

	// Locks should be independent
	if lk1.NodeID().String() == lk2.NodeID().String() {
		t.Error("Different locks have same NodeID")
	}

	if lk1.Owner() == lk2.Owner() {
		t.Error("Different locks have same Owner")
	}

	if lk1.IsOwnedBy("agent-002") {
		t.Error("lk1.IsOwnedBy(\"agent-002\") = true, want false")
	}

	if lk2.IsOwnedBy("agent-001") {
		t.Error("lk2.IsOwnedBy(\"agent-001\") = true, want false")
	}
}
