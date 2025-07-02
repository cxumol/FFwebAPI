// ffwebapi/main.go
package main

import (
	"context"
	"log"
	"net/http"
	// "os"
	"os/signal"
	"syscall"
	"time"

	"ffwebapi/api"
	"ffwebapi/config"
	"ffwebapi/ffmpeg" // <-- Add this import
	"ffwebapi/task"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize dependencies (Runner first)
	ffmpegRunner, err := ffmpeg.NewRunner(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize ffmpeg runner: %v", err)
	}

	// 3. Initialize task manager and inject the runner
	taskManager, err := task.NewManager(cfg, ffmpegRunner) // <-- CHANGED: Pass runner to constructor
    if err != nil {
        log.Fatalf("Failed to initialize task manager: %v", err)
    }

	// 4. Set up router and server
	router := api.SetupRouter(taskManager, cfg)
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// 5. Start background services and HTTP server
	// Create a context that can be canceled
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	taskManager.Start(ctx)

	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 6. Wait for interrupt signal for graceful shutdown
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("Shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the requests it is currently handling
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}