// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakePushWriter records every push-API call so WriteTerm's traversal order
// can be asserted without touching real codec files.
type fakePushWriter struct {
	docs      []int
	freqs     []int
	positions []int
	payloads  [][]byte
	starts    []int
	ends      []int
	finishes  int
}

func (w *fakePushWriter) Init(termsOut store.IndexOutput, state *SegmentWriteState) error {
	return nil
}
func (w *fakePushWriter) NewTermState() *BlockTermState                    { return NewBlockTermState() }
func (w *fakePushWriter) SetField(fieldInfo *index.FieldInfo) (int, error) { return 0, nil }
func (w *fakePushWriter) StartTerm(norms index.NumericDocValues) error     { return nil }
func (w *fakePushWriter) FinishTerm(state *BlockTermState) error           { return nil }
func (w *fakePushWriter) EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, state *BlockTermState, absolute bool) error {
	return nil
}
func (w *fakePushWriter) Close() error { return nil }

func (w *fakePushWriter) StartDoc(docID, freq int) error {
	w.docs = append(w.docs, docID)
	w.freqs = append(w.freqs, freq)
	return nil
}
func (w *fakePushWriter) AddPosition(position int, payload []byte, startOffset, endOffset int) error {
	w.positions = append(w.positions, position)
	// payload is copied to avoid aliasing the slice the postings enum may reuse.
	if payload != nil {
		cp := make([]byte, len(payload))
		copy(cp, payload)
		w.payloads = append(w.payloads, cp)
	} else {
		w.payloads = append(w.payloads, nil)
	}
	w.starts = append(w.starts, startOffset)
	w.ends = append(w.ends, endOffset)
	return nil
}
func (w *fakePushWriter) FinishDoc() error {
	w.finishes++
	return nil
}

// scriptedPostingsEnum is a minimal index.PostingsEnum that replays a fixed
// per-doc/per-position script. Embeds PostingsEnumBase to inherit unused
// no-op defaults.
type scriptedPostingsEnum struct {
	docs   []int
	freqs  []int
	posMat [][]int // positions per doc
	offMat [][]int // [start0, end0, start1, end1, ...] flattened per doc
	payMat [][]byte
	docIdx int
	posIdx int
}

func (s *scriptedPostingsEnum) NextDoc() (int, error) {
	s.docIdx++
	if s.docIdx >= len(s.docs) {
		return index.NO_MORE_DOCS, nil
	}
	s.posIdx = 0
	return s.docs[s.docIdx], nil
}
func (s *scriptedPostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := s.NextDoc()
		if err != nil || d == index.NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (s *scriptedPostingsEnum) DocID() int { return s.docs[s.docIdx] }
func (s *scriptedPostingsEnum) Freq() (int, error) {
	return s.freqs[s.docIdx], nil
}
func (s *scriptedPostingsEnum) NextPosition() (int, error) {
	if s.posIdx >= len(s.posMat[s.docIdx]) {
		return index.NO_MORE_POSITIONS, nil
	}
	p := s.posMat[s.docIdx][s.posIdx]
	s.posIdx++
	return p, nil
}
func (s *scriptedPostingsEnum) StartOffset() (int, error) {
	return s.offMat[s.docIdx][2*(s.posIdx-1)], nil
}
func (s *scriptedPostingsEnum) EndOffset() (int, error) {
	return s.offMat[s.docIdx][2*(s.posIdx-1)+1], nil
}
func (s *scriptedPostingsEnum) GetPayload() ([]byte, error) {
	if s.payMat[s.docIdx] == nil {
		return nil, nil
	}
	return s.payMat[s.docIdx], nil
}
func (s *scriptedPostingsEnum) Cost() int64 { return int64(len(s.docs) - 1) }

func TestWriteTerm_DocsOnly(t *testing.T) {
	t.Parallel()
	pe := &scriptedPostingsEnum{
		docs:   []int{-1, 0, 3, 7}, // docIdx starts at -1
		freqs:  []int{0, 2, 1, 5},
		posMat: [][]int{{}, {}, {}, {}},
	}
	w := &fakePushWriter{}
	// indexHasFreqs=false: StartDoc receives freq=-1; totalTermFreq must be -1.
	n, ttf, err := WriteTerm(w, pe, false, false, false, false)
	if err != nil {
		t.Fatalf("WriteTerm: %v", err)
	}
	if n != 3 {
		t.Errorf("docCount = %d, want 3", n)
	}
	if ttf != -1 {
		t.Errorf("totalTermFreq = %d, want -1 for DOCS-only", ttf)
	}
	if got, want := w.docs, []int{0, 3, 7}; !equalInts(got, want) {
		t.Errorf("docs = %v, want %v", got, want)
	}
	// With freqs=false, StartDoc is called with freq=-1 for every doc.
	if got, want := w.freqs, []int{-1, -1, -1}; !equalInts(got, want) {
		t.Errorf("freqs = %v, want %v (freq=-1 when freqs disabled)", got, want)
	}
	if w.finishes != 3 {
		t.Errorf("finishes = %d, want 3", w.finishes)
	}
	if len(w.positions) != 0 {
		t.Errorf("positions should be empty when indexHasPositions=false, got %v", w.positions)
	}
}

func TestWriteTerm_WithPositions(t *testing.T) {
	t.Parallel()
	pe := &scriptedPostingsEnum{
		docs:   []int{-1, 5},
		freqs:  []int{0, 3},
		posMat: [][]int{{}, {1, 4, 9}},
		offMat: [][]int{nil, {0, 0, 0, 0, 0, 0}}, // unused (indexHasOffsets=false)
		payMat: [][]byte{nil, nil},
	}
	w := &fakePushWriter{}
	// Positions require freqs; indexHasFreqs=true, indexHasPositions=true.
	n, ttf, err := WriteTerm(w, pe, true, true, false, false)
	if err != nil {
		t.Fatalf("WriteTerm: %v", err)
	}
	if n != 1 {
		t.Errorf("docCount = %d, want 1", n)
	}
	if ttf != 3 {
		t.Errorf("totalTermFreq = %d, want 3", ttf)
	}
	if got, want := w.positions, []int{1, 4, 9}; !equalInts(got, want) {
		t.Errorf("positions = %v, want %v", got, want)
	}
}

func TestWriteTerm_NilEnum(t *testing.T) {
	t.Parallel()
	_, _, err := WriteTerm(&fakePushWriter{}, nil, false, false, false, false)
	if err == nil || !errorContains(err, "nil postingsEnum") {
		t.Errorf("expected nil-enum error, got %v", err)
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

func errorContains(err error, sub string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errors.New(sub)) {
		return true
	}
	return containsSubstr(err.Error(), sub)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
