package rpc

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type Ping struct {
	id            serialize.LimitedSizeByteSlice4
	chainMaskList ChainMasks
	root_tip      types.RootBlock `json:"root_tip" ser:"nil"`
}

type ChainMasks [] uint16 // ChainMask

func (ChainMasks) GetLenByteSize() int {
	return 4
}

type SlaveInfos []SlaveInfo

func (SlaveInfos) GetLenByteSize() int {
	return 4
}

type Pong struct {
	id serialize.LimitedSizeByteSlice4 `json:"id" gencodec:"required"`
	// ChainMask
	chainMaskList uint16 `json:"chain_mask_list" gencodec:"required"`
}

type SlaveInfo struct {
	id            serialize.LimitedSizeByteSlice4 `json:"id" gencodec:"required"`
	host          serialize.LimitedSizeByteSlice4 `json:"host" gencodec:"required"`
	port          uint16                          `json:"port" gencodec:"required"`
	chainMaskList ChainMasks                      `json:"chain_mask_list" gencodec:"required"`
}

type ConnectToSlavesRequest struct {
	slaveInfoList SlaveInfos `json:"slave_info_list" gencodec:"required"`
}

type LargeBytes []serialize.LimitedSizeByteSlice4

func (LargeBytes) GetLenByteSize() int {
	return 4
}

type ConnectToSlavesResponse struct {
	resultList LargeBytes `json:"result_list" gencodec:"required"`
}

type ArtificialTxConfig struct {
	targetRootBlockTime  uint32 `json:"target_root_block_time" gencodec:"required"`
	targetMinorBlockTime uint32 `json:"target_minor_block_time" gencodec:"required"`
}

type MineRequest struct {
	artificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
	mining             bool               `json:"mining" gencodec:"required"`
}

type MineResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type GenTxRequest struct {
	numTxPerShard uint32            `json:"num_tx_per_shard" gencodec:"required"`
	xShardPercent uint32            `json:"x_shard_percent" gencodec:"required"`
	tx            types.Transaction `json:"tx" gencodec:"required"`
}

type GenTxResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type CreateClusterPeerConnectionRequest struct {
	clusterPeerId uint64 `json:"cluster_peer_id" gencodec:"required"`
}

type CreateClusterPeerConnectionResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type DestroyClusterPeerConnectionCommand struct {
	clusterPeerId uint64 `json:"cluster_peer_id" gencodec:"required"`
}

type GetMinorBlockRequest struct {
	branch         account.Branch `json:"branch" gencodec:"required"`
	minorBlockHash common.Hash    `json:"minor_block_hash" gencodec:"required"`
	height         uint64         `json:"height" gencodec:"required"`
}

type GetMinorBlockResponse struct {
	errorCode  uint32           `json:"error_code" gencodec:"required"`
	minorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
}

