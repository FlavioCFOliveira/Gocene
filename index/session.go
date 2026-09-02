// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"time"
)

// Session represents a replication session for NRT (Near Real-Time) operations.
// It tracks files that have been replicated and manages the session lifecycle.
//
// This is the Go port of Lucene's replication session pattern used in NRT replication.
type Session struct {
	mu sync.RWMutex

	// id is the unique identifier for this session
	id string

	// files tracks which files have been replicated in this session
	files map[string]bool

	// createdAt is when the session was created
	createdAt time.Time

	// lastActivity is the timestamp of the last activity
	lastActivity time.Time

	// isOpen indicates if the session is still open
	isOpen bool

	// metadata stores optional session metadata
	metadata map[string]string
}

// NewSession creates a new Session with the given ID.
//
// The session ID should be unique across all active sessions. If an empty ID
// is provided, an error will be returned.
func NewSession(id string) (*Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	now := time.Now()
	return &Session{
		id:           id,
		files:        make(map[string]bool),
		createdAt:    now,
		lastActivity: now,
		isOpen:       true,
		metadata:     make(map[string]string),
	}, nil
}

// GetID returns the session ID.
func (s *Session) GetID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// AddFile adds a file to the session's tracking list.
// If the file already exists in the session, this is a no-op.
func (s *Session) AddFile(fileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return fmt.Errorf("session %s is closed", s.id)
	}

	s.files[fileName] = true
	s.lastActivity = time.Now()
	return nil
}

// RemoveFile removes a file from the session's tracking list.
// Returns true if the file was found and removed, false otherwise.
func (s *Session) RemoveFile(fileName string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return false, fmt.Errorf("session %s is closed", s.id)
	}

	if _, exists := s.files[fileName]; exists {
		delete(s.files, fileName)
		s.lastActivity = time.Now()
		return true, nil
	}

	return false, nil
}

// HasFile returns true if the file is tracked in this session.
func (s *Session) HasFile(fileName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files[fileName]
}

// GetFiles returns a slice of all files tracked in this session.
// The returned slice is a copy and can be safely modified by the caller.
func (s *Session) GetFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]string, 0, len(s.files))
	for file := range s.files {
		files = append(files, file)
	}
	return files
}

// GetFileCount returns the number of files tracked in this session.
func (s *Session) GetFileCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.files)
}

// IsExpired returns true if the session has been inactive for longer
// than the specified timeout duration.
func (s *Session) IsExpired(timeout time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastActivity) > timeout
}

// UpdateActivity updates the last activity timestamp to the current time.
func (s *Session) UpdateActivity() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return fmt.Errorf("session %s is closed", s.id)
	}

	s.lastActivity = time.Now()
	return nil
}

// GetLastActivity returns the timestamp of the last activity.
func (s *Session) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

// GetCreatedAt returns the session creation timestamp.
func (s *Session) GetCreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAt
}

// Close closes the session and releases resources.
// After closing, the session cannot be used for tracking files.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return nil
	}

	s.isOpen = false
	s.files = nil
	s.metadata = nil
	return nil
}

// IsOpen returns true if the session is open and usable.
func (s *Session) IsOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isOpen
}

// SetMetadata sets a metadata key-value pair for this session.
func (s *Session) SetMetadata(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return fmt.Errorf("session %s is closed", s.id)
	}

	if s.metadata == nil {
		s.metadata = make(map[string]string)
	}
	s.metadata[key] = value
	s.lastActivity = time.Now()
	return nil
}

// GetMetadata returns the metadata value for the given key.
// Returns an empty string if the key doesn't exist.
func (s *Session) GetMetadata(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata[key]
}

// GetAllMetadata returns a copy of all session metadata.
func (s *Session) GetAllMetadata() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metadata == nil {
		return nil
	}

	copy := make(map[string]string, len(s.metadata))
	for k, v := range s.metadata {
		copy[k] = v
	}
	return copy
}

// GetDuration returns the duration since the session was created.
func (s *Session) GetDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.createdAt)
}

// GetInactivityDuration returns the duration since the last activity.
func (s *Session) GetInactivityDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastActivity)
}

// String returns a string representation of the Session.
func (s *Session) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return fmt.Sprintf("Session{id=%s, files=%d, open=%v, created=%s, lastActivity=%s}",
		s.id, len(s.files), s.isOpen, s.createdAt.Format(time.RFC3339), s.lastActivity.Format(time.RFC3339))
}
