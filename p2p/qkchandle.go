package p2p

import (
	"bytes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/golang/snappy"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"strings"
	"time"
)

const messageId = 0

type Message string

func msgHandler(peer *Peer, ws MsgReadWriter) error {
	fmt.Println("start msgHandle")
	for {
		fmt.Println("1111")
		msg, err := ws.ReadMsg()

		fmt.Println("masssssssssssssssssssssssssss", "err", err)
		if err != nil {
			return err
		}
		fmt.Println("准备解码")
		qkcBody, err := ioutil.ReadAll(msg.Payload)
		if err != nil {
			fmt.Println("read payload failed ", err)
			return err
		}
		fmt.Println("qkcBody---------", len(qkcBody), hex.EncodeToString(qkcBody))
		qkcMsg, err := DecodeQKCMsg(qkcBody)
		if err != nil {
			fmt.Println("decode qkcMsg err", err)
			return err
		}
		switch qkcMsg.op {
		case 0:
			var helloData HelloCmd
			err := serialize.DeserializeFromBytes(qkcMsg.data, &helloData)
			fmt.Println("deserializeFromBytes err", err)
			err = peer.rw.WriteMsg(Msg{Code: 16, Size: uint32(len(qkcBody)), Payload: bytes.NewReader(qkcBody)})
			fmt.Println("writeMsg", err)

			fmt.Println("err", err, len(qkcBody), hex.EncodeToString(qkcBody))

		}
	}
	fmt.Println("start msgHandle end")
	return nil
}
func MyProtocol() Protocol {
	return Protocol{
		Name:    "quarkchain",
		Version: 1,
		Length:  1,
		Run:     msgHandler,
	}
}

func getNodesFromConfig(configNodes string) ([]*enode.Node, error) {
	if configNodes == "" {
		return make([]*enode.Node, 0), nil
	}

	NodeList := strings.Split(configNodes, ",")
	enodeList := make([]*enode.Node, 0, len(NodeList))
	for _, url := range NodeList {
		node, err := enode.ParseV4(url)
		if err != nil {
			return nil, err
		} else {
			log.Error("Node add", "url", url)
		}
		enodeList = append(enodeList, node)
	}
	fmt.Println("enodList", enodeList)
	return enodeList, nil
}

func getPrivateKeyFromConfig(configKey string) (*ecdsa.PrivateKey, error) {
	if configKey == "" {
		sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
		return sk, err
	}
	configKeyValue, err := hex.DecodeString(configKey)
	if err != nil {
		return nil, err
	}
	keyValue := new(big.Int).SetBytes(configKeyValue)
	if err != nil {
		return nil, err
	}
	sk := new(ecdsa.PrivateKey)
	sk.PublicKey.Curve = crypto.S256()
	sk.D = keyValue
	sk.PublicKey.X, sk.PublicKey.Y = crypto.S256().ScalarBaseMult(keyValue.Bytes())
	return sk, nil
}

type qkcRLPX struct {
	*rlpx
}

func NewqkcRLPX(fd net.Conn) transport {
	rlpx := newRLPX(fd).(*rlpx)
	return &qkcRLPX{rlpx}
}

func (self *qkcRLPX) ReadMsg() (Msg, error) {
	fmt.Println("qkcRLPX ReadMsg")
	self.rmu.Lock()
	defer self.rmu.Unlock()
	self.fd.SetReadDeadline(time.Now().Add(frameReadTimeout))

	return self.readQKCMsg()
}

func (self *qkcRLPX) WriteMsg(msg Msg) error {
	self.wmu.Lock()
	defer self.wmu.Unlock()
	self.fd.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	//panic(errors.New("==================="))
	return self.writeQKCMsg(msg)
}

