package main

import (
//proto "biblio/protocol"
//protojson "biblio/protocol/json"
//"log"
)

// PlayerBaseModule manages player base data
type PlayerBaseModule struct {
	PlayerModule
}

func newPlayerBaseModule(p *Player) *PlayerBaseModule {
	return &PlayerBaseModule{
		PlayerModule: PlayerModule{player: p},
	}
}

func (m *PlayerBaseModule) checkLoad() {
	if !m.player.playerBaseData.loaded {
		// TODO: load data
	}
}

func (m *PlayerBaseModule) handle(msg *message) {
	m.checkLoad()

	switch msg.protoID {

	}
}
