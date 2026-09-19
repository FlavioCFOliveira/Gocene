// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsDictionaryExtension is the extension of the file containing the terms
// dictionary (the FST "trie").
//
// Mirrors UniformSplitPostingsFormat.TERMS_DICTIONARY_EXTENSION
// (UniformSplitPostingsFormat.java:41).
const TermsDictionaryExtension = "ustd"

// TermsBlocksExtension is the extension of the file containing the terms blocks
// for each field and the fields metadata.
//
// Mirrors UniformSplitPostingsFormat.TERMS_BLOCKS_EXTENSION
// (UniformSplitPostingsFormat.java:44).
const TermsBlocksExtension = "ustb"

// Versions of the UniformSplit postings format.
//
// Mirrors UniformSplitPostingsFormat.VERSION_START,
// VERSION_ENCODABLE_FIELDS_METADATA and VERSION_CURRENT
// (UniformSplitPostingsFormat.java:46-48).
const (
	// VersionStart is the first version of the format.
	VersionStart = 0
	// VersionEncodableFieldsMetadata is the version that added the encodable
	// fields metadata.
	VersionEncodableFieldsMetadata = 1
	// VersionCurrent is the version written by this release.
	VersionCurrent = VersionEncodableFieldsMetadata
)

// Name is the codec name of this postings format.
//
// Mirrors UniformSplitPostingsFormat.NAME
// (UniformSplitPostingsFormat.java:50).
const Name = "UniformSplit"

// UniformSplitPostingsFormat is the PostingsFormat based on the Uniform Split
// technique.
//
// See UniformSplitTermsWriter.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.UniformSplitPostingsFormat from
// Apache Lucene 10.5.0, which extends PostingsFormat.
type UniformSplitPostingsFormat struct {
	// name renders the name that Java's PostingsFormat(String) superclass
	// constructor stores and that PostingsFormat.getName() returns
	// (UniformSplitPostingsFormat.java:104). Gocene's spi.PostingsFormat is an
	// interface with no state, so the field lives here.
	name string

	targetNumBlockLines int
	deltaNumLines       int
	blockEncoder        BlockEncoder
	blockDecoder        BlockDecoder
	dictionaryOnHeap    bool
}

// NewUniformSplitPostingsFormat creates a UniformSplitPostingsFormat with
// default settings.
//
// Mirrors the public no-argument UniformSplitPostingsFormat() constructor
// (UniformSplitPostingsFormat.java:59).
func NewUniformSplitPostingsFormat() (*UniformSplitPostingsFormat, error) {
	return NewUniformSplitPostingsFormatWithSettings(
		DefaultTargetNumBlockLines,
		DefaultDeltaNumLines,
		nil,
		nil,
		false)
}

// NewUniformSplitPostingsFormatWithSettings mirrors the public
// UniformSplitPostingsFormat(int, int, BlockEncoder, BlockDecoder, boolean)
// constructor (UniformSplitPostingsFormat.java:85). Go has no overloading, so
// the constructors are distinguished by name.
//
// targetNumBlockLines is the target number of lines per block. It must be
// strictly greater than 0. The parameters can be pre-validated with
// ValidateSettings. There is one term per block line, with its corresponding
// details (index.TermState).
//
// deltaNumLines is the maximum allowed delta variation of the number of lines
// per block. It must be greater than or equal to 0 and strictly less than
// targetNumBlockLines. The block size will be targetNumBlockLines +-
// deltaNumLines. The block size must always be less than or equal to
// MaxNumBlockLines.
//
// blockEncoder is an optional block encoder, may be nil if none. If present, it
// is used to encode all terms blocks, as well as the FST dictionary and the
// fields metadata.
//
// blockDecoder is an optional block decoder, may be nil if none. If present, it
// is used to decode all terms blocks, as well as the FST dictionary and the
// fields metadata.
//
// dictionaryOnHeap tells whether to force loading the terms dictionary on-heap.
// By default it is kept off-heap without impact on performance. If block
// encoding/decoding is used, then the dictionary is always loaded on-heap
// whatever this parameter value is.
func NewUniformSplitPostingsFormatWithSettings(
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
) (*UniformSplitPostingsFormat, error) {
	return NewUniformSplitPostingsFormatWithName(
		Name, targetNumBlockLines, deltaNumLines, blockEncoder, blockDecoder, dictionaryOnHeap)
}

