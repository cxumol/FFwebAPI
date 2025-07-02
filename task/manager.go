package task

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "sync"
    "time"

    "ffwebapi/config"
    // "ffwebapi/ffmpeg"
    "github.com/lithammer/shortuuid/v4"
)

type FFmpegRunner interface {
	Run(ctx context.Context, t *Task) (logOutput string, err error)
}

type Manager struct {
    cfg            *config.Config
    tasks          sync.Map // More scalable than a mutex-protected map
    taskQueue      chan *Task
    concurrencySem chan struct{}
    runner         FFmpegRunner
}

func NewManager(cfg *config.Config, runner FFmpegRunner) (*Manager, error) {
    m := &Manager{
        cfg:            cfg,
        tasks:          sync.Map{},
        taskQueue:      make(chan *Task, 100), // Buffered queue
        concurrencySem: make(chan struct{}, cfg.MaxConcurrency),
        runner:         runner,
    }
    return m, nil
}

func (m *Manager) Start(ctx context.Context) {
    log.Println("Task manager started. Concurrency limit:", m.cfg.MaxConcurrency)
    go m.cleanupLoop(ctx)
    go m.workerLoop(ctx)
}

// workerLoop pulls tasks from the queue and processes them
func (m *Manager) workerLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            log.Println("Worker loop shutting down.")
            return
        case task := <-m.taskQueue:
            // Wait for a free processing slot
            m.concurrencySem <- struct{}{}
            go func(t *Task) {
                defer func() { <-m.concurrencySem }() // Release slot
                m.processTask(ctx, t)
            }(task)
        }
    }
}

// processTask handles the execution of a single task
func (m *Manager) processTask(parentCtx context.Context, t *Task) {
    // Create a new context for this specific task for cancellation and timeout
    taskCtx, cancel := context.WithTimeout(parentCtx, m.cfg.FFTimeout)
    t.cancelFunc = cancel // Store cancel func so it can be called externally
    defer cancel()

    // Check if task was canceled while in queue
    if t.Status == StatusCanceled {
        log.Printf("Task %s was canceled before processing.", t.ID)
        return
    }

    log.Printf("Processing task %s", t.ID)
    t.Status = StatusProcessing
    t.StartedAt = time.Now()
    m.tasks.Store(t.ID, t)

    outputLog, err := m.runner.Run(taskCtx, t)
    t.FFMpegOutput = outputLog

    if err != nil {
        if err == context.Canceled || err == context.DeadlineExceeded {
            log.Printf("Task %s canceled or timed out.", t.ID)
            t.Status = StatusCanceled
            t.Error = "Task was canceled or timed out"
        } else {
            log.Printf("Task %s failed: %v", t.ID, err)
            t.Status = StatusFailed
            t.Error = err.Error()
        }
    } else {
        log.Printf("Task %s completed successfully.", t.ID)
        t.Status = StatusCompleted
    }
    t.CompletedAt = time.Now()
    m.tasks.Store(t.ID, t)
}

// cleanupLoop periodically removes old output files
func (m *Manager) cleanupLoop(ctx context.Context) {
    ticker := time.NewTicker(m.cfg.OutputLocalLifetime / 4) // Check 4 times per lifetime
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Println("Cleanup loop shutting down.")
            return
        case <-ticker.C:
            m.tasks.Range(func(key, value interface{}) bool {
                task := value.(*Task)
                if task.Status == StatusCompleted && time.Since(task.CompletedAt) > m.cfg.OutputLocalLifetime {
                    if task.OutputPath != "" {
                        log.Printf("Cleaning up old output file: %s", task.OutputPath)
                        os.Remove(task.OutputPath)
                        // We can also remove the task from the map if desired
                        // m.tasks.Delete(key)
                    }
                }
                return true
            })
        }
    }
}

func (m *Manager) Submit(command, inputMedia, outputExt string) (*Task, error) {
    t := &Task{
        ID:         fmt.Sprintf("%s_%d", shortuuid.New(), time.Now().Unix()),
        Status:     StatusQueued,
        Command:    command,
        InputMedia: inputMedia,
        OutputExt:  outputExt,
        CreatedAt:  time.Now(),
    }

    m.tasks.Store(t.ID, t)
    m.taskQueue <- t
    log.Printf("Task %s submitted to queue.", t.ID)
    return t, nil
}

func (m *Manager) Get(taskID string) (*Task, bool) {
    if val, ok := m.tasks.Load(taskID); ok {
        return val.(*Task), true
    }
    return nil, false
}

func (m *Manager) List() []*Task {
    var taskList []*Task
    m.tasks.Range(func(key, value interface{}) bool {
        taskList = append(taskList, value.(*Task))
        return true
    })
    return taskList
}

func (m *Manager) Cancel(taskID string) error {
    val, ok := m.tasks.Load(taskID)
    if !ok {
        return fmt.Errorf("task %s not found", taskID)
    }

    task := val.(*Task)
    switch task.Status {
    case StatusCompleted, StatusFailed, StatusCanceled:
        return fmt.Errorf("cannot cancel task in state: %s", task.Status)
    case StatusQueued:
        task.Status = StatusCanceled
        task.Error = "Canceled by user while in queue"
        m.tasks.Store(task.ID, task)
        log.Printf("Task %s marked as canceled in queue.", task.ID)
    case StatusProcessing:
        if task.cancelFunc != nil {
            task.cancelFunc()
            log.Printf("Cancellation signal sent to running task %s.", task.ID)
        } else {
            return fmt.Errorf("task %s is processing but has no cancellation handle", task.ID)
        }
    }
    return nil
}

func (m *Manager) GetFilePath(filename string) (string, error) {
    // Security: Prevent path traversal
    cleanFilename := filepath.Base(filename)
    if cleanFilename != filename {
        return "", fmt.Errorf("invalid filename")
    }

    fullPath := filepath.Join(m.cfg.TempDir, cleanFilename)
    if _, err := os.Stat(fullPath); os.IsNotExist(err) {
        return "", fmt.Errorf("file not found")
    }
    return fullPath, nil
}
