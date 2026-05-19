// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestFrozenBufferedUpdates_NewProjectsTermsAndQueries(t *testing.T) {
	t.Parallel()
	bu := NewBufferedUpdates("seg-1")
	bu.AddTerm(NewTerm("body", "alpha"), 5)
	bu.AddTerm(NewTerm("body", "beta"), 10)
	bu.AddTerm(NewTerm("title", "gamma"), 1)
	bu.AddQuery(testQuery{id: 1}, 100)
	bu.AddQuery(testQuery{id: 2}, 200)

	f, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	if !f.Any() {
		t.Fatal("Any() = false, want true")
	}
	if got, want := f.DeleteTermsSize(), int64(3); got != want {
		t.Fatalf("DeleteTermsSize = %d, want %d", got, want)
	}
	if got, want := len(f.DeleteQueries()), 2; got != want {
		t.Fatalf("DeleteQueries length = %d, want %d", got, want)
	}
	if got, want := len(f.DeleteQueryLimits()), 2; got != want {
		t.Fatalf("DeleteQueryLimits length = %d, want %d", got, want)
	}
	if f.BytesUsed() <= 0 {
		t.Fatalf("BytesUsed should be > 0, got %d", f.BytesUsed())
	}
	if f.DelGen() != -1 {
		t.Fatalf("DelGen pre-set = %d, want -1", f.DelGen())
	}
	if f.PrivateSegment() != nil {
		t.Fatalf("PrivateSegment = %v, want nil", f.PrivateSegment())
	}
}

func TestFrozenBufferedUpdates_RejectsPrivatePacketWithTerms(t *testing.T) {
	t.Parallel()
	bu := NewBufferedUpdates("seg-private")
	bu.AddTerm(NewTerm("body", "x"), 1)
	_, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, &SegmentCommitInfo{})
	if err == nil {
		t.Fatal("expected error when privateSegment carries term deletes")
	}
}

func TestFrozenBufferedUpdates_AcceptsPrivatePacketWithOnlyQueries(t *testing.T) {
	t.Parallel()
	bu := NewBufferedUpdates("seg-private")
	bu.AddQuery(testQuery{id: 9}, 50)
	f, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, &SegmentCommitInfo{})
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	if f.PrivateSegment() == nil {
		t.Fatal("PrivateSegment = nil, want non-nil")
	}
}

func TestFrozenBufferedUpdates_NilUpdatesRejected(t *testing.T) {
	t.Parallel()
	_, err := NewFrozenBufferedUpdates(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil updates")
	}
}

func TestFrozenBufferedUpdates_LockUnlockTryLock(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	if !f.TryLock() {
		t.Fatal("first TryLock should succeed")
	}
	if f.TryLock() {
		t.Fatal("second TryLock should fail while locked")
	}
	f.Unlock()
	f.Lock()
	f.Unlock()
}

func TestFrozenBufferedUpdates_SetDelGenIdempotent(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	if err := f.SetDelGen(42); err != nil {
		t.Fatalf("SetDelGen: %v", err)
	}
	if got, want := f.DelGen(), int64(42); got != want {
		t.Fatalf("DelGen = %d, want %d", got, want)
	}
	if err := f.SetDelGen(99); err == nil {
		t.Fatal("second SetDelGen should fail")
	}
}

func TestFrozenBufferedUpdates_ApplyEmptyFiresLatch(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	f.Lock()
	defer f.Unlock()
	if err := f.SetDelGen(1); err != nil {
		t.Fatalf("SetDelGen: %v", err)
	}
	count, err := f.Apply(nil)
	if err != nil {
		t.Fatalf("Apply on empty packet: %v", err)
	}
	if count != 0 {
		t.Fatalf("Apply count = %d, want 0", count)
	}
	if !f.IsApplied() {
		t.Fatal("IsApplied = false, want true")
	}
	select {
	case <-f.Applied():
	default:
		t.Fatal("Applied latch did not fire")
	}
}

