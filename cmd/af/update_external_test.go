//go:build integration

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupUpdateExternalTest(t *testing.T) (string, *service.ProofService, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "af-update-external-test-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := service.InitProofDir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err := service.Init(tmpDir, "Test conjecture", "test-author"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	svc, err := service.NewProofService(tmpDir)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, svc, cleanup
}

func TestUpdateExternalCmd_UpdateSource(t *testing.T) {
	tmpDir, svc, cleanup := setupUpdateExternalTest(t)
	defer cleanup()

	// Add an external with wrong source
	extID, err := svc.AddExternal("Theorem 3.1", "arXiv:2310.09740")
	if err != nil {
		t.Fatalf("AddExternal: %v", err)
	}

	// Update the source
	cmd := newUpdateExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{extID, "--source", "arXiv:2403.10485", "--notes", "Corrected citation", "-d", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update-external failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "updated") {
		t.Errorf("expected 'updated' in output, got: %q", output)
	}

	// Verify the update persisted
	ext, err := svc.ReadExternal(extID)
	if err != nil {
		t.Fatalf("ReadExternal: %v", err)
	}
	if ext.Source != "arXiv:2403.10485" {
		t.Errorf("source = %q, want %q", ext.Source, "arXiv:2403.10485")
	}
	if ext.Notes != "Corrected citation" {
		t.Errorf("notes = %q, want %q", ext.Notes, "Corrected citation")
	}
	if ext.Name != "Theorem 3.1" {
		t.Errorf("name should be unchanged, got %q", ext.Name)
	}
}

func TestUpdateExternalCmd_LookupByName(t *testing.T) {
	tmpDir, svc, cleanup := setupUpdateExternalTest(t)
	defer cleanup()

	_, err := svc.AddExternal("Old Paper", "some source")
	if err != nil {
		t.Fatalf("AddExternal: %v", err)
	}

	// Update by name
	cmd := newUpdateExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"Old Paper", "--name", "New Paper", "-d", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update-external by name failed: %v", err)
	}

	if !strings.Contains(buf.String(), "New Paper") {
		t.Errorf("expected 'New Paper' in output, got: %q", buf.String())
	}
}

func TestUpdateExternalCmd_NotFound(t *testing.T) {
	tmpDir, _, cleanup := setupUpdateExternalTest(t)
	defer cleanup()

	cmd := newUpdateExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"nonexistent", "--source", "x", "-d", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent external")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %q", err.Error())
	}
}

func TestUpdateExternalCmd_NoFieldsProvided(t *testing.T) {
	tmpDir, _, cleanup := setupUpdateExternalTest(t)
	defer cleanup()

	cmd := newUpdateExternalCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"anything", "-d", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no fields provided")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("expected 'at least one' in error, got: %q", err.Error())
	}
}
