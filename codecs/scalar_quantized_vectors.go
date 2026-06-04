// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/Lucene104ScalarQuantizedVectorsFormat.java
// Purpose: per-vector optimized scalar quantization for vector storage —
// compresses float vectors to quantized byte representations, byte-for-byte
// compatible with Apache Lucene 10.4.0.
//
// This file defines the ScalarEncoding enum and the
// Lucene104ScalarQuantizedVectorsFormat. The byte-faithful writer lives in
// lucene104_scalar_quantized_vectors_writer.go; the read-side dequantizing
// values live in lucene104_off_heap_scalar_quantized_float_vector_values.go.
// The full search-integration KnnVectorsReader (getFloatVectorValues / search)
// is tracked separately (rmp #134); the reader in this file validates the
// CodecUtil framing and parses field metadata but does not yet expose the
// value-access surface.

package codecs

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// ScalarEncoding represents the encoding type for scalar quantized vectors.
// This is the Go equivalent of Lucene's
// Lucene104ScalarQuantizedVectorsFormat.ScalarEncoding enum (Lucene 10.4.0);
// the iota order matches the Java declaration order so the enum ordinals line
// up, but on the wire the encoding is identified by its wire number (see
// [ScalarEncoding.GetWireNumber]), not its ordinal.
type ScalarEncoding int

const (
	// ScalarEncodingUnsignedByte quantizes each dimension to 8 bits, treated
	// as an unsigned value. Wire number 0.
	ScalarEncodingUnsignedByte ScalarEncoding = iota
	// ScalarEncodingPackedNibble quantizes each dimension to 4 bits, packing
	// two values into each output byte. Wire number 1.
	ScalarEncodingPackedNibble
	// ScalarEncodingSevenBit quantizes each dimension to 7 bits, treated as a
	// signed value (backwards compatible with older scalar quantization). Wire
	// number 2.
	ScalarEncodingSevenBit
	// ScalarEncodingSingleBitQueryNibble quantizes each dimension to a single
	// bit; query vectors are quantized to 4 bits. Wire number 3.
	ScalarEncodingSingleBitQueryNibble
	// ScalarEncodingDibitQueryNibble quantizes each dimension to 2 bits; query
	// vectors are quantized to 4 bits. Wire number 4.
	ScalarEncodingDibitQueryNibble
)

// scalarEncodingBits maps each encoding to its (bits, bitsPerDim, queryBits,
// queryBitsPerDim). Mirrors the per-constant constructor arguments of the Java
// ScalarEncoding enum.
type scalarEncodingParams struct {
	wireNumber      int
	bits            int
	bitsPerDim      int
	queryBits       int
	queryBitsPerDim int
}

// scalarEncodingTable holds the Java enum constructor parameters in
// declaration (ordinal) order. Mirrors the Java enum constants:
//
//	UNSIGNED_BYTE(0, 8, 8)
//	PACKED_NIBBLE(1, 4, 4)
//	SEVEN_BIT(2, 7, 8)
//	SINGLE_BIT_QUERY_NIBBLE(3, 1, 1, 4, 4)
//	DIBIT_QUERY_NIBBLE(4, 2, 2, 4, 4)
var scalarEncodingTable = [...]scalarEncodingParams{
	ScalarEncodingUnsignedByte:         {wireNumber: 0, bits: 8, bitsPerDim: 8, queryBits: 8, queryBitsPerDim: 8},
	ScalarEncodingPackedNibble:         {wireNumber: 1, bits: 4, bitsPerDim: 4, queryBits: 4, queryBitsPerDim: 4},
	ScalarEncodingSevenBit:             {wireNumber: 2, bits: 7, bitsPerDim: 8, queryBits: 7, queryBitsPerDim: 8},
	ScalarEncodingSingleBitQueryNibble: {wireNumber: 3, bits: 1, bitsPerDim: 1, queryBits: 4, queryBitsPerDim: 4},
	ScalarEncodingDibitQueryNibble:     {wireNumber: 4, bits: 2, bitsPerDim: 2, queryBits: 4, queryBitsPerDim: 4},
}

// String returns the string representation of the ScalarEncoding, matching the
// Java enum constant names.
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
		return fmt.Sprintf("UNKNOWN(%d)", int(se))
	}
}

