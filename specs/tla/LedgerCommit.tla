---------------------------- MODULE LedgerCommit ----------------------------
(***************************************************************************)
(* Exploratory smoketest model of af's write path (bead vibefeld-w2mt.1,   *)
(* seed for vibefeld-8fm6). See README.md for the step-to-Go mapping.      *)
(*                                                                         *)
(* Writers run service.commit: load state without the lock, create         *)
(* ledger.lock with O_CREAT|O_EXCL and a fresh token, compare the ledger   *)
(* tail with the tail they loaded (CAS), publish the batch one event file  *)
(* per step (os.Rename), then remove the lock if the token on disk is      *)
(* still theirs. A CAS conflict is retried with a fresh load.              *)
(* Reapers run `af reap --ledger-lock` (ledger.RemoveIfStale): read the    *)
(* lock, judge it stale, re-read it, unlink the path.                      *)
(* Readers list the ledger directory without the lock (ledger.Scan).       *)
(* A writer process may die at any step, leaving its lock file behind.     *)
(*                                                                         *)
(* One step of a process is one filesystem call; each step is atomic.     *)
(* Not modelled: OS crash / fsync durability, temp files, event contents,  *)
(* lock-acquire polling (a blocked acquire may time out at any time).      *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets

CONSTANTS
  Writers,        \* writer processes; each performs one commit
  Reapers,        \* concurrent `af reap --ledger-lock` runs
  Readers,        \* lock-free readers (LoadState / Scan)
  BatchSize,      \* events per commit
  MaxAttempts,    \* commit attempts per writer (commitRetry n+1)
  MaxCrashes,     \* how many writer processes may die
  PidVisible,     \* TRUE: a reaper can tell whether the holder pid is alive
  NoReplace,      \* TRUE: publishing refuses to overwrite (link / RENAME_NOREPLACE)
  AtomicReaddir   \* TRUE: a directory listing is one atomic snapshot

ASSUME /\ BatchSize \in Nat \ {0}
       /\ MaxAttempts \in Nat \ {0}
       /\ MaxCrashes \in Nat
       /\ {PidVisible, NoReplace, AtomicReaddir} \subseteq BOOLEAN

Procs   == Writers \cup Reapers \cup Readers
MaxSeq  == Cardinality(Writers) * BatchSize
Seqs    == 1..MaxSeq
NoEvent == <<>>          \* no event file at this sequence number
NoLock  == <<>>          \* no ledger.lock file
\* An event is <<w, i>>: the i-th event of writer w's batch.
\* A lock token is <<w, a>>: written by writer w on its a-th attempt.

VARIABLES
  files,     \* seq -> event file at that sequence number, or NoEvent
  lock,      \* token in ledger.lock, or NoLock
  alive,     \* writer -> its process is alive
  crashes,   \* writer deaths so far
  pc,        \* process -> program counter
  attempt,   \* writer -> commit attempts started
  loaded,    \* writer -> ledger tail seen by its last load
  snap,      \* writer -> ledger contents seen by its last load
  base,      \* writer -> first sequence number of its batch
  k,         \* writer -> index of the next event to publish
  outcome,   \* writer -> "none" | "ok" | "conflict" | "error"
  acked,     \* writer -> commit reported success to its caller
  aba,       \* writer -> its CAS passed although the loaded prefix changed
  seen,      \* reaper -> token read by its first read
  visited,   \* reader -> sequence numbers already listed
  listing,   \* reader -> sequence numbers its listing returned
  reaped,    \* ghost: some reaper has unlinked a lock file
  ackAfterReap \* ghost: writer -> its commit succeeded after a reap

vars == <<files, lock, alive, crashes, pc, attempt, loaded, snap, base, k,
          outcome, acked, aba, seen, visited, listing, reaped, ackAfterReap>>
ghosts == <<reaped, ackAfterReap>>

Present == {s \in Seqs : files[s] # NoEvent}
Max(S)  == CHOOSE m \in S : \A x \in S : x <= m
\* ledger.NextSequence is max(seq)+1, so the tail is the largest file present.
Tail    == IF Present = {} THEN 0 ELSE Max(Present)
Holder(t) == t[1]
\* staleLockReason: a live pid is never stale. A hidden pid (another pid
\* namespace or host) looks dead.
Stale(t) == IF PidVisible THEN ~alive[Holder(t)] ELSE TRUE

Init ==
  /\ files   = [s \in Seqs |-> NoEvent]
  /\ lock    = NoLock
  /\ alive   = [w \in Writers |-> TRUE]
  /\ crashes = 0
  /\ pc      = [p \in Procs |-> IF p \in Writers THEN "load"
                                ELSE IF p \in Reapers THEN "read1" ELSE "list"]
  /\ attempt = [w \in Writers |-> 0]
  /\ loaded  = [w \in Writers |-> 0]
  /\ snap    = [w \in Writers |-> [s \in Seqs |-> NoEvent]]
  /\ base    = [w \in Writers |-> 0]
  /\ k       = [w \in Writers |-> 1]
  /\ outcome = [w \in Writers |-> "none"]
  /\ acked   = [w \in Writers |-> FALSE]
  /\ aba     = [w \in Writers |-> FALSE]
  /\ seen    = [r \in Reapers |-> NoLock]
  /\ visited = [r \in Readers |-> {}]
  /\ listing = [r \in Readers |-> {}]
  /\ reaped  = FALSE
  /\ ackAfterReap = [w \in Writers |-> FALSE]

-----------------------------------------------------------------------------
(* Writers: service.commit -> ledger.AppendBatchIfSequence                 *)

\* LoadState, outside the lock.
Load(w) ==
  /\ pc[w] = "load" /\ alive[w]
  /\ loaded'  = [loaded  EXCEPT ![w] = Tail]
  /\ snap'    = [snap    EXCEPT ![w] = files]
  /\ attempt' = [attempt EXCEPT ![w] = @ + 1]
  /\ outcome' = [outcome EXCEPT ![w] = "none"]
  /\ pc'      = [pc      EXCEPT ![w] = "acquire"]
  /\ UNCHANGED <<files, lock, alive, crashes, base, k, acked, aba, seen,
                 visited, listing>>
  /\ UNCHANGED ghosts

\* LedgerLock.tryAcquire: O_CREAT|O_EXCL succeeds only if no lock file exists.
Acquire(w) ==
  /\ pc[w] = "acquire" /\ alive[w] /\ lock = NoLock
  /\ lock' = <<w, attempt[w]>>
  /\ pc'   = [pc EXCEPT ![w] = "cas"]
  /\ UNCHANGED <<files, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen, visited, listing>>
  /\ UNCHANGED ghosts

\* LedgerLock.Acquire: "timeout waiting for lock" (retriable, exit 1).
AcquireTimeout(w) ==
  /\ pc[w] = "acquire" /\ alive[w] /\ lock # NoLock
  /\ pc' = [pc EXCEPT ![w] = "failed"]
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen, visited, listing>>
  /\ UNCHANGED ghosts

\* NextSequence + tail check under the lock.
Cas(w) ==
  /\ pc[w] = "cas" /\ alive[w]
  /\ IF Tail = loaded[w]
       THEN /\ base'    = [base EXCEPT ![w] = Tail + 1]
            /\ k'       = [k    EXCEPT ![w] = 1]
            /\ aba'     = [aba  EXCEPT ![w] = \E s \in 1..Tail : files[s] # snap[w][s]]
            /\ pc'      = [pc   EXCEPT ![w] = "publish"]
            /\ outcome' = outcome
       ELSE /\ outcome' = [outcome EXCEPT ![w] = "conflict"]   \* ErrSequenceMismatch
            /\ pc'      = [pc      EXCEPT ![w] = "release"]
            /\ UNCHANGED <<base, k, aba>>
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, acked,
                 seen, visited, listing>>
  /\ UNCHANGED ghosts

\* os.Rename(temp, final) for one event (+ fsyncDir). Plain rename replaces an
\* existing file; with NoReplace the publish fails, leaving the published
\* prefix in place, and the commit returns an error.
Publish(w) ==
  /\ pc[w] = "publish" /\ alive[w]
  /\ LET s == base[w] + k[w] - 1 IN
       /\ s \in Seqs
       /\ IF NoReplace /\ files[s] # NoEvent
            THEN /\ outcome' = [outcome EXCEPT ![w] = "error"]
                 /\ pc'      = [pc      EXCEPT ![w] = "release"]
                 /\ UNCHANGED <<files, k>>
            ELSE /\ files' = [files EXCEPT ![s] = <<w, k[w]>>]
                 /\ IF k[w] = BatchSize
                      THEN /\ outcome' = [outcome EXCEPT ![w] = "ok"]
                           /\ pc'      = [pc      EXCEPT ![w] = "release"]
                           /\ k'       = k
                      ELSE /\ k'       = [k EXCEPT ![w] = @ + 1]
                           /\ UNCHANGED <<outcome, pc>>
  /\ UNCHANGED <<lock, alive, crashes, attempt, loaded, snap, base, acked, aba,
                 seen, visited, listing>>
  /\ UNCHANGED ghosts

\* Deferred LedgerLock.Release: remove the file only if it carries our token
\* (otherwise log "ownership mismatch" and leave it). Then report the result;
\* a conflict is retried by commitRetry with a fresh load.
Release(w) ==
  /\ pc[w] = "release" /\ alive[w]
  /\ lock' = (IF lock = <<w, attempt[w]>> THEN NoLock ELSE lock)
  /\ acked' = [acked EXCEPT ![w] = (outcome[w] = "ok")]
  /\ pc' = [pc EXCEPT ![w] =
              CASE outcome[w] = "ok"                                  -> "done"
                [] outcome[w] = "conflict" /\ attempt[w] < MaxAttempts -> "load"
                [] OTHER                                               -> "failed"]
  /\ ackAfterReap' = [ackAfterReap EXCEPT ![w] = (outcome[w] = "ok") /\ reaped]
  /\ UNCHANGED <<files, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, aba, seen, visited, listing, reaped>>

\* The writer process dies (kill -9, OOM, agent timeout). Its lock file stays.
Crash(w) ==
  /\ alive[w] /\ pc[w] \notin {"done", "failed"} /\ crashes < MaxCrashes
  /\ alive'   = [alive EXCEPT ![w] = FALSE]
  /\ pc'      = [pc    EXCEPT ![w] = "crashed"]
  /\ crashes' = crashes + 1
  /\ UNCHANGED <<files, lock, attempt, loaded, snap, base, k, outcome, acked,
                 aba, seen, visited, listing>>
  /\ UNCHANGED ghosts

-----------------------------------------------------------------------------
(* Reapers: `af reap --ledger-lock` -> ledger.RemoveIfStale                *)

\* Read the lock file and judge staleness.
ReadLock(r) ==
  /\ pc[r] = "read1"
  /\ IF lock # NoLock /\ Stale(lock)
       THEN /\ seen' = [seen EXCEPT ![r] = lock]
            /\ pc'   = [pc   EXCEPT ![r] = "read2"]
       ELSE /\ pc'   = [pc   EXCEPT ![r] = "done"]
            /\ seen' = seen
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, visited, listing>>
  /\ UNCHANGED ghosts

\* Re-read and compare identity (sameLockForRemoval: same token).
ReRead(r) ==
  /\ pc[r] = "read2"
  /\ pc' = [pc EXCEPT ![r] = IF lock = seen[r] THEN "unlink" ELSE "done"]
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen, visited, listing>>
  /\ UNCHANGED ghosts

\* os.Remove(path): unlinks whatever lock file is there now.
Unlink(r) ==
  /\ pc[r] = "unlink"
  /\ lock' = NoLock
  /\ pc'   = [pc EXCEPT ![r] = "done"]
  /\ reaped' = TRUE
  /\ UNCHANGED <<files, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen, visited, listing, ackAfterReap>>

-----------------------------------------------------------------------------
(* Readers: listEventSequences (os.ReadDir), no lock                       *)

ListAtomic(r) ==
  /\ AtomicReaddir /\ pc[r] = "list"
  /\ listing' = [listing EXCEPT ![r] = Present]
  /\ pc'      = [pc      EXCEPT ![r] = "done"]
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen, visited>>
  /\ UNCHANGED ghosts

\* POSIX leaves it unspecified whether readdir returns an entry added after
\* opendir. Model the scan as visiting directory slots in arbitrary order.
ListStep(r) ==
  /\ ~AtomicReaddir /\ pc[r] = "list"
  /\ \E s \in Seqs \ visited[r] :
       /\ visited' = [visited EXCEPT ![r] = @ \cup {s}]
       /\ listing' = [listing EXCEPT ![r] =
                        IF files[s] # NoEvent THEN @ \cup {s} ELSE @]
       /\ pc' = [pc EXCEPT ![r] =
                   IF visited[r] \cup {s} = Seqs THEN "done" ELSE "list"]
  /\ UNCHANGED <<files, lock, alive, crashes, attempt, loaded, snap, base, k,
                 outcome, acked, aba, seen>>
  /\ UNCHANGED ghosts

