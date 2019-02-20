package p2p

import (
	"encoding/binary"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
)

type Uint32Four []uint32
func (Uint32Four) GetLenByteSize() int {
	return 4
}

type  MinorBlockHeaderFour []types.MinorBlockHeader
func (MinorBlockHeaderFour)GetLenByteSize()int{
	return 4
}

type  MinorBlockFour []types.MinorBlock
func (MinorBlockFour)GetLenByteSize()int{
	return 4
}

type TransactionFour []types.Transaction
func (TransactionFour)GetLenByteSize()int{
	return 4
}



type Hash256Four []serialize.Uint256
func (Hash256Four)GetLenByteSize()int{
	return 4
}

type RootBlockHeaderFour []types.RootBlockHeader
func (RootBlockHeaderFour)GetLenByteSize()int{
	return 4
}

type RootBlockFour []types.RootBlock
func (RootBlockFour)GetLenByteSize()int{
	return 4
}

type P2PeerInfo struct {
	Ip *serialize.Uint128
	Port uint16
}
type PeerInfoFour []P2PeerInfo
func(PeerInfoFour)GetLenByteSize()int{
	return 4
}

type qkcMsg struct {
	metaData metadata
	op       byte
	ipcID    uint64
	dataSize uint32
	data     []byte
}
type metadata struct {
	Branch uint32
}

func DecodeQKCMsg(body []byte) (qkcMsg, error) {
	var qkcmsg qkcMsg

	metaBytes := body[:4]
	rawBytes := body[4:]

	var metaData metadata
	err := serialize.DeserializeFromBytes(metaBytes, &metaData)
	if err != nil {
		return qkcMsg{}, nil
	}
	qkcmsg.metaData = metaData
	qkcmsg.op = rawBytes[0]
	qkcmsg.ipcID = binary.BigEndian.Uint64(rawBytes[1:9])
	qkcmsg.dataSize = uint32(len(rawBytes) - 9)
	qkcmsg.data = make([]byte, qkcmsg.dataSize)
	copy(qkcmsg.data[:], rawBytes[9:])
	return qkcmsg, nil
}

func Encrypt(metadata metadata,op byte ,ipcID uint64,cmd interface{})([]byte,error){
	aimbytes:=make([]byte,0)
	metadataBytes,err:=serialize.SerializeToBytes(metadata)
	if err!=nil{
		return []byte{},err
	}
	aimbytes=append(aimbytes,metadataBytes...)
	aimbytes=append(aimbytes,op)

	rpcIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(rpcIDBytes, ipcID)
	aimbytes=append(aimbytes,rpcIDBytes...)
	cmdBytes,err:=serialize.SerializeToBytes(cmd)
	if err!=nil{
		return []byte{},err
	}
	aimbytes=append(aimbytes,cmdBytes...)
	return aimbytes,nil
}
