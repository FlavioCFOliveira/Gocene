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

// UniformSplitTermsReader is a block-based terms index and dictionary based on
// the Uniform Split technique.
//
// See UniformSplitTermsWriter.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.UniformSplitTermsReader from
// Apache Lucene 10.5.0, which extends FieldsProducer.
type UniformSplitTermsReader struct {
	postingsReader  codecs.PostingsReaderBase
	version         int32
	blockInput      store.IndexInput
	dictionaryInput store.IndexInput

	fieldToTermsMap map[string]*UniformSplitTerms
	// sortedFieldNames keeps the order of the field names; much more efficient
	// than having a TreeMap for the fieldToTermsMap.
	sortedFieldNames []string
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

	r := &UniformSplitTermsReader{postingsReader: postingsReader}

	segmentName := state.SegmentInfo.Name()
	termsName := index.SegmentFileName(segmentName, state.SegmentSuffix, termsBlocksExtension)
	blockInput, err := state.Directory.OpenInput(termsName, state.Context)
	if err != nil {
		return nil, err
	}

	r.version, err = codecs.CheckIndexHeader(
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
		r.version,
		r.version,
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

	if err := r.seekFieldsMetadata(blockInput); err != nil {
		return nil, err
	}
	fieldMetadataCollection, err := r.readFieldsMetadata(
		blockInput,
		blockDecoder,
		state.FieldInfos,
		fieldMetadataReader,
		int32(state.SegmentInfo.MaxDoc()))
	if err != nil {
		return nil, err
	}

	r.fieldToTermsMap = make(map[string]*UniformSplitTerms)
	r.blockInput = blockInput
	r.dictionaryInput = dictionaryInput

	if err := r.fillFieldMap(
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

	fieldNames := make([]string, 0, len(r.fieldToTermsMap))
	for name := range r.fieldToTermsMap {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	r.sortedFieldNames = fieldNames

	success = true
	return r, nil
}

// fillFieldMap mirrors UniformSplitTermsReader.fillFieldMap
// (UniformSplitTermsReader.java:177).
func (r *UniformSplitTermsReader) fillFieldMap(
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
		dictionaryBrowserSupplier, err := r.createDictionaryBrowserSupplier(
			state, dictionaryInput, fieldMetadata, blockDecoder, dictionaryOnHeap)
		if err != nil {
			return err
		}
		r.fieldToTermsMap[fieldMetadata.GetFieldInfo().Name()] = NewUniformSplitTerms(
			blockInput, fieldMetadata, postingsReader, blockDecoder, dictionaryBrowserSupplier)
	}
	return nil
}

// createDictionaryBrowserSupplier mirrors
// UniformSplitTermsReader.createDictionaryBrowserSupplier
// (UniformSplitTermsReader.java:198).
func (r *UniformSplitTermsReader) createDictionaryBrowserSupplier(
	state *index.SegmentReadState,
	dictionaryInput store.IndexInput,
	fieldMetadata *FieldMetadata,
	blockDecoder BlockDecoder,
	dictionaryOnHeap bool,
) (IndexDictionaryBrowserSupplier, error) {
	return NewFSTDictionaryBrowserSupplier(
		dictionaryInput, fieldMetadata.GetDictionaryStartFP(), blockDecoder, dictionaryOnHeap)
}

// readFieldsMetadata reads the fields metadata.
//
// indexInput must be positioned to the fields metadata details by calling
// seekFieldsMetadata before this call. blockDecoder is an optional block
// decoder, may be nil if none.
//
// Mirrors UniformSplitTermsReader.readFieldsMetadata
// (UniformSplitTermsReader.java:214). Java returns a
// Collection<FieldMetadata>; Go carries the same sequence as a slice.
func (r *UniformSplitTermsReader) readFieldsMetadata(
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
	if blockDecoder != nil && r.version >= VersionEncodableFieldsMetadata {
		return r.readEncodedFieldsMetadata(
			numFields, indexInput, blockDecoder, fieldInfos, fieldMetadataReader, maxNumDocs)
	}
	return r.readUnencodedFieldsMetadata(numFields, indexInput, fieldInfos, fieldMetadataReader, maxNumDocs)
}

// readEncodedFieldsMetadata mirrors
// UniformSplitTermsReader.readEncodedFieldsMetadata
// (UniformSplitTermsReader.java:232).
func (r *UniformSplitTermsReader) readEncodedFieldsMetadata(
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
	return r.readUnencodedFieldsMetadata(
		numFields, decodedMetadataInput, fieldInfos, fieldMetadataReader, maxNumDocs)
}

// readUnencodedFieldsMetadata mirrors
// UniformSplitTermsReader.readUnencodedFieldsMetadata
// (UniformSplitTermsReader.java:251).
func (r *UniformSplitTermsReader) readUnencodedFieldsMetadata(
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
	err := util.CloseAll(r.blockInput, r.dictionaryInput, r.postingsReader)
	// Clear so refs to terms index is GCable even if app hangs onto us.
	clear(r.fieldToTermsMap)
	return err
}

// CheckIntegrity mirrors UniformSplitTermsReader.checkIntegrity
// (UniformSplitTermsReader.java:275).
func (r *UniformSplitTermsReader) CheckIntegrity() error {
	// term dictionary
	if _, err := codecs.ChecksumEntireFile(r.blockInput); err != nil {
		return err
	}

	// postings
	return r.postingsReader.CheckIntegrity()
}

// Iterator mirrors UniformSplitTermsReader.iterator
// (UniformSplitTermsReader.java:284), which walks the sorted field names.
func (r *UniformSplitTermsReader) Iterator() (spi.FieldIterator, error) {
	return spi.NewMemoryFieldIterator(r.sortedFieldNames), nil
}

// Terms mirrors UniformSplitTermsReader.terms
// (UniformSplitTermsReader.java:289): `return fieldToTermsMap.get(field)`.
func (r *UniformSplitTermsReader) Terms(field string) (spi.Terms, error) {
	if terms, ok := r.fieldToTermsMap[field]; ok {
		return terms, nil
	}
	return nil, nil
}

// Size mirrors UniformSplitTermsReader.size
// (UniformSplitTermsReader.java:294).
func (r *UniformSplitTermsReader) Size() int {
	return len(r.fieldToTermsMap)
}

// GetMergeInstance returns the receiver: UniformSplitTermsReader does not
// override FieldsProducer.getMergeInstance(), whose default returns `this`.
func (r *UniformSplitTermsReader) GetMergeInstance() spi.FieldsProducer {
	return r
}

// seekFieldsMetadata positions the given IndexInput at the beginning of the
// fields metadata.
//
// Mirrors UniformSplitTermsReader.seekFieldsMetadata
// (UniformSplitTermsReader.java:299).
func (r *UniformSplitTermsReader) seekFieldsMetadata(indexInput store.IndexInput) error {
	if err := indexInput.SetPosition(indexInput.Length() - int64(codecs.FooterLength()) - 8); err != nil {
		return err
	}
	offset, err := indexInput.ReadLong()
	if err != nil {
		return err
	}
	return indexInput.SetPosition(offset)
}

var _ spi.FieldsProducer = (*UniformSplitTermsReader)(nil)
