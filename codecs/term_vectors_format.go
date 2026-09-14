// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TermVectorsFormat is an alias of spi.TermVectorsFormat.
type TermVectorsFormat = spi.TermVectorsFormat

// TermVectorsWriter is an alias of spi.TermVectorsWriter.
type TermVectorsWriter = spi.TermVectorsWriter

// TermVectorsReader is an alias of spi.TermVectorsReader.
type TermVectorsReader = spi.TermVectorsReader

// The term-vectors formats Apache Lucene 10.5.0 ships live in
// org.apache.lucene.codecs.lucene90 (Lucene90TermVectorsFormat) and
// org.apache.lucene.codecs.lucene90.compressing
// (Lucene90CompressingTermVectorsFormat). Their Go ports sit in
// codecs/lucene90 and codecs/lucene90/compressing, which import this package,
// so the codecs that name them here (Lucene104Codec, CompressingCodec) reach
// their constructors through the init()-time registrations below, exactly as
// the stored-fields formats are reached (see stored_fields_format.go).

// lucene90TermVectorsFormatFactory is populated by package codecs/lucene90
// via init.
var lucene90TermVectorsFormatFactory func() TermVectorsFormat

// RegisterLucene90TermVectorsFormat sets the factory that builds
// codecs/lucene90.Lucene90TermVectorsFormat.
func RegisterLucene90TermVectorsFormat(factory func() TermVectorsFormat) {
	lucene90TermVectorsFormatFactory = factory
}

// Lucene90CompressingTermVectorsFormatOptions carries the constructor
// arguments of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90CompressingTermVectorsFormatOptions struct {
	FormatName      string
	SegmentSuffix   string
	CompressionMode CompressionMode
	ChunkSize       int
	MaxDocsPerChunk int
	BlockSize       int
}

// lucene90CompressingTermVectorsFormatFactory is populated by package
// codecs/lucene90/compressing via init.
var lucene90CompressingTermVectorsFormatFactory func(Lucene90CompressingTermVectorsFormatOptions) (TermVectorsFormat, error)

// RegisterLucene90CompressingTermVectorsFormat sets the factory that builds
// codecs/lucene90/compressing.Lucene90CompressingTermVectorsFormat.
func RegisterLucene90CompressingTermVectorsFormat(factory func(Lucene90CompressingTermVectorsFormatOptions) (TermVectorsFormat, error)) {
	lucene90CompressingTermVectorsFormatFactory = factory
}

// deferredTermVectorsFormat resolves its registered format on first use, so a
// codec constructed before the registering package's init() has run still
// reaches the real format.
type deferredTermVectorsFormat struct {
	name    string
	resolve func() (TermVectorsFormat, error)
}

func (f *deferredTermVectorsFormat) Name() string { return f.name }

func (f *deferredTermVectorsFormat) delegate() (TermVectorsFormat, error) {
	tv, err := f.resolve()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, fmt.Errorf("codecs: the registered factory produced no term-vectors format for %q", f.name)
	}
	return tv, nil
}

func (f *deferredTermVectorsFormat) VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error) {
	tv, err := f.delegate()
	if err != nil {
		return nil, err
	}
	return tv.VectorsWriter(state)
}

func (f *deferredTermVectorsFormat) VectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (TermVectorsReader, error) {
	tv, err := f.delegate()
	if err != nil {
		return nil, err
	}
	return tv.VectorsReader(dir, segmentInfo, fieldInfos, context)
}

var _ TermVectorsFormat = (*deferredTermVectorsFormat)(nil)

// NewLucene90TermVectorsFormat returns codecs/lucene90.Lucene90TermVectorsFormat,
// the format Lucene104Codec.termVectorsFormat() returns
// (new Lucene90TermVectorsFormat()).
func NewLucene90TermVectorsFormat() TermVectorsFormat {
	return &deferredTermVectorsFormat{
		name: "Lucene90TermVectorsFormat",
		resolve: func() (TermVectorsFormat, error) {
			if lucene90TermVectorsFormatFactory == nil {
				return nil, fmt.Errorf("codecs: no Lucene90 term-vectors format registered; import codecs/lucene90")
			}
			return lucene90TermVectorsFormatFactory(), nil
		},
	}
}

// NewLucene90CompressingTermVectorsFormat returns
// codecs/lucene90/compressing.Lucene90CompressingTermVectorsFormat built from
// opts.
func NewLucene90CompressingTermVectorsFormat(opts Lucene90CompressingTermVectorsFormatOptions) TermVectorsFormat {
	return &deferredTermVectorsFormat{
		name: opts.FormatName,
		resolve: func() (TermVectorsFormat, error) {
			if lucene90CompressingTermVectorsFormatFactory == nil {
				return nil, fmt.Errorf("codecs: no Lucene90CompressingTermVectorsFormat factory registered; import codecs/lucene90/compressing")
			}
			return lucene90CompressingTermVectorsFormatFactory(opts)
		},
	}
}
