package event

import (
	"errors"
)

type Eventloop[T any] interface {
	Send(event T) error
	Run()
	Close()
	IsClosed() bool
}

type eventloop[T any] struct {
	ch      chan T
	closed  bool
	closeCh chan struct{}
	handler func(T)
	wc      int
}

func NewEventLoop[T any](handler func(T), channelSize int, workerCount int) Eventloop[T] {
	e := new(eventloop[T])
	e.ch = make(chan T, channelSize)
	e.closeCh = make(chan struct{})
	e.closed = false
	e.handler = handler
	e.wc = workerCount
	return e
}

func (e *eventloop[T]) Send(event T) error {
	if e.closed {
		return errors.New("eventloop already closed")
	}

	select {
	case event := <-e.ch:
		e.handle(event)
	case <-e.closeCh:
		return nil
	}
	return nil
}

func (e *eventloop[T]) Run() {
	for range e.wc {
		// workerCount 개수만큼 goroutine을 실행시키고 handle을 실행한다.
		go func() {
			select {
			case event := <-e.ch:
				e.handle(event)
			case <-e.closeCh:
				return
			}
		}()
	}

	<-e.closeCh
}

func (e *eventloop[T]) Close() {
	if e.closed {
		return
	}

	close(e.closeCh)
	e.closed = true
}

func (e *eventloop[T]) IsClosed() bool {
	return e.closed
}

func (e *eventloop[T]) handle(event T) {
	e.handler(event)
}
