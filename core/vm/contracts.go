// Copyright 2014 The go-ethereum Authors
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

package vm

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/crypto/ripemd160"
)

var (
	RootChainPoSWContractAddr              = "514b430000000000000000000000000000000001"
	rootChainPoSWContractBytecode          = `608060405234801561001057600080fd5b50610700806100206000396000f3fe60806040526004361061007b5760003560e01c8063853828b61161004e578063853828b6146101b5578063a69df4b5146101ca578063f83d08ba146101df578063fd8c4646146101e75761007b565b806316934fc4146100d85780632e1a7d4d1461013c578063485d3834146101685780636c19e7831461018f575b336000908152602081905260409020805460ff16156100cb5760405162461bcd60e51b815260040180806020018281038252602681526020018061062e6026913960400191505060405180910390fd5b6100d5813461023b565b50005b3480156100e457600080fd5b5061010b600480360360208110156100fb57600080fd5b50356001600160a01b031661029b565b6040805194151585526020850193909352838301919091526001600160a01b03166060830152519081900360800190f35b34801561014857600080fd5b506101666004803603602081101561015f57600080fd5b50356102cf565b005b34801561017457600080fd5b5061017d61034a565b60408051918252519081900360200190f35b610166600480360360208110156101a557600080fd5b50356001600160a01b0316610351565b3480156101c157600080fd5b506101666103c8565b3480156101d657600080fd5b50610166610436565b6101666104f7565b3480156101f357600080fd5b5061021a6004803603602081101561020a57600080fd5b50356001600160a01b0316610558565b604080519283526001600160a01b0390911660208301528051918290030190f35b8015610297576002820154808201908111610291576040805162461bcd60e51b81526020600482015260116024820152706164646974696f6e206f766572666c6f7760781b604482015290519081900360640190fd5b60028301555b5050565b600060208190529081526040902080546001820154600283015460039093015460ff9092169290916001600160a01b031684565b336000908152602081905260409020805460ff1680156102f3575080600101544210155b6102fc57600080fd5b806002015482111561030d57600080fd5b6002810180548390039055604051339083156108fc029084906000818181858888f19350505050158015610345573d6000803e3d6000fd5b505050565b6203f48081565b336000908152602081905260409020805460ff16156103a15760405162461bcd60e51b81526004018080602001828103825260268152602001806106546026913960400191505060405180910390fd5b6003810180546001600160a01b0319166001600160a01b038416179055610297813461023b565b6103d06105fa565b5033600090815260208181526040918290208251608081018452815460ff16151581526001820154928101929092526002810154928201839052600301546001600160a01b031660608201529061042657600080fd5b61043381604001516102cf565b50565b336000908152602081905260409020805460ff16156104865760405162461bcd60e51b815260040180806020018281038252602b8152602001806106a1602b913960400191505060405180910390fd5b60008160020154116104df576040805162461bcd60e51b815260206004820152601b60248201527f73686f756c642068617665206578697374696e67207374616b65730000000000604482015290519081900360640190fd5b805460ff191660019081178255426203f48001910155565b336000908152602081905260409020805460ff166105465760405162461bcd60e51b815260040180806020018281038252602781526020018061067a6027913960400191505060405180910390fd5b805460ff19168155610433813461023b565b6000806105636105fa565b506001600160a01b03808416600090815260208181526040918290208251608081018452815460ff161580158252600183015493820193909352600282015493810193909352600301549092166060820152906105c75750600091508190506105f5565b60608101516000906001600160a01b03166105e35750836105ea565b5060608101515b604090910151925090505b915091565b6040518060800160405280600015158152602001600081526020016000815260200160006001600160a01b03168152509056fe73686f756c64206f6e6c7920616464207374616b657320696e206c6f636b656420737461746573686f756c64206f6e6c7920736574207369676e657220696e206c6f636b656420737461746573686f756c64206e6f74206c6f636b20616c72656164792d6c6f636b6564206163636f756e747373686f756c64206e6f7420756e6c6f636b20616c72656164792d756e6c6f636b6564206163636f756e7473a265627a7a72315820f2c044ad50ee08e7e49c575b49e8de27cac8322afdb97780b779aa1af44e40d364736f6c634300050b0032`
	currentMntIDAddr                       = "000000000000000000000000000000514b430001"
	transferMntAddr                        = "000000000000000000000000000000514b430002"
	deployRootChainPoSWStakingContractAddr = "000000000000000000000000000000514b430003"

	currentMntIDGas                       = uint64(3)
	transferMntGas                        = uint64(3)
	deployRootChainPoSWStakingContractGas = uint64(3)

	SystemContracts = []SystemContract{
		{
			address:  common.HexToAddress(RootChainPoSWContractAddr),
			bytecode: common.Hex2Bytes(rootChainPoSWContractBytecode),
		},
	}
)

