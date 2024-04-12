package handler

import (
	"fmt"
	"net"
	"time"

	"github.com/atgane/godori/event"
	"github.com/google/uuid"
)

type TcpServer struct {
	socketTable *event.Table[net.Conn]

	listener        net.Listener
	config          TcpServerConfig
	socketEventloop event.Eventloop[*SocketEvent]
	socketHandler   SocketHandler

	closeCh chan struct{}
	closed  bool
}

type TcpServerConfig struct {
	EventChannelSize int
	EventWorkerCount int
	Port             int
	TableInitSize    int
	BufferSize       int
}

func NewTcpServerConfig() *TcpServerConfig {
	c := &TcpServerConfig{
		EventChannelSize: 4096,
		EventWorkerCount: 1,
		TableInitSize:    2048,
		BufferSize:       4096,
	}
	return c
}

type SocketEvent struct {
	SocketUuid string
	CreateAt   int64
	Status     SocketStatus
	Conn       net.Conn
}

type SocketHandler interface {
	OnOpen(e *SocketEvent)
	OnRead(e *SocketEvent, b []byte) uint
	OnClose(e *SocketEvent)
	OnError(e *SocketEvent, err error) // while error caused at socket read
}

type SocketStatus int

const (
	SocketConnected SocketStatus = iota
	SocketDisconnected
)

func NewTcpServer(h SocketHandler, c *TcpServerConfig) *TcpServer {
	s := &TcpServer{}
	s.config = *c
	s.socketEventloop = event.NewEventLoop(s.handleSocket, c.EventChannelSize, c.EventWorkerCount)
	s.socketTable = event.NewTable[net.Conn](c.TableInitSize)
	s.socketHandler = h
	return s
}

func (s *TcpServer) Run() (err error) {
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

func (s *TcpServer) Close() {
	s.closed = true
	close(s.closeCh)
	s.socketEventloop.Close()
	s.listener.Close()
}

func (s *TcpServer) IsClosed() bool {
	return s.closed
}

func (s *TcpServer) loopAccept() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return err
	}

	e := &SocketEvent{
		SocketUuid: uuid.New().String(),
		CreateAt:   time.Now().Unix(),
		Status:     SocketConnected,
		Conn:       conn,
	}

	if err := s.socketEventloop.Send(e); err != nil {
		return err
	}

	return nil
}

func (s *TcpServer) handleSocket(e *SocketEvent) {
	switch e.Status {
	case SocketConnected:
		s.socketTable.Upsert(e.SocketUuid, e.Conn)
		s.socketHandler.OnOpen(e)
		go s.onListen(e)
	case SocketDisconnected:
		s.socketHandler.OnClose(e)
		s.socketTable.Delete(e.SocketUuid)
	}
}

func (s *TcpServer) onListen(e *SocketEvent) {
	b := make([]byte, s.config.BufferSize)
	buf := make([]byte, s.config.BufferSize*2)
	for {
		if _, err := e.Conn.Read(b); err != nil {
			s.socketHandler.OnError(e, err)

			if s.closed {
				return
			}

			s.socketEventloop.Send(&SocketEvent{
				SocketUuid: e.SocketUuid,
				CreateAt:   e.CreateAt,
				Status:     SocketDisconnected,
				Conn:       e.Conn,
			})
			return
		}

		buf = append(buf, b...)
		p := uint(0)
		n := uint(len(buf))
		for p < n {
			r := s.socketHandler.OnRead(e, buf[p:])
			if r == 0 {
				break
			}
			p += r
		}
		buf = buf[p:]
	}
}
