// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLucene104PostingsFormat_LastPosBlockOffset_Zero exercises the
// lastPosBlockOffset=-1 path: totalTermFreq <= BLOCK_SIZE (128).
// A single term with 5 docs × max freq 3 = at most 15 positions.
func TestLucene104PostingsFormat_LastPosBlockOffset_Zero(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	tester := NewPostingsTester(t)
	format := NewLucene104PostingsFormat()
	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositions, dir)
}

// largePosPostingsEnum is a PostingsEnum that yields numDocs documents, each
// with freq=1 at position docIdx, producing totalTermFreq=numDocs positions.
// It is used to exercise the lastPosBlockOffset >= 0 path (totalTermFreq > 128).
type largePosPostingsEnum struct {
	index.PostingsEnumBase
	numDocs int
	docIdx  int
}

func (e *largePosPostingsEnum) NextDoc() (int, error) {
	e.docIdx++
	if e.docIdx >= e.numDocs {
		return index.NO_MORE_DOCS, nil
	}
	return e.docIdx, nil
}

func (e *largePosPostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := e.NextDoc()
		if err != nil || d == index.NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}

func (e *largePosPostingsEnum) DocID() int { return e.docIdx }

func (e *largePosPostingsEnum) Freq() (int, error) { return 1, nil }

func (e *largePosPostingsEnum) NextPosition() (int, error) {
	return e.docIdx, nil // position == docID (arbitrary but deterministic)
}

func (e *largePosPostingsEnum) StartOffset() (int, error) { return -1, nil }
func (e *largePosPostingsEnum) EndOffset() (int, error)   { return -1, nil }
func (e *largePosPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}
func (e *largePosPostingsEnum) Cost() int64 { return int64(e.numDocs) }

// largePosTermsEnum is a TermsEnum over one term backed by largePosPostingsEnum.
type largePosTermsEnum struct {
	index.TermsEnumBase
	term    *index.Term
	numDocs int
	pos     int // -1 = before first; 0 = positioned
}

func (e *largePosTermsEnum) Next() (*index.Term, error) {
	e.pos++
	if e.pos > 0 {
		return nil, nil
	}
	return e.term, nil
}

func (e *largePosTermsEnum) Term() *index.Term { return e.term }

func (e *largePosTermsEnum) DocFreq() (int, error) { return e.numDocs, nil }

func (e *largePosTermsEnum) TotalTermFreq() (int64, error) { return int64(e.numDocs), nil }

func (e *largePosTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

func (e *largePosTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, nil
}

func (e *largePosTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return &largePosPostingsEnum{numDocs: e.numDocs, docIdx: -1}, nil
}

func (e *largePosTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// largePosTerms is a Terms over one term backed by largePosTermsEnum.
type largePosTerms struct {
	index.TermsBase
	term    *index.Term
	numDocs int
}

func (t *largePosTerms) GetIterator() (index.TermsEnum, error) {
	return &largePosTermsEnum{term: t.term, numDocs: t.numDocs, pos: -1}, nil
}

func (t *largePosTerms) HasFreqs() bool     { return true }
func (t *largePosTerms) HasPositions() bool { return true }
func (t *largePosTerms) HasOffsets() bool   { return false }
func (t *largePosTerms) HasPayloads() bool  { return false }

func (t *largePosTerms) GetIteratorWithSeek(_ *index.Term) (index.TermsEnum, error) {
	return t.GetIterator()
}

func (t *largePosTerms) GetMin() (*index.Term, error) { return t.term, nil }
func (t *largePosTerms) GetMax() (*index.Term, error) { return t.term, nil }

func (t *largePosTerms) GetPostingsReader(termText string, _ int) (index.PostingsEnum, error) {
	if termText != t.term.Text() {
		return nil, nil
	}
	return &largePosPostingsEnum{numDocs: t.numDocs, docIdx: -1}, nil
}

// largePosFields wraps largePosTerms as index.Fields.
type largePosFields struct {
	fieldName string
	terms     *largePosTerms
}

func (f *largePosFields) Names() []string { return []string{f.fieldName} }

func (f *largePosFields) Terms(field string) (index.Terms, error) {
	if field == f.fieldName {
		return f.terms, nil
	}
	return nil, nil
}

// TestLucene104PostingsFormat_LastPosBlockOffset_NonZero exercises the
// lastPosBlockOffset >= 0 path: totalTermFreq > BLOCK_SIZE (128).
// A single term is written with 130 documents each carrying freq=1,
// forcing two position blocks and a non-zero lastPosBlockOffset.
func TestLucene104PostingsFormat_LastPosBlockOffset_NonZero(t *testing.T) {
	const numDocs = 130 // totalTermFreq = 130 > lucene104BlockSize(128)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentName := "_0"
	segmentID := make([]byte, 16)
	si := index.NewSegmentInfo(segmentName, numDocs+1, dir)
	si.SetID(segmentID)

	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
	})
	fieldInfos := index.NewFieldInfos()
	fieldInfos.Add(fi)

	writeState := &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	format := NewLucene104PostingsFormat()
	consumer, err := format.FieldsConsumer(writeState)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	term := index.NewTerm("f", "word")
	fields := &largePosFields{
		fieldName: "f",
		terms:     &largePosTerms{term: term, numDocs: numDocs},
	}

	if err := consumer.Write("f", fields.terms); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- Read phase ---
	readState := &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}
	producer, err := format.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	terms, err := producer.Terms("f")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms is nil")
	}

	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	term2, err := te.Next()
	if err != nil || term2 == nil {
		t.Fatalf("Next: err=%v term=%v", err, term2)
	}
	if term2.Text() != "word" {
		t.Fatalf("term text: got %q, want %q", term2.Text(), "word")
	}

	pe, err := te.Postings(index.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}

	// Advance to doc 128 (in the second position block — beyond BLOCK_SIZE=128).
	docID, err := pe.Advance(128)
	if err != nil {
		t.Fatalf("Advance(128): %v", err)
	}
	if docID != 128 {
		t.Fatalf("Advance(128): got docID %d", docID)
	}
	// Freq should be 1.
	freq, err := pe.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	if freq != 1 {
		t.Fatalf("Freq: got %d, want 1", freq)
	}
	// NextPosition should succeed exactly once and return position=128.
	pos, err := pe.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	if pos == index.NO_MORE_POSITIONS {
		t.Fatal("NextPosition returned NO_MORE_POSITIONS on first call")
	}
	if pos != 128 {
		t.Fatalf("NextPosition: got pos %d, want 128", pos)
	}
}

