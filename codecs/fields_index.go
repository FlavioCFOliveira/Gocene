// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// FieldsIndex provides an index for accessing fields data in compressed segments.
// This is the Go port of Lucene's FieldsIndex.
//
// The index maps document IDs to chunk offsets, allowing for efficient
// random access to documents stored in compressed blocks.
type FieldsIndex interface {
	// GetStartPointer returns the file pointer for the chunk containing the given docID.
	GetStartPointer(docID int) (int64, error)

	// GetNumChunks returns the number of chunks in the index.
	GetNumChunks() int

	// GetDocsPerChunk returns the average number of documents per chunk.
	GetDocsPerChunk() int

	// Close releases resources used by this index.
	Close() error
}

// ChunkInfo stores information about a single chunk in the index.
type ChunkInfo struct {
	// StartDocID is the first document ID in this chunk.
	StartDocID int

	// NumDocs is the number of documents in this chunk.
	NumDocs int

	// StartPointer is the file offset where this chunk begins.
	StartPointer int64

	// CompressedLength is the length of the compressed data.
	CompressedLength int

	// UncompressedLength is the length of the uncompressed data.
	UncompressedLength int
}

// FieldsIndexImpl is the standard implementation of FieldsIndex.
// This is the Go port of Lucene's FieldsIndexImpl.
type FieldsIndexImpl struct {
	chunks       []ChunkInfo
	docToChunk   []int // Maps docID -> chunk index
	numChunks    int
	docsPerChunk int
	minDocID     int
	maxDocID     int
	mu           sync.RWMutex
	closed       bool
}

// NewFieldsIndexImpl creates a new FieldsIndexImpl from chunk information.
func NewFieldsIndexImpl(chunks []ChunkInfo) (*FieldsIndexImpl, error) {
	if len(chunks) == 0 {
		return &FieldsIndexImpl{
			chunks:       make([]ChunkInfo, 0),
			docToChunk:   make([]int, 0),
			numChunks:    0,
			docsPerChunk: 0,
		}, nil
	}

	// Calculate total number of documents
	var totalDocs int
	var minDocID, maxDocID int

	for i, chunk := range chunks {
		if i == 0 {
			minDocID = chunk.StartDocID
		}
		if i == len(chunks)-1 {
			maxDocID = chunk.StartDocID + chunk.NumDocs - 1
		}
		totalDocs += chunk.NumDocs
	}

	// Build docToChunk mapping
	docToChunk := make([]int, totalDocs)
	for chunkIdx, chunk := range chunks {
		for docIdx := 0; docIdx < chunk.NumDocs; docIdx++ {
			docID := chunk.StartDocID + docIdx
			if docID < totalDocs {
				docToChunk[docID] = chunkIdx
			}
		}
	}

	// Calculate average docs per chunk
	docsPerChunk := totalDocs / len(chunks)
	if totalDocs%len(chunks) > 0 {
		docsPerChunk++
	}

	return &FieldsIndexImpl{
		chunks:       chunks,
		docToChunk:   docToChunk,
		numChunks:    len(chunks),
		docsPerChunk: docsPerChunk,
		minDocID:     minDocID,
		maxDocID:     maxDocID,
	}, nil
}

// GetStartPointer returns the file pointer for the chunk containing the given docID.
func (idx *FieldsIndexImpl) GetStartPointer(docID int) (int64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return 0, fmt.Errorf("index is closed")
	}

	if docID < 0 || docID >= len(idx.docToChunk) {
		return 0, fmt.Errorf("docID %d out of range [0, %d)", docID, len(idx.docToChunk))
	}

	chunkIdx := idx.docToChunk[docID]
	if chunkIdx < 0 || chunkIdx >= len(idx.chunks) {
		return 0, fmt.Errorf("invalid chunk index %d", chunkIdx)
	}

	return idx.chunks[chunkIdx].StartPointer, nil
}

// GetChunkInfo returns the ChunkInfo for the chunk containing the given docID.
func (idx *FieldsIndexImpl) GetChunkInfo(docID int) (ChunkInfo, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return ChunkInfo{}, fmt.Errorf("index is closed")
	}

	if docID < 0 || docID >= len(idx.docToChunk) {
		return ChunkInfo{}, fmt.Errorf("docID %d out of range [0, %d)", docID, len(idx.docToChunk))
	}

	chunkIdx := idx.docToChunk[docID]
	if chunkIdx < 0 || chunkIdx >= len(idx.chunks) {
		return ChunkInfo{}, fmt.Errorf("invalid chunk index %d", chunkIdx)
	}

	return idx.chunks[chunkIdx], nil
}

// GetNumChunks returns the number of chunks in the index.
func (idx *FieldsIndexImpl) GetNumChunks() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.numChunks
}

// GetDocsPerChunk returns the average number of documents per chunk.
func (idx *FieldsIndexImpl) GetDocsPerChunk() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.docsPerChunk
}

// GetChunkStartDocID returns the starting document ID for a given chunk.
func (idx *FieldsIndexImpl) GetChunkStartDocID(chunkIdx int) (int, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return 0, fmt.Errorf("index is closed")
	}

	if chunkIdx < 0 || chunkIdx >= len(idx.chunks) {
		return 0, fmt.Errorf("chunk index %d out of range [0, %d)", chunkIdx, len(idx.chunks))
	}

	return idx.chunks[chunkIdx].StartDocID, nil
}

