package main

import (
	proto "biblio/protocol"
	"biblio/util"
	"errors"
	"sync"
	atom "sync/atomic"
	"time"
)

var errBindNilClient = errors.New("bind nil client")
var errBindSameClient = errors.New("bind same client")

var mapProtocol2PlayerHandler = map[int16](func(*Player, *message)){
	proto.C2SHeartbeatID: func(player *Player, msg *message) {
		player.playerHeartbeatModule.handle(msg)
	},
}

func dispatchMessageToPlayer(player *Player, msg *message) {
	if fn, ok := mapProtocol2PlayerHandler[msg.protoID]; ok {
		fn(player, msg)
	}
}

// Player wraps a game player. Zero value is invalid.
type Player struct {
	muxState sync.Mutex
	state    playerState

	bindReqs   chan *bindReqToPlayer
	unbindReqs chan bool

	recver *messageBuffer // take message from recver
	sender *messageBuffer // add message to sender

	toStop  int32
	running int32

	unloadFlagGuard int32
	unloadFlag      chan bool

	playerBaseData   *PlayerBaseData
	playerBaseModule *PlayerBaseModule

	playerHeartbeatModule *PlayerHeartbeatModule
}

func newPlayer(r *messageBuffer, s *messageBuffer) *Player {
	p := &Player{
		bindReqs:       make(chan *bindReqToPlayer),
		unbindReqs:     make(chan bool),
		recver:         r,
		sender:         s,
		unloadFlag:     make(chan bool),
		playerBaseData: &PlayerBaseData{},
	}
	p.state = newPlayerStateOffline(p)
	p.playerBaseModule = newPlayerBaseModule(p)
	p.playerHeartbeatModule = newPlayerHeartbeatModule(p)
	return p
}

func (p *Player) setState(s playerState) {
	p.state = s
}

// @public
func (p *Player) onBind() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onBind()
}

// @public
func (p *Player) onBindSuccess() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onBindSuccess()
}

// @public
func (p *Player) onKick() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onKick()
}

// @public
func (p *Player) onKickSuccess() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onKickSuccess()
}

// @public
func (p *Player) onUnload() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onUnload()
}

// @public
func (p *Player) isOnline() bool {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	return p.state.isOnline()
}

// @public
func (p *Player) onHeartbeat() {
	p.muxState.Lock()
	defer p.muxState.Unlock()
	p.state.onHeartbeat()
}

func (p *Player) reqBind(req *bindReqToPlayer) bool {
	select {
	case p.bindReqs <- req:
		return true
	default:
		return false
	}
}

func (p *Player) reqUnbind() bool {
	select {
	case p.unbindReqs <- true:
		return true
	default:
		return false
	}
}

// TODO: after a player created, invoke startBinder
func (p *Player) startBinder() {
	serverInst.wgAddOne()
	go p.doBind()
}

func (p *Player) doBind() {
	defer serverInst.wgDone()

	d := 50 * time.Millisecond
	t := time.NewTimer(d)

	for {
		if needQuit() {
			break
		}
		if p.needUnload() {
			break
		}

		t.Reset(d)
		select {
		case req := <-p.bindReqs:
			p.bind(req)
		case <-p.unbindReqs:
			p.unbind()
		case <-t.C:
		}
	}
}

func (p *Player) bind(req *bindReqToPlayer) {
	p.onBind()
	p.setToStop(true)

	// d := time.Millisecond
	// maxT := req.endTime.Sub(time.Now())
	// var loopCnt int
	// if maxT > 0 {
	// 	loopCnt = int(maxT) / int(d)
	// }
	// var t *time.Ticker
	// if loopCnt > 0 {
	// 	t = time.NewTicker(d)
	// }
	// var succ bool

	// for i := 0; i < loopCnt; i++ {
	// 	if needQuit() {
	// 		break
	// 	}
	// 	if p.needUnload() {
	// 		break
	// 	}

	// 	if !p.isRunning() {
	// 		succ = true
	// 		break
	// 	}

	// 	select {
	// 	case <-t.C:
	// 	}
	// }

	var succ bool

	d1 := 1 * time.Millisecond
	t1 := time.NewTimer(d1)
	d := d1
	d3 := 50 * time.Millisecond

	d2 := 5 * time.Second
	t2 := time.NewTimer(d2)

	for {
		if needQuit() {
			break
		}
		if p.needUnload() {
			break
		}

		if !p.isRunning() {
			succ = true
			break
		}

		t1.Reset(d)
		select {
		case <-t1.C:
		case <-t2.C:
			d = d3
		}
	}

	if succ {
		if p.recver != nil {
			p.recver.notifyClose()
		}
		p.sendMessageAnyway(messageCreater.createS2CClose(util.AnotherClientConnected))
		if p.sender != nil {
			p.sender.notifyClose()
		}

		p.recver = req.recverForPlayer
		p.sender = req.senderForPlayer
		p.setToStop(false)
		p.start()
		p.onBindSuccess()
	}
}

