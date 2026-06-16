// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// ─── rfqCompare unit tests ────────────────────────────────────────────────────

// TestRfqCompare_INTERSECTS verifies the INTERSECTS compare predicate against
// the three expected relations.  Values are 1-dimensional 4-byte ints.
//
// For range fields, each "point" is a [min, max] pair and the BKD cell
// minPackedValue / maxPackedValue are 2*numDims*bytesPerDim = 8 bytes long,
// laid out as [dimMin (4 bytes) | dimMax (4 bytes)].
//
// A cell is CELL_INSIDE_QUERY when the query is guaranteed to intersect every
// doc in it — i.e., qMax >= maxDocMin AND qMin <= minDocMax.  This is a
// pruning signal, not a per-doc judgement.
//
// Query range: [10, 20]  → ranges = encode(10) ++ encode(20)
// BKD cells (single-doc leaves, so cellMin == cellMax == packRange(lo,hi)):
//
//	outside_low  : cell [1,5]   → CELL_OUTSIDE_QUERY (qMin(10)>cellMax(5))
//	outside_high : cell [25,30] → CELL_OUTSIDE_QUERY (qMax(20)<cellMin(25))
//	inside       : cell [12,18] → CELL_INSIDE_QUERY  (all docs match)
//	crosses_low  : cell [5,15]  → CELL_INSIDE_QUERY  (qMax(20)>=docMin(5), qMin(10)<=docMax(15))
//	crosses_high : cell [15,25] → CELL_INSIDE_QUERY  (qMax(20)>=docMin(15), qMin(10)<=docMax(25))
//
// Notes:
//   - "crosses_low/high" are named for the *per-document* relation; at the BKD
//     pruning level, a single-doc leaf where the doc intersects is CELL_INSIDE.
//   - CELL_CROSSES_QUERY only arises when a multi-doc cell contains both
//     matching and non-matching docs, requiring per-doc evaluation.
func TestRfqCompare_INTERSECTS(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	// packRange builds the 8-byte packed cell value [min4 | max4].
	packRange := func(lo, hi int) []byte {
		return append(encode(lo), encode(hi)...)
	}
	cmp := bkd.GetUnsignedComparator(4)

	// ranges = [qMin(4 bytes), qMax(4 bytes)] for the query
	ranges := append(encode(10), encode(20)...)

	cases := []struct {
		name    string
		cellLo  int
		cellHi  int
		wantRel int
	}{
		{"outside_low", 1, 5, rfqRelCellOutside},
		{"outside_high", 25, 30, rfqRelCellOutside},
		{"inside", 12, 18, rfqRelCellInside},
		// Single-doc cells that intersect the query: BKD prunes them as CELL_INSIDE.
		{"intersects_low", 5, 15, rfqRelCellInside},
		{"intersects_high", 15, 25, rfqRelCellInside},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cell := packRange(tc.cellLo, tc.cellHi)
			got := rfqCompare(RangeFieldQueryTypeIntersects, ranges,
				cell, cell, // BKD cell min/max same for a leaf cell
				1, 4, cmp)
			if got != tc.wantRel {
				t.Fatalf("got=%d want=%d", got, tc.wantRel)
			}
		})
	}
}

