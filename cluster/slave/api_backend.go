package slave

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
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

		return shrd.AddMinorBlock(block)
	}
	return ErrorBranch
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
		branch := account.NewBranch(id)
		if _, ok := s.shards[branch.Value]; ok {
			continue
		}
		shardCfg := s.clstrCfg.Quarkchain.GetShardConfigByFullShardID(id)
		if !s.coverShardId(id) || shardCfg.Genesis == nil {
			continue
		}
		if rootBlock.Header().Number >= shardCfg.Genesis.RootHeight {
			shrd, err := shard.NewShard(s.ctx, s.slaveConnManager, s.clstrCfg, id)
			if err != nil {
				log.Error("Failed to create shard", "slave id", s.config.ID, "shard id", shardCfg.ShardID, "err", err)
				return err
			}
			if err = shrd.InitFromRootBlock(rootBlock); err != nil {
				return err
			}
			s.shards[branch.Value] = shrd
		}
	}
	return
}

func (s *SlaveBackend) AddBlockListForSync(blockLst []*types.MinorBlock) (status *core.ShardStatus, err error) {

	if blockLst == nil {
		return
	}

	brch := blockLst[0].Header().Branch
	if shrd, ok := s.shards[brch.Value]; ok {
		if err = shrd.AddBlockListForSync(blockLst); err != nil {
			log.Error("Failed to ")
			return
		}
		return shrd.State.GetShardStatus()
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) AddTx(tx *types.Transaction) (err error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId())
	if shrd, ok := s.shards[branch.Value]; ok {
		if err = shrd.State.AddTx(tx); err != nil {
			return
		}
	} else {
		return ErrorBranch
	}
	return
}

func (s *SlaveBackend) ExecuteTx(tx *types.Transaction, address *account.Address) ([]byte, error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId())
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.ExecuteTx(tx, address, nil)
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GetTransactionCount(address *account.Address) (uint64, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.GetTransactionCount(address.Recipient, nil)
	}
	return 0, ErrorBranch
}

func (s *SlaveBackend) GetBalances(address *account.Address) (*big.Int, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.GetBalance(address.Recipient, nil)
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GetTokenBalance(address *account.Address) (*big.Int, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.GetBalance(address.Recipient, nil)
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GetAccountData(address account.Address, height uint64) ([]*rpc.AccountBranchData, error) {

	var (
		results = make([]*rpc.AccountBranchData, 0)
		bt      []byte
		err     error
	)
	for branch, shrd := range s.shards {
		data := rpc.AccountBranchData{
			Branch: branch,
		}
		data.TransactionCount, err = shrd.State.GetTransactionCount(address.Recipient, &height)
		data.Balance, err = shrd.State.GetBalance(address.Recipient, &height)
		bt, err = shrd.State.GetCode(address.Recipient, &height)
		data.IsContract = len(bt) > 0
		results = append(results, &data)
	}
	return results, err
}

func (s *SlaveBackend) GetMinorBlockByHash(hash common.Hash, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		if shrd.State.GetMinorBlock(hash) == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GetMinorBlockByHeight(height uint64, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		if shrd.State.GetBlockByNumber(height) == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GetTransactionByHash(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, error) {
	if shrd, ok := s.shards[branch]; ok {
		minorBlock, idx := shrd.State.GetTransactionByHash(txHash)
		return minorBlock, idx, nil
	}
	return nil, 0, ErrorBranch
}

func (s *SlaveBackend) GetTransactionReceipt(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, types.Receipts, error) {
	if shrd, ok := s.shards[branch]; ok {
		block, index, receipts := shrd.State.GetTransactionReceipt(txHash)
		return block, index, receipts, nil
	}
	return nil, 0, nil, ErrorBranch
}

func (s *SlaveBackend) GetTransactionListByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.GetTransactionListByAddress(address, start, limit)
	}
	return nil, nil, ErrorBranch
}

func (s *SlaveBackend) GetLogs(address []*account.Address, start uint64, end uint64, branch uint32) ([]*types.Log, error) {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.GetLogs()
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) EstimateGas(tx *types.Transaction, address *account.Address) (uint32, error) {

	var branch = account.NewBranch(tx.EvmTx.FromFullShardId()).Value
	if shrd, ok := s.shards[branch]; ok {
		return shrd.State.EstimateGas(tx, *address)
	}
	return 0, ErrorBranch
}

func (s *SlaveBackend) GetStorageAt(address *account.Address, key common.Hash, height uint64) ([32]byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.GetStorageAt(address.Recipient, key, &height)
	}
	return common.Hash{}, ErrorBranch
}

func (s *SlaveBackend) GetCode(address *account.Address, height uint64) ([]byte, error) {
	branch := s.getBranch(address)
	if shrd, ok := s.shards[branch.Value]; ok {
		return shrd.State.GetCode(address.Recipient, &height)
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) GasPrice(branch uint32) (uint64, error) {
	var (
		price *uint64
	)
	if shrd, ok := s.shards[branch]; ok {
		if price = shrd.State.GasPrice(); price == nil {
			return 0, errors.New(fmt.Sprintf("Failed to get gas price, shard id : %d", shrd.Config.ShardID))
		}
	}
	return *price, ErrorBranch
}

func (s *SlaveBackend) GetWork(branch uint32) (*consensus.MiningWork, error) {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.GetWork()
	}
	return nil, ErrorBranch
}

func (s *SlaveBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash, branch uint32) error {
	if shrd, ok := s.shards[branch]; ok {
		return shrd.SubmitWork(headerHash, nonce, mixHash)
	}
	return ErrorBranch
}

func (s *SlaveBackend) AddCrossShardTxListByMinorBlockHash(minorHash common.Hash,
	txList []*types.CrossShardTransactionDeposit, branch uint32) error {
	if shrd, ok := s.shards[branch]; ok {
		shrd.State.AddCrossShardTxListByMinorBlockHash(minorHash, types.CrossShardTransactionDepositList{TXList: txList})
		return nil
	}
	return ErrorBranch
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
			block = shad.State.GetMinorBlock(hash)
			if block != nil {
				minorList = append(minorList, block)
			}
		}
	} else {
		return nil, ErrorBranch
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
		return nil, ErrorBranch
	}

	for total <= uint64(limit) {
		if direction == 0 {
			total += 1
		} else {
			total -= 1
		}
		iBlock := shad.State.GetBlockByNumber(block.Number() + total)
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

	return ErrorBranch
}
