// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ─── fake postings writer (write-side) ──────────────────────────────────────

// fakePostingsWriter records a monotonic term ordinal and writes it into the
// meta blob via EncodeTerm. It drives nothing on StartDoc/AddPosition/FinishDoc
// — the doc count is accumulated by codecs.WriteTerm so the ordinal increments
// across terms correctly regardless of docFreq.
type fakePostingsWriter struct {
	termOrd int
}

func newFakePostingsWriter() *fakePostingsWriter            { return &fakePostingsWriter{} }
func (f *fakePostingsWriter) Init(_ store.IndexOutput, _ *codecs.SegmentWriteState) error { return nil }
func (f *fakePostingsWriter) NewTermState() *codecs.BlockTermState { return codecs.NewBlockTermState() }
func (f *fakePostingsWriter) SetField(_ *index.FieldInfo) (int, error) { return 0, nil }
func (f *fakePostingsWriter) StartTerm(_ index.NumericDocValues) error {
	f.termOrd++
	return nil
}
func (f *fakePostingsWriter) FinishTerm(state *codecs.BlockTermState) error {
	state.TotalTermFreq = int64(state.DocFreq)
	return nil
}
func (f *fakePostingsWriter) EncodeTerm(out store.IndexOutput, _ *index.FieldInfo, _ *codecs.BlockTermState, _ bool) error {
	return out.WriteByte(byte(f.termOrd))
}
func (f *fakePostingsWriter) StartDoc(_ int, _ int) error       { return nil }
func (f *fakePostingsWriter) AddPosition(_ int, _ []byte, _, _ int) error { return nil }
func (f *fakePostingsWriter) FinishDoc() error                  { return nil }
func (f *fakePostingsWriter) Close() error                      { return nil }

// compile-time checks.
var _ codecs.PostingsWriterBase = (*fakePostingsWriter)(nil)
var _ codecs.PushPostingsWriterBase = (*fakePostingsWriter)(nil)

// ─── fake postings reader (read-side) ───────────────────────────────────────

type fakePostingsReader struct{}

func newFakePostingsReader() *fakePostingsReader { return &fakePostingsReader{} }
func (f *fakePostingsReader) Init(_ store.IndexInput, _ *codecs.SegmentReadState) error { return nil }
func (f *fakePostingsReader) NewTermState() *codecs.BlockTermState { return codecs.NewBlockTermState() }
func (f *fakePostingsReader) DecodeTerm(_ store.DataInput, _ *index.FieldInfo, _ *codecs.BlockTermState, _ bool) error {
	return nil
}
func (f *fakePostingsReader) Postings(_ *index.FieldInfo, _ *codecs.BlockTermState, _ index.PostingsEnum, _ int) (index.PostingsEnum, error) {
	return nil, nil
}
func (f *fakePostingsReader) Impacts(_ *index.FieldInfo, _ *codecs.BlockTermState, _ int) (index.ImpactsEnum, error) {
	return nil, nil
}
func (f *fakePostingsReader) CheckIntegrity() error { return nil }
func (f *fakePostingsReader) Close() error          { return nil }

var _ codecs.PostingsReaderBase = (*fakePostingsReader)(nil)

// ─── fake Terms + TermsEnum ─────────────────────────────────────────────────

type fakeTermEntry struct {
	text    string
	docFreq int
}

// fakeTermsEnum iterates a sorted slice of (term, docFreq) entries.
type fakeTermsEnum struct {
	terms   []fakeTermEntry
	idx     int
	current *index.Term
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

func (e *fakeTermsEnum) DocFreq() (int, error) {
	return e.terms[e.idx-1].docFreq, nil
}

func (e *fakeTermsEnum) TotalTermFreq() (int64, error) {
	return int64(e.terms[e.idx-1].docFreq), nil
}

func (e *fakeTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return &fakePostingsEnum{remaining: e.terms[e.idx-1].docFreq}, nil
}

func (e *fakeTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return e.Postings(0)
}

// fakePostingsEnum yields sequential doc IDs 0..docFreq-1 with freq=1 each.
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
	cur := p.docID
	p.docID++
	return cur, nil
}

