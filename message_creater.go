package main

var messageCreater = jsonCreater

// MessageCreater defines some methods to create different kinds of messages.
type MessageCreater interface {
	createS2CAuth(passed bool) *message
	createS2CClose(reason int8) *message
}
