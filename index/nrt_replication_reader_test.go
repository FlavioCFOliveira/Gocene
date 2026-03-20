package index

import (
	"context"
	"testing"
	"time"
)

func TestNewNRTReplicationReader(t *testing.T) {
	rr, err := NewNRTReplicationReader("http://localhost:8080")
	if err != nil {
		t.Fatalf("failed to create NRTReplicationReader: %v", err)
	}
	defer rr.Close()

	if rr == nil {
		t.Fatal("expected NRTReplicationReader to not be nil")
	}

	if !rr.IsOpen() {
		t.Error("expected reader to be open")
	}

	if rr.GetSource() != "http://localhost:8080" {
		t.Errorf("expected source http://localhost:8080, got %s", rr.GetSource())
	}

	if rr.GetSyncCount() != 0 {
		t.Errorf("expected 0 syncs, got %d", rr.GetSyncCount())
	}
}

func TestNewNRTReplicationReader_EmptySource(t *testing.T) {
	_, err := NewNRTReplicationReader("")
	if err == nil {
		t.Error("expected error for empty source")
	}
}

func TestNRTReplicationReader_GetCurrentRevision(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	revision, err := rr.GetCurrentRevision()
	if err != nil {
		t.Fatalf("failed to get current revision: %v", err)
	}

	if revision == nil {
		t.Fatal("expected revision to not be nil")
	}
}

func TestNRTReplicationReader_GetCurrentRevision_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	_, err := rr.GetCurrentRevision()
	if err == nil {
		t.Error("expected error when reader is closed")
	}
}

func TestNRTReplicationReader_UpdateRevision(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	revision := &IndexRevision{
		Generation: 1,
		Version:    1,
		Timestamp:  time.Now(),
		Files:      []string{"file1.txt", "file2.txt"},
	}

	err := rr.UpdateRevision(revision)
	if err != nil {
		t.Fatalf("failed to update revision: %v", err)
	}

	if rr.GetSyncCount() != 1 {
		t.Errorf("expected 1 sync, got %d", rr.GetSyncCount())
	}

	current, _ := rr.GetCurrentRevision()
	if current.Generation != 1 {
		t.Errorf("expected generation 1, got %d", current.Generation)
	}
}

func TestNRTReplicationReader_UpdateRevision_Nil(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	err := rr.UpdateRevision(nil)
	if err == nil {
		t.Error("expected error for nil revision")
	}
}

func TestNRTReplicationReader_UpdateRevision_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	revision := &IndexRevision{Generation: 1}
	err := rr.UpdateRevision(revision)
	if err == nil {
		t.Error("expected error when updating revision on closed reader")
	}
}

func TestNRTReplicationReader_SetSource(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	err := rr.SetSource("http://localhost:9090")
	if err != nil {
		t.Fatalf("failed to set source: %v", err)
	}

	if rr.GetSource() != "http://localhost:9090" {
		t.Errorf("expected source http://localhost:9090, got %s", rr.GetSource())
	}
}

func TestNRTReplicationReader_SetSource_Empty(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	err := rr.SetSource("")
	if err == nil {
		t.Error("expected error for empty source")
	}
}

func TestNRTReplicationReader_SetSource_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	err := rr.SetSource("http://localhost:9090")
	if err == nil {
		t.Error("expected error when setting source on closed reader")
	}
}

func TestNRTReplicationReader_FileDownloadTracking(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	// Start download
	err := rr.StartFileDownload("file1.txt")
	if err != nil {
		t.Fatalf("failed to start file download: %v", err)
	}

	if !rr.IsFilePending("file1.txt") {
		t.Error("expected file1.txt to be pending")
	}

	pending := rr.GetPendingFiles()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending file, got %d", len(pending))
	}

	// Complete download
	err = rr.CompleteFileDownload("file1.txt")
	if err != nil {
		t.Fatalf("failed to complete file download: %v", err)
	}

	if rr.IsFilePending("file1.txt") {
		t.Error("expected file1.txt to not be pending")
	}

	if !rr.IsFileComplete("file1.txt") {
		t.Error("expected file1.txt to be complete")
	}

	completed := rr.GetCompletedFiles()
	if len(completed) != 1 {
		t.Errorf("expected 1 completed file, got %d", len(completed))
	}
}

