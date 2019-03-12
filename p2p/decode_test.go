package p2p

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"math/rand"
	"reflect"
	"testing"
)

type CodeC struct {
	Data     interface{}
	RPCID    uint64
	MetaData metadata
	Op       P2PCommandOp
}

func GetTestCodeCTest() []CodeC {
	allTestCase := make([]CodeC, 0)
	for op := Hello; op < MaxOPNum; op++ {
		temp := CodeC{
			Data:  OPSerializerMap[op],
			RPCID: uint64(rand.Int()),
			MetaData: metadata{
				Branch: rand.Uint32(),
			},
			Op: op,
		}
		allTestCase = append(allTestCase, temp)
	}
	return allTestCase
}

func VerifySerializeData(t *testing.T, decodeMsg QKCMsg, v CodeC) error {
	//Attention: not check the correctness of serialize data ,it is depend on serialize module,only check it for mistakes
	switch decodeMsg.op {
	case Hello:
		cmd := new(HelloCmd)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case NewMinorBlockHeaderListMsg:
		cmd := new(NewMinorBlockHeaderList)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case NewTransactionListMsg:
		cmd := new(NewTransactionList)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetPeerListRequestMsg:
		cmd := new(GetPeerListRequest)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetPeerListResponseMsg:
		cmd := new(GetPeerListResponse)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetRootBlockHeaderListRequestMsg:
		cmd := new(GetRootBlockHeaderListRequest)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetRootBlockHeaderListResponseMsg:
		cmd := new(GetRootBlockHeaderListResponse)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetRootBlockListRequestMsg:
		cmd := new(GetRootBlockListRequest)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetRootBlockListResponseMsg:
		cmd := new(GetRootBlockListResponse)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetMinorBlockListRequestMsg:
		cmd := new(GetMinorBlockListRequest)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetMinorBlockListResponseMsg:
		cmd := new(GetMinorBlockListResponse)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetMinorBlockHeaderListRequestMsg:
		cmd := new(GetMinorBlockHeaderListRequest)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case GetMinorBlockHeaderListResponseMsg:
		cmd := new(GetMinorBlockHeaderListResponse)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	case NewBlockMinorMsg:
		cmd := new(NewBlockMinor)
		if err := serialize.DeserializeFromBytes(decodeMsg.data, &cmd); err != nil {
			t.Fatal("deserialize from Bytes err", err)
		}
	default:
		t.Fatal("unexcepted decodeMsg op")
	}
	return nil

}
func TestEncryptAndDecode(t *testing.T) {
	caseList := GetTestCodeCTest()
	for _, v := range caseList {
		dataEncrypt, err := Encrypt(v.MetaData, v.Op, v.RPCID, v.Data)
		if err != nil {
			t.Fatal("Encrypt err", err)
		}

		dataDecode, err := DecodeQKCMsg(dataEncrypt)
		if dataDecode.op != v.Op {
			t.Fatal("op is not correct")
		}
		if dataDecode.rpcID != v.RPCID {
			t.Fatal("rpcID is not correct")
		}
		if reflect.DeepEqual(dataDecode.metaData, v.MetaData) == false {
			t.Fatal("metaData is not correct")
		}
		if err = VerifySerializeData(t, dataDecode, v); err != nil {
			t.Fatal("verify serialize Data err", err)
		}
	}
}
