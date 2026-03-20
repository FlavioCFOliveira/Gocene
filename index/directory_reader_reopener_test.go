package index

import (
	"context"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTestDirectoryReader creates a minimal DirectoryReader for testing
func createTestDirectoryReader(t *testing.T) *DirectoryReader {
	dir := store.NewByteBuffersDirectory()
	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		// If we can't open (no segments), create a minimal reader
		// by using a basic DirectoryReader with the directory
		t.Fatalf("failed to create directory reader: %v", err)
	}
	return reader
}

func TestNewDirectoryReaderReopener(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	if reopener == nil {
		t.Fatal("expected reopener to not be nil")
	}

	if reopener.GetCurrent() != reader {
		t.Error("expected current reader to be set")
	}

	if reopener.GetApplyAllDeletes() != true {
		t.Error("expected applyAllDeletes to be true by default")
	}
}

func TestNewDirectoryReaderReopener_Nil(t *testing.T) {
	_, err := NewDirectoryReaderReopener(nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestNewDirectoryReaderReopenerWithWriter(t *testing.T) {
	reader := createTestDirectoryReader(t)
	writer := &IndexWriter{}

	reopener, err := NewDirectoryReaderReopenerWithWriter(reader, writer)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	if !reopener.HasWriter() {
		t.Error("expected reopener to have writer")
	}

	if reopener.GetWriter() != writer {
		t.Error("expected writer to be set")
	}
}

func TestDirectoryReaderReopener_MaybeReopen(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	ctx := context.Background()
	result, err := reopener.MaybeReopen(ctx)
	if err != nil {
		t.Fatalf("failed to maybe reopen: %v", err)
	}

	// In non-NRT mode with no changes, should return no changes
	if result.HasChanges {
		t.Error("expected no changes in non-NRT mode")
	}
}

func TestDirectoryReaderReopener_MaybeReopen_Closed(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}

	reopener.Close()

	_, err = reopener.MaybeReopen(context.Background())
	if err == nil {
		t.Error("expected error when reopening closed reopener")
	}
}

func TestDirectoryReaderReopener_Reopen(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	ctx := context.Background()
	newReader, err := reopener.Reopen(ctx)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}

	if newReader == nil {
		t.Error("expected new reader")
	}
}

func TestDirectoryReaderReopener_Reopen_Closed(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}

	reopener.Close()

	_, err = reopener.Reopen(context.Background())
	if err == nil {
		t.Error("expected error when reopening closed reopener")
	}
}

func TestDirectoryReaderReopener_GetCurrent(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	current := reopener.GetCurrent()
	if current != reader {
		t.Error("expected GetCurrent to return initial reader")
	}
}

func TestDirectoryReaderReopener_Close(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}

	err = reopener.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !reopener.IsClosed() {
		t.Error("expected reopener to be closed")
	}

	// Close again should not error
	err = reopener.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestDirectoryReaderReopener_SetApplyAllDeletes(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	// Default should be true
	if !reopener.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be true by default")
	}

	// Set to false
	reopener.SetApplyAllDeletes(false)
	if reopener.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be false")
	}
}

func TestDirectoryReaderReopener_SetMinReopenInterval(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	// Default should be 100ms
	if reopener.GetMinReopenInterval() != 100*time.Millisecond {
		t.Errorf("expected default 100ms, got %v", reopener.GetMinReopenInterval())
	}

	// Set to 500ms
	reopener.SetMinReopenInterval(500 * time.Millisecond)
	if reopener.GetMinReopenInterval() != 500*time.Millisecond {
		t.Errorf("expected 500ms, got %v", reopener.GetMinReopenInterval())
	}
}

// mockReopenListener is a mock implementation of ReopenListener for testing
type mockReopenListener struct {
	onReopenCalled      bool
	onReopenErrorCalled bool
	lastOldReader       *DirectoryReader
	lastNewReader       *DirectoryReader
	lastError           error
}

func (m *mockReopenListener) OnReopen(oldReader, newReader *DirectoryReader) {
	m.onReopenCalled = true
	m.lastOldReader = oldReader
	m.lastNewReader = newReader
}

func (m *mockReopenListener) OnReopenError(err error) {
	m.onReopenErrorCalled = true
	m.lastError = err
}

func TestDirectoryReaderReopener_AddReopenListener(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	listener := &mockReopenListener{}
	reopener.AddReopenListener(listener)

	// Trigger a reopen
	reopener.Reopen(context.Background())

	if !listener.onReopenCalled {
		t.Error("expected OnReopen to be called")
	}
}

func TestDirectoryReaderReopener_RemoveReopenListener(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	listener := &mockReopenListener{}
	reopener.AddReopenListener(listener)
	reopener.RemoveReopenListener(listener)

	// Trigger a reopen
	reopener.Reopen(context.Background())

	if listener.onReopenCalled {
		t.Error("expected OnReopen to NOT be called after removal")
	}
}

func TestDirectoryReaderReopener_SetCommitUserData(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	reopener.SetCommitUserData(data)

	result := reopener.GetCommitUserData()
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
	if result["key1"] != "value1" {
		t.Error("expected key1 to be value1")
	}

	// Modify original map - should not affect stored data
	data["key3"] = "value3"
	result2 := reopener.GetCommitUserData()
	if _, ok := result2["key3"]; ok {
		t.Error("expected stored data to be a copy")
	}
}

func TestDirectoryReaderReopener_HasWriter(t *testing.T) {
	reader := createTestDirectoryReader(t)

	// Without writer
	reopener1, _ := NewDirectoryReaderReopener(reader)
	defer reopener1.Close()

	if reopener1.HasWriter() {
		t.Error("expected no writer")
	}

	// With writer
	reopener2, _ := NewDirectoryReaderReopenerWithWriter(reader, &IndexWriter{})
	defer reopener2.Close()

	if !reopener2.HasWriter() {
		t.Error("expected to have writer")
	}
}

func TestDirectoryReaderReopener_GetLastReopenTime(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	// Initially should be zero time
	if !reopener.GetLastReopenTime().IsZero() {
		t.Error("expected last reopen time to be zero initially")
	}

	// Trigger reopen
	reopener.Reopen(context.Background())

	// Should now have a time set
	if reopener.GetLastReopenTime().IsZero() {
		t.Error("expected last reopen time to be set after reopen")
	}
}

func TestDirectoryReaderReopener_MinReopenInterval(t *testing.T) {
	reader := createTestDirectoryReader(t)

	reopener, err := NewDirectoryReaderReopener(reader)
	if err != nil {
		t.Fatalf("failed to create reopener: %v", err)
	}
	defer reopener.Close()

	// Set a long interval
	reopener.SetMinReopenInterval(1 * time.Hour)

	// Trigger a reopen
	reopener.Reopen(context.Background())

	// Try to reopen again immediately - should be rate limited
	result, err := reopener.MaybeReopen(context.Background())
	if err != nil {
		t.Fatalf("maybe reopen failed: %v", err)
	}

	if result.HasChanges {
		t.Error("expected no changes due to rate limiting")
	}
}
