package p2p

import (
	"encoding/binary"
	"errors"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"unsafe"
)

const (
	// OPLength op length
	OPLength = 1
	// RPCIDLength rpc length
	RPCIDLength = 8
	// PerP2PLength preP2PLength
	PerP2PLength = OPLength + RPCIDLength
)

// uint32Four 4 uint32
type uint32Four []uint32

// GetLenByteSize func get 4 uint32
func (uint32Four) GetLenByteSize() int {
	return 4
}

// MinorBlockHeaderFour 4 MinorBlockHeader
type MinorBlockHeaderFour []types.MinorBlockHeader

// GetLenByteSize func get 4  MinorBlockHeader
func (MinorBlockHeaderFour) GetLenByteSize() int {
	return 4
}

// MinorBlockFour 4 MinorBlock
type MinorBlockFour []types.MinorBlock

// GetLenByteSize func get 4  MinorBlock
func (MinorBlockFour) GetLenByteSize() int {
	return 4
}

// TransactionFour 4 Transaction
type TransactionFour []types.Transaction

// GetLenByteSize func get 4  Transaction
func (TransactionFour) GetLenByteSize() int {
	return 4
}

// Hash256Four 4 Uint256
type Hash256Four []serialize.Uint256

// GetLenByteSize func get 4  Uint256
func (Hash256Four) GetLenByteSize() int {
	return 4
}

// RootBlockHeaderFour 4 RootBlockHeader
type RootBlockHeaderFour []types.RootBlockHeader

// GetLenByteSize func get 4  RootBlockHeader
func (RootBlockHeaderFour) GetLenByteSize() int {
	return 4
}

// RootBlockFour 4 RootBlock
type RootBlockFour []types.RootBlock

// GetLenByteSize func get 4  RootBlock
func (RootBlockFour) GetLenByteSize() int {
	return 4
}

// P2PeerInfo peerInfo use uint123
type P2PeerInfo struct {
	IP   *serialize.Uint128
	Port uint16
}

// PeerInfoFour 4 P2PeerInfo
type PeerInfoFour []P2PeerInfo

// GetLenByteSize func get 4  P2PeerInfo
func (PeerInfoFour) GetLenByteSize() int {
	return 4
}

// QKCMsg qkc msg struct
type QKCMsg struct {
	metaData metadata
	op       byte
	rpcID    uint64
	data     []byte
}

type metadata struct {
	Branch uint32
}

func (Self metadata) Size() int {
	return int(unsafe.Sizeof(Self))
}

// DecodeQKCMsg decode byte to qkcMsg
func DecodeQKCMsg(body []byte) (QKCMsg, error) {
	if len(body) < (metadata{}.Size() + PerP2PLength) {
		return QKCMsg{}, errors.New("decode qkc msg err body is short")
	}

	var msg QKCMsg
	metaBytes := body[:metadata{}.Size()]
	rawBytes := body[metadata{}.Size():]

	var metaData metadata
	err := serialize.DeserializeFromBytes(metaBytes, &metaData)
	if err != nil {
		return QKCMsg{}, nil
	}
	// metaData
	msg.metaData = metaData
	// op
	msg.op = rawBytes[0]
	// rpcID
	msg.rpcID = binary.BigEndian.Uint64(rawBytes[OPLength:PerP2PLength])

	dataSize := uint32(len(rawBytes) - PerP2PLength)
	// data
	msg.data = make([]byte, dataSize)
	copy(msg.data[:], rawBytes[PerP2PLength:])
	return msg, nil
}

// Encrypt encrypt data to byte array
func Encrypt(metadata metadata, op byte, ipcID uint64, cmd interface{}) ([]byte, error) {
	encryptBytes := make([]byte, 0)
	metadataBytes, err := serialize.SerializeToBytes(metadata)
	if err != nil {
		return []byte{}, err
	}
	// append metadata
	encryptBytes = append(encryptBytes, metadataBytes...)
	// append op
	encryptBytes = append(encryptBytes, op)
	rpcIDBytes := make([]byte, RPCIDLength)
	binary.BigEndian.PutUint64(rpcIDBytes, ipcID)
	// append rpcId
	encryptBytes = append(encryptBytes, rpcIDBytes...)
	cmdBytes, err := serialize.SerializeToBytes(cmd)
	if err != nil {
		return []byte{}, err
	}
	// append cmd
	encryptBytes = append(encryptBytes, cmdBytes...)
	return encryptBytes, nil
}
