// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"
)

// CompressingCodec is a codec that compresses stored fields and term vectors.
//
// This is the Go port of Lucene's CompressingCodec.
// It uses CompressingStoredFieldsFormat and CompressingTermVectorsFormat
// to compress data using configurable compression modes.
//
// The codec is byte-compatible with Apache Lucene's implementation.
type CompressingCodec struct {
	*BaseCodec
	storedFieldsFormat   StoredFieldsFormat
	termVectorsFormat    TermVectorsFormat
	fieldInfosFormat     FieldInfosFormat
	segmentInfosFormat   SegmentInfosFormat
	postingsFormat       PostingsFormat
	docValuesFormat      DocValuesFormat
	normsFormat          NormsFormat
	liveDocsFormat       LiveDocsFormat
	pointsFormat         PointsFormat
	compressionMode      CompressionMode
	chunkSize            int
	maxDocsPerChunk      int
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

	storedFieldsFormat := NewCompressingStoredFieldsFormat(mode, chunkSize, maxDocsPerChunk)
	termVectorsFormat := NewCompressingTermVectorsFormat(mode, chunkSize, maxDocsPerChunk)

	return &CompressingCodec{
		BaseCodec:          NewBaseCodec("CompressingCodec"),
		storedFieldsFormat: storedFieldsFormat,
		termVectorsFormat:  termVectorsFormat,
		fieldInfosFormat:   NewLucene104FieldInfosFormat(),
		segmentInfosFormat: NewLucene104SegmentInfosFormat(),
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

// SegmentInfosFormat returns the segment infos format.
func (c *CompressingCodec) SegmentInfosFormat() SegmentInfosFormat {
	return c.segmentInfosFormat
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
