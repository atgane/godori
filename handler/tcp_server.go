package handler

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
	"github.com/google/uuid"
)

type TcpServer[T any] struct {
	*event.Table[string, *Conn[T]]

	// Configuration for the TCP server
	config *TcpServerConfig
	// Event loop for handling socket events
	socketEventloop event.Eventloop[*SocketEvent[T]]
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
	WriteRetryCount        int // Number of retries for writing to socket
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
		WriteRetryCount:        5,
	}
	return c
}

type SocketEvent[T any] struct {
	*Conn[T]
	status SocketStatus // Status of the socket (connected/disconnected)
}

type Conn[T any] struct {
	SocketUuid string    // Unique identifier for the socket
	CreateAt   time.Time // Timestamp of connection creation
	UpdateAt   time.Time // Timestamp of last update
	Field      T         // Generic field for custom data

	writeEventloop      event.Eventloop[[]byte] // Event loop for writing data
	conn                net.Conn                // Network connection
	onWriteErrorHandler func(err error)         // Error handler for write errors
	writeRetryCount     int                     // Number of write retries
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

// Enumeration for socket status
type SocketStatus int

const (
	SocketConnected SocketStatus = iota
	SocketDisconnected
)

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

	// Initialize socket write loop
	socketUuid := uuid.New().String()
	c := &Conn[T]{
		SocketUuid: socketUuid,
		CreateAt:   time.Now(),
		UpdateAt:   time.Now(),

		conn:            conn,
		writeRetryCount: s.config.WriteRetryCount,
	}
	c.writeEventloop = event.NewEventLoop(c.onWrite, s.config.SocketWriteChannelSize, 1)
	c.onWriteErrorHandler = func(err error) {
		s.socketHandler.OnWriteError(c, err)
	}

	e := &SocketEvent[T]{
		Conn:   c,
		status: SocketConnected,
	}

	if err := s.socketEventloop.Send(e); err != nil {
		return err
	}

	return nil
}

// Handles socket events
func (s *TcpServer[T]) handleSocket(e *SocketEvent[T]) {
	switch e.status {
	case SocketConnected:
		s.Table.Upsert(e.SocketUuid, e.Conn)
		RunWithRecover(func() { s.socketHandler.OnOpen(e.Conn) })
		go s.onListen(e)
		go e.Conn.run()
	case SocketDisconnected:
		RunWithRecover(func() { s.socketHandler.OnClose(e.Conn) })
		e.Conn.close()
		s.Table.Delete(e.SocketUuid)
	}
}

// Listens for data on a connected socket
func (s *TcpServer[T]) onListen(e *SocketEvent[T]) {
	b := make([]byte, s.config.SocketBufferSize)
	buf := make([]byte, 0, s.config.SocketBufferSize*2)
	for {
		e.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.ReadDeadlineSecond)))
		r, err := e.conn.Read(b)
		if err != nil {
			// Handle read errors, including timeouts
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				e.conn.Close()
			}

			RunWithRecover(func() { s.socketHandler.OnReadError(e.Conn, err) })

			if s.closed.Load() {
				return
			}

			s.socketEventloop.Send(&SocketEvent[T]{
				Conn:   e.Conn,
				status: SocketDisconnected,
			})
			return
		}

		e.UpdateAt = time.Now()
		buf = append(buf, b[:r]...)
		p := uint(0)
		n := uint(len(buf))
		for p < n {
			var r uint
			RunWithRecover(func() { r = s.socketHandler.OnRead(e.Conn, buf[p:]) })
			if r == 0 {
				break
			}
			p += r
		}
		buf = buf[p:]
	}
}

// Sends data to be written to the connection
func (c *Conn[T]) Write(w []byte) error { return c.writeEventloop.Send(w) }

// Writes data to the connection
func (c *Conn[T]) onWrite(w []byte) {
	m := uint(len(w))
	n := uint(0)
	for n < m {
		d, err := c.conn.Write(w[n:])
		n += uint(d)
		if err != nil {
			RunWithRecover(func() { c.onWriteErrorHandler(err) })
			return
		}
		if d == 0 { // Prevent infinite loop
			break
		}
	}
}

// Runs the write event loop
func (c *Conn[T]) run() { c.writeEventloop.Run() }

// Closes the connection and its write event loop
func (c *Conn[T]) close() {
	c.writeEventloop.Close()
	c.conn.Close()
}
