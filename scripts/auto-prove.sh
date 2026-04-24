#!/usr/bin/env bash
#
# auto-prove.sh - Automated proof development using headless AI agents
#
# Usage:
#   ./scripts/auto-prove.sh [options]
#
# Options:
#   --max-iterations N    Maximum proof iterations (default: 50)
#   --max-agents N        Maximum total agent calls (default: 100)
#   --delay-seconds N     Delay between agent calls (default: 5)
#   --burst-limit N       Max consecutive calls before longer pause (default: 3)
#   --burst-pause N       Seconds to pause after burst limit (default: 30)
#   --agent-backend NAME  Agent backend to use: claude or codex (default: claude)
#   --codex               Shortcut for --agent-backend codex
#   --claude              Shortcut for --agent-backend claude
#   --dry-run             Show what would be done without calling agents
#   --proof-dir DIR       Directory containing the proof (default: current)
#   --verbose             Show detailed output
#
# Exit codes:
#   0 - Proof completed successfully (all nodes validated)
#   1 - Proof refuted (root node refuted)
#   2 - Max iterations/agents reached
#   3 - No jobs available but proof not complete
#   4 - Error running af commands

set -euo pipefail

# Determine script directory and find af binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Find af binary - check project root first, then PATH
if [[ -x "$PROJECT_ROOT/af" ]]; then
    AF_CMD="$PROJECT_ROOT/af"
elif command -v af &> /dev/null; then
    AF_CMD="af"
else
    AF_CMD=""  # Will be checked later
fi

# Default configuration
MAX_ITERATIONS=50
MAX_AGENTS=100
DELAY_SECONDS=5
BURST_LIMIT=3
BURST_PAUSE=30
DRY_RUN=false
PROOF_DIR="."
VERBOSE=false
AGENT_TIMEOUT=300  # 5 minute timeout per agent call
PARALLEL=5         # number of agents to run concurrently
AGENT_BACKEND="${AF_AGENT_BACKEND:-claude}"

# Counters
ITERATION=0
AGENT_CALLS=0
BURST_COUNT=0

# Colors for output (if terminal supports it)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"
    fi
}

log_success() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')] ✓${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠${NC} $*"
}

log_error() {
    echo -e "${RED}[$(date '+%H:%M:%S')] ✗${NC} $*"
}

usage() {
    awk '
        NR == 1 && /^#!/ { next }
        /^#/ { sub(/^# ?/, "", $0); print; next }
        { exit }
    ' "$0"
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --max-iterations)
            MAX_ITERATIONS="$2"
            shift 2
            ;;
        --max-agents)
            MAX_AGENTS="$2"
            shift 2
            ;;
        --delay-seconds)
            DELAY_SECONDS="$2"
            shift 2
            ;;
        --burst-limit)
            BURST_LIMIT="$2"
            shift 2
            ;;
        --burst-pause)
            BURST_PAUSE="$2"
            shift 2
            ;;
        --agent-backend)
            AGENT_BACKEND="$2"
            shift 2
            ;;
        --codex)
            AGENT_BACKEND="codex"
            shift
            ;;
        --parallel)
            PARALLEL="$2"
            shift 2
            ;;
        --claude)
            AGENT_BACKEND="claude"
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --proof-dir)
            PROOF_DIR="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            log_error "Unknown option: $1"
            exit 4
            ;;
    esac
done

AGENT_BACKEND="${AGENT_BACKEND,,}"

case "$AGENT_BACKEND" in
    claude|codex)
        ;;
    *)
        log_error "Unknown agent backend: $AGENT_BACKEND"
        exit 4
        ;;
esac

# Change to proof directory
cd "$PROOF_DIR" || {
    log_error "Cannot change to proof directory: $PROOF_DIR"
    exit 4
}

# Verify af is available and proof exists
if [[ -z "$AF_CMD" ]]; then
    log_error "af command not found. Please build it first: go build ./cmd/af"
    exit 4
fi

log_verbose "Using af binary: $AF_CMD"

