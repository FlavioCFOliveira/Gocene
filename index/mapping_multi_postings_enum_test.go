// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"strings"
	"testing"
)

// mmpeIdentityDocMap returns the raw docID unchanged, except for entries listed
// in deleted which return -1.
type mmpeIdentityDocMap struct {
	deleted map[int]struct{}
}

func newMmpeIdentityDocMap(deleted ...int) *mmpeIdentityDocMap {
	m := &mmpeIdentityDocMap{deleted: make(map[int]struct{}, len(deleted))}
	for _, d := range deleted {
		m.deleted[d] = struct{}{}
	}
	return m
}

func (m *mmpeIdentityDocMap) Get(oldDocID int) int {
	if _, gone := m.deleted[oldDocID]; gone {
		return -1
	}
	return oldDocID
}

// shiftDocMap maps raw doc d to d + offset.
type shiftDocMap struct{ offset int }

func (s shiftDocMap) Get(oldDocID int) int { return oldDocID + s.offset }

// scriptedPostings is a PostingsEnum driven by hard-coded doc/position/freq
// scripts. Enough surface for the MappingMultiPostingsEnum tests.
type scriptedPostings struct {
	docs      []int
	freqs     []int
	positions [][]int
	payloads  [][][]byte
	startOffs [][]int
	endOffs   [][]int
	docIdx    int
	posIdx    int
}

func (p *scriptedPostings) NextDoc() (int, error) {
	p.docIdx++
	p.posIdx = -1
	if p.docIdx >= len(p.docs) {
		return NO_MORE_DOCS, nil
	}
	return p.docs[p.docIdx], nil
}

func (p *scriptedPostings) Advance(int) (int, error) { return 0, errors.New("unsupported") }
func (p *scriptedPostings) DocID() int {
	if p.docIdx < 0 || p.docIdx >= len(p.docs) {
		return NO_MORE_DOCS
	}
	return p.docs[p.docIdx]
}
func (p *scriptedPostings) Freq() (int, error) {
	if p.docIdx < 0 || p.docIdx >= len(p.freqs) {
		return 0, errors.New("scriptedPostings: Freq out of range")
	}
	return p.freqs[p.docIdx], nil
}
func (p *scriptedPostings) NextPosition() (int, error) {
	if p.docIdx < 0 || p.docIdx >= len(p.positions) {
		return 0, errors.New("scriptedPostings: NextPosition out of doc range")
	}
	p.posIdx++
	if p.posIdx >= len(p.positions[p.docIdx]) {
		return NO_MORE_POSITIONS, nil
	}
	pos := p.positions[p.docIdx][p.posIdx]
	if pos < 0 || pos > MaxPosition {
		// Surface the raw position so MappingMultiPostingsEnum can apply
		// its CorruptIndex bounds check rather than getting swallowed by
		// the helper.
		return pos, nil
	}
	return pos, nil
}
func (p *scriptedPostings) StartOffset() (int, error) {
	if p.startOffs == nil {
		return -1, nil
	}
	return p.startOffs[p.docIdx][p.posIdx], nil
}
func (p *scriptedPostings) EndOffset() (int, error) {
	if p.endOffs == nil {
		return -1, nil
	}
	return p.endOffs[p.docIdx][p.posIdx], nil
}
func (p *scriptedPostings) GetPayload() ([]byte, error) {
	if p.payloads == nil {
		return nil, nil
	}
	return p.payloads[p.docIdx][p.posIdx], nil
}
func (p *scriptedPostings) Cost() int64 { return int64(len(p.docs)) }

func newScriptedPostings(docs []int, freqs []int) *scriptedPostings {
	return &scriptedPostings{
		docs:   docs,
		freqs:  freqs,
		docIdx: -1,
		posIdx: -1,
	}
}

// makeMergeState assembles a MergeState carrying only the DocMaps and
// NeedsIndexSort fields exercised by MappingMultiPostingsEnum.
func makeMergeState(docMaps []DocMap, sorted bool) *MergeState {
	return &MergeState{DocMaps: docMaps, NeedsIndexSort: sorted}
}

