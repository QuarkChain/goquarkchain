package config

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/naoina/toml"
	"github.com/stretchr/testify/assert"
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
	cluster := NewClusterConfig()
	jsonConfig, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("cluster struct marshal error: %v", err)
	}

	// Make sure reward tax rate is correctly marshalled
	if !strings.Contains(string(jsonConfig), "\"REWARD_TAX_RATE\":0.5") {
		t.Error("reward tax rate is not correctly marshalled")
	}

	var c ClusterConfig
	err = json.Unmarshal(jsonConfig, &c)
	if err != nil {
		t.Fatalf("UnMarsshal cluster config error: %v", err)
	}
	if c.DbPathRoot != "./data" {
		t.Fatalf("db path root error")
	}

	_, err = c.GetSlaveConfig("S0")
	if err != nil {
		t.Fatalf("slave should not to be empty: %v", err)
	}
	p2p := c.P2P
	if p2p == nil {
		t.Fatalf("")
	}
	quarkchain := c.Quarkchain
	if quarkchain.RewardTaxRate.Cmp(new(big.Rat).SetFloat64(0.5)) != 0 {
		t.Errorf("wrong marshaling of reward tax rate")
	}

	quarkchain.update(4, 10, 10)
	if quarkchain.ShardSize != 4 {
		t.Fatalf("quarkchain update function set shard size failed, shard size: %d", quarkchain.ShardSize)
	}
	for i := 0; i < int(quarkchain.ShardSize); i++ {
		if quarkchain.GetGenesisRootHeight(uint32(i)) != 0 {
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
		t.Fatalf("the list of ids should be empty.")
	}
}

func TestSlaveConfig(t *testing.T) {
	s := []byte(`{
		"IP": "1.2.3.4",
		"PORT": 123,
		"ID": "S1",
		"SHARD_MASK_LIST": [4]
	}`)

	assert := assert.New(t)

	var sc SlaveConfig
	assert.NoError(json.Unmarshal(s, &sc))
	assert.Equal(uint32(4), sc.ShardMaskList[0].GetMask())

	jsonConfig, err := json.Marshal(&sc)
	assert.NoError(err)
	assert.True(strings.Contains(string(jsonConfig), "MASK_LIST\":[4]"))
}
