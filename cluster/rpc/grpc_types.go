package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// RPCs to initialize a cluster

type Ping struct {
	Id            []byte             `json:"id" bytesizeofslicelen:"4"`
	ChainMaskList []*types.ChainMask `json:"chain_mask_list" bytesizeofslicelen:"4"`
}

type Pong struct {
	Id            []byte             `json:"id" gencodec:"required" bytesizeofslicelen:"4"`
	ChainMaskList []*types.ChainMask `json:"chain_mask_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type SlaveInfo struct {
	Id            string             `json:"id" gencodec:"required"`
	Host          string             `json:"host" gencodec:"required"`
	Port          uint16             `json:"port" gencodec:"required"`
	ChainMaskList []*types.ChainMask `json:"chain_mask_list" gencodec:"required" bytesizeofslicelen:"4"`
}

// ShardStatus shard status for api
type ShardStatus struct {
	Branch             account.Branch
	Height             uint64
	Difficulty         *big.Int
	CoinbaseAddress    account.Address
	Timestamp          uint64
	TxCount60s         uint32
	PendingTxCount     uint32
	TotalTxCount       uint32
	BlockCount60s      uint32
	StaleBlockCount60s uint32
	LastBlockTime      uint64
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

type MasterInfo struct {
	// Initialize ShardState if not None
	RootTip *types.RootBlock `json:"root_tip" ser:"nil"`
	Ip      string           `json:"ip" gencodec:"required"`
	Port    uint16           `json:"port" gencodec:"required"`
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

// Generate transactions for loadtesting
type GenTxRequest struct {
	NumTxPerShard uint32             `json:"num_tx_per_shard" gencodec:"required"`
	XShardPercent uint32             `json:"x_shard_percent" gencodec:"required"`
	Tx            *types.Transaction `json:"tx" gencodec:"required"`
}

// RPCs to lookup data from shards (master -> slaves)
type GetMinorBlockRequest struct {
	Branch         uint32      `json:"branch" gencodec:"required"`
	MinorBlockHash common.Hash `json:"minor_block_hash" gencodec:"required"`
	Height         *uint64     `json:"height" gencodec:"required"`
	NeedExtraInfo  bool        `json:"need_extra_info" gencodec:"required"`
}

type PoSWInfo struct {
	EffectiveDifficulty *big.Int
	PoswMineableBlocks  uint64
	PoswMinedBlocks     uint64
}

func (info *PoSWInfo) IsNil() bool {
	return (info.EffectiveDifficulty == nil || new(big.Int).Cmp(info.EffectiveDifficulty) == 0) &&
		info.PoswMineableBlocks == 0 && info.PoswMinedBlocks == 0
}

type GetMinorBlockHeaderListWithSkipRequest struct {
	p2p.GetMinorBlockHeaderListWithSkipRequest
	PeerID string `json:"peerid" gencodec:"required"`
}

type GetMinorBlockResponse struct {
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
	Extra      *PoSWInfo
}

type GetMinorBlockListRequest struct {
	Branch             uint32        `json:"branch" gencodec:"required"`
	PeerId             string        `json:"peer_id" gencodec:"required"`
	MinorBlockHashList []common.Hash `json:"minor_block_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetMinorBlockListResponse struct {
	MinorBlockList []*types.MinorBlock `json:"minor_block_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type BroadcastMinorBlock struct {
	Branch     uint32            `json:"branch" gencodec:"required"`
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
}

type BroadcastTransactions struct {
	Branch uint32               `json:"branch" gencodec:"required"`
	Txs    []*types.Transaction `json:"txs" gencodec:"required" bytesizeofslicelen:"4"`
}

type MinorHeadRequest struct {
	Branch uint32 `json:"branch" gencodec:"required"`
	PeerID string `json:"peer_id" gencodec:"required"`
}

type GetMinorBlockHeaderListResponse struct {
	MinorBlockHeaderList []*types.MinorBlockHeader `json:"minor_block_header" gencodec:"required" bytesizeofslicelen:"4"`
}

type BroadcastNewTip struct {
	Branch               uint32                    `json:"branch" gencodec:"required"`
	RootBlockHeader      *types.RootBlockHeader    `json:"root_block_header" gencodec:"required"`
	MinorBlockHeaderList []*types.MinorBlockHeader `json:"minor_block_header_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetTransactionRequest struct {
	TxHash common.Hash `json:"tx_hash" gencodec:"required"`
	Branch uint32      `json:"branch" gencodec:"required"`
}

