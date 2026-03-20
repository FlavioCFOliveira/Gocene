package index

import (
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewNRTReader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open directory reader: %v", err)
	}

	nrtReader, err := NewNRTReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader == nil {
		t.Fatal("expected NRTReader to not be nil")
	}

	if !nrtReader.IsOpen() {
		t.Error("expected reader to be open")
	}

	if nrtReader.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", nrtReader.GetVersion())
	}

	if nrtReader.GetRefreshCount() != 0 {
		t.Errorf("expected 0 refreshes, got %d", nrtReader.GetRefreshCount())
	}
}

func TestNewNRTReader_Nil(t *testing.T) {
	_, err := NewNRTReader(nil, nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestNRTReader_Refresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	initialVersion := nrtReader.GetVersion()
	initialRefreshCount := nrtReader.GetRefreshCount()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	err := nrtReader.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if nrtReader.GetVersion() != initialVersion+1 {
		t.Errorf("expected version %d, got %d", initialVersion+1, nrtReader.GetVersion())
	}

	if nrtReader.GetRefreshCount() != initialRefreshCount+1 {
		t.Errorf("expected refresh count %d, got %d", initialRefreshCount+1, nrtReader.GetRefreshCount())
	}
}

func TestNRTReader_Refresh_Closed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	nrtReader.Close()

	err := nrtReader.Refresh()
	if err == nil {
		t.Error("expected error when refreshing closed reader")
	}
}

func TestNRTReader_IncrementVersion(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	initialVersion := nrtReader.GetVersion()

	nrtReader.IncrementVersion()

	if nrtReader.GetVersion() != initialVersion+1 {
		t.Errorf("expected version %d, got %d", initialVersion+1, nrtReader.GetVersion())
	}
}

func TestNRTReader_GetLastRefreshTime(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	initialTime := nrtReader.GetLastRefreshTime()
	if initialTime.IsZero() {
		t.Error("expected initial refresh time to be set")
	}

	// Refresh and check
	time.Sleep(10 * time.Millisecond)
	nrtReader.Refresh()

	newTime := nrtReader.GetLastRefreshTime()
	if !newTime.After(initialTime) {
		t.Error("expected new refresh time to be after initial time")
	}
}

func TestNRTReader_NumDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	// Should return 0 for empty reader
	_ = nrtReader.NumDocs()

	// Close and check
	nrtReader.Close()

	if nrtReader.NumDocs() != 0 {
		t.Error("expected 0 docs when closed")
	}
}

func TestNRTReader_MaxDoc(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	// Should return 0 for empty reader
	_ = nrtReader.MaxDoc()

	// Close and check
	nrtReader.Close()

	if nrtReader.MaxDoc() != 0 {
		t.Error("expected maxDoc 0 when closed")
	}
}

func TestNRTReader_HasDeletions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	// Should return false for empty reader
	_ = nrtReader.HasDeletions()

	// Close and check
	nrtReader.Close()

	if nrtReader.HasDeletions() {
		t.Error("expected no deletions when closed")
	}
}

func TestNRTReader_GetWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	writer := &IndexWriter{}
	nrtReader, _ := NewNRTReader(reader, writer)
	defer nrtReader.Close()

	if nrtReader.GetWriter() != writer {
		t.Error("expected writer to be set")
	}
}

func TestNRTReader_NRTSegmentReaders(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	// Initially empty
	segments := nrtReader.GetNRTSegmentReaders()
	if len(segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segments))
	}

	// Add segment reader
	segmentReader := &NRTSegmentReader{}
	nrtReader.AddNRTSegmentReader(segmentReader)

	segments = nrtReader.GetNRTSegmentReaders()
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
}

func TestNRTReader_Close(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)

	err := nrtReader.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if nrtReader.IsOpen() {
		t.Error("expected reader to be closed")
	}

	// Close again should not error
	err = nrtReader.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestNRTReader_String(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	reader, _ := OpenDirectoryReader(dir)
	nrtReader, _ := NewNRTReader(reader, nil)
	defer nrtReader.Close()

	str := nrtReader.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}