// makeMulti assembles a MultiPostingsEnum carrying (postings, slice) pairs.
func makeMulti(pairs ...EnumWithSlice) *MultiPostingsEnum {
	m := NewMultiPostingsEnum(nil)
	m.SubsWithSlice = pairs
	m.NumSubs = len(pairs)
	return m
}

func TestMappingMultiPostingsEnum_NextDocConcat(t *testing.T) {
	// Two segments, no index sort -> concat strategy. Segment 0 shifts +0,
	// segment 1 shifts +10. Walk doc-by-doc.
	pa := newScriptedPostings([]int{0, 2}, []int{1, 1})
	pb := newScriptedPostings([]int{0, 1, 5}, []int{1, 1, 1})

	ms := makeMergeState([]DocMap{shiftDocMap{offset: 0}, shiftDocMap{offset: 10}}, false)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := mm.Reset(makeMulti(
		EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 0}},
		EnumWithSlice{PostingsEnum: pb, Slice: ReaderSlice{ReaderIndex: 1}},
	)); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if mm.DocID() != -1 {
		t.Fatalf("DocID before NextDoc = %d, want -1", mm.DocID())
	}
	wantDocs := []int{0, 2, 10, 11, 15}
	for _, want := range wantDocs {
		got, err := mm.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if got != want {
			t.Errorf("NextDoc=%d, want %d", got, want)
		}
		if mm.DocID() != want {
			t.Errorf("DocID=%d after NextDoc, want %d", mm.DocID(), want)
		}
	}
	got, err := mm.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if got != NO_MORE_DOCS {
		t.Errorf("trailing NextDoc=%d, want NO_MORE_DOCS", got)
	}
	if mm.DocID() != -1 {
		t.Errorf("DocID after exhaustion=%d, want -1", mm.DocID())
	}
}

func TestMappingMultiPostingsEnum_NextDocSortedWithDeletions(t *testing.T) {
	// Sorted strategy: subs are interleaved by mappedDocID. The DocMap for
	// segment 0 hides raw doc 1; segment 1 maps identity.
	pa := newScriptedPostings([]int{0, 1, 4}, []int{1, 1, 1})
	pb := newScriptedPostings([]int{2, 3}, []int{1, 1})

	ms := makeMergeState([]DocMap{newMmpeIdentityDocMap(1), newMmpeIdentityDocMap()}, true)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := mm.Reset(makeMulti(
		EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 0}},
		EnumWithSlice{PostingsEnum: pb, Slice: ReaderSlice{ReaderIndex: 1}},
	)); err != nil {
		t.Fatalf("reset: %v", err)
	}
	want := []int{0, 2, 3, 4}
	for _, w := range want {
		got, err := mm.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if got != w {
			t.Errorf("NextDoc=%d, want %d", got, w)
		}
	}
	got, _ := mm.NextDoc()
	if got != NO_MORE_DOCS {
		t.Errorf("trailing NextDoc=%d, want NO_MORE_DOCS", got)
	}
}

func TestMappingMultiPostingsEnum_PositionsFreqOffsetsPayload(t *testing.T) {
	pa := &scriptedPostings{
		docs:      []int{0},
		freqs:     []int{2},
		positions: [][]int{{3, 7}},
		startOffs: [][]int{{10, 30}},
		endOffs:   [][]int{{14, 34}},
		payloads:  [][][]byte{{[]byte("p1"), []byte("p2")}},
		docIdx:    -1,
		posIdx:    -1,
	}
	ms := makeMergeState([]DocMap{newMmpeIdentityDocMap()}, false)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := mm.Reset(makeMulti(
		EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 0}},
	)); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := mm.NextDoc(); err != nil {
		t.Fatal(err)
	}
	if freq, _ := mm.Freq(); freq != 2 {
		t.Errorf("Freq=%d, want 2", freq)
	}
	type posTuple struct{ pos, start, end int }
	wantTuples := []posTuple{{3, 10, 14}, {7, 30, 34}}
	wantPayloads := []string{"p1", "p2"}
	for i, w := range wantTuples {
		pos, err := mm.NextPosition()
		if err != nil {
			t.Fatalf("NextPosition[%d]: %v", i, err)
		}
		if pos != w.pos {
			t.Errorf("pos[%d]=%d, want %d", i, pos, w.pos)
		}
		if so, _ := mm.StartOffset(); so != w.start {
			t.Errorf("startOffset[%d]=%d, want %d", i, so, w.start)
		}
		if eo, _ := mm.EndOffset(); eo != w.end {
			t.Errorf("endOffset[%d]=%d, want %d", i, eo, w.end)
		}
		if pl, _ := mm.GetPayload(); string(pl) != wantPayloads[i] {
			t.Errorf("payload[%d]=%q, want %q", i, pl, wantPayloads[i])
		}
	}
	// Note: callers iterate exactly freq() positions; Lucene's
	// MappingMultiPostingsEnum traps a *negative* position as corruption
	// rather than treating it as an exhaustion sentinel, so we do not
	// poll NextPosition past freq().
}

