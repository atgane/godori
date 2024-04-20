package handler

type Handler interface {
	Run() error
	Close()
}
