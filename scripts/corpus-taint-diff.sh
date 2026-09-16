#!/usr/bin/env bash
#
# corpus-taint-diff.sh - diff taint_state for every corpus workspace between
# two af binaries.
#
# D6 changes what taint a node derives (reference and validation dependencies
# now carry taint exactly like children). This script runs the previous binary
# and the new binary over the AISM corpus and prints, per workspace, how many
# nodes changed taint and the old->new pairs, so the release note can name the
# only export fields that may differ: taint_state and validation.taint_counts.
#
# Usage:
#   scripts/corpus-taint-diff.sh <old-af> <new-af>
#
# Optional environment:
#   AF_CORPUS_DIR       corpus root   (default ../almost-idempotent-stochastic-maps/proofs)
#   AF_CORPUS_MANIFEST  manifest      (default docs/corpus-manifest.txt)
#   TMPDIR              scratch dir
#
# Output: one line per workspace
#   <workspace>: changed=<n> [<old>-><new>=<count> ...]
#   <workspace>.detail: <node>:<old>-><new> ...
# plus an aggregate summary on stderr.

set -euo pipefail
cd "$(dirname "$0")/.."

OLD="${1:?usage: corpus-taint-diff.sh <old-af> <new-af>}"
NEW="${2:?usage: corpus-taint-diff.sh <old-af> <new-af>}"
ROOT="${AF_CORPUS_DIR:-../almost-idempotent-stochastic-maps/proofs}"
MANIFEST="${AF_CORPUS_MANIFEST:-docs/corpus-manifest.txt}"

for bin in "$OLD" "$NEW"; do
  [[ -x "$bin" ]] || { echo "corpus-taint-diff.sh: not executable: $bin" >&2; exit 2; }
done
[[ -f "$MANIFEST" ]] || { echo "corpus-taint-diff.sh: manifest not found: $MANIFEST" >&2; exit 2; }
[[ -d "$ROOT" ]] || { echo "corpus-taint-diff.sh: corpus root not found: $ROOT" >&2; exit 2; }

command -v jq >/dev/null || { echo "corpus-taint-diff.sh: jq is required" >&2; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

changed_ws=0
changed_nodes=0
total_ws=0
while IFS=' ' read -r rel _hash _events; do
  [[ -z "$rel" || "$rel" == \#* ]] && continue
  ws="$ROOT/$rel"
  total_ws=$((total_ws + 1))

  if ! "$OLD" export --graph json --dir "$ws" >"$work/old.json" 2>"$work/old.err"; then
    echo "FAIL old export $rel: $(head -1 "$work/old.err" 2>/dev/null)" >&2
    continue
  fi
  if ! "$NEW" export --graph json --dir "$ws" >"$work/new.json" 2>"$work/new.err"; then
    echo "FAIL new export $rel: $(head -1 "$work/new.err" 2>/dev/null)" >&2
    continue
  fi

  summary="$(jq -r -s '
    (.[0].nodes | map({key:.id, value:.taint_state}) | from_entries) as $o
    | (.[1].nodes | map({key:.id, value:.taint_state}) | from_entries) as $n
    | [ $n | to_entries[] | select($o[.key] != .value)
        | {id:.key, old:($o[.key] // "<absent>"), new:.value} ]
    | if length == 0 then "changed=0"
      else
        "changed=\(length) "
        + (group_by("\(.old)->\(.new)")
            | map("\(.[0].old)->\(.[0].new)=\(length)")
            | join(" "))
      end
  ' "$work/old.json" "$work/new.json")"

  n="$(printf '%s' "$summary" | sed -n 's/.*changed=\([0-9]*\).*/\1/p')"
  if [[ "${n:-0}" -gt 0 ]]; then
    changed_ws=$((changed_ws + 1))
    changed_nodes=$((changed_nodes + n))
    echo "$rel: $summary"
    jq -r -s '
      (.[0].nodes | map({key:.id, value:.taint_state}) | from_entries) as $o
      | (.[1].nodes | map({key:.id, value:.taint_state}) | from_entries) as $n
      | [ $n | to_entries[] | select($o[.key] != .value)
          | "\(.key):\($o[.key] // "<absent>")->\(.value)" ]
      | join("\n")
    ' "$work/old.json" "$work/new.json" | sed "s/^/  $rel.detail /"
  else
    echo "$rel: $summary"
  fi
done < "$MANIFEST"

echo "corpus-taint-diff: $changed_ws/$total_ws workspaces changed, $changed_nodes nodes changed" >&2
