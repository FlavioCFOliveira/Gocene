// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package lucene60 implements the backward-codec reader for segments written
// by Apache Lucene 6.0.  All classes in this package are read-only; the write
// path exists solely to support in-place DocValues updates on legacy segments
// (as in the Java original).
//
// Wire format: Lucene 6.0 persisted multi-byte integers in big-endian order.
// Every file opened here is therefore wrapped in an EndiannessReverser from
// backward_codecs/store so that standard Gocene codec utilities (which assume
// little-endian) work transparently.
package lucene60

import (
	"fmt"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Wire-format constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	fnmExtension       = "fnm"
	fnmCodecName       = "Lucene60FieldInfos"
	fnmFormatStart     = 0
	fnmFormatSoftDel   = 1
	fnmFormatSelective = 2
	fnmFormatCurrent   = fnmFormatSelective

	// FieldBits flags.
	storeTermVector = byte(0x1)
	omitNorms       = byte(0x2)
	storePayloads   = byte(0x4)
	softDeletesFlag = byte(0x8)
)

// ─────────────────────────────────────────────────────────────────────────────
// Lucene60FieldInfosFormat
// ─────────────────────────────────────────────────────────────────────────────

// Lucene60FieldInfosFormat reads and writes field metadata for segments
// created by Apache Lucene 6.0.
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60FieldInfosFormat
// (Lucene 10.4.0).  The format is identical to Lucene 9.4's except that:
//   - There are no VectorDimension / VectorEncoding / VectorSimilarityFunction
//     fields (those were added later).
//   - There is no DocValuesSkipIndex field.
//   - Files are stored big-endian; the EndiannessReverser wrappers from
//     backward_codecs/store handle the byte-swap transparently.
type Lucene60FieldInfosFormat struct{}

// NewLucene60FieldInfosFormat returns a Lucene60FieldInfosFormat.
func NewLucene60FieldInfosFormat() *Lucene60FieldInfosFormat {
	return &Lucene60FieldInfosFormat{}
}

// Name implements codecs.FieldInfosFormat.
func (f *Lucene60FieldInfosFormat) Name() string { return "Lucene60FieldInfosFormat" }

// Read decodes field infos from a legacy Lucene 6.0 .fnm file.
func (f *Lucene60FieldInfosFormat) Read(
	dir store.Directory,
	segmentInfo *index.SegmentInfo,
	segmentSuffix string,
	context store.IOContext,
) (*index.FieldInfos, error) {
	fileName := codecs.GetSegmentFileName(segmentInfo.Name(), segmentSuffix, fnmExtension)

	checksumIn, err := bcstore.OpenChecksumInput(dir, fileName, context)
	if err != nil {
		return nil, err
	}

	infos, readErr := readFieldInfosFrom(checksumIn, segmentInfo, segmentSuffix)
	closeErr := checksumIn.Close()
	if readErr != nil {
		return nil, readErr
	}
	return infos, closeErr
}

// Write encodes field infos to a legacy Lucene 6.0 .fnm file.
// Although Lucene 6.0 is a read-only format, write support is preserved for
// in-place DocValues updates on existing segments.
func (f *Lucene60FieldInfosFormat) Write(
	dir store.Directory,
	segmentInfo *index.SegmentInfo,
	segmentSuffix string,
	infos *index.FieldInfos,
	context store.IOContext,
) error {
	fileName := codecs.GetSegmentFileName(segmentInfo.Name(), segmentSuffix, fnmExtension)

	// Stack: rawOutput → checksumOutput → reverserOutput
	// The reverserOutput.GetChecksum() delegates to checksumOutput so that
	// codecs.WriteFooter can record the running CRC32.
	rawOut, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}
	checksumOut := store.NewChecksumIndexOutput(rawOut)
	out := bcstore.NewEndiannessReverserIndexOutput(checksumOut)

	writeErr := writeFieldInfosTo(out, segmentInfo, segmentSuffix, infos)
	closeErr := out.Close()
	if writeErr != nil {
		_ = dir.DeleteFile(fileName)
		return writeErr
	}
	if closeErr != nil {
		_ = dir.DeleteFile(fileName)
		return closeErr
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// internal read / write helpers
// ─────────────────────────────────────────────────────────────────────────────

// checksumIndexInputLike is the subset of methods that the
// EndiannessReverserChecksumIndexInput exposes that we need for codec header
// and footer verification.  We use this instead of *store.ChecksumIndexInput
// so we can pass our reversing wrapper directly.
type checksumIndexInputLike interface {
	store.IndexInput
	GetChecksum() uint32
}

func readFieldInfosFrom(in checksumIndexInputLike, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error) {
	version, err := codecs.CheckIndexHeader(in, fnmCodecName, fnmFormatStart, fnmFormatCurrent, segmentInfo.GetID(), segmentSuffix)
	if err != nil {
		return nil, err
	}

	size, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}

	builder := index.NewFieldInfosBuilder()
	for i := int32(0); i < size; i++ {
		fi, err := readOneFieldInfo(in, version)
		if err != nil {
			return nil, err
		}
		builder.Add(fi)
	}

	if err := checkFooterWithChecksum(in); err != nil {
		return nil, err
	}
	return builder.Build(), nil
}

