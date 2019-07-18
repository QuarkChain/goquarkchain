package agent

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
)

const (
	goStartTemplate  = "nohup ./cluster %s --cluster_config %s >>%s 2>&1 &"
	stopTemplate     = "ps aux | grep '%s' | grep '%s' | grep -v 'grep' | xargs kill;"
	clientIdentifier = "master"
)

type NodeInfo struct {
	*config.SlaveConfig
	startCmd string
	stopCmd  string
}

type ClusterCtrol struct {
	StartCmd  string
	StopCmd   string
	Type      LanguageType
	SlaveList map[string]*NodeInfo
}

func NewCmdCtrol(tp LanguageType, cfg *config.ClusterConfig) (*ClusterCtrol, error) {
	ctrol := new(ClusterCtrol)
	switch tp {
	case goType:
		ctrol.StartCmd = fmt.Sprintf(goStartTemplate, )
	case pyType:

	}
}
