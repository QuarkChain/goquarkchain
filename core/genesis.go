// Modified from go-ethereum under GNU Lesser General Public License
package core

import (
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

type Genesis struct {
	qkcConfig *config.QuarkChainConfig
}

func NewGenesis(config *config.QuarkChainConfig) *Genesis {
	genesis := Genesis{qkcConfig: config}
	return &genesis
}

type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

func (g *Genesis) CreateRootBlock() *types.RootBlock {
	genesis := g.qkcConfig.Root.Genesis
	header := types.RootBlockHeader{
		Version:         genesis.Version,
		Number:          genesis.Height,
		ParentHash:      common.HexToHash(genesis.HashPrevBlock),
		MinorHeaderHash: common.HexToHash(genesis.HashMerkleRoot),
		Time:            genesis.Timestamp,
		Difficulty:      new(big.Int).SetUint64(genesis.Difficulty),
	}

	return types.NewRootBlock(&header, make([]*types.MinorBlockHeader, 0, 0), nil)
}

func (g *Genesis) CreateMinorBlock(rootBlock *types.RootBlock, fullShardId uint32, db ethdb.Database) (*types.MinorBlock, error) {
	if db == nil {
		db = ethdb.NewMemDatabase()
	}
	if g.qkcConfig.ShardList[fullShardId] == nil || g.qkcConfig.ShardList[fullShardId].Genesis == nil {
		return nil, fmt.Errorf("genesis config for shard %d is missing", fullShardId)
	}

	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	branch := account.Branch{Value: fullShardId}
	genesis := g.qkcConfig.ShardList[fullShardId].Genesis

	for addrStr, gAcc := range genesis.Alloc {
		addr, err := account.CreatAddressFromBytes(common.Hex2Bytes(addrStr))
		if err != nil {
			return nil, err
		}
		//todo check full shard id
		recipient := new(common.Address)
		recipient.SetBytes(addr.Recipient.Bytes())
		statedb.AddBalance(*recipient, gAcc.Balance)
		statedb.SetCode(*recipient, gAcc.Code)
		statedb.SetNonce(*recipient, gAcc.Nonce)
		for key, value := range gAcc.Storage {
			statedb.SetState(*recipient, key, value)
		}
	}

	meta := types.MinorBlockMeta{
		Root:   statedb.IntermediateRoot(false),
		TxHash: common.HexToHash(genesis.HashMerkleRoot),
	}

	coinbaseAmount := new(serialize.Uint256)
	coinbaseAmount.Value = new(big.Int).Div(new(big.Int).Mul(g.qkcConfig.ShardList[fullShardId].CoinbaseAmount,
		g.qkcConfig.RewardTaxRate.Denom()), g.qkcConfig.RewardTaxRate.Num())

	gasLimit := new(serialize.Uint256)
	gasLimit.Value = new(big.Int).SetUint64(genesis.GasLimit)

	coinbase := account.CreatEmptyAddress(fullShardId)
	extra := make([]byte, len(genesis.ExtraData))
	copy(extra, genesis.ExtraData)

	header := types.MinorBlockHeader{
		Version:           genesis.Version,
		Number:            uint64(genesis.Height),
		Branch:            branch,
		ParentHash:        common.HexToHash(genesis.HashPrevMinorBlock),
		PrevRootBlockHash: rootBlock.Hash(),
		GasLimit:          gasLimit,
		MetaHash:          meta.Hash(),
		Coinbase:          coinbase,
		CoinbaseAmount:    coinbaseAmount,
		Time:              genesis.Timestamp,
		Difficulty:        new(big.Int).SetUint64(genesis.Difficulty),
		Extra:             extra,
	}

	return types.NewMinorBlock(&header, &meta, make(types.Transactions, 0, 0), make(types.Receipts, 0, 0), nil), nil
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The stored chain configuration will be updated if it is compatible (i.e. does not
// specify a fork block below the local head block). In case of a conflict, the
// error is a *params.ConfigCompatError and the new, unwritten config is returned.
//
// The returned chain configuration is never nil.
func SetupGenesisRootBlock(db ethdb.Database, genesis *Genesis) (*config.QuarkChainConfig, common.Hash, error) {
	if genesis == nil {
		log.Info("Writing default main-net genesis block")
		genesis = &Genesis{config.NewQuarkChainConfig()}
	}

	if genesis.qkcConfig == nil {
		genesis.qkcConfig = config.NewQuarkChainConfig()
	}

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		block, err := genesis.CommitRootBlock(db)
		return genesis.qkcConfig, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	hash := genesis.CreateRootBlock().Hash()
	if hash != stored {
		return genesis.qkcConfig, hash, &GenesisMismatchError{stored, hash}
	}

	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, genesis.qkcConfig)
		return genesis.qkcConfig, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return storedcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	return storedcfg, stored, nil
}
func SetupGenesisMinorBlock(db ethdb.Database, genesis *Genesis, rootBlock *types.RootBlock, fullShardId uint32) (*config.QuarkChainConfig, common.Hash, error) {
	if genesis == nil {
		log.Info("Writing default main-net genesis block")
		genesis = &Genesis{config.NewQuarkChainConfig()}
	}

	if genesis.qkcConfig == nil {
		genesis.qkcConfig = config.NewQuarkChainConfig()
	}

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		block, err := genesis.CommitMinorBlock(db, rootBlock, fullShardId)
		return genesis.qkcConfig, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	block, _ := genesis.CreateMinorBlock(rootBlock, fullShardId, db)
	hash := block.Hash()
	if hash != stored {
		return genesis.qkcConfig, hash, &GenesisMismatchError{stored, hash}
	}

	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, genesis.qkcConfig)
		return genesis.qkcConfig, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return storedcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	return storedcfg, stored, nil
}

// CommitRootBlock writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) CommitRootBlock(db ethdb.Database) (*types.RootBlock, error) {
	block := g.CreateRootBlock()
	if block.Number() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with number > 0")
	}
	rawdb.WriteTd(db, block.Hash(), block.NumberU64(), block.Difficulty())
	rawdb.WriteRootBlock(db, block)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())

	qkcConfig := g.qkcConfig
	if qkcConfig == nil {
		qkcConfig = config.NewQuarkChainConfig()
	}
	rawdb.WriteChainConfig(db, block.Hash(), qkcConfig)
	return block, nil
}

// CommitMinorBlock writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) CommitMinorBlock(db ethdb.Database, rootBlock *types.RootBlock, fullShardId uint32) (*types.MinorBlock, error) {
	if rootBlock == nil {
		rootBlock = g.CreateRootBlock()
	}
	block, err := g.CreateMinorBlock(rootBlock, fullShardId, db)
	if err != nil {
		return nil, err
	}
	if block.Number() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with number > 0")
	}
	rawdb.WriteTd(db, block.Hash(), block.Number(), block.Difficulty())
	rawdb.WriteMinorBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.Number(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.Number())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())

	qkcConfig := g.qkcConfig
	if qkcConfig == nil {
		qkcConfig = config.NewQuarkChainConfig()
	}
	rawdb.WriteChainConfig(db, block.Hash(), qkcConfig)
	return block, nil
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommitRootBlock(db ethdb.Database) *types.RootBlock {
	block, err := g.CommitRootBlock(db)
	if err != nil {
		panic(err)
	}
	return block
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommitMinorBlock(db ethdb.Database, rootBlock *types.RootBlock, fullShardId uint32) *types.MinorBlock {
	block, err := g.CommitMinorBlock(db, rootBlock, fullShardId)
	if err != nil {
		panic(err)
	}
	return block
}
