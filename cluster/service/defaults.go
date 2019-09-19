// Modified from go-ethereum under GNU Lesser General Public License
package service

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/p2p/nat"
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = Config{
	DataDir:      DefaultDataDir(),
	GRPCModules:  []string{"grpc"},
	HTTPModules:  []string{"qkc"},
	WSModules:    []string{"ws"},
	WSOrigins:    []string{"*"},
	IPCPath:      "",
	HTTPTimeouts: rpc.DefaultHTTPTimeouts,
	// SvrModule:        "MasterOp",
	P2P: p2p.Config{
		ListenAddr: ":38291",
		MaxPeers:   25,
		NAT:        nat.Any(),
	},
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "QuarkChain")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "QuarkChain")
		} else {
			return filepath.Join(home, ".QuarkChain")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
