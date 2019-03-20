package qkc

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/qkc/download"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"io/ioutil"
	"math/big"
	"time"
)

// QKCProtocol details
const (
	QKCProtocolName    = "quarkchain"
	QKCProtocolVersion = 1
	QKCProtocolLength  = 16
)

// Manager QKC manager
type Manager struct {
	rootDownloader *downloader.RootDownloader
	minorDownloader *downloader.MinorDownloader
	SubProtocols   p2p.Protocol
	P2PManager     *p2p.PManager

	peers *peerSet // Set of active peers from which rootDownloader can proceed
	log   string
}

// NewQKCManager  new qkc manager
func NewQKCManager(env config.ClusterConfig) (*Manager, error) {
	var err error
	qkcManager := &Manager{
	}
	protocol := p2p.Protocol{
		Name:    QKCProtocolName,
		Version: QKCProtocolVersion,
		Length:  QKCProtocolLength,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			peer := newPeer(int(QKCProtocolVersion), p, rw)
			return qkcManager.handle(peer)
		},
	}
	qkcManager.SubProtocols = protocol
	qkcManager.P2PManager, err = p2p.NewP2PManager(env, protocol)
	if err != nil {
		return nil, err
	}
	qkcManager.rootDownloader = downloader.NewRootDownloader()
	qkcManager.minorDownloader=downloader.NewMinorDownloader()
	qkcManager.rootDownloader.SetFakeMinorDownLoader(qkcManager.minorDownloader)
	return qkcManager, nil

}

// Start manager start
func (m *Manager) Start() {
	if err := m.P2PManager.Start(); err != nil {
		panic(err)
	}
	m.P2PManager.Wait()
}

func (m *Manager) synchronise(peer *peer, header *types.RootBlockHeader) {
	if err := m.rootDownloader.SyncWithPeer(peer.id, peer.version, peer, header); err != nil {
		return
	}
}

// Handshake qkc handshake
func (m *Manager) Handshake(peer *peer) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)

	id := crypto.FromECDSAPub(&m.P2PManager.Server.PrivateKey.PublicKey)
	hello, err := p2p.MakeMsg(p2p.Hello, 0, p2p.Metadata{}, p2p.HelloCmd{
		Version:   0,
		NetWorkID: 24,
		PeerID:    common.BytesToHash(id),
		PeerPort:  38291,
		RootBlockHeader: &types.RootBlockHeader{
			Version:         0,
			Number:          0,
			Time:            1519147489,
			ParentHash:      common.Hash{},
			MinorHeaderHash: common.Hash{},
			Difficulty:      big.NewInt(5000),
		},
	})
	if err != nil {
		return err

	}

	go func() {
		errc <- m.readStatus(peer)
	}()
	go func() {
		errc <- peer.rw.WriteMsg(hello)
	}()

	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			fmt.Println("return disc Read Time out")
			return p2p.DiscReadTimeout
		}
	}
	return nil
}

func (m *Manager) readStatus(peer *peer) (err error) {
	msg, err := peer.rw.ReadMsg()
	if err != nil {
		return err
	}

	qkcBody, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return err
	}
	qkcMsg, err := p2p.DecodeQKCMsg(qkcBody)
	if err != nil {
		return err
	}
	log.Info(m.log, "hello exchange end op", qkcMsg)

	var ss = p2p.HelloCmd{}

	err = serialize.DeserializeFromBytes(qkcMsg.Data, &ss)
	if err != nil {
		return err
	}

	if msg.Code != uint64(p2p.Hello) {
		return errors.New("msgCode is err")
	}
	go m.synchronise(peer, ss.RootBlockHeader)
	return nil
}
func (m *Manager) handle(peer *peer) error {
	if len(m.P2PManager.Server.PeersInfo()) >= m.P2PManager.Server.MaxPeers {
		return p2p.DiscTooManyPeers
	}

	//fmt.Println("准备握手")
	if err := m.Handshake(peer); err != nil {
		return err
	}

	//fmt.Println("握手结束")

	for {
		if err := m.handleMsg(peer); err != nil {
			//log.Error(m.log, "handle msg err", err)
			return err
		}
	}

}