const (
	ROOT_CHAIN_POSW = iota
)

type SystemContract struct {
	address  common.Address
	bytecode []byte
}

func (s *SystemContract) Address() common.Address {
	return s.address
}

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64                                // RequiredPrice calculates the contract gas use
	Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) // Run runs the precompiled contract
	GetEnableTime() uint64
	SetEnableTime(data uint64)
}

// PrecompiledContractsHomestead contains the default set of pre-compiled Ethereum
// contracts used in the Frontier and Homestead releases.
var PrecompiledContractsHomestead = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}): &ecrecover{},
	common.BytesToAddress([]byte{2}): &sha256hash{},
	common.BytesToAddress([]byte{3}): &ripemd160hash{},
	common.BytesToAddress([]byte{4}): &dataCopy{},
}

// PrecompiledContractsByzantium contains the default set of pre-compiled Ethereum
// contracts used in the Byzantium release.
var PrecompiledContractsByzantium = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}):                            &ecrecover{},
	common.BytesToAddress([]byte{2}):                            &sha256hash{},
	common.BytesToAddress([]byte{3}):                            &ripemd160hash{},
	common.BytesToAddress([]byte{4}):                            &dataCopy{},
	common.BytesToAddress([]byte{5}):                            &bigModExp{},
	common.BytesToAddress([]byte{6}):                            &bn256Add{},
	common.BytesToAddress([]byte{7}):                            &bn256ScalarMul{},
	common.BytesToAddress([]byte{8}):                            &bn256Pairing{},
	common.HexToAddress(currentMntIDAddr):                       &currentMntID{},
	common.HexToAddress(transferMntAddr):                        &transferMnt{},
	common.HexToAddress(deployRootChainPoSWStakingContractAddr): &deployRootChainPoSWStakingContract{},
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
func RunPrecompiledContract(p PrecompiledContract, input []byte, contract *Contract, evm *EVM) (ret []byte, err error) {
	gas := p.RequiredGas(input)
	if contract.UseGas(gas) {
		return p.Run(input, evm, contract)
	}
	return nil, ErrOutOfGas
}

// ECRECOVER implemented as a native contract.
type ecrecover struct {
	enableTime uint64
}

func (c *ecrecover) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *ecrecover) SetEnableTime(data uint64) {
	c.enableTime = data
}

func (c *ecrecover) RequiredGas(input []byte) uint64 {
	return params.EcrecoverGas
}

func (c *ecrecover) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	const ecRecoverInputLength = 128

	input = common.RightPadBytes(input, ecRecoverInputLength)
	// "input" is (hash, v, r, s), each 32 bytes
	// but for ecrecover we want (r, s, v)

	r := new(big.Int).SetBytes(input[64:96])
	s := new(big.Int).SetBytes(input[96:128])
	v := input[63] - 27

	// tighter sig s values input homestead only apply to tx sigs
	if !allZero(input[32:63]) || !crypto.ValidateSignatureValues(v, r, s, false) {
		return nil, nil
	}
	// v needs to be at the end for libsecp256k1
	pubKey, err := crypto.Ecrecover(input[:32], append(input[64:128], v))
	// make sure the public key is a valid one
	if err != nil {
		return nil, nil
	}

	// the first byte of pubkey is bitcoin heritage
	return common.LeftPadBytes(crypto.Keccak256(pubKey[1:])[12:], 32), nil
}

// SHA256 implemented as a native contract.
type sha256hash struct {
	enableTime uint64
}

func (c *sha256hash) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *sha256hash) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *sha256hash) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.Sha256PerWordGas + params.Sha256BaseGas
}
func (c *sha256hash) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	h := sha256.Sum256(input)
	return h[:], nil
}

// RIPEMD160 implemented as a native contract.
type ripemd160hash struct {
	enableTime uint64
}

func (c *ripemd160hash) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *ripemd160hash) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *ripemd160hash) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.Ripemd160PerWordGas + params.Ripemd160BaseGas
}
func (c *ripemd160hash) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	ripemd := ripemd160.New()
	ripemd.Write(input)
	return common.LeftPadBytes(ripemd.Sum(nil), 32), nil
}

// data copy implemented as a native contract.
type dataCopy struct{ enableTime uint64 }

func (c *dataCopy) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *dataCopy) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *dataCopy) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.IdentityPerWordGas + params.IdentityBaseGas
}
func (c *dataCopy) Run(in []byte, evm *EVM, contract *Contract) ([]byte, error) {
	return in, nil
}

// bigModExp implements a native big integer exponential modular operation.
type bigModExp struct{ enableTime uint64 }

