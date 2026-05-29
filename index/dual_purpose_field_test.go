// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test holds the on-disk round-trip acceptance test for rmp
// #4780: when a single field name carries BOTH an indexed contribution (e.g.
// StringField) AND a DocValues contribution (e.g. SortedDocValuesField /
// NumericDocValuesField) within one Document, the persisted FieldInfo must
// retain both the IndexOptions and the DocValuesType, the indexed term must be
// queryable, and the DocValues must read back.
//
// Mirrors org.apache.lucene.index.IndexingChain.FieldSchema accumulation
// feeding FieldInfos.Builder.add: per-field options accumulate across the
// document's IndexableFields rather than the last writer overwriting the first.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default; flushDocValues is a no-op without a codec.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// newRoundTripDir returns a Directory of the requested kind for the
// dual-purpose-field round-trips. SimpleFS exercises the on-disk wire format;
// ByteBuffers exercises the in-memory path. Both must preserve the combined
// FieldInfo flags.
func newRoundTripDir(t *testing.T, kind string) store.Directory {
	t.Helper()
	switch kind {
	case "SimpleFS":
		d, err := store.NewSimpleFSDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("NewSimpleFSDirectory: %v", err)
		}
		return d
	default:
		return store.NewByteBuffersDirectory()
	}
}

// TestDualPurposeField_SortedDocValuesPlusIndexed indexes one document whose
// single field name "f" carries a stored, indexed StringField AND a
// SortedDocValuesField, commits, reopens, and verifies that (1) TermQuery(f:x)
// matches the doc, (2) GetSortedDocValues("f") returns the written bytes, and
// (3) the reopened FieldInfo reports both a non-NONE IndexOptions and a
// non-NONE DocValuesType.
func TestDualPurposeField_SortedDocValuesPlusIndexed(t *testing.T) {
	for _, dirName := range []string{"ByteBuffers", "SimpleFS"} {
		dirName := dirName
		t.Run(dirName, func(t *testing.T) {
			dir := newRoundTripDir(t, dirName)

			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			sf, err := document.NewStringField("f", "x", true)
			if err != nil {
				t.Fatalf("NewStringField: %v", err)
			}
			sdv, err := document.NewSortedDocValuesField("f", []byte("alpha"))
			if err != nil {
				t.Fatalf("NewSortedDocValuesField: %v", err)
			}
			doc := &testDocument{fields: []interface{}{sf, sdv}}
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			segs := reader.GetSegmentReaders()
			if len(segs) != 1 {
				t.Fatalf("expected 1 segment, got %d", len(segs))
			}
			r := segs[0]

			// (3) FieldInfo carries both the index options and the DV type.
			fi := r.GetFieldInfos().GetByName("f")
			if fi == nil {
				t.Fatal("FieldInfo for 'f' is nil")
			}
			if fi.IndexOptions() == index.IndexOptionsNone {
				t.Errorf("IndexOptions = NONE, want indexed (dual-purpose field dropped the indexed contribution)")
			}
			if fi.DocValuesType() != index.DocValuesTypeSorted {
				t.Errorf("DocValuesType = %s, want SORTED (dual-purpose field dropped the DocValues contribution)", fi.DocValuesType())
			}

			// (2) DocValues read back.
			dv, err := r.GetSortedDocValues("f")
			if err != nil {
				t.Fatalf("GetSortedDocValues: %v", err)
			}
			if dv == nil {
				t.Fatal("GetSortedDocValues returned nil (DocValues not written)")
			}
			doc0, err := dv.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc0 != 0 {
				t.Fatalf("NextDoc = %d, want 0", doc0)
			}
			ord, err := dv.OrdValue()
			if err != nil {
				t.Fatalf("OrdValue: %v", err)
			}
			term, err := dv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("LookupOrd: %v", err)
			}
			if string(term) != "alpha" {
				t.Fatalf("Sorted DV term = %q, want %q", term, "alpha")
			}

			// (1) The indexed term is queryable.
			searcher := search.NewIndexSearcher(reader)
			td, err := searcher.Search(search.NewTermQuery(index.NewTerm("f", "x")), 10)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if td.TotalHits.Value != 1 {
				t.Fatalf("TermQuery(f:x) TotalHits = %d, want 1 (indexing not preserved)", td.TotalHits.Value)
			}
			if len(td.ScoreDocs) != 1 || td.ScoreDocs[0].Doc != 0 {
				t.Fatalf("TermQuery(f:x) ScoreDocs = %v, want [doc 0]", td.ScoreDocs)
			}
		})
	}
}

// TestDualPurposeField_NumericDocValuesPlusIndexed is the numeric variant: one
// field name "n" carries a stored, indexed StringField AND a
// NumericDocValuesField. After reopen the term must match and
// GetNumericDocValues("n") must return the written value, with the FieldInfo
// reporting both IndexOptions != NONE and DocValuesType == NUMERIC.
func TestDualPurposeField_NumericDocValuesPlusIndexed(t *testing.T) {
	for _, dirName := range []string{"ByteBuffers", "SimpleFS"} {
		dirName := dirName
		t.Run(dirName, func(t *testing.T) {
			dir := newRoundTripDir(t, dirName)

			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			sf, err := document.NewStringField("n", "k", true)
			if err != nil {
				t.Fatalf("NewStringField: %v", err)
			}
			ndv, err := document.NewNumericDocValuesField("n", 42)
			if err != nil {
				t.Fatalf("NewNumericDocValuesField: %v", err)
			}
			doc := &testDocument{fields: []interface{}{sf, ndv}}
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			r := reader.GetSegmentReaders()[0]

			fi := r.GetFieldInfos().GetByName("n")
			if fi == nil {
				t.Fatal("FieldInfo for 'n' is nil")
			}
			if fi.IndexOptions() == index.IndexOptionsNone {
				t.Errorf("IndexOptions = NONE, want indexed")
			}
			if fi.DocValuesType() != index.DocValuesTypeNumeric {
				t.Errorf("DocValuesType = %s, want NUMERIC", fi.DocValuesType())
			}

			dv, err := r.GetNumericDocValues("n")
			if err != nil {
				t.Fatalf("GetNumericDocValues: %v", err)
			}
			if dv == nil {
				t.Fatal("GetNumericDocValues returned nil")
			}
			if _, err := dv.NextDoc(); err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			v, err := dv.LongValue()
			if err != nil {
				t.Fatalf("LongValue: %v", err)
			}
			if v != 42 {
				t.Fatalf("Numeric DV value = %d, want 42", v)
			}

			searcher := search.NewIndexSearcher(reader)
			td, err := searcher.Search(search.NewTermQuery(index.NewTerm("n", "k")), 10)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if td.TotalHits.Value != 1 {
				t.Fatalf("TermQuery(n:k) TotalHits = %d, want 1", td.TotalHits.Value)
			}
		})
	}
}
