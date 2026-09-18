package sharedterms

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// FieldMetadataTermState associates field metadata with its term state.
type FieldMetadataTermState struct {
	FieldMetadata *uniformsplit.FieldMetadata
	FieldInfo     *index.FieldInfo
	State         index.TermState
}

// STBlockLine represents a term and its details stored in the Shared Terms format.
type STBlockLine struct {
	uniformsplit.BlockLine
	TermStates []FieldMetadataTermState
}

// NewSTBlockLine builds an STBlockLine.
func NewSTBlockLine(termBytes []byte, termStates []FieldMetadataTermState) *STBlockLine {
	ts := make([]FieldMetadataTermState, len(termStates))
	copy(ts, termStates)
	return &STBlockLine{
		BlockLine: uniformsplit.BlockLine{
			TermBytes: termBytes,
		},
		TermStates: ts,
	}
}

// STBlockLineSerializer handles writing and reading of STBlockLine.
type STBlockLineSerializer struct{}

// WriteLineTermStates writes all the BlockTermState of the provided STBlockLine to the given output.
func (s *STBlockLineSerializer) WriteLineTermStates(
	out store.DataOutput,
	line *STBlockLine,
	encoder *uniformsplit.DeltaBaseTermStateSerializer) error {

	size := len(line.TermStates)
	if size == 0 {
		return nil
	}

	if size == 1 {
		// When there is only 1 field, write its id as negative, followed by the field TermState.
		fieldID := int32(line.TermStates[0].FieldInfo.Number())
		if err := out.WriteZInt(-fieldID); err != nil {
			return err
		}
		if err := encoder.WriteTermState(out, line.TermStates[0].FieldInfo, line.TermStates[0].State); err != nil {
			return err
		}
		return nil
	}

	if err := out.WriteZInt(int32(size)); err != nil {
		return err
	}
	// First iteration writes the fields ids.
	for i := 0; i < size; i++ {
		if err := out.WriteVInt(int32(line.TermStates[i].FieldInfo.Number())); err != nil {
			return err
		}
	}
	// Second iteration writes the corresponding field TermStates.
	for i := 0; i < size; i++ {
		if err := encoder.WriteTermState(out, line.TermStates[i].FieldInfo, line.TermStates[i].State); err != nil {
			return err
		}
	}
	return nil
}

// ReadTermStateForField reads a single BlockTermState for the provided field in the current block line.
func (s *STBlockLineSerializer) ReadTermStateForField(
	fieldId int,
	in store.DataInput,
	serializer *uniformsplit.DeltaBaseTermStateSerializer,
	header *uniformsplit.BlockHeader,
	fieldInfos *index.FieldInfos,
	reuse index.TermState) (index.TermState, error) {

	numFields, err := in.ReadZInt()
	if err != nil {
		return nil, err
	}
	if numFields <= 0 {
		readFieldId := -numFields
		if fieldId == int(readFieldId) {
			return serializer.ReadTermState(
				int64(header.BaseDocsFP),
				int64(header.BasePositionsFP),
				int64(header.BasePayloadsFP),
				in,
				fieldInfos.FieldInfo(readFieldId),
				reuse)
		}
		return nil, nil
	}

	// Multiple fields. Read ids.
	readFieldIds := make([]int, numFields)
	isFieldInList := false
	for i := 0; i < numFields; i++ {
		id, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		if !isFieldInList && int(id) > fieldId {
			return nil, nil
		}
		if int(id) == fieldId {
			isFieldInList = true
		}
		readFieldIds[i] = int(id)
	}

	if isFieldInList {
		for _, id := range readFieldIds {
			state, err := serializer.ReadTermState(
				int64(header.BaseDocsFP),
				int64(header.BasePositionsFP),
				int64(header.BasePayloadsFP),
				in,
				fieldInfos.FieldInfo(id),
				reuse)
			if err != nil {
				return nil, err
			}
			if id == fieldId {
				return state, nil
			}
		}
	}
	return nil, nil
}

// ReadFieldTermStatesMap reads all the BlockTermState of all the field in the current block line.
func (s *STBlockLineSerializer) ReadFieldTermStatesMap(
	in store.DataInput,
	serializer *uniformsplit.DeltaBaseTermStateSerializer,
	header *uniformsplit.BlockHeader,
	fieldInfos *index.FieldInfos,
	fieldTermStatesMap map[string]index.TermState) error {

	numFields, err := in.ReadZInt()
	if err != nil {
		return err
	}
	if numFields <= 0 {
		fieldId := -numFields
		state, err := serializer.ReadTermState(
			int64(header.BaseDocsFP),
			int64(header.BasePositionsFP),
			int64(header.BasePayloadsFP),
			in,
			fieldInfos.FieldInfo(fieldId),
			nil)
		if err != nil {
			return err
		}
		fieldTermStatesMap[fieldInfos.FieldInfo(fieldId).Name()] = state
		return nil
	}

	readFieldIds := make([]int, numFields)
	for i := 0; i < numFields; i++ {
		id, err := in.ReadVInt()
		if err != nil {
			return err
		}
		readFieldIds[i] = int(id)
	}

	for _, id := range readFieldIds {
		state, err := serializer.ReadTermState(
			int64(header.BaseDocsFP),
			int64(header.BasePositionsFP),
			int64(header.BasePayloadsFP),
			in,
			fieldInfos.FieldInfo(id),
			nil)
		if err != nil {
			return err
		}
		fieldTermStatesMap[fieldInfos.FieldInfo(id).Name()] = state
	}
	return nil
}
