// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// longTerm is an artificially long single term, ported verbatim from
// org.apache.lucene.index.TestReadOnlyIndex, to exercise terms that exceed
// typical token lengths.
const longTerm = "longtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongtermlongterm"

// readOnlyIndexText is the indexed document body, mirroring the Lucene test.
const readOnlyIndexText = "This is the text to be indexed. " + longTerm

// buildReadOnlyIndex creates a single-document index in dir and returns once it
// has been committed and the writer closed, so the index can be opened
// independently for read-only access.
func buildReadOnlyIndex(t *testing.T, dir store.Directory) {
	t.Helper()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, err := document.NewTextField("fieldname", readOnlyIndexText, true)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}

// TestReadOnlyIndex ports org.apache.lucene.index.TestReadOnlyIndex. It builds
// an index on a filesystem directory, then opens it strictly for reading and
// verifies term, stored-field, and phrase access.
//
// The Lucene original wraps the read phase in runWithRestrictedPermissions to
// assert no writes occur. The JVM SecurityManager has no Go equivalent; this
// port instead opens the directory through a fresh SimpleFSDirectory and only
// ever drives DirectoryReader / IndexSearcher, none of which mutate the index.
func TestReadOnlyIndex(t *testing.T) {

	indexPath := t.TempDir()

	// Build phase: create and populate the index, then release all writers.
	buildDir, err := store.NewSimpleFSDirectory(indexPath)
	if err != nil {
		t.Fatalf("Failed to open build directory: %v", err)
	}
	buildReadOnlyIndex(t, buildDir)
	if err := buildDir.Close(); err != nil {
		t.Fatalf("Failed to close build directory: %v", err)
	}

	// Read-only phase: reopen the same path through a separate directory
	// handle and search without ever creating a writer.
	dir, err := store.NewSimpleFSDirectory(indexPath)
	if err != nil {
		t.Fatalf("Failed to open read-only directory: %v", err)
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// The long term must be retrievable as a single token.
	longTermQuery := search.NewTermQuery(index.NewTerm("fieldname", longTerm))
	longTermHits, err := searcher.Search(longTermQuery, 1)
	if err != nil {
		t.Fatalf("Long-term search failed: %v", err)
	}
	if longTermHits.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for long term, got %d", longTermHits.TotalHits.Value)
	}

	// A plain term query must match the single document, and its stored
	// field value must round-trip unchanged.
	query := search.NewTermQuery(index.NewTerm("fieldname", "text"))
	hits, err := searcher.Search(query, 1)
	if err != nil {
		t.Fatalf("Term search failed: %v", err)
	}
	if hits.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for term 'text', got %d", hits.TotalHits.Value)
	}

	for _, scoreDoc := range hits.ScoreDocs {
		hitDoc, err := searcher.Doc(scoreDoc.Doc)
		if err != nil {
			t.Fatalf("Failed to load stored fields for doc %d: %v", scoreDoc.Doc, err)
		}
		values := hitDoc.GetValues("fieldname")
		if len(values) != 1 {
			t.Fatalf("Expected 1 stored value for 'fieldname', got %d", len(values))
		}
		if values[0] != readOnlyIndexText {
			t.Errorf("Stored field mismatch: got %q, want %q", values[0], readOnlyIndexText)
		}
	}

	// A phrase query over adjacent terms must match the document.
	phraseQuery := search.NewPhraseQueryWithStrings("fieldname", "to", "be")
	phraseHits, err := searcher.Search(phraseQuery, 1)
	if err != nil {
		t.Fatalf("Phrase search failed: %v", err)
	}
	if phraseHits.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for phrase 'to be', got %d", phraseHits.TotalHits.Value)
	}
}

// TestStoredFieldsRoundTrip verifies that a document containing a StringField,
// a stored TextField, and a multi-valued TextField can be written, committed,
// and read back byte-equal via IndexSearcher.Doc.
//
// This test satisfies acceptance criterion 1 of rmp task 4638.
func TestStoredFieldsRoundTrip(t *testing.T) {
	const (
		stringFieldName = "id"
		stringFieldVal  = "doc-001"
		textFieldName   = "title"
		textFieldVal    = "The quick brown fox"
		multiFieldName  = "tag"
	)
	multiValues := []string{"go", "lucene", "search"}

	indexPath := t.TempDir()

	// Write phase.
	buildDir, err := store.NewSimpleFSDirectory(indexPath)
	if err != nil {
		t.Fatalf("open build dir: %v", err)
	}

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(buildDir, config)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	doc := document.NewDocument()

	sf, err := document.NewStringField(stringFieldName, stringFieldVal, true)
	if err != nil {
		t.Fatalf("new string field: %v", err)
	}
	doc.Add(sf)

	tf, err := document.NewTextField(textFieldName, textFieldVal, true)
	if err != nil {
		t.Fatalf("new text field: %v", err)
	}
	doc.Add(tf)

	for _, v := range multiValues {
		mf, err := document.NewTextField(multiFieldName, v, true)
		if err != nil {
			t.Fatalf("new multi-value field: %v", err)
		}
		doc.Add(mf)
	}

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := buildDir.Close(); err != nil {
		t.Fatalf("close build dir: %v", err)
	}

	// Read phase: fresh directory handle, no writer involved.
	readDir, err := store.NewSimpleFSDirectory(indexPath)
	if err != nil {
		t.Fatalf("open read dir: %v", err)
	}
	defer readDir.Close()

	reader, err := index.OpenDirectoryReader(readDir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Find the document.
	q := search.NewTermQuery(index.NewTerm(stringFieldName, stringFieldVal))
	hits, err := searcher.Search(q, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hits.TotalHits.Value != 1 {
		t.Fatalf("expected 1 hit, got %d", hits.TotalHits.Value)
	}

	// Retrieve stored fields and verify byte-equal round-trip.
	retrieved, err := searcher.Doc(hits.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc(%d): %v", hits.ScoreDocs[0].Doc, err)
	}

	// StringField.
	idVals := retrieved.GetValues(stringFieldName)
	if len(idVals) != 1 || idVals[0] != stringFieldVal {
		t.Errorf("StringField %q: got %v, want [%q]", stringFieldName, idVals, stringFieldVal)
	}

	// Stored TextField.
	titleVals := retrieved.GetValues(textFieldName)
	if len(titleVals) != 1 || titleVals[0] != textFieldVal {
		t.Errorf("TextField %q: got %v, want [%q]", textFieldName, titleVals, textFieldVal)
	}

	// Multi-valued TextField: all values must be present in original order.
	tagVals := retrieved.GetValues(multiFieldName)
	if len(tagVals) != len(multiValues) {
		t.Fatalf("multi-value field %q: got %d values, want %d", multiFieldName, len(tagVals), len(multiValues))
	}
	for i, want := range multiValues {
		if tagVals[i] != want {
			t.Errorf("multi-value field %q[%d]: got %q, want %q", multiFieldName, i, tagVals[i], want)
		}
	}

	// Verify no nil-coreReaders path was taken: a second term search must
	// return a hit (would fail with "not initialized" if coreReaders were nil).
	q2 := search.NewTermQuery(index.NewTerm(multiFieldName, "lucene"))
	hits2, err := searcher.Search(q2, 1)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if hits2.TotalHits.Value != 1 {
		t.Errorf("multi-value term search: expected 1 hit, got %d", hits2.TotalHits.Value)
	}

}
