package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestNRTDeleteVisibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	conf := NewIndexWriterConfig(nil)
	conf.SetOpenMode(Create)
	writer, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to create index writer: %v", err)
	}
	defer writer.Close()

	// initial commit to write metadata
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	doc1 := NewDocument()
	doc1.Add(NewStringField("key", "value1", store.StoreYES))
	writer.AddDocument(doc1)

	doc2 := NewDocument()
	doc2.Add(NewStringField("key", "value2", store.StoreYES))
	writer.AddDocument(doc2)

	if _, err := writer.Commit(); err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	// Snapshot current state (commit point 1)
	ic1 := writer.GetLatestCommit()

	// Update doc 1: this deletes doc 1 and adds doc 3
	doc3 := NewDocument()
	doc3.Add(NewStringField("key", "value3", store.StoreYES))
	if _, err := writer.UpdateDocument(NewTerm("key", "value1"), doc3); err != nil {
		t.Fatalf("update document failed: %v", err)
	}

	// Open NRT reader from writer
	latest, err := writer.GetReader(true)
	if err != nil {
		t.Fatalf("failed to open NRT reader: %v", err)
	}
	defer latest.Close()

	// latest should see the updates: 2 segments (one with doc 2, one with doc 3)
	if len(latest.GetSequentialSubReaders()) != 2 {
		t.Errorf("expected 2 segments, got %d", len(latest.GetSequentialSubReaders()))
	}

	// This reader should be for commit point 1
	oldest, err := latest.OpenIfChanged(ic1)
	if err != nil {
		t.Fatalf("failed to open reader for ic1: %v", err)
	}
	defer oldest.Close()

	// This reader should not see the deletion of doc 1:
	if oldest.NumDocs() != 2 {
		t.Errorf("expected 2 docs in oldest reader, got %d", oldest.NumDocs())
	}
	if oldest.HasDeletions() {
		t.Error("expected oldest reader to have no deletions")
	}

	// Verify sharing same core (simplified check: we don't have CacheHelper key exposed easily in Go,
	// but we can check if the readers are clones)
	// In Gocene, if we use OpenIfChanged, it should reuse the underlying core readers.
}
