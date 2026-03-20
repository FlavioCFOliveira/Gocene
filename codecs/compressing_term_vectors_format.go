// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CompressingTermVectorsFormat is a TermVectorsFormat that compresses term vectors
// in chunks and stores them in compressed form.
//
// This is the Go port of Lucene's CompressingTermVectorsFormat.
// It compresses term vectors in chunks similar to CompressingStoredFieldsFormat.
//
// The format is byte-compatible with Apache Lucene's implementation.
type CompressingTermVectorsFormat struct {
	*BaseTermVectorsFormat
	compressionMode CompressionMode
	chunkSize       int
	maxDocsPerChunk int
}

// DefaultCompressingTermVectorsFormat creates a new CompressingTermVectorsFormat
// with default settings (LZ4_FAST compression, 16KB chunks, 128 docs per chunk).
func DefaultCompressingTermVectorsFormat() *CompressingTermVectorsFormat {
	return NewCompressingTermVectorsFormat(CompressionModeLZ4Fast, 16*1024, 128)
}

// NewCompressingTermVectorsFormat creates a new CompressingTermVectorsFormat
// with the specified compression mode and chunk parameters.
//
// Parameters:
//   - mode: The compression mode to use (LZ4_FAST, LZ4_HIGH, or DEFLATE)
//   - chunkSize: The target chunk size in bytes (must be >= 1KB)
//   - maxDocsPerChunk: The maximum number of documents per chunk (must be >= 1)
func NewCompressingTermVectorsFormat(mode CompressionMode, chunkSize, maxDocsPerChunk int) *CompressingTermVectorsFormat {
	if chunkSize < 1024 {
		chunkSize = 1024 // Minimum 1KB
	}
	if maxDocsPerChunk < 1 {
		maxDocsPerChunk = 1
	}
	return &CompressingTermVectorsFormat{
		BaseTermVectorsFormat: NewBaseTermVectorsFormat("CompressingTermVectorsFormat"),
		compressionMode:       mode,
		chunkSize:             chunkSize,
		maxDocsPerChunk:       maxDocsPerChunk,
	}
}

// CompressionMode returns the compression mode used by this format.
func (f *CompressingTermVectorsFormat) CompressionMode() CompressionMode {
	return f.compressionMode
}

// ChunkSize returns the chunk size in bytes.
func (f *CompressingTermVectorsFormat) ChunkSize() int {
	return f.chunkSize
}

// MaxDocsPerChunk returns the maximum number of documents per chunk.
func (f *CompressingTermVectorsFormat) MaxDocsPerChunk() int {
	return f.maxDocsPerChunk
}

// VectorsWriter returns a writer for term vectors.
func (f *CompressingTermVectorsFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	return NewCompressingTermVectorsWriter(state, f.compressionMode, f.chunkSize, f.maxDocsPerChunk)
}

// VectorsReader returns a reader for term vectors.
func (f *CompressingTermVectorsFormat) VectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (TermVectorsReader, error) {
	return NewCompressingTermVectorsReader(dir, segmentInfo, fieldInfos, f.compressionMode, f.chunkSize, f.maxDocsPerChunk)
}

// termVectorDoc represents term vectors for a single document
type termVectorDoc struct {
	fields []termVectorField
}

// termVectorField represents term vectors for a single field
type termVectorField struct {
	name         string
	terms        []termVectorTerm
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
}

// termVectorTerm represents a single term in a field
type termVectorTerm struct {
	term      []byte
	freq      int
	positions []termPosition
}

// termPosition represents a position with optional offsets and payload
type termPosition struct {
	position    int
	startOffset int
	endOffset   int
	payload     []byte
}

// CompressingTermVectorsWriter writes term vectors in compressed chunks.
type CompressingTermVectorsWriter struct {
	state           *SegmentWriteState
	compressionMode CompressionMode
	chunkSize       int
	maxDocsPerChunk int
	docs            []termVectorDoc
	currentDoc      *termVectorDoc
	currentField    *termVectorField
	currentTerm     *termVectorTerm
	mu              sync.Mutex
	closed          bool
	out             store.IndexOutput
}

