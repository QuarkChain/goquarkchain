// Modified from go-ethereum under GNU Lesser General Public License
package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/naoina/toml"
	"gopkg.in/urfave/cli.v1"
	"os"
	"reflect"
	"unicode"
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
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() service.Config {
	cfg := service.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.HTTPModules = append(cfg.HTTPModules, "qkc", "shh")
	cfg.WSModules = append(cfg.WSModules, "qkc", "shh")
	cfg.IPCPath = "qkc.ipc"
	cfg.SvrModule = "rpc."
	return cfg
}

func makeConfigNode(ctx *cli.Context) (*service.Node, qkcConfig) {
	// Load defaults.
	cfg := qkcConfig{
		Cluster: config.NewClusterConfig(),
		Service: defaultNodeConfig(),
	}

	// Load cluster config file.
	if file := ctx.GlobalString(ClusterConfigFlag.Name); file != "" {
		if err := loadConfig(file, &cfg.Cluster); err != nil {
			utils.Fatalf("%v", err)
		}
	} else {
		utils.SetClusterConfig(ctx, &cfg.Cluster)
	}
	// Load default cluster config.
	utils.SetNodeConfig(ctx, &cfg.Service)

	ServiceName := ctx.GlobalString(utils.ServiceFlag.Name)
	if ServiceName != clientIdentifier {
		slv, err := cfg.Cluster.GetSlaveConfig(ServiceName)
		if err != nil {
			utils.Fatalf("service type is error: %v", err)
		}
		cfg.Service.Name = ServiceName
		cfg.Service.SvrPort = slv.Port
	}

	stack, err := service.New(&cfg.Service)
	stack.SetModule(ServiceName == clientIdentifier)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	return stack, cfg
}

func makeFullNode(ctx *cli.Context) *service.Node {
	stack, cfg := makeConfigNode(ctx)

	if !stack.GetModule() {
		for _, slv := range cfg.Cluster.SlaveList {
			if cfg.Service.Name == slv.Id {
				utils.RegisterSlaveService(stack, slv)
				break
			}
		}
	} else {
		utils.RegisterMasterService(stack, &cfg.Cluster)
	}

	return stack
}
