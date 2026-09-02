package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestIndexWriter_Rollback(t *testing.T) {
	dir := util.NewByteBuffersDirectory()
	conf := &IndexWriterConfig{
		LiveIndexWriterConfig: spi.DefaultLiveIndexWriterConfig(),
	}

	iw, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to create IndexWriter: %v", err)
	}

	// 1. Add documents and commit
	doc1 := []IndexableField{{Name: "text", Value: "first commit"}}
	if _, err := iw.AddDocument(doc1); err != nil {
		t.Fatalf("failed to add doc: %v", err)
	}
	if _, err := iw.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// 2. Add more documents without committing
	doc2 := []IndexableField{{Name: "text", Value: "should be rolled back"}}
	if _, err := iw.AddDocument(doc2); err != nil {
		t.Fatalf("failed to add doc: %v", err)
	}

	// 3. Perform rollback
	if err := iw.Rollback(); err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	// 4. Verify writer is closed
	if !iw.IsClosed() {
		t.Error("expected IndexWriter to be closed after rollback")
	}

	// 5. Verify the index state
	// Open a reader to check the contents
	reader, err := StandardDirectoryReader.Open(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	searcher := NewIndexSearcher(reader)
	
	// We expect only doc1 to be present
	if searcher.NumDocs() != 1 {
		t.Errorf("expected 1 doc, got %d", searcher.NumDocs())
	}

	// 6. Verify that write.lock is released
	// Try to open a new IndexWriter on the same directory
	iw2, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to open new IndexWriter after rollback: %v", err)
	}
	iw2.Close()
}
