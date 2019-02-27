// Modified from go-ethereum under GNU Lesser General Public License
package rpc

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// RPCs to initialize a cluster

type Ping struct {
	Id            serialize.LimitedSizeByteSlice4
	ChainMaskList ChainMasks
	// Initialize ShardState if not None
	RootTip types.RootBlock `json:"root_tip" ser:"nil"`
}

// TODO if ChainMask type is defined replace uint16 with ChainMask
type ChainMasks []uint16 // ChainMask

func (ChainMasks) GetLenByteSize() int {
	return 4
}

type SlaveInfos []SlaveInfo

func (SlaveInfos) GetLenByteSize() int {
	return 4
}

type Pong struct {
	Id serialize.LimitedSizeByteSlice4 `json:"id" gencodec:"required"`
	// TODO if ChainMask type is defined replace uint16 with ChainMask
	ChainMaskList ChainMasks `json:"chain_mask_list" gencodec:"required"`
}

type SlaveInfo struct {
	Id            serialize.LimitedSizeByteSlice4 `json:"id" gencodec:"required"`
	Host          serialize.LimitedSizeByteSlice4 `json:"host" gencodec:"required"`
	Port          uint16                          `json:"port" gencodec:"required"`
	ChainMaskList ChainMasks                      `json:"chain_mask_list" gencodec:"required"`
}

// Master instructs a slave to connect to other slaves
type ConnectToSlavesRequest struct {
	SlaveInfoList SlaveInfos `json:"slave_info_list" gencodec:"required"`
}

type LargeBytes []serialize.LimitedSizeByteSlice4

func (LargeBytes) GetLenByteSize() int {
	return 4
}

// result_list must have the same size as salve_info_list in the request.
// Empty result means success otherwise it would a serialized error message.
type ConnectToSlavesResponse struct {
	ResultList LargeBytes `json:"result_list" gencodec:"required"`
}

type ArtificialTxConfig struct {
	Targetrootblocktime  uint32 `json:"target_root_block_time" gencodec:"required"`
	TargetMinorBlockTime uint32 `json:"target_minor_block_time" gencodec:"required"`
}

// Send mining instructions to slaves
type MineRequest struct {
	ArtificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
	Mining             bool               `json:"mining" gencodec:"required"`
}

type MineResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

// Generate transactions for loadtesting
type GenTxRequest struct {
	NumTxPerShard uint32            `json:"num_tx_per_shard" gencodec:"required"`
	XShardPercent uint32            `json:"x_shard_percent" gencodec:"required"`
	Tx            types.Transaction `json:"tx" gencodec:"required"`
}

type GenTxResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

// Virtual connection management

/*
	Broadcast to the cluster and announce that a peer connection is created
	Assume always succeed.
*/
type CreateClusterPeerConnectionRequest struct {
	ClusterPeerId uint64 `json:"cluster_peer_id" gencodec:"required"`
}

type CreateClusterPeerConnectionResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

/*
	Broadcast to the cluster and announce that a peer connection is lost
    As a contract, the master will not send traffic after the command.
*/
type DestroyClusterPeerConnectionCommand struct {
	ClusterPeerId uint64 `json:"cluster_peer_id" gencodec:"required"`
}

// RPCs to lookup data from shards (master -> slaves)

type GetMinorBlockRequest struct {
	Branch         account.Branch `json:"branch" gencodec:"required"`
	MinorBlockHash common.Hash    `json:"minor_block_hash" gencodec:"required"`
	Height         uint64         `json:"height" gencodec:"required"`
}

type GetMinorBlockResponse struct {
	ErrorCode  uint32           `json:"error_code" gencodec:"required"`
	MinorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
}

