package tests

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
		"Byzantium": false,
	}
	//pyDataPath="./testdata/pyData/new.log"
	pyDataPath = "./testdata/pyData/all_py_data.txt"
	pyData     = prePythonData()
)

func lineData(str string) (string, string) {
	return str[0:66], str[67:]
}

func prePythonData() map[string]map[string]string {
	fmt.Println("start")
	preString := ""
	ans := make(map[string]map[string]string)
	f, err := os.Open(pyDataPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadString('\n') //以'\n'为结束符读入一行

		line = strings.TrimSpace(line)

		if err != nil || io.EOF == err {
			break
		}
		line = line[15:]
		//	temp:=line
		//	fmt.Println("line",temp,len(temp))
		switch line[0] {
		case '0':
			first, second := lineData(line)
			if _, ok := ans[preString]; ok == false {
				ans[preString] = make(map[string]string)
			}
			ans[preString][second] = first
			//fmt.Println("preString",preString,len(preString),"second",len(second),second,"first",len(first),first)
		default:
			preString = line
		}
	}
	//fmt.Println("end")
	return ans
}

func isSkip(str string) bool {
	for _, v := range skipModule {
		ans := strings.Index(str, v)
		if ans != -1 {
			return true
		}
	}
	return false
}
