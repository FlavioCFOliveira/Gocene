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
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

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
	t.Skip("AddIndexes not yet implemented")

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
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}

// createCountDocument creates a document with a "count" field
func createCountDocument(count int) index.Document {
	fields := make([]interface{}, 0, 1)
	// TODO: Create StringField when available
	// field, _ := document.NewStringField("count", fmt.Sprintf("%d", count), true)
	// fields = append(fields, field)
	_ = count // Placeholder
	return &testDocument{fields: fields}
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

	t.Skip("DirectoryReader.Open not yet implemented")
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
		if err := writer.AddDocument(doc); err != nil {
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

	// Delete documents 0 and 7 using NoMergePolicy
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, _ = index.NewIndexWriter(dir, dontMergeConfig)
	// TODO: Implement DeleteDocuments with Term when available
	// writer.DeleteDocuments(index.NewTerm("id", "0"))
	// writer.DeleteDocuments(index.NewTerm("id", "7"))
	t.Skip("DeleteDocuments with Term not yet implemented")

	writer.Close()

	// Verify 8 live docs, 10 total
	// reader, _ = index.OpenDirectoryReader(dir)
	// assertEquals(t, 8, reader.NumDocs())
	// reader.Close()

	// Force merge deletes
	writer, _ = index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	// TODO: Implement GetDocStats when available
	// assertEquals(t, 8, writer.GetDocStats().NumDocs)
	// assertEquals(t, 10, writer.GetDocStats().MaxDoc)

	// TODO: Implement ForceMergeDeletes when available
	// writer.ForceMergeDeletes()
	t.Skip("ForceMergeDeletes not yet implemented")

	// assertEquals(t, 8, writer.GetDocStats().NumDocs)
	writer.Close()

	// Verify final state
	// reader, _ = index.OpenDirectoryReader(dir)
	// assertEquals(t, 8, reader.MaxDoc())
	// assertEquals(t, 8, reader.NumDocs())
	// reader.Close()
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
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	// Delete every other document
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, _ = index.NewIndexWriter(dir, dontMergeConfig)
	// TODO: Delete every other document
	// for i := 0; i < 98; i += 2 {
	//     writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i)))
	// }
	writer.Close()

	// Force merge deletes with merge factor 3
	writer, _ = index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	// TODO: Implement GetDocStats
	// assertEquals(t, 49, writer.GetDocStats().NumDocs)
	// writer.ForceMergeDeletes()
	writer.Close()

	// Verify
	// reader, _ := index.OpenDirectoryReader(dir)
	// assertEquals(t, 49, reader.MaxDoc())
	// assertEquals(t, 49, reader.NumDocs())
	t.Skip("ForceMergeDeletes not yet implemented")
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
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Close()

	// Delete every other document
	dontMergeConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// dontMergeConfig.SetMergePolicy(index.NewNoMergePolicy())

	writer, _ = index.NewIndexWriter(dir, dontMergeConfig)
	// TODO: Delete every other document
	writer.Close()

	// Force merge deletes without blocking (doWait=false)
	writer, _ = index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	// TODO: Implement ForceMergeDeletes with doWait parameter
	// writer.ForceMergeDeletes(false)
	writer.Close()

	// Verify
	// reader, _ := index.OpenDirectoryReader(dir)
	// assertEquals(t, 49, reader.MaxDoc())
	// assertEquals(t, 49, reader.NumDocs())
	t.Skip("ForceMergeDeletes with doWait parameter not yet implemented")
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
		if err := indexer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	indexer.Close()

	// Delete even documents
	deleterConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// deleterConfig.SetMergePolicy(index.NewNoMergePolicy())

	deleter, _ := index.NewIndexWriter(dir, deleterConfig)
	// TODO: Delete even documents
	// for i := 0; i < 10; i++ {
	//     if i%2 == 0 {
	//         deleter.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i)))
	//     }
	// }
	deleter.Close()

	// Force merge deletes with observer
	iw, _ := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	// TODO: Implement GetDocStats
	// assertEquals(t, 10, iw.GetDocStats().MaxDoc)
	// assertEquals(t, 5, iw.GetDocStats().NumDocs)

	// TODO: Implement ForceMergeDeletes with observer
	// observer := iw.ForceMergeDeletes(false)
	// assertTrue(t, observer.NumMerges() > 0, "Should have scheduled merges")
	// assertTrue(t, observer.Await(30*time.Second), "Merges should complete within 30 seconds")
	// assertEquals(t, observer.NumMerges(), observer.NumCompletedMerges(), "All merges should be completed")
	// assertEquals(t, 5, iw.GetDocStats().MaxDoc)
	// assertEquals(t, 5, iw.GetDocStats().NumDocs)

	// iw.WaitForMerges()
	iw.Close()
	t.Skip("ForceMergeDeletes with observer not yet implemented")
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

	// TODO: Implement ForceMergeDeletes with observer
	// observer := writer.ForceMergeDeletes(false)
	// assertEquals(t, 0, observer.NumMerges(), "Should have zero merges")

	writer.Close()
	t.Skip("ForceMergeDeletes with observer not yet implemented")
}

// TestIndexWriterMerging_MergeObserverAwaitWithTimeout tests MergeObserver
// await with timeout.
// Ported from: TestIndexWriterMerging.testMergeObserverAwaitWithTimeout()
func TestIndexWriterMerging_MergeObserverAwaitWithTimeout(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set merge policy

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
	// TODO: Implement DeleteDocuments
	// iw.DeleteDocuments(index.NewTerm("id", "0"))
	// iw.DeleteDocuments(index.NewTerm("id", "1"))
	// iw.DeleteDocuments(index.NewTerm("id", "2"))
	iw.Commit()

	// TODO: Implement ForceMergeDeletes with observer
	// observer := iw.ForceMergeDeletes(false)
	// assertTrue(t, observer.Await(30*time.Second), "Merges should complete within 30 seconds")
	// assertEquals(t, observer.NumMerges(), observer.NumCompletedMerges(), "All merges should be completed")

	// iw.WaitForMerges()
	iw.Close()
	t.Skip("ForceMergeDeletes with observer not yet implemented")
}

// TestIndexWriterMerging_MergeObserverAwaitTimeout tests MergeObserver
// await timeout behavior.
// Ported from: TestIndexWriterMerging.testMergeObserverAwaitTimeout()
func TestIndexWriterMerging_MergeObserverAwaitTimeout(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// TODO: Implement custom merge scheduler that blocks
	// mergeStarted := make(chan struct{})
	// allowMergeToFinish := make(chan struct{})

	// customScheduler := &blockingMergeScheduler{
	//     mergeStarted: mergeStarted,
	//     allowFinish:  allowMergeToFinish,
	// }

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set custom merge scheduler

	indexer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 20 documents
	for i := 0; i < 20; i++ {
		doc := createIDDocument(i)
		indexer.AddDocument(doc)
	}
	indexer.Commit()

	// Delete first 10 documents
	// for i := 0; i < 10; i++ {
	//     indexer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", i)))
	// }
	indexer.Commit()

	// TODO: Implement ForceMergeDeletes with observer
	// observer := indexer.ForceMergeDeletes(false)
	// if observer.NumMerges() > 0 {
	//     <-mergeStarted
	//     // Should timeout after 10ms
	//     assertFalse(t, observer.Await(10*time.Millisecond), "await should timeout")
	//     close(allowMergeToFinish)
	// }

	// indexer.WaitForMerges()
	indexer.Close()
	t.Skip("Custom merge scheduler and ForceMergeDeletes with observer not yet implemented")
}

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
	// TODO: Set NoMergePolicy when available
	// deleterConfig.SetMergePolicy(index.NewNoMergePolicy())

	deleter, _ := index.NewIndexWriter(dir, deleterConfig)
	// TODO: Delete even documents
	deleter.Close()

	// Force merge deletes with blocking (doWait=true)
	iw, _ := index.NewIndexWriter(dir, index.NewIndexWriterConfig(createTestAnalyzer()))
	// TODO: Implement GetDocStats
	// assertEquals(t, 10, iw.GetDocStats().MaxDoc)
	// assertEquals(t, 5, iw.GetDocStats().NumDocs)

	// observer := iw.ForceMergeDeletes(true)
	// assertTrue(t, observer.NumMerges() > 0, "Should have completed merges")
	// assertTrue(t, observer.Await(), "await should return true immediately")
	// assertEquals(t, 5, iw.GetDocStats().MaxDoc)
	// assertEquals(t, 5, iw.GetDocStats().NumDocs)

	iw.Close()
	t.Skip("ForceMergeDeletes with blocking and observer not yet implemented")
}

