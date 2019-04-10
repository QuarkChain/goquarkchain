package downloader

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

// MinorDownloader minor downloader
type MinorDownloader struct {
	shardConn    *FakeSlaveConn
	shardState   *FakeShardState
	shard        *FakeShard
	maxStaleness uint64
	logInfo      string

	//channel
	minorHeaderCh chan dataPack
	minorBlockCh   chan dataPack
}

// NewMinorDownloader new minor downloader
func NewMinorDownloader() *MinorDownloader {
	d := &MinorDownloader{
		minorHeaderCh: make(chan dataPack, 1),
		minorBlockCh:   make(chan dataPack, 1),
	}
	return d
}


func (m *MinorDownloader) downloadMinorBlockHeaders(p *peerConnection, blockHash common.Hash, branch uint32) []*types.MinorBlockHeader {
	p.peer.RequestMinorHeadersByHash(blockHash, 0, branch, 1, false)
	for {
		select {
		case packet := <-m.minorHeaderCh:
			header := packet.(*minorHeaderPack)
			return header.headers.BlockHeaderList
		}
	}
}

func (m *MinorDownloader) downloadMinorBlock(p *peerConnection, data []*types.MinorBlockHeader, branch uint32) []*types.MinorBlock {
	hashList := make([]common.Hash, 0)
	for _, v := range data {
		hashList = append(hashList, v.Hash())
	}
	p.peer.RequestMinorBlocks(hashList, branch)
	for {
		select {
		case packet := <-m.minorBlockCh:
			bodys := packet.(*minorBlockPack)
			return bodys.blockList

		}
	}
}

// DeliverMinorHeaders deliver minor headers
func (m *MinorDownloader) DeliverMinorHeaders(id string, headers p2p.GetMinorBlockHeaderListResponse) (err error) {
	return m.deliver(id, m.minorHeaderCh, &minorHeaderPack{id, headers}, headerInMeter, headerDropMeter)
}

//DeliverMinorBlocks deliver minor bodies
func (m *MinorDownloader) DeliverMinorBlocks(id string, bodys p2p.GetMinorBlockListResponse) (err error) {
	return m.deliver(id, m.minorBlockCh, &minorBlockPack{id, bodys.MinorBlockList}, bodyInMeter, bodyDropMeter)
}


func (m *MinorDownloader) deliver(id string, destCh chan dataPack, packet dataPack, inMeter, dropMeter metrics.Meter) (err error) {
	// Update the delivery metrics for both good and failed deliveries
	inMeter.Mark(int64(packet.Items()))
	defer func() {
		if err != nil {
			dropMeter.Mark(int64(packet.Items()))
		}
	}()

	select {
	case destCh <- packet:
		return nil
	}
}


// SyncWithPeer 开始同步指定header
func (m *MinorDownloader) SyncWithPeer(p *peerConnection, header *types.MinorBlockHeader,branch uint32) error {
	if m.hasBlockHash(header.Hash()) {
		return errors.New("has")
	}
	BlockHeaderChain := make([]*types.MinorBlockHeader, 0)
	BlockHeaderChain=append(BlockHeaderChain,header)

	for !m.check(BlockHeaderChain) {
		header := BlockHeaderChain[len(BlockHeaderChain)-1]
		blockHash := header.ParentHash
		//height := header.Number - 1

		//TODO implement py
		//if m.shardState.HeaderTip.Number-height < m.maxStaleness {
		//	log.Warn(m.logInfo, "abort tipNumber", m.shardState.HeaderTip.Number, "height", height)
		//	return errors.New("nem>statleness")
		//}

		//if not self.shard_state.db.contain_root_block_by_hash(
		//	block_header_chain[-1].hash_prev_root_block
		//):
		//	return
		log.Info(m.logInfo, "downloader header from branch",  blockHash.Hex())
		blockHeaderResp := m.downloadMinorBlockHeaders(p,blockHash,branch)

		//TODO: check data .need delete
		Headers:=blockHeaderResp
		fmt.Println("====Headers展示开始", "len", len(Headers), "Branch", branch)
		for _, vv := range Headers {
			fmt.Println(vv.Branch.Value, vv.Number, vv.Hash().String())
		}
		fmt.Println("====Headers展示结束")



		log.Info(m.logInfo, "download succ len headers", len(blockHeaderResp))

		if err := m.validateBlockHeaders(blockHeaderResp); err != nil {
			return err
		}

		for _, header := range blockHeaderResp {
			if m.hasBlockHash(header.Hash()) {
				break
			}
			BlockHeaderChain = append(BlockHeaderChain, header)
		}
		break
	}

	for len(BlockHeaderChain) > 0 {
		bodys := m.downloadMinorBlock(p,BlockHeaderChain,branch)

		//TODO check data need delete
		fmt.Println("====Blocks展示开始", "len", len(bodys), "Branch", branch)
		for _, vv := range bodys {
			fmt.Println(vv.Branch().Value, vv.Number(), vv.Hash().String())
		}
		fmt.Println("===Blocks展示结束")

		for _, v := range bodys {
			// if contain
			m.shard.AddBlock(v)

			if len(BlockHeaderChain) >= 2 {
				BlockHeaderChain = BlockHeaderChain[1:]
			} else {
				BlockHeaderChain = make([]*types.MinorBlockHeader, 0)
			}
		}
		break
	}
	return nil
}

func (m *MinorDownloader) validateBlockHeaders(blockHeaderList []*types.MinorBlockHeader) error {
	//for index := 0; index < len(blockHeaderList)-1; index++ {
	//	header := blockHeaderList[index]
	//	prev := blockHeaderList[index+1]
	//	if header.Number != prev.Number+1 {
	//		return errors.New("number is short")
	//	}
	//	if header.PrevRootBlockHash != prev.Hash() {
	//		return errors.New("hash is not match")
	//	}
	//	//shardID:=header.Branch.GetShardID()
	//	//根据shardID判断consense
	//	//验证
	//}
	return nil
}

func (m *MinorDownloader) check(headerChain []*types.MinorBlockHeader) bool {
	header := headerChain[len(headerChain)-1]
	blockHash := header.PrevRootBlockHash
	log.Error("TODO check MinorDownloader", "blockHash", blockHash)
	return false
	//return m.shardState.db.contain_minot_block_by_hash(block_hash)
}

func (m *MinorDownloader) hasBlockHash(hash common.Hash) bool {
	return false
}

