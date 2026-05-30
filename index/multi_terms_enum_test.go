// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// In-memory test doubles for a single field's postings.
//
// These fakes implement the Terms / TermsEnum / PostingsEnum interfaces with a
// simple sorted map of term -> []docID (local to the sub-reader). They let the
// MultiTermsEnum merge logic be exercised in isolation, without standing up a
// full codec. Doc IDs stored here are LOCAL to the sub; the composite doc IDs
// are produced by MultiPostingsEnum via each sub's ReaderSlice base.
// ---------------------------------------------------------------------------

// memTermPostings holds the local doc IDs (ascending) for one term.
type memTermPostings struct {
	docs []int
}

// memTerms is an in-memory Terms over one field.
type memTerms struct {
	field string
	terms map[string]memTermPostings
	// sorted keys (byte order) cached for iteration.
	keys []string
}

func newMemTerms(field string, data map[string][]int) *memTerms {
	mt := &memTerms{field: field, terms: make(map[string]memTermPostings, len(data))}
	for k, docs := range data {
		cp := append([]int(nil), docs...)
		sort.Ints(cp)
		mt.terms[k] = memTermPostings{docs: cp}
		mt.keys = append(mt.keys, k)
	}
	sort.Strings(mt.keys) // byte order for ASCII keys
	return mt
}

func (m *memTerms) GetIterator() (TermsEnum, error) {
	return &memTermsEnum{owner: m, pos: -1}, nil
}
func (m *memTerms) GetIteratorWithSeek(seek *Term) (TermsEnum, error) {
	it := &memTermsEnum{owner: m, pos: -1}
	if seek == nil {
		return it, nil
	}
	if _, err := it.SeekCeil(seek); err != nil {
		return nil, err
	}
	return it, nil
}
func (m *memTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	it := &memTermsEnum{owner: m, pos: -1}
	found, err := it.SeekExact(NewTerm(m.field, termText))
	if err != nil || !found {
		return nil, err
	}
	return it.Postings(flags)
}
func (m *memTerms) Size() int64                         { return int64(len(m.keys)) }
func (m *memTerms) GetDocCount() (int, error)           { return len(m.keys), nil }
func (m *memTerms) GetSumDocFreq() (int64, error)       { return -1, nil }
func (m *memTerms) GetSumTotalTermFreq() (int64, error) { return -1, nil }
func (m *memTerms) HasFreqs() bool                      { return true }
func (m *memTerms) HasOffsets() bool                    { return false }
func (m *memTerms) HasPositions() bool                  { return false }
func (m *memTerms) HasPayloads() bool                   { return false }
func (m *memTerms) GetMin() (*Term, error) {
	if len(m.keys) == 0 {
		return nil, nil
	}
	return NewTerm(m.field, m.keys[0]), nil
}
func (m *memTerms) GetMax() (*Term, error) {
	if len(m.keys) == 0 {
		return nil, nil
	}
	return NewTerm(m.field, m.keys[len(m.keys)-1]), nil
}

// memTermsEnum iterates a memTerms in byte order.
type memTermsEnum struct {
	owner *memTerms
	pos   int // index into owner.keys; -1 before first Next
}

func (e *memTermsEnum) Term() *Term {
	if e.pos < 0 || e.pos >= len(e.owner.keys) {
		return nil
	}
	return NewTerm(e.owner.field, e.owner.keys[e.pos])
}

func (e *memTermsEnum) Next() (*Term, error) {
	e.pos++
	return e.Term(), nil
}

func (e *memTermsEnum) SeekCeil(target *Term) (*Term, error) {
	keys := e.owner.keys
	t := target.Text()
	idx := sort.SearchStrings(keys, t)
	e.pos = idx
	return e.Term(), nil
}

func (e *memTermsEnum) SeekExact(target *Term) (bool, error) {
	landed, err := e.SeekCeil(target)
	if err != nil {
		return false, err
	}
	return landed != nil && landed.Text() == target.Text(), nil
}

func (e *memTermsEnum) DocFreq() (int, error) {
	tp, ok := e.owner.terms[e.curKey()]
	if !ok {
		return 0, nil
	}
	return len(tp.docs), nil
}