// TestRfqCompare_INTERSECTS_MultiDoc verifies that a multi-doc BKD cell
// produces CELL_CROSSES_QUERY when only some docs in the cell would match.
//
// Query range: [10, 20]
// A cell spanning docs [1,5] to [25,30]: some docs match (e.g. [12,18]) and
// some do not (e.g. [1,5]).  The BKD pruner cannot confirm all docs match, so
// it returns CELL_CROSSES_QUERY.
func TestRfqCompare_INTERSECTS_MultiDoc(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	packRange := func(lo, hi int) []byte {
		return append(encode(lo), encode(hi)...)
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// Cell min: packRange(1, 5) — smallest doc-min(1) and smallest doc-max(5)
	// Cell max: packRange(25, 30) — largest doc-min(25) and largest doc-max(30)
	cellMin := packRange(1, 5)
	cellMax := packRange(25, 30)
	got := rfqCompare(RangeFieldQueryTypeIntersects, ranges, cellMin, cellMax, 1, 4, cmp)
	if got != rfqRelCellCrosses {
		t.Fatalf("multi-doc mixed cell: got=%d want=%d (CELL_CROSSES)", got, rfqRelCellCrosses)
	}
}

// TestRfqCompare_WITHIN verifies the WITHIN (doc-inside-query) predicate.
//
// Query range: [10, 20]
// For a RangeField, each indexed value is a packed [min,max] pair.  The BKD
// cell minPackedValue / maxPackedValue are therefore also 2*numDims*bytesPerDim
// = 8 bytes: [dimMin(4)|dimMax(4)].
//
//   - Cell covering only doc [12,18]: cellMin = cellMax = packRange(12,18).
//     All doc ranges are within [10,20] → CELL_INSIDE.
//   - Cell covering docs with ranges up to [5,25]: cell bounds include values
//     outside [10,20] → not CELL_INSIDE.
func TestRfqCompare_WITHIN(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	packRange := func(lo, hi int) []byte {
		return append(encode(lo), encode(hi)...)
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// BKD cell containing a single doc [12,18]; cell min/max are both packRange(12,18).
	got := rfqCompare(RangeFieldQueryTypeWithin, ranges,
		packRange(12, 18), packRange(12, 18), 1, 4, cmp)
	if got != rfqRelCellInside {
		t.Fatalf("within inside: got %d want %d", got, rfqRelCellInside)
	}

	// BKD cell spanning docs from [5,8] to [25,30]; clearly not all within [10,20].
	got = rfqCompare(RangeFieldQueryTypeWithin, ranges,
		packRange(5, 8), packRange(25, 30), 1, 4, cmp)
	if got == rfqRelCellInside {
		t.Fatalf("within outside should not be CELL_INSIDE; got %d", got)
	}
}

// TestRfqCompare_CONTAINS verifies the CONTAINS predicate.
//
// Query range: [10, 20]
// BKD cell packed values are 2*numDims*bytesPerDim = 8 bytes.
//
//   - Cell covering only doc [5,25]: packRange(5,25).  The doc range contains
//     [10,20] → CELL_INSIDE.
//   - Cell covering only doc [12,18]: packRange(12,18).  The doc range is
//     contained by [10,20], not the other way round → not CELL_INSIDE.
func TestRfqCompare_CONTAINS(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	packRange := func(lo, hi int) []byte {
		return append(encode(lo), encode(hi)...)
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// Cell with only doc [5,25]; qMin(10)>=docMin(5) and qMax(20)<=docMax(25).
	got := rfqCompare(RangeFieldQueryTypeContains, ranges,
		packRange(5, 25), packRange(5, 25), 1, 4, cmp)
	if got != rfqRelCellInside {
		t.Fatalf("contains inside: got %d want %d", got, rfqRelCellInside)
	}

	// Cell with only doc [12,18]: doc range does not contain query [10,20].
	got = rfqCompare(RangeFieldQueryTypeContains, ranges,
		packRange(12, 18), packRange(12, 18), 1, 4, cmp)
	if got == rfqRelCellInside {
		t.Fatalf("contains non-match should not be CELL_INSIDE; got %d", got)
	}
}

// TestRfqCompare_CROSSES verifies the CROSSES predicate.
//
// BKD cell packed values are 2*numDims*bytesPerDim = 8 bytes.
// CROSSES = INTERSECTS AND NOT WITHIN (doc range partially overlaps query).
func TestRfqCompare_CROSSES(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	packRange := func(lo, hi int) []byte {
		return append(encode(lo), encode(hi)...)
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// Cell with doc [5,15]: intersects [10,20] but is not within it → not CELL_OUTSIDE.
	got := rfqCompare(RangeFieldQueryTypeCrosses, ranges,
		packRange(5, 15), packRange(5, 15), 1, 4, cmp)
	if got == rfqRelCellOutside {
		t.Fatalf("crosses should not be CELL_OUTSIDE; got %d", got)
	}

	// Cell with doc [1,5]: completely outside [10,20] → CELL_OUTSIDE_QUERY.
	got = rfqCompare(RangeFieldQueryTypeCrosses, ranges,
		packRange(1, 5), packRange(1, 5), 1, 4, cmp)
	if got != rfqRelCellOutside {
		t.Fatalf("crosses disjoint: got %d want %d", got, rfqRelCellOutside)
	}
}

// TestRfqMatches_INTERSECTS verifies the per-document match predicate.
func TestRfqMatches_INTERSECTS(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// doc range [12, 18] intersects [10, 20]
	if !rfqMatches(RangeFieldQueryTypeIntersects, ranges, append(encode(12), encode(18)...), 1, 4, cmp) {
		t.Fatal("expected match for [12,18] INTERSECTS [10,20]")
	}
	// doc range [25, 30] does not intersect [10, 20]
	if rfqMatches(RangeFieldQueryTypeIntersects, ranges, append(encode(25), encode(30)...), 1, 4, cmp) {
		t.Fatal("expected no match for [25,30] INTERSECTS [10,20]")
	}
}

// TestRfqMatches_WITHIN verifies the per-document WITHIN match.
func TestRfqMatches_WITHIN(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// [12, 18] is within [10, 20]
	if !rfqMatches(RangeFieldQueryTypeWithin, ranges, append(encode(12), encode(18)...), 1, 4, cmp) {
		t.Fatal("expected match for [12,18] WITHIN [10,20]")
	}
	// [5, 25] is not within [10, 20]
	if rfqMatches(RangeFieldQueryTypeWithin, ranges, append(encode(5), encode(25)...), 1, 4, cmp) {
		t.Fatal("expected no match for [5,25] WITHIN [10,20]")
	}
}

// TestRfqMatches_CONTAINS verifies the per-document CONTAINS match.
func TestRfqMatches_CONTAINS(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	cmp := bkd.GetUnsignedComparator(4)
	ranges := append(encode(10), encode(20)...)

	// [5, 25] contains [10, 20]
	if !rfqMatches(RangeFieldQueryTypeContains, ranges, append(encode(5), encode(25)...), 1, 4, cmp) {
		t.Fatal("expected match for [5,25] CONTAINS [10,20]")
	}
	// [12, 18] does not contain [10, 20]
	if rfqMatches(RangeFieldQueryTypeContains, ranges, append(encode(12), encode(18)...), 1, 4, cmp) {
		t.Fatal("expected no match for [12,18] CONTAINS [10,20]")
	}
}

// ─── CreateWeight ─────────────────────────────────────────────────────────────

// TestRangeFieldQuery_CreateWeight_LegacyNoError verifies that the legacy
// constructor (no numDims/bytesPerDim) produces a Weight without error.
func TestRangeFieldQuery_CreateWeight_LegacyNoError(t *testing.T) {
	q := NewRangeFieldQuery("f", []byte{0, 0, 0, 10}, []byte{0, 0, 0, 20}, RangeFieldQueryTypeIntersects)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		t.Fatal("CreateWeight returned nil weight")
	}
}

// TestRangeFieldQuery_CreateWeight_FullNoError verifies that the full
// constructor produces a Weight without error.
func TestRangeFieldQuery_CreateWeight_FullNoError(t *testing.T) {
	q, err := NewRangeFieldQueryFull("f",
		[]byte{0, 0, 0, 10}, []byte{0, 0, 0, 20},
		1, 4,
		RangeFieldQueryTypeIntersects,
	)
	if err != nil {
		t.Fatalf("NewRangeFieldQueryFull: %v", err)
	}
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		t.Fatal("CreateWeight returned nil weight")
	}
}

// TestRangeFieldQuery_CreateWeight_NoPointValues verifies that when the leaf
// reader does not expose point values, the weight returns nil scorer (no matches).
func TestRangeFieldQuery_CreateWeight_NoPointValues(t *testing.T) {
	q, _ := NewRangeFieldQueryFull("f",
		[]byte{0, 0, 0, 10}, []byte{0, 0, 0, 20},
		1, 4,
		RangeFieldQueryTypeIntersects,
	)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	// A nil-returning leaf reader (no GetPointValues capability).
	ctx := index.NewLeafReaderContext(&stubNoPointValuesLeaf{maxDoc: 5}, nil, 0, 0)
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier != nil {
		t.Fatal("expected nil ScorerSupplier when no point values")
	}
}

// TestRangeFieldQuery_CreateWeight_WithPointValues verifies that when the leaf
// reader exposes a mock PointValues, the weight performs BKD intersection and
// returns the correct matching documents.
//
// Setup: 5 documents with 1D int ranges encoded as 4-byte big-endian:
//
//	doc 0: [5, 8]   → outside query [10,20]
//	doc 1: [12, 18] → inside query  [10,20] → MATCH
//	doc 2: [25, 30] → outside query [10,20]
//	doc 3: [10, 20] → exactly query → MATCH
//	doc 4: [15, 22] → overlaps     → MATCH (intersects)
func TestRangeFieldQuery_CreateWeight_WithPointValues(t *testing.T) {
	encode := func(v int) []byte {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	type docRange struct {
		docID int
		min   int
		max   int
	}
	docs := []docRange{
		{0, 5, 8},
		{1, 12, 18},
		{2, 25, 30},
		{3, 10, 20},
		{4, 15, 22},
	}
	packed := make([][]byte, len(docs))
	for i, d := range docs {
		packed[i] = append(encode(d.min), encode(d.max)...)
	}

	// For a RangeField BKD tree, the field-level minPackedValue is the
	// component-wise minimum across all indexed [min,max] packed values.
	// Docs: [5,8],[12,18],[25,30],[10,20],[15,22]
	//   field min = packRange( min(5,12,25,10,15), min(8,18,30,20,22) ) = packRange(5,8)
	//   field max = packRange( max(5,12,25,10,15), max(8,18,30,20,22) ) = packRange(25,30)
	mock := &stubRangeFieldPointValues{
		minPV:  append(encode(5), encode(8)...),   // packRange(5,8)
		maxPV:  append(encode(25), encode(30)...), // packRange(25,30)
		docCnt: 5,
		packed: packed,
		docIDs: []int{0, 1, 2, 3, 4},
	}

	q, _ := NewRangeFieldQueryFull("f",
		encode(10), encode(20),
		1, 4,
		RangeFieldQueryTypeIntersects,
	)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	leaf := &stubPointValuesLeaf{
		stubNoPointValuesLeaf: stubNoPointValuesLeaf{maxDoc: 5},
		pv:                    mock,
		field:                 "f",
	}
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatal("expected non-nil ScorerSupplier")
	}

	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if scorer == nil {
		t.Fatal("expected non-nil Scorer")
	}

	// Collect matching doc IDs.
	var got []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		got = append(got, doc)
	}

	// docs 1, 3, 4 match INTERSECTS [10,20].
	want := []int{1, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("got docs %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d: got doc %d, want %d", i, got[i], want[i])
		}
	}
}