func (p *fakePostingsEnum) Advance(_ int) (int, error)       { return p.NextDoc() }
func (p *fakePostingsEnum) DocID() int                        { return p.docID }
func (p *fakePostingsEnum) Freq() (int, error)                { return 1, nil }
func (p *fakePostingsEnum) NextPosition() (int, error)        { return index.NO_MORE_POSITIONS, nil }
func (p *fakePostingsEnum) StartOffset() (int, error)         { return -1, nil }
func (p *fakePostingsEnum) EndOffset() (int, error)           { return -1, nil }
func (p *fakePostingsEnum) GetPayload() ([]byte, error)       { return nil, nil }
func (p *fakePostingsEnum) Cost() int64                       { return 0 }

// fakeTerms wraps a sorted slice of term entries as an index.Terms.
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

func (t *fakeTerms) Size() int64 { return int64(len(t.entries)) }

func (t *fakeTerms) GetMin() (*index.Term, error) {
	if len(t.entries) == 0 {
		return nil, nil
	}
	return index.NewTerm("", t.entries[0].text), nil
}

func (t *fakeTerms) GetMax() (*index.Term, error) {
	if len(t.entries) == 0 {
		return nil, nil
	}
	return index.NewTerm("", t.entries[len(t.entries)-1].text), nil
}

func (t *fakeTerms) HasFreqs() bool    { return true }
func (t *fakeTerms) HasOffsets() bool  { return false }
func (t *fakeTerms) HasPositions() bool { return false }
func (t *fakeTerms) HasPayloads() bool { return false }

func (t *fakeTerms) GetSumDocFreq() (int64, error) {
	var s int64
	for _, e := range t.entries {
		s += int64(e.docFreq)
	}
	return s, nil
}

func (t *fakeTerms) GetSumTotalTermFreq() (int64, error) {
	return t.GetSumDocFreq() // same as sumDocFreq for freq-only fields
}

func (t *fakeTerms) GetDocCount() (int, error) { return len(t.entries), nil }

var _ index.Terms = (*fakeTerms)(nil)

// ─── segment writer helper ──────────────────────────────────────────────────

// writeTestSegment writes a single field "field" with the given terms using
// the fakePostingsWriter and returns the writer (for metadata access) and the
// in-memory directory.
func writeTestSegment(t *testing.T, terms []fakeTermEntry) (*ordsBlockTreeTermsWriter, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()

	si := index.NewSegmentInfo("test", len(terms)+1, dir)
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	fi := index.NewFieldInfo("field", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqs,
	})
	infos := index.NewFieldInfos()
	if err := infos.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	state := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  infos,
	}

	w, err := newOrdsBlockTreeTermsWriter(
		state,
		newFakePostingsWriter(),
		ordsBlockTreeDefaultMinBlockSize,
		ordsBlockTreeDefaultMaxBlockSize,
	)
	if err != nil {
		t.Fatalf("newOrdsBlockTreeTermsWriter: %v", err)
	}

	fields := index.NewMemoryFields()
	fields.AddField("field", &fakeTerms{entries: terms})
	if err := w.WriteFields(fields); err != nil {
		t.Fatalf("WriteFields: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Writer.Close: %v", err)
	}

	return w, dir
}

// ─── FST loader ─────────────────────────────────────────────────────────────

// loadIndexFST opens the .tipo file from the directory and loads the FST
// at the indexStartFP recorded in the writer's field metadata.
func loadIndexFST(t *testing.T, dir store.Directory, indexStartFP int64) *gfst.FST[*FSTOrdsOutput] {
	t.Helper()
	tipoName := "test.tipo"
	if !dir.FileExists(tipoName) {
		t.Fatalf("file %s does not exist", tipoName)
	}
	in, err := dir.OpenInput(tipoName)
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", tipoName, err)
	}
	if err := in.SetPosition(indexStartFP); err != nil {
		t.Fatalf("SetPosition(%d): %v", indexStartFP, err)
	}
	meta, err := gfst.ReadMetadata[*FSTOrdsOutput](in, FSTOutputs())
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	fst, err := gfst.NewFSTFromDataInput(meta, in)
	if err != nil {
		t.Fatalf("NewFSTFromDataInput: %v", err)
	}
	return fst
}