func TestNRTReplicationReader_StartFileDownload_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	err := rr.StartFileDownload("file1.txt")
	if err == nil {
		t.Error("expected error when starting download on closed reader")
	}
}

func TestNRTReplicationReader_CompleteFileDownload_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	err := rr.CompleteFileDownload("file1.txt")
	if err == nil {
		t.Error("expected error when completing download on closed reader")
	}
}

func TestNRTReplicationReader_GetRequiredFiles(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	// Initially empty
	files, err := rr.GetRequiredFiles()
	if err != nil {
		t.Fatalf("failed to get required files: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 required files, got %d", len(files))
	}

	// Update revision with files
	revision := &IndexRevision{
		Generation: 1,
		Files:      []string{"file1.txt", "file2.txt"},
	}
	rr.UpdateRevision(revision)

	files, err = rr.GetRequiredFiles()
	if err != nil {
		t.Fatalf("failed to get required files: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 required files, got %d", len(files))
	}
}

func TestNRTReplicationReader_GetRequiredFiles_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	_, err := rr.GetRequiredFiles()
	if err == nil {
		t.Error("expected error when getting required files from closed reader")
	}
}

func TestNRTReplicationReader_IsSyncComplete(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	// Empty revision should be complete
	if !rr.IsSyncComplete() {
		t.Error("expected sync to be complete for empty revision")
	}

	// Update revision with files
	revision := &IndexRevision{
		Generation: 1,
		Files:      []string{"file1.txt", "file2.txt"},
	}
	rr.UpdateRevision(revision)

	// Should not be complete until files are downloaded
	if rr.IsSyncComplete() {
		t.Error("expected sync to not be complete")
	}

	// Complete file downloads
	rr.CompleteFileDownload("file1.txt")
	rr.CompleteFileDownload("file2.txt")

	if !rr.IsSyncComplete() {
		t.Error("expected sync to be complete after downloading all files")
	}
}

func TestNRTReplicationReader_IsSyncComplete_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	if rr.IsSyncComplete() {
		t.Error("expected sync to not be complete when closed")
	}
}

func TestNRTReplicationReader_Sync(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	initialSyncCount := rr.GetSyncCount()

	ctx := context.Background()
	err := rr.Sync(ctx)
	if err != nil {
		t.Fatalf("failed to sync: %v", err)
	}

	if rr.GetSyncCount() != initialSyncCount+1 {
		t.Errorf("expected sync count to increase by 1")
	}
}

func TestNRTReplicationReader_Sync_Closed(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	rr.Close()

	ctx := context.Background()
	err := rr.Sync(ctx)
	if err == nil {
		t.Error("expected error when syncing closed reader")
	}
}

func TestNRTReplicationReader_Close(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")

	err := rr.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if rr.IsOpen() {
		t.Error("expected reader to be closed")
	}

	// Close again should not error
	err = rr.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestNRTReplicationReader_GetLastSyncTime(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	// Should have initial sync time
	initialTime := rr.GetLastSyncTime()
	if initialTime.IsZero() {
		t.Error("expected initial sync time to be set")
	}

	// Sync
	time.Sleep(10 * time.Millisecond)
	ctx := context.Background()
	rr.Sync(ctx)

	newTime := rr.GetLastSyncTime()
	if !newTime.After(initialTime) {
		t.Error("expected new sync time to be after initial time")
	}
}

func TestNRTReplicationReader_String(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	str := rr.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestNRTReplicationReader_ConcurrentOperations(t *testing.T) {
	rr, _ := NewNRTReplicationReader("http://localhost:8080")
	defer rr.Close()

	done := make(chan bool, 4)

	// Update revision concurrently
	go func() {
		for i := 0; i < 10; i++ {
			revision := &IndexRevision{
				Generation: int64(i),
				Files:      []string{"file1.txt", "file2.txt"},
			}
			rr.UpdateRevision(revision)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get revision concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rr.GetCurrentRevision()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// File operations concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rr.StartFileDownload("file1.txt")
			rr.CompleteFileDownload("file1.txt")
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get status concurrently
	go func() {
		for i := 0; i < 10; i++ {
			rr.GetPendingFiles()
			rr.GetCompletedFiles()
			rr.IsSyncComplete()
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
