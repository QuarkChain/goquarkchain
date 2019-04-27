package rpc

import (
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

// RPCs to initialize a cluster

type Ping struct {
	Id            []byte            `json:"id" bytesizeofslicelen:"4"`
	ChainMaskList []types.ChainMask `json:"chain_mask_list" bytesizeofslicelen:"4"`
	// Initialize ShardState if not None
	RootTip *types.RootBlock `json:"root_tip" ser:"nil"`
}

type Pong struct {
	Id            []byte             `json:"id" gencodec:"required" bytesizeofslicelen:"4"`
	ChainMaskList []*types.ChainMask `json:"chain_mask_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type SlaveInfo struct {
	Id            []byte             `json:"id" gencodec:"required" bytesizeofslicelen:"4"`
	Host          []byte             `json:"host" gencodec:"required" bytesizeofslicelen:"4"`
	Port          uint16             `json:"port" gencodec:"required"`
	ChainMaskList []*types.ChainMask `json:"chain_mask_list" gencodec:"required" bytesizeofslicelen:"4"`
}

// Master instructs a slave to connect to other slaves
type ConnectToSlavesRequest struct {
	SlaveInfoList []*SlaveInfo `json:"slave_info_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type ConnectToSlavesResult struct {
	Result []byte `json:"result" gencodec:"required" bytesizeofslicelen:"4"`
}

// result_list must have the same size as salve_info_list in the request.
// Empty result means success otherwise it would a serialized error message.
type ConnectToSlavesResponse struct {
	ResultList []*ConnectToSlavesResult `json:"result_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type ArtificialTxConfig struct {
	TargetRootBlockTime  uint32 `json:"target_root_block_time" gencodec:"required"`
	TargetMinorBlockTime uint32 `json:"target_minor_block_time" gencodec:"required"`
}

// Send mining instructions to slaves
type MineRequest struct {
	ArtificialTxConfig *ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
	Mining             bool                `json:"mining" gencodec:"required"`
}

type MineResponse struct {
}

// Generate transactions for loadtesting
type GenTxRequest struct {
	NumTxPerShard uint32             `json:"num_tx_per_shard" gencodec:"required"`
	XShardPercent uint32             `json:"x_shard_percent" gencodec:"required"`
	Tx            *types.Transaction `json:"tx" gencodec:"required"`
}

type GenTxResponse struct {
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
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
}

type GetTransactionRequest struct {
	TxHash common.Hash    `json:"tx_hash" gencodec:"required"`
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionResponse struct {
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32            `json:"index" gencodec:"required"`
}

type ExecuteTransactionRequest struct {
	Tx          *types.Transaction `json:"tx" gencodec:"required"`
	FromAddress account.Address    `json:"from_address" gencodec:"required"`
	BlockHeight *uint64            `json:"block_height" ser:"nil"`
}

type ExecuteTransactionResponse struct {
	Result []byte `json:"result" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetTransactionReceiptRequest struct {
	TxHash common.Hash    `json:"tx_hash" gencodec:"required"`
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetTransactionReceiptResponse struct {
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32            `json:"index" gencodec:"required"`
	Receipt    *types.Receipt    `json:"receipt" gencodec:"required"`
}

type GetTransactionListByAddressRequest struct {
	Address account.Address `json:"address" gencodec:"required"`
	Start   []byte          `json:"start" gencodec:"required" bytesizeofslicelen:"4"`
	Limit   uint32          `json:"limit" gencodec:"required"`
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

type GetTransactionListByAddressResponse struct {
	TxList []*TransactionDetail `json:"tx_list" gencodec:"required" bytesizeofslicelen:"4"`
	Next   []byte               `json:"next" gencodec:"required" bytesizeofslicelen:"4"`
}

// RPCs to update blockchains
// master -> slave

// Add root block to each slave
type AddRootBlockRequest struct {
	RootBlock    *types.RootBlock `json:"root_block" gencodec:"required"`
	ExpectSwitch bool             `json:"expect_switch" gencodec:"required"`
}

type AddRootBlockResponse struct {
	Switched bool `json:"switched" gencodec:"required"`
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

type GetEcoInfoListResponse struct {
	EcoInfoList []*EcoInfo `json:"eco_info_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetNextBlockToMineRequest struct {
	Branch             account.Branch      `json:"branch" gencodec:"required"`
	Address            account.Address     `json:"address" gencodec:"required"`
	ArtificialTxConfig *ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type GetNextBlockToMineResponse struct {
	Block *types.MinorBlock `json:"block" gencodec:"required"`
}

// For adding blocks mined through JRPC
type AddMinorBlockRequest struct {
	MinorBlockData []byte `json:"minor_block_data" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddMinorBlockResponse struct {
}

type HeadersInfo struct {
	Branch     account.Branch            `json:"branch" gencodec:"required"`
	HeaderList []*types.MinorBlockHeader `json:"header_list" gencodec:"required" bytesizeofslicelen:"4"`
}

// To collect minor block headers to build a new root block
type GetUnconfirmedHeadersRequest struct {
}

type GetUnconfirmedHeadersResponse struct {
	HeadersInfoList []*HeadersInfo `json:"headers_info_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetAccountDataRequest struct {
	Address     account.Address `json:"address" gencodec:"required"`
	BlockHeight uint64          `json:"block_height" ser:"nil"`
}

type TokenBalancePair struct {
	TokenId uint64      `json:"token_id" gencodec:"required"`
	Balance common.Hash `json:"balance" gencodec:"required"`
}

type AccountBranchData struct {
	Branch           account.Branch     `json:"branch" gencodec:"required"`
	TransactionCount common.Hash        `json:"transaction_count" gencodec:"required"`
	TokenBalances    []TokenBalancePair `json:"token_balances" gencodec:"required" bytesizeofslicelen:"4"`
	IsContract       bool               `json:"is_contract" gencodec:"required"`
}

type GetAccountDataResponse struct {
	AccountBranchDataList []*AccountBranchData `json:"account_branch_data_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddTransactionRequest struct {
	Tx *types.Transaction `json:"tx" gencodec:"required"`
}

type AddTransactionResponse struct {
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

type SyncMinorBlockListRequest struct {
	MinorBlockHashList []common.Hash  `json:"minor_block_hash_list" gencodec:"required" bytesizeofslicelen:"4"`
	Branch             account.Branch `json:"branch" gencodec:"required"`
	ClusterPeerId      uint64         `json:"cluster_peer_id" gencodec:"required"`
}

type SyncMinorBlockListResponse struct {
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
	ArtificialTxConfig ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type AddXshardTxListRequest struct {
	Branch         account.Branch                       `json:"branch" gencodec:"required"`
	MinorBlockHash common.Hash                          `json:"minor_block_hash" gencodec:"required"`
	TxList         []types.CrossShardTransactionDeposit `json:"tx_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddXshardTxListResponse struct {
}

type BatchAddXshardTxListRequest struct {
	AddXshardTxListRequestList []AddMinorBlockHeaderRequest `json:"add_xshard_tx_list_request_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type BatchAddXshardTxListResponse struct {
}
type Topic struct {
	Data [32]byte `json:"topics" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetLogRequest struct {
	Branch     account.Branch    `json:"branch" gencodec:"required"`
	Addresses  []account.Address `json:"addresses" gencodec:"required" bytesizeofslicelen:"4"`
	Topics     []*Topic          `json:"topics" gencodec:"required" bytesizeofslicelen:"4"`
	StartBlock uint64            `json:"start_block" gencodec:"required"`
	EndBlock   uint64            `json:"end_block" gencodec:"required"`
}

type GetLogResponse struct {
	Logs []*types.Log `json:"logs" gencodec:"required" bytesizeofslicelen:"4"`
}

type EstimateGasRequest struct {
	Tx          *types.Transaction `json:"tx" gencodec:"required"`
	FromAddress account.Address    `json:"from_address" gencodec:"required"`
}

type EstimateGasResponse struct {
	Result uint32 `json:"result" gencodec:"required"`
}

type GetStorageRequest struct {
	Address     account.Address    `json:"address" gencodec:"required"`
	Key         *serialize.Uint256 `json:"key" gencodec:"required"`
	BlockHeight uint64             `json:"block_height" ser:"nil"`
}

type GetStorageResponse struct {
	Result *serialize.Uint256 `json:"result" gencodec:"required"`
}

type GetCodeRequest struct {
	Address     account.Address `json:"address" gencodec:"required"`
	BlockHeight uint64          `json:"block_height" ser:"nil"`
}

type GetCodeResponse struct {
	Result []byte `json:"result" gencodec:"required" bytesizeofslicelen:"4"`
}

type GasPriceRequest struct {
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GasPriceResponse struct {
	Result uint64 `json:"result" gencodec:"required"`
}

type GetWorkRequest struct {
	Branch account.Branch `json:"branch" gencodec:"required"`
}

type GetWorkResponse struct {
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
	Success bool `json:"success" gencodec:"required"`
}
type BlockHeight struct {
	Height uint64
	Str    string
}

// ShardStatus shard status for api
type ShardStatus struct {
	Branch             account.Branch
	Height             uint64
	Difficulty         *big.Int
	CoinBaseAddress    account.Address
	TimeStamp          uint64
	TxCount60s         uint32
	PendingTxCount     uint32
	TotalTxCount       uint32
	BlockCount60s      uint32
	StaleBlockCount60s uint32
	LastBlockTime      uint32
}
