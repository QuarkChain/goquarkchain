package tests

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"math/big"
)

// storageJSON represents a 256 bit byte array, but allows less than 256 bits when
// unmarshaling from hex.
type storageJSON common.Hash

func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 // pad on the left
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}

type AllocType byte

// p2p command
const (
	QKCTest AllocType = iota
	ETHTest
)

type GenesisAlloc map[common.Address]GenesisAccount

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Type       AllocType
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance,omitempty"`
	Balances   map[uint64]*big.Int         `json:"balances,omitempty"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

func (g *GenesisAccount) UnmarshalJSON(input []byte) error {
	type GenesisAccount struct {
		Code       *string                          `json:"code,omitempty"`
		Storage    map[storageJSON]storageJSON      `json:"storage,omitempty"`
		Balance    *math.HexOrDecimal256            `json:"balance,omitempty"`
		Balances   map[string]*math.HexOrDecimal256 `json:"balances,omitempty"`
		Nonce      *math.HexOrDecimal64             `json:"nonce,omitempty"`
		PrivateKey *hexutil.Bytes                   `json:"secretKey,omitempty"`
	}
	var dec GenesisAccount
	var err error
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Code != nil { //py use different format to set code in test data !!!!!!!!!!
		if len(*dec.Code) == 0 {
			g.Code = make([]byte, 0)
		} else if len(*dec.Code) >= 2 {
			realCode := *dec.Code
			if (*dec.Code)[0:2] == "0x" {
				realCode = realCode[2:]
			}
			g.Code, err = hex.DecodeString(realCode)
			if err != nil {
				return err
			}
		} else {
			panic(errors.New("?????"))
		}
	}
	if dec.Storage != nil {
		g.Storage = make(map[common.Hash]common.Hash, len(dec.Storage))
		for k, v := range dec.Storage {
			g.Storage[common.Hash(k)] = common.Hash(v)
		}
	}
	if dec.Balance == nil && len(dec.Balances) == 0 {
		return errors.New("missing required field 'balance' for GenesisAccount")
	}
	if dec.Balance != nil && dec.Balances != nil {
		fmt.Println("data.N", dec.Balance)
		fmt.Println("data.NS", dec.Balances)
		return errors.New("balance err? only need one format")
	}
	if dec.Balance != nil {
		g.Balance = (*big.Int)(dec.Balance)
	}

	if dec.Balances != nil {
		g.Balances = make(map[uint64]*big.Int)
		for k, v := range dec.Balances {
			//g.Balances[(*big.Int)(k).Uint64()] = (*big.Int)(v)
			g.Balances[new(big.Int).SetBytes(common.FromHex(k)).Uint64()] = (*big.Int)(v)
		}
	}

	if dec.Nonce != nil {
		g.Nonce = uint64(*dec.Nonce)
	}
	if dec.PrivateKey != nil {
		g.PrivateKey = *dec.PrivateKey
	}

	return nil
}
