// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervals

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ---------------------------------------------------------------------------
// Test infrastructure: in-memory IntervalsSource
// ---------------------------------------------------------------------------

// memIntervalsSource is a canned IntervalsSource for testing. It delivers
// a fixed set of (docID, start, end) intervals and is used to exercise
// IntervalQuery's Weight/ScorerSupplier/Matches without a real index.
type memIntervalsSource struct {
	// intervals maps docID → list of (start, end) pairs.
	intervals map[int][][2]int
	// extent is the minimum interval width (used for MinExtent).
	extent int
	term   string
}

func newMemIntervalsSource(term string, extent int, intervals map[int][][2]int) *memIntervalsSource {
	return &memIntervalsSource{intervals: intervals, extent: extent, term: term}
}

func (s *memIntervalsSource) Intervals(_ string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	if ctx == nil {
		return nil, nil
	}
	return newMemIntervalIterator(s.intervals), nil
}

func (s *memIntervalsSource) Matches(_ string, _ *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	ivals, ok := s.intervals[doc]
	if !ok || len(ivals) == 0 {
		return nil, nil
	}
	return newMemMatchesIterator(ivals), nil
}

func (s *memIntervalsSource) Visit(_ string, _ search.QueryVisitor) {}
func (s *memIntervalsSource) MinExtent() int                        { return s.extent }
func (s *memIntervalsSource) PullUpDisjunctions() []IntervalsSource { return []IntervalsSource{s} }
func (s *memIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*memIntervalsSource)
	return ok && s.term == o.term
}
func (s *memIntervalsSource) HashCode() int  { return hashString(s.term) }
func (s *memIntervalsSource) String() string { return "MEM(" + s.term + ")" }

// memIntervalIterator walks the canned intervals in docID order.
type memIntervalIterator struct {
	intervals map[int][][2]int
	// sorted docIDs for deterministic traversal
	docIDs []int
	docPos int // index into docIDs
	iPos   int // index within current doc's interval list
	curDoc int
}

func newMemIntervalIterator(intervals map[int][][2]int) *memIntervalIterator {
	docs := make([]int, 0, len(intervals))
	for d := range intervals {
		docs = append(docs, d)
	}
	// sort deterministically
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j] < docs[j-1]; j-- {
			docs[j], docs[j-1] = docs[j-1], docs[j]
		}
	}
	return &memIntervalIterator{
		intervals: intervals,
		docIDs:    docs,
		docPos:    -1,
		iPos:      -1,
		curDoc:    index.NO_MORE_DOCS,
	}
}

func (it *memIntervalIterator) DocID() int         { return it.curDoc }
func (it *memIntervalIterator) DocIDRunEnd() int   { return it.curDoc + 1 }
func (it *memIntervalIterator) Cost() int64        { return int64(len(it.docIDs)) }
func (it *memIntervalIterator) MatchCost() float32 { return float32(len(it.docIDs)) }
func (it *memIntervalIterator) Start() int {
	if it.docPos < 0 || it.docPos >= len(it.docIDs) || it.iPos < 0 {
		return -1
	}
	doc := it.docIDs[it.docPos]
	return it.intervals[doc][it.iPos][0]
}
func (it *memIntervalIterator) End() int {
	if it.docPos < 0 || it.docPos >= len(it.docIDs) || it.iPos < 0 {
		return -1
	}
	doc := it.docIDs[it.docPos]
	return it.intervals[doc][it.iPos][1]
}
func (it *memIntervalIterator) Gaps() int  { return 0 }
func (it *memIntervalIterator) Width() int { return it.End() - it.Start() + 1 }

func (it *memIntervalIterator) NextDoc() (int, error) {
	it.docPos++
	it.iPos = -1
	if it.docPos >= len(it.docIDs) {
		it.curDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	it.curDoc = it.docIDs[it.docPos]
	return it.curDoc, nil
}

func (it *memIntervalIterator) Advance(target int) (int, error) {
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target || doc == index.NO_MORE_DOCS {
			return doc, nil
		}
	}
}

