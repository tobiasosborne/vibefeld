//go:build integration

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tobiasosborne/vibefeld/internal/fs"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
	"github.com/tobiasosborne/vibefeld/internal/schema"
	"github.com/tobiasosborne/vibefeld/internal/service"
	"github.com/tobiasosborne/vibefeld/internal/state"
	"github.com/tobiasosborne/vibefeld/internal/support"
	"github.com/tobiasosborne/vibefeld/internal/types"
)

// Scale-test knobs. The workspace is built through the Go service API, which is
// fast enough for 1000 nodes; scripts/synth-workspace.sh does the same through
// the CLI for when byte-level CLI coverage is wanted.
const (
	scaleNodes    = 1000
	scaleFanout   = 10
	scaleMaxDepth = 4
	scaleXrefPct  = 15
	scaleRounds   = 1

	// The e2e scale test keeps its verification rounds light: the tree and
	// cross-reference phases already take ~26 s for 1000 nodes, and the point
	// of this test is the invariants, not throughput. The shell generator runs
	// the full fractions.
	scaleChallengePct = 5
	scaleAmendPct     = 2
	scaleAcceptPct    = 10

	scaleAuthor   = "scale-author"
	scaleProver   = "scale-prover"
	scaleVerifier = "scale-verifier"
)

// scaleRNG is a tiny deterministic 31-bit LCG, the same sequence as
// scripts/synth-workspace.sh. Determinism matters so a failing run can be
// reproduced; af's own challenge IDs are random, so the ledgers are not
// byte-identical, but the shape and the decisions are.
type scaleRNG struct{ state uint64 }

func newScaleRNG(seed uint64) *scaleRNG {
	s := seed & 0x7fffffff
	if s == 0 {
		s = 1
	}
	return &scaleRNG{state: s}
}

func (r *scaleRNG) next() uint64 {
	r.state = (r.state*1103515245 + 12345) & 0x7fffffff
	return r.state
}

func (r *scaleRNG) below(n int) int { return int(r.next() % uint64(n)) }

func (r *scaleRNG) pct(p int) bool { return r.below(100) < p }

// buildScaleWorkspace builds a synthetic proof tree of roughly `nodes` nodes
// with deterministic fan-out and cross-references, then runs verification
// rounds over it. It returns every node ID in creation order.
func buildScaleWorkspace(t *testing.T, dir string, nodes int, seed uint64, rounds, challengePct, amendPct, acceptPct int) []types.NodeID {
	t.Helper()

	if err := fs.InitProofDir(dir); err != nil {
		t.Fatalf("InitProofDir: %v", err)
	}
	ta := time.Now()
	if err := service.Init(dir, "Synthetic scale workspace", scaleAuthor); err != nil {
		t.Fatalf("service.Init: %v", err)
	}
	svc, err := service.NewProofService(dir)
	if err != nil {
		t.Fatalf("NewProofService: %v", err)
	}

	root, err := types.Parse("1")
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	all := []types.NodeID{root}
	frontier := []types.NodeID{root}
	count := 1
	depth := 0

	for count < nodes && depth < scaleMaxDepth {
		var next []types.NodeID
		for _, parent := range frontier {
			if count >= nodes {
				break
			}
			remaining := nodes - count
			kids := scaleFanout
			if kids > remaining {
				kids = remaining
			}

			specs := make([]service.ChildSpec, kids)
			for i := 0; i < kids; i++ {
				childID, perr := types.Parse(fmt.Sprintf("%s.%d", parent.String(), i+1))
				if perr != nil {
					t.Fatalf("parse child: %v", perr)
				}
				specs[i] = service.ChildSpec{
					NodeType:  schema.NodeTypeClaim,
					Statement: fmt.Sprintf("Synthetic step %s", childID),
					Inference: schema.InferenceAssumption,
				}
			}

			if err := svc.ClaimNode(parent, scaleProver, 30*time.Minute); err != nil {
				t.Fatalf("ClaimNode(%s): %v", parent, err)
			}
			ids, err := svc.RefineNodeBulk(parent, scaleProver, specs)
			if err != nil {
				t.Fatalf("RefineNodeBulk(%s): %v", parent, err)
			}
			if err := svc.ReleaseNode(parent, scaleProver); err != nil {
				t.Fatalf("ReleaseNode(%s): %v", parent, err)
			}

			all = append(all, ids...)
			next = append(next, ids...)
			count += kids
		}
		frontier = next
		depth++
	}
	if len(all) != nodes {
		t.Fatalf("built %d nodes, wanted %d", len(all), nodes)
	}
	t.Logf("scale: tree phase: %s (%d nodes)", time.Since(ta), len(all))

	// Cross-references point from a node to a later-created node, the same
	// direction as every tree edge, so the result-use graph stays acyclic.
	tb := time.Now()
	rng := newScaleRNG(seed)
	for i := 0; i < len(all); i++ {
		if !rng.pct(scaleXrefPct) {
			continue
		}
		span := len(all) - i - 1
		if span < 1 {
			continue
		}
		j := i + 1 + rng.below(span)
		_, err := svc.AmendDeps(all[i], service.AmendDepsRequest{
			Add:    []types.NodeID{all[j]},
			Owner:  scaleProver,
			Reason: "synthetic cross-reference",
		})
		if err != nil {
			// A rejected edge (e.g. scope) is fine; the workload does not
			// depend on any particular cross-reference landing.
			continue
		}
	}
	t.Logf("scale: cross-reference phase: %s", time.Since(tb))

	// Verification rounds: challenges, resolutions, amendments, accepts.
	tr := time.Now()
	for round := 1; round <= rounds; round++ {
		var cids []string
		for i, node := range all {
			if !rng.pct(challengePct) {
				continue
			}
			cid := fmt.Sprintf("ch-scale-%d-%d", round, i)
			err := svc.RaiseChallengeWithBatch(node, cid, "statement",
				fmt.Sprintf("synthetic objection, round %d", round), "major",
				scaleVerifier, "", "")
			if err == nil {
				cids = append(cids, cid)
			}
		}
		for _, cid := range cids {
			_ = svc.ResolveChallenge(cid)
		}
		for _, node := range all {
			if !rng.pct(amendPct) {
				continue
			}
			_ = svc.AmendNode(node, scaleProver,
				fmt.Sprintf("Amended %s (round %d)", node, round))
		}
		// Deepest-first so children clear before their parents.
		for k := len(all) - 1; k >= 0; k-- {
			if !rng.pct(acceptPct) {
				continue
			}
			_ = svc.AcceptNode(all[k])
		}
	}
	t.Logf("scale: verification phase: %s", time.Since(tr))

	return all
}

