// ffwebapi/task/manager_test.go
package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"ffwebapi/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner is a mock implementation of the FFmpegRunner interface for testing.
type mockRunner struct {
	runFunc func(ctx context.Context, t *Task) (string, error)
}

func (m *mockRunner) Run(ctx context.Context, t *Task) (string, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, t)
	}
	return "mock output", nil // Default success behavior
}

func testConfig() *config.Config {
	return &config.Config{
		MaxConcurrency:      1,
		FFTimeout:           10 * time.Second,
		OutputLocalLifetime: 1 * time.Hour,
	}
}

func TestTaskManager_Submit(t *testing.T) {
	cfg := testConfig()
	runner := &mockRunner{}
	mgr, err := NewManager(cfg, runner)
	require.NoError(t, err)

	task, err := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, StatusQueued, task.Status)

	retrievedTask, found := mgr.Get(task.ID)
	assert.True(t, found)
	assert.Equal(t, task.ID, retrievedTask.ID)
}

func TestTaskManager_ProcessTask(t *testing.T) {
	t.Run("successful processing", func(t *testing.T) {
		cfg := testConfig()
		runner := &mockRunner{
			runFunc: func(ctx context.Context, t *Task) (string, error) {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return "success log", nil
			},
		}
		mgr, err := NewManager(cfg, runner)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mgr.Start(ctx)

		task, _ := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
		time.Sleep(50 * time.Millisecond) // Give time for processing

		processedTask, found := mgr.Get(task.ID)
		require.True(t, found)
		assert.Equal(t, StatusCompleted, processedTask.Status)
		assert.Equal(t, "success log", processedTask.FFMpegOutput)
	})

	t.Run("failed processing", func(t *testing.T) {
		cfg := testConfig()
		runner := &mockRunner{
			runFunc: func(ctx context.Context, t *Task) (string, error) {
				return "error log", errors.New("ffmpeg failed")
			},
		}
		mgr, err := NewManager(cfg, runner)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mgr.Start(ctx)

		task, _ := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
		time.Sleep(50 * time.Millisecond) // Give time for processing

		processedTask, found := mgr.Get(task.ID)
		require.True(t, found)
		assert.Equal(t, StatusFailed, processedTask.Status)
		assert.Equal(t, "ffmpeg failed", processedTask.Error)
	})
}

func TestTaskManager_Cancel(t *testing.T) {
	t.Run("cancel queued task", func(t *testing.T) {
		cfg := testConfig()
		// By setting MaxConcurrency to 0, we ensure the worker loop never picks up a task
		cfg.MaxConcurrency = 0
		runner := &mockRunner{}
		mgr, err := NewManager(cfg, runner)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mgr.Start(ctx)

		task, _ := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
		err = mgr.Cancel(task.ID)
		require.NoError(t, err)

		canceledTask, found := mgr.Get(task.ID)
		require.True(t, found)
		assert.Equal(t, StatusCanceled, canceledTask.Status)
	})

	t.Run("cancel processing task", func(t *testing.T) {
		cfg := testConfig()
		processingStarted := make(chan bool)
		runner := &mockRunner{
			runFunc: func(ctx context.Context, t *Task) (string, error) {
				close(processingStarted)
				<-ctx.Done() // Block until context is canceled
				return "canceled output", ctx.Err()
			},
		}
		mgr, err := NewManager(cfg, runner)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mgr.Start(ctx)

		task, _ := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
		<-processingStarted // Wait until the task is actually running

		err = mgr.Cancel(task.ID)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond) // Give time for cancellation to propagate
		processedTask, found := mgr.Get(task.ID)
		require.True(t, found)
		assert.Equal(t, StatusCanceled, processedTask.Status)
	})

	t.Run("cannot cancel completed task", func(t *testing.T) {
		cfg := testConfig()
		runner := &mockRunner{}
		mgr, err := NewManager(cfg, runner)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mgr.Start(ctx)

		task, _ := mgr.Submit("-i ${INPUT_MEDIA}", "input.mp4", "mp4")
		time.Sleep(50 * time.Millisecond) // Let it complete

		err = mgr.Cancel(task.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel task in state: completed")
	})
}
