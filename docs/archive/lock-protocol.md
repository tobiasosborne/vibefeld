# Lock Protocol for External Tools

This document describes the lock protocols used by the `af` CLI tool. External tools that need to coordinate with `af` proof operations must understand these protocols to ensure data integrity.

## Overview

AF uses two distinct locking mechanisms:

1. **Ledger Write Lock** - Serializes all write operations to the event ledger
2. **Claim Locks** - Manage exclusive agent access to proof nodes

These serve different purposes and operate independently.

---

## 1. Ledger Write Lock

The ledger write lock ensures that all event append operations are serialized. This is critical for maintaining the integrity of the append-only event log.

### File Location

```
<proof_dir>/ledger/ledger.lock
```

### Lock File Format

The lock file is a JSON document:

```json
{
  "agent_id": "string",
  "acquired_at": "2024-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Identifier of the agent holding the lock |
| `acquired_at` | ISO 8601 timestamp | When the lock was acquired |

### Acquisition Protocol

The ledger lock uses **atomic file creation** via POSIX `O_CREATE|O_EXCL` flags:

1. Attempt to create the lock file with `O_CREATE|O_EXCL|O_WRONLY` mode 0600
2. If creation succeeds:
   - Write the metadata JSON to the file
   - Lock is acquired
3. If creation fails with EEXIST:
   - Another process holds the lock
   - Wait `10ms` and retry
4. Continue retrying until timeout (default: 5 seconds)

### Release Protocol

1. Delete the lock file using `os.Remove()`
2. The lock is released

### Timeout Behavior

- Default acquisition timeout: **5 seconds**
- Poll interval: **10 milliseconds**
- No automatic lock expiration (locks persist until explicitly released or process crash)

### Recovery from Crashed Processes

If a process crashes while holding the ledger lock, the lock file will remain on disk. External tools can:

1. Check the lock file's `acquired_at` timestamp
2. If the lock is "too old" (e.g., > 30 seconds), assume the holder crashed
3. Delete the stale lock file
4. Proceed with acquisition

**Warning**: Only do this if you are certain the original holder has crashed. Incorrect stale lock detection can cause data corruption.

### Example: Acquiring the Ledger Lock

```go
// Constants from af implementation
const lockFileName = "ledger.lock"
const pollInterval = 10 * time.Millisecond
const defaultTimeout = 5 * time.Second

func acquireLedgerLock(ledgerDir, agentID string, timeout time.Duration) error {
    lockPath := filepath.Join(ledgerDir, lockFileName)
    deadline := time.Now().Add(timeout)

    for {
        // Try atomic file creation
        f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
        if err == nil {
            // Write metadata
            meta := map[string]interface{}{
                "agent_id":    agentID,
                "acquired_at": time.Now().UTC().Format(time.RFC3339),
            }
            json.NewEncoder(f).Encode(meta)
            f.Close()
            return nil // Lock acquired
        }

        if time.Now().After(deadline) {
            return errors.New("timeout waiting for lock")
        }

        time.Sleep(pollInterval)
    }
}

