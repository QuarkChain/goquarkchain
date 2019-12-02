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
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	qkcParams "github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/crypto/ripemd160"
	math2 "math"
	"math/big"
)

var (
	RootChainPoSWContractAddr              = "514b430000000000000000000000000000000001"
	rootChainPoSWContractBytecode          = `608060405234801561001057600080fd5b50610700806100206000396000f3fe60806040526004361061007b5760003560e01c8063853828b61161004e578063853828b6146101b5578063a69df4b5146101ca578063f83d08ba146101df578063fd8c4646146101e75761007b565b806316934fc4146100d85780632e1a7d4d1461013c578063485d3834146101685780636c19e7831461018f575b336000908152602081905260409020805460ff16156100cb5760405162461bcd60e51b815260040180806020018281038252602681526020018061062e6026913960400191505060405180910390fd5b6100d5813461023b565b50005b3480156100e457600080fd5b5061010b600480360360208110156100fb57600080fd5b50356001600160a01b031661029b565b6040805194151585526020850193909352838301919091526001600160a01b03166060830152519081900360800190f35b34801561014857600080fd5b506101666004803603602081101561015f57600080fd5b50356102cf565b005b34801561017457600080fd5b5061017d61034a565b60408051918252519081900360200190f35b610166600480360360208110156101a557600080fd5b50356001600160a01b0316610351565b3480156101c157600080fd5b506101666103c8565b3480156101d657600080fd5b50610166610436565b6101666104f7565b3480156101f357600080fd5b5061021a6004803603602081101561020a57600080fd5b50356001600160a01b0316610558565b604080519283526001600160a01b0390911660208301528051918290030190f35b8015610297576002820154808201908111610291576040805162461bcd60e51b81526020600482015260116024820152706164646974696f6e206f766572666c6f7760781b604482015290519081900360640190fd5b60028301555b5050565b600060208190529081526040902080546001820154600283015460039093015460ff9092169290916001600160a01b031684565b336000908152602081905260409020805460ff1680156102f3575080600101544210155b6102fc57600080fd5b806002015482111561030d57600080fd5b6002810180548390039055604051339083156108fc029084906000818181858888f19350505050158015610345573d6000803e3d6000fd5b505050565b6203f48081565b336000908152602081905260409020805460ff16156103a15760405162461bcd60e51b81526004018080602001828103825260268152602001806106546026913960400191505060405180910390fd5b6003810180546001600160a01b0319166001600160a01b038416179055610297813461023b565b6103d06105fa565b5033600090815260208181526040918290208251608081018452815460ff16151581526001820154928101929092526002810154928201839052600301546001600160a01b031660608201529061042657600080fd5b61043381604001516102cf565b50565b336000908152602081905260409020805460ff16156104865760405162461bcd60e51b815260040180806020018281038252602b8152602001806106a1602b913960400191505060405180910390fd5b60008160020154116104df576040805162461bcd60e51b815260206004820152601b60248201527f73686f756c642068617665206578697374696e67207374616b65730000000000604482015290519081900360640190fd5b805460ff191660019081178255426203f48001910155565b336000908152602081905260409020805460ff166105465760405162461bcd60e51b815260040180806020018281038252602781526020018061067a6027913960400191505060405180910390fd5b805460ff19168155610433813461023b565b6000806105636105fa565b506001600160a01b03808416600090815260208181526040918290208251608081018452815460ff161580158252600183015493820193909352600282015493810193909352600301549092166060820152906105c75750600091508190506105f5565b60608101516000906001600160a01b03166105e35750836105ea565b5060608101515b604090910151925090505b915091565b6040518060800160405280600015158152602001600081526020016000815260200160006001600160a01b03168152509056fe73686f756c64206f6e6c7920616464207374616b657320696e206c6f636b656420737461746573686f756c64206f6e6c7920736574207369676e657220696e206c6f636b656420737461746573686f756c64206e6f74206c6f636b20616c72656164792d6c6f636b6564206163636f756e747373686f756c64206e6f7420756e6c6f636b20616c72656164792d756e6c6f636b6564206163636f756e7473a265627a7a72315820f2c044ad50ee08e7e49c575b49e8de27cac8322afdb97780b779aa1af44e40d364736f6c634300050b0032`
	NonReservedNativeTokenContractAddr     = "514b430000000000000000000000000000000002"
	NonReservedNativeTokenContractBytecode = `608060405234801561001057600080fd5b5060405161147b38038061147b8339818101604052604081101561003357600080fd5b50805160209091015160008054911515740100000000000000000000000000000000000000000260ff60a01b196001600160a01b039094166001600160a01b031990931692909217929092161790556001805460ff1916811790556113de8061009d6000396000f3fe6080604052600436106100f35760003560e01c806356e4b68b1161008a5780639ea41be7116100595780639ea41be7146103c1578063b187bd26146103f4578063b9ae736414610409578063fe67a54b1461041e576100f3565b806356e4b68b146102b85780635fc81df1146102e95780636aecd9d71461034c5780638556fed21461038c576100f3565b806332353fbd116100c657806332353fbd146102025780633c69e3d2146102175780633ccfd60b1461025c578063568f02f814610271576100f3565b806308bfc300146100f85780630f2dc31a146101595780631b8dca741461019457806327e235e3146101bd575b600080fd5b34801561010457600080fd5b5061010d610433565b604080516001600160801b03968716815294861660208601526001600160a01b03909316848401526001600160401b039091166060840152909216608082015290519081900360a00190f35b34801561016557600080fd5b506101926004803603604081101561017c57600080fd5b506001600160801b038135169060200135610493565b005b3480156101a057600080fd5b506101a96105fd565b604080519115158252519081900360200190f35b3480156101c957600080fd5b506101f0600480360360208110156101e057600080fd5b50356001600160a01b031661060d565b60408051918252519081900360200190f35b34801561020e57600080fd5b5061019261061f565b34801561022357600080fd5b506101926004803603606081101561023a57600080fd5b506001600160401b03813581169160208101358216916040909101351661069f565b34801561026857600080fd5b506101926107e9565b34801561027d57600080fd5b506102866108be565b604080516001600160801b0390941684526001600160401b039283166020850152911682820152519081900360600190f35b3480156102c457600080fd5b506102cd6108e9565b604080516001600160a01b039092168252519081900360200190f35b3480156102f557600080fd5b5061031c6004803603602081101561030c57600080fd5b50356001600160801b03166108f8565b604080516001600160401b0390941684526001600160a01b03909216602084015282820152519081900360600190f35b6101926004803603606081101561036257600080fd5b5080356001600160801b0390811691602081013590911690604001356001600160401b031661092d565b610192600480360360408110156103a257600080fd5b5080356001600160801b031690602001356001600160a01b0316610db2565b3480156103cd57600080fd5b5061031c600480360360208110156103e457600080fd5b50356001600160801b0316610e5b565b34801561040057600080fd5b506101a9610e99565b34801561041557600080fd5b50610192610ea2565b34801561042a57600080fd5b50610192610f0f565b60025460035460015460009283928392839283926001600160801b0380831693600160801b90930416916001600160a01b0390911690610100900463ffffffff1661047c6110bc565b939992985090965063ffffffff1694509092509050565b6001600160801b038216600090815260056020526040902080546001600160401b0316610507576040805162461bcd60e51b815260206004820152601760248201527f546f6b656e20494420646f65736e27742065786973742e000000000000000000604482015290519081900360640190fd5b8054600160401b90046001600160a01b031633146105565760405162461bcd60e51b81526004018080602001828103825260228152602001806112686022913960400191505060405180910390fd5b6001810180548301908190558211156105ab576040805162461bcd60e51b815260206004820152601260248201527120b23234ba34b7b71037bb32b9333637bb9760711b604482015290519081900360640190fd5b6105b361118a565b8154600160401b90046001600160a01b031681526001600160801b0384166020820152604081018390526000806060838264514b430004600019f16105f757600080fd5b50505050565b600054600160a01b900460ff1681565b60066020526000908152604090205481565b6000546001600160a01b0316331461067e576040805162461bcd60e51b815260206004820152601b60248201527f4f6e6c792073757065727669736f7220697320616c6c6f7765642e0000000000604482015290519081900360640190fd5b6106866110f1565b1561069357610693611132565b6001805460ff19169055565b6000546001600160a01b031633146106fe576040805162461bcd60e51b815260206004820152601b60248201527f4f6e6c792073757065727669736f7220697320616c6c6f7765642e0000000000604482015290519081900360640190fd5b600154600160681b90046001600160801b03161561074d5760405162461bcd60e51b81526004018080602001828103825260368152602001806111ef6036913960400191505060405180910390fd5b61012c6001600160401b038216116107965760405162461bcd60e51b815260040180806020018281038252602981526020018061133d6029913960400191505060405180910390fd5b600480546001600160801b03196001600160401b03948516600160801b0267ffffffffffffffff60801b19968616600160c01b026001600160c01b03909316929092179590951617939093169116179055565b6003546001600160a01b03163314156108335760405162461bcd60e51b81526004018080602001828103825260448152602001806113666044913960600191505060405180910390fd5b336000908152600660205260409020548061087f5760405162461bcd60e51b81526004018080602001828103825260218152602001806112ea6021913960400191505060405180910390fd5b336000818152600660205260408082208290555183156108fc0291849190818181858888f193505050501580156108ba573d6000803e3d6000fd5b5050565b6004546001600160801b038116906001600160401b03600160801b8204811691600160c01b90041683565b6000546001600160a01b031681565b600560205260009081526040902080546001909101546001600160401b03821691600160401b90046001600160a01b03169083565b60015460ff161561097a576040805162461bcd60e51b815260206004820152601260248201527120bab1ba34b7b71034b9903830bab9b2b21760711b604482015290519081900360640190fd5b6109826110f1565b156109a95761098f610f0f565b600154600160681b90046001600160801b0316156109a957fe5b600154600160681b90046001600160801b03166109ee57600180546fffffffffffffffffffffffffffffffff60681b1916600160681b426001600160801b0316021790555b621a5c73836001600160801b031611610a385760405162461bcd60e51b815260040180806020018281038252602f8152602001806112bb602f913960400191505060405180910390fd5b6001600160801b0383166000908152600560205260409020546001600160401b031615610aac576040805162461bcd60e51b815260206004820152601760248201527f546f6b656e20496420616c726561647920657869737473000000000000000000604482015290519081900360640190fd5b600154610100900463ffffffff166001600160401b03821614610b005760405162461bcd60e51b815260040180806020018281038252603181526020018061128a6031913960400191505060405180910390fd5b600454670de0b6b3a76400006001600160401b03600160c01b909204821602166001600160801b0383161015610b675760405162461bcd60e51b815260040180806020018281038252603281526020018061130b6032913960400191505060405180910390fd5b60045460025460646001600160801b03600160801b9283900481166001600160401b039390940492909216830282160490910181169083161015610bdc5760405162461bcd60e51b81526004018080602001828103825260438152602001806112256043913960600191505060405180910390fd5b33610be56111a8565b50604080516060810182526001600160801b03808716825285166020808301919091526001600160a01b0384168284018190526000908152600690915291909120543490810190811015610c75576040805162461bcd60e51b815260206004820152601260248201527120b23234ba34b7b71037bb32b9333637bb9760711b604482015290519081900360640190fd5b6001600160a01b03831660009081526006602090815260409091208290558201516001600160801b0316811015610cf3576040805162461bcd60e51b815260206004820152601a60248201527f4e6f7420656e6f7567682062616c616e636520746f206269642e000000000000604482015290519081900360640190fd5b81516002805460208501516001600160801b03908116600160801b029381166001600160801b031990921691909117169190911790556040820151600380546001600160a01b039092166001600160a01b0319909216919091179055600042610d5a6110bc565b03905061012c6001600160801b0382161015610da957600180546001600160401b0365010000000000808304821661012c86900301909116026cffffffffffffffff0000000000199091161790555b50505050505050565b6001600160801b038216600090815260056020526040902054600160401b90046001600160a01b03163314610e185760405162461bcd60e51b81526004018080602001828103825260268152602001806111c96026913960400191505060405180910390fd5b6001600160801b03909116600090815260056020526040902080546001600160a01b03909216600160401b02600160401b600160e01b0319909216919091179055565b6001600160801b0316600090815260056020526040902080546001909101546001600160401b03821692600160401b9092046001600160a01b031691565b60015460ff1690565b6000546001600160a01b03163314610f01576040805162461bcd60e51b815260206004820152601b60248201527f4f6e6c792073757065727669736f7220697320616c6c6f7765642e0000000000604482015290519081900360640190fd5b6001805460ff191681179055565b60015460ff1615610f5c576040805162461bcd60e51b815260206004820152601260248201527120bab1ba34b7b71034b9903830bab9b2b21760711b604482015290519081900360640190fd5b610f646110f1565b610fae576040805162461bcd60e51b815260206004820152601660248201527520bab1ba34b7b7103430b9903737ba1032b73232b21760511b604482015290519081900360640190fd5b6002546003546001600160a01b0316600090815260066020526040902054600160801b9091046001600160801b03161115610fe557fe5b60028054600380546001600160a01b0390811660009081526006602090815260408083208054600160801b9097046001600160801b0390811690970390558454875487168452600583528184208054600160401b600160e01b031916918616600160401b0291909117905586548616835291829020805467ffffffffffffffff1916426001600160401b031617905592549454815195909216855292169083015280517f64bb607a8887443bda664340203d98f768ced2874cab4af7c0f6d913aabcf1559281900390910190a16110ba611132565b565b600154600454600160681b82046001600160801b039081169116016001600160401b0365010000000000909204919091160190565b60006110fb6110bc565b6001600160801b0316426001600160801b03161015801561112d5750600154600160681b90046001600160801b031615155b905090565b600280546001600160801b03169055600380546001600160a01b03191690556001805463ffffffff61010065010000000000600160e81b031983168190048216840190911602610100600160e81b0319909116179055565b60405180606001604052806003906020820280388339509192915050565b60408051606081018252600080825260208201819052918101919091529056fe4f6e6c7920746865206f776e65722063616e207472616e73666572206f776e6572736869702e41756374696f6e2073657474696e672063616e6e6f74206265206d6f646966696564207768656e206974206973206f6e676f696e672e4269642070726963652073686f756c64206265206c6172676572207468616e2063757272656e74206869676865737420626964207769746820696e6372656d656e742e4f6e6c7920746865206f776e65722063616e206d696e74206e657720746f6b656e2e54617267657420726f756e64206f662061756374696f6e2068617320656e646564206f72206e6f7420737461727465642e546865206c656e677468206f6620746f6b656e206e616d65204d555354206265206c6172676572207468616e20342e4e6f2062616c616e636520617661696c61626c6520746f2077697468647261772e4269642070726963652073686f756c64206265206c6172676572207468616e206d696e696d756d206269642070726963652e4475726174696f6e2073686f756c64206265206c6f6e676572207468616e2035206d696e757465732e48696768657374206269646465722063616e6e6f742077697468647261772062616c616e63652074696c6c2074686520656e64206f6620746869732061756374696f6e2ea265627a7a72315820d7bc3b4b084ed3260256e170d3d4bab64072ca493f11fa4b1588e1e424b43e7864736f6c634300050b0032`
	currentMntIDAddr                       = "000000000000000000000000000000514b430001"
	transferMntAddr                        = "000000000000000000000000000000514b430002"
	deploySystemContractAddr               = "000000000000000000000000000000514b430003"
	mintMNTAddr                            = "000000000000000000000000000000514b430004"
	currentMntIDGas                        = uint64(3)
	transferMntGas                         = uint64(3)
	deployRootChainPoSWStakingContractGas  = uint64(3)
	mintMNTRet                             = common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")

	SystemContracts = []SystemContract{
		{},
		{
			address:   common.HexToAddress(RootChainPoSWContractAddr),
			bytecode:  common.Hex2Bytes(rootChainPoSWContractBytecode),
			timestamp: new(uint64),
		}, {
			address:   common.HexToAddress(NonReservedNativeTokenContractAddr),
			bytecode:  common.Hex2Bytes(NonReservedNativeTokenContractBytecode),
			timestamp: nil,
		},
	}
)

