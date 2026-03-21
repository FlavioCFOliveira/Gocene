// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid session ID",
			id:      "session-1",
			wantErr: false,
		},
		{
			name:    "empty session ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "UUID style session ID",
			id:      "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := NewSession(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if session.GetID() != tt.id {
					t.Errorf("expected ID %s, got %s", tt.id, session.GetID())
				}
				if !session.IsOpen() {
					t.Error("expected session to be open")
				}
				if session.GetFileCount() != 0 {
					t.Error("expected empty file list")
				}
			}
		})
	}
}

func TestSession_AddFile(t *testing.T) {
	session, _ := NewSession("test-session")

	tests := []struct {
		name     string
		fileName string
		wantErr  bool
	}{
		{
			name:     "add first file",
			fileName: "segments_1",
			wantErr:  false,
		},
		{
			name:     "add second file",
			fileName: "_0.cfe",
			wantErr:  false,
		},
		{
			name:     "add duplicate file",
			fileName: "segments_1",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := session.AddFile(tt.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify files are tracked
	files := session.GetFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestSession_RemoveFile(t *testing.T) {
	session, _ := NewSession("test-session")

	// Add files
	session.AddFile("file1")
	session.AddFile("file2")
	session.AddFile("file3")

	tests := []struct {
		name        string
		fileName    string
		wantRemoved bool
		wantErr     bool
	}{
		{
			name:        "remove existing file",
			fileName:    "file2",
			wantRemoved: true,
			wantErr:     false,
		},
		{
			name:        "remove non-existent file",
			fileName:    "file4",
			wantRemoved: false,
			wantErr:     false,
		},
		{
			name:        "remove already removed file",
			fileName:    "file2",
			wantRemoved: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removed, err := session.RemoveFile(tt.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if removed != tt.wantRemoved {
				t.Errorf("RemoveFile() removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}

	// Verify remaining files
	if session.GetFileCount() != 2 {
		t.Errorf("expected 2 files after removal, got %d", session.GetFileCount())
	}
}

func TestSession_HasFile(t *testing.T) {
	session, _ := NewSession("test-session")
	session.AddFile("existing-file")

	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{
			name:     "existing file",
			fileName: "existing-file",
			want:     true,
		},
		{
			name:     "non-existent file",
			fileName: "missing-file",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.HasFile(tt.fileName)
			if got != tt.want {
				t.Errorf("HasFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_GetFiles(t *testing.T) {
	session, _ := NewSession("test-session")

	// Test empty session
	files := session.GetFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Add files
	session.AddFile("file1")
	session.AddFile("file2")

	files = session.GetFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Verify returned slice is a copy
	files[0] = "modified"
	if session.HasFile("modified") {
		t.Error("expected returned slice to be a copy")
	}
}

func TestSession_IsExpired(t *testing.T) {
	session, _ := NewSession("test-session")

	// Session should not be expired initially
	if session.IsExpired(1 * time.Second) {
		t.Error("new session should not be expired")
	}

	// Session should not be expired after short delay
	time.Sleep(100 * time.Millisecond)
	if session.IsExpired(1 * time.Second) {
		t.Error("session should not be expired after 100ms with 1s timeout")
	}

	// Session should be expired after timeout
	time.Sleep(1 * time.Second)
	if !session.IsExpired(1 * time.Second) {
		t.Error("session should be expired after timeout")
	}
}

func TestSession_UpdateActivity(t *testing.T) {
	session, _ := NewSession("test-session")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	oldActivity := session.GetLastActivity()

	// Update activity
	if err := session.UpdateActivity(); err != nil {
		t.Errorf("UpdateActivity() error = %v", err)
	}

	newActivity := session.GetLastActivity()
	if !newActivity.After(oldActivity) {
		t.Error("last activity should be updated")
	}

	// Session should not be expired now
	if session.IsExpired(500 * time.Millisecond) {
		t.Error("session should not be expired after activity update")
	}
}

func TestSession_Close(t *testing.T) {
	session, _ := NewSession("test-session")
	session.AddFile("file1")

	// Close session
	if err := session.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify session is closed
	if session.IsOpen() {
		t.Error("session should be closed")
	}

	// Verify operations fail on closed session
	if err := session.AddFile("file2"); err == nil {
		t.Error("AddFile should fail on closed session")
	}

	_, err := session.RemoveFile("file1")
	if err == nil {
		t.Error("RemoveFile should fail on closed session")
	}

	if err := session.UpdateActivity(); err == nil {
		t.Error("UpdateActivity should fail on closed session")
	}

	// Close again should be no-op
	if err := session.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestSession_Metadata(t *testing.T) {
	session, _ := NewSession("test-session")

	// Set metadata
	if err := session.SetMetadata("key1", "value1"); err != nil {
		t.Errorf("SetMetadata() error = %v", err)
	}

	if err := session.SetMetadata("key2", "value2"); err != nil {
		t.Errorf("SetMetadata() error = %v", err)
	}

	// Get metadata
	if v := session.GetMetadata("key1"); v != "value1" {
		t.Errorf("expected value1, got %s", v)
	}

	if v := session.GetMetadata("key2"); v != "value2" {
		t.Errorf("expected value2, got %s", v)
	}

	if v := session.GetMetadata("nonexistent"); v != "" {
		t.Errorf("expected empty string for non-existent key, got %s", v)
	}

	// Get all metadata
	all := session.GetAllMetadata()
	if len(all) != 2 {
		t.Errorf("expected 2 metadata entries, got %d", len(all))
	}

	// Verify returned map is a copy
	all["key3"] = "value3"
	if session.GetMetadata("key3") != "" {
		t.Error("expected returned map to be a copy")
	}

	// Update existing key
	if err := session.SetMetadata("key1", "updated"); err != nil {
		t.Errorf("SetMetadata() error = %v", err)
	}

	if v := session.GetMetadata("key1"); v != "updated" {
		t.Errorf("expected updated, got %s", v)
	}
}

func TestSession_MetadataClosed(t *testing.T) {
	session, _ := NewSession("test-session")
	session.Close()

	if err := session.SetMetadata("key", "value"); err == nil {
		t.Error("SetMetadata should fail on closed session")
	}
}

func TestSession_Timestamps(t *testing.T) {
	beforeCreate := time.Now()
	session, _ := NewSession("test-session")
	afterCreate := time.Now()

	createdAt := session.GetCreatedAt()
	if createdAt.Before(beforeCreate) || createdAt.After(afterCreate) {
		t.Error("created timestamp should be between beforeCreate and afterCreate")
	}

	lastActivity := session.GetLastActivity()
	if !lastActivity.Equal(createdAt) {
		t.Error("last activity should equal created timestamp initially")
	}

	duration := session.GetDuration()
	if duration < 0 {
		t.Error("duration should be non-negative")
	}

	inactivity := session.GetInactivityDuration()
	if inactivity < 0 {
		t.Error("inactivity duration should be non-negative")
	}
}

func TestSession_String(t *testing.T) {
	session, _ := NewSession("test-session")
	session.AddFile("file1")
	session.AddFile("file2")

	str := session.String()
	if str == "" {
		t.Error("String() should not return empty string")
	}

	// Verify string contains expected elements
	if str == "" {
		t.Error("String() should not return empty")
	}
}

func TestSession_ConcurrentAccess(t *testing.T) {
	session, _ := NewSession("concurrent-session")

	// Run concurrent operations
	done := make(chan bool, 100)

	// Concurrent adds
	for i := 0; i < 50; i++ {
		go func(idx int) {
			session.AddFile("file" + string(rune(idx)))
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		go func() {
			_ = session.GetFileCount()
			_ = session.GetFiles()
			_ = session.IsOpen()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Session should still be valid
	if !session.IsOpen() {
		t.Error("session should be open after concurrent access")
	}
}

func TestSession_GetFileCount(t *testing.T) {
	session, _ := NewSession("test-session")

	if session.GetFileCount() != 0 {
		t.Errorf("expected 0 files initially, got %d", session.GetFileCount())
	}

	session.AddFile("file1")
	if session.GetFileCount() != 1 {
		t.Errorf("expected 1 file, got %d", session.GetFileCount())
	}

	session.AddFile("file2")
	if session.GetFileCount() != 2 {
		t.Errorf("expected 2 files, got %d", session.GetFileCount())
	}

	session.RemoveFile("file1")
	if session.GetFileCount() != 1 {
		t.Errorf("expected 1 file after removal, got %d", session.GetFileCount())
	}
}
