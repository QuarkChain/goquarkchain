# Incident Report: Shard 40001 Sync Failure — "Unknown Ancestor" & Nil Pointer Panic

**Date:** 2026-05-18
**Affected Component:** Shard 40001 (Chain 4, Shard 0), Minor Block Sync
**Severity:** Critical — node stuck, unable to sync
**Branch:** `fix/sync-unknown-ancestor`

---

## 1. Symptoms

The incident unfolded in three stages. The root cause (Symptom 1) was a race
condition that produced an inconsistent DB state during normal operation.
Symptom 2 was the crash that this inconsistency directly caused. Symptom 3
remained after the crash was fixed.

**Symptom 1: Race condition produces inconsistent chain state**

Block 22774780's commit marker (`makeCommitMinorBlock` key) was present in
LevelDB while the block was excluded from the canonical chain. This inconsistency
was not caused by a crash — it was produced by a **race condition** between
`AddBlockListForSync` and a concurrent `AddRootBlock` call (see §2, Bug 1).

The race mechanism:

1. `InsertChainForDeposits([22774780])` completes inside `m.chainmu`: body written,
   commit marker written, canonical hash written.
2. `m.chainmu` is released. The shard layer starts network calls
   (`BroadcastXshardTxList`, `SendMinorBlockHeaderToMaster`) — typically 10–100 ms.
3. Concurrently, `AddRootBlock` (holds neither `s.mu` nor `m.chainmu`) triggers
   `reWriteBlockIndexTo` → acquires `m.chainmu` → `setHead(22774779)` →
   **atomically deletes the commit marker, canonical hash, and block body** for
   22774780 (body deletion was a separate bug fixed in Bug 6a).
4. Network calls complete. The (pre-fix) code then calls
   `CommitMinorBlockByHash(22774780)` — **outside all locks** — re-writing the
   commit marker for a block whose body and canonical hash are now absent.
5. End state: body ✗ (deleted by pre-Bug-6a `setHead`), commit marker ✓ (re-written by racy call), canonical hash ✗.
6. `HasBlock(22774780)` returns `true` (marker present; pre-Bug-5 check was marker-only).
7. Sync skips re-downloading 22774780.
8. `GetBlock(22774780)` returns typed-nil (body absent) → **inserting 22774781 fails**.

The full S0.log trace (05-17 12:00:33 window) confirms the interleaving:

```
37541295  12:00:33.032  Write MinorBlock height=22774780 hash=6a7f44…
              ← putMinorBlock writes body; InsertChainForDeposits still inside m.chainmu

37541296  12:00:33.033  reorg same chain curr=22774778 newBlock=22774780@6a7f44
              ← WriteBlockWithState internal reorg (no-op, same chain); m.chainmu released after this

37541318  12:00:33.101  ready to set current header height=22774779 hash=c47cd9 status=true curr=false
              ← AddRootBlock detects canonical mismatch; running without m.chainmu or s.mu

37541319  12:00:33.101  reWrite orig=22774778@70293b new=22774779@c47cd9
              ← reWriteBlockIndexTo acquires m.chainmu to trigger reorg → setHead

37541321  12:00:33.153  reorg same chain curr=22774780@6a7f44 newBlock=22774779@c47cd9
              ← reorg determines 22774780 must be unwound

37541322  12:00:33.153  Rewinding blockchain target=22774779
              ← setHead atomically batch-deletes body+marker+canonHash for 22774780

37541378  12:00:39.564  Downloading blocks synctask=shard-0 from=22774778 to=22774780
              ← 6 s later: new sync cycle re-downloads; racy CommitMinorBlockByHash
                 fired in the 120 ms window between lines 37541296 and 37541322,
                 leaving marker present despite body+canonHash being deleted
```

This cycle persisted across restarts: each time the node recovered and synced
22774780, the race could recur, leaving 22774781 permanently unreachable.

---

**Symptom 2: Nil pointer panic — direct consequence of Symptom 1**

```
panic: runtime error: invalid memory address or nil pointer dereference
goroutine N [running]:
github.com/QuarkChain/goquarkchain/core/types.(*MinorBlock).NumberU64(...)
github.com/QuarkChain/goquarkchain/consensus/consensus.go:153
```

