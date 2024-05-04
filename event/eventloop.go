package event

import (
	"errors"
	"time"
)

type Eventloop[T any] interface {
	Send(event T) error
	Run()
	Close()
	IsClosed() bool
}

type eventloop[T any] struct {
	ch          chan T
	closed      bool
	closeCh     chan struct{}
	handler     func(T)
	sendTimeout time.Duration
	wc          int
}

func NewEventLoop[T any](handler func(T), channelSize int, workerCount int, sendTimeout time.Duration) Eventloop[T] {
	e := new(eventloop[T])
	e.ch = make(chan T, channelSize)
	e.closeCh = make(chan struct{})
	e.closed = false
	e.handler = handler
	e.sendTimeout = sendTimeout
	e.wc = workerCount
	return e
}

func (e *eventloop[T]) Send(event T) error {
	if e.closed {
		return errors.New("eventloop already closed")
	}

	if e.sendTimeout == 0 {
		e.ch <- event
		return nil
	}

	select {
	case e.ch <- event:
		return nil
	case <-time.After(e.sendTimeout):
		return errors.New("eventloop push timeout")
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
