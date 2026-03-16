// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PostingsFormat handles encoding/decoding of postings (term -> document mappings).
// This is the Go port of Lucene's org.apache.lucene.codecs.PostingsFormat.
//
// Postings are stored in files like _X.pst and contain the mapping from
// terms to documents, frequencies, positions, and offsets.
type PostingsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsConsumer returns a consumer for writing postings.
	// The caller should close the returned consumer when done.
	FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error)

	// FieldsProducer returns a producer for reading postings.
	// The caller should close the returned producer when done.
	FieldsProducer(state *SegmentReadState) (FieldsProducer, error)
}

// BasePostingsFormat provides common functionality.
type BasePostingsFormat struct {
	name string
}

// NewBasePostingsFormat creates a new BasePostingsFormat.
func NewBasePostingsFormat(name string) *BasePostingsFormat {
	return &BasePostingsFormat{name: name}
}

// Name returns the format name.
func (f *BasePostingsFormat) Name() string {
	return f.name
}

// FieldsConsumer returns a fields consumer (must be implemented by subclasses).
func (f *BasePostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return nil, fmt.Errorf("FieldsConsumer not implemented")
}

// FieldsProducer returns a fields producer (must be implemented by subclasses).
func (f *BasePostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	return nil, fmt.Errorf("FieldsProducer not implemented")
}

// Lucene104PostingsFormat is the Lucene 10.4 postings format.
//
// This implementation uses a simple block-based format for storing term dictionaries
// and postings. The format is compatible with Lucene's core data structures but uses
// a simplified encoding for clarity.
type Lucene104PostingsFormat struct {
	*BasePostingsFormat
}

// NewLucene104PostingsFormat creates a new Lucene104PostingsFormat.
func NewLucene104PostingsFormat() *Lucene104PostingsFormat {
	return &Lucene104PostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat("Lucene104PostingsFormat"),
	}
}

// FieldsConsumer returns a fields consumer for writing postings.
// Uses BlockTreeTermsWriter for efficient term dictionary encoding.
func (f *Lucene104PostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	// Use BlockTreeTermsWriter for Lucene-compatible block tree format
	return NewBlockTreeTermsWriter(state)
}

// FieldsProducer returns a fields producer for reading postings.
// Uses BlockTreeTermsReader for efficient term dictionary access.
func (f *Lucene104PostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	// Use BlockTreeTermsReader for Lucene-compatible block tree format
	return NewBlockTreeTermsReader(state)
}

// Lucene104FieldsConsumer writes postings data to disk.
type Lucene104FieldsConsumer struct {
	state   *SegmentWriteState
	fields  map[string]*fieldData
	mu      sync.Mutex
	closed  bool
}

// fieldData stores all data for a single field
type fieldData struct {
	terms map[string]*termData
}

// termData stores all data for a single term
type termData struct {
	text      string
	docFreq   int
	totalFreq int64
	postings  []postingEntry
}

// postingEntry stores data for a single posting (doc + positions)
type postingEntry struct {
	docID     int
	freq      int
	positions []int
	offsets   []offsetEntry
	payload   []byte
}

type offsetEntry struct {
	start int
	end   int
}

// NewLucene104FieldsConsumer creates a new Lucene104FieldsConsumer.
func NewLucene104FieldsConsumer(state *SegmentWriteState) *Lucene104FieldsConsumer {
	return &Lucene104FieldsConsumer{
		state:  state,
		fields: make(map[string]*fieldData),
	}
}