var (
	scaleBinOnce sync.Once
	scaleBinPath string
	scaleBinErr  error
)

// scaleAFBinary returns the path to an af binary, reusing ../af when it exists
// and otherwise building one once for the whole package.
func scaleAFBinary(t *testing.T) string {
	t.Helper()
	scaleBinOnce.Do(func() {
		if env := os.Getenv("AF_BINARY"); env != "" {
			scaleBinPath = env
			return
		}
		if abs, err := filepath.Abs("../af"); err == nil {
			if info, err := os.Stat(abs); err == nil && !info.IsDir() {
				scaleBinPath = abs
				return
			}
		}
		out, err := os.CreateTemp("", "af-scale-*")
		if err != nil {
			scaleBinErr = err
			return
		}
		path := out.Name()
		_ = out.Close()
		cmd := exec.Command("go", "build", "-o", path, "github.com/tobiasosborne/vibefeld/cmd/af")
		if buildOut, err := cmd.CombinedOutput(); err != nil {
			scaleBinErr = fmt.Errorf("go build af: %v\n%s", err, buildOut)
			return
		}
		scaleBinPath = path
	})
	if scaleBinErr != nil {
		t.Fatalf("locating af binary: %v", scaleBinErr)
	}
	return scaleBinPath
}

// TestScale_SyntheticWorkspace1000 builds a 1000-node workspace through the Go
// service API and asserts invariants only. It makes no wall-time assertions:
// the point is that the workspace is replayable, contiguous, auditable and
// support-current-computable at this size.
func TestScale_SyntheticWorkspace1000(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test skipped in -short mode")
	}

	dir := t.TempDir()
	all := buildScaleWorkspace(t, dir, scaleNodes, 1, scaleRounds, scaleChallengePct, scaleAmendPct, scaleAcceptPct)

	ledgerDir := filepath.Join(dir, "ledger")

	// 1. Sequence numbers are contiguous, with no gaps.
	hasGaps, err := ledger.HasGaps(ledgerDir)
	if err != nil {
		t.Fatalf("HasGaps: %v", err)
	}
	if hasGaps {
		t.Fatal("ledger has sequence gaps")
	}
	count, err := ledger.Count(ledgerDir)
	if err != nil {
		t.Fatalf("ledger.Count: %v", err)
	}
	if count < scaleNodes {
		t.Fatalf("ledger has %d events, want at least one per node (%d)", count, scaleNodes)
	}

	// 2. Replay --verify is valid. Check the in-process path first, then the
	// real CLI so the exit code is exercised too.
	ldg, err := ledger.NewLedger(ledgerDir)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	st, err := state.ReplayWithVerify(ldg)
	if err != nil {
		t.Fatalf("ReplayWithVerify: %v", err)
	}

	bin := scaleAFBinary(t)
	replayOut, err := exec.Command(bin, "replay", "--verify", "-d", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("af replay --verify failed: %v\n%s", err, replayOut)
	}

	// 3. af audit non-strict runs without error (it never gates in that mode).
	auditOut, err := exec.Command(bin, "audit", "-d", dir, "-f", "json").CombinedOutput()
	if err != nil {
		t.Fatalf("af audit (non-strict) failed: %v\n%s", err, auditOut)
	}

	// 4. support.Current computes for every node.
	sup := support.Current(st)
	if len(sup) != len(all) {
		t.Errorf("support.Current has %d entries, want %d", len(sup), len(all))
	}
	for _, n := range st.AllNodes() {
		if _, ok := sup[n.ID.String()]; !ok {
			t.Errorf("support.Current missing node %s", n.ID)
		}
	}
}
