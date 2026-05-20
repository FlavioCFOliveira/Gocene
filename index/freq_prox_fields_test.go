// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// freqProxFieldsHarness wires a FreqProxTermsWriterPerField over the shared
// helpers exposed by freqProxStubAttrs (see freq_prox_terms_writer_per_field_test.go).
// Tests use it to feed a known sequence of (term, docID, position, offsets,
// payload) tuples into the writer before wrapping the writer in a
// FreqProxFields view and exercising the reader.
type freqProxFieldsHarness struct {
	stub  *freqProxStubAttrs
	state *FieldInvertState
	w     *FreqProxTermsWriterPerField
}

func newFreqProxFieldsHarness(t *testing.T, opts IndexOptions) *freqProxFieldsHarness {
	t.Helper()
	stub := &freqProxStubAttrs{freq: 1, hasFreq: opts >= IndexOptionsDocsAndFreqs}
	fi := newFreqProxFieldInfo("body", opts)
	state := NewFieldInvertState(10, "body", opts)
	w, err := NewFreqProxTermsWriterPerField(state, freshFreqProxPools(), fi, nil, stub.provider())
	if err != nil {
		t.Fatalf("NewFreqProxTermsWriterPerField: %v", err)
	}
	return &freqProxFieldsHarness{stub: stub, state: state, w: w}
}

// addToken records one token. position is consumed by writeProx (when prox
// is enabled), startOffset/endOffset by writeOffsets (when offsets are
// enabled). payload may be nil. freq controls the per-token freq attribute
// (callers using DOCS only must always pass freq=1 to honour the writer's
// custom-frequency guard).
func (h *freqProxFieldsHarness) addToken(t *testing.T, term string, docID int, position, startOffset, endOffset, freq int, payload []byte) {
	t.Helper()
	h.stub.freq = freq
	h.stub.start = startOffset
	h.stub.end = endOffset
	if payload != nil {
		h.stub.payload = util.NewBytesRef(payload)
	} else {
		h.stub.payload = nil
	}
	h.state.SetPosition(position)
	// Leave FieldInvertState.Offset() at its zero value: the writer uses it
	// as a per-field-instance running base offset that is added to the
	// per-token attrs.StartOffset(). Single-field tests want the running
	// base to stay at 0 so attrs values are written verbatim.
	bytesRef := util.NewBytesRef([]byte(term))
	if err := h.w.Add(bytesRef, docID); err != nil {
		t.Fatalf("Add(%q, doc=%d, pos=%d): %v", term, docID, position, err)
	}
}

func TestFreqProxFields_FieldsIterationPreservesInputOrder(t *testing.T) {
	a := newFreqProxFieldsHarness(t, IndexOptionsDocs)
	b := newFreqProxFieldsHarness(t, IndexOptionsDocs)
	a.w.fieldInfo = newFreqProxFieldInfo("aaa", IndexOptionsDocs)
	b.w.fieldInfo = newFreqProxFieldInfo("bbb", IndexOptionsDocs)
	// fieldName is taken from the embedded TermsHashPerField, not fieldInfo.
	// Re-create writers with names matching their slot rather than mutating.
	statA := NewFieldInvertState(10, "aaa", IndexOptionsDocs)
	statB := NewFieldInvertState(10, "bbb", IndexOptionsDocs)
	wA, err := NewFreqProxTermsWriterPerField(statA, freshFreqProxPools(),
		newFreqProxFieldInfo("aaa", IndexOptionsDocs), nil,
		(&freqProxStubAttrs{freq: 1}).provider())
	if err != nil {
		t.Fatalf("writer aaa: %v", err)
	}
	wB, err := NewFreqProxTermsWriterPerField(statB, freshFreqProxPools(),
		newFreqProxFieldInfo("bbb", IndexOptionsDocs), nil,
		(&freqProxStubAttrs{freq: 1}).provider())
	if err != nil {
		t.Fatalf("writer bbb: %v", err)
	}
	if err := wA.Add(util.NewBytesRef([]byte("x")), 0); err != nil {
		t.Fatalf("Add aaa: %v", err)
	}
	if err := wB.Add(util.NewBytesRef([]byte("y")), 0); err != nil {
		t.Fatalf("Add bbb: %v", err)
	}

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{wA, wB})
	it, err := fields.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}
	got := make([]string, 0, 2)
	for it.HasNext() {
		name, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if name == "" {
			break
		}
		got = append(got, name)
	}
	want := []string{"aaa", "bbb"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Iterator order = %v, want %v", got, want)
	}

	missing, err := fields.Terms("missing")
	if err != nil {
		t.Fatalf("Terms(missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("Terms(missing) = %v, want nil", missing)
	}
	if fields.Size() != -1 {
		t.Fatalf("Size() = %d, want -1 (unsupported per Lucene)", fields.Size())
	}
}

