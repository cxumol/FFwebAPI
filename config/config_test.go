// ffwebapi/config/config_test.go
package config_test // Use an external test package

import (
	"ffwebapi/config" // Import the package we are testing
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads default values correctly", func(t *testing.T) {
		// Ensure no env vars are lingering from other tests
		t.Setenv("FFWEBAPI_PORT", "")
		t.Setenv("FFWEBAPI_MAX_CONCURRENCY", "")
		t.Setenv("FFWEBAPI_AUTH_ENABLE", "")
		t.Setenv("FFWEBAPI_FF_TIMEOUT", "")
		t.Setenv("FFWEBAPI_MAX_INPUT_SIZE", "")

		cfg, err := config.Load() // Use the package prefix
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		assert.Equal(t, "8080", cfg.Port)
		assert.Equal(t, 1, cfg.MaxConcurrency)
		assert.Equal(t, false, cfg.AuthEnable)
		assert.Equal(t, "ffmpeg", cfg.FFBin)
		assert.Equal(t, 12*time.Minute+3*time.Second, cfg.FFTimeout)
		assert.Equal(t, int64(200*1024*1024), cfg.MaxInputSize)
	})

	t.Run("overrides defaults with environment variables", func(t *testing.T) {
		t.Setenv("FFWEBAPI_PORT", "9999")
		t.Setenv("FFWEBAPI_MAX_CONCURRENCY", "10")
		t.Setenv("FFWEBAPI_AUTH_ENABLE", "true")
		t.Setenv("FFWEBAPI_AUTH_KEY", "newsecret")
		t.Setenv("FFWEBAPI_MAX_INPUT_SIZE", "50MB")

		cfg, err := config.Load() // Use the package prefix
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		assert.Equal(t, "9999", cfg.Port)
		assert.Equal(t, 10, cfg.MaxConcurrency)
		assert.Equal(t, true, cfg.AuthEnable)
		assert.Equal(t, "newsecret", cfg.AuthKey)
		assert.Equal(t, int64(50*1024*1024), cfg.MaxInputSize)
	})
}
