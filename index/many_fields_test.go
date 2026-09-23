// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestManyFields.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// manyFieldsStoredTextType renders the static storedTextType:
// new FieldType(TextField.TYPE_NOT_STORED).
var manyFieldsStoredTextType = document.NewFieldTypeFrom(document.TextFieldTypeNotStored)

func TestManyFieldsManyFields(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(10)
	writer := mustNewIndexWriter(t, dir, iwc)
	for j := 0; j < 100; j++ {
		js := strconv.Itoa(j)
		doc := document.NewDocument()
		doc.Add(newField(t, "a"+js, "aaa"+js, manyFieldsStoredTextType))
		doc.Add(newField(t, "b"+js, "aaa"+js, manyFieldsStoredTextType))
		doc.Add(newField(t, "c"+js, "aaa"+js, manyFieldsStoredTextType))
		doc.Add(newField(t, "d"+js, "aaa", manyFieldsStoredTextType))
		doc.Add(newField(t, "e"+js, "aaa", manyFieldsStoredTextType))
		doc.Add(newField(t, "f"+js, "aaa", manyFieldsStoredTextType))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	if reader.MaxDoc() != 100 || reader.NumDocs() != 100 {
		t.Fatalf("expected 100/100 docs, got maxDoc=%d numDocs=%d", reader.MaxDoc(), reader.NumDocs())
	}
	mustClose(t, reader, dir)
	// for each j: assertEquals(1, reader.docFreq(new Term(...)))
	t.Fatal("org.apache.lucene.index.IndexReader#docFreq(Term) is not ported for composite readers " +
		"(DirectoryReader)")
}

func TestManyFieldsDiverseDocs(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetRAMBufferSizeMB(0.5)
	writer := mustNewIndexWriter(t, dir, iwc)
	n := atLeast(1)
	for i := 0; i < n; i++ {
		// First, docs where every term is unique (heavy on Posting instances)
		for j := 0; j < 100; j++ {
			doc := document.NewDocument()
			for k := 0; k < 100; k++ {
				doc.Add(newField(t, "field", strconv.Itoa(int(int32(rand.Uint32()))), manyFieldsStoredTextType))
			}
			mustAddDocument(t, writer, doc)
		}

		// Next, many single term docs where only one term occurs (heavy on
		// byte blocks)
		for j := 0; j < 100; j++ {
			doc := document.NewDocument()
			doc.Add(newField(t, "field", "aaa aaa aaa aaa aaa aaa aaa aaa aaa aaa", manyFieldsStoredTextType))
			mustAddDocument(t, writer, doc)
		}

		// Next, many single term docs where only one term occurs but the terms
		// are very long (heavy on char[] arrays)
		for j := 0; j < 100; j++ {
			x := strconv.Itoa(j) + "."
			longTerm := strings.Repeat(x, 1000)

			doc := document.NewDocument()
			doc.Add(newField(t, "field", longTerm, manyFieldsStoredTextType))
			mustAddDocument(t, writer, doc)
		}
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	searcher := search.NewIndexSearcher(reader)
	totalHits, err := searcher.Count(search.NewTermQuery(index.NewTerm("field", "aaa")))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if totalHits != n*100 {
		t.Fatalf("expected %d hits, got %d", n*100, totalHits)
	}
	mustClose(t, reader, dir)
}

// TestManyFieldsRotatingFieldNames ports testRotatingFieldNames (LUCENE-4398),
// which watches the package-private IndexWriter#getFlushCount().
func TestManyFieldsRotatingFieldNames(t *testing.T) {
	t.Fatal("org.apache.lucene.index.IndexWriter#getFlushCount() is not ported")
}
