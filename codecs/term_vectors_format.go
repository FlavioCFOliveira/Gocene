// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Codec envelope constants for Lucene104TermVectorsWriter.
//
// NOTE (DEVIATION): Lucene 10.4.0 uses "Lucene90TermVectorsData" with the
// LZ4-compressed packed-int chunk format. Gocene uses a simpler sequential
// format under distinct codec names so that these files carry the standard
// 16-byte CodecUtil footer (enabling CFS packing and footer integrity
// checks) while clearly advertising the divergence.
const (
	lucene104TVDataCodec    = "Gocene104TermVectorsData"
	lucene104TVDataVersion  = int32(0)
	lucene104TVIndexCodec   = "Gocene104TermVectorsIndex"
	lucene104TVIndexVersion = int32(0)
)

// TermVectorsFormat is an alias of spi.TermVectorsFormat.
type TermVectorsFormat = spi.TermVectorsFormat

// TermVectorsWriter is an alias of spi.TermVectorsWriter.
type TermVectorsWriter = spi.TermVectorsWriter

// TermVectorsReader is an alias of spi.TermVectorsReader.
type TermVectorsReader = spi.TermVectorsReader

// BaseTermVectorsFormat provides common functionality.
type BaseTermVectorsFormat struct {
	name string
}

// NewBaseTermVectorsFormat creates a new BaseTermVectorsFormat.
func NewBaseTermVectorsFormat(name string) *BaseTermVectorsFormat {
	return &BaseTermVectorsFormat{name: name}
}

// Name returns the format name.
func (f *BaseTermVectorsFormat) Name() string {
	return f.name
}

// Lucene104TermVectorsFormat is the Lucene 10.4 term vectors format.
// This is a placeholder implementation.
type Lucene104TermVectorsFormat struct {
	*BaseTermVectorsFormat
}

// NewLucene104TermVectorsFormat creates a new Lucene104TermVectorsFormat.
func NewLucene104TermVectorsFormat() *Lucene104TermVectorsFormat {
	return &Lucene104TermVectorsFormat{
		BaseTermVectorsFormat: NewBaseTermVectorsFormat("Lucene104TermVectorsFormat"),
	}
}

// VectorsWriter returns a term vectors writer.
func (f *Lucene104TermVectorsFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	// Placeholder: Full implementation would write to .tvx, .tvd, .tvm files
	return NewLucene104TermVectorsWriter(state), nil
}

// VectorsReader returns a term vectors reader.
func (f *Lucene104TermVectorsFormat) VectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (TermVectorsReader, error) {
	return NewLucene104TermVectorsReader(dir, segmentInfo, fieldInfos, context)
}

// -----------------------------------------------------------------------------
// Lucene104TermVectorsWriter — on-disk format (DEVIATION from Lucene 10.4.0)
//
// Lucene 10.4.0 uses "Lucene90TermVectorsData" with LZ4-compressed packed-int
// chunks. Gocene uses a simpler sequential format under the codec name
// "Gocene104TermVectorsData" so that:
//   - The standard 16-byte CodecUtil footer is always present (required by
//     copyFileBody / CFS packing).
//   - Format divergence is clearly advertised in the on-disk header.
//
// File layout (.tvd — data file):
//   IndexHeader("Gocene104TermVectorsData", 0, segID, "")
//   VInt(numDocs)
//   for each doc:
//     VInt(numFields)
//     for each field:
//       String(fieldName)
//       Byte(flags: bit0=hasPositions, bit1=hasOffsets, bit2=hasPayloads)
//       VInt(numTerms)
//       for each term:
//         VInt(termLen) + bytes(term)
//         VInt(freq)
//         for each occurrence (if hasPositions):
//           VInt(position)
//           if hasOffsets: VInt(startOffset), VInt(endOffset)
//           if hasPayloads: VInt(payloadLen) + bytes(payload) [0 len = no payload]
//   Footer
//
// File layout (.tvx — index file):
//   IndexHeader("Gocene104TermVectorsIndex", 0, segID, "")
//   VInt(numDocs)   -- mirrors numDocs in data for quick validation
//   Footer
// -----------------------------------------------------------------------------

