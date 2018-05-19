package main

import (
	"log"
	"net"
	"sync"
	atom "sync/atomic"
	"time"
)

var maxConnectionCount int
var serverAddress string // "ip:port", for example: "127.0.0.1:10001", or ":10001"

func init() {
	maxConnectionCount = 2000
	serverAddress = "127.0.0.1:59632"
}

// Server wrap a server
type Server struct {
	tmpPlayers map[*Player]bool
	players    map[int64]*Player
}

// NewServer returns a server instance.
// You MUST use this function to create a server instance.
func NewServer() *Server {
	return &Server{
		players: make(map[int64]*Player, 1000),
	}
}

func (b *Server) addTmpPlayer(p *Player) {
	b.tmpPlayers[p] = true
}

func (b *Server) delTmpPlayer(p *Player) {
	delete(b.tmpPlayers, p)
}

func (b *Server) addPlayer(uid int64, p *Player) {
	b.delTmpPlayer(p)
	b.players[uid] = p
}

func (b *Server) run(quit *uint32, done chan<- bool) {
	defer func() { done <- true }()
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

	go func() {
		for {
			if q := atom.LoadUint32(quit); q == 1 {
				ln.Close()
				return
			}

			select {
			case <-time.After(50 * time.Millisecond):
				// nothing to do
			}
		}
	}()

	var wg sync.WaitGroup
	clients := make(map[*Client]bool, maxConnectionCount)

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			if q := atom.LoadUint32(quit); q == 1 {
				break
			} else {
				log.Println(err)
				continue
			}
		}

		if c := len(clients); c >= maxConnectionCount {
			conn.Close()
			time.Sleep(50 * time.Millisecond)
		} else {
			client := newClient(conn, newJSONCodec())
			clients[client] = true
			wg.Add(1)
			go client.handleRead(quit, &wg)
			wg.Add(1)
			go client.handleWrite(quit, &wg)

			// TODO: 这部分代码应该是在与web服务器交互的时候创建出来的
			//       tmpPlayer也是临时代码
			player := newPlayer(client)
			b.addTmpPlayer(player)
			wg.Add(1)
			go player.run(quit)
			//go echoFunc(conn, quit, &clientCount)
		}
	}

	wg.Wait()
}
