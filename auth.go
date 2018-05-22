package main

import (
	"errors"
	twm "github.com/ZhangGuangxu/timingwheelm"
	"log"
	"sync"
	"time"
)

var errNoToken = errors.New("no token")
var auther, _ = newAuth()
var itemLifetime = 12 * time.Second

type authItem struct {
	uid        int64
	createTime time.Time
}

func newAuthItem(uid int64) *authItem {
	return &authItem{
		uid:        uid,
		createTime: time.Now(),
	}
}

func (item *authItem) ShouldRelease() bool {
	return time.Now().Sub(item.createTime) > itemLifetime
}

func (item *authItem) Release() {
	auther.clearToken(item.uid)
}

type auth struct {
	mux    sync.Mutex
	tokens map[int64]string
	tw     *twm.TimingWheel
}

func newAuth() (*auth, error) {
	a := &auth{
		tokens: make(map[int64]string, 100),
	}
	var err error
	a.tw, err = twm.NewTimingWheel(itemLifetime, 120)
	return a, err
}

func (a *auth) putToken(uid int64, token string) {
	a.mux.Lock()
	a.tokens[uid] = token
	a.mux.Unlock()

	// 不要放到上面的临界区中，否则会造成潜在的死锁
	a.tw.AddItem(newAuthItem(uid))
}

func (a *auth) compareToken(uid int64, token string) (bool, error) {
	a.mux.Lock()
	t, ok := a.tokens[uid]
	a.mux.Unlock()
	if !ok {
		return false, errNoToken
	}
	return t == token, nil
}

func (a *auth) clearToken(uid int64) {
	a.mux.Lock()
	delete(a.tokens, uid)
	a.mux.Unlock()
}

func (a *auth) startTimingWheel() {
	serverInst.wgAddOne()
	go a.tw.Run(needQuit, func() {
		log.Println("auth timingwheel quit")
		serverInst.wgDone()
	})
}
