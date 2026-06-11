// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/JustCompileSearch.java
//
// Deviation: the Java original is a compilation-check file (no @Test methods)
// that instantiates minimal stub implementations of abstract classes and
// interfaces to verify back-compat. In Go there are no abstract classes;
// interfaces are verified at compile time. This file preserves the intent by
// providing compile-time interface satisfaction checks (var _ Interface = ...)
// for the key search interfaces that the Java file exercised. Signatures follow
// Gocene's Go API rather than the Java API (no error returns on Score/DocID,
// DocIDRunEnd added, Weight has more methods, etc.).

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"testing"
)

// TestJustCompileSearch is a marker test confirming that this file compiles.
// The var _ ... statements below are the real compile-time checks.
func TestJustCompileSearch(t *testing.T) {
	// Compilation-check only; the var _ ... guard lines below verify
	// interface satisfaction at compile time.
	_ = (*justCompileCollector)(nil)
	_ = (*justCompileDISI)(nil)
	_ = (*justCompileScorer)(nil)
	_ = (*justCompileQuery)(nil)
}

// compile-time interface satisfaction checks

var _ Collector = (*justCompileCollector)(nil)
var _ DocIdSetIterator = (*justCompileDISI)(nil)
var _ Scorer = (*justCompileScorer)(nil)
var _ Query = (*justCompileQuery)(nil)

type justCompileCollector struct{}

func (c *justCompileCollector) GetLeafCollector(_ *index.LeafReaderContext) (LeafCollector, error) {
	panic("compile check only")
}
func (c *justCompileCollector) ScoreMode() ScoreMode { panic("compile check only") }

type justCompileDISI struct{}

func (d *justCompileDISI) DocID() int               { panic("compile check only") }
func (d *justCompileDISI) NextDoc() (int, error)    { panic("compile check only") }
func (d *justCompileDISI) Advance(int) (int, error) { panic("compile check only") }
func (d *justCompileDISI) Cost() int64              { panic("compile check only") }
func (d *justCompileDISI) DocIDRunEnd() int         { panic("compile check only") }

// justCompileScorer embeds justCompileDISI to satisfy the embedded DocIdSetIterator.
type justCompileScorer struct{ justCompileDISI }

func (s *justCompileScorer) Score() float32            { panic("compile check only") }
func (s *justCompileScorer) GetMaxScore(_ int) float32 { panic("compile check only") }
func (s *justCompileScorer) AdvanceShallow(int) (int, error) {
	panic("compile check only")
}
func (s *justCompileScorer) Iterator() DocIdSetIterator          { panic("compile check only") }
func (s *justCompileScorer) TwoPhaseIterator() *TwoPhaseIterator { return nil }

type justCompileQuery struct{}

func (q *justCompileQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }
func (q *justCompileQuery) Clone() Query                         { return q }
func (q *justCompileQuery) Equals(_ Query) bool                  { return false }
func (q *justCompileQuery) HashCode() int                        { return 0 }
func (q *justCompileQuery) CreateWeight(_ *IndexSearcher, _ bool, _ float32) (Weight, error) {
	return nil, nil
}

// justCompileWeight is not asserted via var _ because Weight has many methods
// (BulkScorer, Count, Matches, Scorer, ScorerSupplier, Explain, IsCacheable,
// GetQuery) and all return *index.LeafReaderContext — implementing them all
// here would duplicate Weight contract tests. The intent is covered by
// asserting the simpler interfaces above.
var _ Weight = (*justCompileWeight)(nil)

type justCompileWeight struct{}

func (w *justCompileWeight) GetQuery() Query { panic("compile check only") }
func (w *justCompileWeight) Explain(_ *index.LeafReaderContext, _ int) (Explanation, error) {
	panic("compile check only")
}
func (w *justCompileWeight) ScorerSupplier(_ *index.LeafReaderContext) (ScorerSupplier, error) {
	panic("compile check only")
}
func (w *justCompileWeight) Scorer(_ *index.LeafReaderContext) (Scorer, error) {
	panic("compile check only")
}
func (w *justCompileWeight) BulkScorer(_ *index.LeafReaderContext) (BulkScorer, error) {
	panic("compile check only")
}
func (w *justCompileWeight) IsCacheable(_ *index.LeafReaderContext) bool {
	panic("compile check only")
}
func (w *justCompileWeight) Count(_ *index.LeafReaderContext) (int, error) {
	panic("compile check only")
}
func (w *justCompileWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	panic("compile check only")
}
