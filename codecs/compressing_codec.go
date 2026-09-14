// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"
)

// compressingCodecName is the codec name, which Java's CompressingCodec also
// passes as the stored-fields and term-vectors format name
// (CompressingCodec.java:113-124).
const compressingCodecName = "CompressingCodec"

// compressingCodecBlockShift is the fields-index block shift. Java takes it as
// a constructor argument; this Gocene constructor does not carry one, so it
// uses 10, the value Lucene90StoredFieldsFormat.impl(Mode) passes.
const compressingCodecBlockShift = 10

// CompressingCodec is a codec that compresses stored fields and term vectors.
//
// This is the Go port of Lucene's CompressingCodec.
// It uses Lucene90CompressingStoredFieldsFormat and Lucene90CompressingTermVectorsFormat
// to compress data using configurable compression modes.
//
// The codec is byte-compatible with Apache Lucene's implementation.
type CompressingCodec struct {
	*BaseCodec
	storedFieldsFormat StoredFieldsFormat
	termVectorsFormat  TermVectorsFormat
	fieldInfosFormat   FieldInfosFormat
	postingsFormat     PostingsFormat
	docValuesFormat    DocValuesFormat
	normsFormat        NormsFormat
	liveDocsFormat     LiveDocsFormat
	pointsFormat       PointsFormat
	compressionMode    CompressionMode
	chunkSize          int
	maxDocsPerChunk    int
}

// DefaultCompressingCodec creates a new CompressingCodec with default settings.
// Uses LZ4_FAST compression with 16KB chunks and 128 docs per chunk.
func DefaultCompressingCodec() *CompressingCodec {
	return NewCompressingCodec(CompressionModeLZ4Fast, 16*1024, 128)
}

// NewCompressingCodec creates a new CompressingCodec with the specified compression settings.
//
// Parameters:
//   - mode: The compression mode to use (LZ4_FAST, LZ4_HIGH, or DEFLATE)
//   - chunkSize: The target chunk size in bytes (must be >= 1KB)
//   - maxDocsPerChunk: The maximum number of documents per chunk (must be >= 1)
func NewCompressingCodec(mode CompressionMode, chunkSize, maxDocsPerChunk int) *CompressingCodec {
	if chunkSize < 1024 {
		chunkSize = 1024
	}
	if maxDocsPerChunk < 1 {
		maxDocsPerChunk = 1
	}

	// Java: this.storedFieldsFormat = new Lucene90CompressingStoredFieldsFormat(
	//           name, segmentSuffix, compressionMode, chunkSize, maxDocsPerChunk, blockShift)
	// (CompressingCodec.java:113-121) — the codec's own name doubles as the
	// format name. That constructor lives in codecs/lucene90/compressing,
	// which imports this package, so it is reached through the init()-time
	// registration described in stored_fields_format.go.
	//
	// DIVERGENCE, pre-existing: Java's CompressingCodec constructor takes
	// segmentSuffix and blockShift; this Gocene constructor carries neither.
	// The suffix is empty (the ported format has no suffix support) and the
	// block shift is 10, the value Lucene90StoredFieldsFormat.impl(Mode)
	// passes (Lucene90StoredFieldsFormat.java:157-170).
	storedFieldsFormat := NewLucene90CompressingStoredFieldsFormat(
		Lucene90CompressingStoredFieldsFormatOptions{
			FormatName:      compressingCodecName,
			CompressionMode: mode,
			ChunkSize:       chunkSize,
			MaxDocsPerChunk: maxDocsPerChunk,
			BlockShift:      compressingCodecBlockShift,
		})
	// Java: this.termVectorsFormat = new Lucene90CompressingTermVectorsFormat(
	//           name, segmentSuffix, compressionMode, chunkSize, maxDocsPerChunk, blockShift)
	// (CompressingCodec.java). The constructor lives in
	// codecs/lucene90/compressing, reached through the init()-time registration
	// described in term_vectors_format.go; segmentSuffix and blockShift follow
	// the stored-fields divergence noted above.
	termVectorsFormat := NewLucene90CompressingTermVectorsFormat(
		Lucene90CompressingTermVectorsFormatOptions{
			FormatName:      compressingCodecName,
			SegmentSuffix:   "",
			CompressionMode: mode,
			ChunkSize:       chunkSize,
			MaxDocsPerChunk: maxDocsPerChunk,
			BlockSize:       compressingCodecBlockShift,
		})

	return &CompressingCodec{
		BaseCodec:          NewBaseCodec(compressingCodecName),
		storedFieldsFormat: storedFieldsFormat,
		termVectorsFormat:  termVectorsFormat,
		fieldInfosFormat:   NewLucene104FieldInfosFormat(),
		postingsFormat:     NewLucene104PostingsFormat(),
		docValuesFormat:    NewLucene90DocValuesFormat(),
		normsFormat:        NewLucene90NormsFormat(),
		liveDocsFormat:     NewLucene90LiveDocsFormat(),
		pointsFormat:       NewLucene90PointsFormat(),
		compressionMode:    mode,
		chunkSize:          chunkSize,
		maxDocsPerChunk:    maxDocsPerChunk,
	}
}

