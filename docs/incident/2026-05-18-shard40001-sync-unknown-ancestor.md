# Incident Report: Shard 40001 Sync Failure — "Unknown Ancestor" & Nil Pointer Panic

**Date:** 2026-05-18
**Affected Component:** Shard 40001 (Chain 4, Shard 0), Minor Block Sync
**Severity:** Critical — node stuck, unable to sync
**Branch:** `fix/sync-unknown-ancestor`

---

## 1. 问题现象

节点重启后，Shard 40001 出现以下两类错误，导致同步彻底卡住：

**现象 1：nil pointer panic（崩溃）**

```
panic: runtime error: invalid memory address or nil pointer dereference
goroutine N [running]:
github.com/QuarkChain/goquarkchain/core/types.(*MinorBlock).NumberU64(...)
github.com/QuarkChain/goquarkchain/consensus/consensus.go:153
```

**现象 2：unknown ancestor 错误（同步卡死）**

```
WARN [05-18] VerifyHeader unknown ancestor
  block=22774781 blockHash=0x... parentHash=0x...
  chainTip=22774779 chainTipHash=0x...
```

节点 Shard 40001 的链尖停在 22774779，无法插入 22774781，同步长期卡住。

---

## 2. 根因分析

本次事故由多个独立 Bug 叠加触发。

### Bug 1：typed-nil interface 导致 nil pointer panic

**位置：** `consensus/consensus.go:153`（`VerifyHeader` 函数）

**原因：**

`GetBlock()` 的返回类型是 `types.IBlock`（接口），当底层 `*MinorBlock` 为 nil 时，Go 接口本身并不为 nil——接口由 `(type, pointer)` 两个字段组成，type 字段有值，pointer 字段为 nil，因此 `parent == nil` 判断为 **false**，代码继续执行，在 `parent.NumberU64()` 处触发 nil pointer 解引用崩溃。

**触发路径：**

节点因异常关机（OOM / kill）重启后，最近 128 块的 state trie 尚未持久化（QuarkChain 非 archive 模式每 128 块刷盘一次）。`reRunBlockWithState` 在重建 state 时，需要回溯到最后一个有 state 的祖先块。这个过程中，`GetBlock()` 对某个尚未完整入库的块返回了包装了 nil 指针的接口值，触发 panic。

**修复：**

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

使用 `reflect` 穿透接口检查底层指针是否为 nil。

---

### Bug 2：批量同步循环提前退出（核心 Bug）

**位置：** `cluster/shard/api_backend.go`，`ShardBackend.AddBlockListForSync`

**原因：**

同步时，Master 将一批 Minor Block 列表发送给 Slave，Slave 的 `AddBlockListForSync` 逐块插入链。对于已知块（`InsertChainForDeposits` 返回空 xshard 列表），代码原本的意图是"跳过该块，继续处理后续块"，但实际写的是 `return nil`，导致**整个批次在遇到第一个已知块时立即退出**，后续的块全部被丢弃。

**触发路径：**

Root Block 3857740 包含 Shard 4 的三个 Minor Block：22774780、22774781、22774782。

同步时发来的批次为 `[22774780, 22774781, 22774782]`：

1. 处理 22774780：该块已在上一个 root block confirm 时写入（BLOCK_COMMITTED），跳过。
2. 处理 22774781：`InsertChainForDeposits` 调用返回空 xshard（块已知），触发 `return nil`。
3. **`return nil` 退出整个循环**，22774782 被完全跳过。
4. 下次同步重试时，Master 发现链尖仍停在 22774779，循环往复，永久卡住。

**修复：**

```go
// Before
if len(xshardLst) != 1 {
    log.Warn(s.logInfo + " already have this block", ...)
    return nil   // 错误：退出了整个函数
}

// After
if len(xshardLst) != 1 {
    log.Warn(s.logInfo + " already have this block", ...)
    continue     // 正确：跳过当前块，继续处理后续块
}
```

---

### Bug 3：isSameRootChain 违反前置条件崩溃

**位置：** `core/minorblockchain_addon.go`，`isSameRootChain`

**原因：**

`isSameRootChain(long, short)` 调用 `isSameChain`，后者假设 `long.Number >= short.Number`。在某些恢复场景中（如 minor block header 引用的 `prevRootBlockHash` 高度高于当前 `rootTip`），`long.Number < short.Number` 的情况会出现，触发越界或逻辑错误：

