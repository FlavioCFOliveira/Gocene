// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter merge operations.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterMerging
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMerging.java
//
// Focus areas:
//   - Force merge operations (forceMerge, forceMergeDeletes)
//   - Automatic merge behavior
//   - Merge during concurrent indexing
//   - Merge with deletions
//
// GC-178: Test Coverage - IndexWriterMerging
package index_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Link the production Lucene 10.4 codec implementation so that tests in this
// file exercise real on-disk formats instead of the in-memory fallback.
var _ = codecs.GetDefault

// TestIndexWriterMerging_Lucene tests that index merging (specifically addIndexes)
// doesn't change the index order of documents.
// Ported from: TestIndexWriterMerging.testLucene()
func TestIndexWriterMerging_Lucene(t *testing.T) {
	num := 100

	// Create two separate directories
	indexA := store.NewByteBuffersDirectory()
	defer indexA.Close()
	indexB := store.NewByteBuffersDirectory()
	defer indexB.Close()

	// Fill index A with documents 0-99
	fillIndex(t, indexA, 0, num, rand.NewSource(42))
	if fail := verifyIndex(t, indexA, 0); fail {
		t.Error("Index A is invalid")
	}

	// Fill index B with documents 100-199
	fillIndex(t, indexB, num, num, rand.NewSource(43))
	if fail := verifyIndex(t, indexB, num); fail {
		t.Error("Index B is invalid")
	}

	// Create merged index
	merged := store.NewByteBuffersDirectory()
	defer merged.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set merge policy when LogMergePolicy is available
	// config.SetMergePolicy(index.NewLogMergePolicy(2))

	writer, err := index.NewIndexWriter(merged, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add indexes
	// TODO: Implement AddIndexes when available
	// writer.AddIndexes(indexA, indexB)
	t.Fatal("AddIndexes not yet implemented")

	// Force merge to single segment
	// TODO: Implement ForceMerge when available
	// writer.ForceMerge(1)

	writer.Close()

	// Verify merged index
	if fail := verifyIndex(t, merged, 0); fail {
		t.Error("The merged index is invalid")
	}
}

// fillIndex creates an index with documents containing a "count" field
// with values from start to start+numDocs-1
func fillIndex(t *testing.T, dir store.Directory, start, numDocs int, source rand.Source) {
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetOpenMode(index.CREATE)
	config.SetMaxBufferedDocs(2)
	// TODO: Set merge policy when LogMergePolicy is available
	// config.SetMergePolicy(index.NewLogMergePolicy(2))

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := start; i < start+numDocs; i++ {
		doc := createCountDocument(i)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}

// createCountDocument creates a document with a "count" field
func createCountDocument(count int) index.Document {
	doc := document.NewDocument()
	countField, err := document.NewStringField("count", fmt.Sprintf("%d", count), true)
	if err != nil {
		panic(fmt.Sprintf("createCountDocument(%d): %v", count, err))
	}
	doc.Add(countField)
	return doc
}

// verifyIndex checks that documents in the index have the expected count values
func verifyIndex(t *testing.T, directory store.Directory, startAt int) bool {
	fail := false

	// TODO: Implement DirectoryReader.Open when available
	// reader, err := index.OpenDirectoryReader(directory)
	// if err != nil {
	//     t.Fatalf("Failed to open reader: %v", err)
	// }
	// defer reader.Close()

	// max := reader.MaxDoc()
	// storedFields := reader.StoredFields()
	// for i := 0; i < max; i++ {
	//     doc := storedFields.Document(i)
	//     countField := doc.GetField("count")
	//     expected := fmt.Sprintf("%d", i+startAt)
	//     if countField == nil || countField.StringValue() != expected {
	//         t.Logf("Document %d is returning document %v", i+startAt, countField)
	//         fail = true
	//     }
	// }

	t.Fatal("DirectoryReader.Open not yet implemented")
	return fail
}

// TestIndexWriterMerging_ForceMergeDeletes tests forceMergeDeletes when
// 2 singular merges are required (LUCENE-325).
// Ported from: TestIndexWriterMerging.testForceMergeDeletes()
func TestIndexWriterMerging_ForceMergeDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH
	// TODO: Set merge policy when LogMergePolicy is available

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 10 documents
	for i := 0; i < 10; i++ {
		doc := createIDDocument(i)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify 10 docs
	// TODO: Verify with DirectoryReader when available
	// reader, _ := index.OpenDirectoryReader(dir)
	// assertEquals(t, 10, reader.MaxDoc())
	// assertEquals(t, 10, reader.NumDocs())
	// reader.Close()

	// Delete documents 0 and 7 using NoMergePolicy.
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, err = index.NewIndexWriter(dir, dontMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create deleter writer: %v", err)
	}
	if _, err := writer.DeleteDocuments(index.NewTerm("id", "0")); err != nil {
		t.Fatalf("DeleteDocuments id=0: %v", err)
	}
	if _, err := writer.DeleteDocuments(index.NewTerm("id", "7")); err != nil {
		t.Fatalf("DeleteDocuments id=7: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close deleter writer: %v", err)
	}

	// Verify 8 live docs, 10 total.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if got := reader.NumDocs(); got != 8 {
		t.Errorf("reader.NumDocs after deletes = %d, want 8", got)
	}
	if got := reader.MaxDoc(); got != 10 {
		t.Errorf("reader.MaxDoc after deletes = %d, want 10", got)
	}
	reader.Close()

	// Force merge deletes.
	forceMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	forceMergeConfig.SetMergePolicy(index.NewLogMergePolicy())
	writer, err = index.NewIndexWriter(dir, forceMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create force-merge writer: %v", err)
	}
	if got := writer.GetDocStats().NumDocs; got != 8 {
		t.Errorf("GetDocStats().NumDocs before forceMergeDeletes = %d, want 8", got)
	}
	if got := writer.GetDocStats().MaxDoc; got != 10 {
		t.Errorf("GetDocStats().MaxDoc before forceMergeDeletes = %d, want 10", got)
	}
	if err := writer.ForceMergeDeletes(); err != nil {
		t.Fatalf("ForceMergeDeletes: %v", err)
	}
	if got := writer.GetDocStats().NumDocs; got != 8 {
		t.Errorf("GetDocStats().NumDocs after forceMergeDeletes = %d, want 8", got)
	}
	if got := writer.GetDocStats().MaxDoc; got != 8 {
		t.Errorf("GetDocStats().MaxDoc after forceMergeDeletes = %d, want 8", got)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close force-merge writer: %v", err)
	}

	// Verify final state.
	reader, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader final: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 8 {
		t.Errorf("reader.MaxDoc final = %d, want 8", got)
	}
	if got := reader.NumDocs(); got != 8 {
		t.Errorf("reader.NumDocs final = %d, want 8", got)
	}
	if reader.HasDeletions() {
		t.Error("final reader should not report deletions after forceMergeDeletes")
	}
}

// TestIndexWriterMerging_ForceMergeDeletes2 tests forceMergeDeletes when
// many adjacent merges are required (LUCENE-325).
// Ported from: TestIndexWriterMerging.testForceMergeDeletes2()
func TestIndexWriterMerging_ForceMergeDeletes2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH
	// TODO: Set merge policy with merge factor 50

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 98 documents
	for i := 0; i < 98; i++ {
		doc := createIDDocument(i)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	// Delete every other document
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, err = index.NewIndexWriter(dir, dontMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create deleter writer: %v", err)
	}
	for i := 0; i < 98; i += 2 {
		if _, err := writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i))); err != nil {
			t.Fatalf("DeleteDocuments id=%d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close deleter writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if got := reader.NumDocs(); got != 49 {
		t.Errorf("reader.NumDocs after deletes = %d, want 49", got)
	}
	reader.Close()

	// Force merge deletes with merge factor 3
	forceMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	lmp3 := index.NewLogMergePolicy()
	lmp3.SetMergeFactor(3)
	forceMergeConfig.SetMergePolicy(lmp3)
	writer, err = index.NewIndexWriter(dir, forceMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create force-merge writer: %v", err)
	}
	if got := writer.GetDocStats().NumDocs; got != 49 {
		t.Errorf("GetDocStats().NumDocs before forceMergeDeletes = %d, want 49", got)
	}
	if err := writer.ForceMergeDeletes(); err != nil {
		t.Fatalf("ForceMergeDeletes: %v", err)
	}
	if got := writer.GetDocStats().NumDocs; got != 49 {
		t.Errorf("GetDocStats().NumDocs after forceMergeDeletes = %d, want 49", got)
	}
	writer.Close()

	// Verify
	reader, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader final: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 49 {
		t.Errorf("reader.MaxDoc final = %d, want 49", got)
	}
	if got := reader.NumDocs(); got != 49 {
		t.Errorf("reader.NumDocs final = %d, want 49", got)
	}
	if reader.HasDeletions() {
		t.Error("final reader should not report deletions after forceMergeDeletes")
	}
}

