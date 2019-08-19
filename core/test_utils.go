package core

import (
	"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/crypto"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// jiaozi 10^18
	jiaozi                  = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))
	testShardCoinbaseAmount = new(big.Int).Mul(new(big.Int).SetUint64(5), jiaozi)
)

type fakeEnv struct {
	db            ethdb.Database
	clusterConfig *config.ClusterConfig
}

func (e *fakeEnv) SetSuperAccount(addr account.Recipient) {
	e.clusterConfig.Quarkchain.SuperAccount = make([]account.Recipient, 0)
	e.clusterConfig.Quarkchain.SuperAccount = append(e.clusterConfig.Quarkchain.SuperAccount, addr)
}
func getTestEnv(genesisAccount []account.Address, genesisMinorQuarkHash *uint64, chainSize *uint32, shardSize *uint32, genesisRootHeights *map[uint32]uint32, remoteMining *bool) *fakeEnv {
	if genesisAccount == nil {
		temp := account.CreatEmptyAddress(0)
		genesisAccount = []account.Address{temp}
	}

	if genesisMinorQuarkHash == nil {
		temp := uint64(0)
		genesisMinorQuarkHash = &temp
	}

	if chainSize == nil {
		temp := uint32(2)
		chainSize = &temp
	}

	if shardSize == nil {
		temp := uint32(2)
		shardSize = &temp
	}

	if remoteMining == nil {
		temp := false
		remoteMining = &temp
	}

	if !common.IsP2(*shardSize) {
		panic(errors.New("shard size wrong"))
	}

	fakeClusterConfig := config.NewClusterConfig()
	env := &fakeEnv{
		db:            ethdb.NewMemDatabase(),
		clusterConfig: fakeClusterConfig,
	}
	env.clusterConfig.Quarkchain.NetworkID = 3
	env.clusterConfig.Quarkchain.Update(*chainSize, *shardSize, 10, 1)
	if *remoteMining {
		env.clusterConfig.Quarkchain.Root.ConsensusConfig.RemoteMine = true
		env.clusterConfig.Quarkchain.Root.ConsensusType = config.PoWDoubleSha256
		env.clusterConfig.Quarkchain.Root.Genesis.Difficulty = 10
	}
	env.clusterConfig.Quarkchain.Root.DifficultyAdjustmentCutoffTime = 40
	env.clusterConfig.Quarkchain.Root.DifficultyAdjustmentFactor = 1024
	env.clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	env.clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	env.clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	env.clusterConfig.EnableTransactionHistory = true
	addr0 := account.NewAddress(account.BytesToIdentityRecipient(ethCommon.Address{0}.Bytes()), 0)
	ids := env.clusterConfig.Quarkchain.GetGenesisShardIds()

	for index, vv := range genesisAccount {
		data := uint64(0)
		if index == 0 {
			data = uint64(*genesisMinorQuarkHash)
		}
		for _, v := range ids {
			addr := vv.AddressInShard(v)
			shardConfig := fakeClusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
			shardConfig.Genesis.Alloc[addr] = new(big.Int).SetUint64(data)
			addr = addr0.AddressInShard(v)
			shardConfig.Genesis.Alloc[addr] = new(big.Int).SetUint64(0)
		}
	}
	return env
}

func createDefaultShardState(env *fakeEnv, shardID *uint32, diffCalc consensus.DifficultyCalculator, poswOverride *bool, flagEngine *bool) *MinorBlockChain {
	if shardID == nil {
		temp := uint32(0)
		shardID = &temp
	}

	cacheConfig := &CacheConfig{
		TrieCleanLimit: 32,
		TrieDirtyLimit: 32,
		TrieTimeLimit:  5 * time.Minute,
		Disabled:       true, //update trieDB every block
	}
	rBlock := NewGenesis(env.clusterConfig.Quarkchain).MustCommitRootBlock(env.db)

	genesisManager := NewGenesis(env.clusterConfig.Quarkchain)

	fullShardID := env.clusterConfig.Quarkchain.Chains[0].ShardSize | *shardID
	genesisManager.MustCommitMinorBlock(env.db, rBlock, fullShardID)

	var shardState *MinorBlockChain
	var err error
	chainConfig := params.TestChainConfig
	if flagEngine != nil {
		shardState, err = NewMinorBlockChain(env.db, cacheConfig, chainConfig, env.clusterConfig, doublesha256.New(diffCalc, false, []byte{}), vm.Config{}, nil, fullShardID)
		if err != nil {
			panic(err)
		}
	} else {
		shardState, err = NewMinorBlockChain(env.db, cacheConfig, chainConfig, env.clusterConfig, new(consensus.FakeEngine), vm.Config{}, nil, fullShardID)
		if err != nil {
			panic(err)
		}
	}

	_, err = shardState.InitGenesisState(rBlock)
	if err != nil {
		panic(err)
	}
	return shardState

}

func setUp(genesisAccount []account.Address, genesisMinotQuarkash *uint64, shardSize *uint32) *fakeEnv {
	env := getTestEnv(genesisAccount, genesisMinotQuarkash, nil, shardSize, nil, nil)
	return env
}

func createTransferTransaction(
	shardState *MinorBlockChain, key []byte,
	fromAddress account.Address, toAddress account.Address,
	value *big.Int, gas *uint64, gasPrice *uint64, nonce *uint64, data []byte,
) *types.Transaction {
	fakeNetworkID := uint32(3) //default QuarkChain is nil
	realNonce, err := shardState.GetTransactionCount(fromAddress.Recipient, nil)
	if err != nil {
		panic(err)
	}
	if nonce != nil {
		realNonce = *nonce
	}

	realGasPrice := uint64(1)
	if gasPrice != nil {
		realGasPrice = *gasPrice
	}

	realGas := uint64(21000)
	if gas != nil {
		realGas = *gas
	}
	tempTx := types.NewEvmTransaction(realNonce, toAddress.Recipient, value, realGas,
		new(big.Int).SetUint64(realGasPrice), fromAddress.FullShardKey, toAddress.FullShardKey, fakeNetworkID, 0, data)

	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(key))
	if err != nil {
		panic(err)
	}
	tx, err := types.SignTx(tempTx, types.MakeSigner(fakeNetworkID), prvKey)
	if err != nil {
		panic(err)
	}
	return &types.Transaction{
		EvmTx:  tx,
		TxType: types.EvmTx,
	}
}

func transEvmTxToTx(tx *types.EvmTransaction) *types.Transaction {
	return &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  tx,
	}
}

func modifyNumber(block *types.RootBlock, Number uint64) *types.RootBlock {
	header := block.Header()
	header.Number = uint32(Number)
	return types.NewRootBlock(header, nil, []byte{})
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
