package index

import (
	"container/heap"
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewReplicationScheduler(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)
	if rs == nil {
		t.Fatal("expected scheduler to not be nil")
	}

	if rs.IsRunning() {
		t.Error("expected scheduler to not be running initially")
	}

	if rs.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending tasks, got %d", rs.GetPendingCount())
	}
}

func TestReplicationScheduler_StartStop(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)

	// Start
	err := rs.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !rs.IsRunning() {
		t.Error("expected scheduler to be running")
	}

	// Start again should fail
	err = rs.Start()
	if err == nil {
		t.Error("expected error when starting already running scheduler")
	}

	// Stop
	err = rs.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if rs.IsRunning() {
		t.Error("expected scheduler to be stopped")
	}

	// Stop again should not error
	err = rs.Stop()
	if err != nil {
		t.Errorf("second stop failed: %v", err)
	}
}

func TestReplicationScheduler_Schedule(t *testing.T) {
	executed := false
	executor := func(ctx context.Context, task *ReplicationTask) error {
		executed = true
		return nil
	}

	rs := NewReplicationScheduler(executor)
	rs.Start()
	defer rs.Stop()

	task := NewReplicationTask("task-1", nil, nil)
	taskID, err := rs.Schedule(task, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to schedule: %v", err)
	}

	if taskID == "" {
		t.Error("expected task ID to be set")
	}

	if rs.GetPendingCount() != 1 {
		t.Errorf("expected 1 pending task, got %d", rs.GetPendingCount())
	}

	// Wait for execution
	time.Sleep(150 * time.Millisecond)

	if !executed {
		t.Error("expected task to be executed")
	}
}

func TestReplicationScheduler_Schedule_NotRunning(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)
	// Don't start

	task := NewReplicationTask("task-1", nil, nil)
	_, err := rs.Schedule(task, 1*time.Second)
	if err == nil {
		t.Error("expected error when scheduling on stopped scheduler")
	}
}

func TestReplicationScheduler_ScheduleRepeating(t *testing.T) {
	executionCount := 0
	executor := func(ctx context.Context, task *ReplicationTask) error {
		executionCount++
		return nil
	}

	rs := NewReplicationScheduler(executor)
	rs.Start()
	defer rs.Stop()

	task := NewReplicationTask("task-1", nil, nil)
	taskID, err := rs.ScheduleRepeating(task, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to schedule: %v", err)
	}

	if taskID == "" {
		t.Error("expected task ID to be set")
	}

	// Wait for multiple executions
	time.Sleep(200 * time.Millisecond)

	if executionCount < 2 {
		t.Errorf("expected at least 2 executions, got %d", executionCount)
	}
}

func TestReplicationScheduler_ScheduleRepeating_InvalidInterval(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)
	rs.Start()
	defer rs.Stop()

	task := NewReplicationTask("task-1", nil, nil)
	_, err := rs.ScheduleRepeating(task, 0)
	if err == nil {
		t.Error("expected error for zero interval")
	}

	_, err = rs.ScheduleRepeating(task, -1*time.Second)
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestReplicationScheduler_Cancel(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)
	rs.Start()
	defer rs.Stop()

	task := NewReplicationTask("task-1", nil, nil)
	taskID, _ := rs.Schedule(task, 1*time.Hour)

	if rs.GetPendingCount() != 1 {
		t.Errorf("expected 1 pending task, got %d", rs.GetPendingCount())
	}

	cancelled := rs.Cancel(taskID)
	if !cancelled {
		t.Error("expected cancel to succeed")
	}

	if rs.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending tasks, got %d", rs.GetPendingCount())
	}

	// Cancel non-existent task
	cancelled = rs.Cancel("non-existent")
	if cancelled {
		t.Error("expected cancel to fail for non-existent task")
	}
}

func TestReplicationScheduler_ExecutorError(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return errors.New("execution failed")
	}

	rs := NewReplicationScheduler(executor)
	rs.Start()
	defer rs.Stop()

	task := NewReplicationTask("task-1", nil, nil)
	rs.Schedule(task, 50*time.Millisecond)

	// Wait for execution
	time.Sleep(150 * time.Millisecond)

	if task.GetStatus() != ReplicationTaskStatusFailed {
		t.Errorf("expected status FAILED, got %s", task.GetStatus().String())
	}
}

func TestReplicationScheduler_String(t *testing.T) {
	executor := func(ctx context.Context, task *ReplicationTask) error {
		return nil
	}

	rs := NewReplicationScheduler(executor)

	str := rs.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestScheduledTaskHeap(t *testing.T) {
	h := make(scheduledTaskHeap, 0)
	heap.Init(&h)

	// Push tasks with different execution times
	now := time.Now()
	task1 := &ScheduledTask{ExecuteAt: now.Add(3 * time.Second), ID: "3"}
	task2 := &ScheduledTask{ExecuteAt: now.Add(1 * time.Second), ID: "1"}
	task3 := &ScheduledTask{ExecuteAt: now.Add(2 * time.Second), ID: "2"}

	heap.Push(&h, task1)
	heap.Push(&h, task2)
	heap.Push(&h, task3)

	// Pop should return tasks in order
	first := heap.Pop(&h).(*ScheduledTask)
	if first.ID != "1" {
		t.Errorf("expected task 1 first, got %s", first.ID)
	}

	second := heap.Pop(&h).(*ScheduledTask)
	if second.ID != "2" {
		t.Errorf("expected task 2 second, got %s", second.ID)
	}

	third := heap.Pop(&h).(*ScheduledTask)
	if third.ID != "3" {
		t.Errorf("expected task 3 third, got %s", third.ID)
	}
}
