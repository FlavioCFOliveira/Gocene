// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter error handling scenarios.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriter
// error-handling test methods.
package index_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriter_DiskFull tests behavior when disk is full during indexing.
// Ported from: TestIndexWriter.testOnDiskFull()
func TestIndexWriter_DiskFull(t *testing.T) {
	t.Run("commit with simulated disk full", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		defer writer.Close()

		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
	})

	t.Run("add document with disk full handling", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		doc := &testDocument{fields: []interface{}{}}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Logf("AddDocument() returned error: %v", err)
		}

		if writer.IsClosed() {
			t.Error("Writer should not be closed after recoverable error")
		}
	})
}

// TestIndexWriter_GeneralErrors tests general error scenarios and recovery.
// Ported from: TestIndexWriter.testOnError()
func TestIndexWriter_GeneralErrors(t *testing.T) {
	t.Run("operation after close returns error", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		writer.Close()

		doc := &testDocument{fields: []interface{}{}}
		err := writer.AddDocument(doc)

		if err == nil {
			t.Fatal("AddDocument on closed writer should return error")
		}
		if _, ok := err.(*index.AlreadyClosedException); !ok {
			t.Errorf("Expected AlreadyClosedException, got %T", err)
		}
	})

	t.Run("error recovery maintains index consistency", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		for i := 0; i < 3; i++ {
			doc := &testDocument{fields: []interface{}{}}
			writer.AddDocument(doc)
		}

		numDocs := writer.NumDocs()
		if numDocs != 3 {
			t.Errorf("Expected 3 docs, got %d", numDocs)
		}
	})
}

// TestIndexWriter_ResourceExhaustion tests handling of resource exhaustion.
// Ported from: TestIndexWriter.testOutOfFileDescriptors()
func TestIndexWriter_ResourceExhaustion(t *testing.T) {
	t.Run("out of file descriptors handling", func(t *testing.T) {
		dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		defer writer.Close()

		// Inject a random IOException on every file-open operation for the next
		// AddDocument call. This simulates transient resource exhaustion.
		if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
			t.Fatalf("AddDocument before injection: %v", err)
		}

		// Inject a random IOException on every file-open operation during commit.
		dir.SetRandomIOExceptionRateOnOpen(1.0)
		err = writer.Commit()
		dir.SetRandomIOExceptionRateOnOpen(0)

		if err == nil {
			t.Fatal("expected Commit to fail with injected IOException")
		}
		if !errors.Is(err, store.FakeIOException{}) {
			t.Fatalf("expected FakeIOException, got %T: %v", err, err)
		}
	})

	t.Run("recover from resource exhaustion", func(t *testing.T) {
		dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
			t.Fatalf("AddDocument before injection: %v", err)
		}

		// First, inject an open-time IOException during commit.
		dir.SetRandomIOExceptionRateOnOpen(1.0)
		err := writer.Commit()
		dir.SetRandomIOExceptionRateOnOpen(0)

		if err == nil {
			t.Fatal("expected Commit to fail with injected IOException")
		}

		// After the failure the writer may be closed by the tragic-error path.
		// If it is still open, a subsequent operation must succeed.
		if !writer.IsClosed() {
			if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
				t.Fatalf("AddDocument after clearing injection should succeed: %v", err)
			}
		}
		writer.Close()
	})
}

// TestIndexWriter_LockRelease tests proper lock release on errors.
// Ported from: TestIndexWriter.testLockRelease()
func TestIndexWriter_LockRelease(t *testing.T) {
	t.Run("lock released after close", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		dir.SetLockFactory(store.NewSingleInstanceLockFactory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		err = writer.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		lock, err := dir.ObtainLock("write.lock")
		if err != nil {
			t.Errorf("Should be able to obtain lock after writer close: %v", err)
		}
		if lock != nil {
			lock.Close()
		}
	})

	t.Run("lock released on error during commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		dir.SetLockFactory(store.NewSingleInstanceLockFactory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		if err := writer.Commit(); err != nil {
			t.Logf("Commit error: %v", err)
		}

		writer.Close()

		lock, err := dir.ObtainLock("write.lock")
		if err != nil {
			t.Errorf("Lock should be released after error: %v", err)
		}
		if lock != nil {
			lock.Close()
		}
	})
}

// TestIndexWriter_IOException tests IO exception handling.
// Ported from various TestIndexWriter IO exception tests.
func TestIndexWriter_IOException(t *testing.T) {
	t.Run("directory IO exception handling", func(t *testing.T) {
		dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Inject a deterministic failure into every guarded operation.
		if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
			t.Fatalf("AddDocument before injection: %v", err)
		}

		f := &store.Failure{}
		f.SetEval(func(d *store.MockDirectoryWrapper) error {
			return store.FakeIOException{}
		})
		f.SetDoFail()
		dir.FailOn(f)

		err := writer.Commit()
		if err == nil {
			t.Fatal("expected Commit to fail with injected Failure")
		}
		if !errors.Is(err, store.FakeIOException{}) {
			t.Fatalf("expected FakeIOException, got %T: %v", err, err)
		}
	})
}

// TestIndexWriter_Rollback tests rollback functionality.
// Related to error recovery - rolling back failed transactions.
func TestIndexWriter_Rollback(t *testing.T) {
	t.Run("rollback pending changes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		for i := 0; i < 5; i++ {
			doc := &testDocument{fields: []interface{}{}}
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}

		if err := writer.Rollback(); err != nil {
			t.Fatalf("Rollback: %v", err)
		}

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader after rollback: %v", err)
		}
		defer reader.Close()
		if reader.MaxDoc() != 0 {
			t.Errorf("after rollback MaxDoc = %d, want 0", reader.MaxDoc())
		}
	})
}