// GetBits returns the number of bits used per dimension for the document-side
// quantization. Mirrors Java's ScalarEncoding.getBits().
func (se ScalarEncoding) GetBits() int {
	return scalarEncodingTable[se].bits
}

// GetWireNumber returns the number used to identify this encoding on the wire,
// independent of the enum ordinal. Mirrors Java's
// ScalarEncoding.getWireNumber().
func (se ScalarEncoding) GetWireNumber() int {
	return scalarEncodingTable[se].wireNumber
}

// GetDiscreteDimensions returns the number of dimensions rounded up so the
// per-dimension bits fit into whole bytes. Mirrors Java's
// ScalarEncoding.getDiscreteDimensions(int), including the DIBIT_QUERY_NIBBLE
// override that forces dibit packing to byte boundaries assuming single-bit
// striping.
func (se ScalarEncoding) GetDiscreteDimensions(dimensions int) int {
	p := scalarEncodingTable[se]
	if se == ScalarEncodingDibitQueryNibble {
		queryDiscretized := (dimensions*4 + 7) / 8 * 8 / 4
		docDiscretized := (dimensions + 7) / 8 * 8
		if queryDiscretized > docDiscretized {
			return queryDiscretized
		}
		return docDiscretized
	}
	if p.queryBits == p.bits {
		totalBits := dimensions * p.bitsPerDim
		return (totalBits + 7) / 8 * 8 / p.bitsPerDim
	}
	queryDiscretized := (dimensions*p.queryBitsPerDim + 7) / 8 * 8 / p.queryBitsPerDim
	docDiscretized := (dimensions*p.bitsPerDim + 7) / 8 * 8 / p.bitsPerDim
	if queryDiscretized > docDiscretized {
		return queryDiscretized
	}
	return docDiscretized
}

// GetDocPackedLength returns the number of bytes required to store a packed
// document vector of the given (raw, not yet discretized) dimensions. Mirrors
// Java's ScalarEncoding.getDocPackedLength(int), including the
// DIBIT_QUERY_NIBBLE override that stores two single-bit stripes.
func (se ScalarEncoding) GetDocPackedLength(dimensions int) int {
	p := scalarEncodingTable[se]
	discretized := se.GetDiscreteDimensions(dimensions)
	if se == ScalarEncodingDibitQueryNibble {
		// DIBIT is stored as two single-bit stripes.
		return 2 * ((discretized + 7) / 8)
	}
	totalBits := discretized * p.bitsPerDim
	return (totalBits + 7) / 8
}

// GetQueryPackedLength returns the number of bytes required to store a packed
// query vector of the given dimensions. Mirrors Java's
// ScalarEncoding.getQueryPackedLength(int).
func (se ScalarEncoding) GetQueryPackedLength(dimensions int) int {
	p := scalarEncodingTable[se]
	discretized := se.GetDiscreteDimensions(dimensions)
	totalBits := discretized * p.queryBitsPerDim
	return (totalBits + 7) / 8
}

// IsAsymmetric reports whether the document-side and query-side bit-widths
// differ. Mirrors Java's ScalarEncoding.isAsymmetric().
func (se ScalarEncoding) IsAsymmetric() bool {
	p := scalarEncodingTable[se]
	return p.bits != p.queryBits
}

// ScalarEncodingValues returns all scalar encoding values in ordinal order.
func ScalarEncodingValues() []ScalarEncoding {
	return []ScalarEncoding{
		ScalarEncodingUnsignedByte,
		ScalarEncodingPackedNibble,
		ScalarEncodingSevenBit,
		ScalarEncodingSingleBitQueryNibble,
		ScalarEncodingDibitQueryNibble,
	}
}

// ScalarEncodingFromWireNumber returns the encoding for the given wire number.
// Mirrors Java's ScalarEncoding.fromWireNumber(int).
func ScalarEncodingFromWireNumber(wireNumber int) (ScalarEncoding, error) {
	for i := range scalarEncodingTable {
		if scalarEncodingTable[i].wireNumber == wireNumber {
			return ScalarEncoding(i), nil
		}
	}
	return 0, fmt.Errorf("lucene104 sq: no ScalarEncoding for wire number %d", wireNumber)
}