// FastCompressingCodec creates a CompressingCodec optimized for speed.
// Uses LZ4_FAST compression with smaller chunks for faster access.
func FastCompressingCodec() *CompressingCodec {
	return NewCompressingCodec(CompressionModeLZ4Fast, 8*1024, 64)
}

// HighCompressionCompressingCodec creates a CompressingCodec optimized for compression ratio.
// Uses DEFLATE compression with larger chunks for better compression.
func HighCompressionCompressingCodec() *CompressingCodec {
	return NewCompressingCodec(CompressionModeDeflate, 64*1024, 256)
}

// Name returns the name of this codec.
func (c *CompressingCodec) Name() string {
	return c.BaseCodec.Name()
}

// CompressionMode returns the compression mode used by this codec.
func (c *CompressingCodec) CompressionMode() CompressionMode {
	return c.compressionMode
}

// ChunkSize returns the chunk size in bytes.
func (c *CompressingCodec) ChunkSize() int {
	return c.chunkSize
}

// MaxDocsPerChunk returns the maximum number of documents per chunk.
func (c *CompressingCodec) MaxDocsPerChunk() int {
	return c.maxDocsPerChunk
}

// StoredFieldsFormat returns the stored fields format.
func (c *CompressingCodec) StoredFieldsFormat() StoredFieldsFormat {
	return c.storedFieldsFormat
}

// TermVectorsFormat returns the term vectors format.
func (c *CompressingCodec) TermVectorsFormat() TermVectorsFormat {
	return c.termVectorsFormat
}

// FieldInfosFormat returns the field infos format.
func (c *CompressingCodec) FieldInfosFormat() FieldInfosFormat {
	return c.fieldInfosFormat
}

// PostingsFormat returns the postings format.
func (c *CompressingCodec) PostingsFormat() PostingsFormat {
	return c.postingsFormat
}

// DocValuesFormat returns the doc values format.
func (c *CompressingCodec) DocValuesFormat() DocValuesFormat {
	return c.docValuesFormat
}

// NormsFormat returns the norms format.
func (c *CompressingCodec) NormsFormat() NormsFormat {
	return c.normsFormat
}

// LiveDocsFormat returns the live docs format.
func (c *CompressingCodec) LiveDocsFormat() LiveDocsFormat {
	return c.liveDocsFormat
}

// PointsFormat returns the points format.
func (c *CompressingCodec) PointsFormat() PointsFormat {
	return c.pointsFormat
}

// CompressingCodecFactory creates and manages CompressingCodec instances.
type CompressingCodecFactory struct {
	codecs map[string]*CompressingCodec
	mu     sync.RWMutex
}

// NewCompressingCodecFactory creates a new CompressingCodecFactory.
func NewCompressingCodecFactory() *CompressingCodecFactory {
	return &CompressingCodecFactory{
		codecs: make(map[string]*CompressingCodec),
	}
}

// GetOrCreate returns an existing codec or creates a new one with the given settings.
func (f *CompressingCodecFactory) GetOrCreate(name string, mode CompressionMode, chunkSize, maxDocsPerChunk int) *CompressingCodec {
	f.mu.Lock()
	defer f.mu.Unlock()

	if codec, ok := f.codecs[name]; ok {
		return codec
	}

	codec := NewCompressingCodec(mode, chunkSize, maxDocsPerChunk)
	f.codecs[name] = codec
	return codec
}

// Get returns a codec by name.
func (f *CompressingCodecFactory) Get(name string) (*CompressingCodec, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if codec, ok := f.codecs[name]; ok {
		return codec, nil
	}

	return nil, fmt.Errorf("codec '%s' not found", name)
}

// Register registers a codec with the given name.
func (f *CompressingCodecFactory) Register(name string, codec *CompressingCodec) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.codecs[name] = codec
}

// AvailableCodecs returns a list of registered codec names.
func (f *CompressingCodecFactory) AvailableCodecs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.codecs))
	for name := range f.codecs {
		names = append(names, name)
	}
	return names
}

// Global CompressingCodecFactory instance.
var defaultCompressingCodecFactory = NewCompressingCodecFactory()

// RegisterCompressingCodec registers a CompressingCodec with the global registry.
func RegisterCompressingCodec(name string, codec *CompressingCodec) {
	defaultCompressingCodecFactory.Register(name, codec)
}

// GetCompressingCodec returns a CompressingCodec from the global registry.
func GetCompressingCodec(name string) (*CompressingCodec, error) {
	return defaultCompressingCodecFactory.Get(name)
}

// init registers the default CompressingCodec instances.
func init() {
	// Register default compressing codecs
	RegisterCompressingCodec("Compressing", DefaultCompressingCodec())
	RegisterCompressingCodec("CompressingFast", FastCompressingCodec())
	RegisterCompressingCodec("CompressingHighCompression", HighCompressionCompressingCodec())
}