// TestIndexWriterMerging_ForceMergeDeletes3 tests forceMergeDeletes without
// waiting when many adjacent merges are required (LUCENE-325).
// Ported from: TestIndexWriterMerging.testForceMergeDeletes3()
func TestIndexWriterMerging_ForceMergeDeletes3(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 98 documents
	for i := 0; i < 98; i++ {
		doc := createIDDocument(i)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	// Delete every other document
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, err = index.NewIndexWriter(dir, dontMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create deleter writer: %v", err)
	}
	for i := 0; i < 98; i += 2 {
		if _, err := writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i))); err != nil {
			t.Fatalf("DeleteDocuments id=%d: %v", i, err)
		}
	}
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if got := reader.NumDocs(); got != 49 {
		t.Errorf("reader.NumDocs after deletes = %d, want 49", got)
	}
	reader.Close()

	// Force merge deletes without blocking (doWait=false)
	forceMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	lmp3 := index.NewLogMergePolicy()
	lmp3.SetMergeFactor(3)
	forceMergeConfig.SetMergePolicy(lmp3)
	writer, err = index.NewIndexWriter(dir, forceMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create force-merge writer: %v", err)
	}
	observer, err := writer.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver: %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if !observer.AwaitWithTimeout(30 * time.Second) {
		t.Fatal("observer.AwaitWithTimeout returned false")
	}
	if observer.NumCompletedMerges() != observer.NumMerges() {
		t.Errorf("NumCompletedMerges=%d, want %d", observer.NumCompletedMerges(), observer.NumMerges())
	}
	writer.Close()

	// Verify
	reader, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader final: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 49 {
		t.Errorf("reader.MaxDoc final = %d, want 49", got)
	}
	if got := reader.NumDocs(); got != 49 {
		t.Errorf("reader.NumDocs final = %d, want 49", got)
	}
}

