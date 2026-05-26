// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanMatches.java

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// In-memory PostingsEnum backed by a map[docID][]position
// ---------------------------------------------------------------------------

// memPostingsEnum is a test-only PostingsEnum that serves canned per-doc
// positions. It satisfies index.PostingsEnum and is used to build TermSpans
// without a real on-disk index.
type memPostingsEnum struct {
	// sorted docIDs
	docIDs []int
	// positions[docID] = list of positions in that doc
	positions map[int][]int

	// iterator state
	docPos int // index into docIDs; -1 = before first
	posPos int // index within current doc's position list; -1 = before first

	curDoc int
}

func newMemPostingsEnum(positions map[int][]int) *memPostingsEnum {
	docs := make([]int, 0, len(positions))
	for d := range positions {
		docs = append(docs, d)
	}
	// insertion-sort for determinism (small lists)
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j] < docs[j-1]; j-- {
			docs[j], docs[j-1] = docs[j-1], docs[j]
		}
	}
	return &memPostingsEnum{
		docIDs:    docs,
		positions: positions,
		docPos:    -1,
		posPos:    -1,
		curDoc:    index.NO_MORE_DOCS, // start before first doc
	}
}

func (p *memPostingsEnum) DocID() int { return p.curDoc }

func (p *memPostingsEnum) NextDoc() (int, error) {
	p.docPos++
	p.posPos = -1
	if p.docPos >= len(p.docIDs) {
		p.curDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	p.curDoc = p.docIDs[p.docPos]
	return p.curDoc, nil
}

func (p *memPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target || doc == index.NO_MORE_DOCS {
			return doc, nil
		}
	}
}

func (p *memPostingsEnum) Freq() (int, error) {
	if p.curDoc == index.NO_MORE_DOCS {
		return 0, nil
	}
	return len(p.positions[p.curDoc]), nil
}

func (p *memPostingsEnum) NextPosition() (int, error) {
	if p.curDoc == index.NO_MORE_DOCS {
		return -1, nil
	}
	pos := p.positions[p.curDoc]
	p.posPos++
	if p.posPos >= len(pos) {
		return -1, nil
	}
	return pos[p.posPos], nil
}

