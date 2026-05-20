// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"errors"
	"fmt"
)

// Public constants from org.apache.lucene.backward_codecs.lucene95.Lucene95HnswVectorsFormat.
const (
	// Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN is the default maximum connections per HNSW node.
	Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN = 16

	// Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH is the default candidate-queue size.
	Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH = 100

	// Lucene95HnswVectorsFormat_MAX_DIMENSIONS is the maximum vector dimension supported (1024).
	Lucene95HnswVectorsFormat_MAX_DIMENSIONS = 1024
)

// ErrLucene95WriteUnsupported is returned when write operations are attempted on
// the read-only Lucene 9.5 HNSW vectors format.
var ErrLucene95WriteUnsupported = errors.New("old codecs may only be used for reading")

// Lucene95HnswVectorsFormat is the Lucene 9.5 KNN vectors format.
//
// It encodes float32 and byte vector values together with an optional HNSW
// graph. The format is strictly read-only in Gocene; any attempt to obtain a
// writer returns ErrLucene95WriteUnsupported.
//
// Port of org.apache.lucene.backward_codecs.lucene95.Lucene95HnswVectorsFormat.
type Lucene95HnswVectorsFormat struct {
	maxConn   int
	beamWidth int
}

// NewLucene95HnswVectorsFormat returns a Lucene95HnswVectorsFormat with default parameters.
func NewLucene95HnswVectorsFormat() *Lucene95HnswVectorsFormat {
	return NewLucene95HnswVectorsFormatWithParams(
		Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
	)
}

// NewLucene95HnswVectorsFormatWithParams returns a Lucene95HnswVectorsFormat with
// custom maxConn and beamWidth.
func NewLucene95HnswVectorsFormatWithParams(maxConn, beamWidth int) *Lucene95HnswVectorsFormat {
	return &Lucene95HnswVectorsFormat{
		maxConn:   maxConn,
		beamWidth: beamWidth,
	}
}

// Name returns the codec name "Lucene95HnswVectorsFormat".
func (f *Lucene95HnswVectorsFormat) Name() string {
	return "Lucene95HnswVectorsFormat"
}

// MaxConn returns the maximum number of connections per HNSW node.
func (f *Lucene95HnswVectorsFormat) MaxConn() int { return f.maxConn }

// BeamWidth returns the candidate-queue size used during graph construction.
func (f *Lucene95HnswVectorsFormat) BeamWidth() int { return f.beamWidth }

// GetMaxDimensions returns the maximum number of vector dimensions supported (1024).
func (f *Lucene95HnswVectorsFormat) GetMaxDimensions(_ string) int {
	return Lucene95HnswVectorsFormat_MAX_DIMENSIONS
}

// FieldsWriter is not supported for this read-only backward format.
// It always returns ErrLucene95WriteUnsupported.
func (f *Lucene95HnswVectorsFormat) FieldsWriter() error {
	return ErrLucene95WriteUnsupported
}

// String returns a human-readable description matching the Java toString() output.
func (f *Lucene95HnswVectorsFormat) String() string {
	return fmt.Sprintf(
		"Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=%d, beamWidth=%d)",
		f.maxConn, f.beamWidth,
	)
}
