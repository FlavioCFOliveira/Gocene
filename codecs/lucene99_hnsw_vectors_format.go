// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
)

// Lucene99HnswVectorsFormat constants
const (
	// DEFAULT_MAX_CONN is the default maximum number of connections per node in the HNSW graph
	Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN = 16
	// DEFAULT_BEAM_WIDTH is the default beam width for HNSW search
	Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH = 100
	// MAXIMUM_MAX_CONN is the maximum allowed value for maxConn
	Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN = 512
	// MAXIMUM_BEAM_WIDTH is the maximum allowed value for beamWidth
	Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH = 3200
	// DEFAULT_NUM_MERGE_WORKER is the default number of merge workers
	Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER = 1
	// HNSW_GRAPH_THRESHOLD is the threshold for tiny segments
	Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD = 100
)

// Lucene99HnswVectorsFormat implements the KnnVectorsFormat interface for Lucene 9.9+.
// This format uses HNSW (Hierarchical Navigable Small World) graphs for efficient
// approximate nearest neighbor search on vector data.
type Lucene99HnswVectorsFormat struct {
	*BaseKnnVectorsFormat
	maxConn               int
	beamWidth             int
	tinySegmentsThreshold int
	numMergeWorkers       int
}

// NewLucene99HnswVectorsFormat creates a new Lucene99HnswVectorsFormat with default settings.
func NewLucene99HnswVectorsFormat() (*Lucene99HnswVectorsFormat, error) {
	return NewLucene99HnswVectorsFormatWithParams(
		Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
		Lucene99HnswVectorsFormat_HNSW_GRAPH_THRESHOLD,
	)
}

// NewLucene99HnswVectorsFormatWithParams creates a new Lucene99HnswVectorsFormat with custom parameters.
func NewLucene99HnswVectorsFormatWithParams(maxConn, beamWidth, tinySegmentsThreshold int) (*Lucene99HnswVectorsFormat, error) {
	if err := validateLucene99HnswVectorsFormatParams(maxConn, beamWidth); err != nil {
		return nil, err
	}

	return &Lucene99HnswVectorsFormat{
		BaseKnnVectorsFormat:  NewBaseKnnVectorsFormat("Lucene99HnswVectorsFormat"),
		maxConn:               maxConn,
		beamWidth:             beamWidth,
		tinySegmentsThreshold: tinySegmentsThreshold,
		numMergeWorkers:       Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER,
	}, nil
}

// validateLucene99HnswVectorsFormatParams validates the format parameters.
func validateLucene99HnswVectorsFormatParams(maxConn, beamWidth int) error {
	if maxConn <= 0 || maxConn > Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN {
		return fmt.Errorf("maxConn must be positive and less than or equal to %d; maxConn=%d",
			Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, maxConn)
	}
	if beamWidth <= 0 || beamWidth > Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH {
		return fmt.Errorf("beamWidth must be positive and less than or equal to %d; beamWidth=%d",
			Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH, beamWidth)
	}
	return nil
}

// MaxConn returns the maximum number of connections per node.
func (f *Lucene99HnswVectorsFormat) MaxConn() int {
	return f.maxConn
}

// BeamWidth returns the beam width for search.
func (f *Lucene99HnswVectorsFormat) BeamWidth() int {
	return f.beamWidth
}

// TinySegmentsThreshold returns the threshold for tiny segments.
func (f *Lucene99HnswVectorsFormat) TinySegmentsThreshold() int {
	return f.tinySegmentsThreshold
}

// NumMergeWorkers returns the number of merge workers.
func (f *Lucene99HnswVectorsFormat) NumMergeWorkers() int {
	return f.numMergeWorkers
}

// String returns a string representation of this format.
func (f *Lucene99HnswVectorsFormat) String() string {
	return fmt.Sprintf("Lucene99HnswVectorsFormat(name=Lucene99HnswVectorsFormat, maxConn=%d, beamWidth=%d, tinySegmentsThreshold=%d, flatVectorFormat=Lucene99FlatVectorsFormat(vectorsScorer=%s()))",
		f.maxConn, f.beamWidth, f.tinySegmentsThreshold, "DefaultFlatVectorScorer")
}

// FieldsWriter returns a writer for writing KNN vectors.
func (f *Lucene99HnswVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return NewLucene99HnswVectorsWriter(state, f.maxConn, f.beamWidth, f.tinySegmentsThreshold, f.numMergeWorkers)
}

// FieldsReader returns a reader for reading KNN vectors.
func (f *Lucene99HnswVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return NewLucene99HnswVectorsReader(state)
}

// SupportsFloatVectorFallback returns false as this format does not support float vector fallback.
func (f *Lucene99HnswVectorsFormat) SupportsFloatVectorFallback() bool {
	return false
}
