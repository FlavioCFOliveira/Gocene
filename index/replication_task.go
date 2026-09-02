package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicationTaskStatus represents the status of a replication task.
type ReplicationTaskStatus int

const (
	// ReplicationTaskStatusPending indicates the task is pending
	ReplicationTaskStatusPending ReplicationTaskStatus = iota
	// ReplicationTaskStatusRunning indicates the task is running
	ReplicationTaskStatusRunning
	// ReplicationTaskStatusCompleted indicates the task completed successfully
	ReplicationTaskStatusCompleted
	// ReplicationTaskStatusFailed indicates the task failed
	ReplicationTaskStatusFailed
	// ReplicationTaskStatusCancelled indicates the task was cancelled
	ReplicationTaskStatusCancelled
)

// String returns the string representation of the status.
func (s ReplicationTaskStatus) String() string {
	switch s {
	case ReplicationTaskStatusPending:
		return "PENDING"
	case ReplicationTaskStatusRunning:
		return "RUNNING"
	case ReplicationTaskStatusCompleted:
		return "COMPLETED"
	case ReplicationTaskStatusFailed:
		return "FAILED"
	case ReplicationTaskStatusCancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// ReplicationTask represents a single replication operation.
type ReplicationTask struct {
	mu sync.RWMutex

	// ID is the unique task identifier
	ID string

	// Status is the current task status
	status ReplicationTaskStatus

	// SourceRevision is the source index revision
	SourceRevision *IndexRevision

	// TargetRevision is the target index revision (after replication)
	TargetRevision *IndexRevision

	// Files is the list of files to replicate
	Files []string

	// CreatedAt is when the task was created
	CreatedAt time.Time

	// StartedAt is when the task started
	StartedAt time.Time

	// CompletedAt is when the task completed
	CompletedAt time.Time

	// Error is the error message if the task failed
	Error string

	// BytesTransferred is the number of bytes transferred
	BytesTransferred int64

	// Progress is the replication progress (0-100)
	Progress int

	// ctx is the task context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// isCancelled indicates if the task was cancelled
	isCancelled atomic.Bool
}

// NewReplicationTask creates a new ReplicationTask.
func NewReplicationTask(id string, sourceRevision *IndexRevision, files []string) *ReplicationTask {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReplicationTask{
		ID:             id,
		status:         ReplicationTaskStatusPending,
		SourceRevision: sourceRevision,
		Files:          files,
		CreatedAt:      time.Now(),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// GetStatus returns the current task status.
func (t *ReplicationTask) GetStatus() ReplicationTaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.status
}

// SetStatus sets the task status.
func (t *ReplicationTask) SetStatus(status ReplicationTaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status = status

	switch status {
	case ReplicationTaskStatusRunning:
		t.StartedAt = time.Now()
	case ReplicationTaskStatusCompleted, ReplicationTaskStatusFailed, ReplicationTaskStatusCancelled:
		t.CompletedAt = time.Now()
	}
}

// Start marks the task as started.
func (t *ReplicationTask) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status != ReplicationTaskStatusPending {
		return fmt.Errorf("task is not pending")
	}

	t.status = ReplicationTaskStatusRunning
	t.StartedAt = time.Now()

	return nil
}

// Complete marks the task as completed.
func (t *ReplicationTask) Complete() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status != ReplicationTaskStatusRunning {
		return fmt.Errorf("task is not running")
	}

	t.status = ReplicationTaskStatusCompleted
	t.CompletedAt = time.Now()
	t.Progress = 100

	return nil
}

// Fail marks the task as failed.
func (t *ReplicationTask) Fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status = ReplicationTaskStatusFailed
	t.CompletedAt = time.Now()
	if err != nil {
		t.Error = err.Error()
	}
}

// Cancel cancels the task.
func (t *ReplicationTask) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == ReplicationTaskStatusCompleted || t.status == ReplicationTaskStatusFailed {
		return
	}

	t.status = ReplicationTaskStatusCancelled
	t.CompletedAt = time.Now()
	t.isCancelled.Store(true)
	t.cancel()
}

// IsCancelled returns true if the task was cancelled.
func (t *ReplicationTask) IsCancelled() bool {
	return t.isCancelled.Load()
}

// GetContext returns the task context.
func (t *ReplicationTask) GetContext() context.Context {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.ctx
}

// UpdateProgress updates the replication progress.
func (t *ReplicationTask) UpdateProgress(progress int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	t.Progress = progress
}

// GetProgress returns the replication progress.
func (t *ReplicationTask) GetProgress() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Progress
}

// AddBytesTransferred adds to the bytes transferred count.
func (t *ReplicationTask) AddBytesTransferred(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.BytesTransferred += bytes
}

// GetBytesTransferred returns the bytes transferred.
func (t *ReplicationTask) GetBytesTransferred() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.BytesTransferred
}

// GetDuration returns the task duration.
func (t *ReplicationTask) GetDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.status == ReplicationTaskStatusPending {
		return 0
	}

	if t.status == ReplicationTaskStatusRunning {
		return time.Since(t.StartedAt)
	}

	return t.CompletedAt.Sub(t.StartedAt)
}

// GetTotalDuration returns the total duration from creation to completion.
func (t *ReplicationTask) GetTotalDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.status == ReplicationTaskStatusPending {
		return time.Since(t.CreatedAt)
	}

	if t.status == ReplicationTaskStatusRunning {
		return time.Since(t.CreatedAt)
	}

	return t.CompletedAt.Sub(t.CreatedAt)
}

// SetTargetRevision sets the target revision.
func (t *ReplicationTask) SetTargetRevision(revision *IndexRevision) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TargetRevision = revision
}

// GetTargetRevision returns the target revision.
func (t *ReplicationTask) GetTargetRevision() *IndexRevision {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.TargetRevision == nil {
		return nil
	}

	return t.TargetRevision.Clone()
}

// String returns a string representation of the ReplicationTask.
func (t *ReplicationTask) String() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return fmt.Sprintf("ReplicationTask{id=%s, status=%s, progress=%d%%}",
		t.ID, t.status.String(), t.Progress)
}
