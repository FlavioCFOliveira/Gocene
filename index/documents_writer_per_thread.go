// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentsWriterPerThread handles document processing for a single thread.
// Each thread gets its own DWPT to avoid contention during indexing.
//
// This is the Go port of Lucene's org.apache.lucene.index.DocumentsWriterPerThread.
type DocumentsWriterPerThread struct {
	mu sync.RWMutex

	// parent is the DocumentsWriter that owns this DWPT
	parent *DocumentsWriter

	// segmentInfo holds segment information for the segment being built
	segmentInfo *SegmentInfo

	// fieldInfosBuilder builds field info as documents are added
	fieldInfosBuilder *FieldInfosBuilder

	// numDocsInRAM tracks documents in memory for this DWPT
	numDocsInRAM int

	// invertedIndex holds the in-memory postings data
	invertedIndex *InvertedIndex

	// storedFields holds stored field data for each document
	storedFields *StoredFieldsBuffer

	// docValues holds doc values data per field
	docValues map[string]*DocValuesBuffer

	// termVectors holds term vectors per document (if enabled)
	termVectors *TermVectorsBuffer

	// lastDocID is the last document ID assigned
	lastDocID int

	// flushPending indicates a flush is pending
	flushPending bool

	// bytesUsed estimates memory usage
	bytesUsed int64

	// deleteQueue holds pending delete operations
	pendingDeletes []*Term
}

// InvertedIndex holds the in-memory postings data structure.
// This maps terms to document IDs and positions.
type InvertedIndex struct {
	mu sync.RWMutex

	// fields maps field name to per-field postings
	fields map[string]*FieldPostings

	// numTerms total number of unique terms across all fields
	numTerms int64
}

// FieldPostings holds postings for a single field.
type FieldPostings struct {
	mu sync.RWMutex

	// terms maps term text to posting list
	terms map[string]*Posting

	// fieldInfo is the field info for this field
	fieldInfo *FieldInfo
}

// Posting holds the posting list for a single term.
type Posting struct {
	// docIDs is the list of document IDs containing this term
	docIDs []int

	// freqs is the frequency of this term in each document
	freqs []int

	// positions holds positions for each occurrence (if positions are indexed)
	positions [][]int

	// startOffsets holds start character offsets (if offsets are indexed)
	startOffsets [][]int

	// endOffsets holds end character offsets (if offsets are indexed)
	endOffsets [][]int
}

// StoredFieldsBuffer holds stored field data in memory.
type StoredFieldsBuffer struct {
	mu sync.RWMutex

	// documents holds stored field data per document
	documents []*StoredDocument

	// totalBytes estimates total bytes stored
	totalBytes int64
}

// StoredDocument holds stored fields for a single document.
type StoredDocument struct {
	// fields holds the stored fields for this document
	fields []*StoredField
}

// StoredField represents a single stored field value.
type StoredField struct {
	name         string
	stringValue  string
	binaryValue  []byte
	numericValue interface{}
}

// DocValuesBuffer holds doc values for a field.
type DocValuesBuffer struct {
	mu sync.RWMutex

	// values holds doc values per document
	values []interface{}

	// dvType is the doc values type
	dvType DocValuesType
}

// TermVectorsBuffer holds term vectors for documents.
type TermVectorsBuffer struct {
	mu sync.RWMutex

	// vectors holds term vectors per document
	// maps docID -> field name -> term vector data
	vectors []map[string]*FieldTermVector
}

// FieldTermVector holds term vector data for a field.
type FieldTermVector struct {
	// terms is the list of terms in this field
	terms []string

	// freqs is the frequency of each term
	freqs []int

	// positions holds positions for each term (if positions are stored)
	positions [][]int

	// startOffsets holds start offsets for each term (if offsets are stored)
	startOffsets [][]int

	// endOffsets holds end offsets for each term (if offsets are stored)
	endOffsets [][]int
}

// NewDocumentsWriterPerThread creates a new DWPT.
func NewDocumentsWriterPerThread(parent *DocumentsWriter) *DocumentsWriterPerThread {
	return &DocumentsWriterPerThread{
		parent:            parent,
		fieldInfosBuilder: NewFieldInfosBuilder(),
		invertedIndex:     NewInvertedIndex(),
		storedFields:      NewStoredFieldsBuffer(),
		docValues:         make(map[string]*DocValuesBuffer),
		termVectors:       NewTermVectorsBuffer(),
		lastDocID:         -1,
	}
}

