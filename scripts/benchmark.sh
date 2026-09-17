#!/usr/bin/env bash
#
# benchmark.sh - Measure af under concurrent load and at scale.
#
# For each (N, W) pair the script builds a synthetic workspace with N nodes via
# scripts/synth-workspace.sh, copies it, then runs W prover workers and a
# smaller pool of verifier workers for a fixed wall time. Each prover loop is
# claim -> refine one child -> release; each verifier loop is accept --confirm
# on a verifier-ready node. Every command records its latency, exit code and
# whether it was a lock-wait. The script also measures single-command latency
# on the 1000-node workspace and the per-event directory-fsync cost by running
# 200 sequential refines with and without AF_TEST_NO_FSYNC=1.
#
# Output: a markdown summary on stdout and a JSON document (--json, default
# benchmark-results.json).
#
# Usage:
#   scripts/benchmark.sh [options]
#
# Options:
#   -t, --duration SEC   Wall time per (N,W) run (default 30)
#   -n, --nodes LIST     Space/comma-separated node counts (default "100 1000")
#   -w, --writers LIST   Space/comma-separated writer counts (default "10 50")
#   -r, --rounds R       Verification rounds for synth-workspace.sh (default 2)
#       --verifiers V    Verifier workers per run (default: writers/5, min 1)
#       --json FILE      JSON output path (default benchmark-results.json)
#       --workdir DIR    Scratch directory (default mktemp -d)
#       --skip-build     Reuse existing workspaces in --workdir
#       --keep           Keep the scratch directory on exit
#   -h, --help           Show this help
#
# Environment:
#   AF_CMD   af binary under test (default: project root ./af, then PATH)
#
# Exit codes:
#   0  benchmark completed
#   1  bad arguments / missing af / a build step failed
#   4  bash < 4 or jq missing

set -euo pipefail

if (( BASH_VERSINFO[0] < 4 )); then
    echo "benchmark.sh: bash >= 4 is required (this is bash $BASH_VERSION)." >&2
    exit 4
fi
if ! command -v jq >/dev/null 2>&1; then
    echo "benchmark.sh: jq is required." >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SYNTH="$SCRIPT_DIR/synth-workspace.sh"

AF_CMD="${AF_CMD:-}"
if [[ -z "$AF_CMD" ]]; then
    if [[ -x "$PROJECT_ROOT/af" ]]; then
        AF_CMD="$PROJECT_ROOT/af"
    elif command -v af >/dev/null 2>&1; then
        AF_CMD="af"
    else
        echo "benchmark.sh: no af binary found (run scripts/build.sh or set AF_CMD)." >&2
        exit 1
    fi
fi
if [[ ! -x "$AF_CMD" ]]; then
    echo "benchmark.sh: AF_CMD is not executable: $AF_CMD" >&2
    exit 1
fi
AF_CMD="$(cd "$(dirname "$AF_CMD")" && pwd)/$(basename "$AF_CMD")"

DURATION=30
NODES_STR="100 1000"
WRITERS_STR="10 50"
ROUNDS=2
VERIFIERS=""
JSON_OUT="benchmark-results.json"
WORKDIR=""
SKIP_BUILD=false
KEEP=false

usage() { sed -n '2,50p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        -t|--duration) DURATION="$2"; shift 2 ;;
        -n|--nodes)    NODES_STR="${2//,/ }"; shift 2 ;;
        -w|--writers)  WRITERS_STR="${2//,/ }"; shift 2 ;;
        -r|--rounds)   ROUNDS="$2"; shift 2 ;;
        --verifiers)   VERIFIERS="$2"; shift 2 ;;
        --json)        JSON_OUT="$2"; shift 2 ;;
        --workdir)     WORKDIR="$2"; shift 2 ;;
        --skip-build)  SKIP_BUILD=true; shift ;;
        --keep)        KEEP=true; shift ;;
        -h|--help)     usage; exit 0 ;;
        *) echo "benchmark.sh: unknown argument: $1" >&2; exit 1 ;;
    esac