type GetTransactionRequest struct {
	TxHash common.Hash    `json:"tx_hash" gencodec:"required"`
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionResponse struct {
	ErrorCode  uint32           `json:"error_code" gencodec:"required"`
	MinorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32           `json:"index" gencodec:"required"`
}

type ExecuteTransactionRequest struct {
	Tx          types.Transaction `json:"tx" gencodec:"required"`
	FromAddress account.Address   `json:"from_address" gencodec:"required"`
	BlockHeight uint64            `json:"block_height" ser:"nil"`
}

type ExecuteTransactionResponse struct {
	ErrorCode uint32                          `json:"error_code" gencodec:"required"`
	Result    serialize.LimitedSizeByteSlice4 `json:"result" gencodec:"required"`
}

type GetTransactionReceiptRequest struct {
	TxHash common.Hash    `json:"tx_hash" gencodec:"required"`
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionReceiptResponse struct {
	ErrorCode  uint32           `json:"error_code" gencodec:"required"`
	MinorBlock types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32           `json:"index" gencodec:"required"`
	Receipt    types.Receipt    `json:"receipt" gencodec:"required"`
}

type GetTransactionListByAddressRequest struct {
	Address account.Address                 `json:"address" gencodec:"required"`
	Start   serialize.LimitedSizeByteSlice4 `json:"start" gencodec:"required"`
	Limit   uint32                          `json:"limit" gencodec:"required"`
}

type TransactionDetail struct {
	TxHash          common.Hash     `json:"tx_hash" gencodec:"required"`
	FromAddress     account.Address `json:"from_address" gencodec:"required"`
	ToAddress       account.Address `json:"to_address" ser:"nil"`
	Value           common.Hash     `json:"value" gencodec:"required"`
	BlockHeight     uint64          `json:"block_height" gencodec:"required"`
	Timestamp       uint64          `json:"timestamp" gencodec:"required"`
	Success         bool            `json:"success" gencodec:"required"`
	GasTokenId      uint64          `json:"gas_token_id" gencodec:"required"`
	TransferTokenId uint64
}

type TransactionDetails []TransactionDetail

func (TransactionDetails) GetLenByteSize() int {
	return 4
}

type GetTransactionListByAddressResponse struct {
	ErrorCode uint32                          `json:"error_code" gencodec:"required"`
	TxList    TransactionDetails              `json:"tx_list" gencodec:"required"`
	Next      serialize.LimitedSizeByteSlice4 `json:"next" gencodec:"required"`
}

// RPCs to update blockchains
// master -> slave

// Add root block to each slave
type AddRootBlockRequest struct {
	RootBlock    types.RootBlock `json:"root_block" gencodec:"required"`
	ExpectSwitch bool            `json:"expect_switch" gencodec:"required"`
}

type AddRootBlockResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
	Switched  bool   `json:"switched" gencodec:"required"`
}

// Necessary information for master to decide the best block to mine
type EcoInfo struct {
	Branch                           account.Branch `json:"branch" gencodec:"required"`
	Height                           uint64         `json:"height" gencodec:"required"`
	CoinbaseAmount                   common.Hash    `json:"coinbase_amount" gencodec:"required"`
	Difficulty                       *big.Int       `json:"difficulty" gencodec:"required"`
	UnconfirmedHeadersCoinbaseAmount common.Hash    `json:"unconfirmed_headers_coinbase_amount" gencodec:"required"`
}

type GetEcoInfoListRequest struct {
}

type EcoInfos []EcoInfo

func (EcoInfos) GetLenByteSize() int {
	return 4
}

type GetEcoInfoListResponse struct {
	ErrorCode   uint32   `json:"error_code" gencodec:"required"`
	EcoInfoList EcoInfos `json:"eco_info_list" gencodec:"required"`
}

type GetNextBlockToMineRequest struct {
	Branch             account.Branch     `json:"branch" gencodec:"required"`
	Address            account.Address    `json:"address" gencodec:"required"`
	ArtificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type GetNextBlockToMineResponse struct {
	ErrorCode uint32           `json:"error_code" gencodec:"required"`
	Block     types.MinorBlock `json:"block" gencodec:"required"`
}

// For adding blocks mined through JRPC
type AddMinorBlockRequest struct {
	MinorBlockData serialize.LimitedSizeByteSlice4 `json:"minor_block_data" gencodec:"required"`
}

type AddMinorBlockResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

type MinorBlockHeaders []types.MinorBlockHeader

func (MinorBlockHeaders) GetLenByteSize() int {
	return 4
}

type HeadersInfo struct {
	Branch     account.Branch    `json:"branch" gencodec:"required"`
	HeaderList MinorBlockHeaders `json:"header_list" gencodec:"required"`
}

// To collect minor block headers to build a new root block
type GetUnconfirmedHeadersRequest struct {
}

type HeadersInfos []HeadersInfo

func (HeadersInfos) GetLenByteSize() int {
	return 4
}

type GetUnconfirmedHeadersResponse struct {
	ErrorCode       uint32       `json:"error_code" gencodec:"required"`
	HeadersInfoList HeadersInfos `json:"headers_info_list" gencodec:"required"`
}

type GetAccountDataRequest struct {
	Address     account.Address `json:"address" gencodec:"required"`
	BlockHeight uint64          `json:"block_height" ser:"nil"`
}

type TokenBalancePair struct {
	TokenId uint64      `json:"token_id" gencodec:"required"`
	Balance common.Hash `json:"balance" gencodec:"required"`
}

type TokenBalancePairs []TokenBalancePair

func (TokenBalancePairs) GetLenByteSize() int {
	return 4
}

type AccountBranchData struct {
	Branch           account.Branch    `json:"branch" gencodec:"required"`
	TransactionCount common.Hash       `json:"transaction_count" gencodec:"required"`
	TokenBalances    TokenBalancePairs `json:"token_balances" gencodec:"required"`
	IsContract       bool              `json:"is_contract" gencodec:"required"`
}

type AccountBranchDatas []AccountBranchData

func (AccountBranchDatas) GetLenByteSize() int {
	return 4
}

type GetAccountDataResponse struct {
	ErrorCode             uint32             `json:"error_code" gencodec:"required"`
	AccountBranchDataList AccountBranchDatas `json:"account_branch_data_list" gencodec:"required"`
}

type AddTransactionRequest struct {
	Tx types.Transaction `json:"tx" gencodec:"required"`
}

type AddTransactionResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

type ShardStats struct {
	Branch             account.Branch  `json:"branch" gencodec:"required"`
	Height             uint64          `json:"height" gencodec:"required"`
	Difficulty         *big.Int        `json:"difficulty" gencodec:"required"`
	CoinbaseAddress    account.Address `json:"coinbase_address" gencodec:"required"`
	Timestamp          uint64          `json:"timestamp" gencodec:"required"`
	TxCount60s         uint32          `json:"tx_count_60_s" gencodec:"required"`
	PendingTxCount     uint32          `json:"pending_tx_count" gencodec:"required"`
	TotalTxCount       uint32          `json:"total_tx_count" gencodec:"required"`
	BlockCount60s      uint32          `json:"block_count_60_s" gencodec:"required"`
	StaleBlockCount60s uint32          `json:"stale_block_count_60_s" gencodec:"required"`
	LastBlockTime      uint32          `json:"last_block_time" gencodec:"required"`
}

type HashBytes []common.Hash

func (HashBytes) GetLenByteSize() int {
	return 4
}

type SyncMinorBlockListRequest struct {
	MinorBlockHashList HashBytes      `json:"minor_block_hash_list" gencodec:"required"`
	Branch             account.Branch `json:"branch" gencodec:"required"`
	ClusterPeerId      uint64         `json:"cluster_peer_id" gencodec:"required"`
}

type SyncMinorBlockListResponse struct {
	ErrorCode  uint32     `json:"error_code" gencodec:"required"`
	ShardStats ShardStats `json:"shard_stats" ser:"nil"`
}

// slave -> master
/*
	Notify master about a successfully added minro block.
	Piggyback the ShardStats in the same request.
*/
type AddMinorBlockHeaderRequest struct {
	MinorBlockHeader types.MinorBlockHeader `json:"minor_block_header" gencodec:"required"`
	TxCount          uint32                 `json:"tx_count" gencodec:"required"`
	XShardTxCount    uint32                 `json:"x_shard_tx_count" gencodec:"required"`
	ShardStats       ShardStats             `json:"shard_stats" gencodec:"required"`
}

type AddMinorBlockHeaderResponse struct {
	ErrorCode          uint32             `json:"error_code" gencodec:"required"`
	ArtificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type AddXshardTxListRequest struct {
	Branch         account.Branch                  `json:"branch" gencodec:"required"`
	MinorBlockHash common.Hash                     `json:"minor_block_hash" gencodec:"required"`
	TxList         types.CrossShardTransactionList `json:"tx_list" gencodec:"required"`
}

type AddXshardTxListResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

type AddXshardTxListRequests []AddXshardTxListRequest

func (AddXshardTxListRequests) GetLenByteSize() int {
	return 4
}

type BatchAddXshardTxListRequest struct {
	AddXshardTxListRequestList AddXshardTxListRequests `json:"add_xshard_tx_list_request_list" gencodec:"required"`
}

type BatchAddXshardTxListResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
}

type AddressByte []account.Address

func (AddressByte) GetLenByteSize() int {
	return 4
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
	Branch     account.Branch `json:"branch" gencodec:"required"`
	Addresses  AddressByte    `json:"addresses" gencodec:"required"`
	Topics     Uint256Bytes   `json:"topics" gencodec:"required"`
	StartBlock uint64         `json:"start_block" gencodec:"required"`
	EndBlock   uint64         `json:"end_block" gencodec:"required"`
}

type LogByte []types.Log

func (LogByte) GetLenByteSize() int {
	return 4
}

type GetLogResponse struct {
	ErrorCode uint32  `json:"error_code" gencodec:"required"`
	Logs      LogByte `json:"logs" gencodec:"required"`
}

type EstimateGasRequest struct {
	Tx          types.Transaction `json:"tx" gencodec:"required"`
	FromAddress account.Address   `json:"from_address" gencodec:"required"`
}

type EstimateGasResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
	Result    uint32 `json:"result" gencodec:"required"`
}

type GetStorageRequest struct {
	Address     account.Address `json:"address" gencodec:"required"`
	Key         common.Hash     `json:"key" gencodec:"required"`
	BlockHeight uint64          `json:"block_height" ser:"nil"`
}

type GetStorageResponse struct {
	ErrorCode uint32            `json:"error_code" gencodec:"required"`
	Result    serialize.Uint256 `json:"result" gencodec:"required"`
}

type GetCodeRequest struct {
	Address     account.Address `json:"address" gencodec:"required"`
	BlockHeight uint64          `json:"block_height" ser:"nil"`
}

type GetCodeResponse struct {
	ErrorCode uint32                          `json:"error_code" gencodec:"required"`
	Result    serialize.LimitedSizeByteSlice4 `json:"result" gencodec:"required"`
}

type GasPriceRequest struct {
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GasPriceResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
	Result    uint64 `json:"result" gencodec:"required"`
}

type GetWorkRequest struct {
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetWorkResponse struct {
	ErrorCode  uint32      `json:"error_code" gencodec:"required"`
	HeaderHash common.Hash `json:"header_hash" gencodec:"required"`
	Height     uint64      `json:"height" gencodec:"required"`
	Difficulty *big.Int    `json:"difficulty" gencodec:"required"`
}

type SubmitWorkRequest struct {
	Branch     account.Branch `json:"branch" gencodec:"required"`
	HeaderHash common.Hash    `json:"header_hash" gencodec:"required"`
	Nonce      uint64         `json:"nonce" gencodec:"required"`
	MixHash    common.Hash    `json:"mix_hash" gencodec:"required"`
}

type SubmitWorkResponse struct {
	ErrorCode uint32 `json:"error_code" gencodec:"required"`
	Success   bool   `json:"success" gencodec:"required"`
}
