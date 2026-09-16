# Worked example: correcting a wrong dependency edge

This is the small end-to-end example for `af amend-deps` (plan item D2). It
builds a three-node proof, corrects one wrong edge, and re-verifies the reopened
node. The acceptance test for 0.1.9 grows this into the full correction
manifest.

## The proof

Initialize a workspace and build a root with two children:

```bash
af init --conjecture "A implies C by way of B" --author prover-1
af claim 1 --owner prover-1
af refine 1 "A holds" --owner prover-1 -j assumption               # creates 1.1
af refine 1 "A implies B" --owner prover-1 -j modus_ponens          # creates 1.2
af refine 1 "A implies C" --owner prover-1 -j modus_ponens --depends 1.2   # creates 1.3
```

Suppose node `1.3` was recorded as depending on `1.2` but should depend on both
`1.1` and `1.2` (it silently used `A` as well). This is a wrong-edge
correction.

## Dry run

A manifest lets the driver preview the exact edge diff and keep the generated
operation id for the real run:

```json
{
  "schema_version": 1,
  "items": [
    {
      "node": "1.3",
      "add": ["1.1"],
      "owner": "prover-1",
      "reason": "1.3 also cites A (1.1)"
    }
  ]
}
```

```bash
af amend-deps --file corrections.json --dry-run
```

The dry run writes no events. It prints the exact edge diff and the resulting
content hash, and it writes an `operation_id` back into `corrections.json` so the
real run (and any resume after a crash) recognises its own work.

## Apply

```bash
af amend-deps --file corrections.json -f json
```

`1.3` gains `1.1`, its content hash changes, and `af amendments 1.3` now lists a
dependency amendment alongside the original statement:

```bash
af deps 1.3          # edges touched by an amendment are marked with *
af diff 1.3          # includes the dependency change
af amendments 1.3    # lists both statement and dependency amendments
```

If `1.3` had already been accepted, the correction would have required
`--reopen`; the edge replacement and `validated -> pending` would be the same
`node_deps_amended` event, and the manifest output would end with a
re-verification work list.

## Re-verify

Every reopened node is pending with a new hash. Accept it through a verdict file
(authored against the new hash) and then re-check the graph:

```bash
cat > reverify.json <<'EOF'
{
  "schema_version": "1",
  "batch_id": "reverify-1",
  "verified_by": "verifier-1",
  "items": [
    {"node": "1.3", "verdict": "accept", "reason": "re-checked after edge correction"}
  ]
}
EOF

af verdicts apply reverify.json
af export --graph json > graph.after.json
```
