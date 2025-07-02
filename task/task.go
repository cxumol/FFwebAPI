package task

import (
    "context"
    "time"
)

type Status string

const (
    StatusQueued     Status = "queued"
    StatusProcessing Status = "processing"
    StatusCompleted  Status = "completed"
    StatusFailed     Status = "failed"
    StatusCanceled   Status = "canceled"
)

type Task struct {
    ID           string    `json:"id"`
    Status       Status    `json:"status"`
    Command      string    `json:"-"` // Don't expose raw command
    OutputExt    string    `json:"-"`
    InputMedia   string    `json:"-"`
    InputPath    string    `json:"-"` // Path to local temp input file
    OutputPath   string    `json:"outputPath,omitempty"`
    DownloadURL  string    `json:"downloadUrl,omitempty"`
    Error        string    `json:"error,omitempty"`
    CreatedAt    time.Time `json:"createdAt"`
    StartedAt    time.Time `json:"startedAt,omitempty"`
    CompletedAt  time.Time `json:"completedAt,omitempty"`
    FFMpegOutput string    `json:"ffmpegOutput,omitempty"` // Stderr from ffmpeg
    cancelFunc   context.CancelFunc
}