// ─── stubs ────────────────────────────────────────────────────────────────────

// stubNoPointValuesLeaf is a minimal LeafReaderInterface that does not expose
// GetPointValues.
type stubNoPointValuesLeaf struct {
	maxDoc int
}

func (r *stubNoPointValuesLeaf) DocCount() int       { return r.maxDoc }
func (r *stubNoPointValuesLeaf) NumDocs() int        { return r.maxDoc }
func (r *stubNoPointValuesLeaf) MaxDoc() int         { return r.maxDoc }
func (r *stubNoPointValuesLeaf) Close() error        { return nil }
func (r *stubNoPointValuesLeaf) HasDeletions() bool  { return false }
func (r *stubNoPointValuesLeaf) NumDeletedDocs() int { return 0 }
func (r *stubNoPointValuesLeaf) EnsureOpen() error   { return nil }
func (r *stubNoPointValuesLeaf) IncRef() error       { return nil }
func (r *stubNoPointValuesLeaf) DecRef() error       { return nil }
func (r *stubNoPointValuesLeaf) TryIncRef() bool     { return true }
func (r *stubNoPointValuesLeaf) GetRefCount() int32  { return 1 }
func (r *stubNoPointValuesLeaf) GetContext() (index.IndexReaderContext, error) {
	return nil, nil
}
func (r *stubNoPointValuesLeaf) Leaves() ([]*index.LeafReaderContext, error) { return nil, nil }
func (r *stubNoPointValuesLeaf) StoredFields() (index.StoredFields, error)   { return nil, nil }
func (r *stubNoPointValuesLeaf) TermVectors() (index.TermVectors, error)     { return nil, nil }
func (r *stubNoPointValuesLeaf) GetCoreCacheKey() interface{}                { return r }
func (r *stubNoPointValuesLeaf) GetTermVectors(_ int) (index.Fields, error)  { return nil, nil }
func (r *stubNoPointValuesLeaf) Terms(_ string) (index.Terms, error)         { return nil, nil }

