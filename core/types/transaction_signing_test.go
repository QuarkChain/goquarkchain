// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestEIP155Signing(t *testing.T) {
	key, _ := crypto.GenerateKey()
	recipient := publicKey2Recipient(&key.PublicKey)

	signer := NewEIP155Signer(1)
	tx, err := SignTx(NewEvmTransaction(0, recipient, new(big.Int), 0, new(big.Int), 0, 0, 1, 0, nil, 0, 0), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != recipient {
		t.Errorf("exected from and address to be equal. Got %x want %x", from, recipient)
	}
}

func TestEIP155SigningVersion2(t *testing.T) {
	key, _ := crypto.GenerateKey()
	recipient := publicKey2Recipient(&key.PublicKey)

	const chainID = 3
	signer := NewEIP155Signer(chainID)
	// version 2 is a standard EIP155 transaction: chainId is folded into V.
	tx, err := SignTx(NewEvmTransaction(0, recipient, new(big.Int), 0, new(big.Int), 0, 0, chainID, 2, nil, 0, 0), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	// V must be encoded as recid + chainId*2 + 35 (recid is 0 or 1).
	v, _, _ := tx.RawSignatureValues()
	base := new(big.Int).Sub(v, big.NewInt(int64(chainID)*2+35))
	if base.Cmp(big.NewInt(0)) != 0 && base.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("unexpected V encoding: got %v, recid should be 0 or 1", v)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != recipient {
		t.Errorf("expected from and address to be equal. Got %x want %x", from, recipient)
	}
}

func TestEIP155SigningVersion2UsesTransactionNetworkID(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	recipient := publicKey2Recipient(&key.PublicKey)

	const networkID = 3
	signer := NewEIP155Signer(1)
	tx, err := SignTx(NewEvmTransaction(0, recipient, new(big.Int), 0, new(big.Int), 0, 0, networkID, 2, nil, 0, 0), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	v, _, _ := tx.RawSignatureValues()
	recoveryID := new(big.Int).Sub(v, big.NewInt(networkID*2+35))
	if recoveryID.Sign() < 0 || recoveryID.Cmp(big.NewInt(1)) > 0 {
		t.Fatalf("unexpected V encoding for transaction network ID: %v", v)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != recipient {
		t.Fatalf("expected sender %x, got %x", recipient, from)
	}
}
