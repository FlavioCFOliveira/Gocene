// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ── stream stub ───────────────────────────────────────────────────────────────

// fixedPointStream supplies a fixed list of (packed-point, score) pairs.
type fixedPointStream struct {
	points [][]byte
	scores []float32
	idx    int
}

func newFixedPointStream(points [][]byte, scores []float32) *fixedPointStream {
	return &fixedPointStream{points: points, scores: scores}
}

func (s *fixedPointStream) Next() []byte {
	if s.idx >= len(s.points) {
		return nil
	}
	v := s.points[s.idx]
	s.idx++
	return v
}

func (s *fixedPointStream) Score() float32 {
	if s.idx-1 < len(s.scores) {
		return s.scores[s.idx-1]
	}
	return 0
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newPISQ(t *testing.T, points [][]byte, scores []float32) *PointInSetIncludingScoreQuery {
	t.Helper()
	stream := newFixedPointStream(points, scores)
	q, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 4, stream, nil)
	if err != nil {
		t.Fatalf("NewPointInSetIncludingScoreQuery: %v", err)
	}
	return q
}

// makeBitSetFixedOnly returns a *util.FixedBitSet for point-value tests that
// need typed access to Get(). Note: makeBitSet (in to_parent_doc_values_test.go)
// returns util.BitSet; this helper returns the concrete type.
func makeBitSetFixedOnly(t *testing.T, numBits int) (*util.FixedBitSet, error) {
	t.Helper()
	return util.NewFixedBitSet(numBits)
}

// util_newBitSetIter wraps util.NewBitSetIterator for test use.
func util_newBitSetIter(bs util.Bits, cost int64) *util.BitSetIterator {
	return util.NewBitSetIterator(bs, cost)
}

func int32ToBytes(v int32) []byte {
	b := make([]byte, 4)
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	return b
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestPointInSetIncludingScoreQuery_Construction(t *testing.T) {
	pts := [][]byte{int32ToBytes(1), int32ToBytes(3), int32ToBytes(5)}
	sc := []float32{1.0, 2.0, 3.0}
	q := newPISQ(t, pts, sc)
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if q.GetField() != "pt" {
		t.Errorf("GetField() = %q, want %q", q.GetField(), "pt")
	}
	if q.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", q.GetScoreMode())
	}
}

func TestPointInSetIncludingScoreQuery_String(t *testing.T) {
	pts := [][]byte{int32ToBytes(1)}
	sc := []float32{1.0}
	q := newPISQ(t, pts, sc)
	s := q.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestPointInSetIncludingScoreQuery_Equals(t *testing.T) {
	pts := [][]byte{int32ToBytes(1), int32ToBytes(2)}
	sc := []float32{1.0, 2.0}

	q1 := newPISQ(t, pts, sc)
	q2 := newPISQ(t, pts, sc)

	pts2 := [][]byte{int32ToBytes(1), int32ToBytes(3)}
	q3 := newPISQ(t, pts2, sc)

	if !q1.Equals(q2) {
		t.Error("q1.Equals(q2) = false, want true (identical construction)")
	}
	if q1.Equals(q3) {
		t.Error("q1.Equals(q3) = true, want false (different points)")
	}
}

func TestPointInSetIncludingScoreQuery_HashCode(t *testing.T) {
	pts := [][]byte{int32ToBytes(10)}
	sc := []float32{0.5}
	q := newPISQ(t, pts, sc)
	// Just verify it doesn't panic and returns something.
	_ = q.HashCode()
}

func TestPointInSetIncludingScoreQuery_Rewrite(t *testing.T) {
	q := newPISQ(t, [][]byte{int32ToBytes(1)}, []float32{1.0})
	r, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if r != q {
		t.Error("Rewrite should return the same query")
	}
}

func TestPointInSetIncludingScoreQuery_CreateWeight(t *testing.T) {
	q := newPISQ(t, [][]byte{int32ToBytes(1)}, []float32{1.0})
	w, err := q.CreateWeight(nil, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil weight")
	}
	if w.GetQuery() != q {
		t.Error("weight.GetQuery() != original query")
	}
}

func TestPointInSetIncludingScoreQuery_IsCacheable(t *testing.T) {
	q := newPISQ(t, [][]byte{int32ToBytes(1)}, []float32{1.0})
	w, _ := q.CreateWeight(nil, false, 1.0)
	pw := w.(*pointInSetIncludingScoreWeight)
	if !pw.IsCacheable(nil) {
		t.Error("IsCacheable() = false, want true")
	}
}

func TestPointInSetIncludingScoreQuery_ScorerNilCtx(t *testing.T) {
	q := newPISQ(t, [][]byte{int32ToBytes(1)}, []float32{1.0})
	w, _ := q.CreateWeight(nil, false, 1.0)
	pw := w.(*pointInSetIncludingScoreWeight)
	sc, err := pw.Scorer(nil)
	if err != nil {
		t.Fatalf("Scorer(nil): %v", err)
	}
	if sc == nil {
		t.Fatal("expected stub scorer for nil context")
	}
	if sc.DocID() != 0 && sc.DocID() != -1 {
		// stub scorer starts positioned; NO_MORE_DOCS is fine too.
	}
}

func TestPointInSetIncludingScoreQuery_DuplicatePointError(t *testing.T) {
	pts := [][]byte{int32ToBytes(1), int32ToBytes(1)} // duplicate
	sc := []float32{1.0, 2.0}
	stream := newFixedPointStream(pts, sc)
	_, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 4, stream, nil)
	if err == nil {
		t.Error("expected error for duplicate points, got nil")
	}
}

