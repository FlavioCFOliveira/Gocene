package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTestDirectoryReader creates a minimal DirectoryReader for testing
func createNRTTestDirectoryReader(t *testing.T) *DirectoryReader {
	dir := store.NewByteBuffersDirectory()
	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to create directory reader: %v", err)
	}
	return reader
}

func TestNewNRTDirectoryReader(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader == nil {
		t.Fatal("expected NRTDirectoryReader to not be nil")
	}

	if !nrtReader.IsNRT() {
		t.Error("expected reader to be NRT")
	}

	if nrtReader.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", nrtReader.GetVersion())
	}
}

func TestNewNRTDirectoryReader_Nil(t *testing.T) {
	_, err := NewNRTDirectoryReader(nil, nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestNRTDirectoryReader_GetWriter(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	// Without writer
	nrtReader1, _ := NewNRTDirectoryReader(reader, nil)
	defer nrtReader1.Close()

	if nrtReader1.GetWriter() != nil {
		t.Error("expected nil writer")
	}

	// With writer
	writer := &IndexWriter{}
	nrtReader2, _ := NewNRTDirectoryReader(reader, writer)
	defer nrtReader2.Close()

	if nrtReader2.GetWriter() != writer {
		t.Error("expected writer to be set")
	}
}

func TestNRTDirectoryReader_Version(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", nrtReader.GetVersion())
	}

	nrtReader.IncrementVersion()

	if nrtReader.GetVersion() != 2 {
		t.Errorf("expected version 2, got %d", nrtReader.GetVersion())
	}
}

func TestNRTDirectoryReader_NRTSegmentReaders(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// Initially empty
	segments := nrtReader.GetNRTSegmentReaders()
	if len(segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segments))
	}

	// Add segments
	segment1 := &NRTSegmentReader{}
	segment2 := &NRTSegmentReader{}

	nrtReader.AddNRTSegmentReader(segment1)
	nrtReader.AddNRTSegmentReader(segment2)

	segments = nrtReader.GetNRTSegmentReaders()
	if len(segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(segments))
	}

	// Remove segment
	nrtReader.RemoveNRTSegmentReader(segment1)

	segments = nrtReader.GetNRTSegmentReaders()
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
}

func TestNRTDirectoryReader_IsCurrent(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// NRT reader with writer should not be current
	isCurrent, err := nrtReader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent failed: %v", err)
	}

	// Without writer, should be current
	if !isCurrent {
		t.Error("expected reader to be current when no writer")
	}
}

func TestNRTDirectoryReader_ApplyAllDeletes(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// Default should be true
	if !nrtReader.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be true by default")
	}

	// Set to false
	nrtReader.SetApplyAllDeletes(false)
	if nrtReader.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be false")
	}
}

func TestNRTDirectoryReader_Refresh(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// Refresh without writer should fail
	_, err = nrtReader.Refresh()
	if err == nil {
		t.Error("expected error when refreshing without writer")
	}
}

func TestNRTDirectoryReader_Close(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}

	err = nrtReader.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if nrtReader.IsNRT() {
		t.Error("expected reader to not be NRT after close")
	}
}

func TestNRTDirectoryReader_Clone(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// Add some segments
	segment1 := &NRTSegmentReader{}
	nrtReader.AddNRTSegmentReader(segment1)

	// Clone
	cloned, err := nrtReader.Clone()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	defer cloned.Close()

	if !cloned.IsNRT() {
		t.Error("expected cloned to be NRT")
	}

	if cloned.GetVersion() != nrtReader.GetVersion() {
		t.Error("expected cloned to have same version")
	}

	// Should have same segments
	if len(cloned.GetNRTSegmentReaders()) != 1 {
		t.Error("expected cloned to have 1 segment")
	}
}

func TestNRTDirectoryReader_ForEachNRTSegment(t *testing.T) {
	reader := createNRTTestDirectoryReader(t)

	nrtReader, err := NewNRTDirectoryReader(reader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTDirectoryReader: %v", err)
	}
	defer nrtReader.Close()

	// Add segments
	segment1 := &NRTSegmentReader{}
	segment2 := &NRTSegmentReader{}
	nrtReader.AddNRTSegmentReader(segment1)
	nrtReader.AddNRTSegmentReader(segment2)

	// Iterate
	count := 0
	err = nrtReader.ForEachNRTSegment(func(sr *NRTSegmentReader) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("foreach error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 segments, got %d", count)
	}
}
