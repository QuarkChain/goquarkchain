// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"reflect"
)

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
		hashList[i] = sha3_256(bytes)
	}

	for len(hashList) != 1 {
		tempList := make([]common.Hash, 0)
		length := len(hashList)
		for i := 0; i < length-1; {
			tempList = append(tempList,
				sha3_256(append(hashList[i].Bytes(), hashList[i+1].Bytes()...)))
			i = i + 2
		}
		if length%2 == 1 {
			tempList = append(tempList,
				sha3_256(append(hashList[length-1].Bytes(), hashList[length-1].Bytes()...)))
		}
		hashList = tempList
	}
	return hashList[0]
}

func sha3_256(bytes []byte) (hash common.Hash) {
	hw := sha3.NewKeccak256()
	hw.Write(bytes)
	hw.Sum(hash[:0])
	return hash
}
