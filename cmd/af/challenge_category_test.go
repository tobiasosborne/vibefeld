// Package main contains tests for the challenge --category field (vibefeld-twdf).
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tobias/vibefeld/internal/service"
)

func setupChallengeCategoryTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "af-challenge-cat-*")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	if err := service.Init(tmpDir, "Root conjecture", "author"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	return tmpDir, cleanup
}

func execChallengeCat(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newChallengeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func execChallengesCat(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newChallengesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// TestChallengeCmd_HasCategoryFlag verifies the --category flag exists.
func TestChallengeCmd_HasCategoryFlag(t *testing.T) {
	cmd := newChallengeCmd()
	if cmd.Flags().Lookup("category") == nil {
		t.Error("expected challenge command to have a --category flag")
	}
}

// TestChallengeCmd_ValidCategoryPersists raises a challenge with a category and
// verifies it round-trips into state.
func TestChallengeCmd_ValidCategoryPersists(t *testing.T) {
	tmpDir, cleanup := setupChallengeCategoryTest(t)
	defer cleanup()

	out, err := execChallengeCat(t, "1", "--reason", "needs a citation", "--category", "missing", "-d", tmpDir)
	if err != nil {
		t.Fatalf("challenge with category failed: %v\n%s", err, out)
	}

	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	challenges := st.AllChallenges()
	if len(challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(challenges))
	}
	if challenges[0].Category != "missing" {
		t.Errorf("expected category 'missing', got %q", challenges[0].Category)
	}
}

// TestChallengeCmd_InvalidCategoryRejected verifies an unknown category errors
// and no challenge is written.
func TestChallengeCmd_InvalidCategoryRejected(t *testing.T) {
	tmpDir, cleanup := setupChallengeCategoryTest(t)
	defer cleanup()

	out, err := execChallengeCat(t, "1", "--reason", "x", "--category", "bogus", "-d", tmpDir)
	if err == nil {
		t.Fatalf("expected error for invalid category, got none; output: %s", out)
	}

	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	if len(st.AllChallenges()) != 0 {
		t.Error("invalid category must not write a challenge")
	}
}

// TestChallengeCmd_CategoryOptional verifies category may be omitted.
func TestChallengeCmd_CategoryOptional(t *testing.T) {
	tmpDir, cleanup := setupChallengeCategoryTest(t)
	defer cleanup()

	if _, err := execChallengeCat(t, "1", "--reason", "no category here", "-d", tmpDir); err != nil {
		t.Fatalf("challenge without category should succeed: %v", err)
	}
	svc, _ := service.NewProofService(tmpDir)
	st, _ := svc.LoadState()
	c := st.AllChallenges()
	if len(c) != 1 || c[0].Category != "" {
		t.Errorf("expected 1 challenge with empty category, got %+v", c)
	}
}

// TestChallengesCmd_CategoryInJSONAndFilter verifies category appears in JSON
// and that --category filters correctly.
func TestChallengesCmd_CategoryInJSONAndFilter(t *testing.T) {
	tmpDir, cleanup := setupChallengeCategoryTest(t)
	defer cleanup()

	if _, err := execChallengeCat(t, "1", "--reason", "missing lemma", "--category", "missing", "-d", tmpDir); err != nil {
		t.Fatalf("seed challenge 1 failed: %v", err)
	}
	if _, err := execChallengeCat(t, "1", "--reason", "dep not validated", "--category", "dependency", "-d", tmpDir); err != nil {
		t.Fatalf("seed challenge 2 failed: %v", err)
	}

	// JSON should carry the category.
	jsonOut, err := execChallengesCat(t, "--format", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("challenges json failed: %v", err)
	}
	var parsed struct {
		Challenges []struct {
			Category string `json:"category"`
		} `json:"challenges"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonOut)), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonOut)
	}
	if parsed.Total != 2 {
		t.Fatalf("expected 2 challenges, got %d", parsed.Total)
	}
	cats := map[string]bool{}
	for _, c := range parsed.Challenges {
		cats[c.Category] = true
	}
	if !cats["missing"] || !cats["dependency"] {
		t.Errorf("expected both categories present, got %v", cats)
	}

	// --category filter narrows to one.
	filtered, err := execChallengesCat(t, "--category", "missing", "--format", "json", "-d", tmpDir)
	if err != nil {
		t.Fatalf("filtered challenges failed: %v", err)
	}
	var fp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(filtered)), &fp); err != nil {
		t.Fatalf("invalid filtered JSON: %v", err)
	}
	if fp.Total != 1 {
		t.Errorf("expected 1 challenge after --category missing, got %d", fp.Total)
	}
}

// TestChallengesCmd_InvalidCategoryFilterRejected verifies a bad --category
// filter errors.
func TestChallengesCmd_InvalidCategoryFilterRejected(t *testing.T) {
	tmpDir, cleanup := setupChallengeCategoryTest(t)
	defer cleanup()
	if _, err := execChallengesCat(t, "--category", "nope", "-d", tmpDir); err == nil {
		t.Error("expected error for invalid --category filter")
	}
}
