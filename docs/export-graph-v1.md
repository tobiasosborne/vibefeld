# Graph export schema (`af export --graph json`), version 1

`af export --graph json` is a read-only, deterministic projection of the
current proof state to a single JSON document. It exists for external
tooling that consumes af as a store — in particular rk's projection layer
(PRD C5), which joins its registry to af by contract byte-match, located via
the shard's `workspace:` field.

This document never mutates the ledger. It ignores `--format`; `--format`
and `--graph` are independent flags on `af export`.

## Top-level fields

| Field | Type | Description |
|---|---|---|
| `schema_version` | string | Currently `"1"`. Consumers must check this before parsing the rest of the document; a version bump accompanies any incompatible shape change, per a new fixture in `internal/export/graph_test.go`. |
| `workspace` | object | See [Workspace](#workspace). |
| `nodes` | array of object | Every node in the proof tree. See [Node](#node). Ordered by hierarchical ID (`1`, `1.1`, `1.2`, `1.1.1`, ...), never map/ledger-append order. |
| `validation` | object | Cheap validation summary. See [Validation](#validation). |

## Workspace

| Field | Type | Description |
|---|---|---|
| `id` | string | Identifies the workspace: the resolved (absolute) proof directory path passed via `--dir`. This is the value rk's registry locates via its shard's `workspace:` field before matching contract text against node statements in this document. Note: **workspace directories can be renamed** (legal in af); a shard's `workspace:` field is the source of truth for that mapping, not any id stored inside af itself — af records no separate, rename-stable workspace identifier. |
| `title` | string, optional | The proof's configured title (`meta.json` `title`), if any. |
| `conjecture` | string, optional | The proof's configured top-level conjecture (`meta.json` `conjecture`), if any. Distinct from, and not guaranteed byte-identical to, the root node's `statement` (the root statement can be amended after init; `conjecture` in `meta.json` is not). Consumers doing contract byte-match should match against `nodes[].statement`, not `workspace.conjecture`. |

## Node

One entry per node in the proof tree, `nodes[]`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Hierarchical node ID (e.g. `1`, `1.2`, `1.2.3`). |
| `type` | string | Node type: `claim`, `local_assume`, `local_discharge`, `case`, ... (see `af types`). |
| `parent_id` | string, omitted for root | The parent node's `id`. Redundant with the hierarchical `id` scheme but provided explicitly so consumers don't need to reimplement dotted-ID parsing. |
| `child_ids` | array of string, omitted if none | This node's direct children's `id`s, in hierarchical-ID order. |
| `statement` | string | The node's exact statement text — the contract byte-match target for rk's registry↔af join. Never reformatted, escaped for display, or truncated. |
| `latex` | string, omitted if empty | Optional LaTeX form of the statement, if the node has one. |
| `inference` | string | The inference rule justifying this node (see `af inferences`). |
| `content_hash` | string | The node's own SHA256 content hash (over type/statement/latex/inference/context/dependencies), computed by af independently of this export — not a hash of this JSON document. |
| `workflow_state` | string | One of the three recorded state axes: `available`, `claimed`, `blocked`. |
| `epistemic_state` | string | One of the three recorded state axes: `pending`, `validated`, `admitted`, `refuted`, `archived`, `needs_refinement`, `draft`. |
| `taint_state` | string | One of the three recorded state axes: `clean`, `self_admitted`, `tainted`, `unresolved`. |
| `crux` | bool, omitted if false | True if the node is marked critical-path (requires a passing claim-test before acceptance; relevant to rk's batch-composer critical-path exclusion rule, PRD C9). |
| `created` | string | ISO8601 timestamp of when the node was **created in the ledger** — historical, content-bearing data, not an export-time timestamp. Reproduces byte-identically across repeated exports of unchanged state. |
| `author` | string, omitted if empty | The driver-supplied identity of the agent that authored this node's content (rk PRD C3's author-identity kernel surface, item V1). Added additively after v1 shipped — see the "Additive fields" note below; never populated for nodes created before the field existed. |
| `validated_by` | string, omitted if empty | The driver-supplied identity of the verifier who validated this node, if any. Same additive-field note applies. |
| `validation_batch_id` | string, omitted if empty | The batch id recorded when this node was validated as part of a batch (`af verdicts apply`, item V2 — not yet implemented); omitted for singly-validated nodes. |
| `prover_ready` | bool, omitted if false | True iff af's OWN job detection (`internal/jobs.FindProverJobs`, the same classifier `af jobs --role prover` uses) marks this node a prover job: not blocked, and either pending with an open blocking (critical/major) challenge, or in draft/needs_refinement. Lets an external driver read af's job classification instead of re-deriving af's state machine from the raw axes. Additive field (see below). |
| `verifier_ready` | bool, omitted if false | True iff af's OWN job detection marks this node ready for verifier review AND all its children are epistemically cleared — exactly `internal/jobs.FilterReadyVerifierJobs` (`af jobs --role verifier --ready`): a statement, pending, available (not claimed/blocked), no open blocking challenge, every child validated/admitted/archived. A driver dispatches a verifier turn on these and only these (bottom-up-ready). Note this is af's authoritative classifier; the `af status` "Prover/Verifier jobs" summary line uses a separate, cruder workflow-only heuristic (`internal/render/status.go`) that can disagree — external drivers must read these export flags, not parse `af status`. Additive field. |

Fields deliberately **not** included in v1: `claimed_by`/`claimed_at` (ephemeral
workflow metadata, not identity-bearing contract content), `context`,
`dependencies`, `validation_deps`, `scope` (cross-reference metadata not
requested by the C5 join; may be added in a future schema version without
breaking v1 consumers, since new optional fields are additive).

### Additive fields (author/validated_by/validation_batch_id)

`author`, `validated_by`, and `validation_batch_id` were added to `nodes[]`
after v1 first shipped, under the additive-fields rule stated above: they
are optional (`omitempty`), no existing field changed shape or meaning, and
`schema_version` stayed `"1"`. Consumers written against the original v1
table should simply ignore keys they don't recognize. A proof replayed from
a ledger that predates author/verifier-identity provenance omits all three
fields for every node, so re-exporting an old, unmodified proof is still
byte-identical to before this addition.

`prover_ready` and `verifier_ready` were added later under the same rule:
both are `omitempty` bools computed from already-derived state (af's own
`internal/jobs` classifier), no existing field changed, and `schema_version`
stayed `"1"`. They are derived — not stored in the ledger — so they never
affect replay byte-identity of node content; a node needing neither prover
nor verifier work omits both keys.

## Validation

Cheap, already-computed summary — no re-walk of the raw ledger.

| Field | Type | Description |
|---|---|---|
| `total_nodes` | int | Count of `nodes[]`. |
| `epistemic_counts` | object of string→int | Node count per `epistemic_state` value present in the proof. |
| `taint_counts` | object of string→int | Node count per `taint_state` value present in the proof. |
| `total_challenges` | int | Count of all challenges recorded against any node, regardless of status. |
| `challenge_status_counts` | object of string→int | Challenge count per status (`open`, `resolved`, `withdrawn`, `superseded`). |

## Determinism

Two exports of an unchanged proof are byte-identical. This rests on three
properties, each covered by a test in `internal/export/graph_test.go` and
`cmd/af/export_graph_test.go`:

1. **Stable node order.** Nodes are always emitted in hierarchical-ID order
   (the same ordering `af export --format markdown` uses), never Go
   map-iteration order.
2. **Stable key order.** Object fields either come from a fixed Go struct
   field order, or (for the three count maps under `validation`) from Go's
   `encoding/json`, which always sorts `map[string]...` keys alphabetically
   on marshal.
3. **No export-time timestamp.** The only timestamp field, `created`, is
   ledger-recorded node-creation time — content, not export metadata. A
   `generated_at`-style field was deliberately **not** added; if one is
   needed in a future version it must live outside identity-bearing content
   (e.g. as an HTTP-style side channel, not a top-level JSON field mixed into
   `nodes[]` or `workspace`).

## Example

```json
{"schema_version":"1","workspace":{"id":"/abs/path/to/proof","title":"sqrt(2) is irrational","conjecture":"sqrt(2) is irrational"},"nodes":[{"id":"1","type":"claim","child_ids":["1.1"],"statement":"sqrt(2) is irrational","inference":"assumption","content_hash":"...","workflow_state":"claimed","epistemic_state":"pending","taint_state":"unresolved","created":"2026-07-19T00:00:00Z"},{"id":"1.1","type":"claim","parent_id":"1","statement":"Suppose sqrt(2) = p/q in lowest terms","inference":"assumption","content_hash":"...","workflow_state":"available","epistemic_state":"pending","taint_state":"unresolved","created":"2026-07-19T00:00:01Z"}],"validation":{"total_nodes":2,"epistemic_counts":{"pending":2},"taint_counts":{"unresolved":2},"total_challenges":0,"challenge_status_counts":{}}}
```

(af emits this compact — single-line, no indentation — matching the
convention already used by `af status --format json`; re-indent with any
JSON pretty-printer for reading.)

## See also

- `docs/cli-reference.md` — `export` command flags and examples.
- `internal/export/graph.go` — implementation.
- `internal/export/graph_test.go`, `cmd/af/export_graph_test.go` — the red
  corpus this schema is tested against.
