package main

import (
	proto "biblio/protocol"
	"sync"
	//protojson "biblio/protocol/json"
	"time"
	//"log"
)

// PlayerHeartbeatModule handles heart beat
type PlayerHeartbeatModule struct {
	PlayerModule

	mux      sync.Mutex
	lastTime time.Time
}

func newPlayerHeartbeatModule(p *Player) *PlayerHeartbeatModule {
	return &PlayerHeartbeatModule{
		PlayerModule: PlayerModule{player: p},
	}
}

func (m *PlayerHeartbeatModule) getLastTime() time.Time {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.lastTime
}

func (m *PlayerHeartbeatModule) setLastTime(t time.Time) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.lastTime = t
}

func (m *PlayerHeartbeatModule) handle(msg *message) {
	switch msg.protoID {
	case proto.C2SHeartbeatID:
		m.handleHeartbeat(msg)
	}
}

func (m *PlayerHeartbeatModule) handleHeartbeat(msg *message) {
	m.setLastTime(time.Now())
}
