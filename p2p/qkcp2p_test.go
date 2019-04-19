package p2p

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"testing"
	"time"
)

const (
	QKCProtocolVersion = 1
	QKCProtocolLength  = 16
)

func FakeEnv(port uint64) config.ClusterConfig {
	return config.ClusterConfig{
		P2PPort: port,
		P2P: &config.P2PConfig{
			MaxPeers: 25,
			PrivKey:  "3a28b5ba57c53603b0b07b56bba752f7784bf506fa95edc395f5cf6c7514fe94",
		},
	}
}
func FakeEnv2(port uint64, bootNodes string) config.ClusterConfig {
	return config.ClusterConfig{
		P2PPort: port,
		P2P: &config.P2PConfig{
			MaxPeers:  25,
			BootNodes: bootNodes,
		},
	}
}

func StartServer(env config.ClusterConfig, t *testing.T, flag bool) (*P2PManager, error) {
	p2pManager, err := NewP2PManager(env, Protocol{
		Name:    QKCProtocolName,
		Version: QKCProtocolVersion,
		Length:  QKCProtocolLength,
		Run: func(p *Peer, rw MsgReadWriter) error {
			//peer := newPeer(int(QKCProtocolVersion), p, rw)
			return qkcMsgHandle(p, rw)
		},
	})
	if err != nil {
		t.Error("NewP2PManager err", err)
	}
	err = p2pManager.Start()
	if err != nil {
		t.Error("p2pManager start failed err", err)
	}
	if flag == true {
		p2pManager.Wait()
	}

	return p2pManager, nil
}
func TestServerMsgSend(t *testing.T) {
	env1 := FakeEnv(38291)
	bootNode := "enode://e948d976229cad7897a122d86cbb5d149178a84a5b839629e1fcf6af0981f164cb818e8c21371f5a278fcaa1a6ba5b79c77af5b2f9570e878767e9926fd8fcd6@127.0.0.1:38291"
	env2 := FakeEnv2(38292, bootNode)

	p1, err1 := StartServer(env1, t, false)
	p2, err2 := StartServer(env2, t, false)
	if err1 != nil || err2 != nil {
		t.Error("err1", err1, "err2", err2)
	}

	select {
	case <-time.After(1 * time.Second):
		if len(p1.Server.Peers()) != 1 || len(p2.Server.Peers()) != 1 {
			t.Error("connect failed ", "should peer is 1")
		}
		WriteMsgForTest(t, p1.Server.Peers()[0].rw)
		time.Sleep(1 * time.Second)
	}
}

func TestServerConnection(t *testing.T) {
	env1 := FakeEnv(38293)
	bootNode := "enode://e948d976229cad7897a122d86cbb5d149178a84a5b839629e1fcf6af0981f164cb818e8c21371f5a278fcaa1a6ba5b79c77af5b2f9570e878767e9926fd8fcd6@127.0.0.1:38293"
	env2 := FakeEnv2(38294, bootNode)

	p1, err1 := StartServer(env1, t, false)
	p2, err2 := StartServer(env2, t, false)
	if err1 != nil || err2 != nil {
		t.Error("err1", err1, "err2", err2)
	}

	select {
	case <-time.After(2 * time.Second):
		if len(p1.Server.Peers()) != 1 && len(p2.Server.Peers()) != 1 {
			t.Error("peer connect failed")
		}
		peer1 := p1.Server.Peers()[0]
		peer2 := p2.Server.Peers()[0]
		if peer1.LocalAddr().String() != peer2.RemoteAddr().String() {
			t.Error("peer connect err", "ip is not correct")
		}
		if peer2.LocalAddr().String() != peer1.RemoteAddr().String() {
			t.Error("peer connect err", "ip is not correct")
		}

		if p1.Server.NodeInfo().ID != peer2.ID().String() {
			t.Error("peer connect err", "id is not correct")
		}
		if p2.Server.NodeInfo().ID != peer1.ID().String() {
			t.Error("peer connect err", "id is not correct")
		}
	}
}

func WriteMsgForTest(t *testing.T, rw MsgReadWriter) {
	cmd, err := MakeMsg(Hello, 0, Metadata{}, HelloCmd{})
	if err != nil {
		t.Error("HelloCmd makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write HelloCmd Msg err", err)
	}

	cmd, err = MakeMsg(NewTipMsg, 0, Metadata{}, Tip{})
	if err != nil {
		t.Error("Tip makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write Tip Msg err", err)
	}

	cmd, err = MakeMsg(NewTransactionListMsg, 0, Metadata{}, NewTransactionList{})
	if err != nil {
		t.Error("NewTransactionList makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write NewTransactionList Msg err", err)
	}

	cmd, err = MakeMsg(GetPeerListRequestMsg, 0, Metadata{}, GetPeerListRequest{})
	if err != nil {
		t.Error("GetPeerListRequest makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetPeerListRequest Msg err", err)
	}

	cmd, err = MakeMsg(GetPeerListResponseMsg, 0, Metadata{}, GetPeerListResponse{})
	if err != nil {
		t.Error("GetPeerListResponse makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetPeerListResponse Msg err", err)
	}

	cmd, err = MakeMsg(GetRootBlockHeaderListRequestMsg, 0, Metadata{}, GetRootBlockHeaderListRequest{})
	if err != nil {
		t.Error("GetRootBlockHeaderListRequest makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetRootBlockHeaderListRequest Msg err", err)
	}

	cmd, err = MakeMsg(GetRootBlockHeaderListResponseMsg, 0, Metadata{}, GetRootBlockHeaderListResponse{})
	if err != nil {
		t.Error("GetRootBlockHeaderListResponse makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetRootBlockHeaderListResponse Msg err", err)
	}

	cmd, err = MakeMsg(GetRootBlockListRequestMsg, 0, Metadata{}, GetRootBlockListRequest{})
	if err != nil {
		t.Error("GetRootBlockListRequest makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetRootBlockListRequest Msg err", err)
	}

	cmd, err = MakeMsg(GetRootBlockListResponseMsg, 0, Metadata{}, GetRootBlockListResponse{})
	if err != nil {
		t.Error("GetRootBlockListResponse makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetRootBlockListResponse Msg err", err)
	}

	cmd, err = MakeMsg(GetMinorBlockListRequestMsg, 0, Metadata{}, GetMinorBlockListRequest{})
	if err != nil {
		t.Error("GetMinorBlockListRequest makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetMinorBlockListRequest Msg err", err)
	}

	cmd, err = MakeMsg(GetMinorBlockListResponseMsg, 0, Metadata{}, GetMinorBlockListResponse{})
	if err != nil {
		t.Error("GetMinorBlockListResponse makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetMinorBlockListResponse Msg err", err)
	}

	cmd, err = MakeMsg(GetMinorBlockHeaderListRequestMsg, 0, Metadata{}, GetMinorBlockHeaderListRequest{})
	if err != nil {
		t.Error("GetMinorBlockHeaderListRequest makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetMinorBlockHeaderListRequest Msg err", err)
	}

	cmd, err = MakeMsg(GetMinorBlockHeaderListResponseMsg, 0, Metadata{}, GetMinorBlockHeaderListResponse{})
	if err != nil {
		t.Error("GetMinorBlockHeaderListResponse makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write GetMinorBlockHeaderListResponse Msg err", err)
	}

	cmd, err = MakeMsg(NewBlockMinorMsg, 0, Metadata{}, NewBlockMinor{})
	if err != nil {
		t.Error("NewBlockMinor makeSendMsg err", err)
	}
	if err := rw.WriteMsg(cmd); err != nil {
		t.Error("Write NewBlockMinor Msg err", err)
	}
}