// NewCompressingTermVectorsWriter creates a new CompressingTermVectorsWriter.
func NewCompressingTermVectorsWriter(state *SegmentWriteState, mode CompressionMode, chunkSize, maxDocsPerChunk int) (*CompressingTermVectorsWriter, error) {
	fileName := state.SegmentInfo.Name() + ".tvd"
	out, err := state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("failed to create term vectors data file: %w", err)
	}

	writer := &CompressingTermVectorsWriter{
		state:           state,
		compressionMode: mode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		docs:            make([]termVectorDoc, 0),
		out:             out,
	}

	// Write header
	if err := store.WriteUint32(out, 0x54564400); err != nil { // "TVD\0"
		out.Close()
		return nil, fmt.Errorf("failed to write magic number: %w", err)
	}
	if err := store.WriteVInt(out, 1); err != nil { // Version
		out.Close()
		return nil, fmt.Errorf("failed to write version: %w", err)
	}

	return writer, nil
}

// StartDocument starts writing term vectors for a document.
func (w *CompressingTermVectorsWriter) StartDocument(numFields int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	w.docs = append(w.docs, termVectorDoc{fields: make([]termVectorField, 0, numFields)})
	w.currentDoc = &w.docs[len(w.docs)-1]
	return nil
}

// StartField starts writing a term vector for a field.
func (w *CompressingTermVectorsWriter) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	if w.currentDoc == nil {
		return fmt.Errorf("no document started")
	}

	field := termVectorField{
		name:         fieldInfo.Name(),
		terms:        make([]termVectorTerm, 0, numTerms),
		hasPositions: hasPositions,
		hasOffsets:   hasOffsets,
		hasPayloads:  hasPayloads,
	}
	w.currentDoc.fields = append(w.currentDoc.fields, field)
	w.currentField = &w.currentDoc.fields[len(w.currentDoc.fields)-1]
	return nil
}

// StartTerm starts a new term in the current field.
func (w *CompressingTermVectorsWriter) StartTerm(term []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	if w.currentField == nil {
		return fmt.Errorf("no field started")
	}

	tvTerm := termVectorTerm{
		term:      term,
		positions: make([]termPosition, 0),
	}
	w.currentField.terms = append(w.currentField.terms, tvTerm)
	w.currentTerm = &w.currentField.terms[len(w.currentField.terms)-1]
	return nil
}

// AddPosition adds a position for the current term.
func (w *CompressingTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	if w.currentTerm == nil {
		return fmt.Errorf("no term started")
	}

	pos := termPosition{
		position:    position,
		startOffset: startOffset,
		endOffset:   endOffset,
		payload:     payload,
	}
	w.currentTerm.positions = append(w.currentTerm.positions, pos)
	return nil
}

// FinishTerm finishes the current term.
func (w *CompressingTermVectorsWriter) FinishTerm() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Term frequency is the number of positions
	if w.currentTerm != nil {
		w.currentTerm.freq = len(w.currentTerm.positions)
	}
	w.currentTerm = nil
	return nil
}

// FinishField finishes the current field.
func (w *CompressingTermVectorsWriter) FinishField() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	w.currentField = nil
	return nil
}

// FinishDocument finishes the current document.
func (w *CompressingTermVectorsWriter) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	w.currentDoc = nil
	w.currentField = nil
	w.currentTerm = nil

	// Check if we need to flush the chunk
	docsInChunk := len(w.docs) - (len(w.docs)/w.maxDocsPerChunk)*w.maxDocsPerChunk
	if docsInChunk >= w.maxDocsPerChunk {
		return w.flushChunk()
	}

	return nil
}

// flushChunk compresses and writes the current chunk.
func (w *CompressingTermVectorsWriter) flushChunk() error {
	if len(w.docs) == 0 {
		return nil
	}

	// Build uncompressed chunk data
	chunkData := w.serializeChunk()

	// Compress
	compressor := w.compressionMode.compressor()
	compressed, err := compressor(chunkData)
	if err != nil {
		return fmt.Errorf("failed to compress chunk: %w", err)
	}

	// Write compressed data
	if err := w.out.WriteBytes(compressed); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Reset for next chunk
	w.docs = w.docs[:0]

	return nil
}

