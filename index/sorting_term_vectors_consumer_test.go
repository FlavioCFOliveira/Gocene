// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fakeTermVectorsFormat is the minimal TermVectorsFormat the tests
// inject in place of Lucene90CompressingTermVectorsFormat (which is not
// yet reachable from package index — see the Sprint 55 deviation notes
// on SortingTermVectorsConsumer). Each VectorsWriter records its docs
// into a shared store; VectorsReader replays them in document order.
//
// The writer touches the directory exactly once per call (via
// CreateOutput on a deterministic name) so the tracking wrapper sees a
// file enter and exit during flush/abort.
type fakeTermVectorsFormat struct {
	store  *fakeVectorsStore
	name   string
	writes int
}

func newFakeTermVectorsFormat(name string) *fakeTermVectorsFormat {
	return &fakeTermVectorsFormat{name: name, store: &fakeVectorsStore{}}
}

func (f *fakeTermVectorsFormat) Name() string { return f.name }

func (f *fakeTermVectorsFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	name := fmt.Sprintf("_%s_%s_%d.tvd", f.name, state.SegmentInfo.Name(), f.writes)
	f.writes++
	out, err := state.Directory.CreateOutput(name, store.IOContextDefault)
	if err != nil {
		return nil, err
	}
	// Reset shared store on writer (re)open: emulates the buffered
	// segment lifecycle.
	f.store.reset()
	return &fakeTermVectorsWriter{store: f.store, out: out}, nil
}

func (f *fakeTermVectorsFormat) VectorsReader(dir store.Directory, info *SegmentInfo, fis *FieldInfos, ctx store.IOContext) (TermVectorsReader, error) {
	return &fakeTermVectorsReader{store: f.store}, nil
}

// fakeVectorsStore holds one entry per buffered document. Each entry is
// itself a list of (field, terms-in-order) so we preserve the
// write-time ordering and can validate it after sort.
type fakeVectorsStore struct {
	docs []*fakeVectorDoc
}

func (s *fakeVectorsStore) reset() {
	s.docs = s.docs[:0]
}

type fakeVectorDoc struct {
	fields []*fakeVectorField
}

type fakeVectorField struct {
	name         string
	terms        []fakeVectorTerm
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
}

type fakeVectorTerm struct {
	bytes     []byte
	positions []fakeVectorPos
}

type fakeVectorPos struct {
	pos     int
	start   int
	end     int
	payload []byte
}

type fakeTermVectorsWriter struct {
	store    *fakeVectorsStore
	out      store.IndexOutput
	current  *fakeVectorDoc
	curField *fakeVectorField
	curTerm  *fakeVectorTerm
}

func (w *fakeTermVectorsWriter) StartDocument(numFields int) error {
	w.current = &fakeVectorDoc{fields: make([]*fakeVectorField, 0, numFields)}
	return nil
}

func (w *fakeTermVectorsWriter) StartField(fieldInfo *FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	name := ""
	if fieldInfo != nil {
		name = fieldInfo.Name()
	}
	w.curField = &fakeVectorField{
		name:         name,
		terms:        make([]fakeVectorTerm, 0, numTerms),
		hasPositions: hasPositions,
		hasOffsets:   hasOffsets,
		hasPayloads:  hasPayloads,
	}
	return nil
}

func (w *fakeTermVectorsWriter) StartTerm(term []byte) error {
	cp := make([]byte, len(term))
	copy(cp, term)
	w.curTerm = &fakeVectorTerm{bytes: cp}
	return nil
}

func (w *fakeTermVectorsWriter) AddPosition(position, startOffset, endOffset int, payload []byte) error {
	var pcopy []byte
	if payload != nil {
		pcopy = append(pcopy, payload...)
	}
	w.curTerm.positions = append(w.curTerm.positions, fakeVectorPos{
		pos: position, start: startOffset, end: endOffset, payload: pcopy,
	})
	return nil
}

func (w *fakeTermVectorsWriter) FinishTerm() error {
	w.curField.terms = append(w.curField.terms, *w.curTerm)
	w.curTerm = nil
	return nil
}

