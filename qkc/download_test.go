package qkc

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/log"
	"os"
	"testing"
)

func fakeEnv(port uint64) config.ClusterConfig {
	return config.ClusterConfig{
		P2Port: port,
		P2P: &config.P2PConfig{
			MaxPeers:  25,
			PrivKey:   "3a28b5ba57c53603b0b07b56bba752f7784bf506fa95edc395f5cf6c7514fe94",
			BootNodes: "enode://2e711d93d04cfa0c1f271769d4636087809f7b2649329622e1966f3e07de0490eab3ebeb504da04795ba722926fa951896fa9d4570c5088216db4cfe35907f97@192.168.79.137:38291",
		},
	}
}

func init() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
}
func TestDownload(t *testing.T) {
	env := fakeEnv(38291)
	qkcManager, err := NewQKCManager(env)
	go qkcManager.Start()
	fmt.Println("err", err, "qkcManager", qkcManager)
}
