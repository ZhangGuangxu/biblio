package main

import (
	"log"
)

// 用于处理“auth消息在指定超时时间前未收到”
type clientAuthTimeoutItem struct {
	c *Client
}

func (item *clientAuthTimeoutItem) Release() {
	item.c.onTimeout()
}

func (b *Server) startClientTimingWheel() {
	b.wgAddOne()
	go b.twClient.Run(getQuit, func() {
		log.Println("client timingwheel quit")
		b.wgDone()
	})
}

type clientBindingTimeoutItem struct {
	c *Client
}

func (item *clientBindingTimeoutItem) Release() {
	item.c.onTimeout()
}

func (b *Server) startClientBindingTimingWheel() {
	b.wgAddOne()
	go b.twClientBinding.Run(getQuit, func() {
		log.Println("client binding timingwheel quit")
		b.wgDone()
	})
}

type clientBindedTimeoutItem struct {
	c *Client
}

func (item *clientBindedTimeoutItem) Release() {
	item.c.onTimeout()
}

func (b *Server) startClientBindedTimingWheel() {
	b.wgAddOne()
	go b.twClientBinded.Run(getQuit, func() {
		log.Println("client binded timingwheel quit")
		b.wgDone()
	})
}

type playerKickItem struct {
	p *Player
}

func (item *playerKickItem) Release() {
	item.p.onKick()
}

func (b *Server) startPlayerKickTimingWheel() {
	b.wgAddOne()
	go b.twPlayerKick.Run(getQuit, func() {
		log.Println("player kick timingwheel quit")
		b.wgDone()
	})
}

type playerUnloadItem struct {
	p *Player
}

func (item *playerUnloadItem) Release() {
	item.p.onUnload()
}

func (b *Server) startPlayerUnloadTimingWheel() {
	b.wgAddOne()
	go b.twPlayerUnload.Run(getQuit, func() {
		log.Println("player unload timingwheel quit")
		b.wgDone()
	})
}