// tvField accumulates term vectors for one field.
type tvField struct {
	name         string
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
	terms        []*tvTerm
}

// tvTerm accumulates positions/offsets/payloads for one term occurrence.
type tvTerm struct {
	text      []byte
	positions []tvPos
}

// tvPos is one position with optional offset and payload.
type tvPos struct {
	position    int
	startOffset int
	endOffset   int
	payload     []byte
}

// tvDoc is the term-vector accumulator for one document.
type tvDoc struct {
	fields []*tvField
}

// Lucene104TermVectorsWriter writes term vectors to .tvd + .tvx files with
// standard CodecUtil envelopes.
type Lucene104TermVectorsWriter struct {
	state    *SegmentWriteState
	docs     []*tvDoc
	curDoc   *tvDoc
	curField *tvField
	curTerm  *tvTerm
	mu       sync.Mutex
	closed   bool
}

// NewLucene104TermVectorsWriter creates a new Lucene104TermVectorsWriter.
func NewLucene104TermVectorsWriter(state *SegmentWriteState) *Lucene104TermVectorsWriter {
	return &Lucene104TermVectorsWriter{state: state}
}

// StartDocument starts writing term vectors for a document.
func (w *Lucene104TermVectorsWriter) StartDocument(numFields int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	w.curDoc = &tvDoc{fields: make([]*tvField, 0, numFields)}
	return nil
}

// StartField starts writing a term vector for a field.
func (w *Lucene104TermVectorsWriter) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curDoc == nil {
		return fmt.Errorf("StartField called before StartDocument")
	}
	w.curField = &tvField{
		name:         fieldInfo.Name(),
		hasPositions: hasPositions,
		hasOffsets:   hasOffsets,
		hasPayloads:  hasPayloads,
		terms:        make([]*tvTerm, 0, numTerms),
	}
	return nil
}

// StartTerm starts a new term in the current field.
func (w *Lucene104TermVectorsWriter) StartTerm(term []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curField == nil {
		return fmt.Errorf("StartTerm called before StartField")
	}
	cp := make([]byte, len(term))
	copy(cp, term)
	w.curTerm = &tvTerm{text: cp}
	return nil
}

// AddPosition adds a position for the current term.
func (w *Lucene104TermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curTerm == nil {
		return fmt.Errorf("AddPosition called before StartTerm")
	}
	var pl []byte
	if len(payload) > 0 {
		pl = make([]byte, len(payload))
		copy(pl, payload)
	}
	w.curTerm.positions = append(w.curTerm.positions, tvPos{
		position:    position,
		startOffset: startOffset,
		endOffset:   endOffset,
		payload:     pl,
	})
	return nil
}

// FinishTerm finishes the current term.
func (w *Lucene104TermVectorsWriter) FinishTerm() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curTerm == nil {
		return nil
	}
	w.curField.terms = append(w.curField.terms, w.curTerm)
	w.curTerm = nil
	return nil
}

// FinishField finishes the current field.
func (w *Lucene104TermVectorsWriter) FinishField() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curField == nil {
		return nil
	}
	w.curDoc.fields = append(w.curDoc.fields, w.curField)
	w.curField = nil
	return nil
}

// FinishDocument finishes the current document.
func (w *Lucene104TermVectorsWriter) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("term vectors writer is closed")
	}
	if w.curDoc == nil {
		return nil
	}
	w.docs = append(w.docs, w.curDoc)
	w.curDoc = nil
	return nil
}

// Close flushes all accumulated documents to .tvd + .tvx and closes.
func (w *Lucene104TermVectorsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if err := w.writeTVD(); err != nil {
		return fmt.Errorf("term vectors: write .tvd: %w", err)
	}
	if err := w.writeTVX(); err != nil {
		return fmt.Errorf("term vectors: write .tvx: %w", err)
	}
	return nil
}

