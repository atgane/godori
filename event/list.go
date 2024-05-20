package event

import (
	"errors"
	"sync"
)

// List is a thread-safe list structure.
type List[T any] struct {
	list []T          // Slice to store list elements.
	mu   sync.RWMutex // Read-write mutex to protect the list.
}

// NewList creates a new List with an initial size.
func NewList[T any](initSize int) *List[T] {
	li := &List[T]{}
	li.list = make([]T, initSize)

	return li
}

// Broadcast applies the handler function to each element in the list.
func (li *List[T]) Broadcast(handler func(T)) {
	li.mu.RLock()         // Acquire read lock.
	defer li.mu.RUnlock() // Release read lock when function exits.

	for _, e := range li.list {
		handler(e)
	}
}

// BroadcastParallel applies the handler function to each element in the list in parallel.
func (li *List[T]) BroadcastParallel(handler func(T), maxParallel int) {
	li.mu.RLock()         // Acquire read lock.
	defer li.mu.RUnlock() // Release read lock when function exits.

	if maxParallel == 0 {
		maxParallel = len(li.list) // Set maxParallel to size of list if it's 0.
	}

	waitCh := make(chan struct{}, maxParallel) // Channel to control the number of parallel executions.
	wg := &sync.WaitGroup{}
	for _, e := range li.list {
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
func (li *List[T]) Unicast(key int, handler func(T)) error {
	li.mu.RLock()         // Acquire read lock.
	defer li.mu.RUnlock() // Release read lock when function exits.

	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	handler(li.list[key])

	return nil
}

// Insert adds a new element to the end of the list.
func (li *List[T]) Insert(e T) {
	li.mu.Lock()         // Acquire write lock.
	defer li.mu.Unlock() // Release write lock when function exits.

	li.list = append(li.list, e)
}

// Update modifies the element at the specified index.
func (li *List[T]) Update(key int, e T) error {
	li.mu.Lock()         // Acquire write lock.
	defer li.mu.Unlock() // Release write lock when function exits.

	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	li.list[key] = e

	return nil
}

// Delete removes the element at the specified index.
func (li *List[T]) Delete(key int) error {
	li.mu.Lock()         // Acquire write lock.
	defer li.mu.Unlock() // Release write lock when function exits.

	if key >= len(li.list) {
		return errors.New("key exceeds list size")
	}

	li.list = append(li.list[:key], li.list[key+1:]...)

	return nil
}