// TestIndexWriterMerging_ForceMergeDeletesWithObserver tests force merge
// deletes with a MergeObserver.
// Ported from: TestIndexWriterMerging.testForceMergeDeletesWithObserver()
func TestIndexWriterMerging_ForceMergeDeletesWithObserver(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index with 10 documents
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH

	indexer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := createIDDocument(i)
		if _, err := indexer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	indexer.Close()

	// Delete even documents
	deleterConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	deleterConfig.SetMergePolicy(index.NewNoMergePolicy())

	deleter, err := index.NewIndexWriter(dir, deleterConfig)
	if err != nil {
		t.Fatalf("Failed to create deleter writer: %v", err)
	}
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			if _, err := deleter.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i))); err != nil {
				t.Fatalf("DeleteDocuments id=%d: %v", i, err)
			}
		}
	}
	deleter.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if got := reader.NumDocs(); got != 5 {
		t.Errorf("reader.NumDocs after deletes = %d, want 5", got)
	}
	reader.Close()

	// Force merge deletes with observer
	forceMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	forceMergeConfig.SetMergePolicy(index.NewLogMergePolicy())
	iw, err := index.NewIndexWriter(dir, forceMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create force-merge writer: %v", err)
	}
	if got := iw.GetDocStats().MaxDoc; got != 10 {
		t.Errorf("GetDocStats().MaxDoc before forceMergeDeletes = %d, want 10", got)
	}
	if got := iw.GetDocStats().NumDocs; got != 5 {
		t.Errorf("GetDocStats().NumDocs before forceMergeDeletes = %d, want 5", got)
	}

	observer, err := iw.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver: %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if observer.NumMerges() <= 0 {
		t.Errorf("observer.NumMerges() = %d, want > 0", observer.NumMerges())
	}
	if !observer.AwaitWithTimeout(30 * time.Second) {
		t.Fatal("observer.AwaitWithTimeout(30s) returned false")
	}
	if observer.NumCompletedMerges() != observer.NumMerges() {
		t.Errorf("NumCompletedMerges=%d, want %d", observer.NumCompletedMerges(), observer.NumMerges())
	}
	if got := iw.GetDocStats().MaxDoc; got != 5 {
		t.Errorf("GetDocStats().MaxDoc after forceMergeDeletes = %d, want 5", got)
	}
	if got := iw.GetDocStats().NumDocs; got != 5 {
		t.Errorf("GetDocStats().NumDocs after forceMergeDeletes = %d, want 5", got)
	}

	iw.WaitForMerges()
	iw.Close()
}