// NewUniformSplitPostingsFormatWithName mirrors the protected
// UniformSplitPostingsFormat(String, int, int, BlockEncoder, BlockDecoder,
// boolean) constructor (UniformSplitPostingsFormat.java:97).
func NewUniformSplitPostingsFormatWithName(
	name string,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
) (*UniformSplitPostingsFormat, error) {
	if err := ValidateSettings(targetNumBlockLines, deltaNumLines); err != nil {
		return nil, err
	}
	if err := validateBlockEncoder(blockEncoder, blockDecoder); err != nil {
		return nil, err
	}
	return &UniformSplitPostingsFormat{
		name:                name,
		targetNumBlockLines: targetNumBlockLines,
		deltaNumLines:       deltaNumLines,
		blockEncoder:        blockEncoder,
		blockDecoder:        blockDecoder,
		dictionaryOnHeap:    dictionaryOnHeap,
	}, nil
}

// Name renders PostingsFormat.getName(), which returns the name handed to the
// PostingsFormat(String) superclass constructor
// (UniformSplitPostingsFormat.java:104).
func (f *UniformSplitPostingsFormat) Name() string {
	return f.name
}

// FieldsConsumer mirrors UniformSplitPostingsFormat.fieldsConsumer
// (UniformSplitPostingsFormat.java:115).
func (f *UniformSplitPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	postingsWriter, err := codecs.NewLucene104PostingsWriter(state)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			util.CloseAllWhileHandlingException(postingsWriter)
		}
	}()
	termsWriter, err := f.createUniformSplitTermsWriter(
		postingsWriter, state, f.targetNumBlockLines, f.deltaNumLines, f.blockEncoder)
	if err != nil {
		return nil, err
	}
	success = true
	return termsWriter, nil
}

// FieldsProducer mirrors UniformSplitPostingsFormat.fieldsProducer
// (UniformSplitPostingsFormat.java:132).
func (f *UniformSplitPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	postingsReader, err := codecs.NewLucene104PostingsReader(state)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			util.CloseAllWhileHandlingException(postingsReader)
		}
	}()
	termsReader, err := f.createUniformSplitTermsReader(postingsReader, state, f.blockDecoder)
	if err != nil {
		return nil, err
	}
	success = true
	return termsReader, nil
}

// createUniformSplitTermsWriter mirrors
// UniformSplitPostingsFormat.createUniformSplitTermsWriter
// (UniformSplitPostingsFormat.java:147).
func (f *UniformSplitPostingsFormat) createUniformSplitTermsWriter(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
) (spi.FieldsConsumer, error) {
	return NewUniformSplitTermsWriterWithBlockSizes(
		postingsWriter, state, targetNumBlockLines, deltaNumLines, blockEncoder)
}

// createUniformSplitTermsReader mirrors
// UniformSplitPostingsFormat.createUniformSplitTermsReader
// (UniformSplitPostingsFormat.java:158).
func (f *UniformSplitPostingsFormat) createUniformSplitTermsReader(
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder BlockDecoder,
) (spi.FieldsProducer, error) {
	return NewUniformSplitTermsReader(postingsReader, state, blockDecoder, f.dictionaryOnHeap)
}

// validateBlockEncoder mirrors the private static
// UniformSplitPostingsFormat.validateBlockEncoder
// (UniformSplitPostingsFormat.java:164), which throws
// IllegalArgumentException; Gocene reports it as an error.
func validateBlockEncoder(blockEncoder BlockEncoder, blockDecoder BlockDecoder) error {
	if (blockEncoder != nil && blockDecoder == nil) || (blockEncoder == nil && blockDecoder != nil) {
		return fmt.Errorf("invalid blockEncoder=%v and blockDecoder=%v, both must be null or both must be non-null",
			blockEncoder, blockDecoder)
	}
	return nil
}

var _ spi.PostingsFormat = (*UniformSplitPostingsFormat)(nil)