// Lucene104ScalarQuantizedVectorsFormat version and tuning constants. Mirror
// the static definitions in the Java
// Lucene104ScalarQuantizedVectorsFormat (Lucene 10.4.0).
const (
	// Lucene104ScalarQuantizedVectorsFormat_VERSION_START is the initial format version.
	Lucene104ScalarQuantizedVectorsFormat_VERSION_START = 0
	// Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT is the current format version.
	Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT = Lucene104ScalarQuantizedVectorsFormat_VERSION_START
	// Lucene104ScalarQuantizedVectorsFormat_DIRECT_MONOTONIC_BLOCK_SHIFT is the
	// block shift used by the DirectMonotonicWriter that records the sparse
	// ord->doc mapping.
	Lucene104ScalarQuantizedVectorsFormat_DIRECT_MONOTONIC_BLOCK_SHIFT = 16
	// Lucene104ScalarQuantizedVectorsFormat_MAX_DIMENSIONS is the maximum
	// number of dimensions supported.
	Lucene104ScalarQuantizedVectorsFormat_MAX_DIMENSIONS = 1024
	// Lucene104ScalarQuantizedVectorsFormat_QUANTIZED_VECTOR_COMPONENT is the
	// info-stream component name. Mirrors QUANTIZED_VECTOR_COMPONENT.
	Lucene104ScalarQuantizedVectorsFormat_QUANTIZED_VECTOR_COMPONENT = "QVEC"
)

// Internal codec/extension constants. Mirror the package-private static
// constants of the Java format.
const (
	lucene104SQName                 = "Lucene104ScalarQuantizedVectorsFormat"
	lucene104SQMetaCodecName        = "Lucene104ScalarQuantizedVectorsFormatMeta"
	lucene104SQDataCodecName        = "Lucene104ScalarQuantizedVectorsFormatData"
	lucene104SQVersionStart   int32 = Lucene104ScalarQuantizedVectorsFormat_VERSION_START
	lucene104SQVersionCurrent int32 = Lucene104ScalarQuantizedVectorsFormat_VERSION_CURRENT
)

// File extensions for Lucene104ScalarQuantizedVectorsFormat. Mirror the Java
// META_EXTENSION / VECTOR_DATA_EXTENSION and the raw flat delegate extensions.
const (
	// Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION is the
	// quantized vector data file extension (Java VECTOR_DATA_EXTENSION).
	Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION = "veq"
	// Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION is the
	// quantized vector metadata file extension (Java META_EXTENSION).
	Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION = "vemq"
	// Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_EXTENSION is the raw
	// (delegate) vector data file extension.
	Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_EXTENSION = "vec"
	// Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_META_EXTENSION is the
	// raw (delegate) vector metadata file extension.
	Lucene104ScalarQuantizedVectorsFormat_RAW_VECTOR_META_EXTENSION = "vemf"
)

// Lucene104ScalarQuantizedVectorsFormat implements per-vector optimized scalar
// quantization for vector storage. It compresses float vectors to quantized
// byte representations for efficient storage and fast approximate similarity
// computation, byte-for-byte compatible with Apache Lucene 10.4.0.
//
// This is the Go port of Lucene's Lucene104ScalarQuantizedVectorsFormat.
type Lucene104ScalarQuantizedVectorsFormat struct {
	*BaseKnnVectorsFormat
	encoding ScalarEncoding
}

// NewLucene104ScalarQuantizedVectorsFormat creates a new
// Lucene104ScalarQuantizedVectorsFormat with the default encoding
// (UNSIGNED_BYTE). Mirrors the Java no-arg constructor.
func NewLucene104ScalarQuantizedVectorsFormat() *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat(lucene104SQName),
		encoding:             ScalarEncodingUnsignedByte,
	}
}

// NewLucene104ScalarQuantizedVectorsFormatWithEncoding creates a new format
// with the specified encoding. Mirrors the Java single-argument constructor.
func NewLucene104ScalarQuantizedVectorsFormatWithEncoding(encoding ScalarEncoding) *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat(lucene104SQName),
		encoding:             encoding,
	}
}

// Encoding returns the scalar encoding used by this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) Encoding() ScalarEncoding {
	return f.encoding
}

// String returns a string representation of this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorsFormat(name=%s, encoding=%s)",
		lucene104SQName, f.encoding.String())
}

// FieldsWriter returns the byte-faithful writer for quantized vectors.
func (f *Lucene104ScalarQuantizedVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return NewLucene104ScalarQuantizedVectorsWriter(state, f.encoding)
}

