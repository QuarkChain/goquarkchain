package params

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
)

var (
	AccountEnabled      = []byte{1}
	AccountDisabled     = []byte{0}
	superAccountStrList = []string{"0x438befb16aed2d01bc0ba111eee12c65dcdb5275"}
	superAccount        = make(map[account.Recipient]struct{}, 0)
)

func init() {
	for _, v := range superAccountStrList {
		addr := common.BytesToAddress(common.FromHex(v))
		superAccount[addr] = struct{}{}
	}
}

func IsSuperAccount(addr account.Recipient) bool {
	_, ok := superAccount[addr]
	return ok
}
func SetSuperAccount(address account.Recipient) { //only in test
	superAccount[address] = struct{}{}
}