const (
	ROOT_CHAIN_POSW = iota + 1
	NON_RESERVED_NATIVE_TOKEN
)

type SystemContract struct {
	address   common.Address
	bytecode  []byte
	timestamp *uint64
}

func (s *SystemContract) Address() common.Address {
	return s.address
}

func (s *SystemContract) SetTimestamp(timestamp *uint64) {
	s.timestamp = timestamp
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
	common.BytesToAddress([]byte{1}):              &ecrecover{},
	common.BytesToAddress([]byte{2}):              &sha256hash{},
	common.BytesToAddress([]byte{3}):              &ripemd160hash{},
	common.BytesToAddress([]byte{4}):              &dataCopy{},
	common.BytesToAddress([]byte{5}):              &bigModExp{},
	common.BytesToAddress([]byte{6}):              &bn256Add{},
	common.BytesToAddress([]byte{7}):              &bn256ScalarMul{},
	common.BytesToAddress([]byte{8}):              &bn256Pairing{},
	common.HexToAddress(currentMntIDAddr):         &currentMntID{},
	common.HexToAddress(transferMntAddr):          &transferMnt{},
	common.HexToAddress(deploySystemContractAddr): &deploySystemContract{},
	common.HexToAddress(mintMNTAddr):              &mintMNT{math2.MaxUint64},
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

type deploySystemContract struct {
	enableTime uint64
}

func (c *deploySystemContract) GetEnableTime() uint64 {
	return c.enableTime
}
func (c *deploySystemContract) SetEnableTime(data uint64) {
	c.enableTime = data
}

func (r *deploySystemContract) RequiredGas(input []byte) uint64 {
	return deployRootChainPoSWStakingContractGas
}

func (r *deploySystemContract) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	data := getData(input, 0, 32)
	index := new(big.Int).SetBytes(data).Uint64()
	contractIndex := uint64(1)
	if index > 0 {
		contractIndex = index
	}
	if int(contractIndex) >= len(SystemContracts) {
		return nil, nil
	}
	targetAddr := SystemContracts[contractIndex].Address()
	bytecode := SystemContracts[contractIndex].bytecode
	// Use predetermined contract address
	_, addr, leftover, err := evm.Create(contract.self, bytecode, contract.Gas, big.NewInt(0), &targetAddr)
	if err != nil {
		return nil, err
	}
	contract.Gas = leftover
	return addr.Bytes(), nil
}

