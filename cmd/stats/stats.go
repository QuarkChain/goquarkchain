package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/shirou/gopsutil/mem"
	"github.com/ybbus/jsonrpc"
	"math/big"
	"runtime"
	"strings"
	"time"
)

func basic(clt jsonrpc.RPCClient, ip string) string {

	response, err := clt.Call("getStats")
	if err != nil {
		return err.Error()
	}
	if response.Error != nil {
		return response.Error.Error()
	}
	res := response.Result.(map[string]interface{})
	//fmt.Println("response", res)

	msg := "============================\n"
	msg += "QuarkChain Cluster Stats\n"
	msg += "============================\n"
	msg += fmt.Sprintf("CPU:                %d\n", runtime.NumCPU())
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)
	v, _ := mem.VirtualMemory()
	msg += fmt.Sprintf("Memory:             %d GB\n", v.Total/1024/1024/1024)
	msg += fmt.Sprintf("IP:                 %s\n", ip)
	chains, _ := res["chainSize"].(json.Number).Int64()
	msg += fmt.Sprintf("Chains:             %d\n", chains)
	networkId, _ := res["networkId"].(json.Number).Int64()
	msg += fmt.Sprintf("Network Id:         %d\n", networkId)
	peersI := res["peers"].([]interface{})
	peers := make([]string, len(peersI))
	for i, p := range peersI {
		peers[i] = p.(string)
	}
	msg += fmt.Sprintf("Peers:              %s\n", strings.Join(peers, ","))
	msg += "============================"
	return msg
}

func queryStats(client jsonrpc.RPCClient, interval *uint, shards bool) {
	titles := []string{"Timestamp\t", "Syncing", "TPS", "Pend.TX", "Conf.TX", "BPS", "SBPS", "CPU", "ROOT"}
	if shards {
		titles = append(titles, "CHAIN/SHARD-HEIGHT")
	}
	fmt.Println(strings.Join(titles, "\t"))
	intv := time.Duration(*interval)
	ticker := time.NewTicker(intv * time.Second)
	fmt.Println(stats(client, shards))
	for {
		select {
		case <-ticker.C:
			fmt.Println(stats(client, shards))
		}
	}
}

func stats(client jsonrpc.RPCClient, shardsIn bool) string {
	response, err := client.Call("getStats")
	if err != nil {
		return err.Error()
	}
	if response.Error != nil {
		return response.Error.Error()
	}
	res := response.Result.(map[string]interface{})
	t := time.Now()
	msg := t.Format("2006-01-02 15:04:05")
	msg += "\t"
	msg += fmt.Sprintf("%t", res["syncing"])
	msg += "\t"
	txCount, _ := res["txCount60s"].(json.Number).Int64()
	msg += fmt.Sprintf("%2.2f", float64(txCount/60))
	msg += "\t"
	pendingTxCount, _ := res["pendingTxCount"].(json.Number).Int64()
	msg += fmt.Sprintf("%d", pendingTxCount)
	msg += "\t"
	totalTxCount, _ := res["totalTxCount"].(json.Number).Int64()
	msg += fmt.Sprintf("%d", totalTxCount)
	msg += "\t"
	blockCount60s, _ := res["blockCount60s"].(json.Number).Float64()
	msg += fmt.Sprintf("%2.2f", blockCount60s/60)
	msg += "\t"
	staleBlockCount60s, _ := res["staleBlockCount60s"].(json.Number).Float64()
	msg += fmt.Sprintf("%2.2f", staleBlockCount60s/60)
	msg += "\t"
	cpuf := res["cpus"].([]interface{})
	var total float64
	for _, p := range cpuf {
		n, _ := p.(json.Number).Float64()
		total += n
	}
	mean := total / float64(len(cpuf))
	msg += fmt.Sprintf("%2.2f", mean)

	msg += "\t"
	rh, _ := res["rootHeight"].(json.Number).Int64()
	msg += fmt.Sprintf("%d", rh)

	if shardsIn {
		msg += "\t"
		shardsi := res["shards"].([]interface{})
		shards := make([]string, len(shardsi))
		for i, p := range shardsi {
			shard := p.(map[string]interface{})
			shards[i] = fmt.Sprintf("%s/%s-%s", shard["chainId"], shard["shardId"], shard["height"])
		}
		msg += strings.Join(shards, " ")
	}
	return msg
}