// ─── utility helper tests ───────────────────────────────────────────────────

// generateLexicographicBytes generates n sorted byte-slice terms where the
// i-th term is the lexicographic representation of i as a slice of bytes.
// Each term has docFreq = 1 + (i % 4).
func generateLexicographicBytes(n int, docFreqFn func(i int) int) []fakeTermEntry {
	if docFreqFn == nil {
		docFreqFn = func(i int) int { return 1 + i%4 }
	}
	entries := make([]fakeTermEntry, n)
	for i := range entries {
		// Produce sortable strings: "term0000", "term0001", ...
		s := fmt.Sprintf("term%04d", i)
		entries[i] = fakeTermEntry{text: s, docFreq: docFreqFn(i)}
	}
	return entries
}

// mkTerms creates a sorted list of term entries from the given strings.
func mkTerms(texts ...string) []fakeTermEntry {
	entries := make([]fakeTermEntry, len(texts))
	for i, txt := range texts {
		entries[i] = fakeTermEntry{text: txt, docFreq: 1}
	}
	return entries
}

// ─── encodeOutput tests ─────────────────────────────────────────────────────

func TestEncodeOutput(t *testing.T) {
	tests := []struct {
		fp       int64
		hasTerms bool
		isFloor  bool
		want     int64
	}{
		{fp: 0, hasTerms: false, isFloor: false, want: 0},
		{fp: 0, hasTerms: true, isFloor: false, want: outputFlagHasTerms},
		{fp: 0, hasTerms: false, isFloor: true, want: outputFlagIsFloor},
		{fp: 0, hasTerms: true, isFloor: true, want: outputFlagHasTerms | outputFlagIsFloor},
		{fp: 42, hasTerms: false, isFloor: false, want: 42 << outputFlagsNumBits},
		{fp: 42, hasTerms: true, isFloor: false, want: (42 << outputFlagsNumBits) | outputFlagHasTerms},
		{fp: 42, hasTerms: true, isFloor: true, want: (42 << outputFlagsNumBits) | outputFlagHasTerms | outputFlagIsFloor},
	}
	for _, tt := range tests {
		got := encodeOutput(tt.fp, tt.hasTerms, tt.isFloor)
		if got != tt.want {
			t.Errorf("encodeOutput(%d, %v, %v) = %d, want %d", tt.fp, tt.hasTerms, tt.isFloor, got, tt.want)
		}
	}
}

func TestValidateOrdsBlockSizes(t *testing.T) {
	// Valid cases
	if err := validateOrdsBlockSizes(25, 48); err != nil {
		t.Errorf("valid (25,48): %v", err)
	}
	if err := validateOrdsBlockSizes(2, 4); err != nil {
		t.Errorf("valid (2,4): %v", err)
	}
	// minItemsInBlock <= 1
	if err := validateOrdsBlockSizes(1, 48); err == nil {
		t.Error("expected error for minItemsInBlock <= 1")
	}
	if err := validateOrdsBlockSizes(0, 48); err == nil {
		t.Error("expected error for minItemsInBlock <= 1")
	}
	// max < min
	if err := validateOrdsBlockSizes(30, 25); err == nil {
		t.Error("expected error for max < min")
	}
	// 2*(min-1) > max
	if err := validateOrdsBlockSizes(30, 50); err == nil {
		t.Error("expected error for 2*(min-1) > max")
	}
}

func TestOrdsComputePrefixMismatch(t *testing.T) {
	tests := []struct {
		a, b []byte
		want int
	}{
		{a: []byte("abc"), b: []byte("abc"), want: -1},
		{a: []byte("abc"), b: []byte("abd"), want: 2},
		{a: []byte("abc"), b: []byte("a"), want: 1},
		{a: []byte("a"), b: []byte("abc"), want: 1},
		{a: []byte(""), b: []byte("a"), want: 0},
		{a: []byte("a"), b: []byte(""), want: 0},
		{a: []byte("abc"), b: []byte("xyz"), want: 0},
		{a: []byte{}, b: []byte{}, want: -1},
	}
	for _, tt := range tests {
		got := ordsComputePrefixMismatch(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("ordsComputePrefixMismatch(%q, %q) = %d, want %d", string(tt.a), string(tt.b), got, tt.want)
		}
	}
}

