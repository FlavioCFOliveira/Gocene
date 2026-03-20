package index

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// LocalReplicator handles local file-based index replication.
// It replicates index data from a source directory to a target directory.
type LocalReplicator struct {
	mu sync.RWMutex

	// sourcePath is the path to the source directory
	sourcePath string

	// targetPath is the path to the target directory
	targetPath string

	// base provides common replicator functionality
	base *ReplicatorWithStats

	// isOpen indicates if the replicator is open
	isOpen atomic.Bool

	// currentRevision is the current replicated revision
	currentRevision *IndexRevision
}

// NewLocalReplicator creates a new LocalReplicator.
func NewLocalReplicator(sourcePath, targetPath string) (*LocalReplicator, error) {
	if sourcePath == "" {
		return nil, fmt.Errorf("source path cannot be empty")
	}

	if targetPath == "" {
		return nil, fmt.Errorf("target path cannot be empty")
	}

	lr := &LocalReplicator{
		sourcePath: sourcePath,
		targetPath: targetPath,
		base:       NewReplicatorWithStats("local-replicator", nil),
	}

	// Set the check function
	lr.base.SetCheckFunc(func(ctx context.Context) error {
		return lr.performReplication(ctx)
	})

	lr.isOpen.Store(true)

	return lr, nil
}

// performReplication performs the actual replication.
func (lr *LocalReplicator) performReplication(ctx context.Context) error {
	lr.mu.RLock()
	sourcePath := lr.sourcePath
	targetPath := lr.targetPath
	lr.mu.RUnlock()

	// Ensure target directory exists
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	// Get source files
	files, err := lr.getSourceFiles(sourcePath)
	if err != nil {
		return fmt.Errorf("getting source files: %w", err)
	}

	// Copy files
	var totalBytes int64
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		sourceFile := filepath.Join(sourcePath, file)
		targetFile := filepath.Join(targetPath, file)

		bytes, err := lr.copyFile(sourceFile, targetFile)
		if err != nil {
			return fmt.Errorf("copying file %s: %w", file, err)
		}

		totalBytes += bytes
	}

	// Record statistics
	lr.base.RecordBytesTransferred(totalBytes)

	// Update current revision
	lr.mu.Lock()
	lr.currentRevision = &IndexRevision{
		Generation: lr.currentRevision.Generation + 1,
		Version:    lr.currentRevision.Version + 1,
		Files:      files,
	}
	lr.mu.Unlock()

	return nil
}

// getSourceFiles returns the list of files in the source directory.
func (lr *LocalReplicator) getSourceFiles(sourcePath string) ([]string, error) {
	var files []string

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Get relative path
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// copyFile copies a file from source to target.
func (lr *LocalReplicator) copyFile(sourcePath, targetPath string) (int64, error) {
	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, err
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

	// Create target file
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer targetFile.Close()

	// Copy content
	bytes, err := io.Copy(targetFile, sourceFile)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

// CheckNow checks if replication is needed and performs it.
func (lr *LocalReplicator) CheckNow(ctx context.Context) error {
	lr.mu.RLock()
	if !lr.isOpen.Load() {
		lr.mu.RUnlock()
		return fmt.Errorf("local replicator is closed")
	}
	lr.mu.RUnlock()

	return lr.base.CheckNow(ctx)
}

// Start starts the replicator.
func (lr *LocalReplicator) Start() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if !lr.isOpen.Load() {
		return fmt.Errorf("local replicator is closed")
	}

	return lr.base.Start()
}

// Stop stops the replicator.
func (lr *LocalReplicator) Stop() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	return lr.base.Stop()
}

// IsRunning returns true if the replicator is running.
func (lr *LocalReplicator) IsRunning() bool {
	return lr.base.IsRunning()
}

// GetLastException returns the last exception that occurred.
func (lr *LocalReplicator) GetLastException() error {
	return lr.base.GetLastException()
}

// GetSourcePath returns the source path.
func (lr *LocalReplicator) GetSourcePath() string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	return lr.sourcePath
}

// GetTargetPath returns the target path.
func (lr *LocalReplicator) GetTargetPath() string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	return lr.targetPath
}

// SetSourcePath sets the source path.
func (lr *LocalReplicator) SetSourcePath(path string) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if !lr.isOpen.Load() {
		return fmt.Errorf("local replicator is closed")
	}

	if path == "" {
		return fmt.Errorf("source path cannot be empty")
	}

	lr.sourcePath = path
	return nil
}

// SetTargetPath sets the target path.
func (lr *LocalReplicator) SetTargetPath(path string) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if !lr.isOpen.Load() {
		return fmt.Errorf("local replicator is closed")
	}

	if path == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	lr.targetPath = path
	return nil
}

// GetStats returns replication statistics.
func (lr *LocalReplicator) GetStats() ReplicatorStats {
	return lr.base.GetStats()
}

// ResetStats resets replication statistics.
func (lr *LocalReplicator) ResetStats() {
	lr.base.ResetStats()
}

// GetCurrentRevision returns the current replicated revision.
func (lr *LocalReplicator) GetCurrentRevision() *IndexRevision {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	if lr.currentRevision == nil {
		return nil
	}

	return lr.currentRevision.Clone()
}

// Close closes the LocalReplicator.
func (lr *LocalReplicator) Close() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if !lr.isOpen.Load() {
		return nil
	}

	lr.isOpen.Store(false)
	lr.base.Stop()

	return nil
}

// IsOpen returns true if the replicator is open.
func (lr *LocalReplicator) IsOpen() bool {
	return lr.isOpen.Load()
}

// String returns a string representation of the LocalReplicator.
func (lr *LocalReplicator) String() string {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	return fmt.Sprintf("LocalReplicator{open=%v, source=%s, target=%s}",
		lr.isOpen.Load(), lr.sourcePath, lr.targetPath)
}