type GetTransactionResponse struct {
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32            `json:"index" gencodec:"required"`
}

type ExecuteTransactionRequest struct {
	Tx          *types.Transaction `json:"tx" gencodec:"required"`
	FromAddress *account.Address   `json:"from_address" gencodec:"required"`
	BlockHeight *uint64            `json:"block_height" ser:"nil"`
}

type ExecuteTransactionResponse struct {
	Result []byte `json:"result" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetTransactionReceiptRequest struct {
	TxHash common.Hash `json:"tx_hash" gencodec:"required"`
	Branch uint32      `json:"branch" gencodec:"required"`
}

type GetTransactionReceiptResponse struct {
	MinorBlock *types.MinorBlock `json:"minor_block" gencodec:"required"`
	Index      uint32            `json:"index" gencodec:"required"`
	Receipt    *types.Receipt    `json:"receipt" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetTransactionListByAddressRequest struct {
	Address         *account.Address `json:"address" gencodec:"required"`
	TransferTokenID *uint64          `json:"transfer_token_id" gencodec:"required"`
	Start           []byte           `json:"start" gencodec:"required" bytesizeofslicelen:"4"`
	Limit           uint32           `json:"limit" gencodec:"required"`
}

type TransactionDetail struct {
	TxHash          common.Hash       `json:"tx_hash" gencodec:"required"`
	FromAddress     account.Address   `json:"from_address" gencodec:"required"`
	ToAddress       *account.Address  `json:"to_address" ser:"nil"`
	Value           serialize.Uint256 `json:"value" gencodec:"required"`
	BlockHeight     uint64            `json:"block_height" gencodec:"required"`
	Timestamp       uint64            `json:"timestamp" gencodec:"required"`
	Success         bool              `json:"success" gencodec:"required"`
	GasTokenID      uint64            `json:"gas_token_id" gencodec:"required"`
	TransferTokenID uint64            `json:"transfer_token_id" gencodec:"required"`
	IsFromRootChain bool              `json:"is_from_root_chain" gencodec:"required"`
}

type GetTxDetailResponse struct {
	TxList []*TransactionDetail `json:"tx_list" gencodec:"required" bytesizeofslicelen:"4"`
	Next   []byte               `json:"next" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetAllTxRequest struct {
	Branch account.Branch `json:"address" gencodec:"required"`
	Start  []byte         `json:"start" gencodec:"required" bytesizeofslicelen:"4"`
	Limit  uint32         `json:"limit" gencodec:"required"`
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

type HeadersInfo struct {
	Branch     uint32                    `json:"branch" gencodec:"required"`
	HeaderList []*types.MinorBlockHeader `json:"header_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetUnconfirmedHeadersResponse struct {
	HeadersInfoList []*HeadersInfo `json:"headers_info_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetAccountDataRequest struct {
	Address     *account.Address `json:"address" gencodec:"required"`
	BlockHeight *uint64          `json:"block_height" ser:"nil"`
}

type AccountBranchData struct {
	Branch           uint32               `json:"branch" gencodec:"required"`
	TransactionCount uint64               `json:"transaction_count" gencodec:"required"`
	Balance          *types.TokenBalances `json:"token_balances" gencodec:"required" bytesizeofslicelen:"4"`
	IsContract       bool                 `json:"is_contract" gencodec:"required"`
}

type GetAccountDataResponse struct {
	AccountBranchDataList []*AccountBranchData `json:"account_branch_data_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddTransactionRequest struct {
	Tx *types.Transaction `json:"tx" gencodec:"required"`
}

type HashList struct {
	Hashes []common.Hash `json:"hash_list" gencodec:"required" bytesizeofslicelen:"4"`
}

// slave -> master
/*
	Notify master about a successfully added minro block.
	Piggyback the ShardStatus in the same request.
*/
type AddMinorBlockHeaderRequest struct {
	MinorBlockHeader  *types.MinorBlockHeader `json:"minor_block_header" gencodec:"required"`
	TxCount           uint32                  `json:"tx_count" gencodec:"required"`
	XShardTxCount     uint32                  `json:"x_shard_tx_count" gencodec:"required"`
	CoinbaseAmountMap *types.TokenBalances    `json:"coinbase_amount_map" gencodec:"required"`
	ShardStats        *ShardStatus            `json:"shard_stats" gencodec:"required"`
}

type AddMinorBlockHeaderResponse struct {
	ArtificialTxConfig *ArtificialTxConfig `json:"artificial_tx_config" gencodec:"required"`
}

type AddMinorBlockHeaderListRequest struct {
	MinorBlockHeaderList []*types.MinorBlockHeader `json:"minor_block_header_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type CrossShardTransactionList struct {
	TxList []*types.CrossShardTransactionDeposit `json:"tx_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddXshardTxListRequest struct {
	Branch         uint32                                `json:"branch" gencodec:"required"`
	MinorBlockHash common.Hash                           `json:"minor_block_hash" gencodec:"required"`
	TxList         []*types.CrossShardTransactionDeposit `json:"tx_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type BatchAddXshardTxListRequest struct {
	AddXshardTxListRequestList []*AddXshardTxListRequest `json:"add_xshard_tx_list_request_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddBlockListForSyncRequest struct {
	Branch             uint32        `json:"branch" gencodec:"required"`
	PeerId             string        `json:"peer_id" gencodec:"required"`
	MinorBlockHashList []common.Hash `json:"minor_block_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type AddBlockListForSyncResponse struct {
	ShardStatus *ShardStatus `json:"shard_status" gencodec:"required"`
}

type HandleNewTipRequest struct {
	PeerID               string                    `json:"peer_id" gencodec:"required"`
	RootBlockHeader      *types.RootBlockHeader    `json:"root_block_header" gencodec:"required"`
	MinorBlockHeaderList []*types.MinorBlockHeader `json:"minor_block_header_list" gencodec:"required" bytesizeofslicelen:"4"`
}

type GetLogResponse struct {
	Logs []*types.Log `json:"logs" gencodec:"required" bytesizeofslicelen:"4"`
}

type EstimateGasRequest struct {
	Tx          *types.Transaction `json:"tx" gencodec:"required"`
	FromAddress *account.Address   `json:"from_address" gencodec:"required"`
}

type EstimateGasResponse struct {
	Result uint32 `json:"result" gencodec:"required"`
}

type GetStorageRequest struct {
	Address     *account.Address `json:"address" gencodec:"required"`
	Key         common.Hash      `json:"key" gencodec:"required"`
	BlockHeight *uint64          `json:"block_height" ser:"nil"`
}

type GetStorageResponse struct {
	Result common.Hash `json:"result" gencodec:"required"`
}

type GetCodeRequest struct {
	Address     *account.Address `json:"address" gencodec:"required"`
	BlockHeight *uint64          `json:"block_height" ser:"nil"`
}

type GetCodeResponse struct {
	Result []byte `json:"result" gencodec:"required" bytesizeofslicelen:"4"`
}

type GasPriceRequest struct {
	Branch  uint32 `json:"branch" gencodec:"required"`
	TokenID uint64 `json:"tokenID" gencodec:"required"`
}

type GasPriceResponse struct {
	Result uint64 `json:"result" gencodec:"required"`
}

type GetWorkRequest struct {
	Branch       uint32           `json:"branch" gencodec:"required"`
	CoinbaseAddr *account.Address `json:"block_height" ser:"nil"`
}

type GetWorkResponse struct {
	HeaderHash common.Hash `json:"header_hash" gencodec:"required"`
	Height     uint64      `json:"height" gencodec:"required"`
	Difficulty *big.Int    `json:"difficulty" gencodec:"required"`
}

type SubmitWorkRequest struct {
	Branch     uint32      `json:"branch"      gencodec:"required"`
	HeaderHash common.Hash `json:"header_hash" gencodec:"required"`
	Nonce      uint64      `json:"nonce"       gencodec:"required"`
	MixHash    common.Hash `json:"mix_hash"    gencodec:"required"`
}

type SubmitWorkResponse struct {
	Success bool `json:"success" gencodec:"required"`
}

type PeerInfoForDisPlay struct {
	ID   []byte
	IP   uint32
	Port uint32
}

type GetRootChainStakesRequest struct {
	Address        account.Address `json:"address" gencodec:"required"`
	MinorBlockHash common.Hash     `json:"minor_block_hash" gencodec:"required"`
}

type GetRootChainStakesResponse struct {
	Stakes *big.Int           `json:"stakes" gencodec:"required"`
	Signer *account.Recipient `json:"signer" gencodec:"required"`
}

type BlockNumber int64

const (
	// PendingBlockNumber  = BlockNumber(-2)
	LatestBlockNumber = BlockNumber(-1)
	// EarliestBlockNumber = BlockNumber(0)
)

// UnmarshalJSON parses the given JSON fragment into a BlockNumber. It supports:
// - "latest", "earliest" or "pending" as string arguments
// - the block number
// Returned errors:
// - an invalid block number error when the given argument isn't a known strings
// - an out of range error when the given block number is either too little or too large
func (bn *BlockNumber) UnmarshalJSON(data []byte) error {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	switch strings.ToLower(input) {
	/*case "earliest":
	*bn = EarliestBlockNumber
	return nil*/
	case "latest":
		*bn = LatestBlockNumber
		return nil
		/*case "pending":
		*bn = PendingBlockNumber
		return nil*/
	}

	blckNum, err := hexutil.DecodeUint64(input)
	if err != nil {
		return err
	}
	if blckNum > math.MaxInt64 {
		return fmt.Errorf("Blocknumber too high")
	}

	*bn = BlockNumber(blckNum)
	return nil
}

func (bn BlockNumber) Int64() int64 {
	return (int64)(bn)
}

func (bn BlockNumber) Uint64() uint64 {
	return (uint64)(bn)
}

type FilterQuery struct {
	FullShardId uint32
	ethereum.FilterQuery
}

// UnmarshalJSON sets *args fields with given data.
func (args *FilterQuery) UnmarshalJSON(data []byte) error {
	type input struct {
		BlockHash *common.Hash  `json:"blockHash"`
		FromBlock *BlockNumber  `json:"fromBlock"`
		ToBlock   *BlockNumber  `json:"toBlock"`
		Addresses interface{}   `json:"address"`
		Topics    []interface{} `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.BlockHash != nil {
		if raw.FromBlock != nil || raw.ToBlock != nil {
			// BlockHash is mutually exclusive with FromBlock/ToBlock criteria
			return fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock, choose one or the other")
		}
		args.BlockHash = raw.BlockHash
	} else {
		if raw.FromBlock != nil {
			args.FromBlock = big.NewInt(raw.FromBlock.Int64())
		}

		if raw.ToBlock != nil {
			args.ToBlock = big.NewInt(raw.ToBlock.Int64())
		}
	}

	args.Addresses = []common.Address{}

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %v", i, err)
					}
					args.Addresses = append(args.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", err)
			}
			args.Addresses = []common.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		args.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				args.Topics[i] = []common.Hash{top}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						args.Topics[i] = append(args.Topics[i], parsed)
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}

func decodeAddress(s string) (common.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.AddressLength+4 && len(b) != common.AddressLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), common.AddressLength)
	}
	return common.BytesToAddress(b[:common.AddressLength]), err
}

func decodeTopic(s string) (common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.HashLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), common.HashLength)
	}
	return common.BytesToHash(b), err
}
