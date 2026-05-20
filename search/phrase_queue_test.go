// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPhraseQueue.java
//   No Java test peer exists — synthetic Go tests covering the contract.

package search

import "testing"

func pp(position, offset, ord int) *PhrasePositions {
	return &PhrasePositions{Position: position, Offset: offset, Ord: ord}
}

// TestPhraseQueue_OrderByPosition verifies that the smallest position is top.
func TestPhraseQueue_OrderByPosition(t *testing.T) {
	pq := NewPhraseQueue(4)
	pq.Add(pp(10, 0, 0))
	pq.Add(pp(2, 0, 0))
	pq.Add(pp(7, 0, 0))
	if got := pq.Top().Position; got != 2 {
		t.Fatalf("expected position 2, got %d", got)
	}
}

// TestPhraseQueue_TieBreakByOffset verifies offset tiebreak.
func TestPhraseQueue_TieBreakByOffset(t *testing.T) {
	pq := NewPhraseQueue(3)
	pq.Add(pp(5, 2, 0))
	pq.Add(pp(5, 1, 0))
	pq.Add(pp(5, 3, 0))
	if got := pq.Top().Offset; got != 1 {
		t.Fatalf("expected offset 1, got %d", got)
	}
}

// TestPhraseQueue_TieBreakByOrd verifies ord tiebreak.
func TestPhraseQueue_TieBreakByOrd(t *testing.T) {
	pq := NewPhraseQueue(3)
	pq.Add(pp(5, 1, 3))
	pq.Add(pp(5, 1, 1))
	pq.Add(pp(5, 1, 2))
	if got := pq.Top().Ord; got != 1 {
		t.Fatalf("expected ord 1, got %d", got)
	}
}

// TestPhraseQueue_Pop verifies extraction order.
func TestPhraseQueue_Pop(t *testing.T) {
	pq := NewPhraseQueue(3)
	pq.Add(pp(3, 0, 0))
	pq.Add(pp(1, 0, 0))
	pq.Add(pp(2, 0, 0))
	got := pq.Pop()
	if got.Position != 1 {
		t.Fatalf("first pop: expected 1, got %d", got.Position)
	}
	got = pq.Pop()
	if got.Position != 2 {
		t.Fatalf("second pop: expected 2, got %d", got.Position)
	}
}

// TestPhraseQueue_UpdateTop verifies UpdateTop after mutation.
func TestPhraseQueue_UpdateTop(t *testing.T) {
	pq := NewPhraseQueue(3)
	a := pp(1, 0, 0)
	b := pp(3, 0, 0)
	pq.Add(a)
	pq.Add(b)
	pq.Top().Position = 10
	pq.UpdateTop()
	if pq.Top().Position != 3 {
		t.Fatalf("expected top.position=3 after update, got %d", pq.Top().Position)
	}
}

// TestPhraseQueue_Empty verifies empty queue behaviour.
func TestPhraseQueue_Empty(t *testing.T) {
	pq := NewPhraseQueue(4)
	if pq.Size() != 0 {
		t.Fatalf("expected size 0, got %d", pq.Size())
	}
	if pq.Top() != nil {
		t.Fatal("expected nil Top")
	}
	if pq.Pop() != nil {
		t.Fatal("expected nil Pop on empty queue")
	}
}
