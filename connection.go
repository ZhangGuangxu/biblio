package main

type connection interface {
	setParent(interface{})
	handleRead()
	handleWrite()
	handleAuth(*message)
}