// NewInvertedIndex creates a new empty inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		fields: make(map[string]*FieldPostings),
	}
}

// NewStoredFieldsBuffer creates a new empty stored fields buffer.
func NewStoredFieldsBuffer() *StoredFieldsBuffer {
	return &StoredFieldsBuffer{
		documents: make([]*StoredDocument, 0),
	}
}

// NewTermVectorsBuffer creates a new empty term vectors buffer.
func NewTermVectorsBuffer() *TermVectorsBuffer {
	return &TermVectorsBuffer{
		vectors: make([]map[string]*FieldTermVector, 0),
	}
}

// ProcessDocument processes a single document.
// This is the main entry point for indexing a document.
func (dwpt *DocumentsWriterPerThread) ProcessDocument(doc Document) error {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	// Get analyzer from parent
	analyzer := dwpt.parent.analyzer

	// Assign a document ID
	dwpt.lastDocID++
	docID := dwpt.lastDocID
	dwpt.numDocsInRAM++

	// Track field processing for this document
	storedDoc := &StoredDocument{
		fields: make([]*StoredField, 0),
	}

	// Process each field
	for _, fieldInterface := range doc.GetFields() {
		field, ok := fieldInterface.(IndexableField)
		if !ok {
			continue
		}

		fieldName := field.Name()
		ft := field.FieldType()

		// Build FieldInfoOptions from FieldTypeInterface
		opts := FieldInfoOptions{
			IndexOptions:             getIndexOptions(ft),
			StoreTermVectors:         ft.StoreTermVectors(),
			StoreTermVectorPositions: ft.StoreTermVectorPositions(),
			StoreTermVectorOffsets:   ft.StoreTermVectorOffsets(),
			OmitNorms:                false,
			DocValuesType:            ft.GetDocValuesType(),
		}

		// Get or create FieldInfo
		fieldInfo := dwpt.getOrAddFieldInfo(fieldName, opts)

		// Process based on field type
		if ft.IsIndexed() {
			// Index the field in the inverted index
			if err := dwpt.indexField(docID, field, fieldInfo, analyzer); err != nil {
				return err
			}
		}

		if ft.IsStored() {
			// Add to stored fields
			storedDoc.fields = append(storedDoc.fields, &StoredField{
				name:         fieldName,
				stringValue:  field.StringValue(),
				binaryValue:  field.BinaryValue(),
				numericValue: field.NumericValue(),
			})
		}

		if ft.GetDocValuesType() != DocValuesTypeNone {
			// Add to doc values
			dwpt.addDocValue(fieldName, docID, field, ft.GetDocValuesType())
		}

		if ft.StoreTermVectors() {
			// Build term vectors (will be populated during indexing)
			dwpt.buildTermVector(docID, fieldName, field)
		}
	}

	// Add stored document to buffer
	dwpt.storedFields.mu.Lock()
	dwpt.storedFields.documents = append(dwpt.storedFields.documents, storedDoc)
	dwpt.storedFields.totalBytes += int64(len(storedDoc.fields) * 64) // Estimate
	dwpt.storedFields.mu.Unlock()

	// Update memory usage estimate
	dwpt.bytesUsed += dwpt.estimateMemoryUsage(doc)

	return nil
}

// getIndexOptions extracts IndexOptions from FieldTypeInterface
func getIndexOptions(ft FieldTypeInterface) IndexOptions {
	if ft == nil {
		return IndexOptionsDocsAndFreqsAndPositions
	}
	return ft.GetIndexOptions()
}

// getOrAddFieldInfo gets or creates a FieldInfo for a field.
func (dwpt *DocumentsWriterPerThread) getOrAddFieldInfo(fieldName string, opts FieldInfoOptions) *FieldInfo {
	// Check if field already exists
	if fi := dwpt.fieldInfosBuilder.fieldInfos.GetByName(fieldName); fi != nil {
		return fi
	}

	// Create new FieldInfo
	fi := NewFieldInfo(fieldName, dwpt.fieldInfosBuilder.fieldInfos.Size(), opts)
	dwpt.fieldInfosBuilder.Add(fi)
	return fi
}

