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
//
// FieldInfos are stored in .fnm files and contain metadata about
// all fields in the index, including their types, options, and attributes.
type FieldInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads field infos from the given directory and file name.
	// Returns the FieldInfos or an error if reading fails.
	Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error)

	// Write writes field infos to the given output.
	// Returns an error if writing fails.
	Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error
}

// BaseFieldInfosFormat provides common functionality.
type BaseFieldInfosFormat struct {
	name string
}

// NewBaseFieldInfosFormat creates a new BaseFieldInfosFormat.
func NewBaseFieldInfosFormat(name string) *BaseFieldInfosFormat {
	return &BaseFieldInfosFormat{name: name}
}

// Name returns the format name.
func (f *BaseFieldInfosFormat) Name() string {
	return f.name
}

// Read reads field infos (must be implemented by subclasses).
func (f *BaseFieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error) {
	return nil, fmt.Errorf("Read not implemented")
}

// Write writes field infos (must be implemented by subclasses).
func (f *BaseFieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error {
	return fmt.Errorf("Write not implemented")
}

// Lucene104FieldInfosFormat is the Lucene 10.4 field infos format.
//
// File format:
//   - Header: Lucene codec header with version
//   - Number of fields (VInt)
//   - For each field:
//   - Field name (String)
//   - Field number (VInt)
//   - Index options (Byte)
//   - DocValues type (Byte)
//   - Bits: stored, tokenized, omitNorms, storeTermVectors,
//     storeTermVectorPositions, storeTermVectorOffsets, storeTermVectorPayloads
//   - Number of attributes (VInt)
//   - For each attribute: key (String), value (String)
//   - Footer: checksum
type Lucene104FieldInfosFormat struct {
	*BaseFieldInfosFormat
}

// NewLucene104FieldInfosFormat creates a new Lucene104FieldInfosFormat.
func NewLucene104FieldInfosFormat() *Lucene104FieldInfosFormat {
	return &Lucene104FieldInfosFormat{
		BaseFieldInfosFormat: NewBaseFieldInfosFormat("Lucene104FieldInfosFormat"),
	}
}

// Read reads field infos from the given directory.
func (f *Lucene104FieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string) (*index.FieldInfos, error) {
	fileName := f.getFileName(segmentInfo.Name(), segmentSuffix)

	in, err := dir.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("opening field infos file: %w", err)
	}
	defer in.Close()

	// Read header
	if err := f.readHeader(in); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Read number of fields
	numFields, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("reading number of fields: %w", err)
	}

	if numFields < 0 {
		return nil, fmt.Errorf("invalid number of fields: %d", numFields)
	}

	infos := index.NewFieldInfos()

	for i := int32(0); i < numFields; i++ {
		fieldInfo, err := f.readFieldInfo(in)
		if err != nil {
			return nil, fmt.Errorf("reading field info %d: %w", i, err)
		}
		if err := infos.Add(fieldInfo); err != nil {
			return nil, fmt.Errorf("adding field info %d: %w", i, err)
		}
	}

	// Read footer (checksum)
	if err := f.readFooter(in); err != nil {
		return nil, fmt.Errorf("reading footer: %w", err)
	}

	infos.Freeze()
	return infos, nil
}

// Write writes field infos to the given directory.
func (f *Lucene104FieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos) error {
	fileName := f.getFileName(segmentInfo.Name(), segmentSuffix)

	out, err := dir.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return fmt.Errorf("creating field infos file: %w", err)
	}
	defer out.Close()

	// Write header
	if err := f.writeHeader(out); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Write number of fields
	if err := store.WriteVInt(out, int32(infos.Size())); err != nil {
		return fmt.Errorf("writing number of fields: %w", err)
	}

	// Write each field info
	iter := infos.Iterator()
	for fieldInfo := iter.Next(); fieldInfo != nil; fieldInfo = iter.Next() {
		if err := f.writeFieldInfo(out, fieldInfo); err != nil {
			return fmt.Errorf("writing field info: %w", err)
		}
	}

	// Write footer
	if err := f.writeFooter(out); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}

	return nil
}

// readHeader reads the file header.
func (f *Lucene104FieldInfosFormat) readHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("reading magic: %w", err)
	}
	if magic != 0x3163614c { // 'Lac1' in little endian
		return fmt.Errorf("invalid magic number: %x", magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("reading version: %w", err)
	}
	if version != 0 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// writeHeader writes the file header.
func (f *Lucene104FieldInfosFormat) writeHeader(out store.IndexOutput) error {
	// Write magic number 'Lac1'
	if err := store.WriteUint32(out, 0x3163614c); err != nil {
		return err
	}
	// Write version
	return store.WriteUint32(out, 0)
}

// readFooter reads the file footer (checksum).
func (f *Lucene104FieldInfosFormat) readFooter(in store.IndexInput) error {
	// For now, just read and ignore checksum
	// In a full implementation, we would verify the checksum
	_, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}
	return nil
}

