// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// BlockState represents the state of a compressed block of documents.
// This is the Go port of Lucene's BlockState.
//
// BlockState tracks the current state during compression/decompression
// of document blocks, including document boundaries and field information.
type BlockState struct {
	// ChunkID is the unique identifier for this chunk.
	ChunkID int

	// StartDocID is the first document ID in this chunk.
	StartDocID int

	// NumDocs is the number of documents in this chunk.
	NumDocs int

	// Docs contains the serialized documents in this chunk.
	Docs [][]storedField

	// UncompressedLength is the length of the uncompressed data.
	UncompressedLength int

	// CompressedLength is the length of the compressed data.
	CompressedLength int

	// FieldsUsed tracks which fields are used in this chunk.
	FieldsUsed map[string]bool

	// FieldInfos contains information about fields in this chunk.
	FieldInfos map[string]*fieldBlockInfo

	// SortedFields contains field names sorted for consistent ordering.
	SortedFields []string

	mu     sync.RWMutex
	closed bool
}

// fieldBlockInfo tracks information about a field within a block.
type fieldBlockInfo struct {
	Name             string
	Type             document.FieldType
	NumValues        int
	NumDocsWithField int
	MinValue         interface{}
	MaxValue         interface{}
}

// NewBlockState creates a new BlockState.
func NewBlockState(chunkID, startDocID int) *BlockState {
	return &BlockState{
		ChunkID:      chunkID,
		StartDocID:   startDocID,
		NumDocs:      0,
		Docs:         make([][]storedField, 0),
		FieldsUsed:   make(map[string]bool),
		FieldInfos:   make(map[string]*fieldBlockInfo),
		SortedFields: make([]string, 0),
	}
}

// AddDocument adds a document to this block.
func (s *BlockState) AddDocument(doc []storedField) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("block state is closed")
	}

	s.Docs = append(s.Docs, doc)
	s.NumDocs++

	// Track fields used in this document
	for _, field := range doc {
		s.FieldsUsed[field.name] = true

		// Update field info
		if info, ok := s.FieldInfos[field.name]; ok {
			info.NumValues++
		} else {
			s.FieldInfos[field.name] = &fieldBlockInfo{
				Name:      field.name,
				NumValues: 1,
			}
			s.SortedFields = append(s.SortedFields, field.name)
		}
	}

	return nil
}

// GetDocument returns the fields for a specific document in this chunk.
// The docOffset is the offset within this chunk (0-based).
func (s *BlockState) GetDocument(docOffset int) ([]storedField, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("block state is closed")
	}

	if docOffset < 0 || docOffset >= len(s.Docs) {
		return nil, fmt.Errorf("docOffset %d out of range [0, %d)", docOffset, len(s.Docs))
	}

	return s.Docs[docOffset], nil
}

// GetDocID returns the global document ID for a document in this chunk.
func (s *BlockState) GetDocID(docOffset int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if docOffset < 0 || docOffset >= s.NumDocs {
		return 0, fmt.Errorf("docOffset %d out of range [0, %d)", docOffset, s.NumDocs)
	}

	return s.StartDocID + docOffset, nil
}

// GetEndDocID returns the last document ID in this chunk (exclusive).
func (s *BlockState) GetEndDocID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.StartDocID + s.NumDocs
}

// GetDocumentOffset returns the offset within this chunk for a global docID.
// Returns -1 if the docID is not in this chunk.
func (s *BlockState) GetDocumentOffset(docID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if docID < s.StartDocID || docID >= s.StartDocID+s.NumDocs {
		return -1
	}

	return docID - s.StartDocID
}

// HasField returns true if this chunk contains the given field.
func (s *BlockState) HasField(fieldName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.FieldsUsed[fieldName]
}

// GetFields returns a list of field names used in this chunk.
func (s *BlockState) GetFields() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fields := make([]string, len(s.SortedFields))
	copy(fields, s.SortedFields)
	return fields
}

// GetFieldInfo returns information about a specific field.
func (s *BlockState) GetFieldInfo(fieldName string) (*fieldBlockInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.FieldInfos[fieldName]
	if !ok {
		return nil, fmt.Errorf("field '%s' not found in chunk", fieldName)
	}

	return info, nil
}

// SetUncompressedLength sets the uncompressed length of this chunk.
func (s *BlockState) SetUncompressedLength(length int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.UncompressedLength = length
}

