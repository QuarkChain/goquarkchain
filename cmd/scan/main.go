package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ybbus/jsonrpc"
)

var (
	lock         sync.Mutex
	wg           sync.WaitGroup
	c            = jsonrpc.NewClient("http://jrpc.mainnet.quarkchain.io:38391")
	startNumbers = []uint64{17993600, 17938600, 17978900, 18033200, 18058600, 17966100, 17913300, 17853900}
        endNumbers   = []uint64{16916500, 16865500, 16904500, 16956300, 16984500, 16889600, 16845800, 16758900}
//	endNumbers       = []uint64{15212800, 15162000, 15199000, 15255000, 15285000, 15189000, 15168000, 15026000}
	dailyTXs         = make(map[string]int)
	txCount          = 0
	dailyActiveUsers = make(map[string]map[common.Address]bool)
	activeUsers      = make(map[common.Address]bool)
	// startTime = uint64(time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC).Unix())
	startTime = uint64(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC).Unix())
	endTime   = uint64(time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC).Unix())
)

func main() {
	for sid, startNumber := range startNumbers {
            id, sn := uint64(sid), startNumber
            go func(id, sn uint64){
                wg.Add(1)
		defer wg.Done()
		for {
                    scanMinorBlocks(id, sn, sn-5000)
                    sn = sn - 5000
                    if sn < endNumbers[id] {
                        break
                    }
		    PrintState() 
                }
            }(id, sn)
        }

	time.Sleep(time.Second)
	wg.Wait()
	PrintState()
}

func scanMinorBlocks(shardId, startBlockNumber, endBlockNumber uint64) {
	fmt.Printf("start scan shard %d, start %d, end %d\n", shardId, startBlockNumber, endBlockNumber)
	bn := startBlockNumber
	for {
		if bn%1000 == 0 {
			fmt.Printf("%v process %d number %d\n", time.Now().Format("2006-01-02 15:04:05"), shardId, bn)
		}
		resp, err := c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(shardId<<16), hexutil.EncodeUint64(bn), true, false)
		if err != nil {
			fmt.Printf("shard %d height %d error: %s\n", shardId, bn, err.Error())
			continue
		}
		if resp.Error != nil {
			fmt.Printf("shard %d height %d error: %s\n", shardId, bn, resp.Error.Error())
			continue
		}
		if resp.Result == nil {
			fmt.Printf("shard %d height %d error: empty result\n\n", shardId, bn)
			continue
		}

		timestampStr := resp.Result.(map[string]interface{})["timestamp"].(string)
		timestamp, _ := hexutil.DecodeUint64(timestampStr)
		dateStr := time.Unix(int64(timestamp), 0).Format("2006-01-02")

		if timestamp > endTime {
			bn--
			continue
		}
		if timestamp < startTime {
			fmt.Printf("shard %d done, height %d, time %s, start time %d\n", shardId, bn, time.Unix(int64(timestamp), 0).String(), startTime)
			break
		}
		lock.Lock()
		if _, ok := dailyTXs[dateStr]; !ok {
			dailyTXs[dateStr] = 0
		}
		if _, ok := dailyActiveUsers[dateStr]; !ok {
			dailyActiveUsers[dateStr] = make(map[common.Address]bool)
		}
		lock.Unlock()

		transactions := resp.Result.(map[string]interface{})["transactions"].([]interface{})
		if len(transactions) > 0 {
			lock.Lock()
			dailyTXs[dateStr] = dailyTXs[dateStr] + len(transactions)
			txCount += len(transactions)
			for _, tx := range transactions {
				pros := tx.(map[string]interface{})
				from := common.HexToAddress(pros["from"].(string))
				activeUsers[from] = true
				dailyActiveUsers[dateStr][from] = true
			}
			lock.Unlock()
		}
		bn--
		if endBlockNumber > bn {
			break
		}
	}

}

func PrintState() {
	fmt.Println("total tx count", txCount)
	for date, cnt := range dailyTXs {
		fmt.Println(date, cnt)
	}

	fmt.Println("total active user", len(activeUsers))
	for date, au := range dailyActiveUsers {
		fmt.Println(date, len(au))
	}
}

