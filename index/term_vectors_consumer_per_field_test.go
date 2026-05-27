// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// tvStubAttrs mirrors a single token's analysis attributes. Tests mutate it
// between Add calls; the writer reads through the function fields wired to
// its methods.
type tvStubAttrs struct {
	start   int
	end     int
	payload *util.BytesRef
	freq    int
	hasFreq bool
}

func (s *tvStubAttrs) provider() TermVectorsAttributeProvider {
	p := TermVectorsAttributeProvider{
		StartOffset: func() int { return s.start },
		EndOffset:   func() int { return s.end },
		Payload:     func() *util.BytesRef { return s.payload },
	}
	if s.hasFreq {
		p.TermFrequency = func() int { return s.freq }
	}
	return p
}

// capturedPosition records one AddPosition call from the writer.
type capturedPosition struct {
	position    int
	startOffset int
	endOffset   int
	payload     []byte
}

// capturingTVWriter is a TermVectorsWriter that records the full argument of
// every call so a test can assert the decoded position/offset/payload stream.
type capturingTVWriter struct {
	startFieldNumTerms int
	startFieldPos      bool
	startFieldOff      bool
	startFieldPayloads bool
	terms              [][]byte
	positions          []capturedPosition
	finishTerm         int
	finishField        int
}

func (w *capturingTVWriter) StartDocument(int) error { return nil }
func (w *capturingTVWriter) StartField(_ *FieldInfo, numTerms int, p, o, pl bool) error {
	w.startFieldNumTerms = numTerms
	w.startFieldPos = p
	w.startFieldOff = o
	w.startFieldPayloads = pl
	return nil
}
func (w *capturingTVWriter) StartTerm(term []byte) error {
	cp := make([]byte, len(term))
	copy(cp, term)
	w.terms = append(w.terms, cp)
	return nil
}
func (w *capturingTVWriter) AddPosition(pos, s, e int, payload []byte) error {
	var pl []byte
	if payload != nil {
		pl = make([]byte, len(payload))
		copy(pl, payload)
	}
	w.positions = append(w.positions, capturedPosition{pos, s, e, pl})
	return nil
}
func (w *capturingTVWriter) FinishTerm() error     { w.finishTerm++; return nil }
func (w *capturingTVWriter) FinishField() error    { w.finishField++; return nil }
func (w *capturingTVWriter) FinishDocument() error { return nil }
func (w *capturingTVWriter) Close() error          { return nil }

// tvFieldInfo builds a FieldInfo with the given term-vector settings. The
// builder is used so the cross-field consistency rules (offsets/positions
// imply vectors) are applied exactly as in production.
func tvFieldInfo(name string, opts IndexOptions, vectors, positions, offsets, payloads bool) *FieldInfo {
	return NewFieldInfoBuilder(name, 0).
		SetIndexOptions(opts).
		SetStoreTermVectors(vectors).
		SetStoreTermVectorPositions(positions).
		SetStoreTermVectorOffsets(offsets).
		SetStoreTermVectorPayloads(payloads).
		Build()
}

// newTVConsumer builds a TermVectorsConsumer with the supplied writer already
// installed, so per-field tests can drive FinishDocument without the codec
// init path.
func newTVConsumer(w TermVectorsWriter) *TermVectorsConsumer {
	c := NewTermVectorsConsumer(nil, NewSegmentInfo("seg", 1, nil), nil)
	c.Writer = w
	return c
}

func TestTermVectorsConsumerPerField_RejectsBadConstructorInputs(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocs)
	fi := tvFieldInfo("body", IndexOptionsDocs, true, false, false, false)
	none := tvFieldInfo("body", IndexOptionsNone, false, false, false, false)
	consumer := newTVConsumer(&capturingTVWriter{})
	provider := (&tvStubAttrs{}).provider()

	if _, err := NewTermVectorsConsumerPerField(nil, consumer, fi, provider); err == nil {
		t.Fatalf("nil invertState should be rejected")
	}
	if _, err := NewTermVectorsConsumerPerField(state, nil, fi, provider); err == nil {
		t.Fatalf("nil termsHash should be rejected")
	}
	if _, err := NewTermVectorsConsumerPerField(state, consumer, nil, provider); err == nil {
		t.Fatalf("nil fieldInfo should be rejected")
	}
	if _, err := NewTermVectorsConsumerPerField(state, consumer, none, provider); err == nil {
		t.Fatalf("IndexOptionsNone field should be rejected")
	}
}

