package downloader

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

// dataPack is a data message returned by a peer for some query.
type dataPack interface {
	PeerID() string
	Items() int
	Stats() string
}

// rootHeaderPack is a batch of block headers returned by a peer.
type rootHeaderPack struct {
	peerID  string
	headers p2p.GetRootBlockHeaderListResponse
}

func (p *rootHeaderPack) PeerID() string { return p.peerID }
func (p *rootHeaderPack) Items() int     { return len(p.headers.BlockHeaderList) }
func (p *rootHeaderPack) Stats() string  { return fmt.Sprintf("%d", len(p.headers.BlockHeaderList)) }

// rootHeaderPack is a batch of block headers returned by a peer.
type minorHeaderPack struct {
	peerID  string
	headers p2p.GetMinorBlockHeaderListResponse
}

func (p *minorHeaderPack) PeerID() string { return p.peerID }
func (p *minorHeaderPack) Items() int     { return len(p.headers.BlockHeaderList) }
func (p *minorHeaderPack) Stats() string  { return fmt.Sprintf("%d", len(p.headers.BlockHeaderList)) }

// rootBlockPack is a batch of block bodies returned by a peer.
type rootBlockPack struct {
	peerID    string
	blockList []*types.RootBlock
}

func (p *rootBlockPack) PeerID() string { return p.peerID }
func (p *rootBlockPack) Items() int {
	return len(p.blockList)
}
func (p *rootBlockPack) Stats() string { return fmt.Sprintf("%d", len(p.blockList)) }

type minorBlockPack struct {
	peerID    string
	blockList []*types.MinorBlock
}

func (p *minorBlockPack) PeerID() string { return p.peerID }
func (p *minorBlockPack) Items() int {
	return len(p.blockList)
}
func (p *minorBlockPack) Stats() string { return fmt.Sprintf("%d", len(p.blockList)) }

// RootDownloader root blockchain download
type RootDownloader struct {
	Header          *types.RootBlockHeader
	MinorDownloader *MinorDownloader

	MasterService *FakeMasterServer

	//TODO need use real rootState
	//FakeRootState
	RootState    FakeRootState
	MaxStaleness uint32
	logInfo      string

	fakeMapNumber map[common.Hash]bool

	// Channels
	rootHeaderCh  chan dataPack
	rootBlockCh    chan dataPack

}

// NewRootDownloader new root downloader
func NewRootDownloader() *RootDownloader {
	return &RootDownloader{
		rootHeaderCh:  make(chan dataPack, 1),
		rootBlockCh:    make(chan dataPack, 1),
		MasterService: &FakeMasterServer{},
		fakeMapNumber: make(map[common.Hash]bool),
	}
}

//SetFakeMinorDownLoader test for minor block download
func (r *RootDownloader)SetFakeMinorDownLoader(minorDownloader *MinorDownloader){
	r.MinorDownloader =minorDownloader
}

func (r *RootDownloader) check(headerList []*types.RootBlockHeader) bool {
	header := headerList[len(headerList)-1]
	if header.Number == 0 {
		return true
	}
	if r.hasBlockHash(header.Hash()) {
		return false
	}
	return false
}

// SyncWithPeer sync with peer
func (r *RootDownloader) SyncWithPeer(id string, version int, pp Peer, header *types.RootBlockHeader) error {
	r.Header = header
	p := newPeerConnection(id, version, pp, nil)
	if r.hasBlockHash(r.Header.Hash()) {
		return nil
	}
	BlockHeaderChain := make([]*types.RootBlockHeader, 0)
	BlockHeaderChain = append(BlockHeaderChain, header)

	for !r.check(BlockHeaderChain) {
		header := BlockHeaderChain[len(BlockHeaderChain)-1]
		blockHash := header.ParentHash
		height := header.Number - 1

		if r.RootState.Tip.Number-height > r.MaxStaleness {
			log.Error(r.logInfo, "abort syncing due to forking at super old block now height", height, "tipHeight", r.RootState.Tip.Number)
		}
		log.Info(r.logInfo, "downloading block header list from height", height, "hash", r.RootState.Tip.Number)
		blockHeaderList := r.downloadRootBlockHeaders(p, blockHash)
		fmt.Println("====开始展示root block Header", len(blockHeaderList))
		for _, v := range blockHeaderList {
			fmt.Println(v.Hash().String(), v.Number)
		}
		fmt.Println("====展示root block Header 结束")
		for _, v := range blockHeaderList {
			if r.hasBlockHash(v.Hash()) {
				break
			}
			BlockHeaderChain = append(BlockHeaderChain, v)
		}
		break
	}

	log.Info(r.logInfo, "download block start", BlockHeaderChain[0].Number, "end", BlockHeaderChain[len(BlockHeaderChain)-1].Number)
	for len(BlockHeaderChain) > 0 {
		bodys := r.downloadRootBlock(p, BlockHeaderChain) ///下载区块
		log.Info(r.logInfo, "downloaded block from", bodys[0].Number(), "end", bodys[len(bodys)-1].Number())

		fmt.Println("====开始展示root bodys", len(bodys))
		for _, v := range bodys {
			fmt.Println(v.Number(), v.Hash().String())
			for _, vv := range v.MinorBlockHeaders() {
				fmt.Println("***branch", vv.Branch.Value, "Number", vv.Number, "Hash", vv.Hash().String())
			}
		}
		fmt.Println("====展示root bodys 结束")
		for _, v := range bodys {
			r.addBlock(v)
			if len(BlockHeaderChain) >= 2 {
				BlockHeaderChain = BlockHeaderChain[1:]
			} else {
				BlockHeaderChain = make([]*types.RootBlockHeader, 0)
			}
		}
		//delete it for minor download test
		r.testMinor(p, bodys)
	}
	return nil
}

