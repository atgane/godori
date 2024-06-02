package event

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Eventloop is a generic interface defining methods for an event loop.
type Eventloop[T any] interface {
	Send(event T) error // Sends an event to the event loop.
	Run()               // Starts the event loop.
	Close()             // Closes the event loop.
	IsClosed() bool     // Checks if the event loop is closed.
	Len() int           // Returns length of the eventloop channel.
}

// eventloop is a struct implementing the Eventloop interface.
type eventloop[T any] struct {
	ch      chan T        // Channel to send and receive events.
	closed  atomic.Bool   // Indicates if the event loop is closed.
	closeCh chan struct{} // Channel to signal closing of the event loop.
	handler func(T)       // Handler function to process events.
	wc      int           // Worker count for concurrent processing.
}

// NewEventLoop creates and initializes a new event loop.
func NewEventLoop[T any](handler func(T), channelSize int, workerCount int) Eventloop[T] {
	e := new(eventloop[T])
	e.ch = make(chan T, channelSize)
	e.closeCh = make(chan struct{})
	e.closed.Store(false)
	e.handler = handler
	e.wc = workerCount
	return e
}

// Send sends an event to the event loop, returns an error if the loop is closed or blocked.
func (e *eventloop[T]) Send(event T) error {
	if e.closed.Load() {
		return errors.New("eventloop already closed")
	}

	select {
	case e.ch <- event:
		return nil
	default:
		return errors.New("eventloop send block")
	}
}

// Run starts the event loop and its workers.
func (e *eventloop[T]) Run() {
	wg := &sync.WaitGroup{}

	for range e.wc {
		wg.Add(1)
		go func() {
			wg.Done()
			for {
				select {
				case event := <-e.ch: // Prioritize handle event.
					e.handle(event)
					continue
				default:
				}

				select {
				case event := <-e.ch: // Process event.
					e.handle(event)
				case <-e.closeCh: // Check for close signal again.
					if len(e.ch) != 0 {
						continue
					}
					return
				}
			}
		}()
	}

	wg.Wait() // Block until close signal is received.
}

// Close stops the event loop and marks it as closed.
func (e *eventloop[T]) Close() {
	if e.closed.Load() {
		return
	}

	close(e.closeCh)
	e.closed.Store(true)
}

// IsClosed checks if the event loop has been closed.
func (e *eventloop[T]) IsClosed() bool {
	return e.closed.Load()
}

func (e *eventloop[T]) Len() int {
	return len(e.ch)
}

// handle processes an event using the provided handler function.
func (e *eventloop[T]) handle(event T) {
	e.handler(event)
}