func TestFrozenBufferedUpdates_ApplyWithoutDelGen(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	f.Lock()
	defer f.Unlock()
	if _, err := f.Apply(nil); !errors.Is(err, ErrFrozenBufferedUpdatesNotPushed) {
		t.Fatalf("Apply pre-delGen error = %v, want %v",
			err, ErrFrozenBufferedUpdatesNotPushed)
	}
}

func TestFrozenBufferedUpdates_ApplyRequiresLock(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	if _, err := f.Apply(nil); err == nil {
		t.Fatal("Apply without Lock should fail")
	}
}

func TestFrozenBufferedUpdates_ApplyNonEmptyReturnsPendingError(t *testing.T) {
	t.Parallel()
	bu := NewBufferedUpdates("seg")
	bu.AddTerm(NewTerm("body", "k"), 1)
	f, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	f.Lock()
	defer f.Unlock()
	if err := f.SetDelGen(1); err != nil {
		t.Fatalf("SetDelGen: %v", err)
	}
	_, err = f.Apply([]*FrozenSegmentState{{Reader: nil, DelGen: 0, RefCount: 2}})
	if !errors.Is(err, ErrFrozenBufferedUpdatesNotApplicable) {
		t.Fatalf("Apply on non-empty packet error = %v, want %v",
			err, ErrFrozenBufferedUpdatesNotApplicable)
	}
	// The latch must NOT fire when the work was not performed.
	select {
	case <-f.Applied():
		t.Fatal("Applied latch fired despite pending dependency error")
	default:
	}
}

func TestFrozenBufferedUpdates_String(t *testing.T) {
	t.Parallel()
	bu := NewBufferedUpdates("seg")
	bu.AddTerm(NewTerm("body", "k"), 1)
	bu.AddQuery(testQuery{id: 7}, 9)
	f, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	if err := f.SetDelGen(11); err != nil {
		t.Fatalf("SetDelGen: %v", err)
	}
	got := f.String()
	for _, want := range []string{"delGen=11", "unique deleteTerms=1", "numDeleteQueries=1", "bytesUsed="} {
		if !contains(got, want) {
			t.Fatalf("String() = %q, missing %q", got, want)
		}
	}
}

func TestFrozenBufferedUpdates_LatchConcurrentReaders(t *testing.T) {
	t.Parallel()
	f := newEmptyFrozen(t)
	const readers = 8
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			<-f.Applied()
		}()
	}
	f.Lock()
	if err := f.SetDelGen(1); err != nil {
		t.Fatalf("SetDelGen: %v", err)
	}
	if _, err := f.Apply(nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	f.Unlock()
	wg.Wait()
}

func TestTermDocsIterator_SortedHappyPath(t *testing.T) {
	t.Parallel()
	mf := NewMemoryFields()
	mf.AddField("body", newMultiTermTerms([]termWithDocs{
		{text: "alpha", docs: []int{0, 2, 4}},
		{text: "beta", docs: []int{1, 3}},
		{text: "gamma", docs: []int{5}},
	}))
	it := NewTermDocsIteratorFromFields(mf, true)

	cases := []struct {
		term string
		want []int
	}{
		{"alpha", []int{0, 2, 4}},
		{"beta", []int{1, 3}},
		{"gamma", []int{5}},
	}
	for _, c := range cases {
		pe, err := it.NextTerm("body", []byte(c.term))
		if err != nil {
			t.Fatalf("NextTerm(%s): %v", c.term, err)
		}
		if pe == nil {
			t.Fatalf("NextTerm(%s) returned nil postings, want hits", c.term)
		}
		got := drainPostings(t, pe)
		if !equalInts(got, c.want) {
			t.Fatalf("term %s docs = %v, want %v", c.term, got, c.want)
		}
	}
}

