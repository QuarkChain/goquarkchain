package main

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/qkcdb"
)

// Address include recipient and fullShardKey
type QkcAddress struct {
	Recipient    common.Address
	FullShardKey uint32
}

// ToHex return bytes included recipient and fullShardKey
func (Self QkcAddress) ToHex() string {
	address := Self.ToBytes()
	return hexutil.Encode(address)
}

func (Self QkcAddress) ToBytes() []byte {
	address := Self.Recipient.Bytes()
	shardKey := Uint32ToBytes(Self.FullShardKey)
	address = append(address, shardKey...)
	return address
}

func (Self QkcAddress) FullShardKeyToHex() string {
	return hexutil.Encode(Uint32ToBytes(Self.FullShardKey))
}

// Uint32ToBytes trans uint32 num to bytes
func Uint32ToBytes(n uint32) []byte {
	Bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(Bytes, n)
	return Bytes
}

type Client struct {
	client jsonrpc.RPCClient
}

// NewClient creates a client that uses the given RPC client.
func NewClient(host string) *Client {
	client := jsonrpc.NewClient(host)
	return &Client{client: client}
}

func main() {
	paths := make([]string, 2)
	paths[0] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S0/shard-1/db"
	paths[1] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S1/shard-65537/db"
	// paths[2] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S2/shard-131073/db"
	// paths[3] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S3/shard-196609/db"
	// paths[4] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S0/shard-262145/db"
	// paths[5] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S1/shard-327681/db"
	// paths[6] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S2/shard-393217/db"
	// paths[7] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S3/shard-458753/db"

	fmt.Println(10 ^ 10)
	client := NewClient("http://34.222.230.172:38391")
	for idx, path := range paths {
		m := GetBalances(idx, path)
		count := 0
		for acc, _ := range m {
			m[acc], _ = client.GetBalance(&QkcAddress{acc, uint32(idx*65536 + 1)})
			if m[acc] > 1000 {
				fmt.Println(hexutil.Encode(acc.Bytes()), m[acc])
				count++
			}
		}
		fmt.Println("count:", count)
	}
}

func (c *Client) GetBalance(qkcAddr *QkcAddress) (balance uint64, err error) {
	resp, err := c.client.Call("getBalances", []string{qkcAddr.ToHex()})
	if err != nil {
		return
	}
	if resp.Error != nil {
		fmt.Println("getBalances error: ", resp.Error.Error())
		return
	}
	balances := resp.Result.(map[string]interface{})["balances"]
	for _, m := range balances.([]interface{}) {
		bInfo := m.(map[string]interface{})
		token := (bInfo["tokenStr"]).(string)
		if strings.ToLower(token) == "qkc" {
			bi, err := hexutil.DecodeBig(bInfo["balance"].(string))
			if err != nil {
				fmt.Println(err)
			}
			return bi.Div(bi, big.NewInt(1000000000000000000)).Uint64(), nil
		}
	}
	return 0, nil
}

func GetBalances(idx int, path string) map[common.Address]uint64 {
	fmt.Println("--------", idx, "---------")
	accounts := make(map[common.Address]uint64)
	db, err := qkcdb.NewDatabase(path, false, false)
	if err != nil {
		panic(err)
	}
	it := db.NewIterator()
	it.Seek([]byte{})
	for it.Valid() {
		size := len(it.Key())
		if size == 43 {
			value, err := db.Get(it.Key())
			if err != nil {
				panic(err)
			}
			if len(value) == 20 {
				addr := common.BytesToAddress(value)
				accounts[addr] = 0
			}
		}
		it.Next()
	}
	return accounts
}
