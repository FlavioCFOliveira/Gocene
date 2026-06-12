// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestDateSort.java
//
// Tests auto-sorting of a date field encoded as a sortable "long" string
// (DateTools.timeToString at SECOND resolution, stored as SortedDocValues).
// The query TermQuery(text:"document") matches all five docs; a reverse STRING
// sort on the dateTime field must return them newest-first (Document 5..1).

package search_test

import (
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	dateSortTextField     = "text"
	dateSortDateTimeField = "dateTime"
)

// createDateSortDocument ports TestDateSort.createDocument: a stored, analyzed
// text field plus a stored dateTime string and its SortedDocValues sort key.
func createDateSortDocument(t *testing.T, text string, timeMillis int64) *document.Document {
	t.Helper()
	doc := document.NewDocument()

	textField, err := document.NewTextField(dateSortTextField, text, true) // Field.Store.YES
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(textField)

	// DateTools.timeToString(time, SECOND) is computed in GMT, so normalise to UTC.
	dateTimeString := document.TimeToString(time.UnixMilli(timeMillis).UTC(), document.ResolutionSecond)
	dtField, err := document.NewStringField(dateSortDateTimeField, dateTimeString, true) // Field.Store.YES
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(dtField)
	sdv, err := document.NewSortedDocValuesField(dateSortDateTimeField, []byte(dateTimeString))
	if err != nil {
		t.Fatalf("NewSortedDocValuesField: %v", err)
	}
	doc.Add(sdv)

	return doc
}

// TestDateSort_TestDateSort ports TestDateSort.testReverseDateSort.
func TestDateSort_TestDateSort(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Times from the Lucene test (oldest first).
	docs := []struct {
		text string
		time int64
	}{
		{"Document 1", 1192001122000},
		{"Document 2", 1192001126000},
		{"Document 3", 1192101133000},
		{"Document 4", 1192104129000},
		{"Document 5", 1192209943000},
	}
	for _, d := range docs {
		if err := w.AddDocument(createDateSortDocument(t, d.text, d.time)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)

	// Reverse STRING sort on the dateTime field → newest first.
	sort := search.NewSort(search.NewSortFieldReverse(dateSortDateTimeField, search.SortFieldTypeString))
	query := search.NewTermQuery(index.NewTerm(dateSortTextField, "document"))

	top, err := searcher.SearchWithSort(query, 1000, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	expectedOrder := []string{"Document 5", "Document 4", "Document 3", "Document 2", "Document 1"}
	if len(top.ScoreDocs) != len(expectedOrder) {
		t.Fatalf("hit count: got %d want %d", len(top.ScoreDocs), len(expectedOrder))
	}

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	for i, sd := range top.ScoreDocs {
		visitor := search.NewDocumentVisitor()
		if err := storedFields.Document(sd.Doc, visitor); err != nil {
			t.Fatalf("Document(%d): %v", sd.Doc, err)
		}
		gotField := visitor.Document().Get(dateSortTextField)
		if gotField == nil {
			t.Fatalf("hit %d: stored text field missing", i)
		}
		if got := gotField.StringValue(); got != expectedOrder[i] {
			t.Fatalf("hit %d: got %q want %q", i, got, expectedOrder[i])
		}
	}
}
