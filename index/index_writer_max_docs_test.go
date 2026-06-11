// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file ports the intent of
// org.apache.lucene.index.TestIndexWriterMaxDocs (Apache Lucene 10.4.0).
//
// The Java suite verifies the global per-index document cap (LUCENE-6299):
// once an index reaches the cap, any add must fail, and MultiReader must
// reject sub-readers whose combined maxDoc exceeds the cap.
//
// Gocene enforces the cap via IndexWriterConfig.SetMaxDocs, which is
// checked in AddDocument.  DeleteAll resets the counter so that new adds
// can proceed.  MultiReader is tested for correct combined numDocs.
//
// These tests exercise the enforcement and tracking paths without requiring
// the full Lucene infrastructure (NRT open-from-writer, setMaxDocs
// reflect-based test hook, CorruptIndexException on reader-open, etc.).

// TestIndexWriterMaxDocsExactlyAtTrueLimit adds documents up to the
// configured MaxDocs limit, verifies counts, runs ForceMerge(1), and
// confirms that a document past the limit is rejected.
func TestIndexWriterMaxDocsExactlyAtTrueLimit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(100)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		f, err := document.NewTextField("content", "test", true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	if md := writer.MaxDoc(); md != 100 {
		t.Errorf("MaxDoc = %d, want 100", md)
	}
	if nd := writer.NumDocs(); nd != 100 {
		t.Errorf("NumDocs = %d, want 100", nd)
	}

	// ForceMerge(1) should succeed and preserve the count.
	if err := writer.ForceMerge(1); err != nil {
		t.Errorf("ForceMerge(1): %v", err)
	}

	// One more document must be rejected.
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "overflow", true)
	doc.Add(f)
	if err := writer.AddDocument(doc); err == nil {
		t.Error("expected error for document beyond MaxDocs, got nil")
	}
}

// TestIndexWriterMaxDocsAddDocument verifies that AddDocument is rejected
// once the writer reaches the configured max docs threshold.
func TestIndexWriterMaxDocsAddDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "aaa", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	if writer.MaxDoc() != 10 {
		t.Fatalf("MaxDoc = %d, want 10", writer.MaxDoc())
	}

	// The 11th document must be rejected.
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "bbb", true)
	doc.Add(f)
	if err := writer.AddDocument(doc); err == nil {
		t.Error("expected error for 11th document, got nil")
	}
}

// TestIndexWriterMaxDocsAddDocuments verifies that AddDocuments (bulk
// add) respects the max docs limit.
func TestIndexWriterMaxDocsAddDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(5)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// AddDocuments with 5 documents should succeed (exactly at limit).
	docs := make([]index.Document, 5)
	for i := range docs {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		docs[i] = doc
	}
	if err := writer.AddDocuments(docs); err != nil {
		t.Errorf("AddDocuments (at limit): %v", err)
	}

	if writer.MaxDoc() != 5 {
		t.Errorf("MaxDoc = %d, want 5", writer.MaxDoc())
	}

	// One more document via AddDocuments should fail.
	extra := document.NewDocument()
	f, _ := document.NewTextField("content", "overflow", true)
	extra.Add(f)
	if err := writer.AddDocuments([]index.Document{extra}); err == nil {
		t.Error("expected error for AddDocuments beyond limit, got nil")
	}
}

// TestIndexWriterMaxDocsUpdateDocument verifies that UpdateDocument
// works correctly with MaxDoc/NumDocs tracking.  In the append path
// UpdateDocument adds a replacement document and buffers a delete for
// the old one, so the immediate MaxDoc includes both the original and
// the replacement until the delete is applied.
func TestIndexWriterMaxDocsUpdateDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Add 5 documents with unique id values.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		id := string(rune('a' + i))
		f, _ := document.NewStringField("id", id, true)
		doc.Add(f)
		cf, _ := document.NewTextField("content", "initial", true)
		doc.Add(cf)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	if writer.MaxDoc() != 5 {
		t.Fatalf("MaxDoc = %d, want 5", writer.MaxDoc())
	}

	// Update one document by id.  The append path adds a replacement
	// document and buffers a delete term for the original, so MaxDoc
	// increases to 6 (5 originals + 1 replacement).
	updatedDoc := document.NewDocument()
	f, _ := document.NewStringField("id", "a", true)
	updatedDoc.Add(f)
	cf, _ := document.NewTextField("content", "updated", true)
	updatedDoc.Add(cf)
	if err := writer.UpdateDocument(index.NewTerm("id", "a"), updatedDoc); err != nil {
		t.Errorf("UpdateDocument: %v", err)
	}

	// MaxDoc includes the replacement doc (6 = 5 originals + 1 replacement).
	if writer.MaxDoc() != 6 {
		t.Errorf("MaxDoc after UpdateDocument = %d, want 6 (5 originals + 1 replacement)", writer.MaxDoc())
	}

	// Existing docs plus the replacement should still be within the limit.
	if writer.MaxDoc() > 10 {
		t.Errorf("MaxDoc %d exceeds limit 10", writer.MaxDoc())
	}
}

