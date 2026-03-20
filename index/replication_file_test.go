package index

import (
	"errors"
	"testing"
	"time"
)

func TestNewReplicationFile(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)

	if file == nil {
		t.Fatal("expected file to not be nil")
	}

	if file.Name != "test.txt" {
		t.Errorf("expected name 'test.txt', got %s", file.Name)
	}

	if file.SourcePath != "/source/test.txt" {
		t.Errorf("expected source path '/source/test.txt', got %s", file.SourcePath)
	}

	if file.TargetPath != "/target/test.txt" {
		t.Errorf("expected target path '/target/test.txt', got %s", file.TargetPath)
	}

	if file.Size != 1024 {
		t.Errorf("expected size 1024, got %d", file.Size)
	}

	if file.GetStatus() != ReplicationFileStatusPending {
		t.Errorf("expected status PENDING, got %s", file.GetStatus().String())
	}

	if file.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", file.MaxRetries)
	}
}

func TestReplicationFileStatus_String(t *testing.T) {
	tests := []struct {
		status   ReplicationFileStatus
		expected string
	}{
		{ReplicationFileStatusPending, "PENDING"},
		{ReplicationFileStatusTransferring, "TRANSFERRING"},
		{ReplicationFileStatusCompleted, "COMPLETED"},
		{ReplicationFileStatusFailed, "FAILED"},
		{ReplicationFileStatusSkipped, "SKIPPED"},
		{ReplicationFileStatus(99), "UNKNOWN"},
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("expected %s, got %s", test.expected, test.status.String())
		}
	}
}

func TestReplicationFile_Start(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)

	err := file.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if file.GetStatus() != ReplicationFileStatusTransferring {
		t.Errorf("expected status TRANSFERRING, got %s", file.GetStatus().String())
	}

	if file.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestReplicationFile_Start_NotPending(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)
	file.Start()

	err := file.Start()
	if err == nil {
		t.Error("expected error when starting non-pending file")
	}
}

func TestReplicationFile_Complete(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)
	file.Start()

	err := file.Complete()
	if err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	if file.GetStatus() != ReplicationFileStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", file.GetStatus().String())
	}

	if file.TransferredBytes != file.Size {
		t.Errorf("expected TransferredBytes to equal Size")
	}
}

func TestReplicationFile_Complete_NotTransferring(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)

	err := file.Complete()
	if err == nil {
		t.Error("expected error when completing non-transferring file")
	}
}

func TestReplicationFile_Fail(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)
	file.Start()

	expectedErr := errors.New("transfer failed")
	file.Fail(expectedErr)

	if file.GetStatus() != ReplicationFileStatusFailed {
		t.Errorf("expected status FAILED, got %s", file.GetStatus().String())
	}

	if file.Error != expectedErr.Error() {
		t.Errorf("expected error '%s', got '%s'", expectedErr.Error(), file.Error)
	}
}

func TestReplicationFile_Skip(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)

	file.Skip()

	if file.GetStatus() != ReplicationFileStatusSkipped {
		t.Errorf("expected status SKIPPED, got %s", file.GetStatus().String())
	}
}

func TestReplicationFile_Cancel(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1024)

	file.Cancel()

	if !file.IsCancelled() {
		t.Error("expected IsCancelled to be true")
	}
}

func TestReplicationFile_UpdateProgress(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	file.Start()

	file.UpdateProgress(500)

	if file.TransferredBytes != 500 {
		t.Errorf("expected 500 transferred bytes, got %d", file.TransferredBytes)
	}

	if file.GetProgress() != 50 {
		t.Errorf("expected 50%% progress, got %d%%", file.GetProgress())
	}
}

func TestReplicationFile_AddTransferredBytes(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	file.Start()

	file.AddTransferredBytes(300)
	file.AddTransferredBytes(400)

	if file.TransferredBytes != 700 {
		t.Errorf("expected 700 transferred bytes, got %d", file.TransferredBytes)
	}

	// Test overflow protection
	file.AddTransferredBytes(500)
	if file.TransferredBytes != 1000 {
		t.Errorf("expected 1000 transferred bytes (capped), got %d", file.TransferredBytes)
	}
}

func TestReplicationFile_GetProgress(t *testing.T) {
	// Normal case
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	file.UpdateProgress(500)

	if file.GetProgress() != 50 {
		t.Errorf("expected 50%% progress, got %d%%", file.GetProgress())
	}

	// Zero size
	emptyFile := NewReplicationFile("empty.txt", "/source/empty.txt", "/target/empty.txt", 0)
	if emptyFile.GetProgress() != 100 {
		t.Errorf("expected 100%% progress for empty file, got %d%%", emptyFile.GetProgress())
	}
}

