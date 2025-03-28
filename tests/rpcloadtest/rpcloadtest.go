package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
	
	clt "github.com/QuarkChain/goqkcclient/client"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

var (
	client    = clt.NewClient("http://127.0.0.1:38391")
	networkId uint32
	once      sync.Once
)

type Account struct {
	Address account.Address
	Privkey *ecdsa.PrivateKey
	IdNonce map[uint32]uint64 // fullShardId: nonce
	mu      sync.RWMutex
}

// ids:	fullShardId list, in the shards
func NewAccount(addr string, privkey string, ids []uint32) (*Account, error) {
	address, err := account.CreatAddressFromBytes(common.FromHex(addr))
	if err != nil {
		return nil, err
	}
	prvkek, err := crypto.ToECDSA(common.FromHex(privkey))
	if err != nil {
		return nil, err
	}
	acc := &Account{
		Address: address,
		Privkey: prvkek,
		IdNonce: make(map[uint32]uint64),
	}
	for _, id := range ids {
		nonce, err := client.GetNonce(&clt.QkcAddress{Recipient: acc.Address.Recipient, FullShardKey: id})
		if err != nil {
			return nil, err
		}
		acc.IdNonce[id] = nonce
	}

	return acc, nil
}

// rectify the nonce in fullShardId shard when the nonce in sending tx is not right.
func (a *Account) RectifyNonce(fullShardId uint32) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	nonce, err := client.GetNonce(&clt.QkcAddress{Recipient: a.Address.Recipient, FullShardKey: fullShardId})
	if err != nil {
		return err
	}
	a.IdNonce[fullShardId] = nonce
	return nil
}

// create tx about fullShardId shard and the nonce will increase.
func (a *Account) CreateTransaction(fullShardId uint32) (tx *clt.EvmTransaction, err error) {
	once.Do(func() {
		networkId, err = client.NetworkID()
	})

	tx = clt.NewEvmTransaction(
		a.IdNonce[fullShardId],
		&a.Address.Recipient,
		big.NewInt(0),
		uint64(30000),
		big.NewInt(1000000000),
		fullShardId,
		fullShardId,
		clt.TokenIDEncode("QKC"),
		clt.TokenIDEncode("QKC"),
		networkId, 0, nil)
	a.mu.Lock()
	a.IdNonce[fullShardId]++ // increase nonce about fullShardId shard.
	a.mu.Unlock()
	return clt.SignTx(tx, a.Privkey)
}

type GenesisAddress struct {
	Address string `json:"address"`
	PrivKey string `json:"key"`
}

// load test accounts func, return address and privatekey
func loadGenesisAddrs(file string) ([]GenesisAddress, error) {
	if _, err := os.Stat(file); err != nil {
		return nil, nil
	}
	fp, err := os.Open(file)
	if err != nil {
		log.Warn("loadGenesisAddr", "file", file, "err", err)
		return nil, err
	}
	defer fp.Close()
	var addresses []GenesisAddress
	decoder := json.NewDecoder(fp)
	for decoder.More() {
		err := decoder.Decode(&addresses)
		if err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func main() {
	// get all fullShardIds
	fullshardids, err := client.GetFullShardIds()
	if err != nil {
		panic(err)
	}
	log.Info("fullshardId list", fullshardids)

	var (
		stopCh  = make(chan struct{})
		acCount = 8000
		accsCh  = make(chan *account.Address, acCount*len(fullshardids))
		errsCh  = make(chan *account.Address, acCount*2)

		// all acCount number of accounts.
		accTxs = make(map[common.Address]*Account)
		txsCh  = make(chan *struct {
			Tx      *clt.EvmTransaction
			Address common.Address
			Id      uint32
		}, 20000)

		wg sync.WaitGroup
	)

	addresses, err := loadGenesisAddrs("./accounts/loadtest.json")
	if err != nil {
		panic(err)
	}

	// creation tx loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		run := true
		for run {
			select {
			case acc := <-accsCh:
				go func() {
					tx, err := accTxs[acc.Recipient].CreateTransaction(acc.FullShardKey)
					if err != nil {
						fmt.Println("failed to create tx", "err", err)
						return
					}
					txsCh <- &struct {
						Tx      *clt.EvmTransaction
						Address common.Address
						Id      uint32
					}{Tx: tx, Address: acc.Recipient, Id: acc.FullShardKey}
				}()
			case <-stopCh:
				run = false
				break
			}
		}
	}()

	// rectify nonce loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		run := true
		for run {
			select {
			case acc := <-errsCh:
				if err := accTxs[acc.Recipient].RectifyNonce(acc.FullShardKey); err != nil {
					fmt.Println("failed to rectify nonce", "address", acc.ToHex(), "err", err)
				}
				accsCh <- acc
			case <-stopCh:
				run = false
				break
			}
		}
	}()

	// create acCount number of accounts
	for i := 0; i < acCount; i++ {
		acc, err := NewAccount(addresses[i].Address, addresses[i].PrivKey, fullshardids)
		if err != nil {
			panic(err)
		}
		accTxs[acc.Address.Recipient] = acc
		for _, id := range fullshardids {
			accsCh <- &account.Address{Recipient: acc.Address.Recipient, FullShardKey: id}
		}
	}

	var (
		txTime  int64 // The time of send successful txs.
		txCount int   // all successful txs counts.
	)
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run := true

			for run {
				select {
				case tx := <-txsCh:
					ac := &account.Address{Recipient: tx.Address, FullShardKey: tx.Id}
					start := time.Now()
					txid, err := client.SendTransaction(tx.Tx)
					if err != nil {
						time.Sleep(20 * time.Millisecond)
						errsCh <- ac
						fmt.Println("failed to send transaction", "err", err)
					} else {
						txTime += time.Now().Sub(start).Nanoseconds()
						txCount++
						accsCh <- ac
						fmt.Println("succeed to send transaction", "txid", hexutil.Encode(txid))
					}
				case <-stopCh:
					run = false
					break
				}
			}
		}()
	}

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		<-sigc
		close(stopCh)
	}()

	wg.Wait()
	fmt.Printf("tx count %d, time used %d ms, use %f ms pre tx\n", txCount, txTime/1000000, float64(txTime)/float64(txCount)/1000000)
}
