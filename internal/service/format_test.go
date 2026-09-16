package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

const futureEvent ledger.EventType = "future_format_1_1_event"

func registerFutureEvent(t *testing.T) {
	t.Helper()
	ledger.RegisterEventMinFormat(futureEvent, "1.1")
	t.Cleanup(func() { ledger.RegisterEventMinFormat(futureEvent, "1.0") })
}

func TestCheckEventFormats(t *testing.T) {
	registerFutureEvent(t)

	existing := ledger.BaseEvent{EventType: ledger.EventNodeCreated}
	future := ledger.BaseEvent{EventType: futureEvent}

	tests := []struct {
		name    string
		version string
		nilCfg  bool
		events  []ledger.Event
		wantErr bool
	}{
		{"1.0 event on 1.0 workspace", "1.0", false, []ledger.Event{existing}, false},
		{"1.0 event on 1.1 workspace", "1.1", false, []ledger.Event{existing}, false},
		{"1.1 event on 1.1 workspace", "1.1", false, []ledger.Event{future}, false},
		{"1.1 event on 1.0 workspace", "1.0", false, []ledger.Event{future}, true},
		{"mixed batch with a 1.1 event", "1.0", false, []ledger.Event{existing, future}, true},
		{"nil config is a no-op", "", true, []ledger.Event{future}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *config.Config
			if !tt.nilCfg {
				cfg = &config.Config{Version: tt.version}
			}
			err := checkEventFormats(cfg, tt.events)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkEventFormats() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
				t.Errorf("checkEventFormats() code = %v, want FORMAT_TOO_NEW", aferrors.Code(err))
			}
		})
	}
}

// TestNewProofService_FormatGate verifies the service entry point refuses an
// unreadable workspace format and opens both readable formats.
func TestNewProofService_FormatGate(t *testing.T) {
	for _, version := range []string{"1.0", "1.1"} {
		t.Run(version, func(t *testing.T) {
			dir := t.TempDir()
			writeMetaVersion(t, dir, version)
			if _, err := NewProofService(dir); err != nil {
				t.Fatalf("NewProofService() with format %s = %v, want nil", version, err)
			}
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		dir := t.TempDir()
		writeMetaVersion(t, dir, "9.9")
		_, err := NewProofService(dir)
		if err == nil {
			t.Fatal("NewProofService() with format 9.9 = nil, want FORMAT_TOO_NEW")
		}
		if aferrors.Code(err) != aferrors.FORMAT_TOO_NEW {
			t.Errorf("code = %v, want FORMAT_TOO_NEW", aferrors.Code(err))
		}
	})
}

func writeMetaVersion(t *testing.T, dir, version string) {
	t.Helper()
	cfg := config.Default()
	cfg.Version = version
	path := filepath.Join(dir, "meta.json")
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("meta.json missing: %v", err)
	}
}