func (w *fakeTermVectorsWriter) FinishField() error {
	w.current.fields = append(w.current.fields, w.curField)
	w.curField = nil
	return nil
}

func (w *fakeTermVectorsWriter) FinishDocument() error {
	w.store.docs = append(w.store.docs, w.current)
	w.current = nil
	return nil
}

func (w *fakeTermVectorsWriter) Close() error {
	return w.out.Close()
}

// fakeTermVectorsReader serves up Fields built from a stored doc.
type fakeTermVectorsReader struct {
	store *fakeVectorsStore
}

func (r *fakeTermVectorsReader) Get(docID int) (Fields, error) {
	if docID < 0 || docID >= len(r.store.docs) {
		return nil, fmt.Errorf("fakeReader: docID %d out of range [0,%d)", docID, len(r.store.docs))
	}
	doc := r.store.docs[docID]
	if doc == nil || len(doc.fields) == 0 {
		return nil, nil
	}
	mf := NewMemoryFields()
	for _, f := range doc.fields {
		mf.AddField(f.name, newFakeVectorTerms(f))
	}
	return mf, nil
}

func (r *fakeTermVectorsReader) GetField(docID int, field string) (Terms, error) {
	fields, err := r.Get(docID)
	if err != nil || fields == nil {
		return nil, err
	}
	return fields.Terms(field)
}

func (r *fakeTermVectorsReader) Close() error { return nil }

// fakeVectorTerms exposes a fakeVectorField via the Terms interface so
// the production writeTermVectorsDoc port can consume it.
type fakeVectorTerms struct {
	f *fakeVectorField
}

func newFakeVectorTerms(f *fakeVectorField) *fakeVectorTerms { return &fakeVectorTerms{f: f} }

func (t *fakeVectorTerms) GetIterator() (TermsEnum, error) {
	return &fakeVectorTermsEnum{f: t.f, idx: -1}, nil
}
func (t *fakeVectorTerms) GetIteratorWithSeek(seek *Term) (TermsEnum, error) {
	return t.GetIterator()
}
func (t *fakeVectorTerms) Size() int64                         { return int64(len(t.f.terms)) }
func (t *fakeVectorTerms) GetDocCount() (int, error)           { return 1, nil }
func (t *fakeVectorTerms) GetSumDocFreq() (int64, error)       { return int64(len(t.f.terms)), nil }
func (t *fakeVectorTerms) GetSumTotalTermFreq() (int64, error) { return int64(len(t.f.terms)), nil }
func (t *fakeVectorTerms) HasFreqs() bool                      { return true }
func (t *fakeVectorTerms) HasOffsets() bool                    { return t.f.hasOffsets }
func (t *fakeVectorTerms) HasPositions() bool                  { return t.f.hasPositions }
func (t *fakeVectorTerms) HasPayloads() bool                   { return t.f.hasPayloads }
func (t *fakeVectorTerms) GetMin() (*Term, error)              { return nil, nil }
func (t *fakeVectorTerms) GetMax() (*Term, error)              { return nil, nil }
func (t *fakeVectorTerms) GetPostingsReader(text string, flags int) (PostingsEnum, error) {
	return nil, nil
}

type fakeVectorTermsEnum struct {
	f   *fakeVectorField
	idx int
}

func (e *fakeVectorTermsEnum) Next() (*Term, error) {
	e.idx++
	if e.idx >= len(e.f.terms) {
		return nil, nil
	}
	return NewTermFromBytes(e.f.name, e.f.terms[e.idx].bytes), nil
}
func (e *fakeVectorTermsEnum) SeekCeil(term *Term) (*Term, error) { return nil, nil }
func (e *fakeVectorTermsEnum) SeekExact(term *Term) (bool, error) { return false, nil }
func (e *fakeVectorTermsEnum) Term() *Term {
	if e.idx < 0 || e.idx >= len(e.f.terms) {
		return nil
	}
	return NewTermFromBytes(e.f.name, e.f.terms[e.idx].bytes)
}
func (e *fakeVectorTermsEnum) DocFreq() (int, error) { return 1, nil }
func (e *fakeVectorTermsEnum) TotalTermFreq() (int64, error) {
	if e.idx < 0 || e.idx >= len(e.f.terms) {
		return 0, nil
	}
	return int64(len(e.f.terms[e.idx].positions)), nil
}
func (e *fakeVectorTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if e.idx < 0 || e.idx >= len(e.f.terms) {
		return nil, nil
	}
	t := e.f.terms[e.idx]
	return &fakeVectorPostings{positions: t.positions, started: false, currentDoc: -1}, nil
}
func (e *fakeVectorTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return e.Postings(flags)
}

