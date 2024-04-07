package godori

import (
	"errors"
	"sync"
)

type List[T any] struct {
	list []T
	mu   sync.RWMutex
}

func NewList[T any](initSize int) *List[T] {
	li := &List[T]{}
	li.list = make([]T, initSize)

	return li
}

func (li *List[T]) Broadcast(handler func(T)) {
	li.mu.RLock()
	defer li.mu.RUnlock()

	for _, e := range li.list {
		handler(e)
	}
}

func (li *List[T]) BroadcastParallel(handler func(T), maxParallel int) {
	li.mu.RLock()
	defer li.mu.RUnlock()

	if maxParallel == 0 {
		maxParallel = len(li.list)
	}

	waitCh := make(chan struct{}, maxParallel)
	for _, e := range li.list {
		waitCh <- struct{}{}
		go func(e T) {
			handler(e)
			<-waitCh
		}(e)
	}
}

func (li *List[T]) Unicast(key int, handler func(T)) error {
	li.mu.RLock()
	defer li.mu.RUnlock()

	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	handler(li.list[key])

	return nil
}

func (li *List[T]) Insert(e T) {
	li.mu.Lock()
	defer li.mu.Unlock()

	li.list = append(li.list, e)
}

func (li *List[T]) Update(key int, e T) error {
	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	li.list[key] = e

	return nil
}

func (li *List[T]) Delete(key int, e T) error {
	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	li.list = append(li.list[:key], li.list[:key+1]...)

	return nil
}