func TestTermVectorsConsumerPerField_StartGatesOnVectorsFlag(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	consumer := newTVConsumer(&capturingTVWriter{})

	// Vectors disabled: Start returns false and Finish is a no-op.
	noVec := tvFieldInfo("body", IndexOptionsDocsAndFreqs, false, false, false, false)
	w, err := NewTermVectorsConsumerPerField(state, consumer, noVec, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if w.Start(nil, true) {
		t.Fatalf("Start should return false when the field does not store term vectors")
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if consumer.NumVectorFields() != 0 {
		t.Fatalf("NumVectorFields = %d, want 0 (no terms, vectors off)", consumer.NumVectorFields())
	}

	// Vectors enabled: Start returns true.
	vec := tvFieldInfo("body", IndexOptionsDocsAndFreqs, true, false, false, false)
	w2, err := NewTermVectorsConsumerPerField(state, consumer, vec, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if !w2.Start(nil, true) {
		t.Fatalf("Start should return true when the field stores term vectors")
	}
}

func TestTermVectorsConsumerPerField_StartRejectsOffsetsWithoutVectors(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	consumer := newTVConsumer(&capturingTVWriter{})
	// Offsets requested but vectors off: the FieldInfo builder does not
	// auto-enable vectors for offsets, so start() must reject the combination.
	fi := NewFieldInfoBuilder("body", 0).
		SetIndexOptions(IndexOptionsDocsAndFreqs).
		SetStoreTermVectors(false).
		Build()
	// Force the inconsistent state the builder would normally repair.
	fi.OverrideStoreTermVectorOffsets(true)

	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Start should panic when offsets are requested without term vectors")
		}
	}()
	w.Start(nil, true)
}

func TestTermVectorsConsumerPerField_FinishRegistersWhenTermsSeen(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	consumer := newTVConsumer(&capturingTVWriter{})
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqs, true, false, false, false)

	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, (&tvStubAttrs{hasFreq: false}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if !w.Start(nil, true) {
		t.Fatalf("Start should enable vectors")
	}
	if err := w.Add(util.NewBytesRef([]byte("hello")), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if got := consumer.NumVectorFields(); got != 1 {
		t.Fatalf("NumVectorFields = %d, want 1 after Finish with a term", got)
	}
}

func TestTermVectorsConsumerPerField_FreqAccumulatesPerDoc(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	consumer := newTVConsumer(&capturingTVWriter{})
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqs, true, false, false, false)

	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if !w.Start(nil, true) {
		t.Fatalf("Start should enable vectors")
	}
	term := util.NewBytesRef([]byte("hello"))
	for i := 0; i < 4; i++ {
		if err := w.Add(term, 0); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	if got := w.postingsArray.Freqs[0]; got != 4 {
		t.Fatalf("Freqs[0] = %d, want 4", got)
	}
}

// TestTermVectorsConsumerPerField_FinishDocumentRoundTrip drives a full
// positions+offsets+payloads field through Add and FinishDocument, then
// checks that the writer observed the exact positions, offsets and payloads
// that were fed in. This validates the writeProx encode and the FinishDocument
// decode against each other.
func TestTermVectorsConsumerPerField_FinishDocumentRoundTrip(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	tw := &capturingTVWriter{}
	consumer := newTVConsumer(tw)
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true, true, true, true)

	stub := &tvStubAttrs{}
	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, stub.provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if !w.Start(nil, true) {
		t.Fatalf("Start should enable vectors")
	}

	term := util.NewBytesRef([]byte("hello"))
	// Token 0: position 0, offsets [0,5), no payload.
	state.SetPosition(0)
	stub.start, stub.end, stub.payload = 0, 5, nil
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add token 0: %v", err)
	}
	// Token 1: position 3, offsets [8,13), payload {0xAA,0xBB}.
	state.SetPosition(3)
	stub.start, stub.end, stub.payload = 8, 13, util.NewBytesRef([]byte{0xAA, 0xBB})
	if err := w.Add(term, 0); err != nil {
		t.Fatalf("Add token 1: %v", err)
	}

	if err := w.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}

	if !tw.startFieldPos || !tw.startFieldOff || !tw.startFieldPayloads {
		t.Fatalf("StartField flags = pos:%v off:%v payloads:%v, want all true",
			tw.startFieldPos, tw.startFieldOff, tw.startFieldPayloads)
	}
	if tw.startFieldNumTerms != 1 {
		t.Fatalf("StartField numTerms = %d, want 1", tw.startFieldNumTerms)
	}
	if len(tw.terms) != 1 || !bytes.Equal(tw.terms[0], []byte("hello")) {
		t.Fatalf("terms = %v, want [hello]", tw.terms)
	}
	if len(tw.positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(tw.positions))
	}
	want := []capturedPosition{
		{position: 0, startOffset: 0, endOffset: 5, payload: nil},
		{position: 3, startOffset: 8, endOffset: 13, payload: []byte{0xAA, 0xBB}},
	}
	for i, exp := range want {
		got := tw.positions[i]
		if got.position != exp.position || got.startOffset != exp.startOffset || got.endOffset != exp.endOffset {
			t.Fatalf("position %d = %+v, want %+v", i, got, exp)
		}
		if !bytes.Equal(got.payload, exp.payload) {
			t.Fatalf("position %d payload = %v, want %v", i, got.payload, exp.payload)
		}
	}
	if tw.finishTerm != 1 || tw.finishField != 1 {
		t.Fatalf("finishTerm=%d finishField=%d, want 1 and 1", tw.finishTerm, tw.finishField)
	}
}

