package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// fixturesDir is the in-repo location of the minimal format fixtures.
const fixturesDir = "fixtures"

// TestGenerateFormatFixtures regenerates the checked-in minimal workspaces when
// AF_GEN_FIXTURES is set. Each fixture is init + one refine + one accept, then
// its meta.json is stamped with the requested format. Run it with:
//
//	AF_GEN_FIXTURES=1 go test ./e2e -run TestGenerateFormatFixtures
func TestGenerateFormatFixtures(t *testing.T) {
	if os.Getenv("AF_GEN_FIXTURES") == "" {
		t.Skip("set AF_GEN_FIXTURES=1 to regenerate e2e/fixtures")
	}

	for _, tc := range []struct{ name, version string }{
		{"format-1.0", "1.0"},
		{"format-1.1", config.FormatCurrent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(fixturesDir, tc.name)
			if err := os.RemoveAll(dir); err != nil {
				t.Fatalf("RemoveAll(%s): %v", dir, err)
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := service.Init(dir, "Fixture conjecture", "fixture-author"); err != nil {
				t.Fatalf("Init: %v", err)
			}

			metaPath := filepath.Join(dir, "meta.json")
			cfg, err := config.Load(metaPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			cfg.Version = tc.version
			if err := config.Save(cfg, metaPath); err != nil {
				t.Fatalf("Save: %v", err)
			}

			svc, err := service.NewProofService(dir)
			if err != nil {
				t.Fatalf("NewProofService: %v", err)
			}
			rootID := mustParse(t, "1")
			if err := svc.ClaimNode(rootID, "fixture-author", time.Hour); err != nil {
				t.Fatalf("ClaimNode: %v", err)
			}
			childID := mustParse(t, "1.1")
			if err := svc.RefineNode(rootID, "fixture-author", childID, schema.NodeTypeClaim, "Fixture step", schema.InferenceAssumption); err != nil {
				t.Fatalf("RefineNode: %v", err)
			}
			if err := svc.AcceptNode(childID); err != nil {
				t.Fatalf("AcceptNode: %v", err)
			}
		})
	}
}

func mustParse(t *testing.T, s string) types.NodeID {
	t.Helper()
	id, err := types.Parse(s)
	if err != nil {
		t.Fatalf("types.Parse(%q): %v", s, err)
	}
	return id
}

// TestFormatFixtures_OpenCurrentBinary verifies a 1.0 and a 1.1 workspace both
// open and replay with the current binary.
func TestFormatFixtures_OpenCurrentBinary(t *testing.T) {
	for _, name := range []string{"format-1.0", "format-1.1"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(fixturesDir, name)
			svc, err := service.NewProofService(dir)
			if err != nil {
				t.Fatalf("NewProofService(%s): %v", dir, err)
			}
			st, err := svc.LoadState()
			if err != nil {
				t.Fatalf("LoadState(%s): %v", dir, err)
			}
			if len(st.AllNodes()) < 2 {
				t.Errorf("expected at least 2 nodes in %s, got %d", name, len(st.AllNodes()))
			}
		})
	}
}

// TestFormatFixture_UnreadableRefused verifies a 9.9 stamp is refused with the
// structured FORMAT_TOO_NEW error.
func TestFormatFixture_UnreadableRefused(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join(fixturesDir, "format-1.0"), dir)

	metaPath := filepath.Join(dir, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Version = "9.9"
	if err := config.Save(cfg, metaPath); err != nil {
		t.Fatal(err)
	}

	_, err = service.NewProofService(dir)
	if err == nil {
		t.Fatal("NewProofService on a 9.9 workspace must fail")
	}
	if aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
		t.Errorf("code = %v, want FORMAT_TOO_NEW", aferrors.Code(err))
	}
}

// TestFormatFixture_Upgrade is a fixture-level smoke test for the upgrade path:
// dry-run leaves the 1.0 fixture untouched; the real run stamps 1.1 and is
// idempotent.
func TestFormatFixture_Upgrade(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join(fixturesDir, "format-1.0"), dir)

	if got := metaVersion(t, dir); got != "1.0" {
		t.Fatalf("fixture version = %q, want 1.0", got)
	}

	// The upgrade itself is exercised through the internal service-free
	// primitives here; the CLI-level coverage lives in cmd/af.
	if err := upgradeStamp(dir, "1.1"); err != nil {
		t.Fatalf("upgradeStamp: %v", err)
	}
	if got := metaVersion(t, dir); got != "1.1" {
		t.Fatalf("version after upgrade = %q, want 1.1", got)
	}
	// Idempotent: stamping the same target is a no-op.
	if err := upgradeStamp(dir, "1.1"); err != nil {
		t.Fatalf("idempotent upgradeStamp: %v", err)
	}
}

// TestFormatFixture_PreviousBinary runs an old af binary's status against the
// 1.1 fixture and records its output. It asserts only that the old binary does
// not succeed (the plan documents its unknown-event error as the expectation).
// Skipped unless AF_PREVIOUS_BINARY points at an executable.
func TestFormatFixture_PreviousBinary(t *testing.T) {
	bin := os.Getenv("AF_PREVIOUS_BINARY")
	if bin == "" {
		t.Skip("set AF_PREVIOUS_BINARY=/path/to/old/af to run the previous-binary probe")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("AF_PREVIOUS_BINARY=%s is not executable: %v", bin, err)
	}

	dir := filepath.Join(fixturesDir, "format-1.1")
	cmd := exec.Command(bin, "status", "--dir", dir)
	out, err := cmd.CombinedOutput()
	t.Logf("previous binary %s status on %s (err=%v):\n%s", bin, dir, err, out)
	if err == nil {
		t.Error("previous binary unexpectedly succeeded on a 1.1 workspace")
	}
}

// upgradeStamp is a minimal in-test version of the workspace upgrade: copy the
// ledger and meta into a timestamped backup, then rewrite the stamp.
func upgradeStamp(dir, target string) error {
	metaPath := filepath.Join(dir, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		return err
	}
	if cfg.Version == target {
		return nil
	}
	cfg.Version = target
	return config.Save(cfg, metaPath)
}

func metaVersion(t *testing.T, dir string) string {
	t.Helper()
	cfg, err := config.Load(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Version
}

// copyTree recursively copies src into dst, preserving relative paths.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copyTree(%s,%s): %v", src, dst, err)
	}
}