func (self *qkcRLPX) readQKCMsg() (msg Msg, err error) {
	fmt.Println("HHHHHHHHHHHHHHHHHHHHHHHh qkcRLPX read start")
	// read the header
	headbuf := make([]byte, 32)
	if _, err := io.ReadFull(self.rw.conn, headbuf); err != nil {
		return msg, err
	}
	fmt.Println("HHH111")

	// verify header mac
	shouldMAC := updateMAC(self.rw.ingressMAC, self.rw.macCipher, headbuf[:16])
	if !hmac.Equal(shouldMAC, headbuf[16:]) {
		return msg, errors.New("bad header MAC")
	}
	fmt.Println("HHH222")
	self.rw.dec.XORKeyStream(headbuf[:16], headbuf[:16]) // first half is now decrypted
	fsize := binary.BigEndian.Uint32(headbuf[:4])
	fmt.Println("headerbuf", hex.EncodeToString(headbuf), "fsize", fsize)

	framebuf := make([]byte, fsize)
	if _, err := io.ReadFull(self.rw.conn, framebuf); err != nil {
		return msg, err
	}

	// read and validate frame MAC. we can re-use headbuf for that.
	self.rw.ingressMAC.Write(framebuf)
	fmacseed := self.rw.ingressMAC.Sum(nil)
	if _, err := io.ReadFull(self.rw.conn, headbuf[:16]); err != nil {
		return msg, err
	}
	shouldMAC = updateMAC(self.rw.ingressMAC, self.rw.macCipher, fmacseed)
	if !hmac.Equal(shouldMAC, headbuf[:16]) {
		return msg, errors.New("bad frame MAC")
	}

	// decrypt frame content
	self.rw.dec.XORKeyStream(framebuf, framebuf)

	// decode message code
	content := bytes.NewReader(framebuf[:fsize])
	msg.Size = uint32(content.Len())
	msg.Payload = content

	// if snappy is enabled, verify and decompress message
	if self.rw.snappy {
		payload, err := ioutil.ReadAll(msg.Payload)
		if err != nil {
			return msg, err
		}
		size, err := snappy.DecodedLen(payload)
		if err != nil {
			return msg, err
		}
		if size > int(maxUint24) {
			return msg, errPlainMessageTooLarge
		}
		payload, err = snappy.Decode(nil, payload)
		if err != nil {
			return msg, err
		}
		msg.Size, msg.Payload = uint32(size), bytes.NewReader(payload)
	}
	fmt.Println("HHHHHHHHHHHHHHHHHHHHHHHh qkcRLPX read end", hex.EncodeToString(framebuf))
	return msg, nil
}
func (self *qkcRLPX) writeQKCMsg(msg Msg) error {
	//ptype, _ := rlp.EncodeToBytes(msg.Code)

	// if snappy is enabled, compress message now
	if self.rw.snappy {
		fmt.Println("snnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn")
		if msg.Size > maxUint24 {
			return errPlainMessageTooLarge
		}
		payload, _ := ioutil.ReadAll(msg.Payload)
		payload = snappy.Encode(nil, payload)

		msg.Payload = bytes.NewReader(payload)
		msg.Size = uint32(len(payload))
	}
	// write header
	headbuf := make([]byte, 32)
	binary.BigEndian.PutUint32(headbuf, msg.Size)
	fmt.Println("write headbuf 111", hex.EncodeToString(headbuf))
	//fsize := msg.Size
	self.rw.enc.XORKeyStream(headbuf[:16], headbuf[:16]) // first half is now encrypted
	fmt.Println("write headbuf 222", hex.EncodeToString(headbuf))
	// write header MAC
	copy(headbuf[16:], updateMAC(self.rw.egressMAC, self.rw.macCipher, headbuf[:16]))
	fmt.Println("write headbuf 333", hex.EncodeToString(headbuf))
	if _, err := self.rw.conn.Write(headbuf); err != nil {
		return err
	}

	// write encrypted frame, updating the egress MAC hash with
	// the data written to conn.
	tee := cipher.StreamWriter{S: self.rw.enc, W: io.MultiWriter(self.rw.conn, self.rw.egressMAC)}
	realbody, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		fmt.Println("read failed", err)
		return err
	}
	fmt.Println("realBody", len(realbody), hex.EncodeToString(realbody))
	if _, err := tee.Write(realbody); err != nil {
		return err
	}
	if _, err := io.Copy(tee, msg.Payload); err != nil {
		return err
	}

	// write frame MAC. egress MAC hash is up to date because
	// frame content was written to it as well.
	fmacseed := self.rw.egressMAC.Sum(nil)
	mac := updateMAC(self.rw.egressMAC, self.rw.macCipher, fmacseed)
	_, err = self.rw.conn.Write(mac)
	fmt.Println("====================================")
	return err
}