// writeFooter writes the file footer (checksum).
func (f *Lucene104FieldInfosFormat) writeFooter(out store.IndexOutput) error {
	// For now, just write a dummy checksum
	// In a full implementation, we would compute a real checksum
	return store.WriteUint32(out, 0)
}

// readFieldInfo reads a single field info.
func (f *Lucene104FieldInfosFormat) readFieldInfo(in store.IndexInput) (*index.FieldInfo, error) {
	// Read field name
	name, err := store.ReadString(in)
	if err != nil {
		return nil, fmt.Errorf("reading field name: %w", err)
	}

	// Read field number
	number, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("reading field number: %w", err)
	}

	// Read index options
	indexOptionsByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading index options: %w", err)
	}
	indexOptions := index.IndexOptions(indexOptionsByte)

	// Read doc values type
	docValuesTypeByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading doc values type: %w", err)
	}
	docValuesType := index.DocValuesType(docValuesTypeByte)

	// Read bits
	bits, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading bits: %w", err)
	}

	stored := (bits & 0x01) != 0
	tokenized := (bits & 0x02) != 0
	omitNorms := (bits & 0x04) != 0
	storeTermVectors := (bits & 0x08) != 0
	storeTermVectorPositions := (bits & 0x10) != 0
	storeTermVectorOffsets := (bits & 0x20) != 0
	storeTermVectorPayloads := (bits & 0x40) != 0

	opts := index.FieldInfoOptions{
		IndexOptions:             indexOptions,
		DocValuesType:            docValuesType,
		Stored:                   stored,
		Tokenized:                tokenized,
		OmitNorms:                omitNorms,
		StoreTermVectors:         storeTermVectors,
		StoreTermVectorPositions: storeTermVectorPositions,
		StoreTermVectorOffsets:   storeTermVectorOffsets,
		StoreTermVectorPayloads:  storeTermVectorPayloads,
	}

	fieldInfo := index.NewFieldInfo(name, int(number), opts)

	// Read attributes
	numAttrs, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("reading number of attributes: %w", err)
	}

	for i := int32(0); i < numAttrs; i++ {
		key, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading attribute key: %w", err)
		}
		value, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading attribute value: %w", err)
		}
		fieldInfo.PutAttribute(key, value)
	}

	return fieldInfo, nil
}

// writeFieldInfo writes a single field info.
func (f *Lucene104FieldInfosFormat) writeFieldInfo(out store.IndexOutput, fieldInfo *index.FieldInfo) error {
	// Write field name
	if err := store.WriteString(out, fieldInfo.Name()); err != nil {
		return fmt.Errorf("writing field name: %w", err)
	}

	// Write field number
	if err := store.WriteVInt(out, int32(fieldInfo.Number())); err != nil {
		return fmt.Errorf("writing field number: %w", err)
	}

	// Write index options
	if err := out.WriteByte(byte(fieldInfo.IndexOptions())); err != nil {
		return fmt.Errorf("writing index options: %w", err)
	}

	// Write doc values type
	if err := out.WriteByte(byte(fieldInfo.DocValuesType())); err != nil {
		return fmt.Errorf("writing doc values type: %w", err)
	}

	// Write bits
	var bits byte
	if fieldInfo.IsStored() {
		bits |= 0x01
	}
	if fieldInfo.IsTokenized() {
		bits |= 0x02
	}
	if fieldInfo.OmitNorms() {
		bits |= 0x04
	}
	if fieldInfo.StoreTermVectors() {
		bits |= 0x08
	}
	if fieldInfo.StoreTermVectorPositions() {
		bits |= 0x10
	}
	if fieldInfo.StoreTermVectorOffsets() {
		bits |= 0x20
	}
	if fieldInfo.StoreTermVectorPayloads() {
		bits |= 0x40
	}
	if err := out.WriteByte(bits); err != nil {
		return fmt.Errorf("writing bits: %w", err)
	}

	// Write attributes
	attrs := fieldInfo.GetAttributes()
	if err := store.WriteVInt(out, int32(len(attrs))); err != nil {
		return fmt.Errorf("writing number of attributes: %w", err)
	}
	for key, value := range attrs {
		if err := store.WriteString(out, key); err != nil {
			return fmt.Errorf("writing attribute key: %w", err)
		}
		if err := store.WriteString(out, value); err != nil {
			return fmt.Errorf("writing attribute value: %w", err)
		}
	}

	return nil
}

// getFileName returns the field infos file name.
func (f *Lucene104FieldInfosFormat) getFileName(segmentName, segmentSuffix string) string {
	if segmentSuffix != "" {
		return fmt.Sprintf("_%s_%s.fnm", segmentName[1:], segmentSuffix)
	}
	return fmt.Sprintf("_%s.fnm", segmentName[1:])
}
