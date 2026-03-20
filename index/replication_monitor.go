package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicationListener is called when replication events occur.
type ReplicationListener interface {
	// OnTaskStarted is called when a task starts
	OnTaskStarted(task *ReplicationTask)

	// OnTaskCompleted is called when a task completes
	OnTaskCompleted(task *ReplicationTask)

	// OnTaskFailed is called when a task fails
	OnTaskFailed(task *ReplicationTask, err error)

	// OnFileTransferStarted is called when a file transfer starts
	OnFileTransferStarted(file *ReplicationFile)

	// OnFileTransferProgress is called when file transfer progresses
	OnFileTransferProgress(file *ReplicationFile, bytesTransferred int64)

	// OnFileTransferCompleted is called when a file transfer completes
	OnFileTransferCompleted(file *ReplicationFile)
}

// ReplicationMonitor monitors replication progress and status.
type ReplicationMonitor struct {
	mu sync.RWMutex

	// tasks holds monitored tasks
	tasks map[string]*ReplicationTask

	// files holds monitored files
	files map[string]*ReplicationFile

	// listeners holds replication listeners
	listeners []ReplicationListener

	// isRunning indicates if the monitor is running
	isRunning atomic.Bool

	// stopChan signals the monitor to stop
	stopChan chan struct{}

	// wg waits for goroutines
	wg sync.WaitGroup

	// updateInterval is the interval for progress updates
	updateInterval time.Duration
}

// NewReplicationMonitor creates a new ReplicationMonitor.
func NewReplicationMonitor() *ReplicationMonitor {
	return &ReplicationMonitor{
		tasks:        make(map[string]*ReplicationTask),
		files:        make(map[string]*ReplicationFile),
		listeners:    make([]ReplicationListener, 0),
		stopChan:     make(chan struct{}),
		updateInterval: 1 * time.Second,
	}
}

// Start starts the replication monitor.
func (m *ReplicationMonitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning.Load() {
		return fmt.Errorf("monitor is already running")
	}

	m.isRunning.Store(true)

	// Start monitoring goroutine
	m.wg.Add(1)
	go m.monitorLoop()

	return nil
}

// Stop stops the replication monitor.
func (m *ReplicationMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning.Load() {
		return nil
	}

	m.isRunning.Store(false)
	close(m.stopChan)
	m.wg.Wait()

	return nil
}

// IsRunning returns true if the monitor is running.
func (m *ReplicationMonitor) IsRunning() bool {
	return m.isRunning.Load()
}

// AddListener adds a replication listener.
func (m *ReplicationMonitor) AddListener(listener ReplicationListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listeners = append(m.listeners, listener)
}

// RemoveListener removes a replication listener.
func (m *ReplicationMonitor) RemoveListener(listener ReplicationListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, l := range m.listeners {
		if l == listener {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			break
		}
	}
}

// MonitorTask starts monitoring a replication task.
func (m *ReplicationMonitor) MonitorTask(task *ReplicationTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task == nil {
		return
	}

	m.tasks[task.ID] = task
}

// StopMonitoringTask stops monitoring a task.
func (m *ReplicationMonitor) StopMonitoringTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tasks, taskID)
}

// MonitorFile starts monitoring a file replication.
func (m *ReplicationMonitor) MonitorFile(file *ReplicationFile) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if file == nil {
		return
	}

	m.files[file.Name] = file
}

// StopMonitoringFile stops monitoring a file.
func (m *ReplicationMonitor) StopMonitoringFile(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.files, filename)
}

// GetTaskCount returns the number of monitored tasks.
func (m *ReplicationMonitor) GetTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.tasks)
}

// GetFileCount returns the number of monitored files.
func (m *ReplicationMonitor) GetFileCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.files)
}

// GetOverallProgress returns the overall replication progress (0-100).
func (m *ReplicationMonitor) GetOverallProgress() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.tasks) == 0 {
		return 0
	}

	totalProgress := 0
	for _, task := range m.tasks {
		totalProgress += task.GetProgress()
	}

	return totalProgress / len(m.tasks)
}

// GetCompletedTaskCount returns the number of completed tasks.
func (m *ReplicationMonitor) GetCompletedTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusCompleted {
			count++
		}
	}

	return count
}

// GetFailedTaskCount returns the number of failed tasks.
func (m *ReplicationMonitor) GetFailedTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusFailed {
			count++
		}
	}

	return count
}

// GetActiveTaskCount returns the number of active (running) tasks.
func (m *ReplicationMonitor) GetActiveTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusRunning {
			count++
		}
	}

	return count
}

// monitorLoop is the main monitoring loop.
func (m *ReplicationMonitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkProgress()
		}
	}
}

