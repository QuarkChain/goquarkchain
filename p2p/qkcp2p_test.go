package p2p

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/log"
	"testing"
)

func FakeEnv() config.ClusterConfig {
	return config.ClusterConfig{
		P2P: &config.P2PConfig{
			BootNodes: "enode://32d87c5cd4b31d81c5b010af42a2e413af253dc3a91bd3d53c6b2c45291c3de71633bf7793447a0d3ddde601f8d21668fca5b33324f14ebe7516eab0da8bab8f@192.168.79.130:38291",
		//	0xab0edab8f4428b95d374503a6917d51838e1426b9e656aca2baffc1e5f359aea25347049351c74b08ffd5d4af6b7d896e48e85e145184c4206e133c48a91465b@47.92.244.48:38291
			PrivKey:"3a28b5ba57c53603b0b07b56bba752f7784bf506fa95edc395f5cf6c7514fe94",
		},
	}
}

func TestP2PManager_Start(t *testing.T) {
	log.InitLog(5)
	env := FakeEnv()
	p2pManager, err := NewP2PManager(env)
	if err!=nil{
		t.Error("NewP2PManager err",err)
	}
	p2pManager.Start()
	go func(){
		//time.Sleep(20*time.Second)
		fmt.Println("停止p2pManager")
		//p2pManager.Stop()
	}()
	p2pManager.Wait()
}