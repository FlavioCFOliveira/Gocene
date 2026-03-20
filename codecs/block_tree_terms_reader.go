// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockTreeTermsReader reads terms from a block tree structure.
// This is the Go port of Lucene's BlockTreeTermsReader.
// It provides efficient term dictionary access using a trie-based index structure.
type BlockTreeTermsReader struct {
	state      *SegmentReadState
	fields     map[string]*BlockTreeTerms
	fieldNames []string
	mu         sync.RWMutex
	closed     bool
}

// NewBlockTreeTermsReader creates a new BlockTreeTermsReader.
// This is the Go port of BlockTreeTermsReader constructor.
func NewBlockTreeTermsReader(state *SegmentReadState) (*BlockTreeTermsReader, error) {
	reader := &BlockTreeTermsReader{
		state:  state,
		fields: make(map[string]*BlockTreeTerms),
	}

	if err := reader.load(); err != nil {
		return nil, err
	}

	return reader, nil
}

// load reads the block tree terms index from disk.
func (r *BlockTreeTermsReader) load() error {
	segmentName := r.state.SegmentInfo.Name()
	suffix := r.state.SegmentSuffix
	if suffix != "" {
		suffix = "_" + suffix
	}

	// Terms index file: .tip
	tipFileName := fmt.Sprintf("%s%s.tip", segmentName, suffix)
	if !r.state.Directory.FileExists(tipFileName) {
		// No terms index file - return empty reader
		return nil
	}

	// Terms block file: .tim
	timFileName := fmt.Sprintf("%s%s.tim", segmentName, suffix)
	if !r.state.Directory.FileExists(timFileName) {
		return fmt.Errorf("terms block file %s not found", timFileName)
	}

	// Open the terms index file
	tipIn, err := r.state.Directory.OpenInput(tipFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open terms index file %s: %w", tipFileName, err)
	}
	defer tipIn.Close()

	// Open the terms block file
	timIn, err := r.state.Directory.OpenInput(timFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open terms block file %s: %w", timFileName, err)
	}
	defer timIn.Close()

	// Read header from .tip file
	if err := r.readHeader(tipIn, "TIP"); err != nil {
		return err
	}

	// Read field count
	fieldCount, err := store.ReadVInt(tipIn)
	if err != nil {
		return fmt.Errorf("failed to read field count: %w", err)
	}

	// Read each field's terms
	for i := int32(0); i < fieldCount; i++ {
		// Read field name
		fieldName, err := store.ReadString(tipIn)
		if err != nil {
			return fmt.Errorf("failed to read field name: %w", err)
		}

		// Read root file pointer for this field's trie
		rootFP, err := store.ReadVLong(tipIn)
		if err != nil {
			return fmt.Errorf("failed to read root file pointer: %w", err)
		}

		// Read number of terms for this field
		numTerms, err := store.ReadVLong(tipIn)
		if err != nil {
			return fmt.Errorf("failed to read term count: %w", err)
		}

		// Read index options
		indexOptionsByte, err := tipIn.ReadByte()
		if err != nil {
			return fmt.Errorf("failed to read index options: %w", err)
		}
		indexOptions := int8(indexOptionsByte)

		// Read has payloads flag
		hasPayloadsByte, err := tipIn.ReadByte()
		if err != nil {
			return fmt.Errorf("failed to read has payloads flag: %w", err)
		}
		hasPayloads := hasPayloadsByte != 0

		// Create the trie reader for this field
		trieReader, err := NewTrieReader(timIn, rootFP)
		if err != nil {
			return fmt.Errorf("failed to create trie reader for field %s: %w", fieldName, err)
		}

		// Create BlockTreeTerms for this field
		terms := &BlockTreeTerms{
			fieldName:    fieldName,
			numTerms:     numTerms,
			indexOptions: index.IndexOptions(indexOptions),
			hasPayloads:  hasPayloads,
			trieReader:   trieReader,
			timIn:        timIn,
		}

		r.fields[fieldName] = terms
		r.fieldNames = append(r.fieldNames, fieldName)
	}

	return nil
}

// readHeader reads and validates the file header.
func (r *BlockTreeTermsReader) readHeader(in store.IndexInput, codec string) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}

	expectedMagic := uint32(0)
	if codec == "TIP" {
		expectedMagic = 0x54495000 // "TIP\0"
	} else if codec == "TIM" {
		expectedMagic = 0x54494D00 // "TIM\0"
	}

	if magic != expectedMagic {
		return fmt.Errorf("invalid magic number: expected 0x%08X, got 0x%08X", expectedMagic, magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}

	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Terms returns the terms for a field.
// This implements the FieldsProducer interface.
func (r *BlockTreeTermsReader) Terms(field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("BlockTreeTermsReader is closed")
	}

	terms, ok := r.fields[field]
	if !ok {
		return nil, nil
	}

	return terms, nil
}