// TestIndexWriterMerging_MergeObserverNoMerges tests MergeObserver when
// no merges are needed.
// Ported from: TestIndexWriterMerging.testMergeObserverNoMerges()
func TestIndexWriterMerging_MergeObserverNoMerges(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// config.SetMergePolicy(index.NewNoMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := createIDDocument(1)
	writer.AddDocument(doc)
	writer.Commit()

	observer, err := writer.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver: %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if got := observer.NumMerges(); got != 0 {
		t.Errorf("observer.NumMerges() = %d, want 0", got)
	}

	writer.Close()
}

// TestIndexWriterMerging_MergeObserverAwaitWithTimeout tests MergeObserver
// await with timeout.
// Ported from: TestIndexWriterMerging.testMergeObserverAwaitWithTimeout()
func TestIndexWriterMerging_MergeObserverAwaitWithTimeout(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMergePolicy(index.NewLogMergePolicy())

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 10 documents
	for i := 0; i < 10; i++ {
		doc := createIDDocument(i)
		iw.AddDocument(doc)
	}
	iw.Commit()

	// Delete first 3 documents
	for _, id := range []int{0, 1, 2} {
		if _, err := iw.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id))); err != nil {
			t.Fatalf("DeleteDocuments id=%d: %v", id, err)
		}
	}
	iw.Commit()

	observer, err := iw.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver: %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if !observer.AwaitWithTimeout(30 * time.Second) {
		t.Fatal("observer.AwaitWithTimeout(30s) returned false")
	}
	if observer.NumCompletedMerges() != observer.NumMerges() {
		t.Errorf("NumCompletedMerges=%d, want %d", observer.NumCompletedMerges(), observer.NumMerges())
	}

	iw.WaitForMerges()
	iw.Close()
}

