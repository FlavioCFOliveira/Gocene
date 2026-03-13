// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestEmptyTerms(t *testing.T) {
	empty := &EmptyTerms{}

	// Test GetIterator
	te, err := empty.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator error: %v", err)
	}
	if te == nil {
		t.Fatal("GetIterator should return non-nil TermsEnum")
	}

	// Test iterator returns nil immediately
	term, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if term != nil {
		t.Error("Empty Terms should return nil on first Next()")
	}

	// Test GetIteratorWithSeek
	seekTerm := NewTerm("field", "value")
	te2, err := empty.GetIteratorWithSeek(seekTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	if te2 == nil {
		t.Fatal("GetIteratorWithSeek should return non-nil TermsEnum")
	}

	// Test Size
	if empty.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", empty.Size())
	}

	// Test GetDocCount
	docCount, err := empty.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount error: %v", err)
	}
	if docCount != 0 {
		t.Errorf("Expected GetDocCount=0, got %d", docCount)
	}

	// Test GetMin
	min, err := empty.GetMin()
	if err != nil {
		t.Fatalf("GetMin error: %v", err)
	}
	if min != nil {
		t.Error("GetMin should return nil for empty terms")
	}

	// Test GetMax
	max, err := empty.GetMax()
	if err != nil {
		t.Fatalf("GetMax error: %v", err)
	}
	if max != nil {
		t.Error("GetMax should return nil for empty terms")
	}
}

func TestSingleTermTerms(t *testing.T) {
	term := NewTerm("field", "test")
	st := NewSingleTermTerms(term, 5, 10)

	// Test Size
	if st.Size() != 1 {
		t.Errorf("Expected Size=1, got %d", st.Size())
	}

	// Test GetIterator
	te, err := st.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator error: %v", err)
	}

	// First Next should return the term
	gotTerm, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm == nil {
		t.Fatal("Expected term, got nil")
	}
	if !gotTerm.Equals(term) {
		t.Errorf("Expected term %v, got %v", term, gotTerm)
	}

	// Second Next should return nil
	gotTerm2, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("Second Next should return nil")
	}

	// Test GetDocCount
	docCount, err := st.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount error: %v", err)
	}
	if docCount != 1 {
		t.Errorf("Expected GetDocCount=1, got %d", docCount)
	}

	// Test GetSumDocFreq
	sumDocFreq, err := st.GetSumDocFreq()
	if err != nil {
		t.Fatalf("GetSumDocFreq error: %v", err)
	}
	if sumDocFreq != 5 {
		t.Errorf("Expected GetSumDocFreq=5, got %d", sumDocFreq)
	}

	// Test GetSumTotalTermFreq
	sumTotalFreq, err := st.GetSumTotalTermFreq()
	if err != nil {
		t.Fatalf("GetSumTotalTermFreq error: %v", err)
	}
	if sumTotalFreq != 10 {
		t.Errorf("Expected GetSumTotalTermFreq=10, got %d", sumTotalFreq)
	}

	// Test GetMin
	min, err := st.GetMin()
	if err != nil {
		t.Fatalf("GetMin error: %v", err)
	}
	if min == nil {
		t.Fatal("GetMin should return the term")
	}
	if !min.Equals(term) {
		t.Error("GetMin should return the single term")
	}

	// Test GetMax
	max, err := st.GetMax()
	if err != nil {
		t.Fatalf("GetMax error: %v", err)
	}
	if max == nil {
		t.Fatal("GetMax should return the term")
	}
	if !max.Equals(term) {
		t.Error("GetMax should return the single term")
	}
}

