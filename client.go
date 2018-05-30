package main

import (
	proto "biblio/protocol"
	"errors"
	ccq "github.com/ZhangGuangxu/circularqueue"
	"sync"
	atom "sync/atomic"
)

var selfHandleMsgs = map[int16]bool{
	proto.C2SAuthID: true,
}

func isSelfHandleMsgs(protoID int16) bool {
	_, ok := selfHandleMsgs[protoID]
	return ok
}

var mapProtocol2ClientHandler = map[int16](func(*Client, *message)){
	proto.C2SAuthID: func(c *Client, msg *message) {
		c.conn.handleAuth(msg)
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

	conn connection

	// Client自己处理的消息
	selfHandleMsgs *ccq.CircularQueue

	sender messageMediator // 'handleRead' goroutine adds message to sender
	recver messageMediator // 'handleWrite' goroutine takes message from recver

	routineCnt int32
	toClose    int32
}

func newClient() *Client {
	client := &Client{
		selfHandleMsgs: ccq.NewCircularQueue(),
		sender:         newMessageChannel(),
		recver:         newMessageChannel(),
		routineCnt:     2,
	}
	client.setState(newClientStateNotbinded(client))
	return client
}

func (c *Client) setConn(conn connection) {
	c.conn = conn
	c.conn.setParent(c)
}

func (c *Client) start() {
	c.sender.start()
	c.recver.start()
	c.startRead()
	c.startWrite()
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

func (c *Client) startRead() {
	serverInst.wgAddOne()
	go c.conn.handleRead()
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

func (c *Client) addIncomingMessage(protoID int16, proto interface{}) {
	msg := &message{protoID, proto}
	if isSelfHandleMsgs(protoID) {
		c.selfHandleMsgs.Push(msg)
		return
	}

	c.onNewMessageToPlayer()
	c.sender.addMessage(msg)
}

func (c *Client) startWrite() {
	serverInst.wgAddOne()
	go c.conn.handleWrite()
}
