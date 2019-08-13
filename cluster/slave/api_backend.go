package slave

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

func (s *SlaveBackend) GetUnconfirmedHeaderList() ([]*rpc.HeadersInfo, error) {
	var (
		headersInfoLst = make([]*rpc.HeadersInfo, 0)
	)
	for branch, shard := range s.shards {
		if headers, err := shard.GetUnconfirmedHeaderList(); err == nil {
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

func (s *SlaveBackend) AddRootBlock(block *types.RootBlock) (switched bool, err error) {
	switched = false
	for _, shard := range s.shards {
		if switched, err = shard.AddRootBlock(block); err != nil {
			return false, err
		}
	}
	return switched, nil
}

// Create shards based on GENESIS config and root block height if they have
// not been created yet.
func (s *SlaveBackend) CreateShards(rootBlock *types.RootBlock, forceInit bool) (err error) {
	fullShardList := s.getFullShardList()
	var g errgroup.Group
	for _, id := range fullShardList {
		id := id
		if shd, ok := s.shards[id]; ok {
			if forceInit {
				if err := shd.InitFromRootBlock(rootBlock); err != nil {
					return err
				}
			}
			continue
		}
		g.Go(func() error {
			shardCfg := s.clstrCfg.Quarkchain.GetShardConfigByFullShardID(id)
			if rootBlock.Header().Number >= shardCfg.Genesis.RootHeight {
				shard, err := shard.New(s.ctx, rootBlock, s.connManager, s.clstrCfg, id)
				if err != nil {
					log.Error("Failed to create shard", "slave id", s.config.ID, "shard id", shardCfg.ShardID, "err", err)
					return err
				}
				s.shards[id] = shard
				if err = shard.InitFromRootBlock(rootBlock); err != nil {
					shard.Stop()
					return err
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		for _, slv := range s.shards {
			slv.Stop()
		}
		s.shards = make(map[uint32]*shard.ShardBackend)
		return err
	}
	return nil
}

func (s *SlaveBackend) AddBlockListForSync(mHashList []common.Hash, peerId string, branch uint32) (*rpc.ShardStatus, error) {
	shard, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("AddBlockListForSync")
	}

	hashList := make([]common.Hash, 0)
	for _, hash := range mHashList {
		if !shard.MinorBlockChain.HasBlock(hash) {
			hashList = append(hashList, hash)
		}
	}

	var (
		BlockBatchSize = 100
		hashLen        = len(hashList)
		tHashList      []common.Hash
	)
	for len(hashList) > 0 {
		hLen := BlockBatchSize
		if len(hashList) > BlockBatchSize {
			tHashList = hashList[:BlockBatchSize]
		} else {
			tHashList = hashList
			hLen = len(hashList)
		}
		bList, err := s.connManager.GetMinorBlocks(tHashList, peerId, branch)
		if err != nil {
			log.Error("Failed to sync request from master", "branch", branch, "peer-id", peerId, "err", err)
			return nil, err
		}
		if len(bList) != hLen {
			return nil, errors.New("Failed to add minor blocks for syncing root block: length of downloaded block list is incorrect")
		}
		if err := shard.AddBlockListForSync(bList); err != nil {
			return nil, err
		}
		hashList = hashList[hLen:]
	}

	log.Info("sync request from master successful", "branch", branch, "peer-id", peerId, "block-size", hashLen)

	return shard.MinorBlockChain.GetShardStatus()
}

func (s *SlaveBackend) AddTx(tx *types.Transaction) (err error) {
	toShardSize, err := s.clstrCfg.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err != nil {
		return err
	}
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return err
	}
	fromShardSize, err := s.clstrCfg.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return err
	}
	if shard, ok := s.shards[tx.EvmTx.FromFullShardId()]; ok {
		return shard.MinorBlockChain.AddTx(tx)
	}
	return ErrMsg("AddTx")
}

func (s *SlaveBackend) ExecuteTx(tx *types.Transaction, address *account.Address, height *uint64) ([]byte, error) {
	fromShardSize, err := s.clstrCfg.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return nil, err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return nil, err
	}
	if shard, ok := s.shards[tx.EvmTx.FromFullShardId()]; ok {
		return shard.MinorBlockChain.ExecuteTx(tx, address, height)
	}
	return nil, ErrMsg("ExecuteTx")
}

func (s *SlaveBackend) GetTransactionCount(address *account.Address) (uint64, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return 0, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.MinorBlockChain.GetTransactionCount(address.Recipient, nil)
	}
	return 0, ErrMsg("GetTransactionCount")
}

func (s *SlaveBackend) GetBalances(address *account.Address) (map[uint64]*big.Int, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return nil, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		data, err := shard.MinorBlockChain.GetBalance(address.Recipient, nil)
		return data.GetBalanceMap(), err
	}
	return nil, ErrMsg("GetBalances")
}

func (s *SlaveBackend) GetTokenBalanceMap(address *account.Address) (map[uint64]*big.Int, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return nil, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		data, err := shard.MinorBlockChain.GetBalance(address.Recipient, nil)
		return data.GetBalanceMap(), err
	}
	return nil, ErrMsg("GetTokenBalance")
}