-----------------------------------------------------------------------------

Terminated == \A p \in Procs : pc[p] \in {"done", "failed", "crashed"}

Next ==
  \/ \E w \in Writers : \/ Load(w) \/ Acquire(w) \/ AcquireTimeout(w) \/ Cas(w)
                        \/ Publish(w) \/ Release(w) \/ Crash(w)
  \/ \E r \in Reapers : ReadLock(r) \/ ReRead(r) \/ Unlink(r)
  \/ \E r \in Readers : ListAtomic(r) \/ ListStep(r)
  \/ Terminated /\ UNCHANGED vars

Spec == Init /\ [][Next]_vars

-----------------------------------------------------------------------------
(* Properties                                                              *)

InCS(w) == alive[w] /\ pc[w] \in {"cas", "publish"}

\* At most one live writer between acquiring ledger.lock and releasing it.
MutualExclusion == \A v, w \in Writers : v # w => ~(InCS(v) /\ InCS(w))

\* A commit reported as successful stays in the ledger, at the sequence
\* numbers it was assigned.
NoLostAck ==
  \A w \in Writers : acked[w] =>
    \A i \in 1..BatchSize : files[base[w] + i - 1] = <<w, i>>

\* The ledger never has a gap (replay rejects gaps as corruption).
NoGap == Present = 1..Cardinality(Present)

