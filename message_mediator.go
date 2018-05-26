package main

import "time"

type messageMediator interface {
	playerEventNotifier
	clientEventNotifier
	addMessage(msg *message)
	takeMessage(timer *time.Timer) *message
	start()
}

type playerEventNotifier interface {
	notifyClose()
	shouldClose() bool
	notifyBindSuccess()
	isBindSuccess() bool
}

type clientEventNotifier interface {
	notifyClientWriteClosed()
	isClientWriteClosed() bool
	notifyClientReadClosed()
	isClientReadClosed() bool
}
