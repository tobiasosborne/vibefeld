// Package ledger provides event-sourced ledger operations for the AF proof framework.
package ledger

// Ledger provides a facade for ledger operations, combining append, read, and lock functionality.
// It provides a convenient way to work with a ledger directory.
type Ledger struct {
	dir string
}

// NewLedger creates a new Ledger instance for the given directory.
// Returns an error if the directory doesn't exist or is not a directory.
func NewLedger(dir string) (*Ledger, error) {
	if err := validateDirectory(dir); err != nil {
		return nil, err
	}

	return &Ledger{dir: dir}, nil
}

// Dir returns the directory path of the ledger.
func (l *Ledger) Dir() string {
	return l.dir
}

// Append adds an event to the ledger.
// Returns the sequence number assigned to the event.
// The write is atomic: the event is first written to a temp file, then renamed.
// Uses file-based locking to ensure concurrent safety.
func (l *Ledger) Append(event Event) (int, error) {
	return Append(l.dir, event)
}

// ReadAll reads all events from the ledger in sequence order.
// Returns a slice of raw JSON bytes for each event.
// Returns an empty slice if the ledger is empty (no event files).
func (l *Ledger) ReadAll() ([][]byte, error) {
	return ReadAll(l.dir)
}

// Scan iterates over all events in sequence order, calling fn for each.
// Scanning stops if fn returns an error.
// If fn returns ErrStopScan, Scan returns nil (clean stop).
// Any other error from fn is returned by Scan.
func (l *Ledger) Scan(fn ScanFunc) error {
	return Scan(l.dir, fn)
}

// Count returns the number of events in the ledger.
func (l *Ledger) Count() (int, error) {
	return Count(l.dir)
}

// AppendIfSequence adds an event to the ledger only if the current sequence
// matches the expected value. This implements Compare-And-Swap (CAS) semantics
// for optimistic concurrency control.
//
// expectedSeq should be the sequence number of the last event observed when
// the state was loaded (i.e., state.LatestSeq()). If the ledger has been
// modified since then, ErrSequenceMismatch is returned.
//
// Returns the new sequence number on success, or ErrSequenceMismatch if the
// ledger was concurrently modified. Other errors indicate infrastructure failures.
func (l *Ledger) AppendIfSequence(event Event, expectedSeq int) (int, error) {
	return AppendIfSequence(l.dir, event, expectedSeq)
}
