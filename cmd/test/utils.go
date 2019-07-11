package test

import (
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
)

var (
	privKey  = "0x966a253dd39a1832306487c6218da1425e429fae01c1a40eb50965dff31a04ed"
	bootNode = ""
)

func defaultNodeConfig() *service.Config {
	serviceConfig := &service.DefaultConfig
	serviceConfig.Name = ""
	serviceConfig.Version = ""
	serviceConfig.IPCPath = "qkc.ipc"
	serviceConfig.SvrModule = "rpc."
	serviceConfig.SvrPort = config.GrpcPort
	serviceConfig.SvrHost = "127.0.0.1"
	return serviceConfig
}

func createAcc() (account.Account, error) {
	key := account.BytesToIdentityKey(common.FromHex(privKey))
	return account.NewAccountWithKey(key)
}

func createTx(acc account.Address) *types.Transaction {
	evmTx := types.NewEvmTransaction(0,
		acc.Recipient,
		big.NewInt(0),
		uint64(30000),
		big.NewInt(1),
		uint32(acc.FullShardKey),
		uint32(acc.FullShardKey),
		3,
		0,
		[]byte{})
	tx, _ := sign(evmTx, privKey)
	return &types.Transaction{
		EvmTx:  tx,
		TxType: types.EvmTx,
	}
}

func sign(evmTx *types.EvmTransaction, key string) (*types.EvmTransaction, error) {
	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(common.FromHex(key)))
	if err != nil {
		panic(err)
	}
	return types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), prvKey)
}
