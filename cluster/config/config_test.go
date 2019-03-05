package config_test

import (
	"encoding/json"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/naoina/toml"
	"reflect"
	"testing"
	"unicode"
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

func TestClusterConfig(t *testing.T) {
	cluster := config.NewClusterConfig()
	jsCluster, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("cluster struct marshal error: %v", err)
	}
	var data config.ClusterConfig
	err = json.Unmarshal(jsCluster, &data)
	if err != nil {
		t.Fatalf("UnMarsshal cluster config error: %v", err)
	}
	if data.GetDbPathRoot() != "./data" {
		t.Fatalf("db path root error")
	}

	_, err = data.GetSlaveConfig("S0")
	if err != nil {
		t.Fatalf("slave should not to be empty: %v", err)
	}
	p2p := data.GetP2P()
	if p2p == nil {
		t.Fatalf("")
	}
	quarkchain := data.Quarkchain
	quarkchain.Update(4, 10, 10)
	if quarkchain.ShardSize != 4 {
		t.Fatalf("quarkchain update function set shard size failed, shard size: %d", quarkchain.ShardSize)
	}
	for i := 0; i < int(quarkchain.ShardSize); i++ {
		if quarkchain.GetGenesisRootHeight(i) != 0 {
			t.Fatalf("genesis height is not equal to 0.")
		}
	}
	shardIds := quarkchain.GetGenesisShardIds()
	if len(shardIds) != 4 {
		t.Fatalf("shard id list is not enough.")
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
