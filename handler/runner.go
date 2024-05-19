package handler

import (
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
)

// Runner manages the event loop and handles events using the provided RunnerHandler
type Runner[T any] struct {
	// Event loop for runner events
	runnerEventloop event.Eventloop[*RunnerEvent[T]]
	// Handler for processing runner events
	runnerHandler RunnerHandler[T]
	// Configuration for the runner
	config *RunnerConfig

	// Channel to signal runner closure
	closeCh chan struct{}
	// Atomic boolean to check if runner is closed
	closed atomic.Bool
}

// Ensures Runner implements the Handler interface
var _ Handler = (*Runner[*struct{}])(nil)

// Configuration settings for the runner
type RunnerConfig struct {
	EventChannelSize int // Size of the event channel
	EventWorkerCount int // Number of workers for event handling
}

// Creates a new RunnerConfig with default values
func NewRunnerConfig() *RunnerConfig {
	c := &RunnerConfig{
		EventChannelSize: 4096,
		EventWorkerCount: 1,
	}
	return c
}

// RunnerEvent represents an event processed by the Runner
type RunnerEvent[T any] struct {
	CreateAt time.Time // Timestamp of event creation
	Field    T         // Generic field for custom event data
}

// Interface for handling runner events
type RunnerHandler[T any] interface {
	// Called when an event is processed
	OnCall(e *RunnerEvent[T])
}

// Creates a new Runner instance
func NewRunner[T any](handler RunnerHandler[T], config *RunnerConfig) *Runner[T] {
	r := &Runner[T]{}
	r.runnerEventloop = event.NewEventLoop(handler.OnCall, config.EventChannelSize, config.EventWorkerCount)
	r.runnerHandler = handler
	r.config = config

	r.closeCh = make(chan struct{})
	r.closed.Store(false)

	// Handler function that ensures safe execution of event processing
	eventHandler := func(e *RunnerEvent[T]) {
		RunWithRecover(func() { r.runnerHandler.OnCall(e) })
	}

	// Initialize the event loop with the event handler
	r.runnerEventloop = event.NewEventLoop(eventHandler, config.EventChannelSize, config.EventWorkerCount)
	return r
}

// Starts the runner and begins processing events
func (r *Runner[T]) Run() (err error) {
	defer r.Close()

	go r.runnerEventloop.Run()
	<-r.closeCh
	return nil
}

// Sends an event to be processed by the runner
func (r *Runner[T]) Send(event T) error {
	e := &RunnerEvent[T]{
		CreateAt: time.Now(),
		Field:    event,
	}
	return r.runnerEventloop.Send(e)
}

// Closes the runner and stops processing events
func (r *Runner[T]) Close() {
	if r.closed.Load() {
		return
	}

	r.closed.Store(true)
	close(r.closeCh)
	r.runnerEventloop.Close()
}