func queryAddress(client jsonrpc.RPCClient, interval *uint, address, token *string) {
	addr := *address
	if strings.HasPrefix(addr, "0x") {
		addr = addr[2:]
	}
	if len(addr) != 48 {
		fmt.Printf("Err: invalid address %x\n", address)
		return
	}
	fmt.Printf("Querying balances for 0x%s\n", addr)
	titles := []string{"Timestamp\t", "Total", fmt.Sprintf("Shards (%s)", *token)}
	fmt.Println(strings.Join(titles, "\t"))

	queryBalance(client, addr, *token)
	intv := time.Duration(*interval)
	ticker := time.NewTicker(intv * time.Second)
	for {
		select {
		case <-ticker.C:
			queryBalance(client, addr, *token)
		}
	}
}

func queryBalance(client jsonrpc.RPCClient, addr, token string) {
	accBytes, err := hexutil.Decode("0x" + addr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	acc, err := account.CreatAddressFromBytes(accBytes)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	acc.FullShardKey = 0
	response, err := client.Call("getAccountData", acc, nil, true)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if response.Error != nil {
		fmt.Println(response.Error.Error())
		return
	}
	res := response.Result.(map[string]interface{})
	shardsi := res["shards"].([]interface{})
	shardsQKCStr := make([]string, len(shardsi))
	for i := range shardsQKCStr {
		shardsQKCStr[i] = "0"
	}
	total := big.NewInt(0)
	for _, p := range shardsi {
		shardMap := p.(map[string]interface{})
		balanceMaps := shardMap["balances"].([]interface{})
		if len(balanceMaps) > 0 {
			for _, s := range balanceMaps {
				balanceMap := s.(map[string]interface{})
				tokenStr := balanceMap["tokenStr"].(string)
				if strings.Compare(strings.ToUpper(tokenStr), strings.ToUpper(token)) == 0 {
					balanceWei, _ := new(big.Int).SetString(balanceMap["balance"].(string)[2:], 16)
					total = total.Add(total, balanceWei)
					balance := balanceWei.Div(balanceWei, big.NewInt(1000000000000000000))
					id := hexutil.MustDecodeUint64(shardMap["chainId"].(string))
					shardsQKCStr[id] = balance.String()
				}
			}
		}
	}
	total = total.Div(total, big.NewInt(1000000000000000000))
	shardsQKCs := strings.Join(shardsQKCStr, ", ")
	t := time.Now()
	msg := t.Format("2006-01-02 15:04:05")
	msg += "\t"
	msg += total.String()
	msg += "\t"
	msg += shardsQKCs
	fmt.Println(msg)
}

func main() {

	ip := flag.String("ip", "localhost", "Cluster IP")
	prv_port := flag.Int("prv_port", 38491, "Private service port")
	pub_port := flag.Int("pub_port", 38391, "Public service port")
	interval := flag.Uint("i", 10, "Query interval in second")
	address := flag.String("a", "", "Query account balance if a QKC address is provided")
	token := flag.String("t", "QKC", "Query account balance for a specific token")
	shards := flag.Bool("s", false, "Query height of all shards")
	flag.Parse()
	privateEndPoint := jsonrpc.NewClient(fmt.Sprintf("http://%s:%v", *ip, *prv_port))
	publicEndPoint := jsonrpc.NewClient(fmt.Sprintf("http://%s:%v", *ip, *pub_port))
	fmt.Println(basic(privateEndPoint, *ip))
	if len(*address) > 0 {
		queryAddress(publicEndPoint, interval, address, token)
	} else {
		queryStats(privateEndPoint, interval, *shards)
	}
}
