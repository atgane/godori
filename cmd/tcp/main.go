package main

import (
	"github.com/atgane/godori/handler"
)

func main() {
	conf := handler.NewTcpServerConfig()
	conf.Port = 8888
	s := handler.NewTcpServer(&TcpHandler{}, conf)
	s.Run()
}