// TestIndexWriterMerging_MergeObserverAwaitTimeout tests MergeObserver
// await timeout behavior by using a custom MergeScheduler that stalls a merge
// until a signal is provided.
// Ported from: TestIndexWriterMerging.testMergeObserverAwaitTimeout()
func TestIndexWriterMerging_MergeObserverAwaitTimeout(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	mergeStarted := make(chan struct{})
	allowMergeToFinish := make(chan struct{})

	customScheduler := &blockingMergeScheduler{
		mergeStarted: mergeStarted,
		allowFinish:  allowMergeToFinish,
	}

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMergePolicy(index.NewLogMergePolicy())
	config.SetMergeScheduler(customScheduler)

	indexer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 20 documents.
	for i := 0; i < 20; i++ {
		doc := createIDDocument(i)
		if _, err := indexer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument id=%d: %v", i, err)
		}
	}
	if err := indexer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Delete first 10 documents.
	for i := 0; i < 10; i++ {
		if _, err := indexer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i))); err != nil {
			t.Fatalf("DeleteDocuments id=%d: %v", i, err)
		}
	}
	if err := indexer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	observer, err := indexer.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver: %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if observer.NumMerges() == 0 {
		t.Fatal("expected at least one merge")
	}

	// Wait until the scheduler has picked up the merge and is blocked.
	<-mergeStarted

	// The merge is stalled; a short await should time out.
	if observer.AwaitWithTimeout(10 * time.Millisecond) {
		t.Fatal("observer.AwaitWithTimeout(10ms) returned true, want timeout")
	}

	// Allow the blocked merge to finish.
	close(allowMergeToFinish)

	if !observer.AwaitWithTimeout(30 * time.Second) {
		t.Fatal("observer.AwaitWithTimeout(30s) returned false after unblocking")
	}
	if observer.NumCompletedMerges() != observer.NumMerges() {
		t.Errorf("NumCompletedMerges=%d, want %d", observer.NumCompletedMerges(), observer.NumMerges())
	}

	if err := indexer.WaitForMerges(); err != nil {
		t.Fatalf("WaitForMerges: %v", err)
	}
	if err := indexer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// blockingMergeScheduler is a MergeScheduler that stalls the first merge it
// sees until allowFinish is closed. It is used to test MergeObserver timeout
// behavior.
type blockingMergeScheduler struct {
	mergeStarted chan struct{}
	allowFinish  chan struct{}
}

func (s *blockingMergeScheduler) Merge(source index.MergeSource, trigger index.MergeTrigger) error {
	merge := source.GetNextMerge()
	if merge == nil {
		return nil
	}
	close(s.mergeStarted)
	<-s.allowFinish
	err := source.Merge(merge)
	if err != nil {
		merge.Error = err
	}
	source.OnMergeFinished(merge)
	return err
}

func (s *blockingMergeScheduler) Close() error { return nil }

func (s *blockingMergeScheduler) GetRunningMergeCount() int { return 0 }

func (s *blockingMergeScheduler) SetMaxMerges(int) {}

func (s *blockingMergeScheduler) GetMaxMerges() int { return 1 }

// TestIndexWriterMerging_ForceMergeDeletesBlockingWithObserver tests blocking
// force merge deletes with observer.
// Ported from: TestIndexWriterMerging.testForceMergeDeletesBlockingWithObserver()
func TestIndexWriterMerging_ForceMergeDeletesBlockingWithObserver(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index with 10 documents
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH

	indexer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := createIDDocument(i)
		indexer.AddDocument(doc)
	}
	indexer.Close()

	// Delete even documents
	deleterConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	deleterConfig.SetMergePolicy(index.NewNoMergePolicy())

	deleter, err := index.NewIndexWriter(dir, deleterConfig)
	if err != nil {
		t.Fatalf("Failed to create deleter writer: %v", err)
	}
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			if _, err := deleter.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i))); err != nil {
				t.Fatalf("DeleteDocuments id=%d: %v", i, err)
			}
		}
	}
	deleter.Close()

	// Force merge deletes with blocking (doWait=true)
	forceMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	forceMergeConfig.SetMergePolicy(index.NewLogMergePolicy())
	iw, err := index.NewIndexWriter(dir, forceMergeConfig)
	if err != nil {
		t.Fatalf("Failed to create force-merge writer: %v", err)
	}
	if got := iw.GetDocStats().MaxDoc; got != 10 {
		t.Errorf("GetDocStats().MaxDoc before forceMergeDeletes = %d, want 10", got)
	}
	if got := iw.GetDocStats().NumDocs; got != 5 {
		t.Errorf("GetDocStats().NumDocs before forceMergeDeletes = %d, want 5", got)
	}

	observer, err := iw.ForceMergeDeletesWithObserver(true)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver(true): %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if observer.NumMerges() <= 0 {
		t.Errorf("observer.NumMerges() = %d, want > 0", observer.NumMerges())
	}
	if !observer.Await() {
		t.Fatal("observer.Await() returned false")
	}
	if observer.NumCompletedMerges() != observer.NumMerges() {
		t.Errorf("NumCompletedMerges=%d, want %d", observer.NumCompletedMerges(), observer.NumMerges())
	}
	if got := iw.GetDocStats().MaxDoc; got != 5 {
		t.Errorf("GetDocStats().MaxDoc after forceMergeDeletes = %d, want 5", got)
	}
	if got := iw.GetDocStats().NumDocs; got != 5 {
		t.Errorf("GetDocStats().NumDocs after forceMergeDeletes = %d, want 5", got)
	}

	iw.Close()
}

