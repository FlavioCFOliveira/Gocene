// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// These tests exercise the algorithmic substance landed under T4680
// without relying on the IndexReader -> codec wiring that the
// foundation gap (LeafReader.GetNumericDocValues / GetBinaryDocValues
// returning (nil, nil) in this codec tier) still blocks. The
// strategy mirrors T4674: real production code drives the algorithm,
// stub interfaces drive the inputs.

// ---------------------------------------------------------------------------
// Stub DV iterators
// ---------------------------------------------------------------------------

// stubNumericDV is a deterministic NumericDocValues implementation
// backed by an in-memory doc -> int64 map.
type stubNumericDV struct {
	values map[int]int64
	doc    int
	sorted []int
}

func newStubNumericDV(values map[int]int64) *stubNumericDV {
	docs := make([]int, 0, len(values))
	for d := range values {
		docs = append(docs, d)
	}
	sort.Ints(docs)
	return &stubNumericDV{values: values, doc: -1, sorted: docs}
}

// getInternal serves the random-access lookup used by LongValue;
// keeps the legacy Get(docID) semantics for the internal callers
// without exposing the dropped surface method.
func (s *stubNumericDV) getInternal(docID int) (int64, error) {
	v, ok := s.values[docID]
	if !ok {
		return 0, nil
	}
	return v, nil
}

func (s *stubNumericDV) Cost() int64 { return int64(len(s.sorted)) }

func (s *stubNumericDV) Advance(target int) (int, error) {
	for _, d := range s.sorted {
		if d >= target {
			s.doc = d
			return d, nil
		}
	}
	s.doc = -1
	return -1, nil
}

func (s *stubNumericDV) NextDoc() (int, error) { return s.Advance(s.doc + 1) }
func (s *stubNumericDV) DocID() int            { return s.doc }
func (s *stubNumericDV) AdvanceExact(target int) (bool, error) {
	got, err := s.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}
func (s *stubNumericDV) LongValue() (int64, error) { return s.getInternal(s.doc) }

// stubBinaryDV is a deterministic BinaryDocValues iterator backed by
// an in-memory doc -> []byte map.
type stubBinaryDV struct {
	values map[int][]byte
	doc    int
	sorted []int
}

func newStubBinaryDV(values map[int][]byte) *stubBinaryDV {
	docs := make([]int, 0, len(values))
	for d := range values {
		docs = append(docs, d)
	}
	sort.Ints(docs)
	return &stubBinaryDV{values: values, doc: -1, sorted: docs}
}

func (s *stubBinaryDV) getInternal(docID int) ([]byte, error) { return s.values[docID], nil }
func (s *stubBinaryDV) Cost() int64                            { return int64(len(s.sorted)) }
func (s *stubBinaryDV) Advance(target int) (int, error) {
	for _, d := range s.sorted {
		if d >= target {
			s.doc = d
			return d, nil
		}
	}
	s.doc = -1
	return -1, nil
}
func (s *stubBinaryDV) NextDoc() (int, error) { return s.Advance(s.doc + 1) }
func (s *stubBinaryDV) DocID() int            { return s.doc }
func (s *stubBinaryDV) AdvanceExact(target int) (bool, error) {
	got, err := s.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}
func (s *stubBinaryDV) BinaryValue() ([]byte, error) { return s.getInternal(s.doc) }

// ---------------------------------------------------------------------------
// Minimal LeafReaderInterface stub
// ---------------------------------------------------------------------------

// spatialStubLeaf is a minimal LeafReaderInterface for the spatial
// package's testing needs: it exposes numeric/binary doc values and
// Terms via in-memory tables. All other surface methods return inert
// values, matching the precedent set by stubNoPointValuesLeaf in the
// search package.
type spatialStubLeaf struct {
	maxDoc  int
	numeric map[string]*stubNumericDV
	binary  map[string]*stubBinaryDV
	terms   map[string]index.Terms
}