```
Crit isSameRootChain long.Number=3856710 short.Number=3857672
```

**修复：**

```go
func (m *MinorBlockChain) isSameRootChain(long types.IBlock, short types.IBlock) bool {
    if long.NumberU64() < short.NumberU64() {
        return false  // 新增：提前返回，避免违反 isSameChain 前置条件
    }
    f := func(hash common.Hash) common.Hash { ... }
    return isSameChain(f, long, short)
}
```

---

### Bug 4：HasBlock 仅检查 commit marker，不验证 block body

**位置：** `core/minorblockchain.go`，`MinorBlockChain.HasBlock`

**原因：**

`HasBlock` 的实现只检查 `makeCommitMinorBlock` key（commit marker），不检查 `blockKey`（block body）：

```go
// 修复前
func (m *MinorBlockChain) HasBlock(hash common.Hash) bool {
    return m.IsMinorBlockCommittedByHash(hash)  // 只检查 marker
}
```

这两个是 **完全独立的 LevelDB key**。崩溃（OOM / kill）后，commit marker 可能因已持久化而残留，但 block body 可能因未刷盘而丢失。

**触发路径（Bug 1+2 修复后仍会出现）：**

三个 bug 修复后节点重启，22774780 的 commit marker 残留，body 丢失：

1. `SlaveBackend.AddBlockListForSync` 调用 `HasBlock(22774780)` → true（marker 存在）
2. 22774780 被过滤，仅下载 22774781、22774782
3. 插入 22774781 时 `VerifyHeader` 调用 `GetBlock(22774780)` → nil（body 缺失）
4. → `unknown ancestor`，永久循环

**修复：**

```go
// 修复后：同时验证 commit marker 和 block body
func (m *MinorBlockChain) HasBlock(hash common.Hash) bool {
    return m.IsMinorBlockCommittedByHash(hash) && rawdb.HasBlock(m.db, hash)
}
```

body 缺失时 `HasBlock` 返回 false，触发重新下载，body 恢复后同步恢复正常。

---

### Bug 5：setHead 删除 block body，导致崩溃后无法恢复

**位置：** `core/minorblockchain.go`，`MinorBlockChain.setHead`

**原因：**

原始的 `setHead` 在回退链头时，对每个需要删除的块调用了 `rawdb.DeleteMinorBlock`——这会**同时删除 block body**：

```go
// 修复前
for block := m.CurrentBlock(); block != nil && block.NumberU64() > head; block = m.CurrentBlock() {
    rawdb.DeleteMinorBlock(batch, block.Hash())  // 删除了 body！
    rawdb.DeleteCanonicalHash(...)
    m.currentBlock.Store(m.GetBlock(block.ParentHash()))
}
// 若 target block 的 state 丢失，额外重置到 genesis
if _, err := m.StateAt(currentBlock.GetMetaData().Root); err != nil {
    m.currentBlock.Store(m.genesisBlock)  // Bug：将 currentBlock 重置为创世块
}
```

`reRunBlockWithState` 依赖 block body 来逐块重放交易、重建 state trie。`setHead` 删除 body 后，崩溃恢复时 `reRunBlockWithState` 找不到需要重放的块，导致恢复失败。

此外，在 target block 的 state 丢失时将 `currentBlock` 强制重置为 genesis 也是错误的——崩溃后 state 丢失是正常情况，`reRunBlockWithState` 会在 `setHead` 之前重建，重置到 genesis 会抹掉重建结果。

**触发场景：**

`InitFromRootBlock` 调用 `reWriteBlockIndexTo` → `reorg` 路径，间接触发链重组操作时，如果发生 `setHead` 调用，就会删除 block bodies，令后续 `reRunBlockWithState` 无法正常工作。

**修复：**

```go
// 修复后：只删除 receipts 和 commit marker，保留 block body
for block := m.CurrentBlock(); block != nil && block.NumberU64() > head; block = m.CurrentBlock() {
    rawdb.DeleteReceipts(batch, block.Hash())
    rawdb.DeleteMinorBlockCommitStatus(batch, block.Hash())
    rawdb.DeleteCanonicalHash(...)
    m.currentBlock.Store(m.GetBlock(block.ParentHash()))
}
// 移除 genesis 重置逻辑：state 丢失由 reRunBlockWithState 处理
if currentBlock := m.CurrentBlock(); currentBlock == nil {
    m.currentBlock.Store(m.genesisBlock)  // 仅在 currentBlock 为 nil 时才重置
}
```