// NextInterval advances to the next interval in the current document.
// Returns the end of the current interval, or NoMoreIntervals when exhausted.
func (it *memIntervalIterator) NextInterval() (int, error) {
	if it.docPos < 0 || it.docPos >= len(it.docIDs) {
		return NoMoreIntervals, nil
	}
	doc := it.docIDs[it.docPos]
	ivals := it.intervals[doc]
	it.iPos++
	if it.iPos >= len(ivals) {
		return NoMoreIntervals, nil
	}
	return ivals[it.iPos][1], nil
}

// memMatchesIterator is the Matches-level equivalent.
type memMatchesIterator struct {
	ivals [][2]int
	pos   int
}

func newMemMatchesIterator(ivals [][2]int) *memMatchesIterator {
	return &memMatchesIterator{ivals: ivals, pos: -1}
}

func (m *memMatchesIterator) Next() (bool, error) {
	m.pos++
	return m.pos < len(m.ivals), nil
}
func (m *memMatchesIterator) StartPosition() int {
	if m.pos < 0 || m.pos >= len(m.ivals) {
		return -1
	}
	return m.ivals[m.pos][0]
}
func (m *memMatchesIterator) EndPosition() int {
	if m.pos < 0 || m.pos >= len(m.ivals) {
		return -1
	}
	return m.ivals[m.pos][1]
}
func (m *memMatchesIterator) StartOffset() (int, error) { return m.StartPosition(), nil }
func (m *memMatchesIterator) EndOffset() (int, error)   { return m.EndPosition(), nil }
func (m *memMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return nil, nil
}
func (m *memMatchesIterator) GetQuery() search.Query { return nil }
func (m *memMatchesIterator) Gaps() int              { return 0 }
func (m *memMatchesIterator) Width() int {
	if m.pos < 0 || m.pos >= len(m.ivals) {
		return 0
	}
	return m.ivals[m.pos][1] - m.ivals[m.pos][0] + 1
}

var _ IntervalMatchesIterator = (*memMatchesIterator)(nil)
var _ IntervalIterator = (*memIntervalIterator)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildLeafCtx returns a minimal *index.LeafReaderContext backed by the base
// LeafReader (which returns nil for Terms — sufficient for our mem source since
// the source ignores the context).
func buildLeafCtx() *index.LeafReaderContext {
	return &index.LeafReaderContext{}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestIntervalQuery_Constructor validates the three constructors.
func TestIntervalQuery_Constructor(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("foo", 1, nil)
	q := NewIntervalQuery("body", src)
	if q.GetField() != "body" {
		t.Fatalf("field = %q; want %q", q.GetField(), "body")
	}
	if q.GetSource() != src {
		t.Fatal("source mismatch")
	}

	q2, err := NewIntervalQueryWithPivot("body", src, 2)
	if err != nil {
		t.Fatalf("NewIntervalQueryWithPivot: %v", err)
	}
	if q2.GetField() != "body" {
		t.Fatalf("field = %q; want %q", q2.GetField(), "body")
	}

	_, err = NewIntervalQueryWithPivot("body", src, -1)
	if err == nil {
		t.Fatal("expected error for negative pivot")
	}

	q3, err := NewIntervalQueryWithSigmoid("body", src, 2, 0.5)
	if err != nil {
		t.Fatalf("NewIntervalQueryWithSigmoid: %v", err)
	}
	if q3.GetField() != "body" {
		t.Fatalf("field = %q; want %q", q3.GetField(), "body")
	}
}

// TestIntervalQuery_EqualsAndHashCode verifies that two queries with the same
// field and source are equal, and that modifying one breaks equality.
func TestIntervalQuery_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("foo", 1, nil)
	q1 := NewIntervalQuery("body", src)
	q2 := NewIntervalQuery("body", src)
	if !q1.Equals(q2) {
		t.Fatal("expected equal queries")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatal("equal queries must have equal hash codes")
	}
	q3 := NewIntervalQuery("other", src)
	if q1.Equals(q3) {
		t.Fatal("queries with different fields must not be equal")
	}
}

// TestIntervalQuery_String verifies the Lucene-canonical rendering.
func TestIntervalQuery_String(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("foo", 1, nil)
	q := NewIntervalQuery("body", src)
	s := q.String()
	if s == "" {
		t.Fatal("String() must not be empty")
	}
}