type fakeVectorPostings struct {
	positions  []fakeVectorPos
	posIdx     int
	started    bool
	currentDoc int
}

func (p *fakeVectorPostings) NextDoc() (int, error) {
	if p.started {
		p.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	p.started = true
	p.posIdx = -1
	p.currentDoc = 0
	return 0, nil
}
func (p *fakeVectorPostings) Advance(target int) (int, error) { return p.NextDoc() }
func (p *fakeVectorPostings) DocID() int                      { return p.currentDoc }
func (p *fakeVectorPostings) Freq() (int, error)              { return len(p.positions), nil }
func (p *fakeVectorPostings) NextPosition() (int, error) {
	p.posIdx++
	if p.posIdx >= len(p.positions) {
		return NO_MORE_POSITIONS, nil
	}
	return p.positions[p.posIdx].pos, nil
}
func (p *fakeVectorPostings) StartOffset() (int, error) {
	if p.posIdx < 0 || p.posIdx >= len(p.positions) {
		return -1, nil
	}
	return p.positions[p.posIdx].start, nil
}
func (p *fakeVectorPostings) EndOffset() (int, error) {
	if p.posIdx < 0 || p.posIdx >= len(p.positions) {
		return -1, nil
	}
	return p.positions[p.posIdx].end, nil
}
func (p *fakeVectorPostings) GetPayload() ([]byte, error) {
	if p.posIdx < 0 || p.posIdx >= len(p.positions) {
		return nil, nil
	}
	return p.positions[p.posIdx].payload, nil
}
func (p *fakeVectorPostings) Cost() int64 { return 1 }

// fakeCodecTV exposes the fake format as the codec's term-vectors
// format so the final (post-sort) writer path also lands in the same
// store. It satisfies the full Codec interface.
type fakeCodecTV struct {
	tv *fakeTermVectorsFormat
}

func (c *fakeCodecTV) Name() string                           { return "fake-codec-tv" }
func (c *fakeCodecTV) PostingsFormat() PostingsFormat         { return nil }
func (c *fakeCodecTV) StoredFieldsFormat() StoredFieldsFormat { return nil }
func (c *fakeCodecTV) FieldInfosFormat() FieldInfosFormat     { return nil }
func (c *fakeCodecTV) SegmentInfosFormat() SegmentInfosFormat { return nil }
func (c *fakeCodecTV) SegmentInfoFormat() SegmentInfoFormat   { return nil }
func (c *fakeCodecTV) TermVectorsFormat() TermVectorsFormat   { return c.tv }
func (c *fakeCodecTV) CompoundFormat() CompoundFormat         { return nil }
func (c *fakeCodecTV) KnnVectorsFormat() KnnVectorsFormat     { return nil }
func (c *fakeCodecTV) DocValuesFormat() DocValuesFormat       { return nil }

func newTestTVConsumer(t *testing.T, docs int) (*SortingTermVectorsConsumer, *fakeTermVectorsFormat, store.Directory, *SegmentInfo) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", docs, dir)
	tempFormat := newFakeTermVectorsFormat("temp")
	finalFormat := newFakeTermVectorsFormat("final")
	codec := &fakeCodecTV{tv: finalFormat}
	c := NewSortingTermVectorsConsumer(codec, dir, info)
	c.SetTempTermVectorsFormat(tempFormat)
	return c, finalFormat, dir, info
}

