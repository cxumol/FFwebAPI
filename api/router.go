package api

import (
    "ffwebapi/config"
    "ffwebapi/task"
    "github.com/gin-gonic/gin"
)

func SetupRouter(tm *task.Manager, cfg *config.Config) *gin.Engine {
    r := gin.Default()
    h := NewHandler(tm, cfg)
    
    // Health check
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    v1 := r.Group("/api/v1")
    v1.Use(AuthMiddleware(cfg))
    {
        // Sync endpoint (with limitations)
        v1.POST("/call", h.handleSyncCall)

        // Async task endpoints
        v1.POST("/tasks", h.handleCreateTask)
        v1.GET("/tasks", h.handleListTasks)
        v1.GET("/tasks/:taskId", h.handleGetTaskStatus)
        v1.PATCH("/tasks/:taskId/cancel", h.handleCancelTask)

        // File download endpoint (does not need auth if URLs are unguessable)
        // but we put it here for consistency.
        v1.GET("/files/:filename", h.handleGetFile)
    }
    return r
}
