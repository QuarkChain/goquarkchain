// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

// RootBlockGen creates blocks for testing.
// See GenerateRootBlockChain for a detailed explanation.
type RootBlockGen struct {
	i       int
	parent  *types.RootBlock
	chain   []*types.RootBlock
	header  *types.RootBlockHeader
	Headers types.MinorBlockHeaders
	engine  consensus.Engine
}

// SetCoinbase sets the coinbase of the generated block.
// It can be called at most once.
func (b *RootBlockGen) SetCoinbase(addr account.Address) {
	b.header.SetCoinbase(addr)
}

// SetExtra sets the extra data field of the generated block.
func (b *RootBlockGen) SetExtra(data []byte) {
	b.header.SetExtra(data)
}

// SetNonce sets the nonce field of the generated block.
func (b *RootBlockGen) SetNonce(nonce uint64) {
	b.header.SetNonce(nonce)
}

// Number returns the block number of the block being generated.
func (b *RootBlockGen) Number() uint64 {
	return b.header.NumberU64()
}

// PrevBlock returns a previously generated block by number. It panics if
// num is greater or equal to the number of the block being generated.
// For index -1, PrevBlock returns the parent block given to GenerateRootBlockChain.
func (b *RootBlockGen) PrevBlock(index int) *types.RootBlock {
	if index >= b.i {
		panic(fmt.Errorf("block index %d out of range (%d,%d)", index, -1, b.i))
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}

// OffsetTime modifies the time instance of a block, implicitly changing its
// associated difficulty. It's useful to test scenarios where forking is not
// tied to chain length directly.
func (b *RootBlockGen) SetDifficulty(value uint64) {
	b.header.Difficulty = new(big.Int).SetUint64(value)
}

func (b *RootBlockGen) SetTotalDifficulty(value *big.Int) {
	b.header.ToTalDifficulty = new(big.Int).Set(value)
}

// GenerateRootBlockChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions and uncles added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their coinbase will be the zero address.
//
// Blocks created by GenerateRootBlockChain do not contain valid proof of work
// values. Inserting them into BlockChain requires use of FakePow or
// a similar non-validating proof of work implementation.
func GenerateRootBlockChain(parent *types.RootBlock, engine consensus.Engine, n int, gen func(int, *RootBlockGen)) []*types.RootBlock {
	blocks := make([]*types.RootBlock, n)
	genblock := func(i int, parent *types.RootBlock) *types.RootBlock {
		b := &RootBlockGen{i: i, chain: blocks, parent: parent, engine: engine}
		diff, err := engine.CalcDifficulty(nil, parent.Time(), parent.Header())
		if err != nil {
			panic(err) //only for test
		}
		b.header = makeRootBlockHeader(parent, diff)

		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}
		b.SetTotalDifficulty(new(big.Int).Add(parent.TotalDifficulty(), b.header.Difficulty))
		block := types.NewRootBlock(b.header, b.Headers, nil)
		block.Finalize(b.header.CoinbaseAmount, nil, common.Hash{})
		return block
	}
	for i := 0; i < n; i++ {
		block := genblock(i, parent)
		blocks[i] = block
		parent = block
	}
	return blocks
}

func makeRootBlockHeader(parent *types.RootBlock, difficulty *big.Int) *types.RootBlockHeader {
	var time uint64 = parent.Time() + 40

	return &types.RootBlockHeader{
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		Difficulty: difficulty,
		Number:     parent.Number() + 1,
		Time:       time,
	}
}

// makeHeaderChain creates a deterministic chain of Headers rooted at parent.
func makeRootBlockHeaderChain(parent *types.RootBlockHeader, n int, engine consensus.Engine, seed int) []*types.RootBlockHeader {
	blocks := makeRootBlockChain(types.NewRootBlockWithHeader(parent), n, engine, seed)
	headers := make([]*types.RootBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}

	return headers
}

// makeBlockChain creates a deterministic chain of blocks rooted at parent.
func makeRootBlockChain(parent *types.RootBlock, n int, engine consensus.Engine, seed int) []*types.RootBlock {
	blocks := GenerateRootBlockChain(parent, engine, n, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{0: byte(seed), 19: byte(i)}, FullShardKey: 0})
	})
	return blocks
}

// MinorBlockGen creates blocks for testing.
// See GenerateChain for a detailed explanation.
type MinorBlockGen struct {
	i       int
	parent  *types.MinorBlock
	chain   []*types.MinorBlock
	header  *types.MinorBlockHeader
	gasUsed uint64
	statedb *state.StateDB

	gasPool  *GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt

	config *params.ChainConfig
	engine consensus.Engine
}

// SetCoinbase sets the coinbase of the generated block.
// It can be called at most once.
func (b *MinorBlockGen) SetCoinbase(addr account.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("coinbase must be set before adding transactions")
		}
		panic("coinbase can only be set once")
	}
	b.header.Coinbase = addr
	b.gasPool = new(GasPool).AddGas(b.header.GasLimit.Value.Uint64())
}

// SetExtra sets the extra data field of the generated block.
func (b *MinorBlockGen) SetExtra(data []byte) {
	b.header.Extra = data
}

// SetNonce sets the nonce field of the generated block.
func (b *MinorBlockGen) SetNonce(nonce uint64) {
	b.header.Nonce = nonce
}

// AddTx adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTx panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. Notably, contract code relying on the BLOCKHASH instruction
// will panic during execution.
func (b *MinorBlockGen) AddTx(quarkChainConfig *config.QuarkChainConfig, tx *types.Transaction) {
	b.AddTxWithChain(quarkChainConfig, nil, tx)
}

