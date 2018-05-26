package main

import (
	ccq "github.com/ZhangGuangxu/circularqueue"
	atom "sync/atomic"
	"time"
)

type messageChannel struct {
	inCh  chan *message
	q     *ccq.CircularQueue
	outCh chan interface{}

	closeFlag chan bool

	bindSuccessGuard int32
	bindSuccess      chan bool

	// 用于标识Client对象的handleWrite协程已终止
	clientWriteClosed chan bool
	// 用于标识Client对象的handleRead协程已终止
	clientReadClosed chan bool
}

func newMessageChannel() *messageChannel {
	return &messageChannel{
		inCh:              make(chan *message),
		q:                 ccq.NewCircularQueue(),
		outCh:             make(chan interface{}),
		closeFlag:         make(chan bool, 1),
		bindSuccess:       make(chan bool),
		clientWriteClosed: make(chan bool),
		clientReadClosed:  make(chan bool),
	}
}

// @public
func (m *messageChannel) notifyClose() {
	select {
	case m.closeFlag <- true:
	default:
	}
}

// @public
func (m *messageChannel) shouldClose() bool {
	select {
	case <-m.closeFlag:
		return true
	default:
		return false
	}
}

// @public
func (m *messageChannel) notifyBindSuccess() {
	if atom.CompareAndSwapInt32(&m.bindSuccessGuard, 0, 1) {
		close(m.bindSuccess)
	}
}

// @public
func (m *messageChannel) isBindSuccess() bool {
	select {
	case <-m.bindSuccess:
		return true
	default:
		return false
	}
}

// @public
func (m *messageChannel) notifyClientWriteClosed() {
	close(m.clientWriteClosed)
}

func (m *messageChannel) isClientWriteClosed() bool {
	select {
	case <-m.clientWriteClosed:
		return true
	default:
		return false
	}
}

// @public
func (m *messageChannel) notifyClientReadClosed() {
	close(m.clientReadClosed)
}

func (m *messageChannel) isClientReadClosed() bool {
	select {
	case <-m.clientReadClosed:
		return true
	default:
		return false
	}
}

// @public
func (m *messageChannel) start() {
	serverInst.wgAddOne()
	go m.run()
}

func (m *messageChannel) run() {
	defer serverInst.wgDone()

	d := 50 * time.Millisecond
	t := time.NewTimer(d)

	for {
		if m.isClientReadClosed() {
			break
		}
		if m.isClientWriteClosed() {
			break
		}

		if !m.q.IsEmpty() {
			item, err := m.q.Peek()
			if err != nil {
				return
			}

			t.Reset(d)

			select {
			case m.outCh <- item:
				m.q.Retrieve()
			case msg := <-m.inCh:
				m.q.Push(msg)
			case <-t.C:
			case <-getQuit():
				return
			}
		} else {
			t.Reset(d)

			select {
			case msg := <-m.inCh:
				m.q.Push(msg)
			case <-t.C:
			case <-getQuit():
				return
			}
		}
	}
}

// @public
func (m *messageChannel) addMessage(msg *message) {
	if m.isClientWriteClosed() {
		return
	}

	select {
	case m.inCh <- msg:
	}
}

// @public
func (m *messageChannel) takeMessage(t *time.Timer) *message {
	if t != nil {
		select {
		case v := <-m.outCh:
			if msg, ok := v.(*message); ok {
				return msg
			}
		case <-t.C:
		}
	} else {
		select {
		case v := <-m.outCh:
			if msg, ok := v.(*message); ok {
				return msg
			}
		default:
		}
	}
	return nil
}
