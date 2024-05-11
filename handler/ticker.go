package handler

import (
	"sync/atomic"
	"time"
)

type Ticker struct {
	// initialize when construct
	config        *TickerConfig
	tickerHandler TickerrHandler

	// handler default value
	closeCh chan struct{}
	closed  atomic.Bool

	// initialize when run
	ticker *time.Ticker
}

type TickerConfig struct {
	Duration time.Duration
}

func NewTickerConfig() *TickerConfig {
	c := &TickerConfig{}
	return c
}

type TickerEvent struct {
	CreateAt time.Time
}

type TickerrHandler interface {
	OnCall(e *TickerEvent)
}

func NewTicker[T any](h TickerrHandler, c *TickerConfig) *Ticker {
	t := &Ticker{}

	t.config = c
	t.tickerHandler = h

	t.closeCh = make(chan struct{})
	t.closed.Store(false)

	return t
}

func (t *Ticker) Run() (err error) {
	t.ticker = time.NewTicker(t.config.Duration)

	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.tickerHandler.OnCall(&TickerEvent{
					CreateAt: time.Now(),
				})
			case <-t.closeCh:
			}
		}
	}()

	<-t.closeCh
	return nil
}

func (t *Ticker) Close() {
	if t.closed.Load() {
		return
	}

	t.closed.Store(true)
	close(t.closeCh)
	t.ticker.Stop()
}
