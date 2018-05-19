package main

import (
	ccq "github.com/ZhangGuangxu/circularqueue"
	"github.com/ZhangGuangxu/netbuffer"
	"log"
	"net"
	"sync"
	atom "sync/atomic"
	"time"
)

const (
	maxRoutineCount int32 = 2
)

var delay = 50 * time.Millisecond

// Client wraps communication with tcp client.
type Client struct {
	conn     *net.TCPConn
	incoming *netbuffer.Buffer // 接收客户端数据的缓冲区
	outgoing *netbuffer.Buffer // 向客户端发送数据的缓冲区
	cc       Codec

	newIncomingMessage chan bool
	imux               sync.Mutex
	// 解码器解码出来的消息的队列
	incomingMessagesToAdd  *ccq.CircularQueue
	incomingMessagesToTake *ccq.CircularQueue

	newOutgoingMessage chan bool
	omux               sync.Mutex
	// 编码器要编码的消息的队列
	outgoingMessagesToAdd  *ccq.CircularQueue
	outgoingMessagesToTake *ccq.CircularQueue

	routineCnt int32
	toClose    int32
}

func newClient(c *net.TCPConn, rcc Codec) *Client {
	client := &Client{
		conn:                   c,
		incoming:               netbuffer.NewBuffer(),
		outgoing:               netbuffer.NewBuffer(),
		cc:                     rcc,
		newIncomingMessage:     make(chan bool, 1),
		incomingMessagesToAdd:  ccq.NewCircularQueue(),
		incomingMessagesToTake: ccq.NewCircularQueue(),
		newOutgoingMessage:     make(chan bool, 1),
		outgoingMessagesToAdd:  ccq.NewCircularQueue(),
		outgoingMessagesToTake: ccq.NewCircularQueue(),
		routineCnt:             maxRoutineCount,
	}
	rcc.SetClient(client)
	return client
}

func (c *Client) isClosed() bool {
	return atom.LoadInt32(&c.routineCnt) == 0
}

func (c *Client) close() {
	atom.StoreInt32(&c.toClose, 1)
}

func (c *Client) needClose() bool {
	return atom.LoadInt32(&c.toClose) == 1
}

func (c *Client) handleRead(quit *uint32, wg *sync.WaitGroup) {
	defer wg.Done()
	defer atom.AddInt32(&c.routineCnt, -1)
	defer c.conn.CloseRead()

	tmpBuf := make([]byte, maxDataLen)

	for {
		if needQuit() {
			break
		}

		if c.needClose() {
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
			err := c.cc.OnData(c.incoming)
			if err != nil {
				c.close()
				log.Println(err)
				break
			}
		}
	}
}

func (c *Client) addIncomingMessage(protoID int16, proto interface{}) {
	c.imux.Lock()
	c.incomingMessagesToAdd.Push(&message{protoID, proto})
	c.imux.Unlock()

	c.incomingMessageAdded()
}

func (c *Client) incomingMessageAdded() {
	select {
	case c.newIncomingMessage <- true:
	default:
	}
}

func (c *Client) hasIncomingMessage() bool {
	select {
	case <-c.newIncomingMessage:
		return true
	default:
		return false
	}
}

func (c *Client) handleAllIncomingMessage(player *Player) {
	if !c.hasIncomingMessage() {
		return
	}

	c.imux.Lock()
	c.incomingMessagesToAdd, c.incomingMessagesToTake = c.incomingMessagesToTake, c.incomingMessagesToAdd
	c.imux.Unlock()

	for !c.incomingMessagesToTake.IsEmpty() {
		m, err := c.incomingMessagesToTake.Pop()
		if err != nil {
			c.close()
			break
		}

		v, ok := m.(*message)
		if !ok {
			c.close()
			break
		}

		dispatchMessage(player, v)
		c.cc.Release(v.protoID, v.proto)
	}
}

func (c *Client) handleWrite(quit *uint32, wg *sync.WaitGroup) {
	defer wg.Done()
	defer atom.AddInt32(&c.routineCnt, -1)
	defer c.conn.CloseWrite()

	timer := time.NewTimer(0 * time.Millisecond)
	var outgoing bool

	for {
		if needQuit() {
			break
		}

		if c.needClose() {
			break
		}

		if outgoing {
			c.handleAllOutgoingMessage()
		}

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
		} else {
			timer.Reset(delay)
			select {
			case <-c.newOutgoingMessage:
				outgoing = true
			case <-timer.C:
				outgoing = false
			}
		}
	}
}

func (c *Client) addOutgoingMessage(protoID int16, proto interface{}) {
	c.omux.Lock()
	c.outgoingMessagesToAdd.Push(&message{protoID, proto})
	c.omux.Unlock()

	c.outgoingMessageAdded()
}

func (c *Client) outgoingMessageAdded() {
	select {
	case c.newOutgoingMessage <- true:
	default:
	}
}

func (c *Client) handleAllOutgoingMessage() {
	c.omux.Lock()
	c.outgoingMessagesToAdd, c.outgoingMessagesToTake = c.outgoingMessagesToTake, c.outgoingMessagesToAdd
	c.omux.Unlock()

	for !c.outgoingMessagesToTake.IsEmpty() {
		m, err := c.outgoingMessagesToTake.Pop()
		if err != nil {
			c.close()
			break
		}

		v, ok := m.(*message)
		if !ok {
			c.close()
			break
		}

		data, err := c.cc.Encode(v.proto)
		c.cc.Release(v.protoID, v.proto)
		if err != nil {
			c.close()
			break
		}

		c.outgoing.AppendInt32(int32(2 + len(data)))
		c.outgoing.AppendInt16(v.protoID)
		c.outgoing.Append(data)
	}
}