func releaseLedgerLock(ledgerDir string) error {
    lockPath := filepath.Join(ledgerDir, lockFileName)
    return os.Remove(lockPath)
}
```

---

## 2. Claim Locks (Node Locks)

Claim locks manage exclusive access to proof nodes. When an agent claims a node to work on it, a claim lock prevents other agents from modifying that node.

### Storage

Claim locks are stored as **events in the ledger**, not as separate files. The relevant event types are:

- `lock_acquired` - Agent acquired a claim on a node
- `lock_released` - Agent released a claim on a node
- `lock_reaped` - Stale lock was garbage collected

### Event Format: lock_acquired

```json
{
  "type": "lock_acquired",
  "timestamp": "2024-01-15T10:30:00Z",
  "node_id": "1.2.3",
  "owner": "agent-uuid-1234",
  "expires_at": "2024-01-15T10:35:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"lock_acquired"` |
| `timestamp` | ISO 8601 | When the lock was acquired |
| `node_id` | string | Hierarchical node ID (e.g., "1.2.3") |
| `owner` | string | Agent identifier |
| `expires_at` | ISO 8601 | When the lock expires |

### Event Format: lock_released

```json
{
  "type": "lock_released",
  "timestamp": "2024-01-15T10:32:00Z",
  "node_id": "1.2.3",
  "owner": "agent-uuid-1234"
}
```

### Event Format: lock_reaped

```json
{
  "type": "lock_reaped",
  "timestamp": "2024-01-15T10:35:01Z",
  "node_id": "1.2.3",
  "owner": "agent-uuid-1234"
}
```

### Lock Timeout Configuration

The claim lock timeout is configurable per-proof in `meta.json`:

```json
{
  "lock_timeout": "5m"
}
```

- Default: **5 minutes**
- Valid range: **1 second to 1 hour**

### Claim Lock State Reconstruction

To determine current lock state, replay all lock events from the ledger:

```python
# Pseudocode for reconstructing lock state
locks = {}  # node_id -> lock_info

for event in replay_ledger():
    if event.type == "lock_acquired":
        locks[event.node_id] = {
            "owner": event.owner,
            "acquired_at": event.timestamp,
            "expires_at": event.expires_at
        }
    elif event.type in ["lock_released", "lock_reaped"]:
        del locks[event.node_id]

# Filter out expired locks
now = current_time()
active_locks = {
    node_id: lock
    for node_id, lock in locks.items()
    if parse_time(lock["expires_at"]) > now
}
```

### Lock File Format (File-Based Locks)

In addition to ledger events, AF may use file-based claim locks in a `locks/` directory. Each lock file uses this JSON format:

```json
{
  "node_id": "1.2.3",
  "owner": "agent-uuid-1234",
  "acquired_at": "2024-01-15T10:30:00.123456789Z",
  "expires_at": "2024-01-15T10:35:00.123456789Z"
}
```

**Note**: Timestamps use RFC3339Nano format (nanosecond precision) for accurate expiration tracking.

### Stale Lock Cleanup

AF periodically reaps stale (expired) locks:

1. Scan lock files in the `locks/` directory
2. Parse each `.lock` file
3. If `expires_at < now`, the lock is stale
4. Remove the lock file
5. Append a `lock_reaped` event to the ledger

External tools should **not** perform their own lock cleanup - let AF handle it to ensure ledger consistency.

---

## 3. Concurrency Model: Optimistic Locking

Beyond file locks, AF uses **Compare-And-Swap (CAS)** semantics for write operations. This provides optimistic concurrency control.

### How It Works

1. Load current state and note the latest event sequence number
2. Perform validation on the loaded state
3. Attempt to append event with `AppendIfSequence(event, expectedSeq)`
4. If sequence matches, write succeeds
5. If sequence changed (another writer intervened), get `ErrSequenceMismatch`
6. Retry from step 1

### Sequence Number Tracking

Event files in the ledger are named with zero-padded sequence numbers:

```
ledger/
  event-000001.json
  event-000002.json
  event-000003.json
  ...
```

The current sequence is determined by counting event files or parsing the highest filename.

### Example: Safe Write Pattern

```go
func safeAppend(ledgerDir string, event Event) error {
    maxRetries := 3

    for i := 0; i < maxRetries; i++ {
        // Load current sequence
        currentSeq, _ := countEvents(ledgerDir)

        // Try to append with CAS
        _, err := AppendIfSequence(ledgerDir, event, currentSeq)
        if err == nil {
            return nil // Success
        }

        if errors.Is(err, ErrSequenceMismatch) {
            // Concurrent modification - retry
            continue
        }

        return err // Other error
    }

    return errors.New("max retries exceeded")
}
```

---

## 4. Integration Guidelines for External Tools

### Reading Proof State (Read-Only)

For tools that only read proof state:

1. **No locking required** - Event files are immutable once written
2. Replay all events to reconstruct state
3. Be aware state may change between reads

### Modifying Proof State

For tools that write to the proof:

1. **Always acquire the ledger lock first**
2. Use CAS semantics (`AppendIfSequence`) for writes
3. Release the lock after write completes
4. Handle `ErrSequenceMismatch` with retry logic

### Coordinating with Running AF Agents

If AF agents are actively running:

1. Respect claim locks - don't modify nodes claimed by other agents
2. Use short lock timeouts to minimize contention
3. Implement exponential backoff for retries

### Atomic Write Pattern

All writes should follow the atomic write pattern:

1. Write to a temporary file (`.event-*.tmp`)
2. Call `fsync()` to ensure data is on disk
3. Atomic rename to final path (`event-NNNNNN.json`)

This ensures crash safety - partially written events won't corrupt the ledger.

---

## 5. Quick Reference

### File Paths

| Path | Purpose |
|------|---------|
| `<proof>/ledger/ledger.lock` | Ledger write serialization |
| `<proof>/ledger/event-NNNNNN.json` | Event files |
| `<proof>/locks/*.lock` | File-based claim locks (optional) |
| `<proof>/meta.json` | Configuration including lock_timeout |

### Timeouts

| Operation | Default | Configurable |
|-----------|---------|--------------|
| Ledger lock acquisition | 5 seconds | No (hardcoded) |
| Ledger lock poll interval | 10 ms | No (hardcoded) |
| Claim lock duration | 5 minutes | Yes (meta.json) |
| Claim lock range | 1s - 1h | N/A |

### Error Codes

| Error | Meaning | Action |
|-------|---------|--------|
| `ErrSequenceMismatch` | Concurrent modification | Retry operation |
| `timeout waiting for lock` | Lock acquisition failed | Retry with backoff |
| `node already locked` | Claim lock exists | Wait or work on different node |

---

## 6. Example Workflows

### External Tool: Adding an Event

```bash
#!/bin/bash
# Example: External tool appending an event to the ledger

PROOF_DIR="/path/to/proof"
LEDGER_DIR="$PROOF_DIR/ledger"
LOCK_FILE="$LEDGER_DIR/ledger.lock"

# 1. Acquire lock (simplified - production should use proper atomic creation)
while ! mkdir "$LOCK_FILE" 2>/dev/null; do
    sleep 0.01
done

# 2. Get next sequence number
NEXT_SEQ=$(ls "$LEDGER_DIR"/event-*.json 2>/dev/null | wc -l)
NEXT_SEQ=$((NEXT_SEQ + 1))

# 3. Write event to temp file
TEMP_FILE=$(mktemp "$LEDGER_DIR/.event-XXXXX.tmp")
echo '{"type":"custom_event","timestamp":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}' > "$TEMP_FILE"

# 4. Atomic rename
mv "$TEMP_FILE" "$LEDGER_DIR/event-$(printf '%06d' $NEXT_SEQ).json"

# 5. Release lock
rmdir "$LOCK_FILE"
```

### Checking Lock Status

```go
// Check if a node is currently claimed
func isNodeClaimed(ledgerDir string, nodeID string) (bool, string, error) {
    // Replay ledger to get current lock state
    locks := make(map[string]lockInfo)

    events, _ := readAllEvents(ledgerDir)
    for _, event := range events {
        switch e := event.(type) {
        case LockAcquired:
            locks[e.NodeID] = lockInfo{
                Owner:     e.Owner,
                ExpiresAt: e.ExpiresAt,
            }
        case LockReleased, LockReaped:
            delete(locks, e.NodeID)
        }
    }

    // Check if lock exists and is not expired
    if lock, ok := locks[nodeID]; ok {
        if time.Now().Before(lock.ExpiresAt) {
            return true, lock.Owner, nil
        }
    }

    return false, "", nil
}
```

---

## 7. Safety Guarantees

The AF lock protocol provides these guarantees:

1. **Write Serialization**: All ledger writes are serialized through the ledger lock
2. **No Lost Updates**: CAS semantics prevent lost updates from concurrent writers
3. **Crash Safety**: Atomic writes ensure the ledger is never corrupted by crashes
4. **Claim Exclusivity**: Only one agent can claim a node at a time
5. **Lock Expiration**: Claim locks auto-expire, preventing indefinite blocking

The protocol does **not** guarantee:

1. Fairness - Lock acquisition is not FIFO
2. Priority - All agents have equal lock access
3. Starvation prevention - A fast writer could theoretically starve others
