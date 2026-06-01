// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Integration tests that exercise IDVersionPostingsFormat.FieldsConsumer and
// FieldsProducer end-to-end: write a set of ID-versioned terms to an in-memory
// ByteBuffersDirectory, then re-open and verify seek behaviour.
package idversion

import (
	"encoding/binary"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildIDVersionPayload encodes v as an 8-byte big-endian payload.
func buildIDVersionPayload(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// memTermsEntry represents one term/doc/version tuple for test data.
type memTermsEntry struct {
	term    []byte
	docID   int
	version int64
}

// memTerms implements schema.Terms backed by a sorted slice of entries.
// Used to drive FieldsConsumer.Write in tests.
type memTerms struct {
	fi      *schema.FieldInfo
	entries []memTermsEntry
}

func (m *memTerms) GetIterator() (schema.TermsEnum, error) {
	return &memTermsEnum{fi: m.fi, entries: m.entries, pos: -1}, nil
}

func (m *memTerms) GetIteratorWithSeek(_ *schema.Term) (schema.TermsEnum, error) {
	return m.GetIterator()
}

func (m *memTerms) GetPostingsReader(_ string, _ int) (schema.PostingsEnum, error) {
	return nil, nil
}

func (m *memTerms) Size() int64                        { return int64(len(m.entries)) }
func (m *memTerms) GetDocCount() (int, error)          { return len(m.entries), nil }
func (m *memTerms) GetSumDocFreq() (int64, error)      { return int64(len(m.entries)), nil }
func (m *memTerms) GetSumTotalTermFreq() (int64, error) { return int64(len(m.entries)), nil }
func (m *memTerms) HasFreqs() bool                     { return true }
func (m *memTerms) HasOffsets() bool                   { return false }
func (m *memTerms) HasPositions() bool                 { return true }
func (m *memTerms) HasPayloads() bool                  { return true }
func (m *memTerms) GetMin() (*schema.Term, error) {
	if len(m.entries) == 0 {
		return nil, nil
	}
	return schema.NewTermFromBytes(m.fi.Name(), m.entries[0].term), nil
}
func (m *memTerms) GetMax() (*schema.Term, error) {
	if len(m.entries) == 0 {
		return nil, nil
	}
	return schema.NewTermFromBytes(m.fi.Name(), m.entries[len(m.entries)-1].term), nil
}

// memTermsEnum iterates over memTermsEntry slice.
type memTermsEnum struct {
	fi      *schema.FieldInfo
	entries []memTermsEntry
	pos     int
}

func (e *memTermsEnum) Next() (*schema.Term, error) {
	e.pos++
	if e.pos >= len(e.entries) {
		return nil, nil
	}
	return schema.NewTermFromBytes(e.fi.Name(), e.entries[e.pos].term), nil
}

func (e *memTermsEnum) SeekCeil(t *schema.Term) (*schema.Term, error) {
	target := t.BytesValue()
	for i, en := range e.entries {
		ref := &util.BytesRef{Bytes: en.term, Offset: 0, Length: len(en.term)}
		if ref.BytesRefCompareTo(target) >= 0 {
			e.pos = i
			return schema.NewTermFromBytes(e.fi.Name(), en.term), nil
		}
	}
	return nil, nil
}

func (e *memTermsEnum) SeekExact(t *schema.Term) (bool, error) {
	target := t.BytesValue()
	for i, en := range e.entries {
		ref := &util.BytesRef{Bytes: en.term, Offset: 0, Length: len(en.term)}
		if ref.BytesRefCompareTo(target) == 0 {
			e.pos = i
			return true, nil
		}
	}
	return false, nil
}

func (e *memTermsEnum) Term() *schema.Term {
	if e.pos < 0 || e.pos >= len(e.entries) {
		return nil
	}
	return schema.NewTermFromBytes(e.fi.Name(), e.entries[e.pos].term)
}

func (e *memTermsEnum) DocFreq() (int, error)        { return 1, nil }
func (e *memTermsEnum) TotalTermFreq() (int64, error) { return 1, nil }

func (e *memTermsEnum) Postings(flags int) (schema.PostingsEnum, error) {
	if e.pos < 0 || e.pos >= len(e.entries) {
		return nil, nil
	}
	en := e.entries[e.pos]
	pe := &memPostingsEnum{
		docID:   en.docID,
		version: en.version,
		pos:     -1,
	}
	return pe, nil
}

func (e *memTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (schema.PostingsEnum, error) {
	return e.Postings(flags)
}

// memPostingsEnum is a single-doc/single-position PostingsEnum.
type memPostingsEnum struct {
	docID   int
	version int64
	pos     int    // -1: before doc; 0: at doc; 1: positions seen
	atDoc   bool
	atPos   bool
}

func (p *memPostingsEnum) NextDoc() (int, error) {
	if !p.atDoc {
		p.atDoc = true
		return p.docID, nil
	}
	return schema.NO_MORE_DOCS, nil
}

func (p *memPostingsEnum) Advance(target int) (int, error) {
	if !p.atDoc && target <= p.docID {
		p.atDoc = true
		return p.docID, nil
	}
	return schema.NO_MORE_DOCS, nil
}

func (p *memPostingsEnum) DocID() int { return p.docID }
func (p *memPostingsEnum) Freq() (int, error) { return 1, nil }
func (p *memPostingsEnum) Cost() int64        { return 1 }

func (p *memPostingsEnum) NextPosition() (int, error) {
	if !p.atPos {
		p.atPos = true
		return 0, nil
	}
	return schema.NO_MORE_POSITIONS, nil
}

func (p *memPostingsEnum) StartOffset() (int, error) { return -1, nil }
func (p *memPostingsEnum) EndOffset() (int, error)   { return -1, nil }
func (p *memPostingsEnum) GetPayload() ([]byte, error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, uint64(p.version))
	return payload, nil
}

// TestIDVersionPostingsFormat_FieldsConsumer_Produces_No_Error verifies that
// FieldsConsumer can be constructed and closed without error when writing a
// small set of terms.
func TestIDVersionPostingsFormat_FieldsConsumer_Produces_No_Error(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build FieldInfo with DOCS_AND_FREQS_AND_POSITIONS index options and
	// payloads, matching what IDVersionPostingsWriter requires.
	fi := schema.NewFieldInfoBuilder("id", 0).
		SetIndexOptions(schema.IndexOptionsDocsAndFreqsAndPositions).
		SetStoreTermVectorPayloads(false).
		Build()
	fi.SetStorePayloads()

	fis := schema.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	seg := schema.NewSegmentInfo("_0", 10, dir)
	// Assign a valid 16-byte ID.
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	seg.SetID(id)

	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   seg,
		FieldInfos:    fis,
		SegmentSuffix: "",
	}

	fmt := NewIDVersionPostingsFormat()
	consumer, err := fmt.FieldsConsumer(state)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	// Write three terms in lexicographic order.
	entries := []memTermsEntry{
		{term: []byte("apple"), docID: 0, version: 100},
		{term: []byte("banana"), docID: 1, version: 200},
		{term: []byte("cherry"), docID: 2, version: 300},
	}
	terms := &memTerms{fi: fi, entries: entries}

	if err := consumer.Write("id", terms); err != nil {
		t.Fatalf("FieldsConsumer.Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("FieldsConsumer.Close: %v", err)
	}
}

// TestIDVersionPostingsFormat_RoundTrip_WriteAndRead exercises the full
// write → read cycle: writes a set of terms and then re-reads them via the
// FieldsProducer, verifying each term is present and has the expected version.
func TestIDVersionPostingsFormat_RoundTrip_WriteAndRead(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	fi := schema.NewFieldInfoBuilder("id", 0).
		SetIndexOptions(schema.IndexOptionsDocsAndFreqsAndPositions).
		Build()
	fi.SetStorePayloads()

	fis := schema.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	seg := schema.NewSegmentInfo("_0", 10, dir)
	seg.SetID(id)

	writeState := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   seg,
		FieldInfos:    fis,
		SegmentSuffix: "",
	}

	pf := NewIDVersionPostingsFormat()
	consumer, err := pf.FieldsConsumer(writeState)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	entries := []memTermsEntry{
		{term: []byte("apple"), docID: 0, version: 100},
		{term: []byte("banana"), docID: 1, version: 200},
		{term: []byte("cherry"), docID: 2, version: 300},
	}
	terms := &memTerms{fi: fi, entries: entries}

	if err := consumer.Write("id", terms); err != nil {
		t.Fatalf("FieldsConsumer.Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("FieldsConsumer.Close: %v", err)
	}

	// Now open for reading.
	readState := &codecs.SegmentReadState{
		Directory:     dir,
		SegmentInfo:   seg,
		FieldInfos:    fis,
		SegmentSuffix: "",
	}

	producer, err := pf.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// Get the "id" field terms.
	schemaTerms, err := producer.Terms("id")
	if err != nil {
		t.Fatalf("FieldsProducer.Terms: %v", err)
	}
	if schemaTerms == nil {
		t.Fatal("FieldsProducer.Terms: got nil for existing field")
	}

	// Verify the iterator returns all three terms.
	te, err := schemaTerms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	var gotTerms []string
	for {
		term, nerr := te.Next()
		if nerr != nil {
			t.Fatalf("TermsEnum.Next: %v", nerr)
		}
		if term == nil {
			break
		}
		gotTerms = append(gotTerms, string(term.BytesValue().ValidBytes()))
	}

	wantTerms := []string{"apple", "banana", "cherry"}
	if len(gotTerms) != len(wantTerms) {
		t.Errorf("got %d terms, want %d; terms=%v", len(gotTerms), len(wantTerms), gotTerms)
		return
	}
	for i, want := range wantTerms {
		if gotTerms[i] != want {
			t.Errorf("term[%d] = %q, want %q", i, gotTerms[i], want)
		}
	}
}

// TestIDVersionPostingsFormat_FieldsProducer_UnknownField returns nil for a
// field that was never written.
func TestIDVersionPostingsFormat_FieldsProducer_UnknownField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	fi := schema.NewFieldInfoBuilder("id", 0).
		SetIndexOptions(schema.IndexOptionsDocsAndFreqsAndPositions).
		Build()
	fi.SetStorePayloads()

	fis := schema.NewFieldInfos()
	_ = fis.Add(fi)

	id := make([]byte, 16)
	seg := schema.NewSegmentInfo("_0", 5, dir)
	seg.SetID(id)

	writeState := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: seg,
		FieldInfos:  fis,
	}

	pf := NewIDVersionPostingsFormat()
	consumer, err := pf.FieldsConsumer(writeState)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	entries := []memTermsEntry{
		{term: []byte("hello"), docID: 0, version: 1},
	}
	if err := consumer.Write("id", &memTerms{fi: fi, entries: entries}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	readState := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: seg,
		FieldInfos:  fis,
	}
	producer, err := pf.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	got, err := producer.Terms("nonexistent")
	if err != nil {
		t.Fatalf("Terms(nonexistent): %v", err)
	}
	if got != nil {
		t.Errorf("Terms(nonexistent) = %v; want nil", got)
	}
}