// AddTxWithChain adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTxWithChain panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. If contract code relies on the BLOCKHASH instruction,
// the block in chain will be returned.
func (b *MinorBlockGen) AddTxWithChain(quarkChainConfig *config.QuarkChainConfig, bc *MinorBlockChain, tx *types.Transaction) {
	if b.gasPool == nil {
		b.SetCoinbase(account.Address{})
	}
	b.statedb.Prepare(tx.Hash(), common.Hash{}, len(b.txs))
	b.statedb.SetQuarkChainConfig(quarkChainConfig)
	toShardSize, err := quarkChainConfig.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err != nil {
		panic(err)
	}
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		panic(err)
	}

	_, receipt, _, err := ApplyTransaction(b.config, bc, b.gasPool, b.statedb, b.header, tx, &b.gasUsed, vm.Config{})
	if err != nil {
		panic(err)
	}
	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}

// Number returns the block number of the block being generated.
func (b *MinorBlockGen) Number() *big.Int {
	return new(big.Int).SetUint64(b.header.Number)
}

// TxNonce returns the next valid transaction nonce for the
// account at addr. It panics if the account does not exist.
func (b *MinorBlockGen) TxNonce(addr common.Address) uint64 {
	if !b.statedb.Exist(addr) {
		panic("account does not exist")
	}
	return b.statedb.GetNonce(addr)
}

// PrevBlock returns a previously generated block by number. It panics if
// num is greater or equal to the number of the block being generated.
// For index -1, PrevBlock returns the parent block given to GenerateChain.
func (b *MinorBlockGen) PrevBlock(index int) *types.MinorBlock {
	if index >= b.i {
		panic(fmt.Errorf("block index %d out of range (%d,%d)", index, -1, b.i))
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}

// OffsetTime modifies the time instance of a block, implicitly changing its
// associated difficulty. It's useful to test scenarios where forking is not
// tied to chain length directly.
func (b *MinorBlockGen) SetDifficulty(value uint64) {
	b.header.Difficulty = new(big.Int).SetUint64(value)
}

// GenerateMinorChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions and uncles added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their coinbase will be the zero address.
//
// Blocks created by GenerateChain do not contain valid proof of work
// values. Inserting them into MinorBlockChain requires use of FakePow or
// a similar non-validating proof of work implementation.
func GenerateMinorBlockChain(config *params.ChainConfig, quarkChainConfig *config.QuarkChainConfig, parent *types.MinorBlock, engine consensus.Engine, db ethdb.Database, n int, gen func(*config.QuarkChainConfig, int, *MinorBlockGen)) ([]*types.MinorBlock, []types.Receipts) {
	if config == nil {
		config = params.TestChainConfig
	}
	blocks, receipts := make([]*types.MinorBlock, n), make([]types.Receipts, n)
	genblock := func(i int, parent *types.MinorBlock, statedb *state.StateDB) (*types.MinorBlock, types.Receipts) {
		b := &MinorBlockGen{i: i, chain: blocks, parent: parent, statedb: statedb, config: config, engine: engine}
		block := parent.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
		b.header = block.Header()
		if gen != nil {
			gen(quarkChainConfig, i, b)
		}
		block = types.NewMinorBlock(b.header, block.Meta(), block.Transactions(), nil, block.TrackingData())
		for _, v := range b.txs {
			block.AddTx(v)
		}

		txCursor := &types.XShardTxCursorInfo{
			RootBlockHeight: 1,
		}
		statedb.SetTxCursorInfo(txCursor)
		coinbaseAmount := qkcCommon.BigIntMulBigRat(quarkChainConfig.GetShardConfigByFullShardID(quarkChainConfig.Chains[0].ShardSize|0).CoinbaseAmount, quarkChainConfig.RewardTaxRate)
		statedb.AddBalance(block.Header().Coinbase.Recipient, coinbaseAmount, qkcCommon.TokenIDEncode("QKC"))

		b.statedb.Finalise(true)
		rootHash, err := b.statedb.Commit(true)
		if err != nil {
			panic(fmt.Sprintf("state write error: %v", err))
		}
		if err := b.statedb.Database().TrieDB().Commit(rootHash, true); err != nil {
			panic(fmt.Sprintf("trie write error: %v", err))
		}
		temp := types.NewEmptyTokenBalances()
		temp.SetValue(coinbaseAmount, qkcCommon.TokenIDEncode("QKC"))
		block.Finalize(b.receipts, rootHash, statedb.GetGasUsed(), statedb.GetXShardReceiveGasUsed(), temp, statedb.GetTxCursorInfo())
		return block, b.receipts
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}
	return blocks, receipts
}

// makeHeaderChain creates a deterministic chain of Headers rooted at parent.
func makeHeaderChain(parent *types.MinorBlockHeader, metaData *types.MinorBlockMeta, n int, engine consensus.Engine, db ethdb.Database, seed int) []*types.MinorBlockHeader {
	blocks := makeBlockChain(types.NewMinorBlockWithHeader(parent, metaData), n, engine, db, seed)
	headers := make([]*types.MinorBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	return headers
}

// makeBlockChain creates a deterministic chain of blocks rooted at parent.
func makeBlockChain(parent *types.MinorBlock, n int, engine consensus.Engine, db ethdb.Database, seed int) []*types.MinorBlock {
	blocks, _ := GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), parent, engine, db, n, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{0: byte(seed), 19: byte(i)}, FullShardKey: 0})
	})
	return blocks
}
