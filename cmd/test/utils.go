package test

import (
	"crypto/ecdsa"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"math/big"
)

var (
	privStrs = []string{
		"966a253dd39a1832306487c6218da1425e429fae01c1a40eb50965dff31a04ed",
		"653088d87cac950f7134f7aaf727dd173dba00706f30699fad03898b1d0acf0d",
		"8d298c57e269a379c4956583f095b2557c8f07226410e02ae852bc4563864790",
		"1d8556c79fb221ee123c4bf3c81034bcf2c719c064e3acd1ac52b48bd9a541ed",
		"1204fda761249130840855476f0dc005ab102dc1b0117e4642bbb2cc6808cd2a",
		"2c757b4d8aa63527a515f5febaffde0fefa859959411223fce05c0be3962d1f4",
		"37c16fa19244957bb2f23811d5761bc5bf7ef62384950c760c3fe6abd391f693",
	}
	privKeyList = make([]*ecdsa.PrivateKey, len(privStrs), len(privStrs))
	geneAccList = make([]*account.Account, len(privStrs), len(privStrs))
	bootNode    = "enode://9fb2a4ae5e0271638ac9ab77567ba3ccfcc10403f3263b053c7c714696f712566a624355588b733ac2dc40f103ad23edc25027427e80113c5cdf2bc0f060ce4b@127.0.0.1:38291"
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

func getPrivKeyByIndex(idx int) *ecdsa.PrivateKey {
	if privKeyList[idx] == nil {
		priv, err := p2p.GetPrivateKeyFromConfig(privStrs[idx])
		if err != nil {
			utils.Fatalf("failed to transfer privkey from string type", "privkey", privStrs[idx], "err", err)
		}
		privKeyList[idx] = priv
	}
	return privKeyList[idx]
}

func getAccByIndex(idx int) *account.Account {
	if geneAccList[idx] == nil {
		key := account.BytesToIdentityKey(common.FromHex(privStrs[idx]))
		acc, err := account.NewAccountWithKey(key)
		if err != nil {
			utils.Fatalf("failed to create account by private key", "privkey: ", privStrs[idx], "err", err)
		}
		geneAccList[idx] = &acc
	}
	return geneAccList[idx]
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
	tx, _ := sign(evmTx)
	return &types.Transaction{
		EvmTx:  tx,
		TxType: types.EvmTx,
	}
}

func sign(evmTx *types.EvmTransaction) (*types.EvmTransaction, error) {
	return types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), privKeyList[0])
}

func newkey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}

func createNode(ip string, port int) *enode.Node {
	remid := &newkey().PublicKey
	nd := enode.NewV4(remid, []byte(ip), port, 0)
	nd.ID()
	return nd
}
