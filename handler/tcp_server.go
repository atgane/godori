package handler

import (
	"fmt"
	"net"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
	"github.com/google/uuid"
)

type TcpServer[T any] struct {
	*event.Table[string, *Conn[T]]

	// initialize when construct
	config          *TcpServerConfig
	socketEventloop event.Eventloop[*SocketEvent[T]]
	socketHandler   SocketHandler[T]

	// handler default value
	closeCh chan struct{}
	closed  atomic.Bool

	// initialize when run
	listener net.Listener
}

var _ Handler = (*TcpServer[*struct{}])(nil)

type TcpServerConfig struct {
	EventChannelSize       int
	EventWorkerCount       int
	Port                   int
	TableInitSize          int
	SocketBufferSize       int
	SocketWriteChannelSize int
	ReadDeadlineSecond     int
	WriteRetryCount        int
}

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
	socketUuid string
	status     SocketStatus
}

type Conn[T any] struct {
	CreateAt time.Time
	UpdateAt time.Time
	Field    T

	writeEventloop      event.Eventloop[[]byte]
	conn                net.Conn
	onWriteErrorHandler func(err error)
	writeRetryCount     int
}

type SocketHandler[T any] interface {
	OnOpen(socketUuid string, e *Conn[T])
	OnRead(socketUuid string, e *Conn[T], b []byte) uint
	OnClose(socketUuid string, e *Conn[T])
	OnReadError(socketUuid string, e *Conn[T], err error)  // while error caused at socket read
	OnWriteError(socketUuid string, e *Conn[T], err error) // while error caused at socket write
}

type SocketStatus int

const (
	SocketConnected SocketStatus = iota
	SocketDisconnected
)

func NewTcpServer[T any](h SocketHandler[T], c *TcpServerConfig) *TcpServer[T] {
	s := &TcpServer[T]{}
	s.Table = event.NewTable[string, *Conn[T]](c.TableInitSize)

	s.config = c
	s.socketEventloop = event.NewEventLoop(s.handleSocket, c.EventChannelSize, c.EventWorkerCount)
	s.socketHandler = h

	s.closeCh = make(chan struct{})
	s.closed.Store(false)

	return s
}

func (s *TcpServer[T]) Run() (err error) {
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return err
	}
	go s.socketEventloop.Run()

	go func() {
		for {
			if err := s.loopAccept(); err != nil {
				return // tcp server close
			}
		}
	}()

	<-s.closeCh
	return nil
}

func (s *TcpServer[T]) Close() {
	if s.closed.Load() {
		return
	}

	s.closed.Store(true)
	close(s.closeCh)
	s.socketEventloop.Close()
	s.listener.Close()
}

func (s *TcpServer[T]) IsClosed() bool {
	return s.closed.Load()
}

func (s *TcpServer[T]) loopAccept() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return err
	}

	// socket write loop initializing
	// socket write channel must satisfy single event loop
	socketUuid := uuid.New().String()
	c := &Conn[T]{
		CreateAt: time.Now(),
		UpdateAt: time.Now(),

		conn:            conn,
		writeRetryCount: s.config.WriteRetryCount,
	}
	c.writeEventloop = event.NewEventLoop(c.onWrite, s.config.SocketWriteChannelSize, 1)
	c.onWriteErrorHandler = func(err error) {
		s.socketHandler.OnWriteError(socketUuid, c, err)
	}

	e := &SocketEvent[T]{
		Conn:       c,
		socketUuid: socketUuid,
		status:     SocketConnected,
	}

	if err := s.socketEventloop.Send(e); err != nil {
		return err
	}

	return nil
}

func (s *TcpServer[T]) handleSocket(e *SocketEvent[T]) {
	switch e.status {
	case SocketConnected:
		s.Table.Upsert(e.socketUuid, e.Conn)
		RunWithRecover(func() { s.socketHandler.OnOpen(e.socketUuid, e.Conn) })
		go s.onListen(e)
		go e.Conn.run()
	case SocketDisconnected:
		RunWithRecover(func() { s.socketHandler.OnClose(e.socketUuid, e.Conn) })
		e.Conn.close()
		s.Table.Delete(e.socketUuid)
	}
}

func (s *TcpServer[T]) onListen(e *SocketEvent[T]) {
	b := make([]byte, s.config.SocketBufferSize)
	buf := make([]byte, 0, s.config.SocketBufferSize*2)
	for {
		e.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.ReadDeadlineSecond)))
		r, err := e.conn.Read(b)
		if err != nil {
			RunWithRecover(func() { s.socketHandler.OnReadError(e.socketUuid, e.Conn, err) })

			if s.closed.Load() {
				return
			}

			s.socketEventloop.Send(&SocketEvent[T]{
				Conn:       e.Conn,
				socketUuid: e.socketUuid,
				status:     SocketDisconnected,
			})
			return
		}

		e.UpdateAt = time.Now()
		buf = append(buf, b[:r]...)
		p := uint(0)
		n := uint(len(buf))
		for p < n {
			var r uint
			RunWithRecover(func() { r = s.socketHandler.OnRead(e.socketUuid, e.Conn, buf[p:]) })
			if r == 0 {
				break
			}
			p += r
		}
		buf = buf[p:]
	}
}

func (c *Conn[T]) Write(w []byte) error {
	return c.writeEventloop.Send(w)
}

func (c *Conn[T]) onWrite(w []byte) {
	m := uint(len(w))
	retry := 0
	n := uint(0)
	for n < m {
		d, err := c.conn.Write(w[n:])
		n += uint(d)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				runtime.Gosched()
				retry++
				if retry >= c.writeRetryCount {
					RunWithRecover(func() { c.onWriteErrorHandler(err) })
					return
				}
				continue
			}
			RunWithRecover(func() { c.onWriteErrorHandler(err) })
			return
		}
		if d == 0 { // prevent inf loop
			break
		}

		retry = 0
	}
}

func (c *Conn[T]) run()   { c.writeEventloop.Run() }
func (c *Conn[T]) close() { c.writeEventloop.Close() }
