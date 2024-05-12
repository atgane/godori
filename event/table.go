package event

import (
	"errors"
	"sync"
)

type Table[F comparable, T any] struct {
	table map[F]T
	mu    sync.RWMutex
}

func NewTable[F comparable, T any](initSize int) *Table[F, T] {
	t := &Table[F, T]{}
	t.table = make(map[F]T, initSize)

	return t
}

func (t *Table[F, T]) Broadcast(handler func(T)) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, e := range t.table {
		handler(e)
	}
}

func (t *Table[F, T]) BroadcastParallel(handler func(T), maxParallel int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if maxParallel == 0 {
		maxParallel = len(t.table)
	}

	waitCh := make(chan struct{}, maxParallel)
	wg := &sync.WaitGroup{}
	for _, e := range t.table {
		wg.Add(1)
		waitCh <- struct{}{}
		go func(e T) {
			handler(e)
			<-waitCh
			wg.Done()
		}(e)
	}
	wg.Wait()
}

func (t *Table[F, T]) Unicast(key F, handler func(T)) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	e, exist := t.table[key]
	if !exist {
		return errors.New("key not found")
	}

	handler(e)

	return nil
}

func (t *Table[F, T]) Upsert(key F, e T) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.table[key] = e
}

func (t *Table[F, T]) Delete(key F) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.table, key)
}

func (t *Table[F, T]) Len() int {
	t.mu.RLock()
	defer t.mu.Unlock()

	return len(t.table)
}