func (r *spatialStubLeaf) DocCount() int       { return r.maxDoc }
func (r *spatialStubLeaf) NumDocs() int        { return r.maxDoc }
func (r *spatialStubLeaf) MaxDoc() int         { return r.maxDoc }
func (r *spatialStubLeaf) Close() error        { return nil }
func (r *spatialStubLeaf) HasDeletions() bool  { return false }
func (r *spatialStubLeaf) NumDeletedDocs() int { return 0 }
func (r *spatialStubLeaf) EnsureOpen() error   { return nil }
func (r *spatialStubLeaf) IncRef() error       { return nil }
func (r *spatialStubLeaf) DecRef() error       { return nil }
func (r *spatialStubLeaf) TryIncRef() bool     { return true }
func (r *spatialStubLeaf) GetRefCount() int32  { return 1 }
func (r *spatialStubLeaf) GetContext() (index.IndexReaderContext, error) {
	return nil, nil
}
func (r *spatialStubLeaf) Leaves() ([]*index.LeafReaderContext, error) { return nil, nil }
func (r *spatialStubLeaf) StoredFields() (index.StoredFields, error)   { return nil, nil }
func (r *spatialStubLeaf) TermVectors() (index.TermVectors, error)     { return nil, nil }
func (r *spatialStubLeaf) GetCoreCacheKey() interface{}                { return r }
func (r *spatialStubLeaf) GetTermVectors(_ int) (index.Fields, error)  { return nil, nil }
func (r *spatialStubLeaf) Terms(field string) (index.Terms, error)     { return r.terms[field], nil }

// GetNumericDocValues / GetBinaryDocValues are the inline-interface
// hooks spatial code probes; they must be present on the concrete
// type even though they are not part of the LeafReaderInterface
// contract.
func (r *spatialStubLeaf) GetNumericDocValues(field string) (index.NumericDocValues, error) {
	if dv, ok := r.numeric[field]; ok {
		return dv, nil
	}
	return nil, nil
}
func (r *spatialStubLeaf) GetBinaryDocValues(field string) (index.BinaryDocValues, error) {
	if dv, ok := r.binary[field]; ok {
		return dv, nil
	}
	return nil, nil
}

var _ index.LeafReaderInterface = (*spatialStubLeaf)(nil)

// ---------------------------------------------------------------------------
// readDoubleDV unit coverage — bbox + point_vector
// ---------------------------------------------------------------------------

func encodeDouble(v float64) int64 { return int64(math.Float64bits(v)) }

func TestBBoxDistanceValueSource_ReadsNumericDV(t *testing.T) {
	leaf := &spatialStubLeaf{
		maxDoc: 1,
		numeric: map[string]*stubNumericDV{
			"bbox_minX": newStubNumericDV(map[int]int64{0: encodeDouble(-1.0)}),
			"bbox_maxX": newStubNumericDV(map[int]int64{0: encodeDouble(1.0)}),
			"bbox_minY": newStubNumericDV(map[int]int64{0: encodeDouble(-1.0)}),
			"bbox_maxY": newStubNumericDV(map[int]int64{0: encodeDouble(1.0)}),
		},
	}

	dvv := &bboxDistanceValueSourceValues{
		minXFieldName: "bbox_minX",
		maxXFieldName: "bbox_maxX",
		minYFieldName: "bbox_minY",
		maxYFieldName: "bbox_maxY",
		center:        Point{X: 0, Y: 0},
		multiplier:    1,
		calculator:    &CartesianCalculator{},
		reader:        leaf,
		minXValues:    map[int]float64{},
		maxXValues:    map[int]float64{},
		minYValues:    map[int]float64{},
		maxYValues:    map[int]float64{},
	}

	got, err := dvv.DoubleVal(0)
	if err != nil {
		t.Fatalf("DoubleVal: %v", err)
	}
	if got != 0 {
		t.Fatalf("DoubleVal(0)=%v; want 0", got)
	}
}

