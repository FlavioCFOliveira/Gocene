package index

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// CopyJobStatus represents the current status of a copy job.
type CopyJobStatus int32

const (
	// CopyJobStatusPending indicates the job is waiting to start.
	CopyJobStatusPending CopyJobStatus = iota
	// CopyJobStatusRunning indicates the job is currently copying.
	CopyJobStatusRunning
	// CopyJobStatusCompleted indicates the job completed successfully.
	CopyJobStatusCompleted
	// CopyJobStatusFailed indicates the job failed.
	CopyJobStatusFailed
	// CopyJobStatusCancelled indicates the job was cancelled.
	CopyJobStatusCancelled
)

// String returns the string representation of the status.
func (s CopyJobStatus) String() string {
	switch s {
	case CopyJobStatusPending:
		return "PENDING"
	case CopyJobStatusRunning:
		return "RUNNING"
	case CopyJobStatusCompleted:
		return "COMPLETED"
	case CopyJobStatusFailed:
		return "FAILED"
	case CopyJobStatusCancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// CopyJobProgress represents the progress of a copy operation.
type CopyJobProgress struct {
	// BytesCopied is the number of bytes copied so far.
	BytesCopied int64

	// TotalBytes is the total number of bytes to copy.
	TotalBytes int64

	// Percentage is the completion percentage (0-100).
	Percentage float64

	// StartTime is when the copy started.
	StartTime time.Time

	// EndTime is when the copy completed (or failed).
	EndTime time.Time

	// CurrentFile is the file currently being copied.
	CurrentFile string
}

// CopyJob handles copying files for replication operations.
// It provides progress tracking, error handling, and retry mechanisms.
type CopyJob struct {
	mu sync.RWMutex

	// id is the unique identifier for this job.
	id string

	// sourcePath is the source file or directory path.
	sourcePath string

	// targetPath is the target file or directory path.
	targetPath string

	// status is the current job status.
	status atomic.Int32

	// progress holds the current copy progress.
	progress CopyJobProgress

	// error holds any error that occurred during copying.
	error error

	// maxRetries is the maximum number of retry attempts.
	maxRetries int

	// retryDelay is the delay between retry attempts.
	retryDelay time.Duration

	// bufferSize is the size of the copy buffer.
	bufferSize int

	// ctx is the context for cancellation.
	ctx context.Context

	// cancel is the cancel function for the context.
	cancel context.CancelFunc
}

// NewCopyJob creates a new CopyJob.
func NewCopyJob(id, sourcePath, targetPath string) (*CopyJob, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	if sourcePath == "" {
		return nil, fmt.Errorf("source path cannot be empty")
	}

	if targetPath == "" {
		return nil, fmt.Errorf("target path cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())

	job := &CopyJob{
		id:         id,
		sourcePath: sourcePath,
		targetPath: targetPath,
		maxRetries: 3,
		retryDelay: time.Second,
		bufferSize: 64 * 1024, // 64KB buffer
		ctx:        ctx,
		cancel:     cancel,
	}

	job.status.Store(int32(CopyJobStatusPending))

	return job, nil
}

// GetID returns the job ID.
func (j *CopyJob) GetID() string {
	return j.id
}

// GetSourcePath returns the source path.
func (j *CopyJob) GetSourcePath() string {
	return j.sourcePath
}

// GetTargetPath returns the target path.
func (j *CopyJob) GetTargetPath() string {
	return j.targetPath
}

// GetStatus returns the current job status.
func (j *CopyJob) GetStatus() CopyJobStatus {
	return CopyJobStatus(j.status.Load())
}

// GetProgress returns the current progress.
func (j *CopyJob) GetProgress() CopyJobProgress {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return CopyJobProgress{
		BytesCopied: j.progress.BytesCopied,
		TotalBytes:  j.progress.TotalBytes,
		Percentage:  j.calculatePercentage(),
		StartTime:   j.progress.StartTime,
		EndTime:     j.progress.EndTime,
		CurrentFile: j.progress.CurrentFile,
	}
}

// GetError returns any error that occurred.
func (j *CopyJob) GetError() error {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.error
}

// SetMaxRetries sets the maximum number of retry attempts.
func (j *CopyJob) SetMaxRetries(maxRetries int) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if maxRetries >= 0 {
		j.maxRetries = maxRetries
	}
}

// GetMaxRetries returns the maximum number of retry attempts.
func (j *CopyJob) GetMaxRetries() int {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.maxRetries
}

// SetRetryDelay sets the delay between retry attempts.
func (j *CopyJob) SetRetryDelay(delay time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if delay > 0 {
		j.retryDelay = delay
	}
}

// GetRetryDelay returns the delay between retry attempts.
func (j *CopyJob) GetRetryDelay() time.Duration {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.retryDelay
}

// SetBufferSize sets the copy buffer size.
func (j *CopyJob) SetBufferSize(size int) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if size > 0 {
		j.bufferSize = size
	}
}

// GetBufferSize returns the copy buffer size.
func (j *CopyJob) GetBufferSize() int {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.bufferSize
}

// Execute runs the copy operation with retry logic.
func (j *CopyJob) Execute() error {
	// Check if already running or completed
	currentStatus := j.GetStatus()
	if currentStatus == CopyJobStatusRunning {
		return fmt.Errorf("copy job is already running")
	}
	if currentStatus == CopyJobStatusCompleted {
		return fmt.Errorf("copy job is already completed")
	}
	if currentStatus == CopyJobStatusFailed {
		return fmt.Errorf("copy job has failed, create a new job to retry")
	}
	if currentStatus == CopyJobStatusCancelled {
		return fmt.Errorf("copy job was cancelled, create a new job to retry")
	}

	// Set status to running
	if !j.status.CompareAndSwap(int32(CopyJobStatusPending), int32(CopyJobStatusRunning)) {
		// Status changed between check and swap
		if j.GetStatus() == CopyJobStatusRunning {
			return fmt.Errorf("copy job is already running")
		}
	}

	// Initialize progress
	j.mu.Lock()
	j.progress.StartTime = time.Now()
	j.progress.CurrentFile = j.sourcePath
	j.mu.Unlock()

	// Calculate total size
	totalSize, err := j.calculateTotalSize()
	if err != nil {
		j.setError(err)
		j.status.Store(int32(CopyJobStatusFailed))
		return err
	}

	j.mu.Lock()
	j.progress.TotalBytes = totalSize
	j.mu.Unlock()

	// Execute with retries
	var lastErr error
	for attempt := 0; attempt <= j.maxRetries; attempt++ {
		select {
		case <-j.ctx.Done():
			j.status.Store(int32(CopyJobStatusCancelled))
			return j.ctx.Err()
		default:
		}

		if attempt > 0 {
			time.Sleep(j.retryDelay)
		}

		err = j.performCopy()
		if err == nil {
			j.status.Store(int32(CopyJobStatusCompleted))
			j.mu.Lock()
			j.progress.EndTime = time.Now()
			j.progress.BytesCopied = j.progress.TotalBytes
			j.mu.Unlock()
			return nil
		}

		lastErr = err
	}

	// All retries exhausted
	j.setError(fmt.Errorf("copy failed after %d attempts: %w", j.maxRetries+1, lastErr))
	j.status.Store(int32(CopyJobStatusFailed))
	j.mu.Lock()
	j.progress.EndTime = time.Now()
	j.mu.Unlock()

	return j.error
}

// Cancel cancels the copy operation.
func (j *CopyJob) Cancel() error {
	currentStatus := j.GetStatus()
	if currentStatus == CopyJobStatusCompleted {
		return fmt.Errorf("copy job is already completed")
	}
	if currentStatus == CopyJobStatusCancelled {
		return nil
	}

	j.cancel()
	j.status.Store(int32(CopyJobStatusCancelled))

	return nil
}

// IsRunning returns true if the job is running.
func (j *CopyJob) IsRunning() bool {
	return j.GetStatus() == CopyJobStatusRunning
}

// IsCompleted returns true if the job is completed.
func (j *CopyJob) IsCompleted() bool {
	return j.GetStatus() == CopyJobStatusCompleted
}

// IsFailed returns true if the job failed.
func (j *CopyJob) IsFailed() bool {
	return j.GetStatus() == CopyJobStatusFailed
}

// IsCancelled returns true if the job was cancelled.
func (j *CopyJob) IsCancelled() bool {
	return j.GetStatus() == CopyJobStatusCancelled
}

// calculateTotalSize calculates the total size of files to copy.
func (j *CopyJob) calculateTotalSize() (int64, error) {
	info, err := os.Stat(j.sourcePath)
	if err != nil {
		return 0, fmt.Errorf("stat source path: %w", err)
	}

	if !info.IsDir() {
		return info.Size(), nil
	}

	var totalSize int64
	err = filepath.Walk(j.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("walking source directory: %w", err)
	}

	return totalSize, nil
}

// performCopy performs the actual copy operation.
func (j *CopyJob) performCopy() error {
	info, err := os.Stat(j.sourcePath)
	if err != nil {
		return fmt.Errorf("stat source path: %w", err)
	}

	if info.IsDir() {
		return j.copyDirectory()
	}

	return j.copyFile(j.sourcePath, j.targetPath)
}

// copyDirectory copies an entire directory.
func (j *CopyJob) copyDirectory() error {
	return filepath.Walk(j.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-j.ctx.Done():
			return j.ctx.Err()
		default:
		}

		// Calculate target path
		relPath, err := filepath.Rel(j.sourcePath, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}
		targetPath := filepath.Join(j.targetPath, relPath)

		if info.IsDir() {
			// Create directory
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			return nil
		}

		// Update current file
		j.mu.Lock()
		j.progress.CurrentFile = path
		j.mu.Unlock()

		// Copy file
		return j.copyFile(path, targetPath)
	})
}

