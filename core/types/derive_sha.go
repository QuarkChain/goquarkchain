// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
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

	if val.Len() == 0 {
		return common.Hash{}
	}

	hashList := make([]common.Hash, val.Len())
	for i := 0; i < val.Len(); i++ {
		bytes, _ := serialize.SerializeToBytes(val.Index(i).Interface())
		hashList[i] = crypto.Keccak256Hash(bytes)
	}

	for len(hashList) != 1 {
		tempList := make([]common.Hash, 0)
		length := len(hashList)
		for i := 0; i < length-1; {
			tempList = append(tempList,
				crypto.Keccak256Hash(append(hashList[i].Bytes(), hashList[i+1].Bytes()...)))
			i = i + 2
		}
		if length%2 == 1 {
			tempList = append(tempList,
				crypto.Keccak256Hash(append(hashList[length-1].Bytes(), hashList[length-1].Bytes()...)))
		}
		hashList = tempList
	}
	return hashList[0]
}
