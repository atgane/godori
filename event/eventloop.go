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
	case e.ch <- event:
		return nil
	default:
		return errors.New("eventloop send block")
	}
}

func (e *eventloop[T]) Run() {
	for range e.wc {
		go func() {
			for {
				select {
				case event := <-e.ch:
					e.handle(event)
				case <-e.closeCh:
					return
				}
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