func (e *memTermsEnum) TotalTermFreq() (int64, error) {
	tp, ok := e.owner.terms[e.curKey()]
	if !ok {
		return 0, nil
	}
	// One occurrence per doc in this fake.
	return int64(len(tp.docs)), nil
}

func (e *memTermsEnum) curKey() string {
	if e.pos < 0 || e.pos >= len(e.owner.keys) {
		return ""
	}
	return e.owner.keys[e.pos]
}

func (e *memTermsEnum) Postings(_ int) (PostingsEnum, error) {
	tp := e.owner.terms[e.curKey()]
	return &memPostingsEnum{docs: tp.docs, idx: -1, doc: -1}, nil
}

func (e *memTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (PostingsEnum, error) {
	return e.Postings(flags)
}

// memPostingsEnum walks a term's local doc IDs.
type memPostingsEnum struct {
	docs []int
	idx  int
	doc  int
}

func (p *memPostingsEnum) DocID() int { return p.doc }

func (p *memPostingsEnum) NextDoc() (int, error) {
	p.idx++
	if p.idx >= len(p.docs) {
		p.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	p.doc = p.docs[p.idx]
	return p.doc, nil
}

func (p *memPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

func (p *memPostingsEnum) Cost() int64                 { return int64(len(p.docs)) }
func (p *memPostingsEnum) Freq() (int, error)          { return 1, nil }
func (p *memPostingsEnum) NextPosition() (int, error)  { return -1, nil }
func (p *memPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *memPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *memPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

// Compile-time interface assertions for the fakes.
var (
	_ Terms        = (*memTerms)(nil)
	_ TermsEnum    = (*memTermsEnum)(nil)
	_ PostingsEnum = (*memPostingsEnum)(nil)
)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// drainAllDocs reads every (compositeDocID, freq) pair from a PostingsEnum.
func drainAllDocs(t *testing.T, pe PostingsEnum) []int {
	t.Helper()
	var got []int
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		got = append(got, doc)
	}
	return got
}

// TestMultiTermsEnum_MergedIteration builds three sub-segments over one field
// and verifies that the merged enum yields terms in byte order with correctly
// MERGED docFreq/totalTermFreq and correctly base-offset postings across subs.
func TestMultiTermsEnum_MergedIteration(t *testing.T) {
	const field = "body"

	// Sub 0: docs 0..9   (base 0)
	// Sub 1: docs 0..4   (base 10)
	// Sub 2: docs 0..7   (base 15)
	sub0 := newMemTerms(field, map[string][]int{
		"apple":  {0, 3},
		"cherry": {1},
		"mango":  {2, 5, 9},
	})
	sub1 := newMemTerms(field, map[string][]int{
		"apple":  {0, 2, 4}, // shares "apple" with sub0 and sub2
		"banana": {1},
		"mango":  {3}, // shares "mango" with sub0
	})
	sub2 := newMemTerms(field, map[string][]int{
		"apple": {6, 7}, // shares "apple" with sub0 and sub1
		"date":  {0},
	})

	subs := []Terms{sub0, sub1, sub2}
	slices := []ReaderSlice{
		{Start: 0, Length: 10, ReaderIndex: 0},
		{Start: 10, Length: 5, ReaderIndex: 1},
		{Start: 15, Length: 8, ReaderIndex: 2},
	}

	mt, err := NewMultiTerms(subs, slices)
	if err != nil {
		t.Fatalf("NewMultiTerms: %v", err)
	}
	it, err := mt.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}

	// Expected merged term order (byte order, deduplicated):
	//   apple, banana, cherry, date, mango
	type want struct {
		term    string
		docFreq int
		ttf     int64
		docs    []int // composite doc IDs
	}
	wants := []want{
		{
			term:    "apple",
			docFreq: 2 + 3 + 2, // sub0:2, sub1:3, sub2:2
			ttf:     7,
			// sub0 local {0,3} -> {0,3}; sub1 local {0,2,4} -> {10,12,14};
			// sub2 local {6,7} -> {21,22}
			docs: []int{0, 3, 10, 12, 14, 21, 22},
		},
		{
			term:    "banana",
			docFreq: 1, // sub1 only
			ttf:     1,
			docs:    []int{11}, // sub1 local {1} -> {11}
		},
		{
			term:    "cherry",
			docFreq: 1, // sub0 only
			ttf:     1,
			docs:    []int{1}, // sub0 local {1} -> {1}
		},
		{
			term:    "date",
			docFreq: 1, // sub2 only
			ttf:     1,
			docs:    []int{15}, // sub2 local {0} -> {15}
		},
		{
			term:    "mango",
			docFreq: 3 + 1, // sub0:3, sub1:1
			ttf:     4,
			// sub0 local {2,5,9} -> {2,5,9}; sub1 local {3} -> {13}
			docs: []int{2, 5, 9, 13},
		},
	}

	for i, w := range wants {
		term, err := it.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if term == nil {
			t.Fatalf("Next #%d: got nil, want %q", i, w.term)
		}
		if term.Field != field {
			t.Fatalf("Next #%d: field=%q want %q", i, term.Field, field)
		}
		if term.Text() != w.term {
			t.Fatalf("Next #%d: term=%q want %q", i, term.Text(), w.term)
		}

		df, err := it.DocFreq()
		if err != nil {
			t.Fatalf("DocFreq %q: %v", w.term, err)
		}
		if df != w.docFreq {
			t.Fatalf("DocFreq %q: got %d want %d", w.term, df, w.docFreq)
		}

		ttf, err := it.TotalTermFreq()
		if err != nil {
			t.Fatalf("TotalTermFreq %q: %v", w.term, err)
		}
		if ttf != w.ttf {
			t.Fatalf("TotalTermFreq %q: got %d want %d", w.term, ttf, w.ttf)
		}

		pe, err := it.Postings(0)
		if err != nil {
			t.Fatalf("Postings %q: %v", w.term, err)
		}
		gotDocs := drainAllDocs(t, pe)
		if !intsEqual(gotDocs, w.docs) {
			t.Fatalf("Postings %q: got docs %v want %v", w.term, gotDocs, w.docs)
		}
	}

	// Enum must now be exhausted.
	if term, err := it.Next(); err != nil {
		t.Fatalf("final Next: %v", err)
	} else if term != nil {
		t.Fatalf("final Next: want nil, got %q", term.Text())
	}
}

// TestMultiTermsEnum_SeekExactAndCeil verifies seek positioning, the
// merged docFreq after a seek, and that Next after a seekExact correctly
// advances across all subs (the lastSeekExact re-seek path).
func TestMultiTermsEnum_SeekExactAndCeil(t *testing.T) {
	const field = "f"
	sub0 := newMemTerms(field, map[string][]int{"a": {0}, "c": {1}, "e": {2}})
	sub1 := newMemTerms(field, map[string][]int{"b": {0}, "c": {1}, "d": {2}})

	slices := []ReaderSlice{
		{Start: 0, Length: 3, ReaderIndex: 0},
		{Start: 3, Length: 3, ReaderIndex: 1},
	}
	mt, err := NewMultiTerms([]Terms{sub0, sub1}, slices)
	if err != nil {
		t.Fatalf("NewMultiTerms: %v", err)
	}
	it, err := mt.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}

	// SeekExact on "c", which is in both subs -> match, docFreq 2, postings
	// {1 (sub0 base 0), 4 (sub1 base 3)}.
	found, err := it.SeekExact(NewTerm(field, "c"))
	if err != nil {
		t.Fatalf("SeekExact c: %v", err)
	}
	if !found {
		t.Fatalf("SeekExact c: want found")
	}
	if df, _ := it.DocFreq(); df != 2 {
		t.Fatalf("DocFreq after SeekExact c: got %d want 2", df)
	}
	pe, err := it.Postings(0)
	if err != nil {
		t.Fatalf("Postings c: %v", err)
	}
	if got := drainAllDocs(t, pe); !intsEqual(got, []int{1, 4}) {
		t.Fatalf("Postings c: got %v want [1 4]", got)
	}

	// Next after SeekExact must advance to "d" (the next merged term), exercising
	// the lastSeekExact -> seekCeil re-seek path. "c" was consumed; remaining
	// merged terms are d, e.
	term, err := it.Next()
	if err != nil {
		t.Fatalf("Next after SeekExact: %v", err)
	}
	if term == nil || term.Text() != "d" {
		t.Fatalf("Next after SeekExact: got %v want d", termText(term))
	}
	term, err = it.Next()
	if err != nil {
		t.Fatalf("Next->e: %v", err)
	}
	if term == nil || term.Text() != "e" {
		t.Fatalf("Next: got %v want e", termText(term))
	}

	// SeekExact on a missing term returns false.
	missing, err := it.SeekExact(NewTerm(field, "zzz"))
	if err != nil {
		t.Fatalf("SeekExact zzz: %v", err)
	}
	if missing {
		t.Fatalf("SeekExact zzz: want not found")
	}

	// Fresh iterator for SeekCeil checks.
	it2, err := mt.Iterator()
	if err != nil {
		t.Fatalf("Iterator2: %v", err)
	}
	// SeekCeil "bb" (absent) -> smallest term > "bb" is "c".
	landed, err := it2.SeekCeil(NewTerm(field, "bb"))
	if err != nil {
		t.Fatalf("SeekCeil bb: %v", err)
	}
	if landed == nil || landed.Text() != "c" {
		t.Fatalf("SeekCeil bb: got %v want c", termText(landed))
	}
	// SeekCeil exact "a" -> "a".
	landed, err = it2.SeekCeil(NewTerm(field, "a"))
	if err != nil {
		t.Fatalf("SeekCeil a: %v", err)
	}
	if landed == nil || landed.Text() != "a" {
		t.Fatalf("SeekCeil a: got %v want a", termText(landed))
	}
	// SeekCeil past the end -> nil (END).
	landed, err = it2.SeekCeil(NewTerm(field, "zzz"))
	if err != nil {
		t.Fatalf("SeekCeil zzz: %v", err)
	}
	if landed != nil {
		t.Fatalf("SeekCeil zzz: got %v want nil", termText(landed))
	}
}

// TestMultiTermsEnum_EmptySubs verifies that subs with no terms are dropped and
// an all-empty MultiTerms yields an immediately-exhausted enum.
func TestMultiTermsEnum_EmptySubs(t *testing.T) {
	const field = "x"
	empty := newMemTerms(field, map[string][]int{})
	withTerms := newMemTerms(field, map[string][]int{"k": {0, 1}})

	slices := []ReaderSlice{
		{Start: 0, Length: 5, ReaderIndex: 0},
		{Start: 5, Length: 2, ReaderIndex: 1},
	}
	mt, err := NewMultiTerms([]Terms{empty, withTerms}, slices)
	if err != nil {
		t.Fatalf("NewMultiTerms: %v", err)
	}
	it, err := mt.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}
	term, err := it.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if term == nil || term.Text() != "k" {
		t.Fatalf("Next: got %v want k", termText(term))
	}
	if df, _ := it.DocFreq(); df != 2 {
		t.Fatalf("DocFreq k: got %d want 2", df)
	}
	pe, err := it.Postings(0)
	if err != nil {
		t.Fatalf("Postings k: %v", err)
	}
	// withTerms is sub index 1, base 5 -> local {0,1} maps to {5,6}.
	if got := drainAllDocs(t, pe); !intsEqual(got, []int{5, 6}) {
		t.Fatalf("Postings k: got %v want [5 6]", got)
	}
	if term, _ := it.Next(); term != nil {
		t.Fatalf("Next after last: want nil, got %q", term.Text())
	}

	// All-empty -> empty enum.
	mtEmpty, err := NewMultiTerms([]Terms{empty}, []ReaderSlice{{Start: 0, Length: 5, ReaderIndex: 0}})
	if err != nil {
		t.Fatalf("NewMultiTerms empty: %v", err)
	}
	itEmpty, err := mtEmpty.Iterator()
	if err != nil {
		t.Fatalf("Iterator empty: %v", err)
	}
	if term, err := itEmpty.Next(); err != nil || term != nil {
		t.Fatalf("empty enum Next: got (%v,%v) want (nil,nil)", termText(term), err)
	}
}

func termText(t *Term) string {
	if t == nil {
		return "<nil>"
	}
	return t.Text()
}

func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
