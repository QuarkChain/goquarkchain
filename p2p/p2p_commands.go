package p2p

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/account"
	qkcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"reflect"
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
	Ping
	Pong
	GetRootBlockHeaderListWithSkipRequestMsg
	GetRootBlockHeaderListWithSkipResponseMsg
	NewRootBlockMsg
	GetMinorBlockHeaderListWithSkipRequestMsg
	GetMinorBlockHeaderListWithSkipResponseMsg
	MaxOPNum
)

//OPSerializerMap Op and its struct
var OPSerializerMap = map[P2PCommandOp]interface{}{
	Hello:                                      HelloCmd{},
	NewTipMsg:                                  Tip{},
	NewTransactionListMsg:                      NewTransactionList{},
	GetPeerListRequestMsg:                      GetPeerListRequest{},
	GetPeerListResponseMsg:                     GetPeerListResponse{},
	GetRootBlockHeaderListRequestMsg:           GetRootBlockHeaderListRequest{},
	GetRootBlockHeaderListResponseMsg:          GetRootBlockHeaderListResponse{},
	GetRootBlockListRequestMsg:                 GetRootBlockListRequest{},
	GetRootBlockListResponseMsg:                GetRootBlockListResponse{},
	GetMinorBlockListRequestMsg:                GetMinorBlockListRequest{},
	GetMinorBlockListResponseMsg:               GetMinorBlockListResponse{},
	GetMinorBlockHeaderListRequestMsg:          GetMinorBlockHeaderListRequest{},
	GetMinorBlockHeaderListResponseMsg:         GetMinorBlockHeaderListResponse{},
	NewBlockMinorMsg:                           NewBlockMinor{},
	Ping:                                       PingPongCommand{},
	Pong:                                       PingPongCommand{},
	GetRootBlockHeaderListWithSkipRequestMsg:   GetRootBlockHeaderListWithSkipRequest{},
	GetRootBlockHeaderListWithSkipResponseMsg:  GetRootBlockHeaderListResponse{},
	NewRootBlockMsg:                            NewRootBlockCommand{},
	GetMinorBlockHeaderListWithSkipRequestMsg:  GetMinorBlockHeaderListWithSkipRequest{},
	GetMinorBlockHeaderListWithSkipResponseMsg: GetMinorBlockHeaderListResponse{},
}

func (p P2PCommandOp) String() string {
	if _, ok := OPSerializerMap[p]; !ok {
		strconv.Itoa(int(p))
	}
	return reflect.TypeOf(OPSerializerMap[p]).Name()
}

func MakeMsg(op P2PCommandOp, rpcID uint64, metadata Metadata, msg interface{}) (Msg, error) {
	qkcBody, err := Encrypt(metadata, op, rpcID, msg)
	if err != nil {
		return Msg{}, err
	}
	return Msg{Code: 0, Size: uint32(len(qkcBody)), Payload: bytes.NewReader(qkcBody)}, nil
}

//HelloCmd hello cmd struct
type HelloCmd struct {
	Version              uint32
	NetWorkID            uint32
	PeerID               common.Hash
	PeerIP               *serialize.Uint128
	PeerPort             uint16
	ChainMaskList        []uint32 `bytesizeofslicelen:"4"`
	RootBlockHeader      *types.RootBlockHeader
	GenesisRootBlockHash common.Hash
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

// with 32B message which is undefined at the moment
type PingPongCommand struct {
	Message common.Hash
}

type NewRootBlockCommand struct {
	Block *types.RootBlock
}

type GetRootBlockHeaderListWithSkipRequest struct {
	Type      uint8 // 0 block hash, 1 block height
	Data      common.Hash
	Limit     uint32
	Skip      uint32
	Direction uint8 // 0 to genesis, 1 to tip
}

func (c *GetRootBlockHeaderListWithSkipRequest) SetHeight(height uint32) {
	c.Type = qkcom.SkipHeight
	c.Data = common.BytesToHash(big.NewInt(int64(height)).Bytes())
}

func (c *GetRootBlockHeaderListWithSkipRequest) GetHeight() *uint32 {
	var height *uint32
	if c.Type == qkcom.SkipHeight {
		h := uint32(new(big.Int).SetBytes(c.Data[:]).Uint64())
		height = &h
	}
	return height
}

func (c *GetRootBlockHeaderListWithSkipRequest) GetHash() common.Hash {
	if c.Type == qkcom.SkipHash {
		return c.Data
	}
	return common.Hash{}
}

type MinorHeaderListWithSkip struct {
	GetMinorBlockHeaderListWithSkipRequest
	PeerID string `json:"peerid" gencodec:"required"`
}

func NewMinorSkip(hash common.Hash, height *uint64,
	limit, skip uint32, direction uint8, branch uint32, peerId string) *MinorHeaderListWithSkip {
	mSkip := MinorHeaderListWithSkip{}
	mSkip.PeerID = peerId
	mSkip.setCommon(hash, height, limit, skip, direction, branch)
	return &mSkip
}

type GetMinorBlockHeaderListWithSkipRequest struct {
	Type      uint8 // 0 block hash, 1 block height
	Data      common.Hash
	Limit     uint32
	Skip      uint32
	Direction uint8 // 0 to genesis, 1 to tip
	Branch    account.Branch
}

func (g *GetMinorBlockHeaderListWithSkipRequest) setCommon(hash common.Hash, height *uint64,
	limit, skip uint32, direction uint8, branch uint32) {
	g.Data = hash
	g.Limit = limit
	g.Skip = skip
	g.Direction = direction
	g.Branch = account.Branch{Value: branch}
	if height != nil {
		g.setHeight(*height)
	}
}

func (c *GetMinorBlockHeaderListWithSkipRequest) setHeight(height uint64) {
	c.Type = qkcom.SkipHeight
	c.Data = common.BytesToHash(big.NewInt(int64(height)).Bytes())
}

func (c *GetMinorBlockHeaderListWithSkipRequest) GetHeight() *uint64 {
	var height *uint64
	if c.Type == qkcom.SkipHeight {
		h := new(big.Int).SetBytes(c.Data[:]).Uint64()
		height = &h
	}
	return height
}

func (c *GetMinorBlockHeaderListWithSkipRequest) GetHash() common.Hash {
	if c.Type == qkcom.SkipHash {
		return c.Data
	}
	return common.Hash{}
}
