// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// FieldInfosFormat handles encoding/decoding of field metadata.
// This is the Go port of Lucene's org.apache.lucene.codecs.FieldInfosFormat.
type FieldInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads field infos from the given directory and segment info.
	Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error)

	// Write writes field infos to the given directory.
	Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error
}

// Lucene94FieldInfosFormat implements the Lucene 9.4 field infos format.
type Lucene94FieldInfosFormat struct{}

const (
	fiCodecName             = "Lucene94FieldInfos"
	fiFormatStart           = 0
	fiFormatParentField     = 1
	fiFormatDocValueSkipper = 2
	fiFormatCurrent         = fiFormatDocValueSkipper

	// Field flags
	fiStoreTermvector  byte = 0x1
	fiOmitNorms        byte = 0x2
	fiStorePayloads    byte = 0x4
	fiSoftDeletesField byte = 0x8
	fiParentFieldField byte = 0x10
	fiDocvaluesSkipper byte = 0x20
)

func NewLucene94FieldInfosFormat() *Lucene94FieldInfosFormat {
	return &Lucene94FieldInfosFormat{}
}

func (f *Lucene94FieldInfosFormat) Name() string {
	return "Lucene94FieldInfosFormat"
}

func (f *Lucene94FieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, "fnm")
	in, err := dir.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}

	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	var infos *index.FieldInfos
	format, err := CheckIndexHeader(checksumIn, fiCodecName, fiFormatStart, fiFormatCurrent, segmentInfo.GetID(), segmentSuffix)
	if err != nil {
		return nil, err
	}

	size, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}

	builder := index.NewFieldInfosBuilder()
	for i := int32(0); i < size; i++ {
		name, err := store.ReadString(checksumIn)
		if err != nil {
			return nil, err
		}
		fieldNumber, err := store.ReadVInt(checksumIn)
		if err != nil {
			return nil, err
		}
		if fieldNumber < 0 {
			return nil, fmt.Errorf("invalid field number for field: %s, fieldNumber=%d", name, fieldNumber)
		}

		bits, err := checksumIn.ReadByte()
		if err != nil {
			return nil, err
		}

		storeTermVector := (bits & fiStoreTermvector) != 0
		omitNorms := (bits & fiOmitNorms) != 0
		storePayloads := (bits & fiStorePayloads) != 0
		isSoftDeletesField := (bits & fiSoftDeletesField) != 0
		isParentField := false
		if format >= fiFormatParentField {
			isParentField = (bits & fiParentFieldField) != 0
		}

		indexOptionsByte, err := checksumIn.ReadByte()
		if err != nil {
			return nil, err
		}
		indexOptions := index.IndexOptions(indexOptionsByte)

		docValuesTypeByte, err := checksumIn.ReadByte()
		if err != nil {
			return nil, err
		}
		docValuesType := index.DocValuesType(docValuesTypeByte)

		var docValuesSkipIndex index.DocValuesSkipIndexType
		if format >= fiFormatDocValueSkipper {
			skipByte, err := checksumIn.ReadByte()
			if err != nil {
				return nil, err
			}
			docValuesSkipIndex = index.DocValuesSkipIndexType(skipByte)
		} else {
			docValuesSkipIndex = index.DocValuesSkipIndexTypeNone
		}

		dvGen, err := store.ReadInt64(checksumIn)
		if err != nil {
			return nil, err
		}

		attributes, err := store.ReadMapOfStrings(checksumIn)
		if err != nil {
			return nil, err
		}

		pointDataDimensionCount, err := store.ReadVInt(checksumIn)
		if err != nil {
			return nil, err
		}
		var pointNumBytes int32
		pointIndexDimensionCount := pointDataDimensionCount
		if pointDataDimensionCount != 0 {
			pointIndexDimensionCount, err = store.ReadVInt(checksumIn)
			if err != nil {
				return nil, err
			}
			pointNumBytes, err = store.ReadVInt(checksumIn)
			if err != nil {
				return nil, err
			}
		}

		vectorDimension, err := store.ReadVInt(checksumIn)
		if err != nil {
			return nil, err
		}
		vectorEncodingByte, err := checksumIn.ReadByte()
		if err != nil {
			return nil, err
		}
		vectorEncoding := index.VectorEncoding(vectorEncodingByte)

		vectorDistFuncByte, err := checksumIn.ReadByte()
		if err != nil {
			return nil, err
		}
		vectorDistFunc := index.VectorSimilarityFunction(vectorDistFuncByte)

		opts := index.FieldInfoOptions{
			IndexOptions:             indexOptions,
			DocValuesType:            docValuesType,
			DocValuesSkipIndexType:   docValuesSkipIndex,
			DocValuesGen:             dvGen,
			StoreTermVectors:         storeTermVector,
			OmitNorms:                omitNorms,
			StoreTermVectorPayloads:  storePayloads, // Payloads are stored with TV positions
			PointDimensionCount:      int(pointDataDimensionCount),
			PointIndexDimensionCount: int(pointIndexDimensionCount),
			PointNumBytes:            int(pointNumBytes),
			VectorDimension:          int(vectorDimension),
			VectorEncoding:           vectorEncoding,
			VectorSimilarityFunction: vectorDistFunc,
			IsSoftDeletesField:       isSoftDeletesField,
			IsParentField:            isParentField,
		}

		// Payloads in bits means the field has payloads in postings
		// StoreTermVectorPayloads in FieldInfoOptions usually means payloads in TV.
		// Lucene FieldInfo has hasPayloads() which is separate from storeTermVectorPayloads.
		// Actually, let's check FieldInfo constructor in Java.

		fib := index.NewFieldInfoBuilder(name, int(fieldNumber)).
			SetIndexOptions(opts.IndexOptions).
			SetDocValuesType(opts.DocValuesType).
			SetDocValuesSkipIndexType(opts.DocValuesSkipIndexType).
			SetDocValuesGen(opts.DocValuesGen).
			SetStored(opts.Stored).
			SetTokenized(opts.Tokenized).
			SetOmitNorms(opts.OmitNorms).
			SetStoreTermVectors(opts.StoreTermVectors).
			SetStoreTermVectorPositions(opts.StoreTermVectorPositions).
			SetStoreTermVectorOffsets(opts.StoreTermVectorOffsets).
			SetStoreTermVectorPayloads(opts.StoreTermVectorPayloads).
			SetPointDimensions(opts.PointDimensionCount, opts.PointIndexDimensionCount, opts.PointNumBytes).
			SetVectorAttributes(opts.VectorDimension, opts.VectorEncoding, opts.VectorSimilarityFunction).
			SetSoftDeletesField(opts.IsSoftDeletesField).
			SetParentField(opts.IsParentField)

		for k, v := range attributes {
			fib.SetAttribute(k, v)
		}
		builder.Add(fib.Build())
	}

	_, err = CheckFooter(checksumIn)
	if err != nil {
		return nil, err
	}

	infos = builder.Build()
	return infos, nil
}

