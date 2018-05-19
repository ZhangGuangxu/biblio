package main

// message is a wrapper of protocol instance
type message struct {
	protoID int16
	proto   interface{}
}

func dispatchMessage(player *Player, msg *message) {
	if fn, ok := mapProtocol2Handler[msg.protoID]; ok {
		fn(player, msg)
	}
}
