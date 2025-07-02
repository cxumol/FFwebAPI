// ffwebapi/api/handler_test.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"ffwebapi/config"
	"ffwebapi/task"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockRunner struct{}

func (m *mockRunner) Run(ctx context.Context, t *task.Task) (string, error) {
	t.OutputPath = fmt.Sprintf("/tmp/%s_output.mp4", t.ID)
	t.DownloadURL = fmt.Sprintf("/api/v1/files/%s", t.OutputPath)
	return "ok", nil
}

func setupTestRouter() (*gin.Engine, *config.Config, *task.Manager) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		MaxConcurrency: 1,
		AuthEnable:     false,
	}
	runner := &mockRunner{}
	// FIX: The call to NewManager now correctly expects only one return value.
	tm, _ := task.NewManager(cfg, runner)
	router := SetupRouter(tm, cfg)
	return router, cfg, tm
}

func TestHandleCreateTask(t *testing.T) {
	router, _, tm := setupTestRouter()

	w := httptest.NewRecorder()
	reqBody := `{"command": "-i ${INPUT_MEDIA} -vcodec copy", "inputMedia": "test.mkv", "outputExt": "mp4"}`
	req, _ := http.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp["taskId"])

	_, found := tm.Get(resp["taskId"])
	assert.True(t, found)
}

func TestHandleGetTaskStatus(t *testing.T) {
	router, _, tm := setupTestRouter()

	testTask, err := tm.Submit("-i ${INPUT_MEDIA} -vcodec copy", "test.mp4", "mp4")
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // Give time for processing

	testTask.Status = task.StatusCompleted
	testTask.OutputPath = "/some/path/test123_completed_output.mp4"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tasks/"+testTask.ID, nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var respTask task.Task
	err = json.Unmarshal(w.Body.Bytes(), &respTask)
	assert.NoError(t, err)
	assert.Equal(t, testTask.ID, respTask.ID)
	assert.Equal(t, task.StatusCompleted, respTask.Status)
	assert.Contains(t, respTask.DownloadURL, "/api/v1/files/test123_completed_output.mp4")

	// Test Not Found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/tasks/nonexistent", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthMiddleware(t *testing.T) {
	router, cfg, _ := setupTestRouter()

	t.Run("Auth disabled", func(t *testing.T) {
		cfg.AuthEnable = false
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tasks", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Auth enabled, no token", func(t *testing.T) {
		cfg.AuthEnable = true
		cfg.AuthKey = "secret"
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tasks", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Auth enabled, wrong token", func(t *testing.T) {
		cfg.AuthEnable = true
		cfg.AuthKey = "secret"
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Auth enabled, correct token", func(t *testing.T) {
		cfg.AuthEnable = true
		cfg.AuthKey = "secret"
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer secret")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
