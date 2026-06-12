// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Round-trip tests for the test-only Lucene103 postings writer, which lives in
// lucene103_postings_test_writer.go so it can be reused by compat tests.

package codecs

import (
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// l103Doc is a single document within an l103Term, carrying explicit
// per-position payloads and offsets.
type l103Doc struct {
	docID     int
	positions []int
	payloads  [][]byte
	offsets   []l103Off
}

type l103Off struct{ start, end int }

type l103Term struct {
	text string
	docs []l103Doc
}

type l103Terms struct {
	field      string
	terms      []*l103Term // sorted by text
	hasFreqs   bool
	hasPos     bool
	hasOffsets bool
	hasPay     bool
}

func (t *l103Terms) GetIterator() (index.TermsEnum, error) {
	return &l103TermsEnum{parent: t, pos: -1}, nil
}
func (t *l103Terms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te := &l103TermsEnum{parent: t, pos: -1}
	if _, err := te.SeekCeil(seekTerm); err != nil {
		return nil, err
	}
	return te, nil
}
func (t *l103Terms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	for _, tm := range t.terms {
		if tm.text == termText {
			return &l103PostingsEnum{parent: t, term: tm, pos: -1}, nil
		}
	}
	return nil, nil
}
func (t *l103Terms) Size() int64 { return int64(len(t.terms)) }
func (t *l103Terms) GetDocCount() (int, error) {
	set := map[int]struct{}{}
	for _, tm := range t.terms {
		for _, d := range tm.docs {
			set[d.docID] = struct{}{}
		}
	}
	return len(set), nil
}
func (t *l103Terms) GetSumDocFreq() (int64, error) {
	var n int64
	for _, tm := range t.terms {
		n += int64(len(tm.docs))
	}
	return n, nil
}
func (t *l103Terms) GetSumTotalTermFreq() (int64, error) {
	if !t.hasFreqs {
		return -1, nil
	}
	var n int64
	for _, tm := range t.terms {
		for _, d := range tm.docs {
			if t.hasPos {
				n += int64(len(d.positions))
			} else {
				n++
			}
		}
	}
	return n, nil
}
func (t *l103Terms) HasFreqs() bool     { return t.hasFreqs }
func (t *l103Terms) HasOffsets() bool   { return t.hasOffsets }
func (t *l103Terms) HasPositions() bool { return t.hasPos }
func (t *l103Terms) HasPayloads() bool  { return t.hasPay }
func (t *l103Terms) GetMin() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return index.NewTerm(t.field, t.terms[0].text), nil
}
func (t *l103Terms) GetMax() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return index.NewTerm(t.field, t.terms[len(t.terms)-1].text), nil
}

type l103TermsEnum struct {
	index.TermsEnumBase
	parent *l103Terms
	pos    int
	curr   *index.Term
}

func (m *l103TermsEnum) Next() (*index.Term, error) {
	m.pos++
	if m.pos >= len(m.parent.terms) {
		m.curr = nil
		return nil, nil
	}
	m.curr = index.NewTerm(m.parent.field, m.parent.terms[m.pos].text)
	return m.curr, nil
}
func (m *l103TermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	for i, tm := range m.parent.terms {
		if tm.text >= term.Text() {
			m.pos = i
			m.curr = index.NewTerm(m.parent.field, tm.text)
			return m.curr, nil
		}
	}
	m.pos = len(m.parent.terms)
	m.curr = nil
	return nil, nil
}
func (m *l103TermsEnum) SeekExact(term *index.Term) (bool, error) {
	got, err := m.SeekCeil(term)
	return err == nil && got != nil && got.Equals(term), err
}
func (m *l103TermsEnum) Term() *index.Term { return m.curr }
func (m *l103TermsEnum) DocFreq() (int, error) {
	if m.curr == nil {
		return 0, nil
	}
	return len(m.parent.terms[m.pos].docs), nil
}
func (m *l103TermsEnum) TotalTermFreq() (int64, error) {
	if m.curr == nil {
		return 0, nil
	}
	var n int64
	for _, d := range m.parent.terms[m.pos].docs {
		if m.parent.hasPos {
			n += int64(len(d.positions))
		} else {
			n++
		}
	}
	return n, nil
}
func (m *l103TermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if m.curr == nil {
		return nil, nil
	}
	return &l103PostingsEnum{parent: m.parent, term: m.parent.terms[m.pos], pos: -1}, nil
}
func (m *l103TermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return m.Postings(flags)
}

type l103PostingsEnum struct {
	index.PostingsEnumBase
	parent  *l103Terms
	term    *l103Term
	pos     int
	posIdx  int
	currDoc int
}