// Close releases resources.
// This implements the FieldsProducer interface.
func (r *BlockTreeTermsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	r.fields = nil
	r.fieldNames = nil

	return nil
}

// BlockTreeTerms implements index.Terms for block tree structure.
// This provides access to terms for a specific field stored in block tree format.
type BlockTreeTerms struct {
	*index.TermsBase
	fieldName    string
	numTerms     int64
	indexOptions index.IndexOptions
	hasPayloads  bool
	trieReader   *TrieReader
	timIn        store.IndexInput
}

// GetIterator returns a TermsEnum for iterating over all terms in this field.
func (t *BlockTreeTerms) GetIterator() (index.TermsEnum, error) {
	return NewBlockTreeTermsEnum(t, nil)
}

// GetIteratorWithSeek returns a TermsEnum positioned at or after the given term.
func (t *BlockTreeTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	return NewBlockTreeTermsEnum(t, seekTerm)
}

// Size returns the number of unique terms in this field.
func (t *BlockTreeTerms) Size() int64 {
	return t.numTerms
}

// GetDocCount returns the number of documents that contain at least one term in this field.
func (t *BlockTreeTerms) GetDocCount() (int, error) {
	// This would require reading additional statistics from the index
	// For now, return -1 (unknown)
	return -1, nil
}

// GetSumDocFreq returns the total number of postings for this field.
func (t *BlockTreeTerms) GetSumDocFreq() (int64, error) {
	// This would require reading additional statistics from the index
	// For now, return -1 (unknown)
	return -1, nil
}

// GetSumTotalTermFreq returns the total number of term occurrences for this field.
func (t *BlockTreeTerms) GetSumTotalTermFreq() (int64, error) {
	// This would require reading additional statistics from the index
	// For now, return -1 (unknown)
	return -1, nil
}