When sync attempted to insert block 22774781, `VerifyHeader` called
`chain.GetBlock(22774780)`. With the block body absent (Symptom 1, state 5),
`GetBlock` returned a **typed-nil** `types.IBlock` — a non-nil interface wrapping
a nil `*MinorBlock` pointer. The guard `if parent == nil` evaluated to `false`
(typed-nil is not nil at the interface level), so execution continued to
`parent.NumberU64()`, which dereferenced the nil pointer and panicked.

This crash repeated on every restart: the race re-created the bad state, and the
typed-nil panic re-triggered on the next sync attempt. Once Bug 2 was fixed
(`parent == nil` replaced by `qkccommon.IsNil(parent)`), the panic became a
graceful `ErrUnknownAncestor` — but the sync remained stalled (Symptom 3).

---

**Symptom 3: Persistent "unknown ancestor" sync stall — after Symptom 2 is fixed**

```
WARN [05-18] VerifyHeader unknown ancestor
  block=22774781 blockHash=0x... parentHash=0x...
  chainTip=22774779 chainTipHash=0x...
```

After Bug 2 was fixed, the nil pointer panic was replaced by a graceful
`ErrUnknownAncestor` return from `VerifyHeader`. Shard 40001's chain tip remained
stuck at 22774779. Block 22774781 could not be inserted, and sync remained
stalled indefinitely, cycling through Bugs 3, 5, and 6 until all were addressed.

---

## 2. Root Cause Analysis

The incident was triggered by multiple independent bugs compounding on each other.

### Bug 1: Race condition — redundant `CommitMinorBlockByHash` outside all locks

**Location:** `cluster/shard/api_backend.go`, `AddMinorBlock` (line 402) and
`AddBlockListForSync` (line 484)

**Cause:**

After `InsertChainForDeposits` returns, the shard layer performs two network
calls (`BroadcastXshardTxList` / `SendMinorBlockHeaderToMaster`), then calls
`CommitMinorBlockByHash` again. This second call is entirely outside `m.chainmu`
and `m.mu`. It races with `AddRootBlock`, which acquires `m.chainmu` to call
`setHead`. The race produces the "marker present, body/canonical-hash absent" state
described in Symptom 1.

`CommitMinorBlockByHash` was already called inside `insertChain` (via
`WriteBlockWithState:1002` for canonical/side blocks, or `insertSidechain:1272`
for pruned-ancestor blocks) while `m.chainmu` is held. The shard-layer calls
were redundant from the outset.

**Race interleaving (for a single block X):**

```
Goroutine A — AddBlockListForSync (holds s.mu):
  InsertChainForDeposits(X)           ← m.chainmu held inside; body+marker written
  m.chainmu released
  BroadcastXshardTxList(...)          ← network, ~10–100 ms, no locks
                                      ┌─────────────────────────────────────┐
                                      │  Goroutine B — AddRootBlock         │
                                      │  (no s.mu, no m.chainmu)            │
                                      │  reWriteBlockIndexTo →              │
                                      │    m.chainmu.Lock()                 │
                                      │    setHead(X.Number - 1)            │
                                      │      batch.Delete(body+marker[X])   │
                                      │      batch.Delete(canonHash[X])     │
                                      │      batch.Write()   ← atomic       │
                                      │    m.chainmu.Unlock()               │
                                      └─────────────────────────────────────┘
  SendMinorBlockHeaderListToMaster(...)
  CommitMinorBlockByHash(X)           ← re-writes marker[X] with NO lock
                                        canonical hash[X] still absent

Final state:  body[X]         = ABSENT
              marker[X]       = PRESENT  (re-written by racy call)
              canonHash[X]    = ABSENT   (deleted by setHead)

→ HasBlock(X) = true  → sync skips X
→ GetBlockByNumber(X) = nil  → "unknown ancestor" for X+1
```

**Fix:**

Remove both redundant shard-layer calls. Every block processed by
`InsertChainForDeposits` that is successfully committed already has its marker
written inside `insertChain` while holding `m.chainmu`.

