package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
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

type Packet struct {
	*PacketHeader
	*PacketBody

	version   uint16 // 2
	headerLen uint32 // 4
	bodyLen   uint32 // 4
	signature uint16 // 2
}

type PacketHeader struct {
	Code int `json:"code"`
}

type PacketBody struct {
	Msg string
}

const (
	sVersionIndex   = 2
	sHeaderLenIndex = 6
	sBodyLenIndex   = 10
	sSignatureIndex = 12
)

func (t *TcpHandler) parsePacket(b []byte) (*Packet, uint, error) {
	var err error
	// packet receiving maybe not finished
	if len(b) < sSignatureIndex {
		return nil, 0, nil
	}

	p := &Packet{}

	p.version = binary.BigEndian.Uint16(b[:sVersionIndex])
	p.headerLen = binary.BigEndian.Uint32(b[sVersionIndex:sHeaderLenIndex])
	p.bodyLen = binary.BigEndian.Uint32(b[sHeaderLenIndex:sBodyLenIndex])
	p.signature = binary.BigEndian.Uint16(b[sBodyLenIndex:sSignatureIndex])

	// packet receiving maybe not finished
	if len(b) < sSignatureIndex+int(p.headerLen)+int(p.bodyLen) {
		return nil, 0, nil
	}

	header, err := t.parseHeader(b[sSignatureIndex:p.headerLen])
	if err != nil {
		return nil, uint(p.bodyLen), errors.New("header parse error")
	}

	body, err := t.parseBody(b[p.headerLen:p.bodyLen])
	if err != nil {
		return nil, uint(p.bodyLen), errors.New("body parse error")
	}
	p.PacketHeader = header
	p.PacketBody = body

	return p, uint(p.bodyLen), nil
}

func (t *TcpHandler) parseHeader(b []byte) (*PacketHeader, error) {
	// some impl header parser

	h := &PacketHeader{}
	if err := json.Unmarshal(b, h); err != nil {
		return nil, err
	}
	return h, nil
}

func (t *TcpHandler) parseBody(b []byte) (*PacketBody, error) {
	// some impl Body parser

	h := &PacketBody{}
	if err := json.Unmarshal(b, h); err != nil {
		return nil, err
	}
	return h, nil
}
