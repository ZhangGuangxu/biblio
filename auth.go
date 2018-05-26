package main

import (
	"errors"
	twmm "github.com/ZhangGuangxu/timingwheelmm"
	"log"
	"sync"
	"time"
)

var errNoToken = errors.New("no token")
var auther, _ = newAuth()
var itemLifetime = 12 * time.Second

type authItem int64

func (item authItem) Release() {
	auther.delToken(int64(item))
}

type auth struct {
	mux    sync.Mutex
	tokens map[int64]string

	tw *twmm.TimingWheel
}

func newAuth() (*auth, error) {
	a := &auth{
		tokens: make(map[int64]string, 100),
	}
	var err error
	a.tw, err = twmm.NewTimingWheel(itemLifetime, 120)
	return a, err
}

// @public
func (a *auth) addToken(uid int64, token string) {
	a.tw.AddItem(authItem(uid))

	a.mux.Lock()
	defer a.mux.Unlock()
	a.tokens[uid] = token
}

// @public
func (a *auth) checkToken(uid int64, token string) (bool, error) {
	a.mux.Lock()
	t, ok := a.tokens[uid]
	a.mux.Unlock()
	if ok {
		return t == token, nil
	}
	return false, errNoToken
}

// @public
func (a *auth) delToken(uid int64) {
	a.mux.Lock()
	defer a.mux.Unlock()
	delete(a.tokens, uid)
}

// @public
func (a *auth) startTimingWheel() {
	serverInst.wgAddOne()
	go a.tw.Run(getQuit, func() {
		log.Println("auth timingwheel quit")
		serverInst.wgDone()
	})
}
