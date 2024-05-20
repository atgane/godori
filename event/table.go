package event

import (
	"errors"
	"sync"
)

// Table is a thread-safe map structure.
type Table[F comparable, T any] struct {
	table map[F]T      // Underlying map to store key-value pairs.
	mu    sync.RWMutex // Read-write mutex to protect the map.
}

// NewTable creates a new Table with an initial size.
func NewTable[F comparable, T any](initSize int) *Table[F, T] {
	t := &Table[F, T]{}
	t.table = make(map[F]T, initSize)

	return t
}

// Broadcast applies the handler function to each element in the table.
func (t *Table[F, T]) Broadcast(handler func(T)) {
	t.mu.RLock()         // Acquire read lock.
	defer t.mu.RUnlock() // Release read lock when function exits.

	for _, e := range t.table {
		handler(e)
	}
}

// BroadcastParallel applies the handler function to each element in the table in parallel.
func (t *Table[F, T]) BroadcastParallel(handler func(T), maxParallel int) {
	t.mu.RLock()         // Acquire read lock.
	defer t.mu.RUnlock() // Release read lock when function exits.

	if maxParallel == 0 {
		maxParallel = len(t.table) // Set maxParallel to size of table if it's 0.
	}

	waitCh := make(chan struct{}, maxParallel) // Channel to control the number of parallel executions.
	wg := &sync.WaitGroup{}
	for _, e := range t.table {
		wg.Add(1)
		waitCh <- struct{}{}
		go func(e T) {
			handler(e)
			<-waitCh  // Signal that one handler execution is done.
			wg.Done() // Decrement the wait group counter.
		}(e)
	}
	wg.Wait() // Wait for all handler executions to complete.
}

// Unicast applies the handler function to the element identified by the key.
func (t *Table[F, T]) Unicast(key F, handler func(T)) error {
	t.mu.RLock()         // Acquire read lock.
	defer t.mu.RUnlock() // Release read lock when function exits.

	e, exist := t.table[key]
	if !exist {
		return errors.New("key not found")
	}

	handler(e)

	return nil
}

// LoadOrStore returns the existing value for the key if present, otherwise stores and returns the given value.
func (t *Table[F, T]) LoadOrStore(key F, e T) (val T, load bool) {
	t.mu.Lock()         // Acquire write lock.
	defer t.mu.Unlock() // Release write lock when function exits.

	v, exist := t.table[key]
	if !exist {
		t.table[key] = e
		return e, false
	}

	return v, true
}

// Upsert inserts or updates the value for the given key.
func (t *Table[F, T]) Upsert(key F, e T) {
	t.mu.Lock()         // Acquire write lock.
	defer t.mu.Unlock() // Release write lock when function exits.

	t.table[key] = e
}

// Delete removes the element identified by the key.
func (t *Table[F, T]) Delete(key F) {
	t.mu.Lock()         // Acquire write lock.
	defer t.mu.Unlock() // Release write lock when function exits.

	delete(t.table, key)
}

// Len returns the number of elements in the table.
func (t *Table[F, T]) Len() int {
	t.mu.RLock()         // Acquire read lock.
	defer t.mu.RUnlock() // Release read lock when function exits.

	return len(t.table)
}
