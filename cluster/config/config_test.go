package config_test

import (
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"testing"
)

func TestClusterConfig(t *testing.T) {
	cluster := config.NewClusterConfig()
	jsCluster, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("cluster struct marshal error: %v", err)
	}

	data := &config.ClusterConfig{}
	err = json.Unmarshal(jsCluster, data)
	if err != nil {
		t.Fatalf("UnMarsshal cluster config error: %v", err)
	}
	if data.GetDbPathRoot() != "./data" {
		t.Fatalf("db path root error")
	}

	slaveList := data.GetSlaveConfig("S0")
	if slaveList == nil {
		t.Fatalf("slave should not to be empty.")
	}
	p2p := data.GetP2p()
	if p2p == nil {
		t.Fatalf("")
	}
	quarkchain := data.Quarkchain
	quarkchain.Update(4, 10, 10)
	if quarkchain.ShardSize != 4 {
		t.Fatalf("quarkchain update function set shard size failed, shard size: %d", quarkchain.ShardSize)
	}
	for i := quarkchain.ShardSize - 1; i >= 0; i-- {
		if quarkchain.GetGenesisRootHeight(i) != 0 {
			t.Fatalf("genesis height is not equal to 0.")
		}
	}
	shardIds := quarkchain.GetGenesisShardIds()
	if len(shardIds) != 8 {
	} else {
		for i := 0; i < len(shardIds); i++ {
			if shardIds[i] != i {
				t.Fatalf("shard id is mismatched, shard id: %d", i)
			}
		}
	}
	initializeIds := quarkchain.GetInitializedShardIdsBeforeRootHeight(0)
	if len(initializeIds) != 0 {
		t.Fatalf("The list of ids should be empty.")
	}
	// fmt.Println("cluster json: ", string(jsCluster))
}
