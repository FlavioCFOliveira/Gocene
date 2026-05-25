// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fakePostingsWriter is a deterministic PushPostingsWriterBase used by the
// reader/writer integration tests. It performs no compression: every term
// emits a single byte (its 1-based ordinal) into the meta blob via
// EncodeTerm, with absolute=true on the first term in a block and false
// otherwise. The matching fakePostingsReader checks the same byte back.
type fakePostingsWriter struct {
	termsOut store.IndexOutput
	termOrd  int
}

func newFakePostingsWriter() *fakePostingsWriter { return &fakePostingsWriter{} }

func (f *fakePostingsWriter) Init(termsOut store.IndexOutput, _ *SegmentWriteState) error {
	f.termsOut = termsOut
	// Stamp a magic byte so the reader can verify Init ran exactly once.
	return termsOut.WriteByte(0x42)
}

func (f *fakePostingsWriter) NewTermState() *BlockTermState {
	return NewBlockTermState()
}

func (f *fakePostingsWriter) SetField(_ *index.FieldInfo) (int, error) { return 0, nil }

func (f *fakePostingsWriter) StartTerm(_ index.NumericDocValues) error {
	f.termOrd++
	return nil
}

func (f *fakePostingsWriter) FinishTerm(state *BlockTermState) error {
	// docFreq is already populated by the BlockTreeTermsWriter from
	// WriteTerm's docCount return; only TotalTermFreq remains.
	state.TotalTermFreq = int64(state.DocFreq)
	return nil
}

func (f *fakePostingsWriter) EncodeTerm(out store.IndexOutput, _ *index.FieldInfo, _ *BlockTermState, _ bool) error {
	return out.WriteByte(byte(f.termOrd))
}

func (f *fakePostingsWriter) StartDoc(_, _ int) error { return nil }
func (f *fakePostingsWriter) AddPosition(_ int, _ []byte, _, _ int) error {
	return nil
}
func (f *fakePostingsWriter) FinishDoc() error { return nil }
func (f *fakePostingsWriter) Close() error     { return nil }

// fakePostingsReader is the read-side companion: Init verifies the magic
// byte written by the writer; CheckIntegrity is a no-op; everything else
// is unused by the reader-construction tests.
type fakePostingsReader struct {
	initialised bool
}

func newFakePostingsReader() *fakePostingsReader { return &fakePostingsReader{} }

func (f *fakePostingsReader) Init(termsIn store.IndexInput, _ *SegmentReadState) error {
	b, err := termsIn.ReadByte()
	if err != nil {
		return err
	}
	if b != 0x42 {
		return errors.New("fakePostingsReader: missing magic byte")
	}
	f.initialised = true
	return nil
}

func (f *fakePostingsReader) NewTermState() *BlockTermState { return NewBlockTermState() }

func (f *fakePostingsReader) DecodeTerm(_ store.DataInput, _ *index.FieldInfo, _ *BlockTermState, _ bool) error {
	return nil
}

func (f *fakePostingsReader) Postings(_ *index.FieldInfo, _ *BlockTermState, _ index.PostingsEnum, _ int) (index.PostingsEnum, error) {
	return nil, nil
}

func (f *fakePostingsReader) Impacts(_ *index.FieldInfo, _ *BlockTermState, _ int) (any, error) {
	return nil, nil
}

func (f *fakePostingsReader) CheckIntegrity() error { return nil }
func (f *fakePostingsReader) Close() error          { return nil }

// fakeTermsEnum wraps a sorted slice of (term, docFreq) and exposes the
// minimum TermsEnum surface BlockTreeTermsWriter requires.
type fakeTermsEnum struct {
	terms   []fakeTermEntry
	idx     int
	current *index.Term
}

type fakeTermEntry struct {
	text    string
	docFreq int
}

