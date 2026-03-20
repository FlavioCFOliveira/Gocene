package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTReplicationWriter handles writing index data for NRT (Near Real-Time) replication.
// It coordinates with the IndexWriter to provide consistent snapshots for replication.
// This is the Go port of Lucene's NRT replication writer pattern.
type NRTReplicationWriter struct {
	mu sync.RWMutex

	// writer is the underlying IndexWriter
	writer *IndexWriter

	// currentRevision is the current index revision
	currentRevision *IndexRevision

	// sessions holds active replication sessions
	sessions map[string]*ReplicationSession

	// isOpen indicates if the writer is open
	isOpen atomic.Bool

	// lastCommitTime tracks when the last commit occurred
	lastCommitTime time.Time

	// commitCount tracks the number of commits
	commitCount int64
}

// IndexRevision represents a point-in-time snapshot of the index.
type IndexRevision struct {
	// Generation is the commit generation
	Generation int64

	// Version is the index version
	Version int64

	// Timestamp is when this revision was created
	Timestamp time.Time

	// Files is the list of files in this revision
	Files []string

	// SegmentInfos contains segment information
	SegmentInfos *SegmentInfos
}

// ReplicationSession represents an active replication session.
type ReplicationSession struct {
	// ID is the unique session identifier
	ID string

	// Revision is the index revision being replicated
	Revision *IndexRevision

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// ExpiresAt is when the session expires
	ExpiresAt time.Time
}

// NewNRTReplicationWriter creates a new NRTReplicationWriter.
func NewNRTReplicationWriter(writer *IndexWriter) (*NRTReplicationWriter, error) {
	if writer == nil {
		return nil, fmt.Errorf("writer cannot be nil")
	}

	rw := &NRTReplicationWriter{
		writer:          writer,
		sessions:        make(map[string]*ReplicationSession),
		lastCommitTime:  time.Now(),
		currentRevision: &IndexRevision{},
	}

	rw.isOpen.Store(true)

	return rw, nil
}

// GetCurrentRevision returns the current index revision.
func (rw *NRTReplicationWriter) GetCurrentRevision() (*IndexRevision, error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if !rw.isOpen.Load() {
		return nil, fmt.Errorf("replication writer is closed")
	}

	return rw.currentRevision, nil
}

// CreateSession creates a new replication session for the current revision.
func (rw *NRTReplicationWriter) CreateSession(ctx context.Context, ttl time.Duration) (*ReplicationSession, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.isOpen.Load() {
		return nil, fmt.Errorf("replication writer is closed")
	}

	session := &ReplicationSession{
		ID:        generateSessionID(),
		Revision:  rw.currentRevision,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	rw.sessions[session.ID] = session

	return session, nil
}

// GetSession returns a replication session by ID.
func (rw *NRTReplicationWriter) GetSession(sessionID string) (*ReplicationSession, error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if !rw.isOpen.Load() {
		return nil, fmt.Errorf("replication writer is closed")
	}

	session, ok := rw.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	return session, nil
}

// CloseSession closes a replication session.
func (rw *NRTReplicationWriter) CloseSession(sessionID string) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.isOpen.Load() {
		return fmt.Errorf("replication writer is closed")
	}

	delete(rw.sessions, sessionID)

	return nil
}

// UpdateRevision updates the current index revision after a commit.
func (rw *NRTReplicationWriter) UpdateRevision() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.isOpen.Load() {
		return fmt.Errorf("replication writer is closed")
	}

	// Increment generation
	rw.currentRevision.Generation++
	rw.currentRevision.Version++
	rw.currentRevision.Timestamp = time.Now()

	// Update last commit time
	rw.lastCommitTime = time.Now()
	rw.commitCount++

	return nil
}

// GetFileData returns the data for a specific file in the current revision.
func (rw *NRTReplicationWriter) GetFileData(ctx context.Context, filename string) ([]byte, error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if !rw.isOpen.Load() {
		return nil, fmt.Errorf("replication writer is closed")
	}

	// In a real implementation, this would read from the directory
	// For now, return empty data
	return []byte{}, nil
}

// GetFileList returns the list of files in the current revision.
func (rw *NRTReplicationWriter) GetFileList() ([]string, error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if !rw.isOpen.Load() {
		return nil, fmt.Errorf("replication writer is closed")
	}

	return rw.currentRevision.Files, nil
}

// CleanupSessions removes expired sessions.
func (rw *NRTReplicationWriter) CleanupSessions() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now()
	for id, session := range rw.sessions {
		if now.After(session.ExpiresAt) {
			delete(rw.sessions, id)
		}
	}
}

// Close closes the NRTReplicationWriter.
func (rw *NRTReplicationWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.isOpen.Load() {
		return nil
	}

	rw.isOpen.Store(false)

	// Clear all sessions
	rw.sessions = nil
	rw.currentRevision = nil

	return nil
}

// IsOpen returns true if the writer is open.
func (rw *NRTReplicationWriter) IsOpen() bool {
	return rw.isOpen.Load()
}

// GetSessionCount returns the number of active sessions.
func (rw *NRTReplicationWriter) GetSessionCount() int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	return len(rw.sessions)
}

// GetCommitCount returns the number of commits.
func (rw *NRTReplicationWriter) GetCommitCount() int64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	return rw.commitCount
}

// GetLastCommitTime returns the time of the last commit.
func (rw *NRTReplicationWriter) GetLastCommitTime() time.Time {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	return rw.lastCommitTime
}

// String returns a string representation of the NRTReplicationWriter.
func (rw *NRTReplicationWriter) String() string {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	return fmt.Sprintf("NRTReplicationWriter{open=%v, sessions=%d, generation=%d}",
		rw.isOpen.Load(), len(rw.sessions), rw.currentRevision.Generation)
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
