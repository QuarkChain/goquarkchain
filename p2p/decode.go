package p2p

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

type HelloCmd struct {
	Version         uint32
	NetWorkID       uint32
	PeerID          common.Hash
	PeerIP          *serialize.Uint128
	PeerPort        uint16
	ChainMaskList   *Uint32_4
	RootBlockHeader types.RootBlockHeader
}
type Uint32_4 []uint32

func (Uint32_4) GetLenByteSize() int {
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

func DecodeQKCMsg(msg []byte) (qkcMsg, error) {
	var qkcmsg qkcMsg

	metaBytes := msg[:4]
	rawBytes := msg[4:]

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
	fmt.Println(hex.EncodeToString(qkcmsg.data))
	copy(qkcmsg.data[:], rawBytes[9:])
	fmt.Println(hex.EncodeToString(rawBytes[9:]))
	fmt.Println(hex.EncodeToString(qkcmsg.data))
	return qkcmsg, nil
}
