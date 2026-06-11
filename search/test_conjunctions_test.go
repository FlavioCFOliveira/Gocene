// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestConjunctions.java
//
// TestConjunctions_TermConjunctionsWithOmitTF indexes three documents (a
// StringField "title" plus a TextField "body") and, under RawTFSimilarity,
// verifies a MUST/MUST conjunction over title:nutch and body:is matches exactly
// one document with score 3 = tf(title:nutch)=1 + tf(body:is)=2, identical to the
// Java assertEquals(3F, td.scoreDocs[0].score, 0.001F).
//
// TestConjunctions_ScorerGetChildren is simplified to verify basic conjunction
// search correctness; the full scorer tree traversal test is deferred until
// the Scorer/Scorable bridge lands.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const (
	conjF1 = "title"
	conjF2 = "body"
)

// conjDoc builds a document carrying a StringField title and a TextField body.
func conjDoc(t *testing.T, v1, v2 string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f1, err := document.NewStringField(conjF1, v1, false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	f2, err := document.NewTextField(conjF2, v2, false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f1)
	doc.Add(f2)
	return doc
}

// TestConjunctions_TermConjunctionsWithOmitTF ports testTermConjunctionsWithOmitTF.
func TestConjunctions_TermConjunctionsWithOmitTF(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addDoc(conjDoc(t, "lucene", "lucene is a very popular search engine library"))
	ix.addDoc(conjDoc(t, "solr", "solr is a very popular search server and is using lucene"))
	ix.addDoc(conjDoc(t, "nutch", "nutch is an internet search engine with web crawler and is using lucene and hadoop"))
	s, cleanup := ix.searcher()
	defer cleanup()

	s.SetSimilarity(search.NewRawTFSimilarity())

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm(conjF1, "nutch")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm(conjF2, "is")), search.MUST)

	td, err := s.Search(bq, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", td.TotalHits.Value)
	}
	if got := td.ScoreDocs[0].Score; math.Abs(float64(got-3)) > 0.001 {
		t.Errorf("score = %v, want 3 (+/-0.001)", got)
	}
}

// TestConjunctions_ScorerGetChildren verifies basic MUST+FILTER conjunction
// search returns the correct matching document.
func TestConjunctions_ScorerGetChildren(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addText("field", "a b")
	s, cleanup := ix.searcher()
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.FILTER)

	top, err := s.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("totalHits = %d, want 1", top.TotalHits.Value)
	}
}
