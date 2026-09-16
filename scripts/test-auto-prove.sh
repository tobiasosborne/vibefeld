#!/usr/bin/env bash
#
# test-auto-prove.sh - exercise scripts/auto-prove.sh against a stub agent.
#
# This is the regression test for bead ujp4 / plan item D11: the commands that
# auto-prove.sh generates for its agents must actually execute. It creates a
# disposable workspace, installs a stub `claude` that pulls the generated af
# commands out of each prompt and runs them, drives one or two iterations of
# auto-prove.sh, and fails if af reports an unknown flag.
#
# Usage:
#   scripts/test-auto-prove.sh [--af-binary PATH]
#
# If --af-binary is omitted the script builds ./cmd/af into a temp dir.
#
# Exit codes:
#   0  generated commands executed without unknown-flag errors
#   1  an unknown flag (or another explicit failure) was seen
#   2  bad usage
#   4  bash < 4 or the af build failed

set -uo pipefail

if (( BASH_VERSINFO[0] < 4 )); then
    echo "test-auto-prove.sh: bash >= 4 is required (this is bash $BASH_VERSION)." >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

AF_BIN=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --af-binary)
            AF_BIN="$2"
            shift 2
            ;;
        -h|--help)
            sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "test-auto-prove.sh: unknown option: $1" >&2
            exit 2
            ;;
    esac
done

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ -z "$AF_BIN" ]]; then
    AF_BIN="$TMP_DIR/af"
    if ! ( cd "$PROJECT_ROOT" && go build -o "$AF_BIN" ./cmd/af ); then
        echo "test-auto-prove.sh: failed to build af" >&2
        exit 4
    fi
fi
if [[ ! -x "$AF_BIN" ]]; then
    echo "test-auto-prove.sh: af binary not executable: $AF_BIN" >&2
    exit 4
fi
AF_BIN="$(cd "$(dirname "$AF_BIN")" && pwd)/$(basename "$AF_BIN")"

STUB_BIN="$TMP_DIR/bin"
mkdir -p "$STUB_BIN"
# auto-prove.sh resolves AF_CMD itself; give it the exact path and also expose
# the binary as `af` on PATH for any internal `command -v af` fallback.
cp "$AF_BIN" "$STUB_BIN/af"

PROOF_DIR="$TMP_DIR/proof"
mkdir -p "$PROOF_DIR"
if ! "$AF_BIN" init --conjecture "Stub agent conjecture" --author stub-author -d "$PROOF_DIR" >/dev/null 2>&1; then
    # Older binaries may use the short flags.
    "$AF_BIN" init -c "Stub agent conjecture" -a stub-author -d "$PROOF_DIR" >/dev/null
fi

# Stub agent. It replaces `claude`: auto-prove.sh invokes
#   claude --print --dangerously-skip-permissions -p "<prompt>"
# The stub finds the prompt, extracts every af invocation from it (the
# generated commands), substitutes <placeholders>, and runs them, recording
# each command and its output in AF_STUB_LOG.
OUTPUT="$TMP_DIR/auto-prove.log"
STUB_LOG="$TMP_DIR/stub.log"
: > "$STUB_LOG"

cat > "$STUB_BIN/claude" <<'STUB'
#!/usr/bin/env bash
set -uo pipefail
af="${AF_STUB_AF:?AF_STUB_AF must point at the af binary}"
log="${AF_STUB_LOG:?AF_STUB_LOG must point at a log file}"
prompt=""
prev=""
for arg in "$@"; do
    if [[ "$prev" == "-p" ]]; then
        prompt="$arg"
    fi
    prev="$arg"
done
[[ -z "$prompt" && $# -gt 0 ]] && prompt="${!#}"

status=0
while IFS= read -r line; do
    [[ "$line" == *"$af"* ]] || continue
    cmd="${line#*"$af"}"
    # trim leading whitespace
    cmd="${cmd#"${cmd%%[![:space:]]*}"}"
    # replace <placeholder> tokens with a harmless dummy
    cmd="$(printf '%s' "$cmd" | sed 's/<[^>]*>/x/g')"
    sub="${cmd%% *}"
    case "$sub" in
        claim|refine|refine-sibling|accept|challenge|release|resolve-challenge|amend|admit|refute|archive|reap)
            printf 'STUB-RUN: %s %s\n' "$af" "$cmd" >> "$log"
            out="$(bash -c "$af $cmd" 2>&1)"
            rc=$?
            printf 'STUB-OUT: %s\n' "$out" >> "$log"
            [[ $rc -ne 0 ]] && status=$rc
            ;;
    esac
done <<< "$prompt"
exit "$status"
STUB
chmod +x "$STUB_BIN/claude"

set +e
(
    cd "$PROOF_DIR" || exit 4
    AF_CMD="$STUB_BIN/af" \
    AF_AGENT_BACKEND=claude \
    AF_STUB_AF="$STUB_BIN/af" \
    AF_STUB_LOG="$STUB_LOG" \
    PATH="$STUB_BIN:$PATH" \
        bash "$SCRIPT_DIR/auto-prove.sh" \
        --agent-backend claude \
        --proof-dir "$PROOF_DIR" \
        --max-iterations 5 \
        --max-agents 4 \
        --parallel 1 \
        --delay-seconds 0 \
        --burst-pause 0
) >"$OUTPUT" 2>&1
set -e

cat "$OUTPUT"
if [[ -s "$STUB_LOG" ]]; then
    echo "--- stub agent log ---"
    cat "$STUB_LOG"
fi

if grep -Eq "unknown flag|flag provided but not defined|unknown shorthand flag" "$OUTPUT" "$STUB_LOG"; then
    echo "test-auto-prove.sh: generated command used an unknown flag (bead ujp4 regression)" >&2
    exit 1
fi

# A stub run must not silently do nothing: at least one generated command has
# to have been executed.
if ! grep -q "STUB-RUN:" "$STUB_LOG"; then
    echo "test-auto-prove.sh: stub agent never executed a generated command" >&2
    exit 1
fi

exit 0
