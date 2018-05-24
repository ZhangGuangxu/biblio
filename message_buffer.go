package main

import (
	ccq "github.com/ZhangGuangxu/circularqueue"
	"sync"
	"time"
)

// Zero value of messageBuffer is invalid.
type messageBuffer struct {
	mux   sync.Mutex
	qAdd  *ccq.CircularQueue
	qTake *ccq.CircularQueue

	newMsgAdded chan bool
	outCh       chan *message
}

// @public
func newMessageBuffer() *messageBuffer {
	return &messageBuffer{
		qAdd:        ccq.NewCircularQueue(),
		qTake:       ccq.NewCircularQueue(),
		newMsgAdded: make(chan bool, 1),
		outCh:       make(chan *message),
	}
}

// @public
func (s *messageBuffer) addMessage(msg *message) {
	s.mux.Lock()
	s.qAdd.Push(msg)
	s.mux.Unlock()

	s.newMessageAdded()
}

func (s *messageBuffer) newMessageAdded() {
	select {
	case s.newMsgAdded <- true:
	default:
	}
}

func (s *messageBuffer) hasNewMessage(timer *time.Timer) bool {
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
func (s *messageBuffer) takeMessage(timer *time.Timer) *message {
	if s.qTake.IsEmpty() {
		if s.hasNewMessage(timer) {
			s.mux.Lock()
			s.qAdd, s.qTake = s.qTake, s.qAdd
			s.mux.Unlock()
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
