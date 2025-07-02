package api

import (
    "fmt"
    "log"
    "net/http"
    "path/filepath"
    "strings"

    "ffwebapi/config"
    "ffwebapi/ffmpeg"
    "ffwebapi/task"
    "github.com/gin-gonic/gin"
)

type Handler struct {
    taskManager *task.Manager
    cfg         *config.Config
}

func NewHandler(tm *task.Manager, cfg *config.Config) *Handler {
    return &Handler{
        taskManager: tm,
        cfg:         cfg,
    }
}

type TaskRequest struct {
    Command    string `json:"command" form:"command" binding:"required"`
    InputMedia string `json:"inputMedia" form:"inputMedia"`
    OutputExt  string `json:"outputExt" form:"outputExt" binding:"required"`
}

// handleCreateTask handles asynchronous task creation.
func (h *Handler) handleCreateTask(c *gin.Context) {
    var req TaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Sanitize and validate before accepting the task
    splitArgs, err := ffmpeg.SplitCommand(req.Command)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid command syntax: %v", err)})
        return
    }

    if err := ffmpeg.SanitizeAndValidateArgs(splitArgs); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid command: %v", err)})
        return
    }

    t, err := h.taskManager.Submit(req.Command, req.InputMedia, req.OutputExt)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task", "details": err.Error()})
        return
    }

    c.JSON(http.StatusAccepted, gin.H{"taskId": t.ID})
}

// handleListTasks lists all tasks.
func (h *Handler) handleListTasks(c *gin.Context) {
    tasks := h.taskManager.List()
    c.JSON(http.StatusOK, tasks)
}

// buildDownloadURL constructs the full URL for a completed task's file.
func (h *Handler) buildDownloadURL(c *gin.Context, t *task.Task) {
    if t.Status != task.StatusCompleted || t.OutputPath == "" {
        return
    }

    baseURL := h.cfg.BaseURL
    if baseURL == "" {
        scheme := "http"
        if c.Request.TLS != nil {
            scheme = "https"
        }
        baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
    }
    baseURL = strings.TrimSuffix(baseURL, "/")

    filename := filepath.Base(t.OutputPath)
    t.DownloadURL = fmt.Sprintf("%s/api/v1/files/%s", baseURL, filename)
}

// handleGetTaskStatus retrieves the status of a single task.
func (h *Handler) handleGetTaskStatus(c *gin.Context) {
    taskID := c.Param("taskId")
    t, found := h.taskManager.Get(taskID)
    if !found {
        c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
        return
    }

    h.buildDownloadURL(c, t)
    c.JSON(http.StatusOK, t)
}

// handleCancelTask cancels a task.
func (h *Handler) handleCancelTask(c *gin.Context) {
    taskID := c.Param("taskId")
    err := h.taskManager.Cancel(taskID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Task cancellation requested"})
}

// handleGetFile serves a completed output file.
func (h *Handler) handleGetFile(c *gin.Context) {
    filename := c.Param("filename")
    filePath, err := h.taskManager.GetFilePath(filename)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
        return
    }
    c.File(filePath)
}

// handleSyncCall is a placeholder for the sync call logic.
// This is more complex because it bypasses the main queue. For simplicity,
// this example will reject sync calls if the server is already at capacity.
func (h *Handler) handleSyncCall(c *gin.Context) {
    // This endpoint is complex to implement correctly without blocking.
    // A robust implementation might use a separate, smaller queue or a different mechanism.
    // For this example, we will simply return a "not implemented" or "use async" message.
    log.Println("Sync call endpoint is not fully implemented. Please use the async /tasks endpoint.")
    c.JSON(http.StatusNotImplemented, gin.H{"error": "Synchronous calls are not recommended. Please use the /api/v1/tasks endpoint for better performance and reliability."})
}
