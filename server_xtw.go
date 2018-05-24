package main

import (
	"log"
)

func (b *Server) startClientTimingWheel() {
	b.wgAddOne()
	go b.twClient.Run(getQuit, func() {
		log.Println("client timingwheel quit")
		b.wgDone()
	})
}

type playerKickItem struct {
	p *Player
}

func (item *playerKickItem) Release() {
	serverInst.kickPlayer(item.p.uid())
	serverInst.waitToUnload(item.p)
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
	if item.p.isOnline() {
		return
	}
	serverInst.removePlayer(item.p)
	item.p.unload()
}

func (b *Server) startPlayerUnloadTimingWheel() {
	b.wgAddOne()
	go b.twPlayerUnload.Run(getQuit, func() {
		log.Println("player unload timingwheel quit")
		b.wgDone()
	})
}