func (r *RootDownloader) addBlock(block *types.RootBlock) {
	r.syncMinorBlocks(block.MinorBlockHeaders())
	r.MasterService.AddRootBlock(block)
}

func (r *RootDownloader) syncMinorBlocks(blockHeaderList types.MinorBlockHeaders) {
	minorBlockDownloadMap := make(map[account.Branch][]common.Hash)
	for _, mBlockHeader := range blockHeaderList {
		mBlockHash := mBlockHeader.Hash()
		if !r.RootState.IsMinorBlockValidated(mBlockHash) {
			if _, ok := minorBlockDownloadMap[mBlockHeader.Branch]; ok == false {
				minorBlockDownloadMap[mBlockHeader.Branch] = make([]common.Hash, 0)
			}
			minorBlockDownloadMap[mBlockHeader.Branch] = append(minorBlockDownloadMap[mBlockHeader.Branch], mBlockHash)
		}
	}

	for branch, mBlockHashList := range minorBlockDownloadMap {
		//slaveConn := r.FakeMasterServer.GetSlaveConnection(branch).SyncMinorBlockList(cluster)
		//to download
		fmt.Println("branch:", branch, "mBlockHashList:", mBlockHashList)
	}

	//master_server.update_shard_stats(result.shard_stats)

	for _, mHeader := range blockHeaderList {
		r.RootState.AddValidatedMinorBlockHash(mHeader.Hash())
	}

}

func (r *RootDownloader) validateBlockHeaders(blockHeaderList []types.RootBlockHeader) error {
	return nil
}

func (r *RootDownloader) hasBlockHash(blockHash common.Hash) bool {
	if _, ok := r.fakeMapNumber[blockHash]; ok == false {
		return false
	}
	return true
	//return r.RootState.ContainRootBlockByHash(blockHash)
}

func (r *RootDownloader) downloadRootBlockHeaders(p *peerConnection, blockHash common.Hash) []*types.RootBlockHeader {
	p.peer.RequestRootHeadersByHash(blockHash, 0, 1, false)
	for {
		select {
		case packet := <-r.rootHeaderCh:
			header := packet.(*rootHeaderPack)
			return header.headers.BlockHeaderList
		}
	}
}


func (r *RootDownloader) downloadRootBlock(p *peerConnection, data []*types.RootBlockHeader) []*types.RootBlock {
	hashList := make([]common.Hash, 0)
	for _, v := range data {
		hashList = append(hashList, v.Hash())
	}
	p.peer.RequestRootBlocks(hashList)
	for {
		select {
		case packet := <-r.rootBlockCh:
			bodys := packet.(*rootBlockPack)
			return bodys.blockList

		}
	}
}



// DeliverRootHeaders deliver root headers
func (r *RootDownloader) DeliverRootHeaders(id string, headers p2p.GetRootBlockHeaderListResponse) (err error) {
	return r.deliver(id, r.rootHeaderCh, &rootHeaderPack{id, headers}, headerInMeter, headerDropMeter)
}

// DeliverRootBlocks injects a new batch of block bodies received from a remote node.
func (r *RootDownloader) DeliverRootBlocks(id string, bodys p2p.GetRootBlockListResponse) (err error) {
	return r.deliver(id, r.rootBlockCh, &rootBlockPack{id, bodys.RootBlockList}, bodyInMeter, bodyDropMeter)
}

func (r *RootDownloader) deliver(id string, destCh chan dataPack, packet dataPack, inMeter, dropMeter metrics.Meter) (err error) {
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
