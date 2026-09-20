// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// STUniformSplitTermsReader is a block-based terms index and dictionary based
// on the Uniform Split technique, and sharing all the fields terms in the same
// dictionary, with all the fields of a term in the same block line.
//
// See STUniformSplitTermsWriter.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STUniformSplitTermsReader
// from Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.UniformSplitTermsReader.
type STUniformSplitTermsReader struct {
	*uniformsplit.UniformSplitTermsReader
}

// NewSTUniformSplitTermsReader mirrors the public
// STUniformSplitTermsReader(PostingsReaderBase, SegmentReadState,
// BlockDecoder, boolean) constructor (STUniformSplitTermsReader.java:48).
func NewSTUniformSplitTermsReader(
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
) (*STUniformSplitTermsReader, error) {
	return NewSTUniformSplitTermsReaderWithCodec(
		postingsReader,
		state,
		blockDecoder,
		dictionaryOnHeap,
		uniformsplit.FieldMetadataSerializerInstance,
		Name,
		uniformsplit.VersionStart,
		VersionCurrent,
		TermsBlocksExtension,
		TermsDictionaryExtension)
}

// NewSTUniformSplitTermsReaderWithCodec mirrors the protected
// STUniformSplitTermsReader constructor that takes the codec identity
// (STUniformSplitTermsReader.java:70). Go has no overloading, so the
// constructors are distinguished by name.
func NewSTUniformSplitTermsReaderWithCodec(
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
	fieldMetadataReader *uniformsplit.FieldMetadataSerializer,
	codecName string,
	versionStart int32,
	versionCurrent int32,
	termsBlocksExtension string,
	dictionaryExtension string,
) (*STUniformSplitTermsReader, error) {
	// Java's `this` inside UniformSplitTermsReader's constructor, on which
	// fillFieldMap is resolved, is the STUniformSplitTermsReader being
	// constructed. Go has no subclass instance while the base constructor
	// runs, so the (still empty) receiver is handed over as the override
	// target and the base it receives is the `super` parameter of
	// FillFieldMap; see uniformsplit.UniformSplitTermsReaderOverrides.
	r := &STUniformSplitTermsReader{}
	base, err := uniformsplit.NewUniformSplitTermsReaderWithCodec(
		r,
		postingsReader,
		state,
		blockDecoder,
		dictionaryOnHeap,
		fieldMetadataReader,
		codecName,
		versionStart,
		versionCurrent,
		termsBlocksExtension,
		dictionaryExtension)
	if err != nil {
		return nil, err
	}
	r.UniformSplitTermsReader = base
	return r, nil
}

// FillFieldMap mirrors STUniformSplitTermsReader.fillFieldMap
// (STUniformSplitTermsReader.java:96).
func (r *STUniformSplitTermsReader) FillFieldMap(
	super *uniformsplit.UniformSplitTermsReader,
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
	dictionaryInput store.IndexInput,
	blockInput store.IndexInput,
	fieldMetadataCollection []*uniformsplit.FieldMetadata,
	fieldInfos *index.FieldInfos,
) error {
	if len(fieldMetadataCollection) != 0 {
		unionFieldMetadata, err := r.CreateUnionFieldMetadata(fieldMetadataCollection)
		if err != nil {
			return err
		}
		// Share the same immutable dictionary between all fields.
		dictionaryBrowserSupplier, err := super.CreateDictionaryBrowserSupplier(
			state, dictionaryInput, unionFieldMetadata, blockDecoder, dictionaryOnHeap)
		if err != nil {
			return err
		}
		for _, fieldMetadata := range fieldMetadataCollection {
			super.FieldToTermsMap[fieldMetadata.GetFieldInfo().Name()] = NewSTUniformSplitTerms(
				blockInput,
				fieldMetadata,
				unionFieldMetadata,
				postingsReader,
				blockDecoder,
				fieldInfos,
				dictionaryBrowserSupplier)
		}
	}
	return nil
}

// CreateUnionFieldMetadata creates a virtual uniformsplit.FieldMetadata that
// is the union of the given uniformsplit.FieldMetadata. Its
// GetFirstBlockStartFP, GetLastBlockStartFP and GetLastTerm are respectively
// the min and max among the uniformsplit.FieldMetadata provided as parameter.
//
// Mirrors the protected
// STUniformSplitTermsReader.createUnionFieldMetadata(Iterable<FieldMetadata>)
// (STUniformSplitTermsReader.java:134). Java returns the built FieldMetadata
// directly; UnionFieldMetadataBuilder.Build reports the
// IllegalStateException of an empty build as an error, so the error is
// propagated here.
func (r *STUniformSplitTermsReader) CreateUnionFieldMetadata(
	fieldMetadataIterable []*uniformsplit.FieldMetadata,
) (*uniformsplit.FieldMetadata, error) {
	builder := NewUnionFieldMetadataBuilder()
	for _, fieldMetadata := range fieldMetadataIterable {
		builder.AddFieldMetadata(fieldMetadata)
	}
	return builder.Build()
}

var (
	_ spi.FieldsProducer                            = (*STUniformSplitTermsReader)(nil)
	_ uniformsplit.UniformSplitTermsReaderOverrides = (*STUniformSplitTermsReader)(nil)
)