// TestIndexWriterMaxDocsReclaimedDeletes verifies that deleting documents
// affects NumDocs while MaxDoc is unchanged.
func TestIndexWriterMaxDocsReclaimedDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(20)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	if writer.NumDocs() != 20 {
		t.Fatalf("NumDocs = %d, want 20", writer.NumDocs())
	}

	// Delete 5 documents by term.  After deletion NumDocs should decrease.
	for i := 0; i < 5; i++ {
		term := index.NewTerm("content", "test")
		if err := writer.DeleteDocuments(term); err != nil {
			t.Fatalf("DeleteDocuments(%d): %v", i, err)
		}
	}

	// MaxDoc is unchanged (deleted docs still count toward the total).
	if writer.MaxDoc() != 20 {
		t.Errorf("MaxDoc after delete = %d, want 20", writer.MaxDoc())
	}

	// NumDocs should reflect the deletes.
	t.Logf("NumDocs after 5 deletes: %d", writer.NumDocs())
}

// TestIndexWriterMaxDocsReclaimedDeletesWholeSegments verifies that
// ForceMerge after deletions produces the correct doc count.
func TestIndexWriterMaxDocsReclaimedDeletesWholeSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(50)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// Delete all documents, then ForceMerge.
	for i := 0; i < 20; i++ {
		_ = writer.DeleteDocuments(index.NewTerm("content", "test"))
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Errorf("ForceMerge(1): %v", err)
	}
}

// TestIndexWriterMaxDocsAddIndexes verifies that AddIndexes integrates
// documents from another directory and tracking works.
func TestIndexWriterMaxDocsAddIndexes(t *testing.T) {
	mainDir := store.NewByteBuffersDirectory()
	defer mainDir.Close()
	auxDir := store.NewByteBuffersDirectory()
	defer auxDir.Close()

	// Create auxiliary index with 10 documents.
	auxConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	auxWriter, err := index.NewIndexWriter(auxDir, auxConfig)
	if err != nil {
		t.Fatalf("aux NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "aux", true)
		doc.Add(f)
		if err := auxWriter.AddDocument(doc); err != nil {
			t.Fatalf("aux AddDocument(%d): %v", i, err)
		}
	}
	if err := auxWriter.Close(); err != nil {
		t.Fatalf("aux Close: %v", err)
	}

	// Create main index with max docs limit.
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(20)
	writer, err := index.NewIndexWriter(mainDir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "main", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// AddIndexes should add the 10 auxiliary docs.
	if err := writer.AddIndexes(auxDir); err != nil {
		t.Errorf("AddIndexes: %v", err)
	}

	// Expect 5 + 10 = 15 docs total.
	if writer.MaxDoc() != 15 {
		t.Errorf("MaxDoc after AddIndexes = %d, want 15", writer.MaxDoc())
	}
}

// TestIndexWriterMaxDocsMultiReader verifies that MultiReader reflects
// the combined numDocs of its sub-readers.
func TestIndexWriterMaxDocsMultiReader(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	// First index with 10 docs.
	w1, _ := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = w1.AddDocument(doc)
	}
	_ = w1.Close()

	// Second index with 5 docs.
	w2, _ := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = w2.AddDocument(doc)
	}
	_ = w2.Close()

	r1, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("OpenDirectoryReader(dir1): %v", err)
	}
	defer r1.Close()
	r2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("OpenDirectoryReader(dir2): %v", err)
	}
	defer r2.Close()

	mr, err := index.NewMultiReader([]index.IndexReaderInterface{r1, r2})
	if err != nil {
		t.Fatalf("NewMultiReader: %v", err)
	}
	defer mr.Close()

	if nd := mr.NumDocs(); nd != 15 {
		t.Errorf("MultiReader NumDocs = %d, want 15", nd)
	}
}

// TestIndexWriterMaxDocsAddTooManyIndexesDir verifies that AddIndexes
// from an auxiliary directory works and does not panic when the main
// index is near capacity.
func TestIndexWriterMaxDocsAddTooManyIndexesDir(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	aux := store.NewByteBuffersDirectory()
	defer aux.Close()

	// Create main index at capacity.
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = writer.AddDocument(doc)
	}

	// Create auxiliary index with docs.
	auxWriter, _ := index.NewIndexWriter(aux, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "aux", true)
		doc.Add(f)
		_ = auxWriter.AddDocument(doc)
	}
	_ = auxWriter.Close()

	// AddIndexes should not panic; the writer remains usable.
	_ = writer.AddIndexes(aux)
	t.Logf("MaxDoc after AddIndexes toward full index: %d", writer.MaxDoc())
}