func readOneFieldInfo(in store.IndexInput, version int32) (*index.FieldInfo, error) {
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
	hasTermVectors := bits&storeTermVector != 0
	hasOmitNorms := bits&omitNorms != 0
	hasPayloads := bits&storePayloads != 0
	isSoftDeletes := bits&softDeletesFlag != 0

	indexOptionsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	indexOpts, err := decodeIndexOptions(indexOptionsByte)
	if err != nil {
		return nil, err
	}

	dvByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	dvType, err := decodeDocValuesType(dvByte)
	if err != nil {
		return nil, err
	}

	dvGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	attrs, err := store.ReadMapOfStrings(in)
	if err != nil {
		return nil, err
	}

	pointDataDimCount, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	pointIndexDimCount := pointDataDimCount
	var pointNumBytes int32
	if pointDataDimCount != 0 {
		if version >= fnmFormatSelective {
			pointIndexDimCount, err = store.ReadVInt(in)
			if err != nil {
				return nil, err
			}
		}
		pointNumBytes, err = store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
	}

	fib := index.NewFieldInfoBuilder(name, int(fieldNumber)).
		SetIndexOptions(indexOpts).
		SetDocValuesType(dvType).
		SetDocValuesGen(dvGen).
		SetOmitNorms(hasOmitNorms).
		SetStoreTermVectors(hasTermVectors).
		SetPointDimensions(int(pointDataDimCount), int(pointIndexDimCount), int(pointNumBytes)).
		SetSoftDeletesField(isSoftDeletes)
	if hasPayloads {
		fib.SetStoreTermVectorPayloads(true)
	}
	for k, v := range attrs {
		fib.SetAttribute(k, v)
	}
	return fib.Build(), nil
}

