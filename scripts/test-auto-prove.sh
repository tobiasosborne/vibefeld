#!/usr/bin/env bash
#
# test-auto-prove.sh - exercise scripts/auto-prove.sh against a stub agent.
#
# This is the regression test for bead ujp4 / plan item D11: the commands that
# auto-prove.sh generates for its agents must actually execute. It creates a
# disposable workspace, installs a stub `claude` that pulls the generated af
# commands out of each prompt and runs them, drives one or two iterations of
# auto-prove.sh, and fails if af reports an unknown flag. It also exercises the
# D4 completion gate: the real binary completes on a validated, current root,
# while an af wrapper that strips or negates support_current must not.
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
    # Only generated commands are executed. A generated command names the af
    # binary exactly once; prose alternatives such as
    #   "Consider af archive or af refute"
    # name it twice and must be skipped rather than run as one sentence.
    line_without_af="${line//"$af"/}"
    af_count=$(( (${#line} - ${#line_without_af}) / ${#af} ))
    [[ "$af_count" -eq 1 ]] || continue
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

run_auto_prove() {
    local proof_dir="$1" output="$2" max_iter="$3" max_agents="$4"
    local af_path="${5:-$STUB_BIN/af}"
    set +e
    (
        cd "$proof_dir" || exit 4
        AF_CMD="$af_path" \
        AF_AGENT_BACKEND=claude \
        AF_STUB_AF="$af_path" \
        AF_STUB_LOG="$STUB_LOG" \
        PATH="$STUB_BIN:$PATH" \
            bash "$SCRIPT_DIR/auto-prove.sh" \
            --agent-backend claude \
            --proof-dir "$proof_dir" \
            --max-iterations "$max_iter" \
            --max-agents "$max_agents" \
            --parallel 1 \
            --delay-seconds 0 \
            --burst-pause 0
    ) >"$output" 2>&1
    set -e
}

# Scenario 1: a verifier job. The root is claimed and accepted (the accept
# branch), so this exercises claim/accept/release and the completion check.
OUTPUT="$TMP_DIR/auto-prove.log"
run_auto_prove "$PROOF_DIR" "$OUTPUT" 5 4
cat "$OUTPUT"

# Positive case (D4): the real binary emits support_current, the root is
# validated and current, so auto-prove declares the proof complete.
if ! grep -q "PROOF COMPLETE" "$OUTPUT"; then
    echo "test-auto-prove.sh: positive case did not declare PROOF COMPLETE with support_current present" >&2
    exit 1
fi

# Scenario 2: a prover job. A blocking challenge makes the root prover work,
# exercising the generated refine/amend/resolve-challenge templates.
PROVER_DIR="$TMP_DIR/prover-proof"
mkdir -p "$PROVER_DIR"
"$AF_BIN" init -c "Prover stub conjecture" -a stub-author -d "$PROVER_DIR" >/dev/null
"$AF_BIN" challenge 1 --severity critical --reason "needs work" -d "$PROVER_DIR" >/dev/null
PROVER_OUTPUT="$TMP_DIR/auto-prove-prover.log"
run_auto_prove "$PROVER_DIR" "$PROVER_OUTPUT" 1 1
cat "$PROVER_OUTPUT"

# Negative cases (D4): the real binary reports support_current, but an af
# wrapper strips it or forces it false. auto-prove must fail closed and never
# declare the proof complete.
WRAPPER_DIR="$TMP_DIR/wrappers"
mkdir -p "$WRAPPER_DIR"

cat > "$WRAPPER_DIR/af-missing" <<WRAP
#!/usr/bin/env bash
# Forward to the real af, but drop support_current from `status -f json`.
set -o pipefail
real="$AF_BIN"
if [[ "\${1:-}" == "status" ]]; then
    "\$real" "\$@" | jq 'del(.nodes[].support_current)'
    exit \$?
fi
exec "\$real" "\$@"
WRAP
chmod +x "$WRAPPER_DIR/af-missing"

cat > "$WRAPPER_DIR/af-false" <<WRAP
#!/usr/bin/env bash
# Forward to the real af, but force support_current false in `status -f json`.
set -o pipefail
real="$AF_BIN"
if [[ "\${1:-}" == "status" ]]; then
    "\$real" "\$@" | jq '(.nodes[].support_current) = false'
    exit \$?
fi
exec "\$real" "\$@"
WRAP
chmod +x "$WRAPPER_DIR/af-false"

NEG_OUTPUTS=()
run_completion_negative() {
    local label="$1" wrapper="$2"
    local neg_dir="$TMP_DIR/$label-proof"
    local neg_out="$TMP_DIR/$label.log"
    mkdir -p "$neg_dir"
    "$AF_BIN" init -c "Completion $label conjecture" -a stub-author -d "$neg_dir" >/dev/null
    "$AF_BIN" claim 1 --owner verifier-1 --role verifier -d "$neg_dir" >/dev/null
    "$AF_BIN" accept 1 --agent verifier-1 --with-note "Validated for completion gate" --confirm -d "$neg_dir" >/dev/null

    run_auto_prove "$neg_dir" "$neg_out" 2 1 "$wrapper"

    if grep -q "PROOF COMPLETE" "$neg_out"; then
        echo "test-auto-prove.sh: auto-prove declared PROOF COMPLETE for $label (D4)" >&2
        cat "$neg_out" >&2
        exit 1
    fi
    if ! grep -q "support_current" "$neg_out"; then
        echo "test-auto-prove.sh: $label run did not reach the support_current gate" >&2
        cat "$neg_out" >&2
        exit 1
    fi
    NEG_OUTPUTS+=("$neg_out")
    echo "negative case ($label): ok"
}

run_completion_negative missing-support_current "$WRAPPER_DIR/af-missing"
run_completion_negative false-support_current "$WRAPPER_DIR/af-false"

if [[ -s "$STUB_LOG" ]]; then
    echo "--- stub agent log ---"
    cat "$STUB_LOG"
fi

for f in "$OUTPUT" "$PROVER_OUTPUT" "$STUB_LOG" "${NEG_OUTPUTS[@]}"; do
    if grep -Eq "unknown flag|flag provided but not defined|unknown shorthand flag" "$f"; then
        echo "test-auto-prove.sh: generated command used an unknown flag (bead ujp4 regression)" >&2
        exit 1
    fi
done

# A stub run must not silently do nothing: at least one generated command has
# to have been executed, and the prover path must have exercised refine.
if ! grep -q "STUB-RUN:" "$STUB_LOG"; then
    echo "test-auto-prove.sh: stub agent never executed a generated command" >&2
    exit 1
fi
if ! grep -q "STUB-RUN:.*refine" "$STUB_LOG"; then
    echo "test-auto-prove.sh: prover path never exercised the generated refine command" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Completion-gate negative/positive tests (D11 review).
#
# proof_complete is self-contained, so extract its definition from
# auto-prove.sh and drive it with a fake AF_CMD serving crafted JSON. This
# exercises the fail-closed completion gate directly, including the D4
# support_current requirement and the machine-readable external-reference gate.
# ---------------------------------------------------------------------------
extract_function() {
    local fn="$1" file="$2"
    awk -v fn="$fn" '
        index($0, fn "()") == 1 { found = 1 }
        found { print }
        found && $0 == "}" { exit }
    ' "$file"
}

eval "$(extract_function proof_complete "$SCRIPT_DIR/auto-prove.sh")"
log_warning() { printf 'WARN: %s\n' "$*" >&2; }
log_error() { printf 'ERROR: %s\n' "$*" >&2; }

COMPLETE_DIR="$TMP_DIR/complete-cases"
mkdir -p "$COMPLETE_DIR"
write_json() { printf '%s\n' "$2" > "$COMPLETE_DIR/$1"; }

FAKE_AF() {
    case "$1" in
        status) cat "${FAKE_STATUS:?FAKE_STATUS not set}" ;;
        export) cat "${FAKE_EXPORT:?FAKE_EXPORT not set}" ;;
        pending-refs) cat "${FAKE_PENDING:?FAKE_PENDING not set}" ;;
        *) return 99 ;;
    esac
}
AF_CMD=FAKE_AF

