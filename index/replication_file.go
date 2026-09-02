package index

import (
	"fmt"
	"hash/crc32"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicationFileStatus represents the status of a file replication.
type ReplicationFileStatus int

const (
	// ReplicationFileStatusPending indicates the file is pending replication
	ReplicationFileStatusPending ReplicationFileStatus = iota
	// ReplicationFileStatusTransferring indicates the file is being transferred
	ReplicationFileStatusTransferring
	// ReplicationFileStatusCompleted indicates the file was successfully replicated
	ReplicationFileStatusCompleted
	// ReplicationFileStatusFailed indicates the file replication failed
	ReplicationFileStatusFailed
	// ReplicationFileStatusSkipped indicates the file was skipped
	ReplicationFileStatusSkipped
)

// String returns the string representation of the status.
func (s ReplicationFileStatus) String() string {
	switch s {
	case ReplicationFileStatusPending:
		return "PENDING"
	case ReplicationFileStatusTransferring:
		return "TRANSFERRING"
	case ReplicationFileStatusCompleted:
		return "COMPLETED"
	case ReplicationFileStatusFailed:
		return "FAILED"
	case ReplicationFileStatusSkipped:
		return "SKIPPED"
	default:
		return "UNKNOWN"
	}
}

// ReplicationFile represents a file being replicated.
type ReplicationFile struct {
	mu sync.RWMutex

	// Name is the file name
	Name string

	// SourcePath is the source file path
	SourcePath string

	// TargetPath is the target file path
	TargetPath string

	// Size is the file size in bytes
	Size int64

	// Status is the current replication status
	status ReplicationFileStatus

	// TransferredBytes is the number of bytes transferred
	TransferredBytes int64

	// Checksum is the expected file checksum
	Checksum uint32

	// ActualChecksum is the calculated checksum after transfer
	ActualChecksum uint32

	// CreatedAt is when the file replication was created
	CreatedAt time.Time

	// StartedAt is when the file transfer started
	StartedAt time.Time

	// CompletedAt is when the file transfer completed
	CompletedAt time.Time

	// Error is the error message if replication failed
	Error string

	// RetryCount is the number of retry attempts
	RetryCount int

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// isCancelled indicates if the transfer was cancelled
	isCancelled atomic.Bool
}

// NewReplicationFile creates a new ReplicationFile.
func NewReplicationFile(name, sourcePath, targetPath string, size int64) *ReplicationFile {
	return &ReplicationFile{
		Name:       name,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Size:       size,
		status:     ReplicationFileStatusPending,
		CreatedAt:  time.Now(),
		MaxRetries: 3,
	}
}

// GetStatus returns the current file status.
func (f *ReplicationFile) GetStatus() ReplicationFileStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.status
}

// SetStatus sets the file status.
func (f *ReplicationFile) SetStatus(status ReplicationFileStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = status

	switch status {
	case ReplicationFileStatusTransferring:
		f.StartedAt = time.Now()
	case ReplicationFileStatusCompleted, ReplicationFileStatusFailed, ReplicationFileStatusSkipped:
		f.CompletedAt = time.Now()
	}
}

// Start marks the file transfer as started.
func (f *ReplicationFile) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.status != ReplicationFileStatusPending {
		return fmt.Errorf("file is not pending")
	}

	f.status = ReplicationFileStatusTransferring
	f.StartedAt = time.Now()

	return nil
}

// Complete marks the file transfer as completed.
func (f *ReplicationFile) Complete() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.status != ReplicationFileStatusTransferring {
		return fmt.Errorf("file is not transferring")
	}

	f.status = ReplicationFileStatusCompleted
	f.CompletedAt = time.Now()
	f.TransferredBytes = f.Size

	return nil
}

// Fail marks the file transfer as failed.
func (f *ReplicationFile) Fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = ReplicationFileStatusFailed
	f.CompletedAt = time.Now()
	if err != nil {
		f.Error = err.Error()
	}
}

// Skip marks the file as skipped.
func (f *ReplicationFile) Skip() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = ReplicationFileStatusSkipped
	f.CompletedAt = time.Now()
}

// Cancel cancels the file transfer.
func (f *ReplicationFile) Cancel() {
	f.isCancelled.Store(true)
}

// IsCancelled returns true if the transfer was cancelled.
func (f *ReplicationFile) IsCancelled() bool {
	return f.isCancelled.Load()
}

// UpdateProgress updates the transfer progress.
func (f *ReplicationFile) UpdateProgress(transferredBytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.TransferredBytes = transferredBytes
}

// AddTransferredBytes adds to the transferred bytes count.
func (f *ReplicationFile) AddTransferredBytes(bytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.TransferredBytes += bytes
	if f.TransferredBytes > f.Size {
		f.TransferredBytes = f.Size
	}
}

// GetProgress returns the transfer progress percentage (0-100).
func (f *ReplicationFile) GetProgress() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Size == 0 {
		return 100
	}

	progress := int((f.TransferredBytes * 100) / f.Size)
	if progress > 100 {
		progress = 100
	}

	return progress
}

// GetTransferRate returns the transfer rate in bytes per second.
func (f *ReplicationFile) GetTransferRate() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.status != ReplicationFileStatusTransferring || f.TransferredBytes == 0 {
		return 0
	}

	duration := time.Since(f.StartedAt).Seconds()
	if duration == 0 {
		return 0
	}

	return float64(f.TransferredBytes) / duration
}

// GetDuration returns the transfer duration.
func (f *ReplicationFile) GetDuration() time.Duration {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.status == ReplicationFileStatusPending {
		return 0
	}

	if f.status == ReplicationFileStatusTransferring {
		return time.Since(f.StartedAt)
	}

	return f.CompletedAt.Sub(f.StartedAt)
}

// GetTotalDuration returns the total duration from creation to completion.
func (f *ReplicationFile) GetTotalDuration() time.Duration {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.status == ReplicationFileStatusPending {
		return time.Since(f.CreatedAt)
	}

	if f.status == ReplicationFileStatusTransferring {
		return time.Since(f.CreatedAt)
	}

	return f.CompletedAt.Sub(f.CreatedAt)
}

// IncrementRetryCount increments the retry count.
func (f *ReplicationFile) IncrementRetryCount() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.RetryCount++
}

// CanRetry returns true if the file can be retried.
func (f *ReplicationFile) CanRetry() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.RetryCount < f.MaxRetries
}

// ResetForRetry resets the file for a retry attempt.
func (f *ReplicationFile) ResetForRetry() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = ReplicationFileStatusPending
	f.TransferredBytes = 0
	f.Error = ""
	f.StartedAt = time.Time{}
	f.CompletedAt = time.Time{}
	f.isCancelled.Store(false)
}

// SetChecksum sets the expected checksum.
func (f *ReplicationFile) SetChecksum(checksum uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Checksum = checksum
}

// SetActualChecksum sets the actual checksum.
func (f *ReplicationFile) SetActualChecksum(checksum uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ActualChecksum = checksum
}

// VerifyChecksum verifies the file checksum.
func (f *ReplicationFile) VerifyChecksum() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Checksum == 0 {
		return true // No checksum to verify
	}

	return f.Checksum == f.ActualChecksum
}

// CalculateCRC32 calculates the CRC32 checksum of data.
func CalculateCRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// String returns a string representation of the ReplicationFile.
func (f *ReplicationFile) String() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return fmt.Sprintf("ReplicationFile{name=%s, size=%d, status=%s, progress=%d%%}",
		f.Name, f.Size, f.status.String(), f.GetProgress())
}