// indexField indexes a field in the inverted index.
func (dwpt *DocumentsWriterPerThread) indexField(docID int, field IndexableField, fieldInfo *FieldInfo, analyzer analysis.Analyzer) error {
	fieldName := field.Name()

	// Get or create field postings
	dwpt.invertedIndex.mu.Lock()
	fieldPostings, exists := dwpt.invertedIndex.fields[fieldName]
	if !exists {
		fieldPostings = &FieldPostings{
			terms:     make(map[string]*Posting),
			fieldInfo: fieldInfo,
		}
		dwpt.invertedIndex.fields[fieldName] = fieldPostings
	}
	dwpt.invertedIndex.mu.Unlock()

	// Tokenize the field value
	var tokens []string
	if field.FieldType() != nil && field.FieldType().IsTokenized() {
		// Use analyzer to tokenize
		if analyzer != nil {
			tokenStream, err := analyzer.TokenStream(fieldName, strings.NewReader(field.StringValue()))
			if err != nil {
				return err
			}
			if tokenStream != nil {
				defer tokenStream.Close()
				for {
					hasNext, err := tokenStream.IncrementToken()
					if err != nil {
						return err
					}
					if !hasNext {
						break
					}
					// Get the term attribute from the token stream
					if attrSrc, ok := tokenStream.(interface {
						GetAttributeSource() *analysis.AttributeSource
					}); ok {
						if attr := attrSrc.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
							if termAttr, ok := attr.(analysis.CharTermAttribute); ok {
								tokens = append(tokens, termAttr.String())
							}
						}
					}
				}
			}
		}
	} else {
		// Use the value directly as a single term
		tokens = []string{field.StringValue()}
	}

	// Add each token to the inverted index
	position := 0
	for _, token := range tokens {
		dwpt.addTerm(docID, fieldName, token, position, fieldPostings, fieldInfo)
		position++
	}

	return nil
}

// addTerm adds a term to the inverted index.
func (dwpt *DocumentsWriterPerThread) addTerm(docID int, fieldName, term string, position int, fieldPostings *FieldPostings, fieldInfo *FieldInfo) {
	fieldPostings.mu.Lock()
	defer fieldPostings.mu.Unlock()

	posting, exists := fieldPostings.terms[term]
	if !exists {
		posting = &Posting{
			docIDs:       make([]int, 0),
			freqs:        make([]int, 0),
			positions:    make([][]int, 0),
			startOffsets: make([][]int, 0),
			endOffsets:   make([][]int, 0),
		}
		fieldPostings.terms[term] = posting
		dwpt.invertedIndex.numTerms++
	}

	// Find or add document in posting list
	if len(posting.docIDs) > 0 && posting.docIDs[len(posting.docIDs)-1] == docID {
		// Same document, increment frequency
		idx := len(posting.docIDs) - 1
		posting.freqs[idx]++
		if fieldInfo.IndexOptions().HasPositions() {
			posting.positions[idx] = append(posting.positions[idx], position)
		}
	} else {
		// New document
		posting.docIDs = append(posting.docIDs, docID)
		posting.freqs = append(posting.freqs, 1)
		if fieldInfo.IndexOptions().HasPositions() {
			posting.positions = append(posting.positions, []int{position})
		}
	}
}

// addDocValue adds a doc value for a field.
func (dwpt *DocumentsWriterPerThread) addDocValue(fieldName string, docID int, field IndexableField, dvType DocValuesType) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	buf, exists := dwpt.docValues[fieldName]
	if !exists {
		buf = &DocValuesBuffer{
			values: make([]interface{}, 0),
			dvType: dvType,
		}
		dwpt.docValues[fieldName] = buf
	}

	buf.mu.Lock()
	buf.values = append(buf.values, field.NumericValue())
	buf.mu.Unlock()
}

// buildTermVector builds term vector data for a field.
func (dwpt *DocumentsWriterPerThread) buildTermVector(docID int, fieldName string, field IndexableField) {
	// Term vectors will be populated from the inverted index during flush
	// This is a placeholder for now
}

// estimateMemoryUsage estimates memory usage for a document.
func (dwpt *DocumentsWriterPerThread) estimateMemoryUsage(doc Document) int64 {
	// Rough estimate: 1KB per field + overhead
	fields := doc.GetFields()
	return int64(len(fields)*1024 + 256)
}

