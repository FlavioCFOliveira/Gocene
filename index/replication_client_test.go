package index

import (
	"context"
	"testing"
	"time"
)

func TestNewReplicationClient(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, err := NewReplicationClient(serverAddress)
	if err != nil {
		t.Fatalf("failed to create ReplicationClient: %v", err)
	}
	defer rc.Close()

	if rc == nil {
		t.Fatal("expected ReplicationClient to not be nil")
	}

	if rc.GetServerAddress() != serverAddress {
		t.Errorf("expected server address %s, got %s", serverAddress, rc.GetServerAddress())
	}

	if !rc.IsOpen() {
		t.Error("expected client to be open")
	}

	if rc.IsConnected() {
		t.Error("expected client to not be connected initially")
	}

	if rc.GetSyncCount() != 0 {
		t.Errorf("expected 0 syncs, got %d", rc.GetSyncCount())
	}
}

func TestNewReplicationClient_EmptyAddress(t *testing.T) {
	_, err := NewReplicationClient("")
	if err == nil {
		t.Error("expected error for empty server address")
	}
}

func TestReplicationClient_Connect(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	ctx := context.Background()
	err := rc.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	if !rc.IsConnected() {
		t.Error("expected client to be connected")
	}

	if rc.GetSessionID() == "" {
		t.Error("expected session ID to be set")
	}
}

func TestReplicationClient_Connect_Closed(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	rc.Close()

	ctx := context.Background()
	err := rc.Connect(ctx)
	if err == nil {
		t.Error("expected error when connecting closed client")
	}
}

func TestReplicationClient_Disconnect(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	ctx := context.Background()
	rc.Connect(ctx)

	err := rc.Disconnect(ctx)
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}

	if rc.IsConnected() {
		t.Error("expected client to be disconnected")
	}

	if rc.GetSessionID() != "" {
		t.Error("expected session ID to be cleared")
	}
}

func TestReplicationClient_Sync(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	ctx := context.Background()
	rc.Connect(ctx)

	initialSyncCount := rc.GetSyncCount()

	err := rc.Sync(ctx)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if rc.GetSyncCount() != initialSyncCount+1 {
		t.Errorf("expected sync count to increase by 1")
	}
}

func TestReplicationClient_Sync_NotConnected(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	ctx := context.Background()
	err := rc.Sync(ctx)
	if err == nil {
		t.Error("expected error when syncing without connection")
	}
}

func TestReplicationClient_Sync_Closed(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	rc.Close()

	ctx := context.Background()
	err := rc.Sync(ctx)
	if err == nil {
		t.Error("expected error when syncing closed client")
	}
}

func TestReplicationClient_AutoSync(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	ctx := context.Background()
	rc.Connect(ctx)

	// Start auto-sync with short interval
	err := rc.StartAutoSync(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start auto-sync: %v", err)
	}

	if !rc.IsAutoSyncRunning() {
		t.Error("expected auto-sync to be running")
	}

	// Wait for some syncs
	time.Sleep(150 * time.Millisecond)

	if rc.GetSyncCount() < 1 {
		t.Error("expected at least 1 sync from auto-sync")
	}

	// Stop auto-sync
	err = rc.StopAutoSync()
	if err != nil {
		t.Fatalf("failed to stop auto-sync: %v", err)
	}

	if rc.IsAutoSyncRunning() {
		t.Error("expected auto-sync to be stopped")
	}
}

func TestReplicationClient_StartAutoSync_InvalidInterval(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	err := rc.StartAutoSync(0)
	if err == nil {
		t.Error("expected error for zero interval")
	}

	err = rc.StartAutoSync(-1 * time.Second)
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestReplicationClient_StartAutoSync_Closed(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	rc.Close()

	err := rc.StartAutoSync(1 * time.Second)
	if err == nil {
		t.Error("expected error when starting auto-sync on closed client")
	}
}

func TestReplicationClient_SetCurrentRevision(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	revision := &IndexRevision{
		Generation: 1,
		Version:    1,
		Files:      []string{"file1.txt"},
	}

	err := rc.SetCurrentRevision(revision)
	if err != nil {
		t.Fatalf("failed to set current revision: %v", err)
	}

	current := rc.GetCurrentRevision()
	if current == nil {
		t.Fatal("expected current revision to be set")
	}

	if current.Generation != 1 {
		t.Errorf("expected generation 1, got %d", current.Generation)
	}
}

func TestReplicationClient_SetCurrentRevision_Closed(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	rc.Close()

	revision := &IndexRevision{Generation: 1}
	err := rc.SetCurrentRevision(revision)
	if err == nil {
		t.Error("expected error when setting revision on closed client")
	}
}

func TestReplicationClient_PendingFiles(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	// Initially no pending files
	if rc.HasPendingFiles() {
		t.Error("expected no pending files initially")
	}

	// Add pending files
	rc.AddPendingFile("file1.txt")
	rc.AddPendingFile("file2.txt")

	if !rc.HasPendingFiles() {
		t.Error("expected pending files")
	}

	pending := rc.GetPendingFiles()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending files, got %d", len(pending))
	}

	// Remove a file
	rc.RemovePendingFile("file1.txt")

	pending = rc.GetPendingFiles()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending file, got %d", len(pending))
	}
}

func TestReplicationClient_GetLastSyncTime(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	// Should have initial sync time
	initialTime := rc.GetLastSyncTime()
	if initialTime.IsZero() {
		t.Error("expected initial sync time to be set")
	}

	// Connect and sync
	ctx := context.Background()
	rc.Connect(ctx)
	rc.Sync(ctx)

	newTime := rc.GetLastSyncTime()
	if !newTime.After(initialTime) {
		t.Error("expected new sync time to be after initial time")
	}
}

func TestReplicationClient_Close(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)

	err := rc.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if rc.IsOpen() {
		t.Error("expected client to be closed")
	}

	// Close again should not error
	err = rc.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestReplicationClient_Close_WithAutoSync(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)

	ctx := context.Background()
	rc.Connect(ctx)
	rc.StartAutoSync(100 * time.Millisecond)

	err := rc.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if rc.IsOpen() {
		t.Error("expected client to be closed")
	}

	if rc.IsAutoSyncRunning() {
		t.Error("expected auto-sync to be stopped")
	}
}

func TestReplicationClient_String(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	str := rc.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestReplicationClient_ConcurrentOperations(t *testing.T) {
	serverAddress := "localhost:8080"

	rc, _ := NewReplicationClient(serverAddress)
	defer rc.Close()

	done := make(chan bool, 4)

	// Sync goroutine
	go func() {
		for i := 0; i < 10; i++ {
			ctx := context.Background()
			rc.Sync(ctx)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get status goroutine
	go func() {
		for i := 0; i < 10; i++ {
			rc.GetSyncCount()
			rc.GetLastSyncTime()
			rc.IsConnected()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Pending files goroutine
	go func() {
		for i := 0; i < 10; i++ {
			rc.AddPendingFile("file.txt")
			rc.GetPendingFiles()
			rc.RemovePendingFile("file.txt")
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Revision goroutine
	go func() {
		for i := 0; i < 10; i++ {
			revision := &IndexRevision{Generation: int64(i)}
			rc.SetCurrentRevision(revision)
			rc.GetCurrentRevision()
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
