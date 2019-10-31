package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ybbus/jsonrpc"

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
	for index := 0; index < len(toolManager.LocalConfig.Hosts); index++ {
		log.Info("============begin start cluster============", "index", index, "info", toolManager.LocalConfig.Hosts[index])
		log.Info("==== begin gen config")
		toolManager.GenClusterConfig()
		log.Info("==== begin send file to others cluster")
		toolManager.SendFileToCluster()
		log.Info("==== begin start cluster")
		toolManager.StartCluster(index)
		log.Info("============end start cluster============", "index", index, "info", toolManager.LocalConfig.Hosts[index])
		toolManager.ClusterIndex++
	}

	log.Info("ready to check status")

	for true {
		time.Sleep(10 * time.Second)
		toolManager.ClusterIndex = 0
		for index := 0; index < len(toolManager.LocalConfig.Hosts); index++ {
			masterIP := toolManager.GetIpListDependTag("master")[0]
			pubUrl := fmt.Sprintf("http://%s:38491", masterIP)
			client := jsonrpc.NewClient(pubUrl)
			resp, err := client.Call("getPeers")
			if err != nil {
				panic(fmt.Errorf("getPeer from ip %v err %v", masterIP, err))
			}
			if resp == nil {
				panic(fmt.Errorf("getPeer from ip %v resp==nil", masterIP))
			}
			if resp.Error != nil {
				panic(fmt.Errorf("getPeer from ip %v err %v", masterIP, resp.Error))
			}
			log.Info("check peer status", "masterIP", masterIP, "resp", len(resp.Result.(map[string]interface{})))
			toolManager.ClusterIndex++
		}
	}
}
