package ledger

import (
	"path/filepath"
	"testing"
)

// TestFsyncDir_NoFsyncSwitch covers the test-only AF_TEST_NO_FSYNC switch used
// by the benchmark job: when set to "1" fsyncDir is a no-op; any other value
// (including "0") leaves the normal behaviour, so a missing directory still
// reports an error. The switch is unsafe for durability and never set in
// production; this test pins the exact "1" semantics.
func TestFsyncDir_NoFsyncSwitch(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	t.Setenv(nofsyncEnv, "1")
	if err := fsyncDir(missing); err != nil {
		t.Fatalf("fsyncDir with %s=1 = %v, want nil (no-op)", nofsyncEnv, err)
	}

	t.Setenv(nofsyncEnv, "0")
	if err := fsyncDir(missing); err == nil {
		t.Fatalf("fsyncDir with %s=0 = nil, want error for a missing directory", nofsyncEnv)
	}
}