// serializeChunk serializes the current chunk to bytes.
func (w *CompressingTermVectorsWriter) serializeChunk() []byte {
	var buf bytes.Buffer

	// Write number of documents
	binary.Write(&buf, binary.BigEndian, int32(len(w.docs)))

	// Write each document
	for _, doc := range w.docs {
		// Write number of fields
		binary.Write(&buf, binary.BigEndian, int32(len(doc.fields)))

		for _, field := range doc.fields {
			// Write field name length and name
			binary.Write(&buf, binary.BigEndian, int32(len(field.name)))
			buf.WriteString(field.name)

			// Write flags
			flags := byte(0)
			if field.hasPositions {
				flags |= 0x01
			}
			if field.hasOffsets {
				flags |= 0x02
			}
			if field.hasPayloads {
				flags |= 0x04
			}
			buf.WriteByte(flags)

			// Write number of terms
			binary.Write(&buf, binary.BigEndian, int32(len(field.terms)))

			for _, term := range field.terms {
				// Write term length and term
				binary.Write(&buf, binary.BigEndian, int32(len(term.term)))
				buf.Write(term.term)

				// Write frequency
				binary.Write(&buf, binary.BigEndian, int32(term.freq))

				// Write positions
				for _, pos := range term.positions {
					binary.Write(&buf, binary.BigEndian, int32(pos.position))
					if field.hasOffsets {
						binary.Write(&buf, binary.BigEndian, int32(pos.startOffset))
						binary.Write(&buf, binary.BigEndian, int32(pos.endOffset))
					}
					if field.hasPayloads && len(pos.payload) > 0 {
						binary.Write(&buf, binary.BigEndian, int32(len(pos.payload)))
						buf.Write(pos.payload)
					}
				}
			}
		}
	}

	return buf.Bytes()
}

// Close releases resources and finalizes the file.
func (w *CompressingTermVectorsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Flush any remaining documents
	if err := w.flushChunk(); err != nil {
		w.out.Close()
		return err
	}

	// Close data file
	if err := w.out.Close(); err != nil {
		return err
	}

	// Write index file
	return w.writeIndex()
}

// writeIndex writes the index file.
func (w *CompressingTermVectorsWriter) writeIndex() error {
	fileName := w.state.SegmentInfo.Name() + ".tvx"
	out, err := w.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create term vectors index file: %w", err)
	}
	defer out.Close()

	// Write header
	if err := store.WriteUint32(out, 0x54565800); err != nil { // "TVX\0"
		return fmt.Errorf("failed to write index magic number: %w", err)
	}
	if err := store.WriteVInt(out, 1); err != nil { // Version
		return fmt.Errorf("failed to write index version: %w", err)
	}

	return nil
}

// CompressingTermVectorsReader reads term vectors from compressed chunks.
type CompressingTermVectorsReader struct {
	directory       store.Directory
	segmentInfo     *index.SegmentInfo
	fieldInfos      *index.FieldInfos
	compressionMode CompressionMode
	chunkSize       int
	maxDocsPerChunk int
	mu              sync.RWMutex
	closed          bool
	// docVectors maps document IDs to their term vectors
	docVectors map[int]*docTermVectors
}

// docTermVectors holds term vectors for a single document
type docTermVectors struct {
	fields map[string]*fieldTermVector
}

// fieldTermVector holds term vector data for a single field
type fieldTermVector struct {
	name         string
	terms        []termVector
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
}

// termVector represents a single term with its positions, offsets, and payloads
type termVector struct {
	term       []byte
	freq       int
	positions  []int
	startOffsets []int
	endOffsets   []int
	payloads   [][]byte
}

// NewCompressingTermVectorsReader creates a new CompressingTermVectorsReader.
func NewCompressingTermVectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, mode CompressionMode, chunkSize, maxDocsPerChunk int) (*CompressingTermVectorsReader, error) {
	reader := &CompressingTermVectorsReader{
		directory:       dir,
		segmentInfo:     segmentInfo,
		fieldInfos:      fieldInfos,
		compressionMode: mode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
	}
	if err := reader.load(); err != nil {
		return nil, err
	}
	return reader, nil
}