---

## 3. 故障时间线

```
T0  节点异常关机（OOM 或 kill -9）
    → 最近 128 块的 in-memory state trie 未持久化

T1  节点重启
    → reRunBlockWithState 尝试重建 state
    → GetBlock() 返回 typed-nil interface
    → consensus.go:153 nil pointer panic（Bug 1）
    → 节点崩溃重启

T2  节点再次启动（Bug 1 修复后）
    → Shard 40001 链尖恢复到 22774779
    → Root block 3857740 包含 22774780/22774781/22774782

T3  同步开始
    → 批次 [22774780, 22774781, 22774782] 到达
    → 22774780 已 COMMITTED，跳过
    → 22774781 InsertChainForDeposits 返回空 xshard（已知块）
    → return nil（Bug 2）退出批次，22774782 被跳过
    → 永久卡在 22774779

T4  Bug 1+2+3 修复，节点重启
    → 22774780 commit marker 残留，body 丢失
    → HasBlock(22774780) = true（Bug 4）
    → 22774780 被过滤，仅下载 22774781
    → VerifyHeader(parent=22774780) → GetBlock = nil → unknown ancestor

T5  所有 Bug（1-5）修复后节点正常启动
    → HasBlock(22774780) = false（marker 存在但 body 缺失）
    → 22774780 被重新下载，插入成功
    → 22774781、22774782 顺利跟进，同步恢复
```

---

## 4. 为什么缺少任何一个 Bug 都不会完整触发

| Bug | 单独存在时的影响 |
|-----|----------------|
| Bug 1 only | panic 后重启，state 重建正常则可继续同步 |
| Bug 2 only | 正常关机重启后同步可能卡住，但有可能通过重试恢复 |
| Bug 3 only | 仅在特定恢复路径触发，正常运行不受影响 |
| Bug 4 only | 仅在崩溃 + commit marker 残留 + body 丢失场景触发 |
| Bug 5 only | 仅在 setHead 被调用时删除 body，正常重启不触发 |
| Bug 1 + Bug 2 | 崩溃重启后同步必然卡死，无法自动恢复 |
| Bug 1 + Bug 2 + Bug 4 | 修复前两个后，崩溃场景下 body 丢失引发新的 unknown ancestor |

---

## 5. 深层原因：state trie 持久化机制

QuarkChain 非 archive 模式下，state trie 每 128 块才写盘一次（`triesInMemory = 128`）。异常关机会导致最近 128 块的 state 只存在于内存中，重启后这些 state 丢失。

`reRunBlockWithState` 机制用于应对这种情况：从最后一个有持久化 state 的块开始，逐块重放交易以重建 state。这是正常的容错机制，但它依赖于：
1. `GetBlock()` 返回值的安全性（Bug 1 是此处暴露的问题）
2. block body 在 `setHead` 后依然存在（Bug 5 是此处暴露的问题）
3. `HasBlock()` 能准确反映 body 是否真正存在（Bug 4 是此处暴露的问题）

---

## 6. 修复摘要

| 编号 | 文件 | 修改内容 |
|------|------|---------|
| Bug 1 | `consensus/consensus.go` | `parent == nil` → `qkccommon.IsNil(parent)`；增加 Warn 日志 |
| Bug 2 | `cluster/shard/api_backend.go` | `return nil` → `continue` |
| Bug 3 | `core/minorblockchain_addon.go` | `isSameRootChain` 增加 `long < short` 提前返回 |
| Bug 4 | `core/minorblockchain.go` | `HasBlock` 增加 `rawdb.HasBlock` body 存在性检查 |
| Bug 5 | `core/minorblockchain.go` | `setHead` 改为只删 receipts + commit marker，保留 block body；移除 genesis reset |
| 防护 | `core/minorblockchain.go` + `cluster/shard/api_backend.go` | 新增 `IsInitialized()` 防护，shard 未完成初始化时拒绝同步请求 |
| 日志 | `consensus/consensus.go`、`core/minorblock_validator.go`、`cluster/slave/api_backend.go`、`cluster/shard/api_backend.go` | 增加 block number/hash/parentHash/chainTip 等诊断日志，便于后续排查 |

所有修改均在分支 `fix/sync-unknown-ancestor` 中。
