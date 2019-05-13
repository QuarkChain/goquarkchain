package slave

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

func (s *SlaveBackend) GetUnconfirmedHeaderList() ([]*rpc.HeadersInfo, error) {
	var (
		headersInfoLst = make([]*rpc.HeadersInfo, 0)
	)
	for branch, shrd := range s.shards {
		if headers, err := shrd.GetUnconfirmedHeaderList(); err == nil {
			headersInfoLst = append(headersInfoLst, &rpc.HeadersInfo{
				Branch:     branch,
				HeaderList: headers,
			})
		} else {
			return nil, err
		}
	}
	return headersInfoLst, nil
}

// For local miner to submit mined blocks through master
func (s *SlaveBackend) AddMinorBlock(block *types.MinorBlock) error {

	if shrd, ok := s.shards[block.Branch().Value]; ok {

		if block.Header().ParentHash != shrd.MinorBlockChain.CurrentHeader().Hash() {
			// Tip changed, don't bother creating a fork
			return fmt.Errorf("add minor block dropped stale block mined locally branch: %d ,minor height: %d", block.Header().Branch.Value, block.Header().Number)
		}
		return shrd.AddMinorBlock(block)
	}
	return ErrMsg("AddMinorBlock")
}

func (s *SlaveBackend) AddRootBlock(block *types.RootBlock) (switched bool, err error) {
	for _, shrd := range s.shards {
		if err = shrd.AddRootBlock(block); err != nil {
			return
		}
	}
	return true, nil
}

// Create shards based on GENESIS config and root block height if they have
// not been created yet.
func (s *SlaveBackend) CreateShards(rootBlock *types.RootBlock) (err error) {
	var (
		fullShardIds = s.clstrCfg.Quarkchain.GetGenesisShardIds()
	)

	for _, id := range fullShardIds {
		shardCfg := s.clstrCfg.Quarkchain.GetShardConfigByFullShardID(id)
		if !s.coverShardId(id) || shardCfg.Genesis == nil {
			continue
		}
		if rootBlock.Header().Number >= shardCfg.Genesis.RootHeight {
			if shrd, ok := s.shards[id]; ok {
				return shrd.InitFromRootBlock(rootBlock)
			}
			shrd, err := shard.New(s.ctx, rootBlock, s.slaveConnManager, s.clstrCfg, id)
			if err != nil {
				log.Error("Failed to create shard", "slave id", s.config.ID, "shard id", shardCfg.ShardID, "err", err)
				return err
			}
			s.shards[id] = shrd
			break
		}
	}
	return nil
}

func (s *SlaveBackend) AddBlockListForSync(mHashList []common.Hash, peerId string, branch uint32) (*rpc.ShardStatus, error) {

	if mHashList == nil || len(mHashList) == 0 {
		return nil, errors.New("minor block hash list is empty")
	}

	shrd, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("AddBlockListForSync")
	}

	hashList := make([]common.Hash, 0)
	for _, hash := range mHashList {
		if !shrd.MinorBlockChain.HasBlock(hash) {
			hashList = append(hashList, hash)
		}
	}
	mBlockList, err := s.slaveConnManager.GetMinorBlocks(hashList, peerId, branch)
	if err != nil {
		return nil, err
	}

	if err := shrd.AddBlockListForSync(mBlockList); err != nil {
		return nil, err
	}
	return shrd.MinorBlockChain.GetShardStatus()
}

func (s *SlaveBackend) AddTx(tx *types.Transaction) (err error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId())
	if shrd, ok := s.shards[branch.Value]; ok {
		if err = shrd.MinorBlockChain.AddTx(tx); err != nil {
			return
		}
	} else {
		return ErrMsg("AddTx")
	}
	return
}

func (s *SlaveBackend) ExecuteTx(tx *types.Transaction, address *account.Address) ([]byte, error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId())
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.ExecuteTx(tx, address, nil)
	}
	return nil, ErrMsg("ExecuteTx")
}

func (s *SlaveBackend) GetTransactionCount(address *account.Address) (uint64, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.GetTransactionCount(address.Recipient, nil)
	}
	return 0, ErrMsg("GetTransactionCount")
}

func (s *SlaveBackend) GetBalances(address *account.Address) (*big.Int, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.GetBalance(address.Recipient, nil)
	}
	return nil, ErrMsg("GetBalances")
}

func (s *SlaveBackend) GetTokenBalance(address *account.Address) (*big.Int, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.GetBalance(address.Recipient, nil)
	}
	return nil, ErrMsg("GetTokenBalance")
}

func (s *SlaveBackend) GetAccountData(address *account.Address, height uint64) ([]*rpc.AccountBranchData, error) {

	var (
		results = make([]*rpc.AccountBranchData, 0)
		bt      []byte
		err     error
	)
	for branch, shrd := range s.shards {
		data := rpc.AccountBranchData{
			Branch: branch,
		}
		data.TransactionCount, err = shrd.MinorBlockChain.GetTransactionCount(address.Recipient, &height)
		data.Balance, err = shrd.MinorBlockChain.GetBalance(address.Recipient, &height)
		bt, err = shrd.MinorBlockChain.GetCode(address.Recipient, &height)
		data.IsContract = len(bt) > 0
		results = append(results, &data)
	}
	return results, err
}

func (s *SlaveBackend) GetMinorBlockByHash(hash common.Hash, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		if shrd.MinorBlockChain.GetMinorBlock(hash) == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
	}
	return nil, ErrMsg("GetMinorBlockByHash")
}

func (s *SlaveBackend) GetMinorBlockByHeight(height uint64, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		if shrd.MinorBlockChain.GetBlockByNumber(height) == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
	}
	return nil, ErrMsg("GetMinorBlockByHeight")
}