// FieldsReader returns a reader that validates the CodecUtil framing and
// parses the per-field metadata. The full value-access surface
// (getFloatVectorValues / search) is tracked by rmp #134; the round-trip read
// path is exercised today via
// [OffHeapScalarQuantizedFloatVectorValues.Load].
func (f *Lucene104ScalarQuantizedVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return NewLucene104ScalarQuantizedVectorsReader(state, f.encoding)
}

// MaxDimensions returns the maximum supported vector dimension. Mirrors Java's
// getMaxDimensions.
func (f *Lucene104ScalarQuantizedVectorsFormat) MaxDimensions(_ string) int {
	return Lucene104ScalarQuantizedVectorsFormat_MAX_DIMENSIONS
}

// SupportsFloatVectorFallback returns true: the format dequantizes on read and
// surfaces FLOAT32 vectors.
func (f *Lucene104ScalarQuantizedVectorsFormat) SupportsFloatVectorFallback() bool {
	return true
}

// Lucene104ScalarQuantizedFieldEntry holds the parsed .vemq per-field metadata.
// It mirrors the Java FieldEntry record, exposing the fields a reader needs to
// reconstruct the on-disk vectors. Exported so the round-trip test and the
// future search-integration reader (rmp #134) can build an
// [OffHeapScalarQuantizedFloatVectorValues] view via Load.
type Lucene104ScalarQuantizedFieldEntry struct {
	// VectorEncoding is the field's vector encoding (FLOAT32 / BYTE).
	VectorEncoding index.VectorEncoding
	// SimilarityFunction is the field's vector similarity function.
	SimilarityFunction index.VectorSimilarityFunction
	// Dimension is the raw vector dimension.
	Dimension int
	// VectorDataOffset is the .veq offset of the field's quantized vectors.
	VectorDataOffset int64
	// VectorDataLength is the .veq byte length of the field's quantized vectors.
	VectorDataLength int64
	// Size is the number of vectors stored for the field.
	Size int
	// Encoding is the scalar encoding used for the field (present when Size>0).
	Encoding ScalarEncoding
	// Centroid is the per-field centroid (present when Size>0).
	Centroid []float32
	// CentroidDP is the centroid square magnitude (present when Size>0).
	CentroidDP float32

	// DocsWithFieldOffset distinguishes empty(-2) / dense(-1) / sparse(>=0).
	DocsWithFieldOffset int64
	// The remaining fields carry the sparse OrdToDoc state.
	DocsWithFieldLength int64
	JumpTableEntryCount int
	DenseRankPower      byte
	AddressesOffset     int64
	AddressesLength     int64
	// OrdToDocMeta is the DirectMonotonic meta header for the sparse ord->doc
	// mapping (nil for dense/empty).
	OrdToDocMeta *packed.DirectMonotonicMeta
}

// Lucene104ScalarQuantizedVectorsReader validates the .veq / .vemq CodecUtil
// framing and parses the per-field metadata. It is a deliberately partial
// reader: it proves the writer's framing is sound (CheckIntegrity validates the
// .veq checksum) and exposes the parsed field entries for the round-trip read
// path, but the full search-integration surface (getFloatVectorValues, search)
// is tracked by rmp #134. The value-access methods are intentionally absent
// rather than stubbed with fabricated data.
type Lucene104ScalarQuantizedVectorsReader struct {
	encoding   ScalarEncoding
	fieldInfos *index.FieldInfos
	fields     map[int]*Lucene104ScalarQuantizedFieldEntry
	vectorData store.IndexInput
	closed     bool
}