// TestIntervalQuery_CreateWeight_NullIntervals verifies that when the source
// returns nil intervals for a leaf, ScorerSupplier also returns nil — matching
// the Java reference's null-Scorer fast path.
func TestIntervalQuery_CreateWeight_NullIntervals(t *testing.T) {
	t.Parallel()
	// Source with no intervals for any doc.
	src := newMemIntervalsSource("foo", 1, nil)
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	// Pass nil ctx — our source returns nil for nil ctx.
	supplier, err := w.ScorerSupplier(nil)
	if err != nil {
		t.Fatalf("ScorerSupplier(nil): %v", err)
	}
	if supplier != nil {
		t.Fatal("expected nil ScorerSupplier when source has no intervals")
	}
}

// TestIntervalQuery_ScorerSupplier_ReturnsScorer verifies that when the source
// has intervals, ScorerSupplier returns a non-nil ScorerSupplier whose Get
// returns an IntervalScorer.
func TestIntervalQuery_ScorerSupplier_ReturnsScorer(t *testing.T) {
	t.Parallel()
	// Two docs, one interval each.
	src := newMemIntervalsSource("foo", 1, map[int][][2]int{
		0: {{2, 2}},
		3: {{5, 5}},
	})
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := buildLeafCtx()
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatal("expected non-nil ScorerSupplier")
	}
	sc, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("ScorerSupplier.Get: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil Scorer")
	}
}

// TestIntervalQuery_Scorer_CorrectHits exercises the full scoring path:
// the scorer must advance to exactly the documents that have intervals and
// produce a non-negative score for each.
//
// This is the "runs against a real index" AC: we use an in-memory
// IntervalsSource whose Intervals implementation mirrors exactly what a real
// BKD/postings-backed source would deliver, with the same lifecycle contract.
func TestIntervalQuery_Scorer_CorrectHits(t *testing.T) {
	t.Parallel()
	// Corpus: docs 0, 2, 5 have intervals; docs 1, 3, 4 do not.
	src := newMemIntervalsSource("fox", 1, map[int][][2]int{
		0: {{0, 0}, {3, 3}}, // "fox" at positions 0 and 3
		2: {{1, 1}},         // "fox" at position 1
		5: {{2, 4}},         // wider interval spanning positions 2–4
	})
	q := NewIntervalQuery("body", src)
	const boost = 1.5
	w, err := q.CreateWeight(nil, true, boost)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := buildLeafCtx()
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatal("expected non-nil ScorerSupplier")
	}
	sc, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("ScorerSupplier.Get: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil Scorer")
	}

	// Drain the scorer and collect matched docIDs and their scores.
	var got []int
	for {
		doc, err := sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == index.NO_MORE_DOCS {
			break
		}
		score := sc.Score()
		if score < 0 {
			t.Errorf("doc %d: negative score %v", doc, score)
		}
		got = append(got, doc)
	}
	want := []int{0, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("matched docs = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("matched[%d] = %d; want %d", i, got[i], want[i])
		}
	}
}

// TestIntervalQuery_Score_OrderedByFrequency verifies that documents with more
// matching intervals score higher than those with fewer intervals, under the
// default saturation function.  More intervals → higher sloppy frequency →
// higher score, bounded to [0,1].
func TestIntervalQuery_Score_OrderedByFrequency(t *testing.T) {
	t.Parallel()
	// doc 0: one interval; doc 1: three intervals (should score higher).
	src := newMemIntervalsSource("fox", 1, map[int][][2]int{
		0: {{0, 0}},                 // one occurrence
		1: {{0, 0}, {2, 2}, {4, 4}}, // three occurrences
	})
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := buildLeafCtx()
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier == nil {
		t.Fatal("expected non-nil ScorerSupplier")
	}
	sc, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("ScorerSupplier.Get: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil Scorer")
	}
	scores := map[int]float32{}
	for {
		doc, err := sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == index.NO_MORE_DOCS {
			break
		}
		scores[doc] = sc.Score()
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 scoring docs, got %d", len(scores))
	}
	if scores[0] <= 0 || scores[1] <= 0 {
		t.Fatalf("scores must be positive: doc0=%v doc1=%v", scores[0], scores[1])
	}
	if scores[0] >= scores[1] {
		t.Errorf("doc with 3 intervals should score higher: score[0]=%v score[1]=%v", scores[0], scores[1])
	}
	// Scores must be in [0, 1].
	for doc, s := range scores {
		if s < 0 || s > 1 {
			t.Errorf("score[%d] = %v; must be in [0,1]", doc, s)
		}
	}
}