// checkProgress checks the progress of monitored tasks and files.
func (m *ReplicationMonitor) checkProgress() {
	m.mu.RLock()
	tasks := make([]*ReplicationTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	files := make([]*ReplicationFile, 0, len(m.files))
	for _, file := range m.files {
		files = append(files, file)
	}
	m.mu.RUnlock()

	// Check task statuses
	for _, task := range tasks {
		m.notifyTaskStatus(task)
	}

	// Check file statuses
	for _, file := range files {
		m.notifyFileStatus(file)
	}
}

// notifyTaskStatus notifies listeners of task status changes.
func (m *ReplicationMonitor) notifyTaskStatus(task *ReplicationTask) {
	status := task.GetStatus()

	switch status {
	case ReplicationTaskStatusRunning:
		m.notifyTaskStarted(task)
	case ReplicationTaskStatusCompleted:
		m.notifyTaskCompleted(task)
	case ReplicationTaskStatusFailed:
		m.notifyTaskFailed(task, nil)
	}
}

// notifyFileStatus notifies listeners of file status changes.
func (m *ReplicationMonitor) notifyFileStatus(file *ReplicationFile) {
	status := file.GetStatus()

	switch status {
	case ReplicationFileStatusTransferring:
		m.notifyFileTransferStarted(file)
	case ReplicationFileStatusCompleted:
		m.notifyFileTransferCompleted(file)
	}

	// Always notify progress
	m.notifyFileTransferProgress(file)
}

// notifyTaskStarted notifies listeners that a task started.
func (m *ReplicationMonitor) notifyTaskStarted(task *ReplicationTask) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnTaskStarted(task)
	}
}

// notifyTaskCompleted notifies listeners that a task completed.
func (m *ReplicationMonitor) notifyTaskCompleted(task *ReplicationTask) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnTaskCompleted(task)
	}
}

// notifyTaskFailed notifies listeners that a task failed.
func (m *ReplicationMonitor) notifyTaskFailed(task *ReplicationTask, err error) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnTaskFailed(task, err)
	}
}

// notifyFileTransferStarted notifies listeners that a file transfer started.
func (m *ReplicationMonitor) notifyFileTransferStarted(file *ReplicationFile) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnFileTransferStarted(file)
	}
}

// notifyFileTransferProgress notifies listeners of file transfer progress.
func (m *ReplicationMonitor) notifyFileTransferProgress(file *ReplicationFile) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnFileTransferProgress(file, file.TransferredBytes)
	}
}

// notifyFileTransferCompleted notifies listeners that a file transfer completed.
func (m *ReplicationMonitor) notifyFileTransferCompleted(file *ReplicationFile) {
	m.mu.RLock()
	listeners := make([]ReplicationListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnFileTransferCompleted(file)
	}
}

// GetSummary returns a summary of replication status.
func (m *ReplicationMonitor) GetSummary() ReplicationSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ReplicationSummary{
		TotalTasks:       len(m.tasks),
		ActiveTasks:      m.getActiveTaskCount(),
		CompletedTasks:   m.getCompletedTaskCount(),
		FailedTasks:      m.getFailedTaskCount(),
		TotalFiles:       len(m.files),
		OverallProgress:  m.getOverallProgress(),
	}
}

// getActiveTaskCount returns the count of active tasks (internal, must hold lock).
func (m *ReplicationMonitor) getActiveTaskCount() int {
	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusRunning {
			count++
		}
	}
	return count
}

// getCompletedTaskCount returns the count of completed tasks (internal, must hold lock).
func (m *ReplicationMonitor) getCompletedTaskCount() int {
	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusCompleted {
			count++
		}
	}
	return count
}

// getFailedTaskCount returns the count of failed tasks (internal, must hold lock).
func (m *ReplicationMonitor) getFailedTaskCount() int {
	count := 0
	for _, task := range m.tasks {
		if task.GetStatus() == ReplicationTaskStatusFailed {
			count++
		}
	}
	return count
}

// getOverallProgress returns the overall progress (internal, must hold lock).
func (m *ReplicationMonitor) getOverallProgress() int {
	if len(m.tasks) == 0 {
		return 0
	}

	totalProgress := 0
	for _, task := range m.tasks {
		totalProgress += task.GetProgress()
	}

	return totalProgress / len(m.tasks)
}

// String returns a string representation of the ReplicationMonitor.
func (m *ReplicationMonitor) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return fmt.Sprintf("ReplicationMonitor{running=%v, tasks=%d, files=%d, progress=%d%%}",
		m.isRunning.Load(), len(m.tasks), len(m.files), m.getOverallProgress())
}

// ReplicationSummary holds a summary of replication status.
type ReplicationSummary struct {
	// TotalTasks is the total number of tasks
	TotalTasks int

	// ActiveTasks is the number of active (running) tasks
	ActiveTasks int

	// CompletedTasks is the number of completed tasks
	CompletedTasks int

	// FailedTasks is the number of failed tasks
	FailedTasks int

	// TotalFiles is the total number of files
	TotalFiles int

	// OverallProgress is the overall replication progress (0-100)
	OverallProgress int
}
