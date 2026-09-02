package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestNRTDeleteVisibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	conf := NewIndexWriterConfig()
	writer, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// 1. Index a document
	doc := []IndexableField{
		TextField("text", "hello nrt visibility"),
	}
	_, err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("failed to add doc: %v", err)
	}

	// Ensure it's flushed to a segment
	writer.docWriter.FlushAllThreads()

	// 2. Open an existing NRT reader
	reader1, err := writer.GetReader(false)
	if err != nil {
		t.Fatalf("failed to open reader1: %v", err)
	}
	defer reader1.Close()

	// Verify doc is visible
	searcher1 := IndexSearcher{reader: reader1}
	hits1 := searcher1.Count(MatchAllDocsQuery{})
	if hits1 != 1 {
		t.Errorf("expected 1 hit in reader1, got %d", hits1)
	}

	// 3. Delete the document via IndexWriter
	// We need the docID. In a single-segment index, it's 0.
	deleted, err := writer.TryDeleteDocument(0)
	if err != nil {
		t.Fatalf("failed to delete doc: %v", err)
	}
	if !deleted {
		t.Fatal("expected doc to be deleted")
	}

	// 4. Verify it's STILL visible to reader1
	hits1After := searcher1.Count(MatchAllDocsQuery{})
	if hits1After != 1 {
		t.Errorf("expected doc to remain visible in reader1 after delete, got %d", hits1After)
	}

	// 5. Open a new NRT reader and verify doc is INVISIBLE
	reader2, err := writer.GetReader(false)
	if err != nil {
		t.Fatalf("failed to open reader2: %v", err)
	}
	defer reader2.Close()

	searcher2 := IndexSearcher{reader: reader2}
	hits2 := searcher2.Count(MatchAllDocsQuery{})
	if hits2 != 0 {
		t.Errorf("expected 0 hits in reader2 after delete, got %d", hits2)
	}
}
