// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ------------------------------------------------------------------
// Test helpers: minimal encoder + ShapeDocValues factory.
// ------------------------------------------------------------------

// identityShapeEncoder is the search-package counterpart of the
// identityEncoder used in document/shape_doc_values_test.go. It maps
// encoded int32 to float64 with no transformation, which is exactly
// what the doc-values relate path needs when the test query
// Component2D operates in encoded space.
type identityShapeEncoder struct{}

func (identityShapeEncoder) EncodeX(x float64) int32 { return int32(x) }
func (identityShapeEncoder) EncodeY(y float64) int32 { return int32(y) }
func (identityShapeEncoder) DecodeX(x int32) float64 { return float64(x) }
func (identityShapeEncoder) DecodeY(y int32) float64 { return float64(y) }

// nilGeometryFn is the trivial computeCentroid / computeBoundingBox
// hook used by tests that don't care about world-space projection.
func nilGeometryFn(_ *document.ShapeDocValues) geo.Geometry { return nil }

// triangleAt builds an axis-aligned right triangle in encoded space.
// Mirrors the document/shape_doc_values_test.go helper but lives in
// the search package so the doc-values tests stay self-contained.
func triangleAt(ax, ay, bx, by, cx, cy int32) document.DecodedTriangle {
	return document.DecodedTriangle{
		Kind: document.DecodedTriangleTypeTriangle,
		AX:   ax, AY: ay,
		BX: bx, BY: by,
		CX: cx, CY: cy,
		AB: true, BC: true, CA: true,
	}
}

// newTestShapeDocValues constructs a single-triangle ShapeDocValues
// whose bounding box is [(0,0), (10,10)], suitable for relate() tests
// against a Component2D query.
func newTestShapeDocValues(t *testing.T) *document.ShapeDocValues {
	t.Helper()
	sdv, err := document.NewShapeDocValuesFromTessellation(
		identityShapeEncoder{},
		[]document.DecodedTriangle{triangleAt(0, 0, 10, 0, 5, 10)},
		nilGeometryFn,
		nilGeometryFn,
	)
	if err != nil {
		t.Fatalf("newTestShapeDocValues: %v", err)
	}
	return sdv
}

// ------------------------------------------------------------------
// Constructor coverage.
// ------------------------------------------------------------------

// TestNewBaseShapeDocValuesQuery_RejectsContains asserts that the
// constructor returns the canonical sentinel when the relation is
// CONTAINS. Mirrors the Java IllegalArgumentException check.
func TestNewBaseShapeDocValuesQuery_RejectsContains(t *testing.T) {
	t.Parallel()
	_, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationContains,
		fakeComponent2D{},
		func(_ *util.BytesRef) (*document.ShapeDocValues, error) { return nil, nil },
		nil,
	)
	if !errors.Is(err, ErrBaseShapeDocValuesQueryContainsNotSupported) {
		t.Fatalf("err = %v; want ErrBaseShapeDocValuesQueryContainsNotSupported", err)
	}
}

// TestNewBaseShapeDocValuesQuery_RejectsNilDecoder confirms the
// constructor refuses to build a query without a decoder closure.
func TestNewBaseShapeDocValuesQuery_RejectsNilDecoder(t *testing.T) {
	t.Parallel()
	_, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("err = nil; want non-nil (decoder must not be nil)")
	}
}

// TestNewBaseShapeDocValuesQuery_DefaultMatchCost verifies the
// matchCost hook defaults to the Java reference's 60*100 constant.
func TestNewBaseShapeDocValuesQuery_DefaultMatchCost(t *testing.T) {
	t.Parallel()
	q, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func(_ *util.BytesRef) (*document.ShapeDocValues, error) { return nil, nil },
		nil,
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got, want := q.MatchCost(), float32(60*100); got != want {
		t.Errorf("MatchCost = %v; want %v", got, want)
	}
}

// TestNewBaseShapeDocValuesQuery_MatchCostOverride checks the
// WithBaseShapeDocValuesMatchCost option installs the override.
func TestNewBaseShapeDocValuesQuery_MatchCostOverride(t *testing.T) {
	t.Parallel()
	q, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func(_ *util.BytesRef) (*document.ShapeDocValues, error) { return nil, nil },
		nil,
		WithBaseShapeDocValuesMatchCost(func() float32 { return 7 }),
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got, want := q.MatchCost(), float32(7); got != want {
		t.Errorf("MatchCost = %v; want %v", got, want)
	}
}

// ------------------------------------------------------------------
// MatchesShapeDocValuesComponent coverage.
// ------------------------------------------------------------------

// alwaysComponent2D returns a fixed Relation for every relate() call.
// Used to drive MatchesShapeDocValuesComponent without standing up a
// real Component2D tree.
type alwaysComponent2D struct {
	rel geo.Relation
}