// GetChunkNumDocs returns the number of documents in a given chunk.
func (idx *FieldsIndexImpl) GetChunkNumDocs(chunkIdx int) (int, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return 0, fmt.Errorf("index is closed")
	}

	if chunkIdx < 0 || chunkIdx >= len(idx.chunks) {
		return 0, fmt.Errorf("chunk index %d out of range [0, %d)", chunkIdx, len(idx.chunks))
	}

	return idx.chunks[chunkIdx].NumDocs, nil
}

// GetTotalDocs returns the total number of documents indexed.
func (idx *FieldsIndexImpl) GetTotalDocs() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.docToChunk)
}

// Close releases resources used by this index.
func (idx *FieldsIndexImpl) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return nil
	}

	idx.closed = true
	idx.chunks = nil
	idx.docToChunk = nil
	idx.numChunks = 0
	idx.docsPerChunk = 0

	return nil
}

// FieldsIndexReader reads FieldsIndex from storage.
type FieldsIndexReader struct {
	input store.IndexInput
}

// NewFieldsIndexReader creates a new FieldsIndexReader.
func NewFieldsIndexReader(input store.IndexInput) *FieldsIndexReader {
	return &FieldsIndexReader{
		input: input,
	}
}

// Read reads the entire index from the input.
func (r *FieldsIndexReader) Read() (FieldsIndex, error) {
	// Read number of chunks
	numChunks, err := store.ReadVInt(r.input)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk count: %w", err)
	}

	if numChunks == 0 {
		return NewFieldsIndexImpl(nil)
	}

	chunks := make([]ChunkInfo, numChunks)
	for i := 0; i < int(numChunks); i++ {
		startDocID, err := store.ReadVInt(r.input)
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk start docID: %w", err)
		}

		numDocs, err := store.ReadVInt(r.input)
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk num docs: %w", err)
		}

		startPointer, err := r.input.ReadLong()
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk start pointer: %w", err)
		}

		compressedLen, err := store.ReadVInt(r.input)
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk compressed length: %w", err)
		}

		uncompressedLen, err := store.ReadVInt(r.input)
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk uncompressed length: %w", err)
		}

		chunks[i] = ChunkInfo{
			StartDocID:         int(startDocID),
			NumDocs:            int(numDocs),
			StartPointer:       startPointer,
			CompressedLength:   int(compressedLen),
			UncompressedLength: int(uncompressedLen),
		}
	}

	return NewFieldsIndexImpl(chunks)
}

// FieldsIndexWriter writes FieldsIndex to storage.
type FieldsIndexWriter struct {
	output store.IndexOutput
}

// NewFieldsIndexWriter creates a new FieldsIndexWriter.
func NewFieldsIndexWriter(output store.IndexOutput) *FieldsIndexWriter {
	return &FieldsIndexWriter{
		output: output,
	}
}

// Write writes the entire index to the output.
func (w *FieldsIndexWriter) Write(index FieldsIndex) error {
	impl, ok := index.(*FieldsIndexImpl)
	if !ok {
		return fmt.Errorf("index must be a FieldsIndexImpl")
	}

	numChunks := impl.GetNumChunks()

	// Write number of chunks
	if err := store.WriteVInt(w.output, int32(numChunks)); err != nil {
		return fmt.Errorf("failed to write chunk count: %w", err)
	}

	if numChunks == 0 {
		return nil
	}

	impl.mu.RLock()
	defer impl.mu.RUnlock()

	// Write each chunk
	for _, chunk := range impl.chunks {
		if err := store.WriteVInt(w.output, int32(chunk.StartDocID)); err != nil {
			return fmt.Errorf("failed to write chunk start docID: %w", err)
		}

		if err := store.WriteVInt(w.output, int32(chunk.NumDocs)); err != nil {
			return fmt.Errorf("failed to write chunk num docs: %w", err)
		}

		if err := w.output.WriteLong(chunk.StartPointer); err != nil {
			return fmt.Errorf("failed to write chunk start pointer: %w", err)
		}

		if err := store.WriteVInt(w.output, int32(chunk.CompressedLength)); err != nil {
			return fmt.Errorf("failed to write chunk compressed length: %w", err)
		}

		if err := store.WriteVInt(w.output, int32(chunk.UncompressedLength)); err != nil {
			return fmt.Errorf("failed to write chunk uncompressed length: %w", err)
		}
	}

	return nil
}

// CompressingStoredFieldsIndex provides index information for compressed stored fields.
// This is a specialized FieldsIndex for the CompressingStoredFieldsFormat.
type CompressingStoredFieldsIndex struct {
	*FieldsIndexImpl
	segmentName string
}

// NewCompressingStoredFieldsIndex creates a new CompressingStoredFieldsIndex.
func NewCompressingStoredFieldsIndex(segmentName string, chunks []ChunkInfo) (*CompressingStoredFieldsIndex, error) {
	impl, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		return nil, err
	}

	return &CompressingStoredFieldsIndex{
		FieldsIndexImpl: impl,
		segmentName:     segmentName,
	}, nil
}

// GetSegmentName returns the segment name for this index.
func (idx *CompressingStoredFieldsIndex) GetSegmentName() string {
	return idx.segmentName
}

// GetIndexFileName returns the name of the index file.
func (idx *CompressingStoredFieldsIndex) GetIndexFileName() string {
	return idx.segmentName + ".fdx"
}

// GetDataFileName returns the name of the data file.
func (idx *CompressingStoredFieldsIndex) GetDataFileName() string {
	return idx.segmentName + ".fdt"
}
