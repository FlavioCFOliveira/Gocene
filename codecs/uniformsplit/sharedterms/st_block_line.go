// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// STBlockLine represents a term and its details stored in the BlockTermState.
// It is an extension of uniformsplit.BlockLine for the Shared Terms format.
// This means the line contains a term and all its fields TermStates.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockLine from
// Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.BlockLine.
type STBlockLine struct {
	*uniformsplit.BlockLine

	// TermStates is the list of the fields and their TermStates for this
	// block line. Only used for writing.
	//
	// Mirrors the protected final field STBlockLine.termStates
	// (STBlockLine.java:45).
	TermStates []*FieldMetadataTermState
}

// NewSTBlockLine mirrors the public STBlockLine(TermBytes,
// List<FieldMetadataTermState>) constructor (STBlockLine.java:47), whose body
// is `super(termBytes, null)` followed by a defensive copy of the list.
func NewSTBlockLine(termBytes *uniformsplit.TermBytes, termStates []*FieldMetadataTermState) *STBlockLine {
	// assert !termStates.isEmpty();
	line := &STBlockLine{
		BlockLine:  uniformsplit.NewBlockLineForWriting(termBytes, nil),
		TermStates: append([]*FieldMetadataTermState(nil), termStates...),
	}
	// Java keeps one object; the Go base records the subclass so that
	// STBlockWriter.WriteBlockLine can perform the Java downcast
	// `(STBlockLine) line`. See uniformsplit.BlockLine.Derived.
	line.BlockLine.Derived = line
	return line
}

// CollectFields collects the uniformsplit.FieldMetadata of all fields listed
// in this line into collector.
//
// Mirrors STBlockLine.collectFields(Collection<FieldMetadata>)
// (STBlockLine.java:58). The only collector Lucene passes is
// STBlockWriter.fieldsInBlock, a HashSet<FieldMetadata> keyed by identity
// because FieldMetadata overrides neither equals nor hashCode; a Go map keyed
// by the pointer is that same set.
func (l *STBlockLine) CollectFields(collector map[*uniformsplit.FieldMetadata]struct{}) {
	for _, fieldTermState := range l.TermStates {
		collector[fieldTermState.FieldMetadata()] = struct{}{}
	}
}

// STBlockLineSerializer reads block lines encoded incrementally, with all
// fields corresponding to the term of the line.
//
// This type extends uniformsplit.BlockLineSerializer, so it keeps a state of
// the previous term read to decode the next term.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockLine.Serializer
// (STBlockLine.java:68). Go has no nested types, so the Java outer name is
// prefixed, exactly as uniformsplit.BlockLineSerializer renders
// BlockLine.Serializer.
type STBlockLineSerializer struct {
	*uniformsplit.BlockLineSerializer
}

// NewSTBlockLineSerializer mirrors the implicit no-argument
// STBlockLine.Serializer() constructor, which runs BlockLine.Serializer()
// (BlockLine.java:110).
func NewSTBlockLineSerializer() *STBlockLineSerializer {
	return &STBlockLineSerializer{BlockLineSerializer: uniformsplit.NewBlockLineSerializer()}
}

// WriteLineTermStates writes all the BlockTermState of the provided
// STBlockLine to the given output.
//
// Mirrors STBlockLine.Serializer.writeLineTermStates(DataOutput, STBlockLine,
// DeltaBaseTermStateSerializer) (STBlockLine.java:74).
func (s *STBlockLineSerializer) WriteLineTermStates(
	termStatesOutput store.DataOutput,
	line *STBlockLine,
	encoder *uniformsplit.DeltaBaseTermStateSerializer,
) error {
	var fieldMetadataTermState *FieldMetadataTermState
	size := len(line.TermStates)
	// assert size > 0 : "not valid block line with :" + size + " lines.";
	if size == 1 {
		// When there is only 1 field, write its id as negative, followed by
		// the field TermState.
		fieldID := line.TermStates[0].FieldMetadata().GetFieldInfo().Number()
		if err := termStatesOutput.WriteZInt(int32(-fieldID)); err != nil {
			return err
		}
		fieldMetadataTermState = line.TermStates[0]
		return encoder.WriteTermState(
			termStatesOutput,
			fieldMetadataTermState.FieldMetadata().GetFieldInfo(),
			fieldMetadataTermState.State())
	}

	if err := termStatesOutput.WriteZInt(int32(size)); err != nil {
		return err
	}
	// First iteration writes the fields ids.
	for i := 0; i < size; i++ {
		fieldMetadataTermState = line.TermStates[i]
		if err := termStatesOutput.WriteVInt(
			int32(fieldMetadataTermState.FieldMetadata().GetFieldInfo().Number())); err != nil {
			return err
		}
	}
	// Second iteration writes the corresponding field TermStates.
	for i := 0; i < size; i++ {
		fieldMetadataTermState = line.TermStates[i]
		if err := encoder.WriteTermState(
			termStatesOutput,
			fieldMetadataTermState.FieldMetadata().GetFieldInfo(),
			fieldMetadataTermState.State()); err != nil {
			return err
		}
	}
	return nil
}