func (p *memPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *memPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *memPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

func (p *memPostingsEnum) Cost() int64 { return int64(len(p.docIDs)) }

// DocIDRunEnd satisfies search.DocIdSetIterator — memPostingsEnum is only used
// inside TermSpans which wraps it; not called directly in these tests.
func (p *memPostingsEnum) DocIDRunEnd() int { return p.curDoc + 1 }

var _ index.PostingsEnum = (*memPostingsEnum)(nil)

// ---------------------------------------------------------------------------
// Helper: drain all (docID, [start, end)) spans into a map
// ---------------------------------------------------------------------------

// drainSpans advances sp through all documents, collecting every (start, end)
// pair per doc into the returned map. It terminates at search.NO_MORE_DOCS.
func drainSpans(sp Spans) (map[int][][2]int, error) {
	result := make(map[int][][2]int)
	for {
		doc, err := sp.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		for {
			start, err := sp.NextStartPosition()
			if err != nil {
				return nil, err
			}
			if start == NoMorePositions {
				break
			}
			result[doc] = append(result[doc], [2]int{start, sp.EndPosition()})
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// newTerm builds an *index.Term by value (avoids allocating BytesRef manually
// in each test).
// ---------------------------------------------------------------------------

func newTerm(field, text string) *index.Term {
	return &index.Term{
		Field: field,
		Bytes: util.NewBytesRef([]byte(text)),
	}
}

// ---------------------------------------------------------------------------
// buildTermSpans constructs a TermSpans from canned positions. positionsCost
// is set to a nominal positive value (128.0) matching Lucene's default.
// ---------------------------------------------------------------------------

func buildTermSpans(field, text string, positions map[int][]int) *TermSpans {
	pe := newMemPostingsEnum(positions)
	term := index.Term{Field: field, Bytes: util.NewBytesRef([]byte(text))}
	return NewTermSpans(pe, term, 128.0)
}

// ---------------------------------------------------------------------------
// TestSpanMatches_All
//
// Tests that SpanTermQuery and SpanNearQuery correctly identify matching
// spans across several documents using in-memory PostingsEnum mocks.
//
// Deviation from Java: Java TestSpanMatches builds a real RAMDirectory index
// and uses Lucene field types with position indexing. Gocene uses
// memPostingsEnum to simulate the postings layer so that TermSpans /
// NearSpansOrdered / NearSpansUnordered can be exercised without a full
// codec stack (which requires SegmentReader core-readers wiring not yet
// complete in Gocene — see backlog #2709 and SegmentReader core-readers gap).
// ---------------------------------------------------------------------------

func TestSpanMatches_All(t *testing.T) {
	t.Parallel()

	// Corpus layout (field "body"):
	//   doc 0: "w1 w2 w3" → w1@0, w2@1, w3@2
	//   doc 1: "w1"       → w1@0
	//   doc 2: "w2 w3 w1" → w2@0, w3@1, w1@2
	//   doc 3: "w3 w2"    → w3@0, w2@1
	//   doc 4: "w1 w3"    → w1@0, w3@1

	posW1 := map[int][]int{0: {0}, 1: {0}, 2: {2}, 4: {0}}
	posW2 := map[int][]int{0: {1}, 2: {0}, 3: {1}}
	posW3 := map[int][]int{0: {2}, 2: {1}, 3: {0}, 4: {1}}

	tests := []struct {
		name    string
		build   func() Spans
		wantDoc []int            // expected docIDs in order
		wantPos map[int][][2]int // expected (start, end) pairs per doc
	}{
		{
			// SpanTermQuery("w1") → docs 0,1,2,4; positions as above.
			name:    "SpanTermQuery_w1",
			build:   func() Spans { return buildTermSpans("body", "w1", posW1) },
			wantDoc: []int{0, 1, 2, 4},
			wantPos: map[int][][2]int{
				0: {{0, 1}},
				1: {{0, 1}},
				2: {{2, 3}},
				4: {{0, 1}},
			},
		},
		{
			// SpanNearQuery([w1,w2], slop=0, inOrder=true) → doc 0 only (w1@0 w2@1).
			name: "SpanNearQuery_ordered_w1_w2_slop0",
			build: func() Spans {
				s1 := buildTermSpans("body", "w1", posW1)
				s2 := buildTermSpans("body", "w2", posW2)
				ns, err := NewNearSpansOrdered(0, []Spans{s1, s2})
				if err != nil {
					t.Fatalf("NewNearSpansOrdered: %v", err)
				}
				return ns
			},
			wantDoc: []int{0},
			wantPos: map[int][][2]int{
				// match: start=0 (w1 start), end=2 (w2 end = pos+1)
				0: {{0, 2}},
			},
		},
		{
			// SpanNearQuery([w1,w2], slop=1, inOrder=true).
			// doc 0: w1@0 w2@1 → width=0, fits slop=1 → match start=0, end=2
			// doc 2: w2@0 w1@2 → ordered requires w1 before w2, no match (w1@2 > w2@0+1)
			//   Actually in doc 2: w1@2 starts after w2@0 ends (1), so ordered(w1,w2) fails.
			//   NearSpansOrdered must have w1 before w2, doc 2 has w1 only at pos 2 and w2 at pos 0.
			//   Since w1 comes first in the clause list, we need w1 < w2; doc2 w1@2 > w2@0, no match.
			name: "SpanNearQuery_ordered_w1_w2_slop1",
			build: func() Spans {
				s1 := buildTermSpans("body", "w1", posW1)
				s2 := buildTermSpans("body", "w2", posW2)
				ns, err := NewNearSpansOrdered(1, []Spans{s1, s2})
				if err != nil {
					t.Fatalf("NewNearSpansOrdered: %v", err)
				}
				return ns
			},
			wantDoc: []int{0},
			wantPos: map[int][][2]int{
				0: {{0, 2}},
			},
		},
		{
			// SpanNearQuery([w1,w3], slop=1, inOrder=false) → any order within window.
			// doc 0: w1@0, w3@2 → window = [0,3), totalLen=2, gap=1 → atMatch(1): (3-0-2)=1<=1 ✓
			// doc 2: w1@2, w3@1 → window sorted: w3@1 start, w1@2 start. maxEnd=3, minStart=1, totalLen=2 → (3-1-2)=0<=1 ✓
			// doc 4: w1@0, w3@1 → window: maxEnd=2, minStart=0, totalLen=2 → (2-0-2)=0<=1 ✓
			name: "SpanNearQuery_unordered_w1_w3_slop1",
			build: func() Spans {
				s1 := buildTermSpans("body", "w1", posW1)
				s3 := buildTermSpans("body", "w3", posW3)
				ns, err := NewNearSpansUnordered(1, []Spans{s1, s3})
				if err != nil {
					t.Fatalf("NewNearSpansUnordered: %v", err)
				}
				return ns
			},
			wantDoc: []int{0, 2, 4},
		},
		{
			// SpanTermQuery for a term absent from the index → no matches.
			name:    "SpanTermQuery_absent",
			build:   func() Spans { return buildTermSpans("body", "w9", map[int][]int{}) },
			wantDoc: nil,
			wantPos: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sp := tc.build()
			got, err := drainSpans(sp)
			if err != nil {
				t.Fatalf("drainSpans: %v", err)
			}

			// Verify matched docIDs.
			var gotDocs []int
			for d := range got {
				gotDocs = append(gotDocs, d)
			}
			// insertion-sort gotDocs
			for i := 1; i < len(gotDocs); i++ {
				for j := i; j > 0 && gotDocs[j] < gotDocs[j-1]; j-- {
					gotDocs[j], gotDocs[j-1] = gotDocs[j-1], gotDocs[j]
				}
			}

			if len(gotDocs) != len(tc.wantDoc) {
				t.Fatalf("matched docs = %v; want %v", gotDocs, tc.wantDoc)
			}
			for i, d := range tc.wantDoc {
				if gotDocs[i] != d {
					t.Errorf("matched[%d] = %d; want %d", i, gotDocs[i], d)
				}
			}

			// Verify per-doc span positions (if provided).
			if tc.wantPos == nil {
				return
			}
			for doc, wantSpans := range tc.wantPos {
				gotSpans, ok := got[doc]
				if !ok {
					t.Errorf("doc %d: no spans; want %v", doc, wantSpans)
					continue
				}
				if len(gotSpans) != len(wantSpans) {
					t.Errorf("doc %d: spans = %v; want %v", doc, gotSpans, wantSpans)
					continue
				}
				for i, ws := range wantSpans {
					if gotSpans[i] != ws {
						t.Errorf("doc %d span[%d] = %v; want %v", doc, i, gotSpans[i], ws)
					}
				}
			}
		})
	}
}

// TestSpanMatches_TermSpans_Width verifies that TermSpans always reports
// Width() == 0 (no gap contribution for single-term spans).
func TestSpanMatches_TermSpans_Width(t *testing.T) {
	t.Parallel()
	sp := buildTermSpans("body", "w1", map[int][]int{0: {3, 7}})
	doc, err := sp.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc: got doc=%d err=%v", doc, err)
	}
	for {
		pos, err := sp.NextStartPosition()
		if err != nil {
			t.Fatalf("NextStartPosition: %v", err)
		}
		if pos == NoMorePositions {
			break
		}
		if sp.Width() != 0 {
			t.Errorf("TermSpans.Width() = %d; want 0 at position %d", sp.Width(), pos)
		}
	}
}

// TestSpanMatches_GapSpans_Basic verifies GapSpans position arithmetic.
func TestSpanMatches_GapSpans_Basic(t *testing.T) {
	t.Parallel()
	gs := NewGapSpans(3)
	// before NextDoc: doc=-1
	if gs.DocID() != -1 {
		t.Errorf("initial DocID = %d; want -1", gs.DocID())
	}
	doc, err := gs.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("NextDoc = %d; want 0", doc)
	}
	// After NextDoc, pos=-1; start=-1; end = -1+3 = 2.
	if gs.StartPosition() != -1 {
		t.Errorf("StartPosition = %d; want -1", gs.StartPosition())
	}
	if gs.EndPosition() != 2 {
		t.Errorf("EndPosition = %d; want 2 (-1+3)", gs.EndPosition())
	}
	// SkipToPosition(5) → pos=5; end=8.
	newPos, err := gs.SkipToPosition(5)
	if err != nil {
		t.Fatalf("SkipToPosition: %v", err)
	}
	if newPos != 5 {
		t.Errorf("SkipToPosition(5) = %d; want 5", newPos)
	}
	if gs.EndPosition() != 8 {
		t.Errorf("EndPosition = %d; want 8 (5+3)", gs.EndPosition())
	}
	if gs.Width() != 3 {
		t.Errorf("Width = %d; want 3", gs.Width())
	}
}

// TestSpanMatches_SpanWeight_IsCacheable verifies the SpanWeight default.
func TestSpanMatches_SpanWeight_IsCacheable(t *testing.T) {
	t.Parallel()
	term := newTerm("body", "w1")
	q := NewSpanTermQuery(term)
	w, err := q.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if !w.IsCacheable(nil) {
		t.Fatal("IsCacheable must return true by default")
	}
}

// TestSpanMatches_SpanTermQuery_NilContext verifies that GetSpans with a nil
// context returns nil, nil (Lucene fast-path for no matching leaf).
func TestSpanMatches_SpanTermQuery_NilContext(t *testing.T) {
	t.Parallel()
	term := newTerm("body", "w1")
	q := NewSpanTermQuery(term)
	sw, err := q.CreateSpanWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateSpanWeight: %v", err)
	}
	sp, err := sw.GetSpans(nil, PostingsPositions)
	if err != nil {
		t.Fatalf("GetSpans(nil): %v", err)
	}
	if sp != nil {
		t.Fatalf("GetSpans(nil) = %v; want nil", sp)
	}
}