func (p *l103PostingsEnum) NextDoc() (int, error) {
	p.pos++
	p.posIdx = 0
	if p.pos >= len(p.term.docs) {
		p.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	p.currDoc = p.term.docs[p.pos].docID
	return p.currDoc, nil
}
func (p *l103PostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil || doc >= target || doc == index.NO_MORE_DOCS {
			return doc, err
		}
	}
}
func (p *l103PostingsEnum) DocID() int {
	if p.pos < 0 {
		return -1
	}
	return p.currDoc
}
func (p *l103PostingsEnum) Freq() (int, error) {
	if p.pos < 0 || p.pos >= len(p.term.docs) {
		return 0, nil
	}
	if p.parent.hasPos {
		return len(p.term.docs[p.pos].positions), nil
	}
	return 1, nil
}
func (p *l103PostingsEnum) NextPosition() (int, error) {
	if p.pos < 0 || p.pos >= len(p.term.docs) {
		return index.NO_MORE_POSITIONS, nil
	}
	positions := p.term.docs[p.pos].positions
	if p.posIdx >= len(positions) {
		return index.NO_MORE_POSITIONS, nil
	}
	pos := positions[p.posIdx]
	p.posIdx++
	return pos, nil
}
func (p *l103PostingsEnum) StartOffset() (int, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].offsets) {
		return -1, nil
	}
	return p.term.docs[p.pos].offsets[idx].start, nil
}
func (p *l103PostingsEnum) EndOffset() (int, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].offsets) {
		return -1, nil
	}
	return p.term.docs[p.pos].offsets[idx].end, nil
}
func (p *l103PostingsEnum) GetPayload() ([]byte, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].payloads) {
		return nil, nil
	}
	return p.term.docs[p.pos].payloads[idx], nil
}
func (p *l103PostingsEnum) Cost() int64 { return int64(len(p.term.docs)) }

// l103WriteState builds a write state whose FieldInfos reflect the requested
// options (including stored payloads).
func l103WriteState(t *testing.T, dir store.Directory, name, field string, opts index.IndexOptions, storePayloads bool) *SegmentWriteState {
	t.Helper()
	si := index.NewSegmentInfo(name, 100, dir)
	if err := si.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	fis := index.NewFieldInfos()
	fi := index.NewFieldInfo(field, 0, index.FieldInfoOptions{IndexOptions: opts})
	if storePayloads {
		fi.SetStorePayloads()
	}
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}
	return &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
}

