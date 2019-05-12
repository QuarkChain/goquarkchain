package p2p

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"strconv"
)

type P2PCommandOp byte

// p2p command
const (
	Hello P2PCommandOp = iota
	NewTipMsg
	NewTransactionListMsg
	GetPeerListRequestMsg
	GetPeerListResponseMsg
	GetRootBlockHeaderListRequestMsg
	GetRootBlockHeaderListResponseMsg
	GetRootBlockListRequestMsg
	GetRootBlockListResponseMsg
	GetMinorBlockListRequestMsg
	GetMinorBlockListResponseMsg
	GetMinorBlockHeaderListRequestMsg
	GetMinorBlockHeaderListResponseMsg
	NewBlockMinorMsg
	MaxOPNum
)

//OPSerializerMap Op and its struct
var OPSerializerMap = map[P2PCommandOp]interface{}{
	Hello:                              HelloCmd{},
	NewTipMsg:                          Tip{},
	NewTransactionListMsg:              NewTransactionList{},
	GetPeerListRequestMsg:              GetPeerListRequest{},
	GetPeerListResponseMsg:             GetPeerListResponse{},
	GetRootBlockHeaderListRequestMsg:   GetRootBlockHeaderListRequest{},
	GetRootBlockHeaderListResponseMsg:  GetRootBlockHeaderListResponse{},
	GetRootBlockListRequestMsg:         GetRootBlockListRequest{},
	GetRootBlockListResponseMsg:        GetRootBlockListResponse{},
	GetMinorBlockListRequestMsg:        GetMinorBlockListRequest{},
	GetMinorBlockListResponseMsg:       GetMinorBlockListResponse{},
	GetMinorBlockHeaderListRequestMsg:  GetMinorBlockHeaderListRequest{},
	GetMinorBlockHeaderListResponseMsg: GetMinorBlockHeaderListResponse{},
	NewBlockMinorMsg:                   NewBlockMinor{},
}

func (p P2PCommandOp) String() string {
	switch p {
	case Hello:
		return "Hello"
	case NewTipMsg:
		return "NewTipMsg"
	case NewTransactionListMsg:
		return "NewTransactionListMsg"
	case GetPeerListRequestMsg:
		return "GetPeerListRequestMsg"
	case GetPeerListResponseMsg:
		return "GetPeerListResponseMsg"
	case GetRootBlockHeaderListRequestMsg:
		return "GetRootBlockHeaderListRequestMsg"
	case GetRootBlockHeaderListResponseMsg:
		return "GetRootBlockHeaderListResponseMsg"
	case GetRootBlockListRequestMsg:
		return "GetRootBlockListRequestMsg"
	case GetRootBlockListResponseMsg:
		return "GetRootBlockListResponseMsg"
	case GetMinorBlockListRequestMsg:
		return "GetMinorBlockListRequestMsg"
	case GetMinorBlockListResponseMsg:
		return "GetMinorBlockListResponseMsg"
	case GetMinorBlockHeaderListRequestMsg:
		return "GetMinorBlockHeaderListRequestMsg"
	case GetMinorBlockHeaderListResponseMsg:
		return "GetMinorBlockHeaderListResponseMsg"
	case NewBlockMinorMsg:
		return "NewBlockMinorMsg"
	default:
		return strconv.Itoa(int(p))
	}
}

type msgHandleSt struct {
	resp       P2PCommandOp
	handleFunc func([]byte)
}

var (
	//OPNonRPCMap no return rpc Op
	OPNonRPCMap = map[P2PCommandOp]func(P2PCommandOp, []byte){
		Hello:                 handleError,
		NewTipMsg:             handleNewTip,
		NewTransactionListMsg: handleNewTransactionList,
	}

	//OpRPCMap have return rpc Op
	OpRPCMap = map[P2PCommandOp]msgHandleSt{
		GetPeerListRequestMsg: {
			resp:       GetPeerListResponseMsg,
			handleFunc: handleGetPeerListRequest,
		},
		GetRootBlockHeaderListRequestMsg: {
			resp:       GetRootBlockHeaderListResponseMsg,
			handleFunc: handleGetRootBlockHeaderListRequest,
		},
		GetRootBlockListRequestMsg: {
			resp:       GetRootBlockListResponseMsg,
			handleFunc: handleGetRootBlockListRequest,
		},
	}

	//PeerShardOpRPCMap used in virtual connection between local shard and remote shard
	PeerShardOpRPCMap = map[P2PCommandOp]msgHandleSt{
		GetMinorBlockListResponseMsg: {
			resp:       GetMinorBlockListResponseMsg,
			handleFunc: handleGetMinorBlockListRequest,
		},
		GetMinorBlockHeaderListRequestMsg: {
			resp:       GetMinorBlockHeaderListResponseMsg,
			handleFunc: handleGetMinorBlockHeaderListRequest,
		},
	}
)