func (m *Manager) handleMsg(peer *peer) error {
	msg, err := peer.rw.ReadMsg()
	if err != nil {
		return err
	}
	payload, err := ioutil.ReadAll(msg.Payload)
	qkcMsg, err := p2p.DecodeQKCMsg(payload)
	if err != nil {
		return err
	}

	log.Info(m.log, " receive QKC Msgop", qkcMsg.Op.String())
	switch {
	case qkcMsg.Op == p2p.Hello:
		log.Error(m.log, "no excepted hello cmd", "return err")
	case qkcMsg.Op == p2p.NewMinorBlockHeaderListMsg:
		var minorBlockHeaderList p2p.NewMinorBlockHeaderList
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockHeaderList); err != nil {
			return err
		}
		testDisplayNewMinotBlockHeaderList(minorBlockHeaderList.RootBlockHeader, minorBlockHeaderList.MinorBlockHeaderList)
	case qkcMsg.Op == p2p.NewBlockMinorMsg:
		if err := testDisplayNewBlockMinor(qkcMsg); err != nil {
			return err
		}
	case qkcMsg.Op == p2p.GetRootBlockHeaderListResponseMsg:
		var blockHeaderResp p2p.GetRootBlockHeaderListResponse
		if err != nil {
			return err
		}
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderResp); err != nil {
			fmt.Println("反序列化失败 err", err)
			return err
		}
		m.rootDownloader.DeliverRootHeaders(peer.id, blockHeaderResp)

	case qkcMsg.Op == p2p.GetRootBlockListResponseMsg:
		var blockResp p2p.GetRootBlockListResponse

		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockResp); err != nil {
			return err
		}
		m.rootDownloader.DeliverRootBodies(peer.id, blockResp)
	case qkcMsg.Op == p2p.GetMinorBlockHeaderListResponseMsg:
		var minorHeaderResp p2p.GetMinorBlockHeaderListResponse

		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderResp); err != nil {
			return err
		}
		m.minorDownloader.DeliverMinorHeaders(peer.id, minorHeaderResp)
	case qkcMsg.Op == p2p.GetMinorBlockListResponseMsg:
		var minorBlockResp p2p.GetMinorBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockResp); err != nil {
			return err
		}
		m.minorDownloader.DeliverMinorBodies(peer.id, minorBlockResp)

	default:
		return errors.New("未知的消息码")
	}
	return nil
}

func testDisplayNewBlockMinor(msg p2p.QKCMsg) error {
	//TODO only display need delete
	var newBlockMinor p2p.NewBlockMinor

	if err := serialize.DeserializeFromBytes(msg.Data, &newBlockMinor); err != nil {
		return err
	}

	fmt.Println("===root block到来 开始展示", newBlockMinor.Block.Header().Hash().String())
	header := newBlockMinor.Block.Header()
	fmt.Println("Time", header.Time)
	fmt.Println("Number", header.Number)
	fmt.Println("Version", header.Version)
	fmt.Println("Branch", header.Branch)
	fmt.Println("PrevRootBlockHash", header.PrevRootBlockHash.String())
	fmt.Println("Bloom", header.Bloom.Big().String())
	fmt.Println("Coinbase", header.Coinbase.ToHex())
	fmt.Println("Difficulty", header.Difficulty.String())
	fmt.Println("Nonce", header.Nonce)
	fmt.Println("CoinbaseAmount", header.CoinbaseAmount.Value.String())
	fmt.Println("Extra", header.Extra)
	fmt.Println("MixDigest", header.MixDigest.String())
	fmt.Println("===root block到来 展示结束")
	return nil
}

func testDisplayNewMinotBlockHeaderList(header *types.RootBlockHeader, minorHeaderList []*types.MinorBlockHeader) {
	//TODO only display need delete
	fmt.Println("===minor block到来 开始展示", header.Hash().String())
	fmt.Println("Time", header.Time)
	fmt.Println("Number", header.Number)
	fmt.Println("Version", header.Version)
	fmt.Println("ParentHash", header.ParentHash.String())
	fmt.Println("Coinbase", header.Coinbase.ToHex())
	fmt.Println("Difficulty", header.Difficulty.String())
	fmt.Println("Nonce", header.Nonce)
	fmt.Println("CoinbaseAmount", header.CoinbaseAmount.Value.String())
	fmt.Println("Extra", header.Extra)
	fmt.Println("MinorHeaderHash", header.MinorHeaderHash.String())
	fmt.Println("MixDigest", header.MixDigest.String())
	fmt.Println("Signature", header.Signature)
	fmt.Println("[]*types.MinorBlockHeader len", len(minorHeaderList))
	for _, v := range minorHeaderList {
		fmt.Println("NewMinorBlockHeaderList single types.MinorBlockHeader", v.Hash().String())
	}
	fmt.Println("===minor block到来 展示结束")
}
