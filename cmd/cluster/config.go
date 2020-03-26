// Modified from go-ethereum under GNU Lesser General Public License
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"unicode"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/naoina/toml"
	"gopkg.in/urfave/cli.v1"
)

var (
	ClusterConfigFlag = cli.StringFlag{Name: "cluster_config", Usage: "", Value: ""}
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

type qkcConfig struct {
	Service service.Config
	// cluster config
	Cluster config.ClusterConfig
}

func loadConfig(file string, cfg *config.ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}

func defaultNodeConfig() service.Config {
	cfg := service.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	return cfg
}

func makeConfigNode(ctx *cli.Context) (*service.Node, qkcConfig) {
	// Load defaults.
	cfg := qkcConfig{
		Cluster: *config.NewClusterConfig(),
		Service: defaultNodeConfig(),
	}

	// Load cluster config file.
	if file := ctx.GlobalString(ClusterConfigFlag.Name); file != "" {
		if err := loadConfig(file, &cfg.Cluster); err != nil {
			utils.Fatalf("%v", err)
		}
		if err := cfg.Cluster.BackWardChainMaskList(); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	utils.SetClusterConfig(ctx, &cfg.Cluster)

	ServiceName := ctx.GlobalString(utils.ServiceFlag.Name)
	if ServiceName != clientIdentifier {
		slv, err := cfg.Cluster.GetSlaveConfig(ServiceName)
		if err != nil {
			utils.Fatalf("service type error: %v", err)
		}
		// set slave name and grpc endpoint
		cfg.Service.Name = ServiceName
		cfg.Cluster.Quarkchain.GRPCHost = slv.IP
		cfg.Cluster.Quarkchain.GRPCPort = slv.Port

		// set websocket endpoint
		if ctx.GlobalBool(utils.WSEnableFlag.Name) {
			sufPort, _ := strconv.Atoi(slv.ID[1:])
			ip, port := slv.IP, slv.WSPort+uint16(sufPort)
			if ctx.GlobalIsSet(utils.WSRPCHostFlag.Name) {
				ip = ctx.GlobalString(utils.WSRPCHostFlag.Name)
			}
			if ctx.GlobalIsSet(utils.WSRPCPortFlag.Name) {
				port = uint16(ctx.GlobalInt(utils.WSRPCPortFlag.Name))
			}
			cfg.Service.WSEndpoint = fmt.Sprintf("%s:%d", ip, port)
		}

		// load genesis accounts
		if err := config.UpdateGenesisAlloc(&cfg.Cluster); err != nil {
			utils.Fatalf("Update genesis alloc err: %v", err)
		}
	}
	// Load default cluster config.
	utils.SetNodeConfig(ctx, &cfg.Service, &cfg.Cluster)

	stack, err := service.New(&cfg.Service)
	stack.SetIsMaster(ServiceName == clientIdentifier)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	for _, v := range params.PrecompiledContractsAfterEvmEnabled {
		if vm.PrecompiledContractsByzantium[v] != nil {
			vm.PrecompiledContractsByzantium[v].SetEnableTime(cfg.Cluster.Quarkchain.EnableEvmTimeStamp)
		}
	}
	for _, v := range params.PrecompiledContractsMnt {
		if vm.PrecompiledContractsByzantium[v] != nil {
			vm.PrecompiledContractsByzantium[v].SetEnableTime(cfg.Cluster.Quarkchain.EnableNonReservedNativeTokenTimestamp)
		}
	}
	vm.SystemContracts[vm.NON_RESERVED_NATIVE_TOKEN].SetTimestamp(cfg.Cluster.Quarkchain.EnableNonReservedNativeTokenTimestamp)
	vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].SetTimestamp(cfg.Cluster.Quarkchain.EnableGeneralNativeTokenTimestamp)

	return stack, cfg
}

func makeFullNode(ctx *cli.Context) *service.Node {
	stack, cfg := makeConfigNode(ctx)

	if !stack.IsMaster() {
		for _, slv := range cfg.Cluster.SlaveList {
			if cfg.Service.Name == slv.ID {
				utils.RegisterSlaveService(stack, &cfg.Cluster, slv)
				break
			}
		}
	} else {
		utils.RegisterMasterService(stack, &cfg.Cluster)
	}

	return stack
}
