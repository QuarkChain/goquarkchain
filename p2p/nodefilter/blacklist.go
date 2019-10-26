package nodefilter

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
	"sync"
	"time"
)

const (
	// period between adding a dialout_blacklisted node and removing it
	dialoutBlacklistCooldownSec int64 = 24 * 3600
	dialinBlacklistCooldownSec  int64 = 8 * 3600
	unblacklistInterval               = time.Duration(15 * 60)
)

type BlackFilter interface {
	AddDialoutBlacklist(string)
	ChkDialoutBlacklist(string) bool
	PeriodicallyUnblacklist()
}

func NewHandleBlackListErr(text string) error {
	return &BlackErr{text}
}

// errorString is a trivial implementation of error.
type BlackErr struct {
	s string
}

func (e *BlackErr) Error() string {
	return e.s
}

type blackNodes struct {
	currTime time.Time
	// BootstrapNodes | preferedNodes
	WhitelistNodes map[string]*enode.Node
	// IP to unblacklist time, we blacklist by IP
	mu               sync.RWMutex
	dialoutBlacklist map[string]int64
	dialinBlacklist  map[string]int64
}

func NewBlackList(whitelistNodes map[string]*enode.Node) BlackFilter {
	return &blackNodes{
		currTime:         time.Now(),
		WhitelistNodes:   whitelistNodes,
		dialoutBlacklist: make(map[string]int64),
		dialinBlacklist:  make(map[string]int64),
	}
}

func (pm *blackNodes) AddDialoutBlacklist(ip string) {
	if _, ok := pm.WhitelistNodes[ip]; !ok {
		pm.mu.Lock()
		pm.dialoutBlacklist[ip] = time.Now().Unix() + dialoutBlacklistCooldownSec
		pm.mu.Unlock()
	}
}

func (pm *blackNodes) ChkDialoutBlacklist(ip string) bool {
	pm.mu.RLock()
	tm, ok := pm.dialoutBlacklist[ip]
	pm.mu.RUnlock()
	if ok {
		if time.Now().Unix() < tm {
			return true
		}
		pm.mu.Lock()
		delete(pm.dialoutBlacklist, ip)
		pm.mu.Unlock()
	}
	return false
}

func (pm *blackNodes) addDialinBlacklist(ip string) {
	if _, ok := pm.WhitelistNodes[ip]; !ok {
		pm.mu.Lock()
		pm.dialinBlacklist[ip] = time.Now().Unix() + dialinBlacklistCooldownSec
		pm.mu.Unlock()
	}
}

func (pm *blackNodes) chkDialinBlacklist(ip string) bool {
	pm.mu.RLock()
	tm, ok := pm.dialinBlacklist[ip]
	pm.mu.RUnlock()
	if ok {
		if time.Now().Unix() < tm {
			return true
		}
		pm.mu.Lock()
		delete(pm.dialinBlacklist, ip)
		pm.mu.Unlock()
	}
	return false
}

func (b *blackNodes) PeriodicallyUnblacklist() {
	now := time.Now()
	if now.Sub(b.currTime) < unblacklistInterval {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.currTime = now
	for ip, tm := range b.dialoutBlacklist {
		if now.Unix() >= tm {
			delete(b.dialoutBlacklist, ip)
		}
	}
	for ip, tm := range b.dialinBlacklist {
		if now.Unix() >= tm {
			delete(b.dialinBlacklist, ip)
		}
	}
}
