package main

import (
	ws "github.com/gorilla/websocket"
	"log"
	"net/http"
)

type wsAcceptor struct {
	upgrader ws.Upgrader
}

func (a *wsAcceptor) doUpgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	if serverInst.clientCount() >= maxConnectionCount {
		conn.Close()
	} else {
		client := newClient()
		client.setConn(newWSConnection(conn))
		client.start()
		serverInst.addClient(client)
	}
}

func (a *wsAcceptor) start(b *Server) {
	httpServer := &http.Server{Addr: wsAddress, Handler: nil}

	b.wgAddOne()
	go func() {
		defer b.wgDone()
		defer log.Println("http server closer quit")

		for {
			select {
			case <-getQuit():
				httpServer.Close()
				return
			}
		}
	}()

	b.wgAddOne()
	go func() {
		defer b.wgDone()
		defer log.Println("http server quit")

		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			a.doUpgrade(w, r)
		})

		if err := httpServer.ListenAndServe(); err != nil {
			log.Println(err)
			return
		}
	}()
}
