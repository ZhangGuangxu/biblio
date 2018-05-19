package main

import (
	proto "biblio/protocol"
	"time"
)

var mapProtocol2Handler = map[int16](func(*Player, *message)){
	proto.C2SLoginID: func(player *Player, msg *message) {
		player.playerBaseModule.handle(msg)
	},
}

// Player wraps a game player
type Player struct {
	online bool
	client *Client

	playerBaseData   *PlayerBaseData
	playerBaseModule *PlayerBaseModule
}

func newPlayer(c *Client) *Player {
	p := &Player{
		client:         c,
		playerBaseData: &PlayerBaseData{},
	}

	p.playerBaseModule = newPlayerBaseModule(p)
	return p
}

func (p *Player) isOnline() bool {
	if !p.online {
		return p.online
	}

	if p.client.isClosed() {
		p.online = false
	}

	return p.online
}

func (p *Player) run(quit *uint32) {
	timer := time.NewTimer(0 * time.Second)
	d := 5 * time.Millisecond

	for {
		if needQuit() {
			break
		}

		p.client.handleAllIncomingMessage(p)

		timer.Reset(d)
		select {
		case <-timer.C:
		}
	}
}
