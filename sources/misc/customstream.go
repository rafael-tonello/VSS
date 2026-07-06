package misc

import (
	"sync"
)

type CallbackInfo[T any] struct {
	f  func(data T)
	id int
}

type Stream[T any] struct {
	listeners map[int](*CallbackInfo[T])
	lastValue T
	idCount   int

	lock                sync.Mutex
	publishInBackground bool
}

// create a new stream. If 'publishInBackground' is true, all listeners will be called in a new goroutine.
// Be careful when using publishInBackground = true, because messages can be processed out of order by listeners.
func NewStream[T any](publishInBackground bool) *Stream[T] {
	ret := Stream[T]{idCount: 0, listeners: map[int](*CallbackInfo[T]){}, publishInBackground: publishInBackground}

	return &ret
}

func NewStreamWithInitialValue[T any](initialValue T) *Stream[T] {
	ret := Stream[T]{lastValue: initialValue, idCount: 0}
	return &ret
}

func (s *Stream[T]) Listen(f func(data T)) int {
	s.lock.Lock()
	s.listeners[s.idCount] = &(CallbackInfo[T]{f: f, id: s.idCount})
	s.lock.Unlock()
	s.idCount += 1
	return s.idCount - 1
}

func (s *Stream[T]) StopListen(observerId int) {
	s.lock.Lock()
	_, found := s.listeners[observerId]

	if found {
		delete(s.listeners, observerId)
	}
	s.lock.Unlock()
}

// Stream data to all listeners. Return true if at least one listener is present. False otherwise.
func (s *Stream[T]) Stream(data T) bool {
	s.lastValue = data
	s.lock.Lock()

	//copy listeners vector
	listenersCopy := make([]*CallbackInfo[T], 0, len(s.listeners))
	for _, v := range s.listeners {
		listenersCopy = append(listenersCopy, v)
	}

	s.lock.Unlock()

	for _, f := range listenersCopy {
		tmp := f.f

		if s.publishInBackground {
			go tmp(data)
		} else {
			tmp(data)
		}
	}
	return len(s.listeners) > 0
}

func (s *Stream[T]) GetLast() T {
	return s.lastValue
}

// helper functions
func (s *Stream[T]) Subscribe(f func(data T)) int { return s.Listen(f) }

func (s *Stream[T]) Unsubscribe(observerId int) { s.StopListen(observerId) }

// Stream data to all listeners. Return true if at least one listener is present. False otherwise.
func (s *Stream[T]) Publish(data T) { s.Stream(data) }

// Stream data to all listeners. Return true if at least one listener is present. False otherwise.
func (s *Stream[T]) Add(data T) { s.Stream(data) }

func WithCallback[T any](f func(data T)) func(data T) {
	return func(data T) {
		f(data)
	}
}

func WithChannel[T any](ch chan T) func(data T) {
	return func(data T) {
		ch <- data
	}
}

func (s *Stream[T]) UnsubscribeAll() {
	s.lock.Lock()
	s.listeners = map[int](*CallbackInfo[T]){}
	s.lock.Unlock()
}
