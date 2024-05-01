package handler

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atgane/godori/event"
	"github.com/google/uuid"
)

type TcpServer[T any] struct {
	socketTable *event.Table[string, net.Conn]

	listener        net.Listener
	config          *TcpServerConfig
	socketEventloop event.Eventloop[*SocketEvent[T]]
	socketHandler   SocketHandler[T]

	closeCh chan struct{}
	closed  atomic.Bool
}

var _ Handler = (*TcpServer[*struct{}])(nil)

type TcpServerConfig struct {
	EventChannelSize   int
	EventWorkerCount   int
	Port               int
	TableInitSize      int
	BufferSize         int
	ReadDeadlineSecond int
	WriteRetryCount    int
}

func NewTcpServerConfig() *TcpServerConfig {
	c := &TcpServerConfig{
		EventChannelSize: 4096,
		EventWorkerCount: 1,
		TableInitSize:    2048,
		BufferSize:       4096,
		WriteRetryCount:  5,
	}
	return c
}

type SocketEvent[T any] struct {
	SocketUuid      string
	CreateAt        time.Time
	UpdateAt        time.Time
	WriteRetryCount int
	Status          SocketStatus
	Conn            net.Conn
	Field           T
	mu              sync.Mutex
}

type SocketHandler[T any] interface {
	OnOpen(e *SocketEvent[T])
	OnRead(e *SocketEvent[T], b []byte) uint
	OnClose(e *SocketEvent[T])
	OnError(e *SocketEvent[T], err error) // while error caused at socket read
}

type SocketStatus int

const (
	SocketConnected SocketStatus = iota
	SocketDisconnected
)

func NewTcpServer[T any](h SocketHandler[T], c *TcpServerConfig) *TcpServer[T] {
	s := &TcpServer[T]{}
	s.config = c
	s.socketEventloop = event.NewEventLoop(s.handleSocket, c.EventChannelSize, c.EventWorkerCount)
	s.socketTable = event.NewTable[string, net.Conn](c.TableInitSize)
	s.socketHandler = h
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

	e := &SocketEvent[T]{
		SocketUuid:      uuid.New().String(),
		CreateAt:        time.Now(),
		UpdateAt:        time.Now(),
		WriteRetryCount: s.config.WriteRetryCount,
		Status:          SocketConnected,
		Conn:            conn,
	}

	if err := s.socketEventloop.Send(e); err != nil {
		return err
	}

	return nil
}

func (s *TcpServer[T]) handleSocket(e *SocketEvent[T]) {
	switch e.Status {
	case SocketConnected:
		s.socketTable.Upsert(e.SocketUuid, e.Conn)
		RunWithRecover(func() { s.socketHandler.OnOpen(e) })
		go s.onListen(e)
	case SocketDisconnected:
		RunWithRecover(func() { s.socketHandler.OnClose(e) })
		s.socketTable.Delete(e.SocketUuid)
	}
}

func (s *TcpServer[T]) onListen(e *SocketEvent[T]) {
	b := make([]byte, s.config.BufferSize)
	buf := make([]byte, 0, s.config.BufferSize*2)
	for {
		e.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.ReadDeadlineSecond)))
		r, err := e.Conn.Read(b)
		if err != nil {
			RunWithRecover(func() { s.socketHandler.OnError(e, err) })

			if s.closed.Load() {
				return
			}

			s.socketEventloop.Send(&SocketEvent[T]{
				SocketUuid: e.SocketUuid,
				CreateAt:   e.CreateAt,
				UpdateAt:   e.UpdateAt,
				Status:     SocketDisconnected,
				Conn:       e.Conn,
			})
			return
		}

		e.UpdateAt = time.Now()
		buf = append(buf, b[:r]...)
		p := uint(0)
		n := uint(len(buf))
		for p < n {
			var r uint
			RunWithRecover(func() { r = s.socketHandler.OnRead(e, buf[p:]) })
			if r == 0 {
				break
			}
			p += r
		}
		buf = buf[p:]
	}
}

func (s *SocketEvent[T]) Write(w []byte) (n uint, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := uint(len(w))
	retry := 0
	for n < m {
		d, err := s.Conn.Write(w[n:])
		n += uint(d)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				runtime.Gosched()
				retry++
				if retry >= s.WriteRetryCount {
					return n, err
				}
				continue
			}
			return n, err
		}
		if d == 0 { // prevent inf loop
			break
		}

		retry = 0
	}
	return n, nil
}
