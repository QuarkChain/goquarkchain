package p2p

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"net"
	"testing"
	"time"
)

func IP4toInt(IPv4Address net.IP) int64 {
	IPv4Int := big.NewInt(0)
	IPv4Int.SetBytes(IPv4Address.To4())
	return IPv4Int.Int64()
}

func TestConn_String(t *testing.T) {
	ipv4Decimal := IP4toInt(net.ParseIP("172.17.0.2"))
	fmt.Println(ipv4Decimal)
}

func FakeEnv() config.ClusterConfig {
	return config.ClusterConfig{
		P2P: &config.P2PConfig{
			BootNodes: "enode://32d87c5cd4b31d81c5b010af42a2e413af253dc3a91bd3d53c6b2c45291c3de71633bf7793447a0d3ddde601f8d21668fca5b33324f14ebe7516eab0da8bab8f@192.168.79.130:38291",
			PrivKey:"3a28b5ba57c53603b0b07b56bba752f7784bf506fa95edc395f5cf6c7514fe94",
		},
	}
}

func TestP2PManager_Start(t *testing.T) {
	log.InitLog(5)
	env := FakeEnv()
	p2pManager, err := NewP2PManager(env)
	fmt.Println("err", err)
	p2pManager.Start()
	fmt.Println("end end")
	time.Sleep(1000 * time.Second)
}

func TestDialTask_String(t *testing.T) {
	ans := HelloCmd{}
	w := make([]byte, 0, 1)
	if err := serialize.Serialize(&w, ans); err != nil {
		fmt.Println("err")
	}
	fmt.Println("err", len(w))
}

func TestDecodeQKCMsg(t *testing.T) {
	aimString := "00000000000000000000000000000000000000001832d87c5cd4b31d81c5b010af42a2e413af253dc3a91bd3d53c6b2c45291c3de7000000000000000000000000ac1100029593000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005a8c59e1030f42400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	aimBytes, err := hex.DecodeString(aimString)
	fmt.Println("", err)
	qkcMsg, err := DecodeQKCMsg(aimBytes)
	fmt.Println("err", err)
	fmt.Println(qkcMsg.metaData)
}
func TestCapsByNameAndVersion_ENRKey(t *testing.T) {
	aimString := "00000000000000000000000000000000000000001832d87c5cd4b31d81c5b010af42a2e413af253dc3a91bd3d53c6b2c45291c3de7000000000000000000000000ac1100029593000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005a8c59e1030f42400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	aimBytes, err := hex.DecodeString(aimString)
	fmt.Println("len(aimBytes)", len(aimBytes))
	metaData := aimBytes[:4]
	rawData := aimBytes[4:]
	fmt.Println("raw_data", len(rawData))

	var metadata metadata
	bytesBuffer := serialize.NewByteBuffer(metaData)
	err = serialize.Deserialize(bytesBuffer, &metadata)
	fmt.Println("metaData", metadata, "err", err)

	op := rawData[0]
	fmt.Println("op", uint32(op))

	rpcID := binary.BigEndian.Uint64(rawData[1:9])
	fmt.Println("rpcId", rpcID)

	var cmd HelloCmd
	fmt.Println(common.Bytes2Hex(rawData[9:]), len(common.Bytes2Hex(rawData[9:])))
	err = serialize.DeserializeFromBytes(rawData[9:], &cmd)
	fmt.Println("helloCmd", "err", err, "cmd", cmd.RootBlockHeader.Difficulty)

}

func TestBaseServer_Run(t *testing.T) {
	rawBytes := "000000000000001832d87c5cd4b31d81c5b010af42a2e413af253dc3a91bd3d53c6b2c45291c3de7000000000000000000000000ac1100029593000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005a8c59e1030f42400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	rawData, err := hex.DecodeString(rawBytes)
	fmt.Println("err", err)
	var cmd HelloCmd
	fmt.Println(len(common.Bytes2Hex(rawData)), hex.EncodeToString(rawData))
	err = serialize.DeserializeFromBytes(rawData, &cmd)
	fmt.Println("Version", cmd.Version)
	fmt.Println("NetWorkID", cmd.NetWorkID)
	fmt.Println("PeerID", cmd.PeerID.String())
	fmt.Println("PeerIP", cmd.PeerIP)
	fmt.Println("PeerPort", cmd.PeerPort)
	fmt.Println("ChainMaskList", cmd.ChainMaskList)
	fmt.Println("RootBlockHeader.Version", cmd.RootBlockHeader.Version)
	fmt.Println("RootBlockHeader.Number", cmd.RootBlockHeader.Number)
	fmt.Println("RootBlockHeader.ParentHash", cmd.RootBlockHeader.ParentHash.String())
	fmt.Println("RootBlockHeader.MinorHeaderHash", cmd.RootBlockHeader.MinorHeaderHash.String())
	fmt.Println("RootBlockHeader.Coinbase", cmd.RootBlockHeader.Coinbase)
	fmt.Println("RootBlockHeader.CoinbaseAmount", cmd.RootBlockHeader.CoinbaseAmount)
	fmt.Println("RootBlockHeader.Time", cmd.RootBlockHeader.Time)
	fmt.Println("RootBlockHeader.Difficulty", cmd.RootBlockHeader.Difficulty)
	fmt.Println("RootBlockHeader.Nonce", cmd.RootBlockHeader.Nonce)
	fmt.Println("RootBlockHeader.Extra", cmd.RootBlockHeader.Extra)
	fmt.Println("RootBlockHeader.MixDigest", cmd.RootBlockHeader.MixDigest.String())
	fmt.Println("RootBlockHeader.Signature", cmd.RootBlockHeader.Signature)

}
