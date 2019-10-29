// Modified from go-ethereum under GNU Lesser General Public License
package main

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/slave"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/internal/debug"
	"github.com/elastic/gosigar"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"
	"math"
	"os"
	godebug "runtime/debug"
	"sort"
	"strconv"
)

const (
	clientIdentifier = "master" // Client identifier to advertise over the network
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// The app that holds all commands and flags.
	app        = utils.NewApp(gitCommit, "the quarkchain command line interface")
	usageFlags = []cli.Flag{
		ClusterConfigFlag,
		utils.ServiceFlag,
		utils.DataDirFlag,
		utils.LogLevelFlag,
		utils.CleanFlag,
		utils.CacheFlag,
		utils.StartSimulatedMiningFlag,
		utils.GenesisDirFlag,
		utils.NumChainsFlag,
		utils.NumSlavesFlag,
		utils.NumShardsFlag,
		utils.RootBlockIntervalSecFlag,
		utils.MinorBlockIntervalSecFlag,
		utils.NetworkIdFlag,
		utils.PortStartFlag,
		utils.DbPathRootFlag,
		utils.P2pFlag,
		utils.P2pPortFlag,
		utils.CheckDBFlag,
		utils.CheckDBRBlockFromFlag,
		utils.CheckDBRBlockToFlag,
		utils.CheckDBRBlockBatchFlag,

		utils.EnableTransactionHistoryFlag,
		utils.MaxPeersFlag,
		utils.BootnodesFlag,
		utils.UpnpFlag,
		utils.PrivkeyFlag,
	}

	rpcFlags = []cli.Flag{
		utils.RPCDisabledFlag,
		utils.RPCListenAddrFlag,
		utils.RPCPortFlag,
		utils.PrivateRPCListenAddrFlag,
		utils.PrivateRPCPortFlag,
		utils.IPCEnableFlag,
		utils.IPCPathFlag,
		utils.GRPCAddrFlag,
		utils.GRPCPortFlag,
		utils.WSEnableFlag,
		utils.WSRPCHostFlag,
		utils.WSRPCPortFlag,
	}
)

func init() {
	// Initialize the CLI app and start Geth
	app.Action = cluster
	app.HideVersion = true // we have a command to print the version
	app.Commands = []cli.Command{}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, debug.Flags...)
	app.Flags = append(app.Flags, usageFlags...)
	app.Flags = append(app.Flags, rpcFlags...)

	app.Before = func(ctx *cli.Context) error {
		logdir := ""
		if err := debug.Setup(ctx, logdir); err != nil {
			return err
		}
		// Cap the cache allowance and tune the garbage collector
		var (
			mem       gosigar.Mem
			allowance = 1024
		)
		if err := mem.Get(); err == nil {
			allowance = int(mem.Total / 1024 / 1024)
			if cache := ctx.GlobalInt(utils.CacheFlag.Name); cache > allowance {
				log.Warn("Sanitizing cache to Go's GC limits", "provided", cache, "updated", allowance)
				ctx.GlobalSet(utils.CacheFlag.Name, strconv.Itoa(allowance))
			}
		}
		// Ensure Go's GC ignores the database cache for trigger percentage
		cache := ctx.GlobalInt(utils.CacheFlag.Name)
		gogc := math.Max(10, math.Min(90, 100*(float64(cache)/float64(allowance))))
		log.Debug("Sanitizing Go's GC trigger", "percent", int(gogc))
		godebug.SetGCPercent(int(gogc))

		return nil
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// geth is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func cluster(ctx *cli.Context) error {
	if args := ctx.Args(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}
	node := makeFullNode(ctx)
	startService(ctx, node)
	node.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested accounts, and starts the RPC/IPC interfaces and the
// miner.
func startService(ctx *cli.Context, stack *service.Node) {
	debug.Memsize.Add("service", stack)

	// Start up the node itself
	utils.StartService(stack)

	if stack.IsMaster() {
		var master *master.QKCMasterBackend
		if err := stack.Service(&master); err != nil {
			utils.Fatalf("master service not running %v", err)
		}
		if master.GetClusterConfig().CheckDB {
			master.CheckDB()
			os.Exit(0)
			return
		}
		if err := master.Start(); err != nil {
			utils.Fatalf("Failed to init cluster service", "err", err)
		}
		if err := stack.StartP2P(); err != nil {
			utils.Fatalf("failed to start p2p", "err", err)
		}
	} else {
		var slave *slave.SlaveBackend
		if err := stack.Service(&slave); err != nil {
			utils.Fatalf("slave service not running %v", err)
		}
	}
}
