// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestExternalCodecs.java
//
// Verifies that a fully external, custom per-field codec works end to end with
// IndexWriter: documents are indexed into field1/field2/id, a document is
// deleted, term-query counts are checked, the index is force-merged, a second
// document is deleted, and the counts are re-checked.
//
// Faithful adaptation: Lucene's CustomPerFieldCodec extends AssertingCodec and
// routes "field1"/"field2"/"id" to the default postings format and everything
// else to the in-RAM "RAMOnly" format. Gocene has neither AssertingCodec nor a
// RAMOnly postings format, so this port builds an equivalent custom codec that
// embeds the production Lucene104 codec and overrides PostingsFormat() to return
// a PerFieldPostingsFormat — exercising the same external-codec / per-field
// postings dispatch path the Java test verifies. The assertions (post-delete and
// post-merge hit counts) are identical to the Java originals.

package search_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// customPerFieldCodec is the external test codec: it delegates everything to the
// production Lucene104 codec but routes postings through a PerFieldPostingsFormat.
type customPerFieldCodec struct {
	*codecs.Lucene104Codec
	perField *codecs.PerFieldPostingsFormat
}

func newCustomPerFieldCodec() *customPerFieldCodec {
	// Route every field through the production postings format via the per-field
	// dispatcher. (Lucene routes "field1"/"field2"/"id" to the default format and
	// the rest to RAMOnly; Gocene has no RAMOnly, so all fields use the default —
	// the per-field dispatch path is still exercised.)
	perField := codecs.NewPerFieldPostingsFormatWithDefault(codecs.NewLucene104PostingsFormat())
	return &customPerFieldCodec{
		Lucene104Codec: codecs.NewLucene104Codec(),
		perField:       perField,
	}
}

// PostingsFormat overrides the embedded codec to return the per-field format.
func (c *customPerFieldCodec) PostingsFormat() codecs.PostingsFormat {
	return c.perField
}

func TestExternalCodecs_PerFieldCodec(t *testing.T) {
	const numDocs = 173

	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetCodec(newCustomPerFieldCodec())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		f1, e := document.NewTextField("field1", "this field uses the standard codec as the test", false)
		if e != nil {
			t.Fatalf("NewTextField(field1): %v", e)
		}
		doc.Add(f1)
		f2, e := document.NewTextField("field2", "this field uses the memory codec as the test", false)
		if e != nil {
			t.Fatalf("NewTextField(field2): %v", e)
		}
		doc.Add(f2)
		idField, e := document.NewStringField("id", strconv.Itoa(i), false)
		if e != nil {
			t.Fatalf("NewStringField(id): %v", e)
		}
		doc.Add(idField)
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument(%d): %v", i, addErr)
		}
		if (i+1)%10 == 0 {
			if cErr := w.Commit(); cErr != nil {
				t.Fatalf("Commit: %v", cErr)
			}
		}
	}

	if err = w.DeleteDocuments(index.NewTerm("id", "77")); err != nil {
		t.Fatalf("DeleteDocuments(id:77): %v", err)
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit after delete: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if r.NumDocs() != numDocs-1 {
		t.Errorf("numDocs after first delete = %d, want %d", r.NumDocs(), numDocs-1)
	}
	s := search.NewIndexSearcher(r)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("field1", "standard")), numDocs-1)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("field2", "memory")), numDocs-1)
	if err = r.Close(); err != nil {
		t.Fatalf("reader.Close: %v", err)
	}

	if err = w.DeleteDocuments(index.NewTerm("id", "44")); err != nil {
		t.Fatalf("DeleteDocuments(id:44): %v", err)
	}
	if err = w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit after merge: %v", err)
	}

	r, err = index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (post-merge): %v", err)
	}
	if r.MaxDoc() != numDocs-2 {
		t.Errorf("maxDoc after merge = %d, want %d", r.MaxDoc(), numDocs-2)
	}
	if r.NumDocs() != numDocs-2 {
		t.Errorf("numDocs after merge = %d, want %d", r.NumDocs(), numDocs-2)
	}
	s = search.NewIndexSearcher(r)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("field1", "standard")), numDocs-2)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("field2", "memory")), numDocs-2)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("id", "76")), 1)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("id", "77")), 0)
	assertCount(t, s, search.NewTermQuery(index.NewTerm("id", "44")), 0)
	if err = r.Close(); err != nil {
		t.Fatalf("reader.Close (post-merge): %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	if err = dir.Close(); err != nil {
		t.Fatalf("dir.Close: %v", err)
	}
}

// assertCount asserts that query matches exactly want documents.
func assertCount(t *testing.T, s *search.IndexSearcher, query search.Query, want int) {
	t.Helper()
	top, err := s.Search(query, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if int(top.TotalHits.Value) != want {
		t.Errorf("count(%v) = %d, want %d", query, top.TotalHits.Value, want)
	}
}
