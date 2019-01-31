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
	RootTip       types.RootBlock `json:"root_tip" ser:"nil"`
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
	errorCode uint32                          `json:"error_code" gencodec:"required"`
	result    serialize.LimitedSizeByteSlice4 `json:"result" gencodec:"required"`
}

type GetTransactionReceiptRequest struct {
	txHash common.Hash    `json:"tx_hash" gencodec:"required"`
	branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionReceiptResponse struct {
	errorCode  uint32           `json:"error_code" gencodec:"required"`
	minorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
	index      uint32           `json:"index" gencodec:"required"`
	receipt    types.Receipt    `json:"receipt" gencodec:"required"`
}

type GetTransactionListByAddressRequest struct {
	address account.Address                 `json:"address" gencodec:"required"`
	start   serialize.LimitedSizeByteSlice4 `json:"start" gencodec:"required"`
	limit   uint32                          `json:"limit" gencodec:"required"`
}

type TransactionDetail struct {
	txHash          common.Hash     `json:"tx_hash" gencodec:"required"`
	fromAddress     account.Address `json:"from_address" gencodec:"required"`
	toAddress       account.Address `json:"to_address" ser:"nil"`
	value           common.Hash     `json:"value" gencodec:"required"`
	blockHeight     uint64          `json:"block_height" gencodec:"required"`
	timestamp       uint64          `json:"timestamp" gencodec:"required"`
	success         bool            `json:"success" gencodec:"required"`
	gasTokenId      uint64          `json:"gas_token_id" gencodec:"required"`
	transferTokenId uint64
}

type TransactionDetails []TransactionDetail

func (TransactionDetails) GetLenByteSize() int {
	return 4
}

type GetTransactionListByAddressResponse struct {
	errorCode uint32                          `json:"error_code" gencodec:"required"`
	txList    TransactionDetails              `json:"tx_list" gencodec:"required"`
	next      serialize.LimitedSizeByteSlice4 `json:"next" gencodec:"required"`
}

type AddRootBlockRequest struct {
	rootBlock    types.RootBlock `json:"root_block" gencodec:"required"`
	expectSwitch bool            `json:"expect_switch" gencodec:"required"`
}

type AddRootBlockResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
	switched  bool   `json:"switched" gencodec:"required"`
}

type EcoInfo struct {
	branch                           account.Branch `json:"branch" gencodec:"required"`
	height                           uint64         `json:"height" gencodec:"required"`
	coinbaseAmount                   common.Hash    `json:"coinbase_amount" gencodec:"required"`
	difficulty                       *big.Int       `json:"difficulty" gencodec:"required"`
	unconfirmedHeadersCoinbaseAmount common.Hash    `json:"unconfirmed_headers_coinbase_amount" gencodec:"required"`
}

type GetEcoInfoListRequest struct {
}

type EcoInfos []EcoInfo

func (EcoInfos) GetLenByteSize() int {
	return 4
}

type GetEcoInfoListResponse struct {
	errorCode   uint32   `json:"error_code" gencodec:"required"`
	ecoInfoList EcoInfos `json:"eco_info_list" gencodec:"required"`
}

