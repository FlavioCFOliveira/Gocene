// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

const (
	DefaultMaxConn   = 16
	DefaultBeamWidth = 100
)

// Lucene95HnswVectorsFormat implements the KnnVectorsFormat interface for Lucene 9.5.
type Lucene95HnswVectorsFormat struct {
	*codecs.BaseKnnVectorsFormat
	maxConn   int
	beamWidth int
}

// NewLucene95HnswVectorsFormat creates a new Lucene95HnswVectorsFormat with default settings.
func NewLucene95HnswVectorsFormat() *Lucene95HnswVectorsFormat {
	return NewLucene95HnswVectorsFormatWithParams(DefaultMaxConn, DefaultBeamWidth)
}

// NewLucene95HnswVectorsFormatWithParams creates a new Lucene95HnswVectorsFormat with custom parameters.
func NewLucene95HnswVectorsFormatWithParams(maxConn, beamWidth int) *Lucene95HnswVectorsFormat {
	return &Lucene95HnswVectorsFormat{
		BaseKnnVectorsFormat: codecs.NewBaseKnnVectorsFormat("Lucene95HnswVectorsFormat"),
		maxConn:              maxConn,
		beamWidth:            beamWidth,
	}
}

// FieldsWriter returns a writer for writing KNN vectors.
func (f *Lucene95HnswVectorsFormat) FieldsWriter(state *index.SegmentWriteState) (spi.KnnVectorsWriter, error) {
	// Old codecs may only be used for reading.
	return nil, fmt.Errorf("Lucene95HnswVectorsFormat: FieldsWriter is not supported (old codecs may only be used for reading)")
}

// FieldsReader returns a reader for reading KNN vectors.
func (f *Lucene95HnswVectorsFormat) FieldsReader(state *index.SegmentReadState) (spi.KnnVectorsReader, error) {
	// In a full implementation, this would return a Lucene95HnswVectorsReader.
	return nil, fmt.Errorf("Lucene95HnswVectorsFormat: FieldsReader is not yet implemented")
}

// GetMaxDimensions returns the maximum supported dimensions.
func (f *Lucene95HnswVectorsFormat) GetMaxDimensions(fieldName string) int {
	return 1024
}

// String returns a string representation of this format.
func (f *Lucene95HnswVectorsFormat) String() string {
	return fmt.Sprintf("Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=%d, beamWidth=%d)",
		f.maxConn, f.beamWidth)
}
