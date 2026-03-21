package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCopyJob(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		sourcePath  string
		targetPath  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid parameters",
			id:         "test-job-1",
			sourcePath: "/tmp/source",
			targetPath: "/tmp/target",
			wantErr:    false,
		},
		{
			name:        "empty id",
			id:          "",
			sourcePath:  "/tmp/source",
			targetPath:  "/tmp/target",
			wantErr:     true,
			errContains: "id cannot be empty",
		},
		{
			name:        "empty source path",
			id:          "test-job-2",
			sourcePath:  "",
			targetPath:  "/tmp/target",
			wantErr:     true,
			errContains: "source path cannot be empty",
		},
		{
			name:        "empty target path",
			id:          "test-job-3",
			sourcePath:  "/tmp/source",
			targetPath:  "",
			wantErr:     true,
			errContains: "target path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := NewCopyJob(tt.id, tt.sourcePath, tt.targetPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewCopyJob() expected error but got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("NewCopyJob() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewCopyJob() unexpected error = %v", err)
				return
			}

			if job.GetID() != tt.id {
				t.Errorf("job.GetID() = %v, want %v", job.GetID(), tt.id)
			}
			if job.GetSourcePath() != tt.sourcePath {
				t.Errorf("job.GetSourcePath() = %v, want %v", job.GetSourcePath(), tt.sourcePath)
			}
			if job.GetTargetPath() != tt.targetPath {
				t.Errorf("job.GetTargetPath() = %v, want %v", job.GetTargetPath(), tt.targetPath)
			}
			if job.GetStatus() != CopyJobStatusPending {
				t.Errorf("job.GetStatus() = %v, want %v", job.GetStatus(), CopyJobStatusPending)
			}
		})
	}
}