if [[ ! -d "ledger" ]]; then
    log_error "No ledger/ directory found in $PROOF_DIR. Initialize a proof first: $AF_CMD init"
    exit 4
fi

case "$AGENT_BACKEND" in
    claude)
        if ! command -v claude &> /dev/null; then
            log_error "claude command not found. Install Claude Code or rerun with --codex."
            exit 4
        fi
        ;;
    codex)
        if ! command -v codex &> /dev/null; then
            log_error "codex command not found. Install Codex CLI or rerun without --codex."
            exit 4
        fi
        ;;
esac

# Check proof status - returns proof state
check_proof_status() {
    local status_json
    status_json=$($AF_CMD status -f json 2>/dev/null) || return 1

    # Extract root node (id="1") epistemic state
    local root_state
    root_state=$(echo "$status_json" | jq -r '.nodes[] | select(.id == "1") | .epistemic_state // "unknown"')

    # If no root found, return unknown
    if [[ -z "$root_state" ]]; then
        echo "unknown"
    else
        echo "$root_state"
    fi
}

# Get available jobs as JSON
get_jobs() {
    $AF_CMD jobs -f json 2>/dev/null || echo '{"prover_jobs":[],"verifier_jobs":[]}'
}

# Count jobs
count_jobs() {
    local jobs_json="$1"
    local prover_count verifier_count

    prover_count=$(echo "$jobs_json" | jq '.prover_jobs | length')
    verifier_count=$(echo "$jobs_json" | jq '.verifier_jobs | length')

    echo "$((prover_count + verifier_count))"
}

# Track recently-attempted nodes to avoid churning
declare -A RECENT_ATTEMPTS=()  # node_id -> attempt count
RECENT_ATTEMPTS["__init__"]=0  # dummy entry to avoid set -u failures on empty array
MAX_ATTEMPTS_PER_NODE=2     # skip node after this many consecutive failures

# Subtree diversity — avoid drilling down a single branch
LAST_SUBTREE=""
SUBTREE_STREAK=0
MAX_SUBTREE_STREAK=3  # rotate to a different subtree after this many consecutive iterations

# Get the top-level subtree of a node (e.g., "1.10.4.2.1" → "1.10")
get_subtree() {
    echo "$1" | awk -F. '{if(NF>=2) print $1"."$2; else print $0}'
}

# Cache for proof tree structure (refreshed each iteration)
TREE_JSON=""

# Refresh the proof tree cache
refresh_tree() {
    TREE_JSON=$($AF_CMD status -f json 2>/dev/null) || TREE_JSON=""
}

# Check if a node is a leaf (has no children in the proof tree)
is_leaf() {
    local node_id="$1"
    [[ -z "$TREE_JSON" ]] && return 1
    # A child of node X has id "X.<something>" with exactly one more dot-level
    local depth
    depth=$(echo "$node_id" | awk -F. '{print NF}')
    local child_depth=$((depth + 1))
    local has_children
    has_children=$(echo "$TREE_JSON" | jq -r \
        --arg nid "$node_id" \
        --argjson cdepth "$child_depth" \
        '[.nodes[] | select(.id | startswith($nid + ".")) | select(.id | split(".") | length == $cdepth)] | length')
    [[ "$has_children" == "0" ]]
}

# Check if all children of a node are validated/admitted
all_children_validated() {
    local node_id="$1"
    [[ -z "$TREE_JSON" ]] && return 1
    local depth
    depth=$(echo "$node_id" | awk -F. '{print NF}')
    local child_depth=$((depth + 1))
    # Count children that are NOT validated or admitted
    local pending_children
    pending_children=$(echo "$TREE_JSON" | jq -r \
        --arg nid "$node_id" \
        --argjson cdepth "$child_depth" \
        '[.nodes[] | select(.id | startswith($nid + ".")) | select(.id | split(".") | length == $cdepth) | select(.epistemic_state != "validated" and .epistemic_state != "admitted" and .epistemic_state != "archived")] | length')
    [[ "$pending_children" == "0" ]]
}

