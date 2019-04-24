package p2p

import (
	"bytes"
	"crypto/cipher"
	"crypto/hmac"
	"encoding/binary"
	"errors"
	"github.com/ethereum/go-ethereum/log"
	"github.com/golang/snappy"
	"io"
	"io/ioutil"
	"net"
	"time"
)

var (
	msgHandleLog = "qkcMsgHandle"
)

func qkcMsgHandle(peer *Peer, ws MsgReadWriter) error {
	for {
		msg, err := ws.ReadMsg()
		if err != nil {
			log.Error(msgHandleLog, "readMsg err", err)
			return err
		}

		qkcBody, err := ioutil.ReadAll(msg.Payload)
		if err != nil {
			log.Error(msgHandleLog, "read payload failed err", err)
			return err
		}
		qkcMsg, err := DecodeQKCMsg(qkcBody)
		if err != nil {
			log.Error(msgHandleLog, "decode qkc msg err", err)
			return err
		}
		log.Info(msgHandleLog, "recv qkc Op", qkcMsg.Op, "rpcId", qkcMsg.RpcID, "MetaData", qkcMsg.MetaData)

		if _, ok := OPSerializerMap[qkcMsg.Op]; !ok {
			log.Error(msgHandleLog, "unExcepted Op", qkcMsg.Op)
			return err
		}

		if HandleFunc, ok := OPNonRPCMap[qkcMsg.Op]; ok {
			HandleFunc(qkcMsg.Op, qkcMsg.Data)
		} else if HandleFunc, ok := OpRPCMap[qkcMsg.Op]; ok {
			HandleFunc.handleFunc(qkcMsg.Data)
		} else {
			//TODO future
		}
	}
}

type qkcRlp struct {
	*rlpx
}

// NewQKCRlp new qkc rlp
func NewQKCRlp(fd net.Conn) transport {
	rlpx := newRLPX(fd).(*rlpx)
	return &qkcRlp{rlpx}
}

func (q *qkcRlp) ReadMsg() (Msg, error) {
	q.rmu.Lock()
	defer q.rmu.Unlock()
	q.fd.SetReadDeadline(time.Time{})

	return q.readQKCMsg()
}

func (q *qkcRlp) WriteMsg(msg Msg) error {
	q.wmu.Lock()
	defer q.wmu.Unlock()
	q.fd.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	return q.writeQKCMsg(msg)
}

func (q *qkcRlp) readQKCMsg() (msg Msg, err error) {
	// read the header
	headBuf := make([]byte, 32)
	if _, err := io.ReadFull(q.rw.conn, headBuf); err != nil {
		return msg, err
	}

	// verify header mac
	shouldMAC := updateMAC(q.rw.ingressMAC, q.rw.macCipher, headBuf[:16])
	if !hmac.Equal(shouldMAC, headBuf[16:]) {
		return msg, errors.New("bad header MAC")
	}

	q.rw.dec.XORKeyStream(headBuf[:16], headBuf[:16]) // first half is now decrypted
	fSize := binary.BigEndian.Uint32(headBuf[:4])

	frameBuf := make([]byte, fSize)
	if _, err := io.ReadFull(q.rw.conn, frameBuf); err != nil {
		return msg, err
	}

	// read and validate frame MAC. we can re-use headBuf for that.
	q.rw.ingressMAC.Write(frameBuf)
	fMacSeed := q.rw.ingressMAC.Sum(nil)
	if _, err := io.ReadFull(q.rw.conn, headBuf[:16]); err != nil {
		return msg, err
	}
	shouldMAC = updateMAC(q.rw.ingressMAC, q.rw.macCipher, fMacSeed)
	if !hmac.Equal(shouldMAC, headBuf[:16]) {
		return msg, errors.New("bad frame MAC")
	}

	// decrypt frame content
	q.rw.dec.XORKeyStream(frameBuf, frameBuf)

	// decode message code
	content := bytes.NewReader(frameBuf[:fSize])
	msg.Size = uint32(content.Len())
	msg.Payload = content

	// if snappy is enabled, verify and decompress message
	if q.rw.snappy {
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
	msg.Code = baseProtocolLength
	return msg, nil
}

func (q *qkcRlp) writeQKCMsg(msg Msg) error {
	// if snappy is enabled, compress message now
	if q.rw.snappy {
		if msg.Size > maxUint24 {
			return errPlainMessageTooLarge
		}
		payload, _ := ioutil.ReadAll(msg.Payload)
		payload = snappy.Encode(nil, payload)

		msg.Payload = bytes.NewReader(payload)
		msg.Size = uint32(len(payload))
	}
	// write header
	headBuf := make([]byte, 32)
	binary.BigEndian.PutUint32(headBuf, msg.Size)

	q.rw.enc.XORKeyStream(headBuf[:16], headBuf[:16]) // first half is now encrypted
	// write header MAC
	copy(headBuf[16:], updateMAC(q.rw.egressMAC, q.rw.macCipher, headBuf[:16]))
	if _, err := q.rw.conn.Write(headBuf); err != nil {
		return err
	}

	// write encrypted frame, updating the egress MAC hash with
	// the Data written to conn.
	tee := cipher.StreamWriter{S: q.rw.enc, W: io.MultiWriter(q.rw.conn, q.rw.egressMAC)}
	realBody, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return err
	}
	if _, err := tee.Write(realBody); err != nil {
		return err
	}
	if _, err := io.Copy(tee, msg.Payload); err != nil {
		return err
	}

	// write frame MAC. egress MAC hash is up to date because
	// frame content was written to it as well.
	fMacSeed := q.rw.egressMAC.Sum(nil)
	mac := updateMAC(q.rw.egressMAC, q.rw.macCipher, fMacSeed)
	_, err = q.rw.conn.Write(mac)
	return err
}

func (q *qkcRlp) doProtoHandshake(our *protoHandshake) (their *protoHandshake, err error) {
	perHandshake, err := q.rlpx.doProtoHandshake(our)
	if err != nil {
		return nil, err
	}
	return perHandshake, nil
}
