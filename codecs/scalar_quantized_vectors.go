// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/Lucene104ScalarQuantizedVectorsFormat.java
// Purpose: Scalar quantization for vector storage - compresses float vectors to byte/quantized representations

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ScalarEncoding represents the encoding type for scalar quantized vectors.
// This is the Go equivalent of Lucene's ScalarEncoding enum.
type ScalarEncoding int

const (
	// ScalarEncodingUnsignedByte uses 8-bit unsigned byte quantization.
	ScalarEncodingUnsignedByte ScalarEncoding = iota
	// ScalarEncodingSevenBit uses 7-bit quantization.
	ScalarEncodingSevenBit
	// ScalarEncodingPackedNibble packs two 4-bit values into one byte.
	ScalarEncodingPackedNibble
	// ScalarEncodingSingleBitQueryNibble uses single bit quantization for queries.
	ScalarEncodingSingleBitQueryNibble
	// ScalarEncodingDibitQueryNibble uses 2-bit quantization for queries.
	ScalarEncodingDibitQueryNibble
)

// String returns the string representation of the ScalarEncoding.
func (se ScalarEncoding) String() string {
	switch se {
	case ScalarEncodingUnsignedByte:
		return "UNSIGNED_BYTE"
	case ScalarEncodingSevenBit:
		return "SEVEN_BIT"
	case ScalarEncodingPackedNibble:
		return "PACKED_NIBBLE"
	case ScalarEncodingSingleBitQueryNibble:
		return "SINGLE_BIT_QUERY_NIBBLE"
	case ScalarEncodingDibitQueryNibble:
		return "DIBIT_QUERY_NIBBLE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", se)
	}
}

// GetBits returns the number of bits used by this encoding.
func (se ScalarEncoding) GetBits() int {
	switch se {
	case ScalarEncodingUnsignedByte:
		return 8
	case ScalarEncodingSevenBit:
		return 7
	case ScalarEncodingPackedNibble, ScalarEncodingSingleBitQueryNibble, ScalarEncodingDibitQueryNibble:
		return 4
	default:
		return 8
	}
}

// GetDiscreteDimensions returns the number of discrete dimensions for the given encoding.
func (se ScalarEncoding) GetDiscreteDimensions(dims int) int {
	switch se {
	case ScalarEncodingPackedNibble, ScalarEncodingSingleBitQueryNibble, ScalarEncodingDibitQueryNibble:
		return dims
	default:
		return dims
	}
}

// GetDocPackedLength returns the packed length for a document with the given discrete dimensions.
func (se ScalarEncoding) GetDocPackedLength(discreteDims int) int {
	switch se {
	case ScalarEncodingPackedNibble:
		return (discreteDims + 1) / 2
	case ScalarEncodingSingleBitQueryNibble, ScalarEncodingDibitQueryNibble:
		return (discreteDims + 7) / 8
	default:
		return discreteDims
	}
}

// ScalarEncodingValues returns all scalar encoding values.
func ScalarEncodingValues() []ScalarEncoding {
	return []ScalarEncoding{
		ScalarEncodingUnsignedByte,
		ScalarEncodingSevenBit,
		ScalarEncodingPackedNibble,
		ScalarEncodingSingleBitQueryNibble,
		ScalarEncodingDibitQueryNibble,
	}
}

// Lucene104ScalarQuantizedVectorsFormat constants
const (
	// VERSION_START is the initial version
	Lucene104ScalarQuantizedVectorsFormat_VERSION_START = 0
	// VERSION_CURRENT is the current version
	Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT = Lucene104ScalarQuantizedVectorsFormat_VERSION_START
	// DIRECT_MONOTONIC_BLOCK_SHIFT is the block shift for direct monotonic storage
	Lucene104ScalarQuantizedVectorsFormat_DIRECT_MONOTONIC_BLOCK_SHIFT = 16
	// MAX_DIMENSIONS is the maximum number of dimensions supported
	Lucene104ScalarQuantizedVectorsFormat_MAX_DIMENSIONS = 1024
	// DEFAULT_QUANTIZED_VECTOR_COMPONENT is the default component name
	Lucene104ScalarQuantizedVectorsFormat_DEFAULT_QUANTIZED_VECTOR_COMPONENT = "QVEC"
)