// TestIndexWriterMerging_BlockingModeWithNoMerges tests blocking mode when
// no merges are needed.
// Ported from: TestIndexWriterMerging.testBlockingModeWithNoMerges()
func TestIndexWriterMerging_BlockingModeWithNoMerges(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMergePolicy(index.NewNoMergePolicy())

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := createIDDocument(1)
	iw.AddDocument(doc)
	iw.Commit()

	observer, err := iw.ForceMergeDeletesWithObserver(true)
	if err != nil {
		t.Fatalf("ForceMergeDeletesWithObserver(true): %v", err)
	}
	if observer == nil {
		t.Fatal("ForceMergeDeletesWithObserver returned nil observer")
	}
	if got := observer.NumMerges(); got != 0 {
		t.Errorf("observer.NumMerges() = %d, want 0", got)
	}
	if !observer.AwaitWithTimeout(1 * time.Second) {
		t.Fatal("observer.AwaitWithTimeout(1s) returned false")
	}
	if !observer.Await() {
		t.Fatal("observer.Await() returned false")
	}

	future := observer.AwaitAsync()
	if future == nil {
		t.Fatal("observer.AwaitAsync() returned nil future")
	}
	if !future.IsDone() {
		t.Error("future.IsDone() = false, want true")
	}
	if future.IsCompletedExceptionally() {
		t.Error("future.IsCompletedExceptionally() = true, want false")
	}

	iw.Close()
}

// TestIndexWriterMerging_SetMaxMergeDocs tests setting max merge docs (LUCENE-1013).
// Ported from: TestIndexWriterMerging.testSetMaxMergeDocs()
func TestIndexWriterMerging_SetMaxMergeDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set custom merge scheduler that verifies maxMergeDocs
	// config.SetMergeScheduler(&maxMergeDocsVerifierScheduler{})
	config.SetMaxBufferedDocs(2)
	// TODO: Set LogMergePolicy

	// TODO: Set max merge docs to 20
	// lmp := index.NewLogMergePolicy()
	// lmp.SetMaxMergeDocs(20)
	// lmp.SetMergeFactor(2)
	// config.SetMergePolicy(lmp)

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 177 documents
	for i := 0; i < 177; i++ {
		doc := &testDocument{fields: []interface{}{}}
		iw.AddDocument(doc)
	}

	iw.Close()
	t.Fatal("LogMergePolicy with SetMaxMergeDocs not yet implemented")
}

