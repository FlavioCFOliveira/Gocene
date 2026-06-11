// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpansEnum.java
//
// Tests Spans enumeration patterns: NextDoc, Advance, NextStartPosition,
// and combinations thereof.  Uses in-memory TermSpans built from
// memPostingsEnum rather than a full index.

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestSpansEnum_NextDoc(t *testing.T) {
	t.Parallel()

	t.Run("single_doc", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{3: {0}})
		doc, err := sp.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != 3 {
			t.Errorf("doc = %d; want 3", doc)
		}
	})

	t.Run("multiple_docs_in_order", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{1: {0}, 5: {0}, 9: {0}})
		docs := []int{}
		for {
			doc, err := sp.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc == search.NO_MORE_DOCS {
				break
			}
			docs = append(docs, doc)
		}
		if len(docs) != 3 || docs[0] != 1 || docs[1] != 5 || docs[2] != 9 {
			t.Errorf("docs = %v; want [1 5 9]", docs)
		}
	})

	t.Run("no_docs", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{})
		doc, err := sp.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != search.NO_MORE_DOCS {
			t.Errorf("doc = %d; want NO_MORE_DOCS", doc)
		}
	})

	t.Run("doc_id_run_end", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}})
		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		end := sp.DocIDRunEnd()
		if end != 1 {
			t.Errorf("DocIDRunEnd = %d; want 1", end)
		}
	})

	t.Run("cost_non_zero", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}, 1: {0}})
		cost := sp.Cost()
		if cost <= 0 {
			t.Errorf("Cost = %d; want > 0", cost)
		}
	})
}

func TestSpansEnum_Advance(t *testing.T) {
	t.Parallel()

	t.Run("advance_to_existing", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}, 2: {0}, 4: {0}})
		doc, err := sp.Advance(2)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc != 2 {
			t.Errorf("doc = %d; want 2", doc)
		}
	})

	t.Run("advance_to_nonexistent", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}, 5: {0}})
		doc, err := sp.Advance(3)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc != 5 {
			t.Errorf("doc = %d; want 5 (next after 3)", doc)
		}
	})

	t.Run("advance_past_last", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}})
		doc, err := sp.Advance(100)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc != search.NO_MORE_DOCS {
			t.Errorf("doc = %d; want NO_MORE_DOCS", doc)
		}
	})

	t.Run("advance_before_first", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{3: {0}})
		doc, err := sp.Advance(0)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc != 3 {
			t.Errorf("doc = %d; want 3", doc)
		}
	})

	t.Run("next_then_advance", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0}, 3: {0}, 7: {0}})
		// Next the first doc, then Advance to a later one.
		doc1, err := sp.NextDoc()
		if err != nil || doc1 != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		doc2, err := sp.Advance(5)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc2 != 7 {
			t.Errorf("doc = %d; want 7", doc2)
		}
	})
}

func TestSpansEnum_Positions(t *testing.T) {
	t.Parallel()

	t.Run("multiple_positions_per_doc", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {2, 5, 8}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("t"))}
		sp := NewTermSpans(pe, term, 128.0)

		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}

		var positionsFound []int
		for {
			pos, err := sp.NextStartPosition()
			if err != nil {
				t.Fatalf("NextStartPosition: %v", err)
			}
			if pos == NoMorePositions {
				break
			}
			positionsFound = append(positionsFound, pos)
		}

		if len(positionsFound) != 3 || positionsFound[0] != 2 || positionsFound[1] != 5 || positionsFound[2] != 8 {
			t.Errorf("positions = %v; want [2 5 8]", positionsFound)
		}
	})

	t.Run("next_start_position_after_advance", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{2: {1, 4}}
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("t"))}
		pe := newMemPostingsEnum(positions)
		sp := NewTermSpans(pe, term, 128.0)

		doc, err := sp.Advance(2)
		if err != nil || doc != 2 {
			t.Fatalf("Advance: %v", err)
		}

		pos, err := sp.NextStartPosition()
		if err != nil {
			t.Fatalf("NextStartPosition: %v", err)
		}
		if pos != 1 {
			t.Errorf("first pos = %d; want 1", pos)
		}
	})

	t.Run("start_position_before_next_start", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {4}})
		start := sp.StartPosition()
		if start != -1 {
			t.Errorf("StartPosition before NextDoc = %d; want -1", start)
		}
		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		// StartPosition should still be -1 until NextStartPosition is called.
		if sp.StartPosition() != -1 {
			t.Errorf("StartPosition after NextDoc = %d; want -1", sp.StartPosition())
		}
		sp.NextStartPosition()
		if sp.StartPosition() != 4 {
			t.Errorf("StartPosition = %d; want 4", sp.StartPosition())
		}
	})

	t.Run("end_position_is_plus_one", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {7}})
		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
		}
		pos, err := sp.NextStartPosition()
		if err != nil || pos == NoMorePositions {
			t.Fatalf("NextStartPosition: %v", err)
		}
		end := sp.EndPosition()
		if end != 8 {
			t.Errorf("EndPosition = %d; want 8 (7+1)", end)
		}
	})

	t.Run("width_is_zero", func(t *testing.T) {
		t.Parallel()
		sp := buildTermSpans("f", "t", map[int][]int{0: {0, 1}})
		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc: %v", err)
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
				t.Errorf("Width at pos %d = %d; want 0", pos, sp.Width())
			}
		}
	})
}
