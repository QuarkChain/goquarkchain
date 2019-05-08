package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
	var (
		chainSize         uint32 = 2
		shardSizePerChain uint32 = 4
	)
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
	if c.DbPathRoot != "./db" {
		t.Fatalf("db path root error")
	}

	_, err = c.GetSlaveConfig("S0")
	if err != nil {
		t.Fatalf("slave should not to be empty: %v", err)
	}
	if c.P2P == nil {
		t.Fatalf("")
	}
	quarkchain := c.Quarkchain
	if quarkchain.RewardTaxRate.Cmp(new(big.Rat).SetFloat64(0.5)) != 0 {
		t.Errorf("wrong marshaling of reward tax rate")
	}

	shardIds := quarkchain.GetGenesisShardIds()
	// make sure the default chainsize and shardsize
	if len(shardIds) != 2*3 {
		t.Fatalf("shard id list is not enough.")
	}
	for _, fullShardId := range shardIds {
		if quarkchain.GetGenesisRootHeight(fullShardId) != 0 {
			t.Fatalf("genesis height is not equal to 0.")
		}
	}
	initializeIds := quarkchain.GetInitializedShardIdsBeforeRootHeight(0)
	if len(initializeIds) != 0 {
		t.Fatalf("the list of ids should be empty.")
	}
	quarkchain.Update(chainSize, shardSizePerChain, 10, 10)
	if quarkchain.GetShardSizeByChainId(1) != 4 {
		t.Fatalf("quarkchain update function set shard size failed, shard size: %d", quarkchain.GetShardSizeByChainId(1))
	}
}

func TestSlaveConfig(t *testing.T) {
	s := []byte(`{
		"IP": "1.2.3.4",
		"PORT": 123,
		"ID": "S1",
		"CHAIN_MASK_LIST": [4]
	}`)

	var sc SlaveConfig
	assert.NoError(t, json.Unmarshal(s, &sc))
	assert.Equal(t, uint32(4), sc.ChainMaskList[0].GetMask())

	jsonConfig, err := json.Marshal(&sc)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonConfig), "MASK_LIST\":[4]"))
}

func TestLoadClusterConfig(t *testing.T) {
	var (
		goClstr ClusterConfig
		pyClstr ClusterConfig
	)
	if err := loadConfig("./test_config.json", &goClstr); err != nil {
		t.Fatalf("Failed to load json file, err: %v", err)
	}

	if err := loadConfig("../../tests/testdata/testnet/cluster_config_template.json", &pyClstr); err != nil {
		t.Fatalf("Failed to load python config file, err: %v", err)
	}

	if !reflect.DeepEqual(goClstr.SlaveList, pyClstr.SlaveList) {
		t.Fatalf("go config slave list is not equal to python config")
	}
	if goClstr.Quarkchain.ChainSize != pyClstr.Quarkchain.ChainSize {
		t.Fatalf("go config chain size is not equal to python config")
	}
	for i, goChain := range goClstr.Quarkchain.Chains {
		pyCHain := pyClstr.Quarkchain.Chains[i]
		if goChain.ChainID != pyCHain.ChainID {
			t.Fatalf("go config chain size is not equal to python config")
		}
		if goChain.ShardSize != pyCHain.ShardSize {
			t.Fatalf("go config chain size is not equal to python config")
		}
		if reflect.DeepEqual(goChain.Genesis, pyCHain.Genesis) {
			t.Fatalf("go config Genesis is not equal to python config")
		}
		if reflect.DeepEqual(goChain.PoswConfig, pyCHain.PoswConfig) {
			t.Fatalf("go config PoswConfig is not equal to python config")
		}
		if reflect.DeepEqual(goChain.ConsensusConfig, pyCHain.ConsensusConfig) {
			t.Fatalf("go config ConsensusConfig is not equal to python config")
		}
	}
}

func loadConfig(file string, cfg *ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}
