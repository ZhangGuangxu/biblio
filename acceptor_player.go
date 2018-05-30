package main

import (
	"log"
	"net"
	"time"
)

// playerAcceptor accepts connection requests from player-clients.
type playerAcceptor struct {
}

func (a *playerAcceptor) start(b *Server) {
	tcpaddr, err := net.ResolveTCPAddr("tcp", serverAddress)
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
					setQuit()
					break
				}
			}

			if b.clientCount() >= maxConnectionCount {
				conn.Close()
				time.Sleep(50 * time.Millisecond)
			} else {
				client := newClient()
				client.setConn(newTCPConnection(conn))
				client.start()
				b.addClient(client)
			}
		}
	}()
}