// ReadTermStateForField reads a single BlockTermState for the provided field
// in the current block line of the provided input.
//
// termStatesInput is the data input to read the BlockTermState from,
// blockHeader is the current block header and reuse is a previous
// BlockTermState to reuse, or nil to create a new one.
//
// Returns the BlockTermState corresponding to the provided field id; or nil if
// the field does not occur in the line.
//
// Mirrors STBlockLine.Serializer.readTermStateForField(int, DataInput,
// DeltaBaseTermStateSerializer, BlockHeader, FieldInfos, BlockTermState)
// (STBlockLine.java:117).
func (s *STBlockLineSerializer) ReadTermStateForField(
	fieldID int,
	termStatesInput store.DataInput,
	termStateSerializer *uniformsplit.DeltaBaseTermStateSerializer,
	blockHeader *uniformsplit.BlockHeader,
	fieldInfos *index.FieldInfos,
	reuse index.TermState,
) (index.TermState, error) {
	// assert fieldId >= 0;
	numFields, err := termStatesInput.ReadZInt()
	if err != nil {
		return nil, err
	}
	if numFields <= 0 {
		readFieldID := -numFields
		if int32(fieldID) == readFieldID {
			return termStateSerializer.ReadTermState(
				blockHeader.BaseDocsFP(),
				blockHeader.BasePositionsFP(),
				blockHeader.BasePayloadsFP(),
				termStatesInput,
				fieldInfos.FieldInfoByNumber(int(readFieldID)),
				reuse)
		}
		return nil, nil
	}

	// There are multiple fields for the term.
	// We have to read all the field ids (aka field numbers) sequentially.
	// Then if the required field is in the list, we have to read all the
	// TermState sequentially. This could be optimized with a jump-to-middle
	// offset for example, but we don't need that currently.

	isFieldInList := false
	readFieldIDs := make([]int32, numFields)
	for i := int32(0); i < numFields; i++ {
		readFieldID, err := termStatesInput.ReadVInt()
		if err != nil {
			return nil, err
		}
		if !isFieldInList && readFieldID > int32(fieldID) {
			// As the list of fieldIds is sorted we can return early if we
			// find fieldId greater than the seeked one.
			// But if we found the seeked one, we have to read all the list to
			// get to the term state part afterward (there is no jump offset).
			return nil, nil
		}
		isFieldInList = isFieldInList || readFieldID == int32(fieldID)
		readFieldIDs[i] = readFieldID
	}
	if isFieldInList {
		for _, readFieldID := range readFieldIDs {
			termState, err := termStateSerializer.ReadTermState(
				blockHeader.BaseDocsFP(),
				blockHeader.BasePositionsFP(),
				blockHeader.BasePayloadsFP(),
				termStatesInput,
				fieldInfos.FieldInfoByNumber(int(readFieldID)),
				reuse)
			if err != nil {
				return nil, err
			}
			if int32(fieldID) == readFieldID {
				return termState, nil
			}
		}
	}
	return nil, nil
}

// ReadFieldTermStatesMap reads all the BlockTermState of all the fields in the
// current block line of the provided input.
//
// fieldTermStatesMap is filled with the term states for each field. It is
// cleared first. See ReadTermStateForField.
//
// Mirrors STBlockLine.Serializer.readFieldTermStatesMap(DataInput,
// DeltaBaseTermStateSerializer, BlockHeader, FieldInfos, Map<String,
// BlockTermState>) (STBlockLine.java:186).
func (s *STBlockLineSerializer) ReadFieldTermStatesMap(
	termStatesInput store.DataInput,
	termStateSerializer *uniformsplit.DeltaBaseTermStateSerializer,
	blockHeader *uniformsplit.BlockHeader,
	fieldInfos *index.FieldInfos,
	fieldTermStatesMap map[string]index.TermState,
) error {
	clear(fieldTermStatesMap)
	numFields, err := termStatesInput.ReadZInt()
	if err != nil {
		return err
	}
	if numFields <= 0 {
		fieldID := -numFields
		termState, err := termStateSerializer.ReadTermState(
			blockHeader.BaseDocsFP(),
			blockHeader.BasePositionsFP(),
			blockHeader.BasePayloadsFP(),
			termStatesInput,
			fieldInfos.FieldInfoByNumber(int(fieldID)),
			nil)
		if err != nil {
			return err
		}
		fieldTermStatesMap[fieldInfos.FieldInfoByNumber(int(fieldID)).Name()] = termState
		return nil
	}
	fieldIDs, err := s.ReadFieldIDs(termStatesInput, numFields)
	if err != nil {
		return err
	}
	for _, fieldID := range fieldIDs {
		termState, err := termStateSerializer.ReadTermState(
			blockHeader.BaseDocsFP(),
			blockHeader.BasePositionsFP(),
			blockHeader.BasePayloadsFP(),
			termStatesInput,
			fieldInfos.FieldInfoByNumber(int(fieldID)),
			nil)
		if err != nil {
			return err
		}
		fieldTermStatesMap[fieldInfos.FieldInfoByNumber(int(fieldID)).Name()] = termState
	}
	return nil
}

// ReadFieldIDs reads all the field ids in the current block line of the
// provided input.
//
// Mirrors STBlockLine.Serializer.readFieldIds(DataInput, int)
// (STBlockLine.java:228).
func (s *STBlockLineSerializer) ReadFieldIDs(termStatesInput store.DataInput, numFields int32) ([]int32, error) {
	fieldIDs := make([]int32, numFields)
	for i := int32(0); i < numFields; i++ {
		fieldID, err := termStatesInput.ReadVInt()
		if err != nil {
			return nil, err
		}
		fieldIDs[i] = fieldID
	}
	return fieldIDs, nil
}