// writeDoc is a tiny helper that drives the TermVectorsWriter contract
// to push a single document into the buffered writer, with one field
// and a single term whose positions list has freq elements.
func writeDoc(t *testing.T, w TermVectorsWriter, field, term string, positions []fakeVectorPos, hasPositions, hasOffsets, hasPayloads bool) {
	t.Helper()
	if err := w.StartDocument(1); err != nil {
		t.Fatal(err)
	}
	if err := w.StartField(nil, 1, hasPositions, hasOffsets, hasPayloads); err != nil {
		t.Fatal(err)
	}
	if err := w.StartTerm([]byte(term)); err != nil {
		t.Fatal(err)
	}
	for _, p := range positions {
		if err := w.AddPosition(p.pos, p.start, p.end, p.payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.FinishTerm(); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishField(); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishDocument(); err != nil {
		t.Fatal(err)
	}
	// The reader-side fakeVectorTerms.GetIterator uses the writer's
	// recorded name; since StartField got a nil FieldInfo above, set
	// the field name on the most recently buffered field record.
	store := w.(*fakeTermVectorsWriter).store
	if n := len(store.docs); n > 0 {
		doc := store.docs[n-1]
		if len(doc.fields) > 0 {
			doc.fields[len(doc.fields)-1].name = field
		}
	}
}

// -----------------------------------------------------------------------------
// Init / configuration

func TestSortingTermVectorsConsumer_InitRequiresTempFormat(t *testing.T) {
	// Clear any process-wide default the codec bridge may have published
	// so the consumer behaves as it would in a binary that has not
	// blank-imported a codec. The Sprint 116 wiring registers nil by
	// default for term-vectors (the Lucene90CompressingTermVectorsFormat
	// port is still a stub), but the protection guards against future
	// codec additions silently flipping this test.
	prev := DefaultTempTermVectorsFormat()
	RegisterDefaultTempTermVectorsFormat(nil)
	t.Cleanup(func() { RegisterDefaultTempTermVectorsFormat(prev) })

	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)
	c := NewSortingTermVectorsConsumer(&fakeCodecTV{tv: newFakeTermVectorsFormat("temp")}, dir, info)
	// Deliberately skip SetTempTermVectorsFormat.
	if err := c.InitTermVectorsWriter(); !errors.Is(err, ErrTempTermVectorsFormatUnset) {
		t.Fatalf("InitTermVectorsWriter without temp format: got %v, want ErrTempTermVectorsFormatUnset", err)
	}
	if c.TempDirectory() != nil {
		t.Fatalf("TempDirectory should remain nil when init fails")
	}
	if c.Writer() != nil {
		t.Fatalf("Writer should remain nil when init fails")
	}
}

func TestSortingTermVectorsConsumer_InitIsIdempotent(t *testing.T) {
	c, _, _, _ := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	tmp := c.TempDirectory()
	firstWriter := c.Writer()
	if tmp == nil || firstWriter == nil {
		t.Fatalf("TempDirectory/Writer should be non-nil after init")
	}
	firstFiles := tmp.TemporaryFiles()
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if got := c.TempDirectory(); got != tmp {
		t.Fatalf("second init must not replace tracking wrapper")
	}
	if got := c.Writer(); got != firstWriter {
		t.Fatalf("second init must not replace writer")
	}
	if got := len(c.TempDirectory().TemporaryFiles()); got != len(firstFiles) {
		t.Fatalf("second init must not allocate more files: was %d, now %d", len(firstFiles), got)
	}
}

func TestSortingTermVectorsConsumer_TrackingWrapperRecordsFiles(t *testing.T) {
	c, _, _, _ := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}
	files := c.TempDirectory().TemporaryFiles()
	if len(files) != 1 {
		t.Fatalf("expected 1 tracked file after init, got %d (%v)", len(files), files)
	}
	// Defensive copy: mutating the returned slice must not affect the wrapper.
	files[0] = "tampered"
	if again := c.TempDirectory().TemporaryFiles(); again[0] == "tampered" {
		t.Fatalf("TemporaryFiles must return a defensive copy")
	}
}

