#!/usr/bin/env bash
#
# auto-prove.sh - Automated proof development using headless Claude agents
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
    grep '^#' "$0" | grep -v '#!/' | sed 's/^# \?//'
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
declare -A RECENT_ATTEMPTS  # node_id -> attempt count
MAX_ATTEMPTS_PER_NODE=2     # skip node after this many consecutive failures

# Pick a job from a list, skipping recently-failed nodes
# Usage: pick_from_list "$json_array_expr" "$job_type"
pick_from_list() {
    local jobs_json="$1"
    local jq_expr="$2"
    local job_type="$3"

    local node_ids
    node_ids=$(echo "$jobs_json" | jq -r "$jq_expr")

    local nid
    while IFS= read -r nid; do
        [[ -z "$nid" || "$nid" == "null" ]] && continue
        local attempts=${RECENT_ATTEMPTS["$nid"]:-0}
        if [[ $attempts -lt $MAX_ATTEMPTS_PER_NODE ]]; then
            local stmt
            stmt=$(echo "$jobs_json" | jq -r ".${job_type}_jobs[] | select(.node_id == \"$nid\") | .statement")
            echo "$job_type|$nid|$stmt"
            return 0
        else
            log_verbose "Skipping $nid ($attempts recent attempts without progress)"
        fi
    done <<< "$node_ids"

    return 1
}

# Select next job (prioritizes prover leaf work, then verifier)
select_job() {
    local jobs_json="$1"

    # 1. Prover jobs first — this is where the actual work happens
    #    Recommended prover, then all provers in order
    if pick_from_list "$jobs_json" \
        '(.prover_jobs[] | select(.recommended == true) | .node_id), (.prover_jobs[].node_id)' \
        "prover"; then
        return 0
    fi

    # 2. Verifier jobs — only after all prover work is exhausted or skipped
    #    Recommended verifier, then all verifiers in order
    if pick_from_list "$jobs_json" \
        '(.verifier_jobs[] | select(.recommended == true) | .node_id), (.verifier_jobs[].node_id)' \
        "verifier"; then
        return 0
    fi

    # 3. Nothing available (or everything skipped) — reset attempts and try once more
    if [[ ${#RECENT_ATTEMPTS[@]} -gt 0 ]]; then
        log "All jobs skipped due to recent failures. Resetting attempt tracker."
        RECENT_ATTEMPTS=()
        # Retry with clean slate (recursive, but only once since we just cleared)
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

# Call Claude agent headlessly
call_agent() {
    local prompt="$1"
    local job_type="$2"
    local job_id="$3"

    log "Calling Claude agent ($job_type for node $job_id)..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_verbose "DRY RUN: Would call agent with prompt (truncated):"
        log_verbose "${prompt:0:200}..."
        return 0
    fi

    # Call Claude Code in headless mode
    # Using --print to just output the result, -p for prompt
    local output
    local exit_code=0

    # Create a temporary file for the prompt (handles special chars better)
    local prompt_file
    prompt_file=$(mktemp)
    echo "$prompt" > "$prompt_file"

    output=$(claude --print --dangerously-skip-permissions -p "$(cat "$prompt_file")" 2>&1) || exit_code=$?

    rm -f "$prompt_file"

    if [[ $exit_code -ne 0 ]]; then
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

        # Select next job
        local job_info
        if ! job_info=$(select_job "$jobs_json"); then
            log_error "Failed to select job"
            continue
        fi

        local job_type job_id job_statement
        IFS='|' read -r job_type job_id job_statement <<< "$job_info"

        log "Selected: $job_type job for node $job_id"
        log_verbose "Statement: $job_statement"

        # Snapshot ledger event count before agent runs
        local events_before
        events_before=$($AF_CMD log 2>/dev/null | wc -l)

        # Build and execute agent
        local prompt
        prompt=$(build_agent_prompt "$job_type" "$job_id")

        AGENT_CALLS=$((AGENT_CALLS + 1))

        if call_agent "$prompt" "$job_type" "$job_id"; then
            log_success "Agent call $AGENT_CALLS completed"
        else
            log_warning "Agent call $AGENT_CALLS had issues (continuing anyway)"
        fi

        # Check if agent made progress (new ledger events beyond claim/release)
        local events_after
        events_after=$($AF_CMD log 2>/dev/null | wc -l)
        local new_events=$((events_after - events_before))

        # claim + release = 2 events with no real work; >2 means something happened
        if [[ $new_events -gt 2 ]]; then
            log_success "Node $job_id: agent produced $new_events events (progress!)"
            record_attempt "$job_id" "true"
        else
            log_warning "Node $job_id: agent produced only $new_events events (no progress)"
            record_attempt "$job_id" "false"
        fi

        # Apply rate limiting
        if [[ $ITERATION -lt $MAX_ITERATIONS && $AGENT_CALLS -lt $MAX_AGENTS ]]; then
            apply_rate_limit
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
