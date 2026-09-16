package audit

import (
	"fmt"
	"sort"

	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/taint"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Acceptance is the sequence-sensitive fact recorded when a node's verdict
// (validated or admitted) was applied: the content hash of the revision that
// was accepted, and the identities that had contributed to that revision up to
// that ledger sequence.
type Acceptance struct {
	// Seq is the ledger sequence of the NodeValidated/NodeAdmitted event.
	Seq int
	// ContentHash is n.ComputeContentHash() on the state as of Seq.
	ContentHash string
	// Contributors is the deduplicated set of identities that had touched the
	// node's revision by Seq: its author, its proof author at that time, and
	// the owners of every statement/dependency amendment applied so far.
	Contributors []string
	// VerifiedBy is the identity recorded on the verdict event.
	VerifiedBy string
}

// Archival is the sequence-sensitive fact recorded when NodeArchived was
// applied: the obligation nodes whose open challenges were abandoned.
type Archival struct {
	// Seq is the ledger sequence of the NodeArchived event.
	Seq int
	// Abandoned is the event's durable abandoned_obligations snapshot (D9), if
	// the event carried one.
	Abandoned []types.NodeID
	// OpenAtArchive is the node itself and every active (non-severed)
	// descendant that had an open challenge at the archival sequence.
	OpenAtArchive []types.NodeID
}

// Pass is the ordered timeline of sequence-sensitive facts gathered by one
// event-by-event replay of a ledger. It exists so the audit can answer
// "what did the verifier accept, and who had contributed to it then?" and
// "what was abandoned when this branch was archived?" without consulting
// final state (which a later event may have rewritten). It is read-only and
// immutable once built.
type Pass struct {
	acceptances map[string]Acceptance
	archivals   map[string]Archival
}

func newPass() *Pass {
	return &Pass{
		acceptances: make(map[string]Acceptance),
		archivals:   make(map[string]Archival),
	}
}

// Acceptance returns the recorded acceptance for a node, if any.
func (p *Pass) Acceptance(id types.NodeID) (Acceptance, bool) {
	if p == nil {
		return Acceptance{}, false
	}
	a, ok := p.acceptances[id.String()]
	return a, ok
}

// Archival returns the recorded archival for a node, if any.
func (p *Pass) Archival(id types.NodeID) (Archival, bool) {
	if p == nil {
		return Archival{}, false
	}
	a, ok := p.archivals[id.String()]
	return a, ok
}

// replayCount counts BuildPass calls. It is a test hook proving that the audit
// engine itself never replays the ledger (the caller does it once).
var replayCount int

// ReplayCount returns how many times BuildPass has run in this process.
func ReplayCount() int { return replayCount }

// BuildPass replays ldg event by event into a fresh state.State (through
// state.Apply, mirroring state.Replay) and records, at each NodeValidated /
// NodeAdmitted, the accepted content hash and the contributors at that
// sequence, and at each NodeArchived the open-challenge obligations at that
// sequence. It returns the final state and the pass. It replays once; the
// returned state is not loaded with on-disk assumptions or externals (the
// service does that).
func BuildPass(ldg *ledger.Ledger) (*state.State, *Pass, error) {
	if ldg == nil {
		return nil, nil, fmt.Errorf("cannot build audit pass from nil ledger")
	}
	replayCount++

	st := state.NewState()
	pass := newPass()
	amendOwners := make(map[string][]string)

	expectedSeq := 1
	err := ldg.Scan(func(seq int, data []byte) error {
		if seq != expectedSeq {
			if seq < expectedSeq {
				return fmt.Errorf("duplicate sequence number detected: got %d, expected %d", seq, expectedSeq)
			}
			return fmt.Errorf("sequence gap detected: got %d, expected %d", seq, expectedSeq)
		}
		expectedSeq++

		event, err := state.ParseEvent(data)
		if err != nil {
			return fmt.Errorf("failed to parse event %d: %w", seq, err)
		}

		// Observe before Apply: NodeArchived auto-supersedes challenges on the
		// archived node, so the honest "open at archival" set is the pre-apply
		// one; and a verdict's acceptance-time content/contributors are the
		// pre-apply revision.
		observeForPass(pass, st, amendOwners, seq, event)

		if err := state.Apply(st, event); err != nil {
			return fmt.Errorf("failed to apply event %d (%s): %w", seq, event.Type(), err)
		}
		st.SetLatestSeq(seq)
		state.StampDerived(st, event, seq)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	taint.RecomputeAll(st.AllNodes())
	return st, pass, nil
}

// observeForPass records the sequence-sensitive facts for one pre-apply event.
func observeForPass(pass *Pass, st *state.State, amendOwners map[string][]string, seq int, event ledger.Event) {
	switch ev := event.(type) {
	case ledger.NodeAmended:
		key := ev.NodeID.String()
		amendOwners[key] = append(amendOwners[key], ev.Owner)
	case ledger.NodeAmendedReopened:
		key := ev.NodeID.String()
		amendOwners[key] = append(amendOwners[key], ev.Owner)
	case ledger.NodeDepsAmended:
		key := ev.NodeID.String()
		amendOwners[key] = append(amendOwners[key], ev.Owner)
	case ledger.NodeValidated:
		n := st.GetNode(ev.NodeID)
		if n == nil {
			return
		}
		pass.acceptances[ev.NodeID.String()] = Acceptance{
			Seq:          seq,
			ContentHash:  n.ComputeContentHash(),
			Contributors: contributorsAt(n.Author, n.ProofAuthor, amendOwners[ev.NodeID.String()]),
			VerifiedBy:   ev.VerifiedBy,
		}
	case ledger.NodeAdmitted:
		n := st.GetNode(ev.NodeID)
		if n == nil {
			return
		}
		pass.acceptances[ev.NodeID.String()] = Acceptance{
			Seq:          seq,
			ContentHash:  n.ComputeContentHash(),
			Contributors: contributorsAt(n.Author, n.ProofAuthor, amendOwners[ev.NodeID.String()]),
			VerifiedBy:   ev.By,
		}
	case ledger.NodeArchived:
		pass.archivals[ev.NodeID.String()] = Archival{
			Seq:           seq,
			Abandoned:     parseNodeIDs(ev.AbandonedObligations),
			OpenAtArchive: st.OpenChallengeObligations(ev.NodeID),
		}
	}
}

// contributorsAt deduplicates and sorts the identities that contributed to a
// revision. author and proofAuthor may be empty; owners is the amendment-owner
// history in ledger order.
func contributorsAt(author, proofAuthor string, owners []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(author)
	add(proofAuthor)
	for _, o := range owners {
		add(o)
	}
	sort.Strings(out)
	return out
}

// parseNodeIDs parses raw node IDs, dropping unparseable entries.
func parseNodeIDs(raw []string) []types.NodeID {
	out := make([]types.NodeID, 0, len(raw))
	for _, s := range raw {
		if id, err := types.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}
