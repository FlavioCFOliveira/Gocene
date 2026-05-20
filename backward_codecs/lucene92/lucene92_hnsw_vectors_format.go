// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"errors"
	"fmt"
)

// File-format constants from org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsFormat.
const (
	// Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN is the default maximum connections per HNSW node.
	Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN = 16

	// Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH is the default queue size during graph construction.
	Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH = 100

	// Lucene92HnswVectorsFormat_MAX_DIMENSIONS is the maximum vector dimension supported.
	Lucene92HnswVectorsFormat_MAX_DIMENSIONS = 1024

	lucene92MetaCodecName        = "lucene92HnswVectorsFormatMeta"
	lucene92VectorDataCodecName  = "lucene92HnswVectorsFormatData"
	lucene92VectorIndexCodecName = "lucene92HnswVectorsFormatIndex"
	lucene92MetaExtension        = "vem"
	lucene92VectorDataExtension  = "vec"
	lucene92VectorIndexExtension = "vex"
	lucene92VersionStart         = 0
	lucene92VersionCurrent       = lucene92VersionStart
)

// ErrLucene92WriteUnsupported is returned when write operations are attempted on
// the read-only Lucene 9.2 HNSW vectors format.
var ErrLucene92WriteUnsupported = errors.New("old codecs may only be used for reading")

// Lucene92HnswVectorsFormat is the Lucene 9.2 KNN vectors format.
//
// It encodes float32 vector values together with an optional HNSW graph
// connecting documents.  The format is strictly read-only in Gocene; any
// attempt to obtain a writer returns ErrLucene92WriteUnsupported.
//
// Port of org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsFormat.
type Lucene92HnswVectorsFormat struct {
	maxConn   int
	beamWidth int
}

// NewLucene92HnswVectorsFormat returns a Lucene92HnswVectorsFormat with default parameters.
func NewLucene92HnswVectorsFormat() *Lucene92HnswVectorsFormat {
	return NewLucene92HnswVectorsFormatWithParams(
		Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
	)
}

// NewLucene92HnswVectorsFormatWithParams returns a Lucene92HnswVectorsFormat with
// custom maxConn and beamWidth. These values are stored for informational purposes
// only — they are not applied during writing (write is unsupported).
func NewLucene92HnswVectorsFormatWithParams(maxConn, beamWidth int) *Lucene92HnswVectorsFormat {
	return &Lucene92HnswVectorsFormat{
		maxConn:   maxConn,
		beamWidth: beamWidth,
	}
}

// Name returns the codec name "lucene92HnswVectorsFormat".
func (f *Lucene92HnswVectorsFormat) Name() string {
	return "lucene92HnswVectorsFormat"
}

// MaxConn returns the maximum number of connections per HNSW node.
func (f *Lucene92HnswVectorsFormat) MaxConn() int { return f.maxConn }

// BeamWidth returns the candidate-queue size used during graph construction.
func (f *Lucene92HnswVectorsFormat) BeamWidth() int { return f.beamWidth }

// GetMaxDimensions returns the maximum number of vector dimensions supported (1024).
func (f *Lucene92HnswVectorsFormat) GetMaxDimensions(_ string) int {
	return Lucene92HnswVectorsFormat_MAX_DIMENSIONS
}

// FieldsWriter is not supported for this read-only backward format.
// It always returns ErrLucene92WriteUnsupported.
func (f *Lucene92HnswVectorsFormat) FieldsWriter() error {
	return ErrLucene92WriteUnsupported
}

// String returns a human-readable description of this format, matching the
// Java toString() output for byte-level compatibility of diagnostic messages.
func (f *Lucene92HnswVectorsFormat) String() string {
	return fmt.Sprintf(
		"Lucene92HnswVectorsFormat(name = Lucene92HnswVectorsFormat, maxConn = %d, beamWidth=%d)",
		f.maxConn, f.beamWidth,
	)
}