// l103RoundTrip writes terms via the test-only RW format, then reads them back
// via the production read-only format and asserts an exact match.
func l103RoundTrip(t *testing.T, opts index.IndexOptions, storePayloads bool, terms *l103Terms) {
	t.Helper()
	// The block-tree writer requires terms in ascending (BytesRef/UTF-8) order;
	// for our ASCII term texts that is plain string order.
	sort.Slice(terms.terms, func(i, j int) bool { return terms.terms[i].text < terms.terms[j].text })

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	ws := l103WriteState(t, dir, "_0", terms.field, opts, storePayloads)

	rw := NewLucene103RWPostingsFormat()
	consumer, err := rw.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.Write(terms.field, terms); err != nil {
		_ = consumer.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read back through the PRODUCTION read-only format + production reader.
	prod := NewLucene103PostingsFormat()
	rs := &SegmentReadState{Directory: ws.Directory, SegmentInfo: ws.SegmentInfo, FieldInfos: ws.FieldInfos}
	producer, err := prod.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// CheckIntegrity walks the block-tree reader, which delegates to the
	// production Lucene103PostingsReader.CheckIntegrity (CRC footer validation
	// over .doc/.pos/.pay). FieldsProducer (the SPI alias) does not surface
	// CheckIntegrity, so assert to the concrete reader.
	type integrityChecker interface{ CheckIntegrity() error }
	if ic, ok := producer.(integrityChecker); ok {
		if err := ic.CheckIntegrity(); err != nil {
			t.Fatalf("CheckIntegrity: %v", err)
		}
	} else {
		t.Fatalf("FieldsProducer %T does not implement CheckIntegrity", producer)
	}

	l103AssertTerms(t, producer, terms, opts, storePayloads)
}

func l103AssertTerms(t *testing.T, producer FieldsProducer, exp *l103Terms, opts index.IndexOptions, storePayloads bool) {
	t.Helper()
	terms, err := producer.Terms(exp.field)
	if err != nil || terms == nil {
		t.Fatalf("Terms(%q): %v (nil=%v)", exp.field, err, terms == nil)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	hasFreqs := opts >= index.IndexOptionsDocsAndFreqs
	hasPos := opts >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets

	flags := index.PostingsFlagFreqs
	if hasPos {
		flags = index.PostingsFlagPositions
	}
	if hasOffsets {
		flags |= index.PostingsFlagOffsets
	}
	if storePayloads {
		flags |= index.PostingsFlagPayloads
	}

	for _, want := range exp.terms {
		found, err := te.SeekExact(index.NewTerm(exp.field, want.text))
		if err != nil || !found {
			t.Fatalf("SeekExact(%q): found=%v err=%v", want.text, found, err)
		}
		pe, err := te.Postings(flags)
		if err != nil {
			t.Fatalf("Postings(%q): %v", want.text, err)
		}
		for di, wd := range want.docs {
			doc, err := pe.NextDoc()
			if err != nil {
				t.Fatalf("term %q doc[%d] NextDoc: %v", want.text, di, err)
			}
			if doc != wd.docID {
				t.Fatalf("term %q doc[%d]: got %d want %d", want.text, di, doc, wd.docID)
			}
			if hasFreqs {
				wantFreq := 1
				if hasPos {
					wantFreq = len(wd.positions)
				}
				freq, err := pe.Freq()
				if err != nil {
					t.Fatalf("term %q doc[%d] Freq: %v", want.text, di, err)
				}
				if freq != wantFreq {
					t.Fatalf("term %q doc[%d] freq: got %d want %d", want.text, di, freq, wantFreq)
				}
			}
			if hasPos {
				for pi, wpos := range wd.positions {
					pos, err := pe.NextPosition()
					if err != nil {
						t.Fatalf("term %q doc[%d] pos[%d] NextPosition: %v", want.text, di, pi, err)
					}
					if pos != wpos {
						t.Fatalf("term %q doc[%d] pos[%d]: got %d want %d", want.text, di, pi, pos, wpos)
					}
					if hasOffsets {
						so, _ := pe.StartOffset()
						eo, _ := pe.EndOffset()
						if so != wd.offsets[pi].start || eo != wd.offsets[pi].end {
							t.Fatalf("term %q doc[%d] pos[%d] offsets: got (%d,%d) want (%d,%d)",
								want.text, di, pi, so, eo, wd.offsets[pi].start, wd.offsets[pi].end)
						}
					}
					if storePayloads {
						pl, _ := pe.GetPayload()
						wantPl := wd.payloads[pi]
						if len(wantPl) == 0 {
							if len(pl) != 0 {
								t.Fatalf("term %q doc[%d] pos[%d] payload: got %v want empty", want.text, di, pi, pl)
							}
						} else if string(pl) != string(wantPl) {
							t.Fatalf("term %q doc[%d] pos[%d] payload: got %q want %q", want.text, di, pi, pl, wantPl)
						}
					}
				}
			}
		}
		last, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("term %q final NextDoc: %v", want.text, err)
		}
		if last != index.NO_MORE_DOCS {
			t.Fatalf("term %q: expected NO_MORE_DOCS, got %d", want.text, last)
		}
	}
}

// TestLucene103Postings_RoundTrip_AllIndexOptions covers every IndexOptions
// level (DOCS / DOCS_AND_FREQS / +POSITIONS / +OFFSETS) with multiple terms,
// proving the production reader recovers the exact logical input written by the
// test-only writer.
func TestLucene103Postings_RoundTrip_AllIndexOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts index.IndexOptions
	}{
		{"docs", index.IndexOptionsDocs},
		{"docs_freqs", index.IndexOptionsDocsAndFreqs},
		{"docs_freqs_pos", index.IndexOptionsDocsAndFreqsAndPositions},
		{"docs_freqs_pos_off", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hasPos := tc.opts >= index.IndexOptionsDocsAndFreqsAndPositions
			hasOff := tc.opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
			terms := &l103Terms{
				field:      "f",
				hasFreqs:   tc.opts >= index.IndexOptionsDocsAndFreqs,
				hasPos:     hasPos,
				hasOffsets: hasOff,
			}
			// Three terms, each with several docs / varying freqs.
			for ti := 0; ti < 3; ti++ {
				tm := &l103Term{text: fmt.Sprintf("term%d", ti)}
				for d := 0; d < 5; d++ {
					doc := l103Doc{docID: d * 7}
					if hasPos {
						freq := 1 + (ti+d)%3
						pos := 0
						for k := 0; k < freq; k++ {
							pos += 1 + k
							doc.positions = append(doc.positions, pos)
							if hasOff {
								doc.offsets = append(doc.offsets, l103Off{start: k * 6, end: k*6 + 5})
							}
						}
					}
					tm.docs = append(tm.docs, doc)
				}
				terms.terms = append(terms.terms, tm)
			}
			l103RoundTrip(t, tc.opts, false, terms)
		})
	}
}

