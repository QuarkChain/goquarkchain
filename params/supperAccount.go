package params

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
)

var (
	superAccountStrList = []string{"0x438befb16aed2d01bc0ba111eee12c65dcdb5275"}
	superAccount        = make(map[account.Recipient]struct{}, 0)
	// data=='0':newAccount
	//data=='1':disable account
)

func init() {
	for _, v := range superAccountStrList {
		addr := common.BytesToAddress(common.FromHex(v))
		superAccount[addr] = struct{}{}
	}
}

func IsSuperAccount(addr account.Recipient) bool {
	//fmt.Println("???????")
	_, ok := superAccount[addr]
	return ok
}
func SetSuperAccount(address account.Recipient) { //only in test
	superAccount[address] = struct{}{}
}