// -----------------------------------------------------------------------------
// Flush / sort

func TestSortingTermVectorsConsumer_FlushAppliesSortMap(t *testing.T) {
	const n = 3
	c, finalFormat, dir, info := newTestTVConsumer(t, n)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Buffer 3 docs through the temp writer; each doc carries a single
	// vector field with one term whose bytes equal "doc-%d".
	w := c.Writer()
	for i := 0; i < n; i++ {
		writeDoc(t, w, "content", fmt.Sprintf("doc-%d", i),
			[]fakeVectorPos{{pos: 0, start: 0, end: 5}}, true, true, false)
	}

	state := &SegmentWriteState{Directory: dir, SegmentInfo: info}
	if err := c.Flush(state, &reverseSortMap{n: n}); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if got := len(finalFormat.store.docs); got != n {
		t.Fatalf("final doc count: got %d, want %d", got, n)
	}
	want := []string{"doc-2", "doc-1", "doc-0"}
	for i, expected := range want {
		doc := finalFormat.store.docs[i]
		if len(doc.fields) != 1 {
			t.Fatalf("doc[%d]: got %d fields, want 1", i, len(doc.fields))
		}
		if len(doc.fields[0].terms) != 1 {
			t.Fatalf("doc[%d]: got %d terms, want 1", i, len(doc.fields[0].terms))
		}
		if string(doc.fields[0].terms[0].bytes) != expected {
			t.Errorf("doc[%d]: got %q, want %q", i, doc.fields[0].terms[0].bytes, expected)
		}
	}

	// Flush must have cleaned up the temp directory.
	if c.TempDirectory() != nil {
		t.Fatalf("Flush must clear TempDirectory")
	}
}

func TestSortingTermVectorsConsumer_FlushIdentityWhenSortMapNil(t *testing.T) {
	const n = 2
	c, finalFormat, dir, info := newTestTVConsumer(t, n)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatal(err)
	}
	w := c.Writer()
	for i := 0; i < n; i++ {
		writeDoc(t, w, "f", fmt.Sprintf("t-%d", i),
			[]fakeVectorPos{{pos: 0, start: 0, end: 1}}, true, true, false)
	}

	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush nil sortMap: %v", err)
	}
	if got := len(finalFormat.store.docs); got != n {
		t.Fatalf("doc count: got %d, want %d", got, n)
	}
	for i := 0; i < n; i++ {
		want := fmt.Sprintf("t-%d", i)
		got := string(finalFormat.store.docs[i].fields[0].terms[0].bytes)
		if got != want {
			t.Errorf("doc[%d]: got %q, want %q", i, got, want)
		}
	}
}

func TestSortingTermVectorsConsumer_FlushPreservesPositionsAndOffsets(t *testing.T) {
	c, finalFormat, dir, info := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatal(err)
	}
	positions := []fakeVectorPos{
		{pos: 0, start: 0, end: 5, payload: []byte{0x01}},
		{pos: 1, start: 6, end: 11, payload: []byte{0x02}},
	}
	writeDoc(t, c.Writer(), "body", "alpha", positions, true, true, true)

	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(finalFormat.store.docs) != 1 {
		t.Fatalf("expected 1 final doc, got %d", len(finalFormat.store.docs))
	}
	field := finalFormat.store.docs[0].fields[0]
	if !field.hasPositions || !field.hasOffsets || !field.hasPayloads {
		t.Fatalf("flag plumbing lost: positions=%v offsets=%v payloads=%v",
			field.hasPositions, field.hasOffsets, field.hasPayloads)
	}
	got := field.terms[0].positions
	if len(got) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(got))
	}
	for i, want := range positions {
		if got[i].pos != want.pos || got[i].start != want.start || got[i].end != want.end {
			t.Errorf("position %d: got %+v, want %+v", i, got[i], want)
		}
		if len(got[i].payload) != 1 || got[i].payload[0] != want.payload[0] {
			t.Errorf("payload %d: got %v, want %v", i, got[i].payload, want.payload)
		}
	}
}