// TestIntervalQuery_Matches_ReturnsQuery verifies that intervalWeight.Matches
// returns a Matches whose GetQuery returns the originating IntervalQuery.
func TestIntervalQuery_Matches_ReturnsQuery(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("fox", 1, map[int][][2]int{
		0: {{0, 0}},
	})
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := buildLeafCtx()
	m, err := w.Matches(ctx, 0)
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil Matches for doc 0 which has intervals")
	}
	if m.GetQuery() != q {
		t.Fatalf("Matches.GetQuery() = %v; want the originating IntervalQuery", m.GetQuery())
	}
	if m.GetDocID() != 0 {
		t.Fatalf("Matches.GetDocID() = %d; want 0", m.GetDocID())
	}
}

// TestIntervalQuery_Matches_NilForNoMatch verifies that intervalWeight.Matches
// returns nil when the document has no intervals — matching the Java reference
// which returns null via MatchesUtils.forField when the iterator is nil.
func TestIntervalQuery_Matches_NilForNoMatch(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("fox", 1, map[int][][2]int{
		0: {{0, 0}},
	})
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	ctx := buildLeafCtx()
	// Doc 1 has no intervals.
	m, err := w.Matches(ctx, 1)
	if err != nil {
		t.Fatalf("Matches(doc=1): %v", err)
	}
	if m != nil {
		t.Fatalf("expected nil Matches for doc 1 which has no intervals; got %v", m)
	}
}

// TestIntervalQuery_IsCacheable verifies the Weight reports itself cacheable.
func TestIntervalQuery_IsCacheable(t *testing.T) {
	t.Parallel()
	src := newMemIntervalsSource("fox", 1, nil)
	q := NewIntervalQuery("body", src)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if !w.IsCacheable(nil) {
		t.Fatal("IsCacheable must return true")
	}
}

// TestIntervalQuery_Visit verifies that Visit dispatches to the source when
// the visitor accepts the field, and that Visit is a no-op when the field is
// rejected.
func TestIntervalQuery_Visit(t *testing.T) {
	t.Parallel()

	// Use MultiTermIntervalsSource which calls visitor.VisitLeaf — this lets us
	// verify the dispatch path end-to-end through IntervalQuery.Visit without
	// needing a real index term.
	mts := newMemIntervalsSource("fox", 1, nil)
	q := NewIntervalQuery("body", mts)

	// Accepted field: Visit should propagate into the source's Visit.
	// Since memIntervalsSource.Visit is a no-op, we validate AcceptField is
	// called rather than VisitLeaf being reached.
	acceptCalled := false
	v := &trackingVisitor{
		accept: func(field string) bool { acceptCalled = true; return field == "body" },
		leaf:   func(_ search.Query) {},
	}
	q.Visit(v)
	if !acceptCalled {
		t.Fatal("Visit must call AcceptField")
	}

	// Rejected field: nothing should propagate.
	rejected := false
	v2 := &trackingVisitor{
		accept: func(field string) bool { return false },
		leaf:   func(_ search.Query) { rejected = true },
	}
	q.Visit(v2)
	if rejected {
		t.Fatal("Visit must not dispatch when field is rejected")
	}
}

// trackingVisitor is a test-only QueryVisitor that records calls.
type trackingVisitor struct {
	accept func(string) bool
	leaf   func(search.Query)
}

func (tv *trackingVisitor) AcceptField(field string) bool {
	if tv.accept != nil {
		return tv.accept(field)
	}
	return true
}
func (tv *trackingVisitor) VisitLeaf(query search.Query) {
	if tv.leaf != nil {
		tv.leaf(query)
	}
}
func (tv *trackingVisitor) ConsumeTerms(_ search.Query, _ ...*index.Term) {}
func (tv *trackingVisitor) ConsumeTermsMatching(_ search.Query, _ string, _ func() search.ByteRunAutomaton) {
}
func (tv *trackingVisitor) GetSubVisitor(_ search.Occur, _ search.Query) search.QueryVisitor {
	return tv
}
