// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// UniformSplitTermsReaderOverrides is the set of protected
// UniformSplitTermsReader methods that Apache Lucene 10.5.0 subclasses
// override and that UniformSplitTermsReader's own constructor then invokes on
// `this`: fillFieldMap (UniformSplitTermsReader.java:169, 177).
//
// Java resolves that call virtually, on a `this` whose fields are already
// assigned. A Go subclass cannot yet hold a pointer to the embedded base while
// the base constructor is still running, so FillFieldMap takes that base as
// its first parameter: `super` renders the Java `this`, exactly as
// AutomatonNextTermCalculator.parent renders the implicit
// IntersectBlockReader.this.
//
// FillFieldMap is declared `protected` by
// org.apache.lucene.codecs.uniformsplit.UniformSplitTermsReader, so its
// exported Go spelling is the rendering of `protected`: reachable by
// subclasses that live in another package, exactly as
// org.apache.lucene.codecs.uniformsplit.sharedterms.STUniformSplitTermsReader
// overrides it.
type UniformSplitTermsReaderOverrides interface {
	// FillFieldMap mirrors UniformSplitTermsReader.fillFieldMap
	// (UniformSplitTermsReader.java:177).
	FillFieldMap(
		super *UniformSplitTermsReader,
		postingsReader codecs.PostingsReaderBase,
		state *index.SegmentReadState,
		blockDecoder BlockDecoder,
		dictionaryOnHeap bool,
		dictionaryInput store.IndexInput,
		blockInput store.IndexInput,
		fieldMetadataCollection []*FieldMetadata,
		fieldInfos *index.FieldInfos,
	) error
}

// UniformSplitTermsReader is a block-based terms index and dictionary based on
// the Uniform Split technique.
//
// See UniformSplitTermsWriter.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.UniformSplitTermsReader from
// Apache Lucene 10.5.0, which extends FieldsProducer.
type UniformSplitTermsReader struct {
	PostingsReader  codecs.PostingsReaderBase
	Version         int32
	BlockInput      store.IndexInput
	DictionaryInput store.IndexInput

	FieldToTermsMap map[string]UniformSplitTermsBase
	// sortedFieldNames keeps the order of the field names; much more efficient
	// than having a TreeMap for the fieldToTermsMap.
	SortedFieldNames []string
}

// NewUniformSplitTermsReader mirrors the public UniformSplitTermsReader
// constructor (UniformSplitTermsReader.java:74).
//
// blockDecoder is an optional block decoder, may be nil if none. It can be used
// for decompression or decryption.
//
// dictionaryOnHeap tells whether to force loading the terms dictionary on-heap.
// By default it is kept off-heap without impact on performance. If block
// encoding/decoding is used, then the dictionary is always loaded on-heap
// whatever this parameter value is.
func NewUniformSplitTermsReader(
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
) (*UniformSplitTermsReader, error) {
	return NewUniformSplitTermsReaderWithCodec(
		nil,
		postingsReader,
		state,
		blockDecoder,
		dictionaryOnHeap,
		FieldMetadataSerializerInstance,
		Name,
		VersionStart,
		VersionCurrent,
		TermsBlocksExtension,
		TermsDictionaryExtension)
}