// GetNumDocs returns the number of documents in RAM.
func (dwpt *DocumentsWriterPerThread) GetNumDocs() int {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()
	return dwpt.numDocsInRAM
}

// GetBytesUsed returns the estimated memory usage.
func (dwpt *DocumentsWriterPerThread) GetBytesUsed() int64 {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()
	return dwpt.bytesUsed
}

// Reset resets the DWPT for a new segment.
func (dwpt *DocumentsWriterPerThread) Reset() {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	dwpt.numDocsInRAM = 0
	dwpt.lastDocID = -1
	dwpt.bytesUsed = 0
	dwpt.flushPending = false
	dwpt.fieldInfosBuilder = NewFieldInfosBuilder()
	dwpt.invertedIndex = NewInvertedIndex()
	dwpt.storedFields = NewStoredFieldsBuffer()
	dwpt.docValues = make(map[string]*DocValuesBuffer)
	dwpt.termVectors = NewTermVectorsBuffer()
	dwpt.pendingDeletes = nil
}

// PrepareFlush prepares this DWPT for flushing.
// Returns a FlushTicket that can be used to complete the flush.
func (dwpt *DocumentsWriterPerThread) PrepareFlush() (*FlushTicket, error) {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()

	return &FlushTicket{
		numDocs:       dwpt.numDocsInRAM,
		fieldInfos:    dwpt.fieldInfosBuilder.Build(),
		invertedIndex: dwpt.invertedIndex,
		storedFields:  dwpt.storedFields,
		docValues:     dwpt.docValues,
		termVectors:   dwpt.termVectors,
		bytesUsed:     dwpt.bytesUsed,
	}, nil
}

// FlushTicket holds the data needed to flush a segment.
type FlushTicket struct {
	numDocs       int
	fieldInfos    *FieldInfos
	invertedIndex *InvertedIndex
	storedFields  *StoredFieldsBuffer
	docValues     map[string]*DocValuesBuffer
	termVectors   *TermVectorsBuffer
	bytesUsed     int64
}

