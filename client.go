package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
	ccq "github.com/ZhangGuangxu/circularqueue"
	"github.com/ZhangGuangxu/netbuffer"
	"io"
	"log"
	"net"
	atom "sync/atomic"
	"time"
)

const (
	maxRoutineCount int32 = 2
)

var delay = 50 * time.Millisecond

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
	conn     *net.TCPConn
	incoming *netbuffer.Buffer // 接收客户端数据的缓冲区
	outgoing *netbuffer.Buffer // 向客户端发送数据的缓冲区

	// Client自己处理的消息
	selfHandleMsgs *ccq.CircularQueue

	sender *messageBuffer // add message to sender
	recver *messageBuffer // take message from recver

	routineCnt   int32
	toClose      int32
	toCloseRead  chan bool
	toCloseWrite chan bool
}

func newClient(c *net.TCPConn) *Client {
	client := &Client{
		conn:           c,
		incoming:       netbuffer.NewBuffer(),
		outgoing:       netbuffer.NewBuffer(),
		selfHandleMsgs: ccq.NewCircularQueue(),
		sender:         newMessageBuffer(),
		recver:         newMessageBuffer(),
		routineCnt:     maxRoutineCount,
		toCloseRead:    make(chan bool, 1),
		toCloseWrite:   make(chan bool, 1),
	}
	return client
}

// Release releases this client.
func (c *Client) Release() {
	serverInst.removeClient(c)
	c.close()
}

func (c *Client) isClosed() bool {
	return atom.LoadInt32(&c.routineCnt) == 0
}

func (c *Client) closeRead() {
	select {
	case c.toCloseRead <- true:
	default:
	}
}

func (c *Client) needCloseRead() bool {
	select {
	case <-c.toCloseRead:
		return true
	default:
		return false
	}
}

func (c *Client) delayCloseWrite(d time.Duration) {
	time.AfterFunc(d, c.closeWrite)
}

func (c *Client) closeWrite() {
	select {
	case c.toCloseWrite <- true:
	default:
	}
}

func (c *Client) needCloseWrite() bool {
	select {
	case <-c.toCloseWrite:
		return true
	default:
		return false
	}
}

func (c *Client) close() {
	atom.StoreInt32(&c.toClose, 1)
}

func (c *Client) needClose() bool {
	return atom.LoadInt32(&c.toClose) == 1
}

func (c *Client) handleRead() {
	defer serverInst.wgDone()
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
		if c.needCloseRead() {
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
			err := messageCodec.OnData(c.incoming, c)
			if err != nil {
				c.close()
				log.Println(err)
				break
			}
			c.handleMsgs()
		}
		if eof {
			c.close()
			log.Println("EOF")
			break
		}
	}
}

func (c *Client) handleMsgs() {
	for !c.selfHandleMsgs.IsEmpty() {
		m, err := c.selfHandleMsgs.Pop()
		if err != nil {
			c.close()
			break
		}

		v, ok := m.(*message)
		if !ok {
			c.close()
			break
		}

		dispatchMessageToClient(c, v)
	}
}

func (c *Client) handleAuth(msg *message) {
	req, ok := msg.proto.(*protojson.C2SAuth)
	if !ok {
		c.close()
		return
	}

	same, err := auther.compareToken(req.UID, req.Token)
	if err != nil {
		c.close()
		return
	}

	auther.delToken(req.UID)
	serverInst.authReceived(c)
	if same {
		// TODO: Send a message to nofity this client that auth succeed.
		serverInst.reqBind(req.UID, c)
	} else {
		// TODO: Send a message to nofity this client what is wrong?
		c.close()
	}
}

func (c *Client) addIncomingMessage(protoID int16, proto interface{}) {
	msg := &message{protoID, proto}
	if isSelfHandleMsgs(protoID) {
		c.selfHandleMsgs.Push(msg)
		return
	}

	c.sender.addMessage(msg)
}

func (c *Client) handleWrite() {
	defer serverInst.wgDone()
	defer atom.AddInt32(&c.routineCnt, -1)
	defer c.conn.CloseWrite()

	t := time.NewTimer(delay)

	for {
		if needQuit() {
			break
		}
		if c.needClose() {
			break
		}
		if c.needCloseWrite() {
			break
		}

		c.handleOutgoingMessage(t)

		if c.outgoing.ReadableBytes() > 0 {
			c.conn.SetWriteDeadline(time.Now().Add(delay))
			n, err := c.conn.Write(c.outgoing.PeekAllAsByteSlice())
			c.conn.SetWriteDeadline(time.Time{})
			if err != nil {
				if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
					// nothing to do
				} else {
					c.close()
					log.Println(err)
					return
				}
			}
			if n > 0 {
				c.outgoing.Retrieve(n)
			}
		}
	}
}

func (c *Client) handleOutgoingMessage(timer *time.Timer) {
	for {
		msg := c.recver.takeMessage(timer)
		if msg == nil {
			break
		}

		data, err := messageCodec.Encode(msg.proto)
		messageCodec.Release(msg.protoID, msg.proto)
		if err != nil {
			c.close()
			break
		}

		c.outgoing.AppendInt32(int32(2 + len(data)))
		c.outgoing.AppendInt16(msg.protoID)
		c.outgoing.Append(data)
	}
}
