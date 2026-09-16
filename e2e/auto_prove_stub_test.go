//go:build integration

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var bashMajorRE = regexp.MustCompile(`GNU bash, version (\d+)`)

// TestAutoProveStub drives scripts/test-auto-prove.sh, which runs
// scripts/auto-prove.sh against a stub agent on a disposable workspace and
// asserts that the commands auto-prove.sh generates execute without unknown
// flags. It is skipped unless a bash >= 4 is available (bash 3.2 on stock
// macOS cannot run auto-prove.sh at all).
func TestAutoProveStub(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("auto-prove.sh requires bash")
	}
	versionOut, err := exec.Command("bash", "--version").CombinedOutput()
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}
	m := bashMajorRE.FindStringSubmatch(string(versionOut))
	if m == nil {
		t.Skipf("could not parse bash version from %q", strings.TrimSpace(string(versionOut)))
	}
	major, err := strconv.Atoi(m[1])
	if err != nil || major < 4 {
		t.Skipf("bash >= 4 required, found %q", strings.TrimSpace(string(versionOut)))
	}

	script, err := filepath.Abs(filepath.Join("..", "scripts", "test-auto-prove.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("test-auto-prove.sh not found: %v", err)
	}

	cmd := exec.Command("bash", script)
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("test-auto-prove.sh failed: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "unknown flag") {
		t.Fatalf("auto-prove.sh generated a command with an unknown flag:\n%s", output)
	}
}
