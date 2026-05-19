// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// freqProxStubAttrs mirrors a single token's analysis attributes. Tests
// mutate it between Add calls and the writer reads through the function
// fields wired to its methods.
type freqProxStubAttrs struct {
	freq    int
	hasFreq bool
	start   int
	end     int
	payload *util.BytesRef
}

func (s *freqProxStubAttrs) provider() FreqProxAttributeProvider {
	return FreqProxAttributeProvider{
		TermFrequency:        func() int { return s.freq },
		HasTermFreqAttribute: func() bool { return s.hasFreq },
		StartOffset:          func() int { return s.start },
		EndOffset:            func() int { return s.end },
		Payload:              func() *util.BytesRef { return s.payload },
	}
}

func freshFreqProxPools() FreqProxTermsHash {
	bytePool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(util.NewCounter()))
	bytePool.NextBuffer()
	termPool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(util.NewCounter()))
	termPool.NextBuffer()
	return FreqProxTermsHash{
		IntPool:      util.NewIntBlockPool(),
		BytePool:     bytePool,
		TermBytePool: termPool,
		BytesUsed:    util.NewCounter(),
	}
}

func newFreqProxFieldInfo(name string, opts IndexOptions) *FieldInfo {
	o := DefaultFieldInfoOptions()
	o.IndexOptions = opts
	return NewFieldInfo(name, 0, o)
}

func TestFreqProxTermsWriterPerField_DocsOnlyTracksUniqueTerms(t *testing.T) {
	stub := &freqProxStubAttrs{freq: 1}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)

	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}

	term := util.NewBytesRef([]byte("hello"))
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add doc 0: %v", err)
	}
	if err := w.Add(term, 1); err != nil {
		t.Fatalf("Add doc 1 (same term): %v", err)
	}
	other := util.NewBytesRef([]byte("world"))
	if err := w.Add(other, 1); err != nil {
		t.Fatalf("Add other term: %v", err)
	}

	if got, want := state.UniqueTermCount(), 3; got != want {
		t.Fatalf("UniqueTermCount = %d, want %d", got, want)
	}
	if got, want := state.MaxTermFrequency(), 1; got != want {
		t.Fatalf("MaxTermFrequency = %d, want %d (docs-only field treats freq as 1)", got, want)
	}
	if w.postingsArray == nil {
		t.Fatalf("postingsArray was not wired by createPostingsArray hook")
	}
	if w.postingsArray.TermFreqs != nil {
		t.Fatalf("docs-only field should not allocate TermFreqs")
	}
}

func TestFreqProxTermsWriterPerField_FreqIncrementsOnSameDoc(t *testing.T) {
	stub := &freqProxStubAttrs{freq: 1, hasFreq: true}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocsAndFreqs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)

	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}

	term := util.NewBytesRef([]byte("hello"))
	for i := 0; i < 3; i++ {
		if err := w.Add(term, 0); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	if got := w.postingsArray.TermFreqs[0]; got != 3 {
		t.Fatalf("TermFreqs[0] = %d, want 3", got)
	}
	if got := state.MaxTermFrequency(); got != 3 {
		t.Fatalf("MaxTermFrequency = %d, want 3", got)
	}
}

func TestFreqProxTermsWriterPerField_PayloadFlipsStorePayloads(t *testing.T) {
	stub := &freqProxStubAttrs{freq: 1, hasFreq: true}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocsAndFreqsAndPositions)
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqsAndPositions)

	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}

	term := util.NewBytesRef([]byte("hello"))
	state.SetPosition(0)
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add token 0: %v", err)
	}
	if fi.HasStoredPayloads() {
		t.Fatalf("HasStoredPayloads should still be false before any payload is observed")
	}
	stub.payload = util.NewBytesRef([]byte{0xAB, 0xCD})
	state.SetPosition(1)
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add token 1: %v", err)
	}
	if !w.sawPayloads {
		t.Fatalf("sawPayloads should flip after a non-empty payload")
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if !fi.HasStoredPayloads() {
		t.Fatalf("FieldInfo.HasStoredPayloads should be true after Finish observed a payload")
	}
}

func TestFreqProxTermsWriterPerField_OffsetGuardsBackwards(t *testing.T) {
	stub := &freqProxStubAttrs{freq: 1, hasFreq: true, start: 10, end: 15}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqsAndPositionsAndOffsets)

	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}

	term := util.NewBytesRef([]byte("hello"))
	state.SetPosition(0)
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add token 0: %v", err)
	}
	// Second occurrence with a backwards startOffset must panic, matching
	// Lucene's `assert startOffset - lastOffsets >= 0`.
	stub.start = 5
	stub.end = 6
	state.SetPosition(1)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Add with backwards startOffset should have panicked")
		}
	}()
	_ = w.Add(term, 0)
}

func TestFreqProxTermsWriterPerField_CustomFreqRequiresFreqIndexing(t *testing.T) {
	// Custom TermFrequencyAttribute (freq != 1) on a docs-only field must
	// surface the contract violation as an error from Add. Mirrors Lucene's
	// "must index term freq while using custom TermFrequencyAttribute".
	stub := &freqProxStubAttrs{freq: 5, hasFreq: true}
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)

	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}

	term := util.NewBytesRef([]byte("hello"))
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add (newTerm path): %v", err)
	}
	// Re-add on the next doc to take the addTerm path with hasFreq=false.
	if err := w.Add(term, 1); err == nil {
		t.Fatalf("Add should reject custom TermFrequencyAttribute when freq is not indexed")
	}
}

func TestFreqProxTermsWriterPerField_RejectsBadConstructorInputs(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)
	fi := newFreqProxFieldInfo("body", IndexOptionsDocs)
	none := newFreqProxFieldInfo("body", IndexOptionsNone)
	pools := freshFreqProxPools()
	provider := (&freqProxStubAttrs{}).provider()

	if _, err := NewFreqProxTermsWriterPerField(nil, pools, fi, nil, provider); err == nil {
		t.Fatalf("nil invertState should be rejected")
	}
	if _, err := NewFreqProxTermsWriterPerField(state, pools, nil, nil, provider); err == nil {
		t.Fatalf("nil fieldInfo should be rejected")
	}
	if _, err := NewFreqProxTermsWriterPerField(state, FreqProxTermsHash{}, fi, nil, provider); err == nil {
		t.Fatalf("zero TermsHash bundle should be rejected")
	}
	if _, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), none, nil, provider); err == nil {
		t.Fatalf("IndexOptionsNone field should be rejected")
	}
}