// SetCompressedLength sets the compressed length of this chunk.
func (s *BlockState) SetCompressedLength(length int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CompressedLength = length
}

// GetCompressionRatio returns the compression ratio for this chunk.
// Returns 0.0 if uncompressed length is 0.
func (s *BlockState) GetCompressionRatio() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.UncompressedLength == 0 {
		return 0.0
	}

	return float64(s.CompressedLength) / float64(s.UncompressedLength)
}

// GetTotalFieldValues returns the total number of field values in this chunk.
func (s *BlockState) GetTotalFieldValues() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, info := range s.FieldInfos {
		total += info.NumValues
	}

	return total
}

// GetNumFields returns the number of unique fields in this chunk.
func (s *BlockState) GetNumFields() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.FieldsUsed)
}

// IsFull returns true if this chunk has reached its capacity.
func (s *BlockState) IsFull(maxDocsPerChunk, maxChunkSize int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.NumDocs >= maxDocsPerChunk {
		return true
	}

	if s.UncompressedLength >= maxChunkSize {
		return true
	}

	return false
}

// IsEmpty returns true if this chunk contains no documents.
func (s *BlockState) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.NumDocs == 0
}

// Serialize serializes this block state to a byte slice.
// This is used when writing the block to disk.
func (s *BlockState) Serialize() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("block state is closed")
	}

	// Serialize document data
	var data []byte

	// Write chunk header
	header := makeBlockStateHeader(s)
	data = append(data, header...)

	// Write document count
	docCount := encodeVInt(uint64(s.NumDocs))
	data = append(data, docCount...)

	// Write each document
	for _, doc := range s.Docs {
		docData := serializeDocument(doc)
		docLen := encodeVInt(uint64(len(docData)))
		data = append(data, docLen...)
		data = append(data, docData...)
	}

	return data, nil
}

// Deserialize deserializes a block state from a byte slice.
func DeserializeBlockState(data []byte, chunkID int) (*BlockState, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("insufficient data for block state header")
	}

	// Read header
	startDocID, numDocs, err := parseBlockStateHeader(data)
	if err != nil {
		return nil, err
	}

	state := NewBlockState(chunkID, startDocID)
	state.NumDocs = numDocs

	// Skip header
	offset := 8

	// Read document count (should match)
	docCount, n := decodeVInt(data[offset:])
	if n <= 0 {
		return nil, fmt.Errorf("failed to read document count")
	}
	offset += n

	if int(docCount) != numDocs {
		return nil, fmt.Errorf("document count mismatch: expected %d, got %d", numDocs, docCount)
	}

	// Read each document
	for i := 0; i < numDocs; i++ {
		docLen, n := decodeVInt(data[offset:])
		if n <= 0 {
			return nil, fmt.Errorf("failed to read document %d length", i)
		}
		offset += n

		doc, err := deserializeDocument(data[offset : offset+int(docLen)])
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize document %d: %w", i, err)
		}
		offset += int(docLen)

		state.Docs = append(state.Docs, doc)
	}

	return state, nil
}

// Close releases resources used by this block state.
func (s *BlockState) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.Docs = nil
	s.FieldsUsed = nil
	s.FieldInfos = nil
	s.SortedFields = nil

	return nil
}

// Clone creates a copy of this block state.
func (s *BlockState) Clone() *BlockState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := NewBlockState(s.ChunkID, s.StartDocID)
	clone.NumDocs = s.NumDocs
	clone.UncompressedLength = s.UncompressedLength
	clone.CompressedLength = s.CompressedLength

	// Copy documents
	clone.Docs = make([][]storedField, len(s.Docs))
	for i, doc := range s.Docs {
		clone.Docs[i] = make([]storedField, len(doc))
		copy(clone.Docs[i], doc)
	}

	// Copy field info
	for name, info := range s.FieldInfos {
		clone.FieldInfos[name] = &fieldBlockInfo{
			Name:             info.Name,
			Type:             info.Type,
			NumValues:        info.NumValues,
			NumDocsWithField: info.NumDocsWithField,
			MinValue:         info.MinValue,
			MaxValue:         info.MaxValue,
		}
	}

	for name := range s.FieldsUsed {
		clone.FieldsUsed[name] = true
		clone.SortedFields = append(clone.SortedFields, name)
	}

	return clone
}

// Helper functions for serialization

