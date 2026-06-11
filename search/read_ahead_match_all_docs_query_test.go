// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/ReadAheadMatchAllDocsQuery.java
//
// ReadAheadMatchAllDocsQuery is a helper Query used in TopFieldCollector
// tests to validate read-ahead compatibility. It matches all documents by
// returning a DenseConjunctionBulkScorer over a single clause.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestReadAheadMatchAllDocsQuery verifies that DenseConjunctionBulkScorer
// visits every document when given a single all-docs iterator, mirroring
// what the ReadAheadMatchAllDocsQuery helper produces.
func TestReadAheadMatchAllDocsQuery(t *testing.T) {
	maxDoc := 50

	bits, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < maxDoc; i++ {
		bits.Set(i)
	}
	allDocs := util.NewBitSetIterator(bits, int64(maxDoc))

	bs, err := NewDenseConjunctionBulkScorer([]DocIdSetIterator{allDocs}, nil, maxDoc, 1.0)
	if err != nil {
		t.Fatalf("NewDenseConjunctionBulkScorer: %v", err)
	}

	var docIDs []int
	collector := &collectAllLeafCollector{
		collectFn: func(doc int) {
			docIDs = append(docIDs, doc)
		},
	}
	if _, err := bs.Score(collector, nil, 0, maxDoc); err != nil {
		t.Fatalf("Score: %v", err)
	}

	if got := len(docIDs); got != maxDoc {
		t.Fatalf("collected %d docs, want %d", got, maxDoc)
	}
	for i := 0; i < maxDoc; i++ {
		if docIDs[i] != i {
			t.Errorf("doc[%d] = %d, want %d", i, docIDs[i], i)
		}
	}
}

// collectAllLeafCollector is a minimal LeafCollector that invokes a
// closure on each Collect call.
type collectAllLeafCollector struct {
	collectFn func(doc int)
}

func (c *collectAllLeafCollector) ScoreMode() ScoreMode             { return COMPLETE }
func (c *collectAllLeafCollector) SetScorer(Scorer) error          { return nil }
func (c *collectAllLeafCollector) Collect(doc int) error {
	c.collectFn(doc)
	return nil
}