func TestSortingTermVectorsConsumer_FlushMultipleFieldsAndTermsAscending(t *testing.T) {
	c, finalFormat, dir, info := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatal(err)
	}
	// Two fields, ascending; two terms each, ascending.
	w := c.Writer()
	if err := w.StartDocument(2); err != nil {
		t.Fatal(err)
	}
	for _, fname := range []string{"a_field", "z_field"} {
		if err := w.StartField(nil, 2, true, false, false); err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"alpha", "beta"} {
			if err := w.StartTerm([]byte(term)); err != nil {
				t.Fatal(err)
			}
			if err := w.AddPosition(0, -1, -1, nil); err != nil {
				t.Fatal(err)
			}
			if err := w.FinishTerm(); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.FinishField(); err != nil {
			t.Fatal(err)
		}
		// Patch field name on the in-flight doc; the writer has no
		// FieldInfo source in this test.
		store := w.(*fakeTermVectorsWriter).store
		if w.(*fakeTermVectorsWriter).current != nil {
			cur := w.(*fakeTermVectorsWriter).current
			cur.fields[len(cur.fields)-1].name = fname
		} else if len(store.docs) > 0 {
			doc := store.docs[len(store.docs)-1]
			doc.fields[len(doc.fields)-1].name = fname
		}
	}
	if err := w.FinishDocument(); err != nil {
		t.Fatal(err)
	}

	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := len(finalFormat.store.docs); got != 1 {
		t.Fatalf("expected 1 final doc, got %d", got)
	}
	doc := finalFormat.store.docs[0]
	gotFields := make([]string, len(doc.fields))
	for i, f := range doc.fields {
		gotFields[i] = f.name
	}
	if !sort.StringsAreSorted(gotFields) {
		t.Errorf("fields not ascending in final doc: %v", gotFields)
	}
	for i, f := range doc.fields {
		gotTerms := make([]string, len(f.terms))
		for j, term := range f.terms {
			gotTerms[j] = string(term.bytes)
		}
		if !sort.StringsAreSorted(gotTerms) {
			t.Errorf("field %d terms not ascending: %v", i, gotTerms)
		}
	}
}

func TestSortingTermVectorsConsumer_FlushWithoutInitIsNoOp(t *testing.T) {
	c, finalFormat, dir, info := newTestTVConsumer(t, 0)
	// No InitTermVectorsWriter call.
	if err := c.Flush(&SegmentWriteState{Directory: dir, SegmentInfo: info}, nil); err != nil {
		t.Fatalf("Flush without init must be a no-op, got %v", err)
	}
	if len(finalFormat.store.docs) != 0 {
		t.Fatalf("Flush without init must not produce docs, got %d", len(finalFormat.store.docs))
	}
}

func TestSortingTermVectorsConsumer_FlushRequiresState(t *testing.T) {
	c, _, _, _ := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatal(err)
	}
	if err := c.Flush(nil, nil); err == nil {
		t.Fatalf("Flush with nil state must error")
	}
}

// orderedFields is a Fields impl that preserves the caller-specified
// field order (unlike MemoryFields, which sorts at iterator time). It
// lets us drive writeTermVectorsDoc through a deliberately bad
// ordering to exercise the ascending-name guard.
type orderedFields struct {
	names []string
	terms map[string]Terms
}

func newOrderedFields() *orderedFields {
	return &orderedFields{terms: map[string]Terms{}}
}

func (o *orderedFields) Add(name string, terms Terms) {
	o.names = append(o.names, name)
	o.terms[name] = terms
}

func (o *orderedFields) Iterator() (FieldIterator, error) {
	return &orderedFieldIterator{names: o.names, idx: -1}, nil
}

func (o *orderedFields) Size() int { return len(o.names) }

func (o *orderedFields) Terms(field string) (Terms, error) {
	return o.terms[field], nil
}

type orderedFieldIterator struct {
	names []string
	idx   int
}