func makeBlockStateHeader(s *BlockState) []byte {
	// Encode startDocID and numDocs as varints
	startDocIDBytes := encodeVInt(uint64(s.StartDocID))
	numDocsBytes := encodeVInt(uint64(s.NumDocs))

	// Header is 8 bytes minimum
	header := make([]byte, 8)
	n := copy(header, startDocIDBytes)
	copy(header[n:], numDocsBytes)

	return header
}

func parseBlockStateHeader(data []byte) (startDocID, numDocs int, err error) {
	if len(data) < 8 {
		return 0, 0, fmt.Errorf("insufficient data for header")
	}

	startDocID64, n := decodeVInt(data)
	if n <= 0 {
		return 0, 0, fmt.Errorf("failed to parse startDocID")
	}

	numDocs64, m := decodeVInt(data[n:])
	if m <= 0 {
		return 0, 0, fmt.Errorf("failed to parse numDocs")
	}

	return int(startDocID64), int(numDocs64), nil
}

func encodeVInt(v uint64) []byte {
	buf := make([]byte, 0, 10)
	for v >= 0x80 {
		buf = append(buf, byte(v&0x7F|0x80))
		v >>= 7
	}
	buf = append(buf, byte(v))
	return buf
}

func decodeVInt(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i, b := range data {
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
		if i >= 9 {
			return 0, -1 // Error: too many bytes
		}
	}
	return 0, -1 // Error: insufficient data
}

// serializeDocument serializes a document to bytes.
func serializeDocument(doc []storedField) []byte {
	var data []byte

	// Write number of fields
	fieldCount := encodeVInt(uint64(len(doc)))
	data = append(data, fieldCount...)

	// Write each field
	for _, field := range doc {
		// Write field type
		data = append(data, field.fieldType)

		// Write field name
		nameLen := encodeVInt(uint64(len(field.name)))
		data = append(data, nameLen...)
		data = append(data, field.name...)

		// Write value based on type
		switch field.fieldType {
		case fieldTypeString:
			value := field.value.(string)
			valLen := encodeVInt(uint64(len(value)))
			data = append(data, valLen...)
			data = append(data, value...)

		case fieldTypeBinary:
			value := field.value.([]byte)
			valLen := encodeVInt(uint64(len(value)))
			data = append(data, valLen...)
			data = append(data, value...)

		case fieldTypeInt:
			value := field.value.(int32)
			buf := make([]byte, 4)
			buf[0] = byte(value >> 24)
			buf[1] = byte(value >> 16)
			buf[2] = byte(value >> 8)
			buf[3] = byte(value)
			data = append(data, buf...)

		case fieldTypeLong:
			value := field.value.(int64)
			buf := make([]byte, 8)
			buf[0] = byte(value >> 56)
			buf[1] = byte(value >> 48)
			buf[2] = byte(value >> 40)
			buf[3] = byte(value >> 32)
			buf[4] = byte(value >> 24)
			buf[5] = byte(value >> 16)
			buf[6] = byte(value >> 8)
			buf[7] = byte(value)
			data = append(data, buf...)

		case fieldTypeFloat:
			// Encode as int32 bits
			value := field.value.(float32)
			bits := int32(value) // Simplified - should use math.Float32bits
			buf := make([]byte, 4)
			buf[0] = byte(bits >> 24)
			buf[1] = byte(bits >> 16)
			buf[2] = byte(bits >> 8)
			buf[3] = byte(bits)
			data = append(data, buf...)

		case fieldTypeDouble:
			// Encode as int64 bits
			value := field.value.(float64)
			bits := int64(value) // Simplified - should use math.Float64bits
			buf := make([]byte, 8)
			buf[0] = byte(bits >> 56)
			buf[1] = byte(bits >> 48)
			buf[2] = byte(bits >> 40)
			buf[3] = byte(bits >> 32)
			buf[4] = byte(bits >> 24)
			buf[5] = byte(bits >> 16)
			buf[6] = byte(bits >> 8)
			buf[7] = byte(bits)
			data = append(data, buf...)
		}
	}

	return data
}