func (e *fakeTermsEnum) Next() (*index.Term, error) {
	if e.idx >= len(e.terms) {
		e.current = nil
		return nil, nil
	}
	e.current = index.NewTerm("", e.terms[e.idx].text)
	e.idx++
	return e.current, nil
}
func (e *fakeTermsEnum) SeekCeil(_ *index.Term) (*index.Term, error) { return nil, nil }
func (e *fakeTermsEnum) SeekExact(_ *index.Term) (bool, error)       { return false, nil }
func (e *fakeTermsEnum) Term() *index.Term                           { return e.current }
func (e *fakeTermsEnum) DocFreq() (int, error)                       { return e.terms[e.idx-1].docFreq, nil }
func (e *fakeTermsEnum) TotalTermFreq() (int64, error)               { return int64(e.terms[e.idx-1].docFreq), nil }
func (e *fakeTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return &fakePostingsEnum{remaining: e.terms[e.idx-1].docFreq}, nil
}
func (e *fakeTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return e.Postings(0)
}

type fakePostingsEnum struct {
	remaining int
	docID     int
}

func (p *fakePostingsEnum) NextDoc() (int, error) {
	if p.remaining == 0 {
		p.docID = index.NO_MORE_DOCS
		return p.docID, nil
	}
	p.remaining--
	p.docID++
	return p.docID, nil
}
func (p *fakePostingsEnum) Advance(_ int) (int, error)  { return p.NextDoc() }
func (p *fakePostingsEnum) DocID() int                  { return p.docID }
func (p *fakePostingsEnum) Freq() (int, error)          { return 1, nil }
func (p *fakePostingsEnum) NextPosition() (int, error)  { return index.NO_MORE_POSITIONS, nil }
func (p *fakePostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *fakePostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *fakePostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (p *fakePostingsEnum) Cost() int64                 { return 0 }

// fakeTerms is the index.Terms wrapper passed to BlockTreeTermsWriter.
type fakeTerms struct {
	*index.TermsBase
	entries []fakeTermEntry
}

func (t *fakeTerms) GetIterator() (index.TermsEnum, error) {
	return &fakeTermsEnum{terms: t.entries}, nil
}
func (t *fakeTerms) GetIteratorWithSeek(_ *index.Term) (index.TermsEnum, error) {
	return t.GetIterator()
}
func (t *fakeTerms) GetPostingsReader(_ string, _ int) (index.PostingsEnum, error) {
	return nil, nil
}
func (t *fakeTerms) Size() int64                   { return int64(len(t.entries)) }
func (t *fakeTerms) GetMin() (*index.Term, error)  { return nil, nil }
func (t *fakeTerms) GetMax() (*index.Term, error)  { return nil, nil }
func (t *fakeTerms) HasFreqs() bool                { return true }
func (t *fakeTerms) HasOffsets() bool              { return false }
func (t *fakeTerms) HasPositions() bool            { return false }
func (t *fakeTerms) HasPayloads() bool             { return false }
func (t *fakeTerms) GetSumDocFreq() (int64, error) { return int64(len(t.entries)), nil }
func (t *fakeTerms) GetSumTotalTermFreq() (int64, error) {
	return int64(len(t.entries)), nil
}

// buildSegment writes a single field via Lucene103BlockTreeTermsWriter and
// returns the directory plus the segment-write state so the reader-side
// test can re-open the same segment.
func buildSegment(t *testing.T, segmentName string, fieldName string, opts index.IndexOptions, terms []fakeTermEntry) (store.Directory, *SegmentReadState) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()

	si := index.NewSegmentInfo(segmentName, 16, dir)
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	fi := index.NewFieldInfo(fieldName, 0, index.FieldInfoOptions{IndexOptions: opts})
	infos := index.NewFieldInfos()
	if err := infos.Add(fi); err != nil {
		t.Fatalf("infos.Add: %v", err)
	}
	state := &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    infos,
		SegmentSuffix: "",
	}

	w, err := NewLucene103BlockTreeTermsWriter(state, newFakePostingsWriter(), Lucene103DefaultMinBlockSize, Lucene103DefaultMaxBlockSize)
	if err != nil {
		t.Fatalf("NewLucene103BlockTreeTermsWriter: %v", err)
	}
	fields := index.NewMemoryFields()
	fields.AddField(fieldName, &fakeTerms{entries: terms})
	if err := w.WriteFields(fields, nil); err != nil {
		t.Fatalf("WriteFields: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Writer.Close: %v", err)
	}

	return dir, &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    infos,
		SegmentSuffix: "",
	}
}