func (p *Player) notifyBindSuccess() {
	p.sender.notifyBindSuccess()
}

func (p *Player) unbind() {
	p.onKick()
	p.setToStop(true)

	// d := time.Millisecond
	// maxT := 5 * time.Second
	// loopCnt := int(maxT / d)
	// t := time.NewTicker(d)
	// var succ bool

	// for i := 0; i < loopCnt; i++ {
	// 	if needQuit() {
	// 		break
	// 	}
	// 	if p.needUnload() {
	// 		break
	// 	}

	// 	if !p.isRunning() {
	// 		succ = true
	// 		break
	// 	}

	// 	select {
	// 	case <-t.C:
	// 	}
	// }

	var succ bool

	d1 := 1 * time.Millisecond
	t1 := time.NewTimer(d1)
	d := d1
	d3 := 50 * time.Millisecond

	d2 := 5 * time.Second
	t2 := time.NewTimer(d2)

	for {
		if needQuit() {
			break
		}
		if p.needUnload() {
			break
		}

		if !p.isRunning() {
			succ = true
			break
		}

		t1.Reset(d)
		select {
		case <-t1.C:
		case <-t2.C:
			d = d3
		}
	}

	if succ {
		if p.recver != nil {
			p.recver.notifyClose()
		}
		p.sendMessageAnyway(messageCreater.createS2CClose(util.HeartbeatTimeout))
		if p.sender != nil {
			p.sender.notifyClose()
		}

		p.recver = nil
		p.sender = nil
		p.setToStop(false)
		p.onKickSuccess()
	}
}

func (p *Player) setToStop(b bool) {
	var v int32
	if b {
		v = 1
	}
	atom.StoreInt32(&p.toStop, v)
}

func (p *Player) needStop() bool {
	return atom.LoadInt32(&p.toStop) == 1
}

func (p *Player) setRunning(r bool) {
	var v int32
	if r {
		v = 1
	}
	atom.StoreInt32(&p.running, v)
}

func (p *Player) isRunning() bool {
	return atom.LoadInt32(&p.running) == 1
}

func (p *Player) notifyUnload() {
	if atom.CompareAndSwapInt32(&p.unloadFlagGuard, 0, 1) {
		close(p.unloadFlag)
	}
}

func (p *Player) needUnload() bool {
	select {
	case <-p.unloadFlag:
		return true
	default:
		return false
	}
}

func (p *Player) start() {
	p.setRunning(true)
	serverInst.wgAddOne()
	go p.run()
}

func (p *Player) run() {
	defer serverInst.wgDone()
	defer func() {
		p.setRunning(false)
	}()

	delay := 50 * time.Millisecond
	t := time.NewTimer(delay)

	for {
		if needQuit() {
			break
		}
		if p.needUnload() {
			break
		}
		if p.needStop() {
			break
		}

		t.Reset(delay)
		p.handleMsg(t)
	}
}

func (p *Player) handleMsg(t *time.Timer) {
	for {
		if needQuit() {
			break
		}
		if p.needUnload() {
			break
		}
		if p.needStop() {
			break
		}

		msg := p.recver.takeMessage(t)
		if msg == nil {
			break
		}

		dispatchMessageToPlayer(p, msg)
		messageCodec.Release(msg.protoID, msg.proto)
	}
}

func (p *Player) sendProto(protoID int16, proto interface{}) {
	p.sender.addMessage(&message{protoID, proto})
}

func (p *Player) sendMessageAnyway(msg *message) {
	if p.sender != nil {
		p.sender.addMessage(msg)
	}
}

func (p *Player) uid() int64 {
	return p.playerBaseData.uid
}
