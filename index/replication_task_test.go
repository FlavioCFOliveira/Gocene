package index

import (
	"errors"
	"testing"
	"time"
)

func TestNewReplicationTask(t *testing.T) {
	sourceRevision := &IndexRevision{
		Generation: 1,
		Version:    1,
		Files:      []string{"file1.txt"},
	}
	files := []string{"file1.txt", "file2.txt"}

	task := NewReplicationTask("task-1", sourceRevision, files)

	if task == nil {
		t.Fatal("expected task to not be nil")
	}

	if task.ID != "task-1" {
		t.Errorf("expected ID 'task-1', got %s", task.ID)
	}

	if task.GetStatus() != ReplicationTaskStatusPending {
		t.Errorf("expected status PENDING, got %s", task.GetStatus().String())
	}

	if len(task.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(task.Files))
	}

	if task.GetProgress() != 0 {
		t.Errorf("expected progress 0, got %d", task.GetProgress())
	}
}

func TestReplicationTaskStatus_String(t *testing.T) {
	tests := []struct {
		status   ReplicationTaskStatus
		expected string
	}{
		{ReplicationTaskStatusPending, "PENDING"},
		{ReplicationTaskStatusRunning, "RUNNING"},
		{ReplicationTaskStatusCompleted, "COMPLETED"},
		{ReplicationTaskStatusFailed, "FAILED"},
		{ReplicationTaskStatusCancelled, "CANCELLED"},
		{ReplicationTaskStatus(99), "UNKNOWN"},
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("expected %s, got %s", test.expected, test.status.String())
		}
	}
}

func TestReplicationTask_Start(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	err := task.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if task.GetStatus() != ReplicationTaskStatusRunning {
		t.Errorf("expected status RUNNING, got %s", task.GetStatus().String())
	}

	if task.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestReplicationTask_Start_NotPending(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	task.Start()

	err := task.Start()
	if err == nil {
		t.Error("expected error when starting non-pending task")
	}
}

func TestReplicationTask_Complete(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	task.Start()

	err := task.Complete()
	if err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	if task.GetStatus() != ReplicationTaskStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", task.GetStatus().String())
	}

	if task.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}

	if task.GetProgress() != 100 {
		t.Errorf("expected progress 100, got %d", task.GetProgress())
	}
}

func TestReplicationTask_Complete_NotRunning(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	err := task.Complete()
	if err == nil {
		t.Error("expected error when completing non-running task")
	}
}

func TestReplicationTask_Fail(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	task.Start()

	expectedErr := errors.New("replication failed")
	task.Fail(expectedErr)

	if task.GetStatus() != ReplicationTaskStatusFailed {
		t.Errorf("expected status FAILED, got %s", task.GetStatus().String())
	}

	if task.Error != expectedErr.Error() {
		t.Errorf("expected error '%s', got '%s'", expectedErr.Error(), task.Error)
	}
}

func TestReplicationTask_Cancel(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	task.Start()

	task.Cancel()

	if task.GetStatus() != ReplicationTaskStatusCancelled {
		t.Errorf("expected status CANCELLED, got %s", task.GetStatus().String())
	}

	if !task.IsCancelled() {
		t.Error("expected IsCancelled to be true")
	}
}

func TestReplicationTask_Cancel_AlreadyCompleted(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	task.Start()
	task.Complete()

	task.Cancel()

	// Should still be completed
	if task.GetStatus() != ReplicationTaskStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", task.GetStatus().String())
	}
}

func TestReplicationTask_UpdateProgress(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	task.UpdateProgress(50)
	if task.GetProgress() != 50 {
		t.Errorf("expected progress 50, got %d", task.GetProgress())
	}

	// Test bounds
	task.UpdateProgress(-10)
	if task.GetProgress() != 0 {
		t.Errorf("expected progress 0, got %d", task.GetProgress())
	}

	task.UpdateProgress(150)
	if task.GetProgress() != 100 {
		t.Errorf("expected progress 100, got %d", task.GetProgress())
	}
}

func TestReplicationTask_AddBytesTransferred(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	task.AddBytesTransferred(1000)
	task.AddBytesTransferred(500)

	if task.GetBytesTransferred() != 1500 {
		t.Errorf("expected 1500 bytes, got %d", task.GetBytesTransferred())
	}
}

func TestReplicationTask_GetDuration(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	// Pending - should be 0
	if task.GetDuration() != 0 {
		t.Error("expected 0 duration for pending task")
	}

	// Running
	task.Start()
	time.Sleep(10 * time.Millisecond)

	duration := task.GetDuration()
	if duration <= 0 {
		t.Error("expected positive duration for running task")
	}

	// Completed
	task.Complete()
	duration = task.GetDuration()
	if duration <= 0 {
		t.Error("expected positive duration for completed task")
	}
}

func TestReplicationTask_GetTotalDuration(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)
	time.Sleep(10 * time.Millisecond)

	duration := task.GetTotalDuration()
	if duration <= 0 {
		t.Error("expected positive total duration")
	}
}

func TestReplicationTask_SetTargetRevision(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	targetRevision := &IndexRevision{
		Generation: 2,
		Version:    2,
		Files:      []string{"file1.txt", "file2.txt"},
	}

	task.SetTargetRevision(targetRevision)

	retrieved := task.GetTargetRevision()
	if retrieved == nil {
		t.Fatal("expected target revision to be set")
	}

	if retrieved.Generation != 2 {
		t.Errorf("expected generation 2, got %d", retrieved.Generation)
	}
}

func TestReplicationTask_String(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	str := task.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestReplicationTask_ConcurrentOperations(t *testing.T) {
	task := NewReplicationTask("task-1", nil, nil)

	done := make(chan bool, 4)

	// Update progress goroutine
	go func() {
		for i := 0; i < 100; i++ {
			task.UpdateProgress(i)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Add bytes goroutine
	go func() {
		for i := 0; i < 100; i++ {
			task.AddBytesTransferred(100)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Get status goroutine
	go func() {
		for i := 0; i < 100; i++ {
			task.GetStatus()
			task.GetProgress()
			task.GetBytesTransferred()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Get duration goroutine
	go func() {
		for i := 0; i < 100; i++ {
			task.GetDuration()
			task.GetTotalDuration()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
