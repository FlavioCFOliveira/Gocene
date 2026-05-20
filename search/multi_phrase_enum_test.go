// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiPhraseEnum.java
//
// Deviation: the Java test uses a real IndexWriter and DirectoryReader to
// obtain PostingsEnum instances. The Go port uses in-memory stub
// PostingsEnum implementations, exercising the same UnionPostingsEnum contract.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ─── stub PostingsEnum ────────────────────────────────────────────────────────

// postingEntry is a (docID, []position) pair for one document.
type postingEntry struct {
	docID int
	pos   []int
}

// stubPostingsEnum implements index.PostingsEnum over a fixed list of entries.
type stubPostingsEnum struct {
	entries []postingEntry
	cur     int  // index into entries (-1 = before start)
	posIdx  int  // position cursor within current entry
	cost    int64
}

func newStubPostingsEnum(entries []postingEntry) *stubPostingsEnum {
	return &stubPostingsEnum{entries: entries, cur: -1, cost: int64(len(entries))}
}

func (s *stubPostingsEnum) DocID() int {
	if s.cur < 0 {
		return -1 // pre-iteration sentinel (mirrors Java PostingsEnum initial state)
	}
	if s.cur >= len(s.entries) {
		return index.NO_MORE_DOCS
	}
	return s.entries[s.cur].docID
}

func (s *stubPostingsEnum) NextDoc() (int, error) {
	s.cur++
	s.posIdx = 0
	if s.cur >= len(s.entries) {
		return index.NO_MORE_DOCS, nil
	}
	return s.entries[s.cur].docID, nil
}

func (s *stubPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil || doc >= target || doc == index.NO_MORE_DOCS {
			return doc, err
		}
	}
}

func (s *stubPostingsEnum) Freq() (int, error) {
	if s.cur < 0 || s.cur >= len(s.entries) {
		return 0, nil
	}
	return len(s.entries[s.cur].pos), nil
}

func (s *stubPostingsEnum) NextPosition() (int, error) {
	if s.cur < 0 || s.cur >= len(s.entries) {
		return -1, nil
	}
	entry := s.entries[s.cur]
	if s.posIdx >= len(entry.pos) {
		return -1, nil
	}
	p := entry.pos[s.posIdx]
	s.posIdx++
	return p, nil
}

func (s *stubPostingsEnum) StartOffset() (int, error) { return -1, nil }
func (s *stubPostingsEnum) EndOffset() (int, error)   { return -1, nil }
func (s *stubPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (s *stubPostingsEnum) Cost() int64               { return s.cost }

// Compile-time assertion.
var _ index.PostingsEnum = (*stubPostingsEnum)(nil)

// ─── tests ────────────────────────────────────────────────────────────────────

// TestMultiPhraseEnum_OneDocument mirrors TestMultiPhraseEnum.testOneDocument
// (Lucene 10.4.0).
//
// Models document 0 with text "foo bar": p1 for "foo" at position 0,
// p2 for "bar" at position 1.
func TestMultiPhraseEnum_OneDocument(t *testing.T) {
	p1 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{0}}})
	p2 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{1}}})

	union := NewUnionPostingsEnum([]index.PostingsEnum{p1, p2})

	// Before first NextDoc, initial state is the top sub's initial docID (-1).
	if got := union.DocID(); got != -1 {
		t.Fatalf("initial DocID: expected -1, got %d", got)
	}

	doc, err := union.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("expected doc 0, got %d", doc)
	}

	freq, err := union.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	if freq != 2 {
		t.Fatalf("expected freq 2, got %d", freq)
	}

	pos0, err := union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[0]: %v", err)
	}
	if pos0 != 0 {
		t.Fatalf("expected position 0, got %d", pos0)
	}

	pos1, err := union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[1]: %v", err)
	}
	if pos1 != 1 {
		t.Fatalf("expected position 1, got %d", pos1)
	}

	final, err := union.NextDoc()
	if err != nil {
		t.Fatalf("final NextDoc: %v", err)
	}
	if final != index.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", final)
	}
}

