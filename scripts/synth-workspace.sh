#!/usr/bin/env bash
#
# synth-workspace.sh - Generate a synthetic af workspace at scale.
#
# Builds a proof tree with N nodes (default 1000), high fan-out and depth 3-4,
# a fraction of cross-reference dependencies, then R verification rounds
# (challenges on a fraction of nodes, resolutions, amendments, accepts).
# Everything is driven through real `af` commands so the generated ledger is
# exactly what the CLI would write; no internal API is used.
#
# The generator is deterministic for a given --seed: the pseudo-random choices
# (cross-reference targets, which nodes to challenge / amend / accept) come from
# a seeded 31-bit LCG, not from $RANDOM, so reruns with the same seed produce
# the same workspace.
#
# Portability: bash >= 4 is required (same rule as scripts/auto-prove.sh), and
# jq must be on PATH for parsing `af ... -f json` output.
#
# Usage:
#   scripts/synth-workspace.sh [options]
#
# Options:
#   -n, --nodes N        Total nodes to create (default 1000)
#   -s, --seed S         PRNG seed (default 1)
#   -d, --depth D        Maximum tree depth (default 4; high fan-out reaches 3)
#   -f, --fanout F       Children per parent (default 10)
#   -x, --xref PCT       Percent of nodes given a cross-reference dep (default 15)
#   -r, --rounds R       Verification rounds (default 2)
#       --challenge PCT  Percent of nodes challenged per round (default 10)
#       --amend PCT      Percent of nodes amended per round (default 5)
#       --accept PCT     Percent of nodes accepted per round (default 20)
#   -o, --out DIR        Workspace directory (default: mktemp under $TMPDIR)
#   -q, --quiet          Only print the WORKSPACE/EVENTS lines
#   -h, --help           Show this help
#
# Environment:
#   AF_CMD               af binary to use (default: project root ./af, then PATH)
#
# Exit codes:
#   0  workspace generated
#   1  bad arguments or af command failed unexpectedly
#   4  bash < 4 or jq missing
#
# Output (last two lines, stable for parsing):
#   WORKSPACE <path>
#   EVENTS <count>

set -euo pipefail

# --- Portability guard: bash 4 for arrays/associative-free LCG is enough, but
# auto-prove.sh uses the same check; stock macOS bash 3.2 must fail clearly. ---
if (( BASH_VERSINFO[0] < 4 )); then
    echo "synth-workspace.sh: bash >= 4 is required (this is bash $BASH_VERSION)." >&2
    exit 4
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "synth-workspace.sh: jq is required (used to parse 'af ... -f json' output)." >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Resolve the af binary: explicit AF_CMD wins, then project root, then PATH.
AF_CMD="${AF_CMD:-}"
if [[ -z "$AF_CMD" ]]; then
    if [[ -x "$PROJECT_ROOT/af" ]]; then
        AF_CMD="$PROJECT_ROOT/af"
    elif command -v af >/dev/null 2>&1; then
        AF_CMD="af"
    else
        echo "synth-workspace.sh: no af binary found (build one or set AF_CMD)." >&2
        exit 1
    fi
fi
if [[ ! -x "$AF_CMD" ]]; then
    echo "synth-workspace.sh: AF_CMD is not executable: $AF_CMD" >&2
    exit 1
fi

# --- Defaults ---
NODES=1000
SEED=1
MAXDEPTH=4
FANOUT=10
XREF=15
ROUNDS=2
CHALLENGE_PCT=10
AMEND_PCT=5
ACCEPT_PCT=20
OUT=""
QUIET=false

usage() {
    sed -n '2,50p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -n|--nodes)    NODES="$2"; shift 2 ;;
        -s|--seed)     SEED="$2"; shift 2 ;;
        -d|--depth)    MAXDEPTH="$2"; shift 2 ;;
        -f|--fanout)   FANOUT="$2"; shift 2 ;;
        -x|--xref)     XREF="$2"; shift 2 ;;
        -r|--rounds)   ROUNDS="$2"; shift 2 ;;
        --challenge)   CHALLENGE_PCT="$2"; shift 2 ;;
        --amend)       AMEND_PCT="$2"; shift 2 ;;
        --accept)      ACCEPT_PCT="$2"; shift 2 ;;
        -o|--out)      OUT="$2"; shift 2 ;;
        -q|--quiet)    QUIET=true; shift ;;
        -h|--help)     usage; exit 0 ;;
        *) echo "synth-workspace.sh: unknown argument: $1" >&2; exit 1 ;;
    esac
