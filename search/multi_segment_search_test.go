package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// setupMultiSegmentIndex creates an index with 3 segments (via 3 commits).
func setupMultiSegmentIndex(t *testing.T) (*index.IndexWriter, store.Directory, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	iwc := index.NewIndexWriterConfig(nil)
	iwc.SetMaxBufferedDocs(1)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewStringField("id", string(rune('a'+i)), true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		if i%2 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
	}
	return writer, dir, func() {
		writer.Close()
		dir.Close()
	}
}

func TestMultiSegment_ReaderHasSubReaders(t *testing.T) {
	writer, dir, cleanup := setupMultiSegmentIndex(t)
	defer cleanup()
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	subs := reader.GetSequentialSubReaders()
	if len(subs) < 2 {
		t.Fatalf("expected >= 2 sub-readers, got %d", len(subs))
	}
}

func TestMultiSegment_MaxDocAcrossSegments(t *testing.T) {
	writer, dir, cleanup := setupMultiSegmentIndex(t)
	defer cleanup()
	writer.Commit()
	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()
	if reader.MaxDoc() < 5 {
		t.Fatalf("MaxDoc=%d, want >= 5", reader.MaxDoc())
	}
}

func TestMultiSegment_NumDocsAcrossSegments(t *testing.T) {
	writer, dir, cleanup := setupMultiSegmentIndex(t)
	defer cleanup()
	writer.Commit()
	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()
	if reader.NumDocs() != 5 {
		t.Fatalf("NumDocs=%d, want 5", reader.NumDocs())
	}

}
func TestMultiSegment_SubReaderDocBase(t *testing.T) {
	writer, dir, cleanup := setupMultiSegmentIndex(t)
	defer cleanup()
	writer.Commit()
	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()
	subs := reader.GetSequentialSubReaders()
	total := 0
	for _, sub := range subs {
		total += sub.MaxDoc()
	}
	if total != reader.MaxDoc() {
		t.Fatalf("sum(sub.MaxDoc)=%d, reader.MaxDoc=%d", total, reader.MaxDoc())
	}
}