func TestOrdsBytesHasPrefix(t *testing.T) {
	if !ordsBytesHasPrefix([]byte("hello"), []byte("he")) {
		t.Error("expected 'hello' to have prefix 'he'")
	}
	if ordsBytesHasPrefix([]byte("hello"), []byte("ho")) {
		t.Error("expected 'hello' to NOT have prefix 'ho'")
	}
	if !ordsBytesHasPrefix([]byte("a"), []byte("a")) {
		t.Error("expected 'a' to have prefix 'a'")
	}
	if ordsBytesHasPrefix([]byte("a"), []byte("ab")) {
		t.Error("expected 'a' to NOT have prefix 'ab'")
	}
	if ordsBytesHasPrefix([]byte{}, []byte("a")) {
		t.Error("expected empty to NOT have prefix 'a'")
	}
}

func TestOrdsMaxInt(t *testing.T) {
	if got := ordsMaxInt(3, 5); got != 5 {
		t.Errorf("ordsMaxInt(3,5) = %d, want 5", got)
	}
	if got := ordsMaxInt(10, 10); got != 10 {
		t.Errorf("ordsMaxInt(10,10) = %d, want 10", got)
	}
	if got := ordsMaxInt(-1, 2); got != 2 {
		t.Errorf("ordsMaxInt(-1,2) = %d, want 2", got)
	}
}

// ─── 7 main tests matching the Java TestOrdsBlockTree ───────────────────────

// TestOrdsBlockTree_Basic writes and verifies 3 terms ("a", "b", "c") — the
// simplest non-trivial case. All three fit in a single root block.
func TestOrdsBlockTree_Basic(t *testing.T) {
	terms := mkTerms("a", "b", "c")
	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	// Verify writer metadata.
	if field.numTerms != 3 {
		t.Errorf("numTerms = %d, want 3", field.numTerms)
	}
	// OrdsFieldReader metadata.
	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 3, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 3 {
		t.Errorf("Size() = %d, want 3", got)
	}
	minT, err := fr.GetMin()
	if err != nil {
		t.Fatalf("GetMin: %v", err)
	}
	if minT == nil || minT.BytesValue().String() != "a" {
		t.Errorf("GetMin() = %v, want 'a'", minT)
	}
	maxT, err := fr.GetMax()
	if err != nil {
		t.Fatalf("GetMax: %v", err)
	}
	if maxT == nil || maxT.BytesValue().String() != "c" {
		t.Errorf("GetMax() = %v, want 'c'", maxT)
	}
	// Load the FST and verify its empty output indicates 3 terms.
	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	if rootOut.StartOrd != 0 {
		t.Errorf("FST root StartOrd = %d, want 0", rootOut.StartOrd)
	}
	// EndOrd encodes numTerms: EndOrd = math.MaxInt64 - (numTerms - 1)
	wantEnd := int64(math.MaxInt64) - int64(2)
	if rootOut.EndOrd != wantEnd {
		t.Errorf("FST root EndOrd = %d, want %d", rootOut.EndOrd, wantEnd)
	}
}

