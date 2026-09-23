// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestReaderClosed.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// readerClosedSetUp ports setUp(): it returns the reader and the directory
// that tearDown() closes.
func readerClosedSetUp(t *testing.T) (*index.DirectoryReader, *store.MockDirectoryWrapper) {
	t.Helper()
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	r := rand.New(rand.NewSource(rand.Int63()))
	writer, err := testindex.NewRandomIndexWriterWithConfig(r, dir, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field := newStringField(t, "field", "", false)
	doc.Add(field)

	// we generate aweful prefixes: good for testing.
	// but for preflex codec, the test can be very slow, so use less iterations.
	num := atLeast(10)
	for i := 0; i < num; i++ {
		field.SetStringValue(util.RandomUnicodeString(r, 10))
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, writer)
	return reader, dir
}

func TestReaderClosed(t *testing.T) {
	reader, dir := readerClosedSetUp(t)
	defer mustClose(t, dir) // tearDown()
	if !(reader.GetRefCount() > 0) {
		t.Fatal("assertTrue(reader.getRefCount() > 0)")
	}
	searcher := search.NewIndexSearcher(reader)
	query := search.NewStringRange("field", "a", "z", true, true)
	if _, err := searcher.Search(query, 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	mustClose(t, reader)
	if _, err := searcher.Search(query, 5); err != nil {
		var ace *store.AlreadyClosedException
		if !errors.As(err, &ace) {
			t.Fatalf("search after close: %v", err)
		}
		// expected
	}
}

// TestReaderClosedReaderChaining ports testReaderChaining (LUCENE-3800), which
// searches a ParallelLeafReader wrapped in the test-framework
// OwnCacheKeyMultiReader.
func TestReaderClosedReaderChaining(t *testing.T) {
	reader, dir := readerClosedSetUp(t)
	defer mustClose(t, dir) // tearDown()
	if !(reader.GetRefCount() > 0) {
		t.Fatal("assertTrue(reader.getRefCount() > 0)")
	}
	mustClose(t, reader)
	t.Fatal("org.apache.lucene.tests.index.OwnCacheKeyMultiReader is not ported")
}