func TestFreqProxFields_TermsHasFlagsReflectIndexOptions(t *testing.T) {
	cases := []struct {
		name       string
		opts       IndexOptions
		freqs      bool
		positions  bool
		offsets    bool
		expectFlow func(*freqProxFieldsHarness, *testing.T)
	}{
		{
			name: "docs", opts: IndexOptionsDocs,
		},
		{
			name: "docs+freqs", opts: IndexOptionsDocsAndFreqs, freqs: true,
		},
		{
			name: "docs+freqs+pos", opts: IndexOptionsDocsAndFreqsAndPositions, freqs: true, positions: true,
		},
		{
			name: "docs+freqs+pos+offs", opts: IndexOptionsDocsAndFreqsAndPositionsAndOffsets, freqs: true, positions: true, offsets: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newFreqProxFieldsHarness(t, tc.opts)
			h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)

			fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
			terms, err := fields.Terms("body")
			if err != nil {
				t.Fatalf("Terms(body): %v", err)
			}
			if terms == nil {
				t.Fatalf("Terms(body) = nil")
			}
			if got := terms.HasFreqs(); got != tc.freqs {
				t.Fatalf("HasFreqs = %v, want %v", got, tc.freqs)
			}
			if got := terms.HasPositions(); got != tc.positions {
				t.Fatalf("HasPositions = %v, want %v", got, tc.positions)
			}
			if got := terms.HasOffsets(); got != tc.offsets {
				t.Fatalf("HasOffsets = %v, want %v", got, tc.offsets)
			}
			if got := terms.HasPayloads(); got != false {
				t.Fatalf("HasPayloads = %v, want false (no payload observed)", got)
			}
		})
	}
}

