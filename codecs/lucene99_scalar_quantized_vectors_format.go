// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
)

// Lucene99ScalarQuantizedVectorsFormat supports vector quantization, storage, and retrieval.
type Lucene99ScalarQuantizedVectorsFormat struct {
	confidenceInterval float32
	bits               byte
	compress           bool
	flatVectorScorer   FlatVectorsScorer
}

func NewLucene99ScalarQuantizedVectorsFormat(confidenceInterval *float32, bits int, compress bool) (*Lucene99ScalarQuantizedVectorsFormat, error) {
	if confidenceInterval != nil && *confidenceInterval != DynamicConfidenceInterval &&
		(*confidenceInterval < MinimumConfidenceInterval || *confidenceInterval > MaximumConfidenceInterval) {
		return nil, fmt.Errorf("confidenceInterval must be between %f and %f or 0; confidenceInterval=%f",
			MinimumConfidenceInterval, MaximumConfidenceInterval, *confidenceInterval)
	}

	allowedBits := (1 << 8) | (1 << 7) | (1 << 4)
	if bits < 1 || bits > 8 || (allowedBits&(1<<bits)) == 0 {
		return nil, fmt.Errorf("bits must be one of: 4, 7; bits=%d", bits)
	}

	if bits > 4 && compress {
		return nil, fmt.Errorf("compress=true only applies when bits=4")
	}

	return &Lucene99ScalarQuantizedVectorsFormat{
		confidenceInterval: 0, // set below if provided
		bits:               byte(bits),
		compress:           compress,
		flatVectorScorer:   GetLucene99ScalarQuantizedVectorsScorer(),
	}, nil
}

func (f *Lucene99ScalarQuantizedVectorsFormat) Name() string {
	return "Lucene99ScalarQuantizedVectorsFormat"
}

func (f *Lucene99ScalarQuantizedVectorsFormat) FieldsWriter(state SegmentWriteState) (FlatVectorsWriter, error) {
	return nil, fmt.Errorf("Lucene99ScalarQuantizedVectorsFormat: fieldsWriter is not supported (old codecs may only be used for reading)")
}

func (f *Lucene99ScalarQuantizedVectorsFormat) FieldsReader(state SegmentReadState) (FlatVectorsReader, error) {
	rawReader, err := NewLucene99FlatVectorsFormat().FieldsReader(state)
	if err != nil {
		return nil, err
	}
	return NewLucene99ScalarQuantizedVectorsReader(state, rawReader, f.flatVectorScorer)
}

const (
	DirectMonotonicBlockShift = 16
	ScalarQuantizedName       = "Lucene99ScalarQuantizedVectorsFormat"
	ScalarQuantizedVersionStart  = 0
	ScalarQuantizedVersionAddBits = 1
	ScalarQuantizedVersionCurrent = ScalarQuantizedVersionAddBits
	ScalarQuantizedMetaCodecName    = "Lucene99ScalarQuantizedVectorsFormatMeta"
	ScalarQuantizedVectorDataCodecName = "Lucene99ScalarQuantizedVectorsFormatData"
	ScalarQuantizedMetaExtension     = "vemq"
	ScalarQuantizedVectorDataExtension = "veq"

	MinimumConfidenceInterval = 0.9
	MaximumConfidenceInterval = 1.0
	DynamicConfidenceInterval = 0.0
)

func CalculateDefaultConfidenceInterval(vectorDimension int) float32 {
	ci := 1.0 - (1.0 / float32(vectorDimension+1))
	if ci < MinimumConfidenceInterval {
		return MinimumConfidenceInterval
	}
	return ci
}