// writeTVD writes the term vectors data file.
func (w *Lucene104TermVectorsWriter) writeTVD() error {
	si := w.state.SegmentInfo
	fileName := si.Name() + ".tvd"
	raw, err := w.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return err
	}
	out := store.NewChecksumIndexOutput(raw)
	defer out.Close()

	if err := WriteIndexHeader(out, lucene104TVDataCodec, lucene104TVDataVersion, si.GetID(), ""); err != nil {
		return fmt.Errorf("write .tvd header: %w", err)
	}

	if err := store.WriteVInt(out, int32(len(w.docs))); err != nil {
		return err
	}
	for _, doc := range w.docs {
		if err := store.WriteVInt(out, int32(len(doc.fields))); err != nil {
			return err
		}
		for _, f := range doc.fields {
			if err := store.WriteString(out, f.name); err != nil {
				return err
			}
			flags := byte(0)
			if f.hasPositions {
				flags |= 0x01
			}
			if f.hasOffsets {
				flags |= 0x02
			}
			if f.hasPayloads {
				flags |= 0x04
			}
			if err := out.WriteByte(flags); err != nil {
				return err
			}
			if err := store.WriteVInt(out, int32(len(f.terms))); err != nil {
				return err
			}
			for _, t := range f.terms {
				if err := store.WriteVInt(out, int32(len(t.text))); err != nil {
					return err
				}
				if err := out.WriteBytes(t.text); err != nil {
					return err
				}
				freq := int32(len(t.positions))
				if err := store.WriteVInt(out, freq); err != nil {
					return err
				}
				for _, p := range t.positions {
					if f.hasPositions {
						if err := store.WriteVInt(out, int32(p.position)); err != nil {
							return err
						}
					}
					if f.hasOffsets {
						if err := store.WriteVInt(out, int32(p.startOffset)); err != nil {
							return err
						}
						if err := store.WriteVInt(out, int32(p.endOffset)); err != nil {
							return err
						}
					}
					if f.hasPayloads {
						if err := store.WriteVInt(out, int32(len(p.payload))); err != nil {
							return err
						}
						if len(p.payload) > 0 {
							if err := out.WriteBytes(p.payload); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}

	return WriteFooter(out)
}

// writeTVX writes the term vectors index file.
func (w *Lucene104TermVectorsWriter) writeTVX() error {
	si := w.state.SegmentInfo
	fileName := si.Name() + ".tvx"
	raw, err := w.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return err
	}
	out := store.NewChecksumIndexOutput(raw)
	defer out.Close()

	if err := WriteIndexHeader(out, lucene104TVIndexCodec, lucene104TVIndexVersion, si.GetID(), ""); err != nil {
		return fmt.Errorf("write .tvx header: %w", err)
	}
	if err := store.WriteVInt(out, int32(len(w.docs))); err != nil {
		return err
	}
	return WriteFooter(out)
}

// -----------------------------------------------------------------------------
// Lucene104TermVectorsReader — reads .tvd written by Lucene104TermVectorsWriter.
// -----------------------------------------------------------------------------

// tv104Doc holds deserialized term vectors for one document.
type tv104Doc struct {
	fields map[string]*tv104Field
}

// tv104Field holds deserialized term vectors for one field.
type tv104Field struct {
	name         string
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
	terms        []*tv104Term
}

// tv104Term holds one term's data.
type tv104Term struct {
	text      []byte
	positions []tv104Pos
}

// tv104Pos holds one position entry.
type tv104Pos struct {
	position    int
	startOffset int
	endOffset   int
	payload     []byte
}

// Lucene104TermVectorsReader reads .tvd files written by Lucene104TermVectorsWriter.
type Lucene104TermVectorsReader struct {
	docs []tv104Doc
	mu   sync.RWMutex
}

// NewLucene104TermVectorsReader opens a Lucene104TermVectorsReader for the given segment.
func NewLucene104TermVectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, _ *index.FieldInfos, _ store.IOContext) (*Lucene104TermVectorsReader, error) {
	r := &Lucene104TermVectorsReader{}
	if err := r.load(dir, segmentInfo); err != nil {
		return nil, err
	}
	return r, nil
}

// load reads the .tvd file.
func (r *Lucene104TermVectorsReader) load(dir store.Directory, si *index.SegmentInfo) error {
	fileName := si.Name() + ".tvd"
	if !dir.FileExists(fileName) {
		return nil
	}
	rawIn, err := dir.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("open .tvd: %w", err)
	}
	defer rawIn.Close()

	in := store.NewChecksumIndexInput(rawIn)
	segID := si.GetID()
	if _, err := CheckIndexHeader(in, lucene104TVDataCodec,
		lucene104TVDataVersion, lucene104TVDataVersion, segID, ""); err != nil {
		return fmt.Errorf(".tvd header mismatch: %w", err)
	}

	numDocs, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	r.docs = make([]tv104Doc, numDocs)
	for i := int32(0); i < numDocs; i++ {
		numFields, err := store.ReadVInt(in)
		if err != nil {
			return fmt.Errorf("doc %d numFields: %w", i, err)
		}
		doc := tv104Doc{fields: make(map[string]*tv104Field, numFields)}
		for j := int32(0); j < numFields; j++ {
			fieldName, err := store.ReadString(in)
			if err != nil {
				return fmt.Errorf("doc %d field %d name: %w", i, j, err)
			}
			flags, err := in.ReadByte()
			if err != nil {
				return fmt.Errorf("doc %d field %d flags: %w", i, j, err)
			}
			hasPositions := flags&0x01 != 0
			hasOffsets := flags&0x02 != 0
			hasPayloads := flags&0x04 != 0

			numTerms, err := store.ReadVInt(in)
			if err != nil {
				return fmt.Errorf("doc %d field %d numTerms: %w", i, j, err)
			}
			f := &tv104Field{
				name:         fieldName,
				hasPositions: hasPositions,
				hasOffsets:   hasOffsets,
				hasPayloads:  hasPayloads,
				terms:        make([]*tv104Term, 0, numTerms),
			}
			for k := int32(0); k < numTerms; k++ {
				termLen, err := store.ReadVInt(in)
				if err != nil {
					return fmt.Errorf("doc %d field %d term %d len: %w", i, j, k, err)
				}
				termBytes, err := in.ReadBytesN(int(termLen))
				if err != nil {
					return fmt.Errorf("doc %d field %d term %d bytes: %w", i, j, k, err)
				}
				freq, err := store.ReadVInt(in)
				if err != nil {
					return fmt.Errorf("doc %d field %d term %d freq: %w", i, j, k, err)
				}
				t := &tv104Term{text: termBytes, positions: make([]tv104Pos, freq)}
				for p := int32(0); p < freq; p++ {
					var pos tv104Pos
					if hasPositions {
						v, err := store.ReadVInt(in)
						if err != nil {
							return err
						}
						pos.position = int(v)
					}
					if hasOffsets {
						so, err := store.ReadVInt(in)
						if err != nil {
							return err
						}
						eo, err := store.ReadVInt(in)
						if err != nil {
							return err
						}
						pos.startOffset = int(so)
						pos.endOffset = int(eo)
					}
					if hasPayloads {
						pl, err := store.ReadVInt(in)
						if err != nil {
							return err
						}
						if pl > 0 {
							payload, err := in.ReadBytesN(int(pl))
							if err != nil {
								return err
							}
							pos.payload = payload
						}
					}
					t.positions[p] = pos
				}
				f.terms = append(f.terms, t)
			}
			doc.fields[fieldName] = f
		}
		r.docs[i] = doc
	}
	return nil
}

// Get retrieves term vectors for the given document ID.
func (r *Lucene104TermVectorsReader) Get(docID int) (index.Fields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if docID < 0 || docID >= len(r.docs) {
		return &index.EmptyFields{}, nil
	}
	return newTV104Fields(r.docs[docID].fields), nil
}

// GetField retrieves the term vector for a specific field in a document.
func (r *Lucene104TermVectorsReader) GetField(docID int, field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if docID < 0 || docID >= len(r.docs) {
		return nil, nil
	}
	f, ok := r.docs[docID].fields[field]
	if !ok {
		return nil, nil
	}
	return newTV104Terms(f), nil
}

// Close releases resources.
func (r *Lucene104TermVectorsReader) Close() error {
	return nil
}

// -----------------------------------------------------------------------------
// index.Fields / index.Terms / index.TermsEnum / index.PostingsEnum adapters
// for the Lucene104TermVectorsReader deserialized data.
// -----------------------------------------------------------------------------

type tv104Fields struct {
	fields map[string]*tv104Field
	names  []string
}

func newTV104Fields(fields map[string]*tv104Field) *tv104Fields {
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	return &tv104Fields{fields: fields, names: names}
}

func (f *tv104Fields) Iterator() (index.FieldIterator, error) {
	return &tv104FieldIterator{names: f.names, pos: -1}, nil
}

func (f *tv104Fields) Terms(name string) (index.Terms, error) {
	fv, ok := f.fields[name]
	if !ok {
		return nil, nil
	}
	return newTV104Terms(fv), nil
}

func (f *tv104Fields) Size() int { return len(f.fields) }

type tv104FieldIterator struct {
	names []string
	pos   int
}

func (it *tv104FieldIterator) Next() (string, error) {
	it.pos++
	if it.pos >= len(it.names) {
		return "", nil
	}
	return it.names[it.pos], nil
}

func (it *tv104FieldIterator) HasNext() bool { return it.pos+1 < len(it.names) }

type tv104Terms struct {
	f *tv104Field
}

func newTV104Terms(f *tv104Field) *tv104Terms { return &tv104Terms{f: f} }

func (t *tv104Terms) GetIterator() (index.TermsEnum, error) {
	return &tv104TermsEnum{terms: t.f.terms, pos: -1, field: t.f.name}, nil
}

func (t *tv104Terms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	return t.GetIterator()
}

func (t *tv104Terms) Size() int64               { return int64(len(t.f.terms)) }
func (t *tv104Terms) GetDocCount() (int, error) { return 1, nil }
func (t *tv104Terms) GetSumDocFreq() (int64, error) {
	return int64(len(t.f.terms)), nil
}

func (t *tv104Terms) GetSumTotalTermFreq() (int64, error) {
	var total int64
	for _, term := range t.f.terms {
		total += int64(len(term.positions))
	}
	return total, nil
}

func (t *tv104Terms) HasFreqs() bool     { return true }
func (t *tv104Terms) HasOffsets() bool   { return t.f.hasOffsets }
func (t *tv104Terms) HasPositions() bool { return t.f.hasPositions }
func (t *tv104Terms) HasPayloads() bool  { return t.f.hasPayloads }

func (t *tv104Terms) GetMin() (*index.Term, error) {
	if len(t.f.terms) == 0 {
		return nil, nil
	}
	return index.NewTermFromBytes(t.f.name, t.f.terms[0].text), nil
}

func (t *tv104Terms) GetMax() (*index.Term, error) {
	if len(t.f.terms) == 0 {
		return nil, nil
	}
	return index.NewTermFromBytes(t.f.name, t.f.terms[len(t.f.terms)-1].text), nil
}

func (t *tv104Terms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	for _, term := range t.f.terms {
		if string(term.text) == termText {
			return &tv104PostingsEnum{term: term, f: t.f}, nil
		}
	}
	return nil, nil
}

type tv104TermsEnum struct {
	terms []*tv104Term
	pos   int
	field string
}

func (e *tv104TermsEnum) Next() (*index.Term, error) {
	e.pos++
	if e.pos >= len(e.terms) {
		return nil, nil
	}
	return index.NewTermFromBytes(e.field, e.terms[e.pos].text), nil
}

func (e *tv104TermsEnum) DocFreq() (int, error) {
	if e.pos < 0 || e.pos >= len(e.terms) {
		return 0, fmt.Errorf("iterator not positioned")
	}
	return 1, nil
}

func (e *tv104TermsEnum) TotalTermFreq() (int64, error) {
	if e.pos < 0 || e.pos >= len(e.terms) {
		return 0, fmt.Errorf("iterator not positioned")
	}
	return int64(len(e.terms[e.pos].positions)), nil
}

func (e *tv104TermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.pos < 0 || e.pos >= len(e.terms) {
		return nil, fmt.Errorf("iterator not positioned")
	}
	return nil, nil // simplified: only used for term presence checks
}

func (e *tv104TermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

func (e *tv104TermsEnum) SeekCeil(text *index.Term) (*index.Term, error) {
	textStr := text.Bytes.String()
	for i, t := range e.terms {
		if string(t.text) >= textStr {
			e.pos = i
			return index.NewTermFromBytes(e.field, t.text), nil
		}
	}
	return nil, nil
}

func (e *tv104TermsEnum) SeekExact(text *index.Term) (bool, error) {
	textStr := text.Bytes.String()
	for i, t := range e.terms {
		if string(t.text) == textStr {
			e.pos = i
			return true, nil
		}
	}
	return false, nil
}

func (e *tv104TermsEnum) Term() *index.Term {
	if e.pos < 0 || e.pos >= len(e.terms) {
		return nil
	}
	return index.NewTermFromBytes(e.field, e.terms[e.pos].text)
}

type tv104PostingsEnum struct {
	term   *tv104Term
	f      *tv104Field
	posIdx int
	docPos int // 0 = not started, 1 = at doc 0, 2 = exhausted
}

func (p *tv104PostingsEnum) NextDoc() (int, error) {
	if p.docPos == 0 {
		p.docPos = 1
		return 0, nil
	}
	return index.NO_MORE_DOCS, nil
}

func (p *tv104PostingsEnum) Advance(target int) (int, error) {
	if target <= 0 && p.docPos == 0 {
		p.docPos = 1
		return 0, nil
	}
	return index.NO_MORE_DOCS, nil
}

func (p *tv104PostingsEnum) DocID() int {
	switch p.docPos {
	case 0:
		return -1
	case 1:
		return 0
	default:
		return index.NO_MORE_DOCS
	}
}

func (p *tv104PostingsEnum) Freq() (int, error) { return len(p.term.positions), nil }

func (p *tv104PostingsEnum) NextPosition() (int, error) {
	if !p.f.hasPositions {
		return -1, fmt.Errorf("positions not available")
	}
	if p.posIdx >= len(p.term.positions) {
		return -1, nil
	}
	pos := p.term.positions[p.posIdx]
	p.posIdx++
	return pos.position, nil
}

func (p *tv104PostingsEnum) StartOffset() (int, error) {
	if !p.f.hasOffsets || p.posIdx == 0 {
		return -1, nil
	}
	return p.term.positions[p.posIdx-1].startOffset, nil
}

func (p *tv104PostingsEnum) EndOffset() (int, error) {
	if !p.f.hasOffsets || p.posIdx == 0 {
		return -1, nil
	}
	return p.term.positions[p.posIdx-1].endOffset, nil
}

func (p *tv104PostingsEnum) Payload() ([]byte, error) {
	if !p.f.hasPayloads || p.posIdx == 0 {
		return nil, nil
	}
	return p.term.positions[p.posIdx-1].payload, nil
}

func (p *tv104PostingsEnum) GetPayload() ([]byte, error) { return p.Payload() }
func (p *tv104PostingsEnum) Cost() int64                 { return int64(len(p.term.positions)) }