var (
	big1      = big.NewInt(1)
	big4      = big.NewInt(4)
	big8      = big.NewInt(8)
	big16     = big.NewInt(16)
	big32     = big.NewInt(32)
	big64     = big.NewInt(64)
	big96     = big.NewInt(96)
	big480    = big.NewInt(480)
	big1024   = big.NewInt(1024)
	big3072   = big.NewInt(3072)
	big199680 = big.NewInt(199680)
)

func (c *bigModExp) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *bigModExp) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bigModExp) RequiredGas(input []byte) uint64 {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32))
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32))
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32))
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Retrieve the head 32 bytes of exp for the adjusted exponent length
	var expHead *big.Int
	if big.NewInt(int64(len(input))).Cmp(baseLen) <= 0 {
		expHead = new(big.Int)
	} else {
		if expLen.Cmp(big32) > 0 {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), 32))
		} else {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), expLen.Uint64()))
		}
	}
	// Calculate the adjusted exponent length
	var msb int
	if bitlen := expHead.BitLen(); bitlen > 0 {
		msb = bitlen - 1
	}
	adjExpLen := new(big.Int)
	if expLen.Cmp(big32) > 0 {
		adjExpLen.Sub(expLen, big32)
		adjExpLen.Mul(big8, adjExpLen)
	}
	adjExpLen.Add(adjExpLen, big.NewInt(int64(msb)))

	// Calculate the gas cost of the operation
	gas := new(big.Int).Set(math.BigMax(modLen, baseLen))
	switch {
	case gas.Cmp(big64) <= 0:
		gas.Mul(gas, gas)
	case gas.Cmp(big1024) <= 0:
		gas = new(big.Int).Add(
			new(big.Int).Div(new(big.Int).Mul(gas, gas), big4),
			new(big.Int).Sub(new(big.Int).Mul(big96, gas), big3072),
		)
	default:
		gas = new(big.Int).Add(
			new(big.Int).Div(new(big.Int).Mul(gas, gas), big16),
			new(big.Int).Sub(new(big.Int).Mul(big480, gas), big199680),
		)
	}
	gas.Mul(gas, math.BigMax(adjExpLen, big1))
	gas.Div(gas, new(big.Int).SetUint64(params.ModExpQuadCoeffDiv))

	if gas.BitLen() > 64 {
		return math.MaxUint64
	}
	return gas.Uint64()
}

func (c *bigModExp) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32)).Uint64()
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32)).Uint64()
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32)).Uint64()
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Handle a special case when both the base and mod length is zero
	if baseLen == 0 && modLen == 0 {
		return []byte{}, nil
	}
	// Retrieve the operands and execute the exponentiation
	var (
		base = new(big.Int).SetBytes(getData(input, 0, baseLen))
		exp  = new(big.Int).SetBytes(getData(input, baseLen, expLen))
		mod  = new(big.Int).SetBytes(getData(input, baseLen+expLen, modLen))
	)
	if mod.BitLen() == 0 {
		// Modulo 0 is undefined, return zero
		return common.LeftPadBytes([]byte{}, int(modLen)), nil
	}
	return common.LeftPadBytes(base.Exp(base, exp, mod).Bytes(), int(modLen)), nil
}

// newCurvePoint unmarshals a binary blob into a bn256 elliptic curve point,
// returning it, or an error if the point is invalid.
func newCurvePoint(blob []byte) (*bn256.G1, error) {
	p := new(bn256.G1)
	if _, err := p.Unmarshal(blob); err != nil {
		return nil, err
	}
	return p, nil
}

// newTwistPoint unmarshals a binary blob into a bn256 elliptic curve point,
// returning it, or an error if the point is invalid.
func newTwistPoint(blob []byte) (*bn256.G2, error) {
	p := new(bn256.G2)
	if _, err := p.Unmarshal(blob); err != nil {
		return nil, err
	}
	return p, nil
}

// bn256Add implements a native elliptic curve point addition.
type bn256Add struct{ enableTime uint64 }

func (c *bn256Add) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *bn256Add) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256Add) RequiredGas(input []byte) uint64 {
	return params.Bn256AddGas
}

func (c *bn256Add) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	x, err := newCurvePoint(getData(input, 0, 64))
	if err != nil {
		return nil, err
	}
	y, err := newCurvePoint(getData(input, 64, 64))
	if err != nil {
		return nil, err
	}
	res := new(bn256.G1)
	res.Add(x, y)
	return res.Marshal(), nil
}

// bn256ScalarMul implements a native elliptic curve scalar multiplication.
type bn256ScalarMul struct{ enableTime uint64 }

func (c *bn256ScalarMul) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *bn256ScalarMul) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256ScalarMul) RequiredGas(input []byte) uint64 {
	return params.Bn256ScalarMulGas
}

