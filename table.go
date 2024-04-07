package godori

import (
	"errors"
	"sync"
)

type Table[T any] struct {
	table map[string]T
	mu    sync.RWMutex
}

func NewTable[T any](initSize int) *Table[T] {
	t := &Table[T]{}
	t.table = make(map[string]T, initSize)

	return t
}

func (t *Table[T]) Broadcast(handler func(T)) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, e := range t.table {
		handler(e)
	}
}

func (t *Table[T]) BroadcastParallel(handler func(T), maxParallel int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if maxParallel == 0 {
		maxParallel = len(t.table)
	}

	waitCh := make(chan struct{}, maxParallel)
	for _, e := range t.table {
		waitCh <- struct{}{}
		go func(e T) {
			handler(e)
			<-waitCh
		}(e)
	}
}

func (t *Table[T]) Unicast(key string, handler func(T)) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	e, exist := t.table[key]
	if !exist {
		return errors.New("key not found")
	}

	handler(e)

	return nil
}

func (t *Table[T]) Upsert(key string, e T) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.table[key] = e
}

func (t *Table[T]) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.table, key)
}