```go
// cluster/shard/api_backend.go — AddMinorBlock
// Before:
s.MinorBlockChain.CommitMinorBlockByHash(block.Hash())   // removed
if s.MinorBlockChain.CurrentBlock().Hash() != currHead.Hash() {

// After:
if s.MinorBlockChain.CurrentBlock().Hash() != currHead.Hash() {


// cluster/shard/api_backend.go — AddBlockListForSync
// Before:
for _, header := range uncommittedBlockHeaderList {
    s.MinorBlockChain.CommitMinorBlockByHash(header.Hash())  // removed
    s.mBPool.delBlockInPool(header.Hash())
}

// After:
for _, header := range uncommittedBlockHeaderList {
    s.mBPool.delBlockInPool(header.Hash())
}
```

**Why the redundant calls were harmless in normal operation:**

In the common case, `AddRootBlock` has already confirmed the block via
`HasBlock` before `InsertChainForDeposits` is called, so no concurrent `setHead`
is triggered. The race requires a root block arriving during the narrow network-
call window, which is rare but not impossible.

---

### Bug 2: Typed-nil interface causes nil pointer panic

**Location:** `consensus/consensus.go:153` (`VerifyHeader`)

**Cause:**

`GetBlock()` returns `types.IBlock`, an interface. When the underlying
`*MinorBlock` is nil, the interface value itself is **not** nil — a Go interface
is a `(type, pointer)` pair, and the type field is populated even when the
pointer is nil. As a result, `parent == nil` evaluates to `false`, execution
continues, and `parent.NumberU64()` dereferences a nil pointer.

**Trigger path:**

After an unclean shutdown (OOM / kill -9), the state tries for the most recent
128 blocks have not been flushed to disk (QuarkChain flushes every 128 blocks in
non-archive mode). `reRunBlockWithState` walks backward to find the last
persisted-state ancestor. During this walk, `GetBlock()` returns a typed-nil
interface for a block whose body was never fully written, triggering the panic.

**Fix:**

```go
// Before
parent := chain.GetBlock(header.GetParentHash())
if parent == nil {

// After
parent := chain.GetBlock(header.GetParentHash())
if qkccommon.IsNil(parent) {
    log.Warn("VerifyHeader unknown ancestor", ...)
    return ErrUnknownAncestor
}
```

`qkccommon.IsNil` uses `reflect` to inspect the underlying pointer through the
interface.

**Note:** The same class of defect was found in three additional locations and
fixed with the same approach: replace `== nil` / `!= nil` comparisons with
`qkcCommon.IsNil`, and use concrete-type accessors (`GetMinorBlock`) instead of
interface-returning ones (`GetBlock`) where the result is stored in an
`atomic.Value` or type-asserted directly.

| Location | Defect | Fix |
|----------|--------|-----|
| `reRunBlockWithState` (`minorblockchain_addon.go`) | Nil check correct, but error message calls `block.NumberU64()` / `block.Hash()` on the nil pointer | Save `parentHash` before reassigning `block`; use it in the error string |
| `reorg` loop conditions (`minorblockchain.go`) | `oldBlock != nil` / `newBlock != nil` pass for typed-nil, loop body then panics | Replace with `!qkcCommon.IsNil(oldBlock)` / `!qkcCommon.IsNil(newBlock)` |
| `setHead` and `AddRootBlock` (`minorblockchain.go`, `minorblockchain_addon.go`) | `GetBlock` return value (typed-nil `types.IBlock`) passed to `atomic.Value.Store` or type-asserted directly | Use `GetMinorBlock` (returns concrete `*types.MinorBlock`) with a nil guard before storing |

---

### Bug 3: Sync batch loop exits early on a known block (core bug)

**Location:** `cluster/shard/api_backend.go`, `ShardBackend.AddBlockListForSync`

**Cause:**

During sync, the master sends a batch of minor blocks to the slave.
`AddBlockListForSync` inserts them one by one. When `InsertChainForDeposits`
returns an empty xshard list (block already known), the intended behaviour was
to skip that block and continue with the rest. The code instead executed
`return nil`, **exiting the entire function** on the first known block and
silently discarding every subsequent block in the batch.

**Trigger path:**

Root block 3857740 referenced three minor blocks for Shard 4: 22774780,
22774781, and 22774782. The sync batch `[22774780, 22774781, 22774782]`
arrived as follows:

1. 22774780: already `BLOCK_COMMITTED` from the previous root block confirmation — skipped.
2. 22774781: `InsertChainForDeposits` returns empty xshard (block already known) — triggers `return nil`.
3. **`return nil` exits the entire loop.** 22774782 is never processed.
4. On the next retry, the master sees the chain tip still at 22774779 — the cycle repeats indefinitely.

**Fix:**

```go
// Before
if len(xshardLst) != 1 {
    log.Warn(s.logInfo + " already have this block", ...)
    return nil   // wrong: exits the whole function
}

// After
if len(xshardLst) != 1 {
    // insertSidechain already committed this block (no xshard needed) — continue.
    // If the block is not committed, the chain is shutting down — return error.
    if s.getBlockCommitStatusByHash(blockHash) != BLOCK_COMMITTED {
        return fmt.Errorf("InsertChainForDeposits returned empty xshard list for non-committed block %d", block.NumberU64())
    }
    log.Warn(s.logInfo + " already have this block", ...)
    continue     // correct: skip this block, process the rest
}
```

---

### Bug 4: `isSameRootChain` violates precondition of `isSameChain`

**Location:** `core/minorblockchain_addon.go`, `isSameRootChain`

**Cause:**

`isSameRootChain(long, short)` delegates to `isSameChain`, which assumes
`long.Number >= short.Number`. In certain recovery scenarios — for example when
a minor block header's `prevRootBlockHash` points to a root block taller than
the current `rootTip` — this assumption is violated, producing a `log.Crit`
and crashing the process:

```
Crit isSameRootChain long.Number=3856710 short.Number=3857672
```

**Fix:**

```go
func (m *MinorBlockChain) isSameRootChain(long types.IBlock, short types.IBlock) bool {
    if long.NumberU64() < short.NumberU64() {
        return false  // added: guard against violating isSameChain's precondition
    }
    f := func(hash common.Hash) common.Hash { ... }
    return isSameChain(f, long, short)
}
```

---

### Bug 5: `HasBlock` checks only the commit marker, not the block body

**Location:** `core/minorblockchain.go`, `MinorBlockChain.HasBlock`

**Cause:**

`HasBlock` checked only the `makeCommitMinorBlock` key (commit marker), not the
`blockKey` (block body):

```go
// Before
func (m *MinorBlockChain) HasBlock(hash common.Hash) bool {
    return m.IsMinorBlockCommittedByHash(hash)  // checks marker only
}
```

These are **completely independent LevelDB keys**. The "marker present, body
absent" state was observed in production (see Symptom 1 above); `HasBlock` must
defend against it regardless of the exact mechanism.

**Trigger path (persists after Bug 2+3+4 are fixed):**

After the first three fixes, the node restarts. Block 22774780's commit marker
survived the crash but its body is gone:

1. `SlaveBackend.AddBlockListForSync` calls `HasBlock(22774780)` → `true` (marker present)
2. 22774780 is filtered out; only 22774781 and 22774782 are downloaded
3. Inserting 22774781 calls `GetBlock(22774780)` → `nil` (body missing)
4. → `unknown ancestor` — permanent loop

**Fix:**

```go
// After: require both commit marker and block body
func (m *MinorBlockChain) HasBlock(hash common.Hash) bool {
    return m.IsMinorBlockCommittedByHash(hash) && rawdb.HasBlock(m.db, hash)
}
```

When the body is absent, `HasBlock` returns `false`, triggering a re-download.
Once the body is restored, sync resumes normally.

---

### Bug 6a: `setHead` deletes block bodies, breaking crash recovery

**Location:** `core/minorblockchain.go`, `MinorBlockChain.setHead`

**Cause:**

The original `setHead` called `rawdb.DeleteMinorBlock` while rewinding the
chain, which **deleted block bodies**:

```go
// Before
for block := m.CurrentBlock(); block != nil && block.NumberU64() > head; block = m.CurrentBlock() {
    rawdb.DeleteMinorBlock(batch, block.Hash())  // deletes body!
    rawdb.DeleteCanonicalHash(...)
    m.currentBlock.Store(m.GetBlock(block.ParentHash()))
}
```

`reRunBlockWithState` walks backward from `currentBlock` to find the last
persisted-state ancestor, replaying blocks forward to rebuild the state trie.
After `setHead(M)`, `currentBlock` is M, so replay only needs bodies at or
below M — those bodies are not touched by `setHead`. The deleted bodies
(M+1…N, above the new head) are not needed for the immediate crash recovery.