func TestFreqProxFields_TermsStatsUnsupported(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocs)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, err := fields.Terms("body")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}

	if _, err := terms.GetDocCount(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("GetDocCount err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if _, err := terms.GetSumDocFreq(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("GetSumDocFreq err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if _, err := terms.GetSumTotalTermFreq(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("GetSumTotalTermFreq err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if _, err := terms.GetMin(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("GetMin err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if _, err := terms.GetMax(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("GetMax err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if terms.Size() != -1 {
		t.Fatalf("Size = %d, want -1", terms.Size())
	}
}

func TestFreqProxTermsEnum_NextIteratesSortedTerms(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	h.addToken(t, "delta", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "charlie", 1, 0, 0, 1, 1, nil)
	h.addToken(t, "bravo", 1, 0, 0, 1, 1, nil)

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	want := []string{"alpha", "bravo", "charlie", "delta"}
	for _, w := range want {
		term, err := enum.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if term == nil {
			t.Fatalf("Next() = nil, want %q", w)
		}
		if got := term.Text(); got != w {
			t.Fatalf("Next() text = %q, want %q", got, w)
		}
	}
	tail, err := enum.Next()
	if err != nil {
		t.Fatalf("Next tail: %v", err)
	}
	if tail != nil {
		t.Fatalf("Next() past end = %v, want nil", tail)
	}
}

func TestFreqProxTermsEnum_SeekCeilAndExact(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	for _, term := range []string{"alpha", "charlie", "echo"} {
		h.addToken(t, term, 0, 0, 0, 1, 1, nil)
	}

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")

	// Found.
	enum, _ := terms.GetIterator()
	concrete := enum.(*FreqProxTermsEnum)
	got, err := concrete.SeekCeil(NewTerm("body", "charlie"))
	if err != nil {
		t.Fatalf("SeekCeil found: %v", err)
	}
	if got == nil || got.Text() != "charlie" {
		t.Fatalf("SeekCeil(charlie) = %v, want charlie", got)
	}
	if concrete.SeekStatus() != SeekStatusFound {
		t.Fatalf("SeekStatus = %v, want Found", concrete.SeekStatus())
	}

	// Not found, lands on next term.
	enum2, _ := terms.GetIterator()
	concrete2 := enum2.(*FreqProxTermsEnum)
	got2, err := concrete2.SeekCeil(NewTerm("body", "bravo"))
	if err != nil {
		t.Fatalf("SeekCeil not-found: %v", err)
	}
	if got2 == nil || got2.Text() != "charlie" {
		t.Fatalf("SeekCeil(bravo) = %v, want charlie (ceiling)", got2)
	}
	if concrete2.SeekStatus() != SeekStatusNotFound {
		t.Fatalf("SeekStatus = %v, want NotFound", concrete2.SeekStatus())
	}

	// Past end.
	enum3, _ := terms.GetIterator()
	concrete3 := enum3.(*FreqProxTermsEnum)
	got3, err := concrete3.SeekCeil(NewTerm("body", "zulu"))
	if err != nil {
		t.Fatalf("SeekCeil end: %v", err)
	}
	if got3 != nil {
		t.Fatalf("SeekCeil(zulu) = %v, want nil", got3)
	}
	if concrete3.SeekStatus() != SeekStatusEnd {
		t.Fatalf("SeekStatus = %v, want End", concrete3.SeekStatus())
	}

	// SeekExact.
	enum4, _ := terms.GetIterator()
	found, err := enum4.SeekExact(NewTerm("body", "alpha"))
	if err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if !found {
		t.Fatalf("SeekExact(alpha) = false, want true")
	}
	miss, err := enum4.SeekExact(NewTerm("body", "bravo"))
	if err != nil {
		t.Fatalf("SeekExact miss: %v", err)
	}
	if miss {
		t.Fatalf("SeekExact(bravo) = true, want false")
	}
}

func TestFreqProxDocsEnum_ReplaysDocsAndFreqs(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	// term "alpha": doc 0 freq 3, doc 5 freq 1
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "alpha", 5, 0, 0, 1, 1, nil)
	// term "bravo": doc 5 freq 2
	h.addToken(t, "bravo", 5, 0, 0, 1, 1, nil)
	h.addToken(t, "bravo", 5, 0, 0, 1, 1, nil)

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact alpha: %v", err)
	}
	posts, err := enum.Postings(postingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	wantDocs := []int{0, 5}
	wantFreqs := []int{3, 1}
	for i, wd := range wantDocs {
		doc, err := posts.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if doc != wd {
			t.Fatalf("NextDoc[%d] = %d, want %d", i, doc, wd)
		}
		freq, err := posts.Freq()
		if err != nil {
			t.Fatalf("Freq[%d]: %v", i, err)
		}
		if freq != wantFreqs[i] {
			t.Fatalf("Freq[%d] = %d, want %d", i, freq, wantFreqs[i])
		}
	}
	end, err := posts.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc end: %v", err)
	}
	if end != NO_MORE_DOCS {
		t.Fatalf("NextDoc end = %d, want NO_MORE_DOCS", end)
	}
}

func TestFreqProxDocsEnum_RejectsFreqsRequestWhenNotIndexed(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocs)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if _, err := enum.Postings(postingsFlagFreqs); err == nil {
		t.Fatalf("Postings(FREQS) should reject docs-only field")
	}
	// FREQS not requested: docs-only enum works.
	posts, err := enum.Postings(0)
	if err != nil {
		t.Fatalf("Postings(0): %v", err)
	}
	doc, err := posts.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("NextDoc = %d, want 0", doc)
	}
	// Freq accessor must fail because freq was not indexed.
	if _, err := posts.Freq(); err == nil {
		t.Fatalf("Freq() should reject docs-only enum")
	}
}