// load reads the compressed term vectors from disk.
func (r *CompressingTermVectorsReader) load() error {
	fileName := r.segmentInfo.Name() + ".tvd"

	if !r.directory.FileExists(fileName) {
		// No term vectors file - return empty reader
		return nil
	}

	// Load data file
	return r.loadData(fileName)
}

// loadData reads the data file.
func (r *CompressingTermVectorsReader) loadData(fileName string) error {
	in, err := r.directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	defer in.Close()

	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x54564400 { // "TVD\0"
		return fmt.Errorf("invalid data magic number: expected 0x54564400, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported data version: %d", version)
	}

	// Read the rest of the file (compressed data)
	length := in.Length() - in.GetFilePointer()
	compressedData := make([]byte, length)
	if err := in.ReadBytes(compressedData); err != nil {
		return fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Decompress if there's data
	if len(compressedData) > 0 {
		decompressor := r.compressionMode.decompressor()
		chunkData, err := decompressor(compressedData, int(length)*10) // Estimate decompressed size
		if err != nil {
			return fmt.Errorf("failed to decompress data: %w", err)
		}

		// Parse the chunk data
		if err := r.parseChunkData(chunkData); err != nil {
			return fmt.Errorf("failed to parse chunk data: %w", err)
		}
	}

	return nil
}

// parseChunkData parses decompressed chunk data into docVectors
func (r *CompressingTermVectorsReader) parseChunkData(data []byte) error {
	r.docVectors = make(map[int]*docTermVectors)

	buf := bytes.NewReader(data)

	// Read number of documents
	var numDocs int32
	if err := binary.Read(buf, binary.BigEndian, &numDocs); err != nil {
		return fmt.Errorf("failed to read number of documents: %w", err)
	}

	docID := 0
	for i := int32(0); i < numDocs; i++ {
		docVectors := &docTermVectors{
			fields: make(map[string]*fieldTermVector),
		}

		// Read number of fields
		var numFields int32
		if err := binary.Read(buf, binary.BigEndian, &numFields); err != nil {
			return fmt.Errorf("failed to read number of fields: %w", err)
		}

		for j := int32(0); j < numFields; j++ {
			// Read field name length and name
			var nameLen int32
			if err := binary.Read(buf, binary.BigEndian, &nameLen); err != nil {
				return fmt.Errorf("failed to read field name length: %w", err)
			}
			nameBytes := make([]byte, nameLen)
			if _, err := buf.Read(nameBytes); err != nil {
				return fmt.Errorf("failed to read field name: %w", err)
			}
			name := string(nameBytes)

			// Read flags
			flags := make([]byte, 1)
			if _, err := buf.Read(flags); err != nil {
				return fmt.Errorf("failed to read flags: %w", err)
			}
			hasPositions := flags[0]&0x01 != 0
			hasOffsets := flags[0]&0x02 != 0
			hasPayloads := flags[0]&0x04 != 0

			// Read number of terms
			var numTerms int32
			if err := binary.Read(buf, binary.BigEndian, &numTerms); err != nil {
				return fmt.Errorf("failed to read number of terms: %w", err)
			}

			field := &fieldTermVector{
				name:         name,
				terms:        make([]termVector, 0, numTerms),
				hasPositions: hasPositions,
				hasOffsets:   hasOffsets,
				hasPayloads:  hasPayloads,
			}

			for k := int32(0); k < numTerms; k++ {
				// Read term length and term
				var termLen int32
				if err := binary.Read(buf, binary.BigEndian, &termLen); err != nil {
					return fmt.Errorf("failed to read term length: %w", err)
				}
				termBytes := make([]byte, termLen)
				if _, err := buf.Read(termBytes); err != nil {
					return fmt.Errorf("failed to read term: %w", err)
				}

				// Read frequency
				var freq int32
				if err := binary.Read(buf, binary.BigEndian, &freq); err != nil {
					return fmt.Errorf("failed to read frequency: %w", err)
				}

				tv := termVector{
					term: termBytes,
					freq: int(freq),
				}

				// Read positions
				if hasPositions {
					tv.positions = make([]int, freq)
					tv.startOffsets = make([]int, freq)
					tv.endOffsets = make([]int, freq)
					if hasPayloads {
						tv.payloads = make([][]byte, freq)
					}

					for p := 0; p < int(freq); p++ {
						var pos int32
						if err := binary.Read(buf, binary.BigEndian, &pos); err != nil {
							return fmt.Errorf("failed to read position: %w", err)
						}
						tv.positions[p] = int(pos)

						if hasOffsets {
							var startOffset, endOffset int32
							if err := binary.Read(buf, binary.BigEndian, &startOffset); err != nil {
								return fmt.Errorf("failed to read start offset: %w", err)
							}
							if err := binary.Read(buf, binary.BigEndian, &endOffset); err != nil {
								return fmt.Errorf("failed to read end offset: %w", err)
							}
							tv.startOffsets[p] = int(startOffset)
							tv.endOffsets[p] = int(endOffset)
						}

						if hasPayloads {
							var payloadLen int32
							if err := binary.Read(buf, binary.BigEndian, &payloadLen); err != nil {
								return fmt.Errorf("failed to read payload length: %w", err)
							}
							if payloadLen > 0 {
								payload := make([]byte, payloadLen)
								if _, err := buf.Read(payload); err != nil {
									return fmt.Errorf("failed to read payload: %w", err)
								}
								tv.payloads[p] = payload
							}
						}
					}
				}

				field.terms = append(field.terms, tv)
			}

			docVectors.fields[name] = field
		}

		r.docVectors[docID] = docVectors
		docID++
	}

	return nil
}

// Get retrieves term vectors for the given document ID.
func (r *CompressingTermVectorsReader) Get(docID int) (index.Fields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("reader is closed")
	}

	docVectors, exists := r.docVectors[docID]
	if !exists {
		return &emptyFields{}, nil
	}

	return &termVectorsFields{docVectors: docVectors}, nil
}