func (s *SlaveBackend) GetAccountData(address *account.Address, height *uint64) ([]*rpc.AccountBranchData, error) {
	var (
		results = make([]*rpc.AccountBranchData, 0)
		bt      []byte
		err     error
	)
	for branch, shard := range s.shards {
		data := rpc.AccountBranchData{
			Branch: branch,
		}
		if data.TransactionCount, err = shard.MinorBlockChain.GetTransactionCount(address.Recipient, height); err != nil {
			return nil, err
		}
		tokenBalances, err := shard.MinorBlockChain.GetBalance(address.Recipient, height)
		if err != nil {
			return nil, err
		}
		data.Balance = tokenBalances.Copy()
		if bt, err = shard.MinorBlockChain.GetCode(address.Recipient, height); err != nil {
			return nil, err
		}
		data.IsContract = len(bt) > 0
		results = append(results, &data)
	}
	return results, err
}

func (s *SlaveBackend) GetMinorBlockByHash(hash common.Hash, branch uint32) (*types.MinorBlock, error) {
	if shard, ok := s.shards[branch]; ok {
		mBlock := shard.MinorBlockChain.GetMinorBlock(hash)
		if mBlock == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shard.Config.ShardID))
		}
		return mBlock, nil
	}
	return nil, ErrMsg("GetMinorBlockByHash")
}

func (s *SlaveBackend) GetMinorBlockByHeight(height uint64, branch uint32) (*types.MinorBlock, error) {
	if shard, ok := s.shards[branch]; ok {
		mBlock := shard.MinorBlockChain.GetBlockByNumber(height)
		if qcom.IsNil(mBlock) {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shard.Config.ShardID))
		}
		return mBlock.(*types.MinorBlock), nil
	}
	return nil, ErrMsg("GetMinorBlockByHeight")
}

func (s *SlaveBackend) GetMinorBlockExtraInfo(block *types.MinorBlock, branch uint32) (*rpc.PoSWInfo, error) {
	if shard, ok := s.shards[branch]; ok {
		extra, err := shard.MinorBlockChain.PoswInfo(block)
		if err != nil {
			return nil, err
		}
		return extra, nil
	}
	return nil, ErrMsg("GetMinorBlockByHeight")
}

func (s *SlaveBackend) GetTransactionByHash(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, error) {
	if shard, ok := s.shards[branch]; ok {
		minorBlock, idx := shard.MinorBlockChain.GetTransactionByHash(txHash)
		return minorBlock, idx, nil
	}
	return nil, 0, ErrMsg("GetTransactionByHash")
}

func (s *SlaveBackend) GetTransactionReceipt(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, *types.Receipt, error) {
	if shard, ok := s.shards[branch]; ok {
		block, index, receipts := shard.MinorBlockChain.GetTransactionReceipt(txHash)
		return block, index, receipts, nil
	}
	return nil, 0, nil, ErrMsg("GetTransactionReceipt")
}

func (s *SlaveBackend) GetTransactionListByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return nil, nil, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.GetTransactionListByAddress(address, start, limit)
	}
	return nil, nil, ErrMsg("GetTransactionListByAddress")
}

func (s *SlaveBackend) GetLogs(topics [][]common.Hash, address []account.Address, start uint64, end uint64, branch uint32) ([]*types.Log, error) {
	if shard, ok := s.shards[branch]; ok {
		return shard.GetLogs(start, end, address, topics)
	}
	return nil, ErrMsg("GetLogs")
}

func (s *SlaveBackend) EstimateGas(tx *types.Transaction, address *account.Address) (uint32, error) {
	fromShardSize, err := s.clstrCfg.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return 0, err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return 0, err
	}
	branch := account.NewBranch(tx.EvmTx.FromFullShardId()).Value
	if shard, ok := s.shards[branch]; ok {
		return shard.MinorBlockChain.EstimateGas(tx, *address)
	}
	return 0, ErrMsg("EstimateGas")
}

func (s *SlaveBackend) GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return common.Hash{}, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.MinorBlockChain.GetStorageAt(address.Recipient, key, height)
	}
	return common.Hash{}, ErrMsg("GetStorageAt")
}

