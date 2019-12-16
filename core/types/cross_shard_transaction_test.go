package types

import (
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type crossShardTransactionDepositListV0ForTest struct {
	TXList []*CrossShardTransactionDepositV0 `bytesizeofslicelen:"4"`
}

func TestReadCrossShardTransactionDepositListV0(t *testing.T) {
	c0 := crossShardTransactionDepositListV0ForTest{
		TXList: make([]*CrossShardTransactionDepositV0, 0),
	}
	for index := uint64(0); index < 100; index++ {
		u256 := new(serialize.Uint256)
		u256.Value = new(big.Int).SetUint64(index)
		c0.TXList = append(c0.TXList, &CrossShardTransactionDepositV0{
			TxHash: common.BigToHash(new(big.Int).SetUint64(index)),
			From: account.Address{
				Recipient:    common.BigToAddress(new(big.Int).SetUint64(2)),
				FullShardKey: 2,
			},
			To: account.Address{
				Recipient:    common.BigToAddress(new(big.Int).SetUint64(3)),
				FullShardKey: 3,
			},
			Value:           u256,
			GasPrice:        u256,
			GasTokenID:      123,
			TransferTokenID: 456,
			IsFromRootChain: false,
			GasRemained:     u256,
			MessageData:     []byte{},
			CreateContract:  true,
		})
	}

	data, err := serialize.SerializeToBytes(c0)
	assert.NoError(t, err)

	d0 := new(crossShardTransactionDepositListV0ForTest)
	err = serialize.DeserializeFromBytes(data, d0)
	assert.NoError(t, err)
	for k, v := range c0.TXList {
		assert.Equal(t, v.TxHash, (*d0).TXList[k].TxHash)
		assert.Equal(t, v.From, (*d0).TXList[k].From)
		assert.Equal(t, v.To, (*d0).TXList[k].To)
		assert.Equal(t, v.Value.Value.Uint64(), (*d0).TXList[k].Value.Value.Uint64())
		assert.Equal(t, v.GasPrice.Value.Uint64(), (*d0).TXList[k].GasPrice.Value.Uint64())
		assert.Equal(t, v.GasTokenID, (*d0).TXList[k].GasTokenID)
		assert.Equal(t, v.TransferTokenID, (*d0).TXList[k].TransferTokenID)
		assert.Equal(t, v.IsFromRootChain, (*d0).TXList[k].IsFromRootChain)
		assert.Equal(t, v.GasRemained.Value.Uint64(), (*d0).TXList[k].GasRemained.Value.Uint64())
		assert.Equal(t, v.MessageData, (*d0).TXList[k].MessageData)
		assert.Equal(t, v.CreateContract, (*d0).TXList[k].CreateContract)
	}

	d1 := NewCrossShardTransactionDepositList(nil)
	err = serialize.DeserializeFromBytes(data, d1)
	assert.NoError(t, err)
	for k, v := range c0.TXList {
		assert.Equal(t, v.TxHash, (*d1).TXList[k].TxHash)
		assert.Equal(t, v.From, (*d1).TXList[k].From)
		assert.Equal(t, v.To, (*d1).TXList[k].To)
		assert.Equal(t, v.Value.Value.Uint64(), (*d1).TXList[k].Value.Value.Uint64())
		assert.Equal(t, v.GasPrice.Value.Uint64(), (*d1).TXList[k].GasPrice.Value.Uint64())
		assert.Equal(t, v.GasTokenID, (*d1).TXList[k].GasTokenID)
		assert.Equal(t, v.TransferTokenID, (*d1).TXList[k].TransferTokenID)
		assert.Equal(t, v.IsFromRootChain, (*d1).TXList[k].IsFromRootChain)
		assert.Equal(t, v.GasRemained.Value.Uint64(), (*d1).TXList[k].GasRemained.Value.Uint64())
		assert.Equal(t, v.MessageData, (*d1).TXList[k].MessageData)
		assert.Equal(t, v.CreateContract, (*d1).TXList[k].CreateContract)
		assert.Equal(t, uint8(100), (*d1).TXList[k].RefundRate)
	}
}

func TestReadCrossShardTransactionDepositList(t *testing.T) {
	c1 := NewCrossShardTransactionDepositList(nil)
	for index := uint64(0); index < 100; index++ {
		u256 := new(serialize.Uint256)
		u256.Value = new(big.Int).SetUint64(index)
		c1.TXList = append(c1.TXList, &CrossShardTransactionDeposit{
			CrossShardTransactionDepositV0: CrossShardTransactionDepositV0{
				TxHash: common.BigToHash(new(big.Int).SetUint64(index)),
				From: account.Address{
					Recipient:    common.BigToAddress(new(big.Int).SetUint64(2)),
					FullShardKey: 2,
				},
				To: account.Address{
					Recipient:    common.BigToAddress(new(big.Int).SetUint64(3)),
					FullShardKey: 3,
				},
				Value:           u256,
				GasPrice:        u256,
				GasTokenID:      123,
				TransferTokenID: 456,
				IsFromRootChain: false,
				GasRemained:     u256,
				MessageData:     []byte{},
				CreateContract:  true,
			},
			RefundRate: uint8(index),
		})
	}

	data, err := serialize.SerializeToBytes(c1)
	assert.NoError(t, err)

	d1 := NewCrossShardTransactionDepositList(nil)
	err = serialize.DeserializeFromBytes(data, d1)
	assert.NoError(t, err)
	for k, v := range c1.TXList {
		assert.Equal(t, v.TxHash, (*d1).TXList[k].TxHash)
		assert.Equal(t, v.From, (*d1).TXList[k].From)
		assert.Equal(t, v.To, (*d1).TXList[k].To)
		assert.Equal(t, v.Value.Value.Uint64(), (*d1).TXList[k].Value.Value.Uint64())
		assert.Equal(t, v.GasPrice.Value.Uint64(), (*d1).TXList[k].GasPrice.Value.Uint64())
		assert.Equal(t, v.GasTokenID, (*d1).TXList[k].GasTokenID)
		assert.Equal(t, v.TransferTokenID, (*d1).TXList[k].TransferTokenID)
		assert.Equal(t, v.IsFromRootChain, (*d1).TXList[k].IsFromRootChain)
		assert.Equal(t, v.GasRemained.Value.Uint64(), (*d1).TXList[k].GasRemained.Value.Uint64())
		assert.Equal(t, v.MessageData, (*d1).TXList[k].MessageData)
		assert.Equal(t, v.CreateContract, (*d1).TXList[k].CreateContract)
		assert.Equal(t, uint8(k), (*d1).TXList[k].RefundRate)
	}

}
