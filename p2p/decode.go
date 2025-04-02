package p2p

import (
	"encoding/binary"
	"errors"
	"unsafe"

	"github.com/QuarkChain/goquarkchain/serialize"
)

const (
	// OPLength Op length
	OPLength = 1
	// RPCIDLength rpc length
	MetadataLength = 4
	// RPCIDLength rpc length
	RPCIDLength = 8
	// PreP2PLength preP2PLength
	PreP2PLength = MetadataLength + OPLength + RPCIDLength
)

// P2PeerInfo peerInfo use uint123
type P2PeerInfo struct {
	IP   *serialize.Uint128
	Port uint16
}

// QKCMsg qkc msg struct
type QKCMsg struct {
	MetaData Metadata
	Op       P2PCommandOp
	RpcID    uint64
	Data     []byte
}

type Metadata struct {
	Branch uint32
}

func (m Metadata) Size() int {
	return int(unsafe.Sizeof(m))
}

// DecodeQKCMsg decode byte to qkcMsg
func DecodeQKCMsg(body []byte) (QKCMsg, error) {
	if len(body) < PreP2PLength {
		return QKCMsg{}, errors.New("decode qkc msg err body is short")
	}

	var msg QKCMsg
	msg.MetaData.Branch = binary.BigEndian.Uint32(body)
	msg.Op = P2PCommandOp(body[MetadataLength])
	msg.RpcID = binary.BigEndian.Uint64(body[MetadataLength+OPLength : PreP2PLength])
	msg.Data = make([]byte, len(body)-PreP2PLength)
	copy(msg.Data[:], body[PreP2PLength:])
	return msg, nil
}

// Encrypt encrypt Data to byte array
func Encrypt(metadata Metadata, op P2PCommandOp, ipcID uint64, data []byte) ([]byte, error) {
	encryptBytes := make([]byte, PreP2PLength+len(data))
	binary.BigEndian.PutUint32(encryptBytes, metadata.Branch)
	encryptBytes[MetadataLength] = byte(op)
	binary.BigEndian.PutUint64(encryptBytes[MetadataLength+OPLength:], ipcID)
	copy(encryptBytes[PreP2PLength:], data)
	return encryptBytes, nil
}
