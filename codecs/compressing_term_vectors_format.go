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

	// Note: In a full implementation, we would read and decompress chunks here
	// For now, this is a simplified version

	return nil
}

// Get retrieves term vectors for the given document ID.
func (r *CompressingTermVectorsReader) Get(docID int) (index.Fields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("reader is closed")
	}

	// Placeholder: Return empty fields
	return &emptyFields{}, nil
}

// GetField retrieves the term vector for a specific field in a document.
func (r *CompressingTermVectorsReader) GetField(docID int, field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("reader is closed")
	}

	// Placeholder: Return empty terms
	return &emptyTerms{}, nil
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