// Write writes a field's postings to the internal data structure.
func (c *Lucene104FieldsConsumer) Write(field string, terms index.Terms) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("fields consumer is closed")
	}

	// Create field data if not exists
	fd, ok := c.fields[field]
	if !ok {
		fd = &fieldData{terms: make(map[string]*termData)}
		c.fields[field] = fd
	}

	// Iterate over all terms and collect data
	te, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("failed to get terms iterator: %w", err)
	}

	for {
		term, err := te.Next()
		if err != nil {
			return fmt.Errorf("error iterating terms: %w", err)
		}
		if term == nil {
			break
		}

		// Get term statistics
		docFreq, err := te.DocFreq()
		if err != nil {
			return fmt.Errorf("error getting doc freq: %w", err)
		}

		totalFreq, err := te.TotalTermFreq()
		if err != nil {
			return fmt.Errorf("error getting total term freq: %w", err)
		}

		// Get postings for this term
		pe, err := te.Postings(0)
		if err != nil {
			return fmt.Errorf("error getting postings: %w", err)
		}

		td := &termData{
			text:      term.Text(),
			docFreq:   docFreq,
			totalFreq: totalFreq,
			postings:  make([]postingEntry, 0),
		}

		// Collect all postings
		for {
			docID, err := pe.NextDoc()
			if err != nil {
				return fmt.Errorf("error iterating postings: %w", err)
			}
			if docID == index.NO_MORE_DOCS {
				break
			}

			freq, err := pe.Freq()
			if err != nil {
				return fmt.Errorf("error getting freq: %w", err)
			}

			entry := postingEntry{
				docID: docID,
				freq:  freq,
			}

			// Collect positions if available
			if terms.HasPositions() {
				entry.positions = make([]int, 0)
				entry.offsets = make([]offsetEntry, 0)
				for i := 0; i < freq; i++ {
					pos, err := pe.NextPosition()
					if err != nil {
						return fmt.Errorf("error getting position: %w", err)
					}
					// If no more positions available, stop collecting
					if pos == index.NO_MORE_POSITIONS {
						break
					}
					entry.positions = append(entry.positions, pos)

					if terms.HasOffsets() {
						start, err := pe.StartOffset()
						if err != nil {
							return fmt.Errorf("error getting start offset: %w", err)
						}
						end, err := pe.EndOffset()
						if err != nil {
							return fmt.Errorf("error getting end offset: %w", err)
						}
						entry.offsets = append(entry.offsets, offsetEntry{start: start, end: end})
					}
				}
			}

			td.postings = append(td.postings, entry)
		}

		fd.terms[term.Text()] = td
	}

	return nil
}