// NewLucene104ScalarQuantizedVectorsReader opens the .veq data file, validates
// both files' CodecUtil index headers (and the .veq footer checksum), and
// parses the .vemq field records.
func NewLucene104ScalarQuantizedVectorsReader(state *SegmentReadState, encoding ScalarEncoding) (*Lucene104ScalarQuantizedVectorsReader, error) {
	if state == nil || state.SegmentInfo == nil || state.Directory == nil {
		return nil, errors.New("lucene104 sq: invalid SegmentReadState")
	}
	r := &Lucene104ScalarQuantizedVectorsReader{
		encoding:   encoding,
		fieldInfos: state.FieldInfos,
		fields:     make(map[int]*Lucene104ScalarQuantizedFieldEntry),
	}

	versionMeta, err := r.readMetadata(state)
	if err != nil {
		return nil, err
	}

	dataName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION)
	dataIn, err := state.Directory.OpenInput(dataName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene104 sq: open data %q: %w", dataName, err)
	}
	id := state.SegmentInfo.GetID()
	versionData, err := CheckIndexHeader(
		dataIn, lucene104SQDataCodecName, lucene104SQVersionStart, lucene104SQVersionCurrent, id, state.SegmentSuffix,
	)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: data header %q: %w", dataName, err)
	}
	if versionData != versionMeta {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: format versions mismatch: meta=%d, data=%d", versionMeta, versionData)
	}
	if _, err := RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: retrieve data checksum %q: %w", dataName, err)
	}
	r.vectorData = dataIn
	return r, nil
}

// readMetadata reads and validates the .vemq header, parses every field record
// until the -1 sentinel, and checks the footer. Returns the meta version.
func (r *Lucene104ScalarQuantizedVectorsReader) readMetadata(state *SegmentReadState) (int32, error) {
	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, Lucene104ScalarQuantizedVectorsFormat_VECTOR_META_EXTENSION)
	metaRaw, err := state.Directory.OpenInput(metaName, store.IOContextRead)
	if err != nil {
		return 0, fmt.Errorf("lucene104 sq: open meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexInput(metaRaw)

	var versionMeta int32
	var readErr error
	func() {
		id := state.SegmentInfo.GetID()
		v, e := CheckIndexHeader(
			meta, lucene104SQMetaCodecName, lucene104SQVersionStart, lucene104SQVersionCurrent, id, state.SegmentSuffix,
		)
		if e != nil {
			readErr = e
			return
		}
		versionMeta = v
		readErr = r.readFields(meta)
	}()

	_, footerErr := CheckFooter(meta)
	_ = metaRaw.Close()
	if readErr != nil {
		return 0, fmt.Errorf("lucene104 sq: read meta %q: %w", metaName, readErr)
	}
	if footerErr != nil {
		return 0, fmt.Errorf("lucene104 sq: meta footer %q: %w", metaName, footerErr)
	}
	return versionMeta, nil
}

// readFields parses every per-field record until the -1 sentinel.
func (r *Lucene104ScalarQuantizedVectorsReader) readFields(meta store.DataInput) error {
	for {
		fieldNum, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("reading field number: %w", err)
		}
		if fieldNum == -1 {
			break
		}
		var info *index.FieldInfo
		if r.fieldInfos != nil {
			info = r.fieldInfos.GetByNumber(int(fieldNum))
			if info == nil {
				return fmt.Errorf("invalid field number %d", fieldNum)
			}
		}
		entry, err := readScalarQuantizedFieldEntry(meta, info)
		if err != nil {
			return fmt.Errorf("field %d: %w", fieldNum, err)
		}
		r.fields[int(fieldNum)] = entry
	}
	return nil
}

