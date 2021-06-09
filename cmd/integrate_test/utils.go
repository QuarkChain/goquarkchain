package test

import (
	"crypto/ecdsa"
	"math/big"
	"net"
	"strings"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

var (
	testGenesisTokenID = qkcCommon.TokenIDEncode("QKC")
)

var (
	defaultP2PPort = 38291
	privStrs       = []string{
		"966a253dd39a1832306487c6218da1425e429fae01c1a40eb50965dff31a04ed",
		"653088d87cac950f7134f7aaf727dd173dba00706f30699fad03898b1d0acf0d",
		"8d298c57e269a379c4956583f095b2557c8f07226410e02ae852bc4563864790",
		"1d8556c79fb221ee123c4bf3c81034bcf2c719c064e3acd1ac52b48bd9a541ed",
		"1204fda761249130840855476f0dc005ab102dc1b0117e4642bbb2cc6808cd2a",
		"2c757b4d8aa63527a515f5febaffde0fefa859959411223fce05c0be3962d1f4",
		"37c16fa19244957bb2f23811d5761bc5bf7ef62384950c760c3fe6abd391f693",
	}
	privKeyList     = make([]*ecdsa.PrivateKey, len(privStrs), len(privStrs))
	geneAccList     = make([]*account.Account, len(privStrs), len(privStrs))
	defaultbootNode = "enode://9fb2a4ae5e0271638ac9ab77567ba3ccfcc10403f3263b053c7c714696f712566a624355588b733ac2dc40f103ad23edc25027427e80113c5cdf2bc0f060ce4b@127.0.0.1:38291"
	fakeBootNode    = "enode://9fb2a4ae5e0271638ac9ab77567ba3ccfcc10403f3263b053c7c714696f712566a624355588b733ac2dc40f103ad23edc25027427e80113c5cdf2bc0f060ce4b@127.0.0.1:38291," +
		"enode://ec972351eb2dae496b57f1f790b2dd5149e34a862052c2bddd75ab6a7e686abd3306be62a914b8ce0e8eb9e34e6c700bce43fbf0c5677edb273a199a848e3fae@127.0.0.1:38292," +
		"enode://32bd9692c12419aa99b67e625666825ef46fae7d1948ea935936a7f2bac0fdec75acbcd796f218dd7c69f369d18e8f361b0092277f2646e7f2702d80dbdbc009@127.0.0.1:38293," +
		"enode://542cf20c69df6d7ccd57abb6e85d953c1fde99be9fa803cc4942dadae871ab42152064a6cd969d6a035a962de8518a9165f29b3b4f08c262aa976ea4d0434519@127.0.0.1:38294," +
		"enode://748183adf9964295ad09e083eeafb81f971737ab88a9f86b3f5d7ae653a3882db4d97fa39e7367e3d79b03166e2fc29542d2ab59d385a688477036663af24779@127.0.0.1:38295," +
		"enode://fd71383f642ee33f698ee6e834a6999587bd8aac3bf6ada6b89368e9d9283d0b43d61e5e8a8f0a8ed37e1894fffbba2f96104b1454e2669d952e67468599f933@127.0.0.1:38296," +
		"enode://e3437973930235be04a58868c51db97c9c4770a198bac1474bdd0757152b34dbb8f44fdb702a68a3d015e0ef00dd4d6541af534bce8e2cbb7ab0159f2c201009@127.0.0.1:38297"
	genesisBalance = 10000000000000000
)

func getBootNodes(bootNodes string) []*enode.Node {
	if bootNodes == "" {
		return nil
	}
	var nodes = make([]*enode.Node, 0, 0)
	urls := strings.Split(bootNodes, ",")
	for _, url := range urls {
		node, err := enode.ParseV4(url)
		if err != nil {
			utils.Fatalf("Bootstrap URL invalid", "enode", url, "err", err)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func getPrivKeyByIndex(idx int) *ecdsa.PrivateKey {
	if idx >= len(privStrs) {
		return nil
	}
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
	if idx >= len(privStrs) {
		return nil
	}
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

func createTx(acc account.Address, to *account.Address) *types.Transaction {
	if to == nil {
		to = &acc
	}
	evmTx := types.NewEvmTransaction(0,
		to.Recipient,
		big.NewInt(100),
		uint64(30000),
		new(big.Int).SetUint64(1e9+1),
		uint32(acc.FullShardKey),
		uint32(to.FullShardKey),
		3,
		0,
		[]byte{}, testGenesisTokenID, testGenesisTokenID)
	tx, _ := sign(evmTx)
	return &types.Transaction{
		EvmTx:  tx,
		TxType: types.EvmTx,
	}
}

func sign(evmTx *types.EvmTransaction) (*types.EvmTransaction, error) {
	return types.SignTx(evmTx, types.NewEIP155Signer(evmTx.NetworkId(), 0), privKeyList[0])
}

func newkey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}

func (c *clusterNode) createNode() *enode.Node {
	remid := getPrivKeyByIndex(1).PublicKey
	// port := c.clstrCfg.P2PPort
	return enode.NewV4(&remid, net.IPv4(127, 0, 0, 1), 38291, 38291)
}