func TestCopyJobStatus_String(t *testing.T) {
	tests := []struct {
		status CopyJobStatus
		want   string
	}{
		{CopyJobStatusPending, "PENDING"},
		{CopyJobStatusRunning, "RUNNING"},
		{CopyJobStatusCompleted, "COMPLETED"},
		{CopyJobStatusFailed, "FAILED"},
		{CopyJobStatusCancelled, "CANCELLED"},
		{CopyJobStatus(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("CopyJobStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopyJob_Execute_SingleFile(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(sourcePath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create copy job
	job, err := NewCopyJob("test-single-file", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Execute
	if err := job.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Verify status
	if job.GetStatus() != CopyJobStatusCompleted {
		t.Errorf("GetStatus() = %v, want %v", job.GetStatus(), CopyJobStatusCompleted)
	}

	// Verify target file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Errorf("Target file does not exist")
	}

	// Verify content
	targetContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read target file: %v", err)
	}
	if string(targetContent) != string(content) {
		t.Errorf("Target content = %v, want %v", string(targetContent), string(content))
	}

	// Verify progress
	progress := job.GetProgress()
	if progress.BytesCopied != int64(len(content)) {
		t.Errorf("Progress.BytesCopied = %v, want %v", progress.BytesCopied, len(content))
	}
	if progress.TotalBytes != int64(len(content)) {
		t.Errorf("Progress.TotalBytes = %v, want %v", progress.TotalBytes, len(content))
	}
}

func TestCopyJob_Execute_Directory(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source directory structure
	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	// Create subdirectories
	subDir := filepath.Join(sourceDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create files
	files := map[string]string{
		filepath.Join(sourceDir, "file1.txt"): "content1",
		filepath.Join(sourceDir, "file2.txt"): "content2",
		filepath.Join(subDir, "file3.txt"):    "content3",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create copy job
	job, err := NewCopyJob("test-directory", sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Execute
	if err := job.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Verify status
	if job.GetStatus() != CopyJobStatusCompleted {
		t.Errorf("GetStatus() = %v, want %v", job.GetStatus(), CopyJobStatusCompleted)
	}

	// Verify files were copied
	for sourcePath, expectedContent := range files {
		relPath, _ := filepath.Rel(sourceDir, sourcePath)
		targetPath := filepath.Join(targetDir, relPath)

		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Errorf("Failed to read target file %s: %v", targetPath, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: got %v, want %v", targetPath, string(content), expectedContent)
		}
	}
}

func TestCopyJob_Cancel(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")

	// Create large file
	content := make([]byte, 1024*1024) // 1MB
	if err := os.WriteFile(sourcePath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create copy job
	job, err := NewCopyJob("test-cancel", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Cancel should work before execution
	if err := job.Cancel(); err != nil {
		t.Errorf("Cancel() before execution error = %v", err)
	}

	if job.GetStatus() != CopyJobStatusCancelled {
		t.Errorf("GetStatus() after cancel = %v, want %v", job.GetStatus(), CopyJobStatusCancelled)
	}

	// Create new job for execution test
	job2, err := NewCopyJob("test-cancel-2", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Execute in goroutine
	done := make(chan error, 1)
	go func() {
		done <- job2.Execute()
	}()

	// Cancel after a short delay
	time.Sleep(10 * time.Millisecond)
	job2.Cancel()

	// Wait for completion
	select {
	case err := <-done:
		if err != context.Canceled && err != nil {
			t.Logf("Execute() after cancel returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for cancelled job")
	}
}

func TestCopyJob_Retry(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try to copy from non-existent source
	sourcePath := filepath.Join(tempDir, "nonexistent", "file.txt")
	targetPath := filepath.Join(tempDir, "target.txt")

	job, err := NewCopyJob("test-retry", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Set retry parameters
	job.SetMaxRetries(2)
	job.SetRetryDelay(10 * time.Millisecond)

	// Execute should fail after retries
	err = job.Execute()
	if err == nil {
		t.Error("Execute() expected error but got nil")
	}

	if job.GetStatus() != CopyJobStatusFailed {
		t.Errorf("GetStatus() = %v, want %v", job.GetStatus(), CopyJobStatusFailed)
	}

	if job.GetError() == nil {
		t.Error("GetError() expected error but got nil")
	}
}

func TestCopyJob_DoubleExecute(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(sourcePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create copy job
	job, err := NewCopyJob("test-double", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// First execute
	if err := job.Execute(); err != nil {
		t.Fatalf("First Execute() error = %v", err)
	}

	// Second execute should fail (job already completed)
	if err := job.Execute(); err == nil {
		t.Error("Second Execute() expected error but got nil")
	}
	if err := job.Execute(); !containsString(err.Error(), "already completed") {
		t.Errorf("Second Execute() error = %v, should contain 'already completed'", err)
	}
}

func TestCopyJob_Progress(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(sourcePath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create copy job
	job, err := NewCopyJob("test-progress", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Execute
	if err := job.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Check progress
	progress := job.GetProgress()
	if progress.Percentage != 100 {
		t.Errorf("Progress.Percentage = %v, want 100", progress.Percentage)
	}
	if progress.BytesCopied != int64(len(content)) {
		t.Errorf("Progress.BytesCopied = %v, want %v", progress.BytesCopied, len(content))
	}
	if progress.TotalBytes != int64(len(content)) {
		t.Errorf("Progress.TotalBytes = %v, want %v", progress.TotalBytes, len(content))
	}
	if progress.StartTime.IsZero() {
		t.Error("Progress.StartTime is zero")
	}
	if progress.EndTime.IsZero() {
		t.Error("Progress.EndTime is zero")
	}

	// Check duration
	duration := job.GetDuration()
	if duration <= 0 {
		t.Errorf("GetDuration() = %v, want positive duration", duration)
	}

	// Check bytes per second
	bps := job.GetBytesPerSecond()
	if bps <= 0 {
		t.Errorf("GetBytesPerSecond() = %v, want positive value", bps)
	}
}

func TestCopyJob_SettersAndGetters(t *testing.T) {
	job, err := NewCopyJob("test", "/tmp/source", "/tmp/target")
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Test max retries
	job.SetMaxRetries(5)
	if got := job.GetMaxRetries(); got != 5 {
		t.Errorf("GetMaxRetries() = %v, want 5", got)
	}

	// Test negative max retries (should be ignored)
	job.SetMaxRetries(-1)
	if got := job.GetMaxRetries(); got != 5 {
		t.Errorf("GetMaxRetries() after negative = %v, want 5", got)
	}

	// Test retry delay
	job.SetRetryDelay(2 * time.Second)
	if got := job.GetRetryDelay(); got != 2*time.Second {
		t.Errorf("GetRetryDelay() = %v, want 2s", got)
	}

	// Test invalid retry delay (should be ignored)
	job.SetRetryDelay(-1)
	if got := job.GetRetryDelay(); got != 2*time.Second {
		t.Errorf("GetRetryDelay() after invalid = %v, want 2s", got)
	}

	// Test buffer size
	job.SetBufferSize(1024)
	if got := job.GetBufferSize(); got != 1024 {
		t.Errorf("GetBufferSize() = %v, want 1024", got)
	}

	// Test invalid buffer size (should be ignored)
	job.SetBufferSize(-1)
	if got := job.GetBufferSize(); got != 1024 {
		t.Errorf("GetBufferSize() after invalid = %v, want 1024", got)
	}
}

func TestCopyJob_String(t *testing.T) {
	job, err := NewCopyJob("test-job", "/tmp/source", "/tmp/target")
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	str := job.String()
	if !containsString(str, "test-job") {
		t.Errorf("String() should contain job ID")
	}
	if !containsString(str, "PENDING") {
		t.Errorf("String() should contain status")
	}
}

func TestCopyJob_IsMethods(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(sourcePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	job, err := NewCopyJob("test-is", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	// Initial state
	if job.IsRunning() {
		t.Error("IsRunning() should be false for pending job")
	}
	if job.IsCompleted() {
		t.Error("IsCompleted() should be false for pending job")
	}
	if job.IsFailed() {
		t.Error("IsFailed() should be false for pending job")
	}
	if job.IsCancelled() {
		t.Error("IsCancelled() should be false for pending job")
	}

	// After completion
	if err := job.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if job.IsRunning() {
		t.Error("IsRunning() should be false for completed job")
	}
	if !job.IsCompleted() {
		t.Error("IsCompleted() should be true for completed job")
	}
	if job.IsFailed() {
		t.Error("IsFailed() should be false for completed job")
	}
	if job.IsCancelled() {
		t.Error("IsCancelled() should be false for completed job")
	}
}

func TestCopyJob_CancelCompleted(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "copyjob-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.txt")
	targetPath := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(sourcePath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	job, err := NewCopyJob("test-cancel-completed", sourcePath, targetPath)
	if err != nil {
		t.Fatalf("NewCopyJob() error = %v", err)
	}

	if err := job.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Canceling completed job should fail
	if err := job.Cancel(); err == nil {
		t.Error("Cancel() on completed job should return error")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
