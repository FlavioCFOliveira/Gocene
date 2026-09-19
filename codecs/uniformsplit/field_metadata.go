// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FieldMetadata holds metadata and stats for one field in the index.
//
// There is only one instance of FieldMetadata per FieldInfo.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.FieldMetadata from Apache
// Lucene 10.5.0.
type FieldMetadata struct {
	fieldInfo *index.FieldInfo
	isMutable bool
	docsSeen  *util.FixedBitSet

	sumDocFreq       int64
	numTerms         int64
	sumTotalTermFreq int64
	docCount         int32

	dictionaryStartFP int64
	firstBlockStartFP int64
	lastBlockStartFP  int64

	lastTerm *util.BytesRef
}

// NewFieldMetadata constructs field metadata for writing. maxDoc is the total
// number of documents in the segment being written.
//
// Mirrors the FieldMetadata(FieldInfo, int) constructor
// (FieldMetadata.java:56), which delegates with isMutable = true.
func NewFieldMetadata(fieldInfo *index.FieldInfo, maxDoc int) (*FieldMetadata, error) {
	return newFieldMetadata(fieldInfo, maxDoc, true)
}

// NewFieldMetadataForReading constructs immutable virtual field metadata for
// reading.
//
// Mirrors the FieldMetadata(long, long, long, BytesRef) constructor
// (FieldMetadata.java:61).
func NewFieldMetadataForReading(dictionaryStartFP, firstBlockStartFP, lastBlockStartFP int64, lastTerm *util.BytesRef) (*FieldMetadata, error) {
	fm, err := newFieldMetadata(nil, 0, false)
	if err != nil {
		return nil, err
	}
	fm.dictionaryStartFP = dictionaryStartFP
	fm.firstBlockStartFP = firstBlockStartFP
	fm.lastBlockStartFP = lastBlockStartFP
	fm.lastTerm = lastTerm
	return fm, nil
}

// newFieldMetadata constructs field metadata for reading or writing. maxDoc is
// the total number of documents in the segment being written. isMutable is true
// when this FieldMetadata is created for writing the index, false when it is
// used for reading the index.
//
// Mirrors the protected FieldMetadata(FieldInfo, int, boolean) constructor
// (FieldMetadata.java:78).
func newFieldMetadata(fieldInfo *index.FieldInfo, maxDoc int, isMutable bool) (*FieldMetadata, error) {
	// assert isMutable || maxDoc == 0;
	fm := &FieldMetadata{
		fieldInfo: fieldInfo,
		isMutable: isMutable,
	}
	// docsSeen must not be set if this FieldMetadata is immutable, that means it is used for
	// reading the index.
	if isMutable {
		docsSeen, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			return nil, err
		}
		fm.docsSeen = docsSeen
	}
	fm.dictionaryStartFP = -1
	fm.firstBlockStartFP = -1
	fm.lastBlockStartFP = -1
	return fm, nil
}

// UpdateStats updates the field stats with the given BlockTermState for the
// current block line (for one term).
//
// Mirrors FieldMetadata.updateStats (FieldMetadata.java:94).
func (fm *FieldMetadata) UpdateStats(state index.TermState) {
	// assert isMutable;
	base := codecs.BaseState(state)
	// assert state.docFreq > 0;
	fm.sumDocFreq += int64(base.DocFreq)
	if base.TotalTermFreq > 0 {
		fm.sumTotalTermFreq += base.TotalTermFreq
	}
	fm.numTerms++
}

// GetDocsSeen provides the FixedBitSet to keep track of the docs seen when
// calling PostingsWriterBase.WriteTerm.
//
// The returned FixedBitSet is created once in the FieldMetadata constructor.
// It is nil when this FieldMetadata was created immutable during segment
// reading.
//
// Mirrors FieldMetadata.getDocsSeen (FieldMetadata.java:115).
func (fm *FieldMetadata) GetDocsSeen() *util.FixedBitSet {
	return fm.docsSeen
}

// GetFieldInfo returns the FieldInfo of this field.
func (fm *FieldMetadata) GetFieldInfo() *index.FieldInfo {
	return fm.fieldInfo
}

// GetSumDocFreq returns the sum of the doc frequencies of the terms of this
// field.
func (fm *FieldMetadata) GetSumDocFreq() int64 {
	return fm.sumDocFreq
}

// GetNumTerms returns the number of terms of this field.
func (fm *FieldMetadata) GetNumTerms() int64 {
	return fm.numTerms
}