type mintMNT struct {
	enableTime uint64
}

func (m *mintMNT) GetEnableTime() uint64 {
	return m.enableTime
}
func (m *mintMNT) SetEnableTime(data uint64) {
	m.enableTime = data
}
func (m *mintMNT) RequiredGas(input []byte) uint64 {
	return qkcParams.GCallValueTransfer.Uint64()
}

//# 3 inputs: (minter, token ID, amount)
func (m *mintMNT) Run(input []byte, evm *EVM, contract *Contract) ([]byte, error) {
	minterBytes := getData(input, 0, 32)
	minter := common.BytesToAddress(minterBytes)
	mntBytes := getData(input, 32, 32)
	mnt := new(big.Int).SetBytes(mntBytes)
	valueBytes := getData(input, 64, 32)
	value := new(big.Int).SetBytes(valueBytes)
	if big.NewInt(0).Cmp(value) == 0 {
		contract.Gas = 0
		return nil, nil
	}
	if !evm.StateDB.Exist(minter) && !contract.UseGas(params.CallNewAccountGas) {
		contract.Gas = 0
		return nil, ErrOutOfGas
	}
	allowedSender := SystemContracts[NON_RESERVED_NATIVE_TOKEN].Address()
	//  # Only system contract has access to minting new token
	if !bytes.Equal(allowedSender.Bytes(), contract.CallerAddress.Bytes()) {
		contract.Gas = 0
		return nil, core.ErrInvalidSender
	}
	evm.StateDB.AddBalance(minter, value, mnt.Uint64())
	return mintMNTRet, nil
}
