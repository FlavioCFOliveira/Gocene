// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene94/Lucene94FieldInfosFormat.java
// GOC-3329: replace the stub Lucene94FieldInfosFormat with a full read/write port.

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene94FieldInfosFormat is the Go port of
// org.apache.lucene.codecs.lucene94.Lucene94FieldInfosFormat.
//
// Field metadata is persisted in a .fnm file with the layout:
//
//	Header,FieldsCount,<FieldName,FieldNumber,FieldBits,IndexOptions,
//	DocValuesBits,DocValuesSkipIndexBits,DocValuesGen,Attributes,
//	PointDimensionCount,PointIndexDimensionCount,PointNumBytes,
//	VectorDimension,VectorEncoding,VectorSimilarityFunction>^FieldsCount,
//	Footer
type Lucene94FieldInfosFormat struct{}

// Wire-format constants. The names mirror the Java constants so the
// Lucene reference reads 1:1 against this port.
const (
	lucene94FIExtension       = "fnm"
	lucene94FICodecName       = "Lucene94FieldInfos"
	lucene94FIFormatStart     = 0
	lucene94FIFormatParent    = 1
	lucene94FIFormatSkipper   = 2
	lucene94FIFormatCurrent   = lucene94FIFormatSkipper
	lucene94FIStoreTermVector = byte(0x1)
	lucene94FIOmitNorms       = byte(0x2)
	lucene94FIStorePayloads   = byte(0x4)
	lucene94FISoftDeletes     = byte(0x8)
	lucene94FIParentField     = byte(0x10)
	lucene94FIDocValuesSkip   = byte(0x20)
)

// NewLucene94FieldInfosFormat returns a fresh Lucene94FieldInfosFormat.
func NewLucene94FieldInfosFormat() *Lucene94FieldInfosFormat {
	return &Lucene94FieldInfosFormat{}
}

// Name returns the canonical codec name.
func (f *Lucene94FieldInfosFormat) Name() string {
	return "Lucene94FieldInfosFormat"
}

// Read decodes the field infos for the given segment.
func (f *Lucene94FieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, lucene94FIExtension)

	in, err := dir.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}

	checksumIn := store.NewChecksumIndexInput(in)

	infos, readErr := f.readFrom(checksumIn, segmentInfo, segmentSuffix)
	// Always close the checksum input so wrappers may surface close-time errors.
	closeErr := checksumIn.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return infos, nil
}

func (f *Lucene94FieldInfosFormat) readFrom(in *store.ChecksumIndexInput, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error) {
	format, err := CheckIndexHeader(in, lucene94FICodecName, lucene94FIFormatStart, lucene94FIFormatCurrent, segmentInfo.GetID(), segmentSuffix)
	if err != nil {
		return nil, err
	}

	size, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}

	builder := index.NewFieldInfosBuilder()
	for i := int32(0); i < size; i++ {
		name, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		fieldNumber, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		if fieldNumber < 0 {
			return nil, fmt.Errorf("invalid field number for field: %s, fieldNumber=%d", name, fieldNumber)
		}

		bits, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		if bits&0xC0 != 0 {
			return nil, fmt.Errorf("unused bits are set: 0x%02x", bits)
		}
		if format < lucene94FIFormatParent && bits&0xF0 != 0 {
			return nil, fmt.Errorf("parent field bit is set but shouldn't: 0x%02x", bits)
		}
		if format < lucene94FIFormatSkipper && bits&lucene94FIDocValuesSkip != 0 {
			return nil, fmt.Errorf("doc values skipper bit is set but shouldn't: 0x%02x", bits)
		}

		storeTermVector := bits&lucene94FIStoreTermVector != 0
		omitNorms := bits&lucene94FIOmitNorms != 0
		_ = bits & lucene94FIStorePayloads // payload flag is persisted but Gocene FieldInfo derives HasPayloads() from index options
		isSoftDeletesField := bits&lucene94FISoftDeletes != 0
		isParentField := format >= lucene94FIFormatParent && bits&lucene94FIParentField != 0

		indexOptionsByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		indexOptions, err := decodeIndexOptions(indexOptionsByte)
		if err != nil {
			return nil, err
		}

		docValuesTypeByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		docValuesType, err := decodeDocValuesType(docValuesTypeByte)
		if err != nil {
			return nil, err
		}

		docValuesSkipIndex := index.DocValuesSkipIndexTypeNone
		if format >= lucene94FIFormatSkipper {
			skipByte, err := in.ReadByte()
			if err != nil {
				return nil, err
			}
			docValuesSkipIndex, err = decodeDocValuesSkipIndexType(skipByte)
			if err != nil {
				return nil, err
			}
		}

		dvGen, err := store.ReadInt64(in)
		if err != nil {
			return nil, err
		}

		attributes, err := store.ReadMapOfStrings(in)
		if err != nil {
			return nil, err
		}

		pointDataDimensionCount, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		var pointNumBytes int32
		pointIndexDimensionCount := pointDataDimensionCount
		if pointDataDimensionCount != 0 {
			pointIndexDimensionCount, err = store.ReadVInt(in)
			if err != nil {
				return nil, err
			}
			pointNumBytes, err = store.ReadVInt(in)
			if err != nil {
				return nil, err
			}
		}

		vectorDimension, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		vectorEncodingByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		vectorEncoding, err := decodeVectorEncoding(vectorEncodingByte)
		if err != nil {
			return nil, err
		}

		vectorDistFuncByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		vectorDistFunc, err := decodeVectorSimilarityFunction(vectorDistFuncByte)
		if err != nil {
			return nil, err
		}

		fib := index.NewFieldInfoBuilder(name, int(fieldNumber)).
			SetIndexOptions(indexOptions).
			SetDocValuesType(docValuesType).
			SetDocValuesSkipIndexType(docValuesSkipIndex).
			SetDocValuesGen(dvGen).
			SetOmitNorms(omitNorms).
			SetStoreTermVectors(storeTermVector).
			SetPointDimensions(int(pointDataDimensionCount), int(pointIndexDimensionCount), int(pointNumBytes)).
			SetVectorAttributes(int(vectorDimension), vectorEncoding, vectorDistFunc).
			SetSoftDeletesField(isSoftDeletesField).
			SetParentField(isParentField)

		for k, v := range attributes {
			fib.SetAttribute(k, v)
		}
		builder.Add(fib.Build())
	}

	if _, err := CheckFooter(in); err != nil {
		return nil, err
	}
	return builder.Build(), nil
}

