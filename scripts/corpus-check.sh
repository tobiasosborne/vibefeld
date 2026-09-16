#!/usr/bin/env bash
#
# corpus-check.sh - replay and export every corpus workspace in the manifest.
#
# For each line of the manifest, run:
#
#   <af> replay --verify --dir <workspace> -f json
#   <af> export --graph json --dir <workspace>
#   <af> audit -f json --dir <workspace>
#
# and fail on any non-zero exit, or when replay reports "valid": false. The
# audit is non-strict: findings are expected on historical corpora and are not
# a failure, only an audit that errors is.
#
# Usage:
#   scripts/corpus-check.sh [AF_BINARY]     # AF_BINARY defaults to ./af
#
# The corpus root defaults to ../almost-idempotent-stochastic-maps/proofs and
# the manifest to docs/corpus-manifest.txt; override with AF_CORPUS_DIR and
# AF_CORPUS_MANIFEST.

set -euo pipefail
cd "$(dirname "$0")/.."

AF="${1:-./af}"
ROOT="${AF_CORPUS_DIR:-../almost-idempotent-stochastic-maps/proofs}"
MANIFEST="${AF_CORPUS_MANIFEST:-docs/corpus-manifest.txt}"

if [[ ! -x "$AF" ]]; then
  echo "corpus-check.sh: af binary not executable: $AF" >&2
  exit 2
fi
if [[ ! -f "$MANIFEST" ]]; then
  echo "corpus-check.sh: manifest not found: $MANIFEST" >&2
  exit 2
fi
if [[ ! -d "$ROOT" ]]; then
  echo "corpus-check.sh: corpus root not found: $ROOT (set AF_CORPUS_DIR)" >&2
  exit 2
fi

fail=0
count=0
while IFS=' ' read -r rel _hash _events; do
  [[ -z "$rel" || "$rel" == \#* ]] && continue
  ws="$ROOT/$rel"
  count=$((count + 1))

  if ! out="$("$AF" replay --verify --dir "$ws" -f json 2>&1)"; then
    echo "FAIL replay (exit) $rel" >&2
    echo "$out" >&2
    fail=1
    continue
  fi
  if printf '%s' "$out" | grep -q '"valid":[[:space:]]*false'; then
    echo "FAIL replay (valid:false) $rel" >&2
    echo "$out" >&2
    fail=1
    continue
  fi

  if ! "$AF" export --graph json --dir "$ws" >/dev/null 2>&1; then
    echo "FAIL export $rel" >&2
    fail=1
    continue
  fi

  if ! "$AF" audit -f json --dir "$ws" >/dev/null 2>&1; then
    echo "FAIL audit $rel" >&2
    fail=1
    continue
  fi
done < "$MANIFEST"

if [[ "$fail" -ne 0 ]]; then
  echo "corpus check FAILED" >&2
  exit 1
fi
echo "corpus check OK ($count workspaces)"