type GetTransactionRequest struct {
	txHash common.Hash    `json:"tx_hash" gencodec:"required"`
	branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionResponse struct {
	errorCode  uint32           `json:"error_code" gencodec:"required"`
	minorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
	index      uint32           `json:"index" gencodec:"required"`
}

type ExecuteTransactionRequest struct {
	tx          types.Transaction `json:"tx" gencodec:"required"`
	fromAddress account.Address   `json:"from_address" gencodec:"required"`
	blockHeight uint64            `json:"block_height" ser:"nil"`
}

type ExecuteTransactionResponse struct {
	errorCode uint32
	result    serialize.LimitedSizeByteSlice4
}

type GetTransactionReceiptRequest struct {
	txHash common.Hash
	branch account.Branch
}

type GetTransactionReceiptResponse struct {
	errorCode  uint32
	minorBlock types.MinorBlock
	index      uint32
	receipt    types.Receipt
}

type GetTransactionListByAddressRequest struct {
	address account.Address
	start   serialize.LimitedSizeByteSlice4
	limit   uint32
}

type TransactionDetail struct {
	txHash          common.Hash
	fromAddress     account.Address
	toAddress       account.Address `json:"to_address" ser:"nil"`
	value           common.Hash
	blockHeight     uint64
	timestamp       uint64
	success         bool
	gasTokenId      uint64
	transferTokenId uint64
}

type TransactionDetails []TransactionDetail

func (TransactionDetails) GetLenByteSize() int {
	return 4
}

type GetTransactionListByAddressResponse struct {
	errorCode uint32
	txList    TransactionDetails
	next      serialize.LimitedSizeByteSlice4
}

type AddRootBlockRequest struct {
	rootBlock    types.RootBlock
	expectSwitch bool
}

type AddRootBlockResponse struct {
	errorCode uint32
	switched  bool
}

type EcoInfo struct {
	branch                           account.Branch
	height                           uint64
	coinbaseAmount                   common.Hash
	difficulty                       *big.Int
	unconfirmedHeadersCoinbaseAmount common.Hash
}

type GetEcoInfoListRequest struct {
}

type EcoInfos []EcoInfo

func (EcoInfos) GetLenByteSize() int {
	return 4
}

type GetEcoInfoListResponse struct {
	errorCode   uint32
	ecoInfoList EcoInfos
}

type GetNextBlockToMineRequest struct {
	branch             account.Branch
	address            account.Address
	artificialTxConfig ArtificialTxConfig
}

type GetNextBlockToMineResponse struct {
	errorCode uint32
	block     types.MinorBlock
}

type AddMinorBlockRequest struct {
	minorBlockData serialize.LimitedSizeByteSlice4
}

type AddMinorBlockResponse struct {
	errorCode uint32
}

type MinorBlockHeaders []types.MinorBlockHeader

func (MinorBlockHeaders) GetLenByteSize() int {
	return 4
}

type HeadersInfo struct {
	branch     account.Branch
	headerList MinorBlockHeaders
}

type GetUnconfirmedHeadersRequest struct {
}

type HeadersInfos []HeadersInfo

func (HeadersInfos) GetLenByteSize() int {
	return 4
}

type GetUnconfirmedHeadersResponse struct {
	errorCode       uint32
	headersInfoList HeadersInfos
}

type GetAccountDataRequest struct {
	address     account.Address
	blockHeight uint64 `json:"block_height" ser:"nil"`
}

type TokenBalancePair struct {
	tokenId uint64
	balance common.Hash
}

type TokenBalancePairs []TokenBalancePair

func (TokenBalancePairs) GetLenByteSize() int {
	return 4
}

type AccountBranchData struct {
	branch           account.Branch
	transactionCount common.Hash
	tokenBalances    TokenBalancePairs
	isContract       bool
}

type AccountBranchDatas []AccountBranchData

func (AccountBranchDatas) GetLenByteSize() int {
	return 4
}

type GetAccountDataResponse struct {
	errorCode             uint32
	accountBranchDataList AccountBranchDatas
}

type AddTransactionRequest struct {
	tx types.Transaction
}

type AddTransactionResponse struct {
	errorCode uint32
}

type ShardStats struct {
	branch             account.Branch
	height             uint64
	difficulty         *big.Int
	coinbaseAddress    account.Address
	timestamp          uint64
	txCount60s         uint32
	pendingTxCount     uint32
	totalTxCount       uint32
	blockCount60s      uint32
	staleBlockCount60s uint32
	lastBlockTime      uint32
}

type HashBytes []common.Hash

func (HashBytes) GetLenByteSize() int {
	return 4
}

type SyncMinorBlockListRequest struct {
	minorBlockHashList HashBytes
	branch             account.Branch
	clusterPeerId      uint64
}

type SyncMinorBlockListResponse struct {
	errorCode  uint32
	shardStats ShardStats `json:"shard_stats" ser:"nil"`
}

type AddMinorBlockHeaderRequest struct {
	minorBlockHeader types.MinorBlockHeader
	txCount          uint32
	xShardTxCount    uint32
	shardStats       ShardStats
}

type AddMinorBlockHeaderResponse struct {
	errorCode          uint32
	artificialTxConfig ArtificialTxConfig
}

type AddXshardTxListRequest struct {
	branch         account.Branch
	minorBlockHash common.Hash
	txList         types.CrossShardTransactionList
}

type AddXshardTxListResponse struct {
	errorCode uint32
}

type AddXshardTxListRequests []AddXshardTxListRequest

func (AddXshardTxListRequests) GetLenByteSize() int {
	return 1
}

type BatchAddXshardTxListRequest struct {
	addXshardTxListRequestList AddXshardTxListRequests
}

type BatchAddXshardTxListResponse struct {
	errorCode uint32
}

type AddressByte []account.Address

func (AddressByte) GetLenByteSize() int {
	return 1
}

type Uint256s []serialize.Uint256

func (Uint256s) GetLenByteSize() int {
	return 4
}

type Uint256Bytes []Uint256s

func (Uint256Bytes) GetLenByteSize() int {
	return 4
}

type GetLogRequest struct {
	branch     account.Branch
	addresses  AddressByte
	topics     Uint256Bytes
	startBlock uint64
	endBlock   uint64
}

type LogByte []types.Log

func (LogByte) GetLenByteSize() int {
	return 4
}

type GetLogResponse struct {
	errorCode uint32
	logs      LogByte
}

type EstimateGasRequest struct {
	tx          types.Transaction
	fromAddress account.Address
}

type EstimateGasResponse struct {
	errorCode uint32
	result    uint32
}

type GetStorageRequest struct {
	address     account.Address
	key         common.Hash
	blockHeight uint64 `json:"block_height" ser:"nil"`
}

type GetStorageResponse struct {
	errorCode uint32
	result    serialize.Uint256
}

type GetCodeRequest struct {
	address     account.Address
	blockHeight uint64 `json:"block_height" ser:"nil"`
}

type GetCodeResponse struct {
	errorCode uint32
	result    serialize.LimitedSizeByteSlice4
}

type GasPriceRequest struct {
	branch account.Branch
}

type GasPriceResponse struct {
	errorCode uint32
	result    uint64
}

type GetWorkRequest struct {
	branch account.Branch
}

type GetWorkResponse struct {
	errorCode  uint32
	headerHash common.Hash
	height     uint64
	difficulty *big.Int
}

type SubmitWorkRequest struct {
	branch     account.Branch
	headerHash common.Hash
	nonce      uint64
	mixHash    common.Hash
}

type SubmitWorkResponse struct {
	errorCode uint32
	success   bool
}