// Close writes the data to disk and releases resources.
func (c *Lucene104FieldsConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Generate file name
	segmentName := c.state.SegmentInfo.Name()
	suffix := c.state.SegmentSuffix
	if suffix != "" {
		suffix = "_" + suffix
	}
	fileName := fmt.Sprintf("%s%s.pst", segmentName, suffix)

	// Create output
	out, err := c.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", fileName, err)
	}
	defer out.Close()

	// Write magic number
	if err := store.WriteUint32(out, 0x50535400); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// Write version
	if err := store.WriteUint32(out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	// Write number of fields
	if err := store.WriteVInt(out, int32(len(c.fields))); err != nil {
		return fmt.Errorf("failed to write field count: %w", err)
	}

	// Write each field
	for fieldName, fd := range c.fields {
		// Write field name
		if err := store.WriteString(out, fieldName); err != nil {
			return fmt.Errorf("failed to write field name: %w", err)
		}

		// Write number of terms
		if err := store.WriteVInt(out, int32(len(fd.terms))); err != nil {
			return fmt.Errorf("failed to write term count: %w", err)
		}

		// Sort terms for deterministic output
		termTexts := make([]string, 0, len(fd.terms))
		for text := range fd.terms {
			termTexts = append(termTexts, text)
		}
		sort.Strings(termTexts)

		// Write each term
		for _, text := range termTexts {
			td := fd.terms[text]

			// Write term text
			if err := store.WriteString(out, text); err != nil {
				return fmt.Errorf("failed to write term text: %w", err)
			}

			// Write doc freq and total freq
			if err := store.WriteVInt(out, int32(td.docFreq)); err != nil {
				return fmt.Errorf("failed to write doc freq: %w", err)
			}
			if err := store.WriteVLong(out, td.totalFreq); err != nil {
				return fmt.Errorf("failed to write total freq: %w", err)
			}

			// Write number of postings
			if err := store.WriteVInt(out, int32(len(td.postings))); err != nil {
				return fmt.Errorf("failed to write posting count: %w", err)
			}

			// Write each posting
			for _, entry := range td.postings {
				if err := store.WriteVInt(out, int32(entry.docID)); err != nil {
					return fmt.Errorf("failed to write doc id: %w", err)
				}
				if err := store.WriteVInt(out, int32(entry.freq)); err != nil {
					return fmt.Errorf("failed to write freq: %w", err)
				}

				// Write position count (always write, even if 0)
				if err := store.WriteVInt(out, int32(len(entry.positions))); err != nil {
					return fmt.Errorf("failed to write position count: %w", err)
				}
				// Write positions if present
				for _, pos := range entry.positions {
					if err := store.WriteVInt(out, int32(pos)); err != nil {
						return fmt.Errorf("failed to write position: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// Lucene104FieldsProducer reads postings data from disk.
type Lucene104FieldsProducer struct {
	state    *SegmentReadState
	fields   map[string]*fieldDataRead
	mu       sync.RWMutex
	closed   bool
}

// fieldDataRead stores all data for a single field (read format)
type fieldDataRead struct {
	terms map[string]*termDataRead
}

// termDataRead stores all data for a single term (read format)
type termDataRead struct {
	text      string
	docFreq   int
	totalFreq int64
	postings  []postingEntry
}

// NewLucene104FieldsProducer creates a new Lucene104FieldsProducer.
func NewLucene104FieldsProducer(state *SegmentReadState) *Lucene104FieldsProducer {
	p := &Lucene104FieldsProducer{
		state:  state,
		fields: make(map[string]*fieldDataRead),
	}
	// Load data lazily or eagerly - we'll do it eagerly for simplicity
	p.load()
	return p
}

// load reads all data from disk into memory.
func (p *Lucene104FieldsProducer) load() error {
	// Generate file name
	segmentName := p.state.SegmentInfo.Name()
	suffix := p.state.SegmentSuffix
	if suffix != "" {
		suffix = "_" + suffix
	}
	fileName := fmt.Sprintf("%s%s.pst", segmentName, suffix)

	// Check if file exists
	if !p.state.Directory.FileExists(fileName) {
		// No postings file - return empty producer
		return nil
	}

	// Open input
	in, err := p.state.Directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open input file %s: %w", fileName, err)
	}
	defer in.Close()

	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x50535400 {
		return fmt.Errorf("invalid magic number: expected 0x50535400, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	// Read number of fields
	fieldCount, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("failed to read field count: %w", err)
	}

	// Read each field
	for i := int32(0); i < fieldCount; i++ {
		// Read field name
		fieldName, err := store.ReadString(in)
		if err != nil {
			return fmt.Errorf("failed to read field name: %w", err)
		}

		fd := &fieldDataRead{terms: make(map[string]*termDataRead)}
		p.fields[fieldName] = fd

		// Read number of terms
		termCount, err := store.ReadVInt(in)
		if err != nil {
			return fmt.Errorf("failed to read term count: %w", err)
		}

		// Read each term
		for j := int32(0); j < termCount; j++ {
			// Read term text
			text, err := store.ReadString(in)
			if err != nil {
				return fmt.Errorf("failed to read term text: %w", err)
			}

			td := &termDataRead{text: text}

			// Read doc freq and total freq
			docFreq, err := store.ReadVInt(in)
			if err != nil {
				return fmt.Errorf("failed to read doc freq: %w", err)
			}
			td.docFreq = int(docFreq)

			totalFreq, err := store.ReadVLong(in)
			if err != nil {
				return fmt.Errorf("failed to read total freq: %w", err)
			}
			td.totalFreq = totalFreq

			// Read number of postings
			postingCount, err := store.ReadVInt(in)
			if err != nil {
				return fmt.Errorf("failed to read posting count: %w", err)
			}

			td.postings = make([]postingEntry, postingCount)

			// Read each posting
			for k := int32(0); k < postingCount; k++ {
				docID, err := store.ReadVInt(in)
				if err != nil {
					return fmt.Errorf("failed to read doc id: %w", err)
				}
				td.postings[k].docID = int(docID)

				freq, err := store.ReadVInt(in)
				if err != nil {
					return fmt.Errorf("failed to read freq: %w", err)
				}
				td.postings[k].freq = int(freq)

				// Read position count
				posCount, err := store.ReadVInt(in)
				if err != nil {
					return fmt.Errorf("failed to read position count: %w", err)
				}
				if posCount > 0 {
					td.postings[k].positions = make([]int, posCount)
					for m := int32(0); m < posCount; m++ {
						pos, err := store.ReadVInt(in)
						if err != nil {
							return fmt.Errorf("failed to read position: %w", err)
						}
						td.postings[k].positions[m] = int(pos)
					}
				}
			}

			fd.terms[text] = td
		}
	}

	return nil
}

// Terms returns the terms for a field.
func (p *Lucene104FieldsProducer) Terms(field string) (index.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("fields producer is closed")
	}

	fd, ok := p.fields[field]
	if !ok {
		return nil, nil
	}

	return &memoryTerms{fieldData: fd}, nil
}

// Close releases resources.
func (p *Lucene104FieldsProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	p.fields = nil
	return nil
}

// memoryTerms implements index.Terms for in-memory data.
type memoryTerms struct {
	*index.TermsBase
	fieldData *fieldDataRead
}

// GetIterator returns a TermsEnum for iterating over terms.
func (t *memoryTerms) GetIterator() (index.TermsEnum, error) {
	// Sort terms
	termTexts := make([]string, 0, len(t.fieldData.terms))
	for text := range t.fieldData.terms {
		termTexts = append(termTexts, text)
	}
	sort.Strings(termTexts)
	return &memoryTermsEnum{terms: t.fieldData.terms, termTexts: termTexts, pos: -1}, nil
}

// GetIteratorWithSeek returns a TermsEnum positioned at or after the given term.
func (t *memoryTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		_, err = te.SeekCeil(seekTerm)
		if err != nil {
			return nil, err
		}
	}
	return te, nil
}

// Size returns the number of terms.
func (t *memoryTerms) Size() int64 {
	return int64(len(t.fieldData.terms))
}

// GetMin returns the smallest term.
func (t *memoryTerms) GetMin() (*index.Term, error) {
	if len(t.fieldData.terms) == 0 {
		return nil, nil
	}
	termTexts := make([]string, 0, len(t.fieldData.terms))
	for text := range t.fieldData.terms {
		termTexts = append(termTexts, text)
	}
	sort.Strings(termTexts)
	return index.NewTerm("", termTexts[0]), nil
}

// GetMax returns the largest term.
func (t *memoryTerms) GetMax() (*index.Term, error) {
	if len(t.fieldData.terms) == 0 {
		return nil, nil
	}
	termTexts := make([]string, 0, len(t.fieldData.terms))
	for text := range t.fieldData.terms {
		termTexts = append(termTexts, text)
	}
	sort.Strings(termTexts)
	return index.NewTerm("", termTexts[len(termTexts)-1]), nil
}

// memoryTermsEnum implements index.TermsEnum for in-memory data.
type memoryTermsEnum struct {
	terms     map[string]*termDataRead
	termTexts []string
	pos       int
	curr      *index.Term
}

// Next advances to the next term.
func (te *memoryTermsEnum) Next() (*index.Term, error) {
	te.pos++
	if te.pos >= len(te.termTexts) {
		te.curr = nil
		return nil, nil
	}
	te.curr = index.NewTerm("", te.termTexts[te.pos])
	return te.curr, nil
}

// SeekCeil seeks to the term at or after the given term.
func (te *memoryTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	idx := sort.SearchStrings(te.termTexts, term.Text())
	te.pos = idx - 1
	return te.Next()
}

// SeekExact seeks to the exact term.
func (te *memoryTermsEnum) SeekExact(term *index.Term) (bool, error) {
	idx := sort.SearchStrings(te.termTexts, term.Text())
	if idx < len(te.termTexts) && te.termTexts[idx] == term.Text() {
		te.pos = idx - 1
		_, err := te.Next()
		return true, err
	}
	return false, nil
}

// Term returns the current term.
func (te *memoryTermsEnum) Term() *index.Term {
	return te.curr
}

// DocFreq returns the document frequency for the current term.
func (te *memoryTermsEnum) DocFreq() (int, error) {
	if te.curr == nil {
		return 0, nil
	}
	td, ok := te.terms[te.curr.Text()]
	if !ok {
		return 0, nil
	}
	return td.docFreq, nil
}

// TotalTermFreq returns the total term frequency.
func (te *memoryTermsEnum) TotalTermFreq() (int64, error) {
	if te.curr == nil {
		return 0, nil
	}
	td, ok := te.terms[te.curr.Text()]
	if !ok {
		return 0, nil
	}
	return td.totalFreq, nil
}

// Postings returns a PostingsEnum for the current term.
func (te *memoryTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if te.curr == nil {
		return nil, nil
	}
	td, ok := te.terms[te.curr.Text()]
	if !ok {
		return nil, nil
	}
	return &memoryPostingsEnum{postings: td.postings, pos: -1}, nil
}

// PostingsWithLiveDocs returns a PostingsEnum with live docs applied.
func (te *memoryTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	pe, err := te.Postings(flags)
	if err != nil {
		return nil, err
	}
	if pe == nil || liveDocs == nil {
		return pe, nil
	}
	return &liveDocsPostingsEnum{PostingsEnum: pe, liveDocs: liveDocs}, nil
}

// memoryPostingsEnum implements index.PostingsEnum for in-memory data.
type memoryPostingsEnum struct {
	postings []postingEntry
	pos      int
	currDoc  int
}

// NextDoc advances to the next document.
func (pe *memoryPostingsEnum) NextDoc() (int, error) {
	pe.pos++
	if pe.pos >= len(pe.postings) {
		pe.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	pe.currDoc = pe.postings[pe.pos].docID
	return pe.currDoc, nil
}

// Advance advances to the first doc >= target.
func (pe *memoryPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target || doc == index.NO_MORE_DOCS {
			return doc, nil
		}
	}
}

// DocID returns the current document ID.
func (pe *memoryPostingsEnum) DocID() int {
	if pe.pos < 0 {
		return -1
	}
	return pe.currDoc
}

// Freq returns the term frequency in the current document.
func (pe *memoryPostingsEnum) Freq() (int, error) {
	if pe.pos < 0 || pe.pos >= len(pe.postings) {
		return 0, nil
	}
	return pe.postings[pe.pos].freq, nil
}

// NextPosition advances to the next position.
func (pe *memoryPostingsEnum) NextPosition() (int, error) {
	return index.NO_MORE_POSITIONS, nil
}

// StartOffset returns the start offset.
func (pe *memoryPostingsEnum) StartOffset() (int, error) {
	return -1, nil
}

// EndOffset returns the end offset.
func (pe *memoryPostingsEnum) EndOffset() (int, error) {
	return -1, nil
}

// GetPayload returns the payload.
func (pe *memoryPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

// Cost returns an estimate of the cost.
func (pe *memoryPostingsEnum) Cost() int64 {
	return int64(len(pe.postings))
}

// liveDocsPostingsEnum wraps a PostingsEnum with live docs filtering.
type liveDocsPostingsEnum struct {
	index.PostingsEnum
	liveDocs util.Bits
}

// NextDoc advances to the next live document.
func (pe *liveDocsPostingsEnum) NextDoc() (int, error) {
	for {
		doc, err := pe.PostingsEnum.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc == index.NO_MORE_DOCS {
			return doc, nil
		}
		if pe.liveDocs == nil || pe.liveDocs.Get(doc) {
			return doc, nil
		}
	}
}

// Advance advances to the first live doc >= target.
func (pe *liveDocsPostingsEnum) Advance(target int) (int, error) {
	doc, err := pe.PostingsEnum.Advance(target)
	if err != nil {
		return index.NO_MORE_DOCS, err
	}
	if doc == index.NO_MORE_DOCS {
		return doc, nil
	}
	if pe.liveDocs == nil || pe.liveDocs.Get(doc) {
		return doc, nil
	}
	return pe.NextDoc()
}

// FieldsConsumer is a consumer for writing postings.
// This is the Go port of Lucene's org.apache.lucene.codecs.FieldsConsumer.
type FieldsConsumer interface {
	// Write writes a field's postings.
	Write(field string, terms index.Terms) error

	// Close releases resources.
	Close() error
}

// FieldsProducer is a producer for reading postings.
// This is the Go port of Lucene's org.apache.lucene.codecs.FieldsProducer.
type FieldsProducer interface {
	// Terms returns the terms for a field.
	Terms(field string) (index.Terms, error)

	// Close releases resources.
	Close() error
}

// SegmentWriteState holds the state for writing a segment.
type SegmentWriteState struct {
	// Directory is where the segment files are written.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *index.SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *index.FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}

// SegmentReadState holds the state for reading a segment.
type SegmentReadState struct {
	// Directory is where the segment files are read from.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *index.SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *index.FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}