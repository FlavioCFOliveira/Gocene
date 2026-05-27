// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file holds resolver-driven tests for the real Scorer / collector
// logic wired in T4677 (join: BitSetProducer, BlockJoinScorer, JoinUtil,
// TermsWithScoreCollector). The end-to-end DirectoryReader-backed scenarios
// remain blocked on the SegmentReader core-readers gap (see T4665); these
// tests exercise the algorithmic substance through a deterministic fixture
// resolver and a fake Scorer/Weight that bypass the codec path.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ---------- fake scorer / weight / query --------------------------------

// fakeScorer is an in-memory Scorer driven by a precomputed slice of
// (doc, score) pairs. It exists solely to drive the join scorers and the
// terms collector adapter without going through the codec.
type fakeScorer struct {
	docs   []int
	scores []float32
	idx    int

	maxScore float32
	cost     int64
}

func newFakeScorer(docs []int, scores []float32, maxScore float32) *fakeScorer {
	return &fakeScorer{
		docs:     docs,
		scores:   scores,
		idx:      -1,
		maxScore: maxScore,
		cost:     int64(len(docs)),
	}
}

func (s *fakeScorer) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}

func (s *fakeScorer) NextDoc() (int, error) {
	s.idx++
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS, nil
	}
	return s.docs[s.idx], nil
}

func (s *fakeScorer) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == search.NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

func (s *fakeScorer) Cost() int64      { return s.cost }
func (s *fakeScorer) DocIDRunEnd() int { return s.DocID() + 1 }
func (s *fakeScorer) Score() float32 {
	if s.idx < 0 || s.idx >= len(s.scores) {
		return 0
	}
	return s.scores[s.idx]
}
func (s *fakeScorer) GetMaxScore(upTo int) float32 { return s.maxScore }

// ---------- BlockJoinScorer.GetMaxScore / Cost --------------------------

func TestBlockJoinScorer_GetMaxScore_PerScoreMode(t *testing.T) {
	cases := []struct {
		name      string
		mode      ScoreMode
		parentMax float32
		childMax  float32
		wantMax   float32
	}{
		{"None_returns_parent", None, 2.0, 5.0, 2.0},
		{"Max_picks_larger", Max, 2.0, 5.0, 5.0},
		{"Max_picks_larger_parent", Max, 7.0, 5.0, 7.0},
		{"Min_picks_smaller", Min, 2.0, 5.0, 2.0},
		{"Min_picks_smaller_child", Min, 7.0, 5.0, 5.0},
		{"Avg_averages", Avg, 4.0, 6.0, 5.0},
		{"Total_sums", Total, 4.0, 6.0, 10.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := newFakeScorer([]int{0}, []float32{tc.parentMax}, tc.parentMax)
			child := newFakeScorer([]int{0}, []float32{tc.childMax}, tc.childMax)
			s := NewBlockJoinScorer(child, parent, tc.mode)
			if got := s.GetMaxScore(100); got != tc.wantMax {
				t.Errorf("GetMaxScore: got %v want %v", got, tc.wantMax)
			}
		})
	}
}

func TestBlockJoinScorer_GetMaxScore_NoneIgnoresChild(t *testing.T) {
	// Even with a child scorer present, ScoreMode.None must ignore it.
	parent := newFakeScorer([]int{0}, []float32{3.5}, 3.5)
	child := newFakeScorer([]int{0}, []float32{42.0}, 42.0)
	s := NewBlockJoinScorer(child, parent, None)
	if got := s.GetMaxScore(10); got != 3.5 {
		t.Errorf("GetMaxScore with None: got %v want 3.5", got)
	}
}

func TestBlockJoinScorer_Cost_SumsParentAndChild(t *testing.T) {
	parent := newFakeScorer(make([]int, 5), make([]float32, 5), 1)
	child := newFakeScorer(make([]int, 11), make([]float32, 11), 1)
	s := NewBlockJoinScorer(child, parent, Total)
	if got := s.Cost(); got != 16 {
		t.Errorf("Cost: got %d want 16", got)
	}
}

