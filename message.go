package main

// message is a wrapper of protocol instance
type message struct {
	protoID int16
	proto   interface{}
}
