package service

import (
	"errors"
	"os"

	"github.com/tobiasosborne/vibefeld/internal/audit"
	"github.com/tobiasosborne/vibefeld/internal/state"
)

// LoadStateWithPass loads the current proof state and the ordered audit pass
// from a single ledger replay, then loads assumptions and externals from disk
// exactly as LoadState does. The audit CLI and the amend-deps summary need
// both, and one replay keeps them on the same snapshot of the ledger.
func (s *ProofService) LoadStateWithPass() (*state.State, *audit.Pass, error) {
	ldg, err := s.getLedger()
	if err != nil {
		return nil, nil, err
	}
	st, pass, err := audit.BuildPass(ldg)
	if err != nil {
		return nil, nil, err
	}
	if err := s.loadAssumptionsIntoState(st); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	if err := s.loadExternalsIntoState(st); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	return st, pass, nil
}