// TestTermVectorsConsumerPerField_FinishDocumentSetsStoreTermVectors checks
// that FinishDocument flips the owning FieldInfo to advertise stored term
// vectors, mirroring Lucene's FieldInfo.setStoreTermVectors() call.
func TestTermVectorsConsumerPerField_FinishDocumentSetsStoreTermVectors(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	consumer := newTVConsumer(&capturingTVWriter{})
	// Positions-only would auto-enable vectors via the builder; start from a
	// plain vectors field and clear the flag so we can observe the flip.
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqs, true, false, false, false)
	fi.OverrideStoreTermVectors(false)

	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	// Start re-reads storeTermVectors; set it back so the field collects.
	fi.OverrideStoreTermVectors(true)
	if !w.Start(nil, true) {
		t.Fatalf("Start should enable vectors")
	}
	if err := w.Add(util.NewBytesRef([]byte("hello")), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fi.OverrideStoreTermVectors(false)
	if fi.HasTermVectors() {
		t.Fatalf("precondition: HasTermVectors should be false before FinishDocument")
	}
	if err := w.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if !fi.HasTermVectors() {
		t.Fatalf("FinishDocument should have set storeTermVectors on the FieldInfo")
	}
}

// TestTermVectorsConsumerPerField_FinishDocumentNoVectorsIsNoop verifies that
// FinishDocument does nothing when the field did not collect vectors.
func TestTermVectorsConsumerPerField_FinishDocumentNoVectorsIsNoop(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqs)
	tw := &capturingTVWriter{}
	consumer := newTVConsumer(tw)
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqs, false, false, false, false)

	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, (&tvStubAttrs{}).provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	w.Start(nil, true) // returns false; doVectors stays false
	if err := w.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if tw.startFieldNumTerms != 0 || tw.finishField != 0 || len(tw.terms) != 0 {
		t.Fatalf("FinishDocument should not have touched the writer for a no-vectors field")
	}
}

func TestTermVectorsConsumerPerField_GetTermFreqRejectsCustomFreqWithPositions(t *testing.T) {
	state := NewFieldInvertState(10, "body", IndexOptionsDocsAndFreqsAndPositions)
	consumer := newTVConsumer(&capturingTVWriter{})
	fi := tvFieldInfo("body", IndexOptionsDocsAndFreqsAndPositions, true, true, false, false)

	// Custom term frequency != 1 with vector positions enabled must panic,
	// mirroring Lucene's getTermFreq IllegalArgumentException.
	stub := &tvStubAttrs{hasFreq: true, freq: 3}
	w, err := NewTermVectorsConsumerPerField(state, consumer, fi, stub.provider())
	if err != nil {
		t.Fatalf("NewTermVectorsConsumerPerField: %v", err)
	}
	if !w.Start(nil, true) {
		t.Fatalf("Start should enable vectors")
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("custom term frequency with vector positions should panic")
		}
	}()
	_ = w.Add(util.NewBytesRef([]byte("hello")), 0)
}