// TestIndexWriterMerging_NoWaitClose tests close without waiting during
// concurrent indexing.
// Ported from: TestIndexWriterMerging.testNoWaitClose()
func TestIndexWriterMerging_NoWaitClose(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	defer directory.Close()

	for pass := 0; pass < 2; pass++ {
		config := index.NewIndexWriterConfig(createTestAnalyzer())
		config.SetOpenMode(index.CREATE)
		config.SetMaxBufferedDocs(2)
		// TODO: Set merge policy
		// config.SetCommitOnClose(false)

		// if pass == 2 {
		//     config.SetMergeScheduler(index.NewSerialMergeScheduler())
		// }

		writer, err := index.NewIndexWriter(directory, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// TODO: Set merge factor
		// writer.GetConfig().GetMergePolicy().SetMergeFactor(100)

		// Run multiple iterations
		for iter := 0; iter < 3; iter++ {
			// Add documents
			for j := 0; j < 199; j++ {
				doc := createIDDocument(iter*201 + j)
				writer.AddDocument(doc)
			}

			// Delete some documents
			delID := iter * 199
			for j := 0; j < 20; j++ {
				// TODO: Delete documents
				delID += 5
			}

			writer.Commit()

			// Force merges
			// TODO: Set merge factor to 2

			// Start concurrent indexing thread
			var failure atomic.Value
			var wg sync.WaitGroup
			wg.Add(1)

			done := make(chan struct{})
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						for i := 0; i < 100; i++ {
							doc := &testDocument{fields: []interface{}{}}
							_, err := writer.AddDocument(doc)
							if err != nil {
								// Check if already closed
								if _, ok := err.(*index.AlreadyClosedException); ok {
									return
								}
								failure.Store(err)
								return
							}
						}
					}
				}
			}()

			// Close writer (should abort merges)
			writer.Close()
			close(done)
			wg.Wait()

			if f := failure.Load(); f != nil {
				t.Fatalf("Concurrent indexing failed: %v", f)
			}

			// Verify reader can still read
			// reader, _ := index.OpenDirectoryReader(directory)
			// reader.Close()

			// Reopen writer
			reopenConfig := index.NewIndexWriterConfig(createTestAnalyzer())
			reopenConfig.SetOpenMode(index.APPEND)
			// reopenConfig.SetCommitOnClose(false)
			writer, _ = index.NewIndexWriter(directory, reopenConfig)
		}

		writer.Close()
	}
}

// TestIndexWriterMerging_AddEstimatedBytesToMerge tests estimated bytes tracking.
// Ported from: TestIndexWriterMerging.testAddEstimatedBytesToMerge()
func TestIndexWriterMerging_AddEstimatedBytesToMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	config.SetMergePolicy(index.NewNoMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	field, err := document.NewTextField("field", "content", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc := &testDocument{fields: []interface{}{field}}

	for i := 0; i < 10; i++ {
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	segmentInfos := writer.CloneSegmentInfos()
	if segmentInfos.Size() == 0 {
		t.Fatal("expected at least one committed segment")
	}
	merge := index.NewOneMerge(segmentInfos.List())
	writer.AddEstimatedBytesToMerge(merge)

	assertTrue(t, merge.EstimatedMergeBytes > 0, "EstimatedMergeBytes should be > 0")
	assertTrue(t, merge.TotalMergeBytes > 0, "TotalMergeBytes should be > 0")
	assertTrue(t, merge.EstimatedMergeBytes <= merge.TotalMergeBytes, "estimated should be <= total")
}

// Helper functions

// createIDDocument creates a document with an "id" field
func createIDDocument(id int) index.Document {
	doc := document.NewDocument()
	idField, err := document.NewStringField("id", fmt.Sprintf("%d", id), false)
	if err != nil {
		// Construction cannot fail for a plain string, but keep the panic
		// close to the call site for debugging.
		panic(fmt.Sprintf("createIDDocument(%d): %v", id, err))
	}
	doc.Add(idField)
	return doc
}

// assertEquals is a helper for asserting equality
func assertEquals(t *testing.T, expected, actual interface{}, msg ...string) {
	t.Helper()
	if expected != actual {
		if len(msg) > 0 {
			t.Errorf("%s: expected %v, got %v", msg[0], expected, actual)
		} else {
			t.Errorf("expected %v, got %v", expected, actual)
		}
	}
}

// assertTrue is a helper for asserting true
func assertTrue(t *testing.T, condition bool, msg ...string) {
	t.Helper()
	if !condition {
		if len(msg) > 0 {
			t.Errorf("%s: expected true, got false", msg[0])
		} else {
			t.Error("expected true, got false")
		}
	}
}

// assertFalse is a helper for asserting false
func assertFalse(t *testing.T, condition bool, msg ...string) {
	t.Helper()
	if condition {
		if len(msg) > 0 {
			t.Errorf("%s: expected false, got true", msg[0])
		} else {
			t.Error("expected false, got true")
		}
	}
}
