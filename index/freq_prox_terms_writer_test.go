// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// reverseDocMap is a SorterDocMap that reverses the document order. Used
// by SortingDocsEnum / SortingPostingsEnum tests to demonstrate that the
// produced enumerator iterates the postings in the re-mapped order.
type reverseDocMap struct{ size int }

func (m reverseDocMap) OldToNew(old int) int { return m.size - 1 - old }
func (m reverseDocMap) NewToOld(neu int) int { return m.size - 1 - neu }
func (m reverseDocMap) Size() int            { return m.size }

// staticDocsEnum is a minimal PostingsEnum that walks a fixed list of
// docIDs without positions. It is used as the source for the SortingDocsEnum
// tests so the input is deterministic.
type staticDocsEnum struct {
	PostingsEnumBase
	docs []int
	idx  int
}

func newStaticDocsEnum(docs []int) *staticDocsEnum {
	return &staticDocsEnum{
		PostingsEnumBase: PostingsEnumBase{currentDoc: -1},
		docs:             docs,
		idx:              -1,
	}
}

func (s *staticDocsEnum) NextDoc() (int, error) {
	s.idx++
	if s.idx >= len(s.docs) {
		s.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.currentDoc = s.docs[s.idx]
	return s.currentDoc, nil
}

func (s *staticDocsEnum) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

func (s *staticDocsEnum) Freq() (int, error)          { return 1, nil }
func (s *staticDocsEnum) NextPosition() (int, error)  { return -1, nil }
func (s *staticDocsEnum) StartOffset() (int, error)   { return -1, nil }
func (s *staticDocsEnum) EndOffset() (int, error)     { return -1, nil }
func (s *staticDocsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (s *staticDocsEnum) Cost() int64                 { return int64(len(s.docs)) }

func TestSortingDocsEnum_ReversesDocOrder(t *testing.T) {
	in := newStaticDocsEnum([]int{0, 2, 5, 7})
	enum := NewSortingDocsEnum()
	if err := enum.Reset(reverseDocMap{size: 8}, in); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Original docs {0,2,5,7} mapped to {7,5,2,0} then sorted asc gives
	// {0,2,5,7} — proves the sort respects the remapped IDs.
	var got []int
	for {
		doc, err := enum.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		got = append(got, doc)
	}
	want := []int{0, 2, 5, 7}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("doc %d = %d, want %d (got %v)", i, got[i], want[i], got)
		}
	}
}

func TestSortingDocsEnum_PreservesUnsortedInputViaRemapAndSort(t *testing.T) {
	// docMap maps 0->3, 1->2, 2->1, 3->0 -> after sorting we expect
	// {0,1,2,3}.
	in := newStaticDocsEnum([]int{0, 1, 2, 3})
	enum := NewSortingDocsEnum()
	if err := enum.Reset(reverseDocMap{size: 4}, in); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	var got []int
	for {
		doc, err := enum.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		got = append(got, doc)
	}
	if !sort.IntsAreSorted(got) {
		t.Fatalf("expected sorted docs, got %v", got)
	}
}

// staticPostingsEnum is the docs+positions source for SortingPostingsEnum
// tests. Each per-doc payload is given as a slice of (pos, payload) pairs;
// the wire format matches what Lucene's FreqProx pool produces.
type staticPostingsEnum struct {
	PostingsEnumBase
	docs []staticDoc
	idx  int
	pidx int
}

type staticDoc struct {
	docID int
	posts []staticPos
}

type staticPos struct {
	pos     int
	start   int
	end     int
	payload []byte
}

func newStaticPostingsEnum(docs []staticDoc) *staticPostingsEnum {
	return &staticPostingsEnum{
		PostingsEnumBase: PostingsEnumBase{currentDoc: -1},
		docs:             docs,
		idx:              -1,
	}
}

