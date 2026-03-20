package index

import (
	"errors"
	"testing"
	"time"
)

// mockReplicationListener is a mock implementation of ReplicationListener.
type mockReplicationListener struct {
	taskStarted       bool
	taskCompleted     bool
	taskFailed        bool
	fileTransferStarted bool
	fileTransferCompleted bool
	progressUpdates     int
}

func (m *mockReplicationListener) OnTaskStarted(task *ReplicationTask) {
	m.taskStarted = true
}

func (m *mockReplicationListener) OnTaskCompleted(task *ReplicationTask) {
	m.taskCompleted = true
}

func (m *mockReplicationListener) OnTaskFailed(task *ReplicationTask, err error) {
	m.taskFailed = true
}

func (m *mockReplicationListener) OnFileTransferStarted(file *ReplicationFile) {
	m.fileTransferStarted = true
}

func (m *mockReplicationListener) OnFileTransferProgress(file *ReplicationFile, bytesTransferred int64) {
	m.progressUpdates++
}

func (m *mockReplicationListener) OnFileTransferCompleted(file *ReplicationFile) {
	m.fileTransferCompleted = true
}

func TestNewReplicationMonitor(t *testing.T) {
	monitor := NewReplicationMonitor()

	if monitor == nil {
		t.Fatal("expected monitor to not be nil")
	}

	if monitor.IsRunning() {
		t.Error("expected monitor to not be running initially")
	}

	if monitor.GetTaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", monitor.GetTaskCount())
	}

	if monitor.GetFileCount() != 0 {
		t.Errorf("expected 0 files, got %d", monitor.GetFileCount())
	}
}

func TestReplicationMonitor_StartStop(t *testing.T) {
	monitor := NewReplicationMonitor()

	// Start
	err := monitor.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("expected monitor to be running")
	}

	// Start again should fail
	err = monitor.Start()
	if err == nil {
		t.Error("expected error when starting already running monitor")
	}

	// Stop
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if monitor.IsRunning() {
		t.Error("expected monitor to be stopped")
	}

	// Stop again should not error
	err = monitor.Stop()
	if err != nil {
		t.Errorf("second stop failed: %v", err)
	}
}

func TestReplicationMonitor_AddRemoveListener(t *testing.T) {
	monitor := NewReplicationMonitor()

	listener := &mockReplicationListener{}

	monitor.AddListener(listener)

	// Should be able to start/stop without errors
	monitor.Start()
	time.Sleep(50 * time.Millisecond)
	monitor.Stop()
}

func TestReplicationMonitor_MonitorTask(t *testing.T) {
	monitor := NewReplicationMonitor()

	task := NewReplicationTask("task-1", nil, nil)
	monitor.MonitorTask(task)

	if monitor.GetTaskCount() != 1 {
		t.Errorf("expected 1 task, got %d", monitor.GetTaskCount())
	}

	monitor.StopMonitoringTask("task-1")

	if monitor.GetTaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", monitor.GetTaskCount())
	}
}

func TestReplicationMonitor_MonitorFile(t *testing.T) {
	monitor := NewReplicationMonitor()

	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	monitor.MonitorFile(file)

	if monitor.GetFileCount() != 1 {
		t.Errorf("expected 1 file, got %d", monitor.GetFileCount())
	}

	monitor.StopMonitoringFile("test.txt")

	if monitor.GetFileCount() != 0 {
		t.Errorf("expected 0 files, got %d", monitor.GetFileCount())
	}
}

func TestReplicationMonitor_GetOverallProgress(t *testing.T) {
	monitor := NewReplicationMonitor()

	// No tasks - should be 0
	if monitor.GetOverallProgress() != 0 {
		t.Errorf("expected 0%% progress with no tasks, got %d%%", monitor.GetOverallProgress())
	}

	// Add tasks with different progress
	task1 := NewReplicationTask("task-1", nil, nil)
	task1.UpdateProgress(50)
	monitor.MonitorTask(task1)

	task2 := NewReplicationTask("task-2", nil, nil)
	task2.UpdateProgress(100)
	monitor.MonitorTask(task2)

	// (50 + 100) / 2 = 75
	if monitor.GetOverallProgress() != 75 {
		t.Errorf("expected 75%% progress, got %d%%", monitor.GetOverallProgress())
	}
}

func TestReplicationMonitor_GetTaskCounts(t *testing.T) {
	monitor := NewReplicationMonitor()

	// Add tasks with different statuses
	task1 := NewReplicationTask("task-1", nil, nil)
	task1.Start()
	monitor.MonitorTask(task1)

	task2 := NewReplicationTask("task-2", nil, nil)
	task2.Start()
	task2.Complete()
	monitor.MonitorTask(task2)

	task3 := NewReplicationTask("task-3", nil, nil)
	task3.Start()
	task3.Fail(errors.New("failed"))
	monitor.MonitorTask(task3)

	if monitor.GetActiveTaskCount() != 1 {
		t.Errorf("expected 1 active task, got %d", monitor.GetActiveTaskCount())
	}

	if monitor.GetCompletedTaskCount() != 1 {
		t.Errorf("expected 1 completed task, got %d", monitor.GetCompletedTaskCount())
	}

	if monitor.GetFailedTaskCount() != 1 {
		t.Errorf("expected 1 failed task, got %d", monitor.GetFailedTaskCount())
	}
}

func TestReplicationMonitor_GetSummary(t *testing.T) {
	monitor := NewReplicationMonitor()

	task := NewReplicationTask("task-1", nil, nil)
	task.Start()
	monitor.MonitorTask(task)

	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	monitor.MonitorFile(file)

	summary := monitor.GetSummary()

	if summary.TotalTasks != 1 {
		t.Errorf("expected 1 total task, got %d", summary.TotalTasks)
	}

	if summary.ActiveTasks != 1 {
		t.Errorf("expected 1 active task, got %d", summary.ActiveTasks)
	}

	if summary.TotalFiles != 1 {
		t.Errorf("expected 1 total file, got %d", summary.TotalFiles)
	}
}

func TestReplicationMonitor_String(t *testing.T) {
	monitor := NewReplicationMonitor()

	str := monitor.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestReplicationSummary(t *testing.T) {
	summary := ReplicationSummary{
		TotalTasks:       10,
		ActiveTasks:      5,
		CompletedTasks:   3,
		FailedTasks:      2,
		TotalFiles:       100,
		OverallProgress:  50,
	}

	if summary.TotalTasks != 10 {
		t.Errorf("expected 10 total tasks, got %d", summary.TotalTasks)
	}

	if summary.ActiveTasks != 5 {
		t.Errorf("expected 5 active tasks, got %d", summary.ActiveTasks)
	}

	if summary.CompletedTasks != 3 {
		t.Errorf("expected 3 completed tasks, got %d", summary.CompletedTasks)
	}

	if summary.FailedTasks != 2 {
		t.Errorf("expected 2 failed tasks, got %d", summary.FailedTasks)
	}

	if summary.TotalFiles != 100 {
		t.Errorf("expected 100 total files, got %d", summary.TotalFiles)
	}

	if summary.OverallProgress != 50 {
		t.Errorf("expected 50%% overall progress, got %d%%", summary.OverallProgress)
	}
}