func (s *SlaveBackend) GetTransactionByHash(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, error) {
	if shrd, ok := s.shards[branch]; ok {
		minorBlock, idx := shrd.MinorBlockChain.GetTransactionByHash(txHash)
		return minorBlock, idx, nil
	}
	return nil, 0, ErrMsg("GetTransactionByHash")
}

func (s *SlaveBackend) GetTransactionReceipt(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, *types.Receipt, error) {
	if shrd, ok := s.shards[branch]; ok {
		block, index, receipts := shrd.MinorBlockChain.GetTransactionReceipt(txHash)
		return block, index, receipts, nil
	}
	return nil, 0, nil, ErrMsg("GetTransactionReceipt")
}

func (s *SlaveBackend) GetTransactionListByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.GetTransactionListByAddress(address, start, limit)
	}
	return nil, nil, ErrMsg("GetTransactionListByAddress")
}

func (s *SlaveBackend) GetLogs(address []*account.Address, start uint64, end uint64, branch uint32) ([]*types.Log, error) {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.GetLogs()
	}
	return nil, ErrMsg("GetLogs")
}

func (s *SlaveBackend) EstimateGas(tx *types.Transaction, address *account.Address) (uint32, error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId()).Value
	if shrd, ok := s.shards[branch]; ok {
		return shrd.MinorBlockChain.EstimateGas(tx, *address)
	}
	return 0, ErrMsg("EstimateGas")
}

func (s *SlaveBackend) GetStorageAt(address *account.Address, key common.Hash, height uint64) ([32]byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.GetStorageAt(address.Recipient, key, &height)
	}
	return common.Hash{}, ErrMsg("GetStorageAt")
}

func (s *SlaveBackend) GetCode(address *account.Address, height uint64) ([]byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.MinorBlockChain.GetCode(address.Recipient, &height)
	}
	return nil, ErrMsg("GetCode")
}

func (s *SlaveBackend) GasPrice(branch uint32) (uint64, error) {
	var (
		price *uint64
	)
	if shrd, ok := s.shards[branch]; ok {
		if price = shrd.MinorBlockChain.GasPrice(); price == nil {
			return 0, errors.New(fmt.Sprintf("Failed to get gas price, shard id : %d", shrd.Config.ShardID))
		}
	}
	return *price, ErrMsg("GasPrice")
}

func (s *SlaveBackend) GetWork(branch uint32) (*consensus.MiningWork, error) {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.GetWork()
	}
	return nil, ErrMsg("GetWork")
}

func (s *SlaveBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash, branch uint32) error {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.SubmitWork(headerHash, nonce, mixHash)
	}
	return ErrMsg("SubmitWork")
}

func (s *SlaveBackend) AddCrossShardTxListByMinorBlockHash(minorHash common.Hash,
	txList []*types.CrossShardTransactionDeposit, branch uint32) error {
	if shrd, ok := s.shards[branch]; ok {
		shrd.MinorBlockChain.AddCrossShardTxListByMinorBlockHash(minorHash, types.CrossShardTransactionDepositList{TXList: txList})
		return nil
	}
	return ErrMsg("AddCrossShardTxListByMinorBlockHash")
}

func (s *SlaveBackend) GetMinorBlockListByHashList(mHashList []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	var (
		minorList = make([]*types.MinorBlock, 0, len(mHashList))
		block     *types.MinorBlock
		err       error
	)

	if shad, ok := s.shards[branch]; ok {
		for _, hash := range mHashList {
			if hash == (common.Hash{}) {
				return nil, errors.New(fmt.Sprintf("empty hash in GetMinorBlockListByHashList func, slave_id: %s", s.config.ID))
			}
			block = shad.MinorBlockChain.GetMinorBlock(hash)
			if block != nil {
				minorList = append(minorList, block)
			}
		}
	} else {
		return nil, ErrMsg("GetMinorBlockListByHashList")
	}
	return minorList, err
}

func (s *SlaveBackend) GetMinorBlockListByDirection(mHash common.Hash,
	limit uint32, direction uint8, branch uint32) ([]*types.MinorBlock, error) {
	var (
		minorList = make([]*types.MinorBlock, 0, limit)
		block     *types.MinorBlock
		total     uint64
		err       error
	)

	shad, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockListByDirection")
	}

	for total <= uint64(limit) {
		if direction == 0 {
			total += 1
		} else {
			total -= 1
		}
		iBlock := shad.MinorBlockChain.GetBlockByNumber(block.Number() + total)
		if qcom.IsNil(iBlock) {
			return nil, errors.New(
				fmt.Sprintf("Failed to get minor block list by direction, slave_id: %s, start_hash: %v", s.config.ID, mHash))
		}
		minorList = append(minorList, iBlock.(*types.MinorBlock))
	}

	return minorList, err
}

func (s *SlaveBackend) HandleNewTip(tip *p2p.Tip) error {

	if len(tip.MinorBlockHeaderList) != 1 {
		return errors.New("minor block header list must have only one header")
	}

	mBHeader := tip.MinorBlockHeaderList[0]
	if shad, ok := s.shards[mBHeader.Branch.Value]; ok {
		return shad.HandleNewTip(tip.RootBlockHeader, mBHeader)
	}

	return ErrMsg("HandleNewTip")
}

func (s *SlaveBackend) NewMinorBlock(block *types.MinorBlock) error {

	if shrd, ok := s.shards[block.Header().Branch.Value]; ok {
		return shrd.NewMinorBlock(block)
	}
	return ErrMsg("MinorBlock")
}
