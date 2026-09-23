package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockDocIdSetIterator struct {
	docs    []int
	pos     int
	current int
}

func newMockDocIdSetIterator(docs []int) *mockDocIdSetIterator {
	return &mockDocIdSetIterator{
		docs:    docs,
		pos:     0,
		current: -1,
	}
}

func (m *mockDocIdSetIterator) DocID() int {
	return m.current
}

func (m *mockDocIdSetIterator) NextDoc() (int, error) {
	if m.pos >= len(m.docs) {
		m.current = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	m.current = m.docs[m.pos]
	m.pos++
	return m.current, nil
}

func (m *mockDocIdSetIterator) Advance(target int) (int, error) {
	for m.pos < len(m.docs) && m.docs[m.pos] < target {
		m.pos++
	}
	if m.pos >= len(m.docs) {
		m.current = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	m.current = m.docs[m.pos]
	m.pos++
	return m.current, nil
}

func (m *mockDocIdSetIterator) DocIDRunEnd() (int, error) {
	return m.current, nil
}

func (m *mockDocIdSetIterator) Cost() int64 {
	return 1
}

// IntoBitSet carries the default body Lucene gives DocIdSetIterator.IntoBitSet.
func (m *mockDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

type mockVerifier struct {
	matches func(doc int) bool
}

func (v *mockVerifier) Matches() (bool, error) {
	// In a real implementation, we'd have access to the current docID.
	// Here we assume the TwoPhaseIterator manages it.
	// Wait, the Lucene TwoPhaseIterator.matches() doesn't take the docID.
	// It assumes the approximation is positioned.
	// Since we're testing, we need the verifier to know which doc is current.
	// I'll update the mockVerifier to take a pointer to the iterator.
	return false, nil // Will fix in a moment
}

func (v *mockVerifier) MatchCost() float32 {
	return 1.0
}

// Fixed mockVerifier
type fixedVerifier struct {
	iter *mockDocIdSetIterator
	fn   func(doc int) bool
}

func (v *fixedVerifier) Matches() (bool, error) {
	return v.fn(v.iter.current), nil
}

func (v *fixedVerifier) MatchCost() float32 {
	return 1.0
}

func TestTwoPhaseIterator_AsDocIdSetIterator(t *testing.T) {
	docs := []int{1, 2, 3, 4, 5}
	approx := newMockDocIdSetIterator(docs)

	// Verifier matches only even numbers
	verifier := &fixedVerifier{
		iter: approx,
		fn: func(doc int) bool {
			return doc%2 == 0
		},
	}

	tpi := NewTwoPhaseIterator(approx, verifier)
	iter := AsDocIdSetIterator(tpi)

	// First match should be 2
	doc, err := iter.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if doc != 2 {
		t.Errorf("expected 2, got %d", doc)
	}

	// Second match should be 4
	doc, err = iter.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if doc != 4 {
		t.Errorf("expected 4, got %d", doc)
	}

	// Third should be NO_MORE_DOCS
	doc, err = iter.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS, got %d", doc)
	}
}

func TestTwoPhaseIterator_Advance(t *testing.T) {
	docs := []int{1, 2, 3, 4, 5}
	approx := newMockDocIdSetIterator(docs)

	verifier := &fixedVerifier{
		iter: approx,
		fn: func(doc int) bool {
			return doc%2 == 0
		},
	}

	tpi := NewTwoPhaseIterator(approx, verifier)
	iter := AsDocIdSetIterator(tpi)

	// Advance to 3 -> next even is 4
	doc, err := iter.Advance(3)
	if err != nil {
		t.Fatalf("Advance failed: %v", err)
	}
	if doc != 4 {
		t.Errorf("expected 4, got %d", doc)
	}
}

func TestTwoPhaseIterator_IntoBitSet(t *testing.T) {
	docs := []int{1, 2, 3, 4, 5, 6}
	approx := newMockDocIdSetIterator(docs)

	verifier := &fixedVerifier{
		iter: approx,
		fn: func(doc int) bool {
			return doc%2 == 0
		},
	}

	tpi := NewTwoPhaseIterator(approx, verifier)

	bitSet, _ := util.NewFixedBitSet(10)
	// docs: [1, 2, 3, 4, 5, 6]
	// Even: [2, 4, 6]
	// offset = 0
	err := tpi.IntoBitSet(7, bitSet, 0)
	if err != nil {
		t.Fatalf("IntoBitSet failed: %v", err)
	}

	if !bitSet.Get(2) || !bitSet.Get(4) || !bitSet.Get(6) {
		t.Errorf("missing expected bits")
	}
	if bitSet.Get(1) || bitSet.Get(3) || bitSet.Get(5) {
		t.Errorf("found unexpected bits")
	}
}

func TestTwoPhaseIterator_Unwrap(t *testing.T) {
	docs := []int{1, 2, 3}
	approx := newMockDocIdSetIterator(docs)
	verifier := &fixedVerifier{iter: approx, fn: func(doc int) bool { return true }}
	tpi := NewTwoPhaseIterator(approx, verifier)

	iter := AsDocIdSetIterator(tpi)
	unwrapped := Unwrap(iter)
	if unwrapped != tpi {
		t.Errorf("unwrap failed to return original TwoPhaseIterator")
	}

	if Unwrap(approx) != nil {
		t.Errorf("unwrap should return nil for non-wrapped iterator")
	}
}
