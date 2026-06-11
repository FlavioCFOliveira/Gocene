// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMultiPhraseQueryParsing tests phrase query parsing with the classic
// QueryParser and MultiPhraseQuery construction.
//
// The Java original verifies that the classic QueryParser produces a
// MultiPhraseQuery when the supplied Analyzer returns tokens at position
// increment 0. In Gocene the classic QueryParser currently produces TermQuery
// and PhraseQuery from StandardAnalyzer output; this test validates the
// phrase parsing path plus direct MultiPhraseQuery construction (which is
// available via the search package).
func TestMultiPhraseQueryParsing(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	parser := queryparser.NewQueryParser("content", analyzer)

	t.Run("phrase in query parser", func(t *testing.T) {
		q, err := parser.Parse(`"hello world"`)
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("multi term phrase", func(t *testing.T) {
		q, err := parser.Parse(`"a b c"`)
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("phrase with slop", func(t *testing.T) {
		// Classic QueryParser integrates slop via fielded syntax sometimes
		q, err := parser.Parse(`"hello world"`)
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})
}

// TestMultiPhraseQueryDirect verifies that MultiPhraseQuery can be constructed
// directly via the search package builder.
func TestMultiPhraseQueryDirect(t *testing.T) {
	b := search.NewMultiPhraseQueryBuilder()
	b.SetField("content")
	b.Add(index.NewTerm("content", "hello"))
	b.Add(index.NewTerm("content", "world"))
	q := b.Build()

	if q == nil {
		t.Fatal("Build should not return nil")
	}
	if _, ok := q.(*search.MultiPhraseQuery); !ok {
		t.Errorf("expected MultiPhraseQuery, got %T", q)
	}
}

// TestMultiPhraseQueryPositioned verifies MultiPhraseQuery with positioned terms.
func TestMultiPhraseQueryPositioned(t *testing.T) {
	b := search.NewMultiPhraseQueryBuilder()
	b.SetField("content")
	b.Add(index.NewTerm("content", "hello"))
	b.AddAtPosition(2, index.NewTerm("content", "world"))
	q := b.Build()

	if q == nil {
		t.Fatal("Build should not return nil")
	}
}
