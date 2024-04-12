package main

import (
	"fmt"

	"github.com/atgane/godori/handler"
)

type TcpHandler struct{}

var _ handler.SocketHandler = (*TcpHandler)(nil)

func (t *TcpHandler) OnOpen(e *handler.SocketEvent)             {}
func (t *TcpHandler) OnClose(e *handler.SocketEvent)            {}
func (t *TcpHandler) OnError(e *handler.SocketEvent, err error) {}
func (t *TcpHandler) OnRead(e *handler.SocketEvent, b []byte) uint {
	e.Conn.Write(b)
	fmt.Print(string(b))
	return uint(len(b))
}