func TestTermDocsIterator_UnsortedHappyPath(t *testing.T) {
	t.Parallel()
	mf := NewMemoryFields()
	mf.AddField("body", newMultiTermTerms([]termWithDocs{
		{text: "alpha", docs: []int{1}},
		{text: "beta", docs: []int{2}},
	}))
	it := NewTermDocsIteratorFromFields(mf, false)

	// Walk out of order to exercise the unsorted SeekExact path.
	pe, err := it.NextTerm("body", []byte("beta"))
	if err != nil || pe == nil {
		t.Fatalf("NextTerm(beta) = (%v, %v)", pe, err)
	}
	if got := drainPostings(t, pe); !equalInts(got, []int{2}) {
		t.Fatalf("beta docs = %v, want [2]", got)
	}
	pe, err = it.NextTerm("body", []byte("alpha"))
	if err != nil || pe == nil {
		t.Fatalf("NextTerm(alpha) = (%v, %v)", pe, err)
	}
	if got := drainPostings(t, pe); !equalInts(got, []int{1}) {
		t.Fatalf("alpha docs = %v, want [1]", got)
	}
}

func TestTermDocsIterator_SortedMissBelowFirst(t *testing.T) {
	t.Parallel()
	mf := NewMemoryFields()
	mf.AddField("body", newMultiTermTerms([]termWithDocs{
		{text: "mango", docs: []int{0}},
	}))
	it := NewTermDocsIteratorFromFields(mf, true)
	// "apple" sorts before the only term "mango"; sorted-mode shortcut
	// must return nil without consuming the dictionary.
	pe, err := it.NextTerm("body", []byte("apple"))
	if err != nil {
		t.Fatalf("NextTerm(apple): %v", err)
	}
	if pe != nil {
		t.Fatal("expected nil postings for term sorting before first dict term")
	}
}

func TestTermDocsIterator_SortedOutOfOrderRejected(t *testing.T) {
	t.Parallel()
	mf := NewMemoryFields()
	mf.AddField("body", newMultiTermTerms([]termWithDocs{
		{text: "alpha", docs: []int{0}},
		{text: "beta", docs: []int{1}},
	}))
	it := NewTermDocsIteratorFromFields(mf, true)
	if _, err := it.NextTerm("body", []byte("beta")); err != nil {
		t.Fatalf("NextTerm(beta): %v", err)
	}
	if _, err := it.NextTerm("body", []byte("alpha")); err == nil {
		t.Fatal("expected out-of-order error in sorted mode")
	}
}

func TestTermDocsIterator_MissingFieldReturnsNil(t *testing.T) {
	t.Parallel()
	mf := NewMemoryFields()
	it := NewTermDocsIteratorFromFields(mf, false)
	pe, err := it.NextTerm("absent", []byte("anything"))
	if err != nil {
		t.Fatalf("NextTerm on absent field: %v", err)
	}
	if pe != nil {
		t.Fatal("expected nil postings when field is absent")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newEmptyFrozen(t *testing.T) *FrozenBufferedUpdates {
	t.Helper()
	bu := NewBufferedUpdates("seg-empty")
	f, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	return f
}

func drainPostings(t *testing.T, pe PostingsEnum) []int {
	t.Helper()
	var out []int
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			return out
		}
		out = append(out, doc)
	}
}

