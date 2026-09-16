#!/usr/bin/env bash
#
# corpus-manifest.sh - print or check a manifest of corpus workspaces.
#
# For every workspace directory under the corpus root that contains a ledger/,
# print one line:
#
#   <relative-path> <sha256> <event-count>
#
# where <sha256> is the SHA-256 of the raw bytes of ledger/*.json concatenated
# in filename order, and <event-count> is the number of those files. Any change
# to any event file, or to the set of files, changes the digest.
#
# Usage:
#   scripts/corpus-manifest.sh                 # print the manifest to stdout
#   scripts/corpus-manifest.sh --check FILE    # compare, exit 1 on difference
#
# The corpus root defaults to ../almost-idempotent-stochastic-maps/proofs and
# can be overridden with AF_CORPUS_DIR.

set -euo pipefail
cd "$(dirname "$0")/.."

ROOT="${AF_CORPUS_DIR:-../almost-idempotent-stochastic-maps/proofs}"

if [[ ! -d "$ROOT" ]]; then
  echo "corpus-manifest.sh: corpus root not found: $ROOT (set AF_CORPUS_DIR)" >&2
  exit 2
fi

# Print the manifest for the current corpus.
generate() {
  find "$ROOT" -type d -name ledger -print | LC_ALL=C sort | while IFS= read -r led; do
    ws="$(dirname "$led")"
    rel="${ws#"$ROOT"/}"

    # Sort the event files by name; numeric zero-padded names make this the
    # event sequence order. NUL-delimited to survive exotic filenames.
    files="$(find "$led" -maxdepth 1 -type f -name '*.json' | LC_ALL=C sort)"
    if [[ -z "$files" ]]; then
      count=0
      hash="$(printf '' | sha256sum | cut -d' ' -f1)"
    else
      count="$(printf '%s\n' "$files" | wc -l | tr -d ' ')"
      hash="$(printf '%s\n' "$files" | tr '\n' '\0' | xargs -0 cat | sha256sum | cut -d' ' -f1)"
    fi

    printf '%s %s %s\n' "$rel" "$hash" "$count"
  done
}

case "${1:-}" in
  --check)
    manifest="${2:-}"
    if [[ -z "$manifest" || ! -f "$manifest" ]]; then
      echo "usage: $0 --check <manifest>" >&2
      exit 2
    fi
    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' EXIT
    generate > "$tmp"
    if diff -u "$manifest" "$tmp"; then
      echo "corpus manifest OK ($(wc -l < "$tmp" | tr -d ' ') workspaces)"
      exit 0
    fi
    echo "corpus manifest MISMATCH" >&2
    exit 1
    ;;
  --help | -h)
    sed -n '2,18p' "$0"
    exit 0
    ;;
  "")
    generate
    ;;
  *)
    echo "usage: $0 [--check <manifest>]" >&2
    exit 2
    ;;
esac
