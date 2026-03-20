package index

import (
	"context"
	"testing"
	"time"
)

func TestNewNRTReplicationWriter(t *testing.T) {
	// Create a minimal IndexWriter for testing
	writer := &IndexWriter{}

	rw, err := NewNRTReplicationWriter(writer)
	if err != nil {
		t.Fatalf("failed to create NRTReplicationWriter: %v", err)
	}
	defer rw.Close()

	if rw == nil {
		t.Fatal("expected NRTReplicationWriter to not be nil")
	}

	if !rw.IsOpen() {
		t.Error("expected writer to be open")
	}

	if rw.GetSessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", rw.GetSessionCount())
	}

	if rw.GetCommitCount() != 0 {
		t.Errorf("expected 0 commits, got %d", rw.GetCommitCount())
	}
}

func TestNewNRTReplicationWriter_NilWriter(t *testing.T) {
	_, err := NewNRTReplicationWriter(nil)
	if err == nil {
		t.Error("expected error for nil writer")
	}
}

func TestNRTReplicationWriter_GetCurrentRevision(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	revision, err := rw.GetCurrentRevision()
	if err != nil {
		t.Fatalf("failed to get current revision: %v", err)
	}

	if revision == nil {
		t.Fatal("expected revision to not be nil")
	}

	if revision.Generation != 0 {
		t.Errorf("expected generation 0, got %d", revision.Generation)
	}
}

func TestNRTReplicationWriter_GetCurrentRevision_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	_, err := rw.GetCurrentRevision()
	if err == nil {
		t.Error("expected error when writer is closed")
	}
}

func TestNRTReplicationWriter_CreateSession(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	session, err := rw.CreateSession(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session == nil {
		t.Fatal("expected session to not be nil")
	}

	if session.ID == "" {
		t.Error("expected session ID to not be empty")
	}

	if rw.GetSessionCount() != 1 {
		t.Errorf("expected 1 session, got %d", rw.GetSessionCount())
	}
}

func TestNRTReplicationWriter_CreateSession_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	ctx := context.Background()
	_, err := rw.CreateSession(ctx, 5*time.Minute)
	if err == nil {
		t.Error("expected error when creating session on closed writer")
	}
}

func TestNRTReplicationWriter_GetSession(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	session, _ := rw.CreateSession(ctx, 5*time.Minute)

	// Get the session
	retrieved, err := rw.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Error("expected retrieved session to have same ID")
	}
}

func TestNRTReplicationWriter_GetSession_NotFound(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	_, err := rw.GetSession("non-existent-session")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestNRTReplicationWriter_GetSession_Expired(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	// Create session with very short TTL
	session, _ := rw.CreateSession(ctx, 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, err := rw.GetSession(session.ID)
	if err == nil {
		t.Error("expected error for expired session")
	}
}

func TestNRTReplicationWriter_CloseSession(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	session, _ := rw.CreateSession(ctx, 5*time.Minute)

	err := rw.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("failed to close session: %v", err)
	}

	if rw.GetSessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", rw.GetSessionCount())
	}
}

func TestNRTReplicationWriter_CloseSession_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	err := rw.CloseSession("some-session")
	if err == nil {
		t.Error("expected error when closing session on closed writer")
	}
}

func TestNRTReplicationWriter_UpdateRevision(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	err := rw.UpdateRevision()
	if err != nil {
		t.Fatalf("failed to update revision: %v", err)
	}

	if rw.GetCommitCount() != 1 {
		t.Errorf("expected 1 commit, got %d", rw.GetCommitCount())
	}

	revision, _ := rw.GetCurrentRevision()
	if revision.Generation != 1 {
		t.Errorf("expected generation 1, got %d", revision.Generation)
	}

	if revision.Version != 1 {
		t.Errorf("expected version 1, got %d", revision.Version)
	}
}

func TestNRTReplicationWriter_UpdateRevision_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	err := rw.UpdateRevision()
	if err == nil {
		t.Error("expected error when updating revision on closed writer")
	}
}

func TestNRTReplicationWriter_GetFileList(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	files, err := rw.GetFileList()
	if err != nil {
		t.Fatalf("failed to get file list: %v", err)
	}

	// Empty revision should have empty file list
	if files == nil {
		t.Error("expected file list to not be nil")
	}
}

func TestNRTReplicationWriter_GetFileList_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	_, err := rw.GetFileList()
	if err == nil {
		t.Error("expected error when getting file list from closed writer")
	}
}

func TestNRTReplicationWriter_GetFileData(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	data, err := rw.GetFileData(ctx, "test.txt")
	if err != nil {
		t.Fatalf("failed to get file data: %v", err)
	}

	if data == nil {
		t.Error("expected data to not be nil")
	}
}

func TestNRTReplicationWriter_GetFileData_Closed(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	rw.Close()

	ctx := context.Background()
	_, err := rw.GetFileData(ctx, "test.txt")
	if err == nil {
		t.Error("expected error when getting file data from closed writer")
	}
}

func TestNRTReplicationWriter_CleanupSessions(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	ctx := context.Background()
	// Create sessions with short TTL
	session1, _ := rw.CreateSession(ctx, 1*time.Millisecond)
	session2, _ := rw.CreateSession(ctx, 5*time.Minute)

	// Wait for first session to expire
	time.Sleep(10 * time.Millisecond)

	// Cleanup should remove expired sessions
	rw.CleanupSessions()

	if rw.GetSessionCount() != 1 {
		t.Errorf("expected 1 session after cleanup, got %d", rw.GetSessionCount())
	}

	// Verify session1 is gone
	_, err := rw.GetSession(session1.ID)
	if err == nil {
		t.Error("expected session1 to be removed")
	}

	// Verify session2 still exists
	_, err = rw.GetSession(session2.ID)
	if err != nil {
		t.Errorf("expected session2 to exist: %v", err)
	}
}

func TestNRTReplicationWriter_Close(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)

	err := rw.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if rw.IsOpen() {
		t.Error("expected writer to be closed")
	}

	// Close again should not error
	err = rw.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestNRTReplicationWriter_GetLastCommitTime(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	// Should have initial commit time
	initialTime := rw.GetLastCommitTime()
	if initialTime.IsZero() {
		t.Error("expected initial commit time to be set")
	}

	// Update revision
	time.Sleep(10 * time.Millisecond)
	rw.UpdateRevision()

	newTime := rw.GetLastCommitTime()
	if !newTime.After(initialTime) {
		t.Error("expected new commit time to be after initial time")
	}
}

func TestNRTReplicationWriter_String(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	str := rw.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestNRTReplicationWriter_ConcurrentOperations(t *testing.T) {
	writer := &IndexWriter{}
	rw, _ := NewNRTReplicationWriter(writer)
	defer rw.Close()

	done := make(chan bool, 4)

	// Create sessions concurrently
	go func() {
		for i := 0; i < 10; i++ {
			ctx := context.Background()
			rw.CreateSession(ctx, 5*time.Minute)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get revision concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rw.GetCurrentRevision()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Update revision concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rw.UpdateRevision()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get session count concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rw.GetSessionCount()
			time.Sleep(5 * time.Millisecond)
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