func (c *bn256ScalarMul) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	p, err := newCurvePoint(getData(input, 0, 64))
	if err != nil {
		return nil, err
	}
	res := new(bn256.G1)
	res.ScalarMult(p, new(big.Int).SetBytes(getData(input, 64, 32)))
	return res.Marshal(), nil
}

var (
	// true32Byte is returned if the bn256 pairing check succeeds.
	true32Byte = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	// false32Byte is returned if the bn256 pairing check fails.
	false32Byte = make([]byte, 32)

	// errBadPairingInput is returned if the bn256 pairing input is invalid.
	errBadPairingInput = errors.New("bad elliptic curve pairing size")
)

// bn256Pairing implements a pairing pre-compile for the bn256 curve
type bn256Pairing struct{ enableTime uint64 }

func (c *bn256Pairing) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *bn256Pairing) SetEnableTime(data uint64) {
	c.enableTime = data
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bn256Pairing) RequiredGas(input []byte) uint64 {
	return params.Bn256PairingBaseGas + uint64(len(input)/192)*params.Bn256PairingPerPointGas
}

func (c *bn256Pairing) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	// Handle some corner cases cheaply
	if len(input)%192 > 0 {
		return nil, errBadPairingInput
	}
	// Convert the input into a set of coordinates
	var (
		cs []*bn256.G1
		ts []*bn256.G2
	)
	for i := 0; i < len(input); i += 192 {
		c, err := newCurvePoint(input[i : i+64])
		if err != nil {
			return nil, err
		}
		t, err := newTwistPoint(input[i+64 : i+192])
		if err != nil {
			return nil, err
		}
		cs = append(cs, c)
		ts = append(ts, t)
	}
	// Execute the pairing checks and return the results
	if bn256.PairingCheck(cs, ts) {
		return true32Byte, nil
	}
	return false32Byte, nil
}

type currentMntID struct {
	enableTime uint64
}

func (c *currentMntID) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *currentMntID) SetEnableTime(data uint64) {
	c.enableTime = data
}
func (c *currentMntID) RequiredGas(input []byte) uint64 {
	return currentMntIDGas
}

func (c *currentMntID) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	contract.TokenIDQueried = true //TODO later to apply
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, evm.TransferTokenID)
	zeors := make([]byte, 24, 24)
	return append(zeors, b...), nil
}

type transferMnt struct {
	enableTime uint64
}

func (c *transferMnt) GetEnableTime() uint64 {
	return c.enableTime
}

func (c *transferMnt) SetEnableTime(data uint64) {
	c.enableTime = data
}

func (c *transferMnt) RequiredGas(input []byte) uint64 {
	return transferMntGas
}
func (c *transferMnt) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	if len(input) < 96 {
		return nil, errors.New("should 96")
	}

	toBytes := getData(input, 0, 32)
	toAddr := common.BytesToAddress(toBytes)

	mntBytes := getData(input, 32, 32)
	mnt := new(big.Int).SetBytes(mntBytes)

	valueBytes := getData(input, 64, 32)
	value := new(big.Int).SetBytes(valueBytes)

	data := getData(input, 96, uint64(len(input)-96))
	t := evm.TransferTokenID
	evm.TransferTokenID = mnt.Uint64()
	ret, remainedGas, err := evm.Call(contract.caller, toAddr, data, contract.Gas, value)
	err = checkTokenIDQueried(err, contract, evm.TransferTokenID, evm.StateDB.GetQuarkChainConfig().GetDefaultChainTokenID())
	evm.TransferTokenID = t
	gasUsed := contract.Gas - remainedGas
	if ok := contract.UseGas(gasUsed); !ok {
		return nil, ErrOutOfGas
	}
	return ret, err
}

type deployRootChainPoSWStakingContract struct {
	enableTime uint64
}

func (c *deployRootChainPoSWStakingContract) GetEnableTime() uint64 {
	return c.enableTime
}
func (c *deployRootChainPoSWStakingContract) SetEnableTime(data uint64) {
	c.enableTime = data
}

func (r *deployRootChainPoSWStakingContract) RequiredGas(input []byte) uint64 {
	return deployRootChainPoSWStakingContractGas
}

func (r *deployRootChainPoSWStakingContract) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	var (
		targetAddr = SystemContracts[ROOT_CHAIN_POSW].Address()
		bytecode   = SystemContracts[ROOT_CHAIN_POSW].bytecode
		value      = big.NewInt(0)
	)
	// Use predetermined contract address
	_, addr, leftover, err := evm.Create(contract.self, bytecode, contract.Gas, value, &targetAddr)
	if err != nil {
		return nil, err
	}
	contract.Gas = leftover
	return addr.Bytes(), nil
}