func (s *staticPostingsEnum) NextDoc() (int, error) {
	s.idx++
	if s.idx >= len(s.docs) {
		s.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.currentDoc = s.docs[s.idx].docID
	s.pidx = -1
	return s.currentDoc, nil
}

func (s *staticPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

func (s *staticPostingsEnum) Freq() (int, error) { return len(s.docs[s.idx].posts), nil }

func (s *staticPostingsEnum) NextPosition() (int, error) {
	s.pidx++
	return s.docs[s.idx].posts[s.pidx].pos, nil
}

func (s *staticPostingsEnum) StartOffset() (int, error) {
	return s.docs[s.idx].posts[s.pidx].start, nil
}

func (s *staticPostingsEnum) EndOffset() (int, error) {
	return s.docs[s.idx].posts[s.pidx].end, nil
}

func (s *staticPostingsEnum) GetPayload() ([]byte, error) {
	return s.docs[s.idx].posts[s.pidx].payload, nil
}

func (s *staticPostingsEnum) Cost() int64 { return int64(len(s.docs)) }

func TestSortingPostingsEnum_ReplaysPositionsInRemappedOrder(t *testing.T) {
	src := newStaticPostingsEnum([]staticDoc{
		{docID: 0, posts: []staticPos{{pos: 5}, {pos: 9}}},
		{docID: 1, posts: []staticPos{{pos: 1}}},
		{docID: 2, posts: []staticPos{{pos: 7}, {pos: 11}, {pos: 13}}},
	})
	enum := NewSortingPostingsEnum()
	if err := enum.Reset(reverseDocMap{size: 3}, src, true, false); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	// reverseDocMap turns {0,1,2} into {2,1,0}; the sort restores {0,1,2}
	// but with the original docs' posting data: doc 0 (new) = doc 2 (old).
	type expectedDoc struct {
		docID int
		freq  int
		pos   []int
	}
	expected := []expectedDoc{
		{docID: 0, freq: 3, pos: []int{7, 11, 13}},
		{docID: 1, freq: 1, pos: []int{1}},
		{docID: 2, freq: 2, pos: []int{5, 9}},
	}
	for _, want := range expected {
		got, err := enum.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if got != want.docID {
			t.Fatalf("docID = %d, want %d", got, want.docID)
		}
		freq, err := enum.Freq()
		if err != nil {
			t.Fatalf("Freq: %v", err)
		}
		if freq != want.freq {
			t.Fatalf("doc %d freq = %d, want %d", got, freq, want.freq)
		}
		for i := 0; i < freq; i++ {
			pos, err := enum.NextPosition()
			if err != nil {
				t.Fatalf("doc %d NextPosition: %v", got, err)
			}
			if pos != want.pos[i] {
				t.Fatalf("doc %d pos[%d] = %d, want %d", got, i, pos, want.pos[i])
			}
		}
	}
	last, _ := enum.NextDoc()
	if last != NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", last)
	}
}

func TestSortingPostingsEnum_HandlesPayloadsAndOffsets(t *testing.T) {
	src := newStaticPostingsEnum([]staticDoc{
		{
			docID: 0,
			posts: []staticPos{
				{pos: 1, start: 0, end: 4, payload: []byte("hi")},
				{pos: 5, start: 10, end: 14, payload: nil},
			},
		},
		{
			docID: 1,
			posts: []staticPos{
				{pos: 2, start: 0, end: 3, payload: []byte("xy")},
			},
		},
	})
	enum := NewSortingPostingsEnum()
	if err := enum.Reset(reverseDocMap{size: 2}, src, true, true); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	// Expected doc 0 (new) = old doc 1 (single position with payload "xy").
	got, _ := enum.NextDoc()
	if got != 0 {
		t.Fatalf("first new doc = %d, want 0", got)
	}
	pos, _ := enum.NextPosition()
	if pos != 2 {
		t.Fatalf("first pos = %d, want 2", pos)
	}
	if so, _ := enum.StartOffset(); so != 0 {
		t.Fatalf("first startOffset = %d, want 0", so)
	}
	if eo, _ := enum.EndOffset(); eo != 3 {
		t.Fatalf("first endOffset = %d, want 3", eo)
	}
	if p, _ := enum.GetPayload(); string(p) != "xy" {
		t.Fatalf("first payload = %q, want xy", p)
	}
	// Doc 1 (new) = old doc 0.
	got, _ = enum.NextDoc()
	if got != 1 {
		t.Fatalf("second new doc = %d, want 1", got)
	}
	pos, _ = enum.NextPosition()
	if pos != 1 {
		t.Fatalf("second pos[0] = %d, want 1", pos)
	}
	if p, _ := enum.GetPayload(); string(p) != "hi" {
		t.Fatalf("second pos[0] payload = %q, want hi", p)
	}
	pos, _ = enum.NextPosition()
	if pos != 5 {
		t.Fatalf("second pos[1] = %d, want 5", pos)
	}
	if p, _ := enum.GetPayload(); len(p) != 0 {
		t.Fatalf("second pos[1] payload = %q, want empty", p)
	}
}

// captureFieldsConsumer collects every (field, terms) pair handed to Write so
// the FreqProxTermsWriter.Flush test can verify the dispatch order.
type captureFieldsConsumer struct {
	fields []string
	closed bool
	failOn string
}

func (c *captureFieldsConsumer) Write(field string, _ Terms) error {
	if field == c.failOn {
		return errors.New("synthetic failure")
	}
	c.fields = append(c.fields, field)
	return nil
}

func (c *captureFieldsConsumer) Close() error { c.closed = true; return nil }

// stubPostingsFormat returns a captureFieldsConsumer.
type stubPostingsFormat struct{ consumer *captureFieldsConsumer }

func (s *stubPostingsFormat) Name() string { return "stub" }
func (s *stubPostingsFormat) FieldsConsumer(*SegmentWriteState) (FieldsConsumer, error) {
	return s.consumer, nil
}
func (s *stubPostingsFormat) FieldsProducer(*SegmentReadState) (FieldsProducer, error) {
	return nil, errors.New("stub: not implemented")
}

// newFreqProxFieldInfoWithNumber mirrors newFreqProxFieldInfo from
// freq_prox_terms_writer_per_field_test.go but lets the caller supply a
// distinct field number so multi-field tests can register several fields
// inside the same FieldInfos.
func newFreqProxFieldInfoWithNumber(name string, number int, opts IndexOptions) *FieldInfo {
	o := DefaultFieldInfoOptions()
	o.IndexOptions = opts
	return NewFieldInfo(name, number, o)
}

func newSegmentWriteStateWithFields(t *testing.T, fields ...*FieldInfo) *SegmentWriteState {
	t.Helper()
	fi := NewFieldInfos()
	for _, f := range fields {
		if err := fi.Add(f); err != nil {
			t.Fatalf("Add(%q): %v", f.Name(), err)
		}
	}
	fi.Freeze()
	return &SegmentWriteState{FieldInfos: fi}
}

func TestFreqProxTermsWriter_FlushFreqProx_DispatchesSortedByFieldName(t *testing.T) {
	pools := freshFreqProxPools()
	writer, err := NewFreqProxTermsWriter(pools, nil)
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}

	number := 0
	mkField := func(name string) (*FreqProxTermsWriterPerField, *FieldInfo) {
		fi := newFreqProxFieldInfoWithNumber(name, number, IndexOptionsDocs)
		number++
		state := NewFieldInvertState(10, name, IndexOptionsDocs)
		w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, FreqProxAttributeProvider{TermFrequency: func() int { return 1 }})
		if err != nil {
			t.Fatalf("per-field %q: %v", name, err)
		}
		if err := w.Add(util.NewBytesRef([]byte("token")), 0); err != nil {
			t.Fatalf("Add %q: %v", name, err)
		}
		return w, fi
	}

	wb, fib := mkField("bravo")
	wa, fia := mkField("alpha")
	wc, fic := mkField("charlie")

	state := newSegmentWriteStateWithFields(t, fia, fib, fic)
	cap := &captureFieldsConsumer{}
	pf := &stubPostingsFormat{consumer: cap}

	flushed := map[string]*FreqProxTermsWriterPerField{
		"bravo":   wb,
		"alpha":   wa,
		"charlie": wc,
	}
	if err := writer.FlushFreqProx(flushed, state, nil, pf, nil); err != nil {
		t.Fatalf("FlushFreqProx: %v", err)
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(cap.fields) != len(want) {
		t.Fatalf("dispatched fields = %v, want %v", cap.fields, want)
	}
	for i := range want {
		if cap.fields[i] != want[i] {
			t.Fatalf("dispatched[%d] = %q, want %q (got %v)", i, cap.fields[i], want[i], cap.fields)
		}
	}
	if !cap.closed {
		t.Fatalf("consumer.Close was not called")
	}
}

