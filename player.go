package main

import (
	proto "biblio/protocol"
	"errors"
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

// Player wraps a game player
type Player struct {
	client *Client

	running    int32
	unloadFlag chan bool

	playerBaseData   *PlayerBaseData
	playerBaseModule *PlayerBaseModule

	playerHeartbeatModule *PlayerHeartbeatModule
}

func newPlayer(c *Client) *Player {
	p := &Player{
		running:        0,
		unloadFlag:     make(chan bool),
		playerBaseData: &PlayerBaseData{},
	}

	p.playerBaseModule = newPlayerBaseModule(p)
	p.playerHeartbeatModule = newPlayerHeartbeatModule(p)
	return p
}

func (p *Player) setRunning(r bool) {
	var v int32
	if r {
		v = 1
	}
	atom.StoreInt32(&p.running, v)
}

func (p *Player) isRunning() bool {
	if atom.LoadInt32(&p.running) == 1 {
		return true
	}
	return false
}

func (p *Player) isOnline() bool {
	return !p.client.isClosed()
}

// unload make sure it just get invoked once.
func (p *Player) unload() {
	close(p.unloadFlag)
}

func (p *Player) needUnload() bool {
	select {
	case _, ok := <-p.unloadFlag:
		if !ok {
			return true
		}
	default:
		return false
	}

	return false
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

		t.Reset(delay)
		if p.client.hasIncomingMessage(t) {
			p.client.handleAllIncomingMessage(p)
		}
		if !p.isOnline() {
			break
		}
	}
}

func (p *Player) sendProto(protoID int16, proto interface{}) {
	if p.isOnline() {
		p.client.addOutgoingMessage(protoID, proto)
	}
}

func (p *Player) start(c *Client) {
	p.client = c
	p.setRunning(true)
	serverInst.wgAddOne()
	go p.run()
}

func (p *Player) bindClient(c *Client) error {
	if c == nil {
		return errBindNilClient
	}

	delay := 50 * time.Millisecond

	if p.client == nil {
		p.start(c)
	} else {
		if p.client != c {
			p.client.closeRead()

			// TODO: 发送一个proto，通知玩家“账号在其它客户端登录，当前客户端被迫下线”

			p.client.delayCloseWrite(delay)
		} else {
			return errBindSameClient
		}

		serverInst.wgAddOne()
		go func() {
			defer serverInst.wgDone()

			for {
				if needQuit() {
					break
				}
				if p.needUnload() {
					break
				}
				if !p.isRunning() {
					p.start(c)
					break
				}

				time.Sleep(delay)
			}
		}()
	}

	return nil
}
