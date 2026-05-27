// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// SlowImpactsEnum
// ---------------------------------------------------------------------------

// simplePostingsEnum provides a minimal PostingsEnum over a fixed doc list.
type simplePostingsEnum struct {
	docs  []int
	idx   int
	docID int
}

func newSimplePostingsEnum(docs ...int) *simplePostingsEnum {
	return &simplePostingsEnum{docs: docs, idx: -1, docID: -1}
}

func (e *simplePostingsEnum) NextDoc() (int, error) {
	e.idx++
	if e.idx >= len(e.docs) {
		e.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	e.docID = e.docs[e.idx]
	return e.docID, nil
}
func (e *simplePostingsEnum) DocID() int { return e.docID }
func (e *simplePostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := e.NextDoc()
		if err != nil || d == NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (e *simplePostingsEnum) Freq() (int, error)          { return 1, nil }
func (e *simplePostingsEnum) NextPosition() (int, error)  { return 0, nil }
func (e *simplePostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *simplePostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *simplePostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (e *simplePostingsEnum) Cost() int64                 { return int64(len(e.docs)) }

// TestSlowImpactsEnum_DelegatesDocIteration verifies that SlowImpactsEnum
// correctly delegates NextDoc/Advance/DocID to its wrapped PostingsEnum.
func TestSlowImpactsEnum_DelegatesDocIteration(t *testing.T) {
	inner := newSimplePostingsEnum(1, 3, 7)
	sie := NewSlowImpactsEnum(inner)

	doc, err := sie.NextDoc()
	if err != nil || doc != 1 {
		t.Fatalf("want doc=1, got doc=%d err=%v", doc, err)
	}
	doc, err = sie.NextDoc()
	if err != nil || doc != 3 {
		t.Fatalf("want doc=3, got doc=%d err=%v", doc, err)
	}
	doc, err = sie.Advance(7)
	if err != nil || doc != 7 {
		t.Fatalf("want doc=7 after Advance(7), got doc=%d err=%v", doc, err)
	}
	doc, err = sie.NextDoc()
	if err != nil || doc != NO_MORE_DOCS {
		t.Fatalf("want NO_MORE_DOCS, got doc=%d err=%v", doc, err)
	}
}

// TestSlowImpactsEnum_GetImpacts verifies the single-level Impacts with
// freq=MaxInt32 and norm=1, matching Lucene's SlowImpactsEnum.getImpacts().
func TestSlowImpactsEnum_GetImpacts(t *testing.T) {
	inner := newSimplePostingsEnum(0, 1)
	sie := NewSlowImpactsEnum(inner)

	impacts, err := sie.GetImpacts()
	if err != nil {
		t.Fatalf("GetImpacts error: %v", err)
	}
	if n := impacts.NumLevels(); n != 1 {
		t.Fatalf("want NumLevels=1, got %d", n)
	}
	if up := impacts.GetDocIDUpTo(0); up != NO_MORE_DOCS {
		t.Fatalf("want GetDocIDUpTo=NO_MORE_DOCS, got %d", up)
	}
	buf := impacts.GetImpacts(0)
	if buf.Size != 1 {
		t.Fatalf("want buf.Size=1, got %d", buf.Size)
	}
	if buf.Freqs[0] != math.MaxInt32 {
		t.Fatalf("want freq=MaxInt32, got %d", buf.Freqs[0])
	}
	if buf.Norms[0] != 1 {
		t.Fatalf("want norm=1, got %d", buf.Norms[0])
	}
}

// TestSlowImpactsEnum_AdvanceShallow verifies that AdvanceShallow is a no-op.
func TestSlowImpactsEnum_AdvanceShallow(t *testing.T) {
	inner := newSimplePostingsEnum(0)
	sie := NewSlowImpactsEnum(inner)
	if err := sie.AdvanceShallow(100); err != nil {
		t.Fatalf("AdvanceShallow must be no-op, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// MultiPostingsEnum
// ---------------------------------------------------------------------------

// TestMultiPostingsEnum_BasicIteration verifies NextDoc across two subs with
// base offsets applied correctly.
func TestMultiPostingsEnum_BasicIteration(t *testing.T) {
	// sub0: docs [0, 2] in a slice starting at 0
	// sub1: docs [0, 1] in a slice starting at 10 → composite [10, 11]
	sub0 := newSimplePostingsEnum(0, 2)
	sub1 := newSimplePostingsEnum(0, 1)

	mte := &MultiTermsEnum{} // parent token — just needs identity
	mpe := NewMultiPostingsEnum(mte, 2)

	subs := []EnumWithSlice{
		{PostingsEnum: sub0, Slice: ReaderSlice{Start: 0, Length: 5, ReaderIndex: 0}},
		{PostingsEnum: sub1, Slice: ReaderSlice{Start: 10, Length: 5, ReaderIndex: 1}},
	}
	mpe.Reset(subs, 2)

	want := []int{0, 2, 10, 11}
	for _, w := range want {
		doc, err := mpe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc != w {
			t.Fatalf("want doc=%d, got %d", w, doc)
		}
	}
	doc, err := mpe.NextDoc()
	if err != nil || doc != NO_MORE_DOCS {
		t.Fatalf("want NO_MORE_DOCS, got doc=%d err=%v", doc, err)
	}
}

// TestMultiPostingsEnum_Advance verifies Advance finds the correct composite doc.
func TestMultiPostingsEnum_Advance(t *testing.T) {
	sub0 := newSimplePostingsEnum(0, 1, 2)
	sub1 := newSimplePostingsEnum(0, 1) // base 10 → composite 10, 11

	mte := &MultiTermsEnum{}
	mpe := NewMultiPostingsEnum(mte, 2)
	subs := []EnumWithSlice{
		{PostingsEnum: sub0, Slice: ReaderSlice{Start: 0}},
		{PostingsEnum: sub1, Slice: ReaderSlice{Start: 10}},
	}
	mpe.Reset(subs, 2)

	doc, err := mpe.Advance(10)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if doc != 10 {
		t.Fatalf("want doc=10 after Advance(10), got %d", doc)
	}
}

// TestMultiPostingsEnum_Cost verifies that Cost sums across active subs.
func TestMultiPostingsEnum_Cost(t *testing.T) {
	sub0 := newSimplePostingsEnum(0, 1, 2) // cost 3
	sub1 := newSimplePostingsEnum(0, 1)    // cost 2

	mte := &MultiTermsEnum{}
	mpe := NewMultiPostingsEnum(mte, 2)
	subs := []EnumWithSlice{
		{PostingsEnum: sub0, Slice: ReaderSlice{}},
		{PostingsEnum: sub1, Slice: ReaderSlice{}},
	}
	mpe.Reset(subs, 2)

	if got := mpe.Cost(); got != 5 {
		t.Fatalf("want Cost=5, got %d", got)
	}
}

// TestMultiPostingsEnum_CanReuse verifies the parent-identity check.
func TestMultiPostingsEnum_CanReuse(t *testing.T) {
	parent := &MultiTermsEnum{}
	other := &MultiTermsEnum{}
	mpe := NewMultiPostingsEnum(parent, 0)
	mpe.Reset(nil, 0)

	if !mpe.CanReuse(parent) {
		t.Fatal("CanReuse must be true for own parent")
	}
	if mpe.CanReuse(other) {
		t.Fatal("CanReuse must be false for different parent")
	}
}

// ReaderManager tests live in phase8_reader_manager_test.go (package index_test)
// because they require NewIndexWriterConfig(analysis.Analyzer) which imports
// the analysis package — not available from within the index package itself.

// ---------------------------------------------------------------------------
// SimpleMergedSegmentWarmer
// ---------------------------------------------------------------------------

// TestSimpleMergedSegmentWarmer_WarmNoPanic verifies that Warm runs without
// panicking on a stub LeafReader that returns empty data for every accessor.
func TestSimpleMergedSegmentWarmer_WarmNoPanic(t *testing.T) {
	warmer := NewSimpleMergedSegmentWarmer(util.NoOpInfoStream)
	reader := NewLeafReaderWithFieldInfos(
		NewSegmentInfo("test", 0, nil),
		NewFieldInfos(),
	)
	if err := warmer.Warm(reader); err != nil {
		t.Fatalf("Warm returned unexpected error: %v", err)
	}
}

// TestSimpleMergedSegmentWarmer_NilInfoStream verifies that nil infoStream is
// silently replaced by NoOpInfoStream (no panic on warm).
func TestSimpleMergedSegmentWarmer_NilInfoStream(t *testing.T) {
	warmer := NewSimpleMergedSegmentWarmer(nil)
	reader := NewLeafReaderWithFieldInfos(NewSegmentInfo("seg", 0, nil), NewFieldInfos())
	if err := warmer.Warm(reader); err != nil {
		t.Fatalf("Warm with nil infoStream: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SlowCodecReaderWrapper (SlowLeafCodecReader)
// ---------------------------------------------------------------------------

// TestWrapLeafReader_NilInput verifies that WrapLeafReader rejects nil input.
func TestWrapLeafReader_NilInput(t *testing.T) {
	_, err := WrapLeafReader(nil)
	if err == nil {
		t.Fatal("expected error for nil reader, got nil")
	}
}

// TestWrapLeafReader_EmptyReader verifies that the wrapper survives with an
// empty LeafReader and returns nil from every accessor that has nothing to
// return.
func TestWrapLeafReader_EmptyReader(t *testing.T) {
	reader := NewLeafReaderWithFieldInfos(
		NewSegmentInfo("slow_test", 0, nil),
		NewFieldInfos(),
	)
	wrapped, err := WrapLeafReader(reader)
	if err != nil {
		t.Fatalf("WrapLeafReader: %v", err)
	}
	if wrapped == nil {
		t.Fatal("WrapLeafReader returned nil for valid reader")
	}
	// GetDelegate must return the original reader.
	if wrapped.GetDelegate() != reader {
		t.Fatal("GetDelegate must return the original reader")
	}
	// GetFieldInfos must be non-nil (empty).
	fi := wrapped.GetFieldInfos()
	if fi == nil {
		t.Fatal("GetFieldInfos must not return nil for empty reader")
	}
	// GetStoredFieldsReader / GetTermVectorsReader may return nil when the
	// underlying LeafReader's StoredFields/TermVectors calls return errors
	// (e.g. for the base LeafReader stub which requires a subclass override).
	// The important contract is no panic.
	_ = wrapped.GetStoredFieldsReader()
	_ = wrapped.GetTermVectorsReader()
	// GetDocValuesReader must be non-nil.
	if dvr := wrapped.GetDocValuesReader(); dvr == nil {
		t.Fatal("GetDocValuesReader must not return nil")
	}
	// GetPostingsReader must succeed.
	fp, err := wrapped.GetPostingsReader()
	if err != nil {
		t.Fatalf("GetPostingsReader: %v", err)
	}
	if fp == nil {
		t.Fatal("GetPostingsReader returned nil")
	}
}

// ---------------------------------------------------------------------------
// MappedMultiFields
// ---------------------------------------------------------------------------

// TestMappedMultiFields_TermsNilForAbsentField verifies that Terms() returns
// nil for a field not present in any sub-reader.
func TestMappedMultiFields_TermsNilForAbsentField(t *testing.T) {
	subFields := NewMemoryFields()
	multi := NewMultiFields(subFields)
	ms := &MergeState{DocMaps: []DocMap{testIdentDocMap{}}}

	mmf := NewMappedMultiFields(ms, multi)

	terms, err := mmf.Terms("nonexistent")
	if err != nil {
		t.Fatalf("Terms(nonexistent) error: %v", err)
	}
	if terms != nil {
		t.Fatalf("want nil Terms for absent field, got %v", terms)
	}
}

// TestMappedMultiFields_Iterator verifies that Iterator passes through field
// names from the underlying MultiFields.
func TestMappedMultiFields_Iterator(t *testing.T) {
	subFields := NewMemoryFields()
	subFields.AddField("alpha", &EmptyTerms{})
	subFields.AddField("beta", &EmptyTerms{})

	multi := NewMultiFields(subFields)
	ms := &MergeState{DocMaps: []DocMap{testIdentDocMap{}}}

	mmf := NewMappedMultiFields(ms, multi)

	iter, err := mmf.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}
	var names []string
	for {
		name, err := iter.Next()
		if err != nil {
			t.Fatalf("Iterator.Next: %v", err)
		}
		if name == "" {
			break
		}
		names = append(names, name)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("want [alpha, beta], got %v", names)
	}
}

// testIdentDocMap is a DocMap that maps every doc to itself (no deletions).
type testIdentDocMap struct{}

func (testIdentDocMap) Get(docID int) int { return docID }
