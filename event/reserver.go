package event

import (
	"sync/atomic"
	"time"
)

type Reserver struct {
	duration time.Duration // Duration to wait before invoking the handler
	handler  func()        // Function to execute after the duration
	waitChs  chan struct{} // Channel to manage reservation state
	cancelCh chan struct{} // Channel to handle cancellation
	started  atomic.Bool   // Indicates if the reservation has started
}

// NewReserver creates a new Reserver
func NewReserver(duration time.Duration, handler func()) *Reserver {
	return &Reserver{
		duration: duration,
		handler:  handler,
		waitChs:  make(chan struct{}, 1),
		cancelCh: make(chan struct{}, 1),
	}
}

// Start begins the reservation if not already started
func (t *Reserver) Start() {
	select {
	// Try to send a signal to waitChs to indicate the start of the reservation
	case t.waitChs <- struct{}{}:
		t.started.Store(true)

		go func() {
			defer func() {
				t.started.Store(false)
				<-t.waitChs
			}()

			select {
			// Wait for the specified duration before executing the handler
			case <-time.After(t.duration):
				if !t.started.Load() {
					return
				}
				t.handler()
			// If a cancel signal is received, exit the goroutine
			case <-t.cancelCh:
				return
			}
		}()
	// If waitChs already has a signal, do nothing (reservation already started)
	default:
		return
	}
}

// Cancel stops the reservation if it is active
func (t *Reserver) Cancel() {
	if !t.started.CompareAndSwap(true, false) {
		return
	}

	select {
	// Try to send a cancel signal to cancelCh
	case t.cancelCh <- struct{}{}:
	// If cancelCh already has a signal, do nothing
	default:
	}
}