func (it *orderedFieldIterator) Next() (string, error) {
	it.idx++
	if it.idx >= len(it.names) {
		return "", nil
	}
	return it.names[it.idx], nil
}
func (it *orderedFieldIterator) HasNext() bool { return it.idx+1 < len(it.names) }

func TestWriteTermVectorsDoc_RejectsBadFieldOrder(t *testing.T) {
	store := &fakeVectorsStore{}
	w := &fakeTermVectorsWriter{store: store, out: nil}

	// Two fields in descending order.
	zField := &fakeVectorField{name: "z", hasPositions: true,
		terms: []fakeVectorTerm{{bytes: []byte("t"), positions: []fakeVectorPos{{pos: 0}}}}}
	aField := &fakeVectorField{name: "a", hasPositions: true,
		terms: []fakeVectorTerm{{bytes: []byte("t"), positions: []fakeVectorPos{{pos: 0}}}}}
	of := newOrderedFields()
	of.Add("z", newFakeVectorTerms(zField))
	of.Add("a", newFakeVectorTerms(aField))

	if err := writeTermVectorsDoc(w, of, nil); err == nil {
		t.Fatalf("expected error for descending field order, got nil")
	}
}

// -----------------------------------------------------------------------------
// Abort

func TestSortingTermVectorsConsumer_AbortDeletesTempFiles(t *testing.T) {
	c, _, dir, _ := newTestTVConsumer(t, 1)
	if err := c.InitTermVectorsWriter(); err != nil {
		t.Fatal(err)
	}
	files := c.TempDirectory().TemporaryFiles()
	if len(files) != 1 {
		t.Fatalf("expected 1 temp file, got %d", len(files))
	}
	tempName := files[0]

	c.Abort()
	if c.TempDirectory() != nil {
		t.Fatalf("Abort must clear TempDirectory")
	}
	listed, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, n := range listed {
		if n == tempName {
			t.Fatalf("Abort must delete temp file %q (still listed)", tempName)
		}
	}
}

func TestSortingTermVectorsConsumer_AbortIsSafeWithoutInit(t *testing.T) {
	c, _, _, _ := newTestTVConsumer(t, 0)
	// Must not panic.
	c.Abort()
}

// -----------------------------------------------------------------------------
// writeTermVectorsDoc covers an empty (nil Fields) document

func TestWriteTermVectorsDoc_NilVectorsEmitsEmptyDoc(t *testing.T) {
	store := &fakeVectorsStore{}
	w := &fakeTermVectorsWriter{store: store, out: nil}
	// Patch Close to skip the nil out:
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil out triggered panic: %v", r)
		}
	}()
	if err := writeTermVectorsDoc(w, nil, nil); err != nil {
		t.Fatalf("nil vectors: %v", err)
	}
	if len(store.docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(store.docs))
	}
	if len(store.docs[0].fields) != 0 {
		t.Fatalf("expected empty fields, got %d", len(store.docs[0].fields))
	}
}

// -----------------------------------------------------------------------------
// emitPositions sentinel path (no positions, no offsets)

func TestEmitPositions_NoPositionsEmitsSentinelsForFreq(t *testing.T) {
	store := &fakeVectorsStore{}
	w := &fakeTermVectorsWriter{store: store, out: nil}
	if err := w.StartDocument(1); err != nil {
		t.Fatal(err)
	}
	if err := w.StartField(nil, 1, false, false, false); err != nil {
		t.Fatal(err)
	}
	if err := w.StartTerm([]byte("t")); err != nil {
		t.Fatal(err)
	}
	// Pass a nil TermsEnum since the sentinel branch never touches it.
	if err := emitPositions(w, nil, false, false, false, 3); err != nil {
		t.Fatalf("sentinel: %v", err)
	}
	if got := len(w.curTerm.positions); got != 3 {
		t.Fatalf("expected 3 sentinel positions for freq=3, got %d", got)
	}
	for i, p := range w.curTerm.positions {
		if p.pos != -1 || p.start != -1 || p.end != -1 || p.payload != nil {
			t.Errorf("sentinel %d not all -1/nil: %+v", i, p)
		}
	}
}
