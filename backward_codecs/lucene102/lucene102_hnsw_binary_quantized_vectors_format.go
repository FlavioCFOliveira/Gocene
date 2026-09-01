// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
)

var (
	// flatVectorsFormat is the format for storing, reading, merging vectors on disk.
	// It is an instance of Lucene102BinaryQuantizedVectorsFormat.
	flatVectorsFormat = &Lucene102BinaryQuantizedVectorsFormat{}
)

// Lucene102HnswBinaryQuantizedVectorsFormat uses an HNSW graph to store and search for vectors,
// where vectors are binary quantized using Lucene102BinaryQuantizedVectorsFormat before being stored.
//
// This is the Go port of org.apache.lucene.backward_codecs.lucene102.Lucene102HnswBinaryQuantizedVectorsFormat.
type Lucene102HnswBinaryQuantizedVectorsFormat struct {
	*codecs.BaseKnnVectorsFormat
	maxConn         int
	beamWidth       int
	numMergeWorkers int
	mergeExec       io.Closer
}

// NewLucene102HnswBinaryQuantizedVectorsFormat creates a format using default graph construction parameters.
func NewLucene102HnswBinaryQuantizedVectorsFormat() *Lucene102HnswBinaryQuantizedVectorsFormat {
	return NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(
		codecs.Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
		codecs.Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
		codecs.Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER,
		nil,
	)
}

// NewLucene102HnswBinaryQuantizedVectorsFormatWithParams creates a format using the given graph construction parameters.
func NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(maxConn, beamWidth, numMergeWorkers int, mergeExec io.Closer) *Lucene102HnswBinaryQuantizedVectorsFormat {
	if maxConn <= 0 || maxConn > codecs.Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN {
		panic(fmt.Sprintf("maxConn must be positive and less than or equal to %d; maxConn=%d",
			codecs.Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, maxConn))
	}
	if beamWidth <= 0 || beamWidth > codecs.Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH {
		panic(fmt.Sprintf("beamWidth must be positive and less than or equal to %d; beamWidth=%d",
			codecs.Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH, beamWidth))
	}
	if numMergeWorkers == 1 && mergeExec != nil {
		panic("No executor service is needed as we'll use single thread to merge")
	}

	return &Lucene102HnswBinaryQuantizedVectorsFormat{
		BaseKnnVectorsFormat: codecs.NewBaseKnnVectorsFormat("Lucene102HnswBinaryQuantizedVectorsFormat"),
		maxConn:              maxConn,
		beamWidth:            beamWidth,
		numMergeWorkers:      numMergeWorkers,
		mergeExec:            mergeExec,
	}
}

// FieldsWriter is not supported for old codecs.
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) FieldsWriter(state *index.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
	return nil, fmt.Errorf("UnsupportedOperationException: Old codecs may only be used for reading")
}

// FieldsReader returns a reader for the binary quantized HNSW vectors.
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) FieldsReader(state *index.SegmentReadState) (codecs.KnnVectorsReader, error) {
	// In Java: return new Lucene99HnswVectorsReader(state, flatVectorsFormat.fieldsReader(state));
	// In Gocene, NewLucene99HnswVectorsReader currently opens its own flat reader internally.
	return codecs.NewLucene99HnswVectorsReader(state)
}

// GetMaxDimensions returns the maximum supported dimensions.
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) GetMaxDimensions(fieldName string) int {
	return 1024
}

func (f *Lucene102HnswBinaryQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene102HnswBinaryQuantizedVectorsFormat(name=Lucene102HnswBinaryQuantizedVectorsFormat, maxConn=%d, beamWidth=%d, flatVectorFormat=%v)",
		f.maxConn, f.beamWidth, flatVectorsFormat)
}