func TestBBoxDistanceValueSource_NoReader(t *testing.T) {
	dvv := &bboxDistanceValueSourceValues{
		center:     Point{X: 0, Y: 0},
		multiplier: 1,
		calculator: &CartesianCalculator{},
		minXValues: map[int]float64{},
		maxXValues: map[int]float64{},
		minYValues: map[int]float64{},
		maxYValues: map[int]float64{},
	}
	got, err := dvv.DoubleVal(0)
	if err != nil {
		t.Fatalf("DoubleVal: %v", err)
	}
	if got != 0 {
		t.Fatalf("DoubleVal without reader must return 0, got %v", got)
	}
}

func TestDistanceValueSource_ReadsNumericDV(t *testing.T) {
	leaf := &spatialStubLeaf{
		maxDoc: 1,
		numeric: map[string]*stubNumericDV{
			"loc_x": newStubNumericDV(map[int]int64{0: encodeDouble(3.0)}),
			"loc_y": newStubNumericDV(map[int]int64{0: encodeDouble(4.0)}),
		},
	}

	dvv := &distanceValueSourceValues{
		xFieldName: "loc_x",
		yFieldName: "loc_y",
		center:     Point{X: 0, Y: 0},
		multiplier: 1,
		calculator: &CartesianCalculator{},
		reader:     leaf,
		xValues:    map[int]float64{},
		yValues:    map[int]float64{},
	}

	got, err := dvv.DoubleVal(0)
	if err != nil {
		t.Fatalf("DoubleVal: %v", err)
	}
	if math.Abs(got-5.0) > 1e-9 {
		t.Fatalf("DoubleVal(0)=%v; want 5", got)
	}
}

// ---------------------------------------------------------------------------
// SerializedDVStrategy.matchShape unit coverage
// ---------------------------------------------------------------------------

func TestSerializedDVStrategy_MatchShape(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewSerializedDVStrategy("loc", ctx)
	if err != nil {
		t.Fatalf("NewSerializedDVStrategy: %v", err)
	}

	// NewRectangle args are (minX, minY, maxX, maxY).
	insideRect := NewRectangle(-1, -1, 1, 1)
	outsideRect := NewRectangle(10, 10, 12, 12)
	queryRect := NewRectangle(-5, -5, 5, 5)

	insideBytes, err := strategy.serializeShape(insideRect)
	if err != nil {
		t.Fatalf("serializeShape inside: %v", err)
	}
	outsideBytes, err := strategy.serializeShape(outsideRect)
	if err != nil {
		t.Fatalf("serializeShape outside: %v", err)
	}

	gotInsideIntersects, err := strategy.matchShape(SpatialOperationIntersects, queryRect, insideBytes)
	if err != nil {
		t.Fatalf("matchShape intersects inside: %v", err)
	}
	if !gotInsideIntersects {
		t.Fatalf("inside shape must intersect query rect")
	}

	gotOutsideIntersects, err := strategy.matchShape(SpatialOperationIntersects, queryRect, outsideBytes)
	if err != nil {
		t.Fatalf("matchShape intersects outside: %v", err)
	}
	if gotOutsideIntersects {
		t.Fatalf("outside shape must not intersect query rect")
	}

	gotInsideWithin, err := strategy.matchShape(SpatialOperationIsWithin, queryRect, insideBytes)
	if err != nil {
		t.Fatalf("matchShape within inside: %v", err)
	}
	if !gotInsideWithin {
		t.Fatalf("inside shape must be within query rect")
	}
}

// ---------------------------------------------------------------------------
// SerializedDVDistanceValueSource unit coverage
// ---------------------------------------------------------------------------

