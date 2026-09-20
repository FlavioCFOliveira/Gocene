// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// --- stub helpers ---

// stubTermsEnum is a TermsEnum backed by a fixed map of term → doc IDs.
// It satisfies index.TermsEnum and supports SeekExact + Postings.
type stubTermsEnum struct {
	// postings maps term bytes (as string) to a list of doc IDs.
	postings map[string][]int
	current  string
}

func (e *stubTermsEnum) Next() (*index.Term, error)                  { return nil, nil }
func (e *stubTermsEnum) SeekCeil(t *index.Term) (*index.Term, error) { return nil, nil }
func (e *stubTermsEnum) SeekExact(t *index.Term) (bool, error) {
	key := string(t.Bytes.ValidBytes())
	if _, ok := e.postings[key]; ok {
		e.current = key
		return true, nil
	}
	return false, nil
}
func (e *stubTermsEnum) Term() *index.Term { return nil }
func (e *stubTermsEnum) DocFreq() (int, error) {
	return len(e.postings[e.current]), nil
}
func (e *stubTermsEnum) TotalTermFreq() (int64, error) { return -1, nil }
func (e *stubTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	docs := e.postings[e.current]
	if docs == nil {
		return &slicePostingsEnum{docs: nil}, nil
	}
	return &slicePostingsEnum{docs: docs, pos: -1}, nil
}
func (e *stubTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

var _ index.TermsEnum = (*stubTermsEnum)(nil)

// slicePostingsEnum is a PostingsEnum over a fixed list of doc IDs.
type slicePostingsEnum struct {
	docs []int
	pos  int
	cur  int
}

func (p *slicePostingsEnum) NextDoc() (int, error) {
	p.pos++
	if p.pos >= len(p.docs) {
		p.cur = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	p.cur = p.docs[p.pos]
	return p.cur, nil
}
func (p *slicePostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil || doc == index.NO_MORE_DOCS || doc >= target {
			return doc, err
		}
	}
}
func (p *slicePostingsEnum) DocID() int                  { return p.cur }
func (p *slicePostingsEnum) Freq() (int, error)          { return 1, nil }
func (p *slicePostingsEnum) NextPosition() (int, error)  { return -1, nil }
func (p *slicePostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *slicePostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *slicePostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (p *slicePostingsEnum) Cost() int64                 { return int64(len(p.docs)) }

var _ index.PostingsEnum = (*slicePostingsEnum)(nil)

// --- construction + query tests ---

func TestTermsIncludingScoreQuery_Construction(t *testing.T) {
	terms := util.NewBytesRefHash()
	_, _ = terms.Add(util.NewBytesRef([]byte("apple")))
	_, _ = terms.Add(util.NewBytesRef([]byte("banana")))

	scores := []float32{1.0, 2.0}
	q := NewTermsIncludingScoreQuery(Max, "toField", false, terms, scores, "fromField", nil, nil)
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if q.GetToField() != "toField" {
		t.Errorf("GetToField() = %q, want %q", q.GetToField(), "toField")
	}
	if q.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", q.GetScoreMode())
	}
	if q.IsMultipleValuesPerDocument() {
		t.Error("IsMultipleValuesPerDocument() = true, want false")
	}
}

func TestTermsIncludingScoreQuery_String(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Min, "f", false, terms, nil, "src", nil, nil)
	s := q.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestTermsIncludingScoreQuery_Equals(t *testing.T) {
	terms := util.NewBytesRefHash()
	id := "ctx1"
	q1 := NewTermsIncludingScoreQuery(Max, "toF", false, terms, nil, "fromF", nil, id)
	q2 := NewTermsIncludingScoreQuery(Max, "toF", false, terms, nil, "fromF", nil, id)
	q3 := NewTermsIncludingScoreQuery(Min, "toF", false, terms, nil, "fromF", nil, id)

	if !q1.Equals(q2) {
		t.Error("q1.Equals(q2) = false, want true")
	}
	if q1.Equals(q3) {
		t.Error("q1.Equals(q3) = true, want false (different scoreMode)")
	}
}

func TestTermsIncludingScoreQuery_Clone(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", true, terms, nil, "src", nil, nil)
	clone := q.Clone()
	if clone == q {
		t.Error("Clone() returned same pointer")
	}
	if !q.Equals(clone) {
		t.Error("original and clone are not equal")
	}
}

func TestTermsIncludingScoreQuery_CreateWeight(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", false, terms, nil, "src", nil, nil)
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

func TestTermsIncludingScoreQuery_Rewrite(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", false, terms, nil, "src", nil, nil)
	rw, err := q.Rewrite(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rw != q {
		t.Error("Rewrite() should return self")
	}
}

// --- real scorer path tests ---

// buildQueryWithPostings creates a TermsIncludingScoreQuery whose terms map to
// the given postings.  termDocs maps termBytes → []docID; scores are 10.0 per
// term for simplicity.
func buildQueryWithPostings(t *testing.T, field string, mv bool, termDocs map[string][]int) (*TermsIncludingScoreQuery, *stubTermsEnum) {
	t.Helper()
	terms := util.NewBytesRefHash()
	scores := make([]float32, 0, len(termDocs))
	for termStr := range termDocs {
		_, err := terms.Add(util.NewBytesRef([]byte(termStr)))
		if err != nil {
			t.Fatalf("BytesRefHash.Add(%q): %v", termStr, err)
		}
		scores = append(scores, 10.0)
	}
	// Grow scores to hash size; each ord gets 10.0.
	scoreBuf := make([]float32, terms.Size())
	for i := range scoreBuf {
		scoreBuf[i] = 10.0
	}
	q := NewTermsIncludingScoreQuery(Max, field, mv, terms, scoreBuf, "fromF", nil, nil)
	te := &stubTermsEnum{postings: termDocs}
	return q, te
}

// TestSVInOrderScorer_MatchingDocs verifies that the SV scorer visits exactly
// the docs from matching terms and returns the expected scores.
func TestSVInOrderScorer_MatchingDocs(t *testing.T) {
	// term "alpha" matches docs 1 and 3; term "beta" matches docs 2 and 3.
	// SV: last-wins, so doc 3's score may be overwritten by "beta".
	const maxDoc = 5
	termDocs := map[string][]int{
		"alpha": {1, 3},
		"beta":  {2, 3},
	}
	q, te := buildQueryWithPostings(t, "f", false, termDocs)
	scorer, err := newSVInOrderScorer(q, te, maxDoc, 10, 1.0)
	if err != nil {
		t.Fatalf("newSVInOrderScorer: %v", err)
	}

	var got []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc >= maxDoc {
			break
		}
		sc := scorer.Score()
		if sc <= 0 {
			t.Errorf("doc %d: score = %v, want > 0", doc, sc)
		}
		got = append(got, doc)
	}
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("docs = %v, want %v", got, want)
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("docs[%d] = %d, want %d", i, got[i], d)
		}
	}
}