func (f *Lucene94FieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	fileName := GetSegmentFileName(segmentInfo.Name(), segmentSuffix, "fnm")
	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}

	checksumOut := store.NewChecksumIndexOutput(out)
	defer checksumOut.Close()

	err = WriteIndexHeader(checksumOut, fiCodecName, fiFormatCurrent, segmentInfo.GetID(), segmentSuffix)
	if err != nil {
		return err
	}

	err = store.WriteVInt(checksumOut, int32(infos.Size()))
	if err != nil {
		return err
	}

	iter := infos.Iterator()
	for iter.HasNext() {
		fi := iter.Next()
		err = store.WriteString(checksumOut, fi.Name())
		if err != nil {
			return err
		}
		err = store.WriteVInt(checksumOut, int32(fi.Number()))
		if err != nil {
			return err
		}

		var bits byte
		if fi.StoreTermVectors() {
			bits |= fiStoreTermvector
		}
		if fi.OmitNorms() {
			bits |= fiOmitNorms
		}
		if fi.HasPayloads() {
			bits |= fiStorePayloads
		}
		if fi.IsSoftDeletesField() {
			bits |= fiSoftDeletesField
		}
		if fi.IsParentField() {
			bits |= fiParentFieldField
		}
		err = checksumOut.WriteByte(bits)
		if err != nil {
			return err
		}

		err = checksumOut.WriteByte(byte(fi.IndexOptions()))
		if err != nil {
			return err
		}

		err = checksumOut.WriteByte(byte(fi.DocValuesType()))
		if err != nil {
			return err
		}

		err = checksumOut.WriteByte(byte(fi.DocValuesSkipIndexType()))
		if err != nil {
			return err
		}

		err = store.WriteInt64(checksumOut, fi.DocValuesGen())
		if err != nil {
			return err
		}

		err = store.WriteMapOfStrings(checksumOut, fi.GetAttributes())
		if err != nil {
			return err
		}

		err = store.WriteVInt(checksumOut, int32(fi.PointDimensionCount()))
		if err != nil {
			return err
		}
		if fi.PointDimensionCount() != 0 {
			err = store.WriteVInt(checksumOut, int32(fi.PointIndexDimensionCount()))
			if err != nil {
				return err
			}
			err = store.WriteVInt(checksumOut, int32(fi.PointNumBytes()))
			if err != nil {
				return err
			}
		}

		err = store.WriteVInt(checksumOut, int32(fi.VectorDimension()))
		if err != nil {
			return err
		}
		err = checksumOut.WriteByte(byte(fi.VectorEncoding()))
		if err != nil {
			return err
		}
		err = checksumOut.WriteByte(byte(fi.VectorSimilarityFunction()))
		if err != nil {
			return err
		}
	}

	return WriteFooter(checksumOut)
}

// Lucene104FieldInfosFormat is a wrapper for Lucene94FieldInfosFormat
type Lucene104FieldInfosFormat struct {
	*Lucene94FieldInfosFormat
}

func NewLucene104FieldInfosFormat() *Lucene104FieldInfosFormat {
	return &Lucene104FieldInfosFormat{
		Lucene94FieldInfosFormat: NewLucene94FieldInfosFormat(),
	}
}

func (f *Lucene104FieldInfosFormat) Name() string {
	return "Lucene104FieldInfosFormat"
}
