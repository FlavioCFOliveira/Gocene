package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTManager is the main manager for Near Real-Time (NRT) search operations.
// It coordinates between IndexWriter and IndexReader to provide immediate
// visibility of index changes.
// This is the Go port of Lucene's NRTManager pattern.
type NRTManager struct {
	mu sync.RWMutex

	// writer is the IndexWriter for NRT operations
	writer *IndexWriter

	// currentReader is the current NRTDirectoryReader
	currentReader *NRTDirectoryReader

	// isOpen indicates if the manager is open
	isOpen atomic.Bool

	// generation tracks the NRT generation
	generation int64

	// waitingReaders holds channels of readers waiting for updates
	waitingReaders []chan *NRTDirectoryReader

	// maxRefreshSec is the maximum time to wait for a refresh
	maxRefreshSec time.Duration

	// applyAllDeletes indicates if all deletes should be applied
	applyAllDeletes bool

	// reopenListeners are called when a reopen occurs
	reopenListeners []NRTReopenListener
}

// NRTReopenListener is called when the NRT reader is reopened.
type NRTReopenListener interface {
	// BeforeReopen is called before reopen
	BeforeReopen()
	// AfterReopen is called after reopen with the new generation
	AfterReopen(generation int64)
}

// NewNRTManager creates a new NRTManager with the given IndexWriter.
func NewNRTManager(writer *IndexWriter) (*NRTManager, error) {
	if writer == nil {
		return nil, fmt.Errorf("writer cannot be nil")
	}

	manager := &NRTManager{
		writer:          writer,
		generation:      1,
		maxRefreshSec:   60 * time.Second,
		applyAllDeletes: true,
	}

	manager.isOpen.Store(true)

	return manager, nil
}

// GetReader returns the current NRT reader.
// If no reader exists, creates one from the writer.
func (m *NRTManager) GetReader() (*NRTDirectoryReader, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isOpen.Load() {
		return nil, fmt.Errorf("manager is closed")
	}

	if m.currentReader != nil {
		return m.currentReader, nil
	}

	// Need to create a new reader - upgrade to write lock
	m.mu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.currentReader != nil {
		return m.currentReader, nil
	}

	// Create initial reader - in real implementation this would come from writer
	// For now, return nil as we need a proper Directory to open a reader
	return nil, fmt.Errorf("no reader available")
}

// MaybeRefresh checks if the index has changed and refreshes the reader if necessary.
// Returns true if a refresh was performed.
func (m *NRTManager) MaybeRefresh(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen.Load() {
		return false, fmt.Errorf("manager is closed")
	}

	// Check if refresh is needed
	if m.currentReader == nil {
		return false, fmt.Errorf("no current reader")
	}

	isCurrent, err := m.currentReader.IsCurrent()
	if err != nil {
		return false, err
	}

	if isCurrent {
		return false, nil
	}

	// Perform refresh
	m.notifyBeforeReopen()

	// Create new reader - in real implementation this would come from writer
	// For now, we increment generation but don't create a new reader
	m.generation++

	// Notify waiting readers
	m.notifyWaitingReaders()
	m.notifyAfterReopen()

	return true, nil
}

// Refresh blocks until a new reader is available.
func (m *NRTManager) Refresh(ctx context.Context) (*NRTDirectoryReader, error) {
	// Try to get current reader first
	reader, err := m.GetReader()
	if err != nil {
		return nil, err
	}

	// Check if already current
	isCurrent, err := reader.IsCurrent()
	if err != nil {
		return nil, err
	}

	if isCurrent {
		return reader, nil
	}

	// Need to wait for refresh - create a channel
	waitChan := make(chan *NRTDirectoryReader, 1)

	m.mu.Lock()
	m.waitingReaders = append(m.waitingReaders, waitChan)
	m.mu.Unlock()

	// Try refresh
	m.MaybeRefresh(ctx)

	// Wait for new reader with timeout
	select {
	case newReader := <-waitChan:
		return newReader, nil
	case <-time.After(m.maxRefreshSec):
		m.removeWaitingReader(waitChan)
		return nil, fmt.Errorf("timeout waiting for refresh")
	case <-ctx.Done():
		m.removeWaitingReader(waitChan)
		return nil, ctx.Err()
	}
}

