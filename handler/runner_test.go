package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockRunnerHandler struct {
	Called   bool
	LastData int
}

func (m *mockRunnerHandler) OnCall(e *RunnerEvent[int]) {
	m.Called = true
	m.LastData = e.Field
}

func TestRunner(t *testing.T) {
	config := NewRunnerConfig()

	handler := &mockRunnerHandler{}
	runner := NewRunner(handler, config)
	go runner.Run()

	event := 42
	err := runner.Send(event)
	require.NoError(t, err)

	// event subscriber wait
	time.Sleep(time.Millisecond * 100)
	require.True(t, handler.Called)
	require.Equal(t, event, handler.LastData)

	// Close() check
	runner.Close()
	require.True(t, runner.closed.Load())
	require.True(t, runner.runnerEventloop.IsClosed())
}
