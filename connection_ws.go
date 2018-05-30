package main

import (
	protojson "biblio/protocol/json"
	ccq "github.com/ZhangGuangxu/circularqueue"
	"github.com/ZhangGuangxu/netbuffer"
	ws "github.com/gorilla/websocket"
	"log"
	atom "sync/atomic"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 1 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	wsTakeMsgDuration         = 5 * time.Millisecond
	wsTryWriteAllDataDuration = 5 * time.Second

	wsHandleOutgoingMsgDuration = 50 * time.Millisecond
	wsTryWriteDuration          = 50 * time.Millisecond
)

type wsConnection struct {
	conn *ws.Conn

	client *Client

	codec Codec

	incoming *netbuffer.Buffer // 接收网络数据的缓冲区
	outgoing *ccq.CircularQueue

	ticker    *time.Ticker
	timer     *time.Timer
	loopTimer *time.Timer
}

func newWSConnection(c *ws.Conn) *wsConnection {
	return &wsConnection{
		conn:      c,
		codec:     newJSONCodec(),
		incoming:  netbuffer.NewBuffer(),
		outgoing:  ccq.NewCircularQueue(),
		ticker:    time.NewTicker(pingPeriod),
		timer:     time.NewTimer(0 * time.Second),
		loopTimer: time.NewTimer(0 * time.Second),
	}
}

func (w *wsConnection) setParent(parent interface{}) {
	if p, ok := parent.(*Client); ok {
		w.client = p
	}
}

func (w *wsConnection) handleRead() {
	client := w.client
	conn := w.conn
	incoming := w.incoming
	codec := w.codec

	defer serverInst.wgDone()
	defer serverInst.removeClient(client)
	defer atom.AddInt32(&client.routineCnt, -1)
	defer conn.Close()
	defer client.sender.notifyClientReadClosed()

	shouldQuit := func() bool {
		if needQuit() {
			return true
		}
		if client.needClose() {
			return true
		}
		return false
	}

	conn.SetReadLimit(maxDataLen)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		if shouldQuit() {
			return nil
		}

		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if shouldQuit() {
			break
		}

		// If 'break' here, connection will close.
		// But maybe 'handleWrite' needs to send some data before close.
		// So I comment these.
		// if client.sender.shouldClose() {
		// 	break
		// }

		_, data, err := w.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway) {
				log.Println(err)
			}
			client.close()
			break
		}
		incoming.Append(data)

		if err := codec.Unpack(incoming, client); err != nil {
			client.close()
			log.Println(err)
			break
		}
		if err := client.handleMsgs(); err != nil {
			break
		}
	}
}

func (w *wsConnection) handleAuth(msg *message) {
	c := w.client

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

func (w *wsConnection) handleWrite() {
	client := w.client
	conn := w.conn

	defer serverInst.wgDone()
	defer serverInst.removeClient(client)
	defer atom.AddInt32(&client.routineCnt, -1)
	defer conn.Close()
	defer client.recver.notifyClientWriteClosed()
	defer w.ticker.Stop()

	for {
		if needQuit() {
			break
		}
		if client.needClose() {
			break
		}
		if client.recver.shouldClose() {
			w.tryWriteAllLeftData()
			break
		}

		if client.recver.isBindSuccess() {
			client.onBindSuccess()
		}

		if err := w.handleOutgoingMessage(); err != nil {
			break
		}

		if err := w.tryWrite(); err != nil {
			break
		}
		if err := w.writePing(); err != nil {
			break
		}
	}
}

func (w *wsConnection) tryWriteAllLeftData() error {
	t := w.loopTimer
	t.Reset(wsTryWriteAllDataDuration)

	for {
		if err := w.handleOneMessage(); err != nil {
			return err
		}

		if err := w.writeOnce(); err != nil {
			return err
		}

		select {
		case <-t.C:
			return w.writeClose()
		default:
		}
	}

	return nil
}

func (w *wsConnection) handleOutgoingMessage() error {
	t := w.loopTimer
	t.Reset(wsHandleOutgoingMsgDuration)

	for {
		if err := w.handleOneMessage(); err != nil {
			return err
		}

		select {
		case <-t.C:
			return nil
		default:
		}
	}

	return nil
}

func (w *wsConnection) handleOneMessage() error {
	client := w.client
	codec := w.codec
	outgoing := w.outgoing
	timer := w.timer
	timer.Reset(wsTakeMsgDuration)

	msg := client.recver.takeMessage(timer)
	if msg == nil {
		return nil
	}

	data, err := codec.Pack(msg)
	if err != nil {
		client.close()
		return err
	}
	outgoing.Push(data)
	return nil
}

func (w *wsConnection) tryWrite() error {
	outgoing := w.outgoing
	if outgoing.IsEmpty() {
		return nil
	}

	t := w.loopTimer
	t.Reset(wsTryWriteDuration)

	for !outgoing.IsEmpty() {
		if err := w.writeOnce(); err != nil {
			return err
		}

		select {
		case <-t.C:
			return nil
		default:
		}
	}

	return nil
}

func (w *wsConnection) writeOnce() error {
	outgoing := w.outgoing
	if outgoing.IsEmpty() {
		return nil
	}

	client := w.client
	conn := w.conn
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	writer, err := conn.NextWriter(ws.BinaryMessage)
	if err != nil {
		log.Println(err)
		client.close()
		return err
	}
	data, err := outgoing.Peek()
	if err != nil {
		log.Println(err)
		client.close()
		return err
	}
	s, ok := data.([]byte)
	if !ok {
		log.Println(err)
		client.close()
		return err
	}
	if _, err := writer.Write(s); err != nil {
		log.Println(err)
		client.close()
		return err
	}
	if err := writer.Close(); err != nil {
		log.Println(err)
		client.close()
		return err
	}
	outgoing.Retrieve()
	return nil
}

func (w *wsConnection) writePing() error {
	conn := w.conn

	select {
	case <-w.ticker.C:
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(ws.PingMessage, []byte{}); err != nil {
			log.Println(err)
			w.client.close()
			return err
		}
	default:
	}

	return nil
}

func (w *wsConnection) writeClose() error {
	conn := w.conn
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(ws.CloseMessage, []byte{}); err != nil {
		log.Println(err)
		w.client.close()
		return err
	}

	return nil
}
