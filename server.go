package main

import (
	twm "github.com/ZhangGuangxu/timingwheelm"
	"log"
	"net"
	"sync"
	"time"
)

var maxConnectionCount int
var serverAddress string // "ip:port", for example: "127.0.0.1:10001", or ":10001"
var codec Codec

var clientWaitAuthMaxTime = 5 * time.Second
var heartbeatTime = 7 * time.Second
var playerKickTime = 2 * heartbeatTime
var playerUnloadTime = 10 * time.Minute

func init() {
	maxConnectionCount = 2000
	serverAddress = "127.0.0.1:59632"
	codec = newJSONCodec()
}

type bindReq struct {
	uid    int64
	client *Client
}

// Server wrap a server
type Server struct {
	muxp    sync.Mutex
	players map[int64]*Player

	twPlayerKick   *twm.TimingWheel
	twPlayerUnload *twm.TimingWheel

	muxc    sync.Mutex
	clients map[*Client]bool

	twClient *twm.TimingWheel

	bindReqs chan *bindReq

	wg *sync.WaitGroup
}

// NewServer returns a server instance.
// You MUST use this function to create a server instance.
func NewServer() (*Server, error) {
	s := &Server{
		players:  make(map[int64]*Player, 1000),
		clients:  make(map[*Client]bool, maxConnectionCount/10),
		bindReqs: make(chan *bindReq, 100),
		wg:       &sync.WaitGroup{},
	}

	var err error
	s.twClient, err = twm.NewTimingWheel(clientWaitAuthMaxTime, 50)
	if err != nil {
		return nil, err
	}
	s.twPlayerKick, err = twm.NewTimingWheel(playerKickTime, 140)
	if err != nil {
		return nil, err
	}
	s.twPlayerUnload, err = twm.NewTimingWheel(playerUnloadTime, 100)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Server) addPlayer(uid int64, p *Player) {
	b.muxp.Lock()
	b.players[uid] = p
	b.muxp.Unlock()

	b.twPlayerKick.AddItem(&playerKickItem{p})
}

func (b *Server) waitToUnload(p *Player) {
	b.twPlayerUnload.AddItem(&playerUnloadItem{p})
}

func (b *Server) removePlayer(p *Player) {
	b.muxp.Lock()
	defer b.muxp.Unlock()
	delete(b.players, p.uid())
}

func (b *Server) addClient(c *Client) {
	b.muxc.Lock()
	b.clients[c] = true
	b.muxc.Unlock()

	b.twClient.AddItem(newClientItem(c))
}

func (b *Server) removeClient(c *Client) {
	b.muxc.Lock()
	defer b.muxc.Unlock()
	delete(b.clients, c)
}

func (b *Server) clientCount() int {
	b.muxc.Lock()
	defer b.muxc.Unlock()
	return len(b.clients)
}

func (b *Server) reqBind(uid int64, c *Client) {
	b.bindReqs <- &bindReq{uid, c}
}

func (b *Server) doBind() {
	defer b.wgDone()
	defer log.Println("doBind quit")

	delay := 50 * time.Millisecond
	t := time.NewTimer(delay)

	for {
		if needQuit() {
			break
		}

		t.Reset(delay)
		select {
		case req := <-b.bindReqs:
			b.bind(req)
		case <-t.C:
		}
	}
}

func (b *Server) bind(req *bindReq) {
	b.muxp.Lock()
	defer b.muxp.Unlock()
	b.muxc.Lock()
	defer b.muxc.Unlock()

	p, ok := b.players[req.uid]
	if !ok {
		return
	}

	_, ok = b.clients[req.client]
	if !ok {
		return
	}

	if err := p.bindClient(req.client); err == errBindSameClient {
		req.client.close()
	}
}

func (b *Server) run(rwg *sync.WaitGroup) {
	defer rwg.Done()
	defer log.Println("Server.run() quit")

	tcpaddr, err := net.ResolveTCPAddr("tcp4", serverAddress)
	if err != nil {
		log.Println(err)
		return
	}

	ln, err := net.ListenTCP("tcp", tcpaddr)
	if err != nil {
		log.Println(err)
		return
	}

	b.wgAddOne()
	go func() {
		defer b.wgDone()
		defer log.Println("listener closer quit")

		delay := 50 * time.Millisecond
		t := time.NewTimer(delay)

		for {
			if needQuit() {
				ln.Close()
				return
			}

			t.Reset(delay)
			select {
			case <-t.C:
			}
		}
	}()

	b.wgAddOne()
	go b.doBind()

	b.startClientTimingWheel()
	b.startPlayerKickTimingWheel()
	b.startPlayerUnloadTimingWheel()

	b.wgAddOne()
	go func() {
		defer b.wgDone()
		defer log.Println("accepter quit")

		for {
			conn, err := ln.AcceptTCP()
			if err != nil {
				if needQuit() {
					break
				} else {
					log.Println(err)
					continue
				}
			}

			if b.clientCount() >= maxConnectionCount {
				conn.Close()
				time.Sleep(50 * time.Millisecond)
			} else {
				client := newClient(conn, codec)
				b.addClient(client)
				b.wgAdd(2)
				go client.handleRead()
				go client.handleWrite()
			}
		}
	}()

	auther.startTimingWheel()

	// TODO: 为web server提供服务

	b.wg.Wait()
}

func (b *Server) wgAddOne() {
	b.wg.Add(1)
}

func (b *Server) wgAdd(delta int) {
	b.wg.Add(delta)
}

func (b *Server) wgDone() {
	b.wg.Done()
}

type clientItem struct {
	c          *Client
	createTime time.Time
}

func newClientItem(c *Client) *clientItem {
	return &clientItem{
		c:          c,
		createTime: time.Now(),
	}
}

func (item *clientItem) ShouldRelease() bool {
	return time.Now().Sub(item.createTime) > clientWaitAuthMaxTime
}

func (item *clientItem) Release() {
	serverInst.removeClient(item.c)
	item.c.close()
}

func (b *Server) startClientTimingWheel() {
	b.wgAddOne()
	go b.twClient.Run(needQuit, func() {
		log.Println("client timingwheel quit")
		b.wgDone()
	})
}

type playerKickItem struct {
	p *Player
}

func (item *playerKickItem) ShouldRelease() bool {
	return time.Now().Sub(item.p.lastHeartbeatTime()) > playerKickTime
}

func (item *playerKickItem) Release() {
	item.p.kick()
	serverInst.waitToUnload(item.p)
}

func (b *Server) startPlayerKickTimingWheel() {
	b.wgAddOne()
	go b.twPlayerKick.Run(needQuit, func() {
		log.Println("player kick timingwheel quit")
		b.wgDone()
	})
}

type playerUnloadItem struct {
	p *Player
}

func (item *playerUnloadItem) ShouldRelease() bool {
	if item.p.isOnline() {
		return false
	}
	return time.Now().Sub(item.p.lastHeartbeatTime()) > playerUnloadTime
}

func (item *playerUnloadItem) Release() {
	serverInst.removePlayer(item.p)
	item.p.unload()
}

func (b *Server) startPlayerUnloadTimingWheel() {
	b.wgAddOne()
	go b.twPlayerUnload.Run(needQuit, func() {
		log.Println("player unload timingwheel quit")
		b.wgDone()
	})
}
