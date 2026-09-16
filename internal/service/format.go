package service

import (
	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

// checkEventFormats refuses to append any event whose minimum workspace format
// is newer than the workspace's stamped format. It is the 1.0-side half of the
// format gate: even if a caller somehow builds a 1.1 event on a 1.0 workspace,
// the write path stops it with a self-teaching error instead of writing it.
//
// The companion check for a *workspace* newer than this binary lives in
// config.CheckFormat (called from NewProofService and the replay CLI).
func checkEventFormats(cfg *config.Config, events []ledger.Event) error {
	if cfg == nil {
		return nil
	}
	for _, ev := range events {
		if ev == nil {
			continue
		}
		required := ev.Type().MinFormat()
		if config.CompareFormats(required, cfg.Version) > 0 {
			return aferrors.Newf(aferrors.FORMAT_TOO_NEW,
				"event type %s requires workspace format %s; run `af workspace upgrade --to %s`",
				ev.Type(), required, required)
		}
	}
	return nil
}
