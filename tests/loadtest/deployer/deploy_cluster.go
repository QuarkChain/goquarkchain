package main

import (
	"github.com/QuarkChain/goquarkchain/tests/loadtest/deployer/deploy"
	"github.com/ethereum/go-ethereum/log"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"io"
	"os"
)

var (
	configPath = "./deployConfig.json"
)
var (
	ostream log.Handler
	glogger *log.GlogHandler
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
	toolManager := getToolManager()
	log.Info("begin gen config")
	toolManager.GenClusterConfig() //gen ./cluster_config_template.json
	log.Info("begin gen cluster exe")
	toolManager.MakeClusterExe()
	log.Info("begin send file to others cluster")
	toolManager.SendFileToCluster()
	log.Info("begin start cluster")
	toolManager.StartCluster()
	log.Info("start cluster successfully")
}