// TestMultiPhraseEnum_SomeDocuments mirrors TestMultiPhraseEnum.testSomeDocuments
// (Lucene 10.4.0).
//
// Models four documents after forceMerge:
//   doc 0: "foo"     → p1 posts doc 0 pos 0; p2 absent
//   doc 1: ""        → both absent
//   doc 2: "foo bar" → p1 posts doc 2 pos 0; p2 posts doc 2 pos 1
//   doc 3: "bar"     → p2 posts doc 3 pos 0; p1 absent
func TestMultiPhraseEnum_SomeDocuments(t *testing.T) {
	p1 := newStubPostingsEnum([]postingEntry{
		{docID: 0, pos: []int{0}},
		{docID: 2, pos: []int{0}},
	})
	p2 := newStubPostingsEnum([]postingEntry{
		{docID: 2, pos: []int{1}},
		{docID: 3, pos: []int{0}},
	})

	union := NewUnionPostingsEnum([]index.PostingsEnum{p1, p2})

	// initial state
	if got := union.DocID(); got != -1 {
		t.Fatalf("initial DocID: expected -1, got %d", got)
	}

	// doc 0: freq=1, pos=0
	doc, err := union.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc[0]: %v", err)
	}
	if doc != 0 {
		t.Fatalf("expected doc 0, got %d", doc)
	}
	freq, err := union.Freq()
	if err != nil {
		t.Fatalf("Freq[0]: %v", err)
	}
	if freq != 1 {
		t.Fatalf("doc 0: expected freq 1, got %d", freq)
	}
	pos, err := union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[0,0]: %v", err)
	}
	if pos != 0 {
		t.Fatalf("doc 0: expected pos 0, got %d", pos)
	}

	// doc 2: freq=2, pos=0 then 1
	doc, err = union.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc[2]: %v", err)
	}
	if doc != 2 {
		t.Fatalf("expected doc 2, got %d", doc)
	}
	freq, err = union.Freq()
	if err != nil {
		t.Fatalf("Freq[2]: %v", err)
	}
	if freq != 2 {
		t.Fatalf("doc 2: expected freq 2, got %d", freq)
	}
	pos, err = union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[2,0]: %v", err)
	}
	if pos != 0 {
		t.Fatalf("doc 2: expected pos 0, got %d", pos)
	}
	pos, err = union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[2,1]: %v", err)
	}
	if pos != 1 {
		t.Fatalf("doc 2: expected pos 1, got %d", pos)
	}

	// doc 3: freq=1, pos=0
	doc, err = union.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc[3]: %v", err)
	}
	if doc != 3 {
		t.Fatalf("expected doc 3, got %d", doc)
	}
	freq, err = union.Freq()
	if err != nil {
		t.Fatalf("Freq[3]: %v", err)
	}
	if freq != 1 {
		t.Fatalf("doc 3: expected freq 1, got %d", freq)
	}
	pos, err = union.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition[3,0]: %v", err)
	}
	if pos != 0 {
		t.Fatalf("doc 3: expected pos 0, got %d", pos)
	}

	// exhausted
	final, err := union.NextDoc()
	if err != nil {
		t.Fatalf("final NextDoc: %v", err)
	}
	if final != index.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", final)
	}
}

// TestMultiPhraseEnum_Cost verifies that Cost() sums sub costs.
func TestMultiPhraseEnum_Cost(t *testing.T) {
	p1 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{0}}})
	p2 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{1}}, {docID: 1, pos: []int{0}}})

	union := NewUnionPostingsEnum([]index.PostingsEnum{p1, p2})
	if got := union.Cost(); got != p1.Cost()+p2.Cost() {
		t.Fatalf("expected cost %d, got %d", p1.Cost()+p2.Cost(), got)
	}
}

// TestMultiPhraseEnum_PositionsSorted verifies that positions are returned in
// ascending order even when the subs contribute them in mixed order.
func TestMultiPhraseEnum_PositionsSorted(t *testing.T) {
	// p1 posts doc 0 at position 2; p2 posts doc 0 at position 0.
	// After union, positions should come out sorted: 0, 2.
	p1 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{2}}})
	p2 := newStubPostingsEnum([]postingEntry{{docID: 0, pos: []int{0}}})

	union := NewUnionPostingsEnum([]index.PostingsEnum{p1, p2})
	if _, err := union.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if _, err := union.Freq(); err != nil {
		t.Fatalf("Freq: %v", err)
	}
	pos0, _ := union.NextPosition()
	pos1, _ := union.NextPosition()
	if pos0 != 0 || pos1 != 2 {
		t.Fatalf("expected positions [0, 2], got [%d, %d]", pos0, pos1)
	}
}