func TestFreqProxPostingsEnum_ReplaysPositionsAndOffsets(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	// term "alpha" in doc 0 at position 0 [10..15], position 3 [30..36]
	h.addToken(t, "alpha", 0, 0, 10, 15, 1, nil)
	h.addToken(t, "alpha", 0, 3, 30, 36, 1, nil)
	// term "alpha" in doc 7 at position 0 [5..10]
	h.addToken(t, "alpha", 7, 0, 5, 10, 1, nil)
	// term "bravo" in doc 7 at position 2 [40..45]
	h.addToken(t, "bravo", 7, 2, 40, 45, 1, nil)

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact alpha: %v", err)
	}
	posts, err := enum.Postings(postingsFlagOffsets)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}

	// doc 0: freq 2, positions {0,[10,15]}, {3,[30,36]}
	if doc, err := posts.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("doc0 NextDoc = %d, err %v", doc, err)
	}
	if freq, _ := posts.Freq(); freq != 2 {
		t.Fatalf("doc0 freq = %d, want 2", freq)
	}
	if pos, err := posts.NextPosition(); err != nil || pos != 0 {
		t.Fatalf("doc0 pos0 = %d, err %v", pos, err)
	}
	if so, _ := posts.StartOffset(); so != 10 {
		t.Fatalf("doc0 pos0 start = %d, want 10", so)
	}
	if eo, _ := posts.EndOffset(); eo != 15 {
		t.Fatalf("doc0 pos0 end = %d, want 15", eo)
	}
	if pos, err := posts.NextPosition(); err != nil || pos != 3 {
		t.Fatalf("doc0 pos1 = %d, err %v", pos, err)
	}
	if so, _ := posts.StartOffset(); so != 30 {
		t.Fatalf("doc0 pos1 start = %d, want 30", so)
	}
	if eo, _ := posts.EndOffset(); eo != 36 {
		t.Fatalf("doc0 pos1 end = %d, want 36", eo)
	}

	// doc 7: freq 1, positions {0,[5,10]}
	if doc, err := posts.NextDoc(); err != nil || doc != 7 {
		t.Fatalf("doc7 NextDoc = %d, err %v", doc, err)
	}
	if freq, _ := posts.Freq(); freq != 1 {
		t.Fatalf("doc7 freq = %d, want 1", freq)
	}
	if pos, err := posts.NextPosition(); err != nil || pos != 0 {
		t.Fatalf("doc7 pos0 = %d, err %v", pos, err)
	}
	if so, _ := posts.StartOffset(); so != 5 {
		t.Fatalf("doc7 pos0 start = %d, want 5", so)
	}
	if eo, _ := posts.EndOffset(); eo != 10 {
		t.Fatalf("doc7 pos0 end = %d, want 10", eo)
	}

	if end, err := posts.NextDoc(); err != nil || end != NO_MORE_DOCS {
		t.Fatalf("doc7 end = %d, err %v", end, err)
	}
}

func TestFreqProxPostingsEnum_PayloadsRoundtrip(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqsAndPositions)
	// doc 0: position 0 with payload {0xAB,0xCD}, position 2 no payload
	h.addToken(t, "alpha", 0, 0, 0, 0, 1, []byte{0xAB, 0xCD})
	h.addToken(t, "alpha", 0, 2, 0, 0, 1, nil)

	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	if !terms.HasPayloads() {
		t.Fatalf("HasPayloads = false, want true")
	}
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	posts, err := enum.Postings(postingsFlagPayloads)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	if doc, err := posts.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("NextDoc = %d, err %v", doc, err)
	}
	if _, err := posts.NextPosition(); err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	payload, err := posts.GetPayload()
	if err != nil {
		t.Fatalf("GetPayload: %v", err)
	}
	if !bytes.Equal(payload, []byte{0xAB, 0xCD}) {
		t.Fatalf("GetPayload = %v, want [0xAB 0xCD]", payload)
	}
	if _, err := posts.NextPosition(); err != nil {
		t.Fatalf("NextPosition 2: %v", err)
	}
	if p, _ := posts.GetPayload(); p != nil {
		t.Fatalf("GetPayload position 2 = %v, want nil", p)
	}
}

func TestFreqProxPostingsEnum_RejectsPositionsWhenNotIndexed(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	h.addToken(t, "alpha", 0, 0, 0, 0, 1, nil)
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if _, err := enum.Postings(postingsFlagPositions); err == nil {
		t.Fatalf("Postings(POSITIONS) should reject DOCS+FREQS field")
	}
}

func TestFreqProxPostingsEnum_RejectsOffsetsWhenNotIndexed(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqsAndPositions)
	h.addToken(t, "alpha", 0, 0, 0, 0, 1, nil)
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if _, err := enum.Postings(postingsFlagOffsets); err == nil {
		t.Fatalf("Postings(OFFSETS) should reject DOCS+FREQS+POS field")
	}
}