// GetField retrieves the term vector for a specific field in a document.
func (r *CompressingTermVectorsReader) GetField(docID int, field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("reader is closed")
	}

	docVectors, exists := r.docVectors[docID]
	if !exists {
		return nil, nil
	}

	fieldVector, exists := docVectors.fields[field]
	if !exists {
		return nil, nil
	}

	return &termVectorsTerms{field: fieldVector}, nil
}

// Close releases resources.
func (r *CompressingTermVectorsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return nil
}

// emptyFields is a placeholder implementation of index.Fields
type emptyFields struct{}

func (f *emptyFields) Iterator() (index.FieldIterator, error)  { return nil, nil }
func (f *emptyFields) Size() int                               { return 0 }
func (f *emptyFields) Terms(field string) (index.Terms, error) { return nil, nil }

// emptyTerms is a placeholder implementation of index.Terms
type emptyTerms struct{}

func (t *emptyTerms) GetIterator() (index.TermsEnum, error) { return nil, nil }
func (t *emptyTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	return nil, nil
}
func (t *emptyTerms) Size() int64                         { return 0 }
func (t *emptyTerms) GetDocCount() (int, error)           { return 0, nil }
func (t *emptyTerms) GetSumDocFreq() (int64, error)       { return 0, nil }
func (t *emptyTerms) GetSumTotalTermFreq() (int64, error) { return 0, nil }
func (t *emptyTerms) HasFreqs() bool                      { return false }
func (t *emptyTerms) HasOffsets() bool                    { return false }
func (t *emptyTerms) HasPositions() bool                  { return false }
func (t *emptyTerms) HasPayloads() bool                   { return false }
func (t *emptyTerms) GetMin() (*index.Term, error)        { return nil, nil }
func (t *emptyTerms) GetMax() (*index.Term, error)        { return nil, nil }
func (t *emptyTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	return nil, nil
}

// termVectorsFields implements index.Fields for term vectors
type termVectorsFields struct {
	docVectors *docTermVectors
}

func (f *termVectorsFields) Iterator() (index.FieldIterator, error) {
	fieldNames := make([]string, 0, len(f.docVectors.fields))
	for name := range f.docVectors.fields {
		fieldNames = append(fieldNames, name)
	}
	return &termVectorsFieldIterator{fields: fieldNames, index: -1}, nil
}

