package slave

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cluster/slave/filters"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
	"math/big"
)

var (
	MINOR_BLOCK_HEADER_LIST_LIMIT = uint32(100)
	MINOR_BLOCK_BATCH_SIZE        = 50
	NEW_TRANSACTION_LIST_LIMIT    = 1000
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
	fullShardList := s.GetFullShardList()
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
				s.addShard(id, shard)
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
	fmt.Println("ForSync", shard.MinorBlockChain.GetBranch().Value, len(mHashList))
	for _, v := range mHashList {
		fmt.Println(v.String())
	}
	fmt.Println("===end")

	hashList := make([]common.Hash, 0)
	for _, hash := range mHashList {
		if !shard.MinorBlockChain.HasBlock(hash) {
			fmt.Println("hashBlock", shard.MinorBlockChain.GetBranch().Value, hash.String())
			hashList = append(hashList, hash)
		}
	}
	fmt.Println("real HahsList", shard.MinorBlockChain.GetBranch().Value, len(hashList))
	for _, v := range hashList {
		fmt.Println("", v.String())
	}
	fmt.Println("===read -end")

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
		if _, err := shard.AddBlockListForSync(bList); err != nil { //TODO?need fix?
			return nil, err
		}
		hashList = hashList[hLen:]
	}

	log.Info("sync request from master successful", "branch", branch, "peer-id", peerId, "block-size", hashLen)

	return shard.MinorBlockChain.GetShardStats()
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
		minedEnable, mined, err := shard.MinorBlockChain.GetMiningInfo(address.Recipient, tokenBalances)
		if err != nil {
			return nil, err
		}
		data.MinedBlocks = mined
		data.PoswMineableBlocks = minedEnable
		results = append(results, &data)
	}
	return results, err
}

func (s *SlaveBackend) GetMinorBlock(hash common.Hash, height *uint64, branch uint32) (*types.MinorBlock, error) {
	if shard, ok := s.shards[branch]; ok {
		return shard.GetMinorBlock(hash, height)
	}
	return nil, ErrMsg("GetMinorBlock")
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

func (s *SlaveBackend) GetTransactionListByAddress(address *account.Address, transferTokenID *uint64, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	branch, err := s.getBranch(address)
	if err != nil {
		return nil, nil, err
	}
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.GetTransactionListByAddress(address, transferTokenID, start, limit)
	}
	return nil, nil, ErrMsg("GetTransactionListByAddress")
}

func (s *SlaveBackend) GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	if shard, ok := s.shards[branch.Value]; ok {
		return shard.GetAllTx(start, limit)
	}
	return nil, nil, ErrMsg("GetAllTx")
}

