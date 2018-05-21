package main

import (
	proto "biblio/protocol"
	//protojson "biblio/protocol/json"
	"time"
	//"log"
)

// PlayerHeartbeatModule handles heart beat
type PlayerHeartbeatModule struct {
	PlayerModule
	lastTime time.Time
}

func newPlayerHeartbeatModule(p *Player) *PlayerHeartbeatModule {
	return &PlayerHeartbeatModule{
		PlayerModule: PlayerModule{player: p},
	}
}

func (m *PlayerHeartbeatModule) getLastTime() time.Time {
	return m.lastTime
}

func (m *PlayerHeartbeatModule) handle(msg *message) {
	switch msg.protoID {
	case proto.C2SHeartbeatID:
		m.handleHeartbeat(msg)
	}
}

func (m *PlayerHeartbeatModule) handleHeartbeat(msg *message) {
	m.lastTime = time.Now()
}
