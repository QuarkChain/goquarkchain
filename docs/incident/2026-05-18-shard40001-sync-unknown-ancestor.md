# Incident Report: Shard 40001 Sync Failure — "Unknown Ancestor" & Nil Pointer Panic

**Date:** 2026-05-18
**Affected Component:** Shard 40001 (Chain 4, Shard 0), Minor Block Sync
**Severity:** Critical — node stuck, unable to sync
**Branch:** `fix/sync-unknown-ancestor`

---

## 1. Symptoms

After a node restart from a crash, Shard 40001 exhibited two successive errors.

**Symptom 1: nil pointer panic (crash)**

```
panic: runtime error: invalid memory address or nil pointer dereference
goroutine N [running]:
github.com/QuarkChain/goquarkchain/core/types.(*MinorBlock).NumberU64(...)
github.com/QuarkChain/goquarkchain/consensus/consensus.go:153
```

The node crashed repeatedly on startup. Once Symptom 1 was fixed (Bug 1), the
panic was eliminated but the node immediately exhibited Symptom 2.

**Symptom 2: unknown ancestor (sync stall)**

```
WARN [05-18] VerifyHeader unknown ancestor
  block=22774781 blockHash=0x... parentHash=0x...
  chainTip=22774779 chainTipHash=0x...
```

Shard 40001's chain tip was stuck at 22774779. Block 22774781 could not be inserted, and sync remained stalled indefinitely.

**Symptom 3: inconsistent LevelDB state after crash**

After the crash, block 22774780's commit marker (`makeCommitMinorBlock` key) was
present in LevelDB while its block body (`blockKey`) was absent. These are
completely independent keys. The commit marker caused `HasBlock` to return
`true`, so the block was filtered from the download list — but `GetBlock` then
returned `nil` because the body was missing, producing the "unknown ancestor"
error above.

The exact mechanism that produced this state was not conclusively identified
from code inspection. In the current codebase:
- All insertion paths write the body before the marker; a crash during insertion
  leaves body present and marker absent (harmless).
- The deletion path in `setHead` uses a LevelDB batch, making body and marker
  deletion atomic; OOM cannot cause partial batch application.

The inconsistency is real — it was observed in production — but its root cause
requires reviewing the original crash-time logs to confirm.

---

## 2. Root Cause Analysis

The incident was triggered by five independent bugs compounding on each other.

### Bug 1: Typed-nil interface causes nil pointer panic

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

---

### Bug 2: Sync batch loop exits early on a known block (core bug)

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

### Bug 3: `isSameRootChain` violates precondition of `isSameChain`

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

### Bug 4: `HasBlock` checks only the commit marker, not the block body

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
absent" state was observed in production (see Symptom 3 above); `HasBlock` must
defend against it regardless of the exact mechanism.

**Trigger path (persists after Bug 1+2+3 are fixed):**

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

### Bug 5a: `setHead` deletes block bodies, breaking crash recovery

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

### Bug 5b: `setHead` resets `currentBlock` to genesis on missing state

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
T0  Unclean shutdown (OOM or kill -9)
    → In-memory state trie for the most recent 128 blocks is not flushed

T1  Node restarts
    → reRunBlockWithState attempts to rebuild state
    → GetBlock() returns a typed-nil interface
    → consensus.go:153 nil pointer panic (Bug 1)
    → Node crashes and restarts

T2  Node starts again (after Bug 1 fix)
    → Shard 40001 chain tip recovered to 22774779
    → Root block 3857740 references 22774780 / 22774781 / 22774782

T3  Sync begins
    → Batch [22774780, 22774781, 22774782] arrives
    → 22774780 is COMMITTED — skipped
    → 22774781: InsertChainForDeposits returns empty xshard (block known)
    → return nil (Bug 2) exits the batch; 22774782 is never processed
    → Permanently stuck at 22774779

T4  Bugs 1+2+3 fixed, node restarts
    → 22774780 commit marker survived crash, body is missing
    → HasBlock(22774780) = true (Bug 4)
    → 22774780 filtered out; only 22774781 downloaded
    → VerifyHeader(parent=22774780) → GetBlock = nil → unknown ancestor

T5  All bugs (1–5) fixed, node starts normally
    → HasBlock(22774780) = false (marker present but body missing)
    → 22774780 re-downloaded and inserted successfully
    → 22774781 and 22774782 follow; sync resumes
```

---

## 4. Why No Single Bug Was Sufficient

| Bug | Effect in isolation |
|-----|---------------------|
| Bug 1 only | Panic on restart; if state rebuilds cleanly, sync can resume |
| Bug 2 only | Sync may stall after a clean restart but can recover via retry |
| Bug 3 only | Only triggers on specific recovery paths; normal operation unaffected |
| Bug 4 only | Only triggers when crash leaves marker present but body absent |
| Bug 5a only | Only deletes bodies when `setHead` is called; clean restart unaffected |
| Bug 5b only | Only resets to genesis when `setHead` sees missing state |
| Bug 1 + Bug 2 | Crash-restart always causes permanent sync stall |
| Bug 1 + Bug 2 + Bug 4 | After fixing the first two, missing body produces a new unknown ancestor loop |

---

## 5. Underlying Cause: State Trie Persistence Window

In non-archive mode, QuarkChain flushes the state trie to disk every 128 blocks
(`triesInMemory = 128`). An unclean shutdown loses the state for up to the most
recent 128 blocks, which exist only in memory.

`reRunBlockWithState` is the designed recovery mechanism: starting from the last
block with a persisted state, it replays transactions forward to rebuild the
trie. This mechanism depends on three invariants that the bugs above violated:

1. `GetBlock()` must return a safe nil-check-able value — violated by Bug 1.
2. Block bodies must survive a `setHead` call — violated by Bug 5a.
3. `HasBlock()` must accurately reflect whether the body actually exists — violated by Bug 4.

---

## 6. Fix Summary

| # | File | Change |
|---|------|--------|
| Bug 1 | `consensus/consensus.go` | `parent == nil` → `qkccommon.IsNil(parent)`; add Warn log |
| Bug 2 | `cluster/shard/api_backend.go` | `return nil` → `continue`; distinguish `insertSidechain` vs `procInterrupt` sub-cases |
| Bug 3 | `core/minorblockchain_addon.go` | Add `long < short` early-return guard in `isSameRootChain` |
| Bug 4 | `core/minorblockchain.go` | `HasBlock` now requires both commit marker and `rawdb.HasBlock` body check |
| Bug 5a | `core/minorblockchain.go` | `setHead`: replace `DeleteMinorBlock` with `DeleteReceipts` + `DeleteMinorBlockCommitStatus` |
| Bug 5b | `core/minorblockchain.go` | `setHead`: remove state-missing → genesis reset; only reset when `currentBlock == nil` |
| Guard | `core/minorblockchain.go` + `cluster/shard/api_backend.go` | Add `IsInitialized()` check; reject sync requests until `InitFromRootBlock` completes |
| Logging | `consensus/consensus.go`, `core/minorblock_validator.go`, `cluster/shard/api_backend.go` | Add block number / hash / parentHash / prevRootHash to error logs for easier diagnosis |