func equalInts(a, b []int) bool {
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

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// testQuery is a minimal in-test Query implementation. The frozen packet
// only exercises HashCode + identity, never Rewrite/CreateWeight.
type testQuery struct{ id int }

func (q testQuery) Rewrite(_ *IndexReader) (Query, error) { return q, nil }
func (q testQuery) Clone() Query                          { return q }
func (q testQuery) Equals(other Query) bool {
	o, ok := other.(testQuery)
	return ok && o.id == q.id
}
func (q testQuery) HashCode() int { return q.id }
func (q testQuery) CreateWeight(_ IndexSearcher, _ bool, _ float32) (Weight, error) {
	return nil, errors.New("test query: CreateWeight not used in this test")
}

// termWithDocs is a (term, doc-ids) pair used by newMultiTermTerms.
type termWithDocs struct {
	text string
	docs []int
}

// newMultiTermTerms builds a tiny in-memory Terms backed by a sorted slice.
// Doc-ids per term must be monotonically increasing.
func newMultiTermTerms(entries []termWithDocs) Terms {
	return &multiTermTerms{entries: entries}
}

type multiTermTerms struct {
	TermsBase
	entries []termWithDocs
}

func (m *multiTermTerms) GetIterator() (TermsEnum, error) {
	return &multiTermTermsEnum{owner: m, idx: -1}, nil
}

func (m *multiTermTerms) GetIteratorWithSeek(seek *Term) (TermsEnum, error) {
	return m.GetIterator()
}

func (m *multiTermTerms) GetPostingsReader(_ string, _ int) (PostingsEnum, error) {
	return nil, nil
}

func (m *multiTermTerms) Size() int64               { return int64(len(m.entries)) }
func (m *multiTermTerms) GetMin() (*Term, error)    { return nil, nil }
func (m *multiTermTerms) GetMax() (*Term, error)    { return nil, nil }
func (m *multiTermTerms) HasFreqs() bool            { return false }
func (m *multiTermTerms) HasOffsets() bool          { return false }
func (m *multiTermTerms) HasPositions() bool        { return false }
func (m *multiTermTerms) HasPayloads() bool         { return false }
func (m *multiTermTerms) GetDocCount() (int, error) { return 0, nil }

type multiTermTermsEnum struct {
	owner *multiTermTerms
	idx   int
	cur   *Term
}

func (e *multiTermTermsEnum) Next() (*Term, error) {
	e.idx++
	if e.idx >= len(e.owner.entries) {
		e.cur = nil
		return nil, nil
	}
	e.cur = NewTerm("body", e.owner.entries[e.idx].text)
	return e.cur, nil
}

func (e *multiTermTermsEnum) SeekCeil(target *Term) (*Term, error) {
	for i := 0; i < len(e.owner.entries); i++ {
		candidate := NewTerm("body", e.owner.entries[i].text)
		if util.BytesRefCompare(candidate.Bytes, target.Bytes) >= 0 {
			e.idx = i
			e.cur = candidate
			return candidate, nil
		}
	}
	e.idx = len(e.owner.entries)
	e.cur = nil
	return nil, nil
}

func (e *multiTermTermsEnum) SeekExact(target *Term) (bool, error) {
	got, err := e.SeekCeil(target)
	if err != nil || got == nil {
		return false, err
	}
	return util.BytesRefCompare(got.Bytes, target.Bytes) == 0, nil
}

func (e *multiTermTermsEnum) Term() *Term { return e.cur }

func (e *multiTermTermsEnum) DocFreq() (int, error) {
	if e.idx < 0 || e.idx >= len(e.owner.entries) {
		return 0, nil
	}
	return len(e.owner.entries[e.idx].docs), nil
}

func (e *multiTermTermsEnum) TotalTermFreq() (int64, error) {
	c, err := e.DocFreq()
	return int64(c), err
}

func (e *multiTermTermsEnum) Postings(_ int) (PostingsEnum, error) {
	if e.idx < 0 || e.idx >= len(e.owner.entries) {
		return &EmptyPostingsEnum{}, nil
	}
	docsCopy := append([]int(nil), e.owner.entries[e.idx].docs...)
	return &slicePostingsEnum{docs: docsCopy, idx: -1}, nil
}

func (e *multiTermTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (PostingsEnum, error) {
	return e.Postings(0)
}

// slicePostingsEnum is a tiny PostingsEnum backed by a sorted doc-id slice.
type slicePostingsEnum struct {
	docs []int
	idx  int
}

func (s *slicePostingsEnum) DocID() int {
	if s.idx < 0 || s.idx >= len(s.docs) {
		return util.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}

func (s *slicePostingsEnum) NextDoc() (int, error) {
	s.idx++
	return s.DocID(), nil
}

func (s *slicePostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := s.NextDoc()
		if err != nil || d == util.NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}

func (s *slicePostingsEnum) Freq() (int, error)         { return 1, nil }
func (s *slicePostingsEnum) NextPosition() (int, error) { return -1, nil }
func (s *slicePostingsEnum) StartOffset() (int, error)  { return -1, nil }
func (s *slicePostingsEnum) EndOffset() (int, error)    { return -1, nil }
func (s *slicePostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}
func (s *slicePostingsEnum) Cost() int64 { return int64(len(s.docs)) }