func TestLucene103BlockTreeTermsReader_RoundTripSingleField(t *testing.T) {
	terms := make([]fakeTermEntry, 0, 40)
	// 40 sorted distinct terms is enough to force the writer to emit at
	// least one root block plus a few interior blocks once the prefix
	// fans out.
	for i := 0; i < 40; i++ {
		terms = append(terms, fakeTermEntry{
			text:    indexedTermText(i),
			docFreq: 1 + i%4,
		})
	}
	dir, readState := buildSegment(t, "_round1", "title", index.IndexOptionsDocsAndFreqs, terms)
	defer dir.Close()

	r, err := NewLucene103BlockTreeTermsReader(newFakePostingsReader(), readState)
	if err != nil {
		t.Fatalf("NewLucene103BlockTreeTermsReader: %v", err)
	}
	defer r.Close()

	if got := r.SegmentName(); got != "_round1" {
		t.Errorf("SegmentName: want _round1, got %s", got)
	}
	if got := r.Version(); got != Lucene103BlockTreeVersionCurrent {
		t.Errorf("Version: want %d, got %d", Lucene103BlockTreeVersionCurrent, got)
	}
	if got := r.Size(); got != 1 {
		t.Errorf("Size: want 1, got %d", got)
	}
	if names := r.FieldNames(); len(names) != 1 || names[0] != "title" {
		t.Errorf("FieldNames: want [title], got %v", names)
	}

	gotTerms, err := r.Terms("title")
	if err != nil {
		t.Fatalf("Terms(title): %v", err)
	}
	if gotTerms == nil {
		t.Fatal("Terms(title) returned nil")
	}
	if size := gotTerms.Size(); size != int64(len(terms)) {
		t.Errorf("Terms.Size: want %d, got %d", len(terms), size)
	}
	if df, _ := gotTerms.GetSumDocFreq(); df != int64(sumDocFreq(terms)) {
		t.Errorf("GetSumDocFreq: want %d, got %d", sumDocFreq(terms), df)
	}
	if _, err := r.Terms("absent"); err != nil {
		t.Errorf("Terms(absent): want nil error, got %v", err)
	}

	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: %v", err)
	}
}

func TestLucene103BlockTreeTermsReader_RejectsNilArgs(t *testing.T) {
	if _, err := NewLucene103BlockTreeTermsReader(nil, &SegmentReadState{}); err == nil {
		t.Error("nil postingsReader must error")
	}
	if _, err := NewLucene103BlockTreeTermsReader(newFakePostingsReader(), nil); err == nil {
		t.Error("nil state must error")
	}
}

func TestLucene103BlockTreeTermsReader_CloseIsIdempotent(t *testing.T) {
	terms := []fakeTermEntry{{text: "alpha", docFreq: 1}, {text: "beta", docFreq: 2}}
	dir, readState := buildSegment(t, "_idem", "f", index.IndexOptionsDocs, terms)
	defer dir.Close()
	r, err := NewLucene103BlockTreeTermsReader(newFakePostingsReader(), readState)
	if err != nil {
		t.Fatalf("NewLucene103BlockTreeTermsReader: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if _, err := r.Terms("f"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Terms after Close: want closed error, got %v", err)
	}
}

func TestLucene103BlockTreeTermsReader_StringContainsFieldCount(t *testing.T) {
	terms := []fakeTermEntry{{text: "x", docFreq: 1}}
	dir, readState := buildSegment(t, "_str", "f", index.IndexOptionsDocs, terms)
	defer dir.Close()
	r, err := NewLucene103BlockTreeTermsReader(newFakePostingsReader(), readState)
	if err != nil {
		t.Fatalf("NewLucene103BlockTreeTermsReader: %v", err)
	}
	defer r.Close()
	s := r.String()
	if !strings.Contains(s, "fields=1") {
		t.Errorf("String: want fields=1, got %q", s)
	}
}

// indexedTermText produces sortable 4-char strings: term0000 < term0001 < ...
func indexedTermText(i int) string {
	const digits = "0123456789"
	var b [8]byte
	copy(b[:4], "term")
	b[4] = digits[(i/1000)%10]
	b[5] = digits[(i/100)%10]
	b[6] = digits[(i/10)%10]
	b[7] = digits[i%10]
	return string(b[:])
}

func sumDocFreq(t []fakeTermEntry) int {
	total := 0
	for _, e := range t {
		total += e.docFreq
	}
	return total
}
