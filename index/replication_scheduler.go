package index

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ScheduledTask represents a task scheduled for future execution.
type ScheduledTask struct {
	// Task is the replication task
	Task *ReplicationTask

	// ExecuteAt is when the task should be executed
	ExecuteAt time.Time

	// Interval is the repeat interval (0 for one-time tasks)
	Interval time.Duration

	// ID is the scheduled task ID
	ID string
}

// scheduledTaskHeap implements a priority queue for scheduled tasks.
type scheduledTaskHeap []*ScheduledTask

func (h scheduledTaskHeap) Len() int { return len(h) }
func (h scheduledTaskHeap) Less(i, j int) bool {
	return h[i].ExecuteAt.Before(h[j].ExecuteAt)
}
func (h scheduledTaskHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *scheduledTaskHeap) Push(x interface{}) {
	*h = append(*h, x.(*ScheduledTask))
}

func (h *scheduledTaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// ReplicationScheduler manages scheduling of replication tasks.
type ReplicationScheduler struct {
	mu sync.RWMutex

	// tasks is the priority queue of scheduled tasks
	tasks scheduledTaskHeap

	// isRunning indicates if the scheduler is running
	isRunning atomic.Bool

	// stopChan signals the scheduler to stop
	stopChan chan struct{}

	// wg waits for goroutines
	wg sync.WaitGroup

	// nextTaskID is the next task ID
	nextTaskID int64

	// executor is called to execute tasks
	executor func(ctx context.Context, task *ReplicationTask) error
}

// NewReplicationScheduler creates a new ReplicationScheduler.
func NewReplicationScheduler(executor func(ctx context.Context, task *ReplicationTask) error) *ReplicationScheduler {
	rs := &ReplicationScheduler{
		tasks:    make(scheduledTaskHeap, 0),
		stopChan: make(chan struct{}),
		executor: executor,
	}
	heap.Init(&rs.tasks)
	return rs
}

// Start starts the replication scheduler.
func (rs *ReplicationScheduler) Start() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.isRunning.Load() {
		return fmt.Errorf("scheduler is already running")
	}

	rs.isRunning.Store(true)

	// Start scheduler goroutine
	rs.wg.Add(1)
	go rs.schedulerLoop()

	return nil
}

// Stop stops the replication scheduler.
func (rs *ReplicationScheduler) Stop() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.isRunning.Load() {
		return nil
	}

	rs.isRunning.Store(false)
	close(rs.stopChan)
	rs.wg.Wait()

	return nil
}

// IsRunning returns true if the scheduler is running.
func (rs *ReplicationScheduler) IsRunning() bool {
	return rs.isRunning.Load()
}

// Schedule schedules a task for execution.
func (rs *ReplicationScheduler) Schedule(task *ReplicationTask, delay time.Duration) (string, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.isRunning.Load() {
		return "", fmt.Errorf("scheduler is not running")
	}

	id := rs.generateTaskID()
	scheduledTask := &ScheduledTask{
		Task:      task,
		ExecuteAt: time.Now().Add(delay),
		Interval:  0,
		ID:        id,
	}

	heap.Push(&rs.tasks, scheduledTask)

	return id, nil
}

// ScheduleRepeating schedules a task for repeated execution.
func (rs *ReplicationScheduler) ScheduleRepeating(task *ReplicationTask, interval time.Duration) (string, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.isRunning.Load() {
		return "", fmt.Errorf("scheduler is not running")
	}

	if interval <= 0 {
		return "", fmt.Errorf("interval must be positive")
	}

	id := rs.generateTaskID()
	scheduledTask := &ScheduledTask{
		Task:      task,
		ExecuteAt: time.Now().Add(interval),
		Interval:  interval,
		ID:        id,
	}

	heap.Push(&rs.tasks, scheduledTask)

	return id, nil
}

// Cancel cancels a scheduled task.
func (rs *ReplicationScheduler) Cancel(taskID string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for i, task := range rs.tasks {
		if task.ID == taskID {
			heap.Remove(&rs.tasks, i)
			return true
		}
	}

	return false
}

// GetPendingCount returns the number of pending tasks.
func (rs *ReplicationScheduler) GetPendingCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return len(rs.tasks)
}

// schedulerLoop is the main scheduler loop.
func (rs *ReplicationScheduler) schedulerLoop() {
	defer rs.wg.Done()

	for {
		select {
		case <-rs.stopChan:
			return
		default:
		}

		rs.mu.Lock()
		if len(rs.tasks) == 0 {
			rs.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Peek at the next task
		nextTask := rs.tasks[0]
		now := time.Now()

		if nextTask.ExecuteAt.After(now) {
			// Wait until the task is due
			waitTime := nextTask.ExecuteAt.Sub(now)
			rs.mu.Unlock()

			select {
			case <-rs.stopChan:
				return
			case <-time.After(waitTime):
				continue
			}
		}

		// Pop and execute the task
		heap.Pop(&rs.tasks)
		rs.mu.Unlock()

		// Execute the task
		go rs.executeTask(nextTask)
	}
}

// executeTask executes a scheduled task.
func (rs *ReplicationScheduler) executeTask(scheduledTask *ScheduledTask) {
	ctx := context.Background()

	// Start the task
	scheduledTask.Task.Start()

	// Execute
	err := rs.executor(ctx, scheduledTask.Task)
	if err != nil {
		scheduledTask.Task.Fail(err)
	} else {
		scheduledTask.Task.Complete()
	}

	// If repeating, reschedule
	if scheduledTask.Interval > 0 {
		rs.mu.Lock()
		scheduledTask.ExecuteAt = time.Now().Add(scheduledTask.Interval)
		heap.Push(&rs.tasks, scheduledTask)
		rs.mu.Unlock()
	}
}

// generateTaskID generates a unique task ID.
func (rs *ReplicationScheduler) generateTaskID() string {
	id := atomic.AddInt64(&rs.nextTaskID, 1)
	return fmt.Sprintf("scheduled-task-%d", id)
}

// String returns a string representation of the ReplicationScheduler.
func (rs *ReplicationScheduler) String() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return fmt.Sprintf("ReplicationScheduler{running=%v, pending=%d}",
		rs.isRunning.Load(), len(rs.tasks))
}
