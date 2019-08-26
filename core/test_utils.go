package core

import (
	"encoding/hex"
	"errors"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	"math/big"
	"strings"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/posw"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

var (
	testDBPath = map[int]string{}
	// jiaozi 10^18
	jiaozi                       = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))
	testShardCoinbaseAmount      = new(big.Int).Mul(new(big.Int).SetUint64(5), jiaozi)
	testGenesisTokenID           = common.TokenIDEncode("QKC")
	testGenesisMinorTokenBalance = make(map[string]*big.Int)
)

type fakeEnv struct {
	db            ethdb.Database
	clusterConfig *config.ClusterConfig
}

func getOneDBPath() (int, string) {
	for index, v := range testDBPath {
		return index, v
	}
	panic("unexcepted err")
}
func getTestEnv(genesisAccount *account.Address, genesisMinorQuarkHash *uint64, chainSize *uint32, shardSize *uint32, genesisRootHeights *map[uint32]uint32, remoteMining *bool) *fakeEnv {
	if genesisAccount == nil {
		temp := account.CreatEmptyAddress(0)
		genesisAccount = &temp
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

	var fakeDb ethdb.Database
	var err error
	if len(testDBPath) != 0 {
		index, fileName := getOneDBPath()
		fakeDb, err = qkcdb.NewRDBDatabase(fileName, true)
		delete(testDBPath, index)
		checkErr(err)
	} else {
		fakeDb = ethdb.NewMemDatabase()
	}
	env := &fakeEnv{
		db:            fakeDb,
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
	env.clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	env.clusterConfig.EnableTransactionHistory = true
	env.clusterConfig.Quarkchain.MinMiningGasPrice = new(big.Int).SetInt64(0)
	env.clusterConfig.Quarkchain.XShardAddReceiptTimestamp = 1
	env.clusterConfig.Quarkchain.MinTXPoolGasPrice = new(big.Int).SetInt64(0)
	ids := env.clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := genesisAccount.AddressInShard(v)
		shardConfig := fakeClusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		if len(testGenesisMinorTokenBalance) != 0 {
			shardConfig.Genesis.Alloc[addr] = config.Allocation{Balances: testGenesisMinorTokenBalance}
			continue
		}
		temp := make(map[string]*big.Int)
		temp["QKC"] = new(big.Int).SetUint64(*genesisMinorQuarkHash)
		alloc := config.Allocation{Balances: temp}
		shardConfig.Genesis.Alloc[addr] = alloc
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

	if poswOverride != nil && *poswOverride {
		poswConfig := env.clusterConfig.Quarkchain.GetShardConfigByFullShardID(fullShardID).PoswConfig
		poswConfig.Enabled = true
		poswConfig.WindowSize = 3
	}

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

func setUp(genesisAccount *account.Address, genesisMinotQuarkash *uint64, shardSize *uint32) *fakeEnv {
	env := getTestEnv(genesisAccount, genesisMinotQuarkash, nil, shardSize, nil, nil)
	return env
}

func createTransferTransaction(
	shardState *MinorBlockChain, key []byte,
	fromAddress account.Address, toAddress account.Address,
	value *big.Int, gas *uint64, gasPrice *uint64, nonce *uint64, data []byte, gasTokenID *uint64, transferTokenID *uint64,
) *types.Transaction {
	t := shardState.GetGenesisToken()
	if gasTokenID == nil {
		gasTokenID = &t
	}
	if transferTokenID == nil {
		transferTokenID = &t
	}
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
		new(big.Int).SetUint64(realGasPrice), fromAddress.FullShardKey, toAddress.FullShardKey, fakeNetworkID, 0, data, *gasTokenID, *transferTokenID)

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

func CreateFakeMinorCanonicalPoSW(acc1 account.Address, shardId *uint32, genesisMinorQuarkash *uint64) (*MinorBlockChain, error) {
	env := setUp(&acc1, genesisMinorQuarkash, nil)
	chainConfig := env.clusterConfig.Quarkchain.Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := env.clusterConfig.Quarkchain.GetShardConfigByFullShardID(fullShardID)
	diffCalculator := &consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(shardConfig.Genesis.Difficulty)),
		AdjustmentCutoff:  shardConfig.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  shardConfig.DifficultyAdjustmentFactor,
	}
	poswFlag := true
	engineFlag := true
	shardState := createDefaultShardState(env, shardId, diffCalculator, &poswFlag, &engineFlag)
	return shardState, nil
}

func CreateTransferTx(shardState *MinorBlockChain, key []byte,
	from account.Address, to account.Address, value *big.Int, gas, gasPrice, nonce *uint64) *types.Transaction {
	if gasPrice == nil {
		gasPrice = new(uint64)
		*gasPrice = 0
	}
	if gas == nil {
		gas = new(uint64)
		*gas = 21000
	}
	return createTransferTransaction(shardState, key, from, to, value, gas, gasPrice, nonce, nil, nil, nil)
}
func CreateCallContractTx(shardState *MinorBlockChain, key []byte,
	from account.Address, to account.Address, value *big.Int, gas, gasPrice, nonce *uint64, data []byte) *types.Transaction {
	if gasPrice == nil {
		gasPrice = new(uint64)
		*gasPrice = 0
	}
	if gas == nil {
		gas = new(uint64)
		*gas = 21000
	}
	return createTransferTransaction(shardState, key, from, to, value, gas, gasPrice, nonce, data, nil, nil)
}

func GetPoSW(chain *MinorBlockChain) *posw.PoSW {
	return chain.posw.(*posw.PoSW)
}

/*
*solidity src missing
 */
const ContractCreationByteCode = "608060405234801561001057600080fd5b5061013f806100206000396000f300608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063942ae0a714610046575b600080fd5b34801561005257600080fd5b5061005b6100d6565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561009b578082015181840152602081019050610080565b50505050905090810190601f1680156100c85780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b60606040805190810160405280600a81526020017f68656c6c6f576f726c64000000000000000000000000000000000000000000008152509050905600a165627a7a72305820a45303c36f37d87d8dd9005263bdf8484b19e86208e4f8ed476bf393ec06a6510029"

/*

pragma solidity ^0.5.1;
contract Sample {
 function () payable external{}
 function kill() external {selfdestruct(msg.sender);}
}
*/

const ContractCreationByteCodePayable = "6080604052348015600f57600080fd5b5060948061001e6000396000f3fe6080604052600436106039576000357c01000000000000000000000000000000000000000000000000000000009004806341c0e1b514603b575b005b348015604657600080fd5b50604d604f565b005b3373ffffffffffffffffffffffffffffffffffffffff16fffea165627a7a7230582034cc4e996685dcadcc12db798751d2913034a3e963356819f2293c3baea4a18c0029"

/*
contract EventContract {
	event Hi(address indexed);
	constructor() public {
		emit Hi(msg.sender);
	}
	function f() public {
		emit Hi(msg.sender);
	}
}
*/
const ContractCreationWithEventByteCode = "608060405234801561001057600080fd5b503373ffffffffffffffffffffffffffffffffffffffff167fa9378d5bd800fae4d5b8d4c6712b2b64e8ecc86fdc831cb51944000fc7c8ecfa60405160405180910390a260c9806100626000396000f300608060405260043610603f576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806326121ff0146044575b600080fd5b348015604f57600080fd5b5060566058565b005b3373ffffffffffffffffffffffffffffffffffffffff167fa9378d5bd800fae4d5b8d4c6712b2b64e8ecc86fdc831cb51944000fc7c8ecfa60405160405180910390a25600a165627a7a72305820e7fc37b0c126b90719ace62d08b2d70da3ad34d3e6748d3194eb58189b1917c30029"

/*
pragma solidity ^0.5.1;

contract Storage {
	uint pos0;
	mapping(address => uint) pos1;
	function Save() public {
		pos1[msg.sender] = 5678;
	}
}
*/
const ContractWithStorage2 = "6080604052348015600f57600080fd5b5060c68061001e6000396000f3fe6080604052600436106039576000357c010000000000000000000000000000000000000000000000000000000090048063c2e171d714603e575b600080fd5b348015604957600080fd5b5060506052565b005b61162e600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555056fea165627a7a72305820fe440b2cadff2d38365becb4339baa8c7b29ce933a2ad1b43f49feea0e1f7a7e0029"

func ZFill64(input string) string {
	return strings.Repeat("0", 64-len(input)) + input
}

func CreateContract(mBlockChain *MinorBlockChain, key account.Key, fromAddress account.Address,
	toFullShardKey uint32, bytecode string) (*types.Transaction, error) {

	z := big.NewInt(0)
	one := big.NewInt(1)
	nonce, err := mBlockChain.GetTransactionCount(fromAddress.Recipient, nil)
	if err != nil {
		return nil, err
	}
	bytecodeb, err := hex.DecodeString(bytecode)

	if err != nil {
		return nil, err
	}
	t := mBlockChain.GetGenesisToken()
	evmTx := types.NewEvmContractCreation(nonce, z, 1000000, one, fromAddress.FullShardKey, toFullShardKey,
		mBlockChain.Config().NetworkID, 0, bytecodeb, t, t)

	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(key.Bytes()))
	if err != nil {
		return nil, err
	}
	evmTx, err = types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), prvKey)
	if err != nil {
		return nil, err
	}
	return &types.Transaction{TxType: types.EvmTx, EvmTx: evmTx}, nil
}
