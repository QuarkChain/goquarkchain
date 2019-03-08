package p2p

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// p2p command
const (
	HELLO                              = 0
	NewMinorBlockHeaderListMsg         = 1
	NewTransactionListMsg              = 2
	GetPeerListRequestMsg              = 3
	GetPeerListResponseMsg             = 4
	GetRootBlockHeaderListRequestMsg   = 5
	GetRootBlockHeaderListResponseMsg  = 6
	GetRootBlockListRequestMsg         = 7
	GetRootBlockListResponseMsg        = 8
	GetMinorBlockListRequestMsg        = 9
	GetMinorBlockListResponseMsg       = 10
	GetMinorBlockHeaderListRequestMsg  = 11
	GetMinorBlockHeaderListResponseMsg = 12
	NewBlockMinorMsg                   = 13
	MaxOPNum                           = 14
)

//OPSerializerMap op and its struct
var OPSerializerMap = map[byte]interface{}{
	HELLO:                              HelloCmd{},
	NewMinorBlockHeaderListMsg:         NewMinorBlockHeaderList{},
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

type msgHandleSt struct {
	Res  byte
	Func func([]byte)
}

var (
	//OPNonRPCMap no return rpc op
	OPNonRPCMap = map[byte]func(byte, []byte){
		HELLO:                      handleError,
		NewMinorBlockHeaderListMsg: handleNewMinorBlockHeaderList,
		NewTransactionListMsg:      handleNewTransactionList,
	}

	//OpRPCMap have return rpc op
	OpRPCMap = map[byte]msgHandleSt{
		GetPeerListRequestMsg: {
			Res:  GetPeerListResponseMsg,
			Func: handleGetPeerListRequest,
		},
		GetRootBlockHeaderListRequestMsg: {
			Res:  GetRootBlockHeaderListResponseMsg,
			Func: handleGetRootBlockHeaderListRequest,
		},
		GetRootBlockListRequestMsg: {
			Res:  GetRootBlockListResponseMsg,
			Func: handleGetRootBlockListRequest,
		},
	}

	//PeerShardOpRPCMap used in virtual connection between local shard and remote shard
	PeerShardOpRPCMap = map[byte]msgHandleSt{
		GetMinorBlockListResponseMsg: {
			Res:  GetMinorBlockListResponseMsg,
			Func: handleGetMinorBlockListRequest,
		},
		GetMinorBlockHeaderListRequestMsg: {
			Res:  GetMinorBlockHeaderListResponseMsg,
			Func: handleGetMinorBlockHeaderListRequest,
		},
	}
)

func makeMsg(op byte, rpcID uint64, msg interface{}) (Msg, error) {
	qkcBody, err := Encrypt(metadata{}, op, rpcID, msg)
	if err != nil {
		return Msg{}, err
	}
	return Msg{Code: baseProtocolLength, Size: uint32(len(qkcBody)), Payload: bytes.NewReader(qkcBody)}, nil
}

//HelloCmd hello cmd struct
type HelloCmd struct {
	Version         uint32
	NetWorkID       uint32
	PeerID          common.Hash
	PeerIP          *serialize.Uint128
	PeerPort        uint16
	ChainMaskList   []uint32 `bytesize:"4"`
	RootBlockHeader types.RootBlockHeader
}

func (h HelloCmd) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(HELLO, rpcID, h)
}

// NewMinorBlockHeaderList new minor block header list
type NewMinorBlockHeaderList struct {
	RootBlockHeader      types.RootBlockHeader
	MinorBlockHeaderList []types.MinorBlockHeader `bytesize:"4"`
}

func (n NewMinorBlockHeaderList) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(NewMinorBlockHeaderListMsg, rpcID, n)
}

//NewTransactionList new transaction list
type NewTransactionList struct {
	TransactionList []types.Transaction `bytesize:"4"`
}

func (n NewTransactionList) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(NewTransactionListMsg, rpcID, n)
}

// GetPeerListRequest get peer list request
type GetPeerListRequest struct {
	MaxPeers uint32
}

func (g GetPeerListRequest) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetPeerListRequestMsg, rpcID, g)
}

//GetPeerListResponse get peer list response
type GetPeerListResponse struct {
	PeerInfoList []P2PeerInfo `bytesize:"4"`
}

func (g GetPeerListResponse) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetPeerListResponseMsg, rpcID, g)
}

// GetRootBlockHeaderListRequest get root block header list request
type GetRootBlockHeaderListRequest struct {
	BlockHash *serialize.Uint256
	Limit     uint32
	Direction uint8
}

func (g GetRootBlockHeaderListRequest) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetRootBlockHeaderListRequestMsg, rpcID, g)
}

//GetRootBlockHeaderListResponse get root block header list response
type GetRootBlockHeaderListResponse struct {
	RootTip         types.RootBlockHeader
	BlockHeaderList []types.RootBlockHeader `bytesize:"4"`
}

func (g GetRootBlockHeaderListResponse) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetRootBlockHeaderListResponseMsg, rpcID, g)
}

//GetRootBlockListRequest get root block list request
type GetRootBlockListRequest struct {
	RootBlockHashList []serialize.Uint256 `bytesize:"4"`
}

func (g GetRootBlockListRequest) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetRootBlockListRequestMsg, rpcID, g)
}

//GetRootBlockListResponse get root block list response
type GetRootBlockListResponse struct {
	RootBlockList []types.RootBlock `bytesize:"4"`
}

func (g GetRootBlockListResponse) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetRootBlockListResponseMsg, rpcID, g)
}

// GetMinorBlockListRequest get minor block list request
type GetMinorBlockListRequest struct {
	MinorBlockHashList []serialize.Uint256 `bytesize:"4"`
}

func (g GetMinorBlockListRequest) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetMinorBlockListRequestMsg, rpcID, g)
}

//GetMinorBlockListResponse get minor block list response
type GetMinorBlockListResponse struct {
	MinorBlockList []types.MinorBlock `bytesize:"4"`
}

func (g GetMinorBlockListResponse) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetMinorBlockListResponseMsg, rpcID, g)
}

//GetMinorBlockHeaderListRequest get minor block header list request
type GetMinorBlockHeaderListRequest struct {
	BlockHash *serialize.Uint256
	Branch    account.Branch
	Limit     uint32
	Direction uint8
}

func (g GetMinorBlockHeaderListRequest) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetMinorBlockHeaderListRequestMsg, rpcID, g)
}

//GetMinorBlockHeaderListResponse get minor block header list response
type GetMinorBlockHeaderListResponse struct {
	RootTip         types.RootBlockHeader
	ShardTip        types.MinorBlockHeader
	BlockHeaderList []types.MinorBlockHeader `bytesize:"4"`
}

func (g GetMinorBlockHeaderListResponse) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(GetMinorBlockHeaderListResponseMsg, rpcID, g)
}

//NewBlockMinor new block minor
type NewBlockMinor struct {
	Block *types.MinorBlock
}

func (n NewBlockMinor) makeSendMsg(rpcID uint64) (Msg, error) {
	return makeMsg(NewBlockMinorMsg, rpcID, n)
}

//OpNonRpcMap handle func

func handleError(op byte, cmd []byte) {
	log.Info(msgHandleLog, "handleError op", op)
}

func handleNewMinorBlockHeaderList(op byte, cmd []byte) {
	log.Info(msgHandleLog, "handleNewMinorBlockHeaderList op", op)
}

func handleNewTransactionList(op byte, cmd []byte) {
	log.Info(msgHandleLog, "handleNewTransactionList op", op)
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
