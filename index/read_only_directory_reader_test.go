package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTestDirectoryReader creates a minimal DirectoryReader for testing
func createROTestDirectoryReader(t *testing.T) *DirectoryReader {
	dir := store.NewByteBuffersDirectory()
	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to create directory reader: %v", err)
	}
	return reader
}

func TestNewReadOnlyDirectoryReader(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	if roReader == nil {
		t.Fatal("expected ReadOnlyDirectoryReader to not be nil")
	}

	if roReader.GetDelegate() != reader {
		t.Error("expected delegate to be set")
	}
}

func TestNewReadOnlyDirectoryReader_Nil(t *testing.T) {
	_, err := NewReadOnlyDirectoryReader(nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestReadOnlyDirectoryReader_Close(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}

	err = roReader.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !roReader.IsClosed() {
		t.Error("expected reader to be closed")
	}

	// Close again should not error
	err = roReader.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestReadOnlyDirectoryReader_EnsureOpen(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	// Should be open
	if err := roReader.EnsureOpen(); err != nil {
		t.Errorf("expected reader to be open: %v", err)
	}

	// Close it
	roReader.Close()

	// Should be closed
	if err := roReader.EnsureOpen(); err == nil {
		t.Error("expected error when reader is closed")
	}
}

func TestReadOnlyDirectoryReader_Clone(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	cloned, err := roReader.Clone()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	defer cloned.Close()

	if cloned.GetDelegate() != reader {
		t.Error("expected cloned to have same delegate")
	}
}

func TestReadOnlyDirectoryReader_Clone_Closed(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}

	roReader.Close()

	_, err = roReader.Clone()
	if err == nil {
		t.Error("expected error when cloning closed reader")
	}
}

func TestReadOnlyDirectoryReader_Reopen(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	// Reopen
	newReader, err := roReader.Reopen()
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer newReader.Close()

	if newReader == nil {
		t.Error("expected new reader")
	}
}

func TestReadOnlyDirectoryReader_Reopen_Closed(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}

	roReader.Close()

	_, err = roReader.Reopen()
	if err == nil {
		t.Error("expected error when reopening closed reader")
	}
}

func TestReadOnlyDirectoryReader_NumDocs(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	// Should return 0 for empty reader
	_ = roReader.NumDocs()

	// Close and check
	roReader.Close()

	if roReader.NumDocs() != 0 {
		t.Error("expected 0 docs when closed")
	}
}

func TestReadOnlyDirectoryReader_MaxDoc(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	// Should return 0 for empty reader
	_ = roReader.MaxDoc()

	// Close and check
	roReader.Close()

	if roReader.MaxDoc() != 0 {
		t.Error("expected maxDoc 0 when closed")
	}
}

func TestReadOnlyDirectoryReader_HasDeletions(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	// Should return false for empty reader
	_ = roReader.HasDeletions()

	// Close and check
	roReader.Close()

	if roReader.HasDeletions() {
		t.Error("expected no deletions when closed")
	}
}

func TestReadOnlyDirectoryReader_IsCurrent(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	isCurrent, err := roReader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent failed: %v", err)
	}

	// Empty reader should be current
	if !isCurrent {
		t.Error("expected empty reader to be current")
	}
}

func TestReadOnlyDirectoryReader_IsCurrent_Closed(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}

	roReader.Close()

	_, err = roReader.IsCurrent()
	if err == nil {
		t.Error("expected error when reader is closed")
	}
}

func TestReadOnlyDirectoryReader_GetDirectory(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	dir := roReader.GetDirectory()
	// May be nil for empty reader
	_ = dir

	// Close and check
	roReader.Close()

	if roReader.GetDirectory() != nil {
		t.Error("expected nil directory when closed")
	}
}

func TestReadOnlyDirectoryReader_String(t *testing.T) {
	reader := createROTestDirectoryReader(t)

	roReader, err := NewReadOnlyDirectoryReader(reader)
	if err != nil {
		t.Fatalf("failed to create ReadOnlyDirectoryReader: %v", err)
	}
	defer roReader.Close()

	str := roReader.String()
	if str == "" {
		t.Error("expected non-empty string")
	}

	// Close and check
	roReader.Close()

	str = roReader.String()
	if str != "ReadOnlyDirectoryReader(closed)" {
		t.Errorf("expected closed string, got: %s", str)
	}
}