// TestIndexWriterMerging_BlockingModeWithNoMerges tests blocking mode when
// no merges are needed.
// Ported from: TestIndexWriterMerging.testBlockingModeWithNoMerges()
func TestIndexWriterMerging_BlockingModeWithNoMerges(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// config.SetMergePolicy(index.NewNoMergePolicy())

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := createIDDocument(1)
	iw.AddDocument(doc)
	iw.Commit()

	// TODO: Implement ForceMergeDeletes with observer
	// observer := iw.ForceMergeDeletes(true)
	// assertEquals(t, 0, observer.NumMerges(), "Should have zero merges")
	// assertTrue(t, observer.Await(1*time.Second), "await with timeout should return true")
	// assertTrue(t, observer.Await(), "await should return true")

	// TODO: Implement AwaitAsync
	// future := observer.AwaitAsync()
	// assertTrue(t, future.IsDone(), "Future should be done")
	// assertFalse(t, future.IsCompletedExceptionally(), "Future should not be exceptional")

	iw.Close()
	t.Skip("ForceMergeDeletes with observer and AwaitAsync not yet implemented")
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
	t.Skip("LogMergePolicy with SetMaxMergeDocs not yet implemented")
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
							err := writer.AddDocument(doc)
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
	t.Skip("Concurrent merge scheduler and commit on close not fully implemented")
}