// Flush flushes the DWPT data to disk.
// Returns the new segment info.
func (dwpt *DocumentsWriterPerThread) Flush(directory store.Directory, codec Codec, segmentName string) (*SegmentInfo, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.numDocsInRAM == 0 {
		return nil, nil // Nothing to flush
	}

	// Create segment info
	segmentInfo := NewSegmentInfo(segmentName, dwpt.numDocsInRAM, directory)
	segmentInfo.SetID(generateSegmentID())

	// Build field infos
	fieldInfos := dwpt.fieldInfosBuilder.Build()

	// Create segment write state
	writeState := &SegmentWriteState{
		Directory:     directory,
		SegmentInfo:   segmentInfo,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	// 1. Write stored fields
	if err := dwpt.flushStoredFields(codec, writeState); err != nil {
		return nil, fmt.Errorf("failed to flush stored fields: %w", err)
	}

	// 2. Write postings (inverted index)
	if err := dwpt.flushPostings(codec, writeState, fieldInfos); err != nil {
		return nil, fmt.Errorf("failed to flush postings: %w", err)
	}

	// 3. Write field infos
	if err := dwpt.flushFieldInfos(codec, writeState); err != nil {
		return nil, fmt.Errorf("failed to flush field infos: %w", err)
	}

	// Update segment files list
	segmentInfo.SetFiles(dwpt.getGeneratedFiles(segmentName))

	return segmentInfo, nil
}

// flushStoredFields writes stored fields to disk.
func (dwpt *DocumentsWriterPerThread) flushStoredFields(codec Codec, state *SegmentWriteState) error {
	writer, err := codec.StoredFieldsFormat().FieldsWriter(state.Directory, state.SegmentInfo, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer writer.Close()

	for _, doc := range dwpt.storedFields.documents {
		if err := writer.StartDocument(); err != nil {
			return err
		}
		for _, field := range doc.fields {
			// Convert StoredField to IndexableField adapter
			sf := &storedFieldAdapter{field: field}
			if err := writer.WriteField(sf); err != nil {
				return err
			}
		}
		if err := writer.FinishDocument(); err != nil {
			return err
		}
	}

	return nil
}

// flushPostings writes the inverted index to disk.
func (dwpt *DocumentsWriterPerThread) flushPostings(codec Codec, state *SegmentWriteState, fieldInfos *FieldInfos) error {
	consumer, err := codec.PostingsFormat().FieldsConsumer(state)
	if err != nil {
		return err
	}
	defer consumer.Close()

	// Write each field's postings
	for fieldName, fieldPostings := range dwpt.invertedIndex.fields {
		// Convert to Terms format
		terms := &postingTermsAdapter{
			postings: fieldPostings,
		}
		if err := consumer.Write(fieldName, terms); err != nil {
			return err
		}
	}

	return nil
}

// flushFieldInfos writes field infos to disk.
func (dwpt *DocumentsWriterPerThread) flushFieldInfos(codec Codec, state *SegmentWriteState) error {
	// FieldInfosFormat write would be called here
	// For now, we'll handle it as part of segment info
	return nil
}

// getGeneratedFiles returns the list of files generated during flush.
func (dwpt *DocumentsWriterPerThread) getGeneratedFiles(segmentName string) []string {
	// Return the list of segment files
	// This would include: .fdt, .fdx, .tim, .tip, .doc, .pos, etc.
	files := []string{
		segmentName + ".fdt", // Stored fields data
		segmentName + ".fdx", // Stored fields index
		segmentName + ".tim", // Term dictionary
		segmentName + ".tip", // Term index
		segmentName + ".doc", // Doc values
		segmentName + ".pos", // Positions
	}
	return files
}

// storedFieldAdapter adapts StoredField to IndexableField interface.
type storedFieldAdapter struct {
	field *StoredField
}

func (s *storedFieldAdapter) Name() string              { return s.field.name }
func (s *storedFieldAdapter) StringValue() string       { return s.field.stringValue }
func (s *storedFieldAdapter) BinaryValue() []byte       { return s.field.binaryValue }
func (s *storedFieldAdapter) NumericValue() interface{} { return s.field.numericValue }
func (s *storedFieldAdapter) FieldType() FieldTypeInterface {
	return &simpleFieldType{}
}
func (s *storedFieldAdapter) ReaderValue() io.Reader { return nil }

// simpleFieldType provides a simple FieldTypeInterface implementation
type simpleFieldType struct{}

func (f *simpleFieldType) IsIndexed() bool               { return false }
func (f *simpleFieldType) IsStored() bool                { return true }
func (f *simpleFieldType) IsTokenized() bool             { return false }
func (f *simpleFieldType) GetIndexOptions() IndexOptions { return IndexOptionsNone }
func (f *simpleFieldType) GetDocValuesType() DocValuesType {
	return DocValuesTypeNone
}
func (f *simpleFieldType) StoreTermVectors() bool         { return false }
func (f *simpleFieldType) StoreTermVectorPositions() bool { return false }
func (f *simpleFieldType) StoreTermVectorOffsets() bool   { return false }

// postingTermsAdapter adapts FieldPostings to Terms interface.
type postingTermsAdapter struct {
	postings *FieldPostings
}

func (p *postingTermsAdapter) GetIterator() (TermsEnum, error) {
	return &postingTermsEnum{postings: p.postings, terms: getSortedTerms(p.postings)}, nil
}

func (p *postingTermsAdapter) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	terms := getSortedTerms(p.postings)
	// Find position at or after seek term
	for i, t := range terms {
		if t >= seekTerm.Text() {
			return &postingTermsEnum{postings: p.postings, terms: terms, index: i}, nil
		}
	}
	return &postingTermsEnum{postings: p.postings, terms: terms, index: len(terms)}, nil
}

func (p *postingTermsAdapter) Size() int64 {
	return int64(len(p.postings.terms))
}

func (p *postingTermsAdapter) GetDocCount() (int, error) {
	maxDoc := 0
	for _, posting := range p.postings.terms {
		for _, docID := range posting.docIDs {
			if docID > maxDoc {
				maxDoc = docID
			}
		}
	}
	return maxDoc + 1, nil
}

func (p *postingTermsAdapter) GetSumDocFreq() (int64, error) {
	var sum int64
	for _, posting := range p.postings.terms {
		sum += int64(len(posting.docIDs))
	}
	return sum, nil
}

func (p *postingTermsAdapter) GetSumTotalTermFreq() (int64, error) {
	var sum int64
	for _, posting := range p.postings.terms {
		for _, freq := range posting.freqs {
			sum += int64(freq)
		}
	}
	return sum, nil
}

func (p *postingTermsAdapter) HasFreqs() bool   { return true }
func (p *postingTermsAdapter) HasOffsets() bool { return false }
func (p *postingTermsAdapter) HasPositions() bool {
	return p.postings.fieldInfo != nil && p.postings.fieldInfo.IndexOptions().HasPositions()
}
func (p *postingTermsAdapter) HasPayloads() bool      { return false }
func (p *postingTermsAdapter) GetMin() (*Term, error) { return nil, nil }
func (p *postingTermsAdapter) GetMax() (*Term, error) { return nil, nil }

// GetPostingsReader returns the postings for a term.
func (p *postingTermsAdapter) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	posting, ok := p.postings.terms[termText]
	if !ok {
		return nil, nil
	}
	return NewSingleDocPostingsEnum(posting.docIDs[0], posting.freqs[0]), nil
}