// GetSumTotalTermFreq returns the sum of the total term frequencies of the
// terms of this field.
func (fm *FieldMetadata) GetSumTotalTermFreq() int64 {
	return fm.sumTotalTermFreq
}

// GetDocCount returns the number of documents that have at least one term for
// this field.
func (fm *FieldMetadata) GetDocCount() int32 {
	if fm.isMutable {
		return int32(fm.docsSeen.Cardinality())
	}
	return fm.docCount
}

// GetFirstBlockStartFP returns the file pointer to the start of the first block
// of the field.
func (fm *FieldMetadata) GetFirstBlockStartFP() int64 {
	return fm.firstBlockStartFP
}

// SetFirstBlockStartFP sets the file pointer to the start of the first block of
// the field.
func (fm *FieldMetadata) SetFirstBlockStartFP(firstBlockStartFP int64) {
	// assert isMutable;
	fm.firstBlockStartFP = firstBlockStartFP
}

// GetLastBlockStartFP returns the start file pointer for the last block of the
// field.
func (fm *FieldMetadata) GetLastBlockStartFP() int64 {
	return fm.lastBlockStartFP
}

// SetLastBlockStartFP sets the file pointer after the end of the last block of
// the field.
func (fm *FieldMetadata) SetLastBlockStartFP(lastBlockStartFP int64) {
	// assert isMutable;
	fm.lastBlockStartFP = lastBlockStartFP
}

// GetDictionaryStartFP returns the file pointer to the start of the dictionary
// of the field.
func (fm *FieldMetadata) GetDictionaryStartFP() int64 {
	return fm.dictionaryStartFP
}

// SetDictionaryStartFP sets the file pointer to the start of the dictionary of
// the field.
func (fm *FieldMetadata) SetDictionaryStartFP(dictionaryStartFP int64) {
	// assert isMutable;
	fm.dictionaryStartFP = dictionaryStartFP
}

// SetLastTerm sets the last term of the field.
func (fm *FieldMetadata) SetLastTerm(lastTerm *util.BytesRef) {
	// assert lastTerm != null;
	fm.lastTerm = lastTerm
}

// GetLastTerm returns the last term of the field.
func (fm *FieldMetadata) GetLastTerm() *util.BytesRef {
	return fm.lastTerm
}

// FieldMetadataSerializer reads/writes field metadata.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.FieldMetadata.Serializer
// (FieldMetadata.java:183).
type FieldMetadataSerializer struct{}

// FieldMetadataSerializerInstance is the stateless singleton. Mirrors
// FieldMetadata.Serializer.INSTANCE (FieldMetadata.java:186).
var FieldMetadataSerializerInstance = &FieldMetadataSerializer{}

// Write writes the field metadata to the provided output.
//
// Mirrors FieldMetadata.Serializer.write (FieldMetadata.java:188).
func (s *FieldMetadataSerializer) Write(output store.DataOutput, fieldMetadata *FieldMetadata) error {
	// assert fieldMetadata.dictionaryStartFP >= 0;
	// assert fieldMetadata.firstBlockStartFP >= 0;
	// assert fieldMetadata.lastBlockStartFP >= 0;
	// assert fieldMetadata.numTerms > 0;
	// assert fieldMetadata.firstBlockStartFP <= fieldMetadata.lastBlockStartFP;
	// assert fieldMetadata.lastTerm != null;

	if err := output.WriteVInt(int32(fieldMetadata.fieldInfo.Number())); err != nil {
		return err
	}

	if err := output.WriteVLong(fieldMetadata.numTerms); err != nil {
		return err
	}
	if err := output.WriteVLong(fieldMetadata.sumDocFreq); err != nil {
		return err
	}

	if fieldMetadata.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs) {
		// assert fieldMetadata.sumTotalTermFreq >= fieldMetadata.sumDocFreq;
		if err := output.WriteVLong(fieldMetadata.sumTotalTermFreq - fieldMetadata.sumDocFreq); err != nil {
			return err
		}
	}

	if err := output.WriteVInt(fieldMetadata.GetDocCount()); err != nil {
		return err
	}

	if err := output.WriteVLong(fieldMetadata.dictionaryStartFP); err != nil {
		return err
	}
	if err := output.WriteVLong(fieldMetadata.firstBlockStartFP); err != nil {
		return err
	}
	if err := output.WriteVLong(fieldMetadata.lastBlockStartFP); err != nil {
		return err
	}

	if fieldMetadata.lastTerm.Length > 0 {
		if err := output.WriteVInt(int32(fieldMetadata.lastTerm.Length)); err != nil {
			return err
		}
		return output.WriteBytes(fieldMetadata.lastTerm.Bytes, fieldMetadata.lastTerm.Offset, fieldMetadata.lastTerm.Length)
	}
	return output.WriteVInt(0)
}

