package handler

import (
	"fmt"
	"net"
	"sync/atomic"

	"github.com/atgane/godori/event"
)

type TcpServer[T any] struct {
	*event.Table[string, *Conn[T]]

	// Configuration for the TCP server
	config *TcpServerConfig
	// Event loop for handling socket events
	socketEventloop event.Eventloop[*Conn[T]]
	// Handler for socket events
	socketHandler SocketHandler[T]

	// Channel to signal server closure
	closeCh chan struct{}
	// Atomic boolean to check if server is closed
	closed atomic.Bool

	// Listener for accepting new connections
	listener net.Listener
}

// Ensures TcpServer implements the Handler interface
var _ Handler = (*TcpServer[*struct{}])(nil)

type TcpServerConfig struct {
	EventChannelSize       int // Size of the event channel
	EventWorkerCount       int // Number of workers for event handling
	Port                   int // Port to listen on
	TableInitSize          int // Initial size of the connection table
	SocketBufferSize       int // Buffer size for socket reads
	SocketWriteChannelSize int // Size of the socket write channel
	ReadDeadlineSecond     int // Read deadline in seconds
	ConnConfig             *ConnConfig
}

// Creates a new configuration with default values
func NewTcpServerConfig() *TcpServerConfig {
	c := &TcpServerConfig{
		EventChannelSize:       4096,
		EventWorkerCount:       1,
		TableInitSize:          2048,
		SocketBufferSize:       4096,
		SocketWriteChannelSize: 16,
		ReadDeadlineSecond:     30,
		ConnConfig:             NewConnConfig(),
	}
	return c
}

// Interface for handling socket events
type SocketHandler[T any] interface {
	// Called when socket opens
	OnOpen(e *Conn[T])
	// Called when data is read from socket
	// Returns the length of received data
	OnRead(e *Conn[T], b []byte) uint
	// Called when socket closes
	OnClose(e *Conn[T])
	// Called when an error occurs while reading
	OnReadError(e *Conn[T], err error)
	// Called when an error occurs while writing
	OnWriteError(e *Conn[T], err error)
}

// Creates a new TcpServer instance
func NewTcpServer[T any](handler SocketHandler[T], config *TcpServerConfig) *TcpServer[T] {
	s := &TcpServer[T]{}
	s.Table = event.NewTable[string, *Conn[T]](config.TableInitSize)

	s.config = config
	s.socketEventloop = event.NewEventLoop(s.handleSocket, config.EventChannelSize, config.EventWorkerCount)
	s.socketHandler = handler

	s.closeCh = make(chan struct{})
	s.closed.Store(false)

	return s
}

// Starts the TCP server and begins accepting connections
func (s *TcpServer[T]) Run() (err error) {
	defer s.Close()

	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return err
	}
	go s.socketEventloop.Run()

	go func() {
		for {
			if err := s.loopAccept(); err != nil {
				return // TCP server closed
			}
		}
	}()

	<-s.closeCh
	return nil
}

// Closes the TCP server and all connections
func (s *TcpServer[T]) Close() {
	if s.closed.Load() {
		return
	}

	s.closed.Store(true)
	close(s.closeCh)
	s.socketEventloop.Close()
	s.listener.Close()
}

// Checks if the server is closed
func (s *TcpServer[T]) IsClosed() bool {
	return s.closed.Load()
}

// Accepts new connections in a loop
func (s *TcpServer[T]) loopAccept() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return err
	}

	c := NewConn[T](conn, s.config.ConnConfig, s.socketHandler)

	if err := s.socketEventloop.Send(c); err != nil {
		return err
	}

	return nil
}

// Handles socket events
func (s *TcpServer[T]) handleSocket(c *Conn[T]) {
	s.Table.Upsert(c.SocketUuid, c)
	RunWithRecover(func() { s.socketHandler.OnOpen(c) })
	go func() {
		c.run()
		RunWithRecover(func() { s.socketHandler.OnClose(c) })
		s.Table.Delete(c.SocketUuid)
	}()
}
