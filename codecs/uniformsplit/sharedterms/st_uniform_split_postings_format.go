// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TermsDictionaryExtension is the extension of the file containing the terms
// dictionary (the FST "trie").
//
// Mirrors STUniformSplitPostingsFormat.TERMS_DICTIONARY_EXTENSION
// (STUniformSplitPostingsFormat.java:44).
const TermsDictionaryExtension = "stustd"

// TermsBlocksExtension is the extension of the file containing the terms
// blocks for each field and the fields metadata.
//
// Mirrors STUniformSplitPostingsFormat.TERMS_BLOCKS_EXTENSION
// (STUniformSplitPostingsFormat.java:47).
const TermsBlocksExtension = "stustb"

// VersionCurrent is the version written by this release.
//
// Mirrors STUniformSplitPostingsFormat.VERSION_CURRENT
// (STUniformSplitPostingsFormat.java:49), whose initialiser is
// UniformSplitPostingsFormat.VERSION_CURRENT.
const VersionCurrent = uniformsplit.VersionCurrent

// Name is the codec name of this postings format.
//
// Mirrors STUniformSplitPostingsFormat.NAME
// (STUniformSplitPostingsFormat.java:51).
const Name = "SharedTermsUniformSplit"

// STUniformSplitPostingsFormat is the spi.PostingsFormat based on the Uniform
// Split technique and supporting Shared Terms.
//
// Shared Terms means the terms of all fields are stored in the same block
// file, with multiple fields associated to one term (one block line). In the
// same way, the dictionary trie is also shared between all fields. This highly
// reduces the memory required by the field dictionary compared to having one
// separate dictionary per field.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STUniformSplitPostingsFormat
// from Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.UniformSplitPostingsFormat.
type STUniformSplitPostingsFormat struct {
	*uniformsplit.UniformSplitPostingsFormat
}

// NewSTUniformSplitPostingsFormat creates a STUniformSplitPostingsFormat with
// default settings.
//
// Mirrors the public no-argument STUniformSplitPostingsFormat() constructor
// (STUniformSplitPostingsFormat.java:54).
func NewSTUniformSplitPostingsFormat() (*STUniformSplitPostingsFormat, error) {
	return NewSTUniformSplitPostingsFormatWithSettings(
		uniformsplit.DefaultTargetNumBlockLines,
		uniformsplit.DefaultDeltaNumLines,
		nil,
		nil,
		false)
}

// NewSTUniformSplitPostingsFormatWithSettings mirrors the public
// STUniformSplitPostingsFormat(int, int, BlockEncoder, BlockDecoder, boolean)
// constructor (STUniformSplitPostingsFormat.java:68). Go has no overloading,
// so the constructors are distinguished by name.
//
// See uniformsplit.NewUniformSplitPostingsFormatWithSettings for the meaning
// of each parameter.
func NewSTUniformSplitPostingsFormatWithSettings(
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
) (*STUniformSplitPostingsFormat, error) {
	return NewSTUniformSplitPostingsFormatWithName(
		Name, targetNumBlockLines, deltaNumLines, blockEncoder, blockDecoder, dictionaryOnHeap)
}

// NewSTUniformSplitPostingsFormatWithName mirrors the protected
// STUniformSplitPostingsFormat(String, int, int, BlockEncoder, BlockDecoder,
// boolean) constructor (STUniformSplitPostingsFormat.java:77).
func NewSTUniformSplitPostingsFormatWithName(
	name string,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
) (*STUniformSplitPostingsFormat, error) {
	base, err := uniformsplit.NewUniformSplitPostingsFormatWithName(
		name, targetNumBlockLines, deltaNumLines, blockEncoder, blockDecoder, dictionaryOnHeap)
	if err != nil {
		return nil, err
	}
	f := &STUniformSplitPostingsFormat{UniformSplitPostingsFormat: base}
	// Java's `this` inside UniformSplitPostingsFormat.fieldsConsumer and
	// fieldsProducer is the STUniformSplitPostingsFormat being constructed;
	// see uniformsplit.UniformSplitPostingsFormatOverrides.
	f.Overrides = f
	return f, nil
}

// CreateUniformSplitTermsWriter mirrors
// STUniformSplitPostingsFormat.createUniformSplitTermsWriter
// (STUniformSplitPostingsFormat.java:87).
func (f *STUniformSplitPostingsFormat) CreateUniformSplitTermsWriter(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
) (spi.FieldsConsumer, error) {
	return NewSTUniformSplitTermsWriterWithBlockSizes(
		postingsWriter, state, targetNumBlockLines, deltaNumLines, blockEncoder)
}

// CreateUniformSplitTermsReader mirrors
// STUniformSplitPostingsFormat.createUniformSplitTermsReader
// (STUniformSplitPostingsFormat.java:99).
func (f *STUniformSplitPostingsFormat) CreateUniformSplitTermsReader(
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
) (spi.FieldsProducer, error) {
	return NewSTUniformSplitTermsReader(postingsReader, state, blockDecoder, f.DictionaryOnHeap)
}

var (
	_ spi.PostingsFormat                               = (*STUniformSplitPostingsFormat)(nil)
	_ uniformsplit.UniformSplitPostingsFormatOverrides = (*STUniformSplitPostingsFormat)(nil)
)
