package main

import (
	"fmt"

	"github.com/atgane/godori/handler"
)

type TcpHandler struct {
	nu int
}

type SocketEventField struct{}

var _ handler.SocketHandler[SocketEventField] = (*TcpHandler)(nil)

func (t *TcpHandler) OnOpen(e *handler.SocketEvent[SocketEventField])             {}
func (t *TcpHandler) OnClose(e *handler.SocketEvent[SocketEventField])            {}
func (t *TcpHandler) OnError(e *handler.SocketEvent[SocketEventField], err error) {}
func (t *TcpHandler) OnRead(e *handler.SocketEvent[SocketEventField], b []byte) uint {
	e.Write(b)
	t.nu += len(b)
	fmt.Println(t.nu, len(b), string(b))
	return uint(len(b))
}