func (a alwaysComponent2D) MinX() float64                                      { return 0 }
func (a alwaysComponent2D) MaxX() float64                                      { return 10 }
func (a alwaysComponent2D) MinY() float64                                      { return 0 }
func (a alwaysComponent2D) MaxY() float64                                      { return 10 }
func (a alwaysComponent2D) Relate(_, _, _, _ float64) geo.Relation             { return a.rel }
func (a alwaysComponent2D) Contains(_, _ float64) bool                         { return false }
func (a alwaysComponent2D) IntersectsLine(_, _, _, _, _, _, _, _ float64) bool { return false }
func (a alwaysComponent2D) IntersectsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (a alwaysComponent2D) ContainsLine(_, _, _, _, _, _, _, _ float64) bool { return false }
func (a alwaysComponent2D) ContainsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool {
	return false
}
func (a alwaysComponent2D) WithinPoint(_, _ float64) geo.WithinRelation { return geo.WithinDisjoint }
func (a alwaysComponent2D) WithinLine(_, _, _, _, _, _ float64, _ bool, _, _ float64) geo.WithinRelation {
	return geo.WithinDisjoint
}
func (a alwaysComponent2D) WithinTriangle(_, _, _, _, _, _ float64, _ bool, _, _ float64, _ bool, _, _ float64, _ bool) geo.WithinRelation {
	return geo.WithinDisjoint
}

// TestMatchesShapeDocValuesComponent_NilShape confirms the helper
// short-circuits to false when the ShapeDocValues is nil.
func TestMatchesShapeDocValuesComponent_NilShape(t *testing.T) {
	t.Parallel()
	got, err := MatchesShapeDocValuesComponent(nil, document.QueryRelationIntersects, alwaysComponent2D{rel: geo.CellInsideQuery})
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if got {
		t.Errorf("got = true; want false on nil shape")
	}
}

// TestMatchesShapeDocValuesComponent_InsideOutsideBranches drives
// the helper with components that cover the shape's bounding box
// (CellInsideQuery) or do not (CellOutsideQuery) and walks every
// QueryRelation flavour. CROSSES is not in the table because the
// shape comparator's relate() invokes per-triangle hooks that a fake
// component cannot deterministically force into the CROSSES branch
// without re-implementing the full Component2D contract; the
// inside/outside fold is the contract this helper actually owns.
func TestMatchesShapeDocValuesComponent_InsideOutsideBranches(t *testing.T) {
	t.Parallel()
	sdv := newTestShapeDocValues(t)

	cases := []struct {
		name     string
		queryRel document.QueryRelation
		geomRel  geo.Relation
		want     bool
	}{
		{"intersects+inside", document.QueryRelationIntersects, geo.CellInsideQuery, true},
		{"intersects+outside", document.QueryRelationIntersects, geo.CellOutsideQuery, false},
		{"within+inside", document.QueryRelationWithin, geo.CellInsideQuery, true},
		{"within+outside", document.QueryRelationWithin, geo.CellOutsideQuery, false},
		{"disjoint+inside", document.QueryRelationDisjoint, geo.CellInsideQuery, true},
		{"disjoint+outside", document.QueryRelationDisjoint, geo.CellOutsideQuery, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MatchesShapeDocValuesComponent(sdv, tc.queryRel, alwaysComponent2D{rel: tc.geomRel})
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tc.want {
				t.Errorf("matches = %v; want %v", got, tc.want)
			}
		})
	}
}

// ------------------------------------------------------------------
// CreateWeight + two-phase iterator coverage.
// ------------------------------------------------------------------

// shapeStubBinaryDocValues is an in-memory BinaryDocValues used to drive
// the approximation iterator without standing up a real codec.
// Documents are kept in ascending docID order.
type shapeStubBinaryDocValues struct {
	docs    []int
	values  map[int][]byte
	current int // index into docs; -1 before first NextDoc
}

func newShapeStubBinaryDocValues(values map[int][]byte) *shapeStubBinaryDocValues {
	docs := make([]int, 0, len(values))
	for d := range values {
		docs = append(docs, d)
	}
	// Insertion sort: tiny inputs, deterministic ordering.
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j-1] > docs[j]; j-- {
			docs[j-1], docs[j] = docs[j], docs[j-1]
		}
	}
	return &shapeStubBinaryDocValues{docs: docs, values: values, current: -1}
}

// getInternal serves the random-access lookup used by BinaryValue;
// keeps the legacy Get(docID) semantics for the internal callers
// without exposing the dropped surface method.
func (b *shapeStubBinaryDocValues) getInternal(docID int) ([]byte, error) {
	if v, ok := b.values[docID]; ok {
		return v, nil
	}
	return nil, nil
}

func (b *shapeStubBinaryDocValues) Cost() int64 { return int64(len(b.docs)) }

func (b *shapeStubBinaryDocValues) Advance(target int) (int, error) {
	for i, d := range b.docs {
		if d >= target {
			b.current = i
			return d, nil
		}
	}
	b.current = len(b.docs)
	return NO_MORE_DOCS, nil
}

func (b *shapeStubBinaryDocValues) NextDoc() (int, error) {
	b.current++
	if b.current >= len(b.docs) {
		return NO_MORE_DOCS, nil
	}
	return b.docs[b.current], nil
}

func (b *shapeStubBinaryDocValues) DocID() int {
	if b.current < 0 {
		return -1
	}
	if b.current >= len(b.docs) {
		return NO_MORE_DOCS
	}
	return b.docs[b.current]
}