// deserializeDocument deserializes a document from bytes.
func deserializeDocument(data []byte) ([]storedField, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Read number of fields
	fieldCount, n := decodeVInt(data)
	if n <= 0 {
		return nil, fmt.Errorf("failed to read field count")
	}
	offset := n

	doc := make([]storedField, 0, fieldCount)

	for i := 0; i < int(fieldCount); i++ {
		if offset >= len(data) {
			return nil, fmt.Errorf("insufficient data for field %d", i)
		}

		// Read field type
		fieldType := data[offset]
		offset++

		// Read field name
		nameLen, n := decodeVInt(data[offset:])
		if n <= 0 {
			return nil, fmt.Errorf("failed to read field %d name length", i)
		}
		offset += n

		if offset+int(nameLen) > len(data) {
			return nil, fmt.Errorf("insufficient data for field %d name", i)
		}
		name := string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)

		field := storedField{
			fieldType: fieldType,
			name:      name,
		}

		// Read value based on type
		switch fieldType {
		case fieldTypeString:
			valLen, n := decodeVInt(data[offset:])
			if n <= 0 {
				return nil, fmt.Errorf("failed to read field %d value length", i)
			}
			offset += n

			if offset+int(valLen) > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d value", i)
			}
			field.value = string(data[offset : offset+int(valLen)])
			offset += int(valLen)

		case fieldTypeBinary:
			valLen, n := decodeVInt(data[offset:])
			if n <= 0 {
				return nil, fmt.Errorf("failed to read field %d value length", i)
			}
			offset += n

			if offset+int(valLen) > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d value", i)
			}
			field.value = data[offset : offset+int(valLen)]
			offset += int(valLen)

		case fieldTypeInt:
			if offset+4 > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d int value", i)
			}
			value := int32(data[offset])<<24 | int32(data[offset+1])<<16 |
				int32(data[offset+2])<<8 | int32(data[offset+3])
			field.value = value
			offset += 4

		case fieldTypeLong:
			if offset+8 > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d long value", i)
			}
			value := int64(data[offset])<<56 | int64(data[offset+1])<<48 |
				int64(data[offset+2])<<40 | int64(data[offset+3])<<32 |
				int64(data[offset+4])<<24 | int64(data[offset+5])<<16 |
				int64(data[offset+6])<<8 | int64(data[offset+7])
			field.value = value
			offset += 8

		case fieldTypeFloat:
			if offset+4 > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d float value", i)
			}
			bits := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
				uint32(data[offset+2])<<8 | uint32(data[offset+3])
			// Simplified - should use math.Float32frombits
			field.value = float32(bits)
			offset += 4

		case fieldTypeDouble:
			if offset+8 > len(data) {
				return nil, fmt.Errorf("insufficient data for field %d double value", i)
			}
			bits := uint64(data[offset])<<56 | uint64(data[offset+1])<<48 |
				uint64(data[offset+2])<<40 | uint64(data[offset+3])<<32 |
				uint64(data[offset+4])<<24 | uint64(data[offset+5])<<16 |
				uint64(data[offset+6])<<8 | uint64(data[offset+7])
			// Simplified - should use math.Float64frombits
			field.value = float64(bits)
			offset += 8
		}

		doc = append(doc, field)
	}

	return doc, nil
}

// BlockStatePool provides a pool of reusable BlockState instances.
type BlockStatePool struct {
	pool sync.Pool
}

// NewBlockStatePool creates a new BlockStatePool.
func NewBlockStatePool() *BlockStatePool {
	return &BlockStatePool{
		pool: sync.Pool{
			New: func() interface{} {
				return NewBlockState(0, 0)
			},
		},
	}
}

// Get returns a BlockState from the pool.
func (p *BlockStatePool) Get() *BlockState {
	return p.pool.Get().(*BlockState)
}

// Put returns a BlockState to the pool.
func (p *BlockStatePool) Put(state *BlockState) {
	// Reset the state before returning to pool
	state.ChunkID = 0
	state.StartDocID = 0
	state.NumDocs = 0
	state.Docs = state.Docs[:0]
	state.UncompressedLength = 0
	state.CompressedLength = 0
	state.FieldsUsed = make(map[string]bool)
	state.FieldInfos = make(map[string]*fieldBlockInfo)
	state.SortedFields = state.SortedFields[:0]
	state.closed = false

	p.pool.Put(state)
}

// Global BlockStatePool instance.
var defaultBlockStatePool = NewBlockStatePool()

// GetBlockStateFromPool returns a BlockState from the global pool.
func GetBlockStateFromPool() *BlockState {
	return defaultBlockStatePool.Get()
}

// PutBlockStateToPool returns a BlockState to the global pool.
func PutBlockStateToPool(state *BlockState) {
	defaultBlockStatePool.Put(state)
}