var _ index.LeafReaderInterface = (*stubNoPointValuesLeaf)(nil)

// stubPointValuesLeaf also implements GetPointValues for the given field.
type stubPointValuesLeaf struct {
	stubNoPointValuesLeaf
	pv    *stubRangeFieldPointValues
	field string
}

// GetPointValues returns the stub point values wrapped in an indexPointValuesShim
// so it satisfies index.PointValues and can also be cast to rangeFieldPointValues.
func (r *stubPointValuesLeaf) GetPointValues(field string) (index.PointValues, error) {
	if field == r.field {
		return &indexPointValuesShim{inner: r.pv}, nil
	}
	return nil, nil
}

// stubRangeFieldPointValues is an in-memory implementation that satisfies the
// rangeFieldPointValues interface.  It is NOT an index.PointValues (they have
// conflicting method signatures); the test leaf adapter wraps it behind a thin
// index.PointValues shim so GetPointValues can return it.
type stubRangeFieldPointValues struct {
	minPV  []byte
	maxPV  []byte
	docCnt int
	packed [][]byte // one entry per document; matches docIDs by index
	docIDs []int
}

func (p *stubRangeFieldPointValues) GetDocCount() int      { return p.docCnt }
func (p *stubRangeFieldPointValues) GetNumDimensions() int { return 1 }
func (p *stubRangeFieldPointValues) GetBytesPerDimension() int {
	if len(p.packed) > 0 {
		return len(p.packed[0]) / 2
	}
	return 4
}

