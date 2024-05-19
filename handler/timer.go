package handler

import (
	"sync/atomic"
	"time"
)

// Timer executes a handler function after a specified wait time
type Timer struct {
	waitTime time.Duration // Duration to wait before executing the handler
	handler  func() error  // Handler function to execute after the wait time

	closed  atomic.Bool   // Atomic boolean to check if the timer is closed
	closeCh chan struct{} // Channel to signal timer closure
}

// Ensures Timer implements the Handler interface
var _ Handler = (*Timer)(nil)

// NewTimer creates a new Timer instance with the provided handler and wait time
func NewTimer(handler func() error, waitTime time.Duration) *Timer {
	t := &Timer{}
	t.handler = handler
	t.waitTime = waitTime

	t.closed.Store(false)
	t.closeCh = make(chan struct{})
	return t
}

// Run starts the timer and executes the handler function after the wait time
func (t *Timer) Run() error {
	defer t.Close() // Ensure the timer is closed when done
	timer := time.NewTimer(t.waitTime)
	defer timer.Stop() // Stop the timer when done

	select {
	case <-timer.C: // Wait for the timer to expire
		return t.handler()
	case <-t.closeCh: // Handle timer closure
		return nil
	}
}

// Close signals the timer to stop and marks it as closed
func (t *Timer) Close() {
	if t.closed.Load() {
		return
	}

	t.closed.Store(true)
	close(t.closeCh)
}

// IsClosed returns whether the timer has been closed
func (t *Timer) IsClosed() bool {
	return t.closed.Load()
}