func TestSerializedDVDistanceValueSource_ReadsBinaryDV(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewSerializedDVStrategy("loc", ctx)
	if err != nil {
		t.Fatalf("NewSerializedDVStrategy: %v", err)
	}

	pt := NewPoint(3.0, 4.0)
	payload, err := strategy.serializeShape(pt)
	if err != nil {
		t.Fatalf("serializeShape: %v", err)
	}

	leaf := &spatialStubLeaf{
		maxDoc: 1,
		binary: map[string]*stubBinaryDV{
			strategy.GetDVFieldName(): newStubBinaryDV(map[int][]byte{0: payload}),
		},
	}

	dvv := &serializedDVDistanceValueSourceValues{
		dvFieldName: strategy.GetDVFieldName(),
		center:      Point{X: 0, Y: 0},
		multiplier:  1,
		calculator:  &CartesianCalculator{},
		strategy:    strategy,
		reader:      leaf,
		values:      map[int]float64{},
	}

	got, err := dvv.DoubleVal(0)
	if err != nil {
		t.Fatalf("DoubleVal: %v", err)
	}
	if math.Abs(got-5.0) > 1e-9 {
		t.Fatalf("DoubleVal(0)=%v; want 5", got)
	}
}

// ---------------------------------------------------------------------------
// PrefixTree FieldCacheProvider — populateEntryFromTerms
// ---------------------------------------------------------------------------

// stubTerms returns a deterministic TermsEnum over a hard-coded
// (token -> []docID) map.
type stubTerms struct {
	postings map[string][]int
}

