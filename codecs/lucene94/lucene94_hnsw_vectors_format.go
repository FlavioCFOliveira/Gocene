// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

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

// Lucene94HnswVectorsFormat implements the KnnVectorsFormat interface for Lucene 9.4.
type Lucene94HnswVectorsFormat struct {
	*codecs.BaseKnnVectorsFormat
	maxConn   int
	beamWidth int
}

// NewLucene94HnswVectorsFormat creates a new Lucene94HnswVectorsFormat with default settings.
func NewLucene94HnswVectorsFormat() *Lucene94HnswVectorsFormat {
	return NewLucene94HnswVectorsFormatWithParams(DefaultMaxConn, DefaultBeamWidth)
}

// NewLucene94HnswVectorsFormatWithParams creates a new Lucene94HnswVectorsFormat with custom parameters.
func NewLucene94HnswVectorsFormatWithParams(maxConn, beamWidth int) *Lucene94HnswVectorsFormat {
	return &Lucene94HnswVectorsFormat{
		BaseKnnVectorsFormat: codecs.NewBaseKnnVectorsFormat("Lucene94HnswVectorsFormat"),
		maxConn:              maxConn,
		beamWidth:            beamWidth,
	}
}

// FieldsWriter returns a writer for writing KNN vectors.
func (f *Lucene94HnswVectorsFormat) FieldsWriter(state *index.SegmentWriteState) (spi.KnnVectorsWriter, error) {
	// Old codecs may only be used for reading.
	return nil, fmt.Errorf("Lucene94HnswVectorsFormat: FieldsWriter is not supported (old codecs may only be used for reading)")
}

// FieldsReader returns a reader for reading KNN vectors.
func (f *Lucene94HnswVectorsFormat) FieldsReader(state *index.SegmentReadState) (spi.KnnVectorsReader, error) {
	// In a full implementation, this would return a Lucene94HnswVectorsReader.
	return nil, fmt.Errorf("Lucene94HnswVectorsFormat: FieldsReader is not yet implemented")
}

// GetMaxDimensions returns the maximum supported dimensions.
func (f *Lucene94HnswVectorsFormat) GetMaxDimensions(fieldName string) int {
	return 1024
}

// String returns a string representation of this format.
func (f *Lucene94HnswVectorsFormat) String() string {
	return fmt.Sprintf("Lucene94HnswVectorsFormat(name=Lucene94HnswVectorsFormat, maxConn=%d, beamWidth=%d)",
		f.maxConn, f.beamWidth)
}
