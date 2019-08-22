package tests

import (
	"strings"
)

var (
	skipModule = []string{
		"stQuadraticComplexityTest",
		"stMemoryStressTest",
		"MLOAD_Bounds.json",
		"failed_tx_xcf416c53",
		"RevertDepthCreateAddressCollision.json",
		"pairingTest.json",
		"createJS_ExampleContract",
		"static_CallEcrecoverR_prefixed0.json",
		"CallEcrecoverR_prefixed0.json",
		"CALLCODEEcrecoverR_prefixed0.json",
		"static_CallEcrecover80.json",
		"CallEcrecover80.json",
		"CALLCODEEcrecover80.json",
	}
	mapForks = map[string]bool{
		"ConstantinopleFix": true,
	}
)

func isSkip(str string) bool {
	for _, v := range skipModule {
		ans := strings.Index(str, v)
		if ans != -1 {
			return true
		}
	}
	return false
}
