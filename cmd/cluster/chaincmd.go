// Modified from go-ethereum under GNU Lesser General Public License
package main

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"
)

var (
	initCommand = cli.Command{
		Action:    utils.MigrateFlags(initGenesis),
		Name:      "init",
		Usage:     "Bootstrap and initialize a new genesis block",
		ArgsUsage: "<genesisPath>",
		Flags: []cli.Flag{
			utils.DbPathRootFlag,
		},
		Category: "BLOCKCHAIN COMMANDS",
		Description: `
The init command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
)

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(ctx *cli.Context) error {
	cfg := new(config.ClusterConfig)

	if file := ctx.GlobalString(ClusterConfigFlag.Name); file != "" {
		if err := loadConfig(file, cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	} else {
		cfg = config.NewClusterConfig()
	}

	path := service.DefaultDataDir()
	chainType := 0
	isMstr := ctx.GlobalString(utils.ServiceFlag.Name)
	if isMstr != clientIdentifier {
		_, err := cfg.GetSlaveConfig(isMstr)
		if err != nil {
			utils.Fatalf("service type is error: %v", err)
		}
		chainType = 1
	}
	path = filepath.Join(path, isMstr, cfg.DbPathRoot)

	db, err := qkcdb.NewRDBDatabase(path, false)
	if err != nil {
		return err
	}

	genesis := core.NewGenesis(cfg.Quarkchain)

	stored := rawdb.ReadCanonicalHash(db, rawdb.ChainType(chainType), 0)
	if stored == (common.Hash{}) {
		genesis.MustCommitRootBlock(db)
	}
	return nil
}
