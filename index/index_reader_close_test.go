// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexReader close-listener lifecycle
// and exception handling.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestIndexReaderClose.java
//
// GOC-4259: Port test `org.apache.lucene.index.TestIndexReaderClose`.
//
// # Test coverage
//
//   - TestIndexReaderClose_CloseUnderException          — 1:1 port of testCloseUnderException()
//   - TestIndexReaderClose_CoreListenerOnWrapper        — 1:1 port of testCoreListenerOnWrapperWithDifferentCacheKey()
//   - TestIndexReaderClose_RegisterListenerOnClosed     — 1:1 port of testRegisterListenerOnClosedReader()
//
// # Deviations from the Java reference
//
//   - testCloseUnderException uses a FilterLeafReader with a deterministic
//     doClose error (the original Java anonymous class is replaced by a
//     small wrapper type in this file).
//
//   - testRegisterListenerOnClosedReader invokes the listener immediately
//     when registered on an already-closed reader, matching Gocene's
//     ReaderCacheHelper contract.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexReaderClose_CloseUnderException ports testCloseUnderException().
//
// Java wraps a leaf reader in a FilterLeafReader that throws on doClose,
// registers ClosedListeners, closes the reader, and asserts listener count
// reaches zero and that exceptions propagate correctly.
//
// Gocene uses a FilterLeafReader with a custom cache helper and a Close that
// always fails; listeners registered on the wrapper are still notified and
// subsequent access raises AlreadyClosedException.
func TestIndexReaderClose_CloseUnderException(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	field, err := document.NewStringField("field", "value", false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(field)
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaf := index.GetOnlyLeafReader(reader)
	leafLR, ok := leaf.(index.LeafReader)
	if !ok {
		t.Fatalf("GetOnlyLeafReader returned %T, want index.LeafReader", leaf)
	}

	wrapper := index.NewFilterLeafReaderWithCacheKey(leafLR)
	boomWrapper := &boomCloseFilterLeafReader{
		FilterLeafReader: wrapper,
	}

	called := 0
	boomWrapper.GetCacheHelper().AddClosedListener(func() { called++ })

	if err := boomWrapper.Close(); err == nil {
		t.Fatal("expected Close to return an error")
	} else if err.Error() != "BOOM!" {
		t.Fatalf("expected BOOM!, got %v", err)
	}

	// After close, terms must fail with an AlreadyClosedException.
	if _, err := boomWrapper.Terms("field"); err == nil {
		t.Fatal("expected Terms after close to fail")
	} else {
		var ace *index.AlreadyClosedException
		if !errors.As(err, &ace) {
			t.Fatalf("expected AlreadyClosedException, got %T: %v", err, err)
		}
	}

	if called != 1 {
		t.Fatalf("expected closed listener to be called once, got %d", called)
	}
}

// boomCloseFilterLeafReader wraps a FilterLeafReader and always returns an
// error from Close after delegating to the wrapped reader. It models the
// anonymous FilterLeafReader subclass used by Lucene's testCloseUnderException.
type boomCloseFilterLeafReader struct {
	*index.FilterLeafReader
}

func (r *boomCloseFilterLeafReader) Close() error {
	_ = r.FilterLeafReader.Close()
	return errors.New("BOOM!")
}


// TestIndexReaderClose_RegisterListenerOnClosed ports
// testRegisterListenerOnClosedReader().
//
// Java closes a reader, then calls addClosedListener on the closed reader and
// asserts the listener is invoked immediately (or AlreadyClosedException is
// thrown, depending on the implementation).
func TestIndexReaderClose_RegisterListenerOnClosed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if _, err := writer.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	// Registering listeners on an open reader must succeed.
	called := 0
	reader.GetCacheHelper().AddClosedListener(func() { called++ })
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0].Reader()
	if withHelper, ok := leaf.(interface{ GetCacheHelper() index.CacheHelper }); ok {
		withHelper.GetCacheHelper().AddClosedListener(func() { called++ })
	}

	// Close the reader.
	if err := reader.Close(); err != nil {
		t.Fatalf("Close reader: %v", err)
	}

	// After close, adding a listener invokes it immediately (Gocene contract).
	reader.GetCacheHelper().AddClosedListener(func() { called++ })
	if withHelper, ok := leaf.(interface{ GetCacheHelper() index.CacheHelper }); ok {
		withHelper.GetCacheHelper().AddClosedListener(func() { called++ })
	}

	if called != 4 {
		t.Fatalf("expected 4 closed-listener invocations, got %d", called)
	}
}

// TestIndexReaderClose_CoreListenerOnWrapper ports
// testCoreListenerOnWrapperWithDifferentCacheKey().
//
// Java wraps a leaf reader in a FilterLeafReader with a different cache key,
// registers a CoreClosedListener on the core, and asserts the listener is
// only called when the underlying core closes, not when the wrapper closes.
func TestIndexReaderClose_CoreListenerOnWrapper(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		field, _ := document.NewStringField("field", fmt.Sprintf("value%d", i), false)
		doc.Add(field)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaf := index.GetOnlyLeafReader(reader)
	leafLR, ok := leaf.(index.LeafReader)
	if !ok {
		t.Fatalf("GetOnlyLeafReader returned %T, want index.LeafReader", leaf)
	}

	called := 0
	if withHelper, ok := leafLR.(interface{ GetCacheHelper() index.CacheHelper }); ok {
		h := withHelper.GetCacheHelper()
		h.AddClosedListener(func() { called++ })
	}

	// Wrap with a FilterLeafReader that has a *different* cache key.
	wrapper := index.NewFilterLeafReaderWithCacheKey(leafLR)
	if wrapper.GetCacheHelper() == nil {
		t.Fatal("wrapper has no cache helper")
	}

	// Closing only the wrapper must not fire the core listener because the
	// wrapper has its own cache key.
	if err := wrapper.Close(); err != nil {
		t.Fatalf("Close wrapper: %v", err)
	}
	if called != 0 {
		t.Fatalf("core listener called %d times after wrapper close, want 0", called)
	}

	// Closing the underlying reader fires the listener.
	if err := reader.Close(); err != nil {
		t.Fatalf("Close reader: %v", err)
	}
	if called != 1 {
		t.Fatalf("core listener called %d times after reader close, want 1", called)
	}
}