// Close closes the NRTManager.
func (m *NRTManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen.Load() {
		return nil
	}

	m.isOpen.Store(false)

	// Close current reader
	if m.currentReader != nil {
		m.currentReader.Close()
		m.currentReader = nil
	}

	// Notify all waiting readers
	for _, ch := range m.waitingReaders {
		close(ch)
	}
	m.waitingReaders = nil

	return nil
}

// IsOpen returns true if the manager is open.
func (m *NRTManager) IsOpen() bool {
	return m.isOpen.Load()
}

// GetGeneration returns the current NRT generation.
func (m *NRTManager) GetGeneration() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generation
}

// GetWriter returns the IndexWriter.
func (m *NRTManager) GetWriter() *IndexWriter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writer
}

// SetMaxRefreshSec sets the maximum time to wait for a refresh.
func (m *NRTManager) SetMaxRefreshSec(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxRefreshSec = duration
}

// GetMaxRefreshSec returns the maximum time to wait for a refresh.
func (m *NRTManager) GetMaxRefreshSec() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxRefreshSec
}

// SetApplyAllDeletes sets whether all deletes should be applied.
func (m *NRTManager) SetApplyAllDeletes(apply bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applyAllDeletes = apply
}

// GetApplyAllDeletes returns whether all deletes should be applied.
func (m *NRTManager) GetApplyAllDeletes() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.applyAllDeletes
}

// AddReopenListener adds a listener for reopen events.
func (m *NRTManager) AddReopenListener(listener NRTReopenListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reopenListeners = append(m.reopenListeners, listener)
}

// RemoveReopenListener removes a listener for reopen events.
func (m *NRTManager) RemoveReopenListener(listener NRTReopenListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, l := range m.reopenListeners {
		if l == listener {
			m.reopenListeners = append(m.reopenListeners[:i], m.reopenListeners[i+1:]...)
			return
		}
	}
}

// notifyBeforeReopen notifies all listeners before reopen.
func (m *NRTManager) notifyBeforeReopen() {
	for _, listener := range m.reopenListeners {
		listener.BeforeReopen()
	}
}

// notifyAfterReopen notifies all listeners after reopen.
func (m *NRTManager) notifyAfterReopen() {
	for _, listener := range m.reopenListeners {
		listener.AfterReopen(m.generation)
	}
}

// notifyWaitingReaders notifies all waiting readers of a new reader.
func (m *NRTManager) notifyWaitingReaders() {
	if m.currentReader == nil {
		return
	}

	for _, ch := range m.waitingReaders {
		select {
		case ch <- m.currentReader:
		default:
		}
	}
	m.waitingReaders = nil
}

// removeWaitingReader removes a waiting reader channel.
func (m *NRTManager) removeWaitingReader(ch chan *NRTDirectoryReader) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, waiter := range m.waitingReaders {
		if waiter == ch {
			m.waitingReaders = append(m.waitingReaders[:i], m.waitingReaders[i+1:]...)
			return
		}
	}
}

// WaitForGeneration waits until the reader generation reaches or exceeds the target.
func (m *NRTManager) WaitForGeneration(ctx context.Context, targetGen int64) (*NRTDirectoryReader, error) {
	m.mu.RLock()
	currentGen := m.generation
	reader := m.currentReader
	m.mu.RUnlock()

	if currentGen >= targetGen {
		return reader, nil
	}

	// Need to wait
	waitChan := make(chan *NRTDirectoryReader, 1)

	m.mu.Lock()
	if m.generation >= targetGen {
		reader := m.currentReader
		m.mu.Unlock()
		return reader, nil
	}
	m.waitingReaders = append(m.waitingReaders, waitChan)
	m.mu.Unlock()

	// Wait with timeout
	select {
	case newReader := <-waitChan:
		return newReader, nil
	case <-time.After(m.maxRefreshSec):
		m.removeWaitingReader(waitChan)
		return nil, fmt.Errorf("timeout waiting for generation %d", targetGen)
	case <-ctx.Done():
		m.removeWaitingReader(waitChan)
		return nil, ctx.Err()
	}
}