// File extensions for Lucene104ScalarQuantizedVectorsFormat
const (
	// Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION is the vector data file extension
	Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION = "veq"
	// Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION is the vector metadata file extension
	Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION = "vemq"
	// Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_EXTENSION is the raw vector file extension
	Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_EXTENSION = "vec"
	// Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_FILE_EXTENSION is the vector meta file extension
	Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_FILE_EXTENSION = "vemf"
)

// Lucene104ScalarQuantizedVectorsFormat implements scalar quantization for vector storage.
// This format compresses float vectors to quantized byte representations for efficient storage
// and fast approximate similarity computation.
//
// This is the Go port of Lucene's Lucene104ScalarQuantizedVectorsFormat.
type Lucene104ScalarQuantizedVectorsFormat struct {
	*BaseKnnVectorsFormat
	encoding ScalarEncoding
}

// NewLucene104ScalarQuantizedVectorsFormat creates a new Lucene104ScalarQuantizedVectorsFormat
// with the default encoding (UNSIGNED_BYTE).
func NewLucene104ScalarQuantizedVectorsFormat() *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat("Lucene104ScalarQuantizedVectorsFormat"),
		encoding:             ScalarEncodingUnsignedByte,
	}
}

// NewLucene104ScalarQuantizedVectorsFormatWithEncoding creates a new format with the specified encoding.
func NewLucene104ScalarQuantizedVectorsFormatWithEncoding(encoding ScalarEncoding) *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat("Lucene104ScalarQuantizedVectorsFormat"),
		encoding:             encoding,
	}
}

// Encoding returns the scalar encoding used by this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) Encoding() ScalarEncoding {
	return f.encoding
}

// String returns a string representation of this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorsFormat(name=Lucene104ScalarQuantizedVectorsFormat, encoding=%s)",
		f.encoding.String())
}

// FieldsWriter returns a writer for writing quantized vectors.
func (f *Lucene104ScalarQuantizedVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return NewLucene104ScalarQuantizedVectorsWriter(state, f.encoding)
}

// FieldsReader returns a reader for reading quantized vectors.
func (f *Lucene104ScalarQuantizedVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return NewLucene104ScalarQuantizedVectorsReader(state, f.encoding)
}

// SupportsFloatVectorFallback returns true as this format supports float vector fallback.
func (f *Lucene104ScalarQuantizedVectorsFormat) SupportsFloatVectorFallback() bool {
	return true
}

// Lucene104ScalarQuantizedVectorsWriter writes scalar quantized vectors.
type Lucene104ScalarQuantizedVectorsWriter struct {
	encoding      ScalarEncoding
	segmentState  *SegmentWriteState
	vectorDataOut store.IndexOutput
	metaOut       store.IndexOutput
	closed        bool
	fieldWriters  map[string]*quantizedFieldWriter
}