func (s *SlaveBackend) GetCode(address *account.Address, height *uint64) ([]byte, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return nil, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.MinorBlockChain.GetCode(address.Recipient, height)
	}
	return nil, ErrMsg("GetCode")
}

func (s *SlaveBackend) GasPrice(branch uint32, tokenID uint64) (uint64, error) {
	if shard, ok := s.shards[branch]; ok {
		price, err := shard.MinorBlockChain.GasPrice(tokenID)
		if err != nil {
			return 0, errors.New(fmt.Sprintf("Failed to get gas price, shard id : %d, err: %v", shard.Config.ShardID, err))
		}
		return price, nil
	}
	return 0, ErrMsg("GasPrice")
}

func (s *SlaveBackend) GetWork(branch uint32) (*consensus.MiningWork, error) {
	if shard, ok := s.shards[branch]; ok {
		return shard.GetWork()
	}
	return nil, ErrMsg("GetWork")
}

func (s *SlaveBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash, branch uint32) error {
	if shard, ok := s.shards[branch]; ok {
		return shard.SubmitWork(headerHash, nonce, mixHash)
	}
	return ErrMsg("SubmitWork")
}

func (s *SlaveBackend) AddCrossShardTxListByMinorBlockHash(minorHash common.Hash,
	txList []*types.CrossShardTransactionDeposit, branch uint32) error {
	if shard, ok := s.shards[branch]; ok {
		shard.MinorBlockChain.AddCrossShardTxListByMinorBlockHash(minorHash, types.CrossShardTransactionDepositList{TXList: txList})
		return nil
	}
	return ErrMsg("AddCrossShardTxListByMinorBlockHash")
}

func (s *SlaveBackend) GetMinorBlockListByHashList(mHashList []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	var (
		minorList = make([]*types.MinorBlock, 0, len(mHashList))
		block     *types.MinorBlock
	)

	shard, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockListByHashList")
	}
	for _, hash := range mHashList {
		if hash == (common.Hash{}) {
			return nil, errors.New(fmt.Sprintf("empty hash in GetMinorBlockListByHashList func, slave_id: %s", s.config.ID))
		}
		block = shard.MinorBlockChain.GetMinorBlock(hash)
		if block != nil {
			minorList = append(minorList, block)
		}
	}
	return minorList, nil
}

func (s *SlaveBackend) GetMinorBlockHeaderList(mHash common.Hash,
	limit uint32, direction uint8, branch uint32) ([]*types.MinorBlockHeader, error) {
	var (
		headerList = make([]*types.MinorBlockHeader, 0, limit)
		err        error
	)

	if direction != 0 /*directionToGenesis*/ {
		return nil, errors.New("bad direction")
	}

	shard, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockHeaderList")
	}
	if qcom.IsNil(shard.MinorBlockChain.GetHeader(mHash)) {
		return nil, fmt.Errorf("Minor block hash is not exist, minorHash: %s, slave id: %s ", mHash.Hex(), s.config.ID)
	}
	for i := uint32(0); i < limit; i++ {
		header := shard.MinorBlockChain.GetHeader(mHash).(*types.MinorBlockHeader)
		headerList = append(headerList, header)
		if header.NumberU64() == 0 {
			return headerList, nil
		}
		mHash = header.ParentHash
	}
	return headerList, err
}

func (s *SlaveBackend) HandleNewTip(req *rpc.HandleNewTipRequest) error {
	if len(req.MinorBlockHeaderList) != 1 {
		return errors.New("minor block header list must have only one header")
	}

	mBHeader := req.MinorBlockHeaderList[0]
	if shard, ok := s.shards[mBHeader.Branch.Value]; ok {
		return shard.HandleNewTip(req.RootBlockHeader, mBHeader, req.PeerID)
	}

	return ErrMsg("HandleNewTip")
}

func (s *SlaveBackend) NewMinorBlock(block *types.MinorBlock) error {
	if shard, ok := s.shards[block.Header().Branch.Value]; ok {
		return shard.NewMinorBlock(block)
	}
	return ErrMsg("MinorBlock")
}

func (s *SlaveBackend) GenTx(genTxs *rpc.GenTxRequest) error {
	var g errgroup.Group
	for _, shrd := range s.shards {
		sd := shrd
		g.Go(func() error {
			return sd.GenTx(genTxs)
		})
	}
	return g.Wait()
}

func (s *SlaveBackend) SetMining(mining bool) {
	for _, shrd := range s.shards {
		shrd.SetMining(mining)
	}
}
