package main

import (
	"log"
	"net"
	"sync"
	"time"
)

var maxConnectionCount int
var serverAddress string // "ip:port", for example: "127.0.0.1:10001", or ":10001"
var codec Codec

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

	muxc    sync.Mutex
	clients map[*Client]bool

	bindReqs chan *bindReq

	wg *sync.WaitGroup
}

// NewServer returns a server instance.
// You MUST use this function to create a server instance.
func NewServer() *Server {
	return &Server{
		players:  make(map[int64]*Player, 1000),
		clients:  make(map[*Client]bool, maxConnectionCount/10),
		bindReqs: make(chan *bindReq, 1000),
		wg:       &sync.WaitGroup{},
	}
}

func (b *Server) addPlayer(uid int64, p *Player) {
	b.muxp.Lock()
	defer b.muxp.Unlock()
	b.players[uid] = p
}

func (b *Server) addClient(c *Client) {
	b.muxc.Lock()
	defer b.muxc.Unlock()
	b.clients[c] = true
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

	b.wgAddOne()
	go auther.startTimingWheel()

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
