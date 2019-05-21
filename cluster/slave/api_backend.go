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

func (s *SlaveBackend) AddRootBlock(block *types.RootBlock) (switched bool, err error) {
	switched = false
	for _, shrd := range s.shards {
		if switched, err = shrd.AddRootBlock(block); err != nil {
			return false, err
		}
	}
	return switched, nil
}

// Create shards based on GENESIS config and root block height if they have
// not been created yet.
func (s *SlaveBackend) CreateShards(rootBlock *types.RootBlock) (err error) {
	log.Info(s.logInfo, "CreateShards root number", rootBlock.Number(), "root hash", rootBlock.Hash().String())
	defer log.Info(s.logInfo, "CreateShards end len(shards)", len(s.shards))
	fullShardList := s.getFullShardList()
	for _, id := range fullShardList {
		if _, ok := s.shards[id]; ok {
			continue
		}
		shardCfg := s.clstrCfg.Quarkchain.GetShardConfigByFullShardID(id)
		if rootBlock.Header().Number >= shardCfg.Genesis.RootHeight {
			shrd, err := shard.New(s.ctx, rootBlock, s.connManager, s.clstrCfg, id)
			if err != nil {
				log.Error("Failed to create shard", "slave id", s.config.ID, "shard id", shardCfg.ShardID, "err", err)
				return err
			}
			if err = shrd.InitFromRootBlock(rootBlock); err != nil {
				return err
			}
			s.shards[id] = shrd
		}
	}
	return nil
}

func (s *SlaveBackend) AddBlockListForSync(mHashList []common.Hash, peerId string, branch uint32) (*rpc.ShardStatus, error) {
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
			log.Error("Failed to sync request from master", "branch", branch, "peer id", peerId, "err", err)
			return nil, err
		}
		if len(bList) != hLen {
			return nil, errors.New("Failed to add minor blocks for syncing root block: length of downloaded block list is incorrect")
		}
		if err := shrd.AddBlockListForSync(bList); err != nil {
			return nil, err
		}
		hashList = hashList[hLen:]
	}

	log.Info("sync request from master successful", "branch", branch, "peer id", peerId, "block size", hashLen)

	return shrd.MinorBlockChain.GetShardStatus()
}

func (s *SlaveBackend) AddTx(tx *types.Transaction) (err error) {
	if shrd, ok := s.shards[tx.EvmTx.FromFullShardId()]; ok {
		return shrd.MinorBlockChain.AddTx(tx)
	}
	return ErrMsg("AddTx")
}

func (s *SlaveBackend) ExecuteTx(tx *types.Transaction, address *account.Address) ([]byte, error) {
	if shrd, ok := s.shards[tx.EvmTx.FromFullShardId()]; ok {
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

func (s *SlaveBackend) GetAccountData(address *account.Address, height *uint64) ([]*rpc.AccountBranchData, error) {
	var (
		results = make([]*rpc.AccountBranchData, 0)
		bt      []byte
		err     error
	)
	for branch, shrd := range s.shards {
		data := rpc.AccountBranchData{
			Branch: branch,
		}
		if data.TransactionCount, err = shrd.MinorBlockChain.GetTransactionCount(address.Recipient, height); err != nil {
			return nil, err
		}
		if data.Balance, err = shrd.MinorBlockChain.GetBalance(address.Recipient, height); err != nil {
			return nil, err
		}
		if bt, err = shrd.MinorBlockChain.GetCode(address.Recipient, height); err != nil {
			return nil, err
		}
		data.IsContract = len(bt) > 0
		results = append(results, &data)
	}
	return results, err
}

func (s *SlaveBackend) GetMinorBlockByHash(hash common.Hash, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		mBlock := shrd.MinorBlockChain.GetMinorBlock(hash)
		if mBlock == nil {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
		return mBlock, nil
	}
	return nil, ErrMsg("GetMinorBlockByHash")
}

func (s *SlaveBackend) GetMinorBlockByHeight(height uint64, branch uint32) (*types.MinorBlock, error) {
	if shrd, ok := s.shards[branch]; ok {
		mBlock := shrd.MinorBlockChain.GetBlockByNumber(height)
		if qcom.IsNil(mBlock) {
			return nil, errors.New(fmt.Sprintf("empty minor block in state, shard id: %d", shrd.Config.ShardID))
		}
		return mBlock.(*types.MinorBlock), nil
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
	branch := account.NewBranch(tx.EvmTx.FromFullShardId()).Value
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
	if shrd, ok := s.shards[branch]; ok {
		price, err := shrd.MinorBlockChain.GasPrice()
		if err != nil {
			return 0, errors.New(fmt.Sprintf("Failed to get gas price, shard id : %d, err: %v", shrd.Config.ShardID, err))
		}
		return price, nil
	}
	return 0, ErrMsg("GasPrice")
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
	)

	shad, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockListByHashList")
	}
	for _, hash := range mHashList {
		if hash == (common.Hash{}) {
			return nil, errors.New(fmt.Sprintf("empty hash in GetMinorBlockListByHashList func, slave_id: %s", s.config.ID))
		}
		block = shad.MinorBlockChain.GetMinorBlock(hash)
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

	shad, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockHeaderList")
	}
	if !qcom.IsNil(shad.MinorBlockChain.GetHeader(mHash)) {
		return nil, fmt.Errorf("Minor block hash is not exist, minorHash: %s, slave id: %s ", mHash.Hex(), s.config.ID)
	}
	for i := uint32(0); i < limit; i++ {
		header := shad.MinorBlockChain.GetHeader(mHash).(*types.MinorBlockHeader)
		headerList = append(headerList, header)
		if header.NumberU64() == 0 {
			return headerList, nil
		}
		mHash = header.ParentHash
	}
	return headerList, err
}

func (s *SlaveBackend) HandleNewTip(tip *p2p.Tip) error {
	if len(tip.MinorBlockHeaderList) != 1 {
		return errors.New("minor block header list must have only one header")
	}

	mBHeader := tip.MinorBlockHeaderList[0]
	if shad, ok := s.shards[mBHeader.Branch.Value]; ok {
		return shad.HandleNewTip(tip.RootBlockHeader, mBHeader)
	}

	return ErrMsg("HandleNewRootTip")
}

func (s *SlaveBackend) NewMinorBlock(block *types.MinorBlock) error {
	if shrd, ok := s.shards[block.Header().Branch.Value]; ok {
		return shrd.NewMinorBlock(block)
	}
	return ErrMsg("MinorBlock")
}
