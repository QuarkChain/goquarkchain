package types

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type TransactionInput struct {
	Hash common.Hash
	Index uint8
}

type TransactionOutPut struct {
	Address common.Address
	Quarkash *serialize.Uint256
}
type TransactionNet struct {
	InList []*TransactionInput `bytesizeofslicelen:"1"`
	Code []byte `bytesizeofslicelen:"4"`
	OutList []*TransactionOutPut `bytesizeofslicelen:"1"`
	SignList []byte `bytesizeofslicelen:"1"`
}

func (t *TransactionNet)ToTransaction()(*Transaction,error)  {
	tx:=new(EvmTransaction)
	txBytes:=t.Code[1:]
	fmt.Println("len",len(txBytes),hex.EncodeToString(txBytes))
	err:=rlp.DecodeBytes(txBytes,tx)
	if err!=nil{
		panic(err)
	}
	if t.Code[0]!=byte(2){
		fmt.Println("t.Code",t.Code[0],byte(2))
		panic("sn")
		return nil,errors.New("not evmTx")
	}
	return &Transaction{
		TxType:EvmTx,
		EvmTx:tx,
	},nil
}