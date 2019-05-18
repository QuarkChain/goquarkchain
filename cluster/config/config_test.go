package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
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

func TestShardGenesis(t *testing.T) {
	var (
		shardGensis ShardGenesis
	)
	s := []byte(`{"ROOT_HEIGHT":0,"VERSION":0,"HEIGHT":0,"HASH_PREV_MINOR_BLOCK":"0000000000000000000000000000000000000000000000000000000000000001","HASH_MERKLE_ROOT":"0000000000000000000000000000000000000000000000000000000000000002","EXTRA_DATA":"497420776173207468652062657374206f662074696d65732c206974207761732074686520776f727374206f662074696d65732c202e2e2e202d20436861726c6573204469636b656e73","TIMESTAMP":1519147489,"DIFFICULTY":10000,"GAS_LIMIT":12000000,"NONCE":0,"ALLOC":{}}`)
	assert.NoError(t, json.Unmarshal(s, &shardGensis))

	assert.Equal(t, common.FromHex("497420776173207468652062657374206f662074696d65732c206974207761732074686520776f727374206f662074696d65732c202e2e2e202d20436861726c6573204469636b656e73"), shardGensis.ExtraData)
	jsonConfig, err := json.Marshal(&shardGensis)
	assert.NoError(t, err)
	assert.Equal(t, string(s), string(jsonConfig))
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
func TestLoadConfig(t *testing.T) {
	target := `{"P2P_PORT":38291,"JSON_RPC_PORT":38391,"PRIVATE_JSON_RPC_PORT":38491,"ENABLE_TRANSACTION_HISTORY":false,"DB_PATH_ROOT":"./db","LOG_LEVEL":"info","START_SIMULATED_MINING":false,"CLEAN":false,"GENESIS_DIR":"../genesis_data","QUARKCHAIN":{"CHAIN_SIZE":1,"MAX_NEIGHBORS":32,"NETWORK_ID":3,"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD":10000,"BLOCK_EXTRA_DATA_SIZE_LIMIT":1024,"GUARDIAN_PUBLIC_KEY":"ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d","GUARDIAN_PRIVATE_KEY":null,"P2P_PROTOCOL_VERSION":0,"P2P_COMMAND_SIZE_LIMIT":4294967295,"SKIP_ROOT_DIFFICULTY_CHECK":false,"SKIP_ROOT_COINBASE_CHECK":false,"SKIP_MINOR_DIFFICULTY_CHECK":false,"GENESIS_TOKEN":"","ROOT":{"MAX_STALE_ROOT_BLOCK_HEIGHT_DIFF":60,"CONSENSUS_TYPE":"POW_SIMULATE","CONSENSUS_CONFIG":{"TARGET_BLOCK_TIME":10,"REMOTE_MINE":false},"GENESIS":{"VERSION":0,"HEIGHT":0,"HASH_PREV_BLOCK":"","HASH_MERKLE_ROOT":"","TIMESTAMP":1519147489,"DIFFICULTY":1000000,"NONCE":0},"COINBASE_ADDRESS":"","COINBASE_AMOUNT":120000000000000000000,"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME":40,"DIFFICULTY_ADJUSTMENT_FACTOR":1024},"CHAINS":[{"CHAIN_ID":0,"SHARD_SIZE":2,"CONSENSUS_TYPE":"POW_SIMULATE","CONSENSUS_CONFIG":{"TARGET_BLOCK_TIME":3,"REMOTE_MINE":false},"GENESIS":{"ROOT_HEIGHT":0,"VERSION":0,"HEIGHT":0,"HASH_PREV_MINOR_BLOCK":"0000000000000000000000000000000000000000000000000000000000001234","HASH_MERKLE_ROOT":"","EXTRA_DATA":"497420776173207468652062657374206f662074696d65732c206974207761732074686520776f727374206f662074696d65732c202e2e2e202d20436861726c6573204469636b656e73","TIMESTAMP":1519147489,"DIFFICULTY":10000,"GAS_LIMIT":12000000,"NONCE":0,"ALLOC":{"000000000000000000000000000000000000001100000011":111111111,"000000000000000000000000000000000000002200000022":222222222}},"COINBASE_AMOUNT":5000000000000000000,"GAS_LIMIT_EMA_DENOMINATOR":1024,"GAS_LIMIT_ADJUSTMENT_FACTOR":1024,"GAS_LIMIT_MINIMUM":5000,"GAS_LIMIT_MAXIMUM":9223372036854775807,"GAS_LIMIT_USAGE_ADJUSTMENT_NUMERATOR":3,"GAS_LIMIT_USAGE_ADJUSTMENT_DENOMINATOR":2,"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME":7,"DIFFICULTY_ADJUSTMENT_FACTOR":512,"EXTRA_SHARD_BLOCKS_IN_ROOT_BLOCK":3,"POSW_CONFIG":{"ENABLED":false,"DIFF_DIVIDER":20,"WINDOW_SIZE":256,"TOTAL_STAKE_PER_BLOCK":1000000000000000000000000000},"COINBASE_ADDRESS":"0x000000000000000000000000000000000000000000000003"}],"REWARD_TAX_RATE":0.5},"MASTER":{"MASTER_TO_SLAVE_CONNECT_RETRY_DELAY":1},"SLAVE_LIST":[{"HOST":"127.0.0.1","PORT":38000,"ID":"S0","CHAIN_MASK_LIST":[4]},{"HOST":"127.0.0.1","PORT":38001,"ID":"S1","CHAIN_MASK_LIST":[5]},{"HOST":"127.0.0.1","PORT":38002,"ID":"S2","CHAIN_MASK_LIST":[6]},{"HOST":"127.0.0.1","PORT":38003,"ID":"S3","CHAIN_MASK_LIST":[7]}],"SIMPLE_NETWORK":{"BOOT_STRAP_HOST":"127.0.0.1","BOOT_STRAP_PORT":38291},"P2P":{"BOOT_NODES":"","PRIV_KEY":"","MAX_PEERS":25,"UPNP":false,"ALLOW_DIAL_IN_RATIO":1,"PREFERRED_NODES":""},"MONITORING":{"NETWORK_NAME":"","CLUSTER_ID":"127.0.0.1","KAFKA_REST_ADDRESS":"","MINER_TOPIC":"qkc_miner","PROPAGATION_TOPIC":"block_propagation","ERRORS":"error"}}`
	var (
		goClstr ClusterConfig
	)
	if err := loadConfig("../../tests/config_test/test_config.json", &goClstr); err != nil {
		t.Fatalf("Failed to load json file, err: %v", err)
	}
	data, err := json.Marshal(goClstr)
	assert.NoError(t, err)
	assert.Equal(t, string(data), target)

	newConfig := new(ClusterConfig)
	err = json.Unmarshal([]byte(target), newConfig)
	assert.Equal(t, goClstr.Quarkchain.shards, (*newConfig).Quarkchain.shards)

}
