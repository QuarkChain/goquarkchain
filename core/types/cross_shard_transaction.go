package types

import (
	"fmt"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

type CrossShardTransactionDepositV0 struct {
	TxHash          common.Hash
	From            account.Address
	To              account.Address
	Value           *serialize.Uint256
	GasPrice        *serialize.Uint256
	GasTokenID      uint64
	TransferTokenID uint64
	IsFromRootChain bool
	GasRemained     *serialize.Uint256
	MessageData     []byte `bytesizeofslicelen:"4"`
	CreateContract  bool
}

type CrossShardTransactionDeposit struct {
	CrossShardTransactionDepositV0
	RefundRate uint8
}

type CrossShardTransactionDepositListV0 struct {
	TXList []*CrossShardTransactionDepositV0 `bytesizeofslicelen:"4"`
}

type crossShardTransactionDepositListV1 struct {
	TXList []*CrossShardTransactionDeposit `bytesizeofslicelen:"4"`
}

type CrossShardTransactionDepositList struct {
	TXList []*CrossShardTransactionDeposit `bytesizeofslicelen:"4"`
}

func NewCrossShardTransactionDepositList(txList []*CrossShardTransactionDeposit) *CrossShardTransactionDepositList {
	if txList == nil {
		txList = make([]*CrossShardTransactionDeposit, 0)
	}
	return &CrossShardTransactionDepositList{
		TXList: txList,
	}
}

func (c *CrossShardTransactionDepositList) Serialize(w *[]byte) error {
	list := crossShardTransactionDepositListV1{c.TXList}
	bytes, err := serialize.SerializeToBytes(list)
	if err != nil {
		return err
	}
	bytes[0] = 1
	*w = append(*w, bytes...)
	return nil
}

func (c *CrossShardTransactionDepositList) Deserialize(bb *serialize.ByteBuffer) error {
	size, err := bb.GetUInt32()
	if err != nil {
		return err
	}
	version := size >> 24
	size = size << 8
	size = size >> 8
	txList := make([]*CrossShardTransactionDeposit, size)
	switch version {
	case 0:
		for i := 0; i < int(size); i++ {
			cstx := CrossShardTransactionDepositV0{}
			if err := serialize.Deserialize(bb, &cstx); err != nil {
				return err
			}
			txList[i] = &CrossShardTransactionDeposit{cstx, 100}
		}
	case 1:
		for i := 0; i < int(size); i++ {
			cstx := CrossShardTransactionDeposit{}
			if err := serialize.Deserialize(bb, &cstx); err != nil {
				return err
			}
			txList[i] = &cstx
		}
	default:
		return fmt.Errorf("not support version %v", version)

	}
	c.TXList = txList
	return nil
}
