package p2p

import (
	"bytes"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"time"
)

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
)
func makeMsg(op byte,rpcID uint64,msg interface{})(Msg,error){
	qkcBody,err:=Encrypt(metadata{},op,rpcID,msg)
	if err!=nil{
		return Msg{},err
	}
	return Msg{Code: 16, Size: uint32(len(qkcBody)), Payload: bytes.NewReader(qkcBody)},nil
}
type HelloCmd struct {
	Version         uint32
	NetWorkID       uint32
	PeerID          common.Hash
	PeerIP          *serialize.Uint128
	PeerPort        uint16
	ChainMaskList   *Uint32Four
	RootBlockHeader types.RootBlockHeader
}
func (Self HelloCmd)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(HELLO,rpcID,Self)
}

type NewMinorBlockHeaderList struct {
	RootBlockHeader types.RootBlockHeader
	MinorBlockHeaderList *MinorBlockHeaderFour
}
func (Self NewMinorBlockHeaderList)SendMsg(rpcID uint64) (Msg,error){
	return makeMsg(NewMinorBlockHeaderListMsg,rpcID,Self)
}

type NewTransactionList struct {
	TransactionList *TransactionFour
}
func (Self NewTransactionList)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(NewTransactionListMsg,rpcID,Self)
}

type GetPeerListRequest struct {
	MaxPeers uint32
}
func (Self GetPeerListRequest)SendMsg(rpcID uint64)(Msg,error)  {
	return makeMsg(GetPeerListRequestMsg,rpcID,Self)
}

type GetPeerListResponse struct {
	PeerInfoList *PeerInfoFour
}
func (Self GetPeerListResponse)SendMsg(rpcID uint64)(Msg,error)  {
	return makeMsg(GetPeerListResponseMsg,rpcID,Self)
}

type GetRootBlockHeaderListRequest struct {
	BlockHash *serialize.Uint256
	Limit uint32
	Direction uint8
}
func (Self GetRootBlockHeaderListRequest)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetRootBlockHeaderListRequestMsg,rpcID,Self)
}

type GetRootBlockHeaderListResponse struct {
	RootTip types.RootBlockHeader
	BlockHeaderList *RootBlockHeaderFour
}
func (Self GetRootBlockHeaderListResponse)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetRootBlockHeaderListResponseMsg,rpcID,Self)
}

type GetRootBlockListRequest struct {
	RootBlockHashList *Hash256Four
}
func(Self GetRootBlockListRequest)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetRootBlockListRequestMsg,rpcID,Self)
}


type GetRootBlockListResponse struct {
	RootBlockList *RootBlockFour
}
func (Self GetRootBlockListResponse)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetRootBlockListResponseMsg,rpcID,Self)
}

type GetMinorBlockListRequest struct {
	MinorBlockHashList *Hash256Four
}
func(Self GetMinorBlockListRequest)SendMsg(rpcID uint64)(Msg,error){
	return  makeMsg(GetMinorBlockListRequestMsg,rpcID,Self)
}

type GetMinorBlockListResponse struct {
	MinorBlockList *MinorBlockFour
}
func (Self GetMinorBlockListResponse)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetMinorBlockListResponseMsg,rpcID,Self)
}

type GetMinorBlockHeaderListRequest struct {
	BlockHash *serialize.Uint256
	Branch account.Branch
	Limit uint32
	Direction uint8
}
func(Self GetMinorBlockHeaderListRequest)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetMinorBlockHeaderListRequestMsg,rpcID,Self)
}


type GetMinorBlockHeaderListResponse struct {
	RootTip types.RootBlockHeader
	ShardTip types.MinorBlockHeader
	BlockHeaderList *MinorBlockHeaderFour
}
func(Self GetMinorBlockHeaderListResponse)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(GetMinorBlockHeaderListResponseMsg,rpcID,Self)
}

type NewBlockMinor struct {
	Block *types.MinorBlock
}
func(Self NewBlockMinor)SendMsg(rpcID uint64)(Msg,error){
	return makeMsg(NewBlockMinorMsg,rpcID,Self)
}

func TestMsgSend(peer *Peer){
	fmt.Println("开始进入for循环")
	for true{
		time.Sleep(10*time.Second)
		fmt.Println("开始干活")

		//needSendMsg:=HelloCmd{Version:66,NetWorkID:27}.SendMsg()

		//needSendMsg:=SendNew_Minor_Block_Header_List{
		//	Root_Block_Header:types.RootBlockHeader{
		//		Difficulty:big.NewInt(12345),
		//	},
		//}.SendMsg()

		//needSendMsg:=Send_New_Transaction_List{}.SendMsg()

		//needSendMsg:=Send_Get_Peer_List_Request{
		//	Max_Peers:20,
		//}.SendMsg()

		//one:=P2P_PeerInfo{
		//	Ip:&serialize.Uint128{big.NewInt(100)},
		//	Port:2222,
		//}
		//ans:=[]P2P_PeerInfo{}
		//ans=append(ans,one)
		//aa:=PeerInfo_4(ans)
		//needSendMsg:=Send_Get_Peer_List_Response{
		//		Peer_Info_List:&aa,
		//}.SendMsg()

		//needSendMsg:=Send_GetRootBlockHeaderListRequest{
		//	BlockHash:&serialize.Uint256{big.NewInt(1)},
		//	Limit:3,
		//	Direction:0,
		//}.SendMsg()

		//needSendMsg:=Send_GetRootBlockHeaderListResponse{
		//	RootTip:types.RootBlockHeader{
		//		Difficulty:big.NewInt(100),
		//	},
		//}.SendMsg()


		//needSendMsg:=Send_GetRootBlockListRequest{
		//}.SendMsg()

		//needSendMsg:=Send_GetRootBlockListResponse{
		//}.SendMsg()

		//needSendMsg:=SendGetMinorBlockListRequest{}.SendMsg()

		//needSendMsg:=GetMinorBlockListResponse{}.SendMsg()

		//needSendMsg:=GetMinorBlockHeaderListRequest{}.SendMsg()

		//needSendMsg:=GetMinorBlockHeaderListResponse{}.SendMsg()

		//needSendMsg,err:= NewBlockMinor{}.SendMsg(0)
		//if err!=nil{
		//	panic(err)
		//}
		//err = peer.rw.WriteMsg(needSendMsg)
		//if err!=nil{
		//	fmt.Println("send err")
		//	panic(err)
		//}
	}
}
