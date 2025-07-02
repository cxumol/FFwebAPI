package ffmpeg

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "ffwebapi/config"
    "ffwebapi/task"
    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/disk"
    "github.com/shirou/gopsutil/v3/mem"
)

type Runner struct {
    cfg     *config.Config
    tempDir string
}

func NewRunner(cfg *config.Config) (*Runner, error) {
    // Ensure ffmpeg binary is executable
    if _, err := exec.LookPath(cfg.FFBin); err != nil {
        return nil, fmt.Errorf("ffmpeg binary not found or not in PATH: %s", cfg.FFBin)
    }

    // Create and set a temporary directory for all I/O
    tempDir, err := os.MkdirTemp("", "ffwebapi_")
    if err != nil {
        return nil, fmt.Errorf("could not create temp directory: %w", err)
    }
    log.Printf("Using temporary directory: %s", tempDir)
    cfg.TempDir = tempDir

    return &Runner{
        cfg:     cfg,
        tempDir: tempDir,
    }, nil
}

// Run executes an ffmpeg command for a given task.
// It returns the combined stdout/stderr and an error.
func (r *Runner) Run(ctx context.Context, t *task.Task) (string, error) {
    // 1. Check system resources before starting
    if err := r.checkResources(); err != nil {
        return "", fmt.Errorf("insufficient system resources: %w", err)
    }

    // 2. Prepare input file
    inputPath, cleanupInput, err := r.prepareInput(ctx, t.InputMedia, t.ID)
    if err != nil {
        return "", fmt.Errorf("failed to prepare input: %w", err)
    }
    defer cleanupInput()
    t.InputPath = inputPath

    // 3. Prepare command
    // First split the command, then substitute the placeholder.
    // This is safer as it prevents the input path (which could contain spaces) from being split.
    args, err := SplitCommand(t.Command)
    if err != nil {
        return "", err
    }

    foundPlaceholder := false
    for i, arg := range args {
        if strings.Contains(arg, InputMediaPlaceholder) {
            args[i] = strings.Replace(arg, InputMediaPlaceholder, inputPath, 1)
            foundPlaceholder = true
            break // Replace only the first occurrence
        }
    }
    if !foundPlaceholder {
        return "", fmt.Errorf("could not find placeholder %s in command", InputMediaPlaceholder)
    }


    // 4. Prepare output path
    outputFilename := fmt.Sprintf("%s_output.%s", t.ID, t.OutputExt)
    outputPath := filepath.Join(r.tempDir, outputFilename)
    t.OutputPath = outputPath
    args = append(args, outputPath) // FFMpeg's last argument is the output file

    // 5. Execute command
    cmd := exec.CommandContext(ctx, r.cfg.FFBin, args...)
    var outputBuf bytes.Buffer
    cmd.Stdout = &outputBuf
    cmd.Stderr = &outputBuf

    log.Printf("Executing for task %s: %s %s", t.ID, cmd.Path, strings.Join(cmd.Args, " "))

    err = cmd.Run()
    outputLog := outputBuf.String()

    if err != nil {
        // If the command failed, clean up the (likely empty or partial) output file.
        os.Remove(outputPath)
        t.OutputPath = ""
        return outputLog, fmt.Errorf("ffmpeg execution failed: %w", err)
    }

    return outputLog, nil
}

// prepareInput downloads, decodes, or copies the input media to a local temporary file.
// It returns the path to the temp file, a cleanup function, and an error.
func (r *Runner) prepareInput(ctx context.Context, inputMedia string, taskID string) (string, func(), error) {
    // Create a unique temporary file for the input
    tmpFile, err := os.CreateTemp(r.tempDir, fmt.Sprintf("%s_input_*", taskID))
    if err != nil {
        return "", func() {}, err
    }
    
    cleanup := func() {
        tmpFile.Close()
        os.Remove(tmpFile.Name())
    }

    // Handle different input types
    if strings.HasPrefix(inputMedia, "http://") || strings.HasPrefix(inputMedia, "https://") {
        // Input is a URL
        req, _ := http.NewRequestWithContext(ctx, "GET", inputMedia, nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return "", cleanup, err
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            return "", cleanup, fmt.Errorf("failed to download file, status: %s", resp.Status)
        }
        
        // Use a LimitedReader to enforce max input size
        limitedReader := &io.LimitedReader{R: resp.Body, N: r.cfg.MaxInputSize + 1}
        written, err := io.Copy(tmpFile, limitedReader)
        if err != nil {
            return "", cleanup, fmt.Errorf("failed to write downloaded file: %w", err)
        }
        if written > r.cfg.MaxInputSize {
            return "", cleanup, fmt.Errorf("input file size exceeds limit of %d bytes", r.cfg.MaxInputSize)
        }

    } else if strings.HasPrefix(inputMedia, "data:") {
        // Input is a data URI - not implemented for brevity, but this is where it would go
        return "", cleanup, fmt.Errorf("data URI inputs are not yet supported")

    } else {
        // Assume input is a local file path
        srcFile, err := os.Open(inputMedia)
        if err != nil {
            return "", cleanup, fmt.Errorf("could not open local input file: %w", err)
        }
        defer srcFile.Close()

        // Check file size
        info, err := srcFile.Stat()
        if err != nil {
            return "", cleanup, err
        }
        if info.Size() > r.cfg.MaxInputSize {
            return "", cleanup, fmt.Errorf("input file size %d exceeds limit of %d bytes", info.Size(), r.cfg.MaxInputSize)
        }

        if _, err := io.Copy(tmpFile, srcFile); err != nil {
            return "", cleanup, fmt.Errorf("failed to copy local file: %w", err)
        }
    }
    // Need to close here to ensure data is flushed before ffmpeg reads it
    if err := tmpFile.Close(); err != nil {
        return "", cleanup, err
    }
    return tmpFile.Name(), cleanup, nil
}

// checkResources verifies that the system has enough free resources to start a new job.
func (r *Runner) checkResources() error {
    // CPU
    p, err := cpu.Percent(time.Second, false)
    if err != nil {
        log.Printf("Warning: could not get CPU usage: %v", err)
    } else if len(p) > 0 && p[0] > (100.0 - r.cfg.ThrottleCPU) {
        return fmt.Errorf("not enough idle CPU. Current usage: %.2f%%, Idle threshold: %.2f%%", p[0], r.cfg.ThrottleCPU)
    }

    // Memory
    vm, err := mem.VirtualMemory()
    if err != nil {
        log.Printf("Warning: could not get memory usage: %v", err)
    } else if vm.Available < uint64(r.cfg.ThrottleFreeMem) {
        return fmt.Errorf("not enough free memory. Available: %d, Required: %d", vm.Available, r.cfg.ThrottleFreeMem)
    }

    // Disk
    d, err := disk.Usage(r.tempDir)
    if err != nil {
        log.Printf("Warning: could not get disk usage for %s: %v", r.tempDir, err)
    } else if d.Free < uint64(r.cfg.ThrottleFreeDisk) {
        return fmt.Errorf("not enough free disk space. Available: %d, Required: %d", d.Free, r.cfg.ThrottleFreeDisk)
    }
    return nil
}