func (s *SlaveBackend) GetLogs(args *qrpc.FilterQuery) ([]*types.Log, error) {
	if shard, ok := s.shards[args.FullShardId]; ok {
		return shard.GetLogsByFilterQuery(args)
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

func (s *SlaveBackend) GetWork(branch uint32, coinbaseAddr *account.Address) (*consensus.MiningWork, error) {
	if shard, ok := s.shards[branch]; ok {
		return shard.GetWork(coinbaseAddr)
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
	shrd, ok := s.shards[branch]
	if !ok {
		return nil, ErrMsg("GetMinorBlockListByHashList")
	}
	var (
		minorList = make([]*types.MinorBlock, 0, len(mHashList))
	)

	if len(mHashList) > 2*MINOR_BLOCK_BATCH_SIZE {
		return nil, errors.New("Bad number of minor blocks requested")
	}

	for _, hash := range mHashList {
		block, err := shrd.GetMinorBlock(hash, nil)
		if err != nil {
			return nil, err
		}
		minorList = append(minorList, block)
	}
	return minorList, nil
}

func (s *SlaveBackend) getMinorBlockHeaders(req *p2p.GetMinorBlockHeaderListRequest) ([]*types.MinorBlockHeader, error) {
	shard, ok := s.shards[req.Branch.Value]
	if !ok {
		return nil, ErrMsg("GetMinorBlockHeaderList")
	}

	var (
		headerList = make([]*types.MinorBlockHeader, 0, req.Limit)
		err        error
	)

	mHash := req.BlockHash
	if qcom.IsNil(shard.MinorBlockChain.GetHeader(mHash)) {
		return nil, fmt.Errorf("Minor block hash is not exist, minorHash: %s, slave id: %s ", mHash.Hex(), s.config.ID)
	}
	for i := uint32(0); i < req.Limit; i++ {
		header := shard.MinorBlockChain.GetHeader(mHash).(*types.MinorBlockHeader)
		headerList = append(headerList, header)
		if header.NumberU64() == 0 {
			return headerList, nil
		}
		mHash = header.ParentHash
	}
	return headerList, err
}

func (s *SlaveBackend) getMinorBlockHeadersWithSkip(gReq *p2p.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	shrd, ok := s.shards[gReq.Branch.Value]
	if !ok {
		return nil, ErrMsg("GetMinorBlockHeaderList")
	}

	var (
		height     uint64
		headerlist = make([]*types.MinorBlockHeader, 0, gReq.Limit)
		mTip       = shrd.MinorBlockChain.CurrentBlock()
	)
	if gReq.Type == qcom.SkipHash {
		iHeader := shrd.MinorBlockChain.GetHeaderByHash(gReq.GetHash())
		if qcom.IsNil(iHeader) {
			return headerlist, nil
		}
		mHeader := iHeader.(*types.MinorBlockHeader)
		iHeader = shrd.MinorBlockChain.GetHeaderByNumber(height)
		if qcom.IsNil(iHeader) || mHeader.Hash() != iHeader.Hash() {
			return headerlist, nil
		}
		height = mHeader.Number
	} else {
		height = *gReq.GetHeight()
	}

	for len(headerlist) < cap(headerlist) && height >= 0 && height < mTip.NumberU64() {
		iHeader := shrd.MinorBlockChain.GetHeaderByNumber(height)
		if qcom.IsNil(iHeader) {
			break
		}
		headerlist = append(headerlist, iHeader.(*types.MinorBlockHeader))
		if gReq.Direction == qcom.DirectionToGenesis {
			height -= uint64(gReq.Skip) + 1
		} else {
			height += uint64(gReq.Skip) + 1
		}
	}

	return headerlist, nil
}

func (s *SlaveBackend) GetMinorBlockHeaderList(gReq *p2p.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	if gReq.Type == qcom.SkipHash && gReq.Skip == 0 && gReq.Direction == qcom.DirectionToGenesis {
		return s.getMinorBlockHeaders(&p2p.GetMinorBlockHeaderListRequest{
			BlockHash: gReq.GetHash(),
			Branch:    gReq.Branch,
			Limit:     gReq.Limit,
			Direction: gReq.Direction,
		})
	}
	return s.getMinorBlockHeadersWithSkip(gReq)
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
	return ErrMsg("NewMinorBlock")
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

func (s *SlaveBackend) EventMux() *event.TypeMux {
	return s.eventMux
}

func (s *SlaveBackend) GetShardFilter(fullShardId uint32) (filters.ShardFilter, error) {
	if shrd, ok := s.shards[fullShardId]; ok {
		return shrd, nil
	}
	return nil, fmt.Errorf("bad params of fullShardId: %d\n", fullShardId)
}

func (s *SlaveBackend) CheckMinorBlocksInRoot(rootBlock *types.RootBlock) error {
	if rootBlock == nil {
		return errors.New("CheckMinorBlocksInRoot failed: invalid root block")
	}
	for _, header := range rootBlock.MinorBlockHeaders() {
		if shard, ok := s.shards[header.Branch.Value]; ok {
			if err := shard.CheckMinorBlock(header); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SlaveBackend) GetRootChainStakes(address account.Address, lastMinor common.Hash) (*big.Int,
	*account.Recipient, error) {
	for _, shrd := range s.shards {
		if shrd.Config.ChainID == 0 && shrd.Config.ShardID == 0 {
			return shrd.GetRootChainStakes(address, lastMinor)
		}
	}
	return nil, nil, errors.New("not chain 0 shard 0")
}
