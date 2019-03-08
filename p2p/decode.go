package p2p

import (
	"encoding/binary"
	"errors"
	"github.com/QuarkChain/goquarkchain/serialize"
	"unsafe"
)

const (
	// OPLength op length
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
	metaData metadata
	op       byte
	rpcID    uint64
	data     []byte
}

type metadata struct {
	Branch uint32
}

func (m metadata) Size() int {
	return int(unsafe.Sizeof(m))
}

// DecodeQKCMsg decode byte to qkcMsg
func DecodeQKCMsg(body []byte) (QKCMsg, error) {
	if len(body) < (metadata{}.Size() + PreP2PLength) {
		return QKCMsg{}, errors.New("decode qkc msg err body is short")
	}

	var msg QKCMsg
	metaBytes := body[:metadata{}.Size()]
	rawBytes := body[metadata{}.Size():]

	var metaData metadata
	err := serialize.DeserializeFromBytes(metaBytes, &metaData)
	if err != nil {
		return QKCMsg{}, err
	}
	// metaData
	msg.metaData = metaData
	// op
	msg.op = rawBytes[0]
	// rpcID
	msg.rpcID = binary.BigEndian.Uint64(rawBytes[OPLength:PreP2PLength])

	dataSize := uint32(len(rawBytes) - PreP2PLength)
	// data
	msg.data = make([]byte, dataSize)
	copy(msg.data[:], rawBytes[PreP2PLength:])
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