**Trigger path:**

`InitFromRootBlock` → `reWriteBlockIndexTo` → `reorg` (same-chain branch) →
`setHead`. The same-chain reorg path calls `setHead(M)` to rewind `currentBlock`
from height N down to M. This deletes the bodies of blocks M+1…N.

Note: `reRunBlockWithState` only walks backward from `currentBlock` (= M after
the rewind), so it only needs bodies at or below M — those are intact. The
deleted bodies (M+1…N) sit above the new tip and are not accessed during the
next crash recovery. The real cost is that sync must re-download M+1…N from
peers to restore them, even though their bodies were still present locally just
moments before. There is no upside to this deletion and the re-download is
unnecessary work.

**Fix:**

```go
// After: delete only receipts and commit marker; preserve block body
for block := m.CurrentBlock(); block != nil && block.NumberU64() > head; block = m.CurrentBlock() {
    rawdb.DeleteReceipts(batch, block.Hash())
    rawdb.DeleteMinorBlockCommitStatus(batch, block.Hash())
    rawdb.DeleteCanonicalHash(...)
    m.currentBlock.Store(m.GetBlock(block.ParentHash()))
}
```

Block bodies are preserved so `reRunBlockWithState` can replay them on any
future restart. Receipts are safe to drop — they are regenerated during replay.

---

### Bug 6b: `setHead` resets `currentBlock` to genesis on missing state

**Location:** `core/minorblockchain.go`, `MinorBlockChain.setHead`

**Cause:**

When the target block's state trie was unavailable, `setHead` reset
`currentBlock` to the genesis block:

```go
// Before
if _, err := m.StateAt(currentBlock.GetMetaData().Root); err != nil {
    m.currentBlock.Store(m.genesisBlock)  // bug: resets to genesis
}
```

A missing state trie is the **normal post-crash condition** — the in-memory
state was not flushed. `InitFromRootBlock` always calls `reRunBlockWithState`
to rebuild the missing state *before* invoking `setHead`. Resetting to genesis
discards that work and leaves the chain stuck at height 0.

**Fix:**

```go
// After: only reset to genesis when currentBlock is literally nil
if currentBlock := m.CurrentBlock(); currentBlock == nil {
    m.currentBlock.Store(m.genesisBlock)
}
```

Missing state is handled by the caller (`reRunBlockWithState`), not by
`setHead`.

---

## 3. Incident Timeline

```
T0  Node running normally (pre-crash)
    → Race condition (Bug 1 + Bug 6a pre-fix): root block 3857740 arrives
      while AddBlockListForSync is between BroadcastXshardTxList and
      CommitMinorBlockByHash for block 22774780
    → setHead(22774779) atomically deletes body+marker+canonHash for 22774780
      (Bug 6a: setHead was also deleting the body at this time)
    → Racy CommitMinorBlockByHash re-writes marker[22774780] outside all locks
    → End state: body ABSENT, marker PRESENT, canonHash ABSENT (Symptom 1)

T1  Sync attempts to insert block 22774781
    → VerifyHeader(22774781) → GetBlock(22774780) → typed-nil (body absent)
    → parent == nil is false (typed-nil), parent.NumberU64() → PANIC (Bug 2)
    → Node crashes; cycle repeats on each restart (Symptom 2)

T2  Bug 2 fixed: qkccommon.IsNil replaces == nil in VerifyHeader
    → Panic replaced by graceful ErrUnknownAncestor
    → Shard 40001 chain tip at 22774779; root block 3857740 resent

T3  Sync begins (after Bug 2 fix)
    → Batch [22774780, 22774781, 22774782] arrives
    → 22774780: HasBlock = true (marker present, body absent) — COMMITTED, skipped
    → 22774781: InsertChainForDeposits returns empty xshard (block known)
    → return nil (Bug 3) exits the batch; 22774782 never processed
    → Permanently stuck at 22774779 (Symptom 3)

T4  Bugs 2+3+4 fixed, node restarts
    → 22774780 commit marker survived; body still absent after re-download gap
    → HasBlock(22774780) = true (Bug 5: marker-only check)
    → 22774780 filtered out; only 22774781 downloaded
    → VerifyHeader(parent=22774780) → GetBlock = nil → unknown ancestor

T5  All bugs (1–6) fixed, node starts normally
    → HasBlock(22774780) = false (Bug 5 fix: marker+body check; body absent → re-download)
    → 22774780 re-downloaded and inserted successfully
    → 22774781 and 22774782 follow; sync resumes
```

