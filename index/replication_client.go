package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicationClient handles the client-side of index replication.
// It manages replication sessions and coordinates with a ReplicationServer.
type ReplicationClient struct {
	mu sync.RWMutex

	// serverAddress is the address of the replication server
	serverAddress string

	// sessionID is the current replication session ID
	sessionID string

	// currentRevision is the current index revision
	currentRevision *IndexRevision

	// pendingFiles tracks files that need to be downloaded
	pendingFiles map[string]bool

	// isOpen indicates if the client is open
	isOpen atomic.Bool

	// lastSyncTime tracks when the last sync occurred
	lastSyncTime time.Time

	// syncCount tracks the number of syncs
	syncCount int64

	// autoSyncInterval is the interval for automatic syncs
	autoSyncInterval time.Duration

	// stopChan signals the auto-sync goroutine to stop
	stopChan chan struct{}

	// wg waits for goroutines
	wg sync.WaitGroup
}

// NewReplicationClient creates a new ReplicationClient.
func NewReplicationClient(serverAddress string) (*ReplicationClient, error) {
	if serverAddress == "" {
		return nil, fmt.Errorf("server address cannot be empty")
	}

	rc := &ReplicationClient{
		serverAddress:    serverAddress,
		pendingFiles:     make(map[string]bool),
		lastSyncTime:     time.Now(),
		autoSyncInterval: 0,
		stopChan:         make(chan struct{}),
	}

	rc.isOpen.Store(true)

	return rc, nil
}

// Connect establishes a connection to the replication server and creates a session.
func (rc *ReplicationClient) Connect(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.isOpen.Load() {
		return fmt.Errorf("replication client is closed")
	}

	// In a real implementation, this would:
	// 1. Connect to the server
	// 2. Create a replication session
	// 3. Get the current revision
	rc.sessionID = generateReplicationSessionID()

	return nil
}

// Disconnect closes the connection to the replication server.
func (rc *ReplicationClient) Disconnect(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.isOpen.Load() {
		return nil
	}

	// In a real implementation, this would close the session on the server
	rc.sessionID = ""
	rc.pendingFiles = make(map[string]bool)

	return nil
}

// Sync performs a synchronization with the replication server.
func (rc *ReplicationClient) Sync(ctx context.Context) error {
	rc.mu.Lock()
	if !rc.isOpen.Load() {
		rc.mu.Unlock()
		return fmt.Errorf("replication client is closed")
	}

	if rc.sessionID == "" {
		rc.mu.Unlock()
		return fmt.Errorf("not connected to server")
	}
	rc.mu.Unlock()

	// In a real implementation, this would:
	// 1. Fetch the current revision from the server
	// 2. Compare with local revision
	// 3. Download missing files

	rc.mu.Lock()
	rc.lastSyncTime = time.Now()
	rc.syncCount++
	rc.mu.Unlock()

	return nil
}

// StartAutoSync starts automatic synchronization at the configured interval.
func (rc *ReplicationClient) StartAutoSync(interval time.Duration) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.isOpen.Load() {
		return fmt.Errorf("replication client is closed")
	}

	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	rc.autoSyncInterval = interval

	// Start auto-sync goroutine
	rc.wg.Add(1)
	go rc.autoSyncLoop()

	return nil
}

// StopAutoSync stops automatic synchronization.
func (rc *ReplicationClient) StopAutoSync() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.autoSyncInterval == 0 {
		return nil
	}

	close(rc.stopChan)
	rc.wg.Wait()

	rc.autoSyncInterval = 0
	rc.stopChan = make(chan struct{})

	return nil
}

// autoSyncLoop runs the automatic synchronization loop.
func (rc *ReplicationClient) autoSyncLoop() {
	defer rc.wg.Done()

	rc.mu.RLock()
	interval := rc.autoSyncInterval
	rc.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.stopChan:
			return
		case <-ticker.C:
			ctx := context.Background()
			rc.Sync(ctx)
		}
	}
}

// IsAutoSyncRunning returns true if auto-sync is running.
func (rc *ReplicationClient) IsAutoSyncRunning() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.autoSyncInterval > 0
}

// GetServerAddress returns the server address.
func (rc *ReplicationClient) GetServerAddress() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.serverAddress
}

// GetSessionID returns the current session ID.
func (rc *ReplicationClient) GetSessionID() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.sessionID
}

// IsConnected returns true if connected to the server.
func (rc *ReplicationClient) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.sessionID != ""
}

// GetCurrentRevision returns the current index revision.
func (rc *ReplicationClient) GetCurrentRevision() *IndexRevision {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.currentRevision == nil {
		return nil
	}

	return rc.currentRevision.Clone()
}

// SetCurrentRevision sets the current index revision.
func (rc *ReplicationClient) SetCurrentRevision(revision *IndexRevision) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.isOpen.Load() {
		return fmt.Errorf("replication client is closed")
	}

	rc.currentRevision = revision
	return nil
}

// AddPendingFile adds a file to the pending download list.
func (rc *ReplicationClient) AddPendingFile(filename string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.pendingFiles[filename] = true
}

// RemovePendingFile removes a file from the pending download list.
func (rc *ReplicationClient) RemovePendingFile(filename string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	delete(rc.pendingFiles, filename)
}

// GetPendingFiles returns the list of pending files.
func (rc *ReplicationClient) GetPendingFiles() []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	files := make([]string, 0, len(rc.pendingFiles))
	for file := range rc.pendingFiles {
		files = append(files, file)
	}
	return files
}

// HasPendingFiles returns true if there are pending files.
func (rc *ReplicationClient) HasPendingFiles() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return len(rc.pendingFiles) > 0
}

// GetSyncCount returns the number of syncs performed.
func (rc *ReplicationClient) GetSyncCount() int64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.syncCount
}

// GetLastSyncTime returns the time of the last sync.
func (rc *ReplicationClient) GetLastSyncTime() time.Time {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.lastSyncTime
}

// Close closes the ReplicationClient.
func (rc *ReplicationClient) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.isOpen.Load() {
		return nil
	}

	rc.isOpen.Store(false)

	// Stop auto-sync if running
	if rc.autoSyncInterval > 0 {
		close(rc.stopChan)
		rc.wg.Wait()
	}

	// Disconnect if connected
	if rc.sessionID != "" {
		ctx := context.Background()
		rc.disconnectInternal(ctx)
	}

	return nil
}

// disconnectInternal is the internal disconnect method (must be called with lock held).
func (rc *ReplicationClient) disconnectInternal(ctx context.Context) {
	rc.sessionID = ""
	rc.pendingFiles = make(map[string]bool)
}

// IsOpen returns true if the client is open.
func (rc *ReplicationClient) IsOpen() bool {
	return rc.isOpen.Load()
}

// String returns a string representation of the ReplicationClient.
func (rc *ReplicationClient) String() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return fmt.Sprintf("ReplicationClient{open=%v, server=%s, connected=%v, session=%s}",
		rc.isOpen.Load(), rc.serverAddress, rc.sessionID != "", rc.sessionID)
}

// generateReplicationSessionID generates a unique session ID.
func generateReplicationSessionID() string {
	return fmt.Sprintf("replication-session-%d", time.Now().UnixNano())
}
