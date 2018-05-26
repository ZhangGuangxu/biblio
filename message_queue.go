package main

import (
	ccq "github.com/ZhangGuangxu/circularqueue"
	"sync"
	atom "sync/atomic"
	"time"
)

// Zero value of messageQueue is invalid.
type messageQueue struct {
	mux   sync.Mutex
	qAdd  *ccq.CircularQueue
	qTake *ccq.CircularQueue

	newMsgAdded chan bool
	closeFlag   chan bool

	bindSuccessGuard int32
	bindSuccess      chan bool

	// 用于标识Client对象的handleWrite协程已终止
	clientWriteClosed chan bool
	// 用于标识Client对象的handleRead协程已终止
	clientReadClosed chan bool
}

// @public
func newMessageQueue() *messageQueue {
	return &messageQueue{
		qAdd:              ccq.NewCircularQueue(),
		qTake:             ccq.NewCircularQueue(),
		newMsgAdded:       make(chan bool, 1),
		closeFlag:         make(chan bool, 1),
		bindSuccess:       make(chan bool),
		clientWriteClosed: make(chan bool),
		clientReadClosed:  make(chan bool),
	}
}

// @public
func (s *messageQueue) notifyClose() {
	select {
	case s.closeFlag <- true:
	default:
	}
}

// @public
func (s *messageQueue) shouldClose() bool {
	select {
	case <-s.closeFlag:
		return true
	default:
		return false
	}
}

// @public
func (s *messageQueue) notifyBindSuccess() {
	if atom.CompareAndSwapInt32(&s.bindSuccessGuard, 0, 1) {
		close(s.bindSuccess)
	}
}

// @public
func (s *messageQueue) isBindSuccess() bool {
	select {
	case <-s.bindSuccess:
		return true
	default:
		return false
	}
}

// @public
func (s *messageQueue) notifyClientWriteClosed() {
	close(s.clientWriteClosed)
}

func (s *messageQueue) isClientWriteClosed() bool {
	select {
	case <-s.clientWriteClosed:
		return true
	default:
		return false
	}
}

// @public
func (s *messageQueue) notifyClientReadClosed() {
	close(s.clientReadClosed)
}

func (s *messageQueue) isClientReadClosed() bool {
	select {
	case <-s.clientReadClosed:
		return true
	default:
		return false
	}
}

// @public
func (s *messageQueue) addMessage(msg *message) {
	if s.isClientWriteClosed() { // 消息已无法发出了，所以直接丢弃
		return
	}

	s.mux.Lock()
	s.qAdd.Push(msg)
	s.mux.Unlock()

	s.newMessageAdded()
}

func (s *messageQueue) newMessageAdded() {
	select {
	case s.newMsgAdded <- true:
	default:
	}
}

func (s *messageQueue) hasNewMessage(timer *time.Timer) bool {
	if timer != nil {
		select {
		case <-s.newMsgAdded:
			return true
		case <-timer.C:
			return false
		}
	} else {
		select {
		case <-s.newMsgAdded:
			return true
		default:
			return false
		}
	}
}

// @public
// takeMessage takes message from sender. It may return nil.
// timer is a timer to wait for messages. It could be nil.
func (s *messageQueue) takeMessage(timer *time.Timer) *message {
	if s.qTake.IsEmpty() {
		if s.hasNewMessage(timer) {
			s.mux.Lock()
			s.qAdd, s.qTake = s.qTake, s.qAdd
			s.mux.Unlock()
		} else {
			return nil
		}
	}

	if !s.qTake.IsEmpty() {
		if v, err := s.qTake.Pop(); err == nil {
			if m, ok := v.(*message); ok {
				return m
			}
		}
	}

	return nil
}

func (s *messageQueue) start() {}
