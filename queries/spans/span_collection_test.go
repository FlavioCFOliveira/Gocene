// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/spans/TestSpanCollection.java
//
// Tests SpanCollector integration with TermSpans.  In-memory PostingsEnum
// mocks are used to simulate payload and position data without a real
// index.

package spans

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// memCollectingCollector — a SpanCollector that records every CollectLeaf call
// ---------------------------------------------------------------------------

type memCollectingCollector struct {
	calls []collectCall
}

type collectCall struct {
	position int
	term     index.Term
}

func (c *memCollectingCollector) CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error {
	c.calls = append(c.calls, collectCall{position: position, term: term})
	return nil
}

func (c *memCollectingCollector) Reset() {
	c.calls = c.calls[:0]
}

// ---------------------------------------------------------------------------
// memPostingsWithPayload — PostingsEnum that carries payloads
// ---------------------------------------------------------------------------

type memPostingsWithPayload struct {
	*memPostingsEnum
	payloads map[int]map[int][]byte // docID → pos → payload
}

func newMemPostingsWithPayload(positions map[int][]int, payloads map[int]map[int][]byte) *memPostingsWithPayload {
	return &memPostingsWithPayload{
		memPostingsEnum: newMemPostingsEnum(positions),
		payloads:        payloads,
	}
}

func (p *memPostingsWithPayload) GetPayload() ([]byte, error) {
	if p.curDoc == index.NO_MORE_DOCS {
		return nil, nil
	}
	docPayloads, ok := p.payloads[p.curDoc]
	if !ok {
		return nil, nil
	}
	currentPos := p.posPos
	if currentPos < 0 || currentPos >= len(p.positions[p.curDoc]) {
		return nil, nil
	}
	pos := p.positions[p.curDoc][currentPos]
	payload, ok := docPayloads[pos]
	if !ok {
		return nil, nil
	}
	return payload, nil
}

var _ index.PostingsEnum = (*memPostingsWithPayload)(nil)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSpanCollection_Collect(t *testing.T) {
	t.Parallel()

	t.Run("TermSpans_collect_single_doc_single_position", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {3}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("hello"))}
		sp := NewTermSpans(pe, term, 128.0)

		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc = %d, err=%v", doc, err)
		}
		pos, err := sp.NextStartPosition()
		if err != nil || pos == NoMorePositions {
			t.Fatalf("NextStartPosition = %d, err=%v", pos, err)
		}

		collector := &memCollectingCollector{}
		if err := sp.Collect(collector); err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 collect call; got %d", len(collector.calls))
		}
		if collector.calls[0].position != 3 {
			t.Errorf("position = %d; want 3", collector.calls[0].position)
		}
		if string(collector.calls[0].term.Bytes.Bytes) != "hello" {
			t.Errorf("term = %s; want hello", collector.calls[0].term.Bytes.String())
		}
	})

	t.Run("TermSpans_collect_multiple_docs", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {0, 2}, 1: {5}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("multi"))}
		sp := NewTermSpans(pe, term, 128.0)

		collector := &memCollectingCollector{}
		totalCalls := 0

		for {
			doc, err := sp.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc == search.NO_MORE_DOCS {
				break
			}
			for {
				pos, err := sp.NextStartPosition()
				if err != nil {
					t.Fatalf("NextStartPosition: %v", err)
				}
				if pos == NoMorePositions {
					break
				}
				if err := sp.Collect(collector); err != nil {
					t.Fatalf("Collect: %v", err)
				}
				totalCalls++
			}
		}

		// doc 0 has 2 positions, doc 1 has 1 position → 3 total.
		if totalCalls != 3 {
			t.Errorf("expected 3 collect calls; got %d", totalCalls)
		}
	})

	t.Run("collector_reset_clears", func(t *testing.T) {
		t.Parallel()
		collector := &memCollectingCollector{}
		collector.calls = append(collector.calls, collectCall{position: 0, term: index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("x"))}})
		if len(collector.calls) != 1 {
			t.Fatal("expected 1 call before reset")
		}
		collector.Reset()
		if len(collector.calls) != 0 {
			t.Error("expected 0 calls after reset")
		}
	})

	t.Run("TermSpans_collect_after_advance", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {1}, 2: {3}}
		pe := newMemPostingsEnum(positions)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("adv"))}
		sp := NewTermSpans(pe, term, 128.0)

		doc, err := sp.Advance(2)
		if err != nil || doc != 2 {
			t.Fatalf("Advance(2) = %d, err=%v", doc, err)
		}
		pos, err := sp.NextStartPosition()
		if err != nil || pos == NoMorePositions {
			t.Fatalf("NextStartPosition = %d, err=%v", pos, err)
		}

		collector := &memCollectingCollector{}
		if err := sp.Collect(collector); err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 call; got %d", len(collector.calls))
		}
		if collector.calls[0].position != 3 {
			t.Errorf("position = %d; want 3", collector.calls[0].position)
		}
	})

	t.Run("collect_with_payloads", func(t *testing.T) {
		t.Parallel()
		positions := map[int][]int{0: {0}}
		payloads := map[int]map[int][]byte{
			0: {0: []byte("p1")},
		}
		pe := newMemPostingsWithPayload(positions, payloads)
		term := index.Term{Field: "f", Bytes: util.NewBytesRef([]byte("pay"))}
		sp := NewTermSpans(pe, term, 128.0)

		doc, err := sp.NextDoc()
		if err != nil || doc != 0 {
			t.Fatalf("NextDoc = %d, err=%v", doc, err)
		}
		pos, err := sp.NextStartPosition()
		if err != nil || pos == NoMorePositions {
			t.Fatalf("NextStartPosition = %d, err=%v", pos, err)
		}

		// Collect via the standard path — the collector receives the PostingsEnum
		// from which payloads can be read.
		collector := &memCollectingCollector{}
		if err := sp.Collect(collector); err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if len(collector.calls) != 1 {
			t.Fatalf("expected 1 call; got %d", len(collector.calls))
		}
	})
}