// ─── impacts test helpers ─────────────────────────────────────────────────────

// varyingFreqPostingsEnum yields numDocs documents with freq = (docIdx%7)+1.
// It exercises writeImpacts because distinct freq values produce a non-trivial
// CompetitiveImpactAccumulator outcome on the writer side.
type varyingFreqPostingsEnum struct {
	index.PostingsEnumBase
	numDocs int
	docIdx  int
	freq    int
}

func (e *varyingFreqPostingsEnum) NextDoc() (int, error) {
	e.docIdx++
	if e.docIdx >= e.numDocs {
		return index.NO_MORE_DOCS, nil
	}
	e.freq = (e.docIdx % 7) + 1
	return e.docIdx, nil
}

func (e *varyingFreqPostingsEnum) Advance(target int) (int, error) {
	for {
		d, err := e.NextDoc()
		if err != nil || d == index.NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}

func (e *varyingFreqPostingsEnum) DocID() int                  { return e.docIdx }
func (e *varyingFreqPostingsEnum) Freq() (int, error)          { return e.freq, nil }
func (e *varyingFreqPostingsEnum) NextPosition() (int, error)  { return -1, nil }
func (e *varyingFreqPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *varyingFreqPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *varyingFreqPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (e *varyingFreqPostingsEnum) Cost() int64                 { return int64(e.numDocs) }

// varyingFreqTermsEnum is a TermsEnum over one term backed by varyingFreqPostingsEnum.
type varyingFreqTermsEnum struct {
	index.TermsEnumBase
	term    *index.Term
	numDocs int
	pos     int // -1 = before first; 0 = positioned
}

func (e *varyingFreqTermsEnum) Next() (*index.Term, error) {
	e.pos++
	if e.pos > 0 {
		return nil, nil
	}
	return e.term, nil
}

func (e *varyingFreqTermsEnum) Term() *index.Term { return e.term }

func (e *varyingFreqTermsEnum) DocFreq() (int, error) { return e.numDocs, nil }

func (e *varyingFreqTermsEnum) TotalTermFreq() (int64, error) {
	// Approximate: each doc has freq 1..7 cycling; average 4.
	return int64(e.numDocs) * 4, nil
}

func (e *varyingFreqTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

func (e *varyingFreqTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, nil
}

func (e *varyingFreqTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return &varyingFreqPostingsEnum{numDocs: e.numDocs, docIdx: -1}, nil
}

func (e *varyingFreqTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// varyingFreqTerms is a Terms over one term backed by varyingFreqTermsEnum.
type varyingFreqTerms struct {
	index.TermsBase
	term    *index.Term
	numDocs int
}

func (t *varyingFreqTerms) GetIterator() (index.TermsEnum, error) {
	return &varyingFreqTermsEnum{term: t.term, numDocs: t.numDocs, pos: -1}, nil
}

func (t *varyingFreqTerms) HasFreqs() bool     { return true }
func (t *varyingFreqTerms) HasPositions() bool { return false }
func (t *varyingFreqTerms) HasOffsets() bool   { return false }
func (t *varyingFreqTerms) HasPayloads() bool  { return false }

func (t *varyingFreqTerms) GetIteratorWithSeek(_ *index.Term) (index.TermsEnum, error) {
	return t.GetIterator()
}

func (t *varyingFreqTerms) GetMin() (*index.Term, error) { return t.term, nil }
func (t *varyingFreqTerms) GetMax() (*index.Term, error) { return t.term, nil }

func (t *varyingFreqTerms) GetPostingsReader(termText string, _ int) (index.PostingsEnum, error) {
	if termText != t.term.Text() {
		return nil, nil
	}
	return &varyingFreqPostingsEnum{numDocs: t.numDocs, docIdx: -1}, nil
}

// varyingFreqFields wraps varyingFreqTerms as index.Fields.
type varyingFreqFields struct {
	fieldName string
	terms     *varyingFreqTerms
}

func (f *varyingFreqFields) Names() []string { return []string{f.fieldName} }

func (f *varyingFreqFields) Terms(field string) (index.Terms, error) {
	if field == f.fieldName {
		return f.terms, nil
	}
	return nil, nil
}

// TestLucene104PostingsReader_Impacts verifies that blockPostingsEnum satisfies
// index.ImpactsEnum with a non-trivial getImpacts() result.
//
// The test writes 200 documents with varying term frequencies (freq = docIdx%7+1),
// which causes Lucene104PostingsWriter to record non-trivial impact data in the
// level-0 skip entries (multiple distinct freq values collapse to the competitive
// set).  After opening via Impacts(), AdvanceShallow is called for a doc in the
// second block (doc 200); GetImpacts() must then return at least one competitive
// (freq, norm) pair with freq > 0 and norm >= 0.
func TestLucene104PostingsReader_Impacts(t *testing.T) {
	const numDocs = 200

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentName := "_0"
	segmentID := make([]byte, 16)
	si := index.NewSegmentInfo(segmentName, numDocs+1, dir)
	si.SetID(segmentID)

	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqs,
	})
	fieldInfos := index.NewFieldInfos()
	fieldInfos.Add(fi)

	writeState := &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	format := NewLucene104PostingsFormat()
	consumer, err := format.FieldsConsumer(writeState)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	term := index.NewTerm("f", "impact")
	fields := &varyingFreqFields{
		fieldName: "f",
		terms:     &varyingFreqTerms{term: term, numDocs: numDocs},
	}

	if err := consumer.Write("f", fields.terms); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- Read phase via ImpactsEnum ---
	readState := &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}
	producer, err := format.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// Retrieve the raw Lucene104PostingsReader to call Impacts() directly.
	terms, err := producer.Terms("f")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms is nil")
	}

	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	term2, err := te.Next()
	if err != nil || term2 == nil {
		t.Fatalf("Next: err=%v term=%v", err, term2)
	}
	if term2.Text() != "impact" {
		t.Fatalf("term text: got %q, want %q", term2.Text(), "impact")
	}

	// te is a Lucene103SegmentTermsEnum backed by our Lucene104PostingsReader.
	// Postings() returns a *blockPostingsEnum which also implements ImpactsEnum.
	pe, err := te.Postings(index.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	ie, ok := pe.(index.ImpactsEnum)
	if !ok {
		t.Fatalf("Postings() result does not implement ImpactsEnum; type=%T", pe)
	}

	// AdvanceShallow to the second block (doc 150 is beyond block boundary 128).
	if err := ie.AdvanceShallow(150); err != nil {
		t.Fatalf("AdvanceShallow(150): %v", err)
	}

	// GetImpacts must return a non-trivial Impacts.
	impacts, err := ie.GetImpacts()
	if err != nil {
		t.Fatalf("GetImpacts: %v", err)
	}
	if impacts == nil {
		t.Fatal("GetImpacts returned nil")
	}

	numLevels := impacts.NumLevels()
	if numLevels < 1 {
		t.Fatalf("NumLevels() = %d, want >= 1", numLevels)
	}

	// Level-0 impacts must be non-empty.
	buf := impacts.GetImpacts(0)
	if buf == nil {
		t.Fatal("GetImpacts(0) returned nil buffer")
	}
	if buf.Size < 1 {
		t.Fatalf("GetImpacts(0).Size = %d, want >= 1", buf.Size)
	}
	// Verify the impacts are well-formed: freq > 0 and norm >= 0.
	for i := 0; i < buf.Size; i++ {
		if buf.Freqs[i] <= 0 {
			t.Errorf("impacts[%d].Freq = %d, want > 0", i, buf.Freqs[i])
		}
		if buf.Norms[i] < 0 {
			t.Errorf("impacts[%d].Norm = %d, want >= 0", i, buf.Norms[i])
		}
	}
	// With freq=docIdx%7+1 ranging 1..7, the competitive set must include at
	// least the maximum freq seen in the first block, so max freq >= 7.
	maxFreq := 0
	for i := 0; i < buf.Size; i++ {
		if buf.Freqs[i] > maxFreq {
			maxFreq = buf.Freqs[i]
		}
	}
	if maxFreq < 7 {
		t.Errorf("max freq in impacts = %d, want >= 7 (freqs cycle 1..7)", maxFreq)
	}
}
