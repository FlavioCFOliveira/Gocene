// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// This file pins the final Weight.scorer(LeafReaderContext) and
// Weight.bulkScorer(LeafReaderContext) of Lucene 10.5.0 on weights that embed
// BaseWeight and override scorerSupplier: the promoted BaseWeight.Scorer and
// BaseWeight.BulkScorer called BaseWeight.ScorerSupplier (always nil), so
// e.g. FilterWeight.Scorer or BooleanWeight.Scorer returned nil for a
// matching segment. Found by the faithful port of TestMinShouldMatch2, which
// calls weight.scorer directly. FilterWeight is used because it needs no index.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// supplierOnlyWeight overrides only ScorerSupplier, as a Java Weight subclass
// that inherits the final scorer/bulkScorer.
type supplierOnlyWeight struct {
	*BaseWeight
}

func (w *supplierOnlyWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return NewDefaultScorerSupplier(NewConstantScoreScorer(1, COMPLETE, NewRangeDocIdSetIterator(0, 3))), nil
}

func (w *supplierOnlyWeight) IsCacheable(ctx *index.LeafReaderContext) bool { return false }

func TestWeight_FinalScorerAndBulkScorerRegression(t *testing.T) {
	inner := &supplierOnlyWeight{BaseWeight: NewBaseWeight(NewMatchAllDocsQuery())}
	fw := NewFilterWeight(inner)

	scorer, err := fw.Scorer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if scorer == nil {
		t.Fatal("FilterWeight.Scorer returned nil although scorerSupplier supplies one")
	}
	if doc, err := scorer.Iterator().NextDoc(); err != nil || doc != 0 {
		t.Fatalf("nextDoc = %d, %v; want 0", doc, err)
	}
	bulk, err := fw.BulkScorer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if bulk == nil {
		t.Fatal("FilterWeight.BulkScorer returned nil although scorerSupplier supplies one")
	}
}