// Write encodes the field infos for the given segment.
func (f *Lucene94FieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, lucene94FIExtension)

	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}

	checksumOut := store.NewChecksumIndexOutput(out)
	writeErr := f.writeTo(checksumOut, segmentInfo, segmentSuffix, infos)
	closeErr := checksumOut.Close()
	if writeErr != nil || closeErr != nil {
		// Match Lucene's IOUtils.deleteFilesIgnoringExceptions: a failed write
		// or close must not leave a half-formed .fnm in the directory,
		// otherwise a retry will trip "file already exists".
		_ = dir.DeleteFile(fileName)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func (f *Lucene94FieldInfosFormat) writeTo(out *store.ChecksumIndexOutput, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error {
	if err := WriteIndexHeader(out, lucene94FICodecName, lucene94FIFormatCurrent, segmentInfo.GetID(), segmentSuffix); err != nil {
		return err
	}
	if err := store.WriteVInt(out, int32(infos.Size())); err != nil {
		return err
	}

	iter := infos.Iterator()
	for iter.HasNext() {
		fi := iter.Next()
		if err := store.WriteString(out, fi.Name()); err != nil {
			return err
		}
		if err := store.WriteVInt(out, int32(fi.Number())); err != nil {
			return err
		}

		var bits byte
		if fi.StoreTermVectors() {
			bits |= lucene94FIStoreTermVector
		}
		if fi.OmitNorms() {
			bits |= lucene94FIOmitNorms
		}
		if fi.HasPayloads() {
			bits |= lucene94FIStorePayloads
		}
		if fi.IsSoftDeletesField() {
			bits |= lucene94FISoftDeletes
		}
		if fi.IsParentField() {
			bits |= lucene94FIParentField
		}
		if err := out.WriteByte(bits); err != nil {
			return err
		}

		ioByte, err := encodeIndexOptions(fi.IndexOptions())
		if err != nil {
			return err
		}
		if err := out.WriteByte(ioByte); err != nil {
			return err
		}

		dvByte, err := encodeDocValuesType(fi.DocValuesType())
		if err != nil {
			return err
		}
		if err := out.WriteByte(dvByte); err != nil {
			return err
		}

		dvSkipByte, err := encodeDocValuesSkipIndexType(fi.DocValuesSkipIndexType())
		if err != nil {
			return err
		}
		if err := out.WriteByte(dvSkipByte); err != nil {
			return err
		}

		if err := store.WriteInt64(out, fi.DocValuesGen()); err != nil {
			return err
		}
		if err := store.WriteMapOfStrings(out, fi.GetAttributes()); err != nil {
			return err
		}

		if err := store.WriteVInt(out, int32(fi.PointDimensionCount())); err != nil {
			return err
		}
		if fi.PointDimensionCount() != 0 {
			if err := store.WriteVInt(out, int32(fi.PointIndexDimensionCount())); err != nil {
				return err
			}
			if err := store.WriteVInt(out, int32(fi.PointNumBytes())); err != nil {
				return err
			}
		}

		if err := store.WriteVInt(out, int32(fi.VectorDimension())); err != nil {
			return err
		}
		veByte, err := encodeVectorEncoding(fi.VectorEncoding())
		if err != nil {
			return err
		}
		if err := out.WriteByte(veByte); err != nil {
			return err
		}
		vsByte, err := encodeVectorSimilarityFunction(fi.VectorSimilarityFunction())
		if err != nil {
			return err
		}
		if err := out.WriteByte(vsByte); err != nil {
			return err
		}
	}
	return WriteFooter(out)
}

// --- enum encoders / decoders ---------------------------------------------

func encodeIndexOptions(o index.IndexOptions) (byte, error) {
	switch o {
	case index.IndexOptionsNone:
		return 0, nil
	case index.IndexOptionsDocs:
		return 1, nil
	case index.IndexOptionsDocsAndFreqs:
		return 2, nil
	case index.IndexOptionsDocsAndFreqsAndPositions:
		return 3, nil
	case index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets:
		return 4, nil
	}
	return 0, fmt.Errorf("unhandled IndexOptions: %v", o)
}

func decodeIndexOptions(b byte) (index.IndexOptions, error) {
	switch b {
	case 0:
		return index.IndexOptionsNone, nil
	case 1:
		return index.IndexOptionsDocs, nil
	case 2:
		return index.IndexOptionsDocsAndFreqs, nil
	case 3:
		return index.IndexOptionsDocsAndFreqsAndPositions, nil
	case 4:
		return index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, nil
	}
	return 0, fmt.Errorf("invalid IndexOptions byte: %d", b)
}

func encodeDocValuesType(d index.DocValuesType) (byte, error) {
	switch d {
	case index.DocValuesTypeNone:
		return 0, nil
	case index.DocValuesTypeNumeric:
		return 1, nil
	case index.DocValuesTypeBinary:
		return 2, nil
	case index.DocValuesTypeSorted:
		return 3, nil
	case index.DocValuesTypeSortedSet:
		return 4, nil
	case index.DocValuesTypeSortedNumeric:
		return 5, nil
	}
	return 0, fmt.Errorf("unhandled DocValuesType: %v", d)
}

func decodeDocValuesType(b byte) (index.DocValuesType, error) {
	switch b {
	case 0:
		return index.DocValuesTypeNone, nil
	case 1:
		return index.DocValuesTypeNumeric, nil
	case 2:
		return index.DocValuesTypeBinary, nil
	case 3:
		return index.DocValuesTypeSorted, nil
	case 4:
		return index.DocValuesTypeSortedSet, nil
	case 5:
		return index.DocValuesTypeSortedNumeric, nil
	}
	return 0, fmt.Errorf("invalid docvalues byte: %d", b)
}

func encodeDocValuesSkipIndexType(s index.DocValuesSkipIndexType) (byte, error) {
	switch s {
	case index.DocValuesSkipIndexTypeNone:
		return 0, nil
	case index.DocValuesSkipIndexTypeRange:
		return 1, nil
	}
	return 0, fmt.Errorf("unhandled DocValuesSkipIndexType: %v", s)
}

func decodeDocValuesSkipIndexType(b byte) (index.DocValuesSkipIndexType, error) {
	switch b {
	case 0:
		return index.DocValuesSkipIndexTypeNone, nil
	case 1:
		return index.DocValuesSkipIndexTypeRange, nil
	}
	return 0, fmt.Errorf("invalid docvaluesskipindex byte: %d", b)
}

func encodeVectorEncoding(e index.VectorEncoding) (byte, error) {
	switch e {
	case index.VectorEncodingByte:
		return 0, nil
	case index.VectorEncodingFloat32:
		return 1, nil
	}
	return 0, fmt.Errorf("unhandled VectorEncoding: %v", e)
}

func decodeVectorEncoding(b byte) (index.VectorEncoding, error) {
	switch b {
	case 0:
		return index.VectorEncodingByte, nil
	case 1:
		return index.VectorEncodingFloat32, nil
	}
	return 0, fmt.Errorf("invalid vector encoding: %d", b)
}

// Lucene fixes this ordering inside the format so the on-disk byte is stable
// even if the VectorSimilarityFunction enum is reordered upstream.
var lucene94SimilarityFunctions = [...]index.VectorSimilarityFunction{
	index.VectorSimilarityFunctionEuclidean,
	index.VectorSimilarityFunctionDotProduct,
	index.VectorSimilarityFunctionCosine,
	index.VectorSimilarityFunctionMaximumInnerProduct,
}

func encodeVectorSimilarityFunction(v index.VectorSimilarityFunction) (byte, error) {
	for i, f := range lucene94SimilarityFunctions {
		if f == v {
			return byte(i), nil
		}
	}
	return 0, fmt.Errorf("invalid distance function: %v", v)
}

func decodeVectorSimilarityFunction(b byte) (index.VectorSimilarityFunction, error) {
	if int(b) >= len(lucene94SimilarityFunctions) {
		return 0, fmt.Errorf("invalid distance function: %d", b)
	}
	return lucene94SimilarityFunctions[b], nil
}

// errLucene94FormatBug is kept private to surface misuse panics consistently.
var errLucene94FormatBug = errors.New("lucene94 field infos format: internal invariant violated")

var _ = errLucene94FormatBug // reserved for future invariant checks
