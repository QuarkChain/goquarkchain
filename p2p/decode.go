package p2p

import (
	"encoding/binary"
	"errors"
	"github.com/QuarkChain/goquarkchain/serialize"
	"unsafe"
)

const (
	// OPLength Op length
	OPLength = 1
	// RPCIDLength rpc length
	RPCIDLength = 8
	// PreP2PLength preP2PLength
	PreP2PLength = OPLength + RPCIDLength
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
	if len(body) < (Metadata{}.Size() + PreP2PLength) {
		return QKCMsg{}, errors.New("decode qkc msg err body is short")
	}

	var msg QKCMsg
	metaBytes := body[:Metadata{}.Size()]
	rawBytes := body[Metadata{}.Size():]

	var metaData Metadata
	err := serialize.DeserializeFromBytes(metaBytes, &metaData)
	if err != nil {
		return QKCMsg{}, err
	}

	msg.MetaData = metaData
	msg.Op = P2PCommandOp(rawBytes[0])
	msg.RpcID = binary.BigEndian.Uint64(rawBytes[OPLength:PreP2PLength])
	dataSize := uint32(len(rawBytes) - PreP2PLength)
	msg.Data = make([]byte, dataSize)
	copy(msg.Data[:], rawBytes[PreP2PLength:])
	return msg, nil
}

// Encrypt encrypt Data to byte array
func Encrypt(metadata Metadata, op P2PCommandOp, ipcID uint64, cmd interface{}) ([]byte, error) {
	encryptBytes := make([]byte, 0)
	metadataBytes, err := serialize.SerializeToBytes(metadata)
	if err != nil {
		return nil, err
	}

	encryptBytes = append(encryptBytes, metadataBytes...)
	encryptBytes = append(encryptBytes, byte(op))

	rpcIDBytes := make([]byte, RPCIDLength)
	binary.BigEndian.PutUint64(rpcIDBytes, ipcID)

	encryptBytes = append(encryptBytes, rpcIDBytes...)
	cmdBytes, err := serialize.SerializeToBytes(cmd)
	if err != nil {
		return nil, err
	}

	encryptBytes = append(encryptBytes, cmdBytes...)
	return encryptBytes, nil
}