// getSortedTerms returns sorted term strings from postings
func getSortedTerms(postings *FieldPostings) []string {
	postings.mu.RLock()
	defer postings.mu.RUnlock()
	terms := make([]string, 0, len(postings.terms))
	for t := range postings.terms {
		terms = append(terms, t)
	}
	// Sort terms
	for i := 0; i < len(terms)-1; i++ {
		for j := i + 1; j < len(terms); j++ {
			if terms[i] > terms[j] {
				terms[i], terms[j] = terms[j], terms[i]
			}
		}
	}
	return terms
}

// postingTermsEnum iterates over terms in postings
type postingTermsEnum struct {
	postings *FieldPostings
	terms    []string
	index    int
}

func (e *postingTermsEnum) Next() (*Term, error) {
	if e.index >= len(e.terms) {
		return nil, nil
	}
	term := NewTerm(e.postings.fieldInfo.Name(), e.terms[e.index])
	e.index++
	return term, nil
}

func (e *postingTermsEnum) DocFreq() (int, error) {
	if e.index <= 0 || e.index > len(e.terms) {
		return 0, nil
	}
	term := e.terms[e.index-1]
	posting, ok := e.postings.terms[term]
	if !ok {
		return 0, nil
	}
	return len(posting.docIDs), nil
}

func (e *postingTermsEnum) TotalTermFreq() (int64, error) {
	if e.index <= 0 || e.index > len(e.terms) {
		return 0, nil
	}
	term := e.terms[e.index-1]
	posting, ok := e.postings.terms[term]
	if !ok {
		return 0, nil
	}
	var sum int64
	for _, freq := range posting.freqs {
		sum += int64(freq)
	}
	return sum, nil
}

func (e *postingTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return nil, nil
}

func (e *postingTermsEnum) SeekExact(term *Term) (bool, error) {
	for i, t := range e.terms {
		if t == term.Text() {
			e.index = i + 1
			return true, nil
		}
	}
	return false, nil
}

func (e *postingTermsEnum) SeekCeil(term *Term) (*Term, error) {
	for i, t := range e.terms {
		if t >= term.Text() {
			e.index = i + 1
			return NewTerm(e.postings.fieldInfo.Name(), t), nil
		}
	}
	e.index = len(e.terms)
	return nil, nil
}

func (e *postingTermsEnum) Term() *Term {
	if e.index <= 0 || e.index > len(e.terms) {
		return nil
	}
	return NewTerm(e.postings.fieldInfo.Name(), e.terms[e.index-1])
}

func (e *postingTermsEnum) Close() error { return nil }

func (e *postingTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return nil, nil
}

// generateSegmentID generates a unique segment ID.
func generateSegmentID() []byte {
	id := make([]byte, 16)
	// Use timestamp and random data for ID
	now := time.Now().UnixNano()
	for i := 0; i < 8; i++ {
		id[i] = byte(now >> (i * 8))
	}
	// Fill remaining with pseudo-random data
	for i := 8; i < 16; i++ {
		id[i] = byte(i*17 + int(now&0xFF))
	}
	return id
}
