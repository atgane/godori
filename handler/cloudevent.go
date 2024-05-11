package handler

import (
	"context"
	"sync/atomic"

	"github.com/atgane/godori/event"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type CloudEventConsumer struct {
	// initialize when construct
	cloudEventClient    cloudevents.Client
	cloudEventEventloop event.Eventloop[cloudevents.Event]
	cloudEventHandler   CloudEventHandler

	// handler default value
	closeCh chan struct{}
	closed  atomic.Bool

	// initialize when run
	cloudEventCancelFunc context.CancelFunc
}

type CloudEventConfig struct {
	EventChannelSize int
	EventWorkerCount int
}

type CloudEventHandler interface {
	OnReceive(e cloudevents.Event)
}

func NewCloutEventConfig() (*CloudEventConfig, error) {
	c := &CloudEventConfig{
		EventChannelSize: 128,
		EventWorkerCount: 1,
	}
	return c, nil
}

func NewCloudEventConsumer[T any](handler CloudEventHandler, client cloudevents.Client, config CloudEventConfig) (*CloudEventConsumer, error) {
	c := &CloudEventConsumer{}
	c.cloudEventClient = client
	c.cloudEventEventloop = event.NewEventLoop(c.cloudEventHandler.OnReceive, config.EventChannelSize, config.EventWorkerCount)
	c.cloudEventHandler = handler
	return c, nil
}

func (c *CloudEventConsumer) Run() error {
	defer c.Close()

	go c.cloudEventEventloop.Run()

	ctx := context.Background()
	ctx, c.cloudEventCancelFunc = context.WithCancel(ctx)
	if err := c.cloudEventClient.StartReceiver(ctx, c.cloudEventHandler); err != nil {
		if c.closed.Load() {
			return nil
		}

		return err
	}
	return nil
}

func (c *CloudEventConsumer) Close() {
	if c.closed.Load() {
		return
	}

	c.closed.Store(true)
	close(c.closeCh)
	c.cloudEventEventloop.Close()
	c.cloudEventCancelFunc()
}
