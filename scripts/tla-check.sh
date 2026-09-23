#!/usr/bin/env bash
#
# tla-check.sh - model-check the TLA+ specs in specs/tla and compare each
# model's outcome with the one it declares.
#
# Every specs/tla/models/*.cfg starts with a line
#
#   \* expect: pass
#   \* expect: violates <InvariantName>
#
# and the run fails if TLC's result differs (a pass that should violate, a
# violation of the wrong invariant, a deadlock, or a TLC error).
#
# Usage:
#   scripts/tla-check.sh [-v] [MODEL ...]   # MODEL = cfg basename; default all
#
#   -v   print TLC's full output (including counterexample traces)
#
# The TLA+ tools jar is taken from $TLA2TOOLS_JAR, else
# ~/.local/share/tla/tla2tools.jar, else downloaded once (pinned version,
# checksum verified) into ${XDG_CACHE_HOME:-~/.cache}/vibefeld/. Needs java.

set -euo pipefail
cd "$(dirname "$0")/.."

TLA_VERSION="v1.7.4"
TLA_SHA256="936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88"
SPEC_DIR="specs/tla"
SPEC="LedgerCommit.tla"

verbose=0
if [[ "${1:-}" == "-v" ]]; then
  verbose=1
  shift
fi

if ! command -v java >/dev/null 2>&1; then
  echo "tla-check.sh: java not found" >&2
  exit 2
fi

jar="${TLA2TOOLS_JAR:-}"
if [[ -z "$jar" && -f "$HOME/.local/share/tla/tla2tools.jar" ]]; then
  jar="$HOME/.local/share/tla/tla2tools.jar"
fi
if [[ -z "$jar" ]]; then
  cache="${XDG_CACHE_HOME:-$HOME/.cache}/vibefeld"
  jar="$cache/tla2tools-$TLA_VERSION.jar"
  if [[ ! -f "$jar" ]]; then
    mkdir -p "$cache"
    echo "tla-check.sh: downloading tla2tools.jar $TLA_VERSION" >&2
    curl -sSfL -o "$jar.tmp" \
      "https://github.com/tlaplus/tlaplus/releases/download/$TLA_VERSION/tla2tools.jar"
    if ! echo "$TLA_SHA256  $jar.tmp" | sha256sum -c --quiet -; then
      rm -f "$jar.tmp"
      echo "tla-check.sh: checksum mismatch for tla2tools.jar" >&2
      exit 2
    fi
    mv "$jar.tmp" "$jar"
  fi
fi

if [[ $# -gt 0 ]]; then
  models=()
  for m in "$@"; do models+=("$SPEC_DIR/models/${m%.cfg}.cfg"); done
else
  models=("$SPEC_DIR"/models/*.cfg)
fi

meta="$(mktemp -d)"
trap 'rm -rf "$meta"' EXIT

failures=0
for cfg in "${models[@]}"; do
  name="$(basename "$cfg" .cfg)"
  expect="$(sed -n '1s/^\\\* expect: //p' "$cfg")"
  if [[ -z "$expect" ]]; then
    echo "FAIL $name: first line must be '\\* expect: pass|violates <Inv>'"
    failures=$((failures + 1))
    continue
  fi

  out="$meta/$name.out"
  start=$SECONDS
  (cd "$SPEC_DIR" && java -XX:+UseParallelGC -cp "$jar" tlc2.TLC \
      -workers auto -metadir "$meta/$name" -config "models/$name.cfg" "$SPEC") \
      >"$out" 2>&1 || true
  secs=$((SECONDS - start))

  if grep -q "Model checking completed. No error has been found." "$out"; then
    got="pass"
  elif inv="$(sed -n 's/^Error: Invariant \([A-Za-z0-9_]*\) is violated.*/\1/p' "$out" | head -1)" && [[ -n "$inv" ]]; then
    got="violates $inv"
  elif grep -q "Error: Deadlock reached" "$out"; then
    got="deadlock"
  else
    got="tlc-error"
  fi
  states="$(sed -n 's/.* \([0-9][0-9]*\) distinct states found.*/\1/p' "$out" | tail -1)"

  if [[ "$got" == "$expect" ]]; then
    printf 'ok   %-32s %-34s %8s states %4ss\n' "$name" "$got" "${states:-?}" "$secs"
  else
    printf 'FAIL %-32s expected %-25s got %s\n' "$name" "'$expect'" "$got"
    failures=$((failures + 1))
  fi
  if [[ $verbose -eq 1 || ( "$got" != "$expect" && "$got" == "tlc-error" ) ]]; then
    cat "$out"
  fi
done

if [[ $failures -gt 0 ]]; then
  echo "tla-check.sh: $failures model(s) did not match their expectation" >&2
  exit 1
fi
echo "tla-check.sh: all ${#models[@]} models match their expectation"