# expect_complete WANT(0|1) LABEL, with FAKE_STATUS/FAKE_EXPORT/FAKE_PENDING set.
expect_complete() {
    local want="$1" label="$2" got=1
    if proof_complete > "$COMPLETE_DIR/out.txt" 2>&1; then
        got=0
    fi
    if [[ "$got" -ne "$want" ]]; then
        echo "test-auto-prove.sh: proof_complete $label: got exit $got, want $want" >&2
        cat "$COMPLETE_DIR/out.txt" >&2
        exit 1
    fi
}

write_json status-valid.json '{"nodes":[{"id":"1","epistemic_state":"validated","taint_state":"clean","support_current":true}]}'
write_json status-stale.json '{"nodes":[{"id":"1","epistemic_state":"validated","taint_state":"clean","support_current":false}]}'
write_json status-missing.json '{"nodes":[{"id":"1","epistemic_state":"validated","taint_state":"clean"}]}'
write_json status-bad.json '{not json'
write_json export-none.json '{"nodes":[{"id":"1","epistemic_state":"validated"}]}'
write_json export-cited.json '{"nodes":[{"id":"1","epistemic_state":"validated","externals":["ext-abc"]}]}'
write_json export-cited-prefix.json '{"nodes":[{"id":"1","epistemic_state":"validated","externals":["external:ext-abc"]}]}'
write_json export-bad.json '{not json'
write_json pending-empty.json '[]'
write_json pending-cited.json '[{"id":"ext-abc","name":"Example"}]'
write_json pending-named.json '[{"id":"other","name":"ext-abc"}]'

FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-none.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 0 "positive control (validated, support_current true, no externals)"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-cited.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 0 "cited external with nothing pending"
FAKE_STATUS="$COMPLETE_DIR/status-stale.json" FAKE_EXPORT="$COMPLETE_DIR/export-none.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 1 "stale support_current"
FAKE_STATUS="$COMPLETE_DIR/status-missing.json" FAKE_EXPORT="$COMPLETE_DIR/export-none.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 1 "missing support_current"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-cited.json" FAKE_PENDING="$COMPLETE_DIR/pending-cited.json" \
    expect_complete 1 "cited pending external by id"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-cited.json" FAKE_PENDING="$COMPLETE_DIR/pending-named.json" \
    expect_complete 1 "cited pending external by name"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-cited-prefix.json" FAKE_PENDING="$COMPLETE_DIR/pending-cited.json" \
    expect_complete 1 "prefixed external: citation matches pending id"
FAKE_STATUS="$COMPLETE_DIR/status-bad.json" FAKE_EXPORT="$COMPLETE_DIR/export-none.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 1 "jq error on status JSON"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-bad.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 1 "jq error on export JSON"
FAKE_STATUS="$COMPLETE_DIR/status-valid.json" FAKE_EXPORT="$COMPLETE_DIR/export-cited.json" FAKE_PENDING="$COMPLETE_DIR/pending-empty.json" \
    expect_complete 0 "positive control after negative cases"

echo "completion negative tests: ok"

exit 0
