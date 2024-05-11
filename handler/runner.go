package handler

import (
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
)

type Runner[T any] struct {
	// initialize when construct
	runnerEventloop event.Eventloop[*RunnerEvent[T]]
	runnerHandler   RunnerHandler[T]
	config          *RunnerConfig

	// handler default value
	closeCh chan struct{}
	closed  atomic.Bool
}

var _ Handler = (*Runner[*struct{}])(nil)

type RunnerConfig struct {
	EventChannelSize int
	EventWorkerCount int
}

func NewRunnerConfig() *RunnerConfig {
	c := &RunnerConfig{
		EventChannelSize: 4096,
		EventWorkerCount: 1,
	}
	return c
}

type RunnerEvent[T any] struct {
	CreateAt time.Time
	Field    T
}

type RunnerHandler[T any] interface {
	OnCall(e *RunnerEvent[T])
}

func NewRunner[T any](handler RunnerHandler[T], config *RunnerConfig) *Runner[T] {
	r := &Runner[T]{}
	r.runnerEventloop = event.NewEventLoop(handler.OnCall, config.EventChannelSize, config.EventWorkerCount)
	r.runnerHandler = handler
	r.config = config

	r.closeCh = make(chan struct{})
	r.closed.Store(false)

	eventHandler := func(e *RunnerEvent[T]) {
		RunWithRecover(func() { r.runnerHandler.OnCall(e) })
	}

	r.runnerEventloop = event.NewEventLoop(eventHandler, config.EventChannelSize, config.EventWorkerCount)
	return r
}

func (r *Runner[T]) Run() (err error) {
	defer r.Close()

	go r.runnerEventloop.Run()
	<-r.closeCh
	return nil
}

func (r *Runner[T]) Send(event T) error {
	e := &RunnerEvent[T]{
		CreateAt: time.Now(),
		Field:    event,
	}
	return r.runnerEventloop.Send(e)
}

func (r *Runner[T]) Close() {
	if r.closed.Load() {
		return
	}

	r.closed.Store(true)
	close(r.closeCh)
	r.runnerEventloop.Close()
}