done

# Numeric sanity: values are used in arithmetic, so require non-negative ints.
for pair in "nodes:$NODES" "seed:$SEED" "depth:$MAXDEPTH" "fanout:$FANOUT" \
            "xref:$XREF" "rounds:$ROUNDS" "challenge:$CHALLENGE_PCT" \
            "amend:$AMEND_PCT" "accept:$ACCEPT_PCT"; do
    name="${pair%%:*}"; val="${pair#*:}"
    if ! [[ "$val" =~ ^[0-9]+$ ]]; then
        echo "synth-workspace.sh: --$name must be a non-negative integer (got '$val')" >&2
        exit 1
    fi
done
if (( NODES < 1 )); then
    echo "synth-workspace.sh: --nodes must be >= 1" >&2
    exit 1
fi
if (( FANOUT < 1 || FANOUT > 100 )); then
    echo "synth-workspace.sh: --fanout must be in 1..100" >&2
    exit 1
fi

WORKSPACE="$OUT"
if [[ -z "$WORKSPACE" ]]; then
    WORKSPACE="$(mktemp -d "${TMPDIR:-/tmp}/synth-workspace.XXXXXX")/proof"
fi

PROVER="synth-prover"
VERIFIER="synth-verifier"
AUTHOR="synth-author"

# --- Deterministic PRNG: 31-bit LCG (Numerical Recipes constants). ---
# The helpers update RNG_STATE in the current shell and expose the draw in
# RNG_VAL; they must NOT be called inside command substitution, which would run
# them in a subshell and throw the state update away.
RNG_STATE=$(( SEED & 0x7fffffff ))
if (( RNG_STATE == 0 )); then RNG_STATE=1; fi
RNG_VAL=0

# rng_next advances the state and stores the new value in RNG_VAL.
rng_next() {
    RNG_STATE=$(( (RNG_STATE * 1103515245 + 12345) & 0x7fffffff ))
    RNG_VAL=$RNG_STATE
}

# rng_range N: draw a value in [0, N); N must be > 0. Result in RNG_VAL.
rng_range() {
    rng_next
    RNG_VAL=$(( RNG_VAL % $1 ))
}

# pct P: true if a fresh draw is < P (P in 0..100).
pct() {
    rng_next
    (( RNG_VAL % 100 < $1 ))
}

log() {
    if [[ "$QUIET" != true ]]; then
        echo "$@" >&2
    fi
}

af() {
    "$AF_CMD" "$@"
}

# af_verifier runs af with the verifier identity in the environment (AF_AGENT_ID).
af_verifier() {
    AF_AGENT_ID="$VERIFIER" "$AF_CMD" "$@"
}

# --- Phase 0: initialise. ---
log "synth-workspace: initialising $WORKSPACE"
af init -c "Synthetic workspace (nodes=$NODES seed=$SEED depth=$MAXDEPTH fanout=$FANOUT xref=$XREF)" \
    -a "$AUTHOR" -d "$WORKSPACE" >/dev/null

# all_nodes is a bash array, in creation order.
all_nodes=("1")
frontier=("1")
count=1
depth=0

