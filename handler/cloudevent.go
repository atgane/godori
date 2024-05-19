package handler

import (
	"context"
	"sync/atomic"

	"github.com/atgane/godori/event"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// CloudEventConsumer handles the consumption and processing of CloudEvents
type CloudEventConsumer struct {
	// CloudEvents client for receiving events
	cloudEventClient cloudevents.Client
	// Event loop for processing CloudEvents
	cloudEventEventloop event.Eventloop[cloudevents.Event]
	// Handler for processing received CloudEvents
	cloudEventHandler CloudEventHandler

	// Channel to signal consumer closure
	closeCh chan struct{}
	// Atomic boolean to check if consumer is closed
	closed atomic.Bool

	// Cancel function for context management when running
	cloudEventCancelFunc context.CancelFunc
}

// Configuration settings for the CloudEventConsumer
type CloudEventConfig struct {
	EventChannelSize int // Size of the event channel
	EventWorkerCount int // Number of workers for event handling
}

// Interface for handling CloudEvents
type CloudEventHandler interface {
	// Called when a CloudEvent is received
	OnReceive(e cloudevents.Event)
}

// Creates a new CloudEventConfig with default values
func NewCloutEventConfig() (*CloudEventConfig, error) {
	c := &CloudEventConfig{
		EventChannelSize: 128,
		EventWorkerCount: 1,
	}
	return c, nil
}

// Creates a new CloudEventConsumer instance
func NewCloudEventConsumer[T any](handler CloudEventHandler, client cloudevents.Client, config CloudEventConfig) (*CloudEventConsumer, error) {
	c := &CloudEventConsumer{}
	c.cloudEventClient = client
	// Initialize the event loop with the handler's OnReceive method
	c.cloudEventEventloop = event.NewEventLoop(c.cloudEventHandler.OnReceive, config.EventChannelSize, config.EventWorkerCount)
	c.cloudEventHandler = handler
	return c, nil
}

// Starts the CloudEventConsumer and begins receiving events
func (c *CloudEventConsumer) Run() error {
	defer c.Close()

	go c.cloudEventEventloop.Run()

	ctx := context.Background()
	// Create a cancellable context
	ctx, c.cloudEventCancelFunc = context.WithCancel(ctx)
	// Start the CloudEvents receiver with the handler
	if err := c.cloudEventClient.StartReceiver(ctx, c.cloudEventHandler); err != nil {
		if c.closed.Load() {
			return nil
		}

		return err
	}
	return nil
}

// Closes the CloudEventConsumer and stops receiving events
func (c *CloudEventConsumer) Close() {
	if c.closed.Load() {
		return
	}

	c.closed.Store(true)
	close(c.closeCh)
	c.cloudEventEventloop.Close()
	// Cancel the context to stop the receiver
	c.cloudEventCancelFunc()
}