func MakeMsg(op P2PCommandOp, rpcID uint64, metadata Metadata, msg interface{}) (Msg, error) {
	qkcBody, err := Encrypt(metadata, op, rpcID, msg)
	if err != nil {
		return Msg{}, err
	}
	return Msg{Code: 0, Size: uint32(len(qkcBody)), Payload: bytes.NewReader(qkcBody)}, nil
}

//HelloCmd hello cmd struct
type HelloCmd struct {
	Version         uint32
	NetWorkID       uint32
	PeerID          common.Hash
	PeerIP          *serialize.Uint128
	PeerPort        uint16
	ChainMaskList   []uint32 `bytesizeofslicelen:"4"`
	RootBlockHeader *types.RootBlockHeader
}

// Tip new minor block header list
type Tip struct {
	RootBlockHeader      *types.RootBlockHeader
	MinorBlockHeaderList []*types.MinorBlockHeader `bytesizeofslicelen:"4"`
}

//NewTransactionList new transaction list
type NewTransactionList struct {
	TransactionList []*types.Transaction `bytesizeofslicelen:"4"`
}

// GetPeerListRequest get peer list request
type GetPeerListRequest struct {
	MaxPeers uint32
}

//GetPeerListResponse get peer list response
type GetPeerListResponse struct {
	PeerInfoList []P2PeerInfo `bytesizeofslicelen:"4"`
}

// GetRootBlockHeaderListRequest get root block header list request
type GetRootBlockHeaderListRequest struct {
	BlockHash common.Hash
	Limit     uint32
	Direction uint8
}

//GetRootBlockHeaderListResponse get root block header list response
type GetRootBlockHeaderListResponse struct {
	RootTip         *types.RootBlockHeader
	BlockHeaderList []*types.RootBlockHeader `bytesizeofslicelen:"4"`
}

//GetRootBlockListRequest get root block list request
type GetRootBlockListRequest struct {
	RootBlockHashList []common.Hash `bytesizeofslicelen:"4"`
}

//GetRootBlockListResponse get root block list response
type GetRootBlockListResponse struct {
	RootBlockList []*types.RootBlock `bytesizeofslicelen:"4"`
}

// GetMinorBlockListRequest get minor block list request
type GetMinorBlockListRequest struct {
	MinorBlockHashList []common.Hash `bytesizeofslicelen:"4"`
}

//GetMinorBlockListResponse get minor block list response
type GetMinorBlockListResponse struct {
	MinorBlockList []*types.MinorBlock `bytesizeofslicelen:"4"`
}

//GetMinorBlockHeaderListRequest get minor block header list request
type GetMinorBlockHeaderListRequest struct {
	BlockHash common.Hash
	Branch    account.Branch
	Limit     uint32
	Direction uint8
}

//GetMinorBlockHeaderListResponse get minor block header list response
type GetMinorBlockHeaderListResponse struct {
	RootTip         *types.RootBlockHeader
	ShardTip        *types.MinorBlockHeader
	BlockHeaderList []*types.MinorBlockHeader `bytesizeofslicelen:"4"`
}

//NewBlockMinor new block minor
type NewBlockMinor struct {
	Block *types.MinorBlock
}

//OpNonRpcMap handle func

func handleError(op P2PCommandOp, cmd []byte) {
	log.Info(msgHandleLog, "handleError Op", op)
}

func handleNewTip(op P2PCommandOp, cmd []byte) {
	log.Info(msgHandleLog, "handleNewTip Op", op)
}

func handleNewTransactionList(op P2PCommandOp, cmd []byte) {
	log.Info(msgHandleLog, "handleNewTransactionList Op", op)
}

//OpRPCMap handle func
func handleGetPeerListRequest(cmd []byte) {
	log.Info(msgHandleLog, "handleGetPeerListRequest", "")
}

func handleGetRootBlockHeaderListRequest(cmd []byte) {
	log.Info(msgHandleLog, "handleGetRootBlockHeaderListRequest", "")
}

func handleGetRootBlockListRequest(cmd []byte) {
	log.Info(msgHandleLog, "handleGetRootBlockListRequest", "")
}

//PeerShard

func handleGetMinorBlockHeaderListRequest(cmd []byte) {
	log.Info(msgHandleLog, "handleGetMinorBlockHeaderListRequest", "")
}
func handleGetMinorBlockListRequest(cmd []byte) {
	log.Info(msgHandleLog, "handleGetMinorBlockListRequest", "")
}
