// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene90/Lucene90FieldInfosFormat.java

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90FieldInfosFormat is the Go port of the Lucene 9.0 FieldInfosFormat
// from org.apache.lucene.backward_codecs.lucene90.Lucene90FieldInfosFormat.
//
// Wire layout (format version 0 — no DocValuesSkipIndex byte, no parent-field bit):
//
//	Header,FieldsCount,<FieldName,FieldNumber,FieldBits,IndexOptions,
//	DocValuesBits,DocValuesGen,Attributes,
//	PointDimensionCount,[PointIndexDimensionCount,PointNumBytes],
//	VectorDimension,VectorEncoding,VectorSimilarityFunction>^FieldsCount,
//	Footer
//
// Deviation from Lucene94: DocValuesSkipIndexType is not stored; any value
// set on the input FieldInfo is silently downgraded to NONE on write, and
// NONE is always returned on read.
type Lucene90FieldInfosFormat struct{}

const (
	lucene90FIExtension     = "fnm"
	lucene90FICodecName     = "Lucene90FieldInfos"
	lucene90FIFormatStart   = 0
	lucene90FIFormatCurrent = lucene90FIFormatStart

	lucene90FIStoreTermVector = byte(0x1)
	lucene90FIOmitNorms       = byte(0x2)
	lucene90FIStorePayloads   = byte(0x4)
	lucene90FISoftDeletes     = byte(0x8)
	// bits 0x10..0xFF are unused in format 0
)

// NewLucene90FieldInfosFormat returns a fresh Lucene90FieldInfosFormat.
func NewLucene90FieldInfosFormat() *Lucene90FieldInfosFormat {
	return &Lucene90FieldInfosFormat{}
}

// Name returns the canonical codec name.
func (f *Lucene90FieldInfosFormat) Name() string {
	return "Lucene90FieldInfosFormat"
}

// Read decodes the field infos for the given segment from a Lucene90 .fnm file.
func (f *Lucene90FieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, lucene90FIExtension)

	in, err := dir.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}

	checksumIn := store.NewChecksumIndexInput(in)

	infos, readErr := f.readFrom(checksumIn, segmentInfo, segmentSuffix)
	closeErr := checksumIn.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return infos, nil
}

func (f *Lucene90FieldInfosFormat) readFrom(in *store.ChecksumIndexInput, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error) {
	_, err := CheckIndexHeader(in, lucene90FICodecName, lucene90FIFormatStart, lucene90FIFormatCurrent, segmentInfo.GetID(), segmentSuffix)
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
		// Format 0: bits 0xF0 must be zero.
		if bits&0xF0 != 0 {
			return nil, fmt.Errorf("unused bits are set: 0x%02x", bits)
		}

		storeTermVector := bits&lucene90FIStoreTermVector != 0
		omitNorms := bits&lucene90FIOmitNorms != 0
		storePayloads := bits&lucene90FIStorePayloads != 0
		isSoftDeletesField := bits&lucene90FISoftDeletes != 0

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
		// Format 0 has no DocValuesSkipIndex byte; always NONE.

		dvGen, err := store.ReadInt64(in)
		if err != nil {
			return nil, err
		}

		attributes, err := store.ReadMapOfStrings(in)
		if err != nil {
			return nil, err
		}

		// Restore Gocene-private TV sub-flags from attributes.
		storeTermVectorPositions := attributes[attrKeyTVPositions] == "1"
		storeTermVectorOffsets := attributes[attrKeyTVOffsets] == "1"
		storeTermVectorPayloads := attributes[attrKeyTVPayloads] == "1"

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
			SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeNone).
			SetDocValuesGen(dvGen).
			SetOmitNorms(omitNorms).
			SetStoreTermVectors(storeTermVector).
			SetStoreTermVectorPositions(storeTermVectorPositions).
			SetStoreTermVectorOffsets(storeTermVectorOffsets).
			SetStoreTermVectorPayloads(storeTermVectorPayloads).
			SetPointDimensions(int(pointDataDimensionCount), int(pointIndexDimensionCount), int(pointNumBytes)).
			SetVectorAttributes(int(vectorDimension), vectorEncoding, vectorDistFunc).
			SetSoftDeletesField(isSoftDeletesField)

		for k, v := range attributes {
			fib.SetAttribute(k, v)
		}
		fi := fib.Build()
		if storePayloads {
			fi.SetStorePayloads()
		}
		builder.Add(fi)
	}

	if _, err := CheckFooter(in); err != nil {
		return nil, err
	}
	return builder.Build(), nil
}

// Write encodes the field infos for the given segment into a Lucene90 .fnm file.
// DocValuesSkipIndexType is always written as NONE regardless of the input value.
func (f *Lucene90FieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, lucene90FIExtension)

	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}

	checksumOut := store.NewChecksumIndexOutput(out)
	writeErr := f.writeTo(checksumOut, segmentInfo, segmentSuffix, infos)
	closeErr := checksumOut.Close()
	if writeErr != nil || closeErr != nil {
		_ = dir.DeleteFile(fileName)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func (f *Lucene90FieldInfosFormat) writeTo(out *store.ChecksumIndexOutput, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error {
	if err := WriteIndexHeader(out, lucene90FICodecName, lucene90FIFormatCurrent, segmentInfo.GetID(), segmentSuffix); err != nil {
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

		// Persist Gocene-private TV sub-flags as codec attributes.
		if fi.StoreTermVectorPositions() {
			fi.PutCodecAttribute(attrKeyTVPositions, "1")
		}
		if fi.StoreTermVectorOffsets() {
			fi.PutCodecAttribute(attrKeyTVOffsets, "1")
		}
		if fi.StoreTermVectorPayloads() {
			fi.PutCodecAttribute(attrKeyTVPayloads, "1")
		}

		var bits byte
		if fi.StoreTermVectors() {
			bits |= lucene90FIStoreTermVector
		}
		if fi.OmitNorms() {
			bits |= lucene90FIOmitNorms
		}
		if fi.HasStoredPayloads() {
			bits |= lucene90FIStorePayloads
		}
		if fi.IsSoftDeletesField() {
			bits |= lucene90FISoftDeletes
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
		// No DocValuesSkipIndex byte in format 0.

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