func TestReplicationFile_GetTransferRate(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 10000)
	file.Start()

	// Should be 0 immediately after start
	rate := file.GetTransferRate()
	if rate != 0 {
		t.Errorf("expected 0 rate immediately after start, got %f", rate)
	}

	// Simulate transfer
	time.Sleep(10 * time.Millisecond)
	file.AddTransferredBytes(1000)

	rate = file.GetTransferRate()
	if rate <= 0 {
		t.Error("expected positive transfer rate")
	}
}

func TestReplicationFile_GetDuration(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)

	// Pending - should be 0
	if file.GetDuration() != 0 {
		t.Error("expected 0 duration for pending file")
	}

	// Transferring
	file.Start()
	time.Sleep(10 * time.Millisecond)

	duration := file.GetDuration()
	if duration <= 0 {
		t.Error("expected positive duration for transferring file")
	}

	// Completed
	file.Complete()
	duration = file.GetDuration()
	if duration <= 0 {
		t.Error("expected positive duration for completed file")
	}
}

func TestReplicationFile_GetTotalDuration(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	time.Sleep(10 * time.Millisecond)

	duration := file.GetTotalDuration()
	if duration <= 0 {
		t.Error("expected positive total duration")
	}
}

func TestReplicationFile_Retry(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)

	// Initially can retry
	if !file.CanRetry() {
		t.Error("expected CanRetry to be true initially")
	}

	// Increment retry count
	file.IncrementRetryCount()
	file.IncrementRetryCount()
	file.IncrementRetryCount()

	if file.RetryCount != 3 {
		t.Errorf("expected retry count 3, got %d", file.RetryCount)
	}

	if file.CanRetry() {
		t.Error("expected CanRetry to be false after max retries")
	}
}

func TestReplicationFile_ResetForRetry(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)
	file.Start()
	file.AddTransferredBytes(500)
	file.Fail(errors.New("transfer failed"))

	file.ResetForRetry()

	if file.GetStatus() != ReplicationFileStatusPending {
		t.Errorf("expected status PENDING, got %s", file.GetStatus().String())
	}

	if file.TransferredBytes != 0 {
		t.Errorf("expected 0 transferred bytes, got %d", file.TransferredBytes)
	}

	if file.Error != "" {
		t.Error("expected error to be cleared")
	}
}

func TestReplicationFile_Checksum(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)

	// No checksum set - should verify as true
	if !file.VerifyChecksum() {
		t.Error("expected VerifyChecksum to return true when no checksum set")
	}

	// Set checksum
	file.SetChecksum(12345)
	if file.Checksum != 12345 {
		t.Errorf("expected checksum 12345, got %d", file.Checksum)
	}

	// Should fail verification if actual doesn't match
	if file.VerifyChecksum() {
		t.Error("expected VerifyChecksum to return false when checksums don't match")
	}

	// Set actual checksum
	file.SetActualChecksum(12345)
	if file.ActualChecksum != 12345 {
		t.Errorf("expected actual checksum 12345, got %d", file.ActualChecksum)
	}

	if !file.VerifyChecksum() {
		t.Error("expected VerifyChecksum to return true when checksums match")
	}
}

func TestCalculateCRC32(t *testing.T) {
	data := []byte("test data")
	checksum1 := CalculateCRC32(data)
	checksum2 := CalculateCRC32(data)

	if checksum1 != checksum2 {
		t.Error("expected same data to produce same checksum")
	}

	// Different data should produce different checksum
	otherData := []byte("other data")
	otherChecksum := CalculateCRC32(otherData)

	if checksum1 == otherChecksum {
		t.Error("expected different data to produce different checksum")
	}
}

func TestReplicationFile_String(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 1000)

	str := file.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestReplicationFile_ConcurrentOperations(t *testing.T) {
	file := NewReplicationFile("test.txt", "/source/test.txt", "/target/test.txt", 10000)
	file.Start()

	done := make(chan bool, 4)

	// Update progress goroutine
	go func() {
		for i := 0; i < 100; i++ {
			file.AddTransferredBytes(100)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Get status goroutine
	go func() {
		for i := 0; i < 100; i++ {
			file.GetStatus()
			file.GetProgress()
			file.TransferredBytes
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Get duration goroutine
	go func() {
		for i := 0; i < 100; i++ {
			file.GetDuration()
			file.GetTotalDuration()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Get transfer rate goroutine
	go func() {
		for i := 0; i < 100; i++ {
			file.GetTransferRate()
			time.Sleep(1 * time.Millisecond)
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