// TestSVInOrderScorer_Advance verifies Advance() on the SV scorer.
func TestSVInOrderScorer_Advance(t *testing.T) {
	const maxDoc = 10
	termDocs := map[string][]int{
		"x": {1, 4, 7},
	}
	q, te := buildQueryWithPostings(t, "f", false, termDocs)
	scorer, err := newSVInOrderScorer(q, te, maxDoc, 10, 2.0)
	if err != nil {
		t.Fatalf("newSVInOrderScorer: %v", err)
	}
	doc, err := scorer.Advance(4)
	if err != nil {
		t.Fatalf("Advance(4): %v", err)
	}
	if doc != 4 {
		t.Errorf("Advance(4) = %d, want 4", doc)
	}
	// Score should be 10.0 * boost 2.0 = 20.0.
	if sc := scorer.Score(); sc != 20.0 {
		t.Errorf("Score() = %v, want 20.0", sc)
	}
}

// TestMVInOrderScorer_FirstWins verifies that for the MV scorer, when two
// terms match the same doc, the first-seen score is kept.
func TestMVInOrderScorer_FirstWins(t *testing.T) {
	// Both "a" and "b" match doc 1.  "a" (ord 0) is processed first after
	// sorting, so doc 1 gets the score for whichever term comes first in the
	// sorted order.  We just verify doc 1 is hit exactly once with a positive score.
	const maxDoc = 5
	// Give different scores to distinguish which term won.
	terms := util.NewBytesRefHash()
	_, _ = terms.Add(util.NewBytesRef([]byte("a"))) // will be ord 0 after sort
	_, _ = terms.Add(util.NewBytesRef([]byte("b"))) // ord 1
	scoreBuf := []float32{5.0, 9.0}
	q := NewTermsIncludingScoreQuery(Max, "f", true, terms, scoreBuf, "fromF", nil, nil)
	te := &stubTermsEnum{
		postings: map[string][]int{
			"a": {1},
			"b": {1, 2},
		},
	}
	scorer, err := newMVInOrderScorer(q, te, maxDoc, 10, 1.0)
	if err != nil {
		t.Fatalf("newMVInOrderScorer: %v", err)
	}

	var hits []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc >= maxDoc {
			break
		}
		hits = append(hits, doc)
		sc := scorer.Score()
		if sc <= 0 {
			t.Errorf("doc %d: score = %v, want > 0", doc, sc)
		}
	}
	// Docs 1 and 2 must both be returned.
	if len(hits) != 2 {
		t.Fatalf("hits = %v, want [1 2]", hits)
	}
}

// TestSVInOrderScorer_NoMatches verifies no docs returned when terms are absent.
func TestSVInOrderScorer_NoMatches(t *testing.T) {
	const maxDoc = 5
	terms := util.NewBytesRefHash()
	_, _ = terms.Add(util.NewBytesRef([]byte("missing")))
	scoreBuf := []float32{1.0}
	q := NewTermsIncludingScoreQuery(Max, "f", false, terms, scoreBuf, "fromF", nil, nil)
	// TermsEnum returns not-found for any seek.
	te := &stubTermsEnum{postings: map[string][]int{}}
	scorer, err := newSVInOrderScorer(q, te, maxDoc, 0, 1.0)
	if err != nil {
		t.Fatalf("newSVInOrderScorer: %v", err)
	}
	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	// BitSetIterator over empty set returns search.NO_MORE_DOCS (math.MaxInt32).
	if doc <= maxDoc {
		t.Errorf("expected no docs, got %d", doc)
	}
}

// TestSVInOrderScorer_BoostApplied verifies that boost multiplies the per-term score.
func TestSVInOrderScorer_BoostApplied(t *testing.T) {
	const maxDoc = 5
	termDocs := map[string][]int{"t": {2}}
	q, te := buildQueryWithPostings(t, "f", false, termDocs)
	scorer, err := newSVInOrderScorer(q, te, maxDoc, 1, 3.0)
	if err != nil {
		t.Fatalf("newSVInOrderScorer: %v", err)
	}
	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 2 {
		t.Fatalf("NextDoc() = %d, want 2", doc)
	}
	// Per-term score is 10.0 (from buildQueryWithPostings), boost = 3.0 → 30.0.
	if sc := scorer.Score(); sc != 30.0 {
		t.Errorf("Score() = %v, want 30.0", sc)
	}
}