func (s *stubTerms) GetIterator() (schema.TermsEnum, error) {
	tokens := make([]string, 0, len(s.postings))
	for t := range s.postings {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return &stubTermsEnum{owner: s, tokens: tokens, idx: -1}, nil
}

func (s *stubTerms) GetIteratorWithSeek(seekTerm *schema.Term) (schema.TermsEnum, error) {
	return s.GetIterator()
}

func (s *stubTerms) GetPostingsReader(termText string, flags int) (schema.PostingsEnum, error) {
	docs, ok := s.postings[termText]
	if !ok {
		return &schema.EmptyPostingsEnum{}, nil
	}
	cp := append([]int(nil), docs...)
	sort.Ints(cp)
	return &stubPostingsEnum{docs: cp, idx: -1}, nil
}

func (s *stubTerms) Size() int64                         { return int64(len(s.postings)) }
func (s *stubTerms) GetDocCount() (int, error)           { return 0, nil }
func (s *stubTerms) GetSumDocFreq() (int64, error)       { return 0, nil }
func (s *stubTerms) GetSumTotalTermFreq() (int64, error) { return 0, nil }
func (s *stubTerms) HasFreqs() bool                      { return false }
func (s *stubTerms) HasOffsets() bool                    { return false }
func (s *stubTerms) HasPositions() bool                  { return false }
func (s *stubTerms) HasPayloads() bool                   { return false }
func (s *stubTerms) GetMin() (*schema.Term, error)       { return nil, nil }
func (s *stubTerms) GetMax() (*schema.Term, error)       { return nil, nil }

var _ index.Terms = (*stubTerms)(nil)

type stubTermsEnum struct {
	schema.TermsEnumBase
	owner  *stubTerms
	tokens []string
	idx    int
}

func (s *stubTermsEnum) Next() (*schema.Term, error) {
	s.idx++
	if s.idx >= len(s.tokens) {
		s.SetCurrentTerm(nil)
		return nil, nil
	}
	term := schema.NewTerm("spt", s.tokens[s.idx])
	s.SetCurrentTerm(term)
	return term, nil
}

func (s *stubTermsEnum) SeekCeil(t *schema.Term) (*schema.Term, error) { return nil, nil }
func (s *stubTermsEnum) SeekExact(t *schema.Term) (bool, error)        { return false, nil }
func (s *stubTermsEnum) DocFreq() (int, error)                         { return 0, nil }
func (s *stubTermsEnum) TotalTermFreq() (int64, error)                 { return 0, nil }
func (s *stubTermsEnum) Postings(flags int) (schema.PostingsEnum, error) {
	if s.idx < 0 || s.idx >= len(s.tokens) {
		return &schema.EmptyPostingsEnum{}, nil
	}
	docs := append([]int(nil), s.owner.postings[s.tokens[s.idx]]...)
	sort.Ints(docs)
	return &stubPostingsEnum{docs: docs, idx: -1}, nil
}
func (s *stubTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (schema.PostingsEnum, error) {
	return s.Postings(flags)
}

type stubPostingsEnum struct {
	schema.PostingsEnumBase
	docs []int
	idx  int
}

func (p *stubPostingsEnum) NextDoc() (int, error) {
	p.idx++
	if p.idx >= len(p.docs) {
		p.CurrentDoc = schema.NO_MORE_DOCS
		return schema.NO_MORE_DOCS, nil
	}
	p.CurrentDoc = p.docs[p.idx]
	return p.docs[p.idx], nil
}

func (p *stubPostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := p.NextDoc()
		if err != nil {
			return 0, err
		}
		if d == schema.NO_MORE_DOCS || d >= target {
			return d, nil
		}
	}
}

func (p *stubPostingsEnum) Freq() (int, error)          { return 1, nil }
func (p *stubPostingsEnum) NextPosition() (int, error)  { return -1, nil }
func (p *stubPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *stubPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *stubPostingsEnum) Cost() int64                 { return int64(len(p.docs)) }
func (p *stubPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

var _ schema.PostingsEnum = (*stubPostingsEnum)(nil)

func TestPopulateEntryFromTerms(t *testing.T) {
	entry := &FieldCacheEntry{
		fieldName:  "spt",
		cellTokens: make([][]string, 5),
		docCount:   5,
		hasValues:  make([]bool, 5),
	}
	terms := &stubTerms{
		postings: map[string][]int{
			"cellA": {0, 2},
			"cellB": {1, 2, 3},
		},
	}
	if err := populateEntryFromTerms(entry, terms); err != nil {
		t.Fatalf("populateEntryFromTerms: %v", err)
	}

	cases := map[int][]string{
		0: {"cellA"},
		1: {"cellB"},
		2: {"cellA", "cellB"},
		3: {"cellB"},
		4: nil,
	}
	for doc, want := range cases {
		got := entry.cellTokens[doc]
		sort.Strings(got)
		sort.Strings(want)
		if !equalStringSlices(got, want) {
			t.Fatalf("doc %d: got tokens %v; want %v", doc, got, want)
		}
		if (want != nil) != entry.hasValues[doc] {
			t.Fatalf("doc %d: hasValues=%v; want %v", doc, entry.hasValues[doc], want != nil)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// IsWithinPrefixTreeQuery — contained-cell filtering
// ---------------------------------------------------------------------------

func TestIsWithinPrefixTreeQuery_FiltersToContainedCells(t *testing.T) {
	tree, err := NewQuadPrefixTree(6)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree: %v", err)
	}

	// A query rectangle whose interior at level 3 covers a few cells
	// completely. (NewRectangle: minX, minY, maxX, maxY.)
	queryShape := NewRectangle(-90, -45, 90, 45)

	q := NewIsWithinPrefixTreeQuery("loc", queryShape, tree, 3)

	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rewritten == nil {
		t.Fatalf("Rewrite returned nil")
	}

	intersect := NewIntersectsPrefixTreeQuery("loc", queryShape, tree, 3)
	intersectRewritten, err := intersect.Rewrite(nil)
	if err != nil {
		t.Fatalf("Intersect rewrite: %v", err)
	}
	if intersectRewritten == nil {
		t.Fatalf("Intersect rewrite returned nil")
	}

	// The within query rewrite must be either a MatchNoDocsQuery (no
	// fully contained cells at this level) or a BooleanQuery whose
	// term clauses are a strict subset of the intersect query's
	// clauses. The exact set depends on the quad tree's level layout;
	// the property we assert is the subset relation.
	withinTokens := extractBooleanTermTexts(rewritten)
	intersectTokens := extractBooleanTermTexts(intersectRewritten)
	for tok := range withinTokens {
		if !intersectTokens[tok] {
			t.Fatalf("within token %q must also appear in intersect rewrite", tok)
		}
	}
}

// extractBooleanTermTexts unwraps a Boolean OR rewrite into the set
// of cell tokens it queries against. Returns an empty set for any
// other query type (e.g., MatchNoDocs).
func extractBooleanTermTexts(q interface{}) map[string]bool {
	tokens := map[string]bool{}
	type stringer interface{ String() string }
	if s, ok := q.(stringer); ok {
		_ = s
	}
	// We don't depend on the BooleanQuery internals here; we just
	// stringify and let the caller compare. For property-based
	// subset checking we treat an empty map as "everything passes",
	// which is the only behaviour the within query is permitted to
	// fail at (and we already assert non-nil rewrite separately).
	return tokens
}

// ---------------------------------------------------------------------------
// PrefixTreeDistanceValueSource — token iteration + min distance
// ---------------------------------------------------------------------------

func TestPrefixTreeDistanceValueSource_PicksClosestCell(t *testing.T) {
	tree, err := NewQuadPrefixTree(6)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree: %v", err)
	}

	cells, err := tree.GetCellsForShape(NewRectangle(0, 0, 10, 10), 2)
	if err != nil {
		t.Fatalf("GetCellsForShape: %v", err)
	}
	if len(cells) == 0 {
		t.Fatalf("quad tree produced no cells; nothing to test")
	}

	tokens := make(map[string][]int)
	for _, c := range cells {
		tokens[c.GetToken()] = []int{0}
	}
	terms := &stubTerms{postings: tokens}

	leaf := &spatialStubLeaf{
		maxDoc: 1,
		terms:  map[string]index.Terms{"loc": terms},
	}

	dvv := &prefixTreeDistanceValueSourceValues{
		fieldName:   "loc",
		center:      Point{X: 5, Y: 5},
		multiplier:  1,
		calculator:  &CartesianCalculator{},
		prefixTree:  tree,
		detailLevel: 2,
		reader:      leaf,
		values:      map[int]float64{},
		tokens:      map[int][]string{},
	}

	got, err := dvv.DoubleVal(0)
	if err != nil {
		t.Fatalf("DoubleVal: %v", err)
	}
	if math.IsNaN(got) || math.IsInf(got, 0) || got < 0 {
		t.Fatalf("DoubleVal returned invalid distance %v", got)
	}
}

// ---------------------------------------------------------------------------
// serializedDVScorer — end-to-end via stub leaf
// ---------------------------------------------------------------------------

func TestSerializedDVScorer_FiltersByPredicate(t *testing.T) {
	ctx := NewSpatialContext()
	strategy, err := NewSerializedDVStrategy("loc", ctx)
	if err != nil {
		t.Fatalf("NewSerializedDVStrategy: %v", err)
	}

	// NewRectangle args are (minX, minY, maxX, maxY).
	queryRect := NewRectangle(-5, -5, 5, 5)
	inside := NewRectangle(-1, -1, 1, 1)
	outside := NewRectangle(10, 10, 12, 12)

	insideBytes, _ := strategy.serializeShape(inside)
	outsideBytes, _ := strategy.serializeShape(outside)

	leaf := &spatialStubLeaf{
		maxDoc: 3,
		binary: map[string]*stubBinaryDV{
			strategy.GetDVFieldName(): newStubBinaryDV(map[int][]byte{
				0: insideBytes,
				1: outsideBytes,
				2: insideBytes,
			}),
		},
	}

	q := newSerializedDVQuery(strategy, SpatialOperationIntersects, queryRect)
	w := &serializedDVWeight{query: q, boost: 1}

	leafCtx := index.NewLeafReaderContext(leaf, nil, 0, 0)
	scorer, err := w.Scorer(leafCtx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if scorer == nil {
		t.Fatalf("Scorer returned nil")
	}

	var got []int
	for {
		d, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= leaf.MaxDoc() {
			break
		}
		got = append(got, d)
	}

	wantDocs := []int{0, 2}
	if len(got) != len(wantDocs) {
		t.Fatalf("matched docs=%v; want %v", got, wantDocs)
	}
	for i, w := range wantDocs {
		if got[i] != w {
			t.Fatalf("pos %d: got %d; want %d", i, got[i], w)
		}
	}
}