func TestFreqProxTermsEnum_SeekExactOrdAndOrd(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	for _, term := range []string{"alpha", "bravo", "charlie"} {
		h.addToken(t, term, 0, 0, 0, 1, 1, nil)
	}
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	concrete := enum.(*FreqProxTermsEnum)
	concrete.SeekExactOrd(2)
	if got := concrete.Term().Text(); got != "charlie" {
		t.Fatalf("SeekExactOrd(2) = %q, want charlie", got)
	}
	if concrete.Ord() != 2 {
		t.Fatalf("Ord = %d, want 2", concrete.Ord())
	}
	concrete.SeekExactOrd(0)
	if got := concrete.Term().Text(); got != "alpha" {
		t.Fatalf("SeekExactOrd(0) = %q, want alpha", got)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("SeekExactOrd(out-of-range) should panic")
		}
	}()
	concrete.SeekExactOrd(99)
}

func TestFreqProxTermsEnum_StatsUnsupported(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	enum, _ := terms.GetIterator()
	concrete := enum.(*FreqProxTermsEnum)
	if _, err := enum.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if _, err := concrete.DocFreq(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("DocFreq err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if _, err := concrete.TotalTermFreq(); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("TotalTermFreq err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	if err := concrete.Impacts(0); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("Impacts err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
	state, err := concrete.TermState()
	if err != nil {
		t.Fatalf("TermState: %v", err)
	}
	if err := state.CopyFrom(nil); !errors.Is(err, ErrFreqProxFieldsUnsupported) {
		t.Fatalf("TermState.CopyFrom err = %v, want ErrFreqProxFieldsUnsupported", err)
	}
}

func TestFreqProxFields_GetPostingsReaderShortcut(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	h.addToken(t, "alpha", 0, 0, 0, 1, 1, nil)
	h.addToken(t, "alpha", 7, 0, 0, 1, 1, nil)
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	posts, err := terms.GetPostingsReader("alpha", postingsFlagFreqs)
	if err != nil {
		t.Fatalf("GetPostingsReader: %v", err)
	}
	if posts == nil {
		t.Fatalf("GetPostingsReader = nil for known term")
	}
	if doc, err := posts.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("NextDoc = %d, err %v", doc, err)
	}
	if doc, err := posts.NextDoc(); err != nil || doc != 7 {
		t.Fatalf("NextDoc = %d, err %v", doc, err)
	}
	miss, err := terms.GetPostingsReader("zulu", postingsFlagFreqs)
	if err != nil {
		t.Fatalf("GetPostingsReader miss: %v", err)
	}
	if miss != nil {
		t.Fatalf("GetPostingsReader(miss) = %v, want nil", miss)
	}
}

func TestFreqProxFields_IteratorWithSeekDecouplesCursors(t *testing.T) {
	h := newFreqProxFieldsHarness(t, IndexOptionsDocsAndFreqs)
	for _, term := range []string{"alpha", "bravo", "charlie"} {
		h.addToken(t, term, 0, 0, 0, 1, 1, nil)
	}
	fields := NewFreqProxFields([]*FreqProxTermsWriterPerField{h.w})
	terms, _ := fields.Terms("body")
	seek, err := terms.GetIteratorWithSeek(NewTerm("body", "bravo"))
	if err != nil {
		t.Fatalf("GetIteratorWithSeek: %v", err)
	}
	if seek == nil {
		t.Fatalf("GetIteratorWithSeek = nil, want enumerator")
	}
	if got := seek.Term().Text(); got != "bravo" {
		t.Fatalf("GetIteratorWithSeek positioned at %q, want bravo", got)
	}
	pastEnd, err := terms.GetIteratorWithSeek(NewTerm("body", "zulu"))
	if err != nil {
		t.Fatalf("GetIteratorWithSeek end: %v", err)
	}
	if pastEnd != nil {
		t.Fatalf("GetIteratorWithSeek(zulu) = %v, want nil", pastEnd)
	}
	// Cursor decoupling: re-seek the original iterator.
	if _, err := seek.SeekExact(NewTerm("body", "alpha")); err != nil {
		t.Fatalf("SeekExact alpha: %v", err)
	}
	if got := seek.Term().Text(); got != "alpha" {
		t.Fatalf("after SeekExact, current = %q, want alpha", got)
	}
}