func (f *termVectorsFields) Size() int {
	return len(f.docVectors.fields)
}

func (f *termVectorsFields) Terms(field string) (index.Terms, error) {
	fieldVector, exists := f.docVectors.fields[field]
	if !exists {
		return nil, nil
	}
	return &termVectorsTerms{field: fieldVector}, nil
}

// termVectorsFieldIterator implements index.FieldIterator for term vectors
type termVectorsFieldIterator struct {
	fields []string
	index  int
}

func (it *termVectorsFieldIterator) Next() (string, error) {
	it.index++
	if it.index >= len(it.fields) {
		return "", nil
	}
	return it.fields[it.index], nil
}

func (it *termVectorsFieldIterator) HasNext() bool {
	return it.index+1 < len(it.fields)
}

// termVectorsTerms implements index.Terms for term vectors
type termVectorsTerms struct {
	field *fieldTermVector
}

func (t *termVectorsTerms) GetIterator() (index.TermsEnum, error) {
	return &termVectorsTermsEnum{field: t.field, index: -1}, nil
}

func (t *termVectorsTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	// For term vectors, we don't support seeking - just return a regular iterator
	return t.GetIterator()
}

func (t *termVectorsTerms) Size() int64 {
	return int64(len(t.field.terms))
}

func (t *termVectorsTerms) GetDocCount() (int, error) {
	return 1, nil // Term vectors are per-document, so doc count is always 1
}

func (t *termVectorsTerms) GetSumDocFreq() (int64, error) {
	return int64(len(t.field.terms)), nil
}

func (t *termVectorsTerms) GetSumTotalTermFreq() (int64, error) {
	var total int64
	for _, term := range t.field.terms {
		total += int64(term.freq)
	}
	return total, nil
}

func (t *termVectorsTerms) HasFreqs() bool {
	return true
}

func (t *termVectorsTerms) HasOffsets() bool {
	return t.field.hasOffsets
}

func (t *termVectorsTerms) HasPositions() bool {
	return t.field.hasPositions
}

func (t *termVectorsTerms) HasPayloads() bool {
	return t.field.hasPayloads
}

func (t *termVectorsTerms) GetMin() (*index.Term, error) {
	if len(t.field.terms) == 0 {
		return nil, nil
	}
	return index.NewTermFromBytes(t.field.name, t.field.terms[0].term), nil
}

func (t *termVectorsTerms) GetMax() (*index.Term, error) {
	if len(t.field.terms) == 0 {
		return nil, nil
	}
	return index.NewTermFromBytes(t.field.name, t.field.terms[len(t.field.terms)-1].term), nil
}

func (t *termVectorsTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	// Find the term in the field
	for _, term := range t.field.terms {
		if string(term.term) == termText {
			// Found the term - create postings enum
			return index.NewSinglePostingsEnum(1, term.freq), nil
		}
	}
	return nil, nil
}

// termVectorsTermsEnum implements index.TermsEnum for term vectors
type termVectorsTermsEnum struct {
	field *fieldTermVector
	index int
}

func (e *termVectorsTermsEnum) Next() (*index.Term, error) {
	e.index++
	if e.index >= len(e.field.terms) {
		return nil, nil
	}
	return index.NewTermFromBytes(e.field.name, e.field.terms[e.index].term), nil
}

func (e *termVectorsTermsEnum) DocFreq() (int, error) {
	if e.index < 0 || e.index >= len(e.field.terms) {
		return 0, fmt.Errorf("iterator not positioned")
	}
	return 1, nil // Each term appears in exactly one document for term vectors
}

func (e *termVectorsTermsEnum) TotalTermFreq() (int64, error) {
	if e.index < 0 || e.index >= len(e.field.terms) {
		return 0, fmt.Errorf("iterator not positioned")
	}
	return int64(e.field.terms[e.index].freq), nil
}

