// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// TestIndexWriter_TermVectorsRoundTrip tests AC#1 for T4644:
// Writing a document with a TermVectors-enabled field and reading it back
// via SegmentReader.GetTermVectors (called through DirectoryReader) recovers
// the same Fields with the same term text.

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// tvTextField builds a document.Field with term vectors and positions enabled.
func tvTextField(name, value string) (*document.Field, error) {
	ft := document.NewFieldType()
	ft.SetIndexed(true)
	ft.SetStored(false)
	ft.SetTokenized(true)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.SetStoreTermVectors(true)
	ft.StoreTermVectorPositions = true
	ft.Freeze()
	return document.NewField(name, value, ft)
}

// TestIndexWriter_TermVectorsRoundTrip writes one document with two term-vector
// fields ("title" and "body"), commits, reopens via OpenDirectoryReader, and
// verifies that SegmentReader.GetTermVectors(0) returns a Fields containing the
// expected terms in each field.
func TestIndexWriter_TermVectorsRoundTrip(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetUseCompoundFile(false) // Keep loose files so we can verify .tvd is produced.
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()

	titleField, err := tvTextField("title", "hello world")
	if err != nil {
		t.Fatalf("tvTextField(title): %v", err)
	}
	bodyField, err := tvTextField("body", "foo bar baz")
	if err != nil {
		t.Fatalf("tvTextField(body): %v", err)
	}
	doc.Add(titleField)
	doc.Add(bodyField)

	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify that the term vectors data file was written (UseCompoundFile=false
	// keeps loose files so we can check directly).
	allFiles, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	hasTVD := false
	for _, f := range allFiles {
		if len(f) > 4 && f[len(f)-4:] == ".tvd" {
			hasTVD = true
			break
		}
	}
	if !hasTVD {
		t.Errorf("no .tvd file produced after Commit; term vectors writer not wired (all files: %v)", allFiles)
	}

	// Reopen the index.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		// Pre-existing: Lucene104PostingsReader requires _0.psm which the current
		// flushPostings path does not produce. Skip rather than fail so that the
		// postings-writer gap is tracked separately.
		t.Fatalf("OpenDirectoryReader: %v (pre-existing postings gap — tracked separately)", err)
	}
	defer reader.Close()

	// GetTermVectors must return non-nil Fields for doc 0.
	tvFields, err := reader.GetTermVectors(0)
	if err != nil {
		t.Fatalf("GetTermVectors(0): %v", err)
	}
	if tvFields == nil {
		t.Fatal("GetTermVectors returned nil (SegmentCoreReaders TV gap); check .tvd was written")
	}

	// Verify both fields are present with the expected terms.
	checkFieldTerms(t, tvFields, "title", []string{"hello", "world"})
	checkFieldTerms(t, tvFields, "body", []string{"bar", "baz", "foo"})
}

// checkFieldTerms asserts that tvFields contains fieldName with wantTerms
// (order-insensitive).
func checkFieldTerms(t *testing.T, tvFields index.Fields, fieldName string, wantTerms []string) {
	t.Helper()
	terms, err := tvFields.Terms(fieldName)
	if err != nil {
		t.Fatalf("tvFields.Terms(%q): %v", fieldName, err)
	}
	if terms == nil {
		t.Errorf("tvFields.Terms(%q): nil (field not in term vectors)", fieldName)
		return
	}
	iter, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator for %q: %v", fieldName, err)
	}
	var got []string
	for {
		term, err := iter.Next()
		if err != nil {
			t.Fatalf("Next() for %q: %v", fieldName, err)
		}
		if term == nil {
			break
		}
		got = append(got, term.Bytes.String())
	}
	sort.Strings(got)

	want := make([]string, len(wantTerms))
	copy(want, wantTerms)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Errorf("field %q: terms %v, want %v", fieldName, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %q term[%d]: got %q, want %q", fieldName, i, got[i], want[i])
		}
	}
}
