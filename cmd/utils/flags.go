// Modified from go-ethereum under GNU Lesser General Public License
package utils

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/slave"
	"github.com/QuarkChain/goquarkchain/consensus"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"gopkg.in/urfave/cli.v1"
)

var (
	CommandHelpTemplate = `{{.cmd.Name}}{{if .cmd.Subcommands}} command{{end}}{{if .cmd.Flags}} [command options]{{end}} [arguments...]
{{if .cmd.Description}}{{.cmd.Description}}
{{end}}{{if .cmd.Subcommands}}
SUBCOMMANDS:
	{{range .cmd.Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .categorizedFlags}}
{{range $idx, $categorized := .categorizedFlags}}{{$categorized.Name}} OPTIONS:
{{range $categorized.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}{{end}}`
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	cli.CommandHelpTemplate = CommandHelpTemplate
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = params.VersionWithMeta
	if len(gitCommit) >= 8 {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.

var (
	// General settings
	DataDirFlag = DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DirectoryString{service.DefaultDataDir()},
	}
	LogLevelFlag = cli.StringFlag{
		Name:  "log_level",
		Usage: "log level",
	}
	CleanFlag = cli.BoolFlag{
		Name:  "clean",
		Usage: "clean database ?",
	}
	StartSimulatedMiningFlag = cli.BoolFlag{
		Name:  "start_simulated_mining",
		Usage: "start simulated mining ?",
	}
	GenesisDirFlag = cli.StringFlag{
		Name:  "genesis_dir",
		Usage: "gensis data dir",
	}
	NumChainsFlag = cli.IntFlag{
		Name:  "num_chains",
		Usage: "chain number",
	}
	NumShardsFlag = cli.IntFlag{
		Name:  "num_shards",
		Usage: "shard number",
	}
	RootBlockIntervalSecFlag = cli.IntFlag{
		Name:  "root_block_interval_sec",
		Usage: "interval time of root block",
	}
	MinorBlockIntervalSecFlag = cli.IntFlag{
		Name:  "minor_block_interval_sec",
		Usage: "",
	}
	NetworkIdFlag = cli.IntFlag{
		Name:  "network_id",
		Usage: "net work id",
	}
	NumSlavesFlag = cli.IntFlag{
		Name:  "num_slaves",
		Usage: "slaves number",
	}
	PortStartFlag = cli.IntFlag{
		Name:  "port_start",
		Usage: "slave start port",
	}
	DbPathRootFlag = cli.StringFlag{
		Name:  "db_path_root",
		Usage: "Data directory for the databases and keystore",
	}
	P2pFlag = cli.BoolFlag{
		Name:  "p2p",
		Usage: "enables new p2p module",
	}
	EnableTransactionHistoryFlag = cli.BoolFlag{
		Name:  "enable_transaction_history",
		Usage: "enable transaction history function",
	}
	SimpleNetworkBootstrapHostFlag = cli.StringFlag{
		Name:  "simple_network_bootstrap_host",
		Usage: "simple network bootstrap host",
		Value: "127.0.0.1",
	}
	SimpleNetworkBootstrapPortFlag = cli.Uint64Flag{
		Name:  "simple_network_bootstrap_port",
		Usage: "simple network bootstrap port",
		Value: 38291,
	}
	MaxPeersFlag = cli.Uint64Flag{
		Name:  "max_peers",
		Usage: "max peer for new p2p module",
	}
	BootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "comma separated encodes in the format: enode://PUBKEY@IP:PORT",
	}
	UpnpFlag = cli.BoolFlag{
		Name:  "upnp",
		Usage: "if true,automatically runs a upnp service that sets port mapping on upnp-enabled devices",
	}
	PrivkeyFlag = cli.StringFlag{
		Name:  "privkey",
		Usage: "if empty,will be automatically generated; but note that it will be lost upon node reboot",
	}
	ServiceFlag = cli.StringFlag{
		Name:  "service",
		Usage: "svrvice type,if has eight slaves,fill like(S0,S2,...S7)",
		Value: "master",
	}
	CheckDBFlag = cli.BoolFlag{
		Name:  "check_db",
		Usage: "if true, will perform integrity check on db only",
	}
	CheckDBRBlockFromFlag = cli.IntFlag{
		Name:  "check_db_rblock_from",
		Usage: "", //todo add usage
		Value: -1,
	}
	CheckDBRBlockToFlag = cli.IntFlag{
		Name:  "check_db_rblock_to",
		Usage: "", //todo add usage
		Value: 0,
	}

	// Performance tuning settings
	CacheFlag = cli.IntFlag{
		Name:  "cache",
		Usage: "Megabytes of memory allocated to internal caching",
		Value: 1024,
	}
	// RPC settings
	RPCDisabledFlag = cli.BoolFlag{
		Name:  "json_rpc_disable",
		Usage: "disable the public HTTP-RPC server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "json_rpc_addr",
		Usage: "HTTP-RPC server listening interface",
		Value: "0.0.0.0",
	}
	RPCPortFlag = cli.IntFlag{
		Name:  "json_rpc_port",
		Usage: "public HTTP-RPC server listening port",
	}
	PrivateRPCListenAddrFlag = cli.StringFlag{
		Name:  "json_rpc_private_addr",
		Usage: "HTTP-RPC server listening interface",
		Value: service.DefaultHTTPHost,
	}
	PrivateRPCPortFlag = cli.IntFlag{
		Name:  "json_rpc_private_port",
		Usage: "public HTTP-RPC server listening port",
	}

	GRPCAddrFlag = cli.StringFlag{
		Name:  "grpc_addr",
		Usage: "master or slave grpc address",
		Value: config.GrpcHost,
	}
	GRPCPortFlag = cli.IntFlag{
		Name:  "grpc_port",
		Usage: "public json rpc port",
		Value: int(config.GrpcPort),
	}
	P2pPortFlag = cli.IntFlag{
		Name:  "p2p_port",
		Usage: "Network listening port",
	}

	IPCEnableFlag = cli.BoolFlag{
		Name:  "ipc",
		Usage: "enable the IPC-RPC server",
	}
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
	}
	MaxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	NATFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "any",
	}
	NoDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}
	DiscoveryV5Flag = cli.BoolFlag{
		Name:  "v5disc",
		Usage: "Enables the experimental RLPx V5 (Topic Discovery) mechanism",
	}
)

// setBootstrapNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodes(ctx *cli.Context, cfg *p2p.Config, clstrCfg *config.ClusterConfig) {

	if cfg.BootstrapNodes != nil {
		return // already set, don't apply defaults.
	}

	urls := params.MainnetBootnodes
	if clstrCfg.P2P.BootNodes != "" {
		urls = strings.Split(clstrCfg.P2P.BootNodes, ",")
	}

	cfg.BootstrapNodes = make([]*enode.Node, 0, len(urls))
	cfg.WhitelistNodes = make(map[string]*enode.Node)
	for _, url := range urls {
		node, err := enode.ParseV4(url)
		if err != nil {
			log.Crit("Bootstrap URL invalid", "enode", url, "err", err)
		}
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
		cfg.WhitelistNodes[node.IP().String()] = node
	}
}

// setNAT creates a port mapper from command line flags.
func setNAT(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(NATFlag.Name) {
		natif, err := nat.Parse(ctx.GlobalString(NATFlag.Name))
		if err != nil {
			Fatalf("Option %s: %v", NATFlag.Name, err)
		}
		cfg.NAT = natif
	}
}

// splitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// setHTTP creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setHTTP(ctx *cli.Context, cfg *service.Config, clstrCfg *config.ClusterConfig) {
	if !ctx.GlobalBool(RPCDisabledFlag.Name) {
		port := clstrCfg.JSONRPCPort
		if ctx.GlobalIsSet(RPCPortFlag.Name) {
			port = uint16(ctx.GlobalInt(RPCPortFlag.Name))
		}
		cfg.HTTPEndpoint = fmt.Sprintf("%s:%d", ctx.GlobalString(RPCListenAddrFlag.Name), port)
	}
	port := clstrCfg.PrivateJSONRPCPort
	if ctx.GlobalIsSet(PrivateRPCPortFlag.Name) {
		port = uint16(ctx.GlobalInt(PrivateRPCPortFlag.Name))
	}
	cfg.HTTPPrivEndpoint = fmt.Sprintf("%s:%d", ctx.GlobalString(PrivateRPCListenAddrFlag.Name), port)
}

