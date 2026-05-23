// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LeafReaderDocValuesFunc is a function that retrieves a doc-values iterator
// from a *index.LeafReader. It mirrors the @FunctionalInterface
// org.apache.lucene.search.join.DocValuesTermsCollector.Function.
type LeafReaderDocValuesFunc[DV any] func(reader *index.LeafReader) (DV, error)

// DocValuesTermsCollector is an abstract base collector that refreshes a
// doc-values cursor at the start of each segment.
//
// Mirrors org.apache.lucene.search.join.DocValuesTermsCollector.
//
// Gocene deviation: Java uses a SimpleCollector with doSetNextReader; here the
// segment switch is handled by GetLeafCollector, which builds a new leaf
// collector wrapping a fresh DV cursor.
type DocValuesTermsCollector[DV any] struct {
	dvFunc     LeafReaderDocValuesFunc[DV]
	// collectFn is the per-document collect function injected by the concrete subtype.
	collectFn  func(dv DV, doc int) error
	scoreMode  search.ScoreMode
}

// newDocValuesTermsCollector builds the base.
func newDocValuesTermsCollector[DV any](
	dvFunc LeafReaderDocValuesFunc[DV],
	collectFn func(dv DV, doc int) error,
	mode search.ScoreMode,
) *DocValuesTermsCollector[DV] {
	return &DocValuesTermsCollector[DV]{
		dvFunc:    dvFunc,
		collectFn: collectFn,
		scoreMode: mode,
	}
}

// ScoreMode implements search.Collector.
func (c *DocValuesTermsCollector[DV]) ScoreMode() search.ScoreMode { return c.scoreMode }

// GetLeafCollector implements search.Collector.
func (c *DocValuesTermsCollector[DV]) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	var dv DV
	if lr, ok := reader.(*index.LeafReader); ok && c.dvFunc != nil {
		var err error
		dv, err = c.dvFunc(lr)
		if err != nil {
			return nil, err
		}
	}
	fn := c.collectFn
	return &dvTermsLeafCollector[DV]{dv: dv, collectFn: fn}, nil
}

// dvTermsLeafCollector is a leaf collector that delegates to a per-doc function.
type dvTermsLeafCollector[DV any] struct {
	dv        DV
	collectFn func(dv DV, doc int) error
}

func (lc *dvTermsLeafCollector[DV]) SetScorer(_ search.Scorer) error { return nil }

func (lc *dvTermsLeafCollector[DV]) Collect(doc int) error {
	if lc.collectFn == nil {
		return nil
	}
	return lc.collectFn(lc.dv, doc)
}