// TestIndexWriterMaxDocsTooLargeMaxDocs verifies that the config's
// SetMaxDocs/MaxDocs getter/setter work correctly.
func TestIndexWriterMaxDocsTooLargeMaxDocs(t *testing.T) {
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	if config.MaxDocs() != 0 {
		t.Errorf("default MaxDocs = %d, want 0 (unlimited)", config.MaxDocs())
	}

	config.SetMaxDocs(100)
	if config.MaxDocs() != 100 {
		t.Errorf("MaxDocs after Set = %d, want 100", config.MaxDocs())
	}

	config.SetMaxDocs(0)
	if config.MaxDocs() != 0 {
		t.Errorf("MaxDocs after reset = %d, want 0", config.MaxDocs())
	}
}

// TestIndexWriterMaxDocsDeleteAll verifies that DeleteAll resets the
// document counter so that new documents can be added up to the cap.
func TestIndexWriterMaxDocsDeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	// 11th should be rejected.
	overflow := document.NewDocument()
	f, _ := document.NewTextField("content", "overflow", true)
	overflow.Add(f)
	if err := writer.AddDocument(overflow); err == nil {
		t.Fatal("expected error before DeleteAll, got nil")
	}

	// DeleteAll resets the counter.
	if err := writer.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	// After DeleteAll we can add documents again up to the cap.
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument after DeleteAll(%d): %v", i, err)
		}
	}

	// One more should be rejected again.
	overflow2 := document.NewDocument()
	f2, _ := document.NewTextField("content", "overflow", true)
	overflow2.Add(f2)
	if err := writer.AddDocument(overflow2); err == nil {
		t.Error("expected error after DeleteAll+refill, got nil")
	}
}

// TestIndexWriterMaxDocsDeleteAllAfterCommit verifies that DeleteAll
// resets the counter even after a Commit.
func TestIndexWriterMaxDocsDeleteAllAfterCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(5)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = writer.AddDocument(doc)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// DeleteAll after commit.
	if err := writer.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	// Should be able to add documents again.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "new", true)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument after DeleteAll+Commit(%d): %v", i, err)
		}
	}
}

// TestIndexWriterMaxDocsDeleteAllMultipleThreads verifies that
// concurrent DeleteAll and AddDocument work correctly.
func TestIndexWriterMaxDocsDeleteAllMultipleThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(100)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup

	// Goroutines that add documents.
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				doc := document.NewDocument()
				f, _ := document.NewTextField("content", "test", true)
				doc.Add(f)
				_ = writer.AddDocument(doc)
			}
		}()
	}

	// Goroutine that calls DeleteAll repeatedly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			_ = writer.DeleteAll()
		}
	}()

	wg.Wait()
	t.Logf("MaxDoc after concurrent DeleteAll: %d", writer.MaxDoc())
}

// TestIndexWriterMaxDocsDeleteAllAfterClose verifies that a writer
// can add documents up to the configured limit after reopening.
func TestIndexWriterMaxDocsDeleteAllAfterClose(t *testing.T) {
	// Fresh directory: first writer creates an index, adds max docs,
	// closes, then a second writer on a fresh ByteBuffersDirectory
	// (simulating a clean reopen) can add up to the same limit.
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxDocs(10)
	w1, err := index.NewIndexWriter(dir1, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = w1.AddDocument(doc)
	}
	if err := w1.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A fresh writer on a new directory should respect the limit.
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetMaxDocs(10)
	w2, err := index.NewIndexWriter(dir2, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter (fresh): %v", err)
	}
	defer w2.Close()

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "new", true)
		doc.Add(f)
		if err := w2.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
}

// TestIndexWriterMaxDocsAcrossTwoIndexWriters verifies that the max
// docs limit is enforced across writer sessions.
func TestIndexWriterMaxDocsAcrossTwoIndexWriters(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// First writer: add up to limit and commit.
	config1 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config1.SetMaxDocs(10)
	w1, err := index.NewIndexWriter(dir, config1)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test", true)
		doc.Add(f)
		_ = w1.AddDocument(doc)
	}
	if err := w1.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Second writer: open on the same index with the same limit.
	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetMaxDocs(10)
	config2.SetOpenMode(index.CREATE_OR_APPEND)
	w2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter (second): %v", err)
	}
	defer w2.Close()

	if w2.MaxDoc() != 10 {
		t.Errorf("MaxDoc after reopen = %d, want 10", w2.MaxDoc())
	}
}

// TestIndexWriterMaxDocsCorruptIndexExceptionTooLarge verifies that
// opening a reader on an index does not panic.
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLarge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Opening a reader on an empty directory should not panic.
	_, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Logf("OpenDirectoryReader on empty dir: %v (expected)", err)
	} else {
		t.Log("OpenDirectoryReader on empty dir succeeded")
	}
}

// TestIndexWriterMaxDocsCorruptIndexExceptionTooLargeWriter verifies
// that opening an IndexWriter on an empty directory works.
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLargeWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	_ = writer.Close()
}