// GetMinPackedValue satisfies rangeFieldPointValues.
func (p *stubRangeFieldPointValues) GetMinPackedValue() ([]byte, error) { return p.minPV, nil }

// GetMaxPackedValue satisfies rangeFieldPointValues.
func (p *stubRangeFieldPointValues) GetMaxPackedValue() ([]byte, error) { return p.maxPV, nil }

// Intersect walks all documents and calls the visitor for each.
func (p *stubRangeFieldPointValues) Intersect(visitor intersectVisitorRFQ) error {
	rel := visitor.Compare(p.minPV, p.maxPV)
	switch rel {
	case rfqRelCellOutside:
		return nil
	case rfqRelCellInside:
		visitor.Grow(len(p.docIDs))
		for _, docID := range p.docIDs {
			if err := visitor.Visit(docID); err != nil {
				return err
			}
		}
		return nil
	default: // crosses
		visitor.Grow(len(p.docIDs))
		for i, docID := range p.docIDs {
			if err := visitor.VisitByPackedValue(docID, p.packed[i]); err != nil {
				return err
			}
		}
		return nil
	}
}

// EstimatePointCount returns a rough estimate.
func (p *stubRangeFieldPointValues) EstimatePointCount(_ intersectVisitorRFQ) int64 {
	return int64(len(p.docIDs))
}

var _ rangeFieldPointValues = (*stubRangeFieldPointValues)(nil)

// indexPointValuesShim wraps stubRangeFieldPointValues and satisfies both
// index.PointValues (which adds GetDocCountWithValue / GetValueCount) and
// rangeFieldPointValues (which adds Intersect / EstimatePointCount).
// Since stubRangeFieldPointValues now has error-returning GetMin/GetMax, the
// shim only needs to add the two extra index.PointValues methods.
type indexPointValuesShim struct {
	inner *stubRangeFieldPointValues
}

func (s *indexPointValuesShim) GetDocCount() int { return s.inner.GetDocCount() }
func (s *indexPointValuesShim) GetDocCountWithValue() int64 {
	return int64(s.inner.GetDocCount())
}
func (s *indexPointValuesShim) GetValueCount() int64 { return int64(len(s.inner.docIDs)) }
func (s *indexPointValuesShim) GetMinPackedValue() ([]byte, error) {
	return s.inner.GetMinPackedValue()
}
func (s *indexPointValuesShim) GetMaxPackedValue() ([]byte, error) {
	return s.inner.GetMaxPackedValue()
}
func (s *indexPointValuesShim) GetNumDimensions() int     { return s.inner.GetNumDimensions() }
func (s *indexPointValuesShim) GetBytesPerDimension() int { return s.inner.GetBytesPerDimension() }
func (s *indexPointValuesShim) Intersect(visitor intersectVisitorRFQ) error {
	return s.inner.Intersect(visitor)
}
func (s *indexPointValuesShim) EstimatePointCount(visitor intersectVisitorRFQ) int64 {
	return s.inner.EstimatePointCount(visitor)
}

var _ index.PointValues = (*indexPointValuesShim)(nil)
var _ rangeFieldPointValues = (*indexPointValuesShim)(nil)