done

for pair in "duration:$DURATION" "rounds:$ROUNDS"; do
    name="${pair%%:*}"; val="${pair#*:}"
    if ! [[ "$val" =~ ^[0-9]+$ ]] || (( val < 1 )); then
        echo "benchmark.sh: --$name must be a positive integer (got '$val')" >&2
        exit 1
    fi
done

read -r -a NODE_LIST <<< "$NODES_STR"
read -r -a WRITER_LIST <<< "$WRITERS_STR"
if (( ${#NODE_LIST[@]} == 0 || ${#WRITER_LIST[@]} == 0 )); then
    echo "benchmark.sh: empty --nodes or --writers" >&2
    exit 1
fi

if [[ -z "$WORKDIR" ]]; then
    WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/af-benchmark.XXXXXX")"
fi
mkdir -p "$WORKDIR"

cleanup() {
    if [[ "$KEEP" != true ]]; then
        rm -rf "$WORKDIR"
    else
        echo "benchmark.sh: scratch kept at $WORKDIR" >&2
    fi
}
trap cleanup EXIT

# --- Timing helpers -----------------------------------------------------------
# EPOCHREALTIME (bash 5) gives microseconds; fall back to GNU date (Linux).
now_ns() {
    if [[ -n "${EPOCHREALTIME:-}" ]]; then
        local s="${EPOCHREALTIME%.*}" us="${EPOCHREALTIME#*.}"
        # Pad/truncate to 6 digits so 10# is safe.
        us="${us}000000"; us="${us:0:6}"
        printf '%s' $(( s * 1000000000 + 10#$us * 1000 ))
    else
        date +%s%N
    fi
}

ns_to_ms() {
    awk -v n="$1" 'BEGIN { printf "%.2f", n / 1e6 }'
}

# percentile_ns <sorted-file> <p>; nearest-rank percentile.
percentile_ns() {
    awk -v p="$2" '{ a[NR] = $1 } END {
        if (NR == 0) { print 0; exit }
        i = int(p / 100 * NR + 0.999999)
        if (i < 1) i = 1
        if (i > NR) i = NR
        print a[i]
    }' "$1"
}

sum_field() { # file field
    awk -v f="$2" '{ s += $f } END { print s + 0 }' "$1"
}

ledger_bytes() {
    find "$1/ledger" -maxdepth 1 -name '*.json' -printf '%s\n' 2>/dev/null \
        | awk '{ s += $1 } END { print s + 0 }'
}

ledger_events() {
    find "$1/ledger" -maxdepth 1 -name '*.json' 2>/dev/null | wc -l | tr -d ' '
}

# --- Workers ------------------------------------------------------------------
# Each worker appends "cmd dur_ns rc lock" lines to its own log file. rc is the
# af exit code (1 = retriable concurrent modification) and lock is 1 when the
# output names a lock wait.

writer_worker() {
    local w="$1" ws="$2" deadline="$3" log="$4" cands="$5" writers="$6"
    local -a ids=()
    mapfile -t ids < "$cands" || true
    local n=${#ids[@]}
    if (( n == 0 )); then return; fi
    local node out rc t0 t1 lock i=0
    # Partition candidates across writers (stride = writer count) so writers do
    # not all slide through the same window; the ledger lock and CAS retries
    # remain the shared contention this benchmark is about.
    while (( $(now_ns) < deadline )); do
        node="${ids[$(( (w + i * writers) % n ))]}"
        i=$(( i + 1 ))

        t0=$(now_ns)
        out=$(AF_AGENT_ID="w$w" "$AF_CMD" claim "$node" -o "w$w" -r prover -d "$ws" 2>&1) || rc=$?
        rc=${rc:-0}
        t1=$(now_ns)
        lock=0; [[ "$out" == *"acquire lock"* || "$out" == *"timeout waiting for lock"* ]] && lock=1
        printf 'claim %s %s %s\n' "$(( t1 - t0 ))" "$rc" "$lock" >> "$log"
        local claim_rc=$rc
        unset rc

        if (( claim_rc == 0 )); then
            # We hold the claim: refine one child, then always release it.
            t0=$(now_ns)
            out=$("$AF_CMD" refine "$node" "bench step for $node by w$w at $t0" -o "w$w" -d "$ws" 2>&1) || rc=$?
            rc=${rc:-0}
            t1=$(now_ns)
            lock=0; [[ "$out" == *"acquire lock"* || "$out" == *"timeout waiting for lock"* ]] && lock=1
            printf 'refine %s %s %s\n' "$(( t1 - t0 ))" "$rc" "$lock" >> "$log"
            unset rc

            t0=$(now_ns)
            out=$("$AF_CMD" release "$node" -o "w$w" -d "$ws" 2>&1) || rc=$?
            rc=${rc:-0}
            t1=$(now_ns)
            lock=0; [[ "$out" == *"acquire lock"* || "$out" == *"timeout waiting for lock"* ]] && lock=1
            printf 'release %s %s %s\n' "$(( t1 - t0 ))" "$rc" "$lock" >> "$log"
            unset rc
        fi
    done
}

verifier_worker() {
    local v="$1" ws="$2" deadline="$3" log="$4" nver="$5"
    local out rc t0 t1 lock k node
    local -a ids=()
    while (( $(now_ns) < deadline )); do
        out=$("$AF_CMD" jobs -d "$ws" -f json 2>/dev/null) || true
        mapfile -t ids < <(printf '%s' "$out" | jq -r '.verifier_jobs[].node_id' 2>/dev/null)
        if (( ${#ids[@]} == 0 )); then
            sleep 0.05
            continue
        fi
        for (( k = v; k < ${#ids[@]}; k += nver )); do
            if (( $(now_ns) >= deadline )); then break; fi
            node="${ids[$k]}"
            unset rc
            t0=$(now_ns)
            out=$("$AF_CMD" accept "$node" --agent "v$v" --confirm -d "$ws" 2>&1) || rc=$?
            rc=${rc:-0}
            t1=$(now_ns)
            lock=0; [[ "$out" == *"acquire lock"* || "$out" == *"timeout waiting for lock"* ]] && lock=1
            printf 'accept %s %s %s\n' "$(( t1 - t0 ))" "$rc" "$lock" >> "$log"
        done
    done
}

# aggregate_logs <logdir> <outfile>; outfile lines: cmd count p50ns p95ns retries locks
aggregate_logs() {
    local dir="$1" out="$2"
    : > "$out"
    local cmd all count retries locks p50 p95 durs
    for cmd in claim refine release accept; do
        all="$(mktemp)"
        cat "$dir"/*.log 2>/dev/null > "$all" || true
        durs="$(mktemp)"
        awk -v c="$cmd" '$1 == c { print $2 }' "$all" | sort -n > "$durs"
        count=$(wc -l < "$durs" | tr -d ' ')
        retries=$(awk -v c="$cmd" '$1 == c && $3 == 1 { n++ } END { print n + 0 }' "$all")
        locks=$(awk -v c="$cmd" '$1 == c && $4 == 1 { n++ } END { print n + 0 }' "$all")
        p50=$(percentile_ns "$durs" 50)
        p95=$(percentile_ns "$durs" 95)
        printf '%s %s %s %s %s %s\n' "$cmd" "$count" "$p50" "$p95" "$retries" "$locks" >> "$out"
        rm -f "$all" "$durs"
    done
}

# run_load <label> <ws> <writers> <verifiers> <duration> <logdir>
run_load() {
    local label="$1" ws="$2" writers="$3" nver="$4" duration="$5" logdir="$6"
    mkdir -p "$logdir"
    rm -f "$logdir"/*.log 2>/dev/null || true

    local cands="$WORKDIR/cands-$label.txt"
    "$AF_CMD" status -d "$ws" -f json 2>/dev/null \
        | jq -r '.nodes[] | select(.workflow_state == "available" and .epistemic_state == "pending") | .id' \
        > "$cands" || true

    local deadline=$(( $(now_ns) + duration * 1000000000 ))
    local pid w pids=()
    for (( w = 0; w < writers; w++ )); do
        writer_worker "$w" "$ws" "$deadline" "$logdir/writer-$w.log" "$cands" "$writers" &
        pids+=($!)
    done
    for (( w = 0; w < nver; w++ )); do
        verifier_worker "$w" "$ws" "$deadline" "$logdir/verifier-$w.log" "$nver" &
        pids+=($!)
    done
    local p
    for p in "${pids[@]}"; do wait "$p" 2>/dev/null || true; done
}

# --- Build workspaces ---------------------------------------------------------
build_workspace() { # nodes dir
    local nodes="$1" dir="$2" ev
    if [[ "$SKIP_BUILD" == true && -d "$dir/ledger" ]]; then
        ev=$(ledger_events "$dir")
        echo "reusing $dir ($ev events)" >&2
        echo "$ev"
        return
    fi
    rm -rf "$dir"
    echo "building $nodes-node workspace at $dir (this replays the ledger once per command)..." >&2
    local out
    out="$(AF_CMD="$AF_CMD" "$SYNTH" -n "$nodes" -s 1 -r "$ROUNDS" -o "$dir" -q)"
    echo "$out" | awk '/^EVENTS /{ print $2 }'
}

echo "benchmark: af=$AF_CMD duration=${DURATION}s rounds=$ROUNDS" >&2

declare -A WS_EVENTS
for n in "${NODE_LIST[@]}"; do
    WS_EVENTS[$n]="$(build_workspace "$n" "$WORKDIR/ws-$n")"
    echo "benchmark: ws-$n has ${WS_EVENTS[$n]} events" >&2
done

# --- Load runs ----------------------------------------------------------------
RUNS_JSON="$WORKDIR/runs.json"
echo '[]' > "$RUNS_JSON"

for n in "${NODE_LIST[@]}"; do
    base="$WORKDIR/ws-$n"
    for w in "${WRITER_LIST[@]}"; do
        if [[ -n "$VERIFIERS" ]]; then
            nver="$VERIFIERS"
        else
            nver=$(( w / 5 )); (( nver < 1 )) && nver=1
        fi
        label="${n}x${w}"
        runws="$WORKDIR/run-$label"
        rm -rf "$runws"
        cp -a "$base" "$runws"
        logdir="$WORKDIR/logs-$label"
        before_ev=$(ledger_events "$runws")
        before_bytes=$(ledger_bytes "$runws")

        echo "benchmark: run N=$n W=$w V=$nver for ${DURATION}s..." >&2
        run_load "$label" "$runws" "$w" "$nver" "$DURATION" "$logdir"

        after_ev=$(ledger_events "$runws")
        after_bytes=$(ledger_bytes "$runws")
        agg="$WORKDIR/agg-$label.txt"
        aggregate_logs "$logdir" "$agg"

        # Append this run's aggregate to runs.json.
        jq --argjson n "$n" --argjson w "$w" --argjson v "$nver" \
           --argjson before_ev "$before_ev" --argjson after_ev "$after_ev" \
           --argjson before_bytes "$before_bytes" --argjson after_bytes "$after_bytes" \
           --rawfile agg "$agg" \
           '. + [{
                nodes: $n, writers: $w, verifiers: $v,
                events_before: $before_ev, events_appended: ($after_ev - $before_ev),
                ledger_bytes_before: $before_bytes, ledger_bytes: $after_bytes,
                replays_per_command: 1,
                commands: ($agg | split("\n") | map(select(length > 0) | split(" ")) |
                    map({ key: .[0], value: {
                        count: (.[1]|tonumber), p50_ms: ((.[2]|tonumber)/1000000),
                        p95_ms: ((.[3]|tonumber)/1000000), retries: (.[4]|tonumber),
                        lock_waits: (.[5]|tonumber) } }) | from_entries)
              }]' "$RUNS_JSON" > "$RUNS_JSON.tmp" && mv "$RUNS_JSON.tmp" "$RUNS_JSON"

        rm -rf "$runws"
    done
done

# --- Single-command latency on the 1000-node workspace ------------------------
# Use the largest requested N; if it is not 1000 the table still labels it.
SINGLE_N=""
for n in "${NODE_LIST[@]}"; do
    if (( n >= 1000 )); then SINGLE_N="$n"; break; fi
done
[[ -z "$SINGLE_N" ]] && SINGLE_N="${NODE_LIST[${#NODE_LIST[@]}-1]}"
SINGLE_WS="$WORKDIR/ws-$SINGLE_N"
SINGLE_JSON="$WORKDIR/single.json"

measure_single() { # name runs -- cmd...
    local name="$1" runs="$2"; shift 3
    local best_ns=0 r t0 t1 rc
    local -a vals=()
    for (( r = 0; r < runs; r++ )); do
        t0=$(now_ns)
        "$@" >/dev/null 2>&1 || rc=$?
        t1=$(now_ns)
        vals+=("$(( t1 - t0 ))")
        unset rc
    done
    printf '%s\n' "${vals[@]}" | sort -n > "$WORKDIR/single-$name.txt"
    local count=${#vals[@]}
    local p50 p95
    p50=$(percentile_ns "$WORKDIR/single-$name.txt" 50)
    p95=$(percentile_ns "$WORKDIR/single-$name.txt" 95)
    jq -n --arg name "$name" --argjson count "$count" --argjson p50 "$p50" --argjson p95 "$p95" \
        '{ key: $name, value: { count: $count, p50_ms: ($p50/1000000), p95_ms: ($p95/1000000) } }'
}

echo "benchmark: single-command latency on $SINGLE_N-node workspace..." >&2
: > "$SINGLE_JSON"
{
    measure_single status 5 -- "$AF_CMD" status -d "$SINGLE_WS"
    measure_single jobs 5 -- "$AF_CMD" jobs -d "$SINGLE_WS"
    measure_single health 5 -- "$AF_CMD" health -d "$SINGLE_WS"
    measure_single audit 5 -- "$AF_CMD" audit -d "$SINGLE_WS"
    measure_single export_graph 5 -- "$AF_CMD" export --graph json -d "$SINGLE_WS"
} >> "$SINGLE_JSON"
jq -s 'from_entries' "$SINGLE_JSON" > "$SINGLE_JSON.tmp" && mv "$SINGLE_JSON.tmp" "$SINGLE_JSON"

# Bulk accept of 20 verifier-ready nodes. Take 60 ready nodes and measure three
# disjoint batches so every run accepts fresh nodes.
"$AF_CMD" jobs -d "$SINGLE_WS" -f json 2>/dev/null \
    | jq -r '.verifier_jobs[0:60][].node_id' > "$WORKDIR/bulk-ids.txt"
mapfile -t BULK_IDS < "$WORKDIR/bulk-ids.txt"
bulk_vals=()
for (( r = 0; r < 3; r++ )); do
    start=$(( r * 20 )); end=$(( start + 20 ))
    if (( end > ${#BULK_IDS[@]} )); then break; fi
    node_args=()
    for (( k = start; k < end; k++ )); do node_args+=("${BULK_IDS[$k]}"); done
    t0=$(now_ns)
    "$AF_CMD" accept "${node_args[@]}" --agent bench-verifier --confirm -d "$SINGLE_WS" >/dev/null 2>&1 || true
    t1=$(now_ns)
    bulk_vals+=("$(( t1 - t0 ))")
done
if (( ${#bulk_vals[@]} == 0 )); then
    bulk_count=0; bulk_p50=0; bulk_p95=0
else
    printf '%s\n' "${bulk_vals[@]}" | sort -n > "$WORKDIR/single-bulk_accept_20.txt"
    bulk_count=${#bulk_vals[@]}
    bulk_p50=$(percentile_ns "$WORKDIR/single-bulk_accept_20.txt" 50)
    bulk_p95=$(percentile_ns "$WORKDIR/single-bulk_accept_20.txt" 95)
fi
jq -n --argjson count "$bulk_count" --argjson p50 "$bulk_p50" --argjson p95 "$bulk_p95" \
    '{ bulk_accept_20: { count: $count, p50_ms: ($p50/1000000), p95_ms: ($p95/1000000), nodes: 20 } }' > "$WORKDIR/bulk.json"

# --- fsync cost ---------------------------------------------------------------
# 200 sequential single-node refines (claim+refine+release; all three append)
# on a fresh copy of the largest workspace, with and without the test-only
# AF_TEST_NO_FSYNC switch. The delta over events appended is the per-event
# directory-fsync cost.
FSYNC_WS="$WORKDIR/fsync-base"
rm -rf "$FSYNC_WS"
cp -a "$WORKDIR/ws-$SINGLE_N" "$FSYNC_WS"

sed -n '1,200p' <<< "$( "$AF_CMD" status -d "$FSYNC_WS" -f json 2>/dev/null \
    | jq -r '.nodes[] | select(.workflow_state == "available" and .epistemic_state == "pending") | .id' )" \
    > "$WORKDIR/fsync-parents.txt"

run_fsync_round() { # nofsync(0/1) -> "elapsed_ns events"
    local nofsync="$1"
    local ws="$WORKDIR/fsync-$nofsync"
    rm -rf "$ws"
    cp -a "$FSYNC_WS" "$ws"
    local p t0 t1 rc ev_before ev_after
    ev_before=$(ledger_events "$ws")
    t0=$(now_ns)
    while read -r p; do
        [[ -z "$p" ]] && continue
        unset rc
        AF_TEST_NO_FSYNC="$nofsync" "$AF_CMD" claim "$p" -o fsync-prover -r prover -d "$ws" >/dev/null 2>&1 || true
        AF_TEST_NO_FSYNC="$nofsync" "$AF_CMD" refine "$p" "fsync cost step $p" -o fsync-prover -d "$ws" >/dev/null 2>&1 || true
        AF_TEST_NO_FSYNC="$nofsync" "$AF_CMD" release "$p" -o fsync-prover -d "$ws" >/dev/null 2>&1 || true
    done < "$WORKDIR/fsync-parents.txt"
    t1=$(now_ns)
    ev_after=$(ledger_events "$ws")
    printf '%s %s\n' "$(( t1 - t0 ))" "$(( ev_after - ev_before ))"
    rm -rf "$ws"
}

echo "benchmark: fsync cost (200 refines)..." >&2
read -r FSYNC_WITH_NS FSYNC_WITH_EV <<< "$(run_fsync_round 0)"
read -r FSYNC_WITHOUT_NS FSYNC_WITHOUT_EV <<< "$(run_fsync_round 1)"
FSYNC_DELTA_NS=$(( FSYNC_WITH_NS - FSYNC_WITHOUT_NS ))
FSYNC_DELTA_MS=$(ns_to_ms "$FSYNC_DELTA_NS")
FSYNC_WITH_MS=$(ns_to_ms "$FSYNC_WITH_NS")
FSYNC_WITHOUT_MS=$(ns_to_ms "$FSYNC_WITHOUT_NS")
if (( FSYNC_WITH_EV > 0 )); then
    FSYNC_PER_EVENT_US=$(awk -v d="$FSYNC_DELTA_NS" -v e="$FSYNC_WITH_EV" 'BEGIN { printf "%.1f", d / e / 1000 }')
else
    FSYNC_PER_EVENT_US="0"
fi

# --- Assemble JSON + markdown -------------------------------------------------
CONFIG_JSON=$(jq -n --argjson duration "$DURATION" --argjson rounds "$ROUNDS" \
    --argjson nodes "$(printf '%s\n' "${NODE_LIST[@]}" | jq -s .)" \
    --argjson writers "$(printf '%s\n' "${WRITER_LIST[@]}" | jq -s .)" \
    '{ duration_s: $duration, rounds: $rounds, nodes: $nodes, writers: $writers }')

FSYNC_JSON=$(jq -n --argjson with_ms "$FSYNC_WITH_MS" --argjson without_ms "$FSYNC_WITHOUT_MS" \
    --argjson delta_ms "$FSYNC_DELTA_MS" --argjson events "$FSYNC_WITH_EV" \
    --argjson per_event_us "$FSYNC_PER_EVENT_US" \
    '{ with_fsync_ms: $with_ms, without_fsync_ms: $without_ms, delta_ms: $delta_ms,
       events_appended: $events, per_event_us: $per_event_us }')

jq -n --arg generated "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson config "$CONFIG_JSON" \
    --slurpfile runs "$RUNS_JSON" \
    --slurpfile single "$SINGLE_JSON" \
    --slurpfile bulk "$WORKDIR/bulk.json" \
    --argjson fsync "$FSYNC_JSON" \
    '{
        generated_utc: $generated,
        config: $config,
        replays_per_command: 1,
        runs: $runs[0],
        single_command_latency: ($single[0] + $bulk[0]),
        fsync: $fsync
    }' > "$JSON_OUT"

# Markdown summary.
{
    echo "# af benchmark"
    echo
    echo "Generated $(date -u +%Y-%m-%dT%H:%M:%SZ). af: \`$AF_CMD\`. Duration per run: ${DURATION}s; rounds: ${ROUNDS}."
    echo
    echo "## Concurrent load"
    echo
    echo "Each command replays the ledger once, so \`replays/command = 1\` by construction."
    echo
    echo "| N | W | command | count | p50 (ms) | p95 (ms) | retries (exit 1) | lock waits | events app. | ledger bytes |"
    echo "|---|---|---------|-------|----------|----------|------------------|------------|-------------|--------------|"
    for n in "${NODE_LIST[@]}"; do
        for w in "${WRITER_LIST[@]}"; do
            jq -r --argjson n "$n" --argjson w "$w" '
                .runs[] | select(.nodes == $n and .writers == $w) |
                . as $r | ["claim","refine","release","accept"][] as $c |
                "| \($n) | \($w) | \($c) | \($r.commands[$c].count) | \($r.commands[$c].p50_ms | . * 100 | round / 100) | \($r.commands[$c].p95_ms | . * 100 | round / 100) | \($r.commands[$c].retries) | \($r.commands[$c].lock_waits) | \($r.events_appended) | \($r.ledger_bytes) |"
            ' "$JSON_OUT"
        done
    done
    echo
    echo "## Single-command latency (${SINGLE_N}-node workspace)"
    echo
    echo "| command | count | p50 (ms) | p95 (ms) |"
    echo "|---------|-------|----------|----------|"
    jq -r '.single_command_latency | to_entries[] |
        "| \(.key) | \(.value.count) | \(.value.p50_ms | . * 100 | round / 100) | \((.value.p95_ms // 0) | . * 100 | round / 100) |"' "$JSON_OUT"
    echo
    echo "## Directory-fsync cost (200 sequential single-node refines)"
    echo
    echo "| mode | elapsed (ms) |"
    echo "|------|--------------|"
    echo "| fsync | $FSYNC_WITH_MS |"
    echo "| AF_TEST_NO_FSYNC=1 | $FSYNC_WITHOUT_MS |"
    echo
    echo "Delta **${FSYNC_DELTA_MS} ms** over **${FSYNC_WITH_EV}** appended events ≈ **${FSYNC_PER_EVENT_US} µs/event**."
    echo
    echo "JSON written to \`$JSON_OUT\`."
} | tee "$WORKDIR/summary.md"

echo "benchmark: done. JSON: $JSON_OUT" >&2