func TestMappingMultiPostingsEnum_NextPositionCorruptIndex(t *testing.T) {
	cases := []struct {
		name string
		pos  int
		want string
	}{
		{"negative", -1, "is negative"},
		{"oversized", MaxPosition + 1, "is too large"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pa := &scriptedPostings{
				docs:      []int{0},
				freqs:     []int{1},
				positions: [][]int{{tc.pos}},
				docIdx:    -1,
				posIdx:    -1,
			}
			ms := makeMergeState([]DocMap{newMmpeIdentityDocMap()}, false)
			mm, err := NewMappingMultiPostingsEnum("body", ms)
			if err != nil {
				t.Fatalf("ctor: %v", err)
			}
			if _, err := mm.Reset(makeMulti(
				EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 0}},
			)); err != nil {
				t.Fatalf("reset: %v", err)
			}
			if _, err := mm.NextDoc(); err != nil {
				t.Fatal(err)
			}
			_, err = mm.NextPosition()
			if err == nil {
				t.Fatal("NextPosition: want error, got nil")
			}
			var cie *CorruptIndexException
			if !errors.As(err, &cie) {
				t.Fatalf("NextPosition err type = %T, want *CorruptIndexException", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("NextPosition err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestMappingMultiPostingsEnum_AdvanceUnsupported(t *testing.T) {
	ms := makeMergeState([]DocMap{newMmpeIdentityDocMap()}, false)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := mm.Advance(0); err == nil {
		t.Error("Advance: want error, got nil")
	}
}

func TestMappingMultiPostingsEnum_Cost(t *testing.T) {
	pa := newScriptedPostings([]int{0, 1, 2}, []int{1, 1, 1})
	pb := newScriptedPostings([]int{0, 1}, []int{1, 1})
	ms := makeMergeState([]DocMap{newMmpeIdentityDocMap(), newMmpeIdentityDocMap()}, false)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := mm.Reset(makeMulti(
		EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 0}},
		EnumWithSlice{PostingsEnum: pb, Slice: ReaderSlice{ReaderIndex: 1}},
	)); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := mm.Cost(); got != 5 {
		t.Errorf("Cost=%d, want 5", got)
	}
}

func TestMappingMultiPostingsEnum_ResetRejectsBadReaderIndex(t *testing.T) {
	pa := newScriptedPostings([]int{0}, []int{1})
	ms := makeMergeState([]DocMap{newMmpeIdentityDocMap()}, false)
	mm, err := NewMappingMultiPostingsEnum("body", ms)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	_, err = mm.Reset(makeMulti(
		EnumWithSlice{PostingsEnum: pa, Slice: ReaderSlice{ReaderIndex: 99}},
	))
	if err == nil {
		t.Fatal("Reset: want error for out-of-range readerIndex, got nil")
	}
}

func TestMappingMultiPostingsEnum_NilMergeState(t *testing.T) {
	if _, err := NewMappingMultiPostingsEnum("body", nil); err == nil {
		t.Error("ctor with nil mergeState: want error, got nil")
	}
}