type GetNextBlockToMineRequest struct {
	branch             account.Branch     `json:"branch" gencodec:"required"`
	address            account.Address    `json:"address" gencodec:"required"`
	artificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type GetNextBlockToMineResponse struct {
	errorCode uint32           `json:"error_code" gencodec:"required"`
	block     types.MinorBlock `json:"block" gencodec:"required"`
}

type AddMinorBlockRequest struct {
	minorBlockData serialize.LimitedSizeByteSlice4 `json:"minor_block_data" gencodec:"required"`
}

type AddMinorBlockResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type MinorBlockHeaders []types.MinorBlockHeader

func (MinorBlockHeaders) GetLenByteSize() int {
	return 4
}

type HeadersInfo struct {
	branch     account.Branch    `json:"branch" gencodec:"required"`
	headerList MinorBlockHeaders `json:"header_list" gencodec:"required"`
}

type GetUnconfirmedHeadersRequest struct {
}

type HeadersInfos []HeadersInfo

func (HeadersInfos) GetLenByteSize() int {
	return 4
}

type GetUnconfirmedHeadersResponse struct {
	errorCode       uint32       `json:"error_code" gencodec:"required"`
	headersInfoList HeadersInfos `json:"headers_info_list" gencodec:"required"`
}

type GetAccountDataRequest struct {
	address     account.Address `json:"address" gencodec:"required"`
	blockHeight uint64          `json:"block_height" ser:"nil"`
}

type TokenBalancePair struct {
	tokenId uint64      `json:"token_id" gencodec:"required"`
	balance common.Hash `json:"balance" gencodec:"required"`
}

type TokenBalancePairs []TokenBalancePair

func (TokenBalancePairs) GetLenByteSize() int {
	return 4
}

type AccountBranchData struct {
	branch           account.Branch    `json:"branch" gencodec:"required"`
	transactionCount common.Hash       `json:"transaction_count" gencodec:"required"`
	tokenBalances    TokenBalancePairs `json:"token_balances" gencodec:"required"`
	isContract       bool              `json:"is_contract" gencodec:"required"`
}

type AccountBranchDatas []AccountBranchData

func (AccountBranchDatas) GetLenByteSize() int {
	return 4
}

type GetAccountDataResponse struct {
	errorCode             uint32             `json:"error_code" gencodec:"required"`
	accountBranchDataList AccountBranchDatas `json:"account_branch_data_list" gencodec:"required"`
}

type AddTransactionRequest struct {
	tx types.Transaction `json:"tx" gencodec:"required"`
}

type AddTransactionResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type ShardStats struct {
	branch             account.Branch  `json:"branch" gencodec:"required"`
	height             uint64          `json:"height" gencodec:"required"`
	difficulty         *big.Int        `json:"difficulty" gencodec:"required"`
	coinbaseAddress    account.Address `json:"coinbase_address" gencodec:"required"`
	timestamp          uint64          `json:"timestamp" gencodec:"required"`
	txCount60s         uint32          `json:"tx_count_60_s" gencodec:"required"`
	pendingTxCount     uint32          `json:"pending_tx_count" gencodec:"required"`
	totalTxCount       uint32          `json:"total_tx_count" gencodec:"required"`
	blockCount60s      uint32          `json:"block_count_60_s" gencodec:"required"`
	staleBlockCount60s uint32          `json:"stale_block_count_60_s" gencodec:"required"`
	lastBlockTime      uint32          `json:"last_block_time" gencodec:"required"`
}

type HashBytes []common.Hash

func (HashBytes) GetLenByteSize() int {
	return 4
}

type SyncMinorBlockListRequest struct {
	minorBlockHashList HashBytes      `json:"minor_block_hash_list" gencodec:"required"`
	branch             account.Branch `json:"branch" gencodec:"required"`
	clusterPeerId      uint64         `json:"cluster_peer_id" gencodec:"required"`
}

type SyncMinorBlockListResponse struct {
	errorCode  uint32     `json:"error_code" gencodec:"required"`
	shardStats ShardStats `json:"shard_stats" ser:"nil"`
}

type AddMinorBlockHeaderRequest struct {
	minorBlockHeader types.MinorBlockHeader `json:"minor_block_header" gencodec:"required"`
	txCount          uint32                 `json:"tx_count" gencodec:"required"`
	xShardTxCount    uint32                 `json:"x_shard_tx_count" gencodec:"required"`
	shardStats       ShardStats             `json:"shard_stats" gencodec:"required"`
}

type AddMinorBlockHeaderResponse struct {
	errorCode          uint32             `json:"error_code" gencodec:"required"`
	artificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type AddXshardTxListRequest struct {
	branch         account.Branch                  `json:"branch" gencodec:"required"`
	minorBlockHash common.Hash                     `json:"minor_block_hash" gencodec:"required"`
	txList         types.CrossShardTransactionList `json:"tx_list" gencodec:"required"`
}

type AddXshardTxListResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
}

type AddXshardTxListRequests []AddXshardTxListRequest

func (AddXshardTxListRequests) GetLenByteSize() int {
	return 1
}

type BatchAddXshardTxListRequest struct {
	addXshardTxListRequestList AddXshardTxListRequests `json:"add_xshard_tx_list_request_list" gencodec:"required"`
}

type BatchAddXshardTxListResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
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
	branch     account.Branch `json:"branch" gencodec:"required"`
	addresses  AddressByte    `json:"addresses" gencodec:"required"`
	topics     Uint256Bytes   `json:"topics" gencodec:"required"`
	startBlock uint64         `json:"start_block" gencodec:"required"`
	endBlock   uint64         `json:"end_block" gencodec:"required"`
}

type LogByte []types.Log

func (LogByte) GetLenByteSize() int {
	return 4
}

type GetLogResponse struct {
	errorCode uint32  `json:"error_code" gencodec:"required"`
	logs      LogByte `json:"logs" gencodec:"required"`
}

type EstimateGasRequest struct {
	tx          types.Transaction `json:"tx" gencodec:"required"`
	fromAddress account.Address   `json:"from_address" gencodec:"required"`
}

type EstimateGasResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
	result    uint32 `json:"result" gencodec:"required"`
}

type GetStorageRequest struct {
	address     account.Address `json:"address" gencodec:"required"`
	key         common.Hash     `json:"key" gencodec:"required"`
	blockHeight uint64          `json:"block_height" ser:"nil"`
}

type GetStorageResponse struct {
	errorCode uint32            `json:"error_code" gencodec:"required"`
	result    serialize.Uint256 `json:"result" gencodec:"required"`
}

type GetCodeRequest struct {
	address     account.Address `json:"address" gencodec:"required"`
	blockHeight uint64          `json:"block_height" ser:"nil"`
}

type GetCodeResponse struct {
	errorCode uint32                          `json:"error_code" gencodec:"required"`
	result    serialize.LimitedSizeByteSlice4 `json:"result" gencodec:"required"`
}

type GasPriceRequest struct {
	branch account.Branch `json:"branch" gencodec:"required"`
}

type GasPriceResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
	result    uint64 `json:"result" gencodec:"required"`
}

type GetWorkRequest struct {
	branch account.Branch `json:"branch" gencodec:"required"`
}

type GetWorkResponse struct {
	errorCode  uint32      `json:"error_code" gencodec:"required"`
	headerHash common.Hash `json:"header_hash" gencodec:"required"`
	height     uint64      `json:"height" gencodec:"required"`
	difficulty *big.Int    `json:"difficulty" gencodec:"required"`
}

type SubmitWorkRequest struct {
	branch     account.Branch `json:"branch" gencodec:"required"`
	headerHash common.Hash    `json:"header_hash" gencodec:"required"`
	nonce      uint64         `json:"nonce" gencodec:"required"`
	mixHash    common.Hash    `json:"mix_hash" gencodec:"required"`
}

type SubmitWorkResponse struct {
	errorCode uint32 `json:"error_code" gencodec:"required"`
	success   bool   `json:"success" gencodec:"required"`
}
