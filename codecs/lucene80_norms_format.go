// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
)

// Lucene80NormsFormat implements the Lucene 8.0 Norms format.
type Lucene80NormsFormat struct{}

func NewLucene80NormsFormat() *Lucene80NormsFormat {
	return &Lucene80NormsFormat{}
}

func (f *Lucene80NormsFormat) Name() string {
	return "Lucene80Norms"
}

func (f *Lucene80NormsFormat) NormsConsumer(state SegmentWriteState) (NormsConsumer, error) {
	// Old codecs may only be used for reading
	return nil, fmt.Errorf("Lucene80NormsFormat: normsConsumer is not supported (old codecs may only be used for reading)")
}

func (f *Lucene80NormsFormat) NormsProducer(state SegmentReadState) (NormsProducer, error) {
	return NewLucene80NormsProducer(
		state,
		NormsDataCodec,
		NormsDataExtension,
		NormsMetaCodec,
		NormsMetaExtension), nil
}

const (
	NormsDataCodec    = "Lucene80NormsData"
	NormsDataExtension = "nvd"
	NormsMetaCodec    = "Lucene80NormsMetadata"
	NormsMetaExtension = "nvm"
	NormsVersionStart  = 0
	NormsVersionCurrent = NormsVersionStart
)