// Read reads the field metadata from the provided input.
//
// Mirrors FieldMetadata.Serializer.read (FieldMetadata.java:225).
func (s *FieldMetadataSerializer) Read(input store.DataInput, fieldInfos *index.FieldInfos, maxNumDocs int32) (*FieldMetadata, error) {
	fieldID, err := input.ReadVInt()
	if err != nil {
		return nil, err
	}
	fieldInfo := fieldInfos.FieldInfoByNumber(int(fieldID))
	if fieldInfo == nil {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal field id= %d", fieldID), fmt.Sprint(input))
	}
	fieldMetadata, err := newFieldMetadata(fieldInfo, 0, false)
	if err != nil {
		return nil, err
	}

	if fieldMetadata.numTerms, err = input.ReadVLong(); err != nil {
		return nil, err
	}
	if fieldMetadata.numTerms <= 0 {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal number of terms= %d for field= %d", fieldMetadata.numTerms, fieldID),
			fmt.Sprint(input))
	}

	if fieldMetadata.sumDocFreq, err = input.ReadVLong(); err != nil {
		return nil, err
	}
	fieldMetadata.sumTotalTermFreq = fieldMetadata.sumDocFreq
	if fieldMetadata.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs) {
		delta, err := input.ReadVLong()
		if err != nil {
			return nil, err
		}
		fieldMetadata.sumTotalTermFreq += delta
		if fieldMetadata.sumTotalTermFreq < fieldMetadata.sumDocFreq {
			// #positions must be >= #postings.
			return nil, index.NewCorruptIndexException(
				fmt.Sprintf("Illegal sumTotalTermFreq= %d sumDocFreq= %d for field= %d",
					fieldMetadata.sumTotalTermFreq, fieldMetadata.sumDocFreq, fieldID),
				fmt.Sprint(input))
		}
	}

	if fieldMetadata.docCount, err = input.ReadVInt(); err != nil {
		return nil, err
	}
	if fieldMetadata.docCount < 0 || fieldMetadata.docCount > maxNumDocs {
		// #docs with field must be <= #docs.
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal number of docs= %d maxNumDocs= %d for field=%d",
				fieldMetadata.docCount, maxNumDocs, fieldID),
			fmt.Sprint(input))
	}
	if fieldMetadata.sumDocFreq < int64(fieldMetadata.docCount) {
		// #postings must be >= #docs with field.
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal sumDocFreq= %d docCount= %d for field= %d",
				fieldMetadata.sumDocFreq, fieldMetadata.docCount, fieldID),
			fmt.Sprint(input))
	}

	if fieldMetadata.dictionaryStartFP, err = input.ReadVLong(); err != nil {
		return nil, err
	}
	if fieldMetadata.firstBlockStartFP, err = input.ReadVLong(); err != nil {
		return nil, err
	}
	if fieldMetadata.lastBlockStartFP, err = input.ReadVLong(); err != nil {
		return nil, err
	}

	lastTermLength, err := input.ReadVInt()
	if err != nil {
		return nil, err
	}
	// Java allocates `new BytesRef(lastTermLength)` before testing the length,
	// so a negative length raises NegativeArraySizeException and the
	// CorruptIndexException branch below it is unreachable. The test is hoisted
	// here so that the corruption is reported with Lucene's own message instead
	// of a panic; the reachable behaviour is unchanged.
	if lastTermLength < 0 {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal last term length= %d for field= %d", lastTermLength, fieldID),
			fmt.Sprint(input))
	}
	// Java's `new BytesRef(int capacity)` allocates a byte[] of that length;
	// util.NewBytesRefWithCapacity only reserves capacity, so the backing array
	// is grown to the same length here.
	lastTerm := util.NewBytesRefWithCapacity(int(lastTermLength))
	lastTerm.Bytes = util.GrowByte(lastTerm.Bytes, int(lastTermLength))
	if lastTermLength > 0 {
		if err := input.ReadBytes(lastTerm.Bytes, 0, int(lastTermLength)); err != nil {
			return nil, err
		}
		lastTerm.Length = int(lastTermLength)
	}
	fieldMetadata.SetLastTerm(lastTerm)

	return fieldMetadata, nil
}
