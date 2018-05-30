package main

import (
	protojson "biblio/protocol/json"
	"github.com/ZhangGuangxu/netbuffer"
	"io"
	"log"
	"net"
	atom "sync/atomic"
	"time"
)

const (
	readDuration              = 50 * time.Millisecond
	takeMsgDuration           = 5 * time.Millisecond
	handleOutgoingMsgDuration = 50 * time.Millisecond
	writeDataDuration         = 50 * time.Millisecond
	tryTakeAllMsgDuration     = 2 * time.Second
	tryWriteAllDataDuration   = 5 * time.Second
)

type tcpConnection struct {
	conn *net.TCPConn

	client *Client

	codec Codec

	incoming *netbuffer.Buffer // 接收网络数据的缓冲区
	outgoing *netbuffer.Buffer // 将要发送的网络数据的缓冲区

	timer               *time.Timer
	handleOutgoingTimer *time.Timer
}

func newTCPConnection(c *net.TCPConn) *tcpConnection {
	return &tcpConnection{
		conn:                c,
		codec:               newJSONCodec(),
		incoming:            netbuffer.NewBuffer(),
		outgoing:            netbuffer.NewBuffer(),
		timer:               time.NewTimer(0 * time.Second),
		handleOutgoingTimer: time.NewTimer(0 * time.Second),
	}
}

func (t *tcpConnection) setParent(parent interface{}) {
	if p, ok := parent.(*Client); ok {
		t.client = p
	}
}

func (t *tcpConnection) handleRead() {
	client := t.client
	conn := t.conn
	incoming := t.incoming
	codec := t.codec

	defer serverInst.wgDone()
	defer serverInst.removeClient(client)
	defer atom.AddInt32(&client.routineCnt, -1)
	defer conn.CloseRead()
	defer client.sender.notifyClientReadClosed()

	tmpBuf := make([]byte, maxDataLen)
	var eof bool

	for {
		if needQuit() {
			break
		}
		if client.needClose() {
			break
		}
		if client.sender.shouldClose() {
			break
		}

		var buf []byte
		var useTmpBuf bool
		if incoming.WritableBytes() > 0 {
			buf = incoming.WritableByteSlice()
		} else {
			buf = tmpBuf
			useTmpBuf = true
		}

		conn.SetReadDeadline(time.Now().Add(readDuration))
		n, err := conn.Read(buf)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				// nothing to do
			} else if err == io.EOF {
				eof = true
			} else {
				client.close()
				log.Println(err)
				break
			}
		}
		if n > 0 {
			if useTmpBuf {
				// ATTENTION: MUST specify the end index(here is n) of 'buf'!
				incoming.Append(buf[:n])
			}
			if err := codec.Unpack(incoming, client); err != nil {
				client.close()
				log.Println(err)
				break
			}
			if err := client.handleMsgs(); err != nil {
				break
			}
		}
		if eof {
			client.close()
			break
		}
	}
}

func (t *tcpConnection) handleAuth(msg *message) {
	c := t.client

	req, ok := msg.proto.(*protojson.C2SAuth)
	if !ok {
		c.close()
		return
	}

	same, err := auther.checkToken(req.UID, req.Token)
	if err != nil {
		c.close()
		return
	}

	auther.delToken(req.UID)
	c.onBind()
	if same {
		c.recver.addMessage(messageCreater.createS2CAuth(true))
		serverInst.reqBind(req.UID, c)
	} else {
		c.sender.notifyClose()
		c.recver.addMessage(messageCreater.createS2CAuth(false))
		c.recver.notifyClose()
	}
}

func (t *tcpConnection) handleWrite() {
	client := t.client
	conn := t.conn

	defer serverInst.wgDone()
	defer serverInst.removeClient(client)
	defer atom.AddInt32(&client.routineCnt, -1)
	defer conn.CloseWrite()
	defer client.recver.notifyClientWriteClosed()

	for {
		if needQuit() {
			break
		}
		if client.needClose() {
			break
		}
		if client.recver.shouldClose() {
			t.tryWriteAllLeftData()
			break
		}

		if client.recver.isBindSuccess() {
			client.onBindSuccess()
		}

		if err := t.handleOutgoingMessage(handleOutgoingMsgDuration); err != nil {
			break
		}

		if err := t.tryWrite(writeDataDuration); err != nil {
			break
		}
	}
}

func (t *tcpConnection) tryWriteAllLeftData() error {
	if err := t.handleOutgoingMessage(tryTakeAllMsgDuration); err != nil {
		return err
	}

	return t.tryWrite(tryWriteAllDataDuration)
}

func (t *tcpConnection) handleOutgoingMessage(d time.Duration) error {
	client := t.client
	codec := t.codec
	outgoing := t.outgoing
	handleTimer := t.handleOutgoingTimer
	handleTimer.Reset(d)
	timer := t.timer

	for {
		timer.Reset(takeMsgDuration)
		msg := client.recver.takeMessage(timer)
		if msg != nil {
			data, err := codec.Pack(msg)
			if err != nil {
				client.close()
				return err
			}
			outgoing.Append(data)
		}

		select {
		case <-handleTimer.C:
			return nil
		default:
		}
	}

	return nil
}

func (t *tcpConnection) tryWrite(d time.Duration) error {
	if t.outgoing.ReadableBytes() <= 0 {
		return nil
	}

	conn := t.conn
	client := t.client
	outgoing := t.outgoing

	conn.SetWriteDeadline(time.Now().Add(d))
	n, err := conn.Write(outgoing.PeekAllAsByteSlice())
	conn.SetWriteDeadline(time.Time{})
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			// nothing to do
		} else {
			client.close()
			log.Println(err)
			return err
		}
	}
	if n > 0 {
		outgoing.Retrieve(n)
	}
	return nil
}