func TestFreqProxTermsWriter_FlushFreqProx_SkipsEmptyFields(t *testing.T) {
	pools := freshFreqProxPools()
	writer, err := NewFreqProxTermsWriter(pools, nil)
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}

	emptyFI := newFreqProxFieldInfoWithNumber("empty", 0, IndexOptionsDocs)
	emptyState := NewFieldInvertState(10, "empty", IndexOptionsDocs)
	emptyPer, err := NewFreqProxTermsWriterPerField(emptyState, freshFreqProxPools(), emptyFI, nil, FreqProxAttributeProvider{TermFrequency: func() int { return 1 }})
	if err != nil {
		t.Fatalf("empty per-field: %v", err)
	}

	activeFI := newFreqProxFieldInfoWithNumber("active", 1, IndexOptionsDocs)
	activeState := NewFieldInvertState(10, "active", IndexOptionsDocs)
	activePer, err := NewFreqProxTermsWriterPerField(activeState, freshFreqProxPools(), activeFI, nil, FreqProxAttributeProvider{TermFrequency: func() int { return 1 }})
	if err != nil {
		t.Fatalf("active per-field: %v", err)
	}
	if err := activePer.Add(util.NewBytesRef([]byte("hit")), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	state := newSegmentWriteStateWithFields(t, emptyFI, activeFI)
	cap := &captureFieldsConsumer{}
	pf := &stubPostingsFormat{consumer: cap}

	flushed := map[string]*FreqProxTermsWriterPerField{
		"empty":  emptyPer,
		"active": activePer,
	}
	if err := writer.FlushFreqProx(flushed, state, nil, pf, nil); err != nil {
		t.Fatalf("FlushFreqProx: %v", err)
	}
	if len(cap.fields) != 1 || cap.fields[0] != "active" {
		t.Fatalf("dispatched = %v, want [active]", cap.fields)
	}
}

