package main

import (
	"flag"
	"io"
	"os"

	"github.com/QuarkChain/goquarkchain/tests/loadtest/deployer/deploy"
	"github.com/ethereum/go-ethereum/log"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

var (
	configPath = "./deployConfig.json"
)
var (
	ostream log.Handler
	glogger *log.GlogHandler
)
var (
	initConf = flag.Bool("genconf", false, "only gen config , not run cluster")
	initEnv  = flag.Bool("init_env", false, "only init env")
	genLog  = flag.Bool("log", false, "only gen log")
)

func init() {
	usecolor := (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())) && os.Getenv("TERM") != "dumb"
	output := io.Writer(os.Stderr)
	if usecolor {
		output = colorable.NewColorableStderr()
	}
	ostream = log.StreamHandler(output, log.TerminalFormat(usecolor))
	glogger = log.NewGlogHandler(ostream)
	glogger.Verbosity(log.Lvl(4))
	log.Root().SetHandler(glogger)
}
func getToolManager() *deploy.ToolManager {
	config := deploy.LoadConfig(configPath)
	toolManger := deploy.NewToolManager(config)
	return toolManger
}

func main() {
	flag.Parse()
	toolManager := getToolManager()
	if *initConf {
		toolManager.GenAllClusterConfig()
		return
	}

	if *genLog {
		toolManager.StartGenLog()
		return
	}

	if *initEnv {
		log.Info("ready to set Environment!!!")
		toolManager.InitEnv()
		log.Info("Environment set successfully!!!!")
		return
	}
	toolManager.StartClusters()
	log.Info("ready to check status")
	toolManager.CheckPeerStatus()

}