# --- Phase 1: build the tree breadth-first, batching children per parent. ---
while (( count < NODES && depth < MAXDEPTH )); do
    next_frontier=()
    for parent in "${frontier[@]}"; do
        if (( count >= NODES )); then break; fi
        remaining=$(( NODES - count ))
        kids=$FANOUT
        if (( kids > remaining )); then kids=$remaining; fi
        if (( kids < 1 )); then break; fi

        stmts=()
        child_ids=()
        for (( i = 1; i <= kids; i++ )); do
            cid="$parent.$i"
            child_ids+=("$cid")
            stmts+=("Synthetic step $cid (seed $SEED)")
        done

        af claim "$parent" -o "$PROVER" -r prover -d "$WORKSPACE" >/dev/null
        af refine "$parent" "${stmts[@]}" -o "$PROVER" -d "$WORKSPACE" >/dev/null
        af release "$parent" -o "$PROVER" -d "$WORKSPACE" >/dev/null

        for cid in "${child_ids[@]}"; do
            all_nodes+=("$cid")
            next_frontier+=("$cid")
        done
        count=$(( count + kids ))
    done
    if (( ${#next_frontier[@]} > 0 )); then
        frontier=("${next_frontier[@]}")
    else
        frontier=()
    fi
    depth=$(( depth + 1 ))
done

total=${#all_nodes[@]}
log "synth-workspace: tree has $total nodes (depth $depth)"

# --- Phase 2: cross-reference dependencies. ---
# Edges always point from an earlier-created node to a later-created node, the
# same direction as every tree edge, so the resulting graph is acyclic by
# construction and support.Current always has a DAG to fold.
xref_added=0
if (( XREF > 0 && total > 1 )); then
    for (( i = 0; i < total; i++ )); do
        if ! pct "$XREF"; then continue; fi
        span=$(( total - i - 1 ))
        if (( span < 1 )); then continue; fi
        rng_range "$span"
        j=$(( i + 1 + RNG_VAL ))
        src="${all_nodes[$i]}"
        dst="${all_nodes[$j]}"
        if af amend-deps "$src" --add "$dst" --owner "$PROVER" \
                --reason "synthetic cross-reference" -d "$WORKSPACE" >/dev/null 2>&1; then
            xref_added=$(( xref_added + 1 ))
        fi
    done
fi
log "synth-workspace: added $xref_added cross-reference dependencies"

# --- Phase 3: verification rounds. ---
# Each round challenges a fraction of nodes, resolves every challenge it raised,
# amends a fraction, then accepts a fraction deepest-first so children clear
# before parents. Expected refusals (already validated, blocking state, depth)
# are ignored: the goal is a realistic ledger, not a valid proof.
accept_index=0
for (( round = 1; round <= ROUNDS; round++ )); do
    challenge_ids=()

    # 3a. challenges.
    for node in "${all_nodes[@]}"; do
        if ! pct "$CHALLENGE_PCT"; then continue; fi
        out="$(af_verifier challenge "$node" \
                -r "synthetic objection, round $round" -s major \
                -t statement -d "$WORKSPACE" -f json 2>/dev/null)" || true
        cid="$(printf '%s' "$out" | jq -r '.challenge_id // empty' 2>/dev/null || true)"
        if [[ -n "$cid" ]]; then
            challenge_ids+=("$cid")
        fi
    done

    # 3b. resolutions.
    if (( ${#challenge_ids[@]} > 0 )); then
        for cid in "${challenge_ids[@]}"; do
            af resolve-challenge "$cid" -r "synthetic resolution, round $round" \
                -d "$WORKSPACE" >/dev/null 2>&1 || true
        done
    fi

    # 3c. amendments (unclaimed or prover-owned, pending states only).
    for node in "${all_nodes[@]}"; do
        if ! pct "$AMEND_PCT"; then continue; fi
        af amend "$node" -s "Amended step $node (round $round)" -o "$PROVER" \
            -d "$WORKSPACE" >/dev/null 2>&1 || true
    done

    # 3d. accepts, deepest-first so children clear before their parents.
    for (( k = total - 1; k >= 0; k-- )); do
        if ! pct "$ACCEPT_PCT"; then continue; fi
        node="${all_nodes[$k]}"
        af accept "$node" --agent "$VERIFIER" --confirm \
            -d "$WORKSPACE" >/dev/null 2>&1 || true
        accept_index=$(( accept_index + 1 ))
    done

    log "synth-workspace: round $round done ($accept_index accept attempts)"
done

events="$(find "$WORKSPACE/ledger" -maxdepth 1 -name '*.json' | wc -l | tr -d ' ')"

# Stable, parseable trailing lines.
echo "WORKSPACE $WORKSPACE"
echo "EVENTS $events"