func setGRPC(ctx *cli.Context, cfg *service.Config) {
	if ctx.GlobalIsSet(GRPCPortFlag.Name) {
		cfg.SvrPort = uint16(ctx.GlobalInt(GRPCPortFlag.Name))
	}
	if ctx.GlobalIsSet(GRPCAddrFlag.Name) {
		cfg.SvrHost = ctx.GlobalString(GRPCAddrFlag.Name)
	}
}

// setIPC creates an IPC path configuration from the set command line flags,
// returning an empty string if IPC was explicitly disabled, or the set path.
func setIPC(ctx *cli.Context, cfg *service.Config) {
	checkExclusive(ctx, IPCEnableFlag, IPCPathFlag)
	switch {
	case !ctx.GlobalBool(IPCEnableFlag.Name):
		cfg.IPCPath = ""
	case ctx.GlobalIsSet(IPCPathFlag.Name):
		cfg.IPCPath = ctx.GlobalString(IPCPathFlag.Name)
	}
}

func SetP2PConfig(ctx *cli.Context, cfg *p2p.Config, clstrCfg *config.ClusterConfig) {
	// setNodeKey(ctx, cfg)
	setNAT(ctx, cfg)
	cfg.ListenAddr = fmt.Sprintf(":%d", clstrCfg.P2PPort)
	setBootstrapNodes(ctx, cfg, clstrCfg)

	// load p2p privkey
	priv := clstrCfg.P2P.PrivKey
	if ctx.GlobalIsSet(PrivkeyFlag.Name) {
		priv = ctx.GlobalString(PrivkeyFlag.Name)
	}
	if priv != "" {
		privkey, err := p2p.GetPrivateKeyFromConfig(priv)
		if err != nil {
			Fatalf("failed to transfer privkey", "err", err)
		}
		cfg.PrivateKey = privkey
	}

	cfg.NetWorkId = clstrCfg.Quarkchain.NetworkID

	cfg.MaxPeers = int(clstrCfg.P2P.MaxPeers)
	log.Info("Maximum peer count", "QKC", cfg.MaxPeers, "total", cfg.MaxPeers)

	if ctx.GlobalIsSet(MaxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(MaxPendingPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NoDiscoverFlag.Name) {
		cfg.NoDiscovery = true
	}

	// if we're running a light client or server, force enable the v5 peer discovery
	// unless it is explicitly disabled with --nodiscover note that explicitly specifying
	// --v5disc overrides --nodiscover, in which case the later only disables v4 discovery
	forceV5Discovery := !ctx.GlobalBool(NoDiscoverFlag.Name)
	if ctx.GlobalIsSet(DiscoveryV5Flag.Name) {
		cfg.DiscoveryV5 = ctx.GlobalBool(DiscoveryV5Flag.Name)
	} else if forceV5Discovery {
		cfg.DiscoveryV5 = true
	}
}

func SetClusterConfig(ctx *cli.Context, cfg *config.ClusterConfig) {

	var (
		shardSize      = cfg.Quarkchain.Chains[0].ShardSize
		chainSize      = cfg.Quarkchain.ChainSize
		rootBlockTime  = cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime
		minorBlockTime = cfg.Quarkchain.Chains[0].ConsensusConfig.TargetBlockTime
	)
	// quarkchain.update
	if ctx.GlobalIsSet(NumShardsFlag.Name) {
		shardSize = uint32(ctx.GlobalInt(NumShardsFlag.Name))
		if !common.IsP2(uint32(shardSize)) {
			Fatalf("shard size must be pow of 2")
		}
	}
	if ctx.GlobalIsSet(NumChainsFlag.Name) {
		chainSize = uint32(ctx.GlobalInt(NumChainsFlag.Name))
	}
	if ctx.GlobalIsSet(RootBlockIntervalSecFlag.Name) {
		rootBlockTime = uint32(ctx.GlobalInt(RootBlockIntervalSecFlag.Name))
	}
	if ctx.GlobalIsSet(MinorBlockIntervalSecFlag.Name) {
		minorBlockTime = uint32(ctx.GlobalInt(MinorBlockIntervalSecFlag.Name))
	}
	cfg.Quarkchain.Update(chainSize, shardSize, rootBlockTime, minorBlockTime)

	// quarkchain.network_id
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.Quarkchain.NetworkID = uint32(ctx.GlobalInt(NetworkIdFlag.Name))
	}

	// cluster.clean
	if ctx.GlobalIsSet(CleanFlag.Name) {
		cfg.Clean = ctx.GlobalBool(CleanFlag.Name)
	}

	// cluster.start_simulate_mining
	if ctx.GlobalIsSet(StartSimulatedMiningFlag.Name) {
		cfg.StartSimulatedMining = ctx.GlobalBool(StartSimulatedMiningFlag.Name)
	}

	// cluster.genesisDir
	if ctx.GlobalIsSet(GenesisDirFlag.Name) {
		cfg.GenesisDir = ctx.GlobalString(GenesisDirFlag.Name)
	}

	portStart := cfg.SlaveList[0].Port
	if ctx.GlobalIsSet(PortStartFlag.Name) {
		portStart = uint16(ctx.GlobalInt(PortStartFlag.Name))
	}

	numSlaves := config.DefaultNumSlaves
	if ctx.GlobalIsSet(NumSlavesFlag.Name) {
		numSlaves = ctx.GlobalInt(NumSlavesFlag.Name)
	}

	cfg.SlaveList = make([]*config.SlaveConfig, 0)
	for i := 0; i < numSlaves; i++ {
		slaveConfig := config.NewDefaultSlaveConfig()
		slaveConfig.Port = portStart + uint16(i)
		slaveConfig.ID = fmt.Sprintf("S%d", i)
		slaveConfig.ChainMaskList = append(slaveConfig.ChainMaskList, types.NewChainMask(uint32(i)|uint32(numSlaves)))
		cfg.SlaveList = append(cfg.SlaveList, slaveConfig)
	}

	// cluster.loglevel
	if ctx.GlobalIsSet(LogLevelFlag.Name) {
		cfg.LogLevel = ctx.GlobalString(LogLevelFlag.Name)
	}

	// cluster.db_path_root
	if ctx.GlobalIsSet(DbPathRootFlag.Name) {
		cfg.DbPathRoot = ctx.GlobalString(DbPathRootFlag.Name)
	}

	if ctx.GlobalIsSet(P2pPortFlag.Name) {
		cfg.P2PPort = uint16(ctx.GlobalInt(P2pPortFlag.Name))
	}

	if ctx.GlobalIsSet(RPCPortFlag.Name) {
		cfg.JSONRPCPort = uint16(ctx.GlobalInt(RPCPortFlag.Name))
	}

	if ctx.GlobalIsSet(PrivateRPCPortFlag.Name) {
		cfg.PrivateJSONRPCPort = uint16(ctx.GlobalInt(PrivateRPCPortFlag.Name))
	}

	if ctx.GlobalBool(StartSimulatedMiningFlag.Name) {
		cfg.StartSimulatedMining = true
	}
	if ctx.GlobalBool(EnableTransactionHistoryFlag.Name) {
		cfg.EnableTransactionHistory = true
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.Quarkchain.NetworkID = uint32(ctx.GlobalInt(NetworkIdFlag.Name))
	}

	// p2p config
	if ctx.GlobalIsSet(BootnodesFlag.Name) {
		cfg.P2P.BootNodes = ctx.GlobalString(BootnodesFlag.Name)
	}

	if ctx.GlobalIsSet(PrivkeyFlag.Name) {
		cfg.P2P.PrivKey = ctx.GlobalString(PrivkeyFlag.Name)
	}

	if ctx.GlobalIsSet(MaxPeersFlag.Name) {
		cfg.P2P.MaxPeers = ctx.GlobalUint64(MaxPeersFlag.Name)
	}

	if ctx.GlobalBool(UpnpFlag.Name) {
		cfg.P2P.UPnP = true
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func SetNodeConfig(ctx *cli.Context, cfg *service.Config, clstrCfg *config.ClusterConfig) {
	SetP2PConfig(ctx, &cfg.P2P, clstrCfg)
	setIPC(ctx, cfg)
	setHTTP(ctx, cfg, clstrCfg)
	setGRPC(ctx, cfg)
	setDataDir(ctx, cfg, clstrCfg)
	setCheckDBConfig(ctx, clstrCfg)
}

func setCheckDBConfig(ctx *cli.Context, clstrCfg *config.ClusterConfig) {
	clstrCfg.CheckDB = ctx.GlobalBool(CheckDBFlag.Name)
	if ctx.GlobalIsSet(CheckDBRBlockFromFlag.Name) {
		clstrCfg.CheckDBRBlockFrom = ctx.GlobalInt(CheckDBRBlockFromFlag.Name)
	}
	if ctx.GlobalIsSet(CheckDBRBlockToFlag.Name) {
		clstrCfg.CheckDBRBlockTo = ctx.GlobalInt(CheckDBRBlockToFlag.Name)
	}
}

func setDataDir(ctx *cli.Context, cfg *service.Config, clstrCfg *config.ClusterConfig) {
	if ctx.GlobalIsSet(DataDirFlag.Name) {
		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
	}
}

// checkExclusive verifies that only a single instance of the provided flags was
// set by the user. Each flag might optionally be followed by a string type to
// specialize it further.
func checkExclusive(ctx *cli.Context, args ...interface{}) {
	set := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		// Make sure the next argument is a flag and skip if not set
		flag, ok := args[i].(cli.Flag)
		if !ok {
			panic(fmt.Sprintf("invalid argument, not cli.Flag type: %T", args[i]))
		}
		// Check if next arg extends current and expand its name if so
		name := flag.GetName()

		if i+1 < len(args) {
			switch option := args[i+1].(type) {
			case string:
				// Extended flag check, make sure value set doesn't conflict with passed in option
				if ctx.GlobalString(flag.GetName()) == option {
					name += "=" + option
					set = append(set, "--"+name)
				}
				// shift arguments and continue
				i++
				continue

			case cli.Flag:
			default:
				panic(fmt.Sprintf("invalid argument, not cli.Flag or string extension: %T", args[i+1]))
			}
		}
		// Mark the flag if it's set
		if ctx.GlobalIsSet(flag.GetName()) {
			set = append(set, "--"+name)
		}
	}
	if len(set) > 1 {
		Fatalf("Flags %v can't be used at the same time", strings.Join(set, ", "))
	}
}

// RegisterEthService adds an QuarkChain client to the stack.
func RegisterMasterService(stack *service.Node, diffCalculator consensus.EthDifficultyCalculator, cfg *config.ClusterConfig) {
	err := stack.Register(func(ctx *service.ServiceContext) (service.Service, error) {
		// TODO add cluster create function
		return master.New(ctx, diffCalculator, cfg)
	})
	if err != nil {
		Fatalf("Failed to register the QuarkChain service: %v", err)
	}
}

func RegisterSlaveService(stack *service.Node, clusterCfg *config.ClusterConfig, cfg *config.SlaveConfig) {
	err := stack.Register(func(ctx *service.ServiceContext) (service.Service, error) {
		return slave.New(ctx, clusterCfg, cfg)
	})
	if err != nil {
		Fatalf("Failed to register the cluster grpc service: %v", err)
	}
}

// MakeChainDatabase open an LevelDB using the flags passed to the client and will hard crash if it fails.

// MigrateFlags sets the global flag from a local flag when it's set.
// This is a temporary function used for migrating old command/flags to the
// new format.
//
// e.g. geth account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
// geth --keystore /tmp/mykeystore --lightkdf account new
//
// This allows the use of the existing configuration functionality.
// When all flags are migrated this function can be removed and the existing
// configuration functionality must be changed that is uses local flags
func MigrateFlags(action func(ctx *cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, name := range ctx.FlagNames() {
			if ctx.IsSet(name) {
				ctx.GlobalSet(name, ctx.String(name))
			}
		}
		return action(ctx)
	}
}

// MakeDataDir retrieves the currently requested data directory, terminating
// if none (or the empty string) is specified. If the node is starting a testnet,
// the a subdirectory of the specified datadir will be used.
func MakeDataDir(ctx *cli.Context) string {
	if path := ctx.GlobalString(DataDirFlag.Name); path != "" {
		return path
	}
	Fatalf("Cannot determine default data directory, please set manually (--datadir)")
	return ""
}
