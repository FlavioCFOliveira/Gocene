package nrt_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNRTDeleteVisibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	conf := index.NewIndexWriterConfigWithAnalyzer(nil)
	conf.SetOpenMode(index.Create)
	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to create index writer: %v", err)
	}
	defer writer.Close()

	// initial commit to write metadata
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	doc1 := document.NewDocument()
	doc1.Add(mustStringField(t, "key", "value1"))
	writer.AddDocument(doc1)

	doc2 := document.NewDocument()
	doc2.Add(mustStringField(t, "key", "value2"))
	writer.AddDocument(doc2)

	if _, err := writer.Commit(); err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	// Snapshot current state (commit point 1): the newest commit in the
	// directory, which is the one the writer just made.
	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	ic1 := commits.GetLatest()

	// Update doc 1: this deletes doc 1 and adds doc 3
	doc3 := document.NewDocument()
	doc3.Add(mustStringField(t, "key", "value3"))
	if _, err := writer.UpdateDocument(index.NewTerm("key", "value1"), doc3); err != nil {
		t.Fatalf("update document failed: %v", err)
	}

	// Open NRT reader from writer
	latest, err := writer.GetReader(true, false)
	if err != nil {
		t.Fatalf("failed to open NRT reader: %v", err)
	}
	defer latest.Close()

	// latest should see the updates: 2 segments (one with doc 2, one with doc 3)
	if len(latest.GetSequentialSubReaders()) != 2 {
		t.Errorf("expected 2 segments, got %d", len(latest.GetSequentialSubReaders()))
	}

	// This reader should be for commit point 1
	oldest, err := index.OpenIfChangedWithCommit(latest, ic1)
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
}

// mustStringField builds a stored StringField (Field.Store.YES).
func mustStringField(t *testing.T, name, value string) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, true)
	if err != nil {
		t.Fatalf("NewStringField(%q): %v", name, err)
	}
	return f
}