func writeFieldInfosTo(out store.IndexOutput, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error {
	if err := codecs.WriteIndexHeader(out, fnmCodecName, fnmFormatCurrent, segmentInfo.GetID(), segmentSuffix); err != nil {
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

		var fieldBits byte
		if fi.StoreTermVectors() {
			fieldBits |= storeTermVector
		}
		if fi.OmitNorms() {
			fieldBits |= omitNorms
		}
		if fi.HasPayloads() {
			fieldBits |= storePayloads
		}
		if fi.IsSoftDeletesField() {
			fieldBits |= softDeletesFlag
		}
		if err := out.WriteByte(fieldBits); err != nil {
			return err
		}

		iob, err := encodeIndexOptions(fi.IndexOptions())
		if err != nil {
			return err
		}
		if err := out.WriteByte(iob); err != nil {
			return err
		}

		dvb, err := encodeDocValuesType(fi.DocValuesType())
		if err != nil {
			return err
		}
		if err := out.WriteByte(dvb); err != nil {
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
	}
	return codecs.WriteFooter(out)
}

// checkFooterWithChecksum is a variant of codecs.CheckFooter that works with
// our checksumIndexInputLike interface instead of the concrete
// *store.ChecksumIndexInput type.
func checkFooterWithChecksum(in checksumIndexInputLike) error {
	// Verify footer magic and algorithmID.
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16
	if remaining < footerLen {
		return fmt.Errorf("misplaced codec footer (truncated?): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("misplaced codec footer (extended?): remaining=%d", remaining)
	}

	magic, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("codec footer mismatch: actual=%x expected=%x", magic, footerMagic)
	}
	alg, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if alg != 0 {
		return fmt.Errorf("codec footer mismatch: unknown algorithmID %d", alg)
	}

	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := store.ReadInt64(in)
	if err != nil {
		return err
	}
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum failed: actual=%x expected=%x", actualChecksum, expectedChecksum)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// enum encoders / decoders  (mirrors codecs package, for FieldInfo types used
// only in the Lucene 6.0 format)
// ─────────────────────────────────────────────────────────────────────────────

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

// ─────────────────────────────────────────────────────────────────────────────
// Lucene60PointsFormat
// ─────────────────────────────────────────────────────────────────────────────

const (
	pointsDataCodecName  = "Lucene60PointsFormatData"
	pointsMetaCodecName  = "Lucene60PointsFormatMeta"
	pointsDataExtension  = "dim"
	pointsIndexExtension = "dii"

	pointsDataVersionStart    = 0
	pointsDataVersionCurrent  = pointsDataVersionStart
	pointsIndexVersionStart   = 0
	pointsIndexVersionCurrent = pointsIndexVersionStart
)

// Lucene60PointsFormat is the PointsFormat for Apache Lucene 6.0 segments.
// Writing is intentionally not supported (legacy read-only format).
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60PointsFormat
// (Lucene 10.4.0).
type Lucene60PointsFormat struct{}

// NewLucene60PointsFormat returns a Lucene60PointsFormat.
func NewLucene60PointsFormat() *Lucene60PointsFormat { return &Lucene60PointsFormat{} }

// Name implements codecs.PointsFormat.
func (f *Lucene60PointsFormat) Name() string { return "Lucene60PointsFormat" }

// FieldsWriter returns an error — Lucene 6.0 is a read-only format.
func (f *Lucene60PointsFormat) FieldsWriter(_ *codecs.SegmentWriteState) (codecs.PointsWriter, error) {
	return nil, fmt.Errorf("Lucene60PointsFormat: write not supported on legacy format")
}

// FieldsReader opens the Lucene 6.0 BKD points files for reading.
func (f *Lucene60PointsFormat) FieldsReader(state *codecs.SegmentReadState) (codecs.PointsReader, error) {
	return newLucene60PointsReader(state)
}

// ─────────────────────────────────────────────────────────────────────────────
// Lucene60PointsReader
// ─────────────────────────────────────────────────────────────────────────────

// Lucene60PointsReader reads point values written by Lucene 6.0.
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60PointsReader
// (Lucene 10.4.0).
type Lucene60PointsReader struct {
	dataIn    store.IndexInput
	readState *codecs.SegmentReadState
	// fieldToFP maps field number to the file pointer in dataIn where that
	// field's BKD tree begins.
	fieldToFP map[int]int64
}

func newLucene60PointsReader(state *codecs.SegmentReadState) (*Lucene60PointsReader, error) {
	ctx := store.IOContext{Context: store.ContextRead}

	// ── Read index file (.dii) ──────────────────────────────────────────────
	indexFile := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, pointsIndexExtension)
	indexIn, err := bcstore.OpenChecksumInput(state.Directory, indexFile, ctx)
	if err != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: open index %s: %w", indexFile, err)
	}

	fieldToFP := make(map[int]int64)
	readIndexErr := func() error {
		if _, err := codecs.CheckIndexHeader(
			indexIn,
			pointsMetaCodecName,
			pointsIndexVersionStart, pointsIndexVersionCurrent,
			state.SegmentInfo.GetID(),
			state.SegmentSuffix,
		); err != nil {
			return err
		}
		count, err := store.ReadVInt(indexIn)
		if err != nil {
			return err
		}
		for i := int32(0); i < count; i++ {
			fieldNumber, err := store.ReadVInt(indexIn)
			if err != nil {
				return err
			}
			fp, err := store.ReadVLong(indexIn)
			if err != nil {
				return err
			}
			fieldToFP[int(fieldNumber)] = fp
		}
		return checkFooterWithChecksum(indexIn)
	}()
	if cerr := indexIn.Close(); cerr != nil && readIndexErr == nil {
		readIndexErr = cerr
	}
	if readIndexErr != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: read index: %w", readIndexErr)
	}

	// ── Open data file (.dim) ──────────────────────────────────────────────
	dataFile := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, pointsDataExtension)
	dataIn, err := bcstore.OpenInput(state.Directory, dataFile, ctx)
	if err != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: open data %s: %w", dataFile, err)
	}

	if _, err := codecs.CheckIndexHeader(
		dataIn,
		pointsDataCodecName,
		pointsDataVersionStart, pointsDataVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("Lucene60PointsReader: check data header: %w", err)
	}

	return &Lucene60PointsReader{
		dataIn:    dataIn,
		readState: state,
		fieldToFP: fieldToFP,
	}, nil
}

// GetFilePointerForField returns the data file offset for the given field
// number, or -1 if the field has no indexed points.
func (r *Lucene60PointsReader) GetFilePointerForField(fieldNumber int) int64 {
	fp, ok := r.fieldToFP[fieldNumber]
	if !ok {
		return -1
	}
	return fp
}

// CheckIntegrity verifies the CRC32 checksum of the data file.
func (r *Lucene60PointsReader) CheckIntegrity() error {
	_, err := codecs.ChecksumEntireFile(r.dataIn)
	return err
}

// Close releases the data file handle.
func (r *Lucene60PointsReader) Close() error {
	if r.dataIn != nil {
		err := r.dataIn.Close()
		r.dataIn = nil
		r.fieldToFP = nil
		return err
	}
	return nil
}

var _ codecs.PointsReader = (*Lucene60PointsReader)(nil)