// TestLucene103Postings_RoundTrip_SingletonDoc exercises the docFreq==1
// singleton optimisation (docID pulsed into the term dictionary).
func TestLucene103Postings_RoundTrip_SingletonDoc(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true}
	// Several singleton terms with distinct doc IDs (also exercises the
	// EncodeTerm singleton-delta path across consecutive ID-like terms).
	for ti, docID := range []int{0, 1, 2, 100, 5000} {
		tm := &l103Term{text: fmt.Sprintf("id%05d", ti)}
		tm.docs = append(tm.docs, l103Doc{docID: docID, positions: []int{0, 3, 9}})
		terms.terms = append(terms.terms, tm)
	}
	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103Postings_RoundTrip_LargeBlocks writes a term with more than
// 2*BLOCK_SIZE docs, forcing multiple full 128-doc FOR-delta + PFOR blocks plus
// a VInt tail, and a term whose totalTermFreq exceeds BLOCK_SIZE (lastPosBlock).
func TestLucene103Postings_RoundTrip_LargeBlocks(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true}

	// 300 docs (> 2*128): two full FOR-delta blocks + a 44-doc VInt tail.
	big := &l103Term{text: "big"}
	for d := 0; d < 300; d++ {
		doc := l103Doc{docID: d * 3} // gaps of 3 keep deltas small (FOR path)
		// One position per doc keeps totalTermFreq == 300 (> BLOCK_SIZE),
		// exercising the lastPosBlockOffset term-state field.
		doc.positions = []int{d % 17}
		big.docs = append(big.docs, doc)
	}
	terms.terms = append(terms.terms, big)

	// A dense, consecutive 128-doc block (docRange == BLOCK_SIZE) exercises the
	// "all deltas == 1" byte-0 fast path of the writer.
	dense := &l103Term{text: "dense"}
	for d := 0; d < lucene103PostingsBlockSize; d++ {
		dense.docs = append(dense.docs, l103Doc{docID: 1000 + d, positions: []int{1}})
	}
	terms.terms = append(terms.terms, dense)

	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103Postings_RoundTrip_Payloads exercises non-empty payloads,
// including a payload-length change within a doc and an empty payload.
func TestLucene103Postings_RoundTrip_Payloads(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true, hasPay: true}

	tm := &l103Term{text: "payterm"}
	tm.docs = append(tm.docs, l103Doc{
		docID:     0,
		positions: []int{0, 5, 9},
		payloads:  [][]byte{[]byte("p0"), []byte("payload-1"), nil},
	})
	tm.docs = append(tm.docs, l103Doc{
		docID:     4,
		positions: []int{2, 8},
		payloads:  [][]byte{[]byte("x"), []byte("yy")},
	})
	terms.terms = append(terms.terms, tm)

	// >128 positions in a single doc to exercise the packed payload block path
	// in .pay (pforUtil.encode of payload lengths + payload bytes).
	bigPay := &l103Term{text: "bigpay"}
	doc := l103Doc{docID: 0}
	for k := 0; k < 200; k++ {
		doc.positions = append(doc.positions, k*2)
		doc.payloads = append(doc.payloads, []byte(fmt.Sprintf("pl%d", k%5)))
	}
	bigPay.docs = append(bigPay.docs, doc)
	terms.terms = append(terms.terms, bigPay)

	l103RoundTrip(t, opts, true, terms)
}

// TestLucene103Postings_RoundTrip_Offsets exercises offsets (with positions),
// including offset-length changes and a >128-position doc (packed offset block).
func TestLucene103Postings_RoundTrip_Offsets(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true, hasOffsets: true}

	tm := &l103Term{text: "offterm"}
	doc := l103Doc{docID: 0}
	ch := 0
	for k := 0; k < 200; k++ {
		doc.positions = append(doc.positions, k)
		length := 3 + (k % 4) // varying offset lengths
		doc.offsets = append(doc.offsets, l103Off{start: ch, end: ch + length})
		ch += length + 1
	}
	tm.docs = append(tm.docs, doc)
	terms.terms = append(terms.terms, tm)

	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103PostingsFormat_FieldsConsumerReadOnly asserts that the PRODUCTION
// format rejects writes, mirroring Apache Lucene's UnsupportedOperationException.
func TestLucene103PostingsFormat_FieldsConsumerReadOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	ws := l103WriteState(t, dir, "_0", "f", index.IndexOptionsDocsAndFreqs, false)
	prod := NewLucene103PostingsFormat()
	c, err := prod.FieldsConsumer(ws)
	if err == nil {
		t.Fatalf("expected read-only error from production FieldsConsumer, got consumer=%v", c)
	}
	if err != errLucene103ReadOnly {
		t.Fatalf("expected errLucene103ReadOnly, got %v", err)
	}
}
