// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Lucene80DVMode is the compression-mode selector for Lucene80DocValuesFormat.
//
// Port of Lucene80DocValuesFormat.Mode (Lucene 10.4.0).
type Lucene80DVMode int

const (
	// Lucene80DVModeBestSpeed optimises for retrieval speed over compression ratio.
	Lucene80DVModeBestSpeed Lucene80DVMode = iota
	// Lucene80DVModeBestCompression optimises for compression ratio over retrieval speed.
	Lucene80DVModeBestCompression
)

// Lucene80DocValuesFormat is the Lucene 8.0 doc values format.
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesFormat
// (Lucene 10.4.0).
//
// DEFERRED: FieldsConsumer returns an error until Lucene80DocValuesConsumer
// is ported (task 3172). FieldsProducer opens the .dvm/.dvd files and
// validates their headers; per-field decode is deferred.
type Lucene80DocValuesFormat struct {
	*codecs.BaseDocValuesFormat
	mode Lucene80DVMode
}

// NewLucene80DocValuesFormat creates a new format with the default mode
// (BEST_SPEED).
//
// Port of Lucene80DocValuesFormat() (Lucene 10.4.0).
func NewLucene80DocValuesFormat() *Lucene80DocValuesFormat {
	return NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestSpeed)
}

// NewLucene80DocValuesFormatWithMode creates a new format with an explicit
// compression mode.
//
// Port of Lucene80DocValuesFormat(Mode) (Lucene 10.4.0).
func NewLucene80DocValuesFormatWithMode(mode Lucene80DVMode) *Lucene80DocValuesFormat {
	return &Lucene80DocValuesFormat{
		BaseDocValuesFormat: codecs.NewBaseDocValuesFormat("Lucene80"),
		mode:                mode,
	}
}

// Mode returns the configured compression mode.
func (f *Lucene80DocValuesFormat) Mode() Lucene80DVMode { return f.mode }

// FieldsConsumer returns a consumer for writing doc values.
//
// Port of Lucene80DocValuesFormat.fieldsConsumer (Lucene 10.4.0).
func (f *Lucene80DocValuesFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.DocValuesConsumer, error) {
	return NewLucene80DocValuesConsumer(
		state,
		lucene80DVDataCodec,
		lucene80DVDataExt,
		lucene80DVMetaCodec,
		lucene80DVMetaExt,
		f.mode,
	)
}

// FieldsProducer returns a producer for reading doc values.
//
// Port of Lucene80DocValuesFormat.fieldsProducer (Lucene 10.4.0).
func (f *Lucene80DocValuesFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.DocValuesProducer, error) {
	return NewLucene80DocValuesProducer(
		toIndexSegmentReadState(state),
		lucene80DVDataCodec,
		lucene80DVDataExt,
		lucene80DVMetaCodec,
		lucene80DVMetaExt,
	)
}

// toIndexSegmentReadState converts codecs.SegmentReadState to
// index.SegmentReadState. Both types carry the same fields; the conversion
// is necessary because Lucene80DocValuesProducer lives in the backward_codecs
// package and takes the index-package type.
func toIndexSegmentReadState(s *codecs.SegmentReadState) *index.SegmentReadState {
	return &index.SegmentReadState{
		Directory:     s.Directory,
		SegmentInfo:   s.SegmentInfo,
		FieldInfos:    s.FieldInfos,
		SegmentSuffix: s.SegmentSuffix,
	}
}

// compile-time assertion
var _ codecs.DocValuesFormat = (*Lucene80DocValuesFormat)(nil)
