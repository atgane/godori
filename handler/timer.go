package handler

import (
	"sync/atomic"
	"time"
)

type Timer struct {
	config       *TimerConfig
	timerHandler TimerHandler
	ticker       *time.Ticker

	closeCh chan struct{}
	closed  atomic.Bool
}

type TimerConfig struct {
	Duration time.Duration
}

func NewTimerConfig() *TimerConfig {
	c := &TimerConfig{}
	return c
}

type TimerEvent struct {
	CreateAt time.Time
}

type TimerHandler interface {
	OnCall(e *TimerEvent)
}

func NewTimer[T any](h TimerHandler, c *TimerConfig) *Timer {
	t := &Timer{}
	t.config = c
	t.closeCh = make(chan struct{})
	t.closed.Store(false)

	return t
}

func (t *Timer) Run() (err error) {
	t.ticker = time.NewTicker(t.config.Duration)

	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.timerHandler.OnCall(&TimerEvent{
					CreateAt: time.Now(),
				})
			case <-t.closeCh:
			}
		}
	}()

	<-t.closeCh
	return nil
}

func (t *Timer) Close() {
	if t.closed.Load() {
		return
	}

	t.closed.Store(true)
	close(t.closeCh)
	t.ticker.Stop()
}