func TestBlockJoinScorer_Cost_HandlesNilChild(t *testing.T) {
	parent := newFakeScorer(make([]int, 7), make([]float32, 7), 1)
	s := NewBlockJoinScorer(nil, parent, None)
	if got := s.Cost(); got != 7 {
		t.Errorf("Cost with nil child: got %d want 7", got)
	}
}

// ---------- QueryBitSetProducer wiring ----------------------------------

// fakeWeight wraps a precomputed fakeScorer; QueryBitSetProducer calls
// CreateWeight then Scorer(ctx), so this is all we need.
type fakeWeight struct{ scorer search.Scorer }

func (w *fakeWeight) GetQuery() search.Query { return nil }
func (w *fakeWeight) Explain(*index.LeafReaderContext, int) (search.Explanation, error) {
	return nil, nil
}
func (w *fakeWeight) ScorerSupplier(*index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}
func (w *fakeWeight) Scorer(*index.LeafReaderContext) (search.Scorer, error) { return w.scorer, nil }
func (w *fakeWeight) BulkScorer(*index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}
func (w *fakeWeight) IsCacheable(*index.LeafReaderContext) bool { return false }
func (w *fakeWeight) Count(*index.LeafReaderContext) (int, error) {
	return -1, nil
}
func (w *fakeWeight) Matches(*index.LeafReaderContext, int) (search.Matches, error) {
	return nil, nil
}

// fakeQuery returns a fakeWeight so QueryBitSetProducer can run its pipeline
// without any codec involvement.
type fakeQuery struct{ scorer search.Scorer }

func (q *fakeQuery) Rewrite(search.IndexReader) (search.Query, error) { return q, nil }
func (q *fakeQuery) Clone() search.Query                              { return q }
func (q *fakeQuery) Equals(search.Query) bool                         { return false }
func (q *fakeQuery) HashCode() int                                    { return 0 }
func (q *fakeQuery) CreateWeight(*search.IndexSearcher, bool, float32) (search.Weight, error) {
	return &fakeWeight{scorer: q.scorer}, nil
}

// newFakeIndexReader returns an in-memory IndexReader with the requested
// MaxDoc/NumDocs. It satisfies IndexReaderInterface without touching any
// codec machinery, which lets us drive QueryBitSetProducer / TermsWithScore
// collectors via the fakeQuery+fakeScorer pair declared above.
func newFakeIndexReader(maxDoc int) *index.IndexReader {
	r := index.NewIndexReader()
	r.SetMaxDoc(maxDoc)
	r.SetNumDocs(maxDoc)
	return r
}

func TestQueryBitSetProducer_SetsMatchingBits(t *testing.T) {
	matches := []int{0, 3, 4, 7}
	scorer := newFakeScorer(matches, []float32{0, 0, 0, 0}, 0)
	q := &fakeQuery{scorer: scorer}
	p := NewQueryBitSetProducer(q)

	reader := newFakeIndexReader(10)
	ctx := index.NewLeafReaderContext(reader, nil, 0, 0)

	bs, err := p.GetBitSet(ctx)
	if err != nil {
		t.Fatalf("GetBitSet failed: %v", err)
	}
	if bs.Cardinality() != len(matches) {
		t.Errorf("Cardinality: got %d want %d", bs.Cardinality(), len(matches))
	}
	for _, d := range matches {
		if !bs.Get(d) {
			t.Errorf("expected bit %d set", d)
		}
	}
	if bs.Get(1) || bs.Get(2) || bs.Get(5) || bs.Get(6) || bs.Get(8) || bs.Get(9) {
		t.Errorf("unexpected bits set: %+v", bs)
	}
}

func TestQueryBitSetProducer_NilQueryReturnsEmpty(t *testing.T) {
	p := NewQueryBitSetProducer(nil)
	reader := newFakeIndexReader(4)
	ctx := index.NewLeafReaderContext(reader, nil, 0, 0)
	bs, err := p.GetBitSet(ctx)
	if err != nil {
		t.Fatalf("GetBitSet failed: %v", err)
	}
	if bs.Cardinality() != 0 {
		t.Errorf("expected empty bitset, got cardinality %d", bs.Cardinality())
	}
}

