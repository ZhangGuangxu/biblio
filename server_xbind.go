package main

import (
	"log"
	"time"
)

type bindReq struct {
	uid        int64
	client     *Client
	createTime time.Time
}

func newBindReq(uid int64, c *Client) *bindReq {
	return &bindReq{
		uid:        uid,
		client:     c,
		createTime: time.Now(),
	}
}

type bindReqToPlayer struct {
	recverForPlayer *messageBuffer
	senderForPlayer *messageBuffer
	endTime         time.Time
}

func newBindReqToPlayer(rP *messageBuffer, sP *messageBuffer, beginTime time.Time) *bindReqToPlayer {
	return &bindReqToPlayer{
		recverForPlayer: rP,
		senderForPlayer: sP,
		endTime:         beginTime.Add(bindProcessMaxTime),
	}
}

type unbindReq struct {
	uid        int64
	createTime time.Time
}

func newUnbindReq(uid int64) *unbindReq {
	return &unbindReq{
		uid:        uid,
		createTime: time.Now(),
	}
}

func (b *Server) reqBind(uid int64, c *Client) {
	if b.doReqBind(uid, c) {
		b.setNewXBindReqAdded()
	}
}

func (b *Server) doReqBind(uid int64, c *Client) bool {
	b.muxx.Lock()
	defer b.muxx.Unlock()

	if v, ok := b.xBindReqs[uid]; ok {
		if v == nil {
			b.xBindReqs[uid] = newBindReq(uid, c)
			return true
		}

		if _, ok := v.(*bindReq); ok {
			b.xBindReqs[uid] = newBindReq(uid, c)
			return true
		} else if _, ok := v.(*unbindReq); ok {
			b.xBindReqs[uid] = newBindReq(uid, c)
			return true
		}
	}

	return false
}

func (b *Server) kickPlayer(uid int64) {
	if b.doKickPlayer(uid) {
		b.setNewXBindReqAdded()
	}
}

func (b *Server) doKickPlayer(uid int64) bool {
	b.muxx.Lock()
	defer b.muxx.Unlock()

	if v, ok := b.xBindReqs[uid]; ok {
		if v == nil {
			b.xBindReqs[uid] = newUnbindReq(uid)
			return true
		}

		if _, ok := v.(*bindReq); ok {
			// do nothing
		} else if _, ok := v.(*unbindReq); ok {
			// do nothing
		}
	}

	return false
}

func (b *Server) setNewXBindReqAdded() {
	select {
	case b.newXBindReqAdd <- true:
	default:
	}
}

func (b *Server) hasXBindReq() bool {
	b.muxx.Lock()
	defer b.muxx.Unlock()
	return len(b.xBindReqs) > 0
}

func (b *Server) startDoBind() {
	b.wgAddOne()
	go b.doBind()
}

func (b *Server) doBind() {
	defer b.wgDone()
	defer log.Println("doBind quit")

	d1 := 5 * time.Millisecond
	d2 := 50 * time.Millisecond
	t := time.NewTimer(d2)

	for {
		if needQuit() {
			break
		}

		if b.hasXBindReq() {
			t.Reset(d1)
			b.loopBind(t)
		} else {
			t.Reset(d2)
			b.waitBind(t)
		}
	}
}

func (b *Server) loopBind(timer *time.Timer) {
	b.muxx.Lock()
	defer b.muxx.Unlock()

	for k, v := range b.xBindReqs {
		if v != nil {
			if b.xbind(v) {
				delete(b.xBindReqs, k)
			}
		} else {
			delete(b.xBindReqs, k)
		}

		select {
		case <-timer.C:
			return
		default:
		}
	}
}

func (b *Server) waitBind(timer *time.Timer) {
	select {
	case <-timer.C:
	case <-b.newXBindReqAdd:
	}
}

func (b *Server) xbind(req interface{}) bool {
	if bReq, ok := req.(*bindReq); ok {
		return b.bind(bReq)
	} else if uReq, ok := req.(*unbindReq); ok {
		return b.unbind(uReq)
	}

	return true
}

func (b *Server) bind(req *bindReq) bool {
	if time.Now().Sub(req.createTime) > bindProcessMaxTime {
		return true
	}

	b.muxp.Lock()
	defer b.muxp.Unlock()
	b.muxc.Lock()
	defer b.muxc.Unlock()

	p, ok := b.players[req.uid]
	if !ok {
		return true
	}

	_, ok = b.clients[req.client]
	if !ok {
		return true
	}

	return p.reqBind(newBindReqToPlayer(req.client.sender, req.client.recver, req.createTime))
}

func (b *Server) unbind(req *unbindReq) bool {
	if time.Now().Sub(req.createTime) > kickProcessMaxTime {
		return true
	}

	b.muxp.Lock()
	defer b.muxp.Unlock()

	p, ok := b.players[req.uid]
	if !ok {
		return true
	}

	return p.reqUnbind()
}