// HasFreqs returns true if term frequencies are available for this field.
func (t *BlockTreeTerms) HasFreqs() bool {
	return t.indexOptions >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets returns true if term offsets are available for this field.
func (t *BlockTreeTerms) HasOffsets() bool {
	return t.indexOptions >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions returns true if term positions are available for this field.
func (t *BlockTreeTerms) HasPositions() bool {
	return t.indexOptions >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads returns true if payloads are available for this field.
func (t *BlockTreeTerms) HasPayloads() bool {
	return t.hasPayloads
}

// GetMin returns the smallest term in this field.
func (t *BlockTreeTerms) GetMin() (*index.Term, error) {
	// Would need to traverse to the leftmost leaf of the trie
	// For now, return nil
	return nil, nil
}

// GetMax returns the largest term in this field.
func (t *BlockTreeTerms) GetMax() (*index.Term, error) {
	// Would need to traverse to the rightmost leaf of the trie
	// For now, return nil
	return nil, nil
}

// GetPostingsReader returns the postings for a term.
func (t *BlockTreeTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	// Create a term to seek
	seekTerm := index.NewTerm(t.fieldName, termText)

	// Get iterator positioned at the term
	iter, err := t.GetIteratorWithSeek(seekTerm)
	if err != nil {
		return nil, err
	}
	if iter == nil {
		return nil, nil
	}

	// Get current term
	currTerm := iter.Term()
	if currTerm == nil || currTerm.Text() != termText {
		return nil, nil
	}

	// Get postings
	return iter.Postings(flags)
}

// BlockTreeTermsEnum implements index.TermsEnum for block tree terms.
type BlockTreeTermsEnum struct {
	terms       *BlockTreeTerms
	current     *TrieNode
	termBytes   *util.BytesRefBuilder
	seekTerm    *index.Term
	seekPending bool
}

// NewBlockTreeTermsEnum creates a new BlockTreeTermsEnum.
func NewBlockTreeTermsEnum(terms *BlockTreeTerms, seekTerm *index.Term) (*BlockTreeTermsEnum, error) {
	enum := &BlockTreeTermsEnum{
		terms:     terms,
		termBytes: util.NewBytesRefBuilder(),
		seekTerm:  seekTerm,
	}

	if seekTerm != nil {
		enum.seekPending = true
	}

	return enum, nil
}

// Next advances to the next term.
func (e *BlockTreeTermsEnum) Next() (*index.Term, error) {
	if e.seekPending {
		e.seekPending = false
		return e.doSeek()
	}

	// TODO: Implement iteration over trie
	// This requires traversing the trie structure in order
	return nil, nil
}

// doSeek performs the seek operation.
func (e *BlockTreeTermsEnum) doSeek() (*index.Term, error) {
	if e.seekTerm == nil {
		return e.Next()
	}

	seekBytes := []byte(e.seekTerm.Text())

	// Start from root
	e.current = e.terms.trieReader.Root
	e.termBytes.SetLength(0)

	// Traverse the trie following the seek term bytes
	for i, b := range seekBytes {
		child := NewTrieNode()
		found, err := e.terms.trieReader.LookupChild(int(b)&0xFF, e.current, child)
		if err != nil {
			return nil, err
		}
		if found == nil {
			// Term not found, position at next term
			return e.seekToNext(i)
		}

		e.current = child
		// Append byte to termBytes
		e.termBytes.Grow(i + 1)
		e.termBytes.Bytes()[i] = byte(b)
		e.termBytes.SetLength(i + 1)

		// Check if this node has output (i.e., it's a valid term)
		if e.current.HasOutput() && i == len(seekBytes)-1 {
			// Found exact match
			return e.currentTerm(), nil
		}
	}

	// If we get here, seek term is a prefix of some longer term
	// Return the next term after this prefix
	return e.seekToNext(len(seekBytes))
}

// seekToNext seeks to the next term after the current position.
func (e *BlockTreeTermsEnum) seekToNext(depth int) (*index.Term, error) {
	// TODO: Implement seeking to next term
	// This requires finding the next leaf node in the trie
	return nil, nil
}

// currentTerm returns the current term.
func (e *BlockTreeTermsEnum) currentTerm() *index.Term {
	return index.NewTerm(e.terms.fieldName, e.termBytes.Get().String())
}

// SeekCeil seeks to the term at or after the given term.
func (e *BlockTreeTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	e.seekTerm = term
	e.seekPending = true
	return e.Next()
}

// SeekExact seeks to the exact term.
func (e *BlockTreeTermsEnum) SeekExact(term *index.Term) (bool, error) {
	result, err := e.SeekCeil(term)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return result.Equals(term), nil
}

// Term returns the current term.
func (e *BlockTreeTermsEnum) Term() *index.Term {
	if e.current == nil {
		return nil
	}
	return e.currentTerm()
}

// DocFreq returns the document frequency for the current term.
func (e *BlockTreeTermsEnum) DocFreq() (int, error) {
	if e.current == nil || !e.current.HasOutput() {
		return 0, nil
	}

	// Read doc freq from the postings data
	// TODO: Implement reading doc freq from block data
	return 0, nil
}

// TotalTermFreq returns the total term frequency.
func (e *BlockTreeTermsEnum) TotalTermFreq() (int64, error) {
	if e.current == nil || !e.current.HasOutput() {
		return 0, nil
	}

	// Read total term freq from the postings data
	// TODO: Implement reading total term freq from block data
	return 0, nil
}

// Postings returns a PostingsEnum for the current term.
func (e *BlockTreeTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.current == nil || !e.current.HasOutput() {
		return nil, nil
	}

	// TODO: Implement reading postings from block data
	return nil, nil
}

// PostingsWithLiveDocs returns a PostingsEnum with live docs applied.
func (e *BlockTreeTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	pe, err := e.Postings(flags)
	if err != nil {
		return nil, err
	}
	if pe == nil || liveDocs == nil {
		return pe, nil
	}
	return &blockTreeLiveDocsPostingsEnum{PostingsEnum: pe, liveDocs: liveDocs}, nil
}

// blockTreeLiveDocsPostingsEnum wraps a PostingsEnum with live docs filtering.
type blockTreeLiveDocsPostingsEnum struct {
	index.PostingsEnum
	liveDocs util.Bits
}

// NextDoc advances to the next live document.
func (pe *blockTreeLiveDocsPostingsEnum) NextDoc() (int, error) {
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
func (pe *blockTreeLiveDocsPostingsEnum) Advance(target int) (int, error) {
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

// Ensure BlockTreeTermsReader implements FieldsProducer
var _ FieldsProducer = (*BlockTreeTermsReader)(nil)

// Ensure BlockTreeTerms implements index.Terms
var _ index.Terms = (*BlockTreeTerms)(nil)

// Ensure BlockTreeTermsEnum implements index.TermsEnum
var _ index.TermsEnum = (*BlockTreeTermsEnum)(nil)
