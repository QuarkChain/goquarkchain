// Copyright 2015 The go-ethereum Authors
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

package tests

import (
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/QuarkChain/goquarkchain/core/vm"
)

func TestState(t *testing.T) {
	t.Parallel()
	testEVMStateWithFeature(t, qkcStateTestDir, false)
	testEVMStateWithFeature(t, ethStateTestDir, true)
}
func testEVMStateWithFeature(t *testing.T, dir string, useMock bool) {
	st := new(testMatcher)
	// Broken tests:
	st.skipLoad(`^stTransactionTest/OverflowGasRequire\.json`) // gasLimit > 256 bits
	st.skipLoad(`^stTransactionTest/zeroSigTransa[^/]*\.json`) // EIP-86 is not supported yet

	// Expected failures:
	st.fails(`^stRevertTest/RevertPrecompiledTouch(_storage)?\.json/ConstantinopleFix/0`, "bug in test")
	st.fails(`^stRevertTest/RevertPrecompiledTouch(_storage)?\.json/ConstantinopleFix/3`, "bug in test")
	st.walk(t, dir, func(t *testing.T, name string, test *StateTest) {
		for _, subtest := range test.Subtests() {
			subtest := subtest
			subtest.Path = name
			key := fmt.Sprintf("%s/%d", subtest.Fork, subtest.Index)
			name := name + "/" + key
			t.Run(key, func(t *testing.T) {
				withTrace(t, test.gasLimit(subtest), useMock, func(vmconfig vm.Config, useMock bool) error {
					_, err := test.Run(subtest, vmconfig, useMock)
					if err == errors.New("not support") {
						return nil
					}
					return st.checkFailure(t, name, err)
				})
			})
		}
	})
}

// Transactions with gasLimit above this value will not get a VM trace on failure.
const traceErrorLimit = 4000000

// The VM config for state tests that accepts --vm.* command line arguments.
var testVMConfig = func() vm.Config {
	vmconfig := vm.Config{}
	//flag.StringVar(&vmconfig.EVMInterpreter, utils.EVMInterpreterFlag.Name, utils.EVMInterpreterFlag.Value, utils.EVMInterpreterFlag.Usage)
	//flag.StringVar(&vmconfig.EWASMInterpreter, utils.EWASMInterpreterFlag.Name, utils.EWASMInterpreterFlag.Value, utils.EWASMInterpreterFlag.Usage)
	testing.Init()
	flag.Parse()
	return vmconfig
}()

func withTrace(t *testing.T, gasLimit uint64, useMock bool, test func(vm.Config, bool) error) {
	err := test(testVMConfig, useMock)
	if err == nil {
		return
	}
	t.Error(err)
	if gasLimit > traceErrorLimit {
		t.Log("gas limit too high for EVM trace")
		return
	}
	//tracer := vm.NewStructLogger(nil)
	return
	//err2 := test(vm.Config{Debug: true, Tracer: tracer})
	//if !reflect.DeepEqual(err, err2) {
	//	t.Errorf("different error for second run: %v", err2)
	//}
	//buf := new(bytes.Buffer)
	//vm.WriteTrace(buf, tracer.StructLogs())
	//if buf.Len() == 0 {
	//	t.Log("no EVM operation logs generated")
	//} else {
	//	t.Log("EVM operation log:\n" + buf.String())
	//}
	//t.Logf("EVM output: 0x%x", tracer.Output())
	//t.Logf("EVM error: %v", tracer.Error())
}
