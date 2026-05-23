// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// Lucene80NormsFormat is the Lucene 8.0 score normalization format.
//
// Encodes normalization values using the minimum number of bytes needed
// to represent the range (which can be zero). Files: .nvm (metadata),
// .nvd (data).
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80NormsFormat
// (Lucene 10.4.0).
//
// NormsConsumer is not supported — old codecs may only be used for reading.
// NormsProducer opens the .nvm/.nvd files via Lucene80NormsProducer.
type Lucene80NormsFormat struct {
	*codecs.BaseNormsFormat
}

// NewLucene80NormsFormat creates a new Lucene80NormsFormat.
//
// Port of Lucene80NormsFormat() (Lucene 10.4.0).
func NewLucene80NormsFormat() *Lucene80NormsFormat {
	return &Lucene80NormsFormat{
		BaseNormsFormat: codecs.NewBaseNormsFormat("Lucene80"),
	}
}

// NormsConsumer always returns an error; old codecs are read-only.
//
// Port of Lucene80NormsFormat.normsConsumer (Lucene 10.4.0):
// throws UnsupportedOperationException("Old codecs may only be used for reading").
func (f *Lucene80NormsFormat) NormsConsumer(_ *codecs.SegmentWriteState) (codecs.NormsConsumer, error) {
	return nil, fmt.Errorf("lucene80 norms: old codecs may only be used for reading")
}

// NormsProducer opens .nvm/.nvd and returns a Lucene80NormsProducer.
//
// Port of Lucene80NormsFormat.normsProducer (Lucene 10.4.0).
func (f *Lucene80NormsFormat) NormsProducer(state *codecs.SegmentReadState) (codecs.NormsProducer, error) {
	return NewLucene80NormsProducer(
		toIndexSegmentReadState(state),
		lucene80NormsDataCodec,
		lucene80NormsDataExt,
		lucene80NormsMetaCodec,
		lucene80NormsMetaExt,
	)
}

// compile-time assertion
var _ codecs.NormsFormat = (*Lucene80NormsFormat)(nil)
