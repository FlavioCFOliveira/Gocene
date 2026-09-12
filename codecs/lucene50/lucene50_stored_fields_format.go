// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

type Mode int

const (
	BestSpeed Mode = iota
	BestCompression
)

func (m Mode) String() string {
	if m == BestCompression {
		return "BEST_COMPRESSION"
	}
	return "BEST_SPEED"
}

// Lucene50StoredFieldsFormat is a read-only port of Lucene 5.0 stored fields.
type Lucene50StoredFieldsFormat struct {
	mode Mode
}

func NewLucene50StoredFieldsFormat(mode Mode) *Lucene50StoredFieldsFormat {
	return &Lucene50StoredFieldsFormat{mode: mode}
}

func (f *Lucene50StoredFieldsFormat) Name() string {
	return "Lucene50StoredFieldsFormat"
}

func (f *Lucene50StoredFieldsFormat) FieldsReader(dir store.Directory, si *index.SegmentInfo, fn *index.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	// In Lucene, the mode is retrieved from segment attributes.
	val := si.GetAttribute("Lucene50StoredFieldsFormat.mode")
	if val == "" {
		return nil, fmt.Errorf("missing value for Lucene50StoredFieldsFormat.mode for segment: %s", si.Name())
	}

	// Map string to mode
	mode := BestSpeed
	if val == "BEST_COMPRESSION" {
		mode = BestCompression
	}

	// This would normally delegate to a compressing reader.
	return nil, fmt.Errorf("Lucene50StoredFieldsReader (mode %v) not yet implemented", mode)
}

func (f *Lucene50StoredFieldsFormat) FieldsWriter(dir store.Directory, si *index.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	return nil, fmt.Errorf("Lucene50StoredFieldsFormat: Old codecs may only be used for reading")
}
