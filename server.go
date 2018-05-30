package main

import (
	proto "biblio/protocol"
	protojson "biblio/protocol/json"
	twmm "github.com/ZhangGuangxu/timingwheelmm"
	"log"
	"sync"
	"time"
)

var maxConnectionCount int
var serverAddress string // "ip:port", for example: "127.0.0.1:10001", or ":10001"
var protoFactory proto.ProtoFactory
var wsAddress string

var clientWaitAuthMaxTime = 5 * time.Second // 等待接收客户端的auth消息的最大时长
var bindProcessMaxTime = 5 * time.Second    // 收到客户端的auth请求后，要把client bind到player，多久后未完成认为处理超时
var kickProcessMaxTime = 5 * time.Second    // kick player多久后未完成认为处理超时
var heartbeatTime = 7 * time.Second
var playerKickTime = 2*heartbeatTime + 1
var playerUnloadTime = 10 * time.Minute

func init() {
	maxConnectionCount = 2000
	serverAddress = "127.0.0.1:59632"
	protoFactory = protojson.ProtoFactory
	wsAddress = "127.0.0.1:59631"
}

// Server wrap a server
type Server struct {
	muxp    sync.Mutex
	players map[int64]*Player

	twPlayerKick   *twmm.TimingWheel
	twPlayerUnload *twmm.TimingWheel

	muxc    sync.Mutex
	clients map[*Client]bool

	twClient        *twmm.TimingWheel // 用于处理“auth消息在指定超时时间前未收到”
	twClientBinding *twmm.TimingWheel // 用于处理“client bind到player的过程超时的情况”
	// 用于处理指定时间内未收到客户端消息的情况。
	// 主要考虑到player的逻辑处理协程可能发生死循环，同时player处于binding/kicking的状态。
	// 处于死循环，就无法回收player了。而且把player保持在Server.players中会比较稳妥。
	// player的binding/kicking是中间状态，就没有设置处理unload的timingwheel。
	// 这时只能尝试回收client，作为补救措施。
	twClientBinded *twmm.TimingWheel

	muxx      sync.Mutex
	xBindReqs map[int64]interface{}

	newXBindReqAdd chan bool

	wg *sync.WaitGroup

	playerAcceptor *playerAcceptor
	wsAcceptor     *wsAcceptor
}

// NewServer returns a server instance.
// You MUST use this function to create a server instance.
func NewServer() (*Server, error) {
	s := &Server{
		players:        make(map[int64]*Player, 100),
		clients:        make(map[*Client]bool, 100),
		xBindReqs:      make(map[int64]interface{}, 100),
		newXBindReqAdd: make(chan bool, 1),
		wg:             &sync.WaitGroup{},
		playerAcceptor: &playerAcceptor{},
		wsAcceptor:     &wsAcceptor{},
	}

	var err error
	s.twClient, err = twmm.NewTimingWheel(clientWaitAuthMaxTime, 50)
	if err != nil {
		return nil, err
	}
	s.twClientBinding, err = twmm.NewTimingWheel(bindProcessMaxTime*2, 100)
	if err != nil {
		return nil, err
	}
	s.twClientBinded, err = twmm.NewTimingWheel(playerKickTime*2, 300)
	if err != nil {
		return nil, err
	}

	s.twPlayerKick, err = twmm.NewTimingWheel(playerKickTime, 150)
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
	if _, ok := b.players[uid]; ok {
		return
	}
	b.players[uid] = p
}

func (b *Server) addPlayerToKick(item *playerKickItem) {
	b.twPlayerKick.AddItem(item)
}

func (b *Server) stopKick(item *playerKickItem) {
	b.twPlayerKick.DelItem(item)
}

func (b *Server) waitToUnload(item *playerUnloadItem) {
	b.twPlayerUnload.AddItem(item)
}

func (b *Server) stopUnload(item *playerUnloadItem) {
	b.twPlayerUnload.DelItem(item)
}

func (b *Server) removePlayer(p *Player) {
	b.muxp.Lock()
	defer b.muxp.Unlock()
	delete(b.players, p.uid())
}

func (b *Server) addClient(c *Client) {
	b.muxc.Lock()
	defer b.muxc.Unlock()
	b.clients[c] = true
}

func (b *Server) waitAuth(item *clientAuthTimeoutItem) {
	b.twClient.AddItem(item)
}

func (b *Server) stopWaitAuth(item *clientAuthTimeoutItem) {
	b.twClient.DelItem(item)
}

func (b *Server) waitClientBinding(item *clientBindingTimeoutItem) {
	b.twClientBinding.AddItem(item)
}

func (b *Server) stopWaitClientBinding(item *clientBindingTimeoutItem) {
	b.twClientBinding.DelItem(item)
}

func (b *Server) waitClientTimeout(item *clientBindedTimeoutItem) {
	b.twClientBinded.AddItem(item)
}

func (b *Server) stopWaitClientTimeout(item *clientBindedTimeoutItem) {
	b.twClientBinded.DelItem(item)
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

func (b *Server) start(wg *sync.WaitGroup) {
	wg.Add(1)
	go b.run(wg)
}

func (b *Server) run(wg *sync.WaitGroup) {
	defer wg.Done()
	defer log.Println("Server.run() quit")

	b.startDoBind()

	b.startClientTimingWheel()
	b.startClientBindingTimingWheel()
	b.startClientBindedTimingWheel()

	b.startPlayerKickTimingWheel()
	b.startPlayerUnloadTimingWheel()

	b.playerAcceptor.start(b)
	b.wsAcceptor.start(b)

	auther.startTimingWheel()

	// TODO: 监听web-server的请求

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
