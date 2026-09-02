package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Replicator is the main interface for index replication operations.
// It coordinates between source and target for index replication.
type Replicator interface {
	// CheckNow checks if replication is needed and performs it if necessary
	CheckNow(ctx context.Context) error

	// Start starts the replicator
	Start() error

	// Stop stops the replicator
	Stop() error

	// IsRunning returns true if the replicator is running
	IsRunning() bool

	// GetLastException returns the last exception that occurred during replication
	GetLastException() error
}

// ReplicatorBase provides common functionality for replicator implementations.
// It implements the base lifecycle and state management.
type ReplicatorBase struct {
	mu sync.RWMutex

	// name is the replicator name for identification
	name string

	// isRunning indicates if the replicator is running
	isRunning atomic.Bool

	// lastException stores the last error that occurred
	lastException error

	// checkFunc is called to perform replication
	checkFunc func(ctx context.Context) error

	// ctx is the context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewReplicatorBase creates a new ReplicatorBase.
func NewReplicatorBase(name string, checkFunc func(ctx context.Context) error) *ReplicatorBase {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReplicatorBase{
		name:      name,
		checkFunc: checkFunc,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// CheckNow performs the replication check.
func (r *ReplicatorBase) CheckNow(ctx context.Context) error {
	r.mu.RLock()
	if !r.isRunning.Load() {
		r.mu.RUnlock()
		return fmt.Errorf("replicator %s is not running", r.name)
	}
	checkFunc := r.checkFunc
	r.mu.RUnlock()

	if checkFunc == nil {
		return fmt.Errorf("replicator %s has no check function", r.name)
	}

	err := checkFunc(ctx)
	if err != nil {
		r.setLastException(err)
		return fmt.Errorf("replication check failed: %w", err)
	}

	return nil
}

// Start starts the replicator.
func (r *ReplicatorBase) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunning.Load() {
		return fmt.Errorf("replicator %s is already running", r.name)
	}

	// Reset context if it was cancelled
	if r.ctx.Err() != nil {
		r.ctx, r.cancel = context.WithCancel(context.Background())
	}

	r.isRunning.Store(true)
	r.lastException = nil

	return nil
}

// Stop stops the replicator.
func (r *ReplicatorBase) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRunning.Load() {
		return nil
	}

	r.isRunning.Store(false)
	r.cancel()

	return nil
}

// IsRunning returns true if the replicator is running.
func (r *ReplicatorBase) IsRunning() bool {
	return r.isRunning.Load()
}

// GetLastException returns the last exception that occurred.
func (r *ReplicatorBase) GetLastException() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.lastException
}

// setLastException sets the last exception.
func (r *ReplicatorBase) setLastException(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastException = err
}

// GetName returns the replicator name.
func (r *ReplicatorBase) GetName() string {
	return r.name
}

// SetCheckFunc sets the check function.
func (r *ReplicatorBase) SetCheckFunc(checkFunc func(ctx context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.checkFunc = checkFunc
}

// GetContext returns the replicator context.
func (r *ReplicatorBase) GetContext() context.Context {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ctx
}

// ReplicatorStats holds statistics for the replicator.
type ReplicatorStats struct {
	// TotalChecks is the total number of replication checks performed
	TotalChecks int64

	// SuccessfulChecks is the number of successful checks
	SuccessfulChecks int64

	// FailedChecks is the number of failed checks
	FailedChecks int64

	// LastCheckTime is when the last check was performed
	LastCheckTime int64

	// TotalBytesTransferred is the total bytes transferred
	TotalBytesTransferred int64
}

// ReplicatorWithStats extends ReplicatorBase with statistics tracking.
type ReplicatorWithStats struct {
	*ReplicatorBase

	// stats holds replication statistics
	stats ReplicatorStats
}

// NewReplicatorWithStats creates a new ReplicatorWithStats.
func NewReplicatorWithStats(name string, checkFunc func(ctx context.Context) error) *ReplicatorWithStats {
	return &ReplicatorWithStats{
		ReplicatorBase: NewReplicatorBase(name, checkFunc),
	}
}

// CheckNow performs the replication check with statistics tracking.
func (r *ReplicatorWithStats) CheckNow(ctx context.Context) error {
	err := r.ReplicatorBase.CheckNow(ctx)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalChecks++
	r.stats.LastCheckTime = timeNow()

	if err != nil {
		r.stats.FailedChecks++
	} else {
		r.stats.SuccessfulChecks++
	}

	return err
}

// GetStats returns the replication statistics.
func (r *ReplicatorWithStats) GetStats() ReplicatorStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.stats
}

// RecordBytesTransferred records bytes transferred during replication.
func (r *ReplicatorWithStats) RecordBytesTransferred(bytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalBytesTransferred += bytes
}

// ResetStats resets the replication statistics.
func (r *ReplicatorWithStats) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats = ReplicatorStats{}
}

// timeNow returns the current time in milliseconds since epoch.
func timeNow() int64 {
	return timeNowFunc().UnixNano() / int64(time.Millisecond)
}

// timeNowFunc is a variable for testing purposes.
var timeNowFunc = func() time.Time {
	return time.Now()
}
