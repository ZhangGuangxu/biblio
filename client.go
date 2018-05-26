package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
	"errors"
	ccq "github.com/ZhangGuangxu/circularqueue"
	"github.com/ZhangGuangxu/netbuffer"
	"io"
	"log"
	"net"
	"sync"
	atom "sync/atomic"
	"time"
)

var delay = 50 * time.Millisecond
var writeAllLeftDataMaxTime = 5 * time.Second

var selfHandleMsgs = map[int16]bool{
	proto.C2SAuthID: true,
}

func isSelfHandleMsgs(protoID int16) bool {
	_, ok := selfHandleMsgs[protoID]
	return ok
}

var mapProtocol2ClientHandler = map[int16](func(*Client, *message)){
	proto.C2SAuthID: func(c *Client, msg *message) {
		c.handleAuth(msg)
	},
}

func dispatchMessageToClient(c *Client, msg *message) {
	if fn, ok := mapProtocol2ClientHandler[msg.protoID]; ok {
		fn(c, msg)
	}
}

// Client wraps communication with tcp client.
type Client struct {
	muxState sync.Mutex
	state    clientState

	conn *net.TCPConn

	incoming *netbuffer.Buffer // 接收网络数据的缓冲区
	outgoing *netbuffer.Buffer // 将要发送的网络数据的缓冲区

	// Client自己处理的消息
	selfHandleMsgs *ccq.CircularQueue

	sender *messageBuffer // add message to sender
	recver *messageBuffer // take message from recver

	routineCnt int32
	toClose    int32
}

func newClient(c *net.TCPConn) *Client {
	client := &Client{
		conn:           c,
		incoming:       netbuffer.NewBuffer(),
		outgoing:       netbuffer.NewBuffer(),
		selfHandleMsgs: ccq.NewCircularQueue(),
		sender:         newMessageBuffer(),
		recver:         newMessageBuffer(),
		routineCnt:     2,
	}
	client.state = newClientStateNotbinded(client)
	return client
}

func (c *Client) setState(s clientState) {
	c.state = s
}

// @public
func (c *Client) onBind() {
	c.muxState.Lock()
	defer c.muxState.Unlock()
	c.state.onBind()
}

// @public
func (c *Client) onBindSuccess() {
	c.muxState.Lock()
	defer c.muxState.Unlock()
	c.state.onBindSuccess()
}

// @public
func (c *Client) onTimeout() {
	c.muxState.Lock()
	defer c.muxState.Unlock()
	c.state.onTimeout()
}

// @public
func (c *Client) onNewMessageToPlayer() {
	c.muxState.Lock()
	defer c.muxState.Unlock()
	c.state.onNewMessageToPlayer()
}

func (c *Client) close() {
	atom.StoreInt32(&c.toClose, 1)
}

func (c *Client) needClose() bool {
	return atom.LoadInt32(&c.toClose) == 1
}

func (c *Client) isClosed() bool {
	return atom.LoadInt32(&c.routineCnt) == 0
}

func (c *Client) handleRead() {
	defer serverInst.wgDone()
	defer serverInst.removeClient(c)
	defer atom.AddInt32(&c.routineCnt, -1)
	defer c.conn.CloseRead()

	tmpBuf := make([]byte, maxDataLen)
	var eof bool

	for {
		if needQuit() {
			break
		}
		if c.needClose() {
			break
		}
		if c.sender.shouldClose() {
			break
		}

		var buf []byte
		var useTmpBuf bool
		if c.incoming.WritableBytes() > 0 {
			buf = c.incoming.WritableByteSlice()
		} else {
			buf = tmpBuf
			useTmpBuf = true
		}

		c.conn.SetReadDeadline(time.Now().Add(delay))
		n, err := c.conn.Read(buf)
		c.conn.SetReadDeadline(time.Time{})
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				// nothing to do
			} else if err == io.EOF {
				eof = true
			} else {
				c.close()
				log.Println(err)
				break
			}
		}
		if n > 0 {
			if useTmpBuf {
				c.incoming.Append(buf)
			}
			if err := messageCodec.OnData(c.incoming, c); err != nil {
				c.close()
				log.Println(err)
				break
			}
			if err := c.handleMsgs(); err != nil {
				break
			}
		}
		if eof {
			c.close()
			//log.Println("EOF")
			break
		}
	}
}

var errInvalideMessage = errors.New("invalid message")

func (c *Client) handleMsgs() error {
	for !c.selfHandleMsgs.IsEmpty() {
		m, err := c.selfHandleMsgs.Pop()
		if err != nil {
			c.close()
			return err
		}

		v, ok := m.(*message)
		if !ok {
			c.close()
			return errInvalideMessage
		}

		dispatchMessageToClient(c, v)
	}
	return nil
}

func (c *Client) handleAuth(msg *message) {
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

func (c *Client) addIncomingMessage(protoID int16, proto interface{}) {
	msg := &message{protoID, proto}
	if isSelfHandleMsgs(protoID) {
		c.selfHandleMsgs.Push(msg)
		return
	}

	c.onNewMessageToPlayer()
	c.sender.addMessage(msg)
}

func (c *Client) handleWrite() {
	defer serverInst.wgDone()
	defer serverInst.removeClient(c)
	defer atom.AddInt32(&c.routineCnt, -1)
	defer c.conn.CloseWrite()
	defer c.recver.notifyClientWriteClosed()

	t := time.NewTimer(delay)

	for {
		if needQuit() {
			break
		}
		if c.needClose() {
			break
		}
		if c.recver.shouldClose() {
			c.tryWriteAllLeftData()
			break
		}

		if c.recver.isBindSuccess() {
			c.onBindSuccess()
		}

		if err := c.handleOutgoingMessage(t); err != nil {
			break
		}

		if c.outgoing.ReadableBytes() > 0 {
			if err := c.write(); err != nil {
				break
			}
		}
	}
}

func (c *Client) handleOutgoingMessage(timer *time.Timer) error {
	for {
		msg := c.recver.takeMessage(timer)
		if msg == nil {
			break
		}

		data, err := messageCodec.Encode(msg.proto)
		if err != nil {
			c.close()
			return err
		}
		err = messageCodec.Release(msg.protoID, msg.proto)
		if err != nil {
			c.close()
			return err
		}

		c.outgoing.AppendInt32(int32(2 + len(data)))
		c.outgoing.AppendInt16(msg.protoID)
		c.outgoing.Append(data)
	}

	return nil
}

func (c *Client) tryWriteAllLeftData() error {
	t := time.NewTimer(delay)
	if err := c.handleOutgoingMessage(t); err != nil {
		return err
	}

	timer := time.NewTimer(writeAllLeftDataMaxTime)

	for c.outgoing.ReadableBytes() > 0 {
		select {
		case <-timer.C:
			return nil
		default:
		}

		if err := c.write(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) write() error {
	c.conn.SetWriteDeadline(time.Now().Add(delay))
	n, err := c.conn.Write(c.outgoing.PeekAllAsByteSlice())
	c.conn.SetWriteDeadline(time.Time{})
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			// nothing to do
		} else {
			c.close()
			log.Println(err)
			return err
		}
	}
	if n > 0 {
		c.outgoing.Retrieve(n)
	}
	return nil
}