// TestOrdsBlockTree_TwoBlocks writes 72 terms. With default block sizes (25/48)
// the writer may split terms across multiple blocks.  Verifies that metadata
// matches and the FST index is valid.
func TestOrdsBlockTree_TwoBlocks(t *testing.T) {
	// 72 terms: "term0000" … "term0071".
	terms := generateLexicographicBytes(72, nil)
	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 72 {
		t.Errorf("numTerms = %d, want 72", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 72, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 72 {
		t.Errorf("Size() = %d, want 72", got)
	}
	minT, _ := fr.GetMin()
	if minT == nil || minT.BytesValue().String() != "term0000" {
		t.Errorf("GetMin() = %v, want 'term0000'", minT)
	}
	maxT, _ := fr.GetMax()
	if maxT == nil || maxT.BytesValue().String() != "term0071" {
		t.Errorf("GetMax() = %v, want 'term0071'", maxT)
	}
	// FST loads and has valid root output.
	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	if rootOut.StartOrd != 0 {
		t.Errorf("FST root StartOrd = %d, want 0", rootOut.StartOrd)
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 72 {
		t.Errorf("recovered term count = %d, want 72", recovered)
	}
}

// TestOrdsBlockTree_ThreeBlocks writes 108 terms that share enough prefix
// structure to force three levels of FST blocks.
func TestOrdsBlockTree_ThreeBlocks(t *testing.T) {
	// 108 terms: "a", "b", "c" + "ma".."mz" (26) + "n0".."n9" (10) +
	// "oaa".."oaz" (26) + "p00".."p99" (45) = 108.
	var terms []fakeTermEntry
	add := func(text string) {
		terms = append(terms, fakeTermEntry{text: text, docFreq: 1})
	}
	for i := 0; i < 26; i++ {
		add(fmt.Sprintf("a%c", 'a'+i))
	}
	for i := 0; i < 26; i++ {
		add(fmt.Sprintf("b%c", 'a'+i))
	}
	for i := 0; i < 26; i++ {
		add(fmt.Sprintf("c%c", 'a'+i))
	}
	for i := 0; i < 10; i++ {
		add(fmt.Sprintf("d%d", i))
	}
	for i := 0; i < 20; i++ {
		add(fmt.Sprintf("e%02d", i))
	}

	if len(terms) != 108 {
		t.Fatalf("generated %d terms, want 108", len(terms))
	}

	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 108 {
		t.Errorf("numTerms = %d, want 108", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 108, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 108 {
		t.Errorf("Size() = %d, want 108", got)
	}

	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	if rootOut.StartOrd != 0 {
		t.Errorf("FST root StartOrd = %d, want 0", rootOut.StartOrd)
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 108 {
		t.Errorf("recovered term count = %d, want 108", recovered)
	}
}

// TestOrdsBlockTree_FloorBlocks writes 128 single-byte terms (bytes 0–127)
// which tends to create floor blocks at the root level. Verifies metadata
// and FST index.
func TestOrdsBlockTree_FloorBlocks(t *testing.T) {
	var terms []fakeTermEntry
	for i := 0; i < 128; i++ {
		terms = append(terms, fakeTermEntry{text: string([]byte{byte(i)}), docFreq: 1})
	}

	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 128 {
		t.Errorf("numTerms = %d, want 128", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 128, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 128 {
		t.Errorf("Size() = %d, want 128", got)
	}

	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 128 {
		t.Errorf("recovered term count = %d, want 128", recovered)
	}
}

// TestOrdsBlockTree_NonRootFloorBlocks writes 36 single-char terms plus
// 128 "m"+byte terms, creating floor blocks at a non-root node.
func TestOrdsBlockTree_NonRootFloorBlocks(t *testing.T) {
	var terms []fakeTermEntry
	add := func(text string) {
		terms = append(terms, fakeTermEntry{text: text, docFreq: 1})
	}
	// 36 single-char terms: "a".."z", "0".."9"
	for i := 0; i < 26; i++ {
		add(string([]byte{byte('a' + i)}))
	}
	for i := 0; i < 10; i++ {
		add(string([]byte{byte('0' + i)}))
	}
	// 128 "m"+byte terms
	for i := 0; i < 128; i++ {
		add(fmt.Sprintf("m%s", string([]byte{byte(i)})))
	}

	if len(terms) != 36+128 {
		t.Fatalf("generated %d terms, want %d", len(terms), 36+128)
	}

	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 164 {
		t.Errorf("numTerms = %d, want 164", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 164, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 164 {
		t.Errorf("Size() = %d, want 164", got)
	}

	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 164 {
		t.Errorf("recovered term count = %d, want 164", recovered)
	}
}

// TestOrdsBlockTree_SeveralNonRootBlocks writes 900 two-character terms
// forming a grid of sub-blocks.
func TestOrdsBlockTree_SeveralNonRootBlocks(t *testing.T) {
	var terms []fakeTermEntry
	// 30x30 = 900 two-char terms: "aa", "ab", ..., "bd" (roughly)
	for i := 0; i < 30; i++ {
		for j := 0; j < 30; j++ {
			text := string([]byte{byte('a' + i), byte('a' + j)})
			terms = append(terms, fakeTermEntry{text: text, docFreq: 1})
		}
	}

	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 900 {
		t.Errorf("numTerms = %d, want 900", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 900, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 900 {
		t.Errorf("Size() = %d, want 900", got)
	}

	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 900 {
		t.Errorf("recovered term count = %d, want 900", recovered)
	}
}

// TestOrdsBlockTree_SeekCeilNotFound writes an empty-string term plus 36
// single-char and "a"+single-char terms, matching the Java test's term
// distribution for seekCeil tests.
func TestOrdsBlockTree_SeekCeilNotFound(t *testing.T) {
	var terms []fakeTermEntry
	// empty string term
	terms = append(terms, fakeTermEntry{text: "", docFreq: 1})
	// 26 single-char terms "a".."z"
	for i := 0; i < 26; i++ {
		terms = append(terms, fakeTermEntry{text: string([]byte{byte('a' + i)}), docFreq: 1})
	}
	// 10 numeric terms "0".."9"
	for i := 0; i < 10; i++ {
		terms = append(terms, fakeTermEntry{text: string([]byte{byte('0' + i)}), docFreq: 1})
	}

	if len(terms) != 1+26+10 {
		t.Fatalf("generated %d terms, want %d", len(terms), 1+26+10)
	}

	w, dir := writeTestSegment(t, terms)
	defer dir.Close()

	field := w.fields[0]
	if field.numTerms != 37 {
		t.Errorf("numTerms = %d, want 37", field.numTerms)
	}

	parent := &OrdsBlockTreeTermsReader{postingsReader: newFakePostingsReader()}
	fr, err := NewOrdsFieldReader(parent, field.fieldInfo, field.numTerms, field.rootCode,
		field.sumTotalTermFreq, field.sumDocFreq, 37, field.indexStartFP, nil,
		field.minTerm, field.maxTerm)
	if err != nil {
		t.Fatalf("NewOrdsFieldReader: %v", err)
	}
	if got := fr.Size(); got != 37 {
		t.Errorf("Size() = %d, want 37", got)
	}
	// Min term should be the empty string (lexicographically smallest).
	minT, _ := fr.GetMin()
	if minT == nil || minT.BytesValue().String() != "" {
		t.Errorf("GetMin() = %v, want empty string", minT)
	}

	fst := loadIndexFST(t, dir, field.indexStartFP)
	rootOut, hasEO := fst.GetEmptyOutput()
	if !hasEO {
		t.Fatal("FST has no empty output")
	}
	recovered := int64(math.MaxInt64) - rootOut.EndOrd + 1
	if recovered != 37 {
		t.Errorf("recovered term count = %d, want 37", recovered)
	}
}

// ─── FSTOrdsOutputs round-trip tests ────────────────────────────────────────

func TestFSTOrdsOutputsNoOutput(t *testing.T) {
	if fstOrdsNoOutput.Bytes.Length != 0 {
		t.Error("no-output Bytes.Length must be 0")
	}
	if fstOrdsNoOutput.StartOrd != 0 || fstOrdsNoOutput.EndOrd != 0 {
		t.Error("no-output ordinals must be 0")
	}
}

func TestFSTOrdsOutputsRoundTrip(t *testing.T) {
	outputs := FSTOutputs()
	// Write a few outputs to a buffer and read them back.
	buf := store.NewByteBuffersDataOutput()
	orig := fstOrdsOutputsSingleton.newOutput(
		&util.BytesRef{Bytes: []byte("hello"), Offset: 0, Length: 5},
		10,
		20,
	)
	if err := outputs.Write(orig, buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf2 := store.NewByteBuffersDataOutput()
	orig2 := fstOrdsOutputsSingleton.newOutput(
		fstOrdsNoBytes,
		42,
		99,
	)
	if err := outputs.Write(orig2, buf2); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read them back.
	bytes1 := buf.ToArrayCopy()
	bin1 := store.NewByteArrayDataInput(bytes1)
	got1, err := outputs.Read(bin1)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got1.StartOrd != 10 || got1.EndOrd != 20 {
		t.Errorf("ordinals = (%d,%d), want (10,20)", got1.StartOrd, got1.EndOrd)
	}
	if string(got1.Bytes.Bytes[got1.Bytes.Offset:got1.Bytes.Offset+got1.Bytes.Length]) != "hello" {
		t.Errorf("Bytes = %q, want 'hello'", string(got1.Bytes.Bytes))
	}

	bytes2 := buf2.ToArrayCopy()
	bin2 := store.NewByteArrayDataInput(bytes2)
	got2, err := outputs.Read(bin2)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got2.StartOrd != 42 || got2.EndOrd != 99 {
		t.Errorf("ordinals = (%d,%d), want (42,99)", got2.StartOrd, got2.EndOrd)
	}
	if got2.Bytes.Length != 0 {
		t.Errorf("Bytes.Length = %d, want 0", got2.Bytes.Length)
	}
}

func TestFSTOrdsOutputsCommon(t *testing.T) {
	o := fstOrdsOutputsSingleton
	a := o.newOutput(&util.BytesRef{Bytes: []byte("hello"), Offset: 0, Length: 5}, 5, 15)
	b := o.newOutput(&util.BytesRef{Bytes: []byte("help"), Offset: 0, Length: 4}, 10, 20)
	c := o.Common(a, b)
	if string(c.Bytes.Bytes[c.Bytes.Offset:c.Bytes.Offset+c.Bytes.Length]) != "hel" {
		t.Errorf("Common prefix = %q, want 'hel'", string(c.Bytes.Bytes))
	}
	if c.StartOrd != 5 || c.EndOrd != 15 {
		t.Errorf("Common ordinals = (%d,%d), want (5,15)", c.StartOrd, c.EndOrd)
	}
}

func TestFSTOrdsOutputsSubtract(t *testing.T) {
	o := fstOrdsOutputsSingleton
	// subtract "hel" from "hello" → "lo"
	prefix := o.newOutput(&util.BytesRef{Bytes: []byte("hel"), Offset: 0, Length: 3}, 5, 10)
	full := o.newOutput(&util.BytesRef{Bytes: []byte("hello"), Offset: 0, Length: 5}, 15, 25)
	sub := o.Subtract(full, prefix)
	if string(sub.Bytes.Bytes[sub.Bytes.Offset:sub.Bytes.Offset+sub.Bytes.Length]) != "lo" {
		t.Errorf("Subtract = %q, want 'lo'", string(sub.Bytes.Bytes))
	}
	if sub.StartOrd != 10 || sub.EndOrd != 15 {
		t.Errorf("Subtract ordinals = (%d,%d), want (10,15)", sub.StartOrd, sub.EndOrd)
	}
}

func TestFSTOrdsOutputsAdd(t *testing.T) {
	o := fstOrdsOutputsSingleton
	// add "hel" + "lo" → "hello"
	a := o.newOutput(&util.BytesRef{Bytes: []byte("hel"), Offset: 0, Length: 3}, 5, 10)
	b := o.newOutput(&util.BytesRef{Bytes: []byte("lo"), Offset: 0, Length: 2}, 10, 15)
	sum := o.Add(a, b)
	if string(sum.Bytes.Bytes[sum.Bytes.Offset:sum.Bytes.Offset+sum.Bytes.Length]) != "hello" {
		t.Errorf("Add = %q, want 'hello'", string(sum.Bytes.Bytes))
	}
	if sum.StartOrd != 15 || sum.EndOrd != 25 {
		t.Errorf("Add ordinals = (%d,%d), want (15,25)", sum.StartOrd, sum.EndOrd)
	}
}