---

## 4. Why No Single Bug Was Sufficient

| Bug | Effect in isolation |
|-----|---------------------|
| Bug 1 only | Race window is narrow; only manifests when `AddRootBlock` arrives during broadcast latency |
| Bug 2 only | Panic on restart; if state rebuilds cleanly, sync can resume |
| Bug 3 only | Sync may stall after a clean restart but can recover via retry |
| Bug 4 only | Only triggers on specific recovery paths; normal operation unaffected |
| Bug 5 only | Only triggers when crash leaves marker present but body absent |
| Bug 6a only | Only deletes bodies when `setHead` is called; clean restart unaffected |
| Bug 6b only | Only resets to genesis when `setHead` sees missing state |
| Bug 1 + Bug 5 (pre-fix) | Race writes marker back; old `HasBlock` (marker-only) treats block as committed; sync skips re-download; permanent stall |
| Bug 1 + Bug 5 (post-fix) | Race writes marker back; new `HasBlock` (marker + body) still returns `true` (body preserved by Bug 6a fix); canonical hash absent → unknown ancestor persists |
| Bug 2 + Bug 3 | Crash-restart always causes permanent sync stall |
| Bug 2 + Bug 3 + Bug 5 | After fixing the first two, missing body produces a new unknown ancestor loop |

---

## 5. Underlying Cause: State Trie Persistence Window

In non-archive mode, QuarkChain flushes the state trie to disk every 128 blocks
(`triesInMemory = 128`). An unclean shutdown loses the state for up to the most
recent 128 blocks, which exist only in memory.

`reRunBlockWithState` is the designed recovery mechanism: starting from the last
block with a persisted state, it replays transactions forward to rebuild the
trie. This mechanism depends on four invariants that the bugs above violated:

1. `GetBlock()` must return a safe nil-check-able value — violated by Bug 2.
2. Block bodies must survive a `setHead` call — violated by Bug 6a.
3. `HasBlock()` must accurately reflect whether the body actually exists — violated by Bug 5.
4. The commit marker must only be written while `m.chainmu` is held, so that
   `setHead` (which also holds `m.chainmu` when removing it) cannot be
   interleaved with a marker write — violated by Bug 1.

---

## 6. Fix Summary

| # | File | Change |
|---|------|--------|
| Bug 1 | `cluster/shard/api_backend.go` | Remove redundant `CommitMinorBlockByHash` calls in `AddMinorBlock` and `AddBlockListForSync` that raced with `setHead` |
| Bug 2 | `consensus/consensus.go` | `parent == nil` → `qkccommon.IsNil(parent)`; add Warn log; fix three additional nil/typed-nil patterns in `reRunBlockWithState`, `reorg`, `setHead`/`AddRootBlock` |
| Bug 3 | `cluster/shard/api_backend.go` | `return nil` → `continue`; distinguish `insertSidechain` vs `procInterrupt` sub-cases |
| Bug 4 | `core/minorblockchain_addon.go` | Add `long < short` early-return guard in `isSameRootChain` |
| Bug 5 | `core/minorblockchain.go` | `HasBlock` now requires both commit marker and `rawdb.HasBlock` body check |
| Bug 6a | `core/minorblockchain.go` | `setHead`: replace `DeleteMinorBlock` with `DeleteReceipts` + `DeleteMinorBlockCommitStatus` |
| Bug 6b | `core/minorblockchain.go` | `setHead`: remove state-missing → genesis reset; only reset when `currentBlock == nil` |
| Guard | `core/minorblockchain.go` + `cluster/shard/api_backend.go` | Add `IsInitialized()` check; reject sync requests until `InitFromRootBlock` completes |
| Logging | `consensus/consensus.go`, `core/minorblock_validator.go`, `cluster/shard/api_backend.go` | Add block number / hash / parentHash / prevRootHash to error logs for easier diagnosis |