func TestSingleTermTerms_SeekCeil(t *testing.T) {
	term := NewTerm("field", "hello")
	st := NewSingleTermTerms(term, 1, 1)

	// Seek to a term before ours - should return our term
	beforeTerm := NewTerm("field", "alpha")
	te, err := st.GetIteratorWithSeek(beforeTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	gotTerm, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm == nil || !gotTerm.Equals(term) {
		t.Error("SeekCeil before term should return our term")
	}

	// Seek to a term after ours - should return nil
	st2 := NewSingleTermTerms(term, 1, 1)
	afterTerm := NewTerm("field", "zebra")
	te2, err := st2.GetIteratorWithSeek(afterTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	gotTerm2, err := te2.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("SeekCeil after term should return nil")
	}
}

// Mock MultiTermTerms for comprehensive testing
type mockMultiTermTerms struct {
	TermsBase
	terms []*Term
}

func (m *mockMultiTermTerms) GetIterator() (TermsEnum, error) {
	return &mockMultiTermsEnum{terms: m.terms, pos: -1}, nil
}

func (m *mockMultiTermTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	te := &mockMultiTermsEnum{terms: m.terms, pos: -1}
	_, err := te.SeekCeil(seekTerm)
	return te, err
}

func (m *mockMultiTermTerms) Size() int64 {
	return int64(len(m.terms))
}

type mockMultiTermsEnum struct {
	TermsEnumBase
	terms []*Term
	pos   int
}

func (m *mockMultiTermsEnum) Next() (*Term, error) {
	m.pos++
	if m.pos >= len(m.terms) {
		m.currentTerm = nil
		return nil, nil
	}
	m.currentTerm = m.terms[m.pos]
	return m.currentTerm, nil
}

func (m *mockMultiTermsEnum) SeekCeil(term *Term) (*Term, error) {
	idx := sort.Search(len(m.terms), func(i int) bool {
		return m.terms[i].CompareTo(term) >= 0
	})
	m.pos = idx - 1 // Next() will advance it to idx
	return m.Next()
}

func (m *mockMultiTermsEnum) SeekExact(term *Term) (bool, error) {
	got, err := m.SeekCeil(term)
	if err != nil {
		return false, err
	}
	return got != nil && got.Equals(term), nil
}

func (m *mockMultiTermsEnum) DocFreq() (int, error) {
	return 1, nil
}

func (m *mockMultiTermsEnum) TotalTermFreq() (int64, error) {
	return 1, nil
}

func (m *mockMultiTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

func (m *mockMultiTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

func TestTermsEnum_MultiTerm(t *testing.T) {
	terms := []*Term{
		NewTerm("f", "apple"),
		NewTerm("f", "banana"),
		NewTerm("f", "cherry"),
		NewTerm("f", "date"),
	}

	m := &mockMultiTermTerms{terms: terms}
	te, _ := m.GetIterator()

	// Test sequential iteration
	for i, expected := range terms {
		got, _ := te.Next()
		if got == nil || !got.Equals(expected) {
			t.Errorf("At index %d: expected %v, got %v", i, expected, got)
		}
	}
	got, _ := te.Next()
	if got != nil {
		t.Error("Expected nil after end of terms")
	}

	// Test SeekCeil
	te2, _ := m.GetIterator()
	found, _ := te2.SeekCeil(NewTerm("f", "b"))
	if found == nil || !found.Equals(terms[1]) {
		t.Errorf("SeekCeil('b') expected 'banana', got %v", found)
	}

	found, _ = te2.SeekCeil(NewTerm("f", "banana"))
	if found == nil || !found.Equals(terms[1]) {
		t.Errorf("SeekCeil('banana') expected 'banana', got %v", found)
	}

	found, _ = te2.SeekCeil(NewTerm("f", "dog"))
	if found != nil {
		t.Errorf("SeekCeil('dog') expected nil, got %v", found)
	}
}

func TestPostingsEnum_Basic(t *testing.T) {
	// Test SingleDocPostingsEnum
	pe := NewSingleDocPostingsEnum(10, 5)

	doc, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != 10 {
		t.Errorf("Expected doc 10, got %d", doc)
	}

	freq, _ := pe.Freq()
	if freq != 5 {
		t.Errorf("Expected freq 5, got %d", freq)
	}

	doc, _ = pe.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Error("Expected NO_MORE_DOCS")
	}

	// Test Advance
	pe2 := NewSingleDocPostingsEnum(20, 1)
	doc, _ = pe2.Advance(15)
	if doc != 20 {
		t.Errorf("Advance(15) expected 20, got %d", doc)
	}

	pe3 := NewSingleDocPostingsEnum(20, 1)
	doc, _ = pe3.Advance(25)
	if doc != NO_MORE_DOCS {
		t.Errorf("Advance(25) expected NO_MORE_DOCS, got %d", doc)
	}
}

type mockMultiDocPostingsEnum struct {
	PostingsEnumBase
	docs  []int
	freqs []int
	pos   int
	cost  int64
}

func (m *mockMultiDocPostingsEnum) NextDoc() (int, error) {
	m.pos++
	if m.pos >= len(m.docs) {
		m.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	m.currentDoc = m.docs[m.pos]
	return m.currentDoc, nil
}

func (m *mockMultiDocPostingsEnum) Advance(target int) (int, error) {
	idx := sort.Search(len(m.docs), func(i int) bool {
		return m.docs[i] >= target
	})
	m.pos = idx - 1
	return m.NextDoc()
}

func (m *mockMultiDocPostingsEnum) Freq() (int, error) {
	if m.pos < 0 || m.pos >= len(m.freqs) {
		return 0, nil
	}
	return m.freqs[m.pos], nil
}

func (m *mockMultiDocPostingsEnum) NextPosition() (int, error) { return NO_MORE_POSITIONS, nil }
func (m *mockMultiDocPostingsEnum) StartOffset() (int, error)  { return -1, nil }
func (m *mockMultiDocPostingsEnum) EndOffset() (int, error)    { return -1, nil }
func (m *mockMultiDocPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (m *mockMultiDocPostingsEnum) Cost() int64                { return m.cost }

func TestPostingsEnum_MultiDoc(t *testing.T) {
	docs := []int{1, 3, 5, 7, 9}
	freqs := []int{10, 20, 30, 40, 50}
	pe := &mockMultiDocPostingsEnum{
		docs:  docs,
		freqs: freqs,
		pos:   -1,
		cost:  5,
	}

	// Test sequential iteration
	for i, expected := range docs {
		doc, _ := pe.NextDoc()
		if doc != expected {
			t.Errorf("At index %d: expected doc %d, got %d", i, expected, doc)
		}
		freq, _ := pe.Freq()
		if freq != freqs[i] {
			t.Errorf("At index %d: expected freq %d, got %d", i, freqs[i], freq)
		}
	}
	doc, _ := pe.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Error("Expected NO_MORE_DOCS at end")
	}

	// Test Advance
	pe2 := &mockMultiDocPostingsEnum{docs: docs, freqs: freqs, pos: -1}
	doc, _ = pe2.Advance(4)
	if doc != 5 {
		t.Errorf("Advance(4) expected 5, got %d", doc)
	}
	doc, _ = pe2.NextDoc()
	if doc != 7 {
		t.Errorf("NextDoc after Advance expected 7, got %d", doc)
	}
}
