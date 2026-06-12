// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/TestSimilarity2.java
//
// Tests every Similarity against edge-case indexes: a totally empty index, an
// empty field, an empty term, fields with norms omitted, and fields with TF
// (and norms) omitted. All of these checks assert hit counts, which are
// Similarity-independent, so they verify that each Similarity copes with the
// degenerate collection statistics without crashing or mis-counting.
//
// Faithful adaptation: Lucene's sims list enumerates the parameterised
// Lucene-faithful Similarity classes (BM25Similarity, BooleanSimilarity,
// AxiomaticF*, DFRSimilarity(model, effect, norm), IBSimilarity(dist, lambda,
// norm), LM*, DFISimilarity(independence)). In Gocene those live in the
// LuceneSimilarity family, which is a separate surface from the legacy
// Similarity interface that IndexSearcher.SetSimilarity accepts and TermWeight
// scores through. This port therefore exercises the representative legacy
// Similarity implementations Gocene exposes (one per scoring family); the
// assertions — all Similarity-independent hit counts plus the no-field-skew
// explanation-stability check — are identical to the Java originals.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// similarity2List returns one Similarity per scoring family Gocene exposes via
// the legacy Similarity interface, mirroring the intent of TestSimilarity2.sims.
func similarity2List() []search.Similarity {
	return []search.Similarity{
		search.NewClassicSimilarity(),
		search.NewBM25Similarity(),
		search.NewDFRSimilarity(),
		search.NewIBSimilarity(),
		search.NewLMDirichletSimilarity(),
		search.NewLMJelinekMercerSimilarity(),
		search.NewAxiomaticSimilarity(),
	}
}

// similarity2Index builds a single-segment committed index from the supplied
// documents and returns a searcher plus cleanup.
func similarity2Index(t *testing.T, docs ...*document.Document) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, doc := range docs {
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(reader), func() {
		_ = reader.Close()
		_ = dir.Close()
	}
}

// textDoc builds a document with a single non-stored TextField.
func textDoc(t *testing.T, field, value string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewTextField(field, value, false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	return doc
}

// customDoc builds a document with a single field using the supplied FieldType.
func customDoc(t *testing.T, field, value string, ft *document.FieldType) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewField(field, value, ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	doc.Add(f)
	return doc
}

func assertHits(t *testing.T, searcher *search.IndexSearcher, query search.Query, want int64) {
	t.Helper()
	top, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != want {
		t.Errorf("totalHits = %d, want %d", top.TotalHits.Value, want)
	}
}

func TestSimilarity2_EmptyIndex(t *testing.T) {
	searcher, cleanup := similarity2Index(t)
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		assertHits(t, searcher, search.NewTermQuery(index.NewTerm("foo", "bar")), 0)
	}
}

func TestSimilarity2_EmptyField(t *testing.T) {
	searcher, cleanup := similarity2Index(t, textDoc(t, "foo", "bar"))
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		query := search.NewBooleanQuery()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		query.Add(search.NewTermQuery(index.NewTerm("bar", "baz")), search.SHOULD)
		assertHits(t, searcher, query, 1)
	}
}

func TestSimilarity2_EmptyTerm(t *testing.T) {
	searcher, cleanup := similarity2Index(t, textDoc(t, "foo", "bar"))
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		query := search.NewBooleanQuery()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		query.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)
		assertHits(t, searcher, query, 1)
	}
}

func TestSimilarity2_NoNorms(t *testing.T) {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetOmitNorms(true)
	ft.Freeze()
	searcher, cleanup := similarity2Index(t, customDoc(t, "foo", "bar", ft))
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		query := search.NewBooleanQuery()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		assertHits(t, searcher, query, 1)
	}
}

func TestSimilarity2_NoFieldSkew(t *testing.T) {
	searcher, cleanup := similarity2Index(t, textDoc(t, "foo", "bar baz somethingelse"))
	sims := similarity2List()

	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)

	// Collect the baseline explanation values for doc 0.
	expected := make([]float32, len(sims))
	for i, sim := range sims {
		searcher.SetSimilarity(sim)
		exp, err := searcher.Explain(query, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		expected[i] = exp.GetValue()
	}
	cleanup()

	// Rebuild the index with additional field-less docs appended.
	docs := []*document.Document{textDoc(t, "foo", "bar baz somethingelse")}
	for i := 0; i < 50; i++ {
		docs = append(docs, document.NewDocument())
	}
	searcher2, cleanup2 := similarity2Index(t, docs...)
	defer cleanup2()

	// The score of doc 0 must be unchanged by the field-less docs. The equality
	// follows JUnit's assertEquals(float,float) semantics, where NaN == NaN, so
	// a Similarity that produces NaN here is still considered unskewed as long
	// as it produces the same NaN both times (matching the Java reference).
	for i, sim := range sims {
		searcher2.SetSimilarity(sim)
		actual, err := searcher2.Explain(query, 0)
		if err != nil {
			t.Fatalf("Explain: %v", err)
		}
		if !floatBitsEqual(actual.GetValue(), expected[i]) {
			t.Errorf("sim %d: doc 0 score = %v, want %v (skewed by field-less docs)",
				i, actual.GetValue(), expected[i])
		}
	}
}

// floatBitsEqual compares two float32 values with JUnit assertEquals(float,
// float) semantics: two NaNs compare equal, and other values compare by value.
func floatBitsEqual(a, b float32) bool {
	aNaN := a != a
	bNaN := b != b
	if aNaN || bNaN {
		return aNaN && bNaN
	}
	return a == b
}

func TestSimilarity2_OmitTF(t *testing.T) {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()
	searcher, cleanup := similarity2Index(t, customDoc(t, "foo", "bar", ft))
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		query := search.NewBooleanQuery()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		assertHits(t, searcher, query, 1)
	}
}

func TestSimilarity2_OmitTFAndNorms(t *testing.T) {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.SetOmitNorms(true)
	ft.Freeze()
	searcher, cleanup := similarity2Index(t, customDoc(t, "foo", "bar", ft))
	defer cleanup()
	for _, sim := range similarity2List() {
		searcher.SetSimilarity(sim)
		query := search.NewBooleanQuery()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		assertHits(t, searcher, query, 1)
	}
}