// ---------- TermsWithScoreCollector wiring ------------------------------

// fixtureResolver returns deterministic field values keyed by doc id.
type fixtureResolver struct {
	values map[int][]byte
}

func (r fixtureResolver) ResolveJoinValue(_ *search.IndexSearcher, doc int, _ string) ([]byte, error) {
	v, ok := r.values[doc]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func TestCollectTermsWithResolver_DeduplicatesByMode(t *testing.T) {
	// Two distinct terms: "alpha" (doc 0,2) and "beta" (doc 1).
	resolver := fixtureResolver{values: map[int][]byte{
		0: []byte("alpha"),
		1: []byte("beta"),
		2: []byte("alpha"),
	}}
	scorer := newFakeScorer([]int{0, 1, 2}, []float32{2.0, 1.0, 4.0}, 4.0)
	q := &fakeQuery{scorer: scorer}
	reader := newFakeIndexReader(3)
	searcher := search.NewIndexSearcher(reader)

	twss, err := CollectTermsWithScoresWithResolver(searcher, q, "f", Max, resolver)
	if err != nil {
		t.Fatalf("CollectTermsWithScoresWithResolver failed: %v", err)
	}
	if len(twss) != 2 {
		t.Fatalf("expected 2 distinct terms, got %d", len(twss))
	}
	scores := map[string]float32{}
	for _, t := range twss {
		scores[string(t.Term)] = t.Score
	}
	// Max mode keeps the larger score for duplicates: alpha=max(2,4)=4, beta=1.
	if got := scores["alpha"]; got != 4.0 {
		t.Errorf("alpha Max score: got %v want 4.0", got)
	}
	if got := scores["beta"]; got != 1.0 {
		t.Errorf("beta score: got %v want 1.0", got)
	}
}

func TestCollectTermsWithResolver_SkipsDocsWithoutValue(t *testing.T) {
	// Only doc 1 has a value: doc 0 and 2 must be silently skipped.
	resolver := fixtureResolver{values: map[int][]byte{
		1: []byte("only"),
	}}
	scorer := newFakeScorer([]int{0, 1, 2}, []float32{0.5, 0.5, 0.5}, 0.5)
	q := &fakeQuery{scorer: scorer}
	reader := newFakeIndexReader(3)
	searcher := search.NewIndexSearcher(reader)

	terms, err := CollectTermsWithResolver(searcher, q, "f", None, resolver)
	if err != nil {
		t.Fatalf("CollectTermsWithResolver failed: %v", err)
	}
	if len(terms) != 1 || string(terms[0]) != "only" {
		t.Errorf("expected [\"only\"], got %v", terms)
	}
}

// ---------- JoinUtil.BuildBitSet wiring ---------------------------------

func TestJoinUtil_BuildBitSet_RunsQueryAndSetsBits(t *testing.T) {
	matches := []int{0, 2, 4}
	scorer := newFakeScorer(matches, []float32{0, 0, 0}, 0)
	q := &fakeQuery{scorer: scorer}

	reader := index.NewIndexReader()
	reader.SetMaxDoc(8)
	reader.SetNumDocs(8)

	bitSet, err := NewJoinUtil().BuildBitSet(q, reader)
	if err != nil {
		t.Fatalf("BuildBitSet failed: %v", err)
	}
	if bitSet.Cardinality() != len(matches) {
		t.Errorf("Cardinality: got %d want %d", bitSet.Cardinality(), len(matches))
	}
	for _, d := range matches {
		if !bitSet.Get(d) {
			t.Errorf("expected bit %d set", d)
		}
	}
}

func TestJoinUtil_BuildBitSet_NilReaderReturnsEmpty(t *testing.T) {
	bs, err := NewJoinUtil().BuildBitSet(&fakeQuery{}, nil)
	if err != nil {
		t.Fatalf("BuildBitSet failed: %v", err)
	}
	if bs.Length() != 0 {
		t.Errorf("expected length 0, got %d", bs.Length())
	}
}