func (b *shapeStubBinaryDocValues) AdvanceExact(target int) (bool, error) {
	got, err := b.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (b *shapeStubBinaryDocValues) BinaryValue() ([]byte, error) {
	if b.current < 0 || b.current >= len(b.docs) {
		return nil, nil
	}
	return b.getInternal(b.docs[b.current])
}

// TestBaseShapeDocValuesQuery_TwoPhaseWalk drives a stub
// BinaryDocValues through the two-phase iterator wired by
// newScorerSupplier and asserts only the docs the match closure
// approves are surfaced.
func TestBaseShapeDocValuesQuery_TwoPhaseWalk(t *testing.T) {
	t.Parallel()

	// Decoder is a pass-through: it tags the binary value as the
	// payload so the match closure can decide based on it without
	// running the real ShapeDocValues comparator.
	type taggedShape struct{ accept bool }
	tags := map[int]bool{1: true, 3: false, 5: true}

	bdv := newShapeStubBinaryDocValues(map[int][]byte{
		1: {0x01},
		3: {0x03},
		5: {0x05},
	})

	q, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		// Decoder returns a non-nil ShapeDocValues sentinel — the
		// match override below ignores the decoded shape and consults
		// the payload's first byte directly. We need a non-nil result
		// so the production code's "nil shape ⇒ no match" guard
		// doesn't short-circuit.
		func(br *util.BytesRef) (*document.ShapeDocValues, error) {
			// Stash the decision in a closure-shared map keyed by the
			// payload byte; the match override reads from `tags`.
			_ = taggedShape{accept: tags[int(br.Bytes[0])]}
			// Return a real ShapeDocValues (a single dummy triangle)
			// so the production path treats the doc as decodable.
			return document.NewShapeDocValuesFromTessellation(
				identityShapeEncoder{},
				[]document.DecodedTriangle{triangleAt(0, 0, 1, 0, 0, 1)},
				nilGeometryFn,
				nilGeometryFn,
			)
		},
		nil,
		WithBaseShapeDocValuesMatch(func(_ *document.ShapeDocValues) (bool, error) {
			docID := bdv.DocID()
			return tags[docID], nil
		}),
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	supplier := q.newScorerSupplier(bdv /*maxDoc=*/, 10, COMPLETE_NO_SCORES, 1.0)
	if got, want := supplier.Cost(), int64(10); got != want {
		t.Errorf("Cost = %d; want %d", got, want)
	}
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if scorer == nil {
		t.Fatal("scorer is nil")
	}

	var matched []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matched = append(matched, doc)
	}
	want := []int{1, 5}
	if fmt.Sprint(matched) != fmt.Sprint(want) {
		t.Errorf("matched = %v; want %v", matched, want)
	}
}

// fakeLeafReader is a minimal LeafReaderInterface implementation that
// exposes only the surface CreateWeight consults: GetBinaryDocValues
// and GetFieldInfos. Other methods either delegate to the embedded
// *index.LeafReader (which returns nil/empty values) or return
// implementation-defined defaults.
type fakeLeafReader struct {
	*index.LeafReader
	bdv   index.BinaryDocValues
	infos *index.FieldInfos
}

func newFakeLeafReader(field string, bdv index.BinaryDocValues) *fakeLeafReader {
	opts := index.DefaultFieldInfoOptions()
	opts.DocValuesType = index.DocValuesTypeBinary
	fi := index.NewFieldInfo(field, 0, opts)
	infos := index.NewFieldInfos()
	if err := infos.Add(fi); err != nil {
		panic(err)
	}
	si := index.NewSegmentInfo("test", 10, nil)
	leaf := index.NewLeafReaderWithFieldInfos(si, infos)
	return &fakeLeafReader{LeafReader: leaf, bdv: bdv, infos: infos}
}

func (f *fakeLeafReader) GetBinaryDocValues(_ string) (index.BinaryDocValues, error) {
	return f.bdv, nil
}

func (f *fakeLeafReader) GetFieldInfos() *index.FieldInfos { return f.infos }

// TestBaseShapeDocValuesQuery_CreateWeight_MissingField confirms the
// per-leaf supplier returns nil when the configured field has no
// FieldInfo in the segment (the Java reference's "no docs indexed
// this field" fast path).
func TestBaseShapeDocValuesQuery_CreateWeight_MissingField(t *testing.T) {
	t.Parallel()

	bdv := newShapeStubBinaryDocValues(nil)
	leaf := newFakeLeafReader("other_field", bdv) // FieldInfo for a different name.
	ctx := index.NewLeafReaderContext(leaf, nil, 0, 0)

	q, err := NewBaseShapeDocValuesQuery(
		"field",
		document.QueryRelationIntersects,
		fakeComponent2D{},
		func(_ *util.BytesRef) (*document.ShapeDocValues, error) { return nil, nil },
		nil,
	)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	weight, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	supplier, err := weight.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier != nil {
		t.Errorf("ScorerSupplier = %v; want nil for missing field", supplier)
	}
}
