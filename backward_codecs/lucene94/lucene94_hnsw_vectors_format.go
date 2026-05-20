// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"errors"
	"fmt"
)

// Public constants from org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsFormat.
const (
	// Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN is the default maximum connections per HNSW node.
	Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN = 16

	// Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH is the default candidate-queue size.
	Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH = 100

	// Lucene94HnswVectorsFormat_MAX_DIMENSIONS is the maximum vector dimension supported (1024).
	Lucene94HnswVectorsFormat_MAX_DIMENSIONS = 1024
)

// ErrLucene94WriteUnsupported is returned when write operations are attempted on
// the read-only Lucene 9.4 HNSW vectors format.
var ErrLucene94WriteUnsupported = errors.New("old codecs may only be used for reading")

// Lucene94HnswVectorsFormat is the Lucene 9.4 KNN vectors format.
//
// It encodes float32 and byte vector values together with an optional HNSW
// graph. The format is strictly read-only in Gocene; any attempt to obtain a
// writer returns ErrLucene94WriteUnsupported.
//
// Port of org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsFormat.
type Lucene94HnswVectorsFormat struct {
	maxConn   int
	beamWidth int
}

// NewLucene94HnswVectorsFormat returns a Lucene94HnswVectorsFormat with default parameters.
func NewLucene94HnswVectorsFormat() *Lucene94HnswVectorsFormat {
	return NewLucene94HnswVectorsFormatWithParams(
		Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
	)
}

// NewLucene94HnswVectorsFormatWithParams returns a Lucene94HnswVectorsFormat with
// custom maxConn and beamWidth.
func NewLucene94HnswVectorsFormatWithParams(maxConn, beamWidth int) *Lucene94HnswVectorsFormat {
	return &Lucene94HnswVectorsFormat{
		maxConn:   maxConn,
		beamWidth: beamWidth,
	}
}

// Name returns the codec name "Lucene94HnswVectorsFormat".
func (f *Lucene94HnswVectorsFormat) Name() string {
	return "Lucene94HnswVectorsFormat"
}

// MaxConn returns the maximum number of connections per HNSW node.
func (f *Lucene94HnswVectorsFormat) MaxConn() int { return f.maxConn }

// BeamWidth returns the candidate-queue size used during graph construction.
func (f *Lucene94HnswVectorsFormat) BeamWidth() int { return f.beamWidth }

// GetMaxDimensions returns the maximum number of vector dimensions supported (1024).
func (f *Lucene94HnswVectorsFormat) GetMaxDimensions(_ string) int {
	return Lucene94HnswVectorsFormat_MAX_DIMENSIONS
}

// FieldsWriter is not supported for this read-only backward format.
// It always returns ErrLucene94WriteUnsupported.
func (f *Lucene94HnswVectorsFormat) FieldsWriter() error {
	return ErrLucene94WriteUnsupported
}

// String returns a human-readable description matching the Java toString() output.
func (f *Lucene94HnswVectorsFormat) String() string {
	return fmt.Sprintf(
		"Lucene94HnswVectorsFormat(name=Lucene94HnswVectorsFormat, maxConn=%d, beamWidth=%d)",
		f.maxConn, f.beamWidth,
	)
}
