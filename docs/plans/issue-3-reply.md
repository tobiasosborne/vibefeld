# Draft reply to GitHub #3 (for Tobias to post)

Thanks for the detailed report, and for running af at this scale; 123 nodes
and 291 challenges is well past what it was exercised at.

Decision: **option A**. Edge correction is going in on the 0.1.x line, as
`af amend-deps`, in release 0.1.9. Design, in short:

- `af amend-deps <node> [--add ids] [--remove ids] [--add-validated ids]
  [--remove-validated ids] --owner <agent> --reason "<text>"` works on
  `pending`, `draft` and `needs_refinement` nodes. On a `validated` node it
  is refused unless you pass `--reopen`, which returns the node to
  `pending` and replaces the edges in **one ledger event**, so there is
  never a window where the node is pending with its old edges, and the
  verifier who re-accepts sees exactly the corrected dependency set.
- Corrections are append-only events (`node_deps_amended`) with previous
  and new lists, owner and reason; replay verifies the previous lists
  against the state it holds. Nothing is rewritten in place. The node's
  content hash changes, so any verdict or challenge authored against the
  old hash is rejected and has to be regenerated after review.
- Every creation and amendment path gets a proper cycle check over the
  real dependency relation (child-of and depends-on), from the node's own
  position, with the prospective edges overlaid; today `refine` checks
  from the parent's position and bulk paths do not check at all. Scope
  leaks into a `local_assume` block are rejected too.
- For your 21 nodes there is a manifest form: `af amend-deps --file
  manifest.json --dry-run` prints the exact edge diff and any cycle or
  scope rejection per item; the real run applies items in order, reports
  every item exactly once, and is resumable after a crash. It ends with a
  re-verification work list you can feed to `af verdicts apply`.
- Before this lands, the commit path gets fixed so that a verdict's
  expected-hash check and its accept run on the same state read, and
  multi-event operations are sequence-checked as a whole. The workspace
  format goes to 1.1 with an explicit `af workspace upgrade`; a 0.1.8
  binary pointed at an upgraded workspace fails on the first unknown
  event, so stop old workers first.

The full plan is `docs/plans/scale-hardening.md` in the repo. If you can
share a copy of the ledger directory (it is just JSON event files), I
would use it as the acceptance test for the migration: dry-run, real run,
kill-and-resume, re-verify, audit. And if you would like to review the
`amend-deps` spec or contribute the tests-first patch, the door is open.