func TestFreqProxTermsWriter_FlushFreqProx_NoPostingsShortCircuits(t *testing.T) {
	pools := freshFreqProxPools()
	writer, err := NewFreqProxTermsWriter(pools, nil)
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}
	// An empty FieldInfos reports HasPostings()==false; the writer must
	// return early without invoking the consumer.
	state := newSegmentWriteStateWithFields(t)
	cap := &captureFieldsConsumer{}
	pf := &stubPostingsFormat{consumer: cap}

	if err := writer.FlushFreqProx(nil, state, nil, pf, nil); err != nil {
		t.Fatalf("FlushFreqProx: %v", err)
	}
	if len(cap.fields) != 0 {
		t.Fatalf("expected no fields dispatched, got %v", cap.fields)
	}
	if cap.closed {
		t.Fatalf("consumer.Close should not be invoked when no postings exist")
	}
}

func TestFreqProxTermsWriter_FlushFreqProx_PropagatesConsumerError(t *testing.T) {
	pools := freshFreqProxPools()
	writer, err := NewFreqProxTermsWriter(pools, nil)
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)
	per, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, FreqProxAttributeProvider{TermFrequency: func() int { return 1 }})
	if err != nil {
		t.Fatalf("per-field: %v", err)
	}
	if err := per.Add(util.NewBytesRef([]byte("w")), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	swState := newSegmentWriteStateWithFields(t, fi)
	cap := &captureFieldsConsumer{failOn: "body"}
	pf := &stubPostingsFormat{consumer: cap}

	flushed := map[string]*FreqProxTermsWriterPerField{"body": per}
	err = writer.FlushFreqProx(flushed, swState, nil, pf, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !cap.closed {
		t.Fatalf("consumer.Close must run even on write error")
	}
}

// stubNextHandler is a FreqProxNextHandler used by AddField wiring tests.
type stubNextHandler struct {
	addCalls    int
	flushCalled bool
	failAdd     bool
}

func (s *stubNextHandler) AddField(invertState *FieldInvertState, fi *FieldInfo) (*TermsHashPerField, error) {
	if s.failAdd {
		return nil, errors.New("stub: AddField rejected")
	}
	s.addCalls++
	pools := freshFreqProxPools()
	base, err := NewTermsHashPerField(
		1,
		pools.IntPool,
		pools.BytePool,
		pools.TermBytePool,
		pools.BytesUsed,
		nil,
		fi.Name(),
		fi.IndexOptions(),
		TermsHashPerFieldHooks{
			NewTerm:             func(int, int) error { return nil },
			AddTerm:             func(int, int) error { return nil },
			NewPostingsArray:    func() {},
			CreatePostingsArray: func(size int) *ParallelPostingsArray { return NewParallelPostingsArray(size) },
		},
	)
	if err != nil {
		return nil, err
	}
	return base, nil
}

func (s *stubNextHandler) Flush(_ map[string]*TermsHashPerField, _ *SegmentWriteState, _ SorterDocMap) error {
	s.flushCalled = true
	return nil
}

func TestFreqProxTermsWriter_AddField_WiresNextHandler(t *testing.T) {
	pools := freshFreqProxPools()
	next := &stubNextHandler{}
	writer, err := NewFreqProxTermsWriter(pools, next)
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)
	per, err := writer.AddField(state, fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if per == nil {
		t.Fatalf("AddField returned nil per-field")
	}
	if next.addCalls != 1 {
		t.Fatalf("next.addCalls = %d, want 1", next.addCalls)
	}
	if per.NextPerField == nil {
		t.Fatalf("expected NextPerField wired through stub handler")
	}
}

func TestFreqProxTermsWriter_AddField_PropagatesNextHandlerError(t *testing.T) {
	pools := freshFreqProxPools()
	writer, err := NewFreqProxTermsWriter(pools, &stubNextHandler{failAdd: true})
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriter: %v", err)
	}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)
	if _, err := writer.AddField(state, fi); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNewFreqProxTermsWriter_RejectsNilPools(t *testing.T) {
	if _, err := NewFreqProxTermsWriter(FreqProxTermsHash{}, nil); err == nil {
		t.Fatalf("expected error for nil pools, got nil")
	}
}

func TestSortingTerms_PassThroughMetadata(t *testing.T) {
	terms := NewSingleTermTerms(NewTerm("f", "hello"), 1, 1)
	sorted := NewSortingTerms(terms, IndexOptionsDocsAndFreqs, reverseDocMap{size: 1})
	if sorted.Size() != 1 {
		t.Fatalf("Size = %d, want 1", sorted.Size())
	}
	if got, _ := sorted.GetMin(); got == nil || got.Text() != "hello" {
		t.Fatalf("GetMin = %v, want hello", got)
	}
}