# Check if a job is actionable given the current tree state
is_actionable() {
    local job_type="$1"
    local node_id="$2"

    if [[ "$job_type" == "prover" ]]; then
        # Prover on a leaf: good — this is actual derivation work
        if is_leaf "$node_id"; then
            return 0
        fi
        # Non-leaf prover: actionable if it has open challenges to resolve
        local open_challenges
        open_challenges=$($AF_CMD challenges -f json -d "$PROOF_DIR" 2>/dev/null \
            | jq "[.challenges[]? | select(.node_id == \"$node_id\" and .status == \"open\")] | length" 2>/dev/null)
        if [[ "${open_challenges:-0}" -gt 0 ]]; then
            log_verbose "Non-leaf prover $node_id has $open_challenges open challenge(s) — actionable" >&2
            return 0
        fi
        log_verbose "Skipping prover job on non-leaf $node_id (no open challenges)" >&2
        return 1
    else
        # Verifier: only actionable if node is a leaf OR all children are validated
        if is_leaf "$node_id"; then
            return 0
        fi
        if all_children_validated "$node_id"; then
            return 0
        fi
        log_verbose "Skipping verifier job on $node_id (children not all validated)" >&2
        return 1
    fi
}

# Select next job — leaves first, skip unactionable nodes and recently-failed ones
# $2 = space-separated list of node IDs to exclude (already dispatched this batch)
select_job() {
    local jobs_json="$1"
    local exclude_list="${2:-}"

    # Refresh tree structure for filtering (only if not cached this batch)
    [[ -z "$TREE_JSON" ]] && refresh_tree

    # NOTE: this function's stdout is captured by the caller, so all log
    # messages must go to stderr (>&2) to avoid corrupting the return value.

    # 1. Prover leaf nodes first (the actual math work)
    #    When subtree streak is high, sort different subtrees first
    local prover_ids
    if [[ $SUBTREE_STREAK -ge $MAX_SUBTREE_STREAK && -n "$LAST_SUBTREE" ]]; then
        prover_ids=$(echo "$jobs_json" | jq -r '.prover_jobs[].node_id' | awk -F. -v last="$LAST_SUBTREE" '{
            subtree = (NF>=2) ? $1"."$2 : $0
            same = (subtree == last) ? 1 : 0
            print same, $0
        }' | sort -k1,1n | awk '{print $2}')
    else
        prover_ids=$(echo "$jobs_json" | jq -r '.prover_jobs[].node_id')
    fi
    while IFS= read -r nid; do
        [[ -z "$nid" || "$nid" == "null" ]] && continue
        [[ " $exclude_list " == *" $nid "* ]] && continue
        local attempts=${RECENT_ATTEMPTS["$nid"]:-0}
        if [[ $attempts -ge $MAX_ATTEMPTS_PER_NODE ]]; then
            log_verbose "Skipping $nid ($attempts recent attempts without progress)" >&2
            continue
        fi
        if ! is_actionable "prover" "$nid"; then
            continue
        fi
        local stmt
        stmt=$(echo "$jobs_json" | jq -r ".prover_jobs[] | select(.node_id == \"$nid\") | .statement")
        echo "prover|$nid|$stmt"
        return 0
    done <<< "$prover_ids"

    # 2. Verifier nodes where all children are done
    #    Sort deepest-first so leaf nodes (the actual math) get priority
    local verifier_ids
    if [[ $SUBTREE_STREAK -ge $MAX_SUBTREE_STREAK && -n "$LAST_SUBTREE" ]]; then
        # Diversify: different subtrees first, then deepest-first within each group
        log_verbose "Subtree diversity: deprioritising $LAST_SUBTREE (streak: $SUBTREE_STREAK)" >&2
        verifier_ids=$(echo "$jobs_json" | jq -r '.verifier_jobs[].node_id' | awk -F. -v last="$LAST_SUBTREE" '{
            subtree = (NF>=2) ? $1"."$2 : $0
            same = (subtree == last) ? 1 : 0
            print same, NF, $0
        }' | sort -k1,1n -k2,2rn | awk '{print $3}')
    else
        verifier_ids=$(echo "$jobs_json" | jq -r '.verifier_jobs[].node_id' | awk -F. '{print NF, $0}' | sort -k1,1rn | cut -d' ' -f2)
    fi
    while IFS= read -r nid; do
        [[ -z "$nid" || "$nid" == "null" ]] && continue
        [[ " $exclude_list " == *" $nid "* ]] && continue
        local attempts=${RECENT_ATTEMPTS["$nid"]:-0}
        if [[ $attempts -ge $MAX_ATTEMPTS_PER_NODE ]]; then
            log_verbose "Skipping $nid ($attempts recent attempts without progress)" >&2
            continue
        fi
        if ! is_actionable "verifier" "$nid"; then
            continue
        fi
        local stmt
        stmt=$(echo "$jobs_json" | jq -r ".verifier_jobs[] | select(.node_id == \"$nid\") | .statement")
        echo "verifier|$nid|$stmt"
        return 0
    done <<< "$verifier_ids"

    # 3. Everything skipped — reset and try once more
    if [[ ${#RECENT_ATTEMPTS[@]} -gt 0 ]]; then
        log "All actionable jobs skipped due to recent failures. Resetting attempt tracker." >&2
        RECENT_ATTEMPTS=()
        select_job "$jobs_json"
        return $?
    fi

    return 1
}

# Record that we attempted a node (call after agent returns)
record_attempt() {
    local node_id="$1"
    local success="$2"  # "true" if the agent made progress

    if [[ "$success" == "true" ]]; then
        # Reset on success
        RECENT_ATTEMPTS["$node_id"]=0
    else
        local prev=${RECENT_ATTEMPTS["$node_id"]:-0}
        RECENT_ATTEMPTS["$node_id"]=$((prev + 1))
    fi
}

# Build agent prompt for a job
build_agent_prompt() {
    local job_type="$1"
    local job_id="$2"

    local context
    context=$($AF_CMD get "$job_id" --checklist 2>/dev/null || $AF_CMD get "$job_id" 2>/dev/null)

    if [[ "$job_type" == "verifier" ]]; then
        cat <<EOF
You are a VERIFIER agent for a mathematical proof. Your job is to rigorously verify or challenge proof node $job_id.

ROLE: You must ATTACK the proof - look for ANY weakness, gap, or error.

CONTEXT:
$context

INSTRUCTIONS:
1. First, claim the node: $AF_CMD claim $job_id --role verifier
2. Read the verification checklist carefully
3. If the proof step is CORRECT and COMPLETE:
   - Run: $AF_CMD accept $job_id --note "Verified: [brief explanation]"
4. If there is ANY issue (gap, error, unclear reasoning):
   - Run: $AF_CMD challenge $job_id --target <target> --severity <severity> --reason "<detailed reason>"
   - Use critical/major for blocking issues, minor/note for suggestions
5. Release the claim if you cannot complete: $AF_CMD release $job_id

Be STRICT. Mathematical proofs must be airtight. If in doubt, challenge.
EOF
    else
        cat <<EOF
You are a PROVER agent for a mathematical proof. Your job is to address challenges on proof node $job_id.

ROLE: You must DEFEND and REFINE the proof - fix issues or provide more detail.

CONTEXT:
$context

INSTRUCTIONS:
1. First, claim the node: $AF_CMD claim $job_id --role prover
2. Review the open challenges on this node
3. For each challenge:
   - If you can fix it: Use $AF_CMD refine, $AF_CMD amend, or other commands
   - If the challenge is resolved: $AF_CMD resolve-challenge <challenge-id> --note "Fixed by..."
   - If the proof step is actually wrong: Consider $AF_CMD archive or $AF_CMD refute
4. After addressing challenges, release: $AF_CMD release $job_id

Be THOROUGH. Address every concern raised by verifiers.
EOF
    fi
}

# Call agent headlessly
run_claude_agent() {
    local prompt_file="$1"
    timeout "${AGENT_TIMEOUT:-300}" claude --print --dangerously-skip-permissions -p "$(cat "$prompt_file")"
}

run_codex_agent() {
    local prompt_file="$1"
    local output_file="$2"
    timeout "${AGENT_TIMEOUT:-300}" codex exec \
        --dangerously-bypass-approvals-and-sandbox \
        -C "$PWD" \
        -o "$output_file" \
        - < "$prompt_file"
}

call_agent() {
    local prompt="$1"
    local job_type="$2"
    local job_id="$3"

    log "Calling $AGENT_BACKEND agent ($job_type for node $job_id)..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_verbose "DRY RUN: Would call agent with prompt (truncated):"
        log_verbose "${prompt:0:200}..."
        return 0
    fi

    local output
    local exit_code=0

    local prompt_file
    local output_file
    local log_file
    prompt_file=$(mktemp)
    output_file=$(mktemp)
    log_file=$(mktemp)
    printf '%s\n' "$prompt" > "$prompt_file"

    case "$AGENT_BACKEND" in
        claude)
            run_claude_agent "$prompt_file" > "$log_file" 2>&1 || exit_code=$?
            output=$(cat "$log_file")
            ;;
        codex)
            run_codex_agent "$prompt_file" "$output_file" > "$log_file" 2>&1 || exit_code=$?
            output=$(cat "$output_file" 2>/dev/null || true)
            if [[ -z "$output" ]]; then
                output=$(cat "$log_file")
            fi
            ;;
    esac

    rm -f "$prompt_file" "$output_file" "$log_file"

    if [[ $exit_code -eq 124 ]]; then
        log_warning "Agent TIMED OUT after ${AGENT_TIMEOUT}s — reaping stale locks"
        $AF_CMD reap 2>/dev/null || true
    elif [[ $exit_code -ne 0 ]]; then
        log_warning "Agent exited with code $exit_code"
        log_verbose "Output: $output"
    else
        log_success "Agent completed successfully"
        log_verbose "Output: ${output:0:500}..."
    fi

    return $exit_code
}

# Rate limiting
apply_rate_limit() {
    BURST_COUNT=$((BURST_COUNT + 1))

    if [[ $BURST_COUNT -ge $BURST_LIMIT ]]; then
        log "Burst limit reached ($BURST_LIMIT calls). Pausing for ${BURST_PAUSE}s..."
        sleep "$BURST_PAUSE"
        BURST_COUNT=0
    else
        log_verbose "Waiting ${DELAY_SECONDS}s before next call..."
        sleep "$DELAY_SECONDS"
    fi
}

# Main loop
main() {
    log "Starting automated proof development"
    log "Configuration:"
    log "  Max iterations: $MAX_ITERATIONS"
    log "  Max agent calls: $MAX_AGENTS"
    log "  Delay between calls: ${DELAY_SECONDS}s"
    log "  Burst limit: $BURST_LIMIT calls, then ${BURST_PAUSE}s pause"
    log "  Agent backend: $AGENT_BACKEND"
    log "  Parallel agents: $PARALLEL"
    log "  Dry run: $DRY_RUN"
    log ""

    while [[ $ITERATION -lt $MAX_ITERATIONS && $AGENT_CALLS -lt $MAX_AGENTS ]]; do
        ITERATION=$((ITERATION + 1))
        log "=== Iteration $ITERATION / $MAX_ITERATIONS (agents: $AGENT_CALLS / $MAX_AGENTS) ==="

        # Check proof status
        local root_state
        root_state=$(check_proof_status)

        case "$root_state" in
            validated)
                log_success "PROOF COMPLETE! Root node is validated."
                $AF_CMD progress
                exit 0
                ;;
            refuted)
                log_error "PROOF REFUTED. Root node has been refuted."
                $AF_CMD status
                exit 1
                ;;
            admitted)
                log_warning "Proof admitted (contains taint). Continuing..."
                ;;
        esac

        # Get available jobs
        local jobs_json
        jobs_json=$(get_jobs)

        local job_count
        job_count=$(count_jobs "$jobs_json")

        if [[ $job_count -eq 0 ]]; then
            # No jobs but proof not complete - might be stuck
            log_warning "No jobs available but proof not complete"
            log "Checking for stuck states..."
            $AF_CMD health || true

            # Wait and retry a few times
            local stuck_retries=3
            while [[ $stuck_retries -gt 0 && $job_count -eq 0 ]]; do
                log "Waiting 10s and retrying..."
                sleep 10

                # Try to reap stale locks
                $AF_CMD reap 2>/dev/null || true

                jobs_json=$(get_jobs)
                job_count=$(count_jobs "$jobs_json")
                stuck_retries=$((stuck_retries - 1))
            done

            if [[ $job_count -eq 0 ]]; then
                log_error "Still no jobs available. Proof may be stuck."
                $AF_CMD status
                exit 3
            fi
        fi

        log "Found $job_count available jobs"
        log_verbose "Jobs JSON: $jobs_json"

        # Refresh tree once for this batch
        refresh_tree

        # Select up to PARALLEL jobs for this batch
        local batch_pids=()
        local batch_nodes=()
        local exclude_list=""
        local events_before
        events_before=$($AF_CMD log 2>/dev/null | wc -l)

        local batch_size=$PARALLEL
        local remaining=$((MAX_AGENTS - AGENT_CALLS))
        [[ $remaining -lt $batch_size ]] && batch_size=$remaining

        for ((b=0; b < batch_size; b++)); do
            local job_info
            if ! job_info=$(select_job "$jobs_json" "$exclude_list"); then
                [[ $b -eq 0 ]] && log_error "Failed to select any jobs"
                break
            fi

            local job_type job_id job_statement
            IFS='|' read -r job_type job_id job_statement <<< "$job_info"

            log "Dispatching: $job_type for node $job_id"

            # Track subtree for diversity
            local selected_subtree
            selected_subtree=$(get_subtree "$job_id")
            if [[ "$selected_subtree" == "$LAST_SUBTREE" ]]; then
                SUBTREE_STREAK=$((SUBTREE_STREAK + 1))
            else
                LAST_SUBTREE="$selected_subtree"
                SUBTREE_STREAK=1
            fi

            exclude_list="${exclude_list:+$exclude_list }$job_id"

            local prompt
            prompt=$(build_agent_prompt "$job_type" "$job_id")

            AGENT_CALLS=$((AGENT_CALLS + 1))

            if [[ "$DRY_RUN" == "true" ]]; then
                log_verbose "DRY RUN: Would call $job_type agent for $job_id"
                batch_nodes+=("$job_id")
            else
                # Launch agent in background
                call_agent "$prompt" "$job_type" "$job_id" &
                batch_pids+=($!)
                batch_nodes+=("$job_id")
            fi
        done

        if [[ ${#batch_nodes[@]} -eq 0 ]]; then
            continue
        fi

        log "Waiting for ${#batch_nodes[@]} parallel agents: ${batch_nodes[*]}"

        # Wait for all agents in this batch
        for pid in "${batch_pids[@]}"; do
            wait "$pid" || true
        done

        # Check batch progress
        local events_after
        events_after=$($AF_CMD log 2>/dev/null | wc -l)
        local new_events=$((events_after - events_before))
        log_success "Batch complete: ${#batch_nodes[@]} agents, $new_events ledger events"

        # Reap any stale locks from timed-out agents
        $AF_CMD reap 2>/dev/null || true

        # Brief pause between batches
        if [[ $ITERATION -lt $MAX_ITERATIONS && $AGENT_CALLS -lt $MAX_AGENTS ]]; then
            sleep "$DELAY_SECONDS"
        fi
    done

    # Reached limits
    log_warning "Reached iteration/agent limits"
    log "Final status:"
    $AF_CMD progress || true
    $AF_CMD status || true
    exit 2
}

# Run main
main