// NewLucene104ScalarQuantizedVectorsWriter creates a new writer.
func NewLucene104ScalarQuantizedVectorsWriter(state *SegmentWriteState, encoding ScalarEncoding) (*Lucene104ScalarQuantizedVectorsWriter, error) {
	// Create vector data output
	vectorDataFile := fmt.Sprintf("%s_%s_%s.%s",
		state.SegmentInfo.Name(),
		Lucene104ScalarQuantizedVectorsFormat_DEFAULT_QUANTIZED_VECTOR_COMPONENT,
		"0",
		Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION)

	vectorDataOut, err := state.Directory.CreateOutput(vectorDataFile, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector data output: %w", err)
	}

	// Create metadata output
	metaFile := fmt.Sprintf("%s_%s_%s.%s",
		state.SegmentInfo.Name(),
		Lucene104ScalarQuantizedVectorsFormat_DEFAULT_QUANTIZED_VECTOR_COMPONENT,
		"0",
		Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION)

	metaOut, err := state.Directory.CreateOutput(metaFile, store.IOContextWrite)
	if err != nil {
		vectorDataOut.Close()
		return nil, fmt.Errorf("failed to create meta output: %w", err)
	}

	// Write headers
	helper := NewKnnVectorsWriterHelper(vectorDataOut)
	if err := helper.WriteHeader(); err != nil {
		vectorDataOut.Close()
		metaOut.Close()
		return nil, err
	}

	// Write meta header
	if err := writeScalarQuantizedMetaHeader(metaOut); err != nil {
		vectorDataOut.Close()
		metaOut.Close()
		return nil, err
	}

	return &Lucene104ScalarQuantizedVectorsWriter{
		encoding:      encoding,
		segmentState:    state,
		vectorDataOut: vectorDataOut,
		metaOut:       metaOut,
		fieldWriters:  make(map[string]*quantizedFieldWriter),
	}, nil
}

// writeScalarQuantizedMetaHeader writes the metadata file header.
func writeScalarQuantizedMetaHeader(out store.IndexOutput) error {
	// Write magic number for quantized vectors
	if err := store.WriteUint32(out, 0x51564543); err != nil { // "QVEC"
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, uint32(Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT)); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// WriteField writes a quantized vector field.
func (w *Lucene104ScalarQuantizedVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader KnnVectorsReader) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	fieldWriter := &quantizedFieldWriter{
		fieldInfo: fieldInfo,
		encoding:  w.encoding,
		vectors:   make([][]float32, 0),
	}

	w.fieldWriters[fieldInfo.Name()] = fieldWriter
	return nil
}

// Finish finalizes the writing process.
func (w *Lucene104ScalarQuantizedVectorsWriter) Finish() error {
	if w.closed {
		return nil
	}

	// Write field metadata
	for _, fw := range w.fieldWriters {
		if err := w.writeFieldMetadata(fw); err != nil {
			return err
		}
	}

	return nil
}

// writeFieldMetadata writes metadata for a field.
func (w *Lucene104ScalarQuantizedVectorsWriter) writeFieldMetadata(fw *quantizedFieldWriter) error {
	// Write field number
	if err := store.WriteUint32(w.metaOut, uint32(fw.fieldInfo.Number())); err != nil {
		return fmt.Errorf("failed to write field number: %w", err)
	}

	// Write encoding type
	if err := store.WriteUint32(w.metaOut, uint32(fw.encoding)); err != nil {
		return fmt.Errorf("failed to write encoding: %w", err)
	}

	// Write number of vectors
	if err := store.WriteUint32(w.metaOut, uint32(len(fw.vectors))); err != nil {
		return fmt.Errorf("failed to write vector count: %w", err)
	}

	// Write dimensions
	if len(fw.vectors) > 0 {
		if err := store.WriteUint32(w.metaOut, uint32(len(fw.vectors[0]))); err != nil {
			return fmt.Errorf("failed to write dimensions: %w", err)
		}
	}

	return nil
}