// NewUniformSplitTermsReaderWithCodec mirrors the protected
// UniformSplitTermsReader constructor that takes the codec identity
// (UniformSplitTermsReader.java:96). Go has no overloading, so the constructors
// are distinguished by name.
func NewUniformSplitTermsReaderWithCodec(
	overrides UniformSplitTermsReaderOverrides,
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
	fieldMetadataReader *FieldMetadataSerializer,
	codecName string,
	versionStart int32,
	versionCurrent int32,
	termsBlocksExtension string,
	dictionaryExtension string,
) (*UniformSplitTermsReader, error) {
	var dictionaryInput store.IndexInput
	var blockInput store.IndexInput
	success := false
	defer func() {
		if !success {
			util.CloseAllWhileHandlingException(blockInput, dictionaryInput)
		}
	}()

	r := &UniformSplitTermsReader{PostingsReader: postingsReader}

	segmentName := state.SegmentInfo.Name()
	termsName := index.SegmentFileName(segmentName, state.SegmentSuffix, termsBlocksExtension)
	blockInput, err := state.Directory.OpenInput(termsName, state.Context)
	if err != nil {
		return nil, err
	}

	r.Version, err = codecs.CheckIndexHeader(
		blockInput,
		codecName,
		versionStart,
		versionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix)
	if err != nil {
		return nil, err
	}

	indexName := index.SegmentFileName(segmentName, state.SegmentSuffix, dictionaryExtension)
	dictionaryInput, err = state.Directory.OpenInput(indexName, state.Context)
	if err != nil {
		return nil, err
	}

	if _, err := codecs.CheckIndexHeader(
		dictionaryInput,
		codecName,
		r.Version,
		r.Version,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix); err != nil {
		return nil, err
	}
	if _, err := codecs.ChecksumEntireFile(dictionaryInput); err != nil {
		return nil, err
	}

	if err := postingsReader.Init(blockInput, state); err != nil {
		return nil, err
	}
	if _, err := codecs.RetrieveChecksum(blockInput); err != nil {
		return nil, err
	}

	if err := r.SeekFieldsMetadata(blockInput); err != nil {
		return nil, err
	}
	fieldMetadataCollection, err := r.ReadFieldsMetadata(
		blockInput,
		blockDecoder,
		state.FieldInfos,
		fieldMetadataReader,
		int32(state.SegmentInfo.MaxDoc()))
	if err != nil {
		return nil, err
	}

	r.FieldToTermsMap = make(map[string]UniformSplitTermsBase)
	r.BlockInput = blockInput
	r.DictionaryInput = dictionaryInput

	if overrides == nil {
		overrides = r
	}
	if err := overrides.FillFieldMap(
		r,
		postingsReader,
		state,
		blockDecoder,
		dictionaryOnHeap,
		dictionaryInput,
		blockInput,
		fieldMetadataCollection,
		state.FieldInfos); err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, len(r.FieldToTermsMap))
	for name := range r.FieldToTermsMap {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	r.SortedFieldNames = fieldNames

	success = true
	return r, nil
}

// FillFieldMap mirrors UniformSplitTermsReader.fillFieldMap
// (UniformSplitTermsReader.java:177).
func (r *UniformSplitTermsReader) FillFieldMap(
	super *UniformSplitTermsReader,
	postingsReader codecs.PostingsReaderBase,
	state *index.SegmentReadState,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
	dictionaryInput store.IndexInput,
	blockInput store.IndexInput,
	fieldMetadataCollection []*FieldMetadata,
	fieldInfos *index.FieldInfos,
) error {
	for _, fieldMetadata := range fieldMetadataCollection {
		dictionaryBrowserSupplier, err := super.CreateDictionaryBrowserSupplier(
			state, dictionaryInput, fieldMetadata, blockDecoder, dictionaryOnHeap)
		if err != nil {
			return err
		}
		super.FieldToTermsMap[fieldMetadata.GetFieldInfo().Name()] = NewUniformSplitTerms(
			blockInput, fieldMetadata, postingsReader, blockDecoder, dictionaryBrowserSupplier)
	}
	return nil
}

// CreateDictionaryBrowserSupplier mirrors
// UniformSplitTermsReader.createDictionaryBrowserSupplier
// (UniformSplitTermsReader.java:198).
func (r *UniformSplitTermsReader) CreateDictionaryBrowserSupplier(
	state *index.SegmentReadState,
	dictionaryInput store.IndexInput,
	fieldMetadata *FieldMetadata,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
) (IndexDictionaryBrowserSupplier, error) {
	return NewFSTDictionaryBrowserSupplier(
		dictionaryInput, fieldMetadata.GetDictionaryStartFP(), blockDecoder, dictionaryOnHeap)
}

// ReadFieldsMetadata reads the fields metadata.
//
// indexInput must be positioned to the fields metadata details by calling
// seekFieldsMetadata before this call. blockDecoder is an optional block
// decoder, may be nil if none.
//
// Mirrors UniformSplitTermsReader.readFieldsMetadata
// (UniformSplitTermsReader.java:214). Java returns a
// Collection<FieldMetadata>; Go carries the same sequence as a slice.
func (r *UniformSplitTermsReader) ReadFieldsMetadata(
	indexInput store.IndexInput,
	blockDecoder BlockDecoder,
	fieldInfos *index.FieldInfos,
	fieldMetadataReader *FieldMetadataSerializer,
	maxNumDocs int32,
) ([]*FieldMetadata, error) {
	numFields, err := indexInput.ReadVInt()
	if err != nil {
		return nil, err
	}
	if numFields < 0 {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal number of fields= %d", numFields), fmt.Sprint(indexInput))
	}
	if blockDecoder != nil && r.Version >= VersionEncodableFieldsMetadata {
		return r.ReadEncodedFieldsMetadata(
			numFields, indexInput, blockDecoder, fieldInfos, fieldMetadataReader, maxNumDocs)
	}
	return r.ReadUnencodedFieldsMetadata(numFields, indexInput, fieldInfos, fieldMetadataReader, maxNumDocs)
}