func TestPointInSetIncludingScoreQuery_OutOfOrderPointError(t *testing.T) {
	pts := [][]byte{int32ToBytes(5), int32ToBytes(1)} // descending
	sc := []float32{1.0, 2.0}
	stream := newFixedPointStream(pts, sc)
	_, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 4, stream, nil)
	if err == nil {
		t.Error("expected error for out-of-order points, got nil")
	}
}

func TestPointInSetIncludingScoreQuery_WrongBytesPerDim(t *testing.T) {
	pts := [][]byte{{1, 2}} // 2 bytes, but bytesPerDim=4
	sc := []float32{1.0}
	stream := newFixedPointStream(pts, sc)
	_, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 4, stream, nil)
	if err == nil {
		t.Error("expected error for wrong bytesPerDim, got nil")
	}
}

func TestPointInSetIncludingScoreQuery_InvalidBytesPerDim(t *testing.T) {
	stream := newFixedPointStream(nil, nil)
	_, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 0, stream, nil)
	if err == nil {
		t.Error("expected error for bytesPerDim=0, got nil")
	}
}

// TestPointInSetIncludingScoreQuery_MergeVisitor_Compare verifies that the
// mergePointVisitor.Compare method correctly classifies cells.
func TestPointInSetIncludingScoreQuery_MergeVisitor_Compare(t *testing.T) {
	pts := [][]byte{int32ToBytes(10), int32ToBytes(20)}
	sc := []float32{1.0, 2.0}
	q := newPISQ(t, pts, sc)

	result, err := makeBitSetFixedOnly(t, 64)
	if err != nil {
		t.Fatalf("makeBitSetFixedOnly: %v", err)
	}
	scores := make([]float32, 64)
	v := &mergePointVisitor{q: q, result: result, scores: scores}
	v.reset()

	// Cell range [1..9]: query point 10 is above — CELL_OUTSIDE_QUERY (0).
	rel := v.Compare(int32ToBytes(1), int32ToBytes(9))
	if rel != 0 {
		t.Errorf("Compare([1,9]) = %d, want 0 (CELL_OUTSIDE_QUERY)", rel)
	}
	// Cell range [5..15]: query point 10 falls inside — CELL_CROSSES_QUERY (2).
	rel = v.Compare(int32ToBytes(5), int32ToBytes(15))
	if rel != 2 {
		t.Errorf("Compare([5,15]) = %d, want 2 (CELL_CROSSES_QUERY)", rel)
	}
}

// TestMergePointVisitor_VisitByPackedValue exercises the match logic directly.
func TestMergePointVisitor_VisitByPackedValue(t *testing.T) {
	pts := [][]byte{int32ToBytes(5), int32ToBytes(10)}
	sc := []float32{1.5, 2.5}
	stream := newFixedPointStream(pts, sc)
	q, err := NewPointInSetIncludingScoreQuery(Max, nil, false, "pt", 4, stream, nil)
	if err != nil {
		t.Fatalf("NewPointInSetIncludingScoreQuery: %v", err)
	}

	result, err2 := makeBitSetFixedOnly(t, 16)
	if err2 != nil {
		t.Fatalf("makeBitSet: %v", err2)
	}
	scores := make([]float32, 16)
	v := &mergePointVisitor{q: q, result: result, scores: scores}
	v.reset()

	// Visit docID=3 with packed value matching pts[0]=5.
	if err := v.VisitByPackedValue(3, int32ToBytes(5)); err != nil {
		t.Fatalf("VisitByPackedValue: %v", err)
	}
	if !result.Get(3) {
		t.Error("docID 3 should be set after matching visit")
	}
	if scores[3] != 1.5 {
		t.Errorf("scores[3] = %v, want 1.5", scores[3])
	}

	// Visit docID=7 with non-matching value.
	if err := v.VisitByPackedValue(7, int32ToBytes(99)); err != nil {
		t.Fatalf("VisitByPackedValue: %v", err)
	}
	if result.Get(7) {
		t.Error("docID 7 should NOT be set for non-matching value")
	}
}

// TestPointInSetIncludingScoreScorer_Score verifies score retrieval.
func TestPointInSetIncludingScoreScorer_Score(t *testing.T) {
	result, _ := makeBitSetFixedOnly(t, 4)
	result.Set(2)
	scores := []float32{0, 0, 3.14, 0}

	sc := &pointInSetIncludingScoreScorer{
		disi:   util_newBitSetIter(result, 1),
		scores: scores,
	}
	doc, _ := sc.NextDoc()
	if doc != 2 {
		t.Fatalf("NextDoc() = %d, want 2", doc)
	}
	if math.Abs(float64(sc.Score()-3.14)) > 0.001 {
		t.Errorf("Score() = %v, want 3.14", sc.Score())
	}
}
