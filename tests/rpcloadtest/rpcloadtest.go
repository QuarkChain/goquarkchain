package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	clt "github.com/QuarkChain/goqkcclient/client"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

var (
	client    = clt.NewClient("http://127.0.0.1:38391")
	networkId uint32
	once      sync.Once
)

type Account struct {
	Address account.Address
	Privkey *ecdsa.PrivateKey
	IdNonce map[uint32]uint64
	mu      sync.RWMutex
}

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
	a.IdNonce[fullShardId]++
	a.mu.Unlock()
	return clt.SignTx(tx, a.Privkey)
}

type GenesisAddress struct {
	Address string `json:"address"`
	PrivKey string `json:"key"`
}

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
	var (
		stopCh  = make(chan struct{})
		acCount = 8000
		accsCh  = make(chan *account.Address, acCount*2)
		errsCh  = make(chan *account.Address, acCount*2)
		runsCh  = make(chan bool, runtime.NumCPU()*2)

		accTxs = make(map[common.Address]*Account)
		txsCh  = make(chan *struct {
			Tx      *clt.EvmTransaction
			Address common.Address
			Id      uint32
		}, 20000)

		wg sync.WaitGroup
	)
	for i := 0; i < runtime.NumCPU()*2; i++ {
		runsCh <- true
	}

	var (
		fullshardids []uint32
		err          error
	)
	// fullshardids, err = client.GetFullShardIds()
	log.Info("fullshardId list", fullshardids)

	addresses, err := loadGenesisAddrs("./accounts/loadtest.json")
	if err != nil {
		panic(err)
	}

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
		txTime  int64
		txCount int
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
