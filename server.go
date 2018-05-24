package main

import (
	twmm "github.com/ZhangGuangxu/timingwheelmm"
	"log"
	"net"
	"sync"
	"time"
)

var maxConnectionCount int
var serverAddress string // "ip:port", for example: "127.0.0.1:10001", or ":10001"
var messageCodec Codec

var clientWaitAuthMaxTime = 5 * time.Second
var bindProcessMaxTime = 5 * time.Second // 收到客户端的auth请求后，要把client bind到player，多久后未完成认为处理超时
var kickProcessMaxTime = 5 * time.Second // kick player多久后未完成认为处理超时
var heartbeatTime = 7 * time.Second
var playerKickTime = 2 * heartbeatTime
var playerUnloadTime = 10 * time.Minute

func init() {
	maxConnectionCount = 2000
	serverAddress = "127.0.0.1:59632"
	messageCodec = newJSONCodec()
}

// Server wrap a server
type Server struct {
	muxp    sync.Mutex
	players map[int64]*Player

	twPlayerKick   *twmm.TimingWheel
	twPlayerUnload *twmm.TimingWheel

	muxc    sync.Mutex
	clients map[*Client]bool

	twClient *twmm.TimingWheel // 用于等待客户端发送auth消息超时

	muxx      sync.Mutex
	xBindReqs map[int64]interface{}

	newXBindReqAdd chan bool

	wg *sync.WaitGroup
}

// NewServer returns a server instance.
// You MUST use this function to create a server instance.
func NewServer() (*Server, error) {
	s := &Server{
		players:        make(map[int64]*Player, 1000),
		clients:        make(map[*Client]bool, maxConnectionCount/10),
		xBindReqs:      make(map[int64]interface{}, 100),
		newXBindReqAdd: make(chan bool, 1),
		wg:             &sync.WaitGroup{},
	}

	var err error
	s.twClient, err = twmm.NewTimingWheel(clientWaitAuthMaxTime, 50)
	if err != nil {
		return nil, err
	}
	s.twPlayerKick, err = twmm.NewTimingWheel(playerKickTime, 140)
	if err != nil {
		return nil, err
	}
	s.twPlayerUnload, err = twmm.NewTimingWheel(playerUnloadTime, 100)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Server) addPlayer(uid int64, p *Player) {
	b.muxp.Lock()
	defer b.muxp.Unlock()
	b.players[uid] = p
}

func (b *Server) addPlayerToKick(p *Player) {
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

	b.twClient.AddItem(c)
}

func (b *Server) authReceived(c *Client) {
	b.twClient.DelItem(c)
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

		for {
			select {
			case <-getQuit():
				ln.Close()
				return
			}
		}
	}()

	b.startDoBind()

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
				client := newClient(conn)
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