// TestIndexWriterMerging_AddEstimatedBytesToMerge tests estimated bytes tracking.
// Ported from: TestIndexWriterMerging.testAddEstimatedBytesToMerge()
func TestIndexWriterMerging_AddEstimatedBytesToMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// TODO: Set NoMergePolicy when available
	// config.SetMergePolicy(index.NewNoMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	doc := &testDocument{fields: []interface{}{}}
	// TODO: Add text field
	// doc.fields = append(doc.fields, document.NewTextField("field", "content", true))

	for i := 0; i < 10; i++ {
		writer.AddDocument(doc)
	}
	// TODO: Implement Flush when available
	// writer.Flush()

	// TODO: Implement CloneSegmentInfos and OneMerge
	// segmentInfos := writer.CloneSegmentInfos()
	// merge := index.NewOneMerge(segmentInfos.AsList())
	// writer.AddEstimatedBytesToMerge(merge)

	// assertTrue(t, merge.EstimatedMergeBytes() > 0, "estimatedMergeBytes should be > 0")
	// assertTrue(t, merge.TotalMergeBytes() > 0, "totalMergeBytes should be > 0")
	// assertTrue(t, merge.EstimatedMergeBytes() <= merge.TotalMergeBytes(), "estimated should be <= total")
	t.Skip("CloneSegmentInfos, OneMerge, and AddEstimatedBytesToMerge not yet implemented")
}

// Helper functions

// createIDDocument creates a document with an "id" field
func createIDDocument(id int) index.Document {
	fields := make([]interface{}, 0, 1)
	// TODO: Create StringField when available
	// field, _ := document.NewStringField("id", fmt.Sprintf("%d", id), false)
	// fields = append(fields, field)
	_ = id // Placeholder
	return &testDocument{fields: fields}
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
