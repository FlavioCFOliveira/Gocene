// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene87StoredFieldsFormat is a reader-only port of Lucene 8.7 stored fields format.
type Lucene87StoredFieldsFormat struct {
	mode Mode
}

type Mode int

const (
	ModeBestSpeed Mode = iota
	ModeBestCompression
)

const ModeKey = "Lucene87StoredFieldsFormat.mode"

func NewLucene87StoredFieldsFormat(mode Mode) *Lucene87StoredFieldsFormat {
	return &Lucene87StoredFieldsFormat{mode: mode}
}

func (f *Lucene87StoredFieldsFormat) Name() string {
	return "Lucene87StoredFieldsFormat"
}

func (f *Lucene87StoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	value := segmentInfo.GetAttribute(ModeKey)
	if value == "" {
		return nil, fmt.Errorf("missing value for %s for segment: %s", ModeKey, segmentInfo.Name)
	}

	var mode Mode
	switch value {
	case "BEST_SPEED":
		mode = ModeBestSpeed
	case "BEST_COMPRESSION":
		mode = ModeBestCompression
	default:
		return nil, fmt.Errorf("unknown mode: %s", value)
	}

	return f.impl(mode).FieldsReader(dir, segmentInfo, fieldInfos, context)
}

func (f *Lucene87StoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	return nil, fmt.Errorf("old codecs may only be used for reading")
}

func (f *Lucene87StoredFieldsFormat) impl(mode Mode) *CompressingStoredFieldsFormat {
	switch mode {
	case ModeBestSpeed:
		return NewCompressingStoredFieldsFormat(CompressionModeLZ4Fast, 10*8*1024, 10)
	case ModeBestCompression:
		return NewCompressingStoredFieldsFormat(CompressionModeDeflate, 10*48*1024, 10)
	default:
		panic("unsupported mode")
	}
}
