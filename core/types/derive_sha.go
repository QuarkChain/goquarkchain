// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"bytes"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"golang.org/x/crypto/sha3"
	"reflect"
)

type DerivableList interface {
	Len() int
	Bytes(i int) []byte
}

func DeriveSha(list DerivableList) common.Hash {
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.Bytes(i))
	}
	return trie.Hash()
}

var EmptyTrieHash = new(trie.Trie).Hash()

var EmptyHash = common.Hash{}

func CalculateMerkleRoot(list interface{}) (h common.Hash) {
	val := reflect.ValueOf(list)
	if val.Type().Kind() != reflect.Slice {
		panic("expect slice input for CalculateMerkleRoot")
	}
	hashList := make([]common.Hash, val.Len())
	if val.Len() == 0 {
		hashList = append(hashList, common.Hash{})
	} else {
		for i := 0; i < val.Len(); i++ {
			bytes, _ := serialize.SerializeToBytes(val.Index(i).Interface())
			hashList[i] = sha3_256(bytes)
		}
	}
	zBytes := common.Hash{}
	for len(hashList) != 1 {
		tempList := make([]common.Hash, 0, (len(hashList)+1)/2)
		length := len(hashList)
		if length%2 == 1 {
			hashList = append(hashList, zBytes)
		}
		for i := 0; i < length-1; i = i + 2 {
			tempList = append(tempList,
				sha3_256(append(hashList[i].Bytes(), hashList[i+1].Bytes()...)))

		}
		hashList = tempList
		zBytes = sha3_256(append(zBytes.Bytes(), zBytes.Bytes()...))
	}
	return sha3_256(append(hashList[0].Bytes(), qkcCommon.Uint64ToBytes(uint64(val.Len()))...))
}

func sha3_256(bytes []byte) (hash common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	hw.Write(bytes)
	hw.Sum(hash[:0])
	return hash
}