// ReadEncodedFieldsMetadata mirrors
// UniformSplitTermsReader.readEncodedFieldsMetadata
// (UniformSplitTermsReader.java:232).
func (r *UniformSplitTermsReader) ReadEncodedFieldsMetadata(
	numFields int32,
	metadataInput store.DataInput,
	blockDecoder BlockDecoder,
	fieldInfos *index.FieldInfos,
	fieldMetadataReader *FieldMetadataSerializer,
	maxNumDocs int32,
) ([]*FieldMetadata, error) {
	encodedLength, err := metadataInput.ReadVLong()
	if err != nil {
		return nil, err
	}
	if encodedLength < 0 {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal encoded length: %d", encodedLength), fmt.Sprint(metadataInput))
	}
	decodedBytes, err := blockDecoder.Decode(metadataInput, encodedLength)
	if err != nil {
		return nil, err
	}
	decodedMetadataInput := store.NewByteArrayDataInputWithOffset(decodedBytes.Bytes, 0, decodedBytes.Length)
	return r.ReadUnencodedFieldsMetadata(
		numFields, decodedMetadataInput, fieldInfos, fieldMetadataReader, maxNumDocs)
}

// ReadUnencodedFieldsMetadata mirrors
// UniformSplitTermsReader.readUnencodedFieldsMetadata
// (UniformSplitTermsReader.java:251).
func (r *UniformSplitTermsReader) ReadUnencodedFieldsMetadata(
	numFields int32,
	metadataInput store.DataInput,
	fieldInfos *index.FieldInfos,
	fieldMetadataReader *FieldMetadataSerializer,
	maxNumDocs int32,
) ([]*FieldMetadata, error) {
	fieldMetadataCollection := make([]*FieldMetadata, 0, numFields)
	for i := int32(0); i < numFields; i++ {
		fieldMetadata, err := fieldMetadataReader.Read(metadataInput, fieldInfos, maxNumDocs)
		if err != nil {
			return nil, err
		}
		fieldMetadataCollection = append(fieldMetadataCollection, fieldMetadata)
	}
	return fieldMetadataCollection, nil
}

// Close mirrors UniformSplitTermsReader.close
// (UniformSplitTermsReader.java:265).
func (r *UniformSplitTermsReader) Close() error {
	err := util.CloseAll(r.BlockInput, r.DictionaryInput, r.PostingsReader)
	// Clear so refs to terms index is GCable even if app hangs onto us.
	clear(r.FieldToTermsMap)
	return err
}

// CheckIntegrity mirrors UniformSplitTermsReader.checkIntegrity
// (UniformSplitTermsReader.java:275).
func (r *UniformSplitTermsReader) CheckIntegrity() error {
	// term dictionary
	if _, err := codecs.ChecksumEntireFile(r.BlockInput); err != nil {
		return err
	}

	// postings
	return r.PostingsReader.CheckIntegrity()
}

// Iterator mirrors UniformSplitTermsReader.iterator
// (UniformSplitTermsReader.java:284), which walks the sorted field names.
func (r *UniformSplitTermsReader) Iterator() (spi.FieldIterator, error) {
	return spi.NewMemoryFieldIterator(r.SortedFieldNames), nil
}

// Terms mirrors UniformSplitTermsReader.terms
// (UniformSplitTermsReader.java:289): `return fieldToTermsMap.get(field)`.
func (r *UniformSplitTermsReader) Terms(field string) (spi.Terms, error) {
	if terms, ok := r.FieldToTermsMap[field]; ok {
		return terms, nil
	}
	return nil, nil
}

// Size mirrors UniformSplitTermsReader.size
// (UniformSplitTermsReader.java:294).
func (r *UniformSplitTermsReader) Size() int {
	return len(r.FieldToTermsMap)
}

// GetMergeInstance returns the receiver: UniformSplitTermsReader does not
// override FieldsProducer.getMergeInstance(), whose default returns `this`.
func (r *UniformSplitTermsReader) GetMergeInstance() spi.FieldsProducer {
	return r
}

// SeekFieldsMetadata positions the given IndexInput at the beginning of the
// fields metadata.
//
// Mirrors UniformSplitTermsReader.seekFieldsMetadata
// (UniformSplitTermsReader.java:299).
func (r *UniformSplitTermsReader) SeekFieldsMetadata(indexInput store.IndexInput) error {
	if err := indexInput.SetPosition(indexInput.Length() - int64(codecs.FooterLength()) - 8); err != nil {
		return err
	}
	offset, err := indexInput.ReadLong()
	if err != nil {
		return err
	}
	return indexInput.SetPosition(offset)
}

var (
	_ spi.FieldsProducer               = (*UniformSplitTermsReader)(nil)
	_ UniformSplitTermsReaderOverrides = (*UniformSplitTermsReader)(nil)
)
