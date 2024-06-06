package event

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestReserverRaceCondition(t *testing.T) {
	var wg sync.WaitGroup
	reserver := NewReserver(100*time.Millisecond, func() {
		t.Log("Handler executed")
	})

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reserver.Start()
			reserver.Cancel()
		}()
	}

	wg.Wait()

	// Ensure the reserver is not started
	if reserver.started.Load() {
		t.Error("Reserver should not be started")
	}
}

func TestReserverBlocking(t *testing.T) {
	reserver := NewReserver(100*time.Millisecond, func() {
		t.Log("Handler executed")
	})

	reserver.Start()

	// Start another reservation which should be blocked
	started := make(chan bool)
	go func() {
		reserver.Start()
		started <- true
	}()

	select {
	case <-started:
		// Passed
	case <-time.After(200 * time.Millisecond):
		t.Error("Second reservation should start")
	}

	reserver.Cancel()

	// Start another reservation which should now be possible
	go func() {
		reserver.Start()
		started <- true
	}()

	select {
	case <-started:
		// Passed
	case <-time.After(200 * time.Millisecond):
		t.Error("Second reservation should start after cancellation")
	}
}

func TestReserverGoroutineLeak(t *testing.T) {
	reserver := NewReserver(100*time.Millisecond, func() {
		t.Log("Handler executed")
	})

	beforeGoroutines := runtime.NumGoroutine()

	reserver.Start()
	time.Sleep(200 * time.Millisecond) // Wait for handler to execute

	afterGoroutines := runtime.NumGoroutine()
	if afterGoroutines > beforeGoroutines {
		t.Error("Goroutine leak detected")
	}
}
