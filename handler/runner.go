package handler

import (
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
)

type Runner[T any] struct {
	runnerEventloop event.Eventloop[*RunnerEvent[T]]
	runnerHandler   RunnerHandler[T]
	config          *RunnerConfig

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
	CreateAt int64
	Field    T
}

type RunnerHandler[T any] interface {
	OnCall(e *RunnerEvent[T])
}

func NewRunner[T any](h RunnerHandler[T], c *RunnerConfig) *Runner[T] {
	r := &Runner[T]{}
	r.config = c
	r.runnerHandler = h
	r.closeCh = make(chan struct{})

	eventHandler := func(e *RunnerEvent[T]) {
		RunWithRecover(func() { r.runnerHandler.OnCall(e) })
	}

	r.runnerEventloop = event.NewEventLoop(eventHandler, c.EventChannelSize, c.EventWorkerCount)
	return r
}

func (r *Runner[T]) Run() (err error) {
	go r.runnerEventloop.Run()
	<-r.closeCh
	return nil
}

func (r *Runner[T]) Send(event T) error {
	e := &RunnerEvent[T]{
		CreateAt: time.Now().Unix(),
		Field:    event,
	}
	return r.runnerEventloop.Send(e)
}

func (r *Runner[T]) Close() {
	r.closed.Store(true)
	close(r.closeCh)
	r.runnerEventloop.Close()
}