// Close releases resources.
func (w *Lucene104ScalarQuantizedVectorsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error
	if err := w.vectorDataOut.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := w.metaOut.Close(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// quantizedFieldWriter handles writing for a single field
type quantizedFieldWriter struct {
	fieldInfo *index.FieldInfo
	encoding  ScalarEncoding
	vectors   [][]float32
}

// Lucene104ScalarQuantizedVectorsReader reads scalar quantized vectors.
type Lucene104ScalarQuantizedVectorsReader struct {
	encoding     ScalarEncoding
	segmentState *SegmentReadState
	vectorDataIn store.IndexInput
	metaIn       store.IndexInput
	closed       bool
	fields       map[string]*quantizedFieldReader
}

// NewLucene104ScalarQuantizedVectorsReader creates a new reader.
func NewLucene104ScalarQuantizedVectorsReader(state *SegmentReadState, encoding ScalarEncoding) (*Lucene104ScalarQuantizedVectorsReader, error) {
	// Open vector data input
	vectorDataFile := fmt.Sprintf("%s_%s_%s.%s",
		state.SegmentInfo.Name(),
		Lucene104ScalarQuantizedVectorsFormat_DEFAULT_QUANTIZED_VECTOR_COMPONENT,
		"0",
		Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION)

	vectorDataIn, err := state.Directory.OpenInput(vectorDataFile, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("failed to open vector data input: %w", err)
	}

	// Open metadata input
	metaFile := fmt.Sprintf("%s_%s_%s.%s",
		state.SegmentInfo.Name(),
		Lucene104ScalarQuantizedVectorsFormat_DEFAULT_QUANTIZED_VECTOR_COMPONENT,
		"0",
		Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION)

	metaIn, err := state.Directory.OpenInput(metaFile, store.IOContextRead)
	if err != nil {
		vectorDataIn.Close()
		return nil, fmt.Errorf("failed to open meta input: %w", err)
	}

	// Read and validate headers
	helper := NewKnnVectorsReaderHelper(vectorDataIn)
	if err := helper.ReadHeader(); err != nil {
		vectorDataIn.Close()
		metaIn.Close()
		return nil, err
	}

	// Read meta header
	if err := readScalarQuantizedMetaHeader(metaIn); err != nil {
		vectorDataIn.Close()
		metaIn.Close()
		return nil, err
	}

	reader := &Lucene104ScalarQuantizedVectorsReader{
		encoding:     encoding,
		segmentState: state,
		vectorDataIn: vectorDataIn,
		metaIn:       metaIn,
		fields:       make(map[string]*quantizedFieldReader),
	}

	// Load field metadata
	if err := reader.loadFieldMetadata(); err != nil {
		reader.Close()
		return nil, err
	}

	return reader, nil
}

// readScalarQuantizedMetaHeader reads and validates the metadata file header.
func readScalarQuantizedMetaHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x51564543 { // "QVEC"
		return fmt.Errorf("invalid magic number: expected 0x51564543, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != uint32(Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT) {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// loadFieldMetadata loads metadata for all fields.
func (r *Lucene104ScalarQuantizedVectorsReader) loadFieldMetadata() error {
	// Read number of fields
	numFields, err := store.ReadUint32(r.metaIn)
	if err != nil {
		return fmt.Errorf("failed to read field count: %w", err)
	}

	for i := uint32(0); i < numFields; i++ {
		// Read field number
		fieldNum, err := store.ReadUint32(r.metaIn)
		if err != nil {
			return fmt.Errorf("failed to read field number: %w", err)
		}

		// Read encoding
		encVal, err := store.ReadUint32(r.metaIn)
		if err != nil {
			return fmt.Errorf("failed to read encoding: %w", err)
		}

		// Read vector count
		vecCount, err := store.ReadUint32(r.metaIn)
		if err != nil {
			return fmt.Errorf("failed to read vector count: %w", err)
		}

		// Read dimensions
		dims, err := store.ReadUint32(r.metaIn)
		if err != nil {
			return fmt.Errorf("failed to read dimensions: %w", err)
		}

		fieldReader := &quantizedFieldReader{
			fieldNumber: int(fieldNum),
			encoding:    ScalarEncoding(encVal),
			vectorCount: int(vecCount),
			dimensions:  int(dims),
		}

		// Store by field number (will be mapped to name by caller)
		r.fields[fmt.Sprintf("field_%d", fieldNum)] = fieldReader
	}

	return nil
}

// CheckIntegrity checks the integrity of the vectors.
func (r *Lucene104ScalarQuantizedVectorsReader) CheckIntegrity() error {
	if r.closed {
		return fmt.Errorf("reader is closed")
	}

	// Verify we can read all field metadata
	for name, field := range r.fields {
		if field.vectorCount < 0 {
			return fmt.Errorf("invalid vector count for field %s", name)
		}
		if field.dimensions <= 0 || field.dimensions > Lucene104ScalarQuantizedVectorsFormat_MAX_DIMENSIONS {
			return fmt.Errorf("invalid dimensions %d for field %s", field.dimensions, name)
		}
	}

	return nil
}

// Close releases resources.
func (r *Lucene104ScalarQuantizedVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	var firstErr error
	if err := r.vectorDataIn.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.metaIn.Close(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// quantizedFieldReader handles reading for a single field
type quantizedFieldReader struct {
	fieldNumber int
	encoding    ScalarEncoding
	vectorCount int
	dimensions  int
	dataOffset  int64
}

// QuantizedVectorValues provides access to quantized vector values.
type QuantizedVectorValues struct {
	encoding    ScalarEncoding
	dimensions  int
	vectorCount int
	data        []byte
	offsets     []int64
}

// Dimension returns the dimension of the vectors.
func (v *QuantizedVectorValues) Dimension() int {
	return v.dimensions
}

// Size returns the number of vectors.
func (v *QuantizedVectorValues) Size() int {
	return v.vectorCount
}

// GetEncoding returns the vector encoding type.
func (v *QuantizedVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

// Copy creates a copy of the values.
func (v *QuantizedVectorValues) Copy() (KnnVectorValues, error) {
	copiedData := make([]byte, len(v.data))
	copy(copiedData, v.data)

	copiedOffsets := make([]int64, len(v.offsets))
	copy(copiedOffsets, v.offsets)

	return &QuantizedVectorValues{
		encoding:    v.encoding,
		dimensions:  v.dimensions,
		vectorCount: v.vectorCount,
		data:        copiedData,
		offsets:     copiedOffsets,
	}, nil
}

// GetQuantizedVector returns the quantized vector at the given ordinal.
func (v *QuantizedVectorValues) GetQuantizedVector(ordinal int) ([]byte, error) {
	if ordinal < 0 || ordinal >= v.vectorCount {
		return nil, fmt.Errorf("ordinal %d out of range [0, %d)", ordinal, v.vectorCount)
	}

	packedLength := v.encoding.GetDocPackedLength(v.dimensions)
	startOffset := v.offsets[ordinal]
	endOffset := startOffset + int64(packedLength)

	if endOffset > int64(len(v.data)) {
		return nil, fmt.Errorf("vector data out of bounds")
	}

	result := make([]byte, packedLength)
	copy(result, v.data[startOffset:endOffset])
	return result, nil
}

// DequantizeVector dequantizes a quantized vector back to float32.
func DequantizeVector(quantized []byte, encoding ScalarEncoding, dimensions int) []float32 {
	result := make([]float32, dimensions)

	switch encoding {
	case ScalarEncodingUnsignedByte:
		for i := 0; i < dimensions && i < len(quantized); i++ {
			// Convert 0-255 to -1.0 to 1.0
			result[i] = (float32(quantized[i]) / 127.5) - 1.0
		}
	case ScalarEncodingPackedNibble:
		for i := 0; i < dimensions; i++ {
			byteIdx := i / 2
			if byteIdx >= len(quantized) {
				break
			}
			var nibble byte
			if i%2 == 0 {
				nibble = quantized[byteIdx] >> 4
			} else {
				nibble = quantized[byteIdx] & 0x0F
			}
			// Convert 0-15 to -1.0 to 1.0
			result[i] = (float32(nibble) / 7.5) - 1.0
		}
	case ScalarEncodingSingleBitQueryNibble:
		for i := 0; i < dimensions; i++ {
			byteIdx := i / 8
			if byteIdx >= len(quantized) {
				break
			}
			bitIdx := uint(i % 8)
			if (quantized[byteIdx] & (1 << bitIdx)) != 0 {
				result[i] = 1.0
			} else {
				result[i] = -1.0
			}
		}
	case ScalarEncodingDibitQueryNibble:
		for i := 0; i < dimensions; i++ {
			byteIdx := i / 4
			if byteIdx >= len(quantized) {
				break
			}
			shift := uint((i % 4) * 2)
			dibit := (quantized[byteIdx] >> shift) & 0x03
			// Convert 0-3 to -1.0 to 1.0
			result[i] = (float32(dibit) / 1.5) - 1.0
		}
	}

	return result
}

// QuantizeVector quantizes a float32 vector to the specified encoding.
func QuantizeVector(vector []float32, encoding ScalarEncoding) []byte {
	discreteDims := encoding.GetDiscreteDimensions(len(vector))
	packedLength := encoding.GetDocPackedLength(discreteDims)
	result := make([]byte, packedLength)

	switch encoding {
	case ScalarEncodingUnsignedByte:
		for i, v := range vector {
			if i >= discreteDims {
				break
			}
			// Scale -1.0 to 1.0 to 0-255
			scaled := (v + 1.0) * 127.5
			if scaled < 0 {
				scaled = 0
			}
			if scaled > 255 {
				scaled = 255
			}
			result[i] = byte(scaled)
		}
	case ScalarEncodingPackedNibble:
		for i := 0; i < len(vector); i += 2 {
			// Scale -1.0 to 1.0 to 0-15
			v1 := (vector[i] + 1.0) * 7.5
			if v1 < 0 {
				v1 = 0
			}
			if v1 > 15 {
				v1 = 15
			}
			result[i/2] = byte(int(v1) << 4)
			if i+1 < len(vector) {
				v2 := (vector[i+1] + 1.0) * 7.5
				if v2 < 0 {
					v2 = 0
				}
				if v2 > 15 {
					v2 = 15
				}
				result[i/2] |= byte(int(v2))
			}
		}
	case ScalarEncodingSingleBitQueryNibble:
		for i := 0; i < len(vector); i++ {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			if vector[i] > 0 {
				result[byteIdx] |= 1 << bitIdx
			}
		}
	case ScalarEncodingDibitQueryNibble:
		for i := 0; i < len(vector); i++ {
			byteIdx := i / 4
			shift := uint((i % 4) * 2)
			v := (vector[i] + 1.0) * 1.5
			if v < 0 {
				v = 0
			}
			if v > 3 {
				v = 3
			}
			result[byteIdx] |= byte(int(v) << shift)
		}
	}

	return result
}

// CalculateCentroid calculates the centroid of a set of vectors.
func CalculateCentroid(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}

	dims := len(vectors[0])
	centroid := make([]float32, dims)

	for _, vector := range vectors {
		for i := range vector {
			centroid[i] += vector[i]
		}
	}

	count := float32(len(vectors))
	for i := range centroid {
		centroid[i] /= count
	}

	return centroid
}

// CalculateQuantizedSimilarity calculates similarity between quantized vectors.
func CalculateQuantizedSimilarity(simFunc VectorSimilarityFunction, v1, v2 []byte, encoding ScalarEncoding) float32 {
	// Dequantize and compute similarity
	dims := encoding.GetDiscreteDimensions(len(v1) * 2) // Approximate
	f1 := DequantizeVector(v1, encoding, dims)
	f2 := DequantizeVector(v2, encoding, dims)
	return ComputeSimilarity(simFunc, f1, f2)
}

// CorrectiveTerms holds the corrective terms for quantization.
type CorrectiveTerms struct {
	LowerInterval         float32
	UpperInterval         float32
	AdditionalCorrection  float32
	QuantizedComponentSum int64
}

// ComputeCorrectiveTerms computes corrective terms for quantization.
func ComputeCorrectiveTerms(quantizedSum int64, lowerInterval, upperInterval, additionalCorrection float32) *CorrectiveTerms {
	return &CorrectiveTerms{
		LowerInterval:         lowerInterval,
		UpperInterval:         upperInterval,
		AdditionalCorrection:  additionalCorrection,
		QuantizedComponentSum: quantizedSum,
	}
}