// readScalarQuantizedFieldEntry parses one .vemq field record. Mirrors the
// Java FieldEntry.create read order exactly: encoding ordinal, similarity
// ordinal, vint dimension, vlong offset, vlong length, vint size, then (when
// size>0) wire number, centroid floats and centroidDP, then the OrdToDoc
// stored-meta block. The field number is consumed by the caller.
func readScalarQuantizedFieldEntry(meta store.DataInput, info *index.FieldInfo) (*Lucene104ScalarQuantizedFieldEntry, error) {
	encOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	enc := index.VectorEncoding(encOrd)

	simOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	if int(simOrd) < 0 || int(simOrd) >= len(lucene99HnswSimilarityOrdinals) {
		return nil, fmt.Errorf("invalid similarity ordinal: %d", simOrd)
	}
	sim := lucene99HnswSimilarityOrdinals[simOrd]

	dimV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	vectorDataOffset, err := store.ReadVLong(meta)
	if err != nil {
		return nil, err
	}
	vectorDataLength, err := store.ReadVLong(meta)
	if err != nil {
		return nil, err
	}
	size, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}

	entry := &Lucene104ScalarQuantizedFieldEntry{
		VectorEncoding:     enc,
		SimilarityFunction: sim,
		Dimension:          int(dimV),
		VectorDataOffset:   vectorDataOffset,
		VectorDataLength:   vectorDataLength,
		Size:               int(size),
		Encoding:           ScalarEncodingUnsignedByte,
	}

	if size > 0 {
		wireNumber, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		scalarEncoding, e := ScalarEncodingFromWireNumber(int(wireNumber))
		if e != nil {
			return nil, e
		}
		entry.Encoding = scalarEncoding
		centroid := make([]float32, int(dimV))
		for i := range centroid {
			// IndexInput.readInt is little-endian; readFloats reconstructs each
			// float via intBitsToFloat(readInt()). Mirror that here.
			bits, e := meta.ReadInt()
			if e != nil {
				return nil, e
			}
			centroid[i] = math.Float32frombits(uint32(bits))
		}
		entry.Centroid = centroid
		dpBits, e := meta.ReadInt()
		if e != nil {
			return nil, e
		}
		entry.CentroidDP = math.Float32frombits(uint32(dpBits))
	}

	// OrdToDocDISIReaderConfiguration.fromStoredMeta.
	docsWithFieldOffset, err := meta.ReadLong()
	if err != nil {
		return nil, err
	}
	docsWithFieldLength, err := meta.ReadLong()
	if err != nil {
		return nil, err
	}
	jumpTableEntryCount, err := meta.ReadShort()
	if err != nil {
		return nil, err
	}
	denseRankPower, err := meta.ReadByte()
	if err != nil {
		return nil, err
	}
	entry.DocsWithFieldOffset = docsWithFieldOffset
	entry.DocsWithFieldLength = docsWithFieldLength
	entry.JumpTableEntryCount = int(jumpTableEntryCount)
	entry.DenseRankPower = byte(denseRankPower)

	if docsWithFieldOffset > -1 {
		// Sparse: read the DirectMonotonicWriter header that records the
		// ord->doc mapping, mirroring the docsWithFieldOffset > -1 branch of
		// OrdToDocDISIReaderConfiguration.fromStoredMeta.
		addressesOffset, e := meta.ReadLong()
		if e != nil {
			return nil, e
		}
		blockShift, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		ordToDocMeta, e := packed.LoadDirectMonotonicMeta(meta, int64(size), int(blockShift))
		if e != nil {
			return nil, fmt.Errorf("load ord-to-doc monotonic meta: %w", e)
		}
		addressesLength, e := meta.ReadLong()
		if e != nil {
			return nil, e
		}
		entry.AddressesOffset = addressesOffset
		entry.AddressesLength = addressesLength
		entry.OrdToDocMeta = ordToDocMeta
	}

	if info != nil {
		if sim != info.VectorSimilarityFunction() {
			return nil, fmt.Errorf("inconsistent similarity for field %q: %v != %v",
				info.Name(), sim, info.VectorSimilarityFunction())
		}
		if int(dimV) != info.VectorDimension() {
			return nil, fmt.Errorf("inconsistent dimension for field %q: %d != %d",
				info.Name(), dimV, info.VectorDimension())
		}
	}
	return entry, nil
}

// FieldEntry returns the parsed metadata for the named field, or an error if
// the field is unknown. Exposed for the round-trip read path.
func (r *Lucene104ScalarQuantizedVectorsReader) FieldEntry(field string) (*Lucene104ScalarQuantizedFieldEntry, error) {
	if r.fieldInfos == nil {
		return nil, errors.New("lucene104 sq: reader has no field infos")
	}
	info := r.fieldInfos.GetByName(field)
	if info == nil {
		return nil, fmt.Errorf("lucene104 sq: field %q not found", field)
	}
	entry, ok := r.fields[info.Number()]
	if !ok {
		return nil, fmt.Errorf("lucene104 sq: field %q has no vector entry", field)
	}
	return entry, nil
}

// VectorData returns the open .veq input. Exposed for the round-trip read path
// so callers can slice the quantized vector data for [Load].
func (r *Lucene104ScalarQuantizedVectorsReader) VectorData() store.IndexInput { return r.vectorData }

// CheckIntegrity verifies the .veq checksum end-to-end.
func (r *Lucene104ScalarQuantizedVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene104 sq: reader closed")
	}
	_, err := ChecksumEntireFile(r.vectorData)
	return err
}

// Close releases the .veq file handle. Idempotent.
func (r *Lucene104ScalarQuantizedVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.vectorData != nil {
		return r.vectorData.Close()
	}
	return nil
}
