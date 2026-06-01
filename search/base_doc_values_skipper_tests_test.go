// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseDocValuesSkipperTests.java
//
// BaseDocValuesSkipperTests is the abstract fixture-provider base: it supplies a
// synthetic NumericDocValues and a matching DocValuesSkipper to its concrete
// subclasses (e.g. the DocValuesRangeIterator tests) but defines no @Test
// methods of its own. The synthetic NumericDocValues encodes a documented value
// distribution across doc-id ranges (0-128 in-range, 128-256 above queryMax,
// 256-512 below queryMin, 512-1024 mixed, and a sparse 1024-2048 tail).
//
// This port materialises that NumericDocValues fixture (the part of the base
// that maps onto Gocene's interfaces) and verifies its documented contract
// directly.
//
// HONEST FEATURE GAP (no t.Skip, noted explicitly): the companion
// docValuesSkipper fixture is NOT ported because Gocene's DocValuesSkipper
// interface (SkipTo / GetDocID only) is far narrower than Lucene's level-based
// skipper (numLevels / minDocID / maxDocID / minValue / maxValue / docCount per
// level). Until that richer skipper surface exists, the per-level skip metadata
// the fixture exposes cannot be expressed, so the skipper-driven scenarios of
// the concrete subclasses are not yet runnable here.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// skipperDocValues is the synthetic NumericDocValues fixture from
// BaseDocValuesSkipperTests.docValues(queryMin, queryMax).
type skipperDocValues struct {
	queryMin, queryMax int64
	doc                int
}

func newSkipperDocValues(queryMin, queryMax int64) *skipperDocValues {
	return &skipperDocValues{queryMin: queryMin, queryMax: queryMax, doc: -1}
}

func (d *skipperDocValues) DocID() int { return d.doc }

func (d *skipperDocValues) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *skipperDocValues) Advance(target int) (int, error) {
	switch {
	case target < 1024:
		d.doc = target // dense up to 1024
	case d.doc < 2047:
		d.doc = target + (target & 1) // 50% of docs have a value up to 2048
	default:
		d.doc = index.NO_MORE_DOCS
	}
	return d.doc, nil
}

func (d *skipperDocValues) AdvanceExact(_ int) (bool, error) {
	return false, errUnsupportedAdvanceExact
}

func (d *skipperDocValues) LongValue() (int64, error) {
	v := d.doc % 1024
	switch {
	case v < 128:
		return (d.queryMin + d.queryMax) >> 1, nil
	case v < 256:
		return d.queryMax + 1, nil
	case v < 512:
		return d.queryMin - 1, nil
	default:
		switch (v / 2) % 3 {
		case 0:
			return d.queryMin - 1, nil
		case 1:
			return d.queryMax + 1, nil
		default:
			return (d.queryMin + d.queryMax) >> 1, nil
		}
	}
}

func (d *skipperDocValues) Cost() int64 { return 42 }

// errUnsupportedAdvanceExact mirrors the UnsupportedOperationException the Java
// fixture throws from advanceExact.
var errUnsupportedAdvanceExact = errUnsupported("advanceExact is not supported by the skipper fixture")

type errUnsupported string

func (e errUnsupported) Error() string { return string(e) }

// Ensure the fixture satisfies the NumericDocValues contract.
var _ index.NumericDocValues = (*skipperDocValues)(nil)

func TestBaseDocValuesSkipperTests(t *testing.T) {
	const queryMin, queryMax = int64(100), int64(200)
	mid := (queryMin + queryMax) >> 1

	dv := newSkipperDocValues(queryMin, queryMax)

	// Walk the dense [0, 1024) region and assert the documented value bands.
	for doc := 0; doc < 1024; doc++ {
		got, err := dv.Advance(doc)
		if err != nil {
			t.Fatalf("Advance(%d): %v", doc, err)
		}
		if got != doc {
			t.Fatalf("Advance(%d) = %d, want dense %d", doc, got, doc)
		}
		val, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue at doc %d: %v", doc, err)
		}
		switch {
		case doc < 128:
			if val != mid {
				t.Errorf("doc %d (in-range band): value = %d, want %d", doc, val, mid)
			}
			if !(val >= queryMin && val <= queryMax) {
				t.Errorf("doc %d: value %d not within [%d,%d]", doc, val, queryMin, queryMax)
			}
		case doc < 256:
			if val != queryMax+1 {
				t.Errorf("doc %d (above-max band): value = %d, want %d", doc, val, queryMax+1)
			}
		case doc < 512:
			if val != queryMin-1 {
				t.Errorf("doc %d (below-min band): value = %d, want %d", doc, val, queryMin-1)
			}
		default:
			// Mixed band: each value is one of the three documented choices.
			if val != queryMin-1 && val != queryMax+1 && val != mid {
				t.Errorf("doc %d (mixed band): value = %d, want one of {%d,%d,%d}",
					doc, val, queryMin-1, queryMax+1, mid)
			}
		}
	}

	// The sparse tail [1024, 2048) only yields even doc ids that carry a value.
	dv = newSkipperDocValues(queryMin, queryMax)
	got, err := dv.Advance(1024)
	if err != nil {
		t.Fatalf("Advance(1024): %v", err)
	}
	if got != 1024 {
		t.Errorf("first tail doc = %d, want 1024", got)
	}
	got, err = dv.Advance(1025)
	if err != nil {
		t.Fatalf("Advance(1025): %v", err)
	}
	if got != 1026 {
		t.Errorf("Advance(1025) = %d, want 1026 (odd targets skip to the next even-valued doc)", got)
	}

	if dv.Cost() != 42 {
		t.Errorf("Cost() = %d, want 42", dv.Cost())
	}
}