\* A writer's events that are present are a prefix of its batch, at
\* consecutive sequence numbers from its base ("prefix on crash").
BatchPrefix ==
  \A w \in Writers :
    LET mine == {s \in Present : Holder(files[s]) = w} IN
      \/ mine = {}
      \/ \E j \in 1..BatchSize :
           /\ mine = base[w]..(base[w] + j - 1)
           /\ \A s \in mine : files[s] = <<w, s - base[w] + 1>>

\* A CAS that passes means the writer's loaded state is still the ledger
\* prefix; the tail number alone must not be able to hide a rewrite (ABA).
NoABA == \A w \in Writers : ~aba[w]

\* A lock-free reader never sees a gap in a healthy ledger.
ReaderSeesNoGap ==
  \A r \in Readers : pc[r] = "done" => listing[r] = 1..Cardinality(listing[r])

-----------------------------------------------------------------------------
(* Witnesses: things that SHOULD happen. Each is stated as "never", so the  *)
(* model is only trusted when TLC finds a trace violating it (no vacuity).  *)

\* Every writer can commit.
Witness_AllCommit == ~(\A w \in Writers : acked[w])

\* A lock left by a dead writer is reaped and a later commit succeeds.
Witness_CommitAfterReap == ~(\E w \in Writers : ackAfterReap[w])
=============================================================================
