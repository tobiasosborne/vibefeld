// Package main contains shared workspace-opening helpers for af commands.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tobiasosborne/vibefeld/internal/config"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

// openWorkspaceLedger is the shared entry point for cmd/af commands that read
// the ledger directly instead of going through service.ProofService. It reads
// meta.json and refuses a workspace format this binary cannot read *before*
// touching the ledger, then opens the ledger. A missing meta.json is treated
// as an uninitialised workspace (the same as ProofService.LoadConfig) so
// pre-init reads keep working; every af-written workspace has a meta.json.
//
// Use this in every cmd/af command that would otherwise call
// ledger.NewLedger(dir) directly, so the format gate cannot be bypassed by a
// new entry point.
func openWorkspaceLedger(dir string) (*ledger.Ledger, *config.Config, error) {
	metaPath := filepath.Join(dir, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("error reading workspace config: %w", err)
		}
		cfg = config.Default()
	}

	if err := config.CheckFormat(cfg); err != nil {
		return nil, nil, err
	}

	ldg, err := ledger.NewLedger(filepath.Join(dir, "ledger"))
	if err != nil {
		return nil, nil, fmt.Errorf("error accessing ledger: %w", err)
	}

	return ldg, cfg, nil
}
