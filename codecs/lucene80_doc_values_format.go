// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
)

// Lucene80DocValuesFormat implements the Lucene 8.0 DocValues format.
type Lucene80DocValuesFormat struct {
	mode Mode
}

// Mode configuration option for doc values.
type Mode int

const (
	// ModeBestSpeed trades compression ratio for retrieval speed.
	ModeBestSpeed Mode = iota
	// ModeBestCompression trades retrieval speed for compression ratio.
	ModeBestCompression
)

const (
	// ModeKey is the attribute key for compression mode.
	ModeKey = "Lucene80DocValuesFormat.mode"
	// DataCodec is the codec name for doc values data.
	DataCodec = "Lucene80DocValuesData"
	// DataExtension is the file extension for doc values data.
	DataExtension = "dvd"
	// MetaCodec is the codec name for doc values metadata.
	MetaCodec = "Lucene80DocValuesMetadata"
	// MetaExtension is the file extension for doc values metadata.
	MetaExtension = "dvm"
	// VersionStart is the starting version.
	VersionStart = 0
	// VersionBinCompressed is the version where binary compression was introduced.
	VersionBinCompressed = 1
	// VersionConfigurableCompression is the version where compression became configurable.
	VersionConfigurableCompression = 2
	// VersionCurrent is the current version of the Lucene 8.0 format.
	VersionCurrent = VersionConfigurableCompression

	// Numeric values
	NumericType = byte(0)
	BinaryType = byte(1)
	SortedType = byte(2)
	SortedSetType = byte(3)
	SortedNumericType = byte(4)

	// Block shifts and sizes
	DirectMonotonicBlockShift = 16
	NumericBlockShift = 14
	NumericBlockSize = 1 << NumericBlockShift
	BinaryBlockShift = 5
	BinaryDocsPerCompressedBlock = 1 << BinaryBlockShift
	TermsDictBlockShift = 4
	TermsDictBlockSize = 1 << TermsDictBlockShift
	TermsDictBlockMask = TermsDictBlockSize - 1
	TermsDictBlockCompressionThreshold = 32
	TermsDictBlockLZ4Shift = 6
	TermsDictBlockLZ4Size = 1 << TermsDictBlockLZ4Shift
	TermsDictBlockLZ4Mask = TermsDictBlockLZ4Size - 1
	TermsDictCompressorLZ4Code = 1
	TermsDictBlockLZ4Code = TermsDictBlockLZ4Shift<<16 | TermsDictCompressorLZ4Code
	TermsDictReverseIndexShift = 10
	TermsDictReverseIndexSize = 1 << TermsDictReverseIndexShift
	TermsDictReverseIndexMask = TermsDictReverseIndexSize - 1
)

// NewLucene80DocValuesFormat builds a new Lucene80DocValuesFormat.
func NewLucene80DocValuesFormat() *Lucene80DocValuesFormat {
	return NewLucene80DocValuesFormatWithMode(ModeBestSpeed)
}

// NewLucene80DocValuesFormatWithMode builds a Lucene80DocValuesFormat with a specific mode.
func NewLucene80DocValuesFormatWithMode(mode Mode) *Lucene80DocValuesFormat {
	return &Lucene80DocValuesFormat{mode: mode}
}

func (f *Lucene80DocValuesFormat) Name() string {
	return "Lucene80"
}

func (f *Lucene80DocValuesFormat) FieldsConsumer(state SegmentWriteState) (DocValuesConsumer, error) {
	return NewLucene80DocValuesConsumer(
		state, DataCodec, DataExtension, MetaCodec, MetaExtension, f.mode), nil
}

func (f *Lucene80DocValuesFormat) FieldsProducer(state SegmentReadState) (DocValuesProducer, error) {
	return NewLucene80DocValuesProducer(
		state, DataCodec, DataExtension, MetaCodec, MetaExtension), nil
}

func (f *Lucene80DocValuesFormat) String() string {
	return fmt.Sprintf("Lucene80DocValuesFormat(mode=%v)", f.mode)
}
