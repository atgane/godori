package handler

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
	"github.com/google/uuid"
)

type Conn[T any] struct {
	SocketUuid string      // Unique identifier for the socket
	CreateAt   time.Time   // Timestamp of connection creation
	UpdateAt   time.Time   // Timestamp of last update
	Field      T           // Generic field for custom data
	config     *ConnConfig // Configuration for the Conn

	writeEventloop event.Eventloop[[]byte] // Event loop for writing data
	socketHandler  SocketHandler[T]
	conn           net.Conn // Network connection

	// Channel to signal conn closure
	closeCh chan struct{}
	// Atomic boolean to check if conn is closed
	closed atomic.Bool
}

type ConnConfig struct {
	SocketWriteChannelSize int
	SocketReadBufferSize   int
	ReadDeadlineSecond     int
	WriteDeadlineSecond    int
}

func NewConnConfig() *ConnConfig {
	c := &ConnConfig{
		SocketWriteChannelSize: 512,
		SocketReadBufferSize:   4096,
		ReadDeadlineSecond:     30,
		WriteDeadlineSecond:    30,
	}
	return c
}

func NewConn[T any](conn net.Conn, config *ConnConfig, socketHandler SocketHandler[T]) *Conn[T] {
	c := &Conn[T]{}
	c.SocketUuid = uuid.New().String()
	c.CreateAt = time.Now()
	c.UpdateAt = c.CreateAt
	c.config = config

	// must have single eventloop
	c.writeEventloop = event.NewEventLoop(c.onWrite, c.config.SocketWriteChannelSize, 1)
	c.socketHandler = socketHandler
	c.conn = conn

	return c
}

// Runs the write event loop
func (c *Conn[T]) run() {
	go c.writeEventloop.Run()
	go c.onListen()
}

// Sends data to be written to the connection
func (c *Conn[T]) Write(w []byte) error {
	if c.closed.Load() {
		return errors.New("already closed socket")
	}
	return c.writeEventloop.Send(w)
}

// Writes data to the connection
func (c *Conn[T]) onWrite(w []byte) {
	m := uint(len(w))
	n := uint(0)
	for n < m {
		c.conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(c.config.WriteDeadlineSecond)))
		d, err := c.conn.Write(w[n:])
		n += uint(d)
		if err != nil {
			// if normal shutdown, just close function
			if c.closed.Load() {
				return
			}

			RunWithRecover(func() { c.socketHandler.OnWriteError(c, err) })
			return
		}
		if d == 0 { // Prevent infinite loop
			break
		}
	}
}

func (c *Conn[T]) onListen() {
	defer c.close()

	b := make([]byte, c.config.SocketReadBufferSize)
	buf := make([]byte, 0, c.config.SocketReadBufferSize*2)
	for {
		c.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(c.config.ReadDeadlineSecond)))
		r, err := c.conn.Read(b)
		if err != nil {
			// if normal shutdown, just close function
			if c.closed.Load() {
				return
			}

			RunWithRecover(func() { c.socketHandler.OnReadError(c, err) })
			return
		}

		c.UpdateAt = time.Now()
		buf = append(buf, b[:r]...)
		p := uint(0)
		n := uint(len(buf))
		for p < n {
			var r uint
			RunWithRecover(func() { r = c.socketHandler.OnRead(c, buf[p:]) })
			if r == 0 {
				break
			}
			p += r
		}
		buf = buf[p:]
	}
}

// Closes the connection and its write event loop
func (c *Conn[T]) close() {
	if c.closed.Load() {
		return
	}

	c.closed.Store(true)
	close(c.closeCh)
	c.writeEventloop.Close()
	c.conn.Close()
}