// copyFile copies a single file.
func (j *CopyJob) copyFile(sourcePath, targetPath string) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer sourceFile.Close()

	// Get file info for permissions
	info, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	// Create target file
	targetFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}
	defer targetFile.Close()

	// Copy with progress tracking
	j.mu.RLock()
	bufferSize := j.bufferSize
	j.mu.RUnlock()

	buffer := make([]byte, bufferSize)
	var copied int64

	for {
		select {
		case <-j.ctx.Done():
			return j.ctx.Err()
		default:
		}

		n, err := sourceFile.Read(buffer)
		if n > 0 {
			_, writeErr := targetFile.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("writing to target file: %w", writeErr)
			}
			copied += int64(n)

			// Update progress
			j.mu.Lock()
			j.progress.BytesCopied += int64(n)
			j.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading source file: %w", err)
		}
	}

	// Sync to ensure data is written
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("syncing target file: %w", err)
	}

	return nil
}

// calculatePercentage calculates the completion percentage.
func (j *CopyJob) calculatePercentage() float64 {
	if j.progress.TotalBytes == 0 {
		return 0
	}

	percentage := float64(j.progress.BytesCopied) / float64(j.progress.TotalBytes) * 100
	if percentage > 100 {
		return 100
	}
	return percentage
}

// setError sets the error for this job.
func (j *CopyJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.error = err
}

// GetDuration returns the duration of the copy operation.
func (j *CopyJob) GetDuration() time.Duration {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.progress.StartTime.IsZero() {
		return 0
	}

	if j.progress.EndTime.IsZero() {
		return time.Since(j.progress.StartTime)
	}

	return j.progress.EndTime.Sub(j.progress.StartTime)
}

// GetBytesPerSecond returns the average copy speed.
func (j *CopyJob) GetBytesPerSecond() float64 {
	duration := j.GetDuration()
	if duration == 0 {
		return 0
	}

	j.mu.RLock()
	bytesCopied := j.progress.BytesCopied
	j.mu.RUnlock()

	return float64(bytesCopied) / duration.Seconds()
}

// String returns a string representation of the CopyJob.
func (j *CopyJob) String() string {
	progress := j.GetProgress()
	return fmt.Sprintf("CopyJob{id=%s, status=%s, progress=%.1f%%}",
		j.id, j.GetStatus(), progress.Percentage)
}