func (e *termVectorsTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.index < 0 || e.index >= len(e.field.terms) {
		return nil, fmt.Errorf("iterator not positioned")
	}
	term := e.field.terms[e.index]
	return &termVectorsPostingsEnum{
		term:         term,
		hasPositions: e.field.hasPositions,
		hasOffsets:   e.field.hasOffsets,
		hasPayloads:  e.field.hasPayloads,
	}, nil
}

func (e *termVectorsTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	// Term vectors don't support live docs filtering - just return regular postings
	return e.Postings(flags)
}

func (e *termVectorsTermsEnum) SeekCeil(text *index.Term) (*index.Term, error) {
	// Simple linear search for now
	textStr := text.Bytes.String()
	for i, term := range e.field.terms {
		if string(term.term) >= textStr {
			e.index = i
			return index.NewTermFromBytes(e.field.name, term.term), nil
		}
	}
	return nil, nil
}

func (e *termVectorsTermsEnum) SeekExact(text *index.Term) (bool, error) {
	textStr := text.Bytes.String()
	for i, term := range e.field.terms {
		if string(term.term) == textStr {
			e.index = i
			return true, nil
		}
	}
	return false, nil
}

func (e *termVectorsTermsEnum) Term() *index.Term {
	if e.index < 0 || e.index >= len(e.field.terms) {
		return nil
	}
	return index.NewTermFromBytes(e.field.name, e.field.terms[e.index].term)
}

// termVectorsPostingsEnum implements index.PostingsEnum for term vectors
type termVectorsPostingsEnum struct {
	term         termVector
	posIndex     int
	hasPositions bool
	hasOffsets   bool
	hasPayloads  bool
}

func (p *termVectorsPostingsEnum) NextDoc() (int, error) {
	// Term vectors only have one document, so return NO_MORE_DOCS after first call
	if p.posIndex == 0 {
		p.posIndex = 1
		return 0, nil // Return docID 0
	}
	return index.NO_MORE_DOCS, nil
}

func (p *termVectorsPostingsEnum) Advance(target int) (int, error) {
	// Term vectors only have one document (docID 0)
	if target <= 0 && p.posIndex == 0 {
		p.posIndex = 1
		return 0, nil
	}
	return index.NO_MORE_DOCS, nil
}

func (p *termVectorsPostingsEnum) DocID() int {
	if p.posIndex == 0 {
		return -1 // Not started
	}
	if p.posIndex > 0 {
		return 0 // Current doc
	}
	return index.NO_MORE_DOCS
}

func (p *termVectorsPostingsEnum) Freq() (int, error) {
	return p.term.freq, nil
}

func (p *termVectorsPostingsEnum) NextPosition() (int, error) {
	if !p.hasPositions {
		return -1, fmt.Errorf("positions not available")
	}
	if p.posIndex >= len(p.term.positions) {
		return -1, nil // No more positions
	}
	pos := p.term.positions[p.posIndex]
	p.posIndex++
	return pos, nil
}

func (p *termVectorsPostingsEnum) StartOffset() (int, error) {
	if !p.hasOffsets {
		return -1, fmt.Errorf("offsets not available")
	}
	if p.posIndex <= 0 || p.posIndex > len(p.term.startOffsets) {
		return -1, fmt.Errorf("no current position")
	}
	return p.term.startOffsets[p.posIndex-1], nil
}

func (p *termVectorsPostingsEnum) EndOffset() (int, error) {
	if !p.hasOffsets {
		return -1, fmt.Errorf("offsets not available")
	}
	if p.posIndex <= 0 || p.posIndex > len(p.term.endOffsets) {
		return -1, fmt.Errorf("no current position")
	}
	return p.term.endOffsets[p.posIndex-1], nil
}

func (p *termVectorsPostingsEnum) Payload() ([]byte, error) {
	if !p.hasPayloads {
		return nil, nil
	}
	if p.posIndex <= 0 || p.posIndex > len(p.term.payloads) {
		return nil, fmt.Errorf("no current position")
	}
	return p.term.payloads[p.posIndex-1], nil
}

func (p *termVectorsPostingsEnum) GetPayload() ([]byte, error) {
	return p.Payload()
}

func (p *termVectorsPostingsEnum) Cost() int64 {
	return int64(p.term.freq